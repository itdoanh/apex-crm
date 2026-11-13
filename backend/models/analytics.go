package models

import "encoding/json"

// AnalyticsEventPayload chứa dữ liệu gửi từ tracker.js lên /api/analytics/track
type AnalyticsEventPayload struct {
	SessionID         string          `json:"session_id"`
	VisitorID         string          `json:"visitor_id"`
	LandingID         string          `json:"landing_id"`
	Nguon             string          `json:"nguon"`
	UTMSource         string          `json:"utm_source"`
	UTMMedium         string          `json:"utm_medium"`
	UTMCampaign       string          `json:"utm_campaign"`
	UTMContent        string          `json:"utm_content"`
	UTMTerm           string          `json:"utm_term"`
	Referrer          string          `json:"referrer"`
	EntryURL          string          `json:"entry_url"`
	DeviceType        string          `json:"device_type"`
	Browser           string          `json:"browser"`
	OS                string          `json:"os"`
	ScreenRes         string          `json:"screen_res"`
	IsBot             bool            `json:"is_bot"`
	BotScore          int             `json:"bot_score"`
	BotReasons        string          `json:"bot_reasons"`
	DurationSeconds   int             `json:"duration_seconds"`
	MaxScrollPercent  int             `json:"max_scroll_percent"`
	ClicksCount       int             `json:"clicks_count"`
	HasInteractedForm bool            `json:"has_interacted_form"`
	ConvertedLead     bool            `json:"converted_lead"`
	Phone             string          `json:"so_dien_thoai"`
	LeadID            *uint64         `json:"lead_id,omitempty"`
	City              string          `json:"city,omitempty"`
	Region            string          `json:"region,omitempty"`
	Country           string          `json:"country,omitempty"`
	ISP               string          `json:"isp,omitempty"`
	AdPlatform        string          `json:"ad_platform,omitempty"`
	AdPlacement       string          `json:"ad_placement,omitempty"`
	Events            []ActionItem    `json:"events"`
	Actions           []ActionItem    `json:"actions,omitempty"`
	RawData           json.RawMessage `json:"raw_data,omitempty"`

	// ─── Thiết bị chi tiết (2026-11-12) ───────────────────────────
	DeviceBrand       string `json:"device_brand,omitempty"`
	DeviceModel       string `json:"device_model,omitempty"`
	DeviceModelCode   string `json:"device_model_code,omitempty"`
	DeviceName        string `json:"device_name,omitempty"`
	OSVersion         string `json:"os_version,omitempty"`
	BrowserVersion    string `json:"browser_version,omitempty"`
	BrowserEngine     string `json:"browser_engine,omitempty"`
	IsInAppBrowser    bool   `json:"is_in_app_browser,omitempty"`
	InAppType         string `json:"in_app_type,omitempty"`
	DeviceMemory      int    `json:"device_memory,omitempty"`
	DeviceCores       int    `json:"device_cores,omitempty"`
	DeviceTouchpoints int    `json:"device_touchpoints,omitempty"`
	ViewportRes       string `json:"viewport_res,omitempty"`
	ScreenColorDepth  int    `json:"screen_color_depth,omitempty"`
	PixelRatio        string `json:"pixel_ratio,omitempty"`
	Timezone          string `json:"timezone,omitempty"`
	Language          string `json:"language,omitempty"`
	PlatformInfo      string `json:"platform_info,omitempty"`
	ConnectionType    string `json:"connection_type,omitempty"`
	DeviceState       string `json:"device_state,omitempty"`

	// ─── VPN / Proxy ──────────────────────────────────────────────
	IsVPN         bool   `json:"is_vpn,omitempty"`
	IsProxy       bool   `json:"is_proxy,omitempty"`
	IsTor         bool   `json:"is_tor,omitempty"`
	VPNProvider   string `json:"vpn_provider,omitempty"`
	VPNConfidence int    `json:"vpn_confidence,omitempty"`
	VPNReason     string `json:"vpn_reason,omitempty"`

	// ─── Meta Click ID (CAPI) ─────────────────────────────────────
	Fbclid      string `json:"fbclid,omitempty"`
	Fbc         string `json:"fbc,omitempty"`
	Fbp         string `json:"fbp,omitempty"`
	Igshid      string `json:"igshid,omitempty"`
	MetaClickID string `json:"meta_click_id,omitempty"`

	// ─── Meta Ads Macros ──────────────────────────────────────────
	UTMPlacement   string `json:"utm_placement,omitempty"`
	CampaignName   string `json:"campaign_name,omitempty"`
	AdsetName      string `json:"adset_name,omitempty"`
	AdName         string `json:"ad_name,omitempty"`
	AdAccountID    string `json:"ad_account_id,omitempty"`
	SiteSourceName string `json:"site_source_name,omitempty"`

	// ─── Hành vi chi tiết ─────────────────────────────────────────
	VisibleSeconds      int  `json:"visible_seconds,omitempty"`
	HiddenSeconds       int  `json:"hidden_seconds,omitempty"`
	SessionCount        int  `json:"session_count,omitempty"`
	LandingPath         string `json:"landing_path,omitempty"`
	ReferrerHost        string `json:"referrer_host,omitempty"`
	IsNewVisitor        bool   `json:"is_new_visitor,omitempty"`
	HasVideoPlay        bool   `json:"has_video_play,omitempty"`
	HasZaloClick        bool   `json:"has_zalo_click,omitempty"`
	HasTelClick         bool   `json:"has_tel_click,omitempty"`
	FormSubmitAttempts  int    `json:"form_submit_attempts,omitempty"`
	FormValidationError int    `json:"form_validation_errors,omitempty"`
	ScrollDepth25       bool   `json:"scroll_depth_25,omitempty"`
	ScrollDepth50       bool   `json:"scroll_depth_50,omitempty"`
	ScrollDepth75       bool   `json:"scroll_depth_75,omitempty"`
	ScrollDepth90       bool   `json:"scroll_depth_90,omitempty"`
	ScrollDepth100      bool   `json:"scroll_depth_100,omitempty"`

	// ─── Phễu biểu mẫu & autofill (mức PHIÊN — 2026-11-13) ─────
	//
	// Trước đây các trường này chỉ có trong payload `/api/leads` (lưu vào
	// `leads.tracking`) ⇒ không thống kê được cho phiên BỎ form.
	IsAutofill       bool   `json:"is_autofill,omitempty"`
	AutofillSource   string `json:"autofill_source,omitempty"`
	FillDurationMs   int    `json:"fill_duration_ms,omitempty"`
	ValidationPassed bool   `json:"validation_passed,omitempty"`
	FormFocusMs      int    `json:"form_focus_ms,omitempty"`
}

