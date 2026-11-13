package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"apex-backend/models"
)

// ══════════════════════════════════════════════════════════════════════
// BỘ LỌC TOÀN CỤC (GLOBAL FILTERS) — thiết kế lại hệ thống báo cáo 2026-11-13
// ══════════════════════════════════════════════════════════════════════
//
// Mọi báo cáo trong tab "Báo cáo" đều dùng CHUNG một bộ lọc ⇒ số liệu giữa
// các thẻ KPI, biểu đồ và bảng luôn khớp nhau.
//
// Lưu ý bất biến (AGENTS.md #3): mọi so sánh ngày/giờ dùng
// `AT TIME ZONE 'Asia/Ho_Chi_Minh'`.
type ReportFilter struct {
	// ─── Khoảng thời gian ───
	FromDate string // YYYY-MM-DD
	ToDate   string // YYYY-MM-DD
	Days     int    // dùng khi FromDate/ToDate rỗng
	FromHour int    // 0-23, -1 = không lọc
	ToHour   int    // 0-23, -1 = không lọc

	// ─── Landing & nguồn ───
	LandingIDs  []string // rỗng = tất cả landing
	Nguon       string
	LandingPath string

	// ─── Meta Ads attribution ───
	AdAccountID    string
	CampaignName   string
	AdsetName      string
	AdName         string
	SiteSourceName string
	UTMPlacement   string

	// ─── Đối tượng ───
	VisitorType string // all | new | returning

	// ─── Thiết bị ───
	DeviceType     string
	OS             string
	IsInAppBrowser int // -1 = không lọc, 0 = false, 1 = true
	InAppType      string

	// ─── Chất lượng traffic ───
	TrafficScope string // human (mặc định) | all | bot
}

// DefaultReportFilter trả bộ lọc mặc định: 30 ngày gần nhất, chỉ tính người thật.
func DefaultReportFilter() ReportFilter {
	return ReportFilter{
		FromDate:       "",
		ToDate:         "",
		Days:           30,
		FromHour:       -1,
		ToHour:         -1,
		LandingIDs:     nil,
		Nguon:          "",
		LandingPath:    "",
		AdAccountID:    "",
		CampaignName:   "",
		AdsetName:      "",
		AdName:         "",
		SiteSourceName: "",
		UTMPlacement:   "",
		VisitorType:    "all",
		DeviceType:     "",
		OS:             "",
		IsInAppBrowser: -1,
		InAppType:      "",
		TrafficScope:   "human",
	}
}

// where sinh mệnh đề WHERE + danh sách tham số (parameterized).
//
// ⚠️ Nguyên tắc: mỗi điều kiện CÓ placeholder phải tương ứng ĐÚNG 1 tham số.
// Điều kiện hằng số (vd `is_bot = FALSE`) KHÔNG được thêm vào danh sách args.
func (f ReportFilter) where() (string, []interface{}) {
	return f.buildWhere(false)
}

// whereWithoutLanding giống where() nhưng BỎ qua bộ lọc landing_id.
//
// Dùng cho bảng leaderboard (cần nhóm theo landing_id nên không thể lọc cứng).
func (f ReportFilter) whereWithoutLanding() (string, []interface{}) {
	return f.buildWhere(true)
}

func (f ReportFilter) buildWhere(skipLanding bool) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	// add thêm điều kiện CÓ 1 placeholder ($idx) + 1 tham số.
	add := func(expr string, val interface{}) {
		conditions = append(conditions, fmt.Sprintf(expr, idx))
		args = append(args, val)
		idx++
	}
	// addConst thêm điều kiện hằng số (KHÔNG placeholder, KHÔNG tham số).
	addConst := func(expr string) {
		conditions = append(conditions, expr)
	}

	// ─── Landing (multi-select) ───
	if !skipLanding {
		ids := make([]string, 0, len(f.LandingIDs))
		for _, id := range f.LandingIDs {
			if s := strings.TrimSpace(id); s != "" {
				ids = append(ids, s)
			}
		}
		if len(ids) == 1 {
			add("landing_id = $%d", ids[0])
		} else if len(ids) > 1 {
			placeholders := make([]string, 0, len(ids))
			for _, id := range ids {
				placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
				args = append(args, id)
				idx++
			}
			conditions = append(conditions, "landing_id IN ("+strings.Join(placeholders, ", ")+")")
		}
	}

	// ─── Nguồn / đường dẫn ───
	if s := strings.TrimSpace(f.Nguon); s != "" {
		add("nguon = $%d", s)
	}
	if s := strings.TrimSpace(f.LandingPath); s != "" {
		add("landing_path = $%d", s)
	}

	// ─── Khoảng ngày ───
	if s := strings.TrimSpace(f.FromDate); s != "" {
		add("(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ($%d)::date", s)
	}
	if s := strings.TrimSpace(f.ToDate); s != "" {
		add("(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date <= ($%d)::date", s)
	}
	if f.Days > 0 && strings.TrimSpace(f.FromDate) == "" && strings.TrimSpace(f.ToDate) == "" {
		add("(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - ($%d::int))", f.Days)
	}

	// ─── Khung giờ (hỗ trợ quay vòng qua nửa đêm, vd 22h→06h) ───
	if f.FromHour >= 0 && f.ToHour >= 0 && f.ToHour <= 23 {
		if f.FromHour <= f.ToHour {
			conditions = append(conditions, fmt.Sprintf(
				"EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int BETWEEN $%d AND $%d", idx, idx+1))
		} else {
			conditions = append(conditions, fmt.Sprintf(
				"(EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int >= $%d OR EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int <= $%d)", idx, idx+1))
		}
		args = append(args, f.FromHour, f.ToHour)
		idx += 2
	}

	// ─── Meta Ads attribution ───
	if s := strings.TrimSpace(f.AdAccountID); s != "" {
		add("ad_account_id = $%d", s)
	}
	if s := strings.TrimSpace(f.CampaignName); s != "" {
		add("campaign_name = $%d", s)
	}
	if s := strings.TrimSpace(f.AdsetName); s != "" {
		add("adset_name = $%d", s)
	}
	if s := strings.TrimSpace(f.AdName); s != "" {
		add("ad_name = $%d", s)
	}
	if s := strings.TrimSpace(f.SiteSourceName); s != "" {
		add("site_source_name = $%d", s)
	}
	if s := strings.TrimSpace(f.UTMPlacement); s != "" {
		add("utm_placement = $%d", s)
	}

	// ─── Loại khách (mới / quay lại) ───
	//
	// "Khách quay lại" = đã có TỪ 2 phiên trở lên (session_count > 1) — đúng
	// định nghĩa trong đặc tả báo cáo. `is_new_visitor` chỉ nói "phiên đầu tiên"
	// của visitor nên không đủ để nhận diện khách quay lại.
	switch strings.ToLower(strings.TrimSpace(f.VisitorType)) {
	case "new":
		addConst("is_new_visitor = TRUE")
	case "returning":
		addConst("session_count > 1")
	}

	// ─── Thiết bị ───
	if s := strings.TrimSpace(f.DeviceType); s != "" {
		add("device_type = $%d", s)
	}
	if s := strings.TrimSpace(f.OS); s != "" {
		add("os = $%d", s)
	}
	if f.IsInAppBrowser == 1 {
		addConst("is_in_app_browser = TRUE")
	} else if f.IsInAppBrowser == 0 {
		addConst("is_in_app_browser = FALSE")
	}
	if s := strings.TrimSpace(f.InAppType); s != "" {
		add("in_app_type = $%d", s)
	}

	// ─── Chất lượng traffic ───
	switch strings.ToLower(strings.TrimSpace(f.TrafficScope)) {
	case "bot":
		addConst("is_bot = TRUE")
	case "all":
		// không lọc
	default: // human
		addConst("is_bot = FALSE")
	}

	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// mustLoadVNLocation trả về Location Asia/Ho_Chi_Minh.
//
// Dùng để tính "kỳ trước" theo GIỜ VIỆT NAM (không phụ thuộc múi giờ máy chủ).
func mustLoadVNLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		// Windows chưa có tzdata ⇒ fallback về UTC+7 cố định (VN không có DST).
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}

