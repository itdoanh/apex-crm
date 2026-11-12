package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"apex-backend/models"
)

// analyticsFilter là bộ lọc dùng chung cho các báo cáo analytics mở rộng.
//
// Bắt buộc: mọi so sánh ngày/giờ dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'`
// (quy tắc bất biến #3 trong AGENTS.md).
type AnalyticsFilter struct {
	landingID string
	nguon     string
	fromDate  string // YYYY-MM-DD
	toDate    string // YYYY-MM-DD
	days      int
	fromHour  int // 0-23, -1 = không lọc
	toHour    int // 0-23, -1 = không lọc
}

func (f AnalyticsFilter) where() (string, []interface{}) {
	return buildAnalyticsWhereClauseFullRange(f.landingID, f.nguon, f.fromDate, f.toDate, f.days, f.fromHour, f.toHour)
}

// parseAnalyticsFilter đọc query string chuẩn cho các báo cáo analytics.
func parseAnalyticsFilter(landingID, nguon, fromDate, toDate string, days int) AnalyticsFilter {
	if days <= 0 {
		days = 30
	}
	if days > 730 {
		days = 730
	}
	return AnalyticsFilter{
		landingID: strings.TrimSpace(landingID),
		nguon:     strings.TrimSpace(nguon),
		fromDate:  strings.TrimSpace(fromDate),
		toDate:    strings.TrimSpace(toDate),
		days:      days,
		fromHour:  -1,
		toHour:    -1,
	}
}

// WithHourRange tra ve ban sao cua filter co kenh gio (0-23).
// Dung -1 de khong loc.
func (f AnalyticsFilter) WithHourRange(fromHour, toHour int) AnalyticsFilter {
	f.fromHour = fromHour
	f.toHour = toHour
	return f
}

// NewAnalyticsFilter tạo bộ lọc analytics từ query string (dùng ở handler).
func NewAnalyticsFilter(landingID, nguon, fromDate, toDate string, days int) AnalyticsFilter {
	return parseAnalyticsFilter(landingID, nguon, fromDate, toDate, days)
}

