package security

import "testing"

// TestIsJunkPhone_BlocksJunkPatterns: các mẫu SĐT rác phải BỊ CHẶN.
func TestIsJunkPhone_BlocksJunkPatterns(t *testing.T) {
	junk := []string{
		"0987654321", // giảm đều (mẫu anh itdoanh nêu)
		"0123456789", // tăng đều
		"1234567890", // tăng đều
		"0999999999", // lặp toàn bộ
		"0888888888",
		"0000000000",
		"0909090909", // lặp mẫu 2 số
		"0120120120",
		"0123401234", // lặp mẫu 5 số
		"0911111111", // 7 đuôi giống nhau
		"0988888888",
		"+84987654321",
		"098-765-4321",
	}

	for _, p := range junk {
		if ok, reason := IsJunkPhone(p); !ok {
			t.Errorf("SĐT rác %q phải bị chặn", p)
		} else if reason == "" {
			t.Errorf("SĐT rác %q phải có lý do", p)
		}
	}
}

// TestIsJunkPhone_AllowsRealNumbers: số THẬT của khách hàng KHÔNG được chặn.
//
// ⚠️ Đây là test quan trọng nhất — chặt nhầm số thật = mất lead thật.
func TestIsJunkPhone_AllowsRealNumbers(t *testing.T) {
	real := []string{
		"0912345678",
		"0981234567", // có "1234567" ở đuôi nhưng KHÔNG liên tiếp từ đầu
		"0903123456",
		"0356789123",
		"0968123456",
		"0891234567",
		"0977553322",
		"0912345678",
		"0987001122",
		"0869123456",
	}

	for _, p := range real {
		if ok, reason := IsJunkPhone(p); ok {
			t.Errorf("SĐT thật %q bị chặn nhầm (reason=%s)", p, reason)
		}
	}
}

// TestIsJunkPhone_ShortOrInvalid: số không đủ 10 chữ số không được chặn
// (để lớp validate SĐT chuẩn xử lý, trả thông báo rõ ràng cho khách).
func TestIsJunkPhone_ShortOrInvalid(t *testing.T) {
	for _, p := range []string{"", "098765432", "09876543210", "abc"} {
		if ok, _ := IsJunkPhone(p); ok {
			t.Errorf("SĐT không hợp lệ %q không nên bị coi là rác", p)
		}
	}
}

// TestIsTestLeadName: nhận diện lead test theo việc TÊN CHỨA chuỗi "test".
//
// ⚠️ Mở rộng 2026-11-16: trước đây chỉ khớp "(test)" CÓ DẤU NGOẶC ⇒ tên
// "test 1" bị bỏ sót ⇒ lead test vẫn được tính vào thống kê (bug anh
// itdoanh phát hiện). Nay bất kỳ tên nào chứa "test" đều là lead test.
func TestIsTestLeadName(t *testing.T) {
	yes := []string{
		"Nguyễn Văn A (test)",
		"nguyễn văn a (TEST)",
		"Test User (test)",
		"John ( Test )",
		"(test)",
		"test",          // trọn vẹn
		"test 1",        // không có ngoặc — bug cũ
		"TEST",
		"Test",
		"Tester Nguyễn", // chứa "test"
		"Phòng test",    // chứa "test"
		"contest",       // chứa "test"
		"  test  ",      // có khoảng trắng thừa
	}
	for _, n := range yes {
		if !IsTestLeadName(n) {
			t.Errorf("tên %q phải được đánh dấu là lead test", n)
		}
	}

	no := []string{
		"",
		"   ",
		"Nguyễn Văn A",
		"Lý A sèo",
		"Quan Vuvanquan",
		"Trần Phương Thảo",
	}
	for _, n := range no {
		if IsTestLeadName(n) {
			t.Errorf("tên %q KHÔNG được đánh dấu là lead test", n)
		}
	}
}
