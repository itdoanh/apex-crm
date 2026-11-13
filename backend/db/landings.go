package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"apex-backend/models"

	"github.com/jackc/pgx/v5"
)

// SeedDefaultLanding kiểm tra và tự động khởi tạo landing page mặc định 'mxv_080_online'
func (s *Store) SeedDefaultLanding(ctx context.Context) error {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM landing_pages WHERE page_id = 'mxv_080_online'").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Đọc nội dung file ladi.html mẫu từ disk
	var codeHTML string
	candidates := []string{
		filepath.Join("..", "frontend", "landing", "ladi.html"),
		filepath.Join("frontend", "landing", "ladi.html"),
		filepath.Join("..", "ladi.html"),
		"ladi.html",
	}
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			codeHTML = string(b)
			break
		}
	}

	// Đọc nội dung chucmung.html, tracker.js, config.json nếu có
	var chucmungHTML string
	for _, p := range []string{filepath.Join("..", "frontend", "landing", "chucmung.html"), filepath.Join("frontend", "landing", "chucmung.html"), "chucmung.html"} {
		if b, err := os.ReadFile(p); err == nil {
			chucmungHTML = string(b)
			break
		}
	}

	var trackerJS string
	for _, p := range []string{filepath.Join("..", "frontend", "landing", "tracker.js"), filepath.Join("frontend", "landing", "tracker.js")} {
		if b, err := os.ReadFile(p); err == nil {
			trackerJS = string(b)
			break
		}
	}

	var configJSONStr string
	for _, p := range []string{filepath.Join("..", "frontend", "landing", "config.json"), filepath.Join("frontend", "landing", "config.json")} {
		if b, err := os.ReadFile(p); err == nil {
			configJSONStr = string(b)
			break
		}
	}

	// Đọc config.json mẫu
	defaultSettings := map[string]interface{}{
		"title":            "Hội Thảo Giao Dịch Hàng Hóa - Khóa Học MXV 080",
		"meta_description": "Khóa học thực chiến đầu tư hàng hóa phái sinh cùng chuyên gia hàng đầu. Miễn phí tham gia online qua Zoom.",
		"og_image":         "",
		"hotline":          "0984386538",
		"zalo_link":        "https://zalo.me/0984386538",
		"facebook_pixel_id": "1558828029597854",
		"google_analytics_id": "",
		"tiktok_pixel_id":     "",
		"custom_head_scripts": "<!-- Google Tag & Meta Pixel Auto-Injected -->",
		"custom_body_scripts": "",
		"brand_header": map[string]interface{}{
			"agency_1": "Bộ Công Thương",
			"agency_2": "Sở giao dịch hàng hóa Việt Nam MXV",
			"logo_url": "assets/img/logo_APEX_K_NEN.webp",
			"member_text": "Thành viên 080 MXV",
		},
		"tieu_de": map[string]interface{}{
			"badge":    "LỚP HỌC ONLINE MIỄN PHÍ",
			"line_1":   "GIẢI MÃ ĐẦU TƯ",
			"line_2":   "HÀNG HÓA",
			"subtitle": "Hiểu thị trường – Nắm cơ hội – Quản trị rủi ro",
		},
		"metric_boxes": map[string]interface{}{
			"box_1_top":    "03/11/2026",
			"box_1_bottom": "20:00 Online",
			"box_2_top":    "Học Online",
			"box_2_bottom": "Qua Zoom",
			"box_3_top":    "2 Giờ",
			"box_3_bottom": "Thực chiến",
		},
		"thoi_gian_chia_se": map[string]interface{}{
			"target_date": "2026-11-03T20:00:00",
			"ngay":        "03/11/2026",
			"gio":         "20:00",
			"thoi_luong":  "2 giờ",
			"hien_thi":    "20:00 - Thứ Ba, ngày 03/11/2026",
			"hinh_thuc":   "Học Online qua Zoom",
		},
		"khoa_hoc": map[string]interface{}{
			"ten":         "Giải Mã Đầu Tư Hàng Hóa",
			"suat_hoc":    "Chỉ dành cho 100 suất",
			"da_dang_ky":  "Đã có 79 suất đăng ký",
		},
		"form_config": map[string]interface{}{
			"require_phone":    true,
			"require_name":     true,
			"success_redirect": "chucmung.html",
		},
	}

	settingsJSON, _ := json.Marshal(defaultSettings)

	insertSQL := `
		INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (domain, page_id) DO UPDATE SET
			code_html = EXCLUDED.code_html,
			updated_at = NOW()
	`
	_, err = s.pool.Exec(ctx, insertSQL,
		"mxv_080_online",
		"",
		"Hội Thảo Giao Dịch Hàng Hóa MXV 080 (Mẫu Chính)",
		true,
		"published",
		"chucmung.html",
		"landing_chinh",
		settingsJSON,
		codeHTML,
	)
	if err != nil {
		return fmt.Errorf("lỗi seed landing page mặc định: %w", err)
	}

	// Seed các file và thư mục ban đầu vào landing_files
	fileInsert := `
		INSERT INTO landing_files (page_id, domain, file_path, file_type, content)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (domain, page_id, file_path) DO NOTHING
	`
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "index.html", "html", codeHTML)
	if chucmungHTML != "" {
		_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "chucmung.html", "html", chucmungHTML)
	}
	if trackerJS != "" {
		_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "tracker.js", "js", trackerJS)
	}
	if configJSONStr != "" {
		_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "config.json", "json", configJSONStr)
	}
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "assets", "folder", "")
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "assets/css", "folder", "")
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "assets/js", "folder", "")
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "assets/img", "folder", "")
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "assets/css/style.css", "css", "/* Custom CSS for Apex Landing */\n.btn-cta-glow { box-shadow: 0 0 25px rgba(239, 82, 34, 0.6); }")
	_, _ = s.pool.Exec(ctx, fileInsert, "mxv_080_online", "", "assets/js/main.js", "js", "// Custom Javascript for Apex Landing\nconsole.log('Apex Landing loaded');")

	return nil
}

