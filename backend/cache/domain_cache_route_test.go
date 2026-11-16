package cache

import (
	"testing"

	"apex-backend/models"
)

// ⚠️ BUG THẬT (fix 2026-11-16 — bug chồng chéo thông số landing):
//
//   `/chucmung.html` KHÔNG khớp route prefix `/chucmung` vì
//   `strings.HasPrefix("/chucmung.html", "/chucmung/")` = FALSE.
//
//   ⇒ `ResolveRoute` trả `found = false` ⇒ Edge Router rơi xuống nhánh phục vụ
//     file TĨNH từ đĩa ⇒ KHÔNG có `data-landing-id` / `APEX_LANDING_CONFIG`
//     ⇒ tracker.js rơi về fallback ⇒ analytics/lead ghi về landing KHÁC.
//
// Đây là hàm dựng cache trong test vì `routes` là field private.
func newTestDomainCache(routes []models.DomainRoute) *DomainCache {
	c := &DomainCache{routes: make(map[string][]models.DomainRoute)}
	for _, r := range routes {
		dom := r.Domain
		c.routes[dom] = append(c.routes[dom], r)
	}
	// Sắp xếp longest-prefix như `Warmup` làm.
	for dom := range c.routes {
		routesList := c.routes[dom]
		for i := 0; i < len(routesList); i++ {
			for j := i + 1; j < len(routesList); j++ {
				pi, pj := routesList[i].PathPrefix, routesList[j].PathPrefix
				if pj == "/" {
					continue
				}
				if pi == "/" || len(pj) > len(pi) {
					routesList[i], routesList[j] = routesList[j], routesList[i]
				}
			}
		}
		c.routes[dom] = routesList
	}
	return c
}

func TestResolveRouteMatchesHtmlSuffix(t *testing.T) {
	c := newTestDomainCache([]models.DomainRoute{
		{Domain: "ads.example.com", PathPrefix: "/chucmung", LandingID: "landing_lznoew", IsActive: true},
		{Domain: "ads.example.com", PathPrefix: "/landing", LandingID: "landing_cu", IsActive: true},
		{Domain: "ads.example.com", PathPrefix: "/", LandingID: "landing_232", IsActive: true},
	})

	landingID, prefix, subPath, _, found := c.ResolveRoute("ads.example.com", "/chucmung.html")
	if !found {
		t.Fatalf("expected /chucmung.html to match route /chucmung, got found=false")
	}
	if landingID != "landing_lznoew" {
		t.Fatalf("expected landing_lznoew, got %q", landingID)
	}
	if prefix != "/chucmung" {
		t.Fatalf("expected prefix /chucmung, got %q", prefix)
	}
	if subPath != "chucmung.html" {
		t.Fatalf("expected subPath chucmung.html (để Edge Router vào nhánh có inject), got %q", subPath)
	}
}

func TestResolveRouteHtmlSuffixDoesNotShadowLongerRoute(t *testing.T) {
	c := newTestDomainCache([]models.DomainRoute{
		{Domain: "ads.example.com", PathPrefix: "/chucmung", LandingID: "landing_lznoew", IsActive: true},
		{Domain: "ads.example.com", PathPrefix: "/landing", LandingID: "landing_cu", IsActive: true},
		{Domain: "ads.example.com", PathPrefix: "/", LandingID: "landing_232", IsActive: true},
	})

	// `/landing/chucmung.html` phải thuộc route `/landing` (dài hơn).
	landingID, prefix, subPath, _, found := c.ResolveRoute("ads.example.com", "/landing/chucmung.html")
	if !found {
		t.Fatalf("expected found=true")
	}
	if landingID != "landing_cu" {
		t.Fatalf("expected landing_cu, got %q", landingID)
	}
	if prefix != "/landing" {
		t.Fatalf("expected prefix /landing, got %q", prefix)
	}
	if subPath != "chucmung.html" {
		t.Fatalf("expected subPath chucmung.html, got %q", subPath)
	}
}

func TestResolveRouteUnchangedForNormalPaths(t *testing.T) {
	c := newTestDomainCache([]models.DomainRoute{
		{Domain: "ads.example.com", PathPrefix: "/chucmung", LandingID: "landing_lznoew", IsActive: true},
		{Domain: "ads.example.com", PathPrefix: "/landing", LandingID: "landing_cu", IsActive: true},
		{Domain: "ads.example.com", PathPrefix: "/", LandingID: "landing_232", IsActive: true},
	})

	cases := []struct {
		path      string
		wantLand  string
		wantPref  string
		wantSub   string
	}{
		{"/chucmung", "landing_lznoew", "/chucmung", ""},
		{"/chucmung/", "landing_lznoew", "/chucmung", ""},
		{"/chucmung/index.html", "landing_lznoew", "/chucmung", ""},
		{"/landing", "landing_cu", "/landing", ""},
		{"/landing/tracker.js", "landing_cu", "/landing", "tracker.js"},
		{"/", "landing_232", "/", ""},
		{"/tracker.js", "landing_232", "/", "tracker.js"},
	}

	for _, tc := range cases {
		landingID, prefix, subPath, _, found := c.ResolveRoute("ads.example.com", tc.path)
		if !found {
			t.Fatalf("%s: expected found=true", tc.path)
		}
		if landingID != tc.wantLand || prefix != tc.wantPref || subPath != tc.wantSub {
			t.Fatalf("%s: got (%q,%q,%q), want (%q,%q,%q)",
				tc.path, landingID, prefix, subPath, tc.wantLand, tc.wantPref, tc.wantSub)
		}
	}
}
