package googlesheets

import (
	"encoding/json"
	"fmt"
	"strings"

	"apex-backend/models"
)

// ExtractLeadValue trích xuất giá trị trường dữ liệu của Lead theo cấu hình ánh xạ.
//
// 📌 2026-11-15: mở rộng theo dữ liệu mới của CRM + Meta CAPI:
//   - Địa lý đầy đủ (tỉnh gốc `region_name`, ISO-2 `country_code`, ISP)
//   - Pipeline CRM 10 trạng thái (nhãn, giai đoạn, sự kiện Meta tương ứng)
//   - Thiết bị chi tiết (model, engine, OS, màn hình, touchpoints)
//   - Hành vi (click, mốc cuộn, video/Zalo/call, khách mới hay quay lại)
//   - Form & autofill (nguồn autofill, validate, thời gian nhìn form)
//   - VPN / Proxy / Tor / Bot
//
// Danh sách key hợp lệ: `AvailableLeadFields()` (fields.go).
func ExtractLeadValue(fieldName string, customText string, lead *models.Lead, trackingData map[string]interface{}) interface{} {
	if trackingData == nil {
		trackingData = make(map[string]interface{})
	}

	switch fieldName {
	// ─── Thông tin lead ─────────────────────────────────────────────
	case "thoi_gian", "created_at":
		if lead != nil && lead.CreatedAt != "" {
			return lead.CreatedAt
		}
		if v, ok := trackingData["thoi_gian"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "cap_nhat_luc", "updated_at":
		if lead != nil && lead.UpdatedAt != "" {
			return lead.UpdatedAt
		}
		return trackingString(trackingData, "cap_nhat_luc", "updated_at")

	case "fullName", "ho_ten", "name":
		for _, k := range []string{"fullName", "ho_ten", "name", "customer_name", "hoTen"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "so_dien_thoai", "phone", "sdt":
		if lead != nil && lead.Phone != "" {
			return lead.Phone
		}
		for _, k := range []string{"so_dien_thoai", "phone", "phoneNumber", "sdt"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "carrier", "nha_mang":
		return trackingString(trackingData, "carrier", "nha_mang")

	case "landing_id":
		if lead != nil && lead.LandingID != "" {
			return lead.LandingID
		}
		return trackingString(trackingData, "landing_id")

	case "nguon":
		if lead != nil && lead.Nguon != "" {
			return lead.Nguon
		}
		return trackingString(trackingData, "nguon")

	case "ghi_chu", "note":
		if lead != nil && lead.GhiChu != "" {
			return lead.GhiChu
		}
		return ""

	case "nguoi_phu_trach", "assigned_to":
		if lead != nil && lead.AssignedTo != "" {
			return lead.AssignedTo
		}
		return trackingString(trackingData, "nguoi_phu_trach", "assigned_to")

	case "lead_id", "id":
		if lead != nil && lead.ID > 0 {
			return fmt.Sprintf("%d", lead.ID)
		}
		return trackingString(trackingData, "lead_id")

	// ─── CRM Pipeline ───────────────────────────────────────────────
	case "trang_thai", "status":
		if lead != nil && lead.TrangThai != "" {
			return lead.TrangThai
		}
		return models.StatusMoi

	case "trang_thai_label", "status_label":
		key := models.StatusMoi
		if lead != nil && lead.TrangThai != "" {
			key = lead.TrangThai
		}
		if d, ok := models.GetLeadStatus(key); ok {
			return d.Label
		}
		return ""

	case "trang_thai_stage", "status_stage":
		key := models.StatusMoi
		if lead != nil && lead.TrangThai != "" {
			key = lead.TrangThai
		}
		if d, ok := models.GetLeadStatus(key); ok {
			return d.Stage
		}
		return ""

	case "capi_event":
		key := models.StatusMoi
		if lead != nil && lead.TrangThai != "" {
			key = lead.TrangThai
		}
		if eventName, _, ok := models.LeadStatusCAPIEvent(key); ok {
			return eventName
		}
		return ""

	case "capi_has_value":
		key := models.StatusMoi
		if lead != nil && lead.TrangThai != "" {
			key = lead.TrangThai
		}
		_, hasValue, ok := models.LeadStatusCAPIEvent(key)
		if !ok {
			return ""
		}
		if hasValue {
			return "Có"
		}
		return "Không"

	// ─── Địa lý & nhà mạng ──────────────────────────────────────────
	case "city":
		if lead != nil && lead.City != "" {
			return lead.City
		}
		return trackingString(trackingData, "city")

	case "region_name":
		// Tỉnh gốc từ GeoIP. Lead cũ (trước 2026-11-15) chưa lưu `region_name`
		// ⇒ fallback về `city` (server đã chuẩn hoá region_name = city khi GeoIP
		// không trả vùng).
		if v := trackingString(trackingData, "region_name"); v != "" {
			return v
		}
		if lead != nil && lead.City != "" {
			return lead.City
		}
		return ""

	case "region":
		if lead != nil && lead.Region != "" {
			return lead.Region
		}
		return trackingString(trackingData, "region")

	case "country":
		if lead != nil && lead.Country != "" {
			return lead.Country
		}
		return trackingString(trackingData, "country")

	case "country_code":
		if v := trackingString(trackingData, "country_code"); v != "" {
			return v
		}
		// Fallback: suy ra ISO-2 từ tên quốc gia (lead cũ chưa có country_code).
		if lead != nil && lead.Country != "" {
			if code := normalizeCountryISO(lead.Country); code != "" {
				return code
			}
		}
		return ""

	case "isp":
		if lead != nil && lead.ISP != "" {
			return lead.ISP
		}
		return trackingString(trackingData, "isp")

	// ─── Nguồn quảng cáo ────────────────────────────────────────────
	case "ad_platform":
		if lead != nil && lead.AdPlatform != "" {
			return lead.AdPlatform
		}
		return trackingString(trackingData, "ad_platform")

	case "ad_placement":
		if lead != nil && lead.AdPlacement != "" {
			return lead.AdPlacement
		}
		return trackingString(trackingData, "ad_placement")

	case "ad_source_tag":
		if v := trackingString(trackingData, "ad_source_tag"); v != "" {
			return v
		}
		if lead != nil && lead.AdPlatform != "" {
			return lead.AdPlatform
		}
		return ""

	case "utm_source":
		return trackingString(trackingData, "utm_source")

	case "utm_medium":
		return trackingString(trackingData, "utm_medium")

	case "utm_campaign":
		return trackingString(trackingData, "utm_campaign")

	case "utm_content":
		return trackingString(trackingData, "utm_content")

	case "utm_term":
		return trackingString(trackingData, "utm_term")

	// ─── Meta Ads macros ────────────────────────────────────────────
	//
	// Chuỗi UTM khuyến nghị cho Meta Ads Manager:
	//   utm_medium=paid&utm_source={{site_source_name}}
	//   &utm_placement={{placement}}&utm_campaign={{campaign.id}}
	//   &utm_content={{adset.id}}&utm_term={{ad.id}}
	//   &campaign_name={{campaign.name}}&adset_name={{adset.name}}&ad_name={{ad.name}}
	case "utm_placement":
		return trackingString(trackingData, "utm_placement", "placement")

	case "campaign_name":
		return trackingString(trackingData, "campaign_name")

	case "adset_name":
		return trackingString(trackingData, "adset_name")

	case "ad_name":
		return trackingString(trackingData, "ad_name")

	case "ad_account_id", "acc_id":
		return trackingString(trackingData, "ad_account_id", "acc_id", "ad_account")

	case "site_source_name":
		return trackingString(trackingData, "site_source_name")

	// ─── Định danh lượt nhấp Meta (CAPI) ────────────────────────────
	case "fbclid":
		return trackingString(trackingData, "fbclid")

	case "fbc":
		return trackingString(trackingData, "fbc")

	case "fbp":
		return trackingString(trackingData, "fbp")

	case "igshid":
		return trackingString(trackingData, "igshid")

	case "meta_click_id":
		return trackingString(trackingData, "meta_click_id", "fbclid", "igshid")

	case "event_id":
		return trackingString(trackingData, "event_id")

	// ─── Thiết bị & trình duyệt ─────────────────────────────────────
	case "device_type":
		if v := trackingString(trackingData, "device_type", "deviceType", "device"); v != "" {
			return v
		}
		return "desktop"

	case "device_brand":
		return trackingString(trackingData, "device_brand")

	case "device_name":
		return trackingString(trackingData, "device_name")

	case "device_model":
		return trackingString(trackingData, "device_model")

	case "device_model_code":
		return trackingString(trackingData, "device_model_code")

	case "browser":
		return trackingString(trackingData, "browser")

	case "browser_version":
		return trackingString(trackingData, "browser_version")

	case "browser_engine":
		return trackingString(trackingData, "browser_engine")

	case "os":
		return trackingString(trackingData, "os")

	case "os_version":
		return trackingString(trackingData, "os_version")

	case "screen_res":
		return trackingString(trackingData, "screen_res")

	case "viewport_res":
		return trackingString(trackingData, "viewport_res")

	case "screen_color_depth":
		return trackingString(trackingData, "screen_color_depth")

	case "pixel_ratio":
		return trackingString(trackingData, "pixel_ratio")

	case "device_memory":
		return trackingString(trackingData, "device_memory")

	case "device_cores":
		return trackingString(trackingData, "device_cores")

	case "device_touchpoints":
		return trackingString(trackingData, "device_touchpoints")

	case "timezone":
		return trackingString(trackingData, "timezone")

	case "language":
		return trackingString(trackingData, "language")

	case "platform_info":
		return trackingString(trackingData, "platform_info")

	case "connection_type":
		return trackingString(trackingData, "connection_type")

	case "device_state":
		return trackingString(trackingData, "device_state")

	case "is_in_app_browser":
		return boolCell(trackingData, "is_in_app_browser", "Có (In-App)", "Không (trình duyệt ngoài)")

	case "in_app_type":
		return trackingString(trackingData, "in_app_type")

	// ─── Hành vi ─────────────────────────────────────────────────────
	case "is_autofill":
		return boolCell(trackingData, "is_autofill", "Có (1-Tap)", "Không")

	case "fill_duration_ms":
		if v, ok := trackingData["fill_duration_ms"]; ok && v != nil {
			return fmt.Sprintf("%v ms", v)
		}
		return ""

	case "time_on_page":
		for _, k := range []string{"time_on_page", "time_on_page_sec", "timeOnPage"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v giây", v)
			}
		}
		return ""

	case "max_scroll":
		for _, k := range []string{"max_scroll", "max_scroll_percent", "maxScroll"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v%%", v)
			}
		}
		return ""

	case "max_scroll_percent":
		for _, k := range []string{"max_scroll", "max_scroll_percent"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v%%", v)
			}
		}
		return ""

	case "clicks_count":
		return trackingString(trackingData, "clicks_count")

	case "visible_seconds":
		return trackingString(trackingData, "visible_seconds")

	case "hidden_seconds":
		return trackingString(trackingData, "hidden_seconds")

	case "session_count":
		return trackingString(trackingData, "session_count")

	case "session_id":
		return trackingString(trackingData, "session_id", "sessionId", "apex_session_id")

	case "visitor_id":
		return trackingString(trackingData, "visitor_id", "visitorId")

	case "is_new_visitor":
		return boolCell(trackingData, "is_new_visitor", "Khách Mới", "Khách Quay Lại")

	case "scroll_depth_25", "scroll_depth_50", "scroll_depth_75", "scroll_depth_90", "scroll_depth_100":
		return boolCell(trackingData, fieldName, "Có", "Không")

	case "has_video_play":
		return boolCell(trackingData, "has_video_play", "Có", "Không")

	case "has_zalo_click":
		return boolCell(trackingData, "has_zalo_click", "Có", "Không")

	case "has_tel_click":
		return boolCell(trackingData, "has_tel_click", "Có", "Không")

	case "referrer", "referer":
		return trackingString(trackingData, "referrer", "server_referer")

	case "referrer_host":
		return trackingString(trackingData, "referrer_host")

	case "landing_path":
		return trackingString(trackingData, "landing_path")

	case "entry_url":
		return trackingString(trackingData, "entry_url")

	case "url", "page_url", "landing_url":
		// URL trang khách vừa gửi form — trường quan trọng để truy ngược lead về đúng landing page
		if lead != nil && lead.URL != "" {
			return lead.URL
		}
		for _, k := range []string{"url", "page_url", "entry_url", "landing_url"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "ip_address", "ip":
		return trackingString(trackingData, "server_ip", "ip_address", "client_ip", "ip")

	case "server_received_at":
		return trackingString(trackingData, "server_received_at")

	// ─── Form & Autofill ────────────────────────────────────────────
	case "autofill_source":
		return trackingString(trackingData, "autofill_source")

	case "validation_passed":
		return boolCell(trackingData, "validation_passed", "Có", "Không")

	case "form_focus_ms":
		return trackingString(trackingData, "form_focus_ms")

	// ─── VPN / Proxy / Tor ──────────────────────────────────────────
	case "is_vpn":
		return boolCell(trackingData, "is_vpn", "Có", "Không")

	case "is_proxy":
		return boolCell(trackingData, "is_proxy", "Có", "Không")

	case "is_tor":
		return boolCell(trackingData, "is_tor", "Có", "Không")

	case "vpn_provider":
		return trackingString(trackingData, "vpn_provider")

	case "vpn_confidence":
		return trackingString(trackingData, "vpn_confidence")

	case "vpn_reason":
		return trackingString(trackingData, "vpn_reason")

	// ─── Bot ────────────────────────────────────────────────────────
	case "is_bot":
		return boolCell(trackingData, "is_bot", "Nghi Bot", "Người Thật")

	case "bot_score":
		return trackingString(trackingData, "bot_score")

	case "bot_reasons":
		return trackingString(trackingData, "bot_reasons")

	// ─── Khác ───────────────────────────────────────────────────────
	case "tracking_json":
		bytes, err := json.Marshal(trackingData)
		if err == nil {
			return string(bytes)
		}
		return "{}"

	case "custom_text":
		return customText

	default:
		// Kiểm tra trực tiếp trong trackingData
		if v, ok := trackingData[fieldName]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
}

// trackingString lấy giá trị chuỗi đầu tiên tồn tại trong trackingData.
func trackingString(trackingData map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := trackingData[k]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "0" {
				return s
			}
		}
	}
	return ""
}

