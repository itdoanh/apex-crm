package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"apex-backend/db"
)

// ══════════════════════════════════════════════════════════════════════
// REPORT HANDLER — Hệ thống báo cáo & biểu đồ thiết kế lại (2026-11-13)
// ══════════════════════════════════════════════════════════════════════
//
// Toàn bộ endpoint dưới `/api/analytics/report/*` dùng CHUNG một bộ lọc
// (`db.ReportFilter`) được parse từ query string ⇒ mọi thẻ KPI, biểu đồ và
// bảng trong 6 tab luôn thống nhất với nhau.
//
// Danh sách endpoint:
//
//	GET  /api/analytics/report/summary          → thẻ KPI + so sánh kỳ trước
//	GET  /api/analytics/report/timeseries       → biểu đồ 1 (combo bar + line)
//	GET  /api/analytics/report/landings         → bảng 1 (leaderboard đa landing)
//	GET  /api/analytics/report/meta             → biểu đồ 2 + 3 (nền tảng, placement, in-app)
//	GET  /api/analytics/report/ad-hierarchy     → bảng 2 (campaign → adset → ad)
//	GET  /api/analytics/report/heatmap          → biểu đồ 4 (7 ngày × 24 giờ)
//	GET  /api/analytics/report/behavior         → biểu đồ 5, 6, 7 + bảng 3
//	GET  /api/analytics/report/form             → biểu đồ 8, 9 (phễu form, autofill)
//	GET  /api/analytics/report/devices          → biểu đồ 10, 11 + bảng 4
//	GET  /api/analytics/report/security         → biểu đồ 12 (bảo mật, bot)
//	GET  /api/analytics/report/quarantine       → bảng 5 (vùng cách ly)
//	POST /api/analytics/report/quarantine/approve → duyệt thủ công lead bị chặn
//	GET  /api/analytics/report/journey          → hành trình khách hàng (timeline)
//	GET  /api/analytics/report/filters          → danh sách giá trị cho dropdown
type ReportHandler struct {
	store *db.Store
}

func NewReportHandler(store *db.Store) *ReportHandler {
	return &ReportHandler{store: store}
}

// writeJSON ghi phản hồi JSON chuẩn.
func writeJSON(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// reportFilterOf parse bộ lọc từ query string.
func reportFilterOf(r *http.Request) db.ReportFilter {
	return db.ParseReportFilter(r.URL.Query())
}

// ─── GET /api/analytics/report/summary ───────────────────────────────
//
// Trả về thẻ KPI tổng hợp. `compare=1` ⇒ kèm số liệu kỳ trước + % thay đổi.
func (h *ReportHandler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	compare := q.Get("compare") == "1" || strings.EqualFold(q.Get("compare"), "true")

	data, err := h.store.GetReportSummary(reportFilterOf(r), compare)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": data})
}

// ─── GET /api/analytics/report/timeseries ────────────────────────────
//
// Biểu đồ 1 — xu hướng traffic & lead. `granularity` = hour | day | week.
func (h *ReportHandler) HandleTimeSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	items, err := h.store.GetReportTimeSeries(reportFilterOf(r), granularity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"granularity": granularity,
		"data":        items,
	})
}

// ─── GET /api/analytics/report/landings ──────────────────────────────
//
// Bảng 1 — ma trận so sánh hiệu suất từng Landing Page.
func (h *ReportHandler) HandleLandingLeaderboard(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.GetReportLandingLeaderboard(reportFilterOf(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": items})
}

// ─── GET /api/analytics/report/meta ──────────────────────────────────
//
// Biểu đồ 2 + 3 — phân bổ nền tảng Meta, vị trí hiển thị, in-app vs ngoài.
func (h *ReportHandler) HandleMeta(w http.ResponseWriter, r *http.Request) {
	f := reportFilterOf(r)

	platforms, err := h.store.GetReportMetaPlatform(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	placements, err := h.store.GetReportMetaPlacement(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	inApp, err := h.store.GetReportInAppBrowser(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"platforms":  platforms,
			"placements": placements,
			"in_app":     inApp,
		},
	})
}

// ─── GET /api/analytics/report/ad-hierarchy ──────────────────────────
//
// Bảng 2 — phân cấp Campaign → Adset → Ad.
func (h *ReportHandler) HandleAdHierarchy(w http.ResponseWriter, r *http.Request) {
	limit := db.ParseIntSafe(r.URL.Query().Get("limit"), 200)

	items, err := h.store.GetReportAdHierarchy(reportFilterOf(r), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": items})
}

// ─── GET /api/analytics/report/heatmap ───────────────────────────────
//
// Biểu đồ 4 — khung giờ vàng 7 ngày × 24 giờ. `metric` = leads | sessions | cr.
func (h *ReportHandler) HandleHeatmap(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "leads"
	}

	counts, crs, err := h.store.GetReportHourHeatmap(reportFilterOf(r), metric)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"metric": metric,
			// Ma trận [7][24]; chỉ số 0 = Thứ 2 ... 6 = Chủ nhật.
			"matrix":   counts,
			"cr":       crs,
			"days":     []string{"Thứ 2", "Thứ 3", "Thứ 4", "Thứ 5", "Thứ 6", "Thứ 7", "Chủ nhật"},
			"hours":    buildHourLabels(),
		},
	})
}

