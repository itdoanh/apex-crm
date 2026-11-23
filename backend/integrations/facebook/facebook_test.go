package facebook

import (
	"encoding/json"
	"strings"
	"testing"

	"apex-backend/models"
)

// ─── Hashing / chuẩn hoá PII ────────────────────────────────────────────

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"  NGUYEN@Example.COM ":    "nguyen@example.com",
		"Nguyen.Van.A@Example.com": "nguyen.van.a@example.com",
		"":                         "",
	}
	for in, want := range cases {
		if got := NormalizeEmail(in); got != want {
			t.Fatalf("NormalizeEmail(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		// Số VN local (10 số) → E.164
		"0912 345 678":    "+84912345678",
		"0912.345.678":    "+84912345678",
		"0912345678":      "+84912345678",
		// Đã có mã quốc gia
		"+84 912-345-678": "+84912345678",
		"84912345678":     "+84912345678",
		// Quốc tế khác — giữ nguyên
		"+1 202 555 0147": "+12025550147",
		"":                "",
		"abc":             "",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Fatalf("NormalizePhone(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	if got := NormalizeText("  Nguyễn   Văn  A  "); got != "nguyễn văn a" {
		t.Fatalf("NormalizeText sai: %q", got)
	}
	if got := NormalizeText("   "); got != "" {
		t.Fatalf("NormalizeText rỗng sai: %q", got)
	}
}

func TestNormalizeCountry(t *testing.T) {
	cases := map[string]string{
		"VN":    "vn",
		"vn":    "vn",
		"VN-VN": "vn",
		" VI ":  "vi",
	}
	for in, want := range cases {
		if got := NormalizeCountry(in); got != want {
			t.Fatalf("NormalizeCountry(%q) = %q, muốn %q", in, got, want)
		}
	}
}

func TestHashSHA256LengthAndStability(t *testing.T) {
	h := HashSHA256("nguyen@example.com")
	if len(h) != 64 {
		t.Fatalf("hash SHA-256 phải dài 64, thực tế %d", len(h))
	}
	if h != HashSHA256("nguyen@example.com") {
		t.Fatal("hash phải ổn định")
	}
	if h == HashSHA256("nguyen@example.co") {
		t.Fatal("hash khác input phải khác nhau")
	}
}

func TestHashEmailPhoneEmpty(t *testing.T) {
	if HashEmail("  ") != "" {
		t.Fatal("email rỗng phải trả chuỗi rỗng")
	}
	if HashPhone("") != "" {
		t.Fatal("sđt rỗng phải trả chuỗi rỗng")
	}
	if HashText("") != "" {
		t.Fatal("text rỗng phải trả chuỗi rỗng")
	}
}

func TestHashExternalIDUsesPhoneNormalization(t *testing.T) {
	a := HashExternalID("0912 345 678")
	b := HashExternalID("0912345678")
	if a != b {
		t.Fatal("external_id phải băm theo chuẩn hoá sđt (trailing spaces / dấu gạch)")
	}
}

// ─── Parse URL Meta Ads ──────────────────────────────────────────────────

const sampleAdsURL = "https://lophoc.hanghoaphaisinh.net/?utm_medium=paid&utm_source=fb" +
	"&utm_placement=Facebook_Mobile_Reels&utm_campaign=120253099537400306" +
	"&utm_content=120253099537410306&utm_term=120253099537420306" +
	"&campaign_name=Anh+1&adset_name=Anh&ad_name=%E1%BA%A2nh+-+Chung" +
	"&fbclid=IwZXh0bgNhZW0BMABwZG9mAWFkaWQBqzmOy7-XMnNydGMGYXBwX2lk" +
	"&utm_id=120253099537400306"

func TestParseMetaAdsURLFull(t *testing.T) {
	p := ParseMetaAdsURL(sampleAdsURL)

	if p.Source != "fb" {
		t.Fatalf("utm_source = %q", p.Source)
	}
	if p.Medium != "paid" {
		t.Fatalf("utm_medium = %q", p.Medium)
	}
	if p.Placement != "Facebook_Mobile_Reels" {
		t.Fatalf("utm_placement = %q", p.Placement)
	}
	// Ánh xạ quan trọng: utm_campaign = campaign.id, utm_content = adset.id, utm_term = ad.id
	if p.CampaignID != "120253099537400306" {
		t.Fatalf("campaign_id = %q", p.CampaignID)
	}
	if p.AdSetID != "120253099537410306" {
		t.Fatalf("adset_id = %q", p.AdSetID)
	}
	if p.AdID != "120253099537420306" {
		t.Fatalf("ad_id = %q", p.AdID)
	}
	if p.CampaignName != "Anh 1" {
		t.Fatalf("campaign_name = %q", p.CampaignName)
	}
	if p.AdSetName != "Anh" {
		t.Fatalf("adset_name = %q", p.AdSetName)
	}
	if p.AdName != "Ảnh - Chung" {
		t.Fatalf("ad_name = %q", p.AdName)
	}
	if p.Fbclid == "" {
		t.Fatal("thiếu fbclid")
	}
	if !p.HasCampaign() {
		t.Fatal("HasCampaign phải true")
	}
}

func TestParseMetaAdsURLQueryStringOnly(t *testing.T) {
	p := ParseMetaAdsURL("?utm_medium=paid&utm_source=facebook&utm_campaign=111")
	if p.CampaignID != "111" {
		t.Fatalf("campaign_id = %q", p.CampaignID)
	}
	if p.Source != "facebook" {
		t.Fatalf("utm_source = %q", p.Source)
	}
}

func TestParseMetaAdsURLEmpty(t *testing.T) {
	p := ParseMetaAdsURL("")
	if p.CampaignID != "" || p.HasCampaign() {
		t.Fatal("URL rỗng phải cho thông số rỗng")
	}
}

func TestParseMetaAdsURLFallbackUTMID(t *testing.T) {
	p := ParseMetaAdsURL("?utm_id=999&site_source_name=instagram")
	if p.CampaignID != "999" {
		t.Fatalf("campaign_id fallback utm_id = %q", p.CampaignID)
	}
	if p.SiteSourceName != "instagram" {
		t.Fatalf("site_source_name = %q", p.SiteSourceName)
	}
}

func TestDisplayFallbackToID(t *testing.T) {
	p := MetaAdsParams{CampaignID: "1", AdSetID: "2", AdID: "3"}
	if p.DisplayCampaign() != "1" || p.DisplayAdSet() != "2" || p.DisplayAd() != "3" {
		t.Fatal("phải fallback về ID khi không có tên")
	}
	p2 := MetaAdsParams{CampaignID: "1", CampaignName: "Tên"}
	if p2.DisplayCampaign() != "Tên" {
		t.Fatal("phải ưu tiên tên hiển thị")
	}
}

func TestNormalizeFBC(t *testing.T) {
	// fbclid thô → dựng thành fbc chuẩn (timestamp = thời điểm hiện tại, ms)
	got := NormalizeFBC("", "IwZXh0bgNhZW0")
	if !strings.HasPrefix(got, "fb.1.") || !strings.HasSuffix(got, ".IwZXh0bgNhZW0") {
		t.Fatalf("fbc sai: %q", got)
	}
	// fbc đã đúng định dạng → giữ nguyên
	keep := "fb.1.1700000000.IwZXh0bgNhZW0"
	if NormalizeFBC(keep, "") != keep {
		t.Fatal("fbc đúng định dạng phải giữ nguyên")
	}
	// fbc quá ngắn → coi như fbclid
	got2 := NormalizeFBC("abc123", "")
	if !strings.HasPrefix(got2, "fb.1.") || !strings.HasSuffix(got2, ".abc123") {
		t.Fatalf("fbc dựng lại sai: %q", got2)
	}
	if NormalizeFBC("", "") != "" {
		t.Fatal("fbc rỗng phải trả rỗng")
	}
}

func TestNormalizeFBP(t *testing.T) {
	if NormalizeFBP("abc") != "fb.1.0.abc" {
		t.Fatal("fbp phải được chuẩn hoá")
	}
	keep := "fb.1.1700000000.123456"
	if NormalizeFBP(keep) != keep {
		t.Fatal("fbp đúng định dạng phải giữ nguyên")
	}
}

// ─── Build event / user_data ────────────────────────────────────────────

func allHashOnConfig() *models.FacebookCAPIConfig {
	return &models.FacebookCAPIConfig{
		SendUserData:   true,
		HashEmail:      true,
		HashPhone:      true,
		HashName:       true,
		HashAddress:    true,
		HashState:      true,
		HashZip:        true,
		HashCountry:    true,
		HashExternalID: true,
	}
}

func TestBuildUserDataAllHashed(t *testing.T) {
	ud := BuildUserData(allHashOnConfig(), UserDataInput{
		Email:      "NGUYEN@example.com",
		Phone:      "0912 345 678",
		FirstName:  "Văn",
		LastName:   "Nguyễn",
		City:       "Hà Nội",
		State:      "Hà Nội",
		Zip:        "100000",
		Country:    "VN",
		ExternalID: "0912345678",
	})
	if ud == nil {
		t.Fatal("user_data không được nil")
	}
	if len(ud.Em) != 1 || ud.Em[0] != HashEmail("nguyen@example.com") {
		t.Fatalf("em sai: %v", ud.Em)
	}
	if len(ud.Ph) != 1 || ud.Ph[0] != HashPhone("0912345678") {
		t.Fatalf("ph sai: %v", ud.Ph)
	}
	if len(ud.Country) != 1 || ud.Country[0] != HashCountry("vn") {
		t.Fatalf("country sai: %v", ud.Country)
	}
	// Mọi PII đều phải là hash 64 ký tự.
	for name, arr := range map[string][]string{
		"em": ud.Em, "ph": ud.Ph, "fn": ud.Fn, "ln": ud.Ln,
		"ct": ud.CT, "st": ud.ST, "zp": ud.Zp, "country": ud.Country,
		"external_id": ud.ExternalID,
	} {
		for _, h := range arr {
			if len(h) != 64 {
				t.Fatalf("%s phải là SHA-256 64 ký tự, thực tế %q", name, h)
			}
		}
	}
}

func TestBuildUserDataDisabled(t *testing.T) {
	cfg := allHashOnConfig()
	cfg.SendUserData = false
	if ud := BuildUserData(cfg, UserDataInput{Email: "a@b.c"}); ud != nil {
		t.Fatal("khi tắt send_user_data thì user_data phải nil")
	}
}

func TestBuildUserDataEmptyReturnsNil(t *testing.T) {
	if ud := BuildUserData(allHashOnConfig(), UserDataInput{}); ud != nil {
		t.Fatal("không có PII nào thì user_data phải nil (tránh gửi khối rỗng)")
	}
}

func TestBuildEventDefaults(t *testing.T) {
	ev := BuildEvent(EventOptions{})
	if ev.EventName != EventLead {
		t.Fatalf("event_name mặc định phải là Lead, thực tế %q", ev.EventName)
	}
	if ev.ActionSource != "website" {
		t.Fatalf("action_source phải là website, thực tế %q", ev.ActionSource)
	}
	if ev.EventID == "" {
		t.Fatal("event_id phải được tự sinh")
	}
	if ev.EventTime <= 0 {
		t.Fatal("event_time phải > 0")
	}
}

func TestBuildEventKeepsClientEventID(t *testing.T) {
	// event_id phải giữ nguyên id client gửi lên để dedup browser ↔ server.
	ev := BuildEvent(EventOptions{EventID: "  abc-123  "})
	if ev.EventID != "abc-123" {
		t.Fatalf("event_id = %q", ev.EventID)
	}
}

func TestGenerateEventIDFormat(t *testing.T) {
	id := GenerateEventID()
	if len(id) != 36 {
		t.Fatalf("event_id phải dài 36 (UUID), thực tế %d", len(id))
	}
	if id == GenerateEventID() {
		t.Fatal("event_id phải khác nhau mỗi lần")
	}
}

func TestBuildValueCustomData(t *testing.T) {
	cd := BuildValueCustomData(6000000, "vnd", "Khoá học chuyên sâu", "course")
	if cd == nil || cd.Value == nil {
		t.Fatal("custom_data phải có value")
	}
	if *cd.Value != 6000000 {
		t.Fatalf("value = %v", *cd.Value)
	}
	if cd.Currency != "VND" {
		t.Fatalf("currency = %q (phải uppercase)", cd.Currency)
	}
	// currency mặc định khi bỏ trống
	if BuildValueCustomData(1, "", "", "").Currency != "VND" {
		t.Fatal("currency mặc định phải là VND")
	}
}

func TestSplitFullName(t *testing.T) {
	fn, ln := SplitFullName("Nguyễn Văn A")
	if fn != "Nguyễn Văn" || ln != "A" {
		t.Fatalf("tách tên sai: %q / %q", fn, ln)
	}
	fn2, ln2 := SplitFullName("A")
	if fn2 != "A" || ln2 != "" {
		t.Fatalf("tách 1 từ sai: %q / %q", fn2, ln2)
	}
	if f, l := SplitFullName("   "); f != "" || l != "" {
		t.Fatal("tách chuỗi rỗng phải rỗng")
	}
}

// ─── Endpoint Graph API / Gateway ───────────────────────────────────────

func TestBuildEventURLDirect(t *testing.T) {
	got := BuildEventURL("PIX123", "v27.0", false, "", "")
	want := "https://graph.facebook.com/v27.0/PIX123/events"
	if got != want {
		t.Fatalf("URL = %q, muốn %q", got, want)
	}
}

func TestBuildEventURLAddsVPrefix(t *testing.T) {
	got := BuildEventURL("PIX123", "27.0", false, "", "")
	if !strings.Contains(got, "/v27.0/") {
		t.Fatalf("URL phải tự thêm 'v': %q", got)
	}
}

func TestBuildEventURLDefaultVersion(t *testing.T) {
	got := BuildEventURL("PIX123", "", false, "", "")
	if !strings.Contains(got, DefaultAPIVersion) {
		t.Fatalf("URL phải dùng version mặc định %s: %q", DefaultAPIVersion, got)
	}
}

func TestBuildEventURLGateway(t *testing.T) {
	got := BuildEventURL("PIX123", "v27.0", true, "https://apac.gateway.facebook.com", "KEY123")
	if !strings.HasPrefix(got, "https://apac.gateway.facebook.com") {
		t.Fatalf("gateway URL sai: %q", got)
	}
	if !strings.Contains(got, "access_key=KEY123") {
		t.Fatalf("gateway URL phải có access_key: %q", got)
	}
}

func TestBuildEventURLGatewayCustomEventsPath(t *testing.T) {
	got := BuildEventURL("PIX123", "v27.0", true, "https://apac.gateway.facebook.com/events", "KEY123")
	if !strings.Contains(got, "/events?") {
		t.Fatalf("gateway path /events phải được giữ: %q", got)
	}
}

// ─── Payload JSON ───────────────────────────────────────────────────────

func TestCAPIRequestJSONShape(t *testing.T) {
	ev := BuildEvent(EventOptions{
		EventName: EventPurchase,
		EventID:   "e-1",
		UserData: &UserData{
			Em:              []string{HashEmail("a@b.c")},
			ClientIPAddress: "1.2.3.4",
			ClientUserAgent: "UA",
			FBC:             "fb.1.0.x",
			FBP:             "fb.1.0.y",
		},
		CustomData: BuildValueCustomData(6000000, "VND", "Khoá", "course"),
	})
	b, err := json.Marshal(&CAPIRequest{Data: []CAPIEvent{ev}, PartnerAgent: PartnerAgentName})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"event_name":"Purchase"`,
		`"event_id":"e-1"`,
		`"action_source":"website"`,
		`"client_ip_address":"1.2.3.4"`,
		`"client_user_agent":"UA"`,
		`"fbc":"fb.1.0.x"`,
		`"fbp":"fb.1.0.y"`,
		`"currency":"VND"`,
		`"value":6000000`,
		`"partner_agent":"apex_crm"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("payload thiếu %s\n%s", want, s)
		}
	}
}

