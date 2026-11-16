package security

import (
	"strings"
)

// ══════════════════════════════════════════════════════════════════════
// PHÁT HIỆN SỐ ĐIỆN THOẠI "RÁC" (JUNK / FAKE PHONE)
// ══════════════════════════════════════════════════════════════════════
//
// BỐI CẢNH (yêu cầu 2026-11-16 của anh itdoanh):
//   Người dùng cố tình nhập số điện thoại rác (vd "0987654321") chỉ để
//   đốt tiền quảng cáo (QC). Mỗi lượt gửi form ⇒ 1 lead ⇒ 1 sự kiện
//   Meta Conversions API ⇒ tốn tiền quảng cáo.
//
// MỤC TIÊU:
//   1. CLIENT (tracker.js): phát hiện SĐT rác NGAY khi bấm "Đăng ký"
//      ⇒ lưu cookie chặn + chuyển thẳng sang trang "chúc mừng" ⇒ KHÔNG
//      gửi payload lên server ⇒ không tốn tiền QC.
//   2. SERVER (lead_handler.go): chặn phụ (defense in depth) phòng khi
//      client bị bypass ⇒ ghi vào vùng cách ly, KHÔNG tạo lead, KHÔNG
//      bắn CAPI.
//
// ⚠️ NGUYÊN TẮC: KHÔNG được chặn nhầm số thật.
//   ⇒ Chỉ chặn khi CHẮC CHẮN là rác (heuristic rất chặt).
//
// ──────────────────────────────────────────────────────────────────────
// DANH SÁCH MẪU RÁC ĐƯỢC CHẶN
// ──────────────────────────────────────────────────────────────────────
//
// 1. SỐ LẶP TOÀN BỘ: 0000000000, 1111111111, 0999999999
// 2. DÃY SỐ LIÊN TIẾP TĂNG: 0123456789, 1234567890 (tăng đều +1)
// 3. DÃY SỐ LIÊN TIẾP GIẢM: 0987654321, 1098765432 (giảm đều -1)
// 4. SỐ LẶP THEO MẪU 2 SỐ: 0909090909, 0120120120 (ABABABABAB)
// 5. SỐ LẶP THEO MẪU 5 SỐ: 0123401234, 0912309123 (ABCDEABCDE)
// 6. ĐUÔI 7 SỐ GIỐNG NHAU: 0911111111, 0988888888
// 7. DANH SÁCH CỨNG: các mẫu bot hay dùng (0987654321, 0123456789, ...)

// junkHardcoded là danh sách mẫu rác cố định (bot hay dùng).
//
// ⚠️ Lưu ý: danh sách này TRÙNG với `repeatedSequences` trong
// phone_validator.go — nhưng hàm đó CHỈ so khớp CHÍNH XÁC từng số,
// còn `IsJunkPhone` bắt được cả mẫu lặp/liên tiếp tổng quát.
var junkHardcoded = map[string]bool{
	"0000000000": true,
	"0111111111": true,
	"0123456789": true,
	"0222222222": true,
	"0333333333": true,
	"0444444444": true,
	"0555555555": true,
	"0666666666": true,
	"0777777777": true,
	"0888888888": true,
	"0987654321": true,
	"0999999999": true,
	"1234567890": true,
	"2109876543": true,
}

// IsJunkPhone kiểm tra một số điện thoại có phải số "rác" hay không.
//
// Trả về (isJunk, reason).
//
// ⚠️ Hàm này là "heuristic rất chặt" — chỉ trả true khi CHẮC CHẮN rác.
//   Số thật của khách hàng (dù có đuôi lặp nhẹ) KHÔNG bị chặn.
//
// Số đầu vào có thể ở mọi định dạng ("0987654321", "+84987654321", "098-765-4321").
func IsJunkPhone(rawPhone string) (bool, string) {
	phone := NormalizePhone(rawPhone)
	phone = strings.TrimSpace(phone)

	// Chỉ xét số 10 chữ số kiểu Việt Nam — không đủ dữ liệu thì không chặn.
	if len(phone) != 10 {
		return false, ""
	}

	// 1. Danh sách cứng
	if junkHardcoded[phone] {
		return true, "junk_phone: mẫu số rác cố định"
	}

	// 2. 7 số cuối giống hệt nhau (0911111111, 0988888888)
	if hasRepeatedRun(phone[3:], 7) {
		return true, "junk_phone: 7 số cuối giống hệt nhau"
	}

	// 3. Dãy số liên tiếp tăng/giảm đều (0123456789, 0987654321)
	if isSequentialDigits(phone) {
		return true, "junk_phone: dãy số liên tiếp tăng/giảm đều"
	}

	// 4. Số lặp theo mẫu 2 số (0909090909), 5 số (0123401234) hoặc 3 số
	//    lặp trong 9 số đầu (0120120120).
	if hasRepeatedBlock(phone, 2) || hasRepeatedBlock(phone, 5) || hasRepeatedBlock(phone[:9], 3) {
		return true, "junk_phone: số lặp theo mẫu"
	}

	return false, ""
}

