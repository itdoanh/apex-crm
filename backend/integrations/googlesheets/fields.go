package googlesheets

import (
	"apex-backend/models"
)

// ─── DANH MỤC TRƯỜNG CÓ THỂ ÁNH XẠ SANG GOOGLE SHEET ──────────────────
//
// 📌 NGUỒN SỰ THẬT DUY NHẤT (2026-11-15).
//
// Trước đây danh sách trường bị lặp ở 3 nơi:
//   1. `ExtractLeadValue()` — switch case trong `sync.go`
//   2. `HandleGetDefaultMappings` — mảng `availableFields` trong handler
//   3. `frontend/admin/js/sheets.js` — mảng `availableLeadFields`
//
// ⇒ Thêm 1 trường phải sửa 3 file, dễ quên ⇒ UI hiển thị khác backend,
//   sheet ghi sai cột.
//
// ⇒ Nay gộp về 1 danh mục duy nhất trong file này. Backend trả về cho UI
// qua `GET /api/integrations/sheets/defaults` ⇒ frontend không hardcode nữa.

// LeadFieldOption mô tả 1 trường dữ liệu có thể ánh xạ sang 1 cột Sheet.
type LeadFieldOption struct {
	// Key là tên trường dùng trong `models.FieldMapping.Field`.
	Key string `json:"key"`
	// Label là nhãn hiển thị trong dropdown ánh xạ cột.
	Label string `json:"label"`
	// Header là tiêu đề cột mặc định khi dựng bộ cột chuẩn.
	// Rỗng ⇒ dùng `Label`.
	Header string `json:"header,omitempty"`
	// Group để UI gom nhóm cho dễ tìm.
	Group string `json:"group"`
}

// headerOrLabel trả tiêu đề cột (fallback sang Label).
func (o LeadFieldOption) headerOrLabel() string {
	if o.Header != "" {
		return o.Header
	}
	return o.Label
}

// Nhóm trường (dùng để UI gom nhóm).
const (
	FieldGroupLead     = "Thông tin lead"
	FieldGroupCRM      = "CRM Pipeline"
	FieldGroupGeo      = "Địa lý & Nhà mạng"
	FieldGroupAds      = "Nguồn quảng cáo"
	FieldGroupMetaID   = "Định danh lượt nhấp Meta"
	FieldGroupDevice   = "Thiết bị & Trình duyệt"
	FieldGroupBehavior = "Hành vi người dùng"
	FieldGroupForm     = "Form & Autofill"
	FieldGroupSecurity = "VPN / Proxy / Bot"
	FieldGroupOther    = "Khác"
)