// ─── Dataset Quality ────────────────────────────────────────────────────
//
// 📌 Meta v27.0 trả `web` là MẢNG: mỗi phần tử = 1 `event_name` kèm toàn bộ
// chỉ số riêng. Test bên dưới mô phỏng đúng format đó.

func TestParseDatasetQuality(t *testing.T) {
	raw := map[string]interface{}{
		"web": []interface{}{
			map[string]interface{}{
				"event_name": "Purchase",
				"event_match_quality": map[string]interface{}{
					"composite_score": 7.2,
					"match_key_feedback": []interface{}{
						map[string]interface{}{
							"identifier": "email",
							"coverage":   map[string]interface{}{"percentage": 100.0},
						},
					},
					"diagnostics": []interface{}{
						map[string]interface{}{
							"name":                 "Server sending mismatched IP addresses",
							"percentage":           61.5,
							"affected_event_count": 19567,
							"total_event_count":    31830,
						},
					},
				},
				"event_coverage": map[string]interface{}{
					"percentage":      34.1,
					"goal_percentage": 75.0,
				},
				"acr":            map[string]interface{}{"percentage": 37.9},
				"data_freshness": map[string]interface{}{"upload_frequency": "real_time"},
				"event_spend":    map[string]interface{}{"amount": 108947.38, "currency_code": "VND"},
				"dedupe_key_feedback": []interface{}{
					map[string]interface{}{
						"dedupe_key":                               "event_id",
						"browser_events_with_dedupe_key":           map[string]interface{}{"percentage": 100.0},
						"server_events_with_dedupe_key":            map[string]interface{}{"percentage": 100.0},
						"overall_browser_coverage_from_dedupe_key": map[string]interface{}{"percentage": 14.8},
					},
				},
				"value_diagnostics": []interface{}{
					map[string]interface{}{
						"issue_type":  "MISSING_OR_INVALID_VALUE",
						"issue_count": 250,
						"event_count": 1000,
						"issue_rate":  0.25,
					},
				},
				"event_delay": map[string]interface{}{
					"events_delayed_over_2h":  100,
					"events_delayed_over_24h": 10,
					"total_events":            1000,
					"pct_delayed_over_24h":    1.0,
				},
			},
			map[string]interface{}{
				"event_name":          "PageView",
				"event_match_quality": map[string]interface{}{"composite_score": 5.5},
			},
		},
	}

	out := ParseDatasetQuality(raw, "PIX123")

	if len(out.Events) != 2 {
		t.Fatalf("phải có 2 event, thực tế %d", len(out.Events))
	}
	if out.Events[0].EventName != "Purchase" {
		t.Fatalf("event_name sai: %q", out.Events[0].EventName)
	}
	if out.Events[0].EMQ == nil || out.Events[0].EMQ.CompositeScore != 7.2 {
		t.Fatalf("EMQ composite_score sai: %+v", out.Events[0].EMQ)
	}
	if len(out.Events[0].EMQ.MatchKeyFeedback) != 1 ||
		out.Events[0].EMQ.MatchKeyFeedback[0].Identifier != "email" {
		t.Fatalf("match_key_feedback sai: %+v", out.Events[0].EMQ.MatchKeyFeedback)
	}
	if len(out.Events[0].EMQ.Diagnostics) != 1 ||
		out.Events[0].EMQ.Diagnostics[0].AffectedEventCount != 19567 {
		t.Fatalf("diagnostics sai: %+v", out.Events[0].EMQ.Diagnostics)
	}
	if out.Events[0].Coverage == nil || out.Events[0].Coverage.Percentage != 34.1 {
		t.Fatalf("event_coverage sai: %+v", out.Events[0].Coverage)
	}
	if out.Events[0].ACR == nil || out.Events[0].ACR.Percentage != 37.9 {
		t.Fatalf("acr sai: %+v", out.Events[0].ACR)
	}
	if out.Events[0].Freshness == nil || out.Events[0].Freshness.UploadFrequency != "real_time" {
		t.Fatalf("data_freshness sai: %+v", out.Events[0].Freshness)
	}
	if out.Events[0].Spend == nil || out.Events[0].Spend.Amount != 108947.38 {
		t.Fatalf("event_spend sai: %+v", out.Events[0].Spend)
	}
	if len(out.Events[0].DedupeKeys) != 1 ||
		out.Events[0].DedupeKeys[0].DedupeKey != "event_id" {
		t.Fatalf("dedupe_key_feedback sai: %+v", out.Events[0].DedupeKeys)
	}
	if len(out.Events[0].ValueDiags) != 1 ||
		out.Events[0].ValueDiags[0].IssueType != "MISSING_OR_INVALID_VALUE" {
		t.Fatalf("value_diagnostics sai: %+v", out.Events[0].ValueDiags)
	}
	if out.Events[0].Delay == nil || out.Events[0].Delay.EventsDelayedOver24H != 10 {
		t.Fatalf("event_delay sai: %+v", out.Events[0].Delay)
	}

	// Aggregate
	if out.Aggregate.EventCount != 2 {
		t.Fatalf("aggregate.event_count sai: %d", out.Aggregate.EventCount)
	}
	// (7.2 + 5.5) / 2 = 6.35
	if out.Aggregate.AvgEMQ < 6.34 || out.Aggregate.AvgEMQ > 6.36 {
		t.Fatalf("aggregate.avg_emq sai: %v", out.Aggregate.AvgEMQ)
	}
	if out.Aggregate.TotalSpend != 108947.38 {
		t.Fatalf("aggregate.total_spend sai: %v", out.Aggregate.TotalSpend)
	}
	// 1 value diagnostic + 1 EMQ diagnostic = 2 vấn đề
	if out.Aggregate.CriticalIssues != 2 {
		t.Fatalf("aggregate.critical_issues sai: %d", out.Aggregate.CriticalIssues)
	}
}

func TestParseDatasetQualityNil(t *testing.T) {
	out := ParseDatasetQuality(nil, "PIX123")
	if len(out.Events) != 0 {
		t.Fatalf("phải trả 0 event, thực tế %d", len(out.Events))
	}
	if out.PixelID != "PIX123" {
		t.Fatalf("pixel_id sai: %q", out.PixelID)
	}
	if out.Aggregate.EventCount != 0 {
		t.Fatalf("aggregate.event_count phải = 0")
	}
}