// hasRepeatedRun kiểm tra chuỗi có chứa một đoạn liên tiếp >= minLen ký tự
// giống hệt nhau hay không.
//
//   hasRepeatedRun("1111111", 7) → true
//   hasRepeatedRun("1111112", 7) → false
func hasRepeatedRun(s string, minLen int) bool {
	if len(s) < minLen {
		return false
	}
	run := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			run++
			if run >= minLen {
				return true
			}
		} else {
			run = 1
		}
	}
	return false
}

// hasRepeatedBlock kiểm tra chuỗi có gồm MỘT khối ký tự bị lặp nguyên vẹn
// hay không (khối lặp phải khớp CHÍNH XÁC từ đầu chuỗi).
//
//   hasRepeatedBlock("0909090909", 2) → true  (ABABABABAB)
//   hasRepeatedBlock("0123401234", 5) → true  (ABCDEABCDE)
//   hasRepeatedBlock("0912345678", 2) → false
//
// ⚠️ Yêu cầu độ dài chuỗi chia hết cho kích thước khối, và khối phải lặp
//   TRÙNG KHÍT từ đầu — nếu không, số thật có thể bị chặt nhầm.
func hasRepeatedBlock(s string, blockLen int) bool {
	if len(s) == 0 || len(s)%blockLen != 0 {
		return false
	}
	block := s[:blockLen]
	for i := blockLen; i < len(s); i += blockLen {
		if s[i:i+blockLen] != block {
			return false
		}
	}
	return true
}

// isSequentialDigits kiểm tra 10 chữ số có tạo thành dãy LIÊN TIẾP
// tăng đều (+1) hoặc giảm đều (-1) hay không.
//
//   0123456789 → tăng đều  ⇒ RÁC
//   0987654321 → giảm đều  ⇒ RÁC
//   0987654322 → không đều ⇒ hợp lệ (không chặn)
//
// ⚠️ Yêu cầu đều VÀO (mọi cặp kề nhau phải cùng bước) — nếu chỉ kiểm tra
//   "phần đầu tăng" thì số thật như 0981234567 sẽ bị chặn nhầm.
func isSequentialDigits(phone string) bool {
	if len(phone) != 10 {
		return false
	}
	digits := make([]int, 10)
	for i := 0; i < 10; i++ {
		digits[i] = int(phone[i] - '0')
	}

	// Bước +1 (tăng đều) — không cuộn vòng qua 9 → 0.
	if digits[9] == digits[0]+9 {
		for i := 1; i < 10; i++ {
			if digits[i] != digits[i-1]+1 {
				return false
			}
		}
		return true
	}

	// Bước -1 (giảm đều) — không cuộn vòng qua 0 → 9.
	if digits[0] == digits[9]+9 {
		for i := 1; i < 10; i++ {
			if digits[i] != digits[i-1]-1 {
				return false
			}
		}
		return true
	}

	return false
}

// IsTestLeadName kiểm tra tên khách có chứa marker "test" hay không.
//
// BỐI CẢNH (yêu cầu 2026-11-16):
//   Lead test do chính anh itdoanh tạo để kiểm tra ⇒ phải tự động đánh dấu
//   là lead test ⇒ KHÔNG tính vào biểu đồ / thông số (CR%, doanh thu...), NHƯNG
//   vẫn đồng bộ sang Google Sheets bình thường.
//
// NHẬN DIỆN (mở rộng 2026-11-16 sau khi anh test tên "test 1"):
//   Bất kỳ tên nào CHỨA chuỗi "test" (không phân biệt hoa thường) ⇒ lead test.
//   - "test"            ✅
//   - "test 1"          ✅  (trước đây SAI — chỉ khớp "(test)" có ngoặc)
//   - "Nguyễn (test)"   ✅
//   - "TEST"            ✅
//   - "Tester Nguyễn"   ✅  (chứa "test")
//   - "contest"         ✅  (chứa "test")
//
// ⚠️ Quy tắc CỐ Ý rộng: anh itdoanh xác nhận "chứa chuỗi test" là đủ.
//   Tên chứa "test" hiếm khi là khách thật ⇒ ưu tiên không làm nhiễu KPI.
func IsTestLeadName(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	return strings.Contains(strings.ToLower(n), "test")
}