// buildHourLabels trả về nhãn 24 giờ (00 → 23).
func buildHourLabels() []string {
	labels := make([]string, 24)
	for i := 0; i < 24; i++ {
		labels[i] = strconv.Itoa(i)
	}
	return labels
}

// ─── GET /api/analytics/report/behavior ──────────────────────────────
//
// Biểu đồ 5 (phễu cuộn), 6 (phân phối thời gian xem), 7 (tương tác) + bảng 3.
func (h *ReportHandler) HandleBehavior(w http.ResponseWriter, r *http.Request) {
	f := reportFilterOf(r)

	scroll, err := h.store.GetReportScrollFunnel(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	visible, err := h.store.GetReportVisibleTimeDistribution(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	engagement, err := h.store.GetReportEngagement(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	clicked, err := h.store.GetReportTopClickedElements(f, db.ParseIntSafe(r.URL.Query().Get("limit"), 30))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"scroll_funnel":  scroll,
			"visible_dist":   visible,
			"engagement":     engagement,
			"top_clicked":    clicked,
		},
	})
}

// ─── GET /api/analytics/report/form ──────────────────────────────────
//
// Biểu đồ 8 (phễu form) + 9 (nguồn autofill).
func (h *ReportHandler) HandleForm(w http.ResponseWriter, r *http.Request) {
	f := reportFilterOf(r)

	funnel, err := h.store.GetReportFormFunnel(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	autofill, err := h.store.GetReportAutofillSource(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"form_funnel":    funnel,
			"autofill_source": autofill,
		},
	})
}

// ─── GET /api/analytics/report/devices ───────────────────────────────
//
// Biểu đồ 10 (thiết bị & OS), 11 (mạng & phần cứng) + bảng 4.
func (h *ReportHandler) HandleDevices(w http.ResponseWriter, r *http.Request) {
	f := reportFilterOf(r)

	breakdown, err := h.store.GetReportDeviceBreakdown(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	models, err := h.store.GetReportDeviceModels(f, db.ParseIntSafe(r.URL.Query().Get("limit"), 30))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	hardware, err := h.store.GetReportConnectionHardware(f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"breakdown": breakdown,
			"models":    models,
			"hardware":  hardware,
		},
	})
}

// ─── GET /api/analytics/report/security ──────────────────────────────
//
// Biểu đồ 12 — KPI bảo mật + phân loại lý do chặn bot.
func (h *ReportHandler) HandleSecurity(w http.ResponseWriter, r *http.Request) {
	data, err := h.store.GetReportSecurity(reportFilterOf(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": data})
}

// ─── GET /api/analytics/report/quarantine ─────────────────────────────
//
// Bảng 5 — nhật ký vùng cách ly spam.
func (h *ReportHandler) HandleQuarantine(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data, err := h.store.GetReportQuarantine(
		strings.TrimSpace(q.Get("landing_id")),
		db.ParseIntSafe(q.Get("limit"), 50),
		db.ParseIntSafe(q.Get("offset"), 0),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data["data"],
		"total":   data["total"],
		"limit":   data["limit"],
		"offset":  data["offset"],
	})
}

// ─── POST /api/analytics/report/quarantine/approve ────────────────────
//
// Duyệt thủ công một lead bị chặn nhầm ⇒ đẩy thẳng về CRM.
func (h *ReportHandler) HandleApproveQuarantine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "error": "Chỉ chấp nhận POST"})
		return
	}

	var body struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Thiếu id hợp lệ"})
		return
	}

	result, err := h.store.ApproveQuarantineLead(body.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": result})
}

// ─── GET /api/analytics/report/journey ───────────────────────────────
//
// Hành trình khách hàng: hồ sơ + timeline từng giây.
func (h *ReportHandler) HandleJourney(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sessionID := strings.TrimSpace(q.Get("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(q.Get("id"))
	}
	visitorID := strings.TrimSpace(q.Get("visitor_id"))
	phone := strings.TrimSpace(q.Get("phone"))

	// Cho phép tra theo SĐT khách (từ modal chi tiết lead trong CRM).
	if sessionID == "" && visitorID == "" && phone != "" {
		v, err := h.store.FindVisitorIDByPhone(phone)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "Không tìm thấy phiên của số " + phone})
			return
		}
		visitorID = v
	}

	if sessionID == "" && visitorID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Thiếu session_id, visitor_id hoặc phone"})
		return
	}

	data, err := h.store.GetCustomerJourney(sessionID, visitorID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": data})
}

// ─── GET /api/analytics/report/filters ───────────────────────────────
//
// Danh sách giá trị distinct để đổ vào các dropdown của bộ lọc toàn cục.
func (h *ReportHandler) HandleFilterOptions(w http.ResponseWriter, r *http.Request) {
	data, err := h.store.GetReportFilterOptions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": data})
}
