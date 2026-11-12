package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/models"
	"apex-backend/security"

	"golang.org/x/time/rate"
)

type AnalyticsHandler struct {
	cfg         *config.Config
	store       *db.Store
	rateLimiter *security.IPRateLimiter
}

func NewAnalyticsHandler(cfg *config.Config, store *db.Store) *AnalyticsHandler {
	return &AnalyticsHandler{
		cfg:         cfg,
		store:       store,
		rateLimiter: security.NewIPRateLimiter(rate.Limit(2.0), 60, 5*time.Minute), // 120 req/phút, burst 60
	}
}

// detectServerBot phân tích User-Agent để phát hiện bot/crawler phía máy chủ
func detectServerBot(ua string) (bool, int, string) {
	if strings.TrimSpace(ua) == "" {
		return true, 90, "empty_user_agent"
	}
	uaLower := strings.ToLower(ua)

	botSignatures := []struct {
		keyword string
		reason  string
	}{
		{"googlebot", "google_crawler"},
		{"bingbot", "bing_crawler"},
		{"yandexbot", "yandex_crawler"},
		{"baiduspider", "baidu_crawler"},
		{"facebookexternalhit", "facebook_crawler"},
		{"facebot", "facebook_bot"},
		{"bytespider", "bytedance_tiktok_crawler"},
		{"petalbot", "petal_crawler"},
		{"ahrefsbot", "ahrefs_seo_crawler"},
		{"semrushbot", "semrush_seo_crawler"},
		{"dotbot", "dotbot_crawler"},
		{"mj12bot", "majestic_crawler"},
		{"headlesschrome", "headless_browser"},
		{"phantomjs", "phantomjs_headless"},
		{"selenium", "selenium_automation"},
		{"puppeteer", "puppeteer_automation"},
		{"playwright", "playwright_automation"},
		{"lighthouse", "google_lighthouse_audit"},
		{"prerender", "prerender_service"},
		{"python-requests", "python_script"},
		{"python-urllib", "python_script"},
		{"curl/", "curl_cli"},
		{"wget/", "wget_cli"},
		{"postmanruntime", "postman_client"},
		{"go-http-client", "go_http_client"},
		{"java/", "java_client"},
		{"node-fetch", "node_fetch_script"},
		{"axios/", "axios_script"},
		{"crawl", "generic_crawler"},
		{"spider", "generic_spider"},
		{"bot", "generic_bot"},
	}

	for _, sig := range botSignatures {
		if strings.Contains(uaLower, sig.keyword) {
			return true, 100, sig.reason
		}
	}

	return false, 0, ""
}

// parseUserAgentInfo trích xuất Device, Browser và OS từ User-Agent
func parseUserAgentInfo(ua string) (deviceType, browser, os string) {
	uaLower := strings.ToLower(ua)

	// 1. Phân loại thiết bị
	if strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "tablet") {
		deviceType = "tablet"
	} else if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "android") || strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipod") {
		deviceType = "mobile"
	} else {
		deviceType = "desktop"
	}

	// 2. Trình duyệt
	if strings.Contains(uaLower, "zalo") {
		browser = "Zalo In-App"
	} else if strings.Contains(uaLower, "fbav") || strings.Contains(uaLower, "fban") {
		browser = "Facebook In-App"
	} else if strings.Contains(uaLower, "coccoc") {
		browser = "Cốc Cốc"
	} else if strings.Contains(uaLower, "edg") {
		browser = "Edge"
	} else if strings.Contains(uaLower, "chrome") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome") {
		browser = "Safari"
	} else if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(uaLower, "opera") || strings.Contains(uaLower, "opr") {
		browser = "Opera"
	} else {
		browser = "Other"
	}

	// 3. Hệ điều hành
	if strings.Contains(uaLower, "windows") {
		os = "Windows"
	} else if strings.Contains(uaLower, "android") {
		os = "Android"
	} else if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "ipod") || strings.Contains(uaLower, "ios") {
		os = "iOS"
	} else if strings.Contains(uaLower, "macintosh") || strings.Contains(uaLower, "mac os") {
		os = "macOS"
	} else if strings.Contains(uaLower, "linux") {
		os = "Linux"
	} else {
		os = "Other"
	}

	return deviceType, browser, os
}

