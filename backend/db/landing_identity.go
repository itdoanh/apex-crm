package db

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ============================================================
// TÁCH BIỆT ĐỊNH DANH LANDING (fix 2026-11-15)
// ============================================================
//
// BỐI CẢNH (bug anh itdoanh báo — landing `xu_huong_1` chồng chéo `doanh_chung`):
//
//   Khi nhân bản 1 landing, toàn bộ "dấu vân tay" định danh của landing NGUỒN
//   được copy nguyên si sang bản sao:
//
//     1. `landing_pages.nguon_default` = "DTN 1" (nguồn của landing nguồn)
//        ⇒ mọi lead gửi từ bản sao đều bị ghi nhận dưới nguồn của landing
//          nguồn ⇒ báo cáo "nguồn" gộp 2 landing, không tách được.
//     2. `landing_files.config.json` có `landing_id` + `nguon` của landing nguồn
//        ⇒ tracker.js nạp config sai ⇒ analytics ghi về landing nguồn.
//     3. `landing_files/index.html` có
//        `<script src="tracker.js" data-landing-id="..." data-nguon="...">`
//        trỏ về landing nguồn ⇒ `data-landing-id` là nguồn ưu tiên CAO NHẤT
//        trong tracker.js ⇒ mọi view/lead của bản sao bị ghi về landing nguồn.
//     4. `window.APEX_LANDING_CONFIG` khai báo trong HTML cũng trỏ landing nguồn.
//
// ⇒ Bản sao trở thành "bóng ma" của landing nguồn: vẫn chạy, vẫn nhận
//   khách, nhưng mọi số liệu đều rơi vào landing nguồn.
//
// CÁCH SỬA:
//   Mọi dấu vân tay định danh đều được VIẾT LẠI theo `page_id` của bản sao.
//
// ────────────────────────────────────────────────────────────
// LƯU Ý QUAN TRỌNG VỀ `data-nguon`:
//   `data-nguon` được giữ (đổi sang nguồn mới) chứ không xoá, vì tracker.js
//   đọc `data-nguon` TRƯỚC `window.APEX_LANDING_CONFIG.nguon`. Nếu xoá hẳn,
//   tracker rơi về `nguon` của template (landing nguồn).
// ============================================================

// reDataLandingID khớp `data-landing-id="..."` trong thẻ <script>.
var reDataLandingID = regexp.MustCompile(`data-landing-id\s*=\s*"[^"]*"`)

// reDataNguon khớp `data-nguon="..."` trong thẻ <script>.
var reDataNguon = regexp.MustCompile(`data-nguon\s*=\s*"[^"]*"`)

// reConfigJSONLandingID khớp `"landing_id": "..."` trong file config.json.
var reConfigJSONLandingID = regexp.MustCompile(`"landing_id"\s*:\s*"[^"]*"`)

// reConfigJSONNguon khớp `"nguon": "..."` trong file config.json.
var reConfigJSONNguon = regexp.MustCompile(`"nguon"\s*:\s*"[^"]*"`)

// reAPEXConfigAssign khớp phần GÁN của `window.APEX_LANDING_CONFIG = ...`
// (không khớp object để tránh vấn đề nested brace).
var reAPEXConfigAssign = regexp.MustCompile(`window\.APEX_LANDING_CONFIG\s*=\s*`)