// boolCell chuyển 1 cờ boolean trong tracking thành chuỗi hiển thị.
//
// ⚠️ `trueLabel`/`falseLabel` cho phép đổi nghĩa theo ngữ cảnh
// (vd: `is_new_visitor` → "Khách Mới" / "Khách Quay Lại").
//
// Nếu KHÔNG có key trong tracking ⇒ trả `falseLabel` (mặc định an toàn:
// không khẳng định client có hành vi đó).
func boolCell(trackingData map[string]interface{}, key string, trueLabel string, falseLabel string) string {
	v, ok := trackingData[key]
	if !ok || v == nil {
		return falseLabel
	}
	switch b := v.(type) {
	case bool:
		if b {
			return trueLabel
		}
		return falseLabel
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "1", "yes", "y", "có":
			return trueLabel
		case "false", "0", "no", "n", "không":
			return falseLabel
		}
	case float64:
		if b != 0 {
			return trueLabel
		}
		return falseLabel
	case int:
		if b != 0 {
			return trueLabel
		}
		return falseLabel
	}
	return falseLabel
}

// normalizeCountryISO chuyển tên quốc gia tiếng Việt sang mã ISO-2.
//
// Dùng làm fallback cho lead cũ (trước 2026-11-15 chưa lưu `country_code`).
func normalizeCountryISO(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "việt nam", "vietnam", "vn":
		return "VN"
	case "hoa kỳ", "united states", "united states of america", "usa", "us":
		return "US"
	case "trung quốc", "china", "cn":
		return "CN"
	case "hàn quốc", "south korea", "korea", "kr":
		return "KR"
	case "nhật bản", "japan", "jp":
		return "JP"
	case "thailand", "thái lan", "th":
		return "TH"
	case "singapore", "sg":
		return "SG"
	case "cambodia", "campuchia", "kh":
		return "KH"
	case "myanmar", "lào", "laos", "la":
		return "LA"
	}
	return ""
}

// BuildRowValues tạo mảng giá trị cho các cột theo thứ tự của FieldMappings
func BuildRowValues(mappings []models.FieldMapping, lead *models.Lead, trackingData map[string]interface{}) []interface{} {
	row := make([]interface{}, len(mappings))
	for i, m := range mappings {
		val := ExtractLeadValue(m.Field, m.CustomText, lead, trackingData)
		row[i] = val
	}
	return row
}
