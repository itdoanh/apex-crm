package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"apex-backend/cache"
	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/integrations/googlesheets"
	"apex-backend/models"
	"apex-backend/security"

	"golang.org/x/time/rate"
)

type LeadHandler struct {
	cfg          *config.Config
	store        *db.Store
	landingCache *cache.LandingCache
	rateLimiter  *security.IPRateLimiter
}

func NewLeadHandler(cfg *config.Config, store *db.Store, landingCache *cache.LandingCache) *LeadHandler {
	return &LeadHandler{
		cfg:          cfg,
		store:        store,
		landingCache: landingCache,
		rateLimiter:  security.NewIPRateLimiter(rate.Limit(10.0/60.0), 10, 10*time.Minute), // 10 leads/phút, burst 10
	}
}

// sanitizeInput làm sạch chuỗi, loại bỏ các ký tự điều khiển và thẻ HTML để chống XSS/Injection
func sanitizeInput(input string, maxLen int) string {
	s := strings.TrimSpace(input)
	reHTML := regexp.MustCompile(`<[^>]*>`)
	s = reHTML.ReplaceAllString(s, "")
	var sb strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\r' || r == '\t' {
			sb.WriteRune(r)
		}
	}
	s = strings.TrimSpace(sb.String())
	if maxLen > 0 && len([]rune(s)) > maxLen {
		runes := []rune(s)
		s = string(runes[:maxLen])
	}
	return s
}

// CORSMiddleware xử lý Access-Control headers & gán Security Headers cơ bản cho toàn bộ API
func (h *LeadHandler) CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, X-Landing-ID, X-Client-Version")
		if origin != "*" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Security Headers cơ bản bắt buộc cho mọi website hiện đại (Chống Clickjacking, MIME Sniffing, XSS)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// getClientIP trích xuất IP thực của khách ghé thăm (hỗ trợ qua Cloudflare, Nginx, Caddy, Proxy)
func getClientIP(r *http.Request) string {
	// 1. Cloudflare header
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}

	// 2. X-Forwarded-For (lấy IP đầu tiên nếu có chuỗi IP)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 3. X-Real-IP
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	// 4. RemoteAddr từ socket
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// normalizePhone làm sạch số điện thoại
func normalizePhone(raw string) string {
	re := regexp.MustCompile(`[^\d+]`)
	cleaned := re.ReplaceAllString(raw, "")

	if strings.HasPrefix(cleaned, "+84") {
		cleaned = "0" + strings.TrimPrefix(cleaned, "+84")
	} else if strings.HasPrefix(cleaned, "84") && len(cleaned) >= 11 {
		cleaned = "0" + strings.TrimPrefix(cleaned, "84")
	}

	return cleaned
}

// extractFirstString tìm giá trị chuỗi đầu tiên từ danh sách keys trong map
func extractFirstString(data map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if val, exists := data[k]; exists && val != nil {
			str := strings.TrimSpace(fmt.Sprintf("%v", val))
			if str != "" && str != "<nil>" {
				return str
			}
		}
	}
	return ""
}

