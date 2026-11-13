package cache

import (
	"testing"

	"apex-backend/models"
)

// Landing chỉ có bản gốc (`domain=''`) — mxv_080_online.
// Không có domain trong URL lead ⇒ phải lấy đúng config của bản gốc.
func TestGetSheetSyncSettings_BaseOnly(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("mxv_080_online", "", "2026-11-01 09:00:00", map[string]interface{}{
			"google_sheet_sync_mode": "selected",
			"google_sheet_ids":       []interface{}{float64(1)},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("", "mxv_080_online")
	if !has || mode != "selected" || len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected selected/[1], got has=%v mode=%q ids=%v", has, mode, ids)
	}
}

// Lead đến từ domain KHÔNG khớp bất kỳ row nào ⇒ phải fallback sang bản domain
// đang chạy thật (đừng rơi vào "không có cấu hình" ⇒ bắn vào mọi sheet).
func TestGetSheetSyncSettings_UnknownDomainFallsBack(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("doanh_chung", "", "2026-11-13 10:24:54", map[string]interface{}{
			"title": "Buổi chia sẻ",
		}),
		mkLanding("doanh_chung", "lophoc.hanghoaphaisinh.net", "2026-11-13 10:32:05", map[string]interface{}{
			"google_sheet_sync_mode": "selected",
			"google_sheet_ids":       []interface{}{float64(3)},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("localhost:8080", "doanh_chung")
	if !has || mode != "selected" || len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("expected fallback selected/[3], got has=%v mode=%q ids=%v", has, mode, ids)
	}
}

// landing_lznoew: `google_sheet_ids: []` + mode `selected` ⇒ KHÔNG sync vào sheet nào.
func TestGetSheetSyncSettings_EmptyListMeansNoSync(t *testing.T) {
	c := newTestCache([]*models.LandingPage{
		mkLanding("landing_lznoew", "ads.giaodichhanghoatuonglai.com", "2026-11-10 20:55:29", map[string]interface{}{
			"google_sheet_sync_mode": "selected",
			"google_sheet_ids":       []interface{}{},
		}),
	})

	mode, ids, has := c.GetSheetSyncSettings("ads.giaodichhanghoatuonglai.com", "landing_lznoew")
	if !has {
		t.Fatalf("expected hasConfig=true")
	}
	if mode != "selected" {
		t.Fatalf("expected mode=selected, got %q", mode)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty sheet ids, got %v", ids)
	}
}
