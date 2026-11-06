package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"apex-backend/cache"
	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/models"
)

type LandingHandler struct {
	cfg         *config.Config
	store       *db.Store
	cache       *cache.LandingCache
	domainCache *cache.DomainCache
}

func NewLandingHandler(cfg *config.Config, store *db.Store, cache *cache.LandingCache, domainCache *cache.DomainCache) *LandingHandler {
	return &LandingHandler{
		cfg:         cfg,
		store:       store,
		cache:       cache,
		domainCache: domainCache,
	}
}

// ============================================================
// 1. PUBLIC DYNAMIC CONFIGURATION API (CLIENT / TRACKER.JS)
// ============================================================

// HandleGetPublicConfig trả về cấu hình động của Landing Page theo domain hoặc page_id
func (h *LandingHandler) HandleGetPublicConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodOptions {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	if pageID == "" {
		pageID = strings.TrimSpace(r.URL.Query().Get("landing_id"))
	}
	if pageID == "" {
		pageID = "mxv_080_online" // Default fallback
	}

	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" {
		domain = r.Host
	}

	// Đọc từ In-memory RAM Cache (<0.1ms)
	p := h.cache.Get(domain, pageID)
	if p == nil {
		p = h.cache.GetByPageID(pageID)
	}

	if p == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Landing page not found",
			"page_id": pageID,
		})
		return
	}

	// Chuyển đổi sang LandingPublicConfig
	settings := p.Settings
	if settings == nil {
		settings = make(map[string]interface{})
	}

	title, _ := settings["title"].(string)
	hotline, _ := settings["hotline"].(string)
	zaloLink, _ := settings["zalo_link"].(string)
	fbPixel, _ := settings["facebook_pixel_id"].(string)
	gaID, _ := settings["google_analytics_id"].(string)
	ttPixel, _ := settings["tiktok_pixel_id"].(string)
	headScripts, _ := settings["custom_head_scripts"].(string)
	bodyScripts, _ := settings["custom_body_scripts"].(string)

	var thoiGianChiaSe map[string]interface{}
	if tg, ok := settings["thoi_gian_chia_se"].(map[string]interface{}); ok {
		thoiGianChiaSe = tg
	}
	var khoaHoc map[string]interface{}
	if kh, ok := settings["khoa_hoc"].(map[string]interface{}); ok {
		khoaHoc = kh
	}

	// Chuẩn hoá URL chuyển hướng sau đăng ký (fix 2026-11-06).
	//
	// `form_config.success_redirect` / `redirect_url` có thể bị cấu hình sai thành
	// "domain.com/chucmung" (có domain nhúng). Trình duyệt phân giải tương đối theo
	// `<base href="/landing/">` ⇒ quay lại trang CÓ FORM ⇒ khách bấm "Đăng ký" lần
	// nữa ⇒ sinh lead trùng SĐT cùng thời gian.
	//
	// Chuẩn hoá tại server để mọi client (kể cả tracker.js cũ chưa cập nhật) đều
	// nhận đúng đường dẫn tới trang cảm ơn.
	settingsOut := settings
	if fc, ok := settings["form_config"].(map[string]interface{}); ok {
		fcCopy := make(map[string]interface{}, len(fc))
		for k, val := range fc {
			fcCopy[k] = val
		}
		if sr, ok := fcCopy["success_redirect"].(string); ok {
			if norm := normalizeSuccessRedirectValue(sr); norm != "" {
				fcCopy["success_redirect"] = norm
			}
		}
		settingsCopy := make(map[string]interface{}, len(settings))
		for k, val := range settings {
			settingsCopy[k] = val
		}
		settingsCopy["form_config"] = fcCopy
		settingsOut = settingsCopy
	}
	redirectURL := normalizeSuccessRedirectValue(p.RedirectURL)

	pubConfig := models.LandingPublicConfig{
		LandingID:         p.PageID,
		Name:              p.Name,
		Domain:            p.Domain,
		Nguon:             p.NguonDefault,
		IsActive:          p.IsActive,
		Status:            p.Status,
		RedirectURL:       redirectURL,
		Settings:          settingsOut,
		Title:             title,
		Hotline:           hotline,
		ZaloLink:          zaloLink,
		FacebookPixelID:   fbPixel,
		GoogleAnalyticsID: gaID,
		TikTokPixelID:     ttPixel,
		CustomHeadScripts: headScripts,
		CustomBodyScripts: bodyScripts,
		ThoiGianChiaSe:    thoiGianChiaSe,
		KhoaHoc:           khoaHoc,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_ = json.NewEncoder(w).Encode(pubConfig)
}

// ============================================================
// 2. ADMIN CRUD APIS (QUẢN LÝ LANDING PAGE)
// ============================================================

// HandleListLandings liệt kê toàn bộ danh sách landing pages
func (h *LandingHandler) HandleListLandings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	landings, err := h.store.ListLandingPages()
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi truy vấn CSDL: %v", err), http.StatusInternalServerError)
		return
	}
	if landings == nil {
		landings = []models.LandingPage{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"total":    len(landings),
		"landings": landings,
	})
}

// HandleGetLanding lấy thông tin chi tiết của 1 landing page (bao gồm cả code_html)
func (h *LandingHandler) HandleGetLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))

	if pageID == "" {
		http.Error(w, "Thiếu tham số page_id", http.StatusBadRequest)
		return
	}

	p, err := h.store.GetLandingPage(domain, pageID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi truy vấn CSDL: %v", err), http.StatusInternalServerError)
		return
	}
	if p == nil {
		p, err = h.store.GetLandingPageByID(pageID)
	}

	if p == nil {
		http.Error(w, "Không tìm thấy Landing Page", http.StatusNotFound)
		return
	}

	// Lấy thông tin route (path_prefix) và domain hiện tại gắn với landing page này
	if route, errRoute := h.store.GetRouteByLandingID(p.PageID); errRoute == nil && route != nil {
		p.PathPrefix = route.PathPrefix
		if p.Domain == "" && route.Domain != "" {
			p.Domain = route.Domain
		}
	} else if p.Settings != nil {
		if pref, ok := p.Settings["path_prefix"].(string); ok && pref != "" {
			p.PathPrefix = pref
		}
	}
	if p.PathPrefix == "" {
		p.PathPrefix = "/"
	}

	// Lấy danh sách files của landing page
	files, _ := h.store.ListLandingFiles(p.Domain, p.PageID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"landing": p,
		"files":   files,
	})
}

// HandleSaveLanding lưu hoặc tạo mới thông tin cài đặt landing page
func (h *LandingHandler) HandleSaveLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LandingPage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Dữ liệu JSON không hợp lệ: %v", err), http.StatusBadRequest)
		return
	}

	req.PageID = strings.TrimSpace(req.PageID)
	if req.PageID == "" {
		http.Error(w, "Mã Landing (Page ID / slug) không được để trống", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = req.PageID
	}

	// Kiểm tra nếu là landing page mới chưa có mã HTML -> khởi tạo khung HTML sạch sẽ (không copy template cũ)
	existingLanding, _ := h.store.GetLandingPage(req.Domain, req.PageID)
	if existingLanding == nil {
		existingLanding, _ = h.store.GetLandingPageByID(req.PageID)
	}
	if existingLanding == nil || existingLanding.CodeHTML == "" {
		if req.CodeHTML == "" {
			req.CodeHTML = "<!DOCTYPE html>\n<html lang=\"vi\">\n<head>\n    <meta charset=\"UTF-8\">\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n    <title>" + html.EscapeString(req.Name) + "</title>\n</head>\n<body class=\"bg-slate-900 text-white min-h-screen\">\n    <!-- Landing Page: " + html.EscapeString(req.Name) + " -->\n</body>\n</html>"
		}
	} else if req.CodeHTML == "" && existingLanding != nil {
		req.CodeHTML = existingLanding.CodeHTML
	}

	// Chuẩn hóa và bảo toàn path_prefix
	cleanPrefix := strings.TrimSpace(req.PathPrefix)
	if cleanPrefix == "" {
		cleanPrefix = "/"
	}
	if !strings.HasPrefix(cleanPrefix, "/") {
		cleanPrefix = "/" + cleanPrefix
	}
	if len(cleanPrefix) > 1 {
		cleanPrefix = strings.TrimRight(cleanPrefix, "/")
	}
	req.PathPrefix = cleanPrefix
	if req.Settings == nil {
		req.Settings = make(map[string]interface{})
	}
	req.Settings["path_prefix"] = cleanPrefix

	// Lưu vào CSDL PostgreSQL
	if err := h.store.SaveLandingPage(&req); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi lưu CSDL: %v", err), http.StatusInternalServerError)
		return
	}

	// Tự động đồng bộ Route nếu có Domain và PathPrefix
	targetDomain := strings.TrimSpace(req.Domain)
	if targetDomain == "" {
		if existingLanding != nil && existingLanding.Domain != "" {
			targetDomain = existingLanding.Domain
		} else if route, _ := h.store.GetRouteByLandingID(req.PageID); route != nil && route.Domain != "" {
			targetDomain = route.Domain
		}
	}

	if targetDomain != "" {
		if d, err := h.store.GetDomainByName(targetDomain); err == nil && d != nil {
			_, _ = h.store.DeployDomainRoute(d.ID, cleanPrefix, req.PageID, "")
			_ = h.store.SyncLandingFilesToDomain("", targetDomain, req.PageID)
			if h.domainCache != nil {
				_ = h.domainCache.Reload(r.Context())
			}
		}
	}

	// Nếu là landing mới tạo lần đầu, khởi tạo file index.html sạch sẽ
	if existingLanding == nil {
		_ = h.store.SaveLandingFile(req.Domain, req.PageID, "index.html", "html", req.CodeHTML)
	}

	// Làm mới Cache RAM (Pub/Sub Invalidation)
	h.cache.Invalidate(req.Domain, req.PageID)
	saved, _ := h.store.GetLandingPage(req.Domain, req.PageID)
	if saved != nil {
		// Gắn lại path_prefix trả về cho frontend
		if req.PathPrefix != "" {
			saved.PathPrefix = req.PathPrefix
		} else {
			saved.PathPrefix = "/"
		}
		h.cache.Set(saved)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã lưu cài đặt Landing Page thành công",
		"landing": saved,
	})
}