// previousPeriod trả về bộ lọc tương ứng với KỲ TRƯỚC (cùng độ dài).
// Dùng cho chế độ "So với kỳ trước" hiển thị % tăng/giảm trên thẻ KPI.
func (f ReportFilter) previousPeriod() ReportFilter {
	prev := f

	// Khoảng ngày tường minh → lùi về đúng số ngày.
	if f.FromDate != "" && f.ToDate != "" {
		from, err1 := time.Parse("2006-01-02", f.FromDate)
		to, err2 := time.Parse("2006-01-02", f.ToDate)
		if err1 == nil && err2 == nil && !to.Before(from) {
			span := int(to.Sub(from).Hours()/24) + 1
			prevTo := from.AddDate(0, 0, -1)
			prevFrom := prevTo.AddDate(0, 0, -(span - 1))
			prev.FromDate = prevFrom.Format("2006-01-02")
			prev.ToDate = prevTo.Format("2006-01-02")
			prev.Days = 0
			return prev
		}
	}

	// Khoảng "N ngày gần nhất" → N ngày liền trước đó.
	//
	// ⚠️ PHẢI chuyển thành FromDate/ToDate tường minh. Nếu chỉ giữ `Days = N`
	// thì điều kiện SQL vẫn là "từ hôm nay trừ N ngày" ⇒ kỳ trước TRÙNG kỳ
	// hiện tại ⇒ % thay đổi luôn = 0%.
	if f.Days > 0 && f.FromDate == "" && f.ToDate == "" {
		now := time.Now().In(mustLoadVNLocation())
		today := now.Format("2006-01-02")
		prevTo, err := time.Parse("2006-01-02", today)
		if err == nil {
			prevFrom := prevTo.AddDate(0, 0, -f.Days)
			prev.FromDate = prevFrom.Format("2006-01-02")
			prev.ToDate = prevTo.Format("2006-01-02")
			prev.Days = 0
		}
		return prev
	}

	// Không xác định ⇒ không so sánh.
	prev.Days = 0
	prev.FromDate = ""
	prev.ToDate = ""
	return prev
}

// ══════════════════════════════════════════════════════════════════════
// 1. THẺ KPI TỔNG HỢP (Executive Scorecards)
// ══════════════════════════════════════════════════════════════════════

// ReportKPI là 1 thẻ KPI kèm số liệu kỳ trước để tính % thay đổi.
type ReportKPI struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Previous  float64 `json:"previous"`
	ChangePct float64 `json:"change_percent"`
	Direction string  `json:"direction"` // up | down | flat
	Hint      string  `json:"hint,omitempty"`
}

// GetReportSummary trả về toàn bộ thẻ KPI của tab Tổng quan.
//
// `compare` = true ⇒ kèm số liệu kỳ trước và % thay đổi.
func (s *Store) GetReportSummary(f ReportFilter, compare bool) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	current, err := s.reportKPIValues(ctx, f)
	if err != nil {
		return nil, err
	}

	previous := map[string]float64{}
	if compare {
		previous, err = s.reportKPIValues(ctx, f.previousPeriod())
		if err != nil {
			return nil, err
		}
	}

	kpis := buildReportKPIs(current, previous, compare)

	return map[string]interface{}{
		"kpis":      kpis,
		"compare":   compare,
		"from_date": f.FromDate,
		"to_date":   f.ToDate,
		"days":      f.Days,
	}, nil
}