// StripUTF8BOM loại bỏ BOM (byte `\xEF\xBB\xBF`) ở đầu nội dung tệp.
//
// BỐI CẢNH (2026-11-15):
//   Tệp `config.json` / `index.html` được upload từ Windows (Notepad, VS Code)
//   thường CÓ BOM. JSON chuẩn (RFC 8259) KHÔNG cho phép BOM ⇒ `json.Unmarshal`
//   trong Go và `JSON.parse` trong trình duyệt đều THROW với BOM.
//
//   Hậu quả trong hệ thống:
//   - `tracker.js` gọi `res.json()` ⇒ ném lỗi ⇒ fallback `config.json` không
//     bao giờ nạp được ⇒ landing mất cấu hình (title, countdown, form mapping).
//   - `EnsureLandingConfigFile` / `RepairLandingIdentity` dùng `json.Unmarshal`
//     ⇒ parse fail ⇒ cơ chế TỰ HEAL không sửa được file BOM ⇒ bug nhân bản
//     chồng chéo định danh có thể tái diễn.
//
// Hàm này giữ nguyên phần còn lại của nội dung, chỉ cắt BOM ở đầu.
func StripUTF8BOM(content string) string {
	return strings.TrimPrefix(content, "\xEF\xBB\xBF")
}

// RewriteLandingIdentityHTML viết lại mọi dấu vân tay định danh của landing
// trong một tệp HTML.
//
//   - `data-landing-id="..."` → `data-landing-id="<landingID>"`
//   - `data-nguon="..."`      → `data-nguon="<nguon>"`
//   - `window.APEX_LANDING_CONFIG = {...}` → ép `landing_id`/`nguon`
//   - URL tuyệt đối chứa domain của landing nguồn → thay bằng domain mới
//     (nếu `newDomain` khác rỗng).
//
// `nguon` rỗng ⇒ giữ nguyên `data-nguon`/`nguon` hiện có (không ghi đè rỗng).
func RewriteLandingIdentityHTML(content string, landingID string, nguon string, oldDomain string, newDomain string) string {
	if content == "" {
		return content
	}

	landingID = strings.TrimSpace(landingID)
	nguon = strings.TrimSpace(nguon)
	if nguon == "" {
		nguon = landingID
	}
	if landingID == "" && nguon == "" {
		return content
	}

	// 1. `data-landing-id` — nguồn định danh ưu tiên cao nhất của tracker.js.
	if landingID != "" {
		content = reDataLandingID.ReplaceAllString(content, `data-landing-id="`+landingID+`"`)
	}
	if nguon != "" {
		content = reDataNguon.ReplaceAllString(content, `data-nguon="`+nguon+`"`)
	}

	// 2. `window.APEX_LANDING_CONFIG = {...}` — ép `landing_id` + `nguon`.
	content = rewriteAPEXConfigObject(content, landingID, nguon)

	// 3. URL tuyệt đối trỏ domain của landing NGUỒN.
	//
	//    Ví dụ landing nguồn `doanh_chung` chạy trên `lophoc.hanghoaphaisinh.net`
	//    mà bản sao deploy lên `xuhuong.hanghoaphaisinh.net` ⇒ `og:url`,
	//    `og:image`, `canonical` vẫn trỏ domain cũ ⇒ Facebook crawler đánh
	//    giá sai trang, và ảnh OG có thể 404 trên domain mới.
	oldDomain = strings.TrimSpace(strings.ToLower(oldDomain))
	newDomain = strings.TrimSpace(strings.ToLower(newDomain))
	if oldDomain != "" && newDomain != "" && oldDomain != newDomain {
		content = strings.ReplaceAll(content, oldDomain, newDomain)
	}

	return content
}

// rewriteAPEXConfigObject ép `landing_id` + `nguon` vào object được gán cho
// `window.APEX_LANDING_CONFIG`.
//
// Dùng cơ chế quét cân bằng ngoặc `{}` (thay vì regex) để xử lý được object
// JSON có nested object/array.
func rewriteAPEXConfigObject(content string, landingID string, nguon string) string {
	if landingID == "" && nguon == "" {
		return content
	}

	var out strings.Builder
	rest := content

	for {
		loc := reAPEXConfigAssign.FindStringIndex(rest)
		if loc == nil {
			out.WriteString(rest)
			break
		}

		out.WriteString(rest[:loc[1]])

		// Tìm vị trí `{` bắt đầu object.
		braceIdx := strings.Index(rest[loc[1]:], "{")
		if braceIdx < 0 {
			out.WriteString(rest[loc[1]:])
			break
		}
		braceIdx += loc[1]

		end := findMatchingBrace(rest, braceIdx)
		if end < 0 {
			// Không tìm thấy `}` khớp ⇒ giữ nguyên phần còn lại.
			out.WriteString(rest[braceIdx:])
			break
		}

		objText := rest[braceIdx : end+1]
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(objText), &cfg); err == nil {
			if landingID != "" {
				cfg["landing_id"] = landingID
			}
			if nguon != "" {
				cfg["nguon"] = nguon
			}
			if newObj, err := json.Marshal(cfg); err == nil {
				out.WriteString(string(newObj))
			} else {
				out.WriteString(objText)
			}
		} else {
			out.WriteString(objText)
		}

		rest = rest[end+1:]
	}

	return out.String()
}

