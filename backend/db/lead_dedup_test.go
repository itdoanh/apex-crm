package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestSaveLeadDeduplicatesSamePhoneSameLanding là regression test cho bug
// 2026-11-06: 1 lần bấm "Đăng ký" sinh nhiều request ⇒ PostgreSQL tạo nhiều
// dòng lead trùng SĐT cùng thời gian.
//
// KỲ VỌNG:
//   - Lần 1 ⇒ isNew = true (tạo lead mới).
//   - Lần 2 (cùng SĐT + cùng landing) ⇒ isNew = false, trả về cùng lead.ID.
//
// Lưu ý: test này cần DB thật (APEX_TEST_DATABASE_URL) — sẽ SKIP nếu không có.
func TestSaveLeadDeduplicatesSamePhoneSameLanding(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const phone = "0999000111"
	const landingID = "test_dedup_same_landing"

	_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	}()

	tracking, _ := json.Marshal(map[string]interface{}{
		"session_id": "sess_test_dedup_1",
		"visitor_id": "v_test_dedup_1",
	})

	lead1, isNew1, err := store.SaveLead(landingID, "test", phone, "https://example.com/landing", tracking, "", "", "Việt Nam", "", "", "")
	if err != nil {
		t.Fatalf("SaveLead lần 1 lỗi: %v", err)
	}
	if !isNew1 {
		t.Fatal("lần 1 phải tạo lead mới (isNew = true)")
	}

	lead2, isNew2, err := store.SaveLead(landingID, "test", phone, "https://example.com/landing", tracking, "", "", "Việt Nam", "", "", "")
	if err != nil {
		t.Fatalf("SaveLead lần 2 lỗi: %v", err)
	}
	if isNew2 {
		t.Fatal("lần 2 (cùng SĐT + cùng landing) phải được coi là trùng lặp (isNew = false)")
	}
	if lead2.ID != lead1.ID {
		t.Fatalf("lần 2 phải trả về cùng lead.ID, nhận %d vs %d", lead2.ID, lead1.ID)
	}

	var cnt int
	_ = store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM leads WHERE so_dien_thoai = $1`, phone).Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("phải chỉ có 1 dòng lead cho 1 SĐT, nhận %d", cnt)
	}
}

// TestSaveLeadDeduplicatesSamePhoneDifferentLandingWithin10Minutes: bắt trường
// hợp gửi 2 landing liên tiếp cùng 1 SĐT (VD do redirect sai) ⇒ không được
// tạo 2 lead.
func TestSaveLeadDeduplicatesSamePhoneDifferentLandingWithin10Minutes(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const phone = "0999000222"
	const landingA = "test_dedup_landing_a"
	const landingB = "test_dedup_landing_b"

	_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	}()

	tracking, _ := json.Marshal(map[string]interface{}{"session_id": "sess_test_dedup_2"})

	lead1, isNew1, err := store.SaveLead(landingA, "test", phone, "https://example.com/a", tracking, "", "", "Việt Nam", "", "", "")
	if err != nil {
		t.Fatalf("SaveLead landing A lỗi: %v", err)
	}
	if !isNew1 {
		t.Fatal("landing A phải tạo lead mới")
	}

	lead2, isNew2, err := store.SaveLead(landingB, "test", phone, "https://example.com/b", tracking, "", "", "Việt Nam", "", "", "")
	if err != nil {
		t.Fatalf("SaveLead landing B lỗi: %v", err)
	}
	if isNew2 {
		t.Fatal("cùng SĐT trong 10 phút (khác landing) phải được coi là trùng lặp")
	}
	if lead2.ID != lead1.ID {
		t.Fatalf("phải trả về cùng lead.ID, nhận %d vs %d", lead2.ID, lead1.ID)
	}
}

// TestSaveLeadConcurrentSamePhone: N request SONG SONG cùng 1 SĐT ⇒ chỉ đúng
// 1 lead được tạo (race condition TOCTOU — đã chặn bằng pg_advisory_xact_lock).
func TestSaveLeadConcurrentSamePhone(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const phone = "0999000333"
	const landingID = "test_dedup_concurrent"

	_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	}()

	const n = 8
	start := make(chan struct{})
	ids := make(chan uint64, n)
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			tracking, _ := json.Marshal(map[string]interface{}{
				"session_id": "sess_test_dedup_conc",
				"visitor_id": "v_test_dedup_conc",
				"idx":        idx,
			})
			<-start
			lead, _, err := store.SaveLead(landingID, "test", phone, "https://example.com/c", tracking, "", "", "Việt Nam", "", "", "")
			if err != nil {
				errs <- err
				return
			}
			ids <- lead.ID
		}(i)
	}
	close(start)

	for i := 0; i < n; i++ {
		select {
		case err := <-errs:
			t.Fatalf("SaveLead song song lỗi: %v", err)
		case <-ids:
		case <-time.After(20 * time.Second):
			t.Fatal("timeout khi chờ SaveLead song song")
		}
	}

	var cnt int
	_ = store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM leads WHERE so_dien_thoai = $1`, phone).Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("%d request song song cùng SĐT phải tạo đúng 1 lead, nhận %d", n, cnt)
	}
}
