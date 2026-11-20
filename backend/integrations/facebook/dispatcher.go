package facebook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"apex-backend/db"
	"apex-backend/models"
)

// Dispatcher là bộ điều phối gửi sự kiện Conversions API từ server.
//
// NGUYÊN TẮC:
//   - KHÔNG BAO GIỜ chặn luồng chính của hệ thống (submit lead / update lead).
//     Mọi lỗi CAPI chỉ được log, không được làm fail request chính.
//   - Mỗi sự kiện gửi đi đều được ghi log vào `facebook_capi_logs` để đối soát.
type Dispatcher struct {
	store  *db.Store
	client *Client
	// fallback là cấu hình từ biến môi trường (.env) — dùng khi DB chưa có
	// cấu hình nào (landing_id cụ thể lẫn cấu hình mặc định).
	fallback *models.FacebookCAPIConfig
	// disabled = true ⇒ KHÔNG gửi sự kiện nào lên Meta.
	//
	// Dùng cho môi trường `test`: lead thử nghiệm không được làm nhiễu chỉ số
	// Meta Ads thật (PageView / Lead / Purchase đều tính vào ngân sách quảng cáo).
	disabled bool
	// blockedLandings là danh sách landing_id BỊ CẤM CAPI ở tầng server.
	//
	// BỐI CẢNH (fix 2026-11-18 — landing_cu):
	//   `GetFacebookCAPIConfig(landingID)` FALLBACK sang dòng cấu hình MẶC ĐỊNH
	//   (`landing_id = ''`) khi landing chưa có dòng riêng ⇒ chỉ cần admin tạo
	//   1 cấu hình CAPI mặc định là mọi landing (kể cả landing KHÔNG dùng
	//   CAPI) bắt đầu bắn sự kiện lên Meta ⇒ chiến dịch báo lỗi dữ liệu.
	//
	//   ⇒ Danh sách này là chốt chặn TUYỆT ĐỐI ở tầng server, không phụ thuộc
	//   cấu hình trong DB ⇒ không thể bị "vô hiệu hoá" do xoá/sửa cấu hình.
	blockedLandings map[string]bool
	// capiStatusCache cache trạng thái "landing này có bắn CAPI không" (TTL ngắn).
	//
	// ⚠️ `IsCAPIActiveForLanding` được gọi MỖI lần render landing ⇒ không
	//   được query DB mỗi lượt xem. TTL 30s ⇒ thay đổi cấu hình trong CRM áp
	//   dụng sau ≤ 30s (chấp nhận được; Admin có thể restart nếu cần tức thì).
	capiStatusMu    sync.RWMutex
	capiStatusCache map[string]capiStatusEntry
}

// capiStatusEntry là 1 mục cache trạng thái CAPI của landing.
type capiStatusEntry struct {
	active bool
	at     time.Time
}

// capiStatusCacheTTL là thời hạn cache trạng thái CAPI (30 giây).
const capiStatusCacheTTL = 30 * time.Second

// NewDispatcher tạo dispatcher mới.
func NewDispatcher(store *db.Store) *Dispatcher {
	return &Dispatcher{
		store:           store,
		client:          NewClient(),
		fallback:        DefaultConfigFromEnv(),
		blockedLandings: loadBlockedLandings(),
	}
}

