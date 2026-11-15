package db

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRewriteLandingIdentityHTML chứng minh nhân bản landing KHÔNG còn trỏ
// về landing nguồn ở bất kỳ "dấu vân tay" nào.
//
// Bối cảnh: bug anh itdoanh báo 2026-11-15 — `xu_huong_1` nhân bản từ
// `doanh_chung` nhưng vẫn có `data-landing-id="doanh_chung"` ⇒ mọi view/lead
// của bản sao bị ghi về landing nguồn ⇒ thống kê chồng chéo.
func TestRewriteLandingIdentityHTML(t *testing.T) {
	srcHTML := `<!DOCTYPE html>
<html>
<head>
<title>Test</title>
<link rel="canonical" href="https://lophoc.hanghoaphaisinh.net/">
<meta property="og:url" content="https://lophoc.hanghoaphaisinh.net/">
<script>window.APEX_LANDING_CONFIG = {"landing_id":"doanh_chung","nguon":"doanh_chung","name":"Buổi Chia Sẻ"};</script>
</head>
<body>
<!-- data-landing-id dùng bởi tracker.js để ghi đúng landing doanh_chung. -->
<script src="tracker.js" data-landing-id="doanh_chung" data-nguon="doanh_chung"></script>
</body>
</html>`

	out := RewriteLandingIdentityHTML(srcHTML, "xu_huong_1", "xu_huong_1", "lophoc.hanghoaphaisinh.net", "xuhuong.hanghoaphaisinh.net")

	// 1. `data-landing-id` phải trỏ về landing mới.
	if strings.Contains(out, `data-landing-id="doanh_chung"`) {
		t.Errorf("data-landing-id vẫn trỏ về landing nguồn:\n%s", out)
	}
	if !strings.Contains(out, `data-landing-id="xu_huong_1"`) {
		t.Errorf("thiếu data-landing-id mới:\n%s", out)
	}

	// 2. `data-nguon` phải trỏ về nguồn mới.
	if strings.Contains(out, `data-nguon="doanh_chung"`) {
		t.Errorf("data-nguon vẫn trỏ về nguồn của landing nguồn:\n%s", out)
	}
	if !strings.Contains(out, `data-nguon="xu_huong_1"`) {
		t.Errorf("thiếu data-nguon mới:\n%s", out)
	}

	// 3. `window.APEX_LANDING_CONFIG` phải có landing_id/nguon mới.
	idx := strings.Index(out, "window.APEX_LANDING_CONFIG")
	if idx < 0 {
		t.Fatalf("mất window.APEX_LANDING_CONFIG:\n%s", out)
	}
	objStart := strings.Index(out[idx:], "{")
	objEnd := strings.Index(out[idx+objStart:], "}") + objStart
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(out[idx+objStart:idx+objEnd+1]), &cfg); err != nil {
		t.Fatalf("APEX_LANDING_CONFIG không parse được: %v", err)
	}
	if cfg["landing_id"] != "xu_huong_1" {
		t.Errorf("landing_id trong APEX_LANDING_CONFIG = %v, mong đợi xu_huong_1", cfg["landing_id"])
	}
	if cfg["nguon"] != "xu_huong_1" {
		t.Errorf("nguon trong APEX_LANDING_CONFIG = %v, mong đợi xu_huong_1", cfg["nguon"])
	}
	// Các key khác phải được giữ nguyên.
	if cfg["name"] != "Buổi Chia Sẻ" {
		t.Errorf("mất key 'name' trong APEX_LANDING_CONFIG: %v", cfg)
	}

	// 4. URL domain cũ phải được thay bằng domain mới.
	if strings.Contains(out, "lophoc.hanghoaphaisinh.net") {
		t.Errorf("còn sót URL domain của landing nguồn:\n%s", out)
	}
	if !strings.Contains(out, "xuhuong.hanghoaphaisinh.net") {
		t.Errorf("chưa thay URL sang domain mới:\n%s", out)
	}
}

// TestRewriteLandingIdentityHTML_NoSourceDomain giữ nguyên URL khi không có
// domain đích (bản sao chưa Deploy).
func TestRewriteLandingIdentityHTML_NoSourceDomain(t *testing.T) {
	srcHTML := `<meta property="og:url" content="https://lophoc.hanghoaphaisinh.net/">
<script src="tracker.js" data-landing-id="doanh_chung" data-nguon="doanh_chung"></script>`

	out := RewriteLandingIdentityHTML(srcHTML, "xu_huong_1", "xu_huong_1", "lophoc.hanghoaphaisinh.net", "")

	if !strings.Contains(out, "lophoc.hanghoaphaisinh.net") {
		t.Errorf("không được xoá domain nguồn khi chưa có domain đích:\n%s", out)
	}
	if !strings.Contains(out, `data-landing-id="xu_huong_1"`) {
		t.Errorf("thiếu data-landing-id mới:\n%s", out)
	}
}

