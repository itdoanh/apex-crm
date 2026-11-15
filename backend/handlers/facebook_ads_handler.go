package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/integrations/facebook"
	"apex-backend/models"
)

// FacebookAdsHandler xử lý các API quản trị cho module Facebook Ads:
//   - Cấu hình Conversions API (theo từng landing)
//   - Test Event / Debug Event
//   - Dataset Quality API (EMQ, coverage, dedupe, freshness, delay, value)
//   - Log gửi sự kiện
//   - Báo cáo hiệu quả theo chiến dịch / adset / ad (QC)
//   - Danh mục trạng thái pipeline CRM
type FacebookAdsHandler struct {
	cfg   *config.Config
	store *db.Store
}

func NewFacebookAdsHandler(cfg *config.Config, store *db.Store) *FacebookAdsHandler {
	return &FacebookAdsHandler{
		cfg:   cfg,
		store: store,
	}
}

// resolveConfig lấy cấu hình CAPI cho landing: ưu tiên DB → fallback .env.
//
// Cấu hình trong DB (quản lý ở tab "Facebook Ads") là nguồn chính. Biến môi
// trường `.env` chỉ là fallback khi DB chưa có cấu hình nào.
func (h *FacebookAdsHandler) resolveConfig(landingID string) (*models.FacebookCAPIConfig, error) {
	cfg, err := h.store.GetFacebookCAPIConfig(landingID)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return cfg, nil
	}
	return facebook.DefaultConfigFromEnv(), nil
}

// HandleListConfigs trả về toàn bộ cấu hình CAPI (token đã che).
func (h *FacebookAdsHandler) HandleListConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	configs, err := h.store.ListFacebookCAPIConfigs()
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi đọc cấu hình CAPI: "+err.Error())
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"configs":  configs,
		"defaults": facebook.DefaultAPIVersion,
	})
}

// HandleSaveConfig tạo / cập nhật cấu hình CAPI.
func (h *FacebookAdsHandler) HandleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Chỉ chấp nhận POST/PUT", http.StatusMethodNotAllowed)
		return
	}

	var in models.FacebookCAPIConfigInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fbWriteJSONError(w, http.StatusBadRequest, "Dữ liệu JSON không hợp lệ: "+err.Error())
		return
	}

	in.LandingID = strings.TrimSpace(in.LandingID)

	// Token đã che (dạng `abcd****wxyz`) không được ghi đè giá trị thật.
	// Rỗng ⇒ giữ token cũ trong DB (upsert xử lý).
	in.AccessToken = stripMaskedToken(in.AccessToken)
	in.GatewayAccessKey = stripMaskedToken(in.GatewayAccessKey)

	// Lấy cấu hình hiện tại để biết token cũ (trường hợp Admin UI gửi lại
	// token đã che, hoặc bỏ trống trường token khi chỉ muốn đổi cờ bật/tắt).
	var existingToken, existingGatewayKey string
	if prev, err := h.store.GetFacebookCAPIConfig(in.LandingID); err == nil && prev != nil {
		existingToken = prev.AccessToken
		existingGatewayKey = prev.GatewayAccessKey
	}
	hasToken := in.AccessToken != "" || existingToken != ""
	hasGatewayKey := in.GatewayAccessKey != "" || existingGatewayKey != ""

	isActive := in.IsActive != nil && *in.IsActive
	if isActive {
		if in.PixelID == "" {
			fbWriteJSONError(w, http.StatusBadRequest, "Thiếu Pixel ID — không thể bật CAPI")
			return
		}
		if !hasToken && !in.GatewayEnabled {
			fbWriteJSONError(w, http.StatusBadRequest, "Thiếu Access Token (hoặc bật Gateway và nhập Access Key)")
			return
		}
		if in.GatewayEnabled && !hasGatewayKey {
			fbWriteJSONError(w, http.StatusBadRequest, "Thiếu Gateway Access Key")
			return
		}
	}
	if in.APIVersion == "" {
		in.APIVersion = facebook.DefaultAPIVersion
	}
	if in.Currency == "" {
		in.Currency = "VND"
	}
	if in.CourseValue <= 0 {
		in.CourseValue = 6000000
	}

	cfg, err := h.store.UpsertFacebookCAPIConfig(in)
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi lưu cấu hình: "+err.Error())
		return
	}

	cfg.MaskAccessToken()
	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Đã lưu cấu hình Conversions API",
		"config":  cfg,
	})
}

// HandleDeleteConfig xoá cấu hình CAPI của 1 landing.
func (h *FacebookAdsHandler) HandleDeleteConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Chỉ chấp nhận POST/DELETE", http.StatusMethodNotAllowed)
		return
	}

	landingID := strings.TrimSpace(r.URL.Query().Get("landing_id"))
	if landingID == "" {
		fbWriteJSONError(w, http.StatusBadRequest, "Thiếu landing_id")
		return
	}
	if err := h.store.DeleteFacebookCAPIConfig(landingID); err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi xoá cấu hình: "+err.Error())
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Đã xoá cấu hình",
	})
}