// HandleSaveCode lưu mã code HTML (Lưu mã nguồn trực tiếp)
func (h *LandingHandler) HandleSaveCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain   string `json:"domain"`
		PageID   string `json:"page_id"`
		CodeHTML string `json:"code_html"`
		Status   string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.PageID == "" {
		http.Error(w, "Thiếu page_id", http.StatusBadRequest)
		return
	}
	if payload.Status == "" {
		payload.Status = "published"
	}

	if err := h.store.UpdateLandingCode(payload.Domain, payload.PageID, payload.CodeHTML, payload.Status); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi cập nhật mã: %v", err), http.StatusInternalServerError)
		return
	}

	h.cache.Invalidate(payload.Domain, payload.PageID)
	p, _ := h.store.GetLandingPage(payload.Domain, payload.PageID)
	if p != nil {
		h.cache.Set(p)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã lưu mã nguồn thành công",
	})
}

// HandleDeployLanding xuất bản chính thức (Deploy) code của Landing Page
func (h *LandingHandler) HandleDeployLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain   string `json:"domain"`
		PageID   string `json:"page_id"`
		CodeHTML string `json:"code_html"`
		// Files: toàn bộ file trong cây File Manager (đã bao gồm index.html).
		// Deploy sẽ ghi TẤT CẢ file này xuống domain đích, không chỉ index.html.
		Files []struct {
			FilePath string `json:"file_path"`
			FileType string `json:"file_type"`
			Content  string `json:"content"`
		} `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.PageID == "" {
		http.Error(w, "Thiếu page_id", http.StatusBadRequest)
		return
	}

	// 1. Tìm thông tin Landing Page và Route đã gán trước đó (nếu có)
	existingLanding, _ := h.store.GetLandingPageByID(payload.PageID)
	route, _ := h.store.GetRouteByLandingID(payload.PageID)

	targetDomain := strings.TrimSpace(payload.Domain)
	if targetDomain == "" {
		if route != nil && route.Domain != "" {
			targetDomain = route.Domain
		} else if existingLanding != nil && existingLanding.Domain != "" {
			targetDomain = existingLanding.Domain
		}
	}

	// Nếu đã xác định targetDomain cụ thể, tìm chính xác route của domain đó
	var dom *models.Domain
	if targetDomain != "" {
		dom, _ = h.store.GetDomainByName(targetDomain)
		if dom != nil {
			if domRoute, err := h.store.GetRouteByDomainAndLandingID(dom.ID, payload.PageID); err == nil && domRoute != nil {
				route = domRoute
			}
		}
	}

	cleanPrefix := "/"
	if route != nil && route.PathPrefix != "" {
		cleanPrefix = route.PathPrefix
	} else if existingLanding != nil && existingLanding.PathPrefix != "" {
		cleanPrefix = existingLanding.PathPrefix
	}
	cleanPrefix = strings.TrimSpace(cleanPrefix)
	if !strings.HasPrefix(cleanPrefix, "/") {
		cleanPrefix = "/" + cleanPrefix
	}
	if len(cleanPrefix) > 1 {
		cleanPrefix = strings.TrimRight(cleanPrefix, "/")
	}

	// 2. Ghi TOÀN BỘ file của landing (không chỉ index.html) vào bản gốc (domain "")
	//
	// Yêu cầu 2026-11-04: khi Deploy, mọi file trong cây File Manager (ảnh, video,
	// âm thanh, css, js...) phải được lưu & deploy lên domain đích. Trước đây chỉ
	// index.html được đồng bộ ⇒ file upload trong thư mục bị "mất" khi deploy.
	deployedFiles := 0
	for _, f := range payload.Files {
		fp := strings.TrimPrefix(strings.TrimSpace(f.FilePath), "/")
		if fp == "" || strings.Contains(fp, "..") {
			continue
		}
		fileType := strings.TrimSpace(f.FileType)
		if fileType == "" {
			fileType = guessLandingFileType(fp)
		}
		if err := h.store.SaveLandingFile("", payload.PageID, fp, fileType, f.Content); err != nil {
			continue
		}
		deployedFiles++
	}

	if err := h.store.UpdateLandingCode("", payload.PageID, payload.CodeHTML, "published"); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi xuất bản: %v", err), http.StatusInternalServerError)
		return
	}
	_ = h.store.SaveLandingFile("", payload.PageID, "index.html", "html", payload.CodeHTML)
	_ = h.store.ToggleLandingActive("", payload.PageID, true)
	h.cache.Invalidate("", payload.PageID)

	// 3. Nếu landing đã được cấu hình tên miền hoặc có route, tự động đồng bộ và deploy lên tên miền đó
	isCustomDomain := false
	if targetDomain != "" {
		isCustomDomain = true
		_ = h.store.UpdateLandingCode(targetDomain, payload.PageID, payload.CodeHTML, "published")
		_ = h.store.SaveLandingFile(targetDomain, payload.PageID, "index.html", "html", payload.CodeHTML)

		// Đồng bộ TẤT CẢ file (không chỉ index.html) sang domain đích.
		//
		// THỨ TỰ QUAN TRỌNG:
		// 1) SyncLandingFilesToDomain copy các file đang lưu ở domain rỗng (bản gốc)
		//    sang domain đích — bảo đảm file có sẵn trong DB cũng được deploy.
		// 2) Sau đó ghi đè bằng payload.Files (nội dung mới nhất người dùng vừa lưu).
		//    Nếu đảo thứ tự, file upload trong thư mục sẽ bị bản gốc cũ đè mất.
		_ = h.store.SyncLandingFilesToDomain("", targetDomain, payload.PageID)

		if len(payload.Files) > 0 {
			for _, f := range payload.Files {
				fp := strings.TrimPrefix(strings.TrimSpace(f.FilePath), "/")
				if fp == "" || strings.Contains(fp, "..") {
					continue
				}
				fileType := strings.TrimSpace(f.FileType)
				if fileType == "" {
					fileType = guessLandingFileType(fp)
				}
				_ = h.store.SaveLandingFile(targetDomain, payload.PageID, fp, fileType, f.Content)
			}
		}
		_ = h.store.ToggleLandingActive(targetDomain, payload.PageID, true)
		h.cache.Invalidate(targetDomain, payload.PageID)

		// Kích hoạt route trong bảng domain_routes nếu tìm thấy domain trong CSDL (hoặc tự tạo mới nếu chưa có)
		dom, err := h.store.GetDomainByName(targetDomain)
		if err != nil || dom == nil {
			newDom := &models.Domain{
				Domain:           targetDomain,
				NS1:              "ns1.hanghoaphaisinh.net",
				NS2:              "ns2.hanghoaphaisinh.net",
				TargetIP:         "103.124.95.232",
				DefaultLandingID: payload.PageID,
				DNSStatus:        "pending",
				SSLStatus:        "pending",
			}
			dom, _ = h.store.SaveDomain(newDom)
		}
		if dom != nil {
			_, _ = h.store.DeployDomainRoute(dom.ID, cleanPrefix, payload.PageID, "")
		}
	}

	// 4. Làm mới toàn bộ RAM Cache (landingCache & domainCache cho Edge Router)
	if h.domainCache != nil {
		_ = h.domainCache.Reload(r.Context())
	}

	p, _ := h.store.GetLandingPage(targetDomain, payload.PageID)
	if p == nil {
		p, _ = h.store.GetLandingPage("", payload.PageID)
	}
	if p != nil {
		if cleanPrefix != "" {
			p.PathPrefix = cleanPrefix
		}
		h.cache.Set(p)
		if targetDomain != "" && p.Domain != targetDomain {
			domainP := *p
			domainP.Domain = targetDomain
			h.cache.Set(&domainP)
		}
	}

	// 5. Xác định URL triển khai chính xác để phản hồi cho Frontend
	var publicURL string
	var message string
	if isCustomDomain && targetDomain != "" {
		scheme := "https://"
		if strings.Contains(targetDomain, "localhost") || strings.Contains(targetDomain, "127.0.0.1") {
			scheme = "http://"
		}
		displayPath := cleanPrefix
		if displayPath == "/" {
			displayPath = ""
		}
		publicURL = fmt.Sprintf("%s%s%s", scheme, targetDomain, displayPath)
		if displayPath == "" {
			publicURL += "/"
		}
		message = fmt.Sprintf("Đã Deploy thành công lên Tên Miền: %s%s", targetDomain, cleanPrefix)
	} else {
		publicURL = fmt.Sprintf("/p/%s", payload.PageID)
		message = fmt.Sprintf("Đã Deploy thành công Landing Page '%s' lên URL mặc định: /p/%s", payload.PageID, payload.PageID)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"message":          message,
		"public_url":       publicURL,
		"is_custom_domain": isCustomDomain,
		"domain":           targetDomain,
		"path_prefix":      cleanPrefix,
		"deployed_files":   deployedFiles,
	})
}

// HandleToggleActive bật / tắt trạng thái kích hoạt của Landing Page (Feature Flag Kill-Switch)
func (h *LandingHandler) HandleToggleActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain   string `json:"domain"`
		PageID   string `json:"page_id"`
		IsActive bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if err := h.store.ToggleLandingActive(payload.Domain, payload.PageID, payload.IsActive); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi cập nhật trạng thái: %v", err), http.StatusInternalServerError)
		return
	}

	h.cache.Invalidate(payload.Domain, payload.PageID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"is_active": payload.IsActive,
		"message":   fmt.Sprintf("Đã %s Landing Page '%s'", map[bool]string{true: "kích hoạt", false: "tạm ngưng"}[payload.IsActive], payload.PageID),
	})
}

// HandleDuplicateLanding nhân bản 1 Landing Page
func (h *LandingHandler) HandleDuplicateLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		SourceDomain string `json:"source_domain"`
		SourcePageID string `json:"source_page_id"`
		NewPageID    string `json:"new_page_id"`
		NewName      string `json:"new_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.SourcePageID == "" || payload.NewPageID == "" {
		http.Error(w, "Thiếu tham số source_page_id hoặc new_page_id", http.StatusBadRequest)
		return
	}
	if payload.NewName == "" {
		payload.NewName = payload.NewPageID + " (Bản sao)"
	}

	if err := h.store.DuplicateLandingPage(payload.SourceDomain, payload.SourcePageID, payload.NewPageID, payload.NewName); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi nhân bản: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã nhân bản thành công sang '%s'", payload.NewPageID),
	})
}

