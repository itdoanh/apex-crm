package handlers

import (
	"strings"
	"testing"

	"apex-backend/models"
)

// newTestLandingPage tạo nhanh 1 landing page tối thiểu cho unit test.
func newTestLandingPage(pageID string, domain string) *models.LandingPage {
	return &models.LandingPage{
		PageID:   pageID,
		Domain:   domain,
		Name:     pageID,
		IsActive: true,
		Status:   "published",
		Settings: map[string]interface{}{},
	}
}

// TestRewriteRelativeAssetPaths_AbsoluteScript:
// HTML do LadiPage export có `<base href="/landing/">` ĐẶT SAU thẻ
// `<script src="tracker.js">`. Trình duyệt sẽ phân giải `tracker.js` theo URL
// của DOCUMENT (không theo `<base>`) ⇒ `/p/tracker.js` ⇒ 404.
//
// ⇒ Server phải ghi đè thành đường dẫn TUYỆT ĐỐI theo basePath.
func TestRewriteRelativeAssetPaths_AbsoluteScript(t *testing.T) {
	html := `<!DOCTYPE html><html><head>
<script src="tracker.js" data-landing-id="landing_cu"></script>
<base href="/landing/">
</head><body></body></html>`

	out := rewriteRelativeAssetPaths(html, "/p/landing_cu")

	if !strings.Contains(out, `src="/p/landing_cu/tracker.js"`) {
		t.Fatalf("phải ghi đè tracker.js thành đường dẫn tuyệt đối, thực tế:\n%s", out)
	}
	if strings.Contains(out, `src="tracker.js"`) {
		t.Fatalf("không được còn đường dẫn tương đối, thực tế:\n%s", out)
	}
	// `data-landing-id` phải được giữ nguyên.
	if !strings.Contains(out, `data-landing-id="landing_cu"`) {
		t.Fatal("phải giữ nguyên attribute data-landing-id")
	}
}

// TestRewriteRelativeAssetPaths_RootPrefix: basePath = "/" ⇒ đường dẫn tuyệt
// đối gốc `/tracker.js` (tương đương đường dẫn tương đối khi phục vụ ở root).
func TestRewriteRelativeAssetPaths_RootPrefix(t *testing.T) {
	html := `<head><script src="tracker.js"></script></head>`

	out := rewriteRelativeAssetPaths(html, "/")

	if !strings.Contains(out, `src="/tracker.js"`) {
		t.Fatalf("basePath='/' phải cho đường dẫn tuyệt đối gốc, thực tế:\n%s", out)
	}
}

// TestRewriteRelativeAssetPaths_EmptyBasePath: không có basePath ⇒ không đổi gì.
func TestRewriteRelativeAssetPaths_EmptyBasePath(t *testing.T) {
	html := `<head><script src="tracker.js"></script></head>`

	out := rewriteRelativeAssetPaths(html, "")

	if out != html {
		t.Fatalf("basePath rỗng thì HTML phải giữ nguyên, thực tế:\n%s", out)
	}
}

// TestRewriteRelativeAssetPaths_KeepsAbsoluteURLs: URL NGOÀI domain (http://,
// https://, //cdn...) phải được giữ nguyên.
//
// LƯU Ý: `/tracker.js` (đường dẫn GỐC trong cùng domain) ĐƯỢC ghi đè thành
// `/p/landing_cu/tracker.js` — vì gốc có thể trỏ tới landing KHÁC (bug chồng
// chéo landing — 2026-11-05).
func TestRewriteRelativeAssetPaths_KeepsAbsoluteURLs(t *testing.T) {
	html := `<head>
<script src="https://cdn.example.com/tracker.js"></script>
<script src="//cdn.example.com/config.json"></script>
<script src="/tracker.js"></script>
</head>`

	out := rewriteRelativeAssetPaths(html, "/p/landing_cu")

	if !strings.Contains(out, `src="https://cdn.example.com/tracker.js"`) {
		t.Fatalf("URL tuyệt đối http(s) phải giữ nguyên, thực tế:\n%s", out)
	}
	if !strings.Contains(out, `src="//cdn.example.com/config.json"`) {
		t.Fatalf("URL protocol-relative phải giữ nguyên, thực tế:\n%s", out)
	}
	if !strings.Contains(out, `src="/p/landing_cu/tracker.js"`) {
		t.Fatalf("đường dẫn gốc phải được gắn prefix của landing, thực tế:\n%s", out)
	}
}