// ActionItem đại diện cho một hành vi của người dùng trong Timeline
type ActionItem struct {
	Time    int         `json:"time"`   // Giây thứ mấy từ khi vào trang
	Type    string      `json:"type"`   // 'pageview', 'scroll', 'click', 'form_focus', 'form_input', 'converted', 'exit'
	Target  string      `json:"target"` // Nút nào, ô input nào, link nào
	Text    string      `json:"text,omitempty"`
	Detail  interface{} `json:"detail,omitempty"`
	At      string      `json:"at,omitempty"`
}

// AnalyticsSession đại diện cho 1 phiên truy cập trong CSDL
type AnalyticsSession struct {
	SessionID         string          `json:"session_id"`
	VisitorID         string          `json:"visitor_id"`
	LandingID         string          `json:"landing_id"`
	Nguon             string          `json:"nguon"`
	UTMSource         string          `json:"utm_source"`
	UTMMedium         string          `json:"utm_medium"`
	UTMCampaign       string          `json:"utm_campaign"`
	UTMContent        string          `json:"utm_content"`
	UTMTerm           string          `json:"utm_term"`
	Referrer          string          `json:"referrer"`
	EntryURL          string          `json:"entry_url"`
	IPAddress         string          `json:"ip_address"`
	City              string          `json:"city"`
	Region            string          `json:"region"`
	Country           string          `json:"country"`
	ISP               string          `json:"isp"`
	AdPlatform        string          `json:"ad_platform"`
	AdPlacement       string          `json:"ad_placement"`
	UserAgent         string          `json:"user_agent"`
	DeviceType        string          `json:"device_type"`
	Browser           string          `json:"browser"`
	OS                string          `json:"os"`
	ScreenRes         string          `json:"screen_res"`
	IsBot             bool            `json:"is_bot"`
	BotScore          int             `json:"bot_score"`
	BotReasons        string          `json:"bot_reasons"`
	DurationSeconds   int             `json:"duration_seconds"`
	MaxScrollPercent  int             `json:"max_scroll_percent"`
	TotalClicks       int             `json:"total_clicks"`
	HasInteractedForm bool            `json:"has_interacted_form"`
	ConvertedLead     bool            `json:"converted_lead"`
	Phone             string          `json:"so_dien_thoai"`
	LeadID            *uint64         `json:"lead_id,omitempty"`
	Actions           json.RawMessage `json:"actions"`
	FirstSeen         string          `json:"first_seen"`
	LastSeen          string          `json:"last_seen"`

	// ─── Thiết bị chi tiết ────────────────────────────────────────
	DeviceBrand       string `json:"device_brand"`
	DeviceModel       string `json:"device_model"`
	DeviceModelCode   string `json:"device_model_code"`
	DeviceName        string `json:"device_name"`
	OSVersion         string `json:"os_version"`
	BrowserVersion    string `json:"browser_version"`
	BrowserEngine     string `json:"browser_engine"`
	IsInAppBrowser    bool   `json:"is_in_app_browser"`
	InAppType         string `json:"in_app_type"`
	DeviceMemory      int    `json:"device_memory"`
	DeviceCores       int    `json:"device_cores"`
	DeviceTouchpoints int    `json:"device_touchpoints"`
	ViewportRes       string `json:"viewport_res"`
	ScreenColorDepth  int    `json:"screen_color_depth"`
	PixelRatio        string `json:"pixel_ratio"`
	Timezone          string `json:"timezone"`
	Language          string `json:"language"`
	PlatformInfo      string `json:"platform_info"`
	ConnectionType    string `json:"connection_type"`
	DeviceState       string `json:"device_state"`

	// ─── VPN / Proxy ──────────────────────────────────────────────
	IsVPN         bool   `json:"is_vpn"`
	IsProxy       bool   `json:"is_proxy"`
	IsTor         bool   `json:"is_tor"`
	VPNProvider   string `json:"vpn_provider"`
	VPNConfidence int    `json:"vpn_confidence"`
	VPNReason     string `json:"vpn_reason"`

	// ─── Meta Click ID ────────────────────────────────────────────
	Fbclid      string `json:"fbclid"`
	Fbc         string `json:"fbc"`
	Fbp         string `json:"fbp"`
	Igshid      string `json:"igshid"`
	MetaClickID string `json:"meta_click_id"`

	// ─── Meta Ads Macros ──────────────────────────────────────────
	UTMPlacement   string `json:"utm_placement"`
	CampaignName   string `json:"campaign_name"`
	AdsetName      string `json:"adset_name"`
	AdName         string `json:"ad_name"`
	AdAccountID    string `json:"ad_account_id"`
	SiteSourceName string `json:"site_source_name"`

	// ─── Hành vi chi tiết ─────────────────────────────────────────
	VisibleSeconds      int    `json:"visible_seconds"`
	HiddenSeconds       int    `json:"hidden_seconds"`
	SessionCount        int    `json:"session_count"`
	LandingPath         string `json:"landing_path"`
	ReferrerHost        string `json:"referrer_host"`
	IsNewVisitor        bool   `json:"is_new_visitor"`
	HasVideoPlay        bool   `json:"has_video_play"`
	HasZaloClick        bool   `json:"has_zalo_click"`
	HasTelClick         bool   `json:"has_tel_click"`
	FormSubmitAttempts  int    `json:"form_submit_attempts"`
	FormValidationError int    `json:"form_validation_errors"`
	ScrollDepth25       bool   `json:"scroll_depth_25"`
	ScrollDepth50       bool   `json:"scroll_depth_50"`
	ScrollDepth75       bool   `json:"scroll_depth_75"`
	ScrollDepth90       bool   `json:"scroll_depth_90"`
	ScrollDepth100      bool   `json:"scroll_depth_100"`

	// ─── Phễu biểu mẫu & autofill (2026-11-13) ───────────────────
	IsAutofill       bool   `json:"is_autofill"`
	AutofillSource   string `json:"autofill_source"`
	FillDurationMs   int    `json:"fill_duration_ms"`
	ValidationPassed bool   `json:"validation_passed"`
	FormFocusMs      int    `json:"form_focus_ms"`
}