// HandleSubmitLead nhận toàn bộ payload từ bất kỳ Landing Page nào với 4 lớp phòng thủ tự động
func (h *LeadHandler) HandleSubmitLead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận phương thức POST", http.StatusMethodNotAllowed)
		return
	}

	// 1. LỚP BẢO VỆ 1: Rate Limiting theo IP
	clientIP := getClientIP(r)
	if !h.rateLimiter.Allow(clientIP) {
		_ = h.store.SaveSpamQuarantine(clientIP, r.UserAgent(), "unknown", "unknown", "", "rate_limit_exceeded: >10 req/min", nil)
		security.TarpitSleep(1500, 2500)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK) // Shadow ban 200 OK giả lập
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Dữ liệu lead đã được ghi nhận thành công",
			"id":      999999,
		})
		return
	}

	// 2. LỚP BẢO VỆ 2: Giới hạn kích thước Body (32KB chống Buffer Overflow & Slowloris)
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dung lượng request vượt quá giới hạn an toàn (32KB)",
		})
		return
	}

	// Parse JSON động
	var rawData map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &rawData); err != nil {
			rawData = make(map[string]interface{})
			_ = r.ParseForm()
			for k, v := range r.Form {
				if len(v) == 1 {
					rawData[k] = v[0]
				} else {
					rawData[k] = v
				}
			}
		}
	}

	if rawData == nil {
		rawData = make(map[string]interface{})
	}

	// Trích xuất metadata cơ bản & khử độc XSS / Injection (Sanitization)
	landingID := sanitizeInput(extractFirstString(rawData, "landing_id", "landingId", "id_landing", "ladipage_id", "landing"), 100)
	if landingID == "" {
		landingID = "default_landing"
	}
	nguon := sanitizeInput(extractFirstString(rawData, "nguon", "source", "traffic_source", "utm_source"), 100)
	if nguon == "" {
		nguon = "direct"
	}
	phoneRaw := sanitizeInput(extractFirstString(rawData, "so_dien_thoai", "phone", "phoneNumber", "sdt", "tel", "mobile"), 25)
	if fn, ok := rawData["fullName"]; ok && fn != nil {
		rawData["fullName"] = sanitizeInput(fmt.Sprintf("%v", fn), 100)
	}

	// 3. LỚP BẢO VỆ 3: Behavioral Honeypots (Bẫy bot ngầm)
	if isHoneypot, hpReason := security.DetectHoneypot(rawData); isHoneypot {
		_ = h.store.SaveSpamQuarantine(clientIP, r.UserAgent(), landingID, nguon, phoneRaw, hpReason, bodyBytes)
		security.TarpitSleep(1500, 2500) // Tarpit làm trễ bot
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK) // Trả về 200 OK giả để bot không nhận biết
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Dữ liệu lead đã được ghi nhận thành công",
			"id":      999999,
		})
		return
	}

	// 4. LỚP BẢO VỆ 4: Time-to-Submit Verification (Kiểm tra tốc độ gõ tay vs TỰ ĐỘNG ĐIỀN)
	// QUAN TRỌNG: Nếu khách sử dụng tính năng Tự Động Điền (Autofill từ URL/Cookie/Trình duyệt),
	// form gửi đi trong <1s là hoàn toàn bình thường và KHÔNG được chặn!
	var isAutofill bool
	if af, ok := rawData["is_autofill"]; ok && af != nil {
		switch v := af.(type) {
		case bool:
			isAutofill = v
		case string:
			isAutofill = (v == "true" || v == "1")
		case float64:
			isAutofill = (v == 1)
		}
	}
	if afSrc, ok := rawData["autofill_source"]; ok && afSrc != nil && fmt.Sprintf("%v", afSrc) != "" {
		isAutofill = true
	}

	var fillDurationMs int64
	if fd, ok := rawData["fill_duration_ms"]; ok && fd != nil {
		switch v := fd.(type) {
		case float64:
			fillDurationMs = int64(v)
		case string:
			fillDurationMs, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	if isTooFast, speedReason := security.CheckSubmissionSpeed(fillDurationMs, isAutofill); isTooFast {
		_ = h.store.SaveSpamQuarantine(clientIP, r.UserAgent(), landingID, nguon, phoneRaw, speedReason, bodyBytes)
		security.TarpitSleep(1500, 2500)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Dữ liệu lead đã được ghi nhận thành công",
			"id":      999999,
		})
		return
	}

	// 5. LỚP BẢO VỆ 5: Ephemeral Proof-of-Work Challenge (Xác thực mật mã chống cURL / Postman script)
	chID := extractFirstString(rawData, "challenge_id", "challengeId")
	chSalt := extractFirstString(rawData, "challenge_salt", "salt")
	chSig := extractFirstString(rawData, "challenge_signature", "signature")
	chSol := extractFirstString(rawData, "challenge_solution", "solution", "pow_solution")
	chDiff := security.ParseDifficulty(extractFirstString(rawData, "challenge_difficulty", "difficulty"))
	var chExpires int64
	if exp, ok := rawData["challenge_expires_at"]; ok && exp != nil {
		switch v := exp.(type) {
		case float64:
			chExpires = int64(v)
		case string:
			chExpires, _ = strconv.ParseInt(v, 10, 64)
		}
	}

	// Nếu client gửi kèm thông số challenge thì xác thực nghiêm ngặt
	if chID != "" || chSig != "" || chSol != "" {
		validPoW, powReason := security.VerifyChallenge(chID, chSalt, chDiff, chExpires, chSig, chSol)
		if !validPoW {
			_ = h.store.SaveSpamQuarantine(clientIP, r.UserAgent(), landingID, nguon, phoneRaw, "pow_invalid: "+powReason, bodyBytes)
			security.TarpitSleep(1500, 2500)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Dữ liệu lead đã được ghi nhận thành công",
				"id":      999999,
			})
			return
		}
	}

	// 6. LỚP BẢO VỆ 6: Carrier Whitelist & Phone Sanitization (Kiểm định đầu số viễn thông VN)
	isValidPhone, cleanedPhone, phoneErr := security.ValidateVietnamesePhone(phoneRaw)
	if !isValidPhone {
		_ = h.store.SaveSpamQuarantine(clientIP, r.UserAgent(), landingID, nguon, phoneRaw, "invalid_phone: "+phoneErr, bodyBytes)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   phoneErr,
		})
		return
	}
	phone := cleanedPhone
	carrier := security.GetCarrierName(phone)

	// 7. LỚP ĐỊNH VỊ ĐỊA LÝ & NHÀ MẠNG QUA IP THỰC TẾ
	geo := security.ResolveIP(clientIP)
	city := geo.City
	region := geo.Region
	country := geo.Country
	isp := geo.ISP

	// 8. LỚP PHÂN TÍCH NGUỒN QUẢNG CÁO & VỊ TRÍ PHÂN PHỐI CHI TIẾT (AD PLACEMENT)
	entryURL := extractFirstString(rawData, "entry_url", "url")
	if entryURL == "" {
		entryURL = r.Referer()
	}
	referrer := extractFirstString(rawData, "referrer")
	if referrer == "" {
		referrer = r.Referer()
	}
	utmSource := extractFirstString(rawData, "utm_source")
	if utmSource == "" {
		utmSource = nguon
	}
	utmMedium := extractFirstString(rawData, "utm_medium")
	utmCampaign := extractFirstString(rawData, "utm_campaign")
	utmContent := extractFirstString(rawData, "utm_content")

	adInfo := security.ResolveAdPlacement(entryURL, referrer, utmSource, utmMedium, utmCampaign, utmContent)

	// 9. URL TRANG KHÁCH VỪA GỬI FORM — trường quan trọng để truy ngược lead về đúng landing page.
	// tracker.js gửi kèm `url` (URL đầy đủ kèm query string UTM). Nếu thiếu thì fallback theo thứ tự:
	// entry_url → referrer → Origin header → scheme + Host của request hiện tại.
	pageURL := sanitizeInput(extractFirstString(rawData, "url", "page_url", "entry_url", "landing_url"), 1000)
	if pageURL == "" {
		pageURL = sanitizeInput(r.Referer(), 1000)
	}
	if pageURL == "" {
		if origin := sanitizeInput(r.Header.Get("Origin"), 500); origin != "" && origin != "null" {
			pageURL = origin
		}
	}
	if pageURL == "" {
		if host := r.Host; host != "" {
			scheme := "https"
			if r.TLS == nil {
				scheme = "http"
			}
			if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
				scheme = forwardedProto
			}
			pageURL = scheme + "://" + host + r.URL.Path
		}
	}

	// Bổ sung các thông tin máy chủ (Server Enrichment) vào tracking JSON
	rawData["server_ip"] = clientIP
	rawData["server_user_agent"] = r.UserAgent()
	rawData["server_referer"] = r.Referer()
	rawData["server_received_at"] = time.Now().Format("2006-01-02 15:04:05")
	rawData["landing_id"] = landingID
	rawData["nguon"] = nguon
	rawData["carrier"] = carrier
	rawData["city"] = city
	rawData["region"] = region
	rawData["country"] = country
	rawData["isp"] = isp
	rawData["ad_platform"] = adInfo.Platform
	rawData["ad_placement"] = adInfo.Placement
	rawData["ad_source_tag"] = adInfo.SourceTag
	rawData["url"] = pageURL

	trackingBytes, err := json.Marshal(rawData)
	if err != nil {
		trackingBytes = []byte("{}")
	}

	// Lưu vào CSDL PostgreSQL với kiểm tra deduplication 24h tự động
	lead, err := h.store.SaveLead(landingID, nguon, phone, pageURL, trackingBytes, city, region, country, isp, adInfo.Platform, adInfo.Placement)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Lỗi lưu CSDL PostgreSQL: " + err.Error(),
		})
		return
	}

	// Tự động liên kết Lead vào Session Analytics tương ứng
	sessionID := extractFirstString(rawData, "session_id", "sessionId", "apex_session_id")
	if sessionID != "" {
		if err := h.store.AssociateLeadToSession(sessionID, phone, lead.ID); err != nil {
			fmt.Printf("[WARN] Không thể liên kết lead %d với session %s: %v\n", lead.ID, sessionID, err)
		}
	}

	// Tự động đồng bộ thời gian thực sang Google Sheets (chạy ngầm trong goroutine để không làm trễ response)
	go h.syncLeadToGoogleSheets(lead, rawData)

	// Phản hồi thành công
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"message":       "Dữ liệu lead đã được ghi nhận thành công",
		"id":            lead.ID,
		"thoi_gian":     lead.CreatedAt,
		"landing_id":    lead.LandingID,
		"nguon":         lead.Nguon,
		"so_dien_thoai": lead.Phone,
		"url":           lead.URL,
		"carrier":       carrier,
		"city":          city,
		"region":        region,
		"country":       country,
		"isp":           isp,
		"ad_platform":   adInfo.Platform,
		"ad_placement":  adInfo.Placement,
	})
}