// HandleDeleteLanding xóa 1 Landing Page
func (h *LandingHandler) HandleDeleteLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))

	if pageID == "" {
		var payload struct {
			Domain string `json:"domain"`
			PageID string `json:"page_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		pageID = payload.PageID
		domain = payload.Domain
	}

	if pageID == "" {
		http.Error(w, "Thiếu page_id", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteLandingPage(domain, pageID); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi xóa: %v", err), http.StatusInternalServerError)
		return
	}

	h.cache.Invalidate(domain, pageID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã xóa Landing Page '%s'", pageID),
	})
}

// ============================================================
// 3. FILE MANAGER APIS (QUẢN LÝ CÂY THƯ MỤC TỆP TIN LANDING)
// ============================================================

// HandleListFiles liệt kê toàn bộ tệp tin thuộc landing page
func (h *LandingHandler) HandleListFiles(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if pageID == "" {
		http.Error(w, "Thiếu page_id", http.StatusBadRequest)
		return
	}

	files, err := h.store.ListLandingFiles(domain, pageID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   files,
	})
}

// HandleGetFile lấy nội dung 1 tệp tin
func (h *LandingHandler) HandleGetFile(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	filePath := strings.TrimSpace(r.URL.Query().Get("file_path"))
	if filePath == "" {
		filePath = strings.TrimSpace(r.URL.Query().Get("path"))
	}
	if pageID == "" || filePath == "" {
		http.Error(w, "Thiếu page_id hoặc file_path", http.StatusBadRequest)
		return
	}

	f, err := h.store.GetLandingFile(domain, pageID, filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi: %v", err), http.StatusInternalServerError)
		return
	}
	if f == nil {
		http.Error(w, "Tệp tin không tồn tại", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    f,
	})
}

// HandleSaveFile lưu hoặc tạo mới 1 tệp tin
func (h *LandingHandler) HandleSaveFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain   string `json:"domain"`
		PageID   string `json:"page_id"`
		FilePath string `json:"file_path"`
		FileType string `json:"file_type"`
		Content  string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.PageID == "" || payload.FilePath == "" {
		http.Error(w, "Thiếu page_id hoặc file_path", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveLandingFile(payload.Domain, payload.PageID, payload.FilePath, payload.FileType, payload.Content); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi lưu file: %v", err), http.StatusInternalServerError)
		return
	}

	// Nếu là index.html, cập nhật luôn landing_pages.code_html và làm mới cache RAM ngay lập tức
	if payload.FilePath == "index.html" {
		_ = h.store.UpdateLandingCode(payload.Domain, payload.PageID, payload.Content, "published")
		h.cache.Invalidate(payload.Domain, payload.PageID)
		p, _ := h.store.GetLandingPage(payload.Domain, payload.PageID)
		if p != nil {
			h.cache.Set(p)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã lưu tệp tin '%s'", payload.FilePath),
	})
}

// guessLandingFileType suy ra loại file từ đuôi tên file để lưu vào landing_files.
// Dùng chung cho HandleSaveFilesBatch & HandleDeployLanding.
func guessLandingFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".js":
		return "js"
	case ".json":
		return "json"
	case ".folder":
		return "folder"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogg":
		return "audio/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	default:
		return "text"
	}
}

// HandleSaveFilesBatch lưu hàng loạt nhiều tệp tin của landing trong 1 request.
//
// BỐI CẢNH (yêu cầu 2026-11-04):
// Code Studio cho phép upload file từ máy lên server (ảnh, video, âm thanh, tài liệu...)
// vào bất kỳ thư mục nào trong cây File Manager. Khi bấm "Lưu Code" hoặc "Deploy",
// toàn bộ file trong thư mục phải được lưu & deploy lên domain đích — không chỉ index.html.
//
// Endpoint này nhận danh sách file (file_path + content) và ghi tất cả vào bảng
// `landing_files` trong 1 lần gọi DB, thay vì N request riêng lẻ.
func (h *LandingHandler) HandleSaveFilesBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain string `json:"domain"`
		PageID string `json:"page_id"`
		Files  []struct {
			FilePath string `json:"file_path"`
			FileType string `json:"file_type"`
			Content  string `json:"content"`
		} `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	payload.PageID = strings.TrimSpace(payload.PageID)
	if payload.PageID == "" {
		http.Error(w, "Thiếu page_id", http.StatusBadRequest)
		return
	}
	if len(payload.Files) == 0 {
		http.Error(w, "Không có tệp tin nào để lưu", http.StatusBadRequest)
		return
	}

	saved := 0
	failed := make([]string, 0)
	indexHTML := ""

	for _, f := range payload.Files {
		fp := strings.TrimPrefix(strings.TrimSpace(f.FilePath), "/")
		if fp == "" {
			continue
		}
		// Chặn path traversal
		if strings.Contains(fp, "..") {
			failed = append(failed, f.FilePath)
			continue
		}

		fileType := strings.TrimSpace(f.FileType)
		if fileType == "" {
			fileType = guessLandingFileType(fp)
		}

		if err := h.store.SaveLandingFile(payload.Domain, payload.PageID, fp, fileType, f.Content); err != nil {
			failed = append(failed, fp)
			continue
		}
		// Giữ bản gốc (domain rỗng) luôn đồng bộ với bản theo domain,
		// để khi Deploy sang domain khác thì không bị thiếu file.
		if payload.Domain != "" {
			_ = h.store.SaveLandingFile("", payload.PageID, fp, fileType, f.Content)
		}
		saved++
		if fp == "index.html" {
			indexHTML = f.Content
		}
	}

	// Nếu có index.html → cập nhật landing_pages.code_html + làm mới cache RAM
	if indexHTML != "" {
		_ = h.store.UpdateLandingCode(payload.Domain, payload.PageID, indexHTML, "published")
		h.cache.Invalidate(payload.Domain, payload.PageID)
		p, _ := h.store.GetLandingPage(payload.Domain, payload.PageID)
		if p != nil {
			h.cache.Set(p)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  saved > 0,
		"saved":    saved,
		"failed":   failed,
		"message":  fmt.Sprintf("Đã lưu %d/%d tệp tin", saved, len(payload.Files)),
	})
}

// HandleDeleteFile xóa 1 tệp tin
func (h *LandingHandler) HandleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	filePath := strings.TrimSpace(r.URL.Query().Get("file_path"))

	if pageID == "" || filePath == "" {
		var payload struct {
			Domain   string `json:"domain"`
			PageID   string `json:"page_id"`
			FilePath string `json:"file_path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.PageID != "" {
			pageID = payload.PageID
		}
		if payload.FilePath != "" {
			filePath = payload.FilePath
		}
		if payload.Domain != "" {
			domain = payload.Domain
		}
	}

	if err := h.store.DeleteLandingFile(domain, pageID, filePath); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã xóa tệp tin '%s'", filePath),
	})
}

// HandleCreateFolder tạo mới 1 thư mục trong landing page
func (h *LandingHandler) HandleCreateFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain     string `json:"domain"`
		PageID     string `json:"page_id"`
		FolderPath string `json:"folder_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.PageID == "" || payload.FolderPath == "" {
		http.Error(w, "Thiếu page_id hoặc folder_path", http.StatusBadRequest)
		return
	}

	if err := h.store.CreateLandingFolder(payload.Domain, payload.PageID, payload.FolderPath); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi tạo thư mục: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("Đã tạo thư mục '%s'", payload.FolderPath),
		"folder_path": payload.FolderPath,
	})
}