// serverCAPIDenyList là danh sách landing KHÔNG BAO GIỜ được bắn CAPI từ server.
//
// BỐI CẢNH (fix 2026-11-18 — landing_cu):
//   `GetFacebookCAPIConfig(landingID)` FALLBACK sang dòng cấu hình MẶC ĐỊNH
//   (`landing_id = ''`) khi landing chưa có dòng riêng ⇒ chỉ cần admin tạo
//   1 cấu hình CAPI mặc định là mọi landing (kể cả landing KHÔNG dùng CAPI)
//   bắt đầu bắn sự kiện lên Meta ⇒ chiến dịch Meta báo lỗi dữ liệu.
//
//   ⇒ Danh sách này là chốt chặn TUYỆT ĐỐI ở tầng server, KHÔNG phụ thuộc
//   cấu hình trong DB ⇒ không thể bị vô hiệu hoá do xoá/sửa cấu hình.
//
//   - `landing_cu`: landing chỉ dùng Meta Pixel nhúng trong HTML, KHÔNG dùng
//     CAPI server (yêu cầu chủ repo 2026-11-18).
//   - `landing_lznoew`: trang "Chúc mừng" (thank-you page) — PageView ở đây
//     KHÔNG phải lượt xem quảng cáo, KHÔNG được bắn CAPI (tránh nhân đôi
//     PageView và nhiễu chỉ số Landing Page View). Yêu cầu 2026-11-18: fix
//     dedup chỉ áp dụng cho các landing còn lại.
//
// ⚠️ Biến môi trường `META_CAPI_BLOCKED_LANDINGS` BỔ SUNG thêm landing vào
// danh sách (ngăn cách bằng dấu phẩy) mà không cần sửa + build lại binary.
var serverCAPIDenyList = []string{
	"landing_cu",
	"landing_lznoew",
}

// loadBlockedLandings dựng tập landing_id bị cấm CAPI ở tầng server.
//
// Nguồn cấu hình:
//  1. Danh sách mặc định trong code (`serverCAPIDenyList`).
//  2. Biến môi trường `META_CAPI_BLOCKED_LANDINGS` (bổ sung, ngăn cách dấu phẩy).
//
// ⚠️ Ngoài ra, mọi dòng `facebook_capi_configs` có `landing_id <> ''` VÀ
// `is_active = FALSE` cũng bị coi là TẮT — cơ chế này đã có sẵn trong CRM
// (bấm "tắt CAPI" theo từng landing) và được `resolveConfig` xử lý.
func loadBlockedLandings() map[string]bool {
	out := make(map[string]bool)

	for _, id := range serverCAPIDenyList {
		id = strings.TrimSpace(id)
		if id != "" {
			out[id] = true
		}
	}

	for _, part := range strings.Split(os.Getenv("META_CAPI_BLOCKED_LANDINGS"), ",") {
		id := strings.TrimSpace(part)
		if id != "" {
			out[id] = true
		}
	}

	return out
}

// IsLandingBlocked trả về true nếu landing bị cấm CAPI ở tầng server.
func (d *Dispatcher) IsLandingBlocked(landingID string) bool {
	if d == nil {
		return false
	}
	id := strings.TrimSpace(landingID)
	if id == "" {
		return false
	}
	return d.blockedLandings[id]
}

// IsCAPIActiveForLanding trả về true nếu landing SẼ bắn sự kiện CAPI lên Meta.
//
// Dùng bởi `backend/handlers/landing_handler.go` để quyết định có cần gắn
// `eventID` vào lệnh `fbq('track','PageView')` hay không.
//
// ⚠️ LÝ DO (fix 2026-11-18 — landing_cu):
//   `eventID` chỉ có ý nghĩa khi server CAPI bắn sự kiện CÙNG `event_id`
//   (Meta chỉ gộp browser ↔ server khi trùng khớp). Landing KHÔNG bật CAPI
//   (vd `landing_cu` — chỉ dùng Meta Pixel nhúng) thì không cần `eventID`
//   ⇒ giữ nguyên block Meta Pixel GỐC trong HTML, đúng như thiết kế ban đầu,
//   không bị backend sửa đổi.
//
// ⚠️ Hàm này KHÔNG gửi request ra ngoài — chỉ đọc cấu hình trong DB.
// ⚠️ Có CACHE TTL ngắn (30s) vì hàm được gọi MỖI lần render landing ⇒
//    không được query DB mỗi lượt xem.
func (d *Dispatcher) IsCAPIActiveForLanding(landingID string) bool {
	if d == nil {
		return false
	}

	// Chặn tầng server ⇒ KHÔNG cần đọc DB.
	if d.IsLandingBlocked(landingID) {
		return false
	}

	landingID = strings.TrimSpace(landingID)

	d.capiStatusMu.RLock()
	entry, ok := d.capiStatusCache[landingID]
	d.capiStatusMu.RUnlock()
	if ok && time.Since(entry.at) < capiStatusCacheTTL {
		return entry.active
	}

	active := false
	cfg, err := d.resolveConfig(landingID)
	if err == nil && cfg != nil {
		active = cfg.IsActive
	}

	d.capiStatusMu.Lock()
	if d.capiStatusCache == nil {
		d.capiStatusCache = make(map[string]capiStatusEntry)
	}
	d.capiStatusCache[landingID] = capiStatusEntry{active: active, at: time.Now()}
	d.capiStatusMu.Unlock()

	return active
}