// GetLandingPage lấy thông tin chi tiết một Landing Page theo domain và pageID
func (s *Store) GetLandingPage(domain string, pageID string) (*models.LandingPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT page_id, domain, name, is_active, status, redirect_url, nguon_default,
		       settings, code_html,
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_pages
		WHERE (domain = $1 OR domain = '') AND page_id = $2
		ORDER BY (CASE WHEN domain = $1 AND $1 != '' THEN 0 ELSE 1 END) ASC, updated_at DESC
		LIMIT 1
	`
	row := s.pool.QueryRow(ctx, query, domain, pageID)

	var p models.LandingPage
	var settingsBytes []byte

	err := row.Scan(&p.PageID, &p.Domain, &p.Name, &p.IsActive, &p.Status, &p.RedirectURL, &p.NguonDefault,
		&settingsBytes, &p.CodeHTML, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	p.Settings = make(map[string]interface{})
	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &p.Settings)
		if pp, ok := p.Settings["path_prefix"].(string); ok && pp != "" {
			p.PathPrefix = pp
		}
	}
	if p.PathPrefix == "" {
		p.PathPrefix = "/"
	}

	return &p, nil
}

// GetLandingPageByID tìm landing theo page_id độc lập với domain
func (s *Store) GetLandingPageByID(pageID string) (*models.LandingPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT page_id, domain, name, is_active, status, redirect_url, nguon_default,
		       settings, code_html,
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_pages
		WHERE page_id = $1
		LIMIT 1
	`
	row := s.pool.QueryRow(ctx, query, pageID)

	var p models.LandingPage
	var settingsBytes []byte

	err := row.Scan(&p.PageID, &p.Domain, &p.Name, &p.IsActive, &p.Status, &p.RedirectURL, &p.NguonDefault,
		&settingsBytes, &p.CodeHTML, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	p.Settings = make(map[string]interface{})
	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &p.Settings)
		if pp, ok := p.Settings["path_prefix"].(string); ok && pp != "" {
			p.PathPrefix = pp
		}
	}
	if p.PathPrefix == "" {
		p.PathPrefix = "/"
	}

	return &p, nil
}