// HandleToggleConfig bật/tắt nhanh CAPI cho 1 landing (không cần gửi lại
// toàn bộ cấu hình). Dùng nút công tắc trên bảng cấu hình.
//
// Yêu cầu: landing phải có cấu hình riêng trong DB. Landing chưa có ⇒ báo
// lỗi để Admin tạo cấu hình trước (tránh bật nhầm cấu hình mặc định).
func (h *FacebookAdsHandler) HandleToggleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Chỉ chấp nhận POST/PUT", http.StatusMethodNotAllowed)
		return
	}

	var in struct {
		LandingID string `json:"landing_id"`
		IsActive  *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fbWriteJSONError(w, http.StatusBadRequest, "Dữ liệu JSON không hợp lệ: "+err.Error())
		return
	}

	in.LandingID = strings.TrimSpace(in.LandingID)
	if in.LandingID == "" {
		fbWriteJSONError(w, http.StatusBadRequest, "Thiếu landing_id")
		return
	}
	if in.IsActive == nil {
		fbWriteJSONError(w, http.StatusBadRequest, "Thiếu is_active")
		return
	}

	found, err := h.store.ToggleFacebookCAPIConfig(in.LandingID, *in.IsActive)
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi bật/tắt CAPI: "+err.Error())
		return
	}
	if !found {
		fbWriteJSONError(w, http.StatusNotFound,
			"Landing "+in.LandingID+" chưa có cấu hình CAPI riêng — hãy tạo cấu hình trước khi bật/tắt")
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"landing_id": in.LandingID,
		"is_active":  *in.IsActive,
	})
}

// HandleTestEvent gửi 1 sự kiện thử nghiệm lên Meta (Test Events).
func (h *FacebookAdsHandler) HandleTestEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req models.FacebookCAPIEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fbWriteJSONError(w, http.StatusBadRequest, "Dữ liệu JSON không hợp lệ: "+err.Error())
		return
	}

	cfg, err := h.resolveConfig(strings.TrimSpace(req.LandingID))
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi đọc cấu hình: "+err.Error())
		return
	}
	if cfg == nil {
		fbWriteJSONError(w, http.StatusBadRequest, "Chưa cấu hình Conversions API cho landing này")
		return
	}
	if cfg.PixelID == "" {
		fbWriteJSONError(w, http.StatusBadRequest, "Cấu hình chưa có Pixel ID")
		return
	}

	dispatcher := facebook.NewDispatcher(h.store)
	resp, httpStatus, rawBody, err := dispatcher.SendTestEvent(cfg, req, nil)
	if err != nil {
		fbWriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":     false,
			"error":       err.Error(),
			"http_status": httpStatus,
			"response":    json.RawMessage(safeJSON(rawBody)),
		})
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("Meta đã nhận %d sự kiện thử nghiệm", resp.EventsReceived),
		"http_status": httpStatus,
		"response":    json.RawMessage(safeJSON(rawBody)),
	})
}

// HandleDatasetQuality đọc chỉ số chất lượng dữ liệu từ Meta.
//
// ⚠️ Meta yêu cầu chờ ≥20 phút sau khi gửi sự kiện, và số liệu thường trễ
// vài giờ ⇒ endpoint này CHỈ được gọi thủ công từ Admin UI.
func (h *FacebookAdsHandler) HandleDatasetQuality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	landingID := strings.TrimSpace(r.URL.Query().Get("landing_id"))
	cfg, err := h.resolveConfig(landingID)
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi đọc cấu hình: "+err.Error())
		return
	}
	if cfg == nil {
		fbWriteJSONError(w, http.StatusBadRequest, "Chưa cấu hình Conversions API cho landing này")
		return
	}
	if cfg.AccessToken == "" {
		fbWriteJSONError(w, http.StatusBadRequest, "Cấu hình chưa có Access Token để đọc Dataset Quality API")
		return
	}

	client := facebook.NewClient()
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	// `agent_name` là tuỳ chọn — dùng để lọc sự kiện theo đối tác gửi.
	// Giá trị phải ở dạng chuẩn hoá (lowercase). Ta dùng "apex_crm".
	raw, httpStatus, rawBody, err := client.FetchDatasetQuality(ctx, cfg.APIVersion, cfg.PixelID, cfg.AccessToken, "apex_crm")
	if err != nil {
		fbWriteJSON(w, http.StatusOK, map[string]interface{}{
			"success":     false,
			"error":       err.Error(),
			"http_status": httpStatus,
			"response":    json.RawMessage(safeJSON(rawBody)),
		})
		return
	}

	summary := facebook.ParseDatasetQuality(raw, cfg.PixelID)
	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"pixel_id":   cfg.PixelID,
		"fetched_at": summary.FetchedAt,
		"quality":    summary,
		"raw":        raw,
	})
}

