package db

import (
	"context"
	"encoding/json"
	"log"
	"strings"
)

// RepairDuplicateLandingSettings đồng bộ `settings` giữa bản GỐC (`domain=''`)
// và các bản theo domain (`domain <> ''`) của cùng một `page_id`.
//
// ⚠️ BỐI CẢNH (fix 2026-11-13):
//   `landing_pages` có PK composite `(domain, page_id)` nên một landing có thể tồn
//   tại 2 row: bản gốc + bản domain. Bản domain được tạo ra từ lúc "deploy landing"
//   và chỉ chứa MỘT PHẦN cấu hình (`path_prefix`, `google_sheet_ids`,
//   `google_sheet_sync_mode`) — các trường SEO (`title`, `og_image`,
//   `meta_description`) bị ĐỂ TRỐNG.
//
//   Hậu quả: `GetLandingPage(domain, page_id)` ưu tiên bản domain ⇒ landing chạy
//   thật trên domain đích bị mất title/description. Đây chính là lý do ô "Tiêu đề
//   SEO" trong CRM trông trống sau khi deploy.
//
// ⇒ Sửa: với mỗi `page_id` có cả 2 loại row, gộp `settings` của bản gốc vào bản
//   domain — CHỈ điền các khoá đang TRỐNG ở bản domain, KHÔNG ghi đè giá trị đã có.
//
// Bản gốc là nguồn "mặc định" của landing; bản domain là "bản override theo domain".
// Giá trị rỗng ở bản domain = "chưa cấu hình riêng" ⇒ lấy từ bản gốc.
func (s *Store) RepairDuplicateLandingSettings(ctx context.Context) error {
	if err := s.repairDomainFromBase(ctx); err != nil {
		return err
	}
	return s.repairBaseFromDomain(ctx)
}