// HandleListLeads API trả về danh sách leads có phân trang & tìm kiếm đa chiều (SĐT, Tên, Ghi chú, Landing, Nguồn, Trạng thái, Ngày, Giờ)
func (h *LeadHandler) HandleListLeads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 500 {
		limit = 50
	}
	search := strings.TrimSpace(q.Get("search"))
	if search == "" {
		search = strings.TrimSpace(q.Get("phone"))
	}
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	status := strings.TrimSpace(q.Get("status"))
	fromDate := strings.TrimSpace(q.Get("from_date"))
	toDate := strings.TrimSpace(q.Get("to_date"))
	fromHour, _ := strconv.Atoi(q.Get("from_hour"))
	if q.Get("from_hour") == "" {
		fromHour = -1
	}
	toHour, _ := strconv.Atoi(q.Get("to_hour"))
	if q.Get("to_hour") == "" {
		toHour = -1
	}

	leads, total, err := h.store.ListLeads(page, limit, search, landingID, nguon, status, fromDate, toDate, fromHour, toHour)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"data":    leads,
	})
}

// HandleUpdateLead cập nhật trạng thái CRM (Pipeline) và ghi chú tư vấn của khách hàng
func (h *LeadHandler) HandleUpdateLead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, "Chỉ chấp nhận POST/PUT/PATCH", http.StatusMethodNotAllowed)
		return
	}

	var req models.UpdateLeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Thiếu id lead hoặc payload không hợp lệ"})
		return
	}

	if err := h.store.UpdateLead(req.ID, req.TrangThai, req.GhiChu, req.AssignedTo); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Ghi nhận Audit Log hoạt động quản trị
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)
	var adminID *int
	var username string
	if admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser); ok && admin != nil {
		adminID = &admin.ID
		username = admin.Username
	}
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     "UPDATE_LEAD_STATUS",
		Resource:   "leads",
		ResourceID: strconv.FormatUint(req.ID, 10),
		Status:     "success",
		Details:    map[string]interface{}{"lead_id": req.ID, "trang_thai": req.TrangThai, "ghi_chu": req.GhiChu, "assigned_to": req.AssignedTo},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Cập nhật trạng thái CRM thành công",
		"id":      req.ID,
	})
}