// TestRewriteRelativeAssetPaths_RootPathKeptAtRoot: basePath = "/" ⇒
// `/tracker.js` giữ nguyên (đã là đường dẫn tuyệt đối gốc hợp lệ).
func TestRewriteRelativeAssetPaths_RootPathKeptAtRoot(t *testing.T) {
	html := `<head><script src="/tracker.js"></script></head>`

	out := rewriteRelativeAssetPaths(html, "/")

	if !strings.Contains(out, `src="/tracker.js"`) {
		t.Fatalf("basePath='/' phải giữ nguyên đường dẫn gốc, thực tế:\n%s", out)
	}
	if strings.Contains(out, "//tracker.js") {
		t.Fatalf("không được sinh '//', thực tế:\n%s", out)
	}
}

// TestRewriteRelativeAssetPaths_ConfigJSON: `config.json` cũng phải được ghi
// đè vì tracker.js fallback tải nó bằng đường dẫn tương đối.
func TestRewriteRelativeAssetPaths_ConfigJSON(t *testing.T) {
	html := `<head><script src="config.json"></script></head>`

	out := rewriteRelativeAssetPaths(html, "/landing")

	if !strings.Contains(out, `src="/landing/config.json"`) {
		t.Fatalf("phải ghi đè config.json, thực tế:\n%s", out)
	}
}

// TestRewriteRelativeAssetPaths_NoTrailingSlash: basePath có dấu "/" ở cuối
// ⇒ không được sinh ra "//".
func TestRewriteRelativeAssetPaths_NoTrailingSlash(t *testing.T) {
	html := `<head><script src="tracker.js"></script></head>`

	out := rewriteRelativeAssetPaths(html, "/p/landing_cu/")

	if strings.Contains(out, "//tracker.js") {
		t.Fatalf("không được sinh '//' , thực tế:\n%s", out)
	}
	if !strings.Contains(out, `src="/p/landing_cu/tracker.js"`) {
		t.Fatalf("sai đường dẫn, thực tế:\n%s", out)
	}
}

// TestInjectDynamicScriptsWithBase_EndToEnd: kiểm tra đường đi đầy đủ từ
// InjectDynamicScripts → HTML cuối cùng phải trỏ đúng tuyệt đối VÀ vẫn giữ
// `data-landing-id` + cờ form guard.
func TestInjectDynamicScriptsWithBase_EndToEnd(t *testing.T) {
	h := &LandingHandler{}
	p := newTestLandingPage("landing_cu", "ads.giaodichhanghoatuonglai.com")

	html := `<!DOCTYPE html><html><head>
<script src="tracker.js"></script>
<base href="/landing/">
</head><body>
<form id="registrationForm" onsubmit="handleFormSubmit(event)"></form>
<script>window.ApexTracker.sendLeadToBackend({});</script>
</body></html>`

	out := h.InjectDynamicScriptsWithBase(html, p, "/p/landing_cu")

	if !strings.Contains(out, `src="/p/landing_cu/tracker.js"`) {
		t.Fatalf("tracker.js phải là đường dẫn tuyệt đối, thực tế:\n%s", out)
	}
	if !strings.Contains(out, `data-landing-id="landing_cu"`) {
		t.Fatal("phải gắn data-landing-id")
	}
	if !strings.Contains(out, "window.__REGISTRATION_FORM_HANDLED__ = true") {
		t.Fatal("phải giữ cờ form guard")
	}
}
