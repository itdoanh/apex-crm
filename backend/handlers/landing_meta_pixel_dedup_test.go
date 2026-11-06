package handlers

import (
	"strings"
	"testing"

	"apex-backend/models"
)

// TestHtmlHasMetaPixel_HardcodedPixel: HTML export từ LadiPage đã có sẵn
// fbq('init') + fbq('track','PageView') ⇒ backend KHÔNG được inject thêm
// (tránh PageView nhân đôi ⇒ Meta Ads Manager nhận diện sai).
func TestHtmlHasMetaPixel_HardcodedPixel(t *testing.T) {
	html := `<html><head>
<script>
!function(f,b,e,v,n,t,s){...}(window, document,'script','https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '1558828029597854');
fbq('track', 'PageView');
</script>
</head><body></body></html>`

	if !htmlHasMetaPixel(html, "1558828029597854") {
		t.Fatal("HTML có fbevents.js + fbq('init') ⇒ phải nhận diện là đã có pixel")
	}
}

// TestHtmlHasMetaPixel_DifferentID: HTML đã tự quản lý 1 pixel khác ⇒ cũng
// không được chèn thêm pixel thứ 2 (2 pixel cùng chạy cũng nhân đôi PageView).
func TestHtmlHasMetaPixel_DifferentID(t *testing.T) {
	html := `<script>fbq('init', '999888777');</script>`
	if !htmlHasMetaPixel(html, "1558828029597854") {
		t.Fatal("HTML đã có fbq('init') với ID khác ⇒ không được inject thêm")
	}
}

// TestHtmlHasMetaPixel_NoPixel: HTML sạch ⇒ phải inject như cũ.
func TestHtmlHasMetaPixel_NoPixel(t *testing.T) {
	html := `<html><head><title>Không có pixel</title></head><body></body></html>`
	if htmlHasMetaPixel(html, "1558828029597854") {
		t.Fatal("HTML không có pixel ⇒ phải inject")
	}
}

// TestInjectDynamicScripts_NoDuplicatePixel: xác nhận đầu ra render KHÔNG chứa
// 2 lần `fbq('init')` khi HTML đã hardcode pixel.
func TestInjectDynamicScripts_NoDuplicatePixel(t *testing.T) {
	raw := `<!DOCTYPE html><html><head><title>T</title></head><body>
<script>
!function(f,b,e,v,n,t,s){...}(window, document,'script','https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '1558828029597854');
fbq('track', 'PageView');
</script>
</body></html>`

	p := &models.LandingPage{
		PageID: "landing_cu",
		Domain: "ads.giaodichhanghoatuonglai.com",
		Settings: map[string]interface{}{
			"facebook_pixel_id": "1558828029597854",
		},
	}

	h := &LandingHandler{}
	out := h.InjectDynamicScripts(raw, p)

	initCount := strings.Count(out, "fbq('init'")
	if initCount != 1 {
		t.Fatalf("HTML đã có pixel ⇒ sau khi render phải còn đúng 1 lần fbq('init'), thực tế = %d", initCount)
	}
	pvCount := strings.Count(out, "fbq('track', 'PageView')")
	if pvCount != 1 {
		t.Fatalf("PageView phải được bắn đúng 1 lần, thực tế = %d", pvCount)
	}
}

// TestInjectDynamicScripts_StillInjectsWhenNoPixel: landing chưa có pixel ⇒
// backend vẫn phải inject như trước (không làm hỏng landing mới).
func TestInjectDynamicScripts_StillInjectsWhenNoPixel(t *testing.T) {
	raw := `<!DOCTYPE html><html><head><title>T</title></head><body></body></html>`

	p := &models.LandingPage{
		PageID:   "landing_moi",
		Domain:   "example.com",
		Settings: map[string]interface{}{"facebook_pixel_id": "111222333"},
	}

	h := &LandingHandler{}
	out := h.InjectDynamicScripts(raw, p)

	if !strings.Contains(out, "fbq('init', '111222333')") {
		t.Fatal("landing chưa có pixel ⇒ backend phải inject Meta Pixel")
	}
	if !strings.Contains(out, "fbq('track', 'PageView')") {
		t.Fatal("phải bắn PageView khi inject pixel")
	}
}
