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
//
// ⚠️ Meta CHỈ chấp nhận mã quốc gia 2 chữ cái (ISO-3166-1 alpha-2) viết
// THƯỜNG. Hệ thống lưu `country` dạng tiếng Việt ("Việt Nam") ⇒ băm ra
// giá trị KHÔNG BAO GIỜ khớp với dữ liệu Meta ⇒ EMQ mất điểm.
//
// ⇒ Hàm này chuyển mọi biến thể ("Việt Nam", "Vietnam", "VN", "vn") về "vn".
func NormalizeCountry(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	// Bỏ tiền tố "vn-" / "vi-" (dạng "vn-Hà Nội").
	s = strings.TrimPrefix(s, "vn-")
	s = strings.TrimPrefix(s, "vi-")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Bỏ dấu để so sánh được "việt nam" (có dấu) ↔ "viet nam" (không dấu).
	plain := StripVietnameseDiacritics(s)
	plain = strings.Join(strings.Fields(plain), " ")

	// Tên quốc gia đầy đủ → mã ISO-2.
	switch plain {
	case "vietnam", "viet nam", "vn", "vietnam vietnam", "vietnam viet nam", "vn vn":
		return "vn"
	}

	// Đã là mã 2 chữ cái → giữ nguyên (chuyển lowercase).
	if len(plain) == 2 && isAlpha(plain) {
		return plain
	}
	return s
}

// isAlpha kiểm tra chuỗi chỉ gồm chữ cái Latin.
func isAlpha(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return len(s) > 0
}

// NormalizeCity chuẩn hoá tên thành phố / tỉnh cho Meta `ct` và `st`.
//
// ⚠️ Meta so khớp `ct` / `st` với dữ liệu địa lý của họ (tên KHÔNG dấu,
// viết thường): "Hà Nội" → "ha noi", "Hồ Chí Minh" → "ho chi minh".
// Nếu giữ dấu ("hà nội") ⇒ hash không khớp ⇒ EMQ mất điểm `ct`.
//
// ⇒ Bỏ dấu tiếng Việt + lowercase + gộp khoảng trắng.
func NormalizeCity(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = StripVietnameseDiacritics(s)
	return NormalizeText(s)
}

