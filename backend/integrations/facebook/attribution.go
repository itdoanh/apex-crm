package facebook

import (
	"fmt"
	"strings"
)

// ─── Chuẩn hoá PII theo chuẩn Meta Conversions API ────────────────────
//
// 📌 VÌ SAO CẦN FILE NÀY (chẩn đoán EMQ 2026-11-15):
//
// Meta so khớp `user_data` bằng CÁCH BĂM (hash) giá trị đã chuẩn hoá của
// họ. Nếu ta gửi giá trị khác chuẩn ⇒ hash KHÔNG BAO GIỜ trùng ⇒ Meta
// không ghép được sự kiện browser với sự kiện server ⇒ EMQ thấp.
//
// Ba lỗi đã phát hiện trong hệ thống:
//
//	1. `country` gửi "Việt Nam" (tiếng Việt, có dấu) trong khi Meta dùng
//	   ISO-2 "vn" ⇒ 0% khớp. (Meta báo "Quốc gia: 50%")
//	2. `st` gửi "Miền Bắc" (VÙNG KINH TẾ — không phải tỉnh/bang) và
//	   `ct` gửi "Hà Nội" (có dấu) ⇒ không khớp dữ liệu địa lý của Meta.
//	3. `client_ip_address` / `client_user_agent` / `fbp` / `fbc` chỉ gửi
//	   được 50% vì lead tạo trước đây không lưu đủ thông tin.
//
// ⇒ File này gom các hàm chuẩn hoá + logic "lead có đến từ Facebook không".

// ─── Quốc gia / Tỉnh / Thành phố ───────────────────────────────────────

// BuildCountryISO trả mã ISO-2 chuẩn Meta (vd "Việt Nam" → "vn").
//
// Trả về "" nếu không xác định được (Meta yêu cầu bỏ trống thay vì gửi
// giá trị sai — gửi sai còn tệ hơn không gửi).
func BuildCountryISO(raw string) string {
	return NormalizeCountry(raw)
}

// BuildCity chuẩn hoá thành phố cho Meta `ct` (bỏ dấu, lowercase).
func BuildCity(raw string) string {
	return NormalizeCity(raw)
}

// BuildState chuẩn hoá tỉnh/bang cho Meta `st`.
//
// ⚠️ Hệ thống lưu `region` là VÙNG KINH TẾ ("Miền Bắc", "Miền Nam",
// "Miền Trung", "Quốc Tế") — KHÔNG phải tỉnh/bang ⇒ Meta không thể khớp
// ⇒ không gửi `st` khi giá trị là vùng miền.
//
// Trả về "" nếu không phải tỉnh/bang hợp lệ.
func BuildState(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Loại các giá trị vùng miền — Meta không có khái niệm này.
	switch strings.ToLower(s) {
	case "miền bắc", "mien bac", "miền trung", "mien trung",
		"miền nam", "mien nam", "quốc tế", "quoc te", "khác / chưa rõ",
		"khac / chua ro", "hà nội (local)", "hanoi (local)":
		return ""
	}
	return NormalizeCity(s)
}

// ─── Attribution: lead có đến từ Facebook không? ────────────────────────

// FacebookTraffic là kết quả kiểm tra nguồn traffic của 1 lead.
type FacebookTraffic struct {
	// IsFacebook = true ⇒ lead có bằng chứng đến từ quảng cáo Meta.
	IsFacebook bool
	// Source mô tả bằng chứng tìm thấy (dùng để log / debug).
	Source string
}

// fbTrafficSources là tập nguồn UTM / site_source_name của Meta.
//
// Meta Ads đẩy `utm_source={{site_source_name}}` với giá trị trong
// {fb, ig, msg, an, facebook, instagram, messenger, audience_network}.
var fbTrafficSources = map[string]bool{
	"fb":                true,
	"ig":                true,
	"msg":               true,
	"an":                true,
	"facebook":          true,
	"fb.com":            true,
	"instagram":         true,
	"instagram.com":     true,
	"messenger":         true,
	"m":                 true,
	"facebook_com":      true,
	"instagram_com":     true,
	"audience_network":  true,
	"audience network":  true,
	"meta_ads":          true,
	"facebook_ads":      true,
	"instagram_ads":     true,
	"meta_ads_platform": true,
}

