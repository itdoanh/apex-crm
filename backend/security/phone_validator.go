package security

import (
	"regexp"
	"strings"
)

var (
	// Regex chuẩn quốc gia Việt Nam: bắt đầu bằng 0, tiếp theo là mã mạng 2 chữ số, và 7 chữ số thuê bao
	vnPhoneRegex = regexp.MustCompile(`^(0)(3[2-9]|5[25689]|7[06-9]|8[1-9]|9[0-46-9])[0-9]{7}$`)

	// Các mẫu số ảo phổ biến hay bị bot spam
	repeatedSequences = []string{
		"0123456789", "0987654321", "0999999999", "0888888888", "0777777777",
		"0666666666", "0555555555", "0444444444", "0333333333", "0222222222",
		"0111111111", "0000000000",
	}
)

// NormalizePhone làm sạch số điện thoại về chuẩn 10 chữ số
func NormalizePhone(raw string) string {
	re := regexp.MustCompile(`[^\d+]`)
	cleaned := re.ReplaceAllString(raw, "")

	if strings.HasPrefix(cleaned, "+84") {
		cleaned = "0" + strings.TrimPrefix(cleaned, "+84")
	} else if strings.HasPrefix(cleaned, "84") && len(cleaned) >= 11 {
		cleaned = "0" + strings.TrimPrefix(cleaned, "84")
	}

	return cleaned
}

// GetCarrierName nhận diện tên nhà mạng từ số điện thoại
func GetCarrierName(phone string) string {
	if len(phone) < 3 {
		return "Unknown"
	}
	prefix3 := phone[:3]

	// Viettel
	if prefix3 == "086" || prefix3 == "096" || prefix3 == "097" || prefix3 == "098" ||
		(len(phone) >= 3 && phone[:2] == "03") {
		return "Viettel"
	}

	// Vinaphone
	if prefix3 == "088" || prefix3 == "091" || prefix3 == "094" ||
		prefix3 == "081" || prefix3 == "082" || prefix3 == "083" || prefix3 == "084" || prefix3 == "085" {
		return "Vinaphone"
	}

	// Mobifone
	if prefix3 == "089" || prefix3 == "090" || prefix3 == "093" ||
		(len(phone) >= 3 && phone[:2] == "07" && prefix3 != "0775") {
		return "Mobifone"
	}

	// Vietnamobile
	if prefix3 == "092" || prefix3 == "056" || prefix3 == "058" {
		return "Vietnamobile"
	}

	// Wintel / Itelecom / Gmobile / FPT
	if prefix3 == "055" {
		return "Wintel"
	}
	if prefix3 == "087" {
		return "Itelecom"
	}
	if prefix3 == "059" {
		return "Gmobile"
	}

	return "Khác"
}

// ValidateVietnamesePhone kiểm tra tính hợp lệ toàn diện của số điện thoại Việt Nam
func ValidateVietnamesePhone(rawPhone string) (isValid bool, cleanedPhone string, reason string) {
	phone := NormalizePhone(rawPhone)

	if phone == "" {
		return false, "", "Số điện thoại không được để trống"
	}

	if len(phone) != 10 {
		return false, phone, "Số điện thoại phải có đúng 10 chữ số"
	}

	// Kiểm tra xem có trùng với danh sách số ảo/số thử nghiệm không
	for _, seq := range repeatedSequences {
		if phone == seq {
			return false, phone, "Số điện thoại là chuỗi lặp không hợp lệ"
		}
	}

	// Kiểm tra nếu 7 số cuối hoàn toàn giống nhau (vd: 0911111111, 0988888888)
	last7 := phone[3:]
	allSame := true
	for i := 1; i < len(last7); i++ {
		if last7[i] != last7[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false, phone, "Số điện thoại có đuôi số lặp bất thường"
	}

	// Kiểm tra bằng Regex đầu số nhà mạng quốc gia
	if !vnPhoneRegex.MatchString(phone) {
		return false, phone, "Đầu số điện thoại không thuộc nhà mạng viễn thông Việt Nam"
	}

	return true, phone, ""
}
