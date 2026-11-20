package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"apex-backend/models"
)

// ─── Meta Conversions API — truy cập lớp dữ liệu ───────────────────────

// GetFacebookCAPIConfig lấy cấu hình CAPI của 1 landing.
//
// Nếu landing không có cấu hình riêng → trả về cấu hình mặc định
// (landing_id = ”). Nếu không có cả cấu hình mặc định → trả nil, nil
// (nghĩa là: chưa cấu hình CAPI → không gửi gì, KHÔNG phải lỗi).
func (s *Store) GetFacebookCAPIConfig(landingID string) (*models.FacebookCAPIConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := s.queryFacebookCAPIConfig(ctx, landingID)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return cfg, nil
	}
	if landingID == "" {
		return nil, nil
	}
	// Chưa có cấu hình riêng cho landing này → thử cấu hình mặc định.
	def, defErr := s.queryFacebookCAPIConfig(ctx, "")
	if defErr != nil {
		return nil, defErr
	}
	return def, nil
}

// GetFacebookCAPIConfigExact lấy ĐÚNG dòng cấu hình của landing (KHÔNG fallback
// sang cấu hình mặc định).
//
// Dùng cho Admin UI khi cần biết "landing này có dòng riêng trong DB hay chưa":
//   - landing_id = "" ⇒ dòng cấu hình MẶC ĐỊNH (landing_id = '' trong DB).
//   - không có dòng ⇒ trả nil, nil (chưa cấu hình, KHÔNG phải lỗi).
func (s *Store) GetFacebookCAPIConfigExact(landingID string) (*models.FacebookCAPIConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.queryFacebookCAPIConfig(ctx, landingID)
}