// HandleTrackEvent tiếp nhận sự kiện hành vi và cập nhật phiên người dùng
func (h *AnalyticsHandler) HandleTrackEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	// Rate limiting theo IP cho analytics (120 req/phút)
	clientIP := getClientIP(r)
	if !h.rateLimiter.Allow(clientIP) {
		w.WriteHeader(http.StatusNoContent) // Silent drop
		return
	}

	// Giới hạn dung lượng tối đa 64KB cho 1 payload tracking
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || len(bodyBytes) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Body rỗng hoặc vượt quá 64KB"})
		return
	}

	var p models.AnalyticsEventPayload
	if err := json.Unmarshal(bodyBytes, &p); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "JSON không hợp lệ: " + err.Error()})
		return
	}

	if p.SessionID == "" {
		p.SessionID = "sess_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	if p.VisitorID == "" {
		p.VisitorID = p.SessionID
	}
	if p.LandingID == "" {
		p.LandingID = "default_landing"
	}
	if p.Nguon == "" {
		p.Nguon = "direct"
	}
	if len(p.Events) == 0 && len(p.Actions) > 0 {
		p.Events = p.Actions
	}

	// Trích xuất IP thực và User-Agent
	rawUA := r.UserAgent()

	// ─── Phân tích thiết bị chi tiết (model thật, trình duyệt trong app, ...) ───
	//
	// Ưu tiên dữ liệu client gửi lên (JS có thể đọc navigator.userAgentData),
	// fallback về phân tích User-Agent phía server.
	if p.DeviceType == "" || p.Browser == "" || p.OS == "" || p.DeviceModel == "" {
		devInfo := security.ParseDeviceInfo(rawUA)
		if p.DeviceType == "" {
			p.DeviceType = devInfo.DeviceType
		}
		if p.Browser == "" {
			p.Browser = devInfo.Browser
		}
		if p.OS == "" {
			p.OS = devInfo.OS
		}
		if p.DeviceBrand == "" {
			p.DeviceBrand = devInfo.Brand
		}
		if p.DeviceModel == "" {
			p.DeviceModel = devInfo.Model
		}
		if p.DeviceModelCode == "" {
			p.DeviceModelCode = devInfo.ModelCode
		}
		if p.DeviceName == "" {
			p.DeviceName = devInfo.Model
		}
		if p.OSVersion == "" {
			p.OSVersion = devInfo.OSVersion
		}
		if p.BrowserVersion == "" {
			p.BrowserVersion = devInfo.BrowserVersion
		}
		if p.BrowserEngine == "" {
			p.BrowserEngine = devInfo.Engine
		}
		if !p.IsInAppBrowser && devInfo.IsInAppBrowser {
			p.IsInAppBrowser = true
			p.InAppType = devInfo.InAppType
		}
		if p.PlatformInfo == "" {
			p.PlatformInfo = devInfo.Platform
		}
	}

	// ─── Phát hiện VPN / Proxy / Tor ───
	//
	// Dựa trên: IP thuộc dải datacenter/VPN đã biết + header proxy tiêu chuẩn.
	if !p.IsVPN && !p.IsProxy {
		headers := map[string]string{}
		for _, key := range []string{
			"X-Forwarded-For", "X-Real-IP", "Via", "X-Proxy-Id",
			"Proxy-Connection", "X-Forwarded-Proto", "CF-Connecting-IP",
		} {
			if v := r.Header.Get(key); v != "" {
				headers[strings.ToLower(key)] = v
			}
		}
		vpn := security.ResolveVPN(clientIP, headers)
		if vpn != nil {
			if vpn.IsVPN {
				p.IsVPN = true
			}
			if vpn.IsProxy {
				p.IsProxy = true
			}
			if vpn.IsTor {
				p.IsTor = true
			}
			if p.VPNProvider == "" {
				p.VPNProvider = vpn.Provider
			}
			if vpn.ConfidencePct > p.VPNConfidence {
				p.VPNConfidence = vpn.ConfidencePct
			}
			if p.VPNReason == "" {
				p.VPNReason = vpn.Reason
			}
		}
	}

	// ─── Thu thập mã định danh lượt nhấp Meta (CAPI) ───
	//
	// Ưu tiên giá trị client gửi lên (đã đọc cookie `_fbc`/`_fbp` ở JS),
	// fallback đọc từ query string entry_url.
	if p.Fbclid == "" || p.Fbc == "" || p.Fbp == "" {
		cookies := map[string]string{}
		if c, err := r.Cookie("fbc"); err == nil {
			cookies["_fbc"] = c.Value
		}
		if c, err := r.Cookie("fbp"); err == nil {
			cookies["_fbp"] = c.Value
		}
		ids := security.ResolveMetaClickIDs(p.EntryURL, cookies)
		if p.Fbclid == "" {
			p.Fbclid = ids.Fbclid
		}
		if p.Fbc == "" {
			p.Fbc = ids.Fbc
		}
		if p.Fbp == "" {
			p.Fbp = ids.Fbp
		}
		if p.Igshid == "" {
			p.Igshid = ids.Igshid
		}
	}
	if p.MetaClickID == "" && p.Fbclid != "" {
		p.MetaClickID = p.Fbclid
	}

	// ─── Bổ sung macro Meta Ads từ query string nếu client chưa gửi ───
	//
	// Meta thay thế macro trong URL lúc click, ví dụ:
	//   ?utm_campaign={{campaign.id}}&campaign_name={{campaign.name}}
	// ⇒ sau khi Meta render, URL thật sẽ chứa giá trị cụ thể.
	applyMetaMacrosFromURL(&p)

	// Kiểm tra bot phía server
	serverIsBot, serverScore, serverReason := detectServerBot(rawUA)
	if serverIsBot {
		p.IsBot = true
		if p.BotScore < serverScore {
			p.BotScore = serverScore
		}
		if p.BotReasons == "" {
			p.BotReasons = serverReason
		} else if !strings.Contains(p.BotReasons, serverReason) {
			p.BotReasons += "; " + serverReason
		}
	}

	// Phân giải địa lý & nhà mạng thực tế từ IP khách truy cập
	geo := security.ResolveIP(clientIP)
	if p.City == "" {
		p.City = geo.City
	}
	if p.Region == "" {
		p.Region = geo.Region
	}
	if p.Country == "" {
		p.Country = geo.Country
	}
	if p.ISP == "" {
		p.ISP = geo.ISP
	}

	// Phân tích vị trí quảng cáo và kênh chi tiết (Ad Placement)
	if p.AdPlacement == "" || p.AdPlatform == "" {
		ref := p.Referrer
		if ref == "" {
			ref = r.Referer()
		}
		adInfo := security.ResolveAdPlacement(p.EntryURL, ref, p.UTMSource, p.UTMMedium, p.UTMCampaign, p.UTMContent)
		if p.AdPlatform == "" {
			p.AdPlatform = adInfo.Platform
		}
		if p.AdPlacement == "" {
			p.AdPlacement = adInfo.Placement
		}
	}

	// Lưu / Upsert vào PostgreSQL
	if err := h.store.UpsertAnalyticsSession(&p, clientIP, rawUA); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"session_id":   p.SessionID,
		"is_bot":       p.IsBot,
		"city":         p.City,
		"region":       p.Region,
		"isp":          p.ISP,
		"ad_platform":  p.AdPlatform,
		"ad_placement": p.AdPlacement,
	})
}

