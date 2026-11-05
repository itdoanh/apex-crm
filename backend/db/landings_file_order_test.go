package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testConnString lấy chuỗi kết nối DB local cho test (mặc định docker apex-postgres).
func testConnString() string {
	if s := os.Getenv("APEX_TEST_DATABASE_URL"); s != "" {
		return s
	}
	return "postgres://postgres:postgres@localhost:5432/apex_leads?sslmode=disable"
}

// TestGetLandingFilePrefersExactDomain là regression test cho bug 2026-11-05.
//
// BỐI CẢNH: query cũ `WHERE (domain = $1 OR domain = '') ... LIMIT 1` KHÔNG có
// ORDER BY ⇒ PostgreSQL trả về bản ghi TÙY Ý. Trên production, landing phục vụ
// bản `domain=''` CŨ (thiếu cơ chế khóa chống gửi trùng) thay vì bản
// `domain` khớp chính xác ⇒ 1 lần bấm sinh 3 POST /api/leads ⇒ lead thật bị
// ghi vào spam_quarantine ("Lỗi Proof-of-Work").
//
// KỲ VỌNG: bản có `domain` khớp CHÍNH XÁC phải được ưu tiên; chỉ khi không có
// bản khớp chính xác thì mới dùng bản `domain=''`.
func TestGetLandingFilePrefersExactDomain(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const exactDomain = "test-exact.example.com"
	const pageID = "test_lf_ordering"

	// Dọn dữ liệu test
	_, _ = store.pool.Exec(ctx, `DELETE FROM landing_files WHERE page_id = $1`, pageID)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM landing_files WHERE page_id = $1`, pageID)
	}()

	// Bản domain='' — cố TẠI MỚI HƠN (id lớn hơn) để chứng minh rằng query
	// cũ sẽ chọn sai bản này do không có ORDER BY.
	if err := store.SaveLandingFile("", pageID, "tracker.js", "js", "CONTENT_SHARED_OLD"); err != nil {
		t.Fatalf("SaveLandingFile(domain='') lỗi: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)

	// Bản domain khớp chính xác — nội dung ĐÚNG, cũ hơn.
	if err := store.SaveLandingFile(exactDomain, pageID, "tracker.js", "js", "CONTENT_EXACT_NEW"); err != nil {
		t.Fatalf("SaveLandingFile(domain=exact) lỗi: %v", err)
	}

	f, err := store.GetLandingFile(exactDomain, pageID, "tracker.js")
	if err != nil {
		t.Fatalf("GetLandingFile lỗi: %v", err)
	}
	if f == nil {
		t.Fatal("GetLandingFile trả về nil")
	}
	if f.Content != "CONTENT_EXACT_NEW" {
		t.Fatalf("phải ưu tiên bản domain khớp CHÍNH XÁC, nhận được domain=%q content=%q",
			f.Domain, f.Content)
	}
	if f.Domain != exactDomain {
		t.Fatalf("domain kỳ vọng = %q, nhận được %q", exactDomain, f.Domain)
	}
}

// TestGetLandingFileFallsBackToSharedDomain: khi KHÔNG có bản khớp chính xác
// thì phải dùng được bản `domain=''`.
func TestGetLandingFileFallsBackToSharedDomain(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const pageID = "test_lf_fallback"
	_, _ = store.pool.Exec(ctx, `DELETE FROM landing_files WHERE page_id = $1`, pageID)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM landing_files WHERE page_id = $1`, pageID)
	}()

	if err := store.SaveLandingFile("", pageID, "tracker.js", "js", "CONTENT_SHARED"); err != nil {
		t.Fatalf("SaveLandingFile lỗi: %v", err)
	}

	f, err := store.GetLandingFile("khong-ton-tai.example.com", pageID, "tracker.js")
	if err != nil {
		t.Fatalf("GetLandingFile lỗi: %v", err)
	}
	if f == nil || f.Content != "CONTENT_SHARED" {
		t.Fatalf("phải fallback về bản domain='', nhận được %+v", f)
	}
}

// TestSyncSharedStaticFilesToDBCoversAllDomains là regression test cho bug
// 2026-11-05: `SyncSharedStaticFilesToDB` trước đây chỉ duyệt `landing_pages`.
//
// Trên production có các cặp (domain, page_id) CHỈ tồn tại trong `landing_files`
// (ví dụ `('', 'landing_cu')` và `('hanghoaphaisinh.net', 'mxv_080_online')`)
// ⇒ tracker.js của chúng giữ nguyên bản CŨ ⇒ landing chạy code cũ.
//
// KỲ VỌNG: sau khi sync, MỌI cặp (domain, page_id) trong `landing_files`
// đều có tracker.js mới.
func TestSyncSharedStaticFilesToDBCoversAllDomains(t *testing.T) {
	store, err := NewStore(testConnString())
	if err != nil {
		t.Skipf("không kết nối được DB test: %v", err)
	}
	ctx := context.Background()

	const orphanDomain = "test-orphan.example.com"
	const orphanPage = "test_lf_orphan"

	_, _ = store.pool.Exec(ctx, `DELETE FROM landing_files WHERE page_id = $1`, orphanPage)
	defer func() {
		_, _ = store.pool.Exec(ctx, `DELETE FROM landing_files WHERE page_id = $1`, orphanPage)
	}()

	// Cặp (domain, page_id) KHÔNG tồn tại trong landing_pages.
	if err := store.SaveLandingFile(orphanDomain, orphanPage, "tracker.js", "js", "OLD_CONTENT"); err != nil {
		t.Fatalf("SaveLandingFile lỗi: %v", err)
	}

	// Gọi sync với diskRoot trỏ về thư mục chứa frontend/landing/tracker.js.
	// `go test` chạy trong thư mục package (backend/db) ⇒ cần dò ngược lên gốc repo.
	diskRoot := ""
	if _, err := os.Stat(filepath.Join("..", "..", "frontend", "landing", "tracker.js")); err == nil {
		diskRoot = filepath.Join("..", "..")
	} else if _, err := os.Stat(filepath.Join("..", "frontend", "landing", "tracker.js")); err == nil {
		diskRoot = ".."
	} else if _, err := os.Stat(filepath.Join("frontend", "landing", "tracker.js")); err == nil {
		diskRoot = "."
	} else {
		t.Skip("không tìm thấy frontend/landing/tracker.js để sync")
	}

	if err := store.SyncSharedStaticFilesToDB(ctx, diskRoot); err != nil {
		t.Fatalf("SyncSharedStaticFilesToDB lỗi: %v", err)
	}

	f, err := store.GetLandingFile(orphanDomain, orphanPage, "tracker.js")
	if err != nil {
		t.Fatalf("GetLandingFile lỗi: %v", err)
	}
	if f == nil {
		t.Fatal("GetLandingFile trả về nil")
	}
	if f.Content == "OLD_CONTENT" {
		t.Fatal("tracker.js của cặp (domain,page_id) chỉ có trong landing_files CHƯA được đồng bộ — " +
			"landing này sẽ chạy tracker.js cũ")
	}
	if !strings.Contains(f.Content, "LEAD_DEDUP_WINDOW_MS") {
		t.Fatal("tracker.js sau khi sync phải chứa LEAD_DEDUP_WINDOW_MS (cơ chế khóa chống gửi trùng)")
	}
}