// GetAnalyticsTimeSeries trả về chuỗi thời gian (giờ / ngày / tuần) của
// lượt xem, người thật, bot, lead và tỷ lệ chuyển đổi.
//
// granularity: "hour" | "day" | "week"
func (s *Store) GetAnalyticsTimeSeries(f AnalyticsFilter, granularity string) ([]models.TimeSeriesPoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := f.where()

	var expr string
	switch strings.ToLower(strings.TrimSpace(granularity)) {
	case "hour":
		expr = `TO_CHAR(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:00')`
	case "week":
		expr = `TO_CHAR(DATE_TRUNC('week', first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh'), 'YYYY-MM-DD')`
	default: // day
		expr = `TO_CHAR((first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date, 'YYYY-MM-DD')`
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS bucket,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			COUNT(DISTINCT visitor_id)::int AS unique_visitors,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration,
			COALESCE(AVG(max_scroll_percent) FILTER (WHERE NOT is_bot), 0)::float AS avg_scroll
		FROM analytics_sessions
		%s
		GROUP BY bucket
		ORDER BY bucket ASC;
	`, expr, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn timeseries analytics: %w", err)
	}
	defer rows.Close()

	items := make([]models.TimeSeriesPoint, 0)
	for rows.Next() {
		var it models.TimeSeriesPoint
		if err := rows.Scan(
			&it.Label, &it.Views, &it.HumanVisitors, &it.BotVisitors,
			&it.Leads, &it.UniqueVisitors, &it.CR, &it.AvgDuration, &it.AvgScroll,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// GetDeviceAnalytics thống kê theo thiết bị / trình duyệt / hệ điều hành.
//
// dimension: "model" | "brand" | "browser" | "os" | "inapp" | "viewport" | "resolution"
func (s *Store) GetDeviceAnalytics(f AnalyticsFilter, dimension string, limit int) ([]models.DeviceStatItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := f.where()

	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	// Cột nhóm + cột phụ (brand/model) tương ứng từng chiều thống kê.
	var nameExpr, brandExpr, modelExpr string
	switch strings.ToLower(strings.TrimSpace(dimension)) {
	case "brand":
		nameExpr = `COALESCE(NULLIF(device_brand, ''), 'Khác / Không rõ')`
		brandExpr = nameExpr
		modelExpr = `''`
	case "browser":
		nameExpr = `COALESCE(NULLIF(browser, ''), 'Khác / Không rõ')`
		brandExpr = `''`
		modelExpr = `''`
	case "os":
		nameExpr = `COALESCE(NULLIF(os, ''), 'Khác / Không rõ')`
		brandExpr = `''`
		modelExpr = `''`
	case "inapp":
		nameExpr = `CASE
			WHEN is_in_app_browser THEN COALESCE(NULLIF(in_app_type, ''), 'in_app')
			ELSE 'external_browser' END`
		brandExpr = `''`
		modelExpr = `''`
	case "viewport":
		nameExpr = `COALESCE(NULLIF(viewport_res, ''), 'Không rõ')`
		brandExpr = `''`
		modelExpr = `''`
	case "resolution":
		nameExpr = `COALESCE(NULLIF(screen_res, ''), 'Không rõ')`
		brandExpr = `''`
		modelExpr = `''`
	case "model":
		fallthrough
	default:
		// Ưu tiên tên model đã phân tích (vd: "Samsung Galaxy A52"), fallback mã kỹ thuật.
		nameExpr = `COALESCE(NULLIF(device_name, ''), NULLIF(device_model, ''), NULLIF(device_model_code, ''), 'Khác / Không rõ')`
		brandExpr = `COALESCE(NULLIF(device_brand, ''), 'Khác / Không rõ')`
		modelExpr = `COALESCE(NULLIF(device_model, ''), NULLIF(device_model_code, ''), '')`
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS name,
			%s AS brand,
			%s AS model,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration,
			COALESCE(AVG(max_scroll_percent) FILTER (WHERE NOT is_bot), 0)::float AS avg_scroll
		FROM analytics_sessions
		%s
		GROUP BY name, brand, model
		ORDER BY total_views DESC
		LIMIT %d;
	`, nameExpr, brandExpr, modelExpr, whereClause, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê thiết bị: %w", err)
	}
	defer rows.Close()

	items := make([]models.DeviceStatItem, 0)
	for rows.Next() {
		var it models.DeviceStatItem
		if err := rows.Scan(
			&it.Name, &it.Brand, &it.Model, &it.TotalViews, &it.HumanVisitors,
			&it.Leads, &it.ConversionRate, &it.AvgDurationSec, &it.AvgScrollPct,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// GetMetaMacroAnalytics thống kê theo bộ macro Meta Ads (campaign / adset / ad / placement).
func (s *Store) GetMetaMacroAnalytics(f AnalyticsFilter, limit int) ([]models.MetaMacroStatItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := f.where()

	if limit <= 0 {
		limit = 50
	}
	if limit > 300 {
		limit = 300
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(utm_campaign, ''), 'none') AS campaign_id,
			COALESCE(NULLIF(campaign_name, ''), '') AS campaign_name,
			COALESCE(NULLIF(utm_content, ''), '') AS adset_id,
			COALESCE(NULLIF(adset_name, ''), '') AS adset_name,
			COALESCE(NULLIF(utm_term, ''), '') AS ad_id,
			COALESCE(NULLIF(ad_name, ''), '') AS ad_name,
			COALESCE(NULLIF(utm_placement, ''), '') AS placement,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COUNT(*) FILTER (WHERE fbclid != '' OR fbc != '' OR fbp != '')::int AS meta_clicks
		FROM analytics_sessions
		%s
		GROUP BY campaign_id, campaign_name, adset_id, adset_name, ad_id, ad_name, placement
		ORDER BY total_views DESC
		LIMIT %d;
	`, whereClause, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê macro Meta: %w", err)
	}
	defer rows.Close()

	items := make([]models.MetaMacroStatItem, 0)
	for rows.Next() {
		var it models.MetaMacroStatItem
		if err := rows.Scan(
			&it.CampaignID, &it.CampaignName, &it.AdsetID, &it.AdsetName,
			&it.AdID, &it.AdName, &it.Placement,
			&it.TotalViews, &it.HumanVisitors, &it.Leads, &it.ConversionRate, &it.MetaClicks,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// GetBotAnalytics trả về phân tích bot chi tiết: theo lý do, theo landing, theo UA.
func (s *Store) GetBotAnalytics(f AnalyticsFilter, limit int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := f.where()

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 1. Tổng quan bot
	totalQuery := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total_sessions,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_sessions,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_sessions,
			COALESCE(AVG(bot_score), 0)::float AS avg_bot_score,
			MAX(bot_score)::int AS max_bot_score
		FROM analytics_sessions
		%s;
	`, whereClause)

	var totalSessions, botSessions, humanSessions, maxScore int
	var avgScore float64
	if err := s.pool.QueryRow(ctx, totalQuery, args...).Scan(
		&totalSessions, &botSessions, &humanSessions, &avgScore, &maxScore,
	); err != nil {
		return nil, fmt.Errorf("lỗi tổng quan bot: %w", err)
	}

	botRate := 0.0
	if totalSessions > 0 {
		botRate = float64(botSessions) / float64(totalSessions) * 100
	}

	// 2. Theo lý do phát hiện bot
	reasonQuery := fmt.Sprintf(`
		SELECT
			TRIM(BOTH ';' FROM SPLIT_PART(bot_reasons, ';', 1)) AS reason,
			COUNT(*)::int AS cnt
		FROM analytics_sessions
		%s AND is_bot AND bot_reasons != ''
		GROUP BY reason
		ORDER BY cnt DESC
		LIMIT %d;
	`, whereClause, limit)

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

	// 3. Theo landing
	landingQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(landing_id, ''), 'default_landing') AS lid,
			COUNT(*)::int AS total_sessions,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_sessions,
			CASE
				WHEN COUNT(*) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE is_bot)::numeric / COUNT(*)::numeric) * 100, 2)::float
				ELSE 0
			END AS bot_rate
		FROM analytics_sessions
		%s
		GROUP BY lid
		ORDER BY bot_sessions DESC;
	`, whereClause)

	landingRows, err2 := s.pool.Query(ctx, landingQuery, args...)
	landings := make([]map[string]interface{}, 0)
	if err2 == nil {
		defer landingRows.Close()
		for landingRows.Next() {
			var lid string
			var total, bots int
			var rate float64
			if err := landingRows.Scan(&lid, &total, &bots, &rate); err == nil {
				landings = append(landings, map[string]interface{}{
					"landing_id":     lid,
					"total_sessions": total,
					"bot_sessions":   bots,
					"bot_rate":       rate,
				})
			}
		}
	}

	return map[string]interface{}{
		"total_sessions": totalSessions,
		"bot_sessions":   botSessions,
		"human_sessions": humanSessions,
		"bot_rate":       botRate,
		"avg_bot_score":  avgScore,
		"max_bot_score":  maxScore,
		"reasons":        reasons,
		"landings":       landings,
	}, nil
}

// GetVPNAnalytics thống kê VPN / Proxy / Tor.
func (s *Store) GetVPNAnalytics(f AnalyticsFilter) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := f.where()

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total_sessions,
			COUNT(*) FILTER (WHERE is_vpn)::int AS vpn_sessions,
			COUNT(*) FILTER (WHERE is_proxy)::int AS proxy_sessions,
			COUNT(*) FILTER (WHERE is_tor)::int AS tor_sessions,
			COUNT(*) FILTER (WHERE is_vpn AND converted_lead)::int AS vpn_leads,
			COUNT(*) FILTER (WHERE NOT is_vpn AND converted_lead)::int AS normal_leads
		FROM analytics_sessions
		%s;
	`, whereClause)

	var total, vpn, proxy, tor, vpnLeads, normalLeads int
	if err := s.pool.QueryRow(ctx, query, args...).Scan(
		&total, &vpn, &proxy, &tor, &vpnLeads, &normalLeads,
	); err != nil {
		return nil, fmt.Errorf("lỗi thống kê VPN: %w", err)
	}

	vpnRate := 0.0
	if total > 0 {
		vpnRate = float64(vpn) / float64(total) * 100
	}

	// Danh sách nhà cung cấp VPN
	providerQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(vpn_provider, ''), 'Không rõ') AS provider,
			COUNT(*)::int AS cnt,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads
		FROM analytics_sessions
		%s AND is_vpn
		GROUP BY provider
		ORDER BY cnt DESC
		LIMIT 20;
	`, whereClause)

	providerRows, err := s.pool.Query(ctx, providerQuery, args...)
	providers := make([]map[string]interface{}, 0)
	if err == nil {
		defer providerRows.Close()
		for providerRows.Next() {
			var name string
			var cnt, leads int
			if err := providerRows.Scan(&name, &cnt, &leads); err == nil {
				providers = append(providers, map[string]interface{}{
					"name": name, "count": cnt, "leads": leads,
				})
			}
		}
	}

	return map[string]interface{}{
		"total_sessions":  total,
		"vpn_sessions":    vpn,
		"proxy_sessions":  proxy,
		"tor_sessions":    tor,
		"vpn_leads":       vpnLeads,
		"normal_leads":    normalLeads,
		"vpn_rate":        vpnRate,
		"vpn_conversion":  vpnLeads,
		"providers":       providers,
	}, nil
}

// GetLandingReport là báo cáo TỔNG HỢP riêng cho 1 landing page.
//
// Gom đủ: tổng quan, timeseries, thiết bị, macro Meta, bot, VPN, geo, nguồn.
// Đây là API chính cho tab "Báo cáo Landing" trong CRM.
func (s *Store) GetLandingReport(landingID string, nguon string, days int) (map[string]interface{}, error) {
	return s.GetLandingReportRange(landingID, nguon, "", "", days, -1, -1)
}

// GetLandingReportRange báo cáo TỔNG HỢP riêng cho 1 landing page, có hỗ trợ
// khoảng ngày tuỳ chọn (fromDate/toDate) — dùng cho bộ lọc "Từ ngày / Đến ngày"
// trên CRM.
func (s *Store) GetLandingReportRange(landingID string, nguon string, fromDate string, toDate string, days int, fromHour int, toHour int) (map[string]interface{}, error) {
	f := parseAnalyticsFilter(landingID, nguon, fromDate, toDate, days).WithHourRange(fromHour, toHour)

	overview, err := s.GetAnalyticsOverview(f.landingID, f.nguon)
	if err != nil {
		return nil, err
	}

	daily, err := s.GetAnalyticsTimeSeries(f, "day")
	if err != nil {
		return nil, err
	}
	hourly, err := s.GetAnalyticsTimeSeries(f, "hour")
	if err != nil {
		return nil, err
	}
	weekly, err := s.GetAnalyticsTimeSeries(f, "week")
	if err != nil {
		return nil, err
	}

	devices, err := s.GetDeviceAnalytics(f, "model", 25)
	if err != nil {
		return nil, err
	}
	brands, err := s.GetDeviceAnalytics(f, "brand", 20)
	if err != nil {
		return nil, err
	}
	browsers, err := s.GetDeviceAnalytics(f, "browser", 20)
	if err != nil {
		return nil, err
	}
	inApp, err := s.GetDeviceAnalytics(f, "inapp", 10)
	if err != nil {
		return nil, err
	}

	macros, err := s.GetMetaMacroAnalytics(f, 50)
	if err != nil {
		return nil, err
	}
	botStats, err := s.GetBotAnalytics(f, 20)
	if err != nil {
		return nil, err
	}
	vpnStats, err := s.GetVPNAnalytics(f)
	if err != nil {
		return nil, err
	}
	geo, err := s.GetGeoAnalytics(f.landingID, f.nguon, f.fromDate, f.toDate, f.fromHour, f.toHour)
	if err != nil {
		return nil, err
	}
	sources, err := s.GetSourcePerformance(f.landingID, f.nguon, f.fromDate, f.toDate, f.fromHour, f.toHour)
	if err != nil {
		return nil, err
	}
	landings, err := s.GetLandingPerformance(f.landingID, f.nguon, f.fromDate, f.toDate, f.fromHour, f.toHour)
	if err != nil {
		return nil, err
	}
	placements, err := s.GetAdPlacementAnalytics(f.landingID, f.nguon, f.fromDate, f.toDate, f.fromHour, f.toHour)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"landing_id":  f.landingID,
		"nguon":       f.nguon,
		"days":        f.days,
		"overview":    overview,
		"timeseries":  map[string]interface{}{"day": daily, "hour": hourly, "week": weekly},
		"devices":     devices,
		"brands":      brands,
		"browsers":    browsers,
		"in_app":      inApp,
		"meta_macros": macros,
		"bots":        botStats,
		"vpn":         vpnStats,
		"geo":         geo,
		"sources":     sources,
		"landings":    landings,
		"placements":  placements,
	}, nil
}

// GetOverviewExtended trả về tổng quan analytics MỞ RỘNG (thay thế GetAnalyticsOverview cũ).
//
// Bao gồm: tổng view / người thật / bot / lead / CR, người mới vs quay lại,
// form interaction, VPN, in-app browser, scroll funnel, thiết bị distinct,
// và số liệu theo ngày (hôm / hôm qua / tuần / tháng).
//
// fromDate/toDate: YYYY-MM-DD. Rỗng = không giới hạn.
func (s *Store) GetOverviewExtended(landingID string, nguon string, fromDate string, toDate string, fromHour int, toHour int) (*models.AnalyticsOverview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate, 0, fromHour, toHour)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS total_leads,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration,
			COALESCE(AVG(max_scroll_percent) FILTER (WHERE NOT is_bot), 0)::float AS avg_scroll,
			COUNT(*) FILTER (WHERE device_type = 'mobile')::int AS mobile_views,
			COUNT(*) FILTER (WHERE device_type = 'desktop')::int AS desktop_views,
			COUNT(*) FILTER (WHERE device_type = 'tablet')::int AS tablet_views,
			COUNT(DISTINCT visitor_id)::int AS total_visitors,
			COUNT(DISTINCT visitor_id) FILTER (WHERE is_new_visitor)::int AS new_visitors,
			COUNT(DISTINCT visitor_id) FILTER (WHERE NOT is_new_visitor)::int AS returning_visitors,
			COUNT(*) FILTER (WHERE has_interacted_form)::int AS form_interactions,
			COALESCE(SUM(form_submit_attempts), 0)::int AS form_submit_attempts,
			COALESCE(SUM(form_validation_errors), 0)::int AS form_validation_errors,
			COALESCE(AVG(total_clicks) FILTER (WHERE NOT is_bot), 0)::float AS avg_clicks,
			COALESCE(AVG(visible_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_visible,
			COALESCE(AVG(hidden_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_hidden,
			COALESCE(AVG(session_count) FILTER (WHERE NOT is_bot), 0)::float AS avg_session_count,
			COUNT(*) FILTER (WHERE is_vpn)::int AS vpn_visitors,
			COUNT(*) FILTER (WHERE is_proxy)::int AS proxy_visitors,
			COUNT(*) FILTER (WHERE is_in_app_browser)::int AS in_app_visitors,
			COUNT(*) FILTER (WHERE has_video_play)::int AS video_viewers,
			COUNT(*) FILTER (WHERE has_zalo_click)::int AS zalo_clickers,
			COUNT(*) FILTER (WHERE has_tel_click)::int AS tel_clickers,
			COUNT(*) FILTER (WHERE scroll_depth_25)::int AS scroll25,
			COUNT(*) FILTER (WHERE scroll_depth_50)::int AS scroll50,
			COUNT(*) FILTER (WHERE scroll_depth_75)::int AS scroll75,
			COUNT(*) FILTER (WHERE scroll_depth_90)::int AS scroll90,
			COUNT(*) FILTER (WHERE scroll_depth_100)::int AS scroll100,
			COUNT(*) FILTER (WHERE fbclid != '' OR fbc != '' OR fbp != '')::int AS meta_clicks,
			COUNT(DISTINCT NULLIF(device_model_code, ''))::int AS distinct_devices,
			COUNT(DISTINCT NULLIF(device_name, ''))::int AS distinct_device_models,
			COALESCE(SUM(jsonb_array_length(COALESCE(actions, '[]'::jsonb))), 0)::int AS total_actions,
			COUNT(*) FILTER (WHERE (first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date)::int AS today_views,
			COUNT(*) FILTER (WHERE (first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date AND converted_lead)::int AS today_leads,
			COUNT(*) FILTER (WHERE (first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - 1))::int AS yesterday_views,
			COUNT(*) FILTER (WHERE (first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - 6))::int AS week_views,
			COUNT(*) FILTER (WHERE (first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - 29))::int AS month_views
		FROM analytics_sessions
		%s;
	`, whereClause)

	var ov models.AnalyticsOverview
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&ov.TotalViews, &ov.HumanVisitors, &ov.BotVisitors, &ov.TotalLeads,
		&ov.AvgDurationSec, &ov.AvgMaxScrollPct,
		&ov.MobileViews, &ov.DesktopViews, &ov.TabletViews,
		&ov.TotalVisitors, &ov.NewVisitors, &ov.ReturningVisitors,
		&ov.FormInteractions, &ov.FormSubmitAttempts, &ov.FormValidationErrors,
		&ov.AvgClicksPerSession, &ov.AvgVisibleSeconds, &ov.AvgHiddenSeconds, &ov.AvgSessionCount,
		&ov.VPNVisitors, &ov.ProxyVisitors, &ov.InAppVisitors,
		&ov.VideoViewers, &ov.ZaloClickers, &ov.TelClickers,
		&ov.Scroll25Count, &ov.Scroll50Count, &ov.Scroll75Count, &ov.Scroll90Count, &ov.Scroll100Count,
		&ov.MetaClickCount, &ov.DistinctDevices, &ov.DistinctDeviceModels, &ov.TotalActions,
		&ov.TodayViews, &ov.TodayLeads, &ov.YesterdayViews, &ov.WeekViews, &ov.MonthViews,
	)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn tổng quan analytics mở rộng: %w", err)
	}

	if ov.TotalViews > 0 {
		ov.BotRatePercent = float64(ov.BotVisitors) / float64(ov.TotalViews) * 100
		ov.VPNRatePercent = float64(ov.VPNVisitors) / float64(ov.TotalViews) * 100
	}
	if ov.HumanVisitors > 0 {
		ov.ConversionRate = float64(ov.TotalLeads) / float64(ov.HumanVisitors) * 100
	}
	ov.TotalEvents = ov.TotalActions

	return &ov, nil
}