// HandleDeleteFolder xóa 1 thư mục và toàn bộ files, thư mục con bên trong
func (h *LandingHandler) HandleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	folderPath := strings.TrimSpace(r.URL.Query().Get("folder_path"))

	if pageID == "" || folderPath == "" {
		var payload struct {
			Domain     string `json:"domain"`
			PageID     string `json:"page_id"`
			FolderPath string `json:"folder_path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.PageID != "" {
			pageID = payload.PageID
		}
		if payload.FolderPath != "" {
			folderPath = payload.FolderPath
		}
		if payload.Domain != "" {
			domain = payload.Domain
		}
	}

	if pageID == "" || folderPath == "" {
		http.Error(w, "Thiếu page_id hoặc folder_path", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteLandingFolder(domain, pageID, folderPath); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi xóa thư mục: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã xóa thư mục '%s' và toàn bộ nội dung", folderPath),
	})
}

// HandleRenameFolder đổi tên thư mục và cập nhật toàn bộ đường dẫn con
func (h *LandingHandler) HandleRenameFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain  string `json:"domain"`
		PageID  string `json:"page_id"`
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.PageID == "" || payload.OldPath == "" || payload.NewPath == "" {
		http.Error(w, "Thiếu page_id, old_path hoặc new_path", http.StatusBadRequest)
		return
	}

	if err := h.store.RenameLandingFolder(payload.Domain, payload.PageID, payload.OldPath, payload.NewPath); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi đổi tên thư mục: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã đổi tên thư mục '%s' thành '%s'", payload.OldPath, payload.NewPath),
	})
}

// HandleRenameFile đổi tên tệp tin
func (h *LandingHandler) HandleRenameFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Domain  string `json:"domain"`
		PageID  string `json:"page_id"`
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if payload.PageID == "" || payload.OldPath == "" || payload.NewPath == "" {
		http.Error(w, "Thiếu page_id, old_path hoặc new_path", http.StatusBadRequest)
		return
	}

	if err := h.store.RenameLandingFile(payload.Domain, payload.PageID, payload.OldPath, payload.NewPath); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi đổi tên tệp tin: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã đổi tên tệp tin '%s' thành '%s'", payload.OldPath, payload.NewPath),
	})
}

// HandleUploadFile tiếp nhận upload tệp tin assets: ảnh (jpg, png, webp, svg, gif), video (mp4, webm), âm thanh (mp3, wav, ogg)
func (h *LandingHandler) HandleUploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Giới hạn upload lên tới 100MB cho video và tài nguyên media
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "Kích thước tệp quá lớn hoặc định dạng form không hợp lệ (tối đa 100MB)", http.StatusBadRequest)
		return
	}

	pageID := strings.TrimSpace(r.FormValue("page_id"))
	domain := strings.TrimSpace(r.FormValue("domain"))
	targetFolder := strings.TrimSpace(r.FormValue("folder"))

	if pageID == "" {
		http.Error(w, "Thiếu page_id", http.StatusBadRequest)
		return
	}

	disallowedExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".sh": true,
		".ps1": true, ".php": true, ".jsp": true, ".asp": true,
	}

	targetFolder = strings.Trim(targetFolder, "/")

	var uploadedFiles []map[string]interface{}

	// Xử lý tất cả các tệp trong form
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, fileHeaders := range r.MultipartForm.File {
			for _, fHeader := range fileHeaders {
				fileName := filepath.Base(fHeader.Filename)
				fileName = strings.ReplaceAll(fileName, " ", "_")
				ext := strings.ToLower(filepath.Ext(fileName))

				if disallowedExts[ext] {
					continue
				}

				var relativePath string
				if targetFolder != "" {
					relativePath = targetFolder + "/" + fileName
				} else {
					relativePath = fileName
				}

				file, err := fHeader.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(file)
				file.Close()
				if err != nil {
					continue
				}

				mimeType := mime.TypeByExtension(ext)
				if mimeType == "" {
					switch ext {
					case ".webp":
						mimeType = "image/webp"
					case ".svg":
						mimeType = "image/svg+xml"
					case ".ico":
						mimeType = "image/x-icon"
					case ".mp4":
						mimeType = "video/mp4"
					case ".webm":
						mimeType = "video/webm"
					case ".ogg":
						mimeType = "audio/ogg"
					case ".mp3":
						mimeType = "audio/mpeg"
					case ".wav":
						mimeType = "audio/wav"
					case ".m4a":
						mimeType = "audio/mp4"
					default:
						mimeType = "application/octet-stream"
					}
				}

				var fileContent string
				if IsBinaryFileType(ext) {
					b64 := base64.StdEncoding.EncodeToString(data)
					fileContent = fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
				} else {
					fileContent = string(data)
				}

				if err := h.store.SaveLandingFile(domain, pageID, relativePath, mimeType, fileContent); err != nil {
					continue
				}
				if domain != "" {
					_ = h.store.SaveLandingFile("", pageID, relativePath, mimeType, fileContent)
				}

				uploadedFiles = append(uploadedFiles, map[string]interface{}{
					"file_path": relativePath,
					"file_type": mimeType,
					"size":      len(data),
					"url":       fmt.Sprintf("/api/landings/file/raw?page_id=%s&domain=%s&path=%s", pageID, domain, relativePath),
				})
			}
		}
	}

	if len(uploadedFiles) == 0 {
		http.Error(w, "Không có tệp tin hợp lệ nào được tải lên", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"files":     uploadedFiles,
		"file_path": uploadedFiles[0]["file_path"],
		"message":   fmt.Sprintf("Đã tải lên %d tệp tin thành công!", len(uploadedFiles)),
	})
}

// HandleServeRawFile phục vụ file binary (ảnh, video, audio) trực tiếp qua HTTP
func (h *LandingHandler) HandleServeRawFile(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	filePath := strings.TrimSpace(r.URL.Query().Get("path"))
	if filePath == "" {
		filePath = strings.TrimSpace(r.URL.Query().Get("file_path"))
	}
	if pageID == "" || filePath == "" {
		http.Error(w, "Thiếu page_id hoặc path", http.StatusBadRequest)
		return
	}

	f, err := h.store.GetLandingFile(domain, pageID, filePath)
	if err != nil || f == nil {
		f, err = h.store.GetLandingFile("", pageID, filePath)
	}
	if err != nil || f == nil {
		http.Error(w, "Tệp tin không tồn tại", http.StatusNotFound)
		return
	}

	ServeBinaryFile(w, r, f.FilePath, f.FileType, f.Content)
}

// ServeBinaryFile giải mã base64 (nếu có) và phục vụ dữ liệu binary qua HTTP với Cache-Control và Range header
func ServeBinaryFile(w http.ResponseWriter, r *http.Request, filePath string, fileType string, content string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		switch ext {
		case ".webp":
			mimeType = "image/webp"
		case ".svg":
			mimeType = "image/svg+xml"
		case ".ico":
			mimeType = "image/x-icon"
		case ".mp4":
			mimeType = "video/mp4"
		case ".webm":
			mimeType = "video/webm"
		case ".ogg":
			mimeType = "audio/ogg"
		case ".mp3":
			mimeType = "audio/mpeg"
		case ".wav":
			mimeType = "audio/wav"
		case ".m4a":
			mimeType = "audio/mp4"
		default:
			if fileType != "" && strings.Contains(fileType, "/") {
				mimeType = fileType
			} else {
				mimeType = "application/octet-stream"
			}
		}
	}

	if strings.HasPrefix(content, "data:") && strings.Contains(content, ";base64,") {
		idx := strings.Index(content, ";base64,")
		dataMime := strings.TrimPrefix(content[:idx], "data:")
		if dataMime != "" {
			mimeType = dataMime
		}
		b64Data := content[idx+8:]
		rawBytes, err := base64.StdEncoding.DecodeString(b64Data)
		if err == nil {
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeContent(w, r, filepath.Base(filePath), time.Now(), bytes.NewReader(rawBytes))
			return
		}
	} else if IsBinaryFileType(ext) {
		rawBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
		if err == nil && len(rawBytes) > 0 {
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeContent(w, r, filepath.Base(filePath), time.Now(), bytes.NewReader(rawBytes))
			return
		}
	}

	w.Header().Set("Content-Type", mimeType)
	_, _ = w.Write([]byte(content))
}

// IsBinaryFileType kiểm tra phần mở rộng có phải là định dạng nhị phân/media
func IsBinaryFileType(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".ico", ".bmp", ".avif",
		".mp4", ".webm", ".ogg", ".mov", ".avi", ".mkv",
		".mp3", ".wav", ".m4a", ".flac", ".aac",
		".woff", ".woff2", ".ttf", ".otf", ".eot", ".pdf":
		return true
	default:
		return false
	}
}

// ============================================================
// 4. LIVE PREVIEW & DYNAMIC RENDERING (3 KIỂU XEM: PC, LAPTOP, MOBILE)
// ============================================================

// HandlePreviewLanding hiển thị bản preview trực tiếp cho iframe trong admin
func (h *LandingHandler) HandlePreviewLanding(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.URL.Query().Get("page_id"))
	if pageID == "" {
		pageID = "mxv_080_online"
	}
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))

	p := h.cache.Get(domain, pageID)
	if p == nil {
		p = h.cache.GetByPageID(pageID)
	}

	if p == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding:50px;background:#0f172a;color:#fff"><h2>Landing Page chưa được tạo</h2><p>Vui lòng tạo hoặc lưu code trước khi xem trước.</p></body></html>`))
		return
	}

	rawHTML := p.CodeHTML
	// Nếu có bản nháp index.html trong landing_files, ưu tiên lấy bản nháp mới nhất
	if r.URL.Query().Get("draft") == "1" {
		if f, err := h.store.GetLandingFile(domain, pageID, "index.html"); err == nil && f != nil && strings.TrimSpace(f.Content) != "" {
			rawHTML = f.Content
		}
	}

	// Triệt để vô hiệu hóa việc tự động chuyển hướng Cookie 180 ngày sang chucmung.html trong môi trường preview
	rawHTML = strings.ReplaceAll(rawHTML, "window.location.replace('chucmung.html'", "/* [PREVIEW_BYPASS] */ console.log('Bypass preview redirect chucmung:', 'chucmung.html'")
	rawHTML = strings.ReplaceAll(rawHTML, "window.location.replace(\"chucmung.html\"", "/* [PREVIEW_BYPASS] */ console.log('Bypass preview redirect chucmung:', 'chucmung.html'")
	rawHTML = strings.ReplaceAll(rawHTML, "window.location.href = 'chucmung.html'", "/* [PREVIEW_BYPASS] */ console.log('Bypass preview redirect chucmung:', 'chucmung.html'")
	rawHTML = strings.ReplaceAll(rawHTML, "window.location.href = \"chucmung.html\"", "/* [PREVIEW_BYPASS] */ console.log('Bypass preview redirect chucmung:', 'chucmung.html'")
	rawHTML = strings.ReplaceAll(rawHTML, "redirectToChucMung()", "console.log('Bypass redirectToChucMung in preview')")

	renderedHTML := h.InjectDynamicScripts(rawHTML, p)

	// Đảm bảo chèn <base href="/"> và cờ Preview luôn nằm đầu thẻ <head>
	previewInjection := `
<base href="/">
<script>
window.__APEX_PREVIEW__ = true;
window.IS_PREVIEW = true;
window.__APEX_STUDIO__ = true;
window.__APEX_BUILDER_ACTIVE__ = true;
window.redirectToChucMung = function() { console.log('Bypass redirectToChucMung in preview'); };
</script>
`
	if strings.Contains(strings.ToLower(renderedHTML), "<head>") {
		idx := strings.Index(strings.ToLower(renderedHTML), "<head>") + 6
		renderedHTML = renderedHTML[:idx] + previewInjection + renderedHTML[idx:]
	} else {
		renderedHTML = previewInjection + renderedHTML
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(renderedHTML))
}

