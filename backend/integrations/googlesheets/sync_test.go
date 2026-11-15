package googlesheets

import (
	"testing"

	"apex-backend/models"
)

// leadMẫu dựng 1 lead mẫu kèm tracking đầy đủ (mô phỏng lead thật 2026-11-15).
func leadMẫu() (*models.Lead, map[string]interface{}) {
	lead := &models.Lead{
		ID:         1234,
		CreatedAt:  "2026-11-15 10:20:30",
		UpdatedAt:  "2026-11-15 11:00:00",
		LandingID:  "landing_cu",
		Nguon:      "facebook",
		Phone:      "+84912345678",
		TrangThai:  models.StatusDaHocChuyenSau,
		GhiChu:     "Khách hỏi thêm lịch học",
		AssignedTo: "Nguyễn Văn A",
		City:       "Hồ Chí Minh",
		Region:     "Miền Nam",
		Country:    "Việt Nam",
		ISP:        "VNPT Telecom",
		AdPlatform: "Meta Ads",
		AdPlacement: "Facebook_Reels",
		URL:        "https://ads.example.com/landing?utm_source=fb",
	}

	tracking := map[string]interface{}{
		"fullName":        "Tran Thi B",
		"carrier":         "Vinaphone",
		"server_ip":       "113.161.72.45",
		"country_code":    "VN",
		"region_name":     "Hồ Chí Minh",
		"ad_source_tag":   "landing_chinh",
		"device_brand":    "Samsung",
		"device_model":    "SM-A525F",
		"device_name":     "Galaxy A52",
		"browser_engine":  "Blink",
		"os":              "Android",
		"screen_res":      "384x851",
		"clicks_count":    3,
		"visitor_id":      "v_abc123",
		"is_new_visitor":  true,
		"scroll_depth_50": true,
		"has_zalo_click":  true,
		"referrer_host":   "l.facebook.com",
		"landing_path":    "/landing",
		"entry_url":       "https://ads.example.com/landing",
		"autofill_source": "browser_autofill",
		"validation_passed": true,
		"form_focus_ms":   12000,
		"is_proxy":        true,
		"is_bot":          false,
		"bot_score":       12,
		"event_id":        "apex-uuid-1",
		"server_received_at": "2026-11-15 10:20:29",
	}
	return lead, tracking
}

// ─── Trường CRM mới ────────────────────────────────────────────────────

func TestExtract_TrangThaiLabel_Stage_CAPI(t *testing.T) {
	lead, tracking := leadMẫu()

	if got := ExtractLeadValue("trang_thai_label", "", lead, tracking); got != "Đã học chuyên sâu" {
		t.Fatalf("trang_thai_label: got %v", got)
	}
	if got := ExtractLeadValue("trang_thai_stage", "", lead, tracking); got != "paid" {
		t.Fatalf("trang_thai_stage: got %v", got)
	}
	// `da_hoc_chuyen_sau` ⇒ Subscribe (có giá trị)
	if got := ExtractLeadValue("capi_event", "", lead, tracking); got != "Subscribe" {
		t.Fatalf("capi_event: got %v", got)
	}
	if got := ExtractLeadValue("capi_has_value", "", lead, tracking); got != "Có" {
		t.Fatalf("capi_has_value: got %v", got)
	}
}

// `ket_ban_zalo` KHÔNG sinh sự kiện Meta ⇒ capi_event rỗng.
func TestExtract_CAPIEvent_KhongBung_ChoTrangThaiKhongBung(t *testing.T) {
	lead, tracking := leadMẫu()
	lead.TrangThai = models.StatusKetBanZalo

	if got := ExtractLeadValue("capi_event", "", lead, tracking); got != "" {
		t.Fatalf("capi_event (ket_ban_zalo) phải rỗng, got %v", got)
	}
	if got := ExtractLeadValue("trang_thai_stage", "", lead, tracking); got != "contacted" {
		t.Fatalf("trang_thai_stage: got %v", got)
	}
}

