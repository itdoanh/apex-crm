// TestTrackerNoDuplicatePixelInjection — kiểm tra tracker.js KHÔNG tự inject
// thêm Meta Pixel khi HTML đã có sẵn (fix 2026-11-06).
//
// Bối cảnh: `landing_cu` (xuất bản từ LadiPage) đã hardcode block Meta Pixel
// trong HTML ⇒ backend inject thêm 1 block nữa + tracker.js inject thêm 1 block
// nữa ⇒ `fbq('init')` và `fbq('track','PageView')` chạy nhiều lần ⇒ PageView
// nhân đôi ⇒ Meta Ads Manager đếm sai lượt hiển thị / quy ngữ quy đổi.
package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// đường dẫn tracker.js gốc trong repo
var trackerPaths = []string{
	"../../tracker.js",
	"../../frontend/landing/tracker.js",
	"../../backend/static/tracker.js",
	"../../backend/static/js/tracker.js",
	"../../frontend/admin/tracker.js",
	"../../frontend/admin/js/tracker.js",
}

// TestTrackerHasPixelGuard: tracker.js phải có hàm `htmlHasMetaPixel()` và
// điều kiện chặn inject pixel.
func TestTrackerHasPixelGuard(t *testing.T) {
	for _, p := range trackerPaths {
		data, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			t.Fatalf("không đọc được %s: %v", p, err)
		}
		s := string(data)

		if !strings.Contains(s, "function htmlHasMetaPixel()") {
			t.Fatalf("%s: thiếu hàm htmlHasMetaPixel()", p)
		}
		if !strings.Contains(s, "!htmlHasMetaPixel()") {
			t.Fatalf("%s: điều kiện chặn inject pixel chưa được áp dụng", p)
		}
	}
}

// TestTrackerMirrorsAreIdentical: 6 bản tracker.js phải giống hệt nhau
// (quy tắc mirror của repo — sửa 1 chỗ phải copy đủ).
func TestTrackerMirrorsAreIdentical(t *testing.T) {
	var first string
	var firstPath string
	for _, p := range trackerPaths {
		data, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			t.Fatalf("không đọc được %s: %v", p, err)
		}
		s := string(data)
		if firstPath == "" {
			first, firstPath = s, p
			continue
		}
		if s != first {
			t.Fatalf("%s khác %s — phải copy chung nội dung giống hệt", p, firstPath)
		}
	}
}

// TestTrackerNormalizeSuccessRedirect: tracker.js phải chuẩn hoá URL redirect
// sau đăng ký (bỏ domain nhúng, thêm .html) để không quay lại trang có form.
func TestTrackerNormalizeSuccessRedirect(t *testing.T) {
	for _, p := range trackerPaths {
		data, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			t.Fatalf("không đọc được %s: %v", p, err)
		}
		s := string(data)

		if !strings.Contains(s, "function normalizeSuccessRedirect(raw)") {
			t.Fatalf("%s: thiếu hàm normalizeSuccessRedirect()", p)
		}
		if !strings.Contains(s, "normalizeSuccessRedirect(") {
			t.Fatalf("%s: normalizeSuccessRedirect() chưa được dùng", p)
		}
	}
}
