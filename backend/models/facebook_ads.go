package models

import (
	"encoding/json"
	"time"
)

// ─── Meta Conversions API (CAPI) ────────────────────────────────────────
//
// Cấu hình CAPI được lưu theo từng "landing" (landing_id) vì mỗi landing
// có thể dùng pixel riêng. Nếu landing_id = "" thì đó là cấu hình mặc định
// (fallback cho mọi landing chưa có cấu hình riêng).

// FacebookCAPIConfig là cấu hình Meta Conversions API cho 1 landing.
type FacebookCAPIConfig struct {
	ID        uint64 `json:"id"`
	LandingID string `json:"landing_id"`
	IsActive  bool   `json:"is_active"`

	// Pixel / Dataset
	PixelID string `json:"pixel_id"`
	// Access token của hệ thống (System User) hoặc token người dùng
	AccessToken string `json:"access_token"`
	// Phiên bản Graph API, mặc định v27.0
	APIVersion string `json:"api_version"`

	// ─── Cổng API Chuyển đổi (Conversions API Gateway) ──────────────
	// Nếu bật, mọi request sẽ đi qua Gateway thay vì gọi trực tiếp Graph API.
	// Gateway URL dạng: https://<region>.gateway.facebook.com/...
	GatewayEnabled   bool   `json:"gateway_enabled"`
	GatewayEndpoint  string `json:"gateway_endpoint"`
	GatewayAccessKey string `json:"gateway_access_key"`

	// ─── Tự động gửi sự kiện ─────────────────────────────────────────
	// Bật/tắt từng loại sự kiện server-side.
	SendPageView bool `json:"send_page_view"`
	SendLead     bool `json:"send_lead"`
	SendPurchase bool `json:"send_purchase"`
	// Gửi sự kiện dựa trên thay đổi trạng thái CRM trong CRM (khoá học chuyên sâu, mua hàng).
	SendCRMSync bool `json:"send_crm_sync"`

	// ─── Giá trị chuyển đổi ───────────────────────────────────────────
	// Đơn giá khoá học chuyên sâu (VND). Meta yêu cầu value là số thực,
	// không phải số tiền tuyệt đối — quy ước 1 đơn vị = 1 VND.
	CourseValue float64 `json:"course_value"`
	Currency    string  `json:"currency"`

	// Hash các thuộc tính PII khi gửi lên Meta.
	HashEmail      bool `json:"hash_email"`
	HashPhone      bool `json:"hash_phone"`
	HashName       bool `json:"hash_name"`
	HashAddress    bool `json:"hash_address"`
	HashExternalID bool `json:"hash_external_id"`
	HashCity       bool `json:"hash_city"`
	HashState      bool `json:"hash_state"`
	HashZip        bool `json:"hash_zip"`
	HashCountry    bool `json:"hash_country"`

	// Gửi PII từ server (server-side) — Meta yêu cầu bật Advanced Matching.
	SendUserData bool `json:"send_user_data"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// MaskAccessToken che token khi trả về cho client (admin UI).
func (c *FacebookCAPIConfig) MaskAccessToken() {
	if len(c.AccessToken) > 8 {
		c.AccessToken = c.AccessToken[:4] + "****" + c.AccessToken[len(c.AccessToken)-4:]
	} else if c.AccessToken != "" {
		c.AccessToken = "****"
	}
	if len(c.GatewayAccessKey) > 6 {
		c.GatewayAccessKey = c.GatewayAccessKey[:2] + "****" + c.GatewayAccessKey[len(c.GatewayAccessKey)-2:]
	} else if c.GatewayAccessKey != "" {
		c.GatewayAccessKey = "****"
	}
}

// FacebookCAPIEvent là 1 lần gửi sự kiện lên Meta (log giao dịch).
type FacebookCAPIEvent struct {
	ID           uint64          `json:"id"`
	LeadID       uint64          `json:"lead_id"`
	LandingID    string          `json:"landing_id"`
	EventName    string          `json:"event_name"`
	EventID      string          `json:"event_id"`
	ActionSource string          `json:"action_source"`
	PixelID      string          `json:"pixel_id"`
	Status       string          `json:"status"` // success | failed | skipped
	HTTPStatus   int             `json:"http_status"`
	Response     json.RawMessage `json:"response"`
	Error        string          `json:"error"`
	TriggeredBy  string          `json:"triggered_by"` // lead_submit | crm_status_change | page_view | manual_test
	TrangThaiCRM string          `json:"trang_thai_crm"`
	// SessionID dùng để dedup PageView server-side: chỉ gửi 1 PageView
	// cho mỗi session (tracker.js gửi nhiều ping /api/analytics/track).
	SessionID string `json:"session_id,omitempty"`
	CreatedAt string `json:"created_at"`
}

// FacebookCAPIConfigInput là payload cập nhật cấu hình từ Admin UI.
type FacebookCAPIConfigInput struct {
	LandingID string `json:"landing_id"`
	IsActive  *bool  `json:"is_active"`

	PixelID     string `json:"pixel_id"`
	AccessToken string `json:"access_token"`
	APIVersion  string `json:"api_version"`

	GatewayEnabled   bool   `json:"gateway_enabled"`
	GatewayEndpoint  string `json:"gateway_endpoint"`
	GatewayAccessKey string `json:"gateway_access_key"`

	SendPageView bool `json:"send_page_view"`
	SendLead     bool `json:"send_lead"`
	SendPurchase bool `json:"send_purchase"`
	SendCRMSync  bool `json:"send_crm_sync"`

	CourseValue float64 `json:"course_value"`
	Currency    string  `json:"currency"`

	HashEmail      bool `json:"hash_email"`
	HashPhone      bool `json:"hash_phone"`
	HashName       bool `json:"hash_name"`
	HashAddress    bool `json:"hash_address"`
	HashExternalID bool `json:"hash_external_id"`
	HashCity       bool `json:"hash_city"`
	HashState      bool `json:"hash_state"`
	HashZip        bool `json:"hash_zip"`
	HashCountry    bool `json:"hash_country"`

	SendUserData bool `json:"send_user_data"`
}

// FacebookDatasetQuality là kết quả đọc từ Meta Dataset Quality API.
type FacebookDatasetQuality struct {
	PixelID           string                 `json:"pixel_id"`
	FetchedAt         string                 `json:"fetched_at"`
	EventMatchQuality map[string]interface{} `json:"event_match_quality"`
	EventCoverage     map[string]interface{} `json:"event_coverage"`
	DedupeKeyFeedback map[string]interface{} `json:"dedupe_key_feedback"`
	DataFreshness     map[string]interface{} `json:"data_freshness"`
	ValueDiagnostics  map[string]interface{} `json:"value_diagnostics"`
	EventDelay        map[string]interface{} `json:"event_delay"`
	EventSpend        map[string]interface{} `json:"event_spend"`
	EventAdSets       map[string]interface{} `json:"event_ad_sets"`
	Error             string                 `json:"error,omitempty"`
}

// FacebookCAPIStats là số liệu tổng hợp từ DB (khi Graph API không truy cập được).
type FacebookCAPIStats struct {
	TotalSent    int                 `json:"total_sent"`
	TotalSuccess int                 `json:"total_success"`
	TotalFailed  int                 `json:"total_failed"`
	TotalSkipped int                 `json:"total_skipped"`
	ByEvent      map[string]int      `json:"by_event"`
	ByStatus     map[string]int      `json:"by_status"`
	LastError    string              `json:"last_error,omitempty"`
	LastErrorAt  string              `json:"last_error_at,omitempty"`
	RecentEvents []FacebookCAPIEvent `json:"recent_events"`
}

// FacebookCAPIEventRequest là payload test event thủ công từ Admin UI.
type FacebookCAPIEventRequest struct {
	LandingID      string                 `json:"landing_id"`
	EventName      string                 `json:"event_name"`
	EventID        string                 `json:"event_id"`
	Email          string                 `json:"email"`
	Phone          string                 `json:"phone"`
	FirstName      string                 `json:"first_name"`
	LastName       string                 `json:"last_name"`
	City           string                 `json:"city"`
	State          string                 `json:"state"`
	Zip            string                 `json:"zip"`
	Country        string                 `json:"country"`
	ExternalID     string                 `json:"external_id"`
	CustomData     map[string]interface{} `json:"custom_data"`
	Value          float64                `json:"value"`
	Currency       string                 `json:"currency"`
	EventSourceURL string                 `json:"event_source_url"`
	// TestEventCode là mã test lấy từ Events Manager → Test Events.
	//
	// ⚠️ Meta CHỈ chấp nhận mã do Events Manager sinh ra. Nếu gửi mã tự
	// bịa (vd "TEST_123") ⇒ Meta trả lỗi 100 "Invalid parameter" ⇒ tính
	// năng "Test Event" không hoạt động. Vì vậy trường này BẮT BUỘC khi
	// dùng chế độ test; để trống ⇒ gửi sự kiện thật (và sẽ vào dữ liệu thật).
	TestEventCode string `json:"test_event_code"`
}

// NowString trả về thời gian hiện tại theo định dạng chuẩn của repo.
func NowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