// leadFieldCatalog là danh mục TRƯỜNG ĐẦY ĐỦ (nguồn sự thật duy nhất).
//
// ⚠️ Thứ tự phải giữ ổn định — `GetStandardDefaultMappings()` sinh cột A, B, C…
// theo đúng thứ tự này.
var leadFieldCatalog = []LeadFieldOption{
	// ─── Thông tin lead ─────────────────────────────────────────────
	{Key: "thoi_gian", Label: "Thời Gian Đăng Ký", Header: "Thời Gian", Group: FieldGroupLead},
	{Key: "cap_nhat_luc", Label: "Thời Gian Cập Nhật CRM", Header: "Cập Nhật Lúc", Group: FieldGroupLead},
	{Key: "fullName", Label: "Họ Và Tên", Group: FieldGroupLead},
	{Key: "so_dien_thoai", Label: "Số Điện Thoại", Group: FieldGroupLead},
	{Key: "carrier", Label: "Nhà Mạng Viễn Thông", Group: FieldGroupLead},
	{Key: "landing_id", Label: "ID Trang Landing", Group: FieldGroupLead},
	{Key: "url", Label: "URL Trang Khách Đăng Ký", Group: FieldGroupLead},
	{Key: "nguon", Label: "Nguồn Traffic (Kênh)", Group: FieldGroupLead},
	{Key: "nguoi_phu_trach", Label: "Nhân Viên Phụ Trách", Group: FieldGroupLead},
	{Key: "ghi_chu", Label: "Ghi Chú Tư Vấn", Group: FieldGroupLead},
	{Key: "lead_id", Label: "Mã Lead (Hệ Thống)", Group: FieldGroupLead},
	// ⛔ CỜ LEAD TEST (2026-11-16): "Có" khi tên khách chứa "(test)".
	// Lead test KHÔNG tính vào biểu đồ/thông số nhưng VẪN đồng bộ sang
	// Sheet ⇒ cần cột để đối soát & loại khỏi báo cáo doanh thu.
	{Key: "is_test", Label: "Lead Test", Header: "Lead Test", Group: FieldGroupLead},

	// ─── CRM Pipeline ───────────────────────────────────────────────
	{Key: "trang_thai", Label: "Trạng Thái Lead (Mã)", Group: FieldGroupCRM},
	{Key: "trang_thai_label", Label: "Trạng Thái Lead (Nhãn)", Group: FieldGroupCRM},
	{Key: "trang_thai_stage", Label: "Giai Đoạn Pipeline", Group: FieldGroupCRM},
	{Key: "capi_event", Label: "Sự Kiện Meta Tương Ứng", Group: FieldGroupCRM},
	{Key: "capi_has_value", Label: "Sự Kiện Có Giá Trị", Group: FieldGroupCRM},

	// ─── Địa lý & nhà mạng ──────────────────────────────────────────
	{Key: "city", Label: "Thành Phố", Group: FieldGroupGeo},
	{Key: "region_name", Label: "Tỉnh / Thành (Gốc)", Header: "Tỉnh/Thành Gốc", Group: FieldGroupGeo},
	{Key: "region", Label: "Vùng Miền", Group: FieldGroupGeo},
	{Key: "country", Label: "Quốc Gia", Group: FieldGroupGeo},
	{Key: "country_code", Label: "Mã Quốc Gia (ISO-2)", Group: FieldGroupGeo},
	{Key: "isp", Label: "Nhà Mạng Internet (ISP)", Group: FieldGroupGeo},

	// ─── Nguồn quảng cáo ────────────────────────────────────────────
	{Key: "ad_platform", Label: "Nền Tảng Quảng Cáo", Group: FieldGroupAds},
	{Key: "ad_placement", Label: "Vị Trí Phân Phối", Group: FieldGroupAds},
	{Key: "ad_source_tag", Label: "Tag Nguồn Quảng Cáo", Group: FieldGroupAds},
	{Key: "utm_source", Label: "UTM Source", Group: FieldGroupAds},
	{Key: "utm_medium", Label: "UTM Medium", Group: FieldGroupAds},
	{Key: "utm_campaign", Label: "UTM Campaign", Group: FieldGroupAds},
	{Key: "utm_content", Label: "UTM Content (Adset ID)", Group: FieldGroupAds},
	{Key: "utm_term", Label: "UTM Term (Ad ID)", Group: FieldGroupAds},
	{Key: "utm_placement", Label: "Meta Placement ({{placement}})", Group: FieldGroupAds},
	{Key: "campaign_name", Label: "Meta Campaign Name", Group: FieldGroupAds},
	{Key: "adset_name", Label: "Meta Adset Name", Group: FieldGroupAds},
	{Key: "ad_name", Label: "Meta Ad Name", Group: FieldGroupAds},
	{Key: "ad_account_id", Label: "Meta Account ID", Group: FieldGroupAds},
	{Key: "site_source_name", Label: "Meta Site Source Name", Group: FieldGroupAds},

	// ─── Định danh lượt nhấp Meta (CAPI) ────────────────────────────
	{Key: "fbclid", Label: "Meta Click ID (fbclid)", Group: FieldGroupMetaID},
	{Key: "fbc", Label: "Meta Cookie _fbc", Group: FieldGroupMetaID},
	{Key: "fbp", Label: "Meta Cookie _fbp", Group: FieldGroupMetaID},
	{Key: "igshid", Label: "Meta igshid", Group: FieldGroupMetaID},
	{Key: "meta_click_id", Label: "Meta Click ID Tổng", Group: FieldGroupMetaID},
	{Key: "event_id", Label: "Event ID (Dedupe Meta)", Group: FieldGroupMetaID},

	// ─── Thiết bị & trình duyệt ─────────────────────────────────────
	{Key: "device_type", Label: "Thiết Bị (Mobile/Desktop)", Group: FieldGroupDevice},
	{Key: "device_brand", Label: "Hãng Thiết Bị", Group: FieldGroupDevice},
	{Key: "device_name", Label: "Tên Thiết Bị (Model Thật)", Group: FieldGroupDevice},
	{Key: "device_model", Label: "Model Thiết Bị (Thô)", Group: FieldGroupDevice},
	{Key: "device_model_code", Label: "Mã Model Thiết Bị", Group: FieldGroupDevice},
	{Key: "browser", Label: "Trình Duyệt", Group: FieldGroupDevice},
	{Key: "browser_version", Label: "Phiên Bản Trình Duyệt", Group: FieldGroupDevice},
	{Key: "browser_engine", Label: "Engine Trình Duyệt", Group: FieldGroupDevice},
	{Key: "os", Label: "Hệ Điều Hành", Group: FieldGroupDevice},
	{Key: "os_version", Label: "Phiên Bản Hệ Điều Hành", Group: FieldGroupDevice},
	{Key: "screen_res", Label: "Độ Phân Giải Màn Hình", Group: FieldGroupDevice},
	{Key: "viewport_res", Label: "Kích Thước Viewport", Group: FieldGroupDevice},
	{Key: "screen_color_depth", Label: "Độ Sâu Màu Màn Hình", Group: FieldGroupDevice},
	{Key: "pixel_ratio", Label: "Pixel Ratio", Group: FieldGroupDevice},
	{Key: "device_memory", Label: "RAM (GB)", Group: FieldGroupDevice},
	{Key: "device_cores", Label: "Số Lõi CPU", Group: FieldGroupDevice},
	{Key: "device_touchpoints", Label: "Số Điểm Chạm", Group: FieldGroupDevice},
	{Key: "timezone", Label: "Múi Giờ", Group: FieldGroupDevice},
	{Key: "language", Label: "Ngôn Ngữ", Group: FieldGroupDevice},
	{Key: "platform_info", Label: "Thông Tin Nền Tảng", Group: FieldGroupDevice},
	{Key: "connection_type", Label: "Loại Kết Nối (4g/5g)", Group: FieldGroupDevice},
	{Key: "device_state", Label: "Trạng Thái Thiết Bị", Group: FieldGroupDevice},
	{Key: "is_in_app_browser", Label: "Trình Duyệt Trong App", Group: FieldGroupDevice},
	{Key: "in_app_type", Label: "Loại App (FB/IG/Zalo/TikTok)", Group: FieldGroupDevice},

	// ─── Hành vi người dùng ─────────────────────────────────────────
	{Key: "time_on_page", Label: "Thời Gian Xem Trang", Group: FieldGroupBehavior},
	{Key: "max_scroll", Label: "Tỷ Lệ Cuộn Trang (%)", Group: FieldGroupBehavior},
	{Key: "max_scroll_percent", Label: "Cuộn Sâu Nhất (%)", Group: FieldGroupBehavior},
	{Key: "clicks_count", Label: "Số Lần Click", Group: FieldGroupBehavior},
	{Key: "visible_seconds", Label: "Thời Gian Hiển Thị Trang", Group: FieldGroupBehavior},
	{Key: "hidden_seconds", Label: "Thời Gian Ẩn Trang", Group: FieldGroupBehavior},
	{Key: "session_count", Label: "Số Phiên Truy Cập", Group: FieldGroupBehavior},
	{Key: "session_id", Label: "Mã Phiên (Session ID)", Group: FieldGroupBehavior},
	{Key: "visitor_id", Label: "Mã Khách (Visitor ID)", Group: FieldGroupBehavior},
	{Key: "is_new_visitor", Label: "Khách Mới / Quay Lại", Group: FieldGroupBehavior},
	{Key: "scroll_depth_25", Label: "Đã Cuộn Tới 25%", Group: FieldGroupBehavior},
	{Key: "scroll_depth_50", Label: "Đã Cuộn Tới 50%", Group: FieldGroupBehavior},
	{Key: "scroll_depth_75", Label: "Đã Cuộn Tới 75%", Group: FieldGroupBehavior},
	{Key: "scroll_depth_90", Label: "Đã Cuộn Tới 90%", Group: FieldGroupBehavior},
	{Key: "scroll_depth_100", Label: "Đã Cuộn Tới 100%", Group: FieldGroupBehavior},
	{Key: "has_video_play", Label: "Có Phát Video", Group: FieldGroupBehavior},
	{Key: "has_zalo_click", Label: "Có Bấm Zalo", Group: FieldGroupBehavior},
	{Key: "has_tel_click", Label: "Có Bấm Gọi Điện", Group: FieldGroupBehavior},
	{Key: "referrer", Label: "Referrer (Trang Trước)", Group: FieldGroupBehavior},
	{Key: "referrer_host", Label: "Host Trang Trước", Group: FieldGroupBehavior},
	{Key: "landing_path", Label: "Đường Dẫn Trang", Group: FieldGroupBehavior},
	{Key: "entry_url", Label: "URL Vào Landing (Kèm UTM)", Group: FieldGroupBehavior},

	// ─── Form & Autofill ────────────────────────────────────────────
	{Key: "is_autofill", Label: "Tự Động Điền (1-Tap)", Group: FieldGroupForm},
	{Key: "autofill_source", Label: "Nguồn Tự Động Điền", Group: FieldGroupForm},
	{Key: "fill_duration_ms", Label: "Tốc Độ Điền Form (ms)", Group: FieldGroupForm},
	{Key: "validation_passed", Label: "Đã Vượt Validate Form", Group: FieldGroupForm},
	{Key: "form_focus_ms", Label: "Thời Gian Nhìn Form (ms)", Group: FieldGroupForm},

	// ─── VPN / Proxy / Bot ──────────────────────────────────────────
	{Key: "is_vpn", Label: "Dùng VPN", Group: FieldGroupSecurity},
	{Key: "is_proxy", Label: "Dùng Proxy", Group: FieldGroupSecurity},
	{Key: "is_tor", Label: "Dùng Tor", Group: FieldGroupSecurity},
	{Key: "vpn_provider", Label: "Nhà Cung Cấp VPN", Group: FieldGroupSecurity},
	{Key: "vpn_confidence", Label: "Độ Tin Cậy VPN", Group: FieldGroupSecurity},
	{Key: "vpn_reason", Label: "Lý Do Phát Hiện VPN", Group: FieldGroupSecurity},
	{Key: "is_bot", Label: "Nghi Bot", Group: FieldGroupSecurity},
	{Key: "bot_score", Label: "Điểm Nghi Bot", Group: FieldGroupSecurity},
	{Key: "bot_reasons", Label: "Lý Do Nghi Bot", Group: FieldGroupSecurity},

	// ─── Khác ───────────────────────────────────────────────────────
	{Key: "ip_address", Label: "Địa Chỉ IP", Group: FieldGroupOther},
	{Key: "server_received_at", Label: "Thời Gian Server Nhận", Group: FieldGroupOther},
	{Key: "tracking_json", Label: "Toàn Bộ JSON Tracking", Group: FieldGroupOther},
	{Key: "custom_text", Label: "Văn Bản Cố Định", Group: FieldGroupOther},
}