// NewDispatcherWithSync tạo dispatcher và cho phép tắt hoàn toàn việc gửi sự
// kiện ra ngoài (dùng cho môi trường test).
func NewDispatcherWithSync(store *db.Store, externalSyncEnabled bool) *Dispatcher {
	d := NewDispatcher(store)
	d.disabled = !externalSyncEnabled
	return d
}

// LeadContext là dữ liệu lead dùng để dựng sự kiện CAPI.
type LeadContext struct {
	LeadID    uint64
	LandingID string
	Nguon     string
	Phone     string
	FullName  string
	Email     string
	City      string
	Region    string
	Country   string
	ISP       string
	URL       string
	// Tracking là JSON tracking của lead (chứa fbc/fbp/fbclid, macro Meta…).
	Tracking map[string]interface{}
	// ClientIP là IP thật của người dùng (KHÔNG băm, Meta yêu cầu rõ).
	ClientIP string
	// UserAgent là UA thật của trình duyệt (KHÔNG băm).
	UserAgent string
	// EventID là event_id do client gửi lên (để dedup với fbq phía browser).
	EventID string
	// SessionID là định danh phiên (dùng để dedup PageView server-side).
	SessionID string
}

// TrackingString đọc 1 key trong tracking dạng chuỗi.
func (lc *LeadContext) TrackingString(keys ...string) string {
	if lc == nil || lc.Tracking == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := lc.Tracking[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// DispatchResult là kết quả gửi 1 sự kiện.
type DispatchResult struct {
	Sent    bool   `json:"sent"`
	Skipped bool   `json:"skipped"`
	Event   string `json:"event_name"`
	EventID string `json:"event_id"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ─── Điểm gọi chính ─────────────────────────────────────────────────────

// SendLeadEvent gửi sự kiện `Lead` khi khách vừa đăng ký (submit form).
//
// Trigger: lead_submit.
//
// ⚠️ CHỈ GỬI KHI LEAD ĐẾN TỪ FACEBOOK (yêu cầu anh itdoanh 2026-11-15).
// Lead từ organic / Google Ads / TikTok ⇒ bỏ qua.
func (d *Dispatcher) SendLeadEvent(lc *LeadContext) *DispatchResult {
	cfg, err := d.resolveConfig(lc.LandingID)
	if err != nil {
		return d.logSkip(lc, EventLead, "lead_submit", "không đọc được cấu hình CAPI: "+err.Error())
	}
	if cfg == nil || !cfg.IsActive {
		return d.logSkip(lc, EventLead, "lead_submit", "CAPI chưa bật cho landing này")
	}
	if !cfg.SendLead {
		return d.logSkip(lc, EventLead, "lead_submit", "sự kiện Lead đang tắt trong cấu hình")
	}

	// ─── GATE: chỉ gửi cho lead đến từ Facebook ───────────────────────
	if res := d.checkFacebookAttribution(lc, EventLead, "lead_submit", cfg); res != nil {
		return res
	}

	return d.send(cfg, lc, EventLead, nil, "lead_submit", "")
}

// SendPageViewEvent gửi sự kiện `PageView` từ server.
//
// Trigger: page_view.
//
// ⚠️ DEDUP: 1 phiên chỉ gửi 1 PageView (kiểm tra qua `session_id` trong log).
// tracker.js gửi POST /api/analytics/track nhiều lần trong 1 phiên
// (pageview, heartbeat, exit, scroll…) ⇒ không dedup thì 1 lượt xem bị
// đếm 5–10 lần trên Meta Ads Manager.
func (d *Dispatcher) SendPageViewEvent(lc *LeadContext) *DispatchResult {
	cfg, err := d.resolveConfig(lc.LandingID)
	if err != nil {
		return d.logSkip(lc, EventPageView, "page_view", "không đọc được cấu hình CAPI: "+err.Error())
	}
	if cfg == nil || !cfg.IsActive {
		return d.logSkip(lc, EventPageView, "page_view", "CAPI chưa bật cho landing này")
	}
	if !cfg.SendPageView {
		return d.logSkip(lc, EventPageView, "page_view", "sự kiện PageView đang tắt trong cấu hình")
	}

	// Dedup theo session (24 giờ).
	if lc.SessionID != "" {
		sent, dbErr := d.store.HasFacebookCAPISessionEvent(lc.SessionID, EventPageView)
		if dbErr == nil && sent {
			return &DispatchResult{
				Skipped: true,
				Event:   EventPageView,
				Message: "PageView đã gửi cho session này (dedup)",
			}
		}
	}

	// ─── GATE: chỉ gửi PageView cho traffic từ Facebook ───────────────
	if res := d.checkFacebookAttribution(lc, EventPageView, "page_view", cfg); res != nil {
		return res
	}

	return d.send(cfg, lc, EventPageView, nil, "page_view", "")
}

// SendCRMSyncEvent gửi sự kiện chuyển đổi khi sales đổi trạng thái CRM.
//
// Trigger: crm_status_change.
//
// 📌 ÁNH XẠ (yêu cầu anh itdoanh 2026-11-15):
//
//	moi                 → Lead
//	da_goi              → Contact
//	da_hoc_buoi_chia_se → StartTrial
//	da_hoc_chuyen_sau   → Subscribe  (≈6.000.000đ)
//	da_mua_hang         → Purchase
//
// ⚠️ CHỈ GỬI KHI LEAD ĐẾN TỪ FACEBOOK: lead từ organic / Google Ads /
// TikTok / truy cập thẳng ⇒ KHÔNG gửi (không có lượt nhấp Meta để khớp,
// gửi sẽ làm nhiễu chỉ số quảng cáo và hạ EMQ).
func (d *Dispatcher) SendCRMSyncEvent(lc *LeadContext, trangThai string) *DispatchResult {
	eventName, hasValue, ok := models.LeadStatusCAPIEvent(trangThai)
	if !ok {
		return &DispatchResult{
			Skipped: true,
			Event:   "",
			Message: "trạng thái '" + trangThai + "' không sinh sự kiện Meta",
		}
	}

	cfg, err := d.resolveConfig(lc.LandingID)
	if err != nil {
		return d.logSkip(lc, eventName, "crm_status_change", "không đọc được cấu hình CAPI: "+err.Error(), trangThai)
	}
	if cfg == nil || !cfg.IsActive {
		return d.logSkip(lc, eventName, "crm_status_change", "CAPI chưa bật cho landing này", trangThai)
	}
	if !cfg.SendCRMSync {
		return d.logSkip(lc, eventName, "crm_status_change", "đồng bộ CRM đang tắt trong cấu hình", trangThai)
	}
	if hasValue && !cfg.SendPurchase {
		return d.logSkip(lc, eventName, "crm_status_change", "sự kiện có giá trị đang tắt trong cấu hình", trangThai)
	}

	// ─── GATE: chỉ gửi cho lead đến từ Facebook ───────────────────────
	if res := d.checkFacebookAttribution(lc, eventName, "crm_status_change", cfg, trangThai); res != nil {
		return res
	}

	var customData *CustomData
	if hasValue {
		customData = BuildValueCustomData(cfg.CourseValue, cfg.Currency, eventName, "course")
	}

	// ─── EVENT ID: chống đếm trùng ───────────────────────────────────
	//
	// `Lead` (từ trạng thái `moi`): dùng LẠI event_id gốc của lead (client
	// đã gửi lên lúc submit form và dùng chung với `fbq`) ⇒ Meta gộp với
	// sự kiện browser/server lúc submit ⇒ KHÔNG thổi phồng số Lead.
	//
	// Các event khác (Contact / StartTrial / Subscribe / Purchase): sinh
	// event_id MỚI. Nếu dùng lại event_id gốc thì Meta sẽ gộp nhầm với
	// sự kiện Lead ⇒ MẤT sự kiện chuyển đổi.
	lcForEvent := lc
	if eventName != EventLead {
		clone := *lc
		clone.EventID = ""
		lcForEvent = &clone
	}

	return d.send(cfg, lcForEvent, eventName, customData, "crm_status_change", trangThai)
}

// CheckFacebookAttribution chặn sự kiện nếu lead/session KHÔNG đến từ
// Facebook.
//
// Trả về non-nil DispatchResult khi cần BỎ QUA (đã ghi log skip).
// Trả về nil khi được phép gửi.
//
// Export để handler khác (analytics) dùng chung 1 quy tắc.
func (d *Dispatcher) CheckFacebookAttribution(lc *LeadContext, eventName, triggeredBy string) *DispatchResult {
	return d.checkFacebookAttribution(lc, eventName, triggeredBy, nil)
}

// checkFacebookAttribution chặn sự kiện nếu lead KHÔNG đến từ Facebook.
//
// Trả về non-nil DispatchResult khi cần BỎ QUA (đã ghi log skip).
// Trả về nil khi lead ĐẾN TỪ Facebook ⇒ được phép gửi.
//
// 📌 YÊU CẦU: "CAPI chỉ gửi khi lead đó đến từ FB chứ không gửi các lead
// ngoài" — áp dụng cho MỌI sự kiện server-side (Lead / CRM sync / PageView).
//
// ⚠️ Nếu cấu hình `only_facebook_traffic = FALSE` ⇒ KHÔNG gate (gửi cho mọi
// lead). Mặc định TRUE.
func (d *Dispatcher) checkFacebookAttribution(lc *LeadContext, eventName, triggeredBy string, cfg *models.FacebookCAPIConfig, trangThaiCRM ...string) *DispatchResult {
	if lc == nil {
		return d.logSkip(nil, eventName, triggeredBy, "thiếu ngữ cảnh lead", trangThaiCRM...)
	}
	if cfg != nil && !cfg.OnlyFacebookTraffic {
		return nil
	}
	traffic := DetectFacebookTraffic(lc.Tracking)
	if !traffic.IsFacebook {
		return d.logSkip(lc, eventName, triggeredBy,
			"lead không đến từ Facebook ("+traffic.Source+") → không gửi CAPI", trangThaiCRM...)
	}
	return nil
}

// SendTestEvent gửi 1 sự kiện thử nghiệm từ Admin UI (test_event_code).
func (d *Dispatcher) SendTestEvent(cfg *models.FacebookCAPIConfig, req models.FacebookCAPIEventRequest, leadCtx *LeadContext) (*CAPIResponse, int, []byte, error) {
	if cfg == nil {
		return nil, 0, nil, fmt.Errorf("chưa có cấu hình CAPI")
	}

	first, last := SplitFullName(req.FirstName)
	if req.LastName != "" {
		last = req.LastName
	}
	externalID := req.ExternalID
	if externalID == "" && req.Phone != "" {
		externalID = req.Phone
	}

	ud := BuildUserData(cfg, UserDataInput{
		Email:      req.Email,
		Phone:      req.Phone,
		FirstName:  first,
		LastName:   last,
		City:       req.City,
		State:      req.State,
		Zip:        req.Zip,
		Country:    req.Country,
		ExternalID: externalID,
	})

	var customData *CustomData
	if req.Value > 0 {
		customData = BuildValueCustomData(req.Value, req.Currency, req.EventName, "course")
	} else if len(req.CustomData) > 0 {
		cd := &CustomData{Extra: req.CustomData}
		if v, ok := req.CustomData["currency"].(string); ok {
			cd.Currency = v
		}
		if v, ok := req.CustomData["value"].(float64); ok {
			cd.Value = &v
		}
		if v, ok := req.CustomData["content_name"].(string); ok {
			cd.ContentName = v
		}
		customData = cd
	}

	eventName := req.EventName
	if eventName == "" {
		eventName = EventLead
	}
	eventSourceURL := req.EventSourceURL
	if eventSourceURL == "" && leadCtx != nil {
		eventSourceURL = leadCtx.URL
	}

	ev := BuildEvent(EventOptions{
		EventName:      eventName,
		EventID:        req.EventID,
		EventSourceURL: eventSourceURL,
		UserData:       ud,
		CustomData:     customData,
	})

	// Gắn IP + UA thật (Meta dùng để đối soát, KHÔNG băm).
	if leadCtx != nil {
		if leadCtx.ClientIP != "" {
			ev.UserData = withIP(ev.UserData, leadCtx.ClientIP)
		}
		if leadCtx.UserAgent != "" {
			ev.UserData = withUA(ev.UserData, leadCtx.UserAgent)
		}
		if fbc := NormalizeFBC(leadCtx.TrackingString("fbc", "fbclid"), leadCtx.TrackingString("fbclid")); fbc != "" {
			ev.UserData = withFBC(ev.UserData, fbc)
		}
		if fbp := NormalizeFBP(leadCtx.TrackingString("fbp")); fbp != "" {
			ev.UserData = withFBP(ev.UserData, fbp)
		}
	}

	payload := &CAPIRequest{
		Data:         []CAPIEvent{ev},
		TestEventCode: strings.TrimSpace(req.TestEventCode),
		PartnerAgent: "apex-crm",
	}

	// ⚠️ Meta CHỈ chấp nhận `test_event_code` do Events Manager sinh ra.
	// Nếu không có mã ⇒ gửi sự kiện THẬT (vào dữ liệu thật, dùng để
	// đối soát) chứ không phải sự kiện test.

	endpoint := BuildEventURL(cfg.PixelID, cfg.APIVersion, cfg.GatewayEnabled, cfg.GatewayEndpoint, cfg.GatewayAccessKey)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return d.client.SendEvents(ctx, endpoint, cfg.AccessToken, payload)
}

// ─── Nội bộ ─────────────────────────────────────────────────────────────

// DefaultConfigFromEnv dựng cấu hình CAPI mặc định từ biến môi trường.
//
// Đây là **fallback**: cấu hình chính nằm trong bảng `facebook_capi_configs`
// (quản lý từ tab "Facebook Ads" của CRM). Biến môi trường chỉ dùng khi
// DB chưa có cấu hình nào — giúp deploy mới mà không cần thao tác UI.
func DefaultConfigFromEnv() *models.FacebookCAPIConfig {
	pixel := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_PIXEL_ID"))
	token := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_ACCESS_TOKEN"))
	version := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_API_VERSION"))
	if version == "" {
		version = DefaultAPIVersion
	}
	currency := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_CURRENCY"))
	if currency == "" {
		currency = "VND"
	}
	value := 6000000.0
	if v := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_COURSE_VALUE")); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			value = parsed
		}
	}
	gwEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("META_CAPI_GATEWAY_ENABLED")), "true")
	gwEndpoint := strings.TrimSpace(os.Getenv("META_CAPI_GATEWAY_ENDPOINT"))
	gwKey := strings.TrimSpace(os.Getenv("META_CAPI_GATEWAY_ACCESS_KEY"))

	if pixel == "" {
		return nil
	}
	if token == "" && !(gwEnabled && gwKey != "") {
		return nil
	}

	return &models.FacebookCAPIConfig{
		LandingID:        "",
		IsActive:         true,
		PixelID:          pixel,
		AccessToken:      token,
		APIVersion:       version,
		GatewayEnabled:   gwEnabled,
		GatewayEndpoint:  gwEndpoint,
		GatewayAccessKey: gwKey,
		SendPageView:     true,
		SendLead:         true,
		SendPurchase:     true,
		SendCRMSync:      true,
		// Mặc định CHỈ gửi cho lead từ Facebook (yêu cầu anh itdoanh).
		OnlyFacebookTraffic: true,
		CourseValue:      value,
		Currency:         currency,
		HashEmail:        true,
		HashPhone:        true,
		HashName:         true,
		HashAddress:      true,
		HashExternalID:   true,
		HashCity:         true,
		HashState:        true,
		HashZip:          true,
		HashCountry:      true,
		SendUserData:     true,
	}
}

// resolveConfig lấy cấu hình CAPI cho landing: ưu tiên DB → fallback .env.
func (d *Dispatcher) resolveConfig(landingID string) (*models.FacebookCAPIConfig, error) {
	// ⚠️ Môi trường test: KHÔNG gửi sự kiện lên Meta.
	//
	// Lý do: mọi sự kiện CAPI đều bị Meta ghi nhận vào báo cáo quảng cáo thật
	// ⇒ lead thử nghiệm làm sai lệch chỉ số và tốn ngân sách quảng cáo.
	if d.disabled {
		return nil, nil
	}

	// ⛔ CHỐT CHẶN TUYỆT ĐỐI (fix 2026-11-18 — landing_cu).
	//
	// Landing nằm trong danh sách cấm (`serverCAPIDenyList` /
	// `META_CAPI_BLOCKED_LANDINGS`) ⇒ KHÔNG BAO GIỜ bắn CAPI, kể cả khi
	// có cấu hình riêng đang bật hoặc fallback sang cấu hình mặc định.
	//
	// Lý do cần chốt chặn (ngoài cấu hình `is_active = FALSE`):
	//   `GetFacebookCAPIConfig` fallback sang dòng MẶC ĐỊNH khi landing chưa có
	//   dòng riêng ⇒ "tắt" bằng cách xoá dòng cấu hình KHÔNG chặn được.
	if d.IsLandingBlocked(landingID) {
		return nil, nil
	}

	cfg, err := d.store.GetFacebookCAPIConfig(landingID)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return cfg, nil
	}
	return d.fallback, nil
}

// send là hàm gửi chung: dựng payload → gọi Meta → ghi log.
func (d *Dispatcher) send(
	cfg *models.FacebookCAPIConfig,
	lc *LeadContext,
	eventName string,
	customData *CustomData,
	triggeredBy string,
	trangThaiCRM string,
) *DispatchResult {
	if cfg.PixelID == "" {
		return d.logSkip(lc, eventName, triggeredBy, trangThaiCRM, "thiếu Pixel ID")
	}
	if cfg.AccessToken == "" && !cfg.GatewayEnabled {
		return d.logSkip(lc, eventName, triggeredBy, trangThaiCRM, "thiếu Access Token")
	}
	if cfg.GatewayEnabled && cfg.GatewayAccessKey == "" {
		return d.logSkip(lc, eventName, triggeredBy, trangThaiCRM, "thiếu Gateway Access Key")
	}

	// ─── user_data ────────────────────────────────────────────────────
	first, last := SplitFullName(lc.FullName)
	externalID := lc.Phone
	ud := BuildUserData(cfg, UserDataInput{
		Email:      lc.Email,
		Phone:      lc.Phone,
		FirstName:  first,
		LastName:   last,
		City:       lc.City,
		State:      lc.Region,
		Country:    lc.Country,
		ExternalID: externalID,
	})

	// ─── event ────────────────────────────────────────────────────────
	eventID := strings.TrimSpace(lc.EventID)
	ev := BuildEvent(EventOptions{
		EventName:      eventName,
		EventID:        eventID,
		EventSourceURL: lc.URL,
		UserData:       ud,
		CustomData:     customData,
	})

	// IP + UA + fbc/fbp: Meta dùng để khớp lượt nhấp. KHÔNG băm.
	if lc.ClientIP != "" {
		ev.UserData = withIP(ev.UserData, lc.ClientIP)
	}
	if lc.UserAgent != "" {
		ev.UserData = withUA(ev.UserData, lc.UserAgent)
	}
	fbc := NormalizeFBC(lc.TrackingString("fbc"), lc.TrackingString("fbclid"))
	if fbc != "" {
		ev.UserData = withFBC(ev.UserData, fbc)
	}
	fbp := NormalizeFBP(lc.TrackingString("fbp"))
	if fbp != "" {
		ev.UserData = withFBP(ev.UserData, fbp)
	}

	payload := &CAPIRequest{
		Data:         []CAPIEvent{ev},
		PartnerAgent: "apex-crm",
	}

	endpoint := BuildEventURL(cfg.PixelID, cfg.APIVersion, cfg.GatewayEnabled, cfg.GatewayEndpoint, cfg.GatewayAccessKey)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, httpStatus, rawBody, err := d.client.SendEvents(ctx, endpoint, cfg.AccessToken, payload)

	logEv := models.FacebookCAPIEvent{
		LeadID:       lc.LeadID,
		LandingID:    lc.LandingID,
		EventName:    eventName,
		EventID:      ev.EventID,
		ActionSource: "website",
		PixelID:      cfg.PixelID,
		HTTPStatus:   httpStatus,
		TriggeredBy:  triggeredBy,
		TrangThaiCRM: trangThaiCRM,
		SessionID:    lc.SessionID,
	}
	// ⚠️ FB ADS TAB 2026-11-20: lưu value/currency/lead_score gửi kèm để UI
	// hiện chi tiết mỗi lần đổi trạng thái (trước đây log không lưu nên chỉ
	// hiện PageView/Lead lúc mới về).
	if customData != nil {
		if customData.Value != nil {
			logEv.Value = *customData.Value
		}
		logEv.Currency = customData.Currency
		logEv.LeadScore = customData.LeadScore
	}
	if len(rawBody) > 0 {
		logEv.Response = json.RawMessage(rawBody)
	}

	if err != nil {
		logEv.Status = "failed"
		logEv.Error = err.Error()
		_ = d.store.LogFacebookCAPIEvent(logEv)
		return &DispatchResult{
			Sent:    false,
			Event:   eventName,
			EventID: ev.EventID,
			Error:   err.Error(),
		}
	}

	logEv.Status = "success"
	_ = d.store.LogFacebookCAPIEvent(logEv)

	return &DispatchResult{
		Sent:    true,
		Event:   eventName,
		EventID: ev.EventID,
		Message: fmt.Sprintf("Meta đã nhận %d sự kiện", resp.EventsReceived),
	}
}

// logSkip ghi log 1 sự kiện bị bỏ qua (cấu hình chưa bật / thiếu token…).
// ⚠️ FB ADS TAB 2026-11-20: giữ thêm trang_thai_crm để tab hiện rõ mỗi lần
// đổi trạng thái CRM (kể cả khi bị skip do chưa bật CAPI / không phải traffic FB).
func (d *Dispatcher) logSkip(lc *LeadContext, eventName, triggeredBy, reason string, trangThaiCRM ...string) *DispatchResult {
	ev := models.FacebookCAPIEvent{
		ActionSource: "website",
		Status:       "skipped",
		TriggeredBy:  triggeredBy,
		Error:        reason,
	}
	if lc != nil {
		ev.LeadID = lc.LeadID
		ev.LandingID = lc.LandingID
		if lc.HasConversionValue {
			ev.Value = lc.ConversionValue
			ev.Currency = "VND"
		}
		ev.LeadScore = lc.LeadScore
	}
	if len(trangThaiCRM) > 0 {
		ev.TrangThaiCRM = trangThaiCRM[0]
	} else if triggeredBy == "crm_status_change" {
		ev.TrangThaiCRM = eventName
	}
	if eventName != "" {
		ev.EventName = eventName
	}
	_ = d.store.LogFacebookCAPIEvent(ev)
	return &DispatchResult{
		Skipped: true,
		Event:   eventName,
		Message: reason,
	}
}

// ─── Helper gắn trường đặc biệt vào user_data ──────────────────────────
//
// `client_ip_address` và `client_user_agent` PHẢI là giá trị rõ (Meta yêu cầu).
// `fbc`/`fbp` cũng gửi rõ — chúng là định danh lượt nhấp/phiên của Meta.

func withIP(ud *UserData, ip string) *UserData {
	if ud == nil {
		ud = &UserData{}
	}
	ud.ClientIPAddress = ip
	return ud
}

func withUA(ud *UserData, ua string) *UserData {
	if ud == nil {
		ud = &UserData{}
	}
	ud.ClientUserAgent = ua
	return ud
}

func withFBC(ud *UserData, fbc string) *UserData {
	if ud == nil {
		ud = &UserData{}
	}
	ud.FBC = fbc
	return ud
}

func withFBP(ud *UserData, fbp string) *UserData {
	if ud == nil {
		ud = &UserData{}
	}
	ud.FBP = fbp
	return ud
}
