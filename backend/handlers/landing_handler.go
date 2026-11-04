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

	pubConfig := models.LandingPublicConfig{
		LandingID:         p.PageID,
		Name:              p.Name,
		Domain:            p.Domain,
		Nguon:             p.NguonDefault,
		IsActive:          p.IsActive,
		Status:            p.Status,
		RedirectURL:       p.RedirectURL,
		Settings:          settings,
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

	// 2. Cập nhật mã nguồn và trạng thái 'published' cho bản ghi gốc (domain "")
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
		_ = h.store.SyncLandingFilesToDomain("", targetDomain, payload.PageID)
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

// InjectDynamicScripts tự động chèn SEO meta, Facebook Pixel, TikTok Pixel, Google Analytics, và custom scripts
func (h *LandingHandler) InjectDynamicScripts(rawHTML string, p *models.LandingPage) string {
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

	// Tạo snippet config tự động gắn vào window.APEX_LANDING_CONFIG
	configJSON, _ := json.Marshal(map[string]interface{}{
		"landing_id":        p.PageID,
		"name":              p.Name,
		"domain":            p.Domain,
		"nguon":             p.NguonDefault,
		"is_active":         p.IsActive,
		"title":             title,
		// Lưu ý: các key "ép ghi đè" (hotline, zalo_link, brand_header, tieu_de,
		// metric_boxes) KHÔNG được inject xuống client nữa — HTML viết tay trong
		// Code Studio là nguồn sự thật duy nhất.
		// Giữ lại form_mapping (ánh xạ input → trường lead, không sửa nội dung)
		// và khoa_hoc (điền vào trường "package" của lead, không sửa DOM).
		"khoa_hoc":          settings["khoa_hoc"],
		"form_config":       settings["form_config"],
		"form_mapping":      settings["form_mapping"],
		"redirect_url":      p.RedirectURL,
		"settings":          settings,
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
	if fbPixel != "" {
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

	// Tự động kiểm tra và chèn tracker.js kèm form interceptor nếu trang chưa có
	if !strings.Contains(result, "tracker.js") {
		trackerScript := fmt.Sprintf("\n<!-- [APEX AUTO-INJECT TRACKER & FORM INTERCEPTOR] -->\n<script src=\"/tracker.js\" data-landing-id=\"%s\"></script>\n", p.PageID)
		if strings.Contains(strings.ToLower(result), "</body>") {
			idx := strings.Index(strings.ToLower(result), "</body>")
			result = result[:idx] + trackerScript + result[idx:]
		} else {
			result = result + trackerScript
		}
	}

	return result
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
	renderedHTML := h.InjectDynamicScripts(p.CodeHTML, p)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