// htmlHasMetaPixel kiểm tra HTML thô đã chứa Meta Pixel (Facebook Pixel) hay chưa.
//
// Dùng để backend KHÔNG inject thêm block Meta Pixel khi landing đã hardcode sẵn
// (thường do export từ LadiPage) — tránh `fbq('init')` + `fbq('track','PageView')`
// chạy 2 lần ⇒ PageView nhân đôi ⇒ Meta Ads Manager nhận diện sai.
//
// Quy tắc:
//   - Có `fbq('init', '<pixelID>')` (đúng ID đang cấu hình) ⇒ true (đã có pixel).
//   - Có `fbq('init'` với ID khác ⇒ true (HTML đã tự quản lý pixel).
//   - Có `fbevents.js` ⇒ true (đã nạp thư viện Meta Pixel).
func htmlHasMetaPixel(rawHTML string, pixelID string) bool {
	if rawHTML == "" {
		return false
	}
	lower := strings.ToLower(rawHTML)

	// Đã nạp thư viện Meta Pixel ⇒ coi như đã có pixel (tránh chèn 2 lần).
	if strings.Contains(lower, "fbevents.js") {
		return true
	}

	// Đã có lệnh init pixel (bất kể ID nào) ⇒ không chèn thêm.
	if strings.Contains(lower, "fbq('init'") || strings.Contains(lower, "fbq(\"init\"") ||
		strings.Contains(lower, "fbq('init\"") || strings.Contains(lower, "fbq(\"init'") {
		return true
	}

	// Bắt biến thể gọi init không dùng chuỗi literal (vd: fbq("init", PIXEL_ID)).
	if idx := strings.Index(lower, "fbq("); idx != -1 {
		rest := lower[idx:]
		if end := strings.Index(rest, ");"); end != -1 && end < 200 {
			call := rest[:end]
			if strings.Contains(call, "'init'") || strings.Contains(call, "\"init\"") {
				return true
			}
			// fbq("init", ID) — ID có thể là biến; kiểm tra chuỗi pixel ID đã cấu hình.
			if pixelID != "" && strings.Contains(call, strings.ToLower(pixelID)) {
				return true
			}
		}
	}

	return false
}

// normalizeSuccessRedirectValue chuẩn hoá URL chuyển hướng sau khi đăng ký
// để tránh vòng lặp "quay lại trang có form" ⇒ khách bấm đăng ký lại ⇒ lead trùng.
//
// BỐI CẢNH (bug 2026-11-06 — landing_cu sinh lead trùng SĐT cùng thời gian):
//
//	`landing_pages.redirect_url` / `settings.form_config.success_redirect` được
//	cấu hình là "ads.giaodichhanghoatuonglai.com/chucmung" (có domain nhúng).
//	Trình duyệt phân giải URL tương đối theo `<base href="/landing/">` ⇒ thành
//	`/landing/ads.giaodichhanghoatuonglai.com/chucmung` — VẪN là trang CÓ FORM
//	⇒ khách bị quay lại form, bấm "Đăng ký" lần nữa ⇒ sinh lead trùng.
//
// QUY TẮC:
//   - URL tuyệt đối (http/https)          → giữ nguyên (chủ ý sang domain khác).
//   - "domain.com/path" (domain nhúng)    → bỏ domain, giữ "path"; nếu path không
//     có đuôi file thì thêm ".html" để KHÔNG rơi về index của landing.
//   - Đường dẫn tương đối bình thường      → giữ nguyên.
func normalizeSuccessRedirectValue(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}

	// URL tuyệt đối → giữ nguyên.
	if strings.HasPrefix(strings.ToLower(v), "http://") || strings.HasPrefix(strings.ToLower(v), "https://") {
		return v
	}

	// Bỏ query/hash.
	if idx := strings.IndexAny(v, "?#"); idx != -1 {
		v = v[:idx]
	}

	// "domain.com/path" (domain nhúng, KHÔNG có scheme) → chỉ giữ phần path.
	strippedDomain := false
	if slash := strings.Index(v, "/"); slash > 0 && strings.Contains(v[:slash], ".") {
		v = strings.TrimLeft(v[slash+1:], "/")
		strippedDomain = true
	}

	v = strings.TrimLeft(v, "/")
	if v == "" {
		return ""
	}

	// Trang nội bộ không có đuôi file → thêm ".html" (vd "chucmung" → "chucmung.html").
	if !strings.Contains(v, ".") && (strippedDomain || strings.EqualFold(v, "chucmung")) {
		v += ".html"
	}

	return v
}

// InjectDynamicScripts tự động chèn SEO meta, Facebook Pixel, TikTok Pixel, Google Analytics, và custom scripts
//
// `basePath` là prefix của URL đang phục vụ landing (vd: `/p/landing_cu`, `/landing`).
// Dùng để ghi đè các đường dẫn tài nguyên TƯƠNG ĐỐI (`tracker.js`, `config.json`)
// thành TUYỆT ĐỐI — tránh việc trình duyệt phân giải sai do thẻ `<base>` đặt sai
// chỗ trong HTML (bug tracker.js 404 — 2026-11-05). Xem `rewriteRelativeAssetPaths`.
func (h *LandingHandler) InjectDynamicScripts(rawHTML string, p *models.LandingPage) string {
	return h.InjectDynamicScriptsWithBase(rawHTML, p, "")
}

