package handlers

import (
	"os"
	"strings"
	"testing"

	"apex-backend/models"
)

// TestRenderRawLandingNoDuplicatePixel — chạy đúng pipeline render thật
// (`InjectDynamicScripts`) trên HTML gốc của `landing_cu` lấy từ CSDL
// production và kiểm tra Meta Pixel chỉ được init + track PageView đúng 1 lần.
//
// HTML gốc được lưu tạm ở `backend/tests/_tmp_raw_landing.html`.
// Nếu file không tồn tại ⇒ test SKIP (không phụ thuộc mạng/DB).
func TestRenderRawLandingNoDuplicatePixel(t *testing.T) {
	raw, err := os.ReadFile("../tests/_tmp_raw_landing.html")
	if err != nil {
		t.Skipf("không có HTML gốc của landing để test: %v", err)
	}
	html := string(raw)

	if !strings.Contains(html, "fbq('init'") {
		t.Skip("HTML gốc không chứa Meta Pixel ⇒ không áp dụng test này")
	}

	p := &models.LandingPage{
		PageID:       "landing_cu",
		Domain:       "ads.giaodichhanghoatuonglai.com",
		Name:         "Landing cũ",
		NguonDefault: "landing_chinh",
		RedirectURL:  "ads.giaodichhanghoatuonglai.com/chucmung",
		Settings: map[string]interface{}{
			"facebook_pixel_id": "1558828029597854",
			"form_config": map[string]interface{}{
				"success_redirect": "ads.giaodichhanghoatuonglai.com/chucmung",
			},
		},
	}

	h := &LandingHandler{}
	out := h.InjectDynamicScripts(html, p)

	if n := strings.Count(out, "fbq('init'"); n != 1 {
		t.Fatalf("fbq('init') phải xuất hiện đúng 1 lần, thực tế = %d", n)
	}
	if n := strings.Count(out, "fbq('track', 'PageView')"); n != 1 {
		t.Fatalf("PageView phải được track đúng 1 lần, thực tế = %d", n)
	}
	if strings.Contains(out, "ads.giaodichhanghoatuonglai.com/chucmung") {
		t.Fatal("URL chuyển hướng sau đăng ký vẫn chứa domain ⇒ có thể quay lại trang có form")
	}
}