// HandleCRMStats cung cấp toàn bộ dữ liệu thống kê CRM: 24h Hourly breakdown, Daily trends, Status counts, Carrier & Source breakdowns
func (h *LeadHandler) HandleCRMStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("search"))
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	status := strings.TrimSpace(q.Get("status"))
	fromDate := strings.TrimSpace(q.Get("from_date"))
	toDate := strings.TrimSpace(q.Get("to_date"))
	fromHour, _ := strconv.Atoi(q.Get("from_hour"))
	if q.Get("from_hour") == "" {
		fromHour = -1
	}
	toHour, _ := strconv.Atoi(q.Get("to_hour"))
	if q.Get("to_hour") == "" {
		toHour = -1
	}

	stats, err := h.store.GetCRMStats(search, landingID, nguon, status, fromDate, toDate, fromHour, toHour)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	landingIDs, sources, _ := h.store.GetFilterOptions()
	stats["landing_ids"] = landingIDs
	stats["sources"] = sources

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

// HandleExportCSV xuất dữ liệu ra file CSV chuẩn UTF-8 có BOM (mở Excel tiếng Việt không lỗi font) theo đúng bộ lọc
func (h *LeadHandler) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("search"))
	landingID := strings.TrimSpace(q.Get("landing_id"))
	nguon := strings.TrimSpace(q.Get("nguon"))
	status := strings.TrimSpace(q.Get("status"))
	fromDate := strings.TrimSpace(q.Get("from_date"))
	toDate := strings.TrimSpace(q.Get("to_date"))
	fromHour, _ := strconv.Atoi(q.Get("from_hour"))
	if q.Get("from_hour") == "" {
		fromHour = -1
	}
	toHour, _ := strconv.Atoi(q.Get("to_hour"))
	if q.Get("to_hour") == "" {
		toHour = -1
	}

	leads, err := h.store.GetAllLeads(search, landingID, nguon, status, fromDate, toDate, fromHour, toHour)
	if err != nil {
		http.Error(w, "Lỗi lấy dữ liệu export: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ghi nhận Audit Log hoạt động xuất dữ liệu Lead
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)
	var adminID *int
	var username string
	if admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser); ok && admin != nil {
		adminID = &admin.ID
		username = admin.Username
	}
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     "EXPORT_LEADS_EXCEL",
		Resource:   "leads",
		Status:     "success",
		Details:    map[string]interface{}{"total_exported": len(leads), "landing_id": landingID, "nguon": nguon, "status": status},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	filename := fmt.Sprintf("crm_leads_apex_%s.csv", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Ghi UTF-8 BOM để Excel tự nhận diện font tiếng Việt có dấu
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header đầy đủ các trường thông tin CRM
	_ = writer.Write([]string{
		"ID",
		"Thời Gian",
		"Trạng Thái CRM",
		"Họ Tên",
		"Số Điện Thoại",
		"Nhà Mạng",
		"Landing ID",
		"URL Trang Đăng Ký",
		"Nguồn Traffic",
		"Ghi Chú Sales",
		"Người Phụ Trách",
		"Cập Nhật Lúc",
		"Thiết Bị",
		"Tự Động Điền",
		"UTM Campaign",
		"Tracking Raw",
	})

	for _, lead := range leads {
		var trMap map[string]interface{}
		_ = json.Unmarshal(lead.Tracking, &trMap)
		if trMap == nil {
			trMap = make(map[string]interface{})
		}

		fullName := fmt.Sprintf("%v", trMap["fullName"])
		if fullName == "<nil>" {
			fullName = ""
		}
		carrier := fmt.Sprintf("%v", trMap["carrier"])
		if carrier == "<nil>" {
			carrier = ""
		}
		dev := fmt.Sprintf("%v", trMap["device_type"])
		if dev == "<nil>" {
			dev = ""
		}
		utmCamp := fmt.Sprintf("%v", trMap["utm_campaign"])
		if utmCamp == "<nil>" {
			utmCamp = ""
		}
		// URL trang khách gửi form: ưu tiên cột `url` trong DB, fallback về tracking JSON
		pageURL := lead.URL
		if pageURL == "" {
			pageURL = fmt.Sprintf("%v", trMap["url"])
			if pageURL == "<nil>" {
				pageURL = ""
			}
		}
		isAf := "Gõ tay"
		if trMap["is_autofill"] == true || fmt.Sprintf("%v", trMap["is_autofill"]) == "1" || fmt.Sprintf("%v", trMap["autofill_source"]) != "" {
			isAf = "Tự động điền (" + fmt.Sprintf("%v", trMap["autofill_source"]) + ")"
		}

		var compactJSON bytes.Buffer
		_ = json.Compact(&compactJSON, lead.Tracking)

		_ = writer.Write([]string{
			strconv.FormatUint(lead.ID, 10),
			lead.CreatedAt,
			lead.TrangThai,
			fullName,
			lead.Phone,
			carrier,
			lead.LandingID,
			pageURL,
			lead.Nguon,
			lead.GhiChu,
			lead.AssignedTo,
			lead.UpdatedAt,
			dev,
			isAf,
			utmCamp,
			compactJSON.String(),
		})
	}
}