// InjectDynamicScriptsWithBase giống `InjectDynamicScripts` nhưng có thêm `basePath`
// để ghi đè đường dẫn tài nguyên tương đối thành tuyệt đối.
func (h *LandingHandler) InjectDynamicScriptsWithBase(rawHTML string, p *models.LandingPage, basePath string) string {
	if rawHTML == "" {
		rawHTML = `<!DOCTYPE html><html lang="vi"><head><meta charset="UTF-8"><title>Landing Page</title></head><body><h1>Trang Landing Mới</h1></body></html>`
	}

	settings := p.Settings
	if settings == nil {
		settings = make(map[string]interface{})
	}

	title, _ := settings["title"].(string)
	metaDesc, _ := settings["meta_description"].(string)
	ogImage, _ := settings["og_image"].(string)
	fbPixel, _ := settings["facebook_pixel_id"].(string)
	gaID, _ := settings["google_analytics_id"].(string)
	ttPixel, _ := settings["tiktok_pixel_id"].(string)
	headScripts, _ := settings["custom_head_scripts"].(string)
	bodyScripts, _ := settings["custom_body_scripts"].(string)

	// Chuẩn hoá các URL chuyển hướng trước khi gửi xuống client.
	//
	// Nếu giữ nguyên giá trị có domain nhúng (VD "ads.giaodichhanghoatuonglai.com/chucmung")
	// thì trình duyệt phân giải theo `<base href="/landing/">` ⇒ quay lại trang CÓ FORM
	// ⇒ khách bấm "Đăng ký" lần nữa ⇒ sinh lead trùng SĐT cùng thời gian.
	redirectURL := normalizeSuccessRedirectValue(p.RedirectURL)

	// `form_config` là map lồng trong `settings` (JSONB) — copy ra bản riêng để
	// chuẩn hoá `success_redirect` mà không sửa `settings` gốc.
	formConfigOut := settings["form_config"]
	settingsOut := settings
	if fc, ok := settings["form_config"].(map[string]interface{}); ok {
		fcCopy := make(map[string]interface{}, len(fc))
		for k, val := range fc {
			fcCopy[k] = val
		}
		if sr, ok := fcCopy["success_redirect"].(string); ok {
			if norm := normalizeSuccessRedirectValue(sr); norm != "" {
				fcCopy["success_redirect"] = norm
			}
		}
		formConfigOut = fcCopy

		// `settings` cũng được inject xuống client — cần bản copy đã chuẩn hoá
		// để không rò lại URL chứa domain (client đọc `settings.form_config`).
		settingsCopy := make(map[string]interface{}, len(settings))
		for k, val := range settings {
			settingsCopy[k] = val
		}
		settingsCopy["form_config"] = fcCopy
		settingsOut = settingsCopy
	}

	// Tạo snippet config tự động gắn vào window.APEX_LANDING_CONFIG
	configJSON, _ := json.Marshal(map[string]interface{}{
		"landing_id": p.PageID,
		"name":       p.Name,
		"domain":     p.Domain,
		"nguon":      p.NguonDefault,
		"is_active":  p.IsActive,
		"title":      title,
		// Lưu ý: các key "ép ghi đè" (hotline, zalo_link, brand_header, tieu_de,
		// metric_boxes) KHÔNG được inject xuống client nữa — HTML viết tay trong
		// Code Studio là nguồn sự thật duy nhất.
		// Giữ lại form_mapping (ánh xạ input → trường lead, không sửa nội dung)
		// và khoa_hoc (điền vào trường "package" của lead, không sửa DOM).
		"khoa_hoc":     settings["khoa_hoc"],
		"form_config":  formConfigOut,
		"form_mapping": settings["form_mapping"],
		"redirect_url": redirectURL,
		"settings":     settingsOut,
	})

	var headInjection strings.Builder
	headInjection.WriteString("\n<!-- [APEX DYNAMIC INJECTION - HEAD] -->\n")
	if !strings.Contains(strings.ToLower(rawHTML), "<base ") {
		baseHref := "/"
		if p.PathPrefix != "" && p.PathPrefix != "/" {
			cleanPfx := strings.Trim(p.PathPrefix, "/")
			if cleanPfx != "" {
				baseHref = "/" + cleanPfx + "/"
			}
		}
		headInjection.WriteString(fmt.Sprintf("<base href=\"%s\">\n", baseHref))
	}
	headInjection.WriteString(fmt.Sprintf("<script>window.APEX_LANDING_CONFIG = %s;</script>\n", string(configJSON)))

	if title != "" {
		headInjection.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(title)))
	}
	if metaDesc != "" {
		headInjection.WriteString(fmt.Sprintf("<meta name=\"description\" content=\"%s\">\n", html.EscapeString(metaDesc)))
	}
	if ogImage != "" {
		headInjection.WriteString(fmt.Sprintf("<meta property=\"og:image\" content=\"%s\">\n", html.EscapeString(ogImage)))
	}

	// Facebook Pixel Auto-Injection
	//
	// ⚠️ CHỐNG CHÈN TRÙNG PIXEL (sửa bug 2026-11-06):
	// Nhiều landing (vd: `landing_cu`) đã có SẴN block Meta Pixel hardcode trong
	// `code_html` (xuất bản từ LadiPage) — gọi `fbq('init', '<id>')` +
	// `fbq('track', 'PageView')`.
	//
	// Hệ quả khi backend vẫn inject thêm 1 block nữa:
	//   - `fbq('init')` bị gọi 2 lần ⇒ reset pixel ID, mất cơ chế event dedup
	//   - `fbq('track', 'PageView')` bị gọi 2 lần ⇒ PageView nhân đôi ⇒ Meta Ads
	//     Manager đếm sai lượt hiển thị, quy ngữ quy đổi (nhận diện sai).
	//
	// CÁCH SỬA: chỉ inject khi HTML thực sự CHƯA có Meta Pixel cho đúng pixel ID.
	// - `fbq('init', '<id>')` với đúng ID đã cấu hình ⇒ KHÔNG inject (đã có sẵn).
	// - Có `fbq('init')` với ID KHÁC ⇒ KHÔNG inject ID của ta (tránh 2 pixel
	//   cùng chạy, gây nhân đôi PageView).
	// - Có `fbevents.js` nhưng chưa `init` ⇒ vẫn KHÔNG inject (HTML chủ động).
	if fbPixel != "" && !htmlHasMetaPixel(rawHTML, fbPixel) {
		headInjection.WriteString(fmt.Sprintf(`
<!-- Facebook Pixel Code -->
<script>
!function(f,b,e,v,n,t,s){if(f.fbq)return;n=f.fbq=function(){n.callMethod?
n.callMethod.apply(n,arguments):n.queue.push(arguments)};if(!f._fbq)f._fbq=n;
n.push=n;n.loaded=!0;n.version='2.0';n.queue=[];t=b.createElement(e);t.async=!0;
t.src=v;s=b.getElementsByTagName(e)[0];s.parentNode.insertBefore(t,s)}(window,
document,'script','https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '%s');
fbq('track', 'PageView');
</script>
<noscript><img height="1" width="1" style="display:none" src="https://www.facebook.com/tr?id=%s&ev=PageView&noscript=1"/></noscript>
`, fbPixel, fbPixel))
	}

	// Google Analytics Auto-Injection
	if gaID != "" {
		headInjection.WriteString(fmt.Sprintf(`
<!-- Google Analytics -->
<script async src="https://www.googletagmanager.com/gtag/js?id=%s"></script>
<script>
window.dataLayer = window.dataLayer || [];
function gtag(){dataLayer.push(arguments);}
gtag('js', new Date());
gtag('config', '%s');
</script>
`, gaID, gaID))
	}

	// TikTok Pixel Auto-Injection
	if ttPixel != "" {
		headInjection.WriteString(fmt.Sprintf(`
<!-- TikTok Pixel -->
<script>
!function (w, d, t) {
  w.TiktokAnalyticsObject=t;var ttq=w[t]=w[t]||[];ttq.methods=["page","track","identify","instances","debug","on","off","once","ready","alias","group","enableCookie","disableCookie"],ttq.setAndDefer=function(t,e){t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}};for(var i=0;i<ttq.methods.length;i++)ttq.setAndDefer(ttq,ttq.methods[i]);ttq.instance=function(t){for(var e=ttq._i[t]||[],n=0;n<ttq.methods.length;n++)ttq.setAndDefer(e,ttq.methods[n]);return e};ttq.load=function(e,n){var i="https://analytics.tiktok.com/i18n/pixel/events.js";ttq._i=ttq._i||{},ttq._i[e]=[],ttq._i[e]._u=i,ttq._t=ttq._t||{},ttq._t[e]=+new Date,ttq._o=ttq._o||{},ttq._o[e]=n||{};var o=document.createElement("script");o.type="text/javascript",o.async=!0,o.src=i+"?sdkid="+e+"&lib="+t;var a=document.getElementsByTagName("script")[0];a.parentNode.insertBefore(o,a)};
  ttq.load('%s');
  ttq.page();
}(window, document, 'ttq');
</script>
`, ttPixel))
	}

	// Custom Head Scripts
	if headScripts != "" {
		headInjection.WriteString("\n<!-- Custom User Head Scripts -->\n")
		headInjection.WriteString(headScripts)
		headInjection.WriteString("\n")
	}

	// Inject vào trước </head>
	result := rawHTML
	if strings.Contains(strings.ToLower(result), "</head>") {
		idx := strings.Index(strings.ToLower(result), "</head>")
		result = result[:idx] + headInjection.String() + result[idx:]
	} else {
		result = headInjection.String() + result
	}

	// ⚠️ SCRIPT CONFIG PHẢI CHẠY TRƯỚC `tracker.js` (bug chồng chéo landing — 2026-11-05)
	//
	// LÝ DO: tracker.js gọi `loadConfigFile()` ngay khi được parse. Nếu script config
	// của backend nằm SAU tracker.js trong HTML, tracker đọc
	// `window.APEX_LANDING_CONFIG` fallback của template (vd: `mxv_080_online`) ⇒
	// analytics/lead gửi về nhầm landing.
	//
	// CÁCH SỬA: cài "bẫy ghi đè" (property trap) ở ĐẦU <head> — trước mọi script
	// khác. Mỗi lần HTML (template hoặc backend) gán lại `window.APEX_LANDING_CONFIG`,
	// trap MERGE lại các key mới và ép `landing_id` / `domain` / `name` về đúng landing
	// đang phục vụ. Nhờ vậy các key riêng của template (`thoi_gian_chia_se`,
	// `khoa_hoc`, `backend_url`...) vẫn còn nguyên.
	configTrapScript := fmt.Sprintf(`<script>
(function(){
  var __apexCfg = window.APEX_LANDING_CONFIG || {};
  var __apexForce = %s;
  function __apexMerge(v){
    if (v && typeof v === 'object'){
      for (var k in v){ if (Object.prototype.hasOwnProperty.call(v,k)) __apexCfg[k] = v[k]; }
    }
    __apexCfg.landing_id = __apexForce.landing_id;
    __apexCfg.domain = __apexForce.domain;
    __apexCfg.name = __apexForce.name;
    return __apexCfg;
  }
  __apexMerge(window.APEX_LANDING_CONFIG);
  try {
    Object.defineProperty(window, 'APEX_LANDING_CONFIG', {
      configurable: true,
      enumerable: true,
      get: function(){ return __apexCfg; },
      set: function(v){ __apexMerge(v); }
    });
  } catch(e) {
    window.APEX_LANDING_CONFIG = __apexCfg;
  }
})();
</script>`, string(configJSON))

	if idx := strings.Index(strings.ToLower(result), "<head>"); idx != -1 {
		insertAt := idx + len("<head>")
		result = result[:insertAt] + "\n" + configTrapScript + result[insertAt:]
	}

	// Custom Body Scripts inject vào trước </body>
	if bodyScripts != "" {
		var bodyInjection strings.Builder
		bodyInjection.WriteString("\n<!-- [APEX DYNAMIC INJECTION - BODY] -->\n")
		bodyInjection.WriteString(bodyScripts)
		bodyInjection.WriteString("\n")

		if strings.Contains(strings.ToLower(result), "</body>") {
			idx := strings.Index(strings.ToLower(result), "</body>")
			result = result[:idx] + bodyInjection.String() + result[idx:]
		} else {
			result = result + bodyInjection.String()
		}
	}

	// Gắn `data-landing-id` vào thẻ <script src="tracker.js"> đã có sẵn trong HTML.
	//
	// LÝ DO (bug chồng chéo landing — 2026-11-05): tracker.js đọc landing_id từ
	// `window.APEX_LANDING_CONFIG` TRƯỚC khi config được nạp. Nếu HTML template có
	// sẵn `<script src="tracker.js">` mà không có `data-landing-id`, tracker rơi về
	// fallback của template (`mxv_080_online`).
	// `data-landing-id` là nguồn định danh chắc chắn nhất — luôn đúng landing đang
	// phục vụ, không phụ thuộc thứ tự script hay config.
	if strings.Contains(result, "tracker.js") {
		// <script src="tracker.js"> (chưa có data-landing-id) → thêm attribute
		result = strings.ReplaceAll(result,
			`<script src="tracker.js">`,
			fmt.Sprintf(`<script src="tracker.js" data-landing-id="%s">`, p.PageID))
		result = strings.ReplaceAll(result,
			`<script src="/tracker.js">`,
			fmt.Sprintf(`<script src="/tracker.js" data-landing-id="%s">`, p.PageID))
		// <script src="tracker.js" ...> đã có attribute khác → chèn data-landing-id vào
		result = strings.ReplaceAll(result,
			`<script src="tracker.js" data-nguon=`,
			fmt.Sprintf(`<script src="tracker.js" data-landing-id="%s" data-nguon=`, p.PageID))
	}

	// Tự động kiểm tra và chèn tracker.js kèm form interceptor nếu trang chưa có.
	//
	// Dùng `basePath` (prefix của URL đang phục vụ) để trỏ đúng vào tracker.js
	// CỦA CHÍNH landing này — tránh dùng `/tracker.js` ở gốc, vì gốc có thể trỏ
	// tới landing KHÁC (bug chồng chéo landing — 2026-11-05).
	trackerHref := "/tracker.js"
	if basePath != "" && basePath != "/" {
		trackerHref = basePath + "/tracker.js"
	}
	if !strings.Contains(result, "tracker.js") {
		trackerScript := fmt.Sprintf("\n<!-- [APEX AUTO-INJECT TRACKER & FORM INTERCEPTOR] -->\n<script src=\"%s\" data-landing-id=\"%s\"></script>\n", trackerHref, p.PageID)
		if strings.Contains(strings.ToLower(result), "</body>") {
			idx := strings.Index(strings.ToLower(result), "</body>")
			result = result[:idx] + trackerScript + result[idx:]
		} else {
			result = result + trackerScript
		}
	}

	result = injectRegistrationFormGuard(result)

	// Ghi đè đường dẫn tài nguyên tương đối → tuyệt đối (bug tracker.js 404).
	// PHẢI làm SAU cùng với việc chèn/gắn `data-landing-id` ở trên để thẻ
	// `<script src="tracker.js">` đã chuẩn hoá về 1 định dạng duy nhất.
	result = rewriteRelativeAssetPaths(result, basePath)

	return result
}

