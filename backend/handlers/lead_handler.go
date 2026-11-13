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
	"net/url"
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

// buildChallengeFingerprint tạo định danh phiên đăng ký từ payload + SĐT + IP.
//
// Mục đích: server phân biệt được 2 tình huống khi nonce PoW đã dùng:
//   - CÙNG fingerprint ⇒ client gửi trùng payload (nhiều handler submit chạy
//     song song / retry mạng) ⇒ bỏ qua, KHÔNG ghi vào vùng cách ly.
//   - KHÁC fingerprint ⇒ bot chơi lại payload ⇒ ghi vào vùng cách ly.
//
// Ràng buộc: KHÔNG dùng thông tin nhạy cảm thô (tên, email). Chỉ dùng
// session_id + visitor_id + SĐT đã chuẩn hóa + IP.
func buildChallengeFingerprint(rawData map[string]interface{}, phoneRaw, clientIP string) string {
	sessionID := extractFirstString(rawData, "session_id", "sessionId", "apex_session_id")
	visitorID := extractFirstString(rawData, "visitor_id", "visitorId")
	phoneDigits := normalizePhone(phoneRaw)

	parts := []string{
		strings.TrimSpace(sessionID),
		strings.TrimSpace(visitorID),
		strings.TrimSpace(phoneDigits),
		strings.TrimSpace(clientIP),
	}
	return strings.Join(parts, "|")
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

// extractHostFromURL lấy hostname (không kèm port, không kèm "www.") từ 1 URL.
//
// Dùng để tra cấu hình Google Sheets theo ĐÚNG domain mà khách vừa gửi form
// (fix 2026-11-13): 1 landing có thể chạy trên nhiều domain, mỗi domain có thể
// gán bảng tính riêng.
func extractHostFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	// Thêm scheme nếu thiếu để url.Parse xử lý đúng
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	return host
}

