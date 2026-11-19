package handlers

import (
	"strings"
	"testing"

	"apex-backend/models"
)

// ⚠️ META 2026-11-19 — Events Manager báo:
//   "100% sự kiện Lead trên trang web của bạn đang không gửi thông tin hợp lệ
//    về đơn vị tiền tệ" (thiếu thông số giá trị và đơn vị tiền tệ).
//
// Nguyên nhân: landing export từ LadiPage có custom_code hardcode
// `fbq('track', 'Lead')` — KHÔNG kèm `currency`/`value` ⇒ Meta báo lỗi.
//
// ⇒ Backend PHẢI ghi đè lệnh này thành `fbq('track','Lead',{currency:'VND',value:0})`.
// ⇒ LOẠI TRỪ `landing_cu` + `landing_lznoew` (trang chúc mừng) theo yêu cầu.

// TestRewriteLeadWithCurrencyValue_BareCall: lệnh Lead trần phải được bổ sung
// currency (VND) + value (0).
func TestRewriteLeadWithCurrencyValue_BareCall(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single quotes",
			in:   "fbq('track', 'Lead');",
			want: "fbq('track', 'Lead', {currency: 'VND', value: 0});",
		},
		{
			name: "double quotes",
			in:   `fbq("track", "Lead");`,
			want: `fbq("track", "Lead", {currency: 'VND', value: 0});`,
		},
		{
			name: "object rỗng",
			in:   "fbq('track', 'Lead', {});",
			want: "fbq('track', 'Lead', {currency: 'VND', value: 0});",
		},
		{
			name: "không có chấm phẩy",
			in:   "fbq('track', 'Lead')",
			want: "fbq('track', 'Lead', {currency: 'VND', value: 0})",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := rewriteLeadWithCurrencyValue(c.in)
			if out != c.want {
				t.Fatalf("phải bổ sung currency/value, thực tế = %q (mong đợi %q)", out, c.want)
			}
		})
	}
}

// TestRewriteLeadWithCurrencyValue_AlreadyHasCustomData: lệnh ĐÃ có custom_data
// (kể cả rỗng object khác) ⇒ giữ nguyên, KHÔNG nhân đôi tham số.
func TestRewriteLeadWithCurrencyValue_AlreadyHasCustomData(t *testing.T) {
	inputs := []string{
		"fbq('track', 'Lead', {currency: 'VND', value: 0});",
		"fbq('track', 'Lead', { content_name: 'x', currency: 'VND', value: 0 }, { eventID: 'e1' });",
	}
	for _, in := range inputs {
		out := rewriteLeadWithCurrencyValue(in)
		if out != in {
			t.Fatalf("lệnh đã có custom_data không được sửa, thực tế = %q", out)
		}
	}
}

// TestRewriteLeadWithCurrencyValue_KeepOtherEvents: KHÔNG được đụng sự kiện khác
// (vd PageView) khi thêm currency cho Lead.
func TestRewriteLeadWithCurrencyValue_KeepOtherEvents(t *testing.T) {
	in := "fbq('track', 'PageView');"
	out := rewriteLeadWithCurrencyValue(in)
	if out != in {
		t.Fatalf("sự kiện khác không được sửa, thực tế = %q", out)
	}
}

// TestInjectDynamicScripts_LeadCurrency_ExcludedLandings (fix 2026-11-19):
// `landing_cu` + `landing_lznoew` (trang chúc mừng) KHÔNG được bị thêm
// currency/value ⇒ giữ nguyên lệnh gốc.
func TestInjectDynamicScripts_LeadCurrency_ExcludedLandings(t *testing.T) {
	raw := `<!DOCTYPE html><html><head><title>T</title>
<script>
fbq('init', '1558828029597854');
fbq('track', 'Lead');
</script>
</head><body></body></html>`

	for _, pageID := range []string{"landing_cu", "landing_lznoew"} {
		t.Run(pageID, func(t *testing.T) {
			p := &models.LandingPage{
				PageID:   pageID,
				Settings: map[string]interface{}{"facebook_pixel_id": "1558828029597854"},
			}
			h := &LandingHandler{
				capiChecker: &stubCAPIChecker{active: map[string]bool{}},
			}
			out := h.InjectDynamicScripts(raw, p)

			if !strings.Contains(out, "fbq('track', 'Lead');") {
				t.Fatalf("%s phải giữ nguyên lệnh Lead gốc (không thêm currency):\n%s", pageID, out)
			}
			if strings.Contains(out, "currency: 'VND'") {
				t.Fatalf("%s KHÔNG được thêm currency/value:\n%s", pageID, out)
			}
		})
	}
}

// TestInjectDynamicScripts_LeadCurrency_Applied (fix 2026-11-19):
// landing BÌNH THƯỜNG ⇒ lệnh Lead phải có currency (VND) + value (0).
func TestInjectDynamicScripts_LeadCurrency_Applied(t *testing.T) {
	p := &models.LandingPage{
		PageID:   "xu_huong_2",
		Settings: map[string]interface{}{"facebook_pixel_id": "1558828029597854"},
	}

	raw := `<!DOCTYPE html><html><head><title>T</title>
<script>
fbq('init', '1558828029597854');
fbq('track', 'Lead');
</script>
</head><body></body></html>`

	h := &LandingHandler{
		capiChecker: &stubCAPIChecker{active: map[string]bool{}},
	}
	out := h.InjectDynamicScripts(raw, p)

	if !strings.Contains(out, "fbq('track', 'Lead', {currency: 'VND', value: 0})") {
		t.Fatalf("landing thường phải thêm currency/value cho Lead:\n%s", out)
	}
}
