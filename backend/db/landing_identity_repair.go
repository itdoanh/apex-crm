package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// ============================================================
// SỬA CHỮA ĐỊNH DANH LANDING BỊ CHỒNG CHÉO (fix 2026-11-15)
// ============================================================
//
// BỐI CẢNH (bug anh itdoanh báo):
//   `xu_huong_1` (deploy trên `xuhuong.hanghoaphaisinh.net`) được nhân bản từ
//   `doanh_chung` (deploy trên `lophoc.hanghoaphaisinh.net`) nhưng vẫn giữ:
//     - `nguon_default = "DTN 1"` (nguồn của landing nguồn)
//     - `config.json` có `landing_id: "doanh_chung"` + `nguon: "doanh_chung"`
//     - `index.html` có `data-landing-id="doanh_chung" data-nguon="doanh_chung"`
//
//   ⇒ Mọi view + lead của `xu_huong_1` đều bị ghi nhận dưới `doanh_chung`.
//     Không tách được số liệu giữa 2 landing.
//
// CÁCH SỬA:
//   `RepairLandingIdentity` quét TOÀN BỘ landing và:
//     1. Nếu `nguon_default` của landing A trùng với `nguon_default` của
//        landing B (2 landing khác nhau dùng chung 1 nguồn) ⇒ đặt lại theo
//        `page_id` (giữ 1 landing gốc, các landing còn lại tách ra).
//     2. Sửa `config.json` (`landing_id` + `nguon`) về đúng landing.
//     3. Sửa HTML (`data-landing-id`, `data-nguon`, `APEX_LANDING_CONFIG`).
//     4. Sửa `landing_pages.code_html` (HTML được inject khi phục vụ).
//
// ⚠️ Hàm này CHỈ sửa dữ liệu ĐỊNH DANH — KHÔNG đụng nội dung, KHÔNG xoá lead,
//    KHÔNG đụng Google Sheets. An toàn chạy lại nhiều lần (idempotent).
// ============================================================