// reportKPIValues tính toàn bộ chỉ số gốc của 1 khoảng thời gian.
func (s *Store) reportKPIValues(ctx context.Context, f ReportFilter) (map[string]float64, error) {
	whereClause, args := f.where()

	// Lưu ý: bộ lọc "human" đã loại bot ⇒ các chỉ số tính từ đây là của
	// NGƯỜI THẬT. Đối với traffic scope = bot thì chỉ số "người thật" = 0.
	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COUNT(DISTINCT visitor_id) FILTER (WHERE is_new_visitor)::int   AS new_visitors,
			COUNT(DISTINCT visitor_id) FILTER (WHERE NOT is_new_visitor)::int AS returning_visitors,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			COALESCE(AVG(hidden_seconds), 0)::float                    AS avg_hidden,
			COALESCE(AVG(max_scroll_percent), 0)::float                AS avg_scroll,
			COALESCE(AVG(session_count), 0)::float                     AS avg_session_count,
			COUNT(*) FILTER (WHERE has_zalo_click OR has_tel_click)::int  AS contact_clicks,
			COUNT(*) FILTER (WHERE has_interacted_form)::int           AS form_interactions,
			COALESCE(SUM(form_submit_attempts), 0)::int                AS submit_attempts,
			COALESCE(SUM(form_validation_errors), 0)::int              AS validation_errors,
			COUNT(*) FILTER (WHERE validation_passed)::int            AS validation_passed,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS converted,
			COUNT(*) FILTER (WHERE is_bot)::int                        AS bot_sessions,
			COUNT(*) FILTER (WHERE has_video_play)::int                AS video_viewers,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0), 0)::float AS avg_fill_ms,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0 AND NOT is_autofill), 0)::float AS avg_fill_ms_manual,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0 AND is_autofill), 0)::float     AS avg_fill_ms_auto
		FROM analytics_sessions
		%s;
	`, whereClause)

	var (
		sessions, uniqueVisitors, leads, newVisitors, returningVisitors   int
		avgVisible, avgHidden, avgScroll, avgSessionCount                 float64
		contactClicks, formInteractions, submitAttempts, validationErrors int
		validationPassed, converted, botSessions, videoViewers            int
		avgFillMs, avgFillManual, avgFillAuto                             float64
	)

	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&sessions, &uniqueVisitors, &leads, &newVisitors, &returningVisitors,
		&avgVisible, &avgHidden, &avgScroll, &avgSessionCount,
		&contactClicks, &formInteractions, &submitAttempts, &validationErrors,
		&validationPassed, &converted, &botSessions, &videoViewers,
		&avgFillMs, &avgFillManual, &avgFillAuto,
	)
	if err != nil {
		return nil, fmt.Errorf("lỗi tổng quan báo cáo: %w", err)
	}

	cr := 0.0
	if uniqueVisitors > 0 {
		cr = float64(leads) / float64(uniqueVisitors) * 100
	}
	contactRate := 0.0
	if sessions > 0 {
		contactRate = float64(contactClicks) / float64(sessions) * 100
	}
	abandonRate := 0.0
	if formInteractions > 0 {
		abandonRate = float64(formInteractions-converted) / float64(formInteractions) * 100
	}
	valErrRate := 0.0
	if submitAttempts > 0 {
		valErrRate = float64(validationErrors) / float64(submitAttempts) * 100
	}

	return map[string]float64{
		"sessions":              float64(sessions),
		"unique_visitors":       float64(uniqueVisitors),
		"leads":                 float64(leads),
		"new_visitors":          float64(newVisitors),
		"returning_visitors":    float64(returningVisitors),
		"cr":                    cr,
		"avg_visible":           avgVisible,
		"avg_hidden":            avgHidden,
		"avg_scroll":            avgScroll,
		"avg_session_count":     avgSessionCount,
		"contact_clicks":        float64(contactClicks),
		"contact_rate":          contactRate,
		"form_interactions":     float64(formInteractions),
		"submit_attempts":       float64(submitAttempts),
		"validation_errors":     float64(validationErrors),
		"validation_passed":     float64(validationPassed),
		"converted":             float64(converted),
		"bot_sessions":          float64(botSessions),
		"video_viewers":         float64(videoViewers),
		"avg_fill_ms":           avgFillMs,
		"avg_fill_ms_manual":    avgFillManual,
		"avg_fill_ms_auto":      avgFillAuto,
		"form_abandon_rate":     abandonRate,
		"validation_error_rate": valErrRate,
	}, nil
}

// buildReportKPIs dựng danh sách thẻ KPI kèm % thay đổi so với kỳ trước.
func buildReportKPIs(cur map[string]float64, prev map[string]float64, compare bool) []ReportKPI {
	defs := []struct {
		key   string
		label string
		unit  string
		hint  string
	}{
		{"sessions", "Tổng lượt truy cập", "phiên", "Số session hợp lệ"},
		{"unique_visitors", "Khách truy cập duy nhất", "khách", "Đếm theo visitor_id"},
		{"leads", "Tổng Lead hợp lệ", "lead", "Phiên đã converted (đã trừ trùng SĐT)"},
		{"cr", "Tỷ lệ chuyển đổi CR", "%", "Lead / Unique Visitors"},
		{"avg_visible", "Thời gian xem thực TB", "giây", "Loại trừ hidden_seconds"},
		{"contact_rate", "Tỷ lệ bấm liên hệ ngoài form", "%", "has_zalo_click hoặc has_tel_click"},
		{"form_interactions", "Người đã chạm form", "phiên", "has_interacted_form"},
		{"avg_scroll", "Độ sâu cuộn TB", "%", "max_scroll_percent trung bình"},
	}

	out := make([]ReportKPI, 0, len(defs))
	for _, d := range defs {
		v := cur[d.key]
		p := prev[d.key]
		k := ReportKPI{
			Key:   d.key,
			Label: d.label,
			Unit:  d.unit,
			Value: v,
			Hint:  d.hint,
		}
		if compare {
			k.Previous = p
			k.ChangePct, k.Direction = calcChangePct(v, p)
		} else {
			k.Direction = "flat"
		}
		out = append(out, k)
	}
	return out
}

// calcChangePct tính % thay đổi. previous = 0 ⇒ coi như tăng vô hạn (100%).
func calcChangePct(cur, prev float64) (float64, string) {
	if prev == 0 {
		if cur == 0 {
			return 0, "flat"
		}
		return 100, "up"
	}
	pct := (cur - prev) / prev * 100
	dir := "flat"
	if pct > 0.5 {
		dir = "up"
	} else if pct < -0.5 {
		dir = "down"
	}
	return pct, dir
}

// ══════════════════════════════════════════════════════════════════════
// 2. BIỂU ĐỒ 1 — Xu hướng Traffic & Lead theo thời gian (Combo Bar + Line)
// ══════════════════════════════════════════════════════════════════════

// GetReportTimeSeries trả về chuỗi thời gian theo granularity ("hour"|"day"|"week").
//
// Trục X: giờ (khi xem 1 ngày) hoặc ngày (khi xem nhiều ngày).
func (s *Store) GetReportTimeSeries(f ReportFilter, granularity string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	var expr string
	switch strings.ToLower(strings.TrimSpace(granularity)) {
	case "hour":
		expr = `TO_CHAR(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:00')`
	case "week":
		expr = `TO_CHAR(DATE_TRUNC('week', first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh'), 'YYYY-MM-DD')`
	default:
		expr = `TO_CHAR((first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date, 'YYYY-MM-DD')`
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS bucket,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			COALESCE(AVG(max_scroll_percent), 0)::float                AS avg_scroll
		FROM analytics_sessions
		%s
		GROUP BY bucket
		ORDER BY bucket ASC;
	`, expr, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi timeseries báo cáo: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var bucket string
		var sessions, uniqueVisitors, leads int
		var cr, avgVisible, avgScroll float64
		if err := rows.Scan(&bucket, &sessions, &uniqueVisitors, &leads, &cr, &avgVisible, &avgScroll); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"label":           bucket,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"leads":           leads,
			"cr":              cr,
			"avg_visible":     avgVisible,
			"avg_scroll":      avgScroll,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BẢNG 1 — Ma trận so sánh hiệu suất từng Landing Page (Leaderboard)
// ══════════════════════════════════════════════════════════════════════

// GetReportLandingLeaderboard trả về bảng so sánh nhiều landing trong 1 lần gọi.
func (s *Store) GetReportLandingLeaderboard(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	// Bảng SO SÁNH nhiều landing ⇒ bỏ bộ lọc landing_id (luôn nhóm theo landing_id).
	baseWhere, args := f.whereWithoutLanding()

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(landing_id, ''), 'default_landing')          AS landing_id,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(DISTINCT visitor_id) FILTER (WHERE NOT is_new_visitor)::int AS returning_visitors,
			COUNT(DISTINCT visitor_id) FILTER (WHERE is_new_visitor)::int     AS new_visitors,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			COALESCE(AVG(max_scroll_percent), 0)::float                AS avg_scroll,
			COUNT(*) FILTER (WHERE has_zalo_click OR has_tel_click)::int AS contact_clicks,
			COUNT(*) FILTER (WHERE has_interacted_form)::int           AS form_interactions,
			COALESCE(SUM(form_submit_attempts), 0)::int                AS submit_attempts,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COUNT(*) FILTER (WHERE is_bot)::int                        AS bot_sessions
		FROM analytics_sessions
		%s
		GROUP BY landing_id
		ORDER BY sessions DESC;
	`, baseWhere)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi leaderboard landing: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var lid string
		var sessions, uniqueVisitors, returning, newVis int
		var avgVisible, avgScroll float64
		var contactClicks, formInteractions, submitAttempts, leads, botSessions int
		if err := rows.Scan(&lid, &sessions, &uniqueVisitors, &returning, &newVis,
			&avgVisible, &avgScroll, &contactClicks, &formInteractions,
			&submitAttempts, &leads, &botSessions); err != nil {
			return nil, err
		}

		cr := 0.0
		if uniqueVisitors > 0 {
			cr = float64(leads) / float64(uniqueVisitors) * 100
		}
		returningPct := 0.0
		if uniqueVisitors > 0 {
			returningPct = float64(returning) / float64(uniqueVisitors) * 100
		}
		contactRate := 0.0
		if sessions > 0 {
			contactRate = float64(contactClicks) / float64(sessions) * 100
		}
		botRate := 0.0
		if sessions > 0 {
			botRate = float64(botSessions) / float64(sessions) * 100
		}

		items = append(items, map[string]interface{}{
			"landing_id":         lid,
			"sessions":           sessions,
			"unique_visitors":    uniqueVisitors,
			"new_visitors":       newVis,
			"returning_visitors": returning,
			"returning_percent":  returningPct,
			"avg_visible":        avgVisible,
			"avg_scroll":         avgScroll,
			"contact_clicks":     contactClicks,
			"contact_rate":       contactRate,
			"form_interactions":  formInteractions,
			"submit_attempts":    submitAttempts,
			"leads":              leads,
			"cr":                 cr,
			"bot_sessions":       botSessions,
			"bot_rate":           botRate,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 2 — Phân bổ traffic theo nền tảng Meta + vị trí hiển thị
// ══════════════════════════════════════════════════════════════════════

// GetReportMetaPlatform trả về phân bổ theo site_source_name (fb/ig/msg/an).
func (s *Store) GetReportMetaPlatform(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(site_source_name, ''), 'unknown')            AS site_source_name,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COUNT(*) FILTER (WHERE is_bot)::int                        AS bot_sessions,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr
		FROM analytics_sessions
		%s
		GROUP BY site_source_name
		ORDER BY sessions DESC;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi phân bổ nền tảng Meta: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var name string
		var sessions, uniqueVisitors, leads, bots int
		var cr float64
		if err := rows.Scan(&name, &sessions, &uniqueVisitors, &leads, &bots, &cr); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"site_source_name": name,
			"sessions":         sessions,
			"unique_visitors":  uniqueVisitors,
			"leads":            leads,
			"bot_sessions":     bots,
			"cr":               cr,
		})
	}
	return items, rows.Err()
}

// GetReportMetaPlacement trả về phân bổ theo utm_placement (Feed, Reels, Stories...).
func (s *Store) GetReportMetaPlacement(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(utm_placement, ''), 'unknown')               AS placement,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COUNT(*) FILTER (WHERE is_bot)::int                        AS bot_sessions,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr
		FROM analytics_sessions
		%s
		GROUP BY placement
		ORDER BY sessions DESC
		LIMIT 30;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi phân bổ vị trí quảng cáo: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var name string
		var sessions, uniqueVisitors, leads, bots int
		var cr float64
		if err := rows.Scan(&name, &sessions, &uniqueVisitors, &leads, &bots, &cr); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"placement":       name,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"leads":           leads,
			"bot_sessions":    bots,
			"cr":              cr,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 3 — Hiệu quả trình duyệt In-App vs trình duyệt ngoài
// ══════════════════════════════════════════════════════════════════════