// rewriteRelativeAssetPaths đổi các đường dẫn tài nguyên TƯƠNG ĐỐI trong HTML
// thành đường dẫn TUYỆT ĐỐI, dựa trên `basePath` (prefix của URL đang phục vụ).
//
// BỐI CẢNH (bug tracker.js 404 — 2026-11-05):
//   Landing HTML do LadiPage export có thể chứa `<base href="/landing/">` ĐẶT SAU
//   thẻ `<script src="tracker.js">`. Theo HTML spec, `<base>` chỉ có hiệu lực với
//   các URL được phân giải SAU khi thẻ `<base>` được parse. Vì vậy trình duyệt
//   phân giải `src="tracker.js"` theo URL của DOCUMENT, KHÔNG theo `<base>`.
//
//   Hậu quả trên production:
//     - Landing phục vụ tại `/p/landing_cu` ⇒ trình duyệt gọi `/p/tracker.js`
//       ⇒ 404 ⇒ `window.ApexTracker` KHÔNG tồn tại ⇒ landing tự gửi lead không
//       được (hàm `if (window.ApexTracker && ...)` bị bỏ qua).
//     - Landing phục vụ tại `/landing/` ⇒ gọi `/landing/tracker.js` ⇒ OK.
//
//   ⇒ Sửa bằng cách ghi đè `src` của các tài nguyên dùng chung thành đường dẫn
//     TUYỆT ĐỐI theo `basePath` (ví dụ `/p/landing_cu/tracker.js`), không phụ thuộc
//     vị trí của thẻ `<base>` trong HTML.
//
// LƯU Ý: chỉ sửa các đường dẫn tài nguyên dùng chung (tracker.js, config.json).
// Đường dẫn tuyệt đối NGOÀI domain (bắt đầu bằng `http://`, `https://`, `//`) được
// giữ nguyên. Đường dẫn gốc `/tracker.js` cũng được ghi đè khi `basePath` khác
// `/`, vì gốc có thể trỏ tới landing KHÁC (bug chồng chéo landing — 2026-11-05).
func rewriteRelativeAssetPaths(html string, basePath string) string {
	if basePath == "" {
		return html
	}
	basePath = strings.TrimSuffix(basePath, "/")

	// `<script src="tracker.js" ...>` và `<script src="config.json" ...>`
	reScript := regexp.MustCompile(`(?i)(<script\b[^>]*\bsrc=")(tracker\.js|config\.json)(")`)
	html = reScript.ReplaceAllString(html, "${1}"+basePath+"/${2}${3}")

	// `<link ... href="config.json">` / `href="tracker.js"` (hiếm gặp nhưng có thể có)
	reLink := regexp.MustCompile(`(?i)(<link\b[^>]*\bhref=")(tracker\.js|config\.json)(")`)
	html = reLink.ReplaceAllString(html, "${1}"+basePath+"/${2}${3}")

	// Đường dẫn GỐC `/tracker.js` → gắn thêm prefix của landing đang phục vụ.
	if basePath != "" && basePath != "/" {
		reRootScript := regexp.MustCompile(`(?i)(<script\b[^>]*\bsrc=")/(tracker\.js|config\.json)(")`)
		html = reRootScript.ReplaceAllString(html, "${1}"+basePath+"/${2}${3}")
		reRootLink := regexp.MustCompile(`(?i)(<link\b[^>]*\bhref=")/(tracker\.js|config\.json)(")`)
		html = reRootLink.ReplaceAllString(html, "${1}"+basePath+"/${2}${3}")
	}

	return html
}

