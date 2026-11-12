package db

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// SeedDoanhChungLanding tự động khởi tạo landing page `doanh_chung` từ mã nguồn
// trong thư mục `frontend/landing/doanh_chung`.
//
// BỐI CẢNH (2026-11-12):
//   Landing `doanh_chung` (buổi chia sẻ đầu tư hàng hóa phái sinh) được tạo
//   từ bộ mã nguồn gốc ở `C:\code\apex_moi\apex\chungkhoan\chung`.
//
//   YÊU CẦU TÁCH BIỆT HOÀN TOÀN:
//     - Mọi thông số tracking (visitor_id, cookie, localStorage) phải gắn với
//       `landing_id = doanh_chung` ⇒ không lẫn với landing khác.
//     - Lead phải đi qua `POST /api/leads` của Apex (có PoW + honeypot + dedupe
//       + geo + Google Sheets), KHÔNG bypass qua Google Apps Script.
//
// CƠ CHẾ:
//   1. Nếu landing đã tồn tại trong `landing_pages` ⇒ KHÔNG ghi đè code_html
//      (để không mất chỉnh sửa của anh itdoanh trong Code Studio).
//   2. Nếu chưa tồn tại ⇒ tạo landing_pages + nạp TOÀN BỘ cây file từ đĩa
//      (kể cả file nhị phân: ảnh, video, font — lưu base64 trong `landing_files`).
func (s *Store) SeedDoanhChungLanding(ctx context.Context) error {
	const pageID = "doanh_chung"

	// 1. Kiểm tra đã tồn tại chưa
	var count int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM landing_pages WHERE page_id = $1", pageID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[INFO] SeedDoanhChungLanding: landing '%s' đã tồn tại — giữ nguyên nội dung", pageID)
		return nil
	}

	// 2. Tìm thư mục nguồn trên đĩa
	srcDir := findLandingSourceDir("doanh_chung")
	if srcDir == "" {
		log.Printf("[WARN] SeedDoanhChungLanding: không tìm thấy thư mục nguồn doanh_chung trên đĩa — bỏ qua seed")
		return nil
	}

	// 3. Đọc file chính (index.html = chung.html)
	indexPath := filepath.Join(srcDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		// Bản gốc đặt tên file là `chung.html` ⇒ đổi tên sang `index.html` khi nạp.
		altPath := filepath.Join(srcDir, "chung.html")
		if _, err2 := os.Stat(altPath); err2 != nil {
			log.Printf("[WARN] SeedDoanhChungLanding: không tìm thấy index.html/chung.html trong %s", srcDir)
			return nil
		}
		indexPath = altPath
	}

	codeHTMLBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("lỗi đọc %s: %w", indexPath, err)
	}
	codeHTML := string(codeHTMLBytes)

	// 4. Cấu hình mặc định cho landing
	defaultSettings := map[string]interface{}{
		"title":            "Tối Ưu Dòng Tiền Đầu Tư 2026 | Buổi Chia Sẻ Trực Tuyến APEX Fintech - MXV",
		"meta_description": "Buổi chia sẻ trực tuyến miễn phí 2 giờ cùng Chuyên gia Nguyễn Tuấn Anh. Học cách giao dịch hàng hóa phái sinh trong phiên, quản trị vốn và rủi ro.",
		"og_image":         "assets/img/apex_top1_thiphan_quy2_2026.webp",
		"hotline":          "0984386538",
		"zalo_link":        "https://zalo.me/0984386538",
		"facebook_pixel_id": "",
		"google_analytics_id": "",
		"tiktok_pixel_id":     "",
		"custom_head_scripts": "",
		"custom_body_scripts": "",
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
			name = EXCLUDED.name,
			updated_at = NOW()
	`
	if _, err := s.pool.Exec(ctx, insertSQL,
		pageID,
		"",
		"Buổi Chia Sẻ Đầu Tư Hàng Hóa Phái Sinh (doanh_chung)",
		true,
		"published",
		"chucmung.html",
		"doanh_chung",
		settingsJSON,
		codeHTML,
	); err != nil {
		return fmt.Errorf("lỗi seed landing doanh_chung: %w", err)
	}

	// 5. Nạp cây file từ đĩa vào `landing_files`
	fileInsert := `
		INSERT INTO landing_files (page_id, domain, file_path, file_type, content)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (domain, page_id, file_path) DO NOTHING
	`

	loaded := 0
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "src/") || rel == "tailwind.config.js" {
			// File nguồn Tailwind (không dùng lúc runtime — bản build đã nằm ở assets/vendor/tailwind)
			return nil
		}

		// Đổi tên `chung.html` → `index.html` (landing edge router phục vụ `index.html`)
		if rel == "chung.html" {
			rel = "index.html"
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		fileType := detectLandingFileType(rel)
		content := string(data)
		if isBinaryLandingExt(rel) {
			mime := landingMimeType(rel)
			content = fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
		}

		if _, execErr := s.pool.Exec(ctx, fileInsert, pageID, "", rel, fileType, content); execErr != nil {
			log.Printf("[WARN] SeedDoanhChungLanding: không lưu được %s: %v", rel, execErr)
			return nil
		}
		loaded++
		return nil
	})
	if err != nil {
		log.Printf("[WARN] SeedDoanhChungLanding: lỗi duyệt cây file: %v", err)
	}

	// 6. Bảo đảm `tracker.js` chuẩn của Apex nằm trong landing này.
	//
	// ⚠️ BẮT BUỘC: dùng bản tracker.js CHUẨN của nền tảng (có PoW, macro Meta,
	// thiết bị chi tiết, storage tách theo landing) — KHÔNG dùng bản frontend-only
	// của bản gốc (đã xóa khỏi đĩa).
	for _, cand := range []string{
		filepath.Join("..", "frontend", "landing", "tracker.js"),
		filepath.Join("frontend", "landing", "tracker.js"),
	} {
		if b, readErr := os.ReadFile(cand); readErr == nil && len(b) > 0 {
			if _, execErr := s.pool.Exec(ctx, fileInsert, pageID, "", "tracker.js", "js", string(b)); execErr == nil {
				loaded++
			}
			break
		}
	}

	// 7. Bảo đảm `config.json` có `landing_id` đúng
	if b, readErr := os.ReadFile(filepath.Join(srcDir, "config.json")); readErr == nil && len(b) > 0 {
		var cfg map[string]interface{}
		if json.Unmarshal(b, &cfg) == nil {
			cfg["landing_id"] = pageID
			if newCfg, mErr := json.MarshalIndent(cfg, "", "  "); mErr == nil {
				_, _ = s.pool.Exec(ctx, fileInsert, pageID, "", "config.json", "json", string(newCfg))
				loaded++
			}
		}
	}

	// 8. Đảm bảo có thư mục `chucmung.html` (trang cảm ơn) — lấy từ bản mẫu chung
	for _, cand := range []string{
		filepath.Join("..", "frontend", "landing", "chucmung.html"),
		filepath.Join("frontend", "landing", "chucmung.html"),
	} {
		if b, readErr := os.ReadFile(cand); readErr == nil && len(b) > 0 {
			_, _ = s.pool.Exec(ctx, fileInsert, pageID, "", "chucmung.html", "html", string(b))
			loaded++
			break
		}
	}

	log.Printf("[INFO] SeedDoanhChungLanding: đã nạp %d file cho landing '%s' (nguồn: %s)", loaded, pageID, srcDir)
	return nil
}

// findLandingSourceDir tìm thư mục chứa mã nguồn landing trên đĩa.
func findLandingSourceDir(pageID string) string {
	candidates := []string{
		filepath.Join("..", "frontend", "landing", pageID),
		filepath.Join("frontend", "landing", pageID),
		pageID,
		filepath.Join("..", pageID),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

// detectLandingFileType suy ra loại file cho `landing_files.file_type`.
func detectLandingFileType(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".js", ".mjs":
		return "js"
	case ".json":
		return "json"
	case ".mp4":
		return "video/mp4"
	case ".webp":
		return "image/webp"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".woff2":
		return "font/woff2"
	default:
		return "text"
	}
}

// isBinaryLandingExt kiểm tra file có cần lưu dạng base64 hay không.
func isBinaryLandingExt(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".ico", ".bmp", ".avif",
		".mp4", ".webm", ".ogg", ".mov", ".avi", ".mkv",
		".mp3", ".wav", ".m4a", ".flac", ".aac",
		".woff", ".woff2", ".ttf", ".otf", ".eot", ".pdf":
		return true
	default:
		return false
	}
}

// landingMimeType trả về MIME type cho file nhị phân.
func landingMimeType(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