// GetSessionsStatsSummary trả về tổng hợp số phiên theo trạng thái bot / chuyển đổi
// (dùng cho bộ lọc nhanh trong CRM).
func (s *Store) GetSessionsStatsSummary(landingID string, nguon string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClause(landingID, nguon)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total,
			COUNT(*) FILTER (WHERE is_bot)::int AS bots,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS humans,
			COUNT(*) FILTER (WHERE converted_lead)::int AS converted,
			COUNT(*) FILTER (WHERE has_interacted_form)::int AS interacted,
			COUNT(*) FILTER (WHERE is_vpn)::int AS vpn
		FROM analytics_sessions
		%s;
	`, whereClause)

	var total, bots, humans, converted, interacted, vpn int
	if err := s.pool.QueryRow(ctx, query, args...).Scan(
		&total, &bots, &humans, &converted, &interacted, &vpn,
	); err != nil {
		return nil, fmt.Errorf("lỗi tổng hợp phiên: %w", err)
	}

	return map[string]interface{}{
		"total":      total,
		"bots":       bots,
		"humans":     humans,
		"converted":  converted,
		"interacted": interacted,
		"vpn":        vpn,
	}, nil
}

// ParseIntSafe chuyển chuỗi sang int, mặc định khi lỗi.
func ParseIntSafe(s string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