// GetReportInAppBrowser so sánh CR giữa in-app browser và trình duyệt ngoài.
func (s *Store) GetReportInAppBrowser(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			CASE
				WHEN is_in_app_browser THEN COALESCE(NULLIF(in_app_type, ''), 'in_app_other')
				ELSE 'external_browser' END                           AS group_name,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COUNT(*) FILTER (WHERE has_interacted_form)::int           AS form_interactions,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			COALESCE(AVG(max_scroll_percent), 0)::float                AS avg_scroll,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr
		FROM analytics_sessions
		%s
		GROUP BY group_name
		ORDER BY sessions DESC;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê in-app browser: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var name string
		var sessions, uniqueVisitors, leads, formInteractions int
		var avgVisible, avgScroll, cr float64
		if err := rows.Scan(&name, &sessions, &uniqueVisitors, &leads,
			&formInteractions, &avgVisible, &avgScroll, &cr); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"group_name":        name,
			"sessions":          sessions,
			"unique_visitors":   uniqueVisitors,
			"leads":             leads,
			"form_interactions": formInteractions,
			"avg_visible":       avgVisible,
			"avg_scroll":        avgScroll,
			"cr":                cr,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BẢNG 2 — Phân cấp hiệu suất quảng cáo (Campaign → Adset → Ad)
// ══════════════════════════════════════════════════════════════════════

// GetReportAdHierarchy trả về cấp campaign → adset → ad với đầy đủ chỉ số.
func (s *Store) GetReportAdHierarchy(f ReportFilter, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	whereClause, args := f.where()

	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(campaign_name, ''), NULLIF(utm_campaign, ''), 'none')  AS campaign_name,
			COALESCE(NULLIF(adset_name, ''), NULLIF(utm_content, ''), 'none')     AS adset_name,
			COALESCE(NULLIF(ad_name, ''), NULLIF(utm_term, ''), 'none')          AS ad_name,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE visible_seconds < 5)::int           AS fast_bounce,
			COUNT(*) FILTER (WHERE scroll_depth_50)::int               AS scroll_50,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COUNT(*) FILTER (WHERE fbclid != '' OR fbc != '' OR fbp != '')::int AS capi_matched,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr,
			CASE WHEN COUNT(*) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE visible_seconds < 5)::numeric / COUNT(*)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS fast_bounce_rate,
			CASE WHEN COUNT(*) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE scroll_depth_50)::numeric / COUNT(*)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS scroll_50_rate,
			CASE WHEN COUNT(*) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE fbclid != '' OR fbc != '' OR fbp != '')::numeric / COUNT(*)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS capi_match_rate
		FROM analytics_sessions
		%s
		GROUP BY campaign_name, adset_name, ad_name
		ORDER BY sessions DESC
		LIMIT %d;
	`, whereClause, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi phân cấp quảng cáo: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var campaign, adset, ad string
		var sessions, uniqueVisitors, fastBounce, scroll50, leads, capiMatched int
		var cr, fastBounceRate, scroll50Rate, capiRate float64
		if err := rows.Scan(&campaign, &adset, &ad, &sessions, &uniqueVisitors,
			&fastBounce, &scroll50, &leads, &capiMatched,
			&cr, &fastBounceRate, &scroll50Rate, &capiRate); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"campaign_name":    campaign,
			"adset_name":       adset,
			"ad_name":          ad,
			"sessions":         sessions,
			"unique_visitors":  uniqueVisitors,
			"fast_bounce":      fastBounce,
			"fast_bounce_rate": fastBounceRate,
			"scroll_50":        scroll50,
			"scroll_50_rate":   scroll50Rate,
			"leads":            leads,
			"cr":               cr,
			"capi_matched":     capiMatched,
			"capi_match_rate":  capiRate,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 4 — Heatmap khung giờ vàng (7 ngày × 24 giờ)
// ══════════════════════════════════════════════════════════════════════

// GetReportHourHeatmap trả về ma trận 7 ngày (Thứ 2..Chủ nhật) × 24 giờ.
//
// metric: "leads" | "sessions" | "cr"
func (s *Store) GetReportHourHeatmap(f ReportFilter, metric string) ([][]int, [][]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	crExpr := `CASE WHEN COUNT(DISTINCT visitor_id) > 0
		THEN (COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100
		ELSE 0 END`

	query := fmt.Sprintf(`
		SELECT
			EXTRACT(ISODOW FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int AS iso_dow,
			EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int  AS hour_of_day,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			%s                                                         AS cr
		FROM analytics_sessions
		%s
		GROUP BY iso_dow, hour_of_day;
	`, crExpr, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("lỗi heatmap khung giờ: %w", err)
	}
	defer rows.Close()

	// Ma trận [7][24] — chỉ số 0 = Thứ 2 ... 6 = Chủ nhật.
	counts := make([][]int, 7)
	crs := make([][]float64, 7)
	for i := range counts {
		counts[i] = make([]int, 24)
		crs[i] = make([]float64, 24)
	}

	for rows.Next() {
		var dow, hour int
		var sessions, uniqueVisitors, leads int
		var cr float64
		if err := rows.Scan(&dow, &hour, &sessions, &uniqueVisitors, &leads, &cr); err != nil {
			return nil, nil, err
		}
		if dow < 1 || dow > 7 || hour < 0 || hour > 23 {
			continue
		}
		row := dow - 1
		switch strings.ToLower(strings.TrimSpace(metric)) {
		case "sessions":
			counts[row][hour] = sessions
		case "cr":
			crs[row][hour] = cr
		default:
			counts[row][hour] = leads
		}
	}
	return counts, crs, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 5 — Phễu cuộn trang (Scroll Depth Funnel)
// ══════════════════════════════════════════════════════════════════════

// GetReportScrollFunnel trả về tỷ lệ % đạt từng mốc cuộn.
func (s *Store) GetReportScrollFunnel(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int                                              AS pageview,
			COUNT(*) FILTER (WHERE scroll_depth_25)::int              AS s25,
			COUNT(*) FILTER (WHERE scroll_depth_50)::int              AS s50,
			COUNT(*) FILTER (WHERE scroll_depth_75)::int              AS s75,
			COUNT(*) FILTER (WHERE scroll_depth_90)::int              AS s90,
			COUNT(*) FILTER (WHERE scroll_depth_100)::int             AS s100
		FROM analytics_sessions
		%s;
	`, whereClause)

	var pageview, s25, s50, s75, s90, s100 int
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&pageview, &s25, &s50, &s75, &s90, &s100); err != nil {
		return nil, fmt.Errorf("lỗi phễu cuộn: %w", err)
	}

	milestones := []struct {
		Key   string
		Label string
		Count int
	}{
		{"pageview", "Pageview", pageview},
		{"scroll_25", "Cuộn 25%", s25},
		{"scroll_50", "Cuộn 50%", s50},
		{"scroll_75", "Cuộn 75%", s75},
		{"scroll_90", "Cuộn 90%", s90},
		{"scroll_100", "Cuộn 100%", s100},
	}

	items := make([]map[string]interface{}, 0, len(milestones))
	for _, m := range milestones {
		pct := 0.0
		if pageview > 0 {
			pct = float64(m.Count) / float64(pageview) * 100
		}
		items = append(items, map[string]interface{}{
			"key":     m.Key,
			"label":   m.Label,
			"count":   m.Count,
			"percent": pct,
		})
	}
	return items, nil
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 6 — Phân phối thời gian xem thực (visible_seconds)
// ══════════════════════════════════════════════════════════════════════

// GetReportVisibleTimeDistribution trả về phân phối visible_seconds theo bucket.
func (s *Store) GetReportVisibleTimeDistribution(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			CASE
				WHEN visible_seconds < 5 THEN '0-5s'
				WHEN visible_seconds < 15 THEN '5-15s'
				WHEN visible_seconds < 30 THEN '15-30s'
				WHEN visible_seconds < 60 THEN '30-60s'
				WHEN visible_seconds < 180 THEN '1-3 phút'
				ELSE '>3 phút' END                                      AS bucket,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			COALESCE(AVG(hidden_seconds), 0)::float                    AS avg_hidden
		FROM analytics_sessions
		%s
		GROUP BY bucket
		ORDER BY MIN(visible_seconds);
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi phân phối thời gian xem: %w", err)
	}
	defer rows.Close()

	order := []string{"0-5s", "5-15s", "15-30s", "30-60s", "1-3 phút", ">3 phút"}
	byBucket := make(map[string]map[string]interface{})
	for rows.Next() {
		var bucket string
		var sessions, uniqueVisitors, leads int
		var avgVisible, avgHidden float64
		if err := rows.Scan(&bucket, &sessions, &uniqueVisitors, &leads, &avgVisible, &avgHidden); err != nil {
			return nil, err
		}
		byBucket[bucket] = map[string]interface{}{
			"bucket":          bucket,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"leads":           leads,
			"avg_visible":     avgVisible,
			"avg_hidden":      avgHidden,
		}
	}
	rows.Close()

	items := make([]map[string]interface{}, 0, len(order))
	for _, b := range order {
		if v, ok := byBucket[b]; ok {
			items = append(items, v)
			continue
		}
		items = append(items, map[string]interface{}{
			"bucket":          b,
			"sessions":        0,
			"unique_visitors": 0,
			"leads":           0,
			"avg_visible":     0,
			"avg_hidden":      0,
		})
	}
	return items, nil
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 7 — Tương tác đa phương tiện & kênh phụ
// ══════════════════════════════════════════════════════════════════════

// GetReportEngagement so sánh CR giữa nhóm có/không hành vi.
func (s *Store) GetReportEngagement(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	// Mỗi hành vi là 1 "nhóm": có hành vi vs không có hành vi.
	groups := []struct {
		Key   string
		Label string
		Cond  string
	}{
		{"video_play", "Có xem video", "has_video_play"},
		{"video_skip", "Không xem video", "NOT has_video_play"},
		{"zalo_click", "Có bấm Zalo", "has_zalo_click"},
		{"zalo_skip", "Không bấm Zalo", "NOT has_zalo_click"},
		{"tel_click", "Có bấm gọi", "has_tel_click"},
		{"tel_skip", "Không bấm gọi", "NOT has_tel_click"},
		{"form_focus", "Đã chạm form", "has_interacted_form"},
		{"form_skip", "Chưa chạm form", "NOT has_interacted_form"},
	}

	items := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		query := fmt.Sprintf(`
			SELECT
				COUNT(*)::int                                              AS sessions,
				COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
				COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
				CASE WHEN COUNT(DISTINCT visitor_id) > 0
					THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
					ELSE 0 END                                              AS cr
			FROM analytics_sessions
			%s AND (%s);
		`, whereClause, g.Cond)

		var sessions, uniqueVisitors, leads int
		var cr float64
		if err := s.pool.QueryRow(ctx, query, args...).Scan(&sessions, &uniqueVisitors, &leads, &cr); err != nil {
			return nil, fmt.Errorf("lỗi thống kê tương tác (%s): %w", g.Key, err)
		}
		items = append(items, map[string]interface{}{
			"key":             g.Key,
			"label":           g.Label,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"leads":           leads,
			"cr":              cr,
		})
	}
	return items, nil
}