// ListLandingPages trả về danh sách tất cả các landing page kèm số lượng lead và lượt truy cập
//
// ⚠️ DEDUPE THEO `page_id` (fix 2026-11-13):
//   Bảng `landing_pages` có PK composite `(domain, page_id)`. Một landing có thể
//   tồn tại 2 row: bản GỐC (`domain = ''`) + bản theo domain đích
//   (vd: `('doanh_chung','')` và `('doanh_chung','lophoc.hanghoaphaisinh.net')`).
//
//   Query cũ trả về CẢ 2 row ⇒ CRM hiển thị landing bị "nhân đôi" sau mỗi lần
//   thêm domain / deploy landing ⇒ người dùng tưởng hệ thống tự duplicate.
//
//   ⇒ Gom theo `page_id`, ưu tiên row có domain khớp chính xác với route đang
//     hoạt động; nếu không có thì lấy row mới cập nhật nhất.
func (s *Store) ListLandingPages() ([]models.LandingPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	query := `
		SELECT lp.page_id, lp.domain, lp.name, lp.is_active, lp.status, lp.redirect_url, lp.nguon_default,
		       lp.settings,
		       TO_CHAR(lp.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(lp.updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(l_stat.leads_count, 0) as leads_count,
		       COALESCE(s_stat.views_count, 0) as views_count
		FROM landing_pages lp
		LEFT JOIN (
			SELECT landing_id, COUNT(*) as leads_count
			FROM leads
			GROUP BY landing_id
		) l_stat ON l_stat.landing_id = lp.page_id
		LEFT JOIN (
			SELECT landing_id, COUNT(*) as views_count
			FROM analytics_sessions
			GROUP BY landing_id
		) s_stat ON s_stat.landing_id = lp.page_id
		ORDER BY lp.updated_at DESC
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.LandingPage
	for rows.Next() {
		var p models.LandingPage
		var settingsBytes []byte
		err := rows.Scan(
			&p.PageID, &p.Domain, &p.Name, &p.IsActive, &p.Status, &p.RedirectURL, &p.NguonDefault,
			&settingsBytes, &p.CreatedAt, &p.UpdatedAt, &p.LeadsCount, &p.ViewsCount,
		)
		if err != nil {
			return nil, err
		}
		p.Settings = make(map[string]interface{})
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &p.Settings)
			if pp, ok := p.Settings["path_prefix"].(string); ok && pp != "" {
				p.PathPrefix = pp
			}
		}
		if p.PathPrefix == "" {
			p.PathPrefix = "/"
		}
		result = append(result, p)
	}

	return dedupeLandingPagesByPageID(result), nil
}

// dedupeLandingPagesByPageID gom nhiều row cùng `page_id` (bản gốc `domain=''`
// + bản theo domain) thành MỘT entry duy nhất.
//
// Quy tắc chọn dòng thắng:
//  1. Dòng có `domain` khác rỗng (bản đã gán domain) — ưu tiên vì đó là trạng
//     thái đang chạy thật trên domain đích.
//  2. Trong cùng nhóm, lấy dòng `updated_at` mới nhất.
//
// `settings` của các dòng bị gộp được TRỘN lại (giá trị khác rỗng ưu tiên)
// vì bản gốc và bản domain có thể đang lưu 2 nhóm cấu hình khác nhau
// (vd: bản gốc có `title`/`meta_description`, bản domain có `google_sheet_ids`).
// Gộp giữ nguyên mọi cấu hình ⇒ không mất gì.
//
// Dòng bị bỏ qua KHÔNG bị xoá khỏi DB — chỉ ẩn khỏi danh sách để tránh
// hiển thị trùng. Việc xoá dữ liệu production cần anh itdoanh xác nhận.
func dedupeLandingPagesByPageID(list []models.LandingPage) []models.LandingPage {
	if len(list) <= 1 {
		return list
	}

	type entry struct {
		winner  models.LandingPage
		merged  map[string]interface{}
		hasMeta bool
	}
	best := make(map[string]*entry, len(list))
	order := make([]string, 0, len(list))

	for i := range list {
		p := list[i]
		key := strings.ToLower(strings.TrimSpace(p.PageID))
		if key == "" {
			continue
		}
		cur, exists := best[key]
		if !exists {
			cp := p
			best[key] = &entry{winner: cp, merged: mergeSettingsMaps(nil, p.Settings), hasMeta: true}
			order = append(order, key)
			continue
		}

		// Gộp settings (giá trị khác rỗng ưu tiên) — KHÔNG để mất cấu hình.
		cur.merged = mergeSettingsMaps(cur.merged, p.Settings)

		// Ưu tiên bản có domain khác rỗng; nếu cả hai cùng trạng thái thì lấy
		// bản cập nhật mới hơn.
		curHasDomain := strings.TrimSpace(cur.winner.Domain) != ""
		newHasDomain := strings.TrimSpace(p.Domain) != ""
		if newHasDomain && !curHasDomain {
			cur.winner = p
			continue
		}
		if curHasDomain == newHasDomain && p.UpdatedAt > cur.winner.UpdatedAt {
			cur.winner = p
		}
	}

	out := make([]models.LandingPage, 0, len(order))
	for _, key := range order {
		if e, ok := best[key]; ok {
			p := e.winner
			p.Settings = e.merged
			out = append(out, p)
		}
	}
	return out
}

// mergeSettingsMaps trộn 2 map settings, giá trị khác rỗng của map sau ghi đè
// map trước. Các key lồng (map) được trộn đệ quy 1 cấp cho `form_config`.
func mergeSettingsMaps(base, incoming map[string]interface{}) map[string]interface{} {
	if len(incoming) == 0 {
		return base
	}
	out := make(map[string]interface{}, len(base)+len(incoming))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range incoming {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			// Giá trị rỗng không ghi đè giá trị đang có.
			if _, exists := out[k]; exists {
				continue
			}
		}
		if nested, ok := v.(map[string]interface{}); ok {
			if existing, ok2 := out[k].(map[string]interface{}); ok2 {
				out[k] = mergeSettingsMaps(existing, nested)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// GetAllLandingsWithCode tải toàn bộ landing pages kèm code_html phục vụ nạp in-memory cache
func (s *Store) GetAllLandingsWithCode(ctx context.Context) ([]models.LandingPage, error) {
	query := `
		SELECT lp.page_id, lp.domain, lp.name, lp.is_active, lp.status, lp.redirect_url, lp.nguon_default,
		       lp.settings, COALESCE(lp.code_html, ''),
		       TO_CHAR(lp.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(lp.updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_pages lp
		ORDER BY lp.updated_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.LandingPage
	for rows.Next() {
		var p models.LandingPage
		var settingsBytes []byte
		err := rows.Scan(
			&p.PageID, &p.Domain, &p.Name, &p.IsActive, &p.Status, &p.RedirectURL, &p.NguonDefault,
			&settingsBytes, &p.CodeHTML, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		p.Settings = make(map[string]interface{})
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &p.Settings)
			if pp, ok := p.Settings["path_prefix"].(string); ok && pp != "" {
				p.PathPrefix = pp
			}
		}
		if p.PathPrefix == "" {
			p.PathPrefix = "/"
		}
		result = append(result, p)
	}

	return result, nil
}

// SaveLandingPage tạo mới hoặc cập nhật thông tin cấu hình Landing Page.
//
// ⚠️ CHỐNG TẠO LANDING TRÙNG (fix 2026-11-13):
//   `landing_pages` có PK composite `(domain, page_id)` ⇒ cùng 1 `page_id` có thể
//   tồn tại nhiều row: bản GỐC (`domain = ''`) + bản theo domain đích.
//
//   Trước đây mỗi lần bấm "Lưu cài đặt" trong CRM với 1 domain được chọn sẽ
//   INSERT thêm 1 row `(page_id, domain)` ⇒ CRM hiển thị landing bị NHÂN ĐÔI
//   sau mỗi lần thêm miền / deploy landing.
//
//   CÁCH SỬA:
//   - Nếu landing đã có bản GỐC (`domain = ''`) ⇒ CẬP NHẬT bản gốc đó, KHÔNG
//     tạo row domain mới. Việc gắn landing với domain đã do `domain_routes`
//     đảm nhiệm (PK `(domain_id, path_prefix)`) ⇒ không cần row riêng.
//   - Nếu landing chỉ có bản theo domain (không có bản gốc) ⇒ cập nhật bản đó
//     (giữ nguyên hành vi cũ, KHÔNG tạo thêm row).
//   - Nếu bản domain đích đã tồn tại ⇒ cập nhật ĐỒNG THỜI bản gốc và bản domain
//     để 2 bản luôn có cùng nội dung ⇒ không mất cấu hình nào.
func (s *Store) SaveLandingPage(p *models.LandingPage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if p.Settings == nil {
		p.Settings = make(map[string]interface{})
	}
	settingsJSON, err := json.Marshal(p.Settings)
	if err != nil {
		settingsJSON = []byte("{}")
	}

	p.PageID = strings.TrimSpace(p.PageID)
	if p.PageID == "" {
		p.PageID = "landing_" + fmt.Sprintf("%d", time.Now().Unix())
	}
	if p.Status == "" {
		p.Status = "published"
	}
	if p.NguonDefault == "" {
		p.NguonDefault = "landing_chinh"
	}
	p.Domain = strings.ToLower(strings.TrimSpace(p.Domain))

	// Xác định các row (`domain`) cần cập nhật.
	targetDomains := []string{""}
	if p.Domain != "" {
		templateExists := s.landingPageRowExists(ctx, "", p.PageID)
		if templateExists {
			// Bản gốc đã tồn tại ⇒ KHÔNG tạo row domain mới.
			// Nếu bản domain đích đã có (do lần deploy trước tạo ra) thì cập
			// nhật luôn để 2 bản đồng bộ, tránh lệch nội dung.
			if s.landingPageRowExists(ctx, p.Domain, p.PageID) {
				targetDomains = append(targetDomains, p.Domain)
			}
		} else {
			// Chưa có bản gốc ⇒ dùng bản theo domain làm bản chính.
			targetDomains = []string{p.Domain}
		}
	} else if !s.landingPageRowExists(ctx, "", p.PageID) {
		// Không có bản gốc VÀ không chỉ định domain ⇒ landing này đang tồn tại
		// dưới dạng domain-specific (vd `landing_232`). Cập nhật bản đó thay vì
		// tạo thêm bản gốc ⇒ tránh nhân đôi landing.
		if dom, ok := s.findExistingLandingDomain(ctx, p.PageID); ok {
			targetDomains = []string{dom}
		}
	}

	upsertSQL := `
		INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (domain, page_id) DO UPDATE SET
			name = EXCLUDED.name,
			is_active = EXCLUDED.is_active,
			status = EXCLUDED.status,
			redirect_url = EXCLUDED.redirect_url,
			nguon_default = EXCLUDED.nguon_default,
			settings = EXCLUDED.settings,
			code_html = CASE WHEN EXCLUDED.code_html != '' THEN EXCLUDED.code_html ELSE landing_pages.code_html END,
			updated_at = NOW()
	`

	for _, dom := range targetDomains {
		rowSettingsJSON := settingsJSON

		if dom != "" {
			// Bản theo domain có thể chứa cấu hình RIÊNG không có ở bản gốc
			// (vd `path_prefix`, `google_sheet_ids`, `google_sheet_sync_mode`).
			// Upsert thẳng sẽ XOÁ mất các khoá đó ⇒ gộp trước khi ghi.
			merged, err := s.mergeDomainSettings(ctx, dom, p.PageID, p.Settings)
			if err == nil {
				if b, err2 := json.Marshal(merged); err2 == nil {
					rowSettingsJSON = b
				}
			}
		}

		if _, err := s.pool.Exec(ctx, upsertSQL,
			p.PageID, dom, p.Name, p.IsActive, p.Status, p.RedirectURL, p.NguonDefault, rowSettingsJSON, p.CodeHTML,
		); err != nil {
			return err
		}
	}

	// Nếu gán domain cụ thể, tự động đồng bộ tệp tin từ domain rỗng
	if p.Domain != "" {
		_, _ = s.pool.Exec(ctx, `
			INSERT INTO landing_files (page_id, domain, file_path, file_type, content, updated_at)
			SELECT page_id, $1, file_path, file_type, content, NOW()
			FROM landing_files
			WHERE page_id = $2 AND domain = ''
			ON CONFLICT (domain, page_id, file_path) DO NOTHING
		`, p.Domain, p.PageID)
	}

	return nil
}

// landingPageRowExists kiểm tra đã có row `(domain, page_id)` trong landing_pages chưa
func (s *Store) landingPageRowExists(ctx context.Context, domain, pageID string) bool {
	var exists bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM landing_pages WHERE domain = $1 AND page_id = $2)",
		domain, pageID,
	).Scan(&exists)
	return err == nil && exists
}

