package models

// FieldMapping đại diện cho cấu hình ánh xạ giữa 1 cột trên Google Sheet với trường dữ liệu của Lead
type FieldMapping struct {
	Column     string `json:"column"`      // Ký hiệu cột: A, B, C... hoặc tên tiêu đề
	Header     string `json:"header"`      // Tên tiêu đề trên Google Sheet (vd: "Họ và tên", "Số điện thoại")
	Field      string `json:"field"`       // Tên trường Lead: thoi_gian, fullName, so_dien_thoai, carrier, landing_id, url, nguon, trang_thai, ghi_chu, is_autofill, fill_duration_ms, time_on_page, max_scroll, device_type, ip_address, utm_source, utm_campaign, utm_content, session_id, referrer, tracking_json, custom_text
	CustomText string `json:"custom_text"` // Giá trị văn bản cố định (nếu field == 'custom_text')
}

// GoogleSheetConfig đại diện cho 1 cấu hình kết nối tới Google Sheet (hỗ trợ nhiều sheet cùng lúc)
type GoogleSheetConfig struct {
	ID               int            `json:"id"`
	Name             string         `json:"name"`               // Tên gợi nhớ kết nối (vd: "Sheet Team Hà Nội", "Sheet Tổng MKT")
	AuthType         string         `json:"auth_type"`          // 'service_account' hoặc 'oauth2'
	CredentialsJSON  string         `json:"credentials_json"`   // Nội dung JSON Service Account hoặc OAuth token
	SpreadsheetID    string         `json:"spreadsheet_id"`     // ID của bảng tính Google Sheet
	SpreadsheetName  string         `json:"spreadsheet_name"`   // Tên bảng tính hiển thị
	SheetTabName     string         `json:"sheet_tab_name"`     // Tên Tab trang tính (vd: "Trang tính1", "Leads_2026")
	FieldMappings    []FieldMapping `json:"field_mappings"`     // Danh sách các cột đã cấu hình ánh xạ
	FilterLandingID  string         `json:"filter_landing_id"`  // Chỉ đồng bộ lead từ landing_id này (rỗng = tất cả)
	FilterNguon      string         `json:"filter_nguon"`       // Chỉ đồng bộ lead từ nguồn này (rỗng = tất cả)
	IsActive         bool           `json:"is_active"`          // Bật/Tắt đồng bộ
	AutoCreateHeader bool           `json:"auto_create_header"` // Tự động ghi dòng tiêu đề nếu tab còn trống
	LastSyncedAt     *string        `json:"last_synced_at"`     // Thời gian đồng bộ gần nhất
	CreatedAt        string         `json:"created_at"`         // Thời gian tạo
	UpdatedAt        string         `json:"updated_at"`         // Thời gian cập nhật
}

// GoogleSheetSyncLog lưu vết nhật ký từng lần đẩy dữ liệu Lead sang Google Sheets
type GoogleSheetSyncLog struct {
	ID           int    `json:"id"`
	ConfigID     int    `json:"config_id"`
	ConfigName   string `json:"config_name,omitempty"`
	LeadID       uint64 `json:"lead_id"`
	Status       string `json:"status"` // 'success' hoặc 'failed'
	RowRange     string `json:"row_range"`
	LeadPhone    string `json:"lead_phone"`
	LeadName     string `json:"lead_name"`
	ErrorMessage string `json:"error_message"`
	SyncedAt     string `json:"synced_at"`
}

// SheetTabInfo thông tin một Tab trong Google Sheet
type SheetTabInfo struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	RowCount    int64  `json:"row_count"`
	ColumnCount int64  `json:"column_count"`
}

// SheetMetadataResponse thông tin toàn bộ Spreadsheet
type SheetMetadataResponse struct {
	SpreadsheetID   string         `json:"spreadsheet_id"`
	Title           string         `json:"title"`
	SpreadsheetURL  string         `json:"spreadsheet_url"`
	Tabs            []SheetTabInfo `json:"tabs"`
	ServiceAccount  string         `json:"service_account,omitempty"`
}

// DriveFileInfo thông tin file Spreadsheet tìm thấy trên Google Drive
type DriveFileInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WebViewLink string `json:"web_view_link"`
}
