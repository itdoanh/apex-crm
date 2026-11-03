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
	"apex-backend/integrations/googlesheets"
	"apex-backend/models"
)

type GoogleSheetHandler struct {
	cfg   *config.Config
	store *db.Store
}

func NewGoogleSheetHandler(cfg *config.Config, store *db.Store) *GoogleSheetHandler {
	return &GoogleSheetHandler{
		cfg:   cfg,
		store: store,
	}
}

// HandleGetBotInfo trả về thông tin Email Bot Service Account của hệ thống để hiển thị cho người dùng Share Sheet
func (h *GoogleSheetHandler) HandleGetBotInfo(w http.ResponseWriter, r *http.Request) {
	botEmail := h.cfg.GoogleServiceAccountEmail
	if botEmail == "" {
		botEmail = googlesheets.DefaultBotEmail
	}
	hasKey := h.cfg.GoogleServiceAccountJSON != "" || h.cfg.GoogleServiceAccountFile != ""
	if !hasKey {
		if _, err := googlesheets.ResolveCredentials(""); err == nil {
			hasKey = true
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"bot_email":     botEmail,
		"is_configured": hasKey,
		"instructions":  "Vui lòng bấm nút 'Chia sẻ (Share)' trên file Google Sheet của bạn và cấp quyền 'Người chỉnh sửa (Editor)' cho email bot hệ thống.",
	})
}

// HandleGetMetadata bóc tách ID từ URL hoặc chuỗi, kết nối tới Google Sheets API lấy Title & danh sách các Tab
func (h *GoogleSheetHandler) HandleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CredentialsJSON string `json:"credentials_json"`
		SpreadsheetID   string `json:"spreadsheet_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu JSON không hợp lệ: " + err.Error(),
		})
		return
	}

	req.CredentialsJSON = strings.TrimSpace(req.CredentialsJSON)
	req.SpreadsheetID = strings.TrimSpace(req.SpreadsheetID)

	if req.SpreadsheetID == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Vui lòng dán Link Google Sheet hoặc Spreadsheet ID",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	meta, err := googlesheets.GetSpreadsheetMetadata(ctx, req.CredentialsJSON, req.SpreadsheetID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    meta,
	})
}

// HandleGetHeaders đọc dòng 1 (tiêu đề cột) của Tab được chọn
func (h *GoogleSheetHandler) HandleGetHeaders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CredentialsJSON string `json:"credentials_json"`
		SpreadsheetID   string `json:"spreadsheet_id"`
		TabName         string `json:"tab_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu JSON không hợp lệ",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	headers, err := googlesheets.GetSheetHeaders(ctx, req.CredentialsJSON, req.SpreadsheetID, req.TabName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"headers": headers,
	})
}

// HandleCreateHeaders tự động ghi dòng tiêu đề chuẩn vào dòng 1 của tab
func (h *GoogleSheetHandler) HandleCreateHeaders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CredentialsJSON string   `json:"credentials_json"`
		SpreadsheetID   string   `json:"spreadsheet_id"`
		TabName         string   `json:"tab_name"`
		Headers         []string `json:"headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu JSON không hợp lệ",
		})
		return
	}

	if len(req.Headers) == 0 {
		defaultMappings := googlesheets.GetStandardDefaultMappings()
		req.Headers = make([]string, len(defaultMappings))
		for i, m := range defaultMappings {
			req.Headers[i] = m.Header
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if err := googlesheets.WriteHeaders(ctx, req.CredentialsJSON, req.SpreadsheetID, req.TabName, req.Headers); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Đã tạo thành công dòng tiêu đề với %d cột trên Tab '%s'", len(req.Headers), req.TabName),
	})
}

// HandleTestAppend gửi một dòng dữ liệu mẫu vào Google Sheet để kiểm tra kết nối và ánh xạ cột
func (h *GoogleSheetHandler) HandleTestAppend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CredentialsJSON string                 `json:"credentials_json"`
		SpreadsheetID   string                 `json:"spreadsheet_id"`
		TabName         string                 `json:"tab_name"`
		FieldMappings   []models.FieldMapping  `json:"field_mappings"`
		TestLead        map[string]interface{} `json:"test_lead"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu JSON không hợp lệ",
		})
		return
	}

	if len(req.FieldMappings) == 0 {
		req.FieldMappings = googlesheets.GetStandardDefaultMappings()
	}

	// Chuẩn bị Mock Lead nếu client không truyền
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	mockLead := &models.Lead{
		ID:        999999,
		CreatedAt: nowStr,
		LandingID: "mxv_080_online",
		Nguon:     "google_ads_test",
		Phone:     "0984386538",
		TrangThai: "moi",
		GhiChu:    "Dòng kiểm tra kết nối từ Admin Apex CRM",
	}

	mockTracking := map[string]interface{}{
		"fullName":         "Nguyễn Văn Thử Nghiệm",
		"so_dien_thoai":    "0984386538",
		"carrier":          "Viettel",
		"landing_id":       "mxv_080_online",
		"nguon":            "google_ads_test",
		"is_autofill":      true,
		"fill_duration_ms": 1250,
		"time_on_page":     45,
		"max_scroll":       85,
		"device_type":      "mobile",
		"ip_address":       "113.190.234.12",
		"utm_campaign":     "Chien_Dich_Test_2026",
		"utm_source":       "google",
		"utm_medium":       "cpc",
		"session_id":       "test_sess_abc123",
	}

	if req.TestLead != nil {
		for k, v := range req.TestLead {
			mockTracking[k] = v
		}
		if name, ok := req.TestLead["fullName"]; ok && name != nil {
			mockTracking["fullName"] = name
		}
		if phone, ok := req.TestLead["so_dien_thoai"]; ok && phone != nil {
			mockLead.Phone = fmt.Sprintf("%v", phone)
			mockTracking["so_dien_thoai"] = phone
		}
	}

	rowValues := googlesheets.BuildRowValues(req.FieldMappings, mockLead, mockTracking)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	updatedRange, err := googlesheets.AppendRow(ctx, req.CredentialsJSON, req.SpreadsheetID, req.TabName, rowValues)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"message":       "Gửi dòng dữ liệu thử nghiệm thành công!",
		"updated_range": updatedRange,
		"row_values":    rowValues,
	})
}