// AvailableLeadFields trả về toàn bộ danh mục trường cho UI ánh xạ cột.
//
// ⚠️ Hàm trả về BẢN SAO — caller không được sửa danh mục gốc.
func AvailableLeadFields() []LeadFieldOption {
	out := make([]LeadFieldOption, len(leadFieldCatalog))
	copy(out, leadFieldCatalog)
	return out
}

// AvailableLeadFieldKeys trả về danh sách key hợp lệ (dùng để validate).
func AvailableLeadFieldKeys() map[string]bool {
	m := make(map[string]bool, len(leadFieldCatalog))
	for _, o := range leadFieldCatalog {
		m[o.Key] = true
	}
	return m
}

// columnLetter trả ký hiệu cột Sheet (0 → A, 25 → Z, 26 → AA, 27 → AB…).
func columnLetter(index int) string {
	if index < 0 {
		return ""
	}
	letters := ""
	for index >= 0 {
		letters = string(rune('A'+(index%26))) + letters
		index = index/26 - 1
	}
	return letters
}

// GetStandardDefaultMappings trả về cấu hình ánh xạ cột tiêu chuẩn mặc định.
//
// ⚠️ 2026-11-15: bộ cột mặc định giờ bao phủ TOÀN BỘ danh mục trường
// (trước đây chỉ 45 cột, thiếu địa lý / CRM / hành vi / bảo mật).
//
// ⇒ Admin bấm "Đặt Lại Chuẩn Mới" trong UI là có ngay đầy đủ cột mới.
//
// Lưu ý: cấu hình đã lưu trong DB KHÔNG bị đổi ⇒ sheet cũ giữ nguyên
// mapping cũ cho tới khi Admin bấm đặt lại (tránh phá dữ liệu đang ghi).
func GetStandardDefaultMappings() []models.FieldMapping {
	out := make([]models.FieldMapping, 0, len(leadFieldCatalog))
	for i, o := range leadFieldCatalog {
		out = append(out, models.FieldMapping{
			Column:     columnLetter(i),
			Header:     o.headerOrLabel(),
			Field:      o.Key,
			CustomText: "",
		})
	}
	return out
}

