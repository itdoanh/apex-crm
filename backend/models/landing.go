package models

// LandingPage đại diện cho một trang Landing Page được cấu hình và quản lý từ CRM
type LandingPage struct {
	PageID       string                 `json:"page_id"`
	Domain       string                 `json:"domain"`
	PathPrefix   string                 `json:"path_prefix,omitempty"`
	Name         string                 `json:"name"`
	IsActive     bool                   `json:"is_active"`
	Status       string                 `json:"status"` // 'published', 'draft', 'archived'
	RedirectURL  string                 `json:"redirect_url"`
	NguonDefault string                 `json:"nguon_default"`
	Settings     map[string]interface{} `json:"settings"`
	CodeHTML     string                 `json:"code_html,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
	LeadsCount   int                    `json:"leads_count,omitempty"`
	ViewsCount   int                    `json:"views_count,omitempty"`

	// ⚠️ HumanViewsCount / BotViewsCount (fix 2026-11-16 — thống kê chồng chéo).
	//
	//   `ViewsCount` = TỔNG sessions (bao gồm bot). Tab "Trung Tâm Báo Cáo"
	//   dùng `human_visitors` (loại bot) làm mẫu số CR. Nếu tab Quản lý
	//   Landing dùng tổng sessions làm mẫu số ⇒ CR ở 2 tab LỆCH NHAU ⇒
	//   người dùng thấy "thông số không chính xác".
	//
	//   ⇒ Tách rõ: `ViewsCount` (tổng), `HumanViewsCount` (người thật),
	//     `BotViewsCount` (bot). CR tính theo `HumanViewsCount` để khớp
	//     với Trung Tâm Báo Cáo.
	HumanViewsCount int `json:"human_views_count,omitempty"`
	BotViewsCount   int `json:"bot_views_count,omitempty"`

	// Domains liệt kê TẤT CẢ domain mà landing này đang được deploy
	// (từ bảng `domain_routes`). Landing nhân bản rồi deploy lên nhiều
	// domain sẽ có nhiều domain — trước đây UI chỉ hiện 1 domain (hoặc
	// "Chung Domain") ⇒ người dùng tưởng thông số bị gộp chồng.
	Domains []string `json:"domains,omitempty"`
}

// LandingFile đại diện cho một tệp tin (HTML, CSS, JS) thuộc thư mục của Landing Page
type LandingFile struct {
	ID        int    `json:"id"`
	PageID    string `json:"page_id"`
	Domain    string `json:"domain"`
	FilePath  string `json:"file_path"` // vd: "index.html", "style.css", "main.js"
	FileType  string `json:"file_type"` // 'html', 'css', 'js', 'json'
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

// LandingPublicConfig là payload cấu hình trả về cho client / tracker.js
type LandingPublicConfig struct {
	LandingID         string                 `json:"landing_id"`
	Name              string                 `json:"name"`
	Domain            string                 `json:"domain"`
	Nguon             string                 `json:"nguon"`
	IsActive          bool                   `json:"is_active"`
	Status            string                 `json:"status"`
	RedirectURL       string                 `json:"redirect_url"`
	Settings          map[string]interface{} `json:"settings"`
	Title             string                 `json:"title,omitempty"`
	Hotline           string                 `json:"hotline,omitempty"`
	ZaloLink          string                 `json:"zalo_link,omitempty"`
	FacebookPixelID   string                 `json:"facebook_pixel_id,omitempty"`
	GoogleAnalyticsID string                 `json:"google_analytics_id,omitempty"`
	TikTokPixelID     string                 `json:"tiktok_pixel_id,omitempty"`
	CustomHeadScripts string                 `json:"custom_head_scripts,omitempty"`
	CustomBodyScripts string                 `json:"custom_body_scripts,omitempty"`
	ThoiGianChiaSe    map[string]interface{} `json:"thoi_gian_chia_se,omitempty"`
	KhoaHoc           map[string]interface{} `json:"khoa_hoc,omitempty"`
}