func (s *Store) queryFacebookCAPIConfig(ctx context.Context, landingID string) (*models.FacebookCAPIConfig, error) {
	const q = `
		SELECT id, landing_id, is_active, pixel_id, access_token, api_version,
		       gateway_enabled, gateway_endpoint, gateway_access_key,
		       send_page_view, send_lead, send_purchase, send_crm_sync,
		       only_facebook_traffic,
		       course_value, currency,
		       hash_email, hash_phone, hash_name, hash_address, hash_external_id,
		       hash_city, hash_state, hash_zip, hash_country, send_user_data,
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM facebook_capi_configs
		WHERE landing_id = $1
	`
	var c models.FacebookCAPIConfig
	err := s.pool.QueryRow(ctx, q, landingID).Scan(
		&c.ID, &c.LandingID, &c.IsActive, &c.PixelID, &c.AccessToken, &c.APIVersion,
		&c.GatewayEnabled, &c.GatewayEndpoint, &c.GatewayAccessKey,
		&c.SendPageView, &c.SendLead, &c.SendPurchase, &c.SendCRMSync,
		&c.OnlyFacebookTraffic,
		&c.CourseValue, &c.Currency,
		&c.HashEmail, &c.HashPhone, &c.HashName, &c.HashAddress, &c.HashExternalID,
		&c.HashCity, &c.HashState, &c.HashZip, &c.HashCountry, &c.SendUserData,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		// Không có cấu hình ⇒ KHÔNG phải lỗi, chỉ là chưa bật CAPI.
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// ListFacebookCAPIConfigs trả về toàn bộ cấu hình CAPI (token đã che).
func (s *Store) ListFacebookCAPIConfigs() ([]models.FacebookCAPIConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `
		SELECT id, landing_id, is_active, pixel_id, access_token, api_version,
		       gateway_enabled, gateway_endpoint, gateway_access_key,
		       send_page_view, send_lead, send_purchase, send_crm_sync,
		       only_facebook_traffic,
		       course_value, currency,
		       hash_email, hash_phone, hash_name, hash_address, hash_external_id,
		       hash_city, hash_state, hash_zip, hash_country, send_user_data,
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM facebook_capi_configs
		ORDER BY (landing_id = '') DESC, landing_id ASC
	`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.FacebookCAPIConfig, 0)
	for rows.Next() {
		var c models.FacebookCAPIConfig
		if err := rows.Scan(
			&c.ID, &c.LandingID, &c.IsActive, &c.PixelID, &c.AccessToken, &c.APIVersion,
			&c.GatewayEnabled, &c.GatewayEndpoint, &c.GatewayAccessKey,
			&c.SendPageView, &c.SendLead, &c.SendPurchase, &c.SendCRMSync,
			&c.OnlyFacebookTraffic,
			&c.CourseValue, &c.Currency,
			&c.HashEmail, &c.HashPhone, &c.HashName, &c.HashAddress, &c.HashExternalID,
			&c.HashCity, &c.HashState, &c.HashZip, &c.HashCountry, &c.SendUserData,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			continue
		}
		c.MaskAccessToken()
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertFacebookCAPIConfig tạo hoặc cập nhật cấu hình CAPI.
//
// ⚠️ Access token & gateway key: nếu input để trống thì GIỮ NGUYÊN giá trị
// cũ trong DB (không ghi đè bằng rỗng). Lý do: Admin UI gửi token đã che
// (dạng `abcd****wxyz`) khi load → nếu ghi thẳng sẽ hỏng token thật.
func (s *Store) UpsertFacebookCAPIConfig(in models.FacebookCAPIConfigInput) (*models.FacebookCAPIConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	isActive := false
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	apiVersion := in.APIVersion
	if apiVersion == "" {
		apiVersion = "v27.0"
	}
	currency := in.Currency
	if currency == "" {
		currency = "VND"
	}
	courseValue := in.CourseValue
	if courseValue <= 0 {
		courseValue = 6000000
	}
	// `only_facebook_traffic` mặc định TRUE (chỉ gửi lead từ Facebook).
	onlyFB := true
	if in.OnlyFacebookTraffic != nil {
		onlyFB = *in.OnlyFacebookTraffic
	}

	const q = `
		INSERT INTO facebook_capi_configs (
			landing_id, is_active, pixel_id, access_token, api_version,
			gateway_enabled, gateway_endpoint, gateway_access_key,
			send_page_view, send_lead, send_purchase, send_crm_sync,
			only_facebook_traffic,
			course_value, currency,
			hash_email, hash_phone, hash_name, hash_address, hash_external_id,
			hash_city, hash_state, hash_zip, hash_country, send_user_data,
			created_at, updated_at
		) VALUES (
			$1, $2, $3,
			CASE WHEN $4 = '' THEN '' ELSE $4 END,
			$5, $6, $7,
			CASE WHEN $8 = '' THEN '' ELSE $8 END,
			$9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25,
			NOW(), NOW()
		)
		ON CONFLICT (landing_id) DO UPDATE SET
			is_active = EXCLUDED.is_active,
			pixel_id = EXCLUDED.pixel_id,
			access_token = CASE
				WHEN $4 = '' THEN facebook_capi_configs.access_token
				ELSE EXCLUDED.access_token
			END,
			api_version = EXCLUDED.api_version,
			gateway_enabled = EXCLUDED.gateway_enabled,
			gateway_endpoint = EXCLUDED.gateway_endpoint,
			gateway_access_key = CASE
				WHEN $8 = '' THEN facebook_capi_configs.gateway_access_key
				ELSE EXCLUDED.gateway_access_key
			END,
			send_page_view = EXCLUDED.send_page_view,
			send_lead = EXCLUDED.send_lead,
			send_purchase = EXCLUDED.send_purchase,
			send_crm_sync = EXCLUDED.send_crm_sync,
			only_facebook_traffic = EXCLUDED.only_facebook_traffic,
			course_value = EXCLUDED.course_value,
			currency = EXCLUDED.currency,
			hash_email = EXCLUDED.hash_email,
			hash_phone = EXCLUDED.hash_phone,
			hash_name = EXCLUDED.hash_name,
			hash_address = EXCLUDED.hash_address,
			hash_external_id = EXCLUDED.hash_external_id,
			hash_city = EXCLUDED.hash_city,
			hash_state = EXCLUDED.hash_state,
			hash_zip = EXCLUDED.hash_zip,
			hash_country = EXCLUDED.hash_country,
			send_user_data = EXCLUDED.send_user_data,
			updated_at = NOW()
		RETURNING id
	`
	var id uint64
	err := s.pool.QueryRow(ctx, q,
		in.LandingID, isActive, in.PixelID, in.AccessToken, apiVersion,
		in.GatewayEnabled, in.GatewayEndpoint, in.GatewayAccessKey,
		in.SendPageView, in.SendLead, in.SendPurchase, in.SendCRMSync,
		onlyFB,
		courseValue, currency,
		in.HashEmail, in.HashPhone, in.HashName, in.HashAddress, in.HashExternalID,
		in.HashCity, in.HashState, in.HashZip, in.HashCountry, in.SendUserData,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("lỗi lưu cấu hình CAPI: %w", err)
	}

	cfg, err := s.queryFacebookCAPIConfig(ctx, in.LandingID)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// DeleteFacebookCAPIConfig xoá cấu hình CAPI của 1 landing.
//
// ⚠️ `landingID = ""` ⇒ xoá đúng DÒNG CẤU HÌNH MẶC ĐỊNH (`landing_id = ''`),
// KHÔNG phải xoá toàn bộ bảng. Truy vấn luôn khớp CHÍNH XÁC `landing_id` nên
// không bao giờ xoá nhầm cấu hình của landing khác.
func (s *Store) DeleteFacebookCAPIConfig(landingID string) (deleted int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ct, err := s.pool.Exec(ctx, `DELETE FROM facebook_capi_configs WHERE landing_id = $1`, landingID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// ToggleFacebookCAPIConfig bật/tắt nhanh CAPI cho 1 landing mà không cần
// gửi lại toàn bộ cấu hình (token, pixel, gateway…).
//
// ⚠️ Truy vấn khớp CHÍNH XÁC `landing_id`:
//   - `landingID = "abc"` ⇒ toggle dòng cấu hình riêng của landing `abc`.
//   - `landingID = ""`   ⇒ toggle DÒNG CẤU HÌNH MẶC ĐỊNH (landing_id = '')
//     — tắt dòng này ⇒ mọi landing CHƯA có cấu hình riêng cũng ngừng gửi
//     CAPI (fallback về cấu hình mặc định ⇒ `is_active = false` ⇒ dispatcher
//     bỏ qua). Landing ĐÃ có cấu hình riêng không bị ảnh hưởng.
//
// Trả về `found=false` nếu không có dòng nào khớp `landing_id`.
func (s *Store) ToggleFacebookCAPIConfig(landingID string, isActive bool) (found bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ct, err := s.pool.Exec(ctx,
		`UPDATE facebook_capi_configs SET is_active = $2, updated_at = NOW() WHERE landing_id = $1`,
		landingID, isActive)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}

// LogFacebookCAPIEvent ghi log 1 lần gửi sự kiện lên Meta.
func (s *Store) LogFacebookCAPIEvent(e models.FacebookCAPIEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	respBytes := e.Response
	if len(respBytes) == 0 {
		respBytes = []byte("{}")
	}
	var leadID interface{}
	if e.LeadID > 0 {
		leadID = int64(e.LeadID)
	}

	const q = `
		INSERT INTO facebook_capi_logs (
			lead_id, landing_id, event_name, event_id, action_source, pixel_id,
			status, http_status, response, error, triggered_by, trang_thai_crm,
			value, currency, lead_score, session_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $14, $15, $16, NOW())
	`
	_, err := s.pool.Exec(ctx, q,
		leadID, e.LandingID, e.EventName, e.EventID, e.ActionSource, e.PixelID,
		e.Status, e.HTTPStatus, string(respBytes), e.Error, e.TriggeredBy, e.TrangThaiCRM,
		e.Value, e.Currency, leadScoreVal(e.LeadScore),
		e.SessionID,
	)
	return err
}

// leadScoreVal đổi *float64 thành driver value (nil ⇒ NULL).
func leadScoreVal(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// HasFacebookCAPISessionEvent kiểm tra session này đã gửi sự kiện đó chưa.
//
// Dùng để dedup PageView server-side: tracker.js gửi POST
// /api/analytics/track nhiều lần trong 1 phiên (pageview, heartbeat, exit,
// scroll…) ⇒ nếu mỗi lần đều bắn PageView lên Meta thì 1 lượt xem bị
// đếm nhiều lần.
//
// Trả về true nếu ĐÃ gửi sự kiện `eventName` cho `sessionID` trong 24 giờ.
func (s *Store) HasFacebookCAPISessionEvent(sessionID, eventName string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `
		SELECT EXISTS (
			SELECT 1 FROM facebook_capi_logs
			WHERE session_id = $1
			  AND event_name = $2
			  AND status IN ('success', 'skipped')
			  AND created_at >= NOW() - INTERVAL '24 hours'
		)
	`
	var exists bool
	if err := s.pool.QueryRow(ctx, q, sessionID, eventName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// GetSessionPvEventID đọc `event_id` của PageView đã lưu theo phiên.
//
// Dùng làm FALLBACK khi client không gửi `event_id` trong payload analytics
// (vd: trình duyệt cache bản tracker.js cũ, hoặc ping đầu tiên bị throttle).
//
// ⚠️ Bắt buộc cho dedup Pixel ↔ CAPI (fix 2026-11-16): server phải dùng
// ĐÚNG event_id mà Pixel đã bắn, nếu không Meta sẽ tính 1 lượt xem thành 2.
func (s *Store) GetSessionPvEventID(sessionID string) (string, error) {
	if sessionID == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `SELECT pv_event_id FROM analytics_sessions WHERE session_id = $1`
	var eventID string
	if err := s.pool.QueryRow(ctx, q, sessionID).Scan(&eventID); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return eventID, nil
}

// ListFacebookCAPILogs trả về log gửi sự kiện (mới nhất trước).
func (s *Store) ListFacebookCAPILogs(landingID string, status string, eventName string, leadID uint64, limit int) ([]models.FacebookCAPIEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	conditions := []string{"1=1"}
	args := []interface{}{}
	add := func(cond string, val interface{}) {
		args = append(args, val)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if landingID != "" {
		add("landing_id = $%d", landingID)
	}
	if status != "" {
		add("status = $%d", status)
	}
	if eventName != "" {
		add("event_name = $%d", eventName)
	}
	if leadID > 0 {
		add("lead_id = $%d", int64(leadID))
	}
	args = append(args, limit)

	q := fmt.Sprintf(`
		SELECT id, COALESCE(lead_id, 0), landing_id, event_name, event_id,
		       action_source, pixel_id, status, http_status, response, error,
		       triggered_by, trang_thai_crm,
		       COALESCE(value, 0)::float8, COALESCE(currency, 'VND'), lead_score,
		       session_id,
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM facebook_capi_logs
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d
	`, joinConditions(conditions), len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.FacebookCAPIEvent, 0)
	for rows.Next() {
		var e models.FacebookCAPIEvent
		var respRaw []byte
		if err := rows.Scan(
			&e.ID, &e.LeadID, &e.LandingID, &e.EventName, &e.EventID,
			&e.ActionSource, &e.PixelID, &e.Status, &e.HTTPStatus, &respRaw, &e.Error,
			&e.TriggeredBy, &e.TrangThaiCRM,
			&e.Value, &e.Currency, &e.LeadScore, &e.SessionID, &e.CreatedAt,
		); err != nil {
			continue
		}
		e.Response = json.RawMessage(respRaw)
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetFacebookCAPIStats tổng hợp số liệu gửi sự kiện trong N ngày gần nhất.
func (s *Store) GetFacebookCAPIStats(landingID string, days int) (*models.FacebookCAPIStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if days <= 0 {
		days = 30
	}

	// landingID = '' ⇒ lấy TẤT CẢ landing (dùng cho báo cáo tổng).
	//
	// ⚠️ `created_at` là TIMESTAMPTZ ⇒ phải so sánh với TIMESTAMPTZ.
	// `make_interval(...)` trả về INTERVAL ⇒ KHÔNG dùng trực tiếp
	// (`timestamptz >= interval` ⇒ lỗi 42P01/42883 → HTTP 500).
	// Phải trừ khỏi NOW() (NOW() là timestamptz ⇒ timestamptz - interval OK).
	where := "(landing_id = $1 OR $1 = '') AND created_at >= NOW() - make_interval(days => $2::int)"
	args := []interface{}{landingID, days}

	q := `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE status = 'success')::int,
			COUNT(*) FILTER (WHERE status = 'failed')::int,
			COUNT(*) FILTER (WHERE status = 'skipped')::int
		FROM facebook_capi_logs
		WHERE ` + where
	var total, success, failed, skipped int
	if err := s.pool.QueryRow(ctx, q, args...).Scan(&total, &success, &failed, &skipped); err != nil {
		return nil, err
	}

	byEvent := map[string]int{}
	qEvent := `
		SELECT event_name, COUNT(*)::int
		FROM facebook_capi_logs
		WHERE ` + where + `
		GROUP BY event_name
	`
	rows, err := s.pool.Query(ctx, qEvent, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				byEvent[name] = cnt
			}
		}
	}

	byStatus := map[string]int{"success": success, "failed": failed, "skipped": skipped}

	lastError := ""
	lastErrorAt := ""
	qErr := `
		SELECT error, TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM facebook_capi_logs
		WHERE status = 'failed' AND ` + where + `
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := s.pool.QueryRow(ctx, qErr, args...).Scan(&lastError, &lastErrorAt); err == nil {
		// ghi đè chỉ khi có giá trị
	} else {
		lastError, lastErrorAt = "", ""
	}

	recent, _ := s.ListFacebookCAPILogs(landingID, "", "", 0, 20)

	return &models.FacebookCAPIStats{
		TotalSent:    total,
		TotalSuccess: success,
		TotalFailed:  failed,
		TotalSkipped: skipped,
		ByEvent:      byEvent,
		ByStatus:     byStatus,
		LastError:    lastError,
		LastErrorAt:  lastErrorAt,
		RecentEvents: recent,
	}, nil
}

// ─── Thống kê QC chiến dịch Meta (từ bảng leads) ────────────────────────

// MetaCampaignBreakdown là 1 dòng báo cáo hiệu quả theo chiến dịch.
type MetaCampaignBreakdown struct {
	CampaignID   string  `json:"campaign_id"`
	CampaignName string  `json:"campaign_name"`
	AdSetID      string  `json:"adset_id"`
	AdSetName    string  `json:"adset_name"`
	AdID         string  `json:"ad_id"`
	AdName       string  `json:"ad_name"`
	Placement    string  `json:"placement"`
	Leads        int     `json:"leads"`
	NewLeads     int     `json:"new_leads"`
	WonLeads     int     `json:"won_leads"`
	WonRate      float64 `json:"won_rate"`
	Revenue      float64 `json:"revenue"`
}

// GetMetaCampaignBreakdown tổng hợp hiệu quả theo chiến dịch / adset / ad.
//
// Trích từ `leads.tracking` (JSONB) vì macro Meta đã được tracker.js lưu sẵn:
//
//	utm_campaign = campaign.id, utm_content = adset.id, utm_term = ad.id
//	campaign_name / adset_name / ad_name = tên hiển thị
func (s *Store) GetMetaCampaignBreakdown(landingID string, nguon string, fromDate string, toDate string, limit int) ([]MetaCampaignBreakdown, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	conditions := []string{
		"COALESCE(NULLIF(tracking->>'utm_source',''), '') <> ''",
		// ⚠️ Lead test (tên chứa chuỗi "test") KHÔNG tính vào thống kê chiến dịch
		// (leads / doanh thu) — yêu cầu 2026-11-16.
		"COALESCE(is_test, FALSE) = FALSE",
	}
	args := []interface{}{}
	add := func(cond string, val interface{}) {
		args = append(args, val)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}
	if landingID != "" {
		add("landing_id = $%d", landingID)
	}
	if nguon != "" {
		add("nguon = $%d", nguon)
	}
	if fromDate != "" {
		add("(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= $%d::date", fromDate)
	}
	if toDate != "" {
		add("(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::date <= $%d::date", toDate)
	}
	args = append(args, limit)

	q := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(tracking->>'utm_campaign',''), '') AS campaign_id,
			COALESCE(NULLIF(tracking->>'campaign_name',''), '') AS campaign_name,
			COALESCE(NULLIF(tracking->>'utm_content',''), '') AS adset_id,
			COALESCE(NULLIF(tracking->>'adset_name',''), '') AS adset_name,
			COALESCE(NULLIF(tracking->>'utm_term',''), '') AS ad_id,
			COALESCE(NULLIF(tracking->>'ad_name',''), '') AS ad_name,
			COALESCE(NULLIF(tracking->>'utm_placement',''), '') AS placement,
			COUNT(*)::int AS leads,
			COUNT(*) FILTER (WHERE trang_thai IN ('moi','da_goi','ket_ban_zalo'))::int AS new_leads,
			COUNT(*) FILTER (WHERE trang_thai IN ('da_mua_hang','chot'))::int AS won_leads,
			COALESCE(SUM(CASE WHEN trang_thai IN ('da_mua_hang','chot') THEN 6000000 ELSE 0 END), 0)::float8 AS revenue
		FROM leads
		WHERE %s
		GROUP BY 1,2,3,4,5,6,7
		ORDER BY leads DESC
		LIMIT $%d
	`, joinConditions(conditions), len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetaCampaignBreakdown, 0)
	for rows.Next() {
		var b MetaCampaignBreakdown
		if err := rows.Scan(
			&b.CampaignID, &b.CampaignName, &b.AdSetID, &b.AdSetName,
			&b.AdID, &b.AdName, &b.Placement,
			&b.Leads, &b.NewLeads, &b.WonLeads, &b.Revenue,
		); err != nil {
			continue
		}
		if b.Leads > 0 {
			b.WonRate = float64(b.WonLeads) / float64(b.Leads) * 100
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// joinConditions nối các điều kiện WHERE bằng "AND".
func joinConditions(conditions []string) string {
	out := ""
	for i, c := range conditions {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out
}