// applyMetaMacrosFromURL bổ sung các macro Meta Ads còn thiếu từ query string.
//
// Một số landing (đặc biệt là landing export từ bên thứ ba) chỉ gửi URL mà
// không tách sẵn từng tham số. Hàm này đọc trực tiếp từ URL để không bỏ sót
// thông số quảng cáo.
func applyMetaMacrosFromURL(p *models.AnalyticsEventPayload) {
	if p.EntryURL == "" {
		return
	}
	u, err := url.Parse(p.EntryURL)
	if err != nil {
		return
	}
	q := u.Query()

	pick := func(dst *string, keys ...string) {
		if *dst != "" {
			return
		}
		for _, k := range keys {
			if v := strings.TrimSpace(q.Get(k)); v != "" {
				*dst = v
				return
			}
		}
	}

	pick(&p.UTMSource, "utm_source")
	pick(&p.UTMMedium, "utm_medium")
	pick(&p.UTMCampaign, "utm_campaign")
	pick(&p.UTMContent, "utm_content")
	pick(&p.UTMTerm, "utm_term")
	pick(&p.UTMPlacement, "utm_placement", "placement")
	pick(&p.CampaignName, "campaign_name")
	pick(&p.AdsetName, "adset_name")
	pick(&p.AdName, "ad_name")
	pick(&p.AdAccountID, "ad_account_id", "acc_id", "ad_account")
	pick(&p.SiteSourceName, "site_source_name")
	pick(&p.Fbclid, "fbclid")
	pick(&p.Igshid, "igshid")
}