// injectRegistrationFormGuard đặt cờ `window.__REGISTRATION_FORM_HANDLED__ = true`
// cho các landing TỰ XỬ LÝ form đăng ký của riêng nó.
//
// BỐI CẢNH (bug 2026-11-05 — lead thật bị gắn nhầm "Lỗi Proof-of-Work"):
//   `tracker.js` có sẵn "form interceptor" (initAutoFormInterceptor) gắn listener
//   submit cho mọi form. Landing nào đã có handler riêng (capture listener +
//   `onsubmit="handleFormSubmit"`) mà KHÔNG báo cho tracker.js biết ⇒ 1 lần bấm
//   sinh 3 POST /api/leads với CÙNG `challenge_id` + `challenge_solution` ⇒
//   request thứ 2, #3 bị server ghi vào `spam_quarantine` với lý do "Replay attack
//   detected" ⇒ LEAD THẬT hiển thị sai là "Lỗi Proof-of-Work" trong tab
//   Bảo mật & Bot (dù lead vẫn về CRM + Google Sheets đúng).
//
//   Việc sửa trong HTML template (`frontend/landing/index.html`) là KHÔNG ĐỦ,
//   vì HTML thực tế của mỗi landing nằm trong CSDL (`landing_pages.code_html`)
//   và được người dùng sửa trong Code Studio.
//
// ⇒ Vì vậy, server tự động đặt cờ khi phát hiện landing TỰ GỌI
//   `sendLeadToBackend` cho form `registrationForm` mà chưa khai báo cờ.
//   Đây là lớp bảo vệ CUỐI CÙNG — không phụ thuộc nội dung HTML trong DB.
//
// ĐIỀU KIỆN đặt cờ (cả 3 phải đúng):
//   1. HTML có gọi `sendLeadToBackend` (landing tự gửi lead).
//   2. HTML có form `registrationForm`.
//   3. HTML CHƯA tự đặt `__REGISTRATION_FORM_HANDLED__` (tránh ghi đè ý định
//      của người dùng — ví dụ landing muốn tracker.js gửi thay).
//
// ⇒ Cờ được đặt ở ĐẦU <head> để chắc chắn có hiệu lực TRƯỚC khi
//   `initAutoFormInterceptor` chạy.
func injectRegistrationFormGuard(html string) string {
	if !strings.Contains(html, "sendLeadToBackend") {
		return html
	}
	if !strings.Contains(html, "registrationForm") {
		return html
	}
	// Đã có cờ do landing tự khai báo → không ghi đè.
	if strings.Contains(html, "__REGISTRATION_FORM_HANDLED__") {
		return html
	}

	guard := `<script>
/* [APEX FORM GUARD] Landing này tự xử lý form đăng ký (đã gọi ApexTracker.sendLeadToBackend).
   Báo cho tracker.js BỎ QUA form registrationForm để 1 lần bấm chỉ sinh 1 request /api/leads.
   Nếu không, tracker.js gửi thêm 1 lần với cùng challenge_id + challenge_solution
   ⇒ server ghi vào spam_quarantine ("Lỗi Proof-of-Work") ⇒ lead thật bị lọc nhầm. */
window.__REGISTRATION_FORM_HANDLED__ = true;
</script>
`

	// Ưu tiên chèn ngay sau <head> để chạy TRƯỚC tracker.js.
	lower := strings.ToLower(html)
	if idx := strings.Index(lower, "<head>"); idx != -1 {
		insertAt := idx + len("<head>")
		return html[:insertAt] + "\n" + guard + html[insertAt:]
	}
	if idx := strings.Index(lower, "<head"); idx != -1 {
		// <head ...> — tìm điểm kết thúc thẻ mở.
		if gt := strings.Index(lower[idx:], ">"); gt != -1 {
			insertAt := idx + gt + 1
			return html[:insertAt] + "\n" + guard + html[insertAt:]
		}
	}
	return guard + html
}

// ============================================================
// 5. ROUTING MIDDLEWARE & DYNAMIC SERVING (/p/:page_id)
// ============================================================

// ServePublicLanding phục vụ landing page công khai tại /p/:page_id
func (h *LandingHandler) ServePublicLanding(w http.ResponseWriter, r *http.Request) {
	// Trích xuất page_id từ URL path: /p/mxv_080_online hoặc /p/mxv_080_online/style.css
	path := strings.TrimPrefix(r.URL.Path, "/p/")
	parts := strings.SplitN(path, "/", 2)
	pageID := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	if pageID == "" {
		pageID = "mxv_080_online"
	}

	// Nếu điều hướng tới chucmung.html
	if pageID == "chucmung.html" || strings.HasSuffix(pageID, "/chucmung.html") {
		http.Redirect(w, r, "/chucmung.html", http.StatusFound)
		return
	}

	// Kiểm tra nếu là sub-resource (css, js, images)
	if subPath != "" && subPath != "index.html" {
		if subPath == "chucmung.html" {
			http.Redirect(w, r, "/chucmung.html", http.StatusFound)
			return
		}

		f, err := h.store.GetLandingFile(r.Host, pageID, subPath)
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

		// Fallback tìm tệp tĩnh từ thư mục web gốc an toàn
		cleanSub := filepath.Clean(filepath.Base(subPath))
		candidates := []string{
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

		http.NotFound(w, r)
		return
	}

	// Lấy Landing Page từ RAM Cache
	p := h.cache.Get(r.Host, pageID)
	if p == nil {
		p = h.cache.GetByPageID(pageID)
	}

	if p == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="vi"><head><meta charset="UTF-8"><title>404 - Không tìm thấy trang</title><style>body{background:#061120;color:#fff;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;text-align:center}.box{background:#0e2342;padding:40px;border-radius:16px;border:1px solid #1e3a6a;max-width:450px}</style></head><body><div class="box"><h1 style="color:#ef5222;margin-top:0">404</h1><h2>Trang không tồn tại</h2><p style="color:#94a3b8">Đường dẫn bạn yêu cầu không khả dụng hoặc đã bị thay đổi.</p></div></body></html>`))
		return
	}

	// ROUTING MIDDLEWARE: Kiểm tra cờ is_active (Feature Flag Kill-Switch)
	if !p.IsActive {
		if p.RedirectURL != "" {
			http.Redirect(w, r, p.RedirectURL, http.StatusFound)
			return
		}
		// Trang bảo trì / tạm dừng
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="vi"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Thông Báo - Trang Tạm Ngưng Nhận Đăng Ký</title><style>body{background:#061120;color:#f8fafc;font-family:'Segoe UI',sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;padding:20px;box-sizing:border-box}.card{background:#0A1C33;border:1px solid #1E3A60;border-radius:24px;padding:40px 32px;max-width:520px;text-align:center;box-shadow:0 25px 50px -12px rgba(0,0,0,0.5)}.icon{width:72px;height:72px;background:rgba(239,82,34,0.15);border:2px solid #ef5222;border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto 24px;font-size:32px}.btn{display:inline-block;background:linear-gradient(135deg,#EF5222,#F97316);color:#fff;font-weight:700;padding:14px 28px;border-radius:12px;text-decoration:none;margin-top:20px}</style></head><body><div class="card"><div class="icon">🔒</div><h2 style="margin:0 0 12px;font-size:24px;color:#fff">Trang Tạm Ngưng Nhận Đăng Ký</h2><p style="color:#94a3b8;font-size:15px;line-height:1.6;margin:0 0 20px">Chương trình hiện tại đã đủ số lượng học viên hoặc đang trong thời gian bảo trì nâng cấp hệ thống. Quý khách vui lòng liên hệ hotline hoặc quay lại sau.</p><a href="tel:0984386538" class="btn">Hotline: 0984.386.538</a></div></body></html>`))
		return
	}

	// Trang hoạt động bình thường -> Render mã HTML kèm Auto-Injection
	//
	// `basePath` = `/p/<page_id>` ⇒ các tài nguyên tương đối (`tracker.js`,
	// `config.json`) được ghi đè thành đường dẫn TUYỆT ĐỐI, không phụ thuộc vị
	// trí thẻ `<base>` trong HTML (bug tracker.js 404 — 2026-11-05).
	renderedHTML := h.InjectDynamicScriptsWithBase(p.CodeHTML, p, "/p/"+pageID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// HTML landing chứa `data-landing-id` + `window.APEX_LANDING_CONFIG` quyết định
	// analytics ghi về landing nào ⇒ KHÔNG được trình duyệt cache lại.
	// (bug chồng chéo landing — 2026-11-05)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write([]byte(renderedHTML))
}

// CheckLandingActive kiểm tra trạng thái active của trang phục vụ routing middleware
func (h *LandingHandler) CheckLandingActive(domain, pageID string) (bool, string) {
	p := h.cache.Get(domain, pageID)
	if p == nil {
		p = h.cache.GetByPageID(pageID)
	}
	if p == nil {
		return true, "" // Nếu chưa cấu hình thì mặc định cho phép chạy
	}
	return p.IsActive, p.RedirectURL
}
