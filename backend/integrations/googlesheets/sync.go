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

// BuildRowValues tạo mảng giá trị cho các cột theo thứ tự của FieldMappings
func BuildRowValues(mappings []models.FieldMapping, lead *models.Lead, trackingData map[string]interface{}) []interface{} {
	row := make([]interface{}, len(mappings))
	for i, m := range mappings {
		val := ExtractLeadValue(m.Field, m.CustomText, lead, trackingData)
		row[i] = val
	}
	return row
}

// GetStandardDefaultMappings trả về cấu hình ánh xạ cột tiêu chuẩn mặc định
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
	}
}
