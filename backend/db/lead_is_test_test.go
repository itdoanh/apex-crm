package db

import (
	"context"
	"encoding/json"
	"testing"
)

// ══════════════════════════════════════════════════════════════════════
// TEST LEAD — TỰ ĐỘNG ĐÁNH DẤU & LỌC KHỎI THỐNG KÊ (yêu cầu 2026-11-16)
// ══════════════════════════════════════════════════════════════════════
//
// Yêu cầu của anh itdoanh:
//   "Khi nhận được các lead trong tên có (test) thì sẽ tự đánh dấu lead đó
//    là test và không tính vào các biểu đồ, thông số (vẫn đồng bộ lead test
//    đó lên sheet bình thường)."
//
// ⇒ SaveLead tự nhận diện tên chứa "(test)" ⇒ set is_test = TRUE.
// ⇒ Mọi thống kê / biểu đồ dùng `buildWhereClause` (mặc định) ⇒ LOẠI lead test.
// ⇒ Bảng CRM + export CSV dùng `buildWhereClauseEx(..., includeTest=true)`
//    ⇒ VẪN THẤY lead test (có badge "TEST").

// TestSaveLeadMarksTestLead: tên chứa "(test)" ⇒ lead.IsTest = TRUE.
func TestSaveLeadMarksTestLead(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const phone = "0912000111"
	const landingID = "test_is_test_marking"

	_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	}()

	tracking, _ := json.Marshal(map[string]interface{}{
		"session_id": "sess_test_marking_1",
		"visitor_id": "v_test_marking_1",
		"fullName":   "Nguyễn Test (test)",
	})

	lead, isNew, err := store.SaveLead(landingID, "test", phone, "https://example.com/landing", tracking, "", "", "Việt Nam", "", "", "")
	if err != nil {
		t.Fatalf("SaveLead lỗi: %v", err)
	}
	if !isNew {
		t.Fatal("phải tạo lead mới")
	}
	if !lead.IsTest {
		t.Fatal("lead có tên chứa \"(test)\" phải được đánh dấu is_test = TRUE")
	}

	// Kiểm tra cột trong DB thực sự được ghi.
	var dbIsTest bool
	if err := store.pool.QueryRow(ctx, `SELECT COALESCE(is_test, FALSE) FROM leads WHERE id = $1`, lead.ID).Scan(&dbIsTest); err != nil {
		t.Fatalf("không đọc được is_test: %v", err)
	}
	if !dbIsTest {
		t.Fatal("cột leads.is_test phải = TRUE cho lead test")
	}
}

// TestSaveLeadNormalLeadNotTest: tên bình thường ⇒ is_test = FALSE.
func TestSaveLeadNormalLeadNotTest(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const phone = "0912000222"
	const landingID = "test_is_test_normal"

	_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai = $1`, phone)
	}()

	tracking, _ := json.Marshal(map[string]interface{}{
		"session_id": "sess_test_marking_2",
		"fullName":   "Nguyễn Văn A",
	})

	lead, _, err := store.SaveLead(landingID, "test", phone, "https://example.com/landing", tracking, "", "", "Việt Nam", "", "", "")
	if err != nil {
		t.Fatalf("SaveLead lỗi: %v", err)
	}
	if lead.IsTest {
		t.Fatal("lead tên bình thường KHÔNG được đánh dấu là test")
	}
}

// TestBuildWhereClauseExcludesTestLeads: WHERE mặc định loại lead test,
// bản includeTest = true thì giữ lại.
func TestBuildWhereClauseExcludesTestLeads(t *testing.T) {
	whereDefault, _ := buildWhereClause("", "", "", "", "", "", -1, -1)
	if whereDefault == "" {
		t.Fatal("WHERE mặc định phải có điều kiện loại lead test")
	}
	if !containsStr(whereDefault, "is_test") {
		t.Fatalf("WHERE mặc định phải lọc is_test, nhận: %s", whereDefault)
	}

	whereInclude, _ := buildWhereClauseEx("", "", "", "", "", "", -1, -1, true)
	if containsStr(whereInclude, "is_test") {
		t.Fatalf("WHERE includeTest=true KHÔNG được lọc is_test, nhận: %s", whereInclude)
	}
}

// TestGetCRMStatsExcludesTestLeads: thống kê CRM không tính lead test.
func TestGetCRMStatsExcludesTestLeads(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const phoneTest = "0912000333"
	const phoneReal = "0912000444"
	const landingID = "test_is_test_stats"

	_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai IN ($1, $2)`, phoneTest, phoneReal)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM leads WHERE so_dien_thoai IN ($1, $2)`, phoneTest, phoneReal)
	}()

	trackingTest, _ := json.Marshal(map[string]interface{}{
		"session_id": "sess_test_stats_1",
		"fullName":   "Lead Test (test)",
	})
	trackingReal, _ := json.Marshal(map[string]interface{}{
		"session_id": "sess_test_stats_2",
		"fullName":   "Lead Thật",
	})

	if _, _, err := store.SaveLead(landingID, "test", phoneTest, "https://example.com/x", trackingTest, "", "", "Việt Nam", "", "", ""); err != nil {
		t.Fatalf("SaveLead lead test lỗi: %v", err)
	}
	if _, _, err := store.SaveLead(landingID, "test", phoneReal, "https://example.com/x", trackingReal, "", "", "Việt Nam", "", "", ""); err != nil {
		t.Fatalf("SaveLead lead thật lỗi: %v", err)
	}

	stats, err := store.GetCRMStats("", landingID, "test", "", "", "", -1, -1)
	if err != nil {
		t.Fatalf("GetCRMStats lỗi: %v", err)
	}
	summary, ok := stats["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("summary không đúng định dạng")
	}
	total, _ := summary["total"].(int)
	if total != 1 {
		t.Fatalf("thống kê CRM phải chỉ tính 1 lead thật (bỏ qua lead test), nhận %d", total)
	}
}

// containsRaw là helper kiểm tra substring (tránh import thêm).
func containsStr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
