package facebook

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// ─── Chuẩn hoá & băm SHA-256 theo chuẩn Meta Conversions API ───────────
//
// Meta yêu cầu: PII (email, phone, tên, thành phố, quốc gia, tỉnh) phải được
// băm SHA-256 SAU KHI chuẩn hoá (lowercase + trim + bỏ khoảng trắng thừa).
//
// Quy tắc chuẩn hoá chính thức của Meta:
//   - `em`  : trim + lowercase
//   - `ph`  : bỏ mọi ký tự không phải số ở CUỐI (giữ dấu `+` ở đầu)
//   - `fn`/`ln`/`ct`/`st`/`zp`/`country` : trim + lowercase (bỏ khoảng trắng thừa bên trong)
//   - `external_id` : hash thẳng giá trị đã chuẩn hoá (thường là SĐT)
//
// Lưu ý: Meta chấp nhận cả email đã băm lẫn email chưa băm (chỉ băm một lần).

// NormalizeEmail chuẩn hoá email trước khi băm.
func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// NormalizePhone chuẩn hoá số điện thoại theo chuẩn Meta.
//
// 📌 Meta yêu cầu `ph` ở định dạng **E.164** (vd `+84912345678`) để khoá so
// khớp hiệu quả nhất. Số Việt Nam ở dạng local (`0912345678`) được đổi sang
// `+84912345678`.
//
// Quy tắc:
//   - Giữ dấu `+` ở đầu (chỉ dành cho số quốc tế), bỏ mọi ký tự phi số khác.
//   - `0xxx…` (10 số kiểu VN) ⇒ `+84xxx…`
//   - `84xxx…` (11 số, không có `+`) ⇒ `+84xxx…`
func NormalizePhone(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	plus := strings.HasPrefix(s, "+")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if plus {
		return "+" + out
	}

	// Chuẩn hoá số Việt Nam về E.164.
	switch {
	case strings.HasPrefix(out, "84") && len(out) == 11:
		return "+" + out
	case len(out) == 10 && strings.HasPrefix(out, "0"):
		return "+84" + out[1:]
	}
	return out
}

// NormalizeText chuẩn hoá chuỗi văn bản tự do (tên, thành phố, tỉnh).
// Trim + lowercase + gộp khoảng trắng liên tiếp thành 1.
func NormalizeText(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

// NormalizeCountry chuẩn hoá mã quốc gia (ISO-2).
func NormalizeCountry(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "vn-")
	s = strings.TrimPrefix(s, "vi-")
	if s == "" {
		return ""
	}
	return s
}

// HashSHA256 trả về chuỗi hex 64 ký tự của SHA-256.
func HashSHA256(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// HashEmail băm email đã chuẩn hoá.
func HashEmail(raw string) string {
	n := NormalizeEmail(raw)
	if n == "" {
		return ""
	}
	return HashSHA256(n)
}

// HashPhone băm số điện thoại đã chuẩn hoá.
func HashPhone(raw string) string {
	n := NormalizePhone(raw)
	if n == "" {
		return ""
	}
	return HashSHA256(n)
}

// HashText băm chuỗi văn bản đã chuẩn hoá (tên, thành phố, tỉnh, zip).
func HashText(raw string) string {
	n := NormalizeText(raw)
	if n == "" {
		return ""
	}
	return HashSHA256(n)
}

// HashCountry băm mã quốc gia đã chuẩn hoá.
func HashCountry(raw string) string {
	n := NormalizeCountry(raw)
	if n == "" {
		return ""
	}
	return HashSHA256(n)
}

// HashExternalID băm external_id (thường dùng SĐT chuẩn hoá) để đối soát
// với khách hàng đã có trong CRM hoặc trong Ads Manager.
func HashExternalID(raw string) string {
	n := NormalizePhone(raw)
	if n == "" {
		n = NormalizeText(raw)
	}
	if n == "" {
		return ""
	}
	return HashSHA256(n)
}
