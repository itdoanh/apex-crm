package cache

import (
	"testing"

	"apex-backend/models"
)

func mkLanding(pageID, domain, updatedAt string, settings map[string]interface{}) *models.LandingPage {
	return &models.LandingPage{
		PageID:   pageID,
		Domain:   domain,
		UpdatedAt: updatedAt,
		Settings: settings,
	}
}

func newTestCache(rows []*models.LandingPage) *LandingCache {
	c := &LandingCache{
		byDomainPage: make(map[string]*models.LandingPage),
		byPageID:     make(map[string]*models.LandingPage),
		byPageIDRows: make(map[string]map[string]*models.LandingPage),
	}
	for _, p := range rows {
		key := makeKey(p.Domain, p.PageID)
		c.byDomainPage[key] = p
		pageKey := p.PageID
		if c.byPageIDRows[pageKey] == nil {
			c.byPageIDRows[pageKey] = make(map[string]*models.LandingPage)
		}
		c.byPageIDRows[pageKey][p.Domain] = p
		if _, exists := c.byPageID[pageKey]; !exists {
			c.byPageID[pageKey] = p
		}
	}
	return c
}

// ⚠️ BUG THẬT (2026-11-13): landing có 2 row (bản gốc + bản domain).
// Cấu hình Google Sheets chỉ nằm ở bản domain.
// Đọc bản gốc ⇒ không có sheet ids ⇒ rơi vào nhánh "đồng bộ mặc định"
// ⇒ lead bắn vào MỌI sheet active.
func TestGetSheetSyncSettings_PicksDomainRow(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("doanh_chung", "", "2026-11-13 10:24:54", map[string]interface{}{
			"title": "Buổi chia sẻ",
		}),
		mkLanding("doanh_chung", "lophoc.hanghoaphaisinh.net", "2026-11-13 10:32:05", map[string]interface{}{
			"google_sheet_sync_mode": "selected",
			"google_sheet_ids":       []interface{}{float64(3)},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("lophoc.hanghoaphaisinh.net", "doanh_chung")
	if !has {
		t.Fatalf("expected hasConfig=true")
	}
	if mode != "selected" {
		t.Fatalf("expected mode=selected, got %q", mode)
	}
	if len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("expected sheet ids [3], got %v", ids)
	}
}

// Landing chỉ có bản domain (không có bản gốc) — landing_cu.
func TestGetSheetSyncSettings_OnlyDomainRow(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("landing_cu", "ads.giaodichhanghoatuonglai.com", "2026-11-10 20:55:29", map[string]interface{}{
			"google_sheet_sync_mode": "selected",
			"google_sheet_ids":       []interface{}{float64(2)},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("ads.giaodichhanghoatuonglai.com", "landing_cu")
	if !has || mode != "selected" || len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected selected/[2], got has=%v mode=%q ids=%v", has, mode, ids)
	}
}

// Domain khác không khớp ⇒ phải fallback sang bản gốc / bản domain khác.
func TestGetSheetSyncSettings_FallbackToBaseRow(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("mxv_080_online", "", "2026-11-01 09:00:00", map[string]interface{}{
			"google_sheet_sync_mode": "selected",
			"google_sheet_ids":       []interface{}{float64(1)},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("hanghoaphaisinh.net", "mxv_080_online")
	if !has || mode != "selected" || len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected fallback selected/[1], got has=%v mode=%q ids=%v", has, mode, ids)
	}
}

// Không có cấu hình sheet nào ⇒ hasConfig=false (không tự suy diễn).
func TestGetSheetSyncSettings_NoConfig(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("landing_x", "", "2026-11-01 09:00:00", map[string]interface{}{
			"title": "abc",
		}),
	})

	_, _, has := c.GetSheetSyncSettings("", "landing_x")
	if has {
		t.Fatalf("expected hasConfig=false")
	}
}

// Bản domain khai báo tắt đồng bộ ⇒ phải thắng bản gốc.
func TestGetSheetSyncSettings_DisabledWins(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("landing_y", "", "2026-11-01 09:00:00", map[string]interface{}{
			"google_sheet_sync_mode": "all",
			"google_sheet_ids":       []interface{}{float64(1)},
		}),
		mkLanding("landing_y", "ads.example.com", "2026-11-02 09:00:00", map[string]interface{}{
			"google_sheet_sync_mode": "disabled",
			"google_sheet_ids":       []interface{}{},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("ads.example.com", "landing_y")
	if !has {
		t.Fatalf("expected hasConfig=true")
	}
	if mode != "disabled" {
		t.Fatalf("expected mode=disabled, got %q", mode)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no sheet ids, got %v", ids)
	}
}