// Trạng thái cũ (`chot`) phải được chuẩn hoá về `da_mua_hang` ⇒ Purchase.
func TestExtract_CAPIEvent_ChuanHoaTrangThaiCu(t *testing.T) {
	lead, tracking := leadMẫu()
	lead.TrangThai = models.StatusChot

	if got := ExtractLeadValue("capi_event", "", lead, tracking); got != "Purchase" {
		t.Fatalf("capi_event (chot) phải = Purchase, got %v", got)
	}
}

// ─── Địa lý ────────────────────────────────────────────────────────────

func TestExtract_DiaLy_TinhGoc_ISO2(t *testing.T) {
	lead, tracking := leadMẫu()

	if got := ExtractLeadValue("city", "", lead, tracking); got != "Hồ Chí Minh" {
		t.Fatalf("city: got %v", got)
	}
	if got := ExtractLeadValue("region_name", "", lead, tracking); got != "Hồ Chí Minh" {
		t.Fatalf("region_name: got %v", got)
	}
	if got := ExtractLeadValue("region", "", lead, tracking); got != "Miền Nam" {
		t.Fatalf("region: got %v", got)
	}
	if got := ExtractLeadValue("country_code", "", lead, tracking); got != "VN" {
		t.Fatalf("country_code: got %v", got)
	}
	if got := ExtractLeadValue("isp", "", lead, tracking); got != "VNPT Telecom" {
		t.Fatalf("isp: got %v", got)
	}
}

// Lead cũ (trước 2026-11-15) chưa có `country_code` ⇒ phải suy ra ISO-2 từ tên nước.
func TestExtract_CountryCode_Fallback_TuTenNuoc(t *testing.T) {
	lead, tracking := leadMẫu()
	delete(tracking, "country_code")

	if got := ExtractLeadValue("country_code", "", lead, tracking); got != "VN" {
		t.Fatalf("country_code fallback: got %v", got)
	}
}

// ─── Trường văn bản / số ──────────────────────────────────────────────

func TestExtract_ThongTinLead(t *testing.T) {
	lead, tracking := leadMẫu()

	if got := ExtractLeadValue("lead_id", "", lead, tracking); got != "1234" {
		t.Fatalf("lead_id: got %v", got)
	}
	if got := ExtractLeadValue("nguoi_phu_trach", "", lead, tracking); got != "Nguyễn Văn A" {
		t.Fatalf("nguoi_phu_trach: got %v", got)
	}
	if got := ExtractLeadValue("cap_nhat_luc", "", lead, tracking); got != "2026-11-15 11:00:00" {
		t.Fatalf("cap_nhat_luc: got %v", got)
	}
	if got := ExtractLeadValue("ad_platform", "", lead, tracking); got != "Meta Ads" {
		t.Fatalf("ad_platform: got %v", got)
	}
	if got := ExtractLeadValue("ad_placement", "", lead, tracking); got != "Facebook_Reels" {
		t.Fatalf("ad_placement: got %v", got)
	}
	if got := ExtractLeadValue("ad_source_tag", "", lead, tracking); got != "landing_chinh" {
		t.Fatalf("ad_source_tag: got %v", got)
	}
	if got := ExtractLeadValue("server_received_at", "", lead, tracking); got != "2026-11-15 10:20:29" {
		t.Fatalf("server_received_at: got %v", got)
	}
}

// ─── Cờ boolean ────────────────────────────────────────────────────────

func TestExtract_BooleanFlags(t *testing.T) {
	lead, tracking := leadMẫu()

	if got := ExtractLeadValue("is_new_visitor", "", lead, tracking); got != "Khách Mới" {
		t.Fatalf("is_new_visitor: got %v", got)
	}
	if got := ExtractLeadValue("scroll_depth_50", "", lead, tracking); got != "Có" {
		t.Fatalf("scroll_depth_50: got %v", got)
	}
	if got := ExtractLeadValue("has_zalo_click", "", lead, tracking); got != "Có" {
		t.Fatalf("has_zalo_click: got %v", got)
	}
	if got := ExtractLeadValue("is_proxy", "", lead, tracking); got != "Có" {
		t.Fatalf("is_proxy: got %v", got)
	}
	if got := ExtractLeadValue("is_bot", "", lead, tracking); got != "Người Thật" {
		t.Fatalf("is_bot: got %v", got)
	}
	if got := ExtractLeadValue("validation_passed", "", lead, tracking); got != "Có" {
		t.Fatalf("validation_passed: got %v", got)
	}
}