// HandleListDriveFiles duyệt danh sách các file Google Sheet từ tài khoản Service Account có quyền truy cập
func (h *GoogleSheetHandler) HandleListDriveFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CredentialsJSON string `json:"credentials_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu JSON không hợp lệ",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	files, err := googlesheets.ListDriveSpreadsheets(ctx, req.CredentialsJSON)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   files,
	})
}

// HandleListConfigs trả về danh sách các kết nối Google Sheet đã lưu
func (h *GoogleSheetHandler) HandleListConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	configs, err := h.store.ListGoogleSheetConfigs()
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Ẩn bớt private key trong credentials_json để an toàn khi gửi ra client
	masked := make([]map[string]interface{}, len(configs))
	for i, c := range configs {
		saEmail := googlesheets.GetServiceAccountEmail(c.CredentialsJSON)
		masked[i] = map[string]interface{}{
			"id":                   c.ID,
			"name":                 c.Name,
			"auth_type":            c.AuthType,
			"service_account_email": saEmail,
			"spreadsheet_id":       c.SpreadsheetID,
			"spreadsheet_name":     c.SpreadsheetName,
			"sheet_tab_name":       c.SheetTabName,
			"field_mappings":       c.FieldMappings,
			"filter_landing_id":    c.FilterLandingID,
			"filter_nguon":         c.FilterNguon,
			"is_active":            c.IsActive,
			"auto_create_header":   c.AutoCreateHeader,
			"last_synced_at":       c.LastSyncedAt,
			"created_at":           c.CreatedAt,
			"updated_at":           c.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"configs": masked,
	})
}

// HandleGetConfig lấy thông tin chi tiết của 1 cấu hình để sửa (bao gồm credentials_json)
func (h *GoogleSheetHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		idStr = strings.TrimPrefix(r.URL.Path, "/api/integrations/sheets/configs/")
	}
	id, _ := strconv.Atoi(idStr)
	if id <= 0 {
		http.Error(w, "Thiếu ID cấu hình", http.StatusBadRequest)
		return
	}

	cfg, err := h.store.GetGoogleSheetConfig(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Không tìm thấy cấu hình: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"config":  cfg,
	})
}

// HandleSaveConfig thêm mới hoặc cập nhật cấu hình kết nối Google Sheet
func (h *GoogleSheetHandler) HandleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var cfg models.GoogleSheetConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu JSON không hợp lệ: " + err.Error(),
		})
		return
	}

	cfg.SpreadsheetID = googlesheets.ExtractSpreadsheetID(cfg.SpreadsheetID)
	if cfg.Name == "" {
		cfg.Name = "Google Sheet Sync"
	}
	if cfg.SheetTabName == "" {
		cfg.SheetTabName = "Trang tính1"
	}
	if len(cfg.FieldMappings) == 0 {
		cfg.FieldMappings = googlesheets.GetStandardDefaultMappings()
	}

	if cfg.ID > 0 {
		// Cập nhật
		// Nếu credentials_json rỗng trong request update, giữ nguyên credentials cũ
		if strings.TrimSpace(cfg.CredentialsJSON) == "" {
			old, err := h.store.GetGoogleSheetConfig(cfg.ID)
			if err == nil && old != nil {
				cfg.CredentialsJSON = old.CredentialsJSON
			}
		}
		if err := h.store.UpdateGoogleSheetConfig(&cfg); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Lỗi cập nhật cấu hình: " + err.Error(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Cập nhật cấu hình Google Sheet thành công!",
			"config":  cfg,
		})
		return
	}

	// Thêm mới
	if strings.TrimSpace(cfg.CredentialsJSON) == "" {
		if h.cfg.GoogleServiceAccountJSON != "" {
			cfg.CredentialsJSON = h.cfg.GoogleServiceAccountJSON
		} else if resolved, err := googlesheets.ResolveCredentials(""); err == nil {
			cfg.CredentialsJSON = resolved
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Chưa cấu hình Service Account Bot trên máy chủ.",
			})
			return
		}
	}

	created, err := h.store.CreateGoogleSheetConfig(&cfg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Lỗi lưu cấu hình Google Sheet: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Thêm mới cấu hình Google Sheet thành công!",
		"config":  created,
	})
}

// HandleDeleteConfig xóa một cấu hình Google Sheet
func (h *GoogleSheetHandler) HandleDeleteConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Chỉ chấp nhận POST hoặc DELETE", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ID <= 0 {
		idStr := r.URL.Query().Get("id")
		req.ID, _ = strconv.Atoi(idStr)
	}

	if req.ID <= 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "ID cấu hình không hợp lệ",
		})
		return
	}

	if err := h.store.DeleteGoogleSheetConfig(req.ID); err != nil {
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
		"message": "Đã xóa cấu hình kết nối Google Sheet thành công",
	})
}

// HandleToggleConfig bật hoặc tắt cấu hình đồng bộ
func (h *GoogleSheetHandler) HandleToggleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ chấp nhận POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       int  `json:"id"`
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Dữ liệu không hợp lệ",
		})
		return
	}

	if err := h.store.ToggleGoogleSheetConfig(req.ID, req.IsActive); err != nil {
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
		"success":   true,
		"id":        req.ID,
		"is_active": req.IsActive,
	})
}

// HandleListLogs xem danh sách lịch sử đồng bộ (Audit Logs)
func (h *GoogleSheetHandler) HandleListLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Chỉ chấp nhận GET", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	configID, _ := strconv.Atoi(q.Get("config_id"))

	if limit <= 0 {
		limit = 50
	}

	logs, total, err := h.store.ListSyncLogs(limit, offset, configID)
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
		"limit":   limit,
		"offset":  offset,
		"logs":    logs,
	})
}

// HandleGetDefaultMappings trả về danh sách các trường chuẩn sẵn có để người dùng chọn trên giao diện ánh xạ
func (h *GoogleSheetHandler) HandleGetDefaultMappings(w http.ResponseWriter, r *http.Request) {
	defaultMappings := googlesheets.GetStandardDefaultMappings()
	availableFields := []map[string]string{
		{"key": "thoi_gian", "label": "Thời Gian Đăng Ký"},
		{"key": "fullName", "label": "Họ Và Tên"},
		{"key": "so_dien_thoai", "label": "Số Điện Thoại"},
		{"key": "carrier", "label": "Nhà Mạng Viễn Thông"},
		{"key": "landing_id", "label": "ID Trang Landing"},
		{"key": "nguon", "label": "Nguồn Traffic (Kênh)"},
		{"key": "trang_thai", "label": "Trạng Thái Lead (Pipeline)"},
		{"key": "ghi_chu", "label": "Ghi Chú Tư Vấn"},
		{"key": "is_autofill", "label": "Tự Động Điền (1-Tap)"},
		{"key": "fill_duration_ms", "label": "Tốc Độ Điền Form (ms)"},
		{"key": "time_on_page", "label": "Thời Gian Xem Trang"},
		{"key": "max_scroll", "label": "Tỷ Lệ Cuộn Trang (%)"},
		{"key": "device_type", "label": "Thiết Bị (Mobile/Desktop)"},
		{"key": "ip_address", "label": "Địa Chỉ IP"},
		{"key": "utm_source", "label": "UTM Source"},
		{"key": "utm_medium", "label": "UTM Medium"},
		{"key": "utm_campaign", "label": "UTM Campaign"},
		{"key": "utm_content", "label": "UTM Content"},
		{"key": "utm_term", "label": "UTM Term"},
		{"key": "session_id", "label": "Mã Phiên (Session ID)"},
		{"key": "tracking_json", "label": "Toàn Bộ JSON Tracking"},
		{"key": "custom_text", "label": "Văn Bản Tùy Chọn Cố Định"},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"default_mappings": defaultMappings,
		"available_fields": availableFields,
	})
}
