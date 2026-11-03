package db

import (
	"context"
	"encoding/json"
	"fmt"
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
		ORDER BY domain DESC
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

	return result, nil
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

// SaveLandingPage tạo mới hoặc cập nhật thông tin cấu hình Landing Page
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

	query := `
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

	_, err = s.pool.Exec(ctx, query,
		p.PageID, p.Domain, p.Name, p.IsActive, p.Status, p.RedirectURL, p.NguonDefault, settingsJSON, p.CodeHTML,
	)
	if err != nil {
		return err
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

// ListLandingFiles liệt kê toàn bộ tệp tin thuộc landing page
func (s *Store) ListLandingFiles(domain string, pageID string) ([]models.LandingFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, page_id, domain, file_path, file_type, content,
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $2
		ORDER BY file_path ASC
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

// GetLandingFile lấy nội dung 1 tệp tin
func (s *Store) GetLandingFile(domain string, pageID string, filePath string) (*models.LandingFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, page_id, domain, file_path, file_type, content,
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM landing_files
		WHERE (domain = $1 OR domain = '') AND page_id = $2 AND file_path = $3
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
