package handlers

import (
	"strings"
	"testing"

	"apex-backend/models"
)

// stubCAPIChecker là CAPIStatusChecker giả để test logic render mà không cần DB.
type stubCAPIChecker struct {
	active map[string]bool
}

func (s *stubCAPIChecker) IsCAPIActiveForLanding(landingID string) bool {
	if s == nil {
		return false
	}
	return s.active[landingID]
}

// TestRewritePageViewWithEventID_NoEventID: lệnh PageView hardcode trong HTML
// landing (export từ LadiPage) phải được ghi đè để kèm eventID (tham số thứ 4
// của fbq) ⇒ Meta gộp được với sự kiện server CAPI.
func TestRewritePageViewWithEventID_NoEventID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single quotes",
			in:   "fbq('track', 'PageView');",
			want: "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID });",
		},
		{
			name: "double quotes",
			in:   `fbq("track", "PageView");`,
			want: "fbq(\"track\", \"PageView\", {}, { eventID: window.APEX_PV_EVENT_ID });",
		},
		{
			name: "không có chấm phẩy",
			in:   "fbq('track', 'PageView')",
			want: "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID })",
		},
		{
			name: "đã có object rỗng",
			in:   "fbq('track', 'PageView', {});",
			want: "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID });",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := rewritePageViewWithEventID(c.in)
			if !strings.Contains(out, "eventID: window.APEX_PV_EVENT_ID") {
				t.Fatalf("phải thêm eventID, thực tế = %q", out)
			}
			if !strings.Contains(out, "PageView") {
				t.Fatalf("phải giữ PageView, thực tế = %q", out)
			}
			// eventID phải là tham số THỨ 4 (sau object tham số 3).
			if !strings.Contains(out, ", {}, { eventID:") {
				t.Fatalf("eventID phải là tham số thứ 4, thực tế = %q", out)
			}
		})
	}
}

// TestRewritePageViewWithEventID_AlreadyHasEventID: lệnh ĐÃ có eventID phải
// được giữ nguyên (không nhân đôi).
func TestRewritePageViewWithEventID_AlreadyHasEventID(t *testing.T) {
	in := "fbq('track', 'PageView', {}, { eventID: pvEventId });"
	out := rewritePageViewWithEventID(in)
	if out != in {
		t.Fatalf("lệnh đã đúng không được sửa, thực tế = %q", out)
	}
}

// TestRewritePageViewWithEventID_OtherEvents: KHÔNG được đụng vào sự kiện khác.
func TestRewritePageViewWithEventID_OtherEvents(t *testing.T) {
	in := "fbq('track', 'Lead', { content_name: 'x' }, { eventID: 'e1' });"
	out := rewritePageViewWithEventID(in)
	if out != in {
		t.Fatalf("sự kiện khác không được sửa, thực tế = %q", out)
	}
}

// TestInjectPageViewEventIDSnippet_Idempotent: chỉ sinh 1 giá trị event ID.
//
// ⚠️ Landing BẬT CAPI server ⇒ phải gắn `eventID` để Meta gộp sự kiện
// browser (Pixel) với sự kiện server (CAPI).
func TestInjectPageViewEventIDSnippet_Idempotent(t *testing.T) {
	p := &models.LandingPage{
		PageID:   "landing_cu",
		Domain:   "ads.giaodichhanghoatuonglai.com",
		Settings: map[string]interface{}{"facebook_pixel_id": "1558828029597854"},
	}

	raw := `<!DOCTYPE html><html><head><title>T</title>
<script>
fbq('init', '1558828029597854');
fbq('track', 'PageView');
</script>
</head><body></body></html>`

	h := &LandingHandler{
		capiChecker: &stubCAPIChecker{active: map[string]bool{"landing_cu": true}},
	}
	out := h.InjectDynamicScripts(raw, p)

	// Chỉ có 1 chỗ SINH giá trị event_id.
	if n := strings.Count(out, "window.APEX_PV_EVENT_ID ="); n != 1 {
		t.Fatalf("chỉ được sinh 1 event_id, thực tế = %d", n)
	}
	// Chỉ có 1 lệnh PageView.
	if n := strings.Count(out, "fbq('track', 'PageView'"); n != 1 {
		t.Fatalf("chỉ được có 1 lệnh PageView, thực tế = %d", n)
	}
	// Lệnh PageView phải kèm eventID.
	if !strings.Contains(out, "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID })") {
		t.Fatalf("PageView phải kèm eventID:\n%s", out)
	}
	// Script sinh event_id phải nằm TRƯỚC lệnh PageView (để fbq chạy sau khi
	// giá trị đã sẵn sàng).
	idxGen := strings.Index(out, "window.APEX_PV_EVENT_ID =")
	idxPV := strings.Index(out, "fbq('track', 'PageView'")
	if idxGen == -1 || idxPV == -1 || idxGen > idxPV {
		t.Fatalf("script sinh event_id phải nằm TRƯỚC lệnh PageView (gen=%d, pv=%d)", idxGen, idxPV)
	}
}