// ══════════════════════════════════════════════════════════════════════
// BẢNG 3 — Top điểm chạm (Top Clicked Elements)
// ══════════════════════════════════════════════════════════════════════

// GetReportTopClickedElements tổng hợp event type='click' trong `actions` JSONB.
//
// Dùng `jsonb_array_elements` để tách từng event ⇒ nhóm theo target + text.
func (s *Store) GetReportTopClickedElements(f ReportFilter, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	whereClause, args := f.where()

	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}

	// Cột `actions` là JSONB array. Mỗi phần tử có: type, target, text, time.
	// Lưu ý: truy vấn này dùng `actions` nên phải đặt tên bảng rõ ràng.
	query := fmt.Sprintf(`
		WITH expanded AS (
			SELECT
				s.landing_id,
				s.session_id,
				s.visitor_id,
				ev->>'type'   AS event_type,
				ev->>'target' AS target,
				ev->>'text'   AS text,
				COALESCE((ev->>'time')::int, 0) AS offset_seconds
			FROM analytics_sessions s
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(s.actions, '[]'::jsonb)) AS ev
			%s
		)
		SELECT
			COALESCE(NULLIF(target, ''), '(không rõ)')      AS target,
			COALESCE(NULLIF(text, ''), '')                   AS text,
			COUNT(*)::int                                    AS clicks,
			COUNT(DISTINCT session_id)::int                 AS sessions,
			COUNT(DISTINCT visitor_id)::int                 AS visitors,
			COUNT(DISTINCT landing_id)::int                 AS landings
		FROM expanded
		WHERE event_type = 'click'
		GROUP BY target, text
		ORDER BY clicks DESC
		LIMIT %d;
	`, whereClause, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê điểm chạm: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var target, text string
		var clicks, sessions, visitors, landings int
		if err := rows.Scan(&target, &text, &clicks, &sessions, &visitors, &landings); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"target":   target,
			"text":     text,
			"clicks":   clicks,
			"sessions": sessions,
			"visitors": visitors,
			"landings": landings,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 8 — Phễu chuyển đổi form chi tiết
// ══════════════════════════════════════════════════════════════════════

// GetReportFormFunnel trả về các bước của phễu form + chẩn đoán.
func (s *Store) GetReportFormFunnel(f ReportFilter) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int                                              AS pageview,
			COUNT(*) FILTER (WHERE has_interacted_form)::int           AS interacted,
			COUNT(*) FILTER (WHERE form_submit_attempts > 0)::int      AS submit_attempt,
			COUNT(*) FILTER (WHERE validation_passed)::int            AS validation_passed,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS converted,
			COALESCE(SUM(form_submit_attempts), 0)::int                AS total_attempts,
			COALESCE(SUM(form_validation_errors), 0)::int              AS total_errors,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0), 0)::float AS avg_fill_ms,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0 AND NOT is_autofill), 0)::float AS avg_fill_manual,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0 AND is_autofill), 0)::float     AS avg_fill_auto,
			COUNT(*) FILTER (WHERE is_autofill)::int                   AS autofill_sessions
		FROM analytics_sessions
		%s;
	`, whereClause)

	var pageview, interacted, submitAttempt, validationPassed, converted int
	var totalAttempts, totalErrors, autofillSessions int
	var avgFillMs, avgFillManual, avgFillAuto float64

	if err := s.pool.QueryRow(ctx, query, args...).Scan(
		&pageview, &interacted, &submitAttempt, &validationPassed, &converted,
		&totalAttempts, &totalErrors, &avgFillMs, &avgFillManual, &avgFillAuto, &autofillSessions,
	); err != nil {
		return nil, fmt.Errorf("lỗi phễu form: %w", err)
	}

	steps := []struct {
		Key   string
		Label string
		Count int
	}{
		{"pageview", "Pageview", pageview},
		{"form_focus", "Form Focus / Autofill", interacted},
		{"submit_attempt", "Form Submit Attempt", submitAttempt},
		{"validation_passed", "Validation Passed", validationPassed},
		{"converted", "Converted Lead", converted},
	}

	funnel := make([]map[string]interface{}, 0, len(steps))
	for _, s := range steps {
		pct := 0.0
		if pageview > 0 {
			pct = float64(s.Count) / float64(pageview) * 100
		}
		stepRate := 0.0
		if s.Count > 0 {
			stepRate = float64(converted) / float64(s.Count) * 100
		}
		funnel = append(funnel, map[string]interface{}{
			"key":       s.Key,
			"label":     s.Label,
			"count":     s.Count,
			"percent":   pct,
			"step_rate": stepRate,
		})
	}

	abandonRate := 0.0
	if interacted > 0 {
		abandonRate = float64(interacted-converted) / float64(interacted) * 100
	}
	valErrRate := 0.0
	if totalAttempts > 0 {
		valErrRate = float64(totalErrors) / float64(totalAttempts) * 100
	}

	return map[string]interface{}{
		"funnel":                      funnel,
		"form_abandon_rate":           abandonRate,
		"validation_error_rate":       valErrRate,
		"avg_fill_duration_ms":        avgFillMs,
		"avg_fill_duration_ms_manual": avgFillManual,
		"avg_fill_duration_ms_auto":   avgFillAuto,
		"total_attempts":              totalAttempts,
		"total_errors":                totalErrors,
		"autofill_sessions":           autofillSessions,
	}, nil
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 9 — Tỷ trọng nguồn autofill
// ══════════════════════════════════════════════════════════════════════

// GetReportAutofillSource trả về phân bổ nguồn autofill.
func (s *Store) GetReportAutofillSource(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			CASE
				WHEN is_autofill AND autofill_source = 'url_params' THEN 'Tham số URL'
				WHEN is_autofill AND autofill_source = 'local_storage' THEN 'Lưu trình duyệt cũ'
				WHEN is_autofill AND autofill_source = 'browser_autofill' THEN 'Tự động điền 1 chạm'
				WHEN is_autofill AND autofill_source != '' THEN 'Autofill khác'
				WHEN is_autofill THEN 'Autofill (không rõ nguồn)'
				ELSE 'Nhập tay' END                            AS source_name,
			COUNT(*)::int                                      AS sessions,
			COUNT(DISTINCT visitor_id)::int                    AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int        AS leads,
			COALESCE(AVG(fill_duration_ms) FILTER (WHERE fill_duration_ms > 0), 0)::float AS avg_fill_ms
		FROM analytics_sessions
		%s
		GROUP BY source_name
		ORDER BY sessions DESC;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê nguồn autofill: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var name string
		var sessions, uniqueVisitors, leads int
		var avgFill float64
		if err := rows.Scan(&name, &sessions, &uniqueVisitors, &leads, &avgFill); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"source_name":     name,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"leads":           leads,
			"avg_fill_ms":     avgFill,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 10 + BẢNG 4 — Thiết bị, hệ điều hành, viewport
// ══════════════════════════════════════════════════════════════════════

// GetReportDeviceBreakdown trả về phân bổ thiết bị (device_type → os).
func (s *Store) GetReportDeviceBreakdown(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(device_type, ''), 'unknown')                 AS device_type,
			COALESCE(NULLIF(os, ''), 'unknown')                         AS os,
			COALESCE(NULLIF(os_version, ''), '')                        AS os_version,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr
		FROM analytics_sessions
		%s
		GROUP BY device_type, os, os_version
		ORDER BY sessions DESC
		LIMIT 60;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi phân bổ thiết bị: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var deviceType, os, osVersion string
		var sessions, uniqueVisitors, leads int
		var avgVisible, cr float64
		if err := rows.Scan(&deviceType, &os, &osVersion, &sessions, &uniqueVisitors,
			&leads, &avgVisible, &cr); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"device_type":     deviceType,
			"os":              os,
			"os_version":      osVersion,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"leads":           leads,
			"avg_visible":     avgVisible,
			"cr":              cr,
		})
	}
	return items, rows.Err()
}

// GetReportDeviceModels trả về bảng top mã thiết bị + kích thước màn hình.
func (s *Store) GetReportDeviceModels(f ReportFilter, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(device_model_code, ''), NULLIF(device_name, ''), 'Không rõ') AS model_code,
			COALESCE(NULLIF(device_name, ''), NULLIF(device_model, ''), 'Không rõ')      AS model_name,
			COALESCE(NULLIF(viewport_res, ''), 'Không rõ')                             AS viewport_res,
			COALESCE(NULLIF(screen_res, ''), '')                                        AS screen_res,
			COUNT(*)::int                                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                                            AS unique_visitors,
			COUNT(*) FILTER (WHERE total_clicks > 0)::int                              AS clicked_sessions,
			COUNT(*) FILTER (WHERE has_interacted_form)::int                           AS interacted_sessions,
			COUNT(*) FILTER (WHERE converted_lead)::int                                AS leads,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                                              AS cr
		FROM analytics_sessions
		%s
		GROUP BY model_code, model_name, viewport_res, screen_res
		ORDER BY sessions DESC
		LIMIT %d;
	`, whereClause, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi bảng mã thiết bị: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var modelCode, modelName, viewport, screen string
		var sessions, uniqueVisitors, clicked, interacted, leads int
		var cr float64
		if err := rows.Scan(&modelCode, &modelName, &viewport, &screen,
			&sessions, &uniqueVisitors, &clicked, &interacted, &leads, &cr); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"model_code":          modelCode,
			"model_name":          modelName,
			"viewport_res":        viewport,
			"screen_res":          screen,
			"sessions":            sessions,
			"unique_visitors":     uniqueVisitors,
			"clicked_sessions":    clicked,
			"interacted_sessions": interacted,
			"leads":               leads,
			"cr":                  cr,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 11 — Chất lượng mạng & cấu hình phần cứng