// mergeDomainSettings gộp `settings` của bản domain hiện có với `settings` mới.
//
// Thứ tự ưu tiên:
//  1. Giá trị mới (p.Settings) — người dùng vừa lưu trên CRM.
//  2. Giá trị đang có ở bản domain (vd `google_sheet_ids` cấu hình riêng cho domain).
//  3. Bỏ qua — không tự ý điền.
//
// Hàm này ngăn việc upsert bản domain làm mất cấu hình domain-specific.
func (s *Store) mergeDomainSettings(ctx context.Context, domain, pageID string, incoming map[string]interface{}) (map[string]interface{}, error) {
	var existingBytes []byte
	err := s.pool.QueryRow(ctx,
		"SELECT settings FROM landing_pages WHERE domain = $1 AND page_id = $2",
		domain, pageID,
	).Scan(&existingBytes)
	if err != nil {
		// Chưa có bản domain ⇒ dùng luôn settings mới.
		return incoming, nil
	}

	existing := make(map[string]interface{})
	if len(existingBytes) > 0 {
		_ = json.Unmarshal(existingBytes, &existing)
	}

	// Bản domain là "mặc định" cũ; settings mới ghi đè lên.
	merged := mergeSettingsMaps(existing, incoming)
	return merged, nil
}

// findExistingLandingDomain tìm domain của row landing đang tồn tại (bản domain-specific).
// Trả về domain mới cập nhật nhất.
func (s *Store) findExistingLandingDomain(ctx context.Context, pageID string) (string, bool) {
	var domain string
	err := s.pool.QueryRow(ctx, `
		SELECT domain FROM landing_pages
		WHERE page_id = $1 AND domain <> ''
		ORDER BY updated_at DESC
		LIMIT 1
	`, pageID).Scan(&domain)
	if err != nil {
		return "", false
	}
	return domain, true
}