// GeoStatItem thống kê lượt truy cập và lead theo khu vực địa lý
type GeoStatItem struct {
	Name           string  `json:"name"`            // Tên Tỉnh/Thành phố hoặc Vùng miền
	Region         string  `json:"region,omitempty"`// Miền Bắc, Miền Trung, Miền Nam
	TotalViews     int     `json:"total_views"`
	HumanVisitors  int     `json:"human_visitors"`
	Leads          int     `json:"leads"`
	ConversionRate float64 `json:"conversion_rate"`
}

// AdPlacementStatItem thống kê lượt truy cập và lead theo nền tảng & vị trí quảng cáo
type AdPlacementStatItem struct {
	Platform       string  `json:"platform"`        // Meta Ads, Google Ads, TikTok Ads, Zalo Ads, Direct...
	Placement      string  `json:"placement"`       // Instagram, Facebook Feed, Audience Network, Google Search, GDN, YouTube...
	TotalViews     int     `json:"total_views"`
	HumanVisitors  int     `json:"human_visitors"`
	Leads          int     `json:"leads"`
	ConversionRate float64 `json:"conversion_rate"`
	AvgDurationSec float64 `json:"avg_duration_seconds"`
}

// AnalyticsOverview số liệu tổng quan
type AnalyticsOverview struct {
	TotalViews        int     `json:"total_views"`
	HumanVisitors     int     `json:"human_visitors"`
	BotVisitors       int     `json:"bot_visitors"`
	BotRatePercent    float64 `json:"bot_rate_percent"`
	TotalLeads        int     `json:"total_leads"`
	ConversionRate    float64 `json:"conversion_rate"`
	AvgDurationSec    float64 `json:"avg_duration_seconds"`
	AvgMaxScrollPct   float64 `json:"avg_max_scroll_percent"`
	MobileViews       int     `json:"mobile_views"`
	DesktopViews      int     `json:"desktop_views"`
	TabletViews       int     `json:"tablet_views"`

	// ─── Mở rộng 2026-11-12 ──────────────────────────────────────
	TotalVisitors       int     `json:"total_visitors"`
	NewVisitors         int     `json:"new_visitors"`
	ReturningVisitors   int     `json:"returning_visitors"`
	FormInteractions     int     `json:"form_interactions"`
	FormSubmitAttempts   int     `json:"form_submit_attempts"`
	FormValidationErrors int     `json:"form_validation_errors"`
	AvgClicksPerSession  float64 `json:"avg_clicks_per_session"`
	AvgVisibleSeconds    float64 `json:"avg_visible_seconds"`
	AvgHiddenSeconds     float64 `json:"avg_hidden_seconds"`
	AvgSessionCount      float64 `json:"avg_session_count"`
	VPNVisitors          int     `json:"vpn_visitors"`
	VPNRatePercent       float64 `json:"vpn_rate_percent"`
	ProxyVisitors        int     `json:"proxy_visitors"`
	InAppVisitors        int     `json:"in_app_visitors"`
	VideoViewers         int     `json:"video_viewers"`
	ZaloClickers         int     `json:"zalo_clickers"`
	TelClickers          int     `json:"tel_clickers"`
	Scroll25Count        int     `json:"scroll_25_count"`
	Scroll50Count        int     `json:"scroll_50_count"`
	Scroll75Count        int     `json:"scroll_75_count"`
	Scroll90Count        int     `json:"scroll_90_count"`
	Scroll100Count       int     `json:"scroll_100_count"`
	AutofillCount        int     `json:"autofill_count"`
	MetaClickCount       int     `json:"meta_click_count"`
	DistinctDevices      int     `json:"distinct_devices"`
	DistinctDeviceModels int     `json:"distinct_device_models"`
	TodayViews           int     `json:"today_views"`
	TodayLeads           int     `json:"today_leads"`
	YesterdayViews       int     `json:"yesterday_views"`
	WeekViews            int     `json:"week_views"`
	MonthViews           int     `json:"month_views"`
	TotalEvents          int     `json:"total_events"`
	TotalActions         int     `json:"total_actions"`
}

