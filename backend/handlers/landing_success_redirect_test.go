package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"apex-backend/models"
)

// TestNormalizeSuccessRedirectValue: URL chuyển hướng chứa domain nhúng phải
// được chuẩn hoá để KHÔNG quay lại trang có form (⇒ lead trùng SĐT).
func TestNormalizeSuccessRedirectValue(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ads.giaodichhanghoatuonglai.com/chucmung", "chucmung.html"},
		{"chucmung.html", "chucmung.html"},
		{"chucmung", "chucmung.html"},
		{"https://www.giaodichhanghoatuonglai.com/camon", "https://www.giaodichhanghoatuonglai.com/camon"},
		{"http://example.com/x", "http://example.com/x"},
		{"", ""},
		{"ads.giaodichhanghoatuonglai.com/landing/chucmung", "landing/chucmung.html"},
	}

	for _, c := range cases {
		got := normalizeSuccessRedirectValue(c.in)
		if got != c.want {
			t.Fatalf("normalizeSuccessRedirectValue(%q) = %q, mong đợi %q", c.in, got, c.want)
		}
	}
}

// TestInjectDynamicScriptsNormalizesRedirect: config inject xuống client phải
// có `success_redirect`/`redirect_url` đã chuẩn hoá (không chứa domain).
func TestInjectDynamicScriptsNormalizesRedirect(t *testing.T) {
	raw := `<!DOCTYPE html><html><head><title>T</title></head><body></body></html>`

	p := &models.LandingPage{
		PageID:      "landing_cu",
		Domain:      "ads.giaodichhanghoatuonglai.com",
		RedirectURL: "ads.giaodichhanghoatuonglai.com/chucmung",
		Settings: map[string]interface{}{
			"form_config": map[string]interface{}{
				"require_name":     true,
				"require_phone":    true,
				"success_redirect": "ads.giaodichhanghoatuonglai.com/chucmung",
			},
		},
	}

	h := &LandingHandler{}
	out := h.InjectDynamicScripts(raw, p)

	if strings.Contains(out, "ads.giaodichhanghoatuonglai.com/chucmung") {
		t.Fatalf("config inject xuống client vẫn chứa domain trong URL chuyển hướng:\n%s", out)
	}
	if !strings.Contains(out, "chucmung.html") {
		t.Fatal("phải có success_redirect dạng 'chucmung.html'")
	}

	// Parse lại config để chắc chắn JSON hợp lệ
	idx := strings.Index(out, "var __apexForce = ")
	if idx == -1 {
		t.Fatal("không tìm thấy config trap")
	}
	rest := out[idx+len("var __apexForce = "):]
	end := strings.Index(rest, ";")
	if end == -1 {
		t.Fatal("không đóng được config trap")
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rest[:end])), &cfg); err != nil {
		t.Fatalf("config JSON không hợp lệ: %v", err)
	}
	if cfg["redirect_url"] != "chucmung.html" {
		t.Fatalf("redirect_url = %v, mong đợi 'chucmung.html'", cfg["redirect_url"])
	}
	fc, _ := cfg["form_config"].(map[string]interface{})
	if fc == nil || fc["success_redirect"] != "chucmung.html" {
		t.Fatalf("form_config.success_redirect = %v, mong đợi 'chucmung.html'", fc)
	}
	// Không được mất các trường khác của form_config.
	if fc["require_name"] != true || fc["require_phone"] != true {
		t.Fatalf("form_config bị mất trường: %v", fc)
	}
}

// TestPublicConfigNormalizesRedirect: `/api/landing/config` (endpoint public
// mà tracker.js gọi) cũng phải trả `success_redirect`/`redirect_url` đã chuẩn
// hoá — nếu không, client tải tracker.js cũ vẫn quay lại trang có form.
func TestPublicConfigNormalizesRedirect(t *testing.T) {
	p := &models.LandingPage{
		PageID:      "landing_cu",
		Domain:      "ads.giaodichhanghoatuonglai.com",
		RedirectURL: "ads.giaodichhanghoatuonglai.com/chucmung",
		Settings: map[string]interface{}{
			"form_config": map[string]interface{}{
				"success_redirect": "ads.giaodichhanghoatuonglai.com/chucmung",
			},
		},
	}

	// `normalizeSuccessRedirectValue` là hàm dùng chung cho cả render HTML
	// lẫn public config — kiểm tra trực tiếp hành vi chuẩn hoá.
	if got := normalizeSuccessRedirectValue(p.RedirectURL); got != "chucmung.html" {
		t.Fatalf("redirect_url chuẩn hoá = %q, mong đợi 'chucmung.html'", got)
	}
	fc := p.Settings["form_config"].(map[string]interface{})
	if got := normalizeSuccessRedirectValue(fc["success_redirect"].(string)); got != "chucmung.html" {
		t.Fatalf("form_config.success_redirect chuẩn hoá = %q, mong đợi 'chucmung.html'", got)
	}
}