// parseHourRange đọc from_hour / to_hour từ query string.
// Trả về (-1, -1) nếu không có hoặc không hợp lệ.
func parseHourRange(q url.Values) (int, int) {
	fromHour := -1
	toHour := -1
	if v := strings.TrimSpace(q.Get("from_hour")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 23 {
			fromHour = n
		}
	}
	if v := strings.TrimSpace(q.Get("to_hour")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 23 {
			toHour = n
		}
	}
	return fromHour, toHour
}

// HandleOverview trả về báo cáo tổng quan
func (h *AnalyticsHandler) HandleOverview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	overview, err := h.store.GetOverviewExtended(landingID, nguon, strings.TrimSpace(q.Get("from_date")), strings.TrimSpace(q.Get("to_date")), hourFrom, hourTo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    overview,
	})
}

// HandleTimeseries trả về chuỗi thời gian (giờ / ngày / tuần)
func (h *AnalyticsHandler) HandleTimeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)
	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "day"
	}
	days := db.ParseIntSafe(q.Get("days"), 30)

	f := db.NewAnalyticsFilter(landingID, nguon, q.Get("from_date"), q.Get("to_date"), days).WithHourRange(hourFrom, hourTo)
	items, err := h.store.GetAnalyticsTimeSeries(f, granularity)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"granularity": granularity,
		"data":        items,
	})
}

// HandleDevices thống kê theo thiết bị / trình duyệt / in-app browser
func (h *AnalyticsHandler) HandleDevices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)
	dimension := q.Get("dimension")
	if dimension == "" {
		dimension = "model"
	}
	limit := db.ParseIntSafe(q.Get("limit"), 20)

	f := db.NewAnalyticsFilter(landingID, nguon, q.Get("from_date"), q.Get("to_date"), db.ParseIntSafe(q.Get("days"), 30)).WithHourRange(hourFrom, hourTo)
	items, err := h.store.GetDeviceAnalytics(f, dimension, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"dimension": dimension,
		"data":      items,
	})
}

// HandleMetaMacros thống kê theo bộ macro Meta Ads
func (h *AnalyticsHandler) HandleMetaMacros(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)
	limit := db.ParseIntSafe(q.Get("limit"), 50)

	f := db.NewAnalyticsFilter(landingID, nguon, q.Get("from_date"), q.Get("to_date"), db.ParseIntSafe(q.Get("days"), 30)).WithHourRange(hourFrom, hourTo)
	items, err := h.store.GetMetaMacroAnalytics(f, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    items,
	})
}