// LandingStatItem thống kê hiệu quả của từng Landing Page
type LandingStatItem struct {
	LandingID        string  `json:"landing_id"`
	TotalViews       int     `json:"total_views"`
	HumanVisitors    int     `json:"human_visitors"`
	BotVisitors      int     `json:"bot_visitors"`
	FormInteractions int     `json:"form_interactions"`
	Leads            int     `json:"leads"`
	ConversionRate   float64 `json:"conversion_rate"`
	AvgDurationSec   float64 `json:"avg_duration_seconds"`
	AvgScrollPct     float64 `json:"avg_scroll_percent"`

	// ─── Mở rộng 2026-11-12 ──────────────────────────────────────
	TotalVisitors   int     `json:"total_visitors"`
	UniqueVisitors  int     `json:"unique_visitors"`
	NewVisitors     int     `json:"new_visitors"`
	VPNVisitors     int     `json:"vpn_visitors"`
	AvgClicks       float64 `json:"avg_clicks"`
	AvgVisibleSec   float64 `json:"avg_visible_seconds"`
	FormAttempts    int     `json:"form_submit_attempts"`
	AutofillCount   int     `json:"autofill_count"`
	TopDevice       string  `json:"top_device,omitempty"`
	TopBrowser      string  `json:"top_browser,omitempty"`
	TopPlacement    string  `json:"top_placement,omitempty"`
	TopSource       string  `json:"top_source,omitempty"`
	MetaClicks      int     `json:"meta_clicks"`
	TotalEvents     int     `json:"total_events"`
	AvgSessionCount float64 `json:"avg_session_count"`
}

