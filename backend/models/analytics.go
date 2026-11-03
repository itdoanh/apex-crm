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
}