// ══════════════════════════════════════════════════════════════════════

// GetReportConnectionHardware nhóm theo connection_type, device_memory, device_cores.
func (s *Store) GetReportConnectionHardware(f ReportFilter) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(connection_type, ''), 'unknown')            AS connection_type,
			CASE
				WHEN device_memory <= 0 THEN 'Không rõ'
				WHEN device_memory <= 2 THEN '≤ 2GB'
				WHEN device_memory <= 4 THEN '3-4GB'
				WHEN device_memory <= 8 THEN '5-8GB'
				ELSE '> 8GB' END                                         AS memory_bucket,
			CASE
				WHEN device_cores <= 0 THEN 'Không rõ'
				WHEN device_cores <= 4 THEN '≤ 4 nhân'
				WHEN device_cores <= 8 THEN '5-8 nhân'
				ELSE '> 8 nhân' END                                      AS cores_bucket,
			COUNT(*)::int                                              AS sessions,
			COUNT(DISTINCT visitor_id)::int                            AS unique_visitors,
			COUNT(*) FILTER (WHERE visible_seconds < 5)::int           AS fast_bounce,
			COUNT(*) FILTER (WHERE converted_lead)::int                AS leads,
			COALESCE(AVG(visible_seconds), 0)::float                   AS avg_visible,
			CASE WHEN COUNT(DISTINCT visitor_id) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(DISTINCT visitor_id)::numeric) * 100, 2)::float
				ELSE 0 END                                              AS cr
		FROM analytics_sessions
		%s
		GROUP BY connection_type, memory_bucket, cores_bucket
		ORDER BY sessions DESC
		LIMIT 60;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê mạng & phần cứng: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var conn, mem, cores string
		var sessions, uniqueVisitors, fastBounce, leads int
		var avgVisible, cr float64
		if err := rows.Scan(&conn, &mem, &cores, &sessions, &uniqueVisitors,
			&fastBounce, &leads, &avgVisible, &cr); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"connection_type": conn,
			"memory_bucket":   mem,
			"cores_bucket":    cores,
			"sessions":        sessions,
			"unique_visitors": uniqueVisitors,
			"fast_bounce":     fastBounce,
			"leads":           leads,
			"avg_visible":     avgVisible,
			"cr":              cr,
		})
	}
	return items, rows.Err()
}

// ══════════════════════════════════════════════════════════════════════
// BIỂU ĐỒ 12 + BẢNG 5 — Bảo mật, chống bot & vùng cách ly
// ══════════════════════════════════════════════════════════════════════

// GetReportSecurity trả về KPI bảo mật + phân loại lý do chặn.
func (s *Store) GetReportSecurity(f ReportFilter) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	// Tổng quan traffic bot
	whereClause, args := f.where()

	totalQuery := fmt.Sprintf(`
		SELECT
			COUNT(*)::int                                          AS total_sessions,
			COUNT(*) FILTER (WHERE is_bot)::int                    AS bot_sessions,
			COUNT(*) FILTER (WHERE NOT is_bot)::int                AS human_sessions,
			COALESCE(AVG(bot_score) FILTER (WHERE is_bot), 0)::float AS avg_bot_score,
			MAX(bot_score)::int                                    AS max_bot_score
		FROM analytics_sessions
		%s;
	`, whereClause)

	var totalSessions, botSessions, humanSessions, maxScore int
	var avgScore float64
	if err := s.pool.QueryRow(ctx, totalQuery, args...).Scan(
		&totalSessions, &botSessions, &humanSessions, &avgScore, &maxScore); err != nil {
		return nil, fmt.Errorf("lỗi tổng quan bảo mật: %w", err)
	}

	botRate := 0.0
	if totalSessions > 0 {
		botRate = float64(botSessions) / float64(totalSessions) * 100
	}

	// 2. Phân loại lý do chặn bot (client heuristics + server)
	reasonQuery := fmt.Sprintf(`
		SELECT
			TRIM(BOTH ';' FROM SPLIT_PART(bot_reasons, ';', 1)) AS reason,
			COUNT(*)::int                                       AS count
		FROM analytics_sessions
		%s AND is_bot AND bot_reasons != ''
		GROUP BY reason
		ORDER BY count DESC
		LIMIT 40;
	`, whereClause)

	reasonRows, err := s.pool.Query(ctx, reasonQuery, args...)
	reasons := make([]map[string]interface{}, 0)
	if err == nil {
		defer reasonRows.Close()
		for reasonRows.Next() {
			var name string
			var cnt int
			if err := reasonRows.Scan(&name, &cnt); err == nil && name != "" {
				reasons = append(reasons, map[string]interface{}{"name": name, "count": cnt})
			}
		}
	}

	// 3. Nhóm lý do theo nguồn phát hiện
	grouped := map[string]int{
		"Client Heuristics": 0,
		"Honeypot":          0,
		"Proof-of-Work":     0,
		"Rate Limit / Spam": 0,
		"Server Signature":  0,
		"Khác":              0,
	}
	groupRows, err2 := s.pool.Query(ctx, reasonQuery, args...)
	if err2 == nil {
		defer groupRows.Close()
		for groupRows.Next() {
			var name string
			var cnt int
			if err := groupRows.Scan(&name, &cnt); err != nil || name == "" {
				continue
			}
			group := "Khác"
			ln := strings.ToLower(name)
			switch {
			case strings.Contains(ln, "honeypot") || strings.Contains(ln, "fax") ||
				strings.Contains(ln, "terms") || strings.Contains(ln, "website") ||
				strings.Contains(ln, "email_verify") || strings.Contains(ln, "company"):
				group = "Honeypot"
			case strings.Contains(ln, "pow") || strings.Contains(ln, "challenge") ||
				strings.Contains(ln, "proof"):
				group = "Proof-of-Work"
			case strings.Contains(ln, "rate") || strings.Contains(ln, "spam") ||
				strings.Contains(ln, "duplicate") || strings.Contains(ln, "replay"):
				group = "Rate Limit / Spam"
			case strings.Contains(ln, "webdriver") || strings.Contains(ln, "headless") ||
				strings.Contains(ln, "screen") || strings.Contains(ln, "language") ||
				strings.Contains(ln, "interaction") || strings.Contains(ln, "automation"):
				group = "Client Heuristics"
			case strings.Contains(ln, "crawler") || strings.Contains(ln, "bot") ||
				strings.Contains(ln, "spider") || strings.Contains(ln, "script") ||
				strings.Contains(ln, "cli") || strings.Contains(ln, "client"):
				group = "Server Signature"
			}
			grouped[group] += cnt
		}
	}

	reasonGroups := make([]map[string]interface{}, 0, len(grouped))
	for g, c := range grouped {
		reasonGroups = append(reasonGroups, map[string]interface{}{"name": g, "count": c})
	}

	// 4. Vùng cách ly (spam_quarantine)
	var quarantineCount int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*)::int FROM spam_quarantine").Scan(&quarantineCount); err != nil {
		quarantineCount = 0
	}

	return map[string]interface{}{
		"total_sessions":   totalSessions,
		"bot_sessions":     botSessions,
		"human_sessions":   humanSessions,
		"bot_rate":         botRate,
		"avg_bot_score":    avgScore,
		"max_bot_score":    maxScore,
		"quarantine_count": quarantineCount,
		"reasons":          reasons,
		"reason_groups":    reasonGroups,
	}, nil
}