// Key không tồn tại ⇒ trả về nhãn "false" (không khẳng định client có hành vi đó).
func TestExtract_Boolean_MissingKey_TranFalseLabel(t *testing.T) {
	lead, tracking := leadMẫu()
	delete(tracking, "has_tel_click")

	if got := ExtractLeadValue("has_tel_click", "", lead, tracking); got != "Không" {
		t.Fatalf("has_tel_click (thiếu key): got %v", got)
	}
}

// ─── Danh mục trường ───────────────────────────────────────────────────

// Mỗi trường trong danh mục phải có 1 case trong `ExtractLeadValue` — nếu
// không, giá trị sẽ rơi vào nhánh `default` (vẫn trả đúng nếu key có trong
// tracking, nhưng KHÔNG có fallback cột lead).
func TestExtract_MoiTruongTrongDanhMucPhaiCoFallback(t *testing.T) {
	lead, _ := leadMẫu()

	// Những trường KHÔNG nên rơi vào nhánh default (vì chúng nằm ở cột lead
	// hoặc cần chuyển đổi kiểu dữ liệu).
	cầnFallback := []string{
		"trang_thai_label", "trang_thai_stage", "capi_event", "capi_has_value",
		"city", "region_name", "region", "country", "country_code", "isp",
		"ad_platform", "ad_placement", "ad_source_tag",
		"lead_id", "nguoi_phu_trach", "cap_nhat_luc",
		"device_brand", "device_name", "device_model", "browser_engine", "os",
		"screen_res", "clicks_count", "visitor_id",
		"is_new_visitor", "scroll_depth_25", "scroll_depth_50", "scroll_depth_75",
		"scroll_depth_90", "scroll_depth_100",
		"has_video_play", "has_zalo_click", "has_tel_click",
		"autofill_source", "validation_passed", "form_focus_ms",
		"is_vpn", "is_proxy", "is_tor", "is_bot", "bot_score", "bot_reasons",
		"referrer_host", "landing_path", "entry_url", "server_received_at",
		"event_id", "igshid", "meta_click_id",
	}

	for _, key := range cầnFallback {
		if FieldGroup(key) == "" {
			t.Errorf("trường %q không có trong danh mục AvailableLeadFields()", key)
		}
	}

	// Kiểm tra thực thi: với tracking rỗng, mọi trường boolean phải trả về
	// nhãn "false" chứ không phải chuỗi rỗng (trừ capi_event).
	for _, key := range cầnFallback {
		switch key {
		case "capi_event":
			continue
		}
		v := ExtractLeadValue(key, "", lead, map[string]interface{}{})
		s, ok := v.(string)
		if !ok {
			t.Errorf("%q phải trả về string, got %T", key, v)
			continue
		}
		if s == "" && key != "referrer_host" && key != "landing_path" && key != "entry_url" &&
			key != "server_received_at" && key != "event_id" && key != "igshid" &&
			key != "meta_click_id" && key != "bot_score" && key != "bot_reasons" &&
			key != "device_brand" && key != "device_name" && key != "device_model" &&
			key != "browser_engine" && key != "os" && key != "screen_res" &&
			key != "clicks_count" && key != "visitor_id" && key != "autofill_source" &&
			key != "form_focus_ms" {
			t.Errorf("%q: kỳ vọng có giá trị fallback, got chuỗi rỗng", key)
		}
	}
}