// TestRewritePageViewWithoutEventID: lệnh PageView ĐÃ có eventID phải được gỡ
// khi landing KHÔNG bật CAPI ⇒ HTML về đúng block Meta Pixel GỐC.
func TestRewritePageViewWithoutEventID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single quotes",
			in:   "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID });",
			want: "fbq('track', 'PageView');",
		},
		{
			name: "double quotes",
			in:   `fbq("track", "PageView", {}, { eventID: window.APEX_PV_EVENT_ID });`,
			want: `fbq("track", "PageView");`,
		},
		{
			name: "không có chấm phẩy",
			in:   "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID })",
			want: "fbq('track', 'PageView')",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := rewritePageViewWithoutEventID(c.in)
			if out != c.want {
				t.Fatalf("phải gỡ eventID, thực tế = %q (mong đợi %q)", out, c.want)
			}
		})
	}
}

// TestRewritePageViewWithoutEventID_KeepOtherEvents: KHÔNG được đụng sự kiện khác
// (vd `fbq('track','Lead', …, { eventID: '…' })`).
func TestRewritePageViewWithoutEventID_KeepOtherEvents(t *testing.T) {
	in := "fbq('track', 'Lead', { content_name: 'x' }, { eventID: 'e1' });"
	out := rewritePageViewWithoutEventID(in)
	if out != in {
		t.Fatalf("sự kiện khác không được sửa, thực tế = %q", out)
	}
}

// TestInjectDynamicScripts_NoCAPI_NoEventID (fix 2026-11-18 — landing_cu):
// landing CHỈ dùng Meta Pixel nhúng (KHÔNG bật CAPI server) ⇒ KHÔNG được
// gắn `eventID` và KHÔNG được chèn script sinh event_id.
//
// ⇒ HTML giữ NGUYÊN block Meta Pixel GỐC, đúng như thiết kế ban đầu.
// ⇒ Quan trọng: chỉ cần server bắn CAPI PageView mới cần `eventID`; nếu
//    gắn `eventID` mà server KHÔNG bắn ⇒ Meta ghi nhận PageView thiếu
//    thông số ⇒ chiến dịch báo lỗi dữ liệu.
func TestInjectDynamicScripts_NoCAPI_NoEventID(t *testing.T) {
	p := &models.LandingPage{
		PageID:   "landing_cu",
		Domain:   "ads.giaodichhanghoatuonglai.com",
		Settings: map[string]interface{}{"facebook_pixel_id": "1558828029597854"},
	}

	raw := `<!DOCTYPE html><html><head><title>T</title>
<script>
!function(f,b,e,v,n,t,s){if(f.fbq)return;n=f.fbq=function(){n.callMethod?
n.callMethod.apply(n,arguments):n.queue.push(arguments)};if(!f._fbq)f._fbq=n;
n.push=n;n.loaded=!0;n.version='2.0';n.queue=[];t=b.createElement(e);t.async=!0;
t.src=v;s=b.getElementsByTagName(e)[0];s.parentNode.insertBefore(t,s)}(window,
document,'script','https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '1558828029597854');
fbq('track', 'PageView');
</script>
</head><body></body></html>`

	h := &LandingHandler{
		capiChecker: &stubCAPIChecker{active: map[string]bool{}},
	}
	out := h.InjectDynamicScripts(raw, p)

	// KHÔNG được sinh event_id.
	if strings.Contains(out, "window.APEX_PV_EVENT_ID") {
		t.Fatalf("landing KHÔNG bật CAPI ⇒ không được gắn eventID:\n%s", out)
	}
	// PageView phải giữ NGUYÊN dạng gốc.
	if !strings.Contains(out, "fbq('track', 'PageView');") {
		t.Fatalf("PageView phải giữ nguyên dạng gốc (không eventID):\n%s", out)
	}
	// Vẫn phải có đúng 1 lệnh PageView + 1 lệnh init.
	if n := strings.Count(out, "fbq('track', 'PageView'"); n != 1 {
		t.Fatalf("phải có đúng 1 lệnh PageView, thực tế = %d", n)
	}
	if n := strings.Count(out, "fbq('init'"); n != 1 {
		t.Fatalf("phải có đúng 1 lệnh fbq('init'), thực tế = %d", n)
	}
}

// TestInjectDynamicScripts_NoCAPI_RemovesEventIDFromTemplate (fix 2026-11-18):
// landing KHÔNG bật CAPI nhưng HTML template ĐÃ có `eventID` (export LadiPage)
// ⇒ backend phải GỠ `eventID` ⇒ HTML về đúng block Meta Pixel GỐC.
func TestInjectDynamicScripts_NoCAPI_RemovesEventIDFromTemplate(t *testing.T) {
	p := &models.LandingPage{
		PageID:   "landing_cu",
		Domain:   "ads.giaodichhanghoatuonglai.com",
		Settings: map[string]interface{}{"facebook_pixel_id": "1558828029597854"},
	}

	raw := `<!DOCTYPE html><html><head><title>T</title>
<script>
window.APEX_PV_EVENT_ID = window.APEX_PV_EVENT_ID || ('pv_' + Date.now());
fbq('init', '1558828029597854');
fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID });
</script>
</head><body></body></html>`

	h := &LandingHandler{
		capiChecker: &stubCAPIChecker{active: map[string]bool{}},
	}
	out := h.InjectDynamicScripts(raw, p)

	if strings.Contains(out, "eventID: window.APEX_PV_EVENT_ID") {
		t.Fatalf("landing KHÔNG bật CAPI ⇒ phải gỡ eventID khỏi PageView:\n%s", out)
	}
	if !strings.Contains(out, "fbq('track', 'PageView')") {
		t.Fatalf("PageView phải giữ nguyên dạng gốc:\n%s", out)
	}
}
