package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"apex-backend/cache"
	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/handlers"
)

//go:embed static/admin.html
var adminHTML []byte

func main() {
	log.Println("==================================================")
	log.Println("  APEX FINTECH - LANDING PAGE BACKEND SERVER (GO)")
	log.Println("  Hệ thống lưu trữ CSDL PostgreSQL (Docker / Cloud)")
	log.Println("==================================================")

	cfg := config.Load()

	// 1. Khởi tạo CSDL PostgreSQL
	store, err := db.NewStore(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Không thể khởi tạo database PostgreSQL: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("[ERROR] Đóng CSDL lỗi: %v", err)
		}
	}()
	log.Printf("[INFO] CSDL PostgreSQL đã sẵn sàng kết nối tới database '%s' tại %s:%s", cfg.DBName, cfg.DBHost, cfg.DBPort)

	// Tự động seed landing page mặc định 'mxv_080_online' nếu chưa có
	if err := store.SeedDefaultLanding(context.Background()); err != nil {
		log.Printf("[WARN] Không thể seed landing page mặc định: %v", err)
	}

	// Tự động seed tên miền mặc định (hanghoaphaisinh.net, dautuhanghoa.vn)
	if err := store.SeedDefaultDomains(context.Background()); err != nil {
		log.Printf("[WARN] Không thể seed tên miền mặc định: %v", err)
	}

	// Tự động khởi tạo duy nhất 1 tài khoản Admin tối cao nếu hệ thống chưa có
	if err := store.SeedDefaultAdmin(context.Background(), "admin", "ApexAdmin@2026!"); err != nil {
		log.Printf("[WARN] Không thể seed tài khoản admin mặc định: %v", err)
	}

	// 2. Khởi tạo In-Memory RAM Cache siêu tốc (<0.1ms) & Warmup
	landingCache := cache.NewLandingCache(store)
	if err := landingCache.Warmup(context.Background()); err != nil {
		log.Printf("[WARN] Lỗi nạp cache landing pages: %v", err)
	}

	// Khởi tạo In-Memory RAM Cache định tuyến đa tên miền (Multi-Domain Router)
	domainCache := cache.NewDomainCache(store)
	if err := domainCache.Warmup(context.Background()); err != nil {
		log.Printf("[WARN] Lỗi nạp cache domain routes: %v", err)
	}

	// 3. Khởi tạo Handlers
	leadHandler := handlers.NewLeadHandler(cfg, store, landingCache)
	analyticsHandler := handlers.NewAnalyticsHandler(cfg, store)
	sheetHandler := handlers.NewGoogleSheetHandler(cfg, store)
	landingHandler := handlers.NewLandingHandler(cfg, store, landingCache)
	domainHandler := handlers.NewDomainHandler(cfg, store, domainCache)
	authHandler := handlers.NewAuthHandler(cfg, store)

	// 4. Đăng ký Routes
	mux := http.NewServeMux()

	// Authentication & Security APIs (Công khai)
	mux.HandleFunc("/api/auth/login", leadHandler.CORSMiddleware(authHandler.HandleLogin))
	mux.HandleFunc("/api/admin/login", leadHandler.CORSMiddleware(authHandler.HandleLogin))
	mux.HandleFunc("/api/auth/logout", leadHandler.CORSMiddleware(authHandler.HandleLogout))
	mux.HandleFunc("/api/admin/logout", leadHandler.CORSMiddleware(authHandler.HandleLogout))
	mux.HandleFunc("/api/auth/me", leadHandler.CORSMiddleware(authHandler.HandleGetMe))
	mux.HandleFunc("/api/admin/me", leadHandler.CORSMiddleware(authHandler.HandleGetMe))

	// Admin Account & Session Management APIs (Bảo vệ nghiêm ngặt bằng Argon2id & Session Token)
	mux.HandleFunc("/api/admin/change-password", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(authHandler.HandleChangePassword)))
	mux.HandleFunc("/api/admin/sessions", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(authHandler.HandleListSessions)))
	mux.HandleFunc("/api/admin/sessions/revoke", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(authHandler.HandleRevokeSession)))
	mux.HandleFunc("/api/admin/sessions/revoke-others", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(authHandler.HandleRevokeOtherSessions)))
	mux.HandleFunc("/api/admin/audit-logs", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(authHandler.HandleListAuditLogs)))

	// Landing Page Management & Dynamic Configuration (Ladi Studio & Feature Flags)
	mux.HandleFunc("/api/landing/config", leadHandler.CORSMiddleware(landingHandler.HandleGetPublicConfig))
	mux.HandleFunc("/api/landings", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleListLandings)))
	mux.HandleFunc("/api/landings/get", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleGetLanding)))
	mux.HandleFunc("/api/landings/save", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleSaveLanding)))
	mux.HandleFunc("/api/landings/code", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleSaveCode)))
	mux.HandleFunc("/api/landings/deploy", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDeployLanding)))
	mux.HandleFunc("/api/landings/toggle", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleToggleActive)))
	mux.HandleFunc("/api/landings/duplicate", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDuplicateLanding)))
	mux.HandleFunc("/api/landings/delete", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDeleteLanding)))
	mux.HandleFunc("/api/landings/files", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleListFiles)))
	mux.HandleFunc("/api/landings/file", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			landingHandler.HandleGetFile(w, r)
		} else if r.Method == http.MethodPost {
			landingHandler.HandleSaveFile(w, r)
		} else if r.Method == http.MethodDelete {
			landingHandler.HandleDeleteFile(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.HandleFunc("/api/landings/file/rename", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleRenameFile)))
	mux.HandleFunc("/api/landings/folder", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			landingHandler.HandleCreateFolder(w, r)
		} else if r.Method == http.MethodDelete {
			landingHandler.HandleDeleteFolder(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.HandleFunc("/api/landings/folder/create", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleCreateFolder)))
	mux.HandleFunc("/api/landings/folder/delete", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDeleteFolder)))
	mux.HandleFunc("/api/landings/folder/rename", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleRenameFolder)))
	mux.HandleFunc("/api/landings/preview", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandlePreviewLanding)))
	mux.HandleFunc("/api/landings/chucmung.html", leadHandler.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/chucmung.html", http.StatusFound)
	}))
	mux.HandleFunc("/p/chucmung.html", leadHandler.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/chucmung.html", http.StatusFound)
	}))

	// Multi-Landing Public Serving Route (/p/:page_id)
	mux.HandleFunc("/p/", leadHandler.CORSMiddleware(landingHandler.ServePublicLanding))

	// Google Sheets Multi-Destination Sync APIs (Đồng bộ đa bảng tính thời gian thực - Bảo vệ Admin)
	mux.HandleFunc("/api/integrations/sheets/bot-info", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleGetBotInfo)))
	mux.HandleFunc("/api/integrations/sheets/meta", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleGetMetadata)))
	mux.HandleFunc("/api/integrations/sheets/headers", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleGetHeaders)))
	mux.HandleFunc("/api/integrations/sheets/create-headers", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleCreateHeaders)))
	mux.HandleFunc("/api/integrations/sheets/test", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleTestAppend)))
	mux.HandleFunc("/api/integrations/sheets/drive-files", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleListDriveFiles)))
	mux.HandleFunc("/api/integrations/sheets/configs", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			sheetHandler.HandleListConfigs(w, r)
		} else if r.Method == http.MethodPost {
			sheetHandler.HandleSaveConfig(w, r)
		} else if r.Method == http.MethodDelete {
			sheetHandler.HandleDeleteConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.HandleFunc("/api/integrations/sheets/configs/get", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleGetConfig)))
	mux.HandleFunc("/api/integrations/sheets/configs/delete", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleDeleteConfig)))
	mux.HandleFunc("/api/integrations/sheets/configs/toggle", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleToggleConfig)))
	mux.HandleFunc("/api/integrations/sheets/logs", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleListLogs)))
	mux.HandleFunc("/api/integrations/sheets/defaults", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(sheetHandler.HandleGetDefaultMappings)))

	// Analytics API Endpoints (Tracking chi tiết từ landing là public, các báo cáo phân tích bảo vệ nghiêm ngặt)
	mux.HandleFunc("/api/analytics/track", leadHandler.CORSMiddleware(analyticsHandler.HandleTrackEvent))
	mux.HandleFunc("/api/analytics/overview", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleOverview)))
	mux.HandleFunc("/api/analytics/landings", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleLandingPerformance)))
	mux.HandleFunc("/api/analytics/sources", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleSourcePerformance)))
	mux.HandleFunc("/api/analytics/geo", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleGeoAnalytics)))
	mux.HandleFunc("/api/analytics/ad-placements", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleAdPlacementAnalytics)))
	mux.HandleFunc("/api/analytics/sessions", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleListSessions)))
	mux.HandleFunc("/api/analytics/session/", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleSessionDetail)))

	// Lead Ingestion & Management APIs (POST submit lead là public, GET danh sách và thao tác là bảo vệ Admin)
	mux.HandleFunc("/api/leads", leadHandler.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			leadHandler.HandleSubmitLead(w, r)
		} else if r.Method == http.MethodGet {
			authHandler.RequireAdminAuth(leadHandler.HandleListLeads)(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/leads/export.csv", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(leadHandler.HandleExportCSV)))
	mux.HandleFunc("/api/leads/update", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(leadHandler.HandleUpdateLead)))
	mux.HandleFunc("/api/crm/stats", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(leadHandler.HandleCRMStats)))
	mux.HandleFunc("/api/stats", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(leadHandler.HandleStats)))

	// Security & Anti-Spam APIs (Proof-of-Work Challenge là public, Thống kê tấn công & Vùng cách ly bảo vệ Admin)
	mux.HandleFunc("/api/auth/challenge", leadHandler.CORSMiddleware(leadHandler.HandleGetChallenge))
	mux.HandleFunc("/api/security/stats", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(leadHandler.HandleGetSecurityStats)))
	mux.HandleFunc("/api/security/quarantine", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(leadHandler.HandleListQuarantine)))

	// Quản Lý Tên Miền & Multi-Domain Custom DNS Routing (Bảo vệ nghiêm ngặt)
	mux.HandleFunc("/api/domains", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			domainHandler.HandleListDomains(w, r)
		} else if r.Method == http.MethodPost {
			domainHandler.HandleSaveDomain(w, r)
		} else if r.Method == http.MethodDelete {
			domainHandler.HandleDeleteDomain(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.HandleFunc("/api/domains/save", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(domainHandler.HandleSaveDomain)))
	mux.HandleFunc("/api/domains/delete", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(domainHandler.HandleDeleteDomain)))
	mux.HandleFunc("/api/domains/check-dns", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(domainHandler.HandleCheckDNS)))
	mux.HandleFunc("/api/domains/deploy", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(domainHandler.HandleDeployRoute)))
	mux.HandleFunc("/api/domains/routes/delete", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(domainHandler.HandleDeleteRoute)))

	// ACME On-Demand SSL Verification Endpoint (Dùng cho Caddy / CertMagic ask endpoint)
	mux.HandleFunc("/api/internal/check-domain", domainHandler.HandleCheckDomainAllowed)
	mux.HandleFunc("/api/check-domain", domainHandler.HandleCheckDomainAllowed)

	// Health Check
	mux.HandleFunc("/health", leadHandler.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
	}))

	// Giao diện Admin Dashboard (hỗ trợ cả /admin và /ads - crm.hanghoaphaisinh.net/ads)
	adminHandler := func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// Kiểm tra nếu là yêu cầu file tài nguyên tĩnh con (css, js, map, png, svg...)
		subPath := strings.TrimPrefix(reqPath, "/admin")
		subPath = strings.TrimPrefix(subPath, "/ads")
		subPath = strings.TrimPrefix(subPath, "/")

		if subPath != "" && filepath.Ext(subPath) != "" && subPath != "index.html" {
			candidates := []string{
				filepath.Join("..", "frontend", "admin", subPath),
				filepath.Join("frontend", "admin", subPath),
				filepath.Join("static", subPath),
				filepath.Join("..", "backend", "static", subPath),
			}
			for _, target := range candidates {
				cleanTarget := filepath.Clean(target)
				if fi, err := os.Stat(cleanTarget); err == nil && !fi.IsDir() {
					if filepath.Ext(cleanTarget) == ".html" {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
					}
					http.ServeFile(w, r, cleanTarget)
					return
				}
			}
		}

		// Ưu tiên đọc từ frontend/admin/index.html nếu có trên đĩa
		candidates := []string{
			filepath.Join("..", "frontend", "admin", "index.html"),
			filepath.Join("frontend", "admin", "index.html"),
			filepath.Join("static", "index.html"),
			filepath.Join("static", "admin.html"),
		}
		for _, p := range candidates {
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(data)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(adminHTML)
	}
	mux.HandleFunc("/admin", adminHandler)
	mux.HandleFunc("/admin/", adminHandler)
	mux.HandleFunc("/ads", adminHandler)
	mux.HandleFunc("/ads/", adminHandler)
	mux.HandleFunc("/studio.html", adminHandler)

	// Phục vụ file config.js cho Admin Dashboard khi chạy độc lập hoặc qua proxy
	adminConfigHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		candidates := []string{
			filepath.Join("..", "frontend", "admin", "config.js"),
			filepath.Join("frontend", "admin", "config.js"),
		}
		for _, p := range candidates {
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				_, _ = w.Write(data)
				return
			}
		}
		_, _ = w.Write([]byte(`window.APEX_ADMIN_CONFIG = { API_BASE_URL: '', BASE_PATH: '/ads' };`))
	}
	mux.HandleFunc("/ads/config.js", adminConfigHandler)
	mux.HandleFunc("/admin/config.js", adminConfigHandler)
	mux.HandleFunc("/config.js", adminConfigHandler)

	// Multi-Domain Edge Router & Static Content Engine
	mux.HandleFunc("/", leadHandler.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// 1. Chống Path Traversal (tuyệt đối không cho phép .. hoặc ký tự nguy hiểm)
		if strings.Contains(reqPath, "..") || strings.Contains(reqPath, "//") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// 2. Tra cứu Dynamic Multi-Domain Routing từ RAM Cache
		landingID, _, subPath, redirectURL, found := domainCache.ResolveRoute(r.Host, reqPath)
		if found {
			if redirectURL != "" {
				http.Redirect(w, r, redirectURL, http.StatusFound)
				return
			}

			// Nếu là yêu cầu sub-resource (tracker.js, style.css, chucmung.html, hình ảnh...)
			if subPath != "" && subPath != "index.html" {
				lowerSub := strings.ToLower(subPath)
				if strings.HasSuffix(lowerSub, ".json") || strings.HasSuffix(lowerSub, ".env") || strings.HasSuffix(lowerSub, ".key") || strings.HasSuffix(lowerSub, ".sql") || strings.HasSuffix(lowerSub, ".exe") {
					http.NotFound(w, r)
					return
				}
				if subPath == "chucmung.html" {
					http.Redirect(w, r, "/chucmung.html", http.StatusFound)
					return
				}

				// Tìm trong landing_files DB
				f, err := store.GetLandingFile(r.Host, landingID, subPath)
				if err == nil && f != nil {
					switch f.FileType {
					case "css":
						w.Header().Set("Content-Type", "text/css; charset=utf-8")
					case "js":
						w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
					case "html":
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
					default:
						w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					}
					_, _ = w.Write([]byte(f.Content))
					return
				}

				// Fallback sang các thư mục tĩnh của landing
				cleanSub := filepath.Clean(filepath.Base(subPath))
				candidates := []string{
					filepath.Join("..", "frontend", "landing", subPath),
					filepath.Join("frontend", "landing", subPath),
					filepath.Join("..", "frontend", "landing", cleanSub),
					filepath.Join("frontend", "landing", cleanSub),
					filepath.Join("..", subPath),
					subPath,
					filepath.Join("..", cleanSub),
					cleanSub,
				}
				for _, target := range candidates {
					cleanTarget := filepath.Clean(target)
					if strings.Contains(cleanTarget, "backend") && !strings.Contains(cleanTarget, "static") {
						continue
					}
					if fi, err := os.Stat(cleanTarget); err == nil && !fi.IsDir() {
						http.ServeFile(w, r, cleanTarget)
						return
					}
				}
			}

			// Phục vụ nội dung Landing Page HTML (index.html)
			p := landingCache.Get(r.Host, landingID)
			if p == nil {
				p = landingCache.GetByPageID(landingID)
			}
			if p != nil {
				if !p.IsActive {
					if p.RedirectURL != "" {
						http.Redirect(w, r, p.RedirectURL, http.StatusFound)
						return
					}
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="vi"><head><meta charset="UTF-8"><title>Thông Báo - Trang Tạm Ngưng</title><style>body{background:#061120;color:#fff;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;text-align:center}.box{background:#0a1c33;padding:40px;border-radius:20px;border:1px solid #1e3a60;max-width:480px}</style></head><body><div class="box"><div style="font-size:48px;margin-bottom:16px">🔒</div><h2>Trang Tạm Ngưng Nhận Đăng Ký</h2><p style="color:#94a3b8">Hệ thống đang trong thời gian bảo trì hoặc chương trình đã đủ số lượng học viên. Quý khách vui lòng liên hệ hotline hoặc quay lại sau.</p></div></body></html>`))
					return
				}

				renderedHTML := landingHandler.InjectDynamicScripts(p.CodeHTML, p)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(renderedHTML))
				return
			}
		}

		// 3. Nếu là root '/' và không thuộc domain route nào -> redirect về /admin
		if reqPath == "/" {
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}

		cleanPath := filepath.Clean(strings.TrimPrefix(reqPath, "/"))

		// 4. Chống rò rỉ file ẩn (.env, .git, .DS_Store,...)
		baseName := filepath.Base(cleanPath)
		if strings.HasPrefix(baseName, ".") {
			http.NotFound(w, r)
			return
		}

		// Nếu yêu cầu chucmung.html ở bất kỳ tiền tố đường dẫn nào
		if baseName == "chucmung.html" && reqPath != "/chucmung.html" {
			http.Redirect(w, r, "/chucmung.html", http.StatusFound)
			return
		}

		// 5. Whitelist danh sách phần mở rộng tệp tĩnh ĐƯỢC PHÉP phục vụ công khai
		allowedExts := map[string]bool{
			".html":  true,
			".js":    true,
			".css":   true,
			".json":  true,
			".png":   true,
			".jpg":   true,
			".jpeg":  true,
			".gif":   true,
			".svg":   true,
			".webp":  true,
			".ico":   true,
			".woff":  true,
			".woff2": true,
			".ttf":   true,
			".mp4":   true,
		}
		ext := strings.ToLower(filepath.Ext(cleanPath))
		if !allowedExts[ext] {
			http.NotFound(w, r)
			return
		}

		// 6. Routing Middleware: Kiểm tra cờ is_active nếu yêu cầu ladi.html
		if cleanPath == "ladi.html" {
			active, redirectURL := landingHandler.CheckLandingActive(r.Host, "mxv_080_online")
			if !active {
				if redirectURL != "" {
					http.Redirect(w, r, redirectURL, http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="vi"><head><meta charset="UTF-8"><title>Thông Báo - Trang Tạm Ngưng</title><style>body{background:#061120;color:#fff;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;text-align:center}.box{background:#0a1c33;padding:40px;border-radius:20px;border:1px solid #1e3a60;max-width:480px}</style></head><body><div class="box"><div style="font-size:48px;margin-bottom:16px">🔒</div><h2>Trang Tạm Ngưng Nhận Đăng Ký</h2><p style="color:#94a3b8">Hệ thống đang trong thời gian bảo trì hoặc chương trình đã đủ số lượng học viên. Quý khách vui lòng liên hệ hotline hoặc quay lại sau.</p></div></body></html>`))
				return
			}
		}

		// 7. Tìm kiếm file trong các thư mục web an toàn (frontend/landing, frontend/admin, static, root)
		candidates := []string{
			filepath.Join("..", "frontend", "landing", cleanPath),
			filepath.Join("frontend", "landing", cleanPath),
			filepath.Join("..", "frontend", "admin", cleanPath),
			filepath.Join("frontend", "admin", cleanPath),
			filepath.Join("static", cleanPath),
			filepath.Join("..", "backend", "static", cleanPath),
			filepath.Join("..", cleanPath),
			cleanPath,
		}
		for _, target := range candidates {
			cleanTarget := filepath.Clean(target)
			if strings.Contains(cleanTarget, "backend") && !strings.Contains(cleanTarget, "static") {
				continue
			}
			if fi, err := os.Stat(cleanTarget); err == nil && !fi.IsDir() {
				http.ServeFile(w, r, cleanTarget)
				return
			}
		}

		http.NotFound(w, r)
	}))

	serverAddr := ":" + cfg.Port
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,  // Chống Slowloris attack hiệu quả
		ReadTimeout:       10 * time.Second, // Timeout đọc dữ liệu
		WriteTimeout:      10 * time.Second, // Timeout ghi phản hồi
		IdleTimeout:       30 * time.Second, // Timeout dọn kết nối idle
		MaxHeaderBytes:    1 << 20,          // Giới hạn header 1MB
	}

	// 4. Khởi chạy Server trong goroutine
	go func() {
		log.Printf("[INFO] Server đang lắng nghe tại http://localhost%s", serverAddr)
		log.Printf("[INFO] Dashboard Quản trị Leads: http://localhost%s/admin hoặc http://localhost%s/ads", serverAddr, serverAddr)
		log.Printf("[INFO] API tiếp nhận dữ liệu:   POST http://localhost%s/api/leads", serverAddr)
		log.Printf("[INFO] Quản lý Tên Miền & DNS:  GET  http://localhost%s/api/domains", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server gặp sự cố: %v", err)
		}
	}()

	// 5. Lắng nghe tín hiệu dừng an toàn (Graceful Shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Println("[INFO] Đang đóng server an toàn...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server Shutdown thất bại: %v", err)
	}

	log.Println("[INFO] Server đã dừng hoàn tất.")
}