// Bộ cột chuẩn phải khớp số trường trong danh mục và sinh đúng ký hiệu cột.
func TestGetStandardDefaultMappings_KhopDanhMuc(t *testing.T) {
	fields := AvailableLeadFields()
	mappings := GetStandardDefaultMappings()

	if len(mappings) != len(fields) {
		t.Fatalf("số cột chuẩn (%d) phải bằng số trường trong danh mục (%d)", len(mappings), len(fields))
	}

	kýHiệuKỳVọng := []string{"A", "B", "Z", "AA", "AB"}
	idxTheoKyHieu := map[string]int{"A": 0, "B": 1, "Z": 25, "AA": 26, "AB": 27}
	for _, want := range kýHiệuKỳVọng {
		idx := idxTheoKyHieu[want]
		if idx >= len(mappings) {
			continue
		}
		if mappings[idx].Column != want {
			t.Errorf("cột thứ %d: got %q, want %q", idx, mappings[idx].Column, want)
		}
	}

	for i, m := range mappings {
		if m.Field != fields[i].Key {
			t.Errorf("mapping[%d].Field = %q, want %q", i, m.Field, fields[i].Key)
		}
		if m.Header == "" {
			t.Errorf("mapping[%d] (%s) thiếu header", i, m.Field)
		}
	}
}

// Không được có 2 trùng key trong danh mục.
func TestAvailableLeadFields_KhongTrungKey(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range AvailableLeadFields() {
		if seen[f.Key] {
			t.Errorf("trùng key: %s", f.Key)
		}
		seen[f.Key] = true
		if f.Label == "" {
			t.Errorf("key %s thiếu label", f.Key)
		}
		if f.Group == "" {
			t.Errorf("key %s thiếu group", f.Key)
		}
	}
}

// ─── MergeStandardMappings ─────────────────────────────────────────────

// Bổ sung cột mới phải KHÔNG đụng vào cột cũ (thứ tự + tiêu đề giữ nguyên).
func TestMergeStandardMappings_GiuNguyenCotCu(t *testing.T) {
	existing := []models.FieldMapping{
		{Column: "A", Header: "Thời Gian", Field: "thoi_gian"},
		{Column: "B", Header: "Họ Và Tên", Field: "fullName"},
		{Column: "C", Header: "Số Điện Thoại", Field: "so_dien_thoai"},
	}
	merged, newHeaders := MergeStandardMappings(existing)

	if len(newHeaders) != len(AvailableLeadFields())-3 {
		t.Fatalf("số cột mới = %d, want %d", len(newHeaders), len(AvailableLeadFields())-3)
	}
	if merged[0].Header != "Thời Gian" || merged[1].Field != "fullName" {
		t.Fatalf("cột cũ bị đổi: %#v", merged[:3])
	}
	// Cột mới bắt đầu từ D (= index 3).
	if merged[3].Column != "D" {
		t.Fatalf("cột mới đầu tiên phải là D, got %q", merged[3].Column)
	}
	// Không trùng field giữa cột cũ và cột mới.
	seen := map[string]bool{}
	for _, m := range merged {
		if seen[m.Field] {
			t.Errorf("trùng field sau khi merge: %s", m.Field)
		}
		seen[m.Field] = true
	}
}

// Cấu hình đã đầy đủ ⇒ không thêm gì.
func TestMergeStandardMappings_KhongThemKhiDayDu(t *testing.T) {
	merged, newHeaders := MergeStandardMappings(GetStandardDefaultMappings())
	if len(newHeaders) != 0 {
		t.Fatalf("không được thêm cột, got %d", len(newHeaders))
	}
	if len(merged) != len(GetStandardDefaultMappings()) {
		t.Fatalf("số cột phải giữ nguyên")
	}
}

// Cột lạ (không còn trong danh mục) vẫn phải được giữ.
func TestMergeStandardMappings_GiuCotLa(t *testing.T) {
	existing := []models.FieldMapping{
		{Column: "A", Header: "Cột Cũ custom", Field: "field_bien_dong"},
	}
	merged, _ := MergeStandardMappings(existing)
	if merged[0].Field != "field_bien_dong" || merged[0].Header != "Cột Cũ custom" {
		t.Fatalf("cột lạ bị mất: %#v", merged[0])
	}
}