// HandleStats trả về thống kê nhanh và danh sách bộ lọc landing_id & nguon
func (h *LeadHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	landingID := q.Get("landing_id")
	nguon := q.Get("nguon")

	total, today, err := h.store.CountStats(landingID, nguon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	landingIDs, sources, _ := h.store.GetFilterOptions()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"total":       total,
		"today":       today,
		"landing_ids": landingIDs,
		"sources":     sources,
	})
}

// HandleGetChallenge phát hành thử thách Proof-of-Work ngắn hạn cho Front-End
func (h *LeadHandler) HandleGetChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận GET hoặc POST", http.StatusMethodNotAllowed)
		return
	}

	diff := 3 // Độ khó mặc định SHA-256 (3 số 0, máy khách mất ~15-30ms)
	if dStr := r.URL.Query().Get("difficulty"); dStr != "" {
		diff = security.ParseDifficulty(dStr)
	}

	challenge, err := security.GenerateChallenge(diff)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    challenge,
	})
}

// HandleGetSecurityStats trả về báo cáo số lượng cuộc tấn công & spam bị chặn
func (h *LeadHandler) HandleGetSecurityStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.store.GetSecurityStats()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

// HandleListQuarantine trả về danh sách nhật ký spam bị cách ly
func (h *LeadHandler) HandleListQuarantine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	items, total, err := h.store.ListSpamQuarantine(limit, offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"total":   total,
		"data":    items,
	})
}

