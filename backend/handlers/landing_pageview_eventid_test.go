package handlers

import (
	"strings"
	"testing"

	"apex-backend/models"
)

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

	h := &LandingHandler{}
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