// enrichLeadTracking bổ sung thông số Meta Ads, thiết bị & VPN vào tracking JSON
// của lead — những phần client KHÔNG gửi được hoặc gửi thiếu.
//
// BỔ SUNG (2026-11-12):
//   1. Thiết bị: phân tích User-Agent phía server (fallback khi client không
//      gửi) — tên thương mại (vd: "Samsung Galaxy A52"), trình duyệt trong app.
//   2. VPN / Proxy: dựa trên IP + header proxy.
//   3. Meta CAPI: suy ra `_fbc` từ `fbclid` nếu thiếu (để gửi CAPI khớp
//      chính xác lượt nhấp), chuẩn hoá `site_source_name` (fb/ig/msg/an).
//
// Giá trị client gửi lên LUÔN được ưu tiên (không ghi đè).
func enrichLeadTracking(rawData map[string]interface{}, r *http.Request, clientIP string, pageURL string) {
	if rawData == nil {
		return
	}

	setIfEmpty := func(key string, value string) {
		if value == "" {
			return
		}
		if cur, ok := rawData[key]; ok && cur != nil {
			if s := strings.TrimSpace(fmt.Sprintf("%v", cur)); s != "" {
				return
			}
		}
		rawData[key] = value
	}

	// ─── 1. Thiết bị (phân tích từ User-Agent) ───
	ua := r.UserAgent()
	if ua != "" {
		devInfo := security.ParseDeviceInfo(ua)
		setIfEmpty("device_type", devInfo.DeviceType)
		setIfEmpty("browser", devInfo.Browser)
		setIfEmpty("browser_version", devInfo.BrowserVersion)
		setIfEmpty("browser_engine", devInfo.Engine)
		setIfEmpty("os", devInfo.OS)
		setIfEmpty("os_version", devInfo.OSVersion)
		setIfEmpty("device_brand", devInfo.Brand)
		setIfEmpty("device_model", devInfo.Model)
		setIfEmpty("device_model_code", devInfo.ModelCode)
		setIfEmpty("device_name", devInfo.Model)
		setIfEmpty("platform_info", devInfo.Platform)
		if devInfo.IsInAppBrowser {
			if cur, ok := rawData["is_in_app_browser"]; !ok || cur == nil || fmt.Sprintf("%v", cur) == "false" {
				rawData["is_in_app_browser"] = true
			}
			setIfEmpty("in_app_type", devInfo.InAppType)
		}
	}

	// ─── 2. VPN / Proxy ───
	if _, ok := rawData["is_vpn"]; !ok {
		headers := map[string]string{}
		for _, key := range []string{
			"X-Forwarded-For", "X-Real-IP", "Via", "X-Proxy-Id",
			"Proxy-Connection", "X-Forwarded-Proto", "CF-Connecting-IP",
		} {
			if v := r.Header.Get(key); v != "" {
				headers[strings.ToLower(key)] = v
			}
		}
		if vpn := security.ResolveVPN(clientIP, headers); vpn != nil {
			if vpn.IsVPN {
				rawData["is_vpn"] = true
			}
			if vpn.IsProxy {
				rawData["is_proxy"] = true
			}
			if vpn.IsTor {
				rawData["is_tor"] = true
			}
			setIfEmpty("vpn_provider", vpn.Provider)
			if vpn.ConfidencePct > 0 {
				rawData["vpn_confidence"] = vpn.ConfidencePct
			}
			setIfEmpty("vpn_reason", vpn.Reason)
		}
	}

	// ─── 3. Meta CAPI ───
	entryURL := pageURL
	if entryURL == "" {
		entryURL = extractFirstString(rawData, "entry_url")
	}
	cookies := map[string]string{}
	if c, err := r.Cookie("fbc"); err == nil {
		cookies["_fbc"] = c.Value
	}
	if c, err := r.Cookie("fbp"); err == nil {
		cookies["_fbp"] = c.Value
	}
	ids := security.ResolveMetaClickIDs(entryURL, cookies)
	setIfEmpty("fbclid", ids.Fbclid)
	setIfEmpty("fbc", ids.Fbc)
	setIfEmpty("fbp", ids.Fbp)
	setIfEmpty("igshid", ids.Igshid)
	if extractFirstString(rawData, "meta_click_id") == "" && ids.Fbclid != "" {
		rawData["meta_click_id"] = ids.Fbclid
	}

	// Chuẩn hoá `site_source_name` (chỉ 4 giá trị hợp lệ: fb / ig / msg / an)
	if extractFirstString(rawData, "site_source_name") == "" {
		src := strings.ToLower(extractFirstString(rawData, "utm_source"))
		switch src {
		case "facebook", "fb":
			rawData["site_source_name"] = "fb"
		case "instagram", "ig":
			rawData["site_source_name"] = "ig"
		case "messenger", "msg":
			rawData["site_source_name"] = "msg"
		case "audience network", "an":
			rawData["site_source_name"] = "an"
		}
	}
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
	if nguon == "" || nguon == "landing_chinh" {
		// ⚠️ FIX 2026-11-13: nguồn mặc định = landing_id.
		//
		// VÌ SAO: client cũ gửi `nguon` = `landing_chinh` cho MỌI landing ⇒ 4
		// landing khác nhau cùng ghi 1 nguồn ⇒ không lọc tách được, số liệu chồng
		// chéo. Mỗi landing có nguồn riêng (tên landing) ⇒ lọc đúng & khớp CRM.
		nguon = landingID
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
		powRes := security.VerifyChallenge(chID, chSalt, chDiff, chExpires, chSig, chSol,
			buildChallengeFingerprint(rawData, phoneRaw, clientIP))

		// ⚠️ TRƯỜNG HỢP GỬI TRÙNG DO CLIENT (sửa bug 2026-11-05):
		// Khách thật bấm "Đăng ký" 1 lần nhưng client có thể gửi CÙNG payload
		// nhiều lần trong vài trăm ms (nhiều handler submit chạy song song,
		// retry mạng, keepalive + sendBeacon). Request thứ 2+ dùng lại nonce đã
		// tiêu thụ → bị coi là replay → ghi vào spam_quarantine → LEAD THẬT bị
		// hiện sai là "Lỗi Proof-of-Work" trong tab Bảo mật & Bot.
		//
		// Cách xử lý đúng: request trùng lặp trong cùng phiên đăng ký ⇒ coi như
		// idempotent, trả về 200 OK (giống hệt response thành công) và KHÔNG ghi
		// vào vùng cách ly. Lead đã được lưu bởi request gốc.
		if !powRes.OK && powRes.Code == security.PowCodeDuplicateSubmit {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Dữ liệu lead đã được ghi nhận thành công",
				"id":      999999,
			})
			return
		}

		if !powRes.OK {
			_ = h.store.SaveSpamQuarantine(clientIP, r.UserAgent(), landingID, nguon, phoneRaw, "pow_invalid: "+powRes.Reason, bodyBytes)
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

		// Đánh dấu challenge đang xử lý để các request trùng lặp song song
		// từ cùng phiên đăng ký được nhận diện là duplicate (không phải spam).
		security.MarkChallengeActive(chID)
		defer security.ReleaseChallengeActive(chID)
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

	// ─── LÀM GIÀU THÔNG SỐ META ADS + THIẾT BỊ (2026-11-12) ───────
	//
	// Client (tracker.js / apex-adapter.js) đã gửi đầy đủ macro Meta Ads,
	// định danh lượt nhấp (fbclid/_fbc/_fbp) và thông tin thiết bị chi tiết.
	// Phần này bổ sung những gì client KHÔNG gửi được:
	//   - Phân tích thiết bị từ User-Agent phía server (fallback).
	//   - Phát hiện VPN / Proxy từ IP + header.
	//   - Suy ra `_fbc` từ `fbclid` nếu thiếu (để CAPI khớp lượt nhấp).
	enrichLeadTracking(rawData, r, clientIP, pageURL)

	trackingBytes, err := json.Marshal(rawData)
	if err != nil {
		trackingBytes = []byte("{}")
	}

	// Lưu vào CSDL PostgreSQL với kiểm tra deduplication 24h tự động
	lead, isNew, err := h.store.SaveLead(landingID, nguon, phone, pageURL, trackingBytes, city, region, country, isp, adInfo.Platform, adInfo.Placement)
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

	// Tự động đồng bộ thời gian thực sang Google Sheets (chạy ngầm trong goroutine để không làm trễ response).
	//
	// ⚠️ CHỈ đồng bộ khi lead MỚI (isNew = true):
	// Khi khách bấm "Đăng ký" 1 lần nhưng client gửi nhiều request (double-click,
	// nhiều handler submit chạy song song, retry mạng), các request sau bị
	// dedup trong PostgreSQL và trả về lại lead cũ (isNew = false).
	// Nếu đồng bộ mọi lần ⇒ Google Sheets nhận N dòng giống nhau cho 1 lead ⇒
	// báo cáo bán hàng thấy "mấy cái lead giống nhau cùng thời gian".
	if isNew {
		go h.syncLeadToGoogleSheets(lead, rawData)
	} else {
		fmt.Printf("[INFO] Lead #%d (SĐT %s) trùng lặp — bỏ qua đồng bộ Google Sheets\n", lead.ID, phone)
	}

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
	syncMode := "" // "selected", "all", "disabled"
	hasSheetConfig := false

	// 1. Lấy domain mà khách vừa gửi form (từ URL trang) để tra cấu hình
	//    Google Sheets ĐÚNG theo landing đang chạy trên domain đó.
	leadDomain := extractHostFromURL(lead.URL)
	if leadDomain == "" {
		leadDomain = extractHostFromURL(extractFirstString(rawData, "url", "page_url", "entry_url", "landing_url"))
	}

	// 2. Đọc cấu hình riêng của landing page này.
	//
	// ⚠️ BẤT BUỘC gộp từ MỌI row của landing (fix 2026-11-13 — lead bắn tràn
	//    sang nhiều sheet):
	//    `landing_pages` có PK composite `(domain, page_id)` ⇒ 1 landing tồn tại
	//    ở NHIỀU row: bản GỐC (`domain=''`, template) + bản THEO DOMAIN.
	//    `google_sheet_ids` / `google_sheet_sync_mode` thường CHỈ nằm ở bản theo
	//    domain. Đọc bản gốc ⇒ không có sheet IDs ⇒ rơi vào nhánh "đồng bộ mặc
	//    định" ⇒ khớp MỌI sheet active ⇒ lead của landing này bị gửi sang
	//    sheet của landing khác (đúng triệu chứng anh itdoanh báo).
	if h.landingCache != nil {
		syncMode, targetSheetIDs, hasSheetConfig = h.landingCache.GetSheetSyncSettings(leadDomain, lead.LandingID)
	}

	// 3. Fallback khi RAM cache chưa có landing này (landing vừa tạo, cache miss).
	//    Gộp MỌI row của landing (bản gốc + bản theo domain) để không bỏ sót
	//    `google_sheet_ids` chỉ nằm ở bản domain.
	if !hasSheetConfig {
		if mode, ids, found := h.store.GetLandingSheetSyncSettings(leadDomain, lead.LandingID); found {
			syncMode = mode
			targetSheetIDs = ids
			hasSheetConfig = mode != "" || ids != nil
		}
	}

	// 4. Nếu landing page chọn "disabled" -> Dừng ngay, không gửi lead sang bất kỳ sheet nào
	if syncMode == "disabled" {
		fmt.Printf("[GOOGLE_SHEET_SYNC_INFO] Landing '%s' đã tắt đồng bộ Google Sheet.\n", lead.LandingID)
		return
	}

	// 5. Landing đã cấu hình "chỉ các sheet đã chọn" nhưng danh sách rỗng
	//    -> KHÔNG gửi bừa bãi vào mọi sheet.
	if hasSheetConfig && syncMode != "all" && len(targetSheetIDs) == 0 {
		fmt.Printf("[GOOGLE_SHEET_SYNC_INFO] Landing '%s' không có Google Sheet nào được gán -> bỏ qua đồng bộ.\n", lead.LandingID)
		return
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
