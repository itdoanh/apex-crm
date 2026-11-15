package facebook

import (
	"testing"

	"apex-backend/models"
)

// ─── Chuẩn hoá PII theo chuẩn Meta ─────────────────────────────────────

func TestNormalizeCountry_ISO2(t *testing.T) {
	cases := map[string]string{
		"Việt Nam":      "vn",
		"Vietnam":       "vn",
		"VN":            "vn",
		"vn":            "vn",
		"vi-VN":         "vn",
		"United States": "united states",
	}
	for in, want := range cases {
		if got := NormalizeCountry(in); got != want {
			t.Errorf("NormalizeCountry(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripVietnameseDiacritics(t *testing.T) {
	cases := map[string]string{
		"Hà Nội":             "Ha Noi",
		"Hồ Chí Minh":        "Ho Chi Minh",
		"Đắk Lắk":            "Dak Lak",
		"Quận Năm":           "Quan Nam",
		"Thành Phố Tây Ninh": "Thanh Pho Tay Ninh",
		"Bắc Ninh":           "Bac Ninh",
		"Ha Noi":             "Ha Noi",
	}
	for in, want := range cases {
		if got := StripVietnameseDiacritics(in); got != want {
			t.Errorf("StripVietnameseDiacritics(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeCity(t *testing.T) {
	if got := NormalizeCity("Hồ Chí Minh"); got != "ho chi minh" {
		t.Errorf("NormalizeCity = %q, want %q", got, "ho chi minh")
	}
}

// `st` phải BỎ QUA khi là vùng kinh tế (Meta không có khái niệm vùng).
func TestBuildState_SkipEconomicRegion(t *testing.T) {
	for _, v := range []string{"Miền Bắc", "Miền Nam", "Miền Trung", "Quốc Tế"} {
		if got := BuildState(v); got != "" {
			t.Errorf("BuildState(%q) = %q, want rỗng (vùng kinh tế không phải tỉnh)", v, got)
		}
	}
	if got := BuildState("Hồ Chí Minh"); got != "ho chi minh" {
		t.Errorf("BuildState(Hồ Chí Minh) = %q, want %q", got, "ho chi minh")
	}
}

// ─── Gating: chỉ gửi CAPI cho lead từ Facebook ─────────────────────────

func TestDetectFacebookTraffic(t *testing.T) {
	cases := []struct {
		name     string
		tracking map[string]interface{}
		wantFB   bool
	}{
		{"có fbc", map[string]interface{}{"fbc": "fb.1.1700000000.abc"}, true},
		{"có fbclid", map[string]interface{}{"fbclid": "IwZXh0bgNhZW0"}, true},
		{"utm_source fb", map[string]interface{}{"utm_source": "fb"}, true},
		{"site_source_name ig", map[string]interface{}{"site_source_name": "ig"}, true},
		{"chỉ fbp", map[string]interface{}{"fbp": "fb.1.1700000000.123"}, true},
		{"google ads", map[string]interface{}{"utm_source": "google", "utm_medium": "cpc"}, false},
		{"tiktok", map[string]interface{}{"utm_source": "tiktok"}, false},
		{"organic truy cập thẳng", map[string]interface{}{"utm_source": "organic"}, false},
		{"rỗng", map[string]interface{}{}, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		got := DetectFacebookTraffic(c.tracking)
		if got.IsFacebook != c.wantFB {
			t.Errorf("%s: IsFacebook = %v, want %v (source=%s)", c.name, got.IsFacebook, c.wantFB, got.Source)
		}
	}
}

// ─── Mapping trạng thái CRM → sự kiện Meta ────────────────────────────

func TestLeadStatusCAPIEvent(t *testing.T) {
	cases := []struct {
		status    string
		wantEvent string
		wantValue bool
		wantOK    bool
	}{
		{"moi", "Lead", false, true},
		{"da_goi", "Contact", false, true},
		{"da_hoc_buoi_chia_se", "StartTrial", false, true},
		{"da_hoc_chuyen_sau", "Subscribe", true, true},
		{"da_mua_hang", "Purchase", true, true},
		{"ket_ban_zalo", "", false, false},
		{"spam_khong_lien_lac", "", false, false},
		{"khong_quan_tam", "", false, false},
		{"huy", "", false, false},
		{"da_hoc_truoc_do", "", false, false},
	}
	for _, c := range cases {
		ev, hasVal, ok := models.LeadStatusCAPIEvent(c.status)
		if ok != c.wantOK || ev != c.wantEvent || hasVal != c.wantValue {
			t.Errorf("%s: got (%q,%v,%v), want (%q,%v,%v)",
				c.status, ev, hasVal, ok, c.wantEvent, c.wantValue, c.wantOK)
		}
	}
}

// Tương thích ngược: giá trị cũ vẫn map đúng.
func TestLeadStatusCAPIEvent_LegacyValues(t *testing.T) {
	if ev, _, ok := models.LeadStatusCAPIEvent("chot"); !ok || ev != "Purchase" {
		t.Errorf("chot phải map sang Purchase, got (%q,%v)", ev, ok)
	}
	if ev, _, ok := models.LeadStatusCAPIEvent("da_vao_zoom"); !ok || ev != "StartTrial" {
		t.Errorf("da_vao_zoom phải map sang StartTrial, got (%q,%v)", ev, ok)
	}
}

// ─── BuildUserData: country phải là ISO-2 ─────────────────────────────

func TestBuildUserData_CountryISO2(t *testing.T) {
	cfg := &models.FacebookCAPIConfig{
		SendUserData:   true,
		HashPhone:      true,
		HashName:       true,
		HashCity:       true,
		HashState:      true,
		HashCountry:    true,
		HashExternalID: true,
	}
	ud := BuildUserData(cfg, UserDataInput{
		Phone:      "0912345678",
		FirstName:  "Nguyễn",
		LastName:   "Văn A",
		City:       "Hà Nội",
		State:      "Miền Bắc",
		Country:    "Việt Nam",
		ExternalID: "0912345678",
	})
	if ud == nil {
		t.Fatal("user_data rỗng")
	}
	// country phải băm giá trị ISO-2 "vn"
	want := HashSHA256("vn")
	if len(ud.Country) != 1 || ud.Country[0] != want {
		t.Errorf("country = %v, want [%s]", ud.Country, want)
	}
	// ct phải băm "ha noi"
	wantCT := HashSHA256("ha noi")
	if len(ud.CT) != 1 || ud.CT[0] != wantCT {
		t.Errorf("ct = %v, want [%s]", ud.CT, wantCT)
	}
	// st phải RỖNG (vùng kinh tế không phải tỉnh)
	if len(ud.ST) != 0 {
		t.Errorf("st phải rỗng vì 'Miền Bắc' là vùng kinh tế, got %v", ud.ST)
	}
	// ph phải băm E.164
	wantPH := HashSHA256("+84912345678")
	if len(ud.Ph) != 1 || ud.Ph[0] != wantPH {
		t.Errorf("ph = %v, want [%s]", ud.Ph, wantPH)
	}
}