// GetReportQuarantine trả về nhật ký vùng cách ly (BẢNG 5).
func (s *Store) GetReportQuarantine(landingID string, limit int, offset int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := ""
	var args []interface{}
	if s := strings.TrimSpace(landingID); s != "" {
		whereClause = "WHERE landing_id = $1"
		args = append(args, s)
	}

	countQuery := "SELECT COUNT(*)::int FROM spam_quarantine " + whereClause
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("lỗi đếm vùng cách ly: %w", err)
	}

	// Cột `payload` chứa toàn bộ form data (tên, SĐT, fill_duration_ms...).
	listQuery := fmt.Sprintf(`
		SELECT
			id,
			TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS') AS created_at,
			landing_id,
			nguon,
			phone,
			reason,
			ip_address,
			user_agent,
			payload::text
		FROM spam_quarantine
		%s
		ORDER BY created_at DESC
		LIMIT %d OFFSET %d;
	`, whereClause, limit, offset)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi lấy danh sách vùng cách ly: %w", err)
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int
		var createdAt, lid, nguon, phone, reason, ip, ua, payloadText string
		if err := rows.Scan(&id, &createdAt, &lid, &nguon, &phone, &reason, &ip, &ua, &payloadText); err != nil {
			return nil, err
		}

		// Trích thông tin bổ sung từ payload (fill_duration_ms, tên, visible_seconds).
		extra := map[string]interface{}{}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(payloadText), &payload); err == nil {
			extra = payload
		}
		name := ""
		if v, ok := extra["fullName"].(string); ok {
			name = v
		} else if v, ok := extra["name"].(string); ok {
			name = v
		} else if v, ok := extra["ho_ten"].(string); ok {
			name = v
		}

		items = append(items, map[string]interface{}{
			"id":               id,
			"created_at":       createdAt,
			"landing_id":       lid,
			"nguon":            nguon,
			"phone":            phone,
			"reason":           reason,
			"ip_address":       ip,
			"user_agent":       ua,
			"name":             name,
			"fill_duration_ms": extra["fill_duration_ms"],
			"visible_seconds":  extra["visible_seconds"],
			"max_scroll":       extra["max_scroll"],
			"payload":          extra,
		})
	}
	return map[string]interface{}{
		"data":   items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}, rows.Err()
}

// ApproveQuarantineLead cứu một lead bị chặn nhầm: đẩy thẳng vào CRM.
//
// ⚠️ Đây là hành động ghi dữ liệu nghiệp vụ — chỉ admin đã đăng nhập mới gọi được.
func (s *Store) ApproveQuarantineLead(id int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var payloadText, reason, landingID, nguon string
	err := s.pool.QueryRow(ctx, `
		SELECT payload::text, reason, landing_id, nguon
		FROM spam_quarantine
		WHERE id = $1
	`, id).Scan(&payloadText, &reason, &landingID, &nguon)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy request trong vùng cách ly: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
		return nil, fmt.Errorf("payload không hợp lệ: %w", err)
	}

	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := payload[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}

	phone := pick("phoneNumber", "phone", "so_dien_thoai", "tel")
	if phone == "" {
		return nil, fmt.Errorf("request bị chặn không có số điện thoại ⇒ không thể duyệt")
	}
	name := pick("fullName", "name", "ho_ten", "customer_name")

	trackingJSON, _ := json.Marshal(payload)

	// Dùng SaveLead để tận dụng cơ chế chống trùng lặp sẵn có (24h).
	lead, isNew, err := s.SaveLead(
		landingID, nguon, phone,
		pick("url", "entry_url"),
		trackingJSON,
		pick("city"), pick("region"), pick("country"), pick("isp"),
		pick("ad_platform"), pick("ad_placement"),
	)
	if err != nil {
		return nil, fmt.Errorf("không tạo được lead: %w", err)
	}
	if !isNew {
		return nil, fmt.Errorf("số điện thoại %s đã tồn tại trong CRM", phone)
	}

	// Ghi chú lại nguồn gốc (duyệt thủ công từ vùng cách ly).
	_, _ = s.pool.Exec(ctx,
		"UPDATE leads SET ghi_chu = $1 WHERE id = $2",
		"Duyệt thủ công từ vùng cách ly spam (lý do: "+reason+")",
		lead.ID,
	)

	// Xoá khỏi vùng cách ly sau khi duyệt thành công.
	_, _ = s.pool.Exec(ctx, "DELETE FROM spam_quarantine WHERE id = $1", id)

	return map[string]interface{}{
		"success": true,
		"lead_id": lead.ID,
		"phone":   phone,
		"name":    name,
	}, nil
}

// ══════════════════════════════════════════════════════════════════════
// TRUY VẤT HÀNH TRÌNH KHÁCH HÀNG (CUSTOMER JOURNEY)
// ══════════════════════════════════════════════════════════════════════