// SourceStatItem thống kê hiệu quả của từng Nguồn traffic & Content quảng cáo
type SourceStatItem struct {
	Nguon          string  `json:"nguon"`
	UTMCampaign    string  `json:"utm_campaign"`
	UTMContent     string  `json:"utm_content"`
	TotalViews     int     `json:"total_views"`
	HumanVisitors  int     `json:"human_visitors"`
	BotVisitors    int     `json:"bot_visitors"`
	Leads          int     `json:"leads"`
	ConversionRate float64 `json:"conversion_rate"`
	AvgDurationSec float64 `json:"avg_duration_seconds"`

	// ─── Mở rộng 2026-11-12 ──────────────────────────────────────
	UTMPlacement   string  `json:"utm_placement,omitempty"`
	CampaignName   string  `json:"campaign_name,omitempty"`
	AdsetName      string  `json:"adset_name,omitempty"`
	AdName         string  `json:"ad_name,omitempty"`
	SiteSourceName string  `json:"site_source_name,omitempty"`
	MetaClicks     int     `json:"meta_clicks"`
	VPNVisitors    int     `json:"vpn_visitors"`
	AvgClicks      float64 `json:"avg_clicks"`
	AvgVisibleSec  float64 `json:"avg_visible_seconds"`
	AutofillCount  int     `json:"autofill_count"`
}

// TimeSeriesPoint là 1 điểm dữ liệu trên biểu đồ theo thời gian.
type TimeSeriesPoint struct {
	Label         string  `json:"label"`
	Views         int     `json:"views"`
	HumanVisitors int     `json:"human_visitors"`
	BotVisitors   int     `json:"bot_visitors"`
	Leads         int     `json:"leads"`
	CR            float64 `json:"conversion_rate"`
	AvgDuration   float64 `json:"avg_duration_seconds"`
	AvgScroll     float64 `json:"avg_scroll_percent"`
	UniqueVisitors int    `json:"unique_visitors"`
}

// DeviceStatItem thống kê theo thiết bị / trình duyệt.
type DeviceStatItem struct {
	Name           string  `json:"name"`
	Brand          string  `json:"brand,omitempty"`
	Model          string  `json:"model,omitempty"`
	OS             string  `json:"os,omitempty"`
	Browser        string  `json:"browser,omitempty"`
	TotalViews     int     `json:"total_views"`
	HumanVisitors  int     `json:"human_visitors"`
	Leads          int     `json:"leads"`
	ConversionRate float64 `json:"conversion_rate"`
	AvgDurationSec float64 `json:"avg_duration_seconds"`
	AvgScrollPct   float64 `json:"avg_scroll_percent"`
}

// MetaMacroStatItem thống kê theo macro Meta Ads.
type MetaMacroStatItem struct {
	CampaignID     string  `json:"campaign_id"`
	CampaignName   string  `json:"campaign_name,omitempty"`
	AdsetID        string  `json:"adset_id,omitempty"`
	AdsetName      string  `json:"adset_name,omitempty"`
	AdID           string  `json:"ad_id,omitempty"`
	AdName         string  `json:"ad_name,omitempty"`
	Placement      string  `json:"placement,omitempty"`
	TotalViews     int     `json:"total_views"`
	HumanVisitors  int     `json:"human_visitors"`
	Leads          int     `json:"leads"`
	ConversionRate float64 `json:"conversion_rate"`
	MetaClicks     int     `json:"meta_clicks"`
}