// UpdateLandingCode cập nhật mã code HTML hoặc deploy của landing page
func (s *Store) UpdateLandingCode(domain string, pageID string, codeHTML string, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE landing_pages
		SET code_html = $1, status = $2, updated_at = NOW()
		WHERE (domain = $3 OR domain = '') AND page_id = $4
	`
	_, err := s.pool.Exec(ctx, query, codeHTML, status, domain, pageID)
	if err != nil {
		return err
	}

	// Cập nhật luôn file index.html trong landing_files
	fileQuery := `
		INSERT INTO landing_files (page_id, domain, file_path, file_type, content, updated_at)
		VALUES ($1, $2, 'index.html', 'html', $3, NOW())
		ON CONFLICT (domain, page_id, file_path) DO UPDATE SET
			content = EXCLUDED.content,
			updated_at = NOW()
	`
	_, _ = s.pool.Exec(ctx, fileQuery, pageID, domain, codeHTML)
	return nil
}

// ToggleLandingActive bật hoặc tắt kích hoạt Landing Page
func (s *Store) ToggleLandingActive(domain string, pageID string, isActive bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE landing_pages
		SET is_active = $1, updated_at = NOW()
		WHERE (domain = $2 OR domain = '') AND page_id = $3
	`
	_, err := s.pool.Exec(ctx, query, isActive, domain, pageID)
	return err
}

// DeleteLandingPage xóa 1 landing page và các tệp tin của nó
func (s *Store) DeleteLandingPage(domain string, pageID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Xóa files
	_, _ = s.pool.Exec(ctx, "DELETE FROM landing_files WHERE (domain = $1 OR domain = '') AND page_id = $2", domain, pageID)

	// Xóa landing page
	_, err := s.pool.Exec(ctx, "DELETE FROM landing_pages WHERE (domain = $1 OR domain = '') AND page_id = $2", domain, pageID)
	return err
}

// DuplicateLandingPage nhân bản 1 landing page sang pageID mới
func (s *Store) DuplicateLandingPage(sourceDomain string, sourcePageID string, newPageID string, newName string) error {
	src, err := s.GetLandingPage(sourceDomain, sourcePageID)
	if err != nil || src == nil {
		return fmt.Errorf("không tìm thấy landing page nguồn để nhân bản: %v", err)
	}

	newPage := *src
	newPage.PageID = newPageID
	newPage.Name = newName
	newPage.Status = "draft"
	newPage.IsActive = true

	if err := s.SaveLandingPage(&newPage); err != nil {
		return err
	}
	if src.CodeHTML != "" {
		_ = s.UpdateLandingCode(newPage.Domain, newPage.PageID, src.CodeHTML, "draft")
	}

	// Sao chép các tệp tin phụ
	files, _ := s.ListLandingFiles(sourceDomain, sourcePageID)
	for _, f := range files {
		_ = s.SaveLandingFile(newPage.Domain, newPage.PageID, f.FilePath, f.FileType, f.Content)
	}

	return nil
}

// ============================================================
// FILE MANAGER DATABASE OPERATIONS
// ============================================================