// HandleListLogs trả về log gửi sự kiện lên Meta.
func (h *FacebookAdsHandler) HandleListLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	leadID, _ := strconv.ParseUint(q.Get("lead_id"), 10, 64)

	logs, err := h.store.ListFacebookCAPILogs(
		strings.TrimSpace(q.Get("landing_id")),
		strings.TrimSpace(q.Get("status")),
		strings.TrimSpace(q.Get("event_name")),
		leadID,
		limit,
	)
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi đọc log CAPI: "+err.Error())
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"logs":    logs,
	})
}

// HandleStats trả về số liệu tổng hợp gửi sự kiện CAPI.
func (h *FacebookAdsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	days, _ := strconv.Atoi(q.Get("days"))
	if days <= 0 {
		days = 30
	}

	stats, err := h.store.GetFacebookCAPIStats(strings.TrimSpace(q.Get("landing_id")), days)
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi thống kê CAPI: "+err.Error())
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}

// HandleCampaignBreakdown trả về báo cáo hiệu quả theo chiến dịch/adset/ad.
func (h *FacebookAdsHandler) HandleCampaignBreakdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	rows, err := h.store.GetMetaCampaignBreakdown(
		strings.TrimSpace(q.Get("landing_id")),
		strings.TrimSpace(q.Get("nguon")),
		strings.TrimSpace(q.Get("from_date")),
		strings.TrimSpace(q.Get("to_date")),
		limit,
	)
	if err != nil {
		fbWriteJSONError(w, http.StatusInternalServerError, "Lỗi báo cáo chiến dịch: "+err.Error())
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"rows":    rows,
	})
}

// HandleParseURL phân tích URL quảng cáo Meta thành bộ thông số chuẩn.
//
// Dùng để anh itdoanh dán URL thật vào và kiểm tra xem hệ thống có đọc đúng
// campaign.id / adset.id / ad.id hay không (tránh lỗi cấu hình URL Parameters).
func (h *FacebookAdsHandler) HandleParseURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET/POST", http.StatusMethodNotAllowed)
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" && r.Method == http.MethodPost {
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			rawURL = strings.TrimSpace(req.URL)
		}
	}
	if rawURL == "" {
		fbWriteJSONError(w, http.StatusBadRequest, "Thiếu URL cần phân tích")
		return
	}

	params := facebook.ParseMetaAdsURL(rawURL)
	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"params":       params,
		"has_campaign": params.HasCampaign(),
		"fbc":          facebook.NormalizeFBC("", params.Fbclid),
		"fbp":          facebook.NormalizeFBP(""),
		"notes": []string{
			"utm_campaign = campaign.id (ID chiến dịch)",
			"utm_content  = adset.id  (ID adset)",
			"utm_term     = ad.id     (ID quảng cáo)",
			"campaign_name / adset_name / ad_name = tên hiển thị (cần bật 'Include names in URL' trong Ads Manager)",
			"utm_id thường trùng campaign.id → dùng làm fallback",
		},
	})
}

// HandleStatusCatalog trả về danh mục trạng thái pipeline CRM.
//
// Frontend dùng endpoint này để dựng Kanban 10 cột + dropdown trạng thái,
// nhờ vậy không phải hard-code danh sách ở 2 nơi (JS + Go).
func (h *FacebookAdsHandler) HandleStatusCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	fbWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"statuses": models.ListLeadStatusDefs(),
		"order":    models.LeadStatusOrder,
		"note":     "'tiem_nang' và 'chot' là giá trị cũ, tự động hợp nhất vào 'da_hoc_buoi_chia_se' và 'da_mua_hang'.",
	})
}

// ─── Helper ─────────────────────────────────────────────────────────────

// stripMaskedToken loại bỏ token đã che (dạng `abcd****wxyz`) trước khi ghi DB.
//
// Admin UI load cấu hình lên rồi gửi lại → token vẫn ở dạng đã che. Nếu ghi
// thẳng vào DB thì token thật bị mất ⇒ CAPI hỏng. Vì vậy ta giữ token cũ.
func stripMaskedToken(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.Contains(v, "****") {
		return ""
	}
	return v
}

// safeJSON đảm bảo body phản hồi luôn là JSON hợp lệ.
func safeJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	if json.Valid(b) {
		return b
	}
	// Escape thành chuỗi JSON.
	out, _ := json.Marshal(string(b))
	return out
}

func fbWriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func fbWriteJSONError(w http.ResponseWriter, status int, message string) {
	fbWriteJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// đảm bảo db package được dùng (import check).
var _ = db.Store{}
