package googlesheets

import (
	"encoding/json"
	"fmt"
	"strings"

	"apex-backend/models"
)

// ExtractLeadValue trích xuất giá trị trường dữ liệu của Lead theo cấu hình ánh xạ
func ExtractLeadValue(fieldName string, customText string, lead *models.Lead, trackingData map[string]interface{}) interface{} {
	if trackingData == nil {
		trackingData = make(map[string]interface{})
	}

	switch fieldName {
	case "thoi_gian", "created_at":
		if lead != nil && lead.CreatedAt != "" {
			return lead.CreatedAt
		}
		if t, ok := trackingData["thoi_gian"]; ok && t != nil {
			return fmt.Sprintf("%v", t)
		}
		return ""

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
		if v, ok := trackingData["carrier"]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "landing_id":
		if lead != nil && lead.LandingID != "" {
			return lead.LandingID
		}
		if v, ok := trackingData["landing_id"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "nguon":
		if lead != nil && lead.Nguon != "" {
			return lead.Nguon
		}
		if v, ok := trackingData["nguon"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "trang_thai", "status":
		if lead != nil && lead.TrangThai != "" {
			return lead.TrangThai
		}
		return "moi"

	case "ghi_chu", "note":
		if lead != nil && lead.GhiChu != "" {
			return lead.GhiChu
		}
		return ""

	case "is_autofill":
		if v, ok := trackingData["is_autofill"]; ok {
			switch b := v.(type) {
			case bool:
				if b {
					return "Có (1-Tap)"
				}
				return "Không"
			case string:
				if strings.ToLower(b) == "true" {
					return "Có (1-Tap)"
				}
			}
		}
		return "Không"

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

	case "device_type":
		for _, k := range []string{"device_type", "deviceType", "device"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v", v)
			}
		}
		return "desktop"

	case "ip_address", "ip":
		for _, k := range []string{"server_ip", "ip_address", "client_ip", "ip"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "utm_source":
		if v, ok := trackingData["utm_source"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "utm_medium":
		if v, ok := trackingData["utm_medium"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "utm_campaign":
		if v, ok := trackingData["utm_campaign"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "utm_content":
		if v, ok := trackingData["utm_content"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "utm_term":
		if v, ok := trackingData["utm_term"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "url", "page_url", "entry_url", "landing_url":
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

	case "referrer", "referer":
		for _, k := range []string{"referrer", "server_referer"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "session_id":
		for _, k := range []string{"session_id", "sessionId", "apex_session_id"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "tracking_json":
		bytes, err := json.Marshal(trackingData)
		if err == nil {
			return string(bytes)
		}
		return "{}"

	// ─── Macro Meta Ads (2026-11-12) ───────────────────────────────
	//
	// Chuỗi UTM khuyến nghị cho Meta Ads Manager:
	//   utm_medium=paid&utm_source={{site_source_name}}
	//   &utm_placement={{placement}}&utm_campaign={{campaign.id}}
	//   &utm_content={{adset.id}}&utm_term={{ad.id}}
	//   &campaign_name={{campaign.name}}&adset_name={{adset.name}}&ad_name={{ad.name}}
	case "utm_placement":
		for _, k := range []string{"utm_placement", "placement"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "campaign_name":
		if v, ok := trackingData["campaign_name"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "adset_name":
		if v, ok := trackingData["adset_name"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "ad_name":
		if v, ok := trackingData["ad_name"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "ad_account_id", "acc_id":
		for _, k := range []string{"ad_account_id", "acc_id", "ad_account"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	case "site_source_name":
		if v, ok := trackingData["site_source_name"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	// ─── Định danh lượt nhấp Meta (CAPI) ───────────────────────────
	case "fbclid":
		if v, ok := trackingData["fbclid"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "fbc":
		if v, ok := trackingData["fbc"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "fbp":
		if v, ok := trackingData["fbp"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "igshid":
		if v, ok := trackingData["igshid"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""

	case "meta_click_id":
		for _, k := range []string{"meta_click_id", "fbclid", "igshid"} {
			if v, ok := trackingData[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""

	// ─── Thiết bị & trình duyệt chi tiết ──────────────────────────
	case "device_brand":
		return trackingString(trackingData, "device_brand")

	case "device_model", "device_name":
		return trackingString(trackingData, "device_model", "device_name")

	case "device_model_code":
		return trackingString(trackingData, "device_model_code")

	case "browser":
		return trackingString(trackingData, "browser")

	case "browser_version":
		return trackingString(trackingData, "browser_version")

	case "os_version":
		return trackingString(trackingData, "os_version")

	case "is_in_app_browser":
		if v, ok := trackingData["is_in_app_browser"]; ok {
			switch b := v.(type) {
			case bool:
				if b {
					return "Có (In-App)"
				}
				return "Không (trình duyệt ngoài)"
			case string:
				if strings.ToLower(b) == "true" {
					return "Có (In-App)"
				}
			}
		}
		return "Không (trình duyệt ngoài)"

	case "in_app_type":
		return trackingString(trackingData, "in_app_type")

	case "device_memory":
		return trackingString(trackingData, "device_memory")

	case "device_cores":
		return trackingString(trackingData, "device_cores")

	case "timezone":
		return trackingString(trackingData, "timezone")

	case "connection_type":
		return trackingString(trackingData, "connection_type")

	case "device_state":
		return trackingString(trackingData, "device_state")

	case "platform_info":
		return trackingString(trackingData, "platform_info")

	// ─── VPN / Proxy ───────────────────────────────────────────────
	case "is_vpn":
		if v, ok := trackingData["is_vpn"]; ok {
			switch b := v.(type) {
			case bool:
				if b {
					return "Có"
				}
				return "Không"
			case string:
				if strings.ToLower(b) == "true" {
					return "Có"
				}
			}
		}
		return "Không"

	case "vpn_provider":
		return trackingString(trackingData, "vpn_provider")

	case "vpn_reason":
		return trackingString(trackingData, "vpn_reason")

	// ─── Hành vi ───────────────────────────────────────────────────
	case "visible_seconds":
		return trackingString(trackingData, "visible_seconds")

	case "hidden_seconds":
		return trackingString(trackingData, "hidden_seconds")

	case "session_count":
		return trackingString(trackingData, "session_count")

	case "max_scroll_percent":
		for _, k := range []string{"max_scroll", "max_scroll_percent"} {
			if v, ok := trackingData[k]; ok && v != nil {
				return fmt.Sprintf("%v%%", v)
			}
		}
		return ""

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

// BuildRowValues tạo mảng giá trị cho các cột theo thứ tự của FieldMappings
func BuildRowValues(mappings []models.FieldMapping, lead *models.Lead, trackingData map[string]interface{}) []interface{} {
	row := make([]interface{}, len(mappings))
	for i, m := range mappings {
		val := ExtractLeadValue(m.Field, m.CustomText, lead, trackingData)
		row[i] = val
	}
	return row
}

// GetStandardDefaultMappings trả về cấu hình ánh xạ cột tiêu chuẩn mặc định.
//
// Bao gồm ĐẦY ĐỦ thông số tracking (2026-11-12): macro Meta Ads, định danh
// lượt nhấp Meta (CAPI), thông tin thiết bị chi tiết, trạng thái VPN và
// hành vi người dùng.
func GetStandardDefaultMappings() []models.FieldMapping {
	return []models.FieldMapping{
		{Column: "A", Header: "Thời Gian", Field: "thoi_gian"},
		{Column: "B", Header: "Họ Và Tên", Field: "fullName"},
		{Column: "C", Header: "Số Điện Thoại", Field: "so_dien_thoai"},
		{Column: "D", Header: "Nhà Mạng", Field: "carrier"},
		{Column: "E", Header: "Landing ID", Field: "landing_id"},
		{Column: "F", Header: "Nguồn Traffic", Field: "nguon"},
		{Column: "G", Header: "Trạng Thái", Field: "trang_thai"},
		{Column: "H", Header: "Tự Động Điền", Field: "is_autofill"},
		{Column: "I", Header: "UTM Campaign", Field: "utm_campaign"},
		{Column: "J", Header: "Địa Chỉ IP", Field: "ip_address"},
		{Column: "K", Header: "Thiết Bị", Field: "device_type"},
		{Column: "L", Header: "Thời Gian Trên Trang", Field: "time_on_page"},
		{Column: "M", Header: "Cuộn Sâu Nhất", Field: "max_scroll"},
		{Column: "N", Header: "Ghi Chú", Field: "ghi_chu"},
		{Column: "O", Header: "URL Trang Đăng Ký", Field: "url"},
		// ─── Meta Ads macros ───
		{Column: "P", Header: "Meta UTM Source", Field: "utm_source"},
		{Column: "Q", Header: "Meta Placement", Field: "utm_placement"},
		{Column: "R", Header: "Meta Adset ID (utm_content)", Field: "utm_content"},
		{Column: "S", Header: "Meta Ad ID (utm_term)", Field: "utm_term"},
		{Column: "T", Header: "Meta Campaign Name", Field: "campaign_name"},
		{Column: "U", Header: "Meta Adset Name", Field: "adset_name"},
		{Column: "V", Header: "Meta Ad Name", Field: "ad_name"},
		{Column: "W", Header: "Meta Account ID", Field: "ad_account_id"},
		{Column: "X", Header: "Meta Site Source", Field: "site_source_name"},
		// ─── Định danh lượt nhấp Meta (CAPI) ───
		{Column: "Y", Header: "Meta Click ID (fbclid)", Field: "fbclid"},
		{Column: "Z", Header: "Meta _fbc", Field: "fbc"},
		{Column: "AA", Header: "Meta _fbp", Field: "fbp"},
		{Column: "AB", Header: "Meta Click ID tổng", Field: "meta_click_id"},
		// ─── Thiết bị & trình duyệt ───
		{Column: "AC", Header: "Hãng Thiết Bị", Field: "device_brand"},
		{Column: "AD", Header: "Tên Thiết Bị", Field: "device_name"},
		{Column: "AE", Header: "Mã Model", Field: "device_model_code"},
		{Column: "AF", Header: "Trình Duyệt", Field: "browser"},
		{Column: "AG", Header: "Phiên Bản Trình Duyệt", Field: "browser_version"},
		{Column: "AH", Header: "Trình Duyệt Trong App", Field: "is_in_app_browser"},
		{Column: "AI", Header: "Loại App", Field: "in_app_type"},
		{Column: "AJ", Header: "RAM (GB)", Field: "device_memory"},
		{Column: "AK", Header: "Số Lõi CPU", Field: "device_cores"},
		{Column: "AL", Header: "Múi Giờ", Field: "timezone"},
		{Column: "AM", Header: "Loại Mạng", Field: "connection_type"},
		{Column: "AN", Header: "Trạng Thái Thiết Bị", Field: "device_state"},
		// ─── VPN / Hành vi ───
		{Column: "AO", Header: "Dùng VPN", Field: "is_vpn"},
		{Column: "AP", Header: "Nhà Cung Cấp VPN", Field: "vpn_provider"},
		{Column: "AQ", Header: "Thời Gian Hiển Thị", Field: "visible_seconds"},
		{Column: "AR", Header: "Thời Gian Ẩn", Field: "hidden_seconds"},
		{Column: "AS", Header: "Số Phiên", Field: "session_count"},
	}
}