// HandleBotAnalytics thống kê bot chi tiết
func (h *AnalyticsHandler) HandleBotAnalytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	f := db.NewAnalyticsFilter(landingID, nguon, q.Get("from_date"), q.Get("to_date"), db.ParseIntSafe(q.Get("days"), 30)).WithHourRange(hourFrom, hourTo)
	data, err := h.store.GetBotAnalytics(f, db.ParseIntSafe(q.Get("limit"), 20))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// HandleVPNAnalytics thống kê VPN / Proxy / Tor
func (h *AnalyticsHandler) HandleVPNAnalytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	f := db.NewAnalyticsFilter(landingID, nguon, q.Get("from_date"), q.Get("to_date"), db.ParseIntSafe(q.Get("days"), 30)).WithHourRange(hourFrom, hourTo)
	data, err := h.store.GetVPNAnalytics(f)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// HandleLandingReport báo cáo TỔNG HỢP riêng cho 1 landing page.
//
// Gom đủ: tổng quan, timeseries (giờ/ngày/tuần), thiết bị, macro Meta,
// bot, VPN, địa lý, nguồn traffic, vị trí quảng cáo.
func (h *AnalyticsHandler) HandleLandingReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)
	days := db.ParseIntSafe(q.Get("days"), 30)

	report, err := h.store.GetLandingReportRange(landingID, nguon, strings.TrimSpace(q.Get("from_date")), strings.TrimSpace(q.Get("to_date")), days, hourFrom, hourTo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    report,
	})
}

// HandleLandingPerformance so sánh hiệu quả các Landing
func (h *AnalyticsHandler) HandleLandingPerformance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	items, err := h.store.GetLandingPerformance(landingID, nguon, strings.TrimSpace(q.Get("from_date")), strings.TrimSpace(q.Get("to_date")), hourFrom, hourTo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    items,
	})
}

// HandleSourcePerformance thống kê theo nguồn và content
func (h *AnalyticsHandler) HandleSourcePerformance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	items, err := h.store.GetSourcePerformance(landingID, nguon, strings.TrimSpace(q.Get("from_date")), strings.TrimSpace(q.Get("to_date")), hourFrom, hourTo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    items,
	})
}

// HandleListSessions danh sách chi tiết các phiên
func (h *AnalyticsHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	filterBot := strings.TrimSpace(q.Get("bot"))
	filterConverted := strings.TrimSpace(q.Get("converted"))
	searchPhone := strings.TrimSpace(q.Get("phone"))

	sessions, total, err := h.store.ListAnalyticsSessions(page, limit, landingID, nguon, filterBot, filterConverted, searchPhone)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    sessions,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// HandleSessionDetail chi tiết 1 phiên kèm toàn bộ Timeline hành vi
func (h *AnalyticsHandler) HandleSessionDetail(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("id"))
	if sessionID == "" {
		sessionID = strings.TrimPrefix(r.URL.Path, "/api/analytics/session/")
	}
	if sessionID == "" {
		http.Error(w, "Thiếu tham số session_id", http.StatusBadRequest)
		return
	}

	sess, err := h.store.GetSessionDetail(sessionID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    sess,
	})
}

// HandleGeoAnalytics thống kê địa lý theo Tỉnh/Thành phố, Vùng miền và Nhà mạng viễn thông
func (h *AnalyticsHandler) HandleGeoAnalytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	geoData, err := h.store.GetGeoAnalytics(landingID, nguon, strings.TrimSpace(q.Get("from_date")), strings.TrimSpace(q.Get("to_date")), hourFrom, hourTo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    geoData,
	})
}

// HandleAdPlacementAnalytics thống kê chi tiết theo Nền tảng & Vị trí quảng cáo (FB vs Insta vs AN vs Messenger, Google Search vs GDN vs YouTube...)
func (h *AnalyticsHandler) HandleAdPlacementAnalytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	hourFrom, hourTo := parseHourRange(q)

	adData, err := h.store.GetAdPlacementAnalytics(landingID, nguon, strings.TrimSpace(q.Get("from_date")), strings.TrimSpace(q.Get("to_date")), hourFrom, hourTo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    adData,
	})
}