// findMatchingBrace trả về index của `}` khớp với `{` tại `start`.
func findMatchingBrace(s string, start int) int {
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// RewriteLandingConfigJSON viết lại `landing_id` (+ `nguon` nếu được yêu cầu)
// trong `config.json`.
//
//   - `landingID` : giá trị mới cho `landing_id` (bắt buộc, rỗng ⇒ no-op).
//   - `nguon`     : giá trị mới cho `nguon`.
//     - Rỗng ⇒ GIỮ NGUYÊN `nguon` hiện có (trường hợp chuẩn hoá 1 landing đang
//       chạy, `nguon` do user cấu hình là hợp lệ).
//     - Khác rỗng ⇒ ép `nguon` = giá trị này (trường hợp NHÂN BẢN — bản sao
//       phải có nguồn riêng, không được dùng chung nguồn với landing nguồn).
//
//   - Nếu file là JSON hợp lệ → parse rồi set, marshal lại.
//   - Nếu không phải JSON hợp lệ → dùng regex thay thế (fallback an toàn).
func RewriteLandingConfigJSON(content string, landingID string, nguon string) string {
	landingID = strings.TrimSpace(landingID)
	nguon = strings.TrimSpace(nguon)
	if content == "" || landingID == "" {
		return content
	}

	// BOM làm `json.Unmarshal` fail ⇒ phải cắt BOM TRƯỚC khi parse.
	// Đồng thời kết quả marshal lại KHÔNG có BOM ⇒ JSON chuẩn, trình duyệt
	// `JSON.parse` chạy được.
	content = StripUTF8BOM(content)

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(content), &cfg); err == nil {
		cfg["landing_id"] = landingID
		if nguon != "" {
			cfg["nguon"] = nguon
		} else if cur, ok := cfg["nguon"].(string); !ok || strings.TrimSpace(cur) == "" {
			// Chưa cấu hình nguồn ⇒ đặt theo `landing_id` (quy tắc mặc định).
			cfg["nguon"] = landingID
		}
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err == nil {
			return string(out)
		}
	}

	// Fallback: JSON hỏng ⇒ thay bằng regex.
	content = reConfigJSONLandingID.ReplaceAllString(content, `"landing_id": "`+landingID+`"`)
	if nguon != "" {
		content = reConfigJSONNguon.ReplaceAllString(content, `"nguon": "`+nguon+`"`)
	}
	return content
}

// ValidateLandingIdentity trả về lỗi nếu định danh của landing không hợp lệ.
func ValidateLandingIdentity(pageID string, nguonDefault string) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return fmt.Errorf("page_id không được để trống")
	}
	if strings.ContainsAny(pageID, " \t\r\n/\\") {
		return fmt.Errorf("page_id không được chứa khoảng trắng hoặc dấu gạch chéo: %q", pageID)
	}
	if len(pageID) > 100 {
		return fmt.Errorf("page_id quá dài (tối đa 100 ký tự): %d", len(pageID))
	}
	if n := strings.TrimSpace(nguonDefault); n != "" && len(n) > 100 {
		return fmt.Errorf("nguon_default quá dài (tối đa 100 ký tự): %d", len(n))
	}
	return nil
}