// TestRewriteLandingConfigJSON chứng minh `config.json` của bản sao có
// landing_id + nguon riêng.
func TestRewriteLandingConfigJSON(t *testing.T) {
	src := `{
  "landing_id": "doanh_chung",
  "nguon": "doanh_chung",
  "title": "Tối Ưu Dòng Tiền",
  "hotline": "0984386538"
}`

	out := RewriteLandingConfigJSON(src, "xu_huong_1", "xu_huong_1")

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("config.json không parse được: %v", err)
	}
	if cfg["landing_id"] != "xu_huong_1" {
		t.Errorf("landing_id = %v, mong đợi xu_huong_1", cfg["landing_id"])
	}
	if cfg["nguon"] != "xu_huong_1" {
		t.Errorf("nguon = %v, mong đợi xu_huong_1", cfg["nguon"])
	}
	// Các key khác phải giữ nguyên.
	if cfg["hotline"] != "0984386538" {
		t.Errorf("mất key 'hotline': %v", cfg)
	}
}

// TestRewriteLandingConfigJSON_KeepCustomNguon đảm bảo `nguon` do user cấu
// hình (không phải mặc định) KHÔNG bị đổi khi chuẩn hoá landing đang chạy.
func TestRewriteLandingConfigJSON_KeepCustomNguon(t *testing.T) {
	src := `{"landing_id":"doanh_chung","nguon":"DTN 1"}`

	out := RewriteLandingConfigJSON(src, "doanh_chung", "")

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("config.json không parse được: %v", err)
	}
	if cfg["nguon"] != "DTN 1" {
		t.Errorf("nguon tuần chỉnh bị đổi: %v", cfg["nguon"])
	}
}

// TestValidateLandingIdentity chứng minh page_id không hợp lệ bị chặn.
func TestValidateLandingIdentity(t *testing.T) {
	cases := []struct {
		name      string
		pageID    string
		nguon     string
		wantError bool
	}{
		{"hợp lệ", "xu_huong_1", "xu_huong_1", false},
		{"rỗng", "", "", true},
		{"có khoảng trắng", "xu hung 1", "", true},
		{"có dấu gạch chéo", "a/b", "", true},
		{"quá dài", strings.Repeat("a", 101), "", true},
	}
	for _, c := range cases {
		err := ValidateLandingIdentity(c.pageID, c.nguon)
		if c.wantError && err == nil {
			t.Errorf("%s: mong đợi có lỗi, nhưng nhận nil", c.name)
		}
		if !c.wantError && err != nil {
			t.Errorf("%s: mong đợi không lỗi, nhưng nhận: %v", c.name, err)
		}
	}
}

// TestRewriteAPEXConfigObject_NestedBrace chứng minh hàm xử lý được object
// JSON có nested object (regex đơn giản sẽ hỏng ở đây).
func TestRewriteAPEXConfigObject_NestedBrace(t *testing.T) {
	src := `<script>window.APEX_LANDING_CONFIG = {"landing_id":"doanh_chung","settings":{"form_config":{"require_phone":true},"a":{"b":{"c":1}}},"nguon":"doanh_chung"};</script>`

	out := rewriteAPEXConfigObject(src, "xu_huong_1", "xu_huong_1")

	idx := strings.Index(out, "{")
	end := strings.Index(out[idx:], "}") + idx
	// Tìm `}` cuối cùng của object (thay vì `}` đầu tiên).
	end = strings.LastIndex(out, "}")
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(out[idx:end+1]), &cfg); err != nil {
		t.Fatalf("object nested không parse được: %v\n%s", err, out)
	}
	if cfg["landing_id"] != "xu_huong_1" {
		t.Errorf("landing_id = %v, mong đợi xu_huong_1", cfg["landing_id"])
	}
	if cfg["nguon"] != "xu_huong_1" {
		t.Errorf("nguon = %v, mong đợi xu_huong_1", cfg["nguon"])
	}
	settings, ok := cfg["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("mất nested 'settings': %v", cfg)
	}
	fc, ok := settings["form_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("mất nested 'form_config': %v", settings)
	}
	if fc["require_phone"] != true {
		t.Errorf("mất nested 'require_phone': %v", fc)
	}
}