// ListLandingFiles trả về danh toàn bộ file của landing theo domain + page_id.
//
// LƯU Ý: dùng DISTINCT ON (file_path) để mỗi đường dẫn chỉ xuất hiện 1 lần.
// Trước đây query trả về cả bản domain='' và bản domain=<cụ thể> cho cùng một
// file_path → Code Studio có thể nạp nhầm nội dung của template gốc (domain rỗng)
// thay vì nội dung của landing đang mở (domain cụ thể) ⇒ preview hiển thị sai landing.
func (s *Store) ListLandingFiles(domain string, pageID string) ([]models.LandingFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT DISTINCT ON (file_path) id, page_id, domain, file_path, file_type, content,
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $2
		ORDER BY file_path, (CASE WHEN domain = $1 AND $1 != '' THEN 0 ELSE 1 END) ASC, updated_at DESC
	`
	rows, err := s.pool.Query(ctx, query, domain, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.LandingFile
	for rows.Next() {
		var f models.LandingFile
		if err := rows.Scan(&f.ID, &f.PageID, &f.Domain, &f.FilePath, &f.FileType, &f.Content, &f.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, nil
}

// GetLandingFile lấy nội dung 1 tệp tin.
//
// ⚠️ THỨ TỰ ƯU TIÊN (bắt buộc — bug 2026-11-05):
// Query cũ dùng `WHERE (domain = $1 OR domain = '') ... LIMIT 1` KHÔNG có ORDER BY
// ⇒ PostgreSQL trả về bản ghi TÙY Ý (thực tế là bản có id nhỏ nhất ⇒ bản CŨ nhất
// trong cùng nhóm). Hậu quả trên production:
//
//	domain=ads.giaodichhanghoatuonglai.com, page_id=landing_cu, file_path=tracker.js
//	  id=48  (domain CHÍNH XÁC, len=60022, mới, có LEAD_DEDUP_WINDOW_MS)
//	  id=399 (domain=''      , len=57349, CŨ, KHÔNG có LEAD_DEDUP_WINDOW_MS)
//	⇒ Edge Router phục vụ id=399 ⇒ landing chạy tracker.js CŨ ⇒ mất cơ chế
//	  khóa chống gửi trùng ở client ⇒ 1 lần bấm sinh 3 POST /api/leads ⇒
//	  lead THẬT bị ghi vào spam_quarantine với lý do "Lỗi Proof-of-Work".
//
// ⇒ BẮT BUỘC ưu tiên bản có `domain` KHỚP CHÍNH XÁC, sau đó mới tới bản
// `domain = ''` (bản dùng chung), và trong cùng nhóm thì lấy bản CẬP NHẬT
// GẦN NHẤT. Cùng thứ tự này đã được dùng ở `GetLandingPage` — giữ nhất quán.
func (s *Store) GetLandingFile(domain string, pageID string, filePath string) (*models.LandingFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, page_id, domain, file_path, file_type, content,
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $2 AND file_path = $3
		ORDER BY (CASE WHEN domain = $1 AND $1 <> '' THEN 0 ELSE 1 END) ASC, updated_at DESC
		LIMIT 1
	`
	row := s.pool.QueryRow(ctx, query, domain, pageID, filePath)
	var f models.LandingFile
	err := row.Scan(&f.ID, &f.PageID, &f.Domain, &f.FilePath, &f.FileType, &f.Content, &f.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// SaveLandingFile tạo mới hoặc lưu nội dung tệp tin
func (s *Store) SaveLandingFile(domain string, pageID string, filePath string, fileType string, content string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filePath = strings.TrimPrefix(filePath, "/")
	if fileType == "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".html", ".htm":
			fileType = "html"
		case ".css":
			fileType = "css"
		case ".js":
			fileType = "js"
		case ".json":
			fileType = "json"
		default:
			fileType = "text"
		}
	}

	query := `
		INSERT INTO landing_files (page_id, domain, file_path, file_type, content, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (domain, page_id, file_path) DO UPDATE SET
			file_type = EXCLUDED.file_type,
			content = EXCLUDED.content,
			updated_at = NOW()
	`
	_, err := s.pool.Exec(ctx, query, pageID, domain, filePath, fileType, content)
	return err
}

// SyncLandingFilesToDomain đồng bộ toàn bộ file và thư mục từ sourceDomain (hoặc domain rỗng mặc định) sang targetDomain
func (s *Store) SyncLandingFilesToDomain(sourceDomain, targetDomain, pageID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if targetDomain == "" || pageID == "" {
		return nil
	}

	query := `
		INSERT INTO landing_files (page_id, domain, file_path, file_type, content, updated_at)
		SELECT DISTINCT ON (file_path) page_id, $2, file_path, file_type, content, NOW()
		FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $3
		ORDER BY file_path, (CASE WHEN domain = $1 AND $1 != '' THEN 0 ELSE 1 END) ASC, updated_at DESC
		ON CONFLICT (domain, page_id, file_path) DO UPDATE SET
			file_type = EXCLUDED.file_type,
			content = EXCLUDED.content,
			updated_at = NOW()
	`
	_, err := s.pool.Exec(ctx, query, sourceDomain, targetDomain, pageID)
	return err
}