// FieldLabel trả nhãn hiển thị của 1 key trường (dùng cho header tự động).
func FieldLabel(key string) string {
	for _, o := range leadFieldCatalog {
		if o.Key == key {
			return o.Label
		}
	}
	return key
}

// FieldGroup trả nhóm của 1 key trường (rỗng nếu không xác định).
func FieldGroup(key string) string {
	for _, o := range leadFieldCatalog {
		if o.Key == key {
			return o.Group
		}
	}
	return ""
}

// MergeStandardMappings bổ sung các trường chuẩn CÒN THIẾU vào cuối danh
// sách ánh xạ hiện có.
//
// 📌 Dùng khi cấu hình Sheet đã lưu trong DB được tạo từ bộ cột cũ (15/45
// cột) ⇒ cần thêm cột mới (địa lý, CRM, thiết bị…) mà KHÔNG được phá
// cột người dùng đã tự chỉnh.
//
// Nguyên tắc:
//   - Giữ nguyên thứ tự + tiêu đề + mapping của các cột ĐANG CÓ.
//   - Các trường chuẩn chưa có ⇒ thêm vào CUỐI, đánh số cột liên tiếp.
//   - Không xoá cột nào (kể cả cột không còn trong danh mục).
//
// Trả về (mappings mới, danh sách header của các cột MỚI thêm).
func MergeStandardMappings(existing []models.FieldMapping) ([]models.FieldMapping, []string) {
	merged := make([]models.FieldMapping, len(existing))
	copy(merged, existing)

	đãCó := make(map[string]bool, len(merged))
	for _, m := range merged {
		đãCó[m.Field] = true
	}

	newHeaders := make([]string, 0)
	for _, o := range leadFieldCatalog {
		if đãCó[o.Key] {
			continue
		}
		merged = append(merged, models.FieldMapping{
			Column:     columnLetter(len(merged)),
			Header:     o.headerOrLabel(),
			Field:      o.Key,
			CustomText: "",
		})
		newHeaders = append(newHeaders, o.headerOrLabel())
	}

	return merged, newHeaders
}
