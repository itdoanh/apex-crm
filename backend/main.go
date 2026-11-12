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

// setStaticCacheHeaders đặt header chống cache cho tài nguyên tĩnh (Admin UI).
//
// Bối cảnh: server KHÔNG trả Cache-Control => trình duyệt áp dụng "heuristic freshness"
// (10% thời gian từ Last-Modified). Hệ quả: sau khi deploy file JS/HTML mới, trình duyệt
// vẫn chạy bản cache cũ → UI "thiếu" tính năng vừa thêm (vd: trường `url` trong danh sách
// ánh xạ Google Sheets không xuất hiện dù server đã đúng).
//
// `no-cache` KHÔNG phải "không cache" — nghĩa là "luôn phải hỏi server trước khi dùng".
// Server trả 304 Not Modified nếu file không đổi ⇒ vẫn nhẹ, nhưng luôn thấy bản mới nhất.
// `must-revalidate` bảo đảm không dùng bản cache khi offline.
func setStaticCacheHeaders(w http.ResponseWriter, filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".html":
		// HTML chứa tham chiếu script → không được cache
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	case ".js", ".css":
		// Luôn revalidate (304 nếu không đổi) → deploy có hiệu lực ngay
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	default:
		// Ảnh / font / media: cache dài hạn (kèm ETag/Last-Modified mặc định của http.ServeFile)
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
}

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

	// Tự động seed landing 'doanh_chung' (buổi chia sẻ đầu tư hàng hóa phái sinh)
	// từ mã nguồn trong `frontend/landing/doanh_chung`.
	// KHÔNG ghi đè nếu landing đã tồn tại (để giữ chỉnh sửa trong Code Studio).
	if err := store.SeedDoanhChungLanding(context.Background()); err != nil {
		log.Printf("[WARN] Không thể seed landing doanh_chung: %v", err)
	}

	// Đồng bộ file tĩnh dùng chung (tracker.js, config.json) từ đĩa vào bảng landing_files.
	// Edge Router phục vụ file theo thứ tự: landing_files (DB) TRƯỚC, rồi mới tới đĩa.
	// Nếu không sync, bản DB cũ sẽ "che" bản đĩa mới ⇒ sửa code trong git không có tác dụng.
	if err := store.SyncSharedStaticFilesToDB(context.Background(), "."); err != nil {
		log.Printf("[WARN] Không thể đồng bộ file tĩnh dùng chung vào landing_files: %v", err)
	}

	// Đảm bảo mỗi landing có `config.json` RIÊNG với `landing_id` đúng của nó.
	// Nếu không, tracker.js sẽ gửi analytics/lead về nhầm landing (bug chồng chéo).
	if err := store.EnsureLandingConfigFile(context.Background()); err != nil {
		log.Printf("[WARN] Không thể chuẩn hoá landing_id trong config.json: %v", err)
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
	landingHandler := handlers.NewLandingHandler(cfg, store, landingCache, domainCache)
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
	mux.HandleFunc("/api/landings/save-code", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleSaveCode)))
	mux.HandleFunc("/api/landings/deploy", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDeployLanding)))
	mux.HandleFunc("/api/landings/toggle", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleToggleActive)))
	mux.HandleFunc("/api/landings/duplicate", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDuplicateLanding)))
	mux.HandleFunc("/api/landings/delete", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleDeleteLanding)))
	mux.HandleFunc("/api/landings/files", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleListFiles)))
	mux.HandleFunc("/api/landings/file/save", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleSaveFile)))
	mux.HandleFunc("/api/landings/files/save", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleSaveFilesBatch)))
	mux.HandleFunc("/api/landings/files/upload", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleUploadFile)))
	mux.HandleFunc("/api/landings/file/upload", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(landingHandler.HandleUploadFile)))
	mux.HandleFunc("/api/landings/file/raw", leadHandler.CORSMiddleware(landingHandler.HandleServeRawFile))
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
	// Báo cáo analytics mở rộng (2026-11-12)
	mux.HandleFunc("/api/analytics/timeseries", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleTimeseries)))
	mux.HandleFunc("/api/analytics/devices", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleDevices)))
	mux.HandleFunc("/api/analytics/meta-macros", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleMetaMacros)))
	mux.HandleFunc("/api/analytics/bots", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleBotAnalytics)))
	mux.HandleFunc("/api/analytics/vpn", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleVPNAnalytics)))
	mux.HandleFunc("/api/analytics/landing-report", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(analyticsHandler.HandleLandingReport)))

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
	mux.HandleFunc("/api/domains/check-ssl", leadHandler.CORSMiddleware(authHandler.RequireAdminAuth(domainHandler.HandleCheckSSL)))
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

	// LadiPage SDK Stub & Compatibility Polyfill Runtime
	const ladiSDKStub = `/**
 * LadiPage SDK Stub & Compatibility Runtime
 * Apex CRM Multi-Landing Platform
 */
(function(window){'use strict';window.ladi=window.ladi||{};window.LadiPage=window.LadiPage||window.ladi;window.ladihtml=window.ladihtml||window.ladi;if(!window.ladi.isInit){window.ladi.isInit=true;window.ladi.version='1.0.0-compat';window.ladi.events=window.ladi.events||{};window.ladi.on=function(e,cb){window.ladi.events[e]=window.ladi.events[e]||[];window.ladi.events[e].push(cb);};window.ladi.trigger=function(e,d){if(window.ladi.events[e]){window.ladi.events[e].forEach(function(fn){try{fn(d);}catch(_){}});}};window.ladi.init=function(){window.ladi.trigger('ready');};if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',function(){window.ladi.init();});}else{window.ladi.init();}}})(window);`

	// Khai báo trước router đa tên miền để adminHandler có thể ủy quyền khi custom domain gọi route con
	var landingEdgeRouter func(w http.ResponseWriter, r *http.Request)

	// Giao diện Admin Dashboard (hỗ trợ cả /admin và /ads - crm.hanghoaphaisinh.net/ads)
	adminHandler := func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// QUAN TRỌNG: Nếu đây là custom domain (không phải domain admin crm.hanghoaphaisinh.net và không phải localhost / server IP)
		// VÀ domainCache có route khớp với reqPath trên host này (ví dụ: host "ads.giaodichhanghoatuonglai.com" có route "/ads" hoặc "/landing")
		// -> Nhường quyền xử lý cho multi-domain edge router, KHÔNG ĐƯỢC chặn để hiện Admin!
		normHost := cache.NormalizeHost(r.Host)
		if normHost != "crm.hanghoaphaisinh.net" && normHost != "localhost" && normHost != "127.0.0.1" && normHost != "103.124.95.232" {
			if domainCache != nil && domainCache.HasDomain(r.Host) {
				if _, _, _, _, found := domainCache.ResolveRoute(r.Host, reqPath); found {
					if landingEdgeRouter != nil {
						landingEdgeRouter(w, r)
						return
					}
				}
			}
		}

		// Nếu truy cập /ads hoặc /admin không có dấu gạch chéo cuối -> redirect về /ads/ hoặc /admin/
		// Để trình duyệt hiểu đúng base path khi load các tài nguyên tương đối (js/, css/)
		if reqPath == "/ads" || reqPath == "/admin" {
			http.Redirect(w, r, reqPath+"/", http.StatusMovedPermanently)
			return
		}

		// Kiểm tra nếu là yêu cầu file tài nguyên tĩnh con (css, js, map, png, svg...)
		subPath := strings.TrimPrefix(reqPath, "/admin")
		subPath = strings.TrimPrefix(subPath, "/ads")
		subPath = strings.TrimPrefix(subPath, "/")

		if subPath != "" && filepath.Ext(subPath) != "" && subPath != "index.html" {
			cleanSub := filepath.Clean(filepath.Base(subPath))

			// Phục vụ tức thì ladihtml.sdk.min.js cho Code Studio / Preview nếu được yêu cầu tại bất kỳ tiền tố nào
			if cleanSub == "ladihtml.sdk.min.js" {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				_, _ = w.Write([]byte(ladiSDKStub))
				return
			}

			candidates := []string{
				filepath.Join("..", "frontend", "admin", subPath),
				filepath.Join("frontend", "admin", subPath),
				filepath.Join("..", "frontend", "landing", subPath),
				filepath.Join("frontend", "landing", subPath),
				filepath.Join("static", subPath),
				filepath.Join("..", "backend", "static", subPath),
				filepath.Join("static", "js", subPath),
				filepath.Join("..", "backend", "static", "js", subPath),
				filepath.Join("..", "frontend", "admin", cleanSub),
				filepath.Join("frontend", "admin", cleanSub),
				filepath.Join("..", "frontend", "landing", cleanSub),
				filepath.Join("frontend", "landing", cleanSub),
				filepath.Join("static", cleanSub),
				filepath.Join("..", "backend", "static", cleanSub),
				filepath.Join("..", subPath),
				subPath,
				filepath.Join("..", cleanSub),
				cleanSub,
			}
			for _, target := range candidates {
				cleanTarget := filepath.Clean(target)
				if fi, err := os.Stat(cleanTarget); err == nil && !fi.IsDir() {
					ext := strings.ToLower(filepath.Ext(cleanTarget))
					switch ext {
					case ".html":
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
					case ".js":
						w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
					case ".css":
						w.Header().Set("Content-Type", "text/css; charset=utf-8")
					case ".json":
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
					case ".svg":
						w.Header().Set("Content-Type", "image/svg+xml")
					case ".png":
						w.Header().Set("Content-Type", "image/png")
					case ".webp":
						w.Header().Set("Content-Type", "image/webp")
					case ".jpg", ".jpeg":
						w.Header().Set("Content-Type", "image/jpeg")
					case ".ico":
						w.Header().Set("Content-Type", "image/x-icon")
					}
					// Chống cache trình duyệt cho tài nguyên tĩnh của Admin UI.
					// Không có Cache-Control => Chrome dùng heuristic cache (10% tuổi file)
					// => sau khi deploy file mới, trình duyệt vẫn chạy JS cũ và UI "thiếu" tính năng.
					// no-cache = phải revalidate với server mỗi lần (304 nếu không đổi) => nhẹ mà luôn đúng.
					setStaticCacheHeaders(w, cleanTarget)
					http.ServeFile(w, r, cleanTarget)
					return
				}
			}
			// Nếu yêu cầu file script .js mà không tìm thấy trên đĩa, trả về 404 có MIME JS chuẩn để tránh lỗi strict MIME checking
			if strings.HasSuffix(strings.ToLower(cleanSub), ".js") {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("/* 404: script not found */"))
				return
			}
			http.NotFound(w, r)
			return
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
				setStaticCacheHeaders(w, p)
				_, _ = w.Write(data)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		setStaticCacheHeaders(w, "admin.html")
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
				setStaticCacheHeaders(w, p)
				_, _ = w.Write(data)
				return
			}
		}
		setStaticCacheHeaders(w, "config.js")
		_, _ = w.Write([]byte(`window.APEX_ADMIN_CONFIG = { API_BASE_URL: '', BASE_PATH: '/ads' };`))
	}
	mux.HandleFunc("/ads/config.js", adminConfigHandler)
	mux.HandleFunc("/admin/config.js", adminConfigHandler)
	mux.HandleFunc("/config.js", adminConfigHandler)

	// Multi-Domain Edge Router & Static Content Engine
	landingEdgeRouter = func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// 0. HTML landing KHÔNG được cache lại bởi trình duyệt.
		//
		// LÝ DO (bug chồng chéo landing — 2026-11-05): HTML landing chứa
		// `data-landing-id` + `window.APEX_LANDING_CONFIG` quyết định analytics
		// ghi về landing nào. Nếu trình duyệt cache HTML cũ (heuristic dựa trên
		// `Last-Modified` vì thiếu `Cache-Control`), khách vẫn chạy HTML cũ ⇒
		// tracker.js gửi analytics/lead về nhầm landing.
		setNoCache := func() {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		if strings.HasSuffix(strings.ToLower(reqPath), ".html") {
			setNoCache()
		}

		// 1. Chống Path Traversal (tuyệt đối không cho phép .. hoặc ký tự nguy hiểm)
		if strings.Contains(reqPath, "..") || strings.Contains(reqPath, "//") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// 2. Tra cứu Dynamic Multi-Domain Routing từ RAM Cache
		//
		// `routePrefix` là prefix của route khớp (vd: `/landing`, `/chucmung`, `/`).
		// Dùng để ghi đè đường dẫn tài nguyên tương đối (`tracker.js`) thành tuyệt
		// đối, không phụ thuộc vị trí thẻ `<base>` trong HTML (bug tracker.js 404 —
		// 2026-11-05).
		landingID, routePrefix, subPath, redirectURL, found := domainCache.ResolveRoute(r.Host, reqPath)
		if found {
			if redirectURL != "" {
				http.Redirect(w, r, redirectURL, http.StatusFound)
				return
			}

			// Nếu là yêu cầu sub-resource (tracker.js, style.css, chucmung.html, config.json, hình ảnh...)
			if subPath != "" && subPath != "index.html" {
				cleanSub := filepath.Clean(filepath.Base(subPath))
				lowerSub := strings.ToLower(cleanSub)
				// Bảo vệ file nhạy cảm và binary thực thi, cho phép config.json và tài nguyên tĩnh
				if strings.Contains(lowerSub, "service-account") || strings.Contains(lowerSub, ".env") || strings.HasSuffix(lowerSub, ".key") || strings.HasSuffix(lowerSub, ".sql") || strings.HasSuffix(lowerSub, ".exe") {
					http.NotFound(w, r)
					return
				}

				// Phục vụ tức thì ladihtml.sdk.min.js cho các landing LadiPage
				if cleanSub == "ladihtml.sdk.min.js" {
					w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
					_, _ = w.Write([]byte(ladiSDKStub))
					return
				}

				// Phục vụ trực tiếp trang chucmung.html, tuyệt đối không redirect vòng lặp vô tận
				if subPath == "chucmung.html" || cleanSub == "chucmung.html" {
					// ⚠️ BẮT BUỘC: chucmung.html phải đi qua InjectDynamicScripts để gắn
					// `window.APEX_LANDING_CONFIG` + `data-landing-id` cho tracker.js.
					// Nếu serve thẳng (từ DB hay đĩa) mà không inject, tracker.js sẽ rơi
					// về fallback `mxv_080_online` ⇒ analytics/lead của trang chúc mừng
					// bị ghi nhận nhầm sang landing khác. (bug chồng chéo landing 2026-11-05)

					// Xác định landing context cho trang chúc mừng.
					// Fallback: route gốc `/` có thể trỏ tới landing không tồn tại
					// (dangling route). Khi đó dùng route `/chucmung` của host làm
					// landing context để tracker.js không rơi về fallback chung.
					cmLandingID := landingID
					cmLanding := landingCache.Get(r.Host, cmLandingID)
					if cmLanding == nil {
						cmLanding = landingCache.GetByPageID(cmLandingID)
					}
					if cmLanding == nil {
						if fallbackID, ok := domainCache.ResolveChucMungRoute(r.Host); ok {
							cmLandingID = fallbackID
							cmLanding = landingCache.Get(r.Host, cmLandingID)
							if cmLanding == nil {
								cmLanding = landingCache.GetByPageID(cmLandingID)
							}
						}
					}

					serveChucMung := func(content string) {
						if cmLanding != nil {
							// `routePrefix` = prefix của route đang phục vụ (vd: `/chucmung`,
							// `/`). Ghi đè đường dẫn tài nguyên tương đối (`tracker.js`)
							// thành tuyệt đối để luôn tải được (bug tracker.js 404 — 2026-11-05).
							rendered := landingHandler.InjectDynamicScriptsWithBase(content, cmLanding, routePrefix)
							w.Header().Set("Content-Type", "text/html; charset=utf-8")
							setNoCache()
							_, _ = w.Write([]byte(rendered))
							return
						}
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						setNoCache()
						_, _ = w.Write([]byte(content))
					}

					// Ưu tiên bản trong DB (landing_files) — đây là bản người dùng sửa
					// trong Code Studio, KHÔNG phải bản mẫu trên đĩa.
					if f, err := store.GetLandingFile(r.Host, cmLandingID, "chucmung.html"); err == nil && f != nil && strings.TrimSpace(f.Content) != "" {
						serveChucMung(f.Content)
						return
					}

					cmCandidates := []string{
						filepath.Join("..", "frontend", "landing", "chucmung.html"),
						filepath.Join("frontend", "landing", "chucmung.html"),
						filepath.Join("..", "chucmung.html"),
						"chucmung.html",
					}
					for _, cp := range cmCandidates {
						if fi, err := os.Stat(cp); err == nil && !fi.IsDir() {
							if data, readErr := os.ReadFile(cp); readErr == nil && len(data) > 0 {
								serveChucMung(string(data))
								return
							}
							w.Header().Set("Content-Type", "text/html; charset=utf-8")
							http.ServeFile(w, r, cp)
							return
						}
					}
				}

				// Tìm trong landing_files DB (hỗ trợ cả văn bản lẫn nhị phân ảnh, video, âm thanh)
				f, err := store.GetLandingFile(r.Host, landingID, subPath)
				if err == nil && f != nil {
					handlers.ServeBinaryFile(w, r, f.FilePath, f.FileType, f.Content)
					return
				}

				// Fallback sang các thư mục tĩnh của landing và admin
				candidates := []string{
					filepath.Join("..", "frontend", "landing", subPath),
					filepath.Join("frontend", "landing", subPath),
					filepath.Join("..", "frontend", "landing", cleanSub),
					filepath.Join("frontend", "landing", cleanSub),
					filepath.Join("..", "frontend", "admin", subPath),
					filepath.Join("frontend", "admin", subPath),
					filepath.Join("static", subPath),
					filepath.Join("..", "backend", "static", subPath),
					filepath.Join("static", "js", subPath),
					filepath.Join("..", "backend", "static", "js", subPath),
					filepath.Join("..", "subPath"),
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
						ext := strings.ToLower(filepath.Ext(cleanTarget))
						switch ext {
						case ".html":
							w.Header().Set("Content-Type", "text/html; charset=utf-8")
						case ".js":
							w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
						case ".css":
							w.Header().Set("Content-Type", "text/css; charset=utf-8")
						case ".json":
							w.Header().Set("Content-Type", "application/json; charset=utf-8")
						case ".svg":
							w.Header().Set("Content-Type", "image/svg+xml")
						case ".png":
							w.Header().Set("Content-Type", "image/png")
						case ".webp":
							w.Header().Set("Content-Type", "image/webp")
					case ".jpg", ".jpeg":
						w.Header().Set("Content-Type", "image/jpeg")
					case ".ico":
						w.Header().Set("Content-Type", "image/x-icon")
					}
					setStaticCacheHeaders(w, cleanTarget)
					http.ServeFile(w, r, cleanTarget)
					return
				}
			}

				// QUAN TRỌNG: Nếu là tài nguyên tĩnh có đuôi mở rộng (.js, .css, .png, .jpg, .json, .ico...)
				// mà không tìm thấy -> trả về 404 ngay với MIME type đúng, TUYỆT ĐỐI không fallback sang trang Landing HTML!
				if filepath.Ext(subPath) != "" && filepath.Ext(subPath) != ".html" {
					if strings.HasSuffix(strings.ToLower(cleanSub), ".js") {
						w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte("/* 404: script not found */"))
						return
					}
					http.NotFound(w, r)
					return
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

				// `routePrefix` = prefix của route đang phục vụ (vd: `/landing`, `/`).
				// Ghi đè đường dẫn tài nguyên tương đối thành tuyệt đối để
				// `tracker.js` luôn tải được (bug tracker.js 404 — 2026-11-05).
				renderedHTML := landingHandler.InjectDynamicScriptsWithBase(p.CodeHTML, p, routePrefix)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				// HTML landing chứa `data-landing-id` + `window.APEX_LANDING_CONFIG`
				// quyết định analytics ghi về landing nào ⇒ không được cache lại.
				setNoCache()
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
			".bmp":   true,
			".avif":  true,
			".woff":  true,
			".woff2": true,
			".ttf":   true,
			".otf":   true,
			".eot":   true,
			".mp4":   true,
			".webm":  true,
			".ogg":   true,
			".mov":   true,
			".avi":   true,
			".mkv":   true,
			".mp3":   true,
			".wav":   true,
			".m4a":   true,
			".flac":  true,
			".aac":   true,
			".pdf":   true,
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
				setStaticCacheHeaders(w, cleanTarget)
				http.ServeFile(w, r, cleanTarget)
				return
			}
		}

		http.NotFound(w, r)
	}

	// Đăng ký router đa tên miền vào root path '/'
	mux.HandleFunc("/", leadHandler.CORSMiddleware(landingEdgeRouter))

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
