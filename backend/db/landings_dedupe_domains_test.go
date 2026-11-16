package db

import (
	"testing"

	"apex-backend/models"
)

// ⚠️ BUG THẬT (fix 2026-11-16 — thống kê chồng chéo giữa các landing):
//
//   Landing nhân bản rồi deploy lên NHIỀU domain vẫn có NHIỀU row trong
//   `landing_pages` (bản gốc + bản theo domain). `dedupeLandingPagesByPageID`
//   gộp về 1 card ⇒ danh sách `Domains` phải gộp TẤT CẢ domain, nếu không
//   UI chỉ hiện 1 domain ⇒ người dùng tưởng số liệu đang gộp chồng.
func TestDedupeMergesDeployDomains(t *testing.T) {
	list := []models.LandingPage{
		{
			PageID:    "xu_huong_1",
			Domain:    "",
			UpdatedAt: "2026-11-15 10:00:00",
			Domains:   []string{"lophoc.hanghoaphaisinh.net", "xuhuong.hanghoaphaisinh.net"},
		},
		{
			PageID:    "xu_huong_1",
			Domain:    "daotao.rin.io.vn",
			UpdatedAt: "2026-11-15 11:00:00",
			Domains:   []string{"daotao.rin.io.vn"},
		},
	}

	out := dedupeLandingPagesByPageID(list)
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	if len(out[0].Domains) != 3 {
		t.Fatalf("expected 3 domains sau khi gộp, got %v", out[0].Domains)
	}
	want := map[string]bool{
		"lophoc.hanghoaphaisinh.net":    true,
		"xuhuong.hanghoaphaisinh.net":   true,
		"daotao.rin.io.vn":              true,
	}
	for _, d := range out[0].Domains {
		if !want[d] {
			t.Fatalf("unexpected domain %q in %v", d, out[0].Domains)
		}
	}
}

// Domain trùng lặp giữa các row phải được loại (không hiện 2 badge).
func TestDedupeDeduplicatesDeployDomains(t *testing.T) {
	list := []models.LandingPage{
		{PageID: "a", Domain: "", UpdatedAt: "2026-11-15 10:00:00", Domains: []string{"x.com"}},
		{PageID: "a", Domain: "x.com", UpdatedAt: "2026-11-15 11:00:00", Domains: []string{"x.com"}},
	}

	out := dedupeLandingPagesByPageID(list)
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	if len(out[0].Domains) != 1 || out[0].Domains[0] != "x.com" {
		t.Fatalf("expected exactly [x.com], got %v", out[0].Domains)
	}
}

// Landing không có domain nào ⇒ `Domains` = nil (UI hiện "Chưa Deploy").
func TestDedupeNoDomains(t *testing.T) {
	list := []models.LandingPage{
		{PageID: "mxv_080_online", Domain: "", UpdatedAt: "2026-11-15 10:00:00"},
	}

	out := dedupeLandingPagesByPageID(list)
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	if len(out[0].Domains) != 0 {
		t.Fatalf("expected empty domains, got %v", out[0].Domains)
	}
}
