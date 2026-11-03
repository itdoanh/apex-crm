package security

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Danh sách các trường Honeypot được cắm bẫy ngầm trong biểu mẫu
// Đã loại trừ các từ khóa nhạy cảm với Autofill của trình duyệt (như email, name, tel)
// để trình duyệt không bao giờ tự động điền nhầm vào bẫy của người thật.
var honeypotFields = []string{
	"website_fax_number_sec",
	"fax_company_prefix",
	"form_b64_signature_field",
	"terms_confirm_bot",
	"hp_verify_token",
	"user_email_verify",
	"company_website_url",
	"extra_bio",
}

// DetectHoneypot kiểm tra xem kẻ gửi request có điền bất kỳ trường bẫy nào không
func DetectHoneypot(data map[string]interface{}) (isSpam bool, reason string) {
	for _, field := range honeypotFields {
		if val, exists := data[field]; exists && val != nil {
			str := strings.TrimSpace(fmt.Sprintf("%v", val))
			if str != "" && str != "false" && str != "0" && str != "<nil>" {
				return true, fmt.Sprintf("honeypot_triggered:%s", field)
			}
		}
	}
	return false, ""
}

// CheckSubmissionSpeed kiểm tra thời gian thao tác điền form của người dùng.
// QUAN TRỌNG: Nếu người dùng sử dụng tính năng TỰ ĐỘNG ĐIỀN (Autofill từ URL, LocalStorage hoặc Trình duyệt),
// tốc độ điền và gửi form sẽ cực nhanh (<1 giây) và HOÀN TOÀN HỢP LỆ, không được chặn!
func CheckSubmissionSpeed(fillDurationMs int64, isAutofill bool) (isSpam bool, reason string) {
	if isAutofill {
		return false, ""
	}
	// Nếu không có cờ tự động điền mà thời gian điền form dưới 800ms (tốc độ mà con người không thể gõ tay)
	if fillDurationMs > 0 && fillDurationMs < 800 {
		return true, fmt.Sprintf("rapid_submission_bot:%dms", fillDurationMs)
	}
	return false, ""
}

// TarpitSleep tạo độ trễ giả lập nhằm làm chậm tối đa tốc độ quét của bot/crawler
func TarpitSleep(minMs, maxMs int) {
	if maxMs <= minMs {
		maxMs = minMs + 500
	}
	diff := int64(maxMs - minMs)
	n, err := rand.Int(rand.Reader, big.NewInt(diff))
	sleepMs := int64(minMs)
	if err == nil {
		sleepMs += n.Int64()
	}
	time.Sleep(time.Duration(sleepMs) * time.Millisecond)
}