// fbRefererHosts là các host referrer thuộc Meta.
var fbRefererHosts = []string{
	"facebook.com",
	"fb.com",
	"fb.me",
	"instagram.com",
	"l.facebook.com",
	"m.facebook.com",
	"lm.facebook.com",
	"messenger.com",
	"meta.com",
}

// DetectFacebookTraffic kiểm tra lead có đến từ quảng cáo Facebook/Instagram.
//
// 📌 YÊU CẦU CỦA ANH: "CAPI chỉ gửi khi lead đó đến từ FB chứ không gửi
// các lead ngoài". Traffic organic / Google Ads / TikTok / truy cập thẳng
// ⇒ KHÔNG gửi CAPI (vì không có lượt nhấp Meta để khớp, và gửi sẽ làm
// nhiễu chỉ số quảng cáo + hạ EMQ).
//
// Thứ tự ưu tiên bằng chứng (mạnh → yếu):
//  1. `fbc`     — cookie `_fbc` (định danh lượt nhấp Meta)  ★ mạnh nhất
//  2. `fbclid`  — tham số click ID trong URL                  ★ mạnh
//  3. `fbp`     — cookie `_fbp` (định danh trình duyệt đã từng thấy Meta)
//  4. `utm_source` / `site_source_name` ∈ tập nguồn Meta
//  5. `utm_medium = paid` + referrer thuộc domain Meta
//
// ⚠️ `fbp` chỉ là bằng chứng GIỚI (yếu nhất): một người dùng từng click
// FB rồi vào landing bằng organic vẫn có `_fbp`. Vì vậy ưu tiên `fbc`/
// `fbclid` trước, và coi `fbp` là bằng chứng cuối cùng.
func DetectFacebookTraffic(tracking map[string]interface{}) FacebookTraffic {
	if len(tracking) == 0 {
		return FacebookTraffic{IsFacebook: false, Source: "không có tracking"}
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := tracking[k]; ok && v != nil {
				s := strings.TrimSpace(toStr(v))
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
		return ""
	}

	// 1. `fbc` — bằng chứng mạnh nhất (định danh lượt nhấp Meta).
	if v := get("fbc", "_fbc"); v != "" {
		return FacebookTraffic{IsFacebook: true, Source: "fbc"}
	}

	// 2. `fbclid` — click ID Meta gắn vào URL.
	if v := get("fbclid", "fb_click_id", "meta_click_id"); v != "" {
		return FacebookTraffic{IsFacebook: true, Source: "fbclid"}
	}

	// 3. `site_source_name` / `utm_source` thuộc Meta.
	src := strings.ToLower(get("site_source_name", "utm_source"))
	if src != "" && fbTrafficSources[src] {
		return FacebookTraffic{IsFacebook: true, Source: "utm_source=" + src}
	}

	// 4. Referrer thuộc domain Meta + paid.
	medium := strings.ToLower(get("utm_medium"))
	referer := strings.ToLower(get("referrer", "server_referer", "entry_url"))
	if isMetaReferer(referer) && (medium == "paid" || medium == "cpc" || medium == "ads") {
		return FacebookTraffic{IsFacebook: true, Source: "referrer_meta+" + medium}
	}

	// 5. `fbp` — bằng chứng yếu nhất (bị loại trừ nếu `fbc`/`fbclid` vắng).
	if v := get("fbp", "_fbp"); v != "" {
		return FacebookTraffic{IsFacebook: true, Source: "fbp"}
	}

	return FacebookTraffic{IsFacebook: false, Source: "không có bằng chứng Meta"}
}

// isMetaReferer kiểm tra URL/referrer có thuộc domain Meta không.
func isMetaReferer(raw string) bool {
	r := strings.ToLower(strings.TrimSpace(raw))
	if r == "" {
		return false
	}
	for _, host := range fbRefererHosts {
		if strings.Contains(r, host) {
			return true
		}
	}
	return false
}

// toStr chuyển interface{} thành string an toàn.
func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}