// GetCustomerJourney trả về hồ sơ hành vi + timeline chi tiết của 1 khách.
//
// `sessionID` = 1 phiên cụ thể.
// `visitorID` = toàn bộ các phiên của 1 khách (khách quay lại).
func (s *Store) GetCustomerJourney(sessionID string, visitorID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var actionsStr string
	var whereClause string
	var args []interface{}

	if sid := strings.TrimSpace(sessionID); sid != "" {
		whereClause = "WHERE session_id = $1"
		args = append(args, sid)
	} else if vid := strings.TrimSpace(visitorID); vid != "" {
		whereClause = "WHERE visitor_id = $1"
		args = append(args, vid)
	} else {
		return nil, fmt.Errorf("thiếu session_id hoặc visitor_id")
	}

	query := fmt.Sprintf(`
		SELECT
			session_id, visitor_id, landing_id, nguon,
			COALESCE(utm_source, ''), COALESCE(utm_medium, ''), COALESCE(utm_campaign, ''),
			COALESCE(utm_content, ''), COALESCE(utm_term, ''),
			COALESCE(campaign_name, ''), COALESCE(adset_name, ''), COALESCE(ad_name, ''),
			COALESCE(site_source_name, ''), COALESCE(utm_placement, ''),
			COALESCE(device_type, ''), COALESCE(browser, ''), COALESCE(browser_version, ''),
			COALESCE(os, ''), COALESCE(os_version, ''),
			COALESCE(device_model_code, ''), COALESCE(device_name, ''),
			COALESCE(device_brand, ''), COALESCE(connection_type, ''),
			COALESCE(viewport_res, ''), COALESCE(screen_res, ''),
			visible_seconds, hidden_seconds, session_count,
			max_scroll_percent, total_clicks,
			has_interacted_form, converted_lead, form_submit_attempts, form_validation_errors,
			is_bot, bot_score, COALESCE(bot_reasons, ''),
			COALESCE(so_dien_thoai, ''), lead_id,
			COALESCE(referrer_host, ''), COALESCE(entry_url, ''),
			COALESCE(actions::text, '[]'),
			TO_CHAR(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS') AS first_seen
		FROM analytics_sessions
		%s
		ORDER BY first_seen ASC;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi lấy hành trình khách hàng: %w", err)
	}
	defer rows.Close()

	sessions := make([]map[string]interface{}, 0)
	timeline := make([]map[string]interface{}, 0)

	for rows.Next() {
		var p models.AnalyticsSession
		if err := rows.Scan(
			&p.SessionID, &p.VisitorID, &p.LandingID, &p.Nguon,
			&p.UTMSource, &p.UTMMedium, &p.UTMCampaign, &p.UTMContent, &p.UTMTerm,
			&p.CampaignName, &p.AdsetName, &p.AdName,
			&p.SiteSourceName, &p.UTMPlacement,
			&p.DeviceType, &p.Browser, &p.BrowserVersion,
			&p.OS, &p.OSVersion,
			&p.DeviceModelCode, &p.DeviceName, &p.DeviceBrand, &p.ConnectionType,
			&p.ViewportRes, &p.ScreenRes,
			&p.VisibleSeconds, &p.HiddenSeconds, &p.SessionCount,
			&p.MaxScrollPercent, &p.TotalClicks,
			&p.HasInteractedForm, &p.ConvertedLead, &p.FormSubmitAttempts, &p.FormValidationError,
			&p.IsBot, &p.BotScore, &p.BotReasons,
			&p.Phone, &p.LeadID,
			&p.ReferrerHost, &p.EntryURL,
			&actionsStr, &p.FirstSeen,
		); err != nil {
			return nil, err
		}

		// Gom timeline từ mảng actions.
		var actions []map[string]interface{}
		if err := json.Unmarshal([]byte(actionsStr), &actions); err == nil {
			for _, a := range actions {
				timeline = append(timeline, map[string]interface{}{
					"session_id": p.SessionID,
					"landing_id": p.LandingID,
					"time":       a["time"],
					"type":       a["type"],
					"target":     a["target"],
					"text":       a["text"],
					"detail":     a["detail"],
					"at":         p.FirstSeen,
				})
			}
		}

		sessions = append(sessions, map[string]interface{}{
			"session_id":             p.SessionID,
			"visitor_id":             p.VisitorID,
			"landing_id":             p.LandingID,
			"nguon":                  p.Nguon,
			"first_seen":             p.FirstSeen,
			"campaign_name":          p.CampaignName,
			"adset_name":             p.AdsetName,
			"ad_name":                p.AdName,
			"site_source_name":       p.SiteSourceName,
			"utm_placement":          p.UTMPlacement,
			"device_type":            p.DeviceType,
			"device_model_code":      p.DeviceModelCode,
			"device_name":            p.DeviceName,
			"browser":                p.Browser,
			"browser_version":        p.BrowserVersion,
			"os":                     p.OS,
			"os_version":             p.OSVersion,
			"connection_type":        p.ConnectionType,
			"viewport_res":           p.ViewportRes,
			"visible_seconds":        p.VisibleSeconds,
			"hidden_seconds":         p.HiddenSeconds,
			"session_count":          p.SessionCount,
			"max_scroll_percent":     p.MaxScrollPercent,
			"total_clicks":           p.TotalClicks,
			"has_interacted_form":    p.HasInteractedForm,
			"converted_lead":         p.ConvertedLead,
			"form_submit_attempts":   p.FormSubmitAttempts,
			"form_validation_errors": p.FormValidationError,
			"is_bot":                 p.IsBot,
			"bot_score":              p.BotScore,
			"bot_reasons":            p.BotReasons,
			"phone":                  p.Phone,
			"lead_id":                p.LeadID,
			"referrer_host":          p.ReferrerHost,
			"entry_url":              p.EntryURL,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("không tìm thấy phiên nào")
	}

	// Sắp xếp timeline theo thời gian.
	sortTimeline(timeline)

	profile := sessions[0]
	return map[string]interface{}{
		"profile":  profile,
		"sessions": sessions,
		"timeline": timeline,
	}, nil
}

// sortTimeline sắp xếp timeline theo giây tương đối (time) trong mỗi phiên.
func sortTimeline(timeline []map[string]interface{}) {
	sort.SliceStable(timeline, func(i, j int) bool {
		ti, _ := timeline[i]["time"].(float64)
		tj, _ := timeline[j]["time"].(float64)
		return ti < tj
	})
}

// ParseReportFilter đọc toàn bộ tham số bộ lọc từ query string.
//
// Dùng chung cho MỌI endpoint báo cáo ⇒ đảm bảo các tab luôn thống nhất.
func ParseReportFilter(q url.Values) ReportFilter {
	f := DefaultReportFilter()

	f.FromDate = strings.TrimSpace(q.Get("from_date"))
	f.ToDate = strings.TrimSpace(q.Get("to_date"))
	f.Days = ParseIntSafe(q.Get("days"), 30)
	if f.Days < 0 {
		f.Days = 0
	}
	if f.Days > 730 {
		f.Days = 730
	}

	// Giờ
	f.FromHour = -1
	f.ToHour = -1
	if v := strings.TrimSpace(q.Get("from_hour")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 23 {
			f.FromHour = n
		}
	}
	if v := strings.TrimSpace(q.Get("to_hour")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 23 {
			f.ToHour = n
		}
	}

	// Landing (multi-select): chấp nhận cả `landing_id` lẫn `landing_ids` (CSV).
	raw := strings.TrimSpace(q.Get("landing_ids"))
	if raw == "" {
		raw = strings.TrimSpace(q.Get("landing_id"))
	}
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if s := strings.TrimSpace(part); s != "" && s != "all" {
				f.LandingIDs = append(f.LandingIDs, s)
			}
		}
	}

	f.Nguon = strings.TrimSpace(q.Get("nguon"))
	f.LandingPath = strings.TrimSpace(q.Get("landing_path"))
	f.AdAccountID = strings.TrimSpace(q.Get("ad_account_id"))
	f.CampaignName = strings.TrimSpace(q.Get("campaign_name"))
	f.AdsetName = strings.TrimSpace(q.Get("adset_name"))
	f.AdName = strings.TrimSpace(q.Get("ad_name"))
	f.SiteSourceName = strings.TrimSpace(q.Get("site_source_name"))
	f.UTMPlacement = strings.TrimSpace(q.Get("utm_placement"))

	f.VisitorType = strings.ToLower(strings.TrimSpace(q.Get("visitor_type")))
	if f.VisitorType == "" {
		f.VisitorType = "all"
	}

	f.DeviceType = strings.TrimSpace(q.Get("device_type"))
	f.OS = strings.TrimSpace(q.Get("os"))
	f.InAppType = strings.TrimSpace(q.Get("in_app_type"))

	f.IsInAppBrowser = -1
	if v := strings.TrimSpace(q.Get("is_in_app_browser")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes":
			f.IsInAppBrowser = 1
		case "0", "false", "no":
			f.IsInAppBrowser = 0
		}
	}

	f.TrafficScope = strings.ToLower(strings.TrimSpace(q.Get("traffic_scope")))
	switch f.TrafficScope {
	case "human", "all", "bot":
	default:
		f.TrafficScope = "human"
	}

	return f
}

// ══════════════════════════════════════════════════════════════════════
// DANH SÁCH GIÁ TRỊ CHO BỘ LỌC TOÀN CỤC
// ══════════════════════════════════════════════════════════════════════

// GetReportFilterOptions trả về các giá trị distinct để đổ vào dropdown.
//
// Mọi truy vấn đều không có tham số (chỉ lấy distinct) ⇒ không cần lọc.
func (s *Store) GetReportFilterOptions() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	// Hàm helper: lấy cột distinct, bỏ rỗng, giới hạn N.
	distinct := func(column string, limit int) []string {
		query := fmt.Sprintf(`
			SELECT DISTINCT %s AS v
			FROM analytics_sessions
			WHERE %s != ''
			ORDER BY v ASC
			LIMIT %d;
		`, column, column, limit)
		rows, err := s.pool.Query(ctx, query)
		if err != nil {
			return []string{}
		}
		defer rows.Close()
		out := make([]string, 0)
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err == nil && strings.TrimSpace(v) != "" {
				out = append(out, v)
			}
		}
		return out
	}

	// Landing: lấy từ cả bảng landing_pages (để landing mới chưa có traffic
	// vẫn xuất hiện trong dropdown) lẫn analytics_sessions.
	landingRows, err := s.pool.Query(ctx, `
		SELECT page_id FROM landing_pages GROUP BY page_id ORDER BY page_id ASC
	`)
	landings := make([]string, 0)
	if err == nil {
		defer landingRows.Close()
		for landingRows.Next() {
			var v string
			if err := landingRows.Scan(&v); err == nil && strings.TrimSpace(v) != "" {
				landings = append(landings, v)
			}
		}
	}
	// Bổ sung landing chỉ có trong analytics (landing đã xoá khỏi bảng landing_pages).
	for _, v := range distinct("landing_id", 100) {
		found := false
		for _, existing := range landings {
			if existing == v {
				found = true
				break
			}
		}
		if !found {
			landings = append(landings, v)
		}
	}

	return map[string]interface{}{
		"landing_ids":       landings,
		"nguon":             distinct("nguon", 50),
		"landing_paths":     distinct("landing_path", 50),
		"ad_account_ids":    distinct("ad_account_id", 30),
		"campaign_names":    distinct("campaign_name", 100),
		"adset_names":       distinct("adset_name", 100),
		"ad_names":          distinct("ad_name", 100),
		"site_source_names": distinct("site_source_name", 20),
		"utm_placements":    distinct("utm_placement", 40),
		"device_types":      distinct("device_type", 10),
		"os":                distinct("os", 20),
		"in_app_types":      distinct("in_app_type", 20),
	}, nil
}

// FindVisitorIDByPhone trả về visitor_id gần nhất của 1 số điện thoại.
//
// Dùng cho modal chi tiết Lead: tra hành trình khách hàng theo SĐT.
func (s *Store) FindVisitorIDByPhone(phone string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var visitorID string
	err := s.pool.QueryRow(ctx, `
		SELECT visitor_id
		FROM analytics_sessions
		WHERE so_dien_thoai = $1 AND visitor_id != ''
		ORDER BY last_seen DESC
		LIMIT 1
	`, phone).Scan(&visitorID)
	if err != nil {
		return "", fmt.Errorf("không tìm thấy phiên của số %s: %w", phone, err)
	}
	return visitorID, nil
}