// SyncSharedStaticFilesToDB đồng bộ các file tĩnh dùng chung (tracker.js, config.json...)
// từ đĩa server vào bảng `landing_files` cho MỌI landing đang tồn tại.
//
// BỐI CẢNH (bug đã gặp 2026-11-04):
// Khi landing được deploy, toàn bộ file của landing được copy vào bảng `landing_files`.
// Edge Router phục vụ file theo thứ tự: `landing_files` (DB) TRƯỚC, rồi mới fallback tới đĩa.
// ⇒ Khi sửa `tracker.js` trong git rồi deploy, file trên đĩa đã mới nhưng bản trong DB vẫn cũ
// ⇒ production vẫn serve bản cũ, thay đổi "không có tác dụng".
//
// Hàm này chạy khi backend khởi động: đọc file tĩnh dùng chung từ đĩa và ghi đè vào DB
// cho tất cả landing, đảm bảo code trong git là nguồn sự thật duy nhất.
func (s *Store) SyncSharedStaticFilesToDB(ctx context.Context, diskRoot string) error {
	// Danh sách file tĩnh dùng chung cần đồng bộ từ đĩa → DB
	//
	// ⚠️ CHỈ đồng bộ `tracker.js`. KHÔNG đồng bộ `config.json`!
	//
	// LÝ DO (bug chồng chéo landing — 2026-11-05):
	// `config.json` chứa `landing_id` của landing mẫu (`mxv_080_online`). Nếu copy
	// file này cho MỌI landing thì `landing_cu` cũng nhận `landing_id: mxv_080_online`
	// ⇒ tracker.js gửi analytics/lead về nhầm landing ⇒ số liệu view/lead của các
	// landing bị trộn lẫn (landing_cu ghi nhận view của mxv_080_online và ngược lại).
	//
	// `config.json` là file RIÊNG của từng landing (nằm trong cây File Manager),
	// được quản lý qua CRM → Code Studio. Backend sẽ tự sinh `config.json` đúng
	// `landing_id` khi landing chưa có file này (xem `EnsureLandingConfigFile`).
	sharedFiles := []struct {
		relPath  string
		fileType string
	}{
		{"tracker.js", "js"},
	}

	// Thư mục chứa file tĩnh dùng chung (ưu tiên frontend/landing)
	searchDirs := []string{
		filepath.Join(diskRoot, "frontend", "landing"),
		filepath.Join(diskRoot, "..", "frontend", "landing"),
		diskRoot,
		filepath.Join(diskRoot, ".."),
	}

	// Lấy danh sách landing cần đồng bộ.
	//
	// ⚠️ PHẢI lấy HỢP của `landing_pages` VÀ `landing_files` (bug 2026-11-05).
	//
	// Query cũ chỉ đọc `landing_pages`. Nhưng `landing_files` chứa nhiều cặp
	// (domain, page_id) KHÔNG tồn tại trong `landing_pages` — ví dụ production:
	//
	//	landing_pages : ('','mxv_080_online'), ('ads...','landing_cu'), ('ads...','landing_lznoew')
	//	landing_files  : ('','landing_cu')                              ← THIẾU trong landing_pages
	//	                 ('hanghoaphaisinh.net','mxv_080_online')     ← THIẾU trong landing_pages
	//
	// ⇒ 2 bản `tracker.js` đó giữ nguyên bản CŨ ⇒ landing chạy code cũ ⇒
	//   mất cơ chế khóa chống gửi trùng ở client ⇒ 1 lần bấm sinh 3 POST
	//   /api/leads ⇒ lead THẬT bị ghi vào spam_quarantine ("Lỗi Proof-of-Work").
	//
	// ⇒ Lấy UNION của cả hai nguồn để KHÔNG bản ghi nào bị bỏ sót.
	rows, err := s.pool.Query(ctx, `
		SELECT domain, page_id FROM landing_pages
		UNION
		SELECT domain, page_id FROM landing_files
	`)
	if err != nil {
		return err
	}
	type landingKey struct{ domain, pageID string }
	var landings []landingKey
	for rows.Next() {
		var k landingKey
		if err := rows.Scan(&k.domain, &k.pageID); err == nil {
			landings = append(landings, k)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	updated := 0
	for _, sf := range sharedFiles {
		var content string
		found := false
		for _, dir := range searchDirs {
			p := filepath.Join(dir, sf.relPath)
			data, err := os.ReadFile(p)
			if err == nil && len(data) > 0 {
				content = string(data)
				found = true
				break
			}
		}
		if !found {
			log.Printf("[WARN] SyncStatic: không tìm thấy %s trên đĩa, bỏ qua", sf.relPath)
			continue
		}

		for _, lk := range landings {
			if err := s.SaveLandingFile(lk.domain, lk.pageID, sf.relPath, sf.fileType, content); err == nil {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("[INFO] SyncStatic: đã đồng bộ %d bản ghi landing_files từ đĩa (tracker.js)", updated)
	}
	return nil
}

// EnsureLandingConfigFile đảm bảo mỗi landing có file `config.json` RIÊNG với
// `landing_id` đúng của nó.
//
// BỐI CẢNH (bug chồng chéo landing — 2026-11-05):
// `config.json` từng được copy chung cho mọi landing ⇒ mọi landing đều có
// `landing_id: "mxv_080_online"` ⇒ tracker.js gửi analytics/lead về nhầm landing.
//
// Hàm này chạy sau khi backend khởi động: với mỗi landing, nếu `config.json`
// đang tồn tại mà `landing_id` trong đó KHÁC với `page_id` của landing ⇒ tự
// động sửa lại `landing_id` cho đúng (giữ nguyên các key khác như `nguon`,
// `thoi_gian_chia_se`, `khoa_hoc`, `backend_url` do người dùng cấu hình).
func (s *Store) EnsureLandingConfigFile(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, "SELECT DISTINCT domain, page_id FROM landing_pages")
	if err != nil {
		return err
	}
	type landingKey struct{ domain, pageID string }
	var landings []landingKey
	for rows.Next() {
		var k landingKey
		if err := rows.Scan(&k.domain, &k.pageID); err == nil {
			landings = append(landings, k)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	fixed := 0
	for _, lk := range landings {
		f, err := s.GetLandingFile(lk.domain, lk.pageID, "config.json")
		if err != nil || f == nil || strings.TrimSpace(f.Content) == "" {
			continue
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(f.Content), &cfg); err != nil {
			continue
		}
		current, _ := cfg["landing_id"].(string)
		if current == lk.pageID {
			continue
		}
		cfg["landing_id"] = lk.pageID
		newContent, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			continue
		}
		if err := s.SaveLandingFile(lk.domain, lk.pageID, "config.json", "json", string(newContent)); err == nil {
			fixed++
			log.Printf("[INFO] EnsureLandingConfigFile: đã sửa landing_id của config.json '%s' (domain='%s'): '%s' -> '%s'",
				lk.pageID, lk.domain, current, lk.pageID)
		}
	}

	if fixed > 0 {
		log.Printf("[INFO] EnsureLandingConfigFile: đã sửa %d config.json có landing_id sai", fixed)
	}
	return nil
}

// DeleteLandingFile xóa 1 tệp tin khỏi landing page
func (s *Store) DeleteLandingFile(domain string, pageID string, filePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if filePath == "index.html" {
		return fmt.Errorf("không thể xóa tệp tin chính index.html")
	}

	query := `
		DELETE FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $2 AND file_path = $3
	`
	_, err := s.pool.Exec(ctx, query, domain, pageID, filePath)
	return err
}

// CreateLandingFolder tạo một thư mục mới cho landing page
func (s *Store) CreateLandingFolder(domain string, pageID string, folderPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	folderPath = strings.Trim(strings.TrimSpace(folderPath), "/")
	if folderPath == "" {
		return fmt.Errorf("đường dẫn thư mục không được để trống")
	}
	if strings.Contains(folderPath, "..") {
		return fmt.Errorf("đường dẫn thư mục không hợp lệ")
	}

	query := `
		INSERT INTO landing_files (page_id, domain, file_path, file_type, content, updated_at)
		VALUES ($1, $2, $3, 'folder', '', NOW())
		ON CONFLICT (domain, page_id, file_path) DO NOTHING
	`
	_, err := s.pool.Exec(ctx, query, pageID, domain, folderPath)
	return err
}

// DeleteLandingFolder xóa một thư mục và toàn bộ tệp tin, thư mục con bên trong
func (s *Store) DeleteLandingFolder(domain string, pageID string, folderPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	folderPath = strings.Trim(strings.TrimSpace(folderPath), "/")
	if folderPath == "" {
		return fmt.Errorf("không thể xóa thư mục gốc")
	}

	pattern := folderPath + "/%"
	query := `
		DELETE FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $2
		  AND (file_path = $3 OR file_path LIKE $4)
		  AND file_path != 'index.html'
	`
	_, err := s.pool.Exec(ctx, query, domain, pageID, folderPath, pattern)
	return err
}

// RenameLandingFolder đổi tên thư mục và cập nhật toàn bộ đường dẫn con
func (s *Store) RenameLandingFolder(domain string, pageID string, oldPath string, newPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oldPath = strings.Trim(strings.TrimSpace(oldPath), "/")
	newPath = strings.Trim(strings.TrimSpace(newPath), "/")
	if oldPath == "" || newPath == "" {
		return fmt.Errorf("đường dẫn thư mục không được để trống")
	}
	if oldPath == newPath {
		return nil
	}
	if strings.Contains(newPath, "..") {
		return fmt.Errorf("đường dẫn mới không hợp lệ")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Đổi tên record thư mục chính nó
	updateFolderQuery := `
		UPDATE landing_files
		SET file_path = $1, updated_at = NOW()
		WHERE (domain = $2 OR domain = '') AND page_id = $3 AND file_path = $4
	`
	_, err = tx.Exec(ctx, updateFolderQuery, newPath, domain, pageID, oldPath)
	if err != nil {
		return err
	}

	// 2. Cập nhật tất cả các file/thư mục con bên trong
	pattern := oldPath + "/%"
	updateChildrenQuery := `
		UPDATE landing_files
		SET file_path = $1 || SUBSTRING(file_path FROM CAST($2 AS integer)),
		    updated_at = NOW()
		WHERE (domain = $3 OR domain = '') AND page_id = $4 AND file_path LIKE $5
	`
	offset := int32(len(oldPath) + 1)
	_, err = tx.Exec(ctx, updateChildrenQuery, newPath, offset, domain, pageID, pattern)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RenameLandingFile đổi tên 1 tệp tin
func (s *Store) RenameLandingFile(domain string, pageID string, oldPath string, newPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oldPath = strings.Trim(strings.TrimSpace(oldPath), "/")
	newPath = strings.Trim(strings.TrimSpace(newPath), "/")
	if oldPath == "" || newPath == "" {
		return fmt.Errorf("đường dẫn tệp tin không được để trống")
	}
	if oldPath == "index.html" {
		return fmt.Errorf("không thể đổi tên tệp tin chính index.html")
	}
	if newPath == "index.html" {
		return fmt.Errorf("không thể đổi tên thành index.html trùng lặp")
	}
	if strings.Contains(newPath, "..") {
		return fmt.Errorf("đường dẫn mới không hợp lệ")
	}

	ext := strings.ToLower(filepath.Ext(newPath))
	fileType := "text"
	switch ext {
	case ".html", ".htm":
		fileType = "html"
	case ".css":
		fileType = "css"
	case ".js":
		fileType = "js"
	case ".json":
		fileType = "json"
	}

	query := `
		UPDATE landing_files
		SET file_path = $1, file_type = $2, updated_at = NOW()
		WHERE (domain = $3 OR domain = '') AND page_id = $4 AND file_path = $5
	`
	cmdTag, err := s.pool.Exec(ctx, query, newPath, fileType, domain, pageID, oldPath)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("tệp tin không tồn tại để đổi tên")
	}
	return nil
}
