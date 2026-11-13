package db

import (
	"testing"

	"apex-backend/models"
)

// ⚠️ BUG THẬT (2026-11-13): `ListLandingPages` sắp xếp `updated_at DESC`.
// Nếu merge settings theo thứ tự mới→cũ thì bản CŨ ghi đè bản MỚI ⇒ CRM hiển
// thị sai sheet đang gán cho landing.
func TestDedupePrefersNewerSettings(t *testing.T) {
	list := []models.LandingPage{
		// MỚI NHẤT (đầu danh sách) — có google_sheet_ids = [3]
		{
			PageID:    "doanh_chung",
			Domain:    "lophoc.hanghoaphaisinh.net",
			UpdatedAt: "2026-11-13 10:32:05",
			Settings: map[string]interface{}{
				"google_sheet_sync_mode": "selected",
				"google_sheet_ids":       []interface{}{float64(3)},
			},
		},
		// CŨ HƠN — chỉ có title, KHÔNG có google_sheet_ids
		{
			PageID:    "doanh_chung",
			Domain:    "",
			UpdatedAt: "2026-11-13 10:24:54",
			Settings: map[string]interface{}{
				"title": "Buổi chia sẻ",
			},
		},
	}

	out := dedupeLandingPagesByPageID(list)
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}

	ids, ok := out[0].Settings["google_sheet_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected google_sheet_ids to survive merge, got %#v", out[0].Settings["google_sheet_ids"])
	}
	if len(ids) != 1 || int(ids[0].(float64)) != 3 {
		t.Fatalf("expected sheet id [3] (bản mới), got %v", ids)
	}
	if out[0].Settings["title"] != "Buổi chia sẻ" {
		t.Fatalf("expected title từ bản gốc được giữ, got %#v", out[0].Settings["title"])
	}
}