// repairBaseFromDomain đồng bộ cấu hình Google Sheets TỪ bản domain VỀ bản gốc.
//
// ⚠️ BẤT BUỘC (fix 2026-11-13 — lead bắn tràn sang nhiều sheet):
//   `google_sheet_ids` / `google_sheet_sync_mode` là cấu hình ở MỨC LANDING.
//   Nếu chỉ lưu ở bản theo domain mà bản gốc thiếu ⇒ mọi đường đọc landing
//   bằng `domain = ''` (list CRM, fallback DB, tool debug) sẽ KHÔNG thấy cấu
//   hình ⇒ hiển thị sai sheet trong CRM.
//
//   ⇒ Đồng bộ `google_sheet_sync_mode` + `google_sheet_ids` từ bản domain (mới
//     nhất) về bản gốc khi bản gốc chưa có (hoặc rỗng).
//
//   KHÔNG ghi đè giá trị đã có ở bản gốc — bản gốc là nguồn mặc định, anh
//   itdoanh có thể đã cấu hình riêng.
func (s *Store) repairBaseFromDomain(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT lp.page_id,
		       lp.domain,
		       lp.settings,
		       COALESCE(base.settings, '{}'::jsonb) AS base_settings
		FROM landing_pages lp
		LEFT JOIN landing_pages base
		       ON base.page_id = lp.page_id AND base.domain = ''
		WHERE lp.domain <> ''
		  AND base.page_id IS NOT NULL
		ORDER BY lp.updated_at DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type repair struct {
		pageID      string
		domain      string
		newSettings map[string]interface{}
	}
	repairs := make([]repair, 0)
	seen := make(map[string]bool)

	for rows.Next() {
		var pageID, domain string
		var domBytes, baseBytes []byte
		if err := rows.Scan(&pageID, &domain, &domBytes, &baseBytes); err != nil {
			return err
		}

		// Chỉ xét row domain MỚI NHẤT cho mỗi page_id.
		if seen[pageID] {
			continue
		}

		domSettings := make(map[string]interface{})
		if len(domBytes) > 0 {
			_ = json.Unmarshal(domBytes, &domSettings)
		}
		baseSettings := make(map[string]interface{})
		if len(baseBytes) > 0 {
			_ = json.Unmarshal(baseBytes, &baseSettings)
		}

		changed := false
		for _, key := range []string{"google_sheet_sync_mode", "google_sheet_ids"} {
			domVal, ok := domSettings[key]
			if !ok || domVal == nil {
				continue
			}
			if s, isStr := domVal.(string); isStr && strings.TrimSpace(s) == "" {
				continue
			}
			// Bản gốc đã có giá trị ⇒ KHÔNG ghi đè.
			if existing, ok2 := baseSettings[key]; ok2 {
				if es, ok3 := existing.(string); ok3 && strings.TrimSpace(es) == "" {
					baseSettings[key] = domVal
					changed = true
				}
				continue
			}
			baseSettings[key] = domVal
			changed = true
		}

		if changed {
			seen[pageID] = true
			repairs = append(repairs, repair{pageID: pageID, domain: domain, newSettings: baseSettings})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range repairs {
		settingsJSON, err := json.Marshal(r.newSettings)
		if err != nil {
			continue
		}
		res, err := s.pool.Exec(ctx, `
			UPDATE landing_pages
			SET settings = $1, updated_at = NOW()
			WHERE page_id = $2 AND domain = ''
		`, settingsJSON, r.pageID)
		if err != nil {
			log.Printf("[WARN] repairBaseFromDomain: không sửa được %s: %v", r.pageID, err)
			continue
		}
		log.Printf("[INFO] repairBaseFromDomain: đã đồng bộ cấu hình Google Sheets về bản gốc của landing '%s' (domain nguồn=%s, rows=%d)",
			r.pageID, r.domain, res.RowsAffected())
	}

	return nil
}

// repairDomainFromBase gộp `settings` của bản GỐC (`domain=''`) vào bản theo
// domain — CHỈ điền các khoá đang TRỐNG ở bản domain, KHÔNG ghi đè giá trị đã có.
func (s *Store) repairDomainFromBase(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT lp.page_id,
		       lp.domain,
		       lp.settings,
		       COALESCE(base.settings, '{}'::jsonb) AS base_settings
		FROM landing_pages lp
		LEFT JOIN landing_pages base
		       ON base.page_id = lp.page_id AND base.domain = ''
		WHERE lp.domain <> ''
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type repair struct {
		pageID      string
		domain      string
		newSettings map[string]interface{}
	}
	repairs := make([]repair, 0)

	for rows.Next() {
		var pageID, domain string
		var domBytes, baseBytes []byte
		if err := rows.Scan(&pageID, &domain, &domBytes, &baseBytes); err != nil {
			return err
		}

		domSettings := make(map[string]interface{})
		if len(domBytes) > 0 {
			_ = json.Unmarshal(domBytes, &domSettings)
		}
		baseSettings := make(map[string]interface{})
		if len(baseBytes) > 0 {
			_ = json.Unmarshal(baseBytes, &baseSettings)
		}
		if len(baseSettings) == 0 {
			continue
		}

		// Chỉ điền khoá đang TRỐNG ở bản domain. Khoá đã có giá trị (kể cả
		// `path_prefix`, `google_sheet_ids` riêng của domain) được giữ nguyên.
		changed := false
		for k, v := range baseSettings {
			if v == nil {
				continue
			}
			if s, ok := v.(string); ok && s == "" {
				continue // không điền chuỗi rỗng
			}
			if existing, ok := domSettings[k]; ok {
				if es, ok2 := existing.(string); ok2 && es == "" {
					domSettings[k] = v // giá trị rỗng ở domain ⇒ lấy từ bản gốc
					changed = true
				}
				continue
			}
			domSettings[k] = v // khoá chưa tồn tại ở domain ⇒ bổ sung
			changed = true
		}

		if changed {
			repairs = append(repairs, repair{pageID: pageID, domain: domain, newSettings: domSettings})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range repairs {
		settingsJSON, err := json.Marshal(r.newSettings)
		if err != nil {
			continue
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE landing_pages
			SET settings = $1, updated_at = NOW()
			WHERE page_id = $2 AND domain = $3
		`, settingsJSON, r.pageID, r.domain)
		if err != nil {
			log.Printf("[WARN] RepairDuplicateLandingSettings: không sửa được %s@%s: %v", r.pageID, r.domain, err)
			continue
		}
		log.Printf("[INFO] RepairDuplicateLandingSettings: đã đồng bộ settings cho landing '%s' (domain=%s)", r.pageID, r.domain)
	}

	return nil
}