// StripVietnameseDiacritics bỏ dấu tiếng Việt.
//
// ⚠️ KHÔNG dùng `unicode/norm` (NFD) vì bản Go cài trên server không có
// package đó. Thay vào đó dùng bảng tra NFD tối giản: mỗi ký tự có dấu
// được map thẳng sang chữ cái gốc (chữ cái Latin có dấu trong tiếng Việt
// đều là 1 ký tự Unicode duy nhất, không phải tổ hợp base + mark).
//
// ⇒ Kết quả: "Hà Nội" → "Ha Noi", "Hồ Chí Minh" → "Ho Chi Minh",
//   "Đắk Lắk" → "Dak Lak".
func StripVietnameseDiacritics(raw string) string {
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if repl, ok := vietnameseDiacritics[r]; ok {
			b.WriteRune(repl)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// vietnameseDiacritics là bảng tra chữ cái Latin có dấu → chữ cái gốc.
//
// Đầy đủ 26 chữ cái × 5 kiểu dấu (sắc, huyền, hỏi, ngã, nặng) + chữ Đ/đ
// + các nguyên âm ê/ô/ư có dấu ⇒ bảng tra chuẩn của tiếng Việt.
var vietnameseDiacritics = map[rune]rune{
	'A': 'A',
	'Á': 'A',
	'À': 'A',
	'Ả': 'A',
	'Ã': 'A',
	'Ạ': 'A',
	'Ắ': 'A',
	'Ằ': 'A',
	'Ẳ': 'A',
	'Ẵ': 'A',
	'Ặ': 'A',
	'Ấ': 'A',
	'Ầ': 'A',
	'Ẩ': 'A',
	'Ẫ': 'A',
	'Ậ': 'A',
	'Â': 'A',
	'Ă': 'A',
	'B': 'B',
	'C': 'C',
	'D': 'D',
	'Đ': 'D',
	'E': 'E',
	'É': 'E',
	'È': 'E',
	'Ẻ': 'E',
	'Ẽ': 'E',
	'Ẹ': 'E',
	'Ế': 'E',
	'Ề': 'E',
	'Ể': 'E',
	'Ễ': 'E',
	'Ệ': 'E',
	'Ê': 'E',
	'F': 'F',
	'G': 'G',
	'H': 'H',
	'I': 'I',
	'Í': 'I',
	'Ì': 'I',
	'Ỉ': 'I',
	'Ĩ': 'I',
	'Ị': 'I',
	'J': 'J',
	'K': 'K',
	'L': 'L',
	'M': 'M',
	'N': 'N',
	'Ñ': 'N',
	'O': 'O',
	'Ó': 'O',
	'Ò': 'O',
	'Ỏ': 'O',
	'Õ': 'O',
	'Ọ': 'O',
	'Ố': 'O',
	'Ồ': 'O',
	'Ổ': 'O',
	'Ỗ': 'O',
	'Ộ': 'O',
	'Ớ': 'O',
	'Ờ': 'O',
	'Ở': 'O',
	'Ỡ': 'O',
	'Ợ': 'O',
	'Ô': 'O',
	'Ơ': 'O',
	'P': 'P',
	'Q': 'Q',
	'R': 'R',
	'S': 'S',
	'T': 'T',
	'U': 'U',
	'Ú': 'U',
	'Ù': 'U',
	'Ủ': 'U',
	'Ũ': 'U',
	'Ụ': 'U',
	'Ứ': 'U',
	'Ừ': 'U',
	'Ử': 'U',
	'Ữ': 'U',
	'Ự': 'U',
	'Ư': 'U',
	'V': 'V',
	'W': 'W',
	'X': 'X',
	'Y': 'Y',
	'Ý': 'Y',
	'Ỳ': 'Y',
	'Ỷ': 'Y',
	'Ỹ': 'Y',
	'Ỵ': 'Y',
	'Z': 'Z',
	'a': 'a',
	'á': 'a',
	'à': 'a',
	'ả': 'a',
	'ã': 'a',
	'ạ': 'a',
	'ắ': 'a',
	'ằ': 'a',
	'ẳ': 'a',
	'ẵ': 'a',
	'ặ': 'a',
	'ấ': 'a',
	'ầ': 'a',
	'ẩ': 'a',
	'ẫ': 'a',
	'ậ': 'a',
	'â': 'a',
	'ă': 'a',
	'b': 'b',
	'c': 'c',
	'd': 'd',
	'đ': 'd',
	'e': 'e',
	'é': 'e',
	'è': 'e',
	'ẻ': 'e',
	'ẽ': 'e',
	'ẹ': 'e',
	'ế': 'e',
	'ề': 'e',
	'ể': 'e',
	'ễ': 'e',
	'ệ': 'e',
	'ê': 'e',
	'f': 'f',
	'g': 'g',
	'h': 'h',
	'i': 'i',
	'í': 'i',
	'ì': 'i',
	'ỉ': 'i',
	'ĩ': 'i',
	'ị': 'i',
	'j': 'j',
	'k': 'k',
	'l': 'l',
	'm': 'm',
	'n': 'n',
	'ñ': 'n',
	'o': 'o',
	'ó': 'o',
	'ò': 'o',
	'ỏ': 'o',
	'õ': 'o',
	'ọ': 'o',
	'ố': 'o',
	'ồ': 'o',
	'ổ': 'o',
	'ỗ': 'o',
	'ộ': 'o',
	'ớ': 'o',
	'ờ': 'o',
	'ở': 'o',
	'ỡ': 'o',
	'ợ': 'o',
	'ô': 'o',
	'ơ': 'o',
	'p': 'p',
	'q': 'q',
	'r': 'r',
	's': 's',
	't': 't',
	'u': 'u',
	'ú': 'u',
	'ù': 'u',
	'ủ': 'u',
	'ũ': 'u',
	'ụ': 'u',
	'ứ': 'u',
	'ừ': 'u',
	'ử': 'u',
	'ữ': 'u',
	'ự': 'u',
	'ư': 'u',
	'v': 'v',
	'w': 'w',
	'x': 'x',
	'y': 'y',
	'ý': 'y',
	'ỳ': 'y',
	'ỷ': 'y',
	'ỹ': 'y',
	'ỵ': 'y',
	'z': 'z',
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
