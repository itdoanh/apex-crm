package db

import (
	"context"
	"log"
)

// RepairMetaPixelSnippet sửa lỗi typo trong snippet Meta Pixel hardcode của HTML landing.
//
// BỐI CẢNH (bug 2026-11-13):
//   Một số landing (xuất bản từ LadiPage hoặc nhập tay) có block Meta Pixel
//   viết sai: `if(!f._fbq)f._bq=n;` — THIẾU chữ `f` ở `_fbq`.
//
//   Meta Pixel chính thức yêu cầu `if(!f._fbq) f._fbq = n;`. Khi gán sai thành
//   `f._bq`, biến `window._fbq` KHÔNG BAO GIỜ được tạo. `fbevents.js` sau khi nạp
//   sẽ tự tạo instance thứ hai (`window._fbq` riêng) ⇒ phát hiện
//   `window.fbq !== window._fbq` ⇒ Meta báo:
//       "[Meta Pixel] - Multiple pixels with conflicting versions were detected
//        on this page."
//   ⇒ PageView có thể bị đếm sai/lặp, ảnh hưởng tối ưu ngân sách Meta Ads.
//
// CÁCH SỬA:
//   Thay literal `f._bq=n` → `f._fbq=n` trong:
//     - `landing_pages.code_html`
//     - `landing_files.content` (các file HTML của mọi landing)
//
//   Dùng `strpos()` (KHÔNG dùng `LIKE`) vì `_` trong `LIKE` là wildcard ⇒
//   có thể match nhầm dòng khác. `replace()` là literal nên chỉ thay đúng chuỗi.
func (s *Store) RepairMetaPixelSnippet(ctx context.Context) error {
	const badSnippet = "f._bq=n"
	const goodSnippet = "f._fbq=n"

	// 1. landing_pages.code_html
	tagPages, err := s.pool.Exec(ctx, `
		UPDATE landing_pages
		SET code_html = replace(code_html, $1, $2),
		    updated_at = NOW()
		WHERE strpos(code_html, $1) > 0
	`, badSnippet, goodSnippet)
	if err != nil {
		return err
	}
	if tagPages.RowsAffected() > 0 {
		log.Printf("[INFO] RepairMetaPixelSnippet: đã sửa typo Meta Pixel trong %d landing_pages.code_html", tagPages.RowsAffected())
	}

	// 2. landing_files.content — chỉ các file HTML (bỏ qua file nhị phân base64)
	tagFiles, err := s.pool.Exec(ctx, `
		UPDATE landing_files
		SET content = replace(content, $1, $2),
		    updated_at = NOW()
		WHERE file_type IN ('html', 'htm', 'text', '')
		  AND strpos(content, $1) > 0
	`, badSnippet, goodSnippet)
	if err != nil {
		return err
	}
	if tagFiles.RowsAffected() > 0 {
		log.Printf("[INFO] RepairMetaPixelSnippet: đã sửa typo Meta Pixel trong %d landing_files", tagFiles.RowsAffected())
	}

	return nil
}

// RepairMetaPixelPageViewEventID bổ sung `eventID` cho lệnh `fbq('track','PageView')`
// hardcode trong HTML landing đã lưu trong CSDL.
//
// BỐI CẢNH (fix 2026-11-16 — dedup PageView Pixel ↔ CAPI):
//   Meta CHỈ gộp sự kiện browser (`fbq`) với sự kiện máy chủ (CAPI) khi cả hai
//   dùng CHUNG `event_id`. Landing export từ LadiPage có sẵn
//   `fbq('track', 'PageView')` KHÔNG kèm eventID ⇒ nếu server CAPI cũng bắn
//   `PageView` ⇒ Meta tính **1 lượt xem thành 2 lượt PageView** ⇒ sai lệch toàn
//   bộ chỉ số Landing Page View + phân phối quảng cáo.
//
// ⚠️ Backend CŨNG rewrite lúc render (`rewritePageViewWithEventID` trong
// `backend/handlers/landing_handler.go`) ⇒ HTML phục vụ luôn đúng. Hàm này
// sửa thêm DỮ LIỆU trong DB để:
//   1. HTML xuất/nhập trong Code Studio hiển thị đúng.
//   2. Landing phục vụ qua đường KHÔNG đi qua `InjectDynamicScriptsWithBase`
//      vẫn có eventID.
//
// CÁCH SỬA: thay `fbq('track', 'PageView')` → `fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID })`.
//
// ⚠️ KHÔNG đụng vào lệnh ĐÃ có `eventID` (pattern chỉ khớp lệnh không có tham số
// thứ 4) ⇒ chạy lại nhiều lần vẫn an toàn (idempotent).
func (s *Store) RepairMetaPixelPageViewEventID(ctx context.Context) error {
	const oldCall = "fbq('track', 'PageView')"
	const newCall = "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID })"

	// 1. landing_pages.code_html
	tagPages, err := s.pool.Exec(ctx, `
		UPDATE landing_pages
		SET code_html = replace(code_html, $1, $2),
		    updated_at = NOW()
		WHERE strpos(code_html, $1) > 0
	`, oldCall, newCall)
	if err != nil {
		return err
	}
	if tagPages.RowsAffected() > 0 {
		log.Printf("[INFO] RepairMetaPixelPageViewEventID: đã thêm eventID cho PageView trong %d landing_pages.code_html", tagPages.RowsAffected())
	}

	// 2. landing_files.content — chỉ các file HTML (bỏ qua file nhị phân base64)
	tagFiles, err := s.pool.Exec(ctx, `
		UPDATE landing_files
		SET content = replace(content, $1, $2),
		    updated_at = NOW()
		WHERE file_type IN ('html', 'htm', 'text', '')
		  AND strpos(content, $1) > 0
	`, oldCall, newCall)
	if err != nil {
		return err
	}
	if tagFiles.RowsAffected() > 0 {
		log.Printf("[INFO] RepairMetaPixelPageViewEventID: đã thêm eventID cho PageView trong %d landing_files", tagFiles.RowsAffected())
	}

	return nil
}