// syncLeadToGoogleSheets tự động đồng bộ một lead mới sang các Google Sheet được phân quyền riêng cho Landing Page
func (h *LeadHandler) syncLeadToGoogleSheets(lead *models.Lead, rawData map[string]interface{}) {
	if lead == nil {
		return
	}

	var targetSheetIDs []int
	var syncMode string // "selected", "all", "disabled"

	// 1. Kiểm tra cấu hình riêng của landing page này (từ RAM cache siêu tốc hoặc DB)
	if h.landingCache != nil {
		lp := h.landingCache.Get("", lead.LandingID)
		if lp == nil {
			lp = h.landingCache.GetByPageID(lead.LandingID)
		}
		if lp == nil {
			lp, _ = h.store.GetLandingPage("", lead.LandingID)
			if lp == nil {
				lp, _ = h.store.GetLandingPageByID(lead.LandingID)
			}
		}

		if lp != nil && lp.Settings != nil {
			if mode, ok := lp.Settings["google_sheet_sync_mode"].(string); ok {
				syncMode = strings.TrimSpace(mode)
			}
			// Nếu landing page chọn "disabled" -> Dừng ngay, không gửi lead sang bất kỳ sheet nào
			if syncMode == "disabled" {
				fmt.Printf("[GOOGLE_SHEET_SYNC_INFO] Landing '%s' đã tắt đồng bộ Google Sheet.\n", lead.LandingID)
				return
			}

			// Đọc danh sách sheet IDs riêng được gán cho Landing Page này
			if rawIDs, ok := lp.Settings["google_sheet_ids"].([]interface{}); ok {
				for _, idVal := range rawIDs {
					switch v := idVal.(type) {
					case float64:
						targetSheetIDs = append(targetSheetIDs, int(v))
					case int:
						targetSheetIDs = append(targetSheetIDs, v)
					case string:
						if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
							targetSheetIDs = append(targetSheetIDs, n)
						}
					}
				}
			}
		}
	}

	configs, err := h.store.GetActiveGoogleSheetConfigsForLead(lead.LandingID, lead.Nguon, targetSheetIDs, syncMode)
	if err != nil || len(configs) == 0 {
		return
	}

	leadName := fmt.Sprintf("%v", googlesheets.ExtractLeadValue("fullName", "", lead, rawData))
	if leadName == "<nil>" {
		leadName = ""
	}

	for _, cfg := range configs {
		go func(c models.GoogleSheetConfig) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			// Nếu cấu hình bật AutoCreateHeader: kiểm tra nếu dòng 1 còn trống thì tạo header trước
			if c.AutoCreateHeader && len(c.FieldMappings) > 0 {
				headers, hErr := googlesheets.GetSheetHeaders(ctx, c.CredentialsJSON, c.SpreadsheetID, c.SheetTabName)
				if hErr == nil && len(headers) == 0 {
					headerCols := make([]string, len(c.FieldMappings))
					for idx, m := range c.FieldMappings {
						headerCols[idx] = m.Header
					}
					_ = googlesheets.WriteHeaders(ctx, c.CredentialsJSON, c.SpreadsheetID, c.SheetTabName, headerCols)
				}
			}

			// Xây dựng mảng giá trị của dòng theo cấu hình ánh xạ cột
			rowValues := googlesheets.BuildRowValues(c.FieldMappings, lead, rawData)

			// Gửi dòng sang Google Sheets
			rowRange, err := googlesheets.AppendRow(ctx, c.CredentialsJSON, c.SpreadsheetID, c.SheetTabName, rowValues)

			syncLog := &models.GoogleSheetSyncLog{
				ConfigID:  c.ID,
				LeadID:    lead.ID,
				LeadPhone: lead.Phone,
				LeadName:  leadName,
			}

			if err != nil {
				syncLog.Status = "failed"
				syncLog.ErrorMessage = err.Error()
				fmt.Printf("[GOOGLE_SHEET_SYNC_ERROR] Config '%s' (ID=%d): %v\n", c.Name, c.ID, err)
			} else {
				syncLog.Status = "success"
				syncLog.RowRange = rowRange
				_ = h.store.UpdateConfigLastSynced(c.ID)
				fmt.Printf("[GOOGLE_SHEET_SYNC_SUCCESS] Config '%s' (ID=%d): đã thêm dòng %s\n", c.Name, c.ID, rowRange)
			}

			_ = h.store.CreateSyncLog(syncLog)
		}(cfg)
	}
}