// RepairLandingIdentity sửa chữa định danh của toàn bộ landing bị chồng chéo.
//
// Trả về số lượng bản ghi đã được sửa (để log + báo cáo).
func (s *Store) RepairLandingIdentity(ctx context.Context) (int, error) {
	type landingRow struct {
		Domain string
		PageID string
		Nguon  string
		HTML   string
	}

	rows, err := s.pool.Query(ctx, `
		SELECT domain, page_id, nguon_default, COALESCE(code_html, '')
		FROM landing_pages
		ORDER BY page_id, domain
	`)
	if err != nil {
		return 0, err
	}

	var landings []landingRow
	nguonCount := map[string]int{}
	nguonOwner := map[string]string{}

	for rows.Next() {
		var r landingRow
		if err := rows.Scan(&r.Domain, &r.PageID, &r.Nguon, &r.HTML); err != nil {
			continue
		}
		landings = append(landings, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Đếm số landing dùng chung mỗi `nguon_default`.
	//
	// ⚠️ Chỉ tính landing KHÁC NHAU: 1 landing có nhiều row (domain='' +
	//    domain cụ thể) vẫn là 1 landing ⇒ không tính là trùng.
	seenPerLanding := map[string]bool{}
	for _, r := range landings {
		key := strings.ToLower(strings.TrimSpace(r.PageID))
		if seenPerLanding[key] {
			continue
		}
		seenPerLanding[key] = true
		n := strings.TrimSpace(r.Nguon)
		if n == "" {
			continue
		}
		nguonCount[n]++
		if _, ok := nguonOwner[n]; !ok {
			nguonOwner[n] = r.PageID
		}
	}

	// Tính `nguon` đúng cho từng landing.
	//
	//   - `nguon` trùng giữa NHIỀU landing ⇒ chỉ giữ 1 landing gốc (landing
	//     đứng đầu theo thứ tự), các landing còn lại tách ra theo `page_id`.
	//   - `nguon` rỗng / "landing_chinh" ⇒ đặt theo `page_id`.
	correctNguon := map[string]string{}
	for _, r := range landings {
		key := strings.ToLower(strings.TrimSpace(r.PageID))
		if _, ok := correctNguon[key]; ok {
			continue
		}
		n := strings.TrimSpace(r.Nguon)
		switch {
		case n == "" || n == "landing_chinh":
			correctNguon[key] = r.PageID
		case nguonCount[n] > 1 && nguonOwner[n] != r.PageID:
			// Trùng với landing khác ⇒ tách riêng.
			correctNguon[key] = r.PageID
		default:
			correctNguon[key] = n
		}
	}

	// Sửa `landing_pages.nguon_default`.
	fixedRows := 0
	for _, r := range landings {
		want := correctNguon[strings.ToLower(strings.TrimSpace(r.PageID))]
		if want == "" || want == strings.TrimSpace(r.Nguon) {
			continue
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE landing_pages SET nguon_default = $1, updated_at = NOW() WHERE domain = $2 AND page_id = $3`,
			want, r.Domain, r.PageID); err != nil {
			log.Printf("[WARN] RepairLandingIdentity: không sửa được nguon_default của '%s' (domain='%s'): %v",
				r.PageID, r.Domain, err)
			continue
		}
		fixedRows++
		log.Printf("[INFO] RepairLandingIdentity: nguon_default của landing '%s' (domain='%s'): '%s' -> '%s'",
			r.PageID, r.Domain, r.Nguon, want)
	}

	// Sửa `landing_files` (config.json + HTML) và `code_html`.
	fixedFiles := 0
	seenLanding := map[string]bool{}

	for _, r := range landings {
		key := strings.ToLower(strings.TrimSpace(r.PageID))
		if seenLanding[key] {
			continue
		}
		seenLanding[key] = true

		wantNguon := correctNguon[key]
		if wantNguon == "" {
			wantNguon = r.PageID
		}

		// 4a. `code_html` của landing.
		if strings.TrimSpace(r.HTML) != "" {
			rewritten := RewriteLandingIdentityHTML(r.HTML, r.PageID, wantNguon, "", "")
			if rewritten != r.HTML {
				if _, err := s.pool.Exec(ctx,
					`UPDATE landing_pages SET code_html = $1, updated_at = NOW() WHERE domain = $2 AND page_id = $3`,
					rewritten, r.Domain, r.PageID); err != nil {
					log.Printf("[WARN] RepairLandingIdentity: không sửa được code_html của '%s': %v", r.PageID, err)
				} else {
					fixedFiles++
				}
			}
		}

		// 4b. Các file trong `landing_files` của landing.
		type fileRow struct {
			domain, filePath, fileType, content string
		}
		fileRows, err := s.pool.Query(ctx, `
			SELECT domain, file_path, file_type, content
			FROM landing_files
			WHERE page_id = $1
			ORDER BY file_path
		`, r.PageID)
		if err != nil {
			continue
		}
		var files []fileRow
		for fileRows.Next() {
			var fr fileRow
			if err := fileRows.Scan(&fr.domain, &fr.filePath, &fr.fileType, &fr.content); err != nil {
				continue
			}
			files = append(files, fr)
		}
		fileRows.Close()

		for _, fr := range files {
			var rewritten string
			switch strings.ToLower(fr.filePath) {
			case "config.json":
				rewritten = RewriteLandingConfigJSON(fr.content, r.PageID, wantNguon)
			default:
				if !strings.HasSuffix(strings.ToLower(fr.filePath), ".html") {
					continue
				}
				rewritten = RewriteLandingIdentityHTML(fr.content, r.PageID, wantNguon, "", "")
			}
			if rewritten == fr.content {
				continue
			}
			if _, err := s.pool.Exec(ctx, `
				UPDATE landing_files
				SET content = $1, updated_at = NOW()
				WHERE domain = $2 AND page_id = $3 AND file_path = $4
			`, rewritten, fr.domain, r.PageID, fr.filePath); err != nil {
				log.Printf("[WARN] RepairLandingIdentity: không sửa được file '%s' của landing '%s': %v",
					fr.filePath, r.PageID, err)
				continue
			}
			fixedFiles++
			log.Printf("[INFO] RepairLandingIdentity: đã sửa định danh trong file '%s' của landing '%s' (domain='%s')",
				fr.filePath, r.PageID, fr.domain)
		}
	}

	total := fixedRows + fixedFiles
	if total > 0 {
		log.Printf("[INFO] RepairLandingIdentity: đã sửa %d row landing_pages + %d file landing_files", fixedRows, fixedFiles)
	}
	return total, nil
}

// setConfigJSONNguon đặt `nguon` trong config.json (JSON hợp lệ).
func setConfigJSONNguon(content string, nguon string) string {
	nguon = strings.TrimSpace(nguon)
	if content == "" || nguon == "" {
		return content
	}
	// BOM làm `json.Unmarshal` fail ⇒ cắt BOM trước khi parse.
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(StripUTF8BOM(content)), &cfg); err != nil {
		return reConfigJSONNguon.ReplaceAllString(content, `"nguon": "`+nguon+`"`)
	}
	cfg["nguon"] = nguon
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return content
	}
	return string(out)
}

// LandingIdentityReport trả về báo cáo định danh của toàn bộ landing (để CRM
// hiển thị cảnh báo khi phát hiện landing dùng chung nguồn).
//
// Kiểu trả về: map[page_id] -> thông tin.
func (s *Store) LandingIdentityReport(ctx context.Context) (map[string]map[string]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT page_id,
		       MIN(COALESCE(nguon_default, '')) AS nguon,
		       COUNT(DISTINCT domain) AS so_domain
		FROM landing_pages
		GROUP BY page_id
		ORDER BY page_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := map[string]map[string]string{}
	nguonUsage := map[string][]string{}

	for rows.Next() {
		var pageID, nguon string
		var soDomain int
		if err := rows.Scan(&pageID, &nguon, &soDomain); err != nil {
			continue
		}
		report[pageID] = map[string]string{
			"nguon":     nguon,
			"so_domain": fmt.Sprintf("%d", soDomain),
		}
		if nguon != "" {
			nguonUsage[nguon] = append(nguonUsage[nguon], pageID)
		}
	}

	// Đánh dấu nguồn bị dùng chung.
	for nguon, ids := range nguonUsage {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			report[id]["canh_bao"] = "nguồn '" + nguon + "' đang dùng chung với: " + strings.Join(ids, ", ")
		}
	}

	return report, nil
}
