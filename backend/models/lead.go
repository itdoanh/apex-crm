package models

import (
	"encoding/json"
)

// Lead đại diện cho 1 bản ghi lead trong CSDL PostgreSQL
// Lưu trữ: id, thời gian, landing_id, nguồn, số điện thoại, trạng thái CRM, ghi chú, tracking (JSONB)
type Lead struct {
	ID          uint64          `json:"id"`
	CreatedAt   string          `json:"thoi_gian"`       // Thời gian theo múi giờ VN (YYYY-MM-DD HH:mm:ss)
	LandingID   string          `json:"landing_id"`      // Định danh Landing Page (ví dụ: mxv_080, lop_hoc_vip,...)
	Nguon       string          `json:"nguon"`           // Nguồn traffic cấu hình theo từng Landing (facebook, tiktok, google,...)
	Phone       string          `json:"so_dien_thoai"`   // Số điện thoại khách hàng đã chuẩn hóa
	TrangThai   string          `json:"trang_thai"`      // Trạng thái CRM: moi, da_goi, ket_ban_zalo, da_vao_zoom, tiem_nang, chot, huy
	GhiChu      string          `json:"ghi_chu"`         // Ghi chú của sales/telesale
	AssignedTo  string          `json:"nguoi_phu_trach"` // Nhân viên phụ trách
	UpdatedAt   string          `json:"cap_nhat_luc"`    // Thời gian cập nhật gần nhất
	City        string          `json:"city"`            // Tỉnh/Thành phố xác thực theo IP thực tế
	Region      string          `json:"region"`          // Vùng miền: Miền Bắc, Miền Trung, Miền Nam, Quốc Tế
	Country     string          `json:"country"`         // Quốc gia
	ISP         string          `json:"isp"`             // Nhà mạng viễn thông: VNPT, Viettel, FPT, MobiFone...
	AdPlatform  string          `json:"ad_platform"`     // Meta Ads, Google Ads, TikTok Ads, Zalo Ads, Direct...
	AdPlacement string          `json:"ad_placement"`    // Vị trí phân phối: Instagram, FB Feed, Audience Network, GDN, YouTube...
	URL         string          `json:"url"`             // URL trang khách vừa gửi form (landing + query string) — dùng để truy ngược lead về đúng trang
	Tracking    json.RawMessage `json:"tracking"`        // Chứa toàn bộ thông tin trình duyệt + form dạng JSON linh hoạt
}

// LeadSubmission là payload nhận từ client landing page
type LeadSubmission struct {
	LandingID string `json:"landing_id"`
	Nguon     string `json:"nguon"`
	Phone     string `json:"so_dien_thoai"`
}

// UpdateLeadRequest là payload cập nhật trạng thái & ghi chú CRM từ Admin
type UpdateLeadRequest struct {
	ID         uint64 `json:"id"`
	TrangThai  string `json:"trang_thai"`
	GhiChu     string `json:"ghi_chu"`
	AssignedTo string `json:"nguoi_phu_trach"`
}

