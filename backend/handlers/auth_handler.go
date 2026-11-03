package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/models"
	"apex-backend/security"
)

type contextKey string

const (
	AdminContextKey   contextKey = "current_admin"
	SessionContextKey contextKey = "current_session"
)

type AuthHandler struct {
	cfg   *config.Config
	store *db.Store
}

func NewAuthHandler(cfg *config.Config, store *db.Store) *AuthHandler {
	return &AuthHandler{
		cfg:   cfg,
		store: store,
	}
}

// ExtractClientIP trích xuất địa chỉ IP chính xác của client
func ExtractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		if host == "::1" || host == "[::1]" {
			return "127.0.0.1 (Localhost)"
		}
		return host
	}
	return r.RemoteAddr
}

// ExtractSessionToken lấy raw session token từ Cookie hoặc Authorization Header
func (h *AuthHandler) ExtractSessionToken(r *http.Request) string {
	// 1. Thử lấy từ Cookie
	if cookie, err := r.Cookie("apex_admin_session"); err == nil && cookie != nil && cookie.Value != "" {
		return strings.TrimSpace(cookie.Value)
	}

	// 2. Thử lấy từ Header Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}

	// 3. Thử lấy từ Header X-Admin-Session
	if customHeader := r.Header.Get("X-Admin-Session"); customHeader != "" {
		return strings.TrimSpace(customHeader)
	}

	return ""
}

// RequireAdminAuth Middleware bảo vệ tất cả endpoint nhạy cảm của Admin
func (h *AuthHandler) RequireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Cho phép preflight request OPTIONS vượt qua (CORS)
		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}

		rawToken := h.ExtractSessionToken(r)
		if rawToken == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "unauthorized",
				"message": "Vui lòng đăng nhập tài khoản quản trị để tiếp tục.",
				"code":    "AUTH_REQUIRED",
			})
			return
		}

		tokenHash := security.HashSessionToken(rawToken)
		sess, admin, err := h.store.GetSessionByTokenHash(tokenHash)
		if err != nil || sess == nil || admin == nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "unauthorized",
				"message": "Phiên đăng nhập đã hết hạn hoặc không hợp lệ. Vui lòng đăng nhập lại.",
				"code":    "SESSION_INVALID",
			})
			return
		}

		// Cập nhật ngầm thời điểm hoạt động gần nhất
		go func(sid int64) {
			_ = h.store.TouchSession(sid)
		}(sess.ID)

		// Lưu Admin và Session vào context để các handler sau có thể sử dụng ghi log
		ctx := context.WithValue(r.Context(), AdminContextKey, admin)
		ctx = context.WithValue(ctx, SessionContextKey, sess)

		next(w, r.WithContext(ctx))
	}
}

// HandleLogin xử lý đăng nhập tài khoản Admin duy nhất với mã băm Argon2id và cấp session cookie bền vững
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Dữ liệu đăng nhập không hợp lệ",
		})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)

	// Tìm tài khoản Admin
	admin, err := h.store.GetAdminByUsername(req.Username)
	if err != nil || admin == nil || !admin.IsActive {
		// Ghi nhận Audit Log đăng nhập thất bại
		_ = h.store.RecordAuditLog(&models.AdminAuditLog{
			Username:   req.Username,
			Action:     "AUTH_LOGIN_FAILED",
			Resource:   "auth",
			Status:     "failed",
			Details:    map[string]interface{}{"reason": "Tài khoản không tồn tại hoặc đã bị khóa", "attempted_username": req.Username},
			IPAddress:  clientIP,
			UserAgent:  ua,
			DeviceType: deviceType,
			Browser:    browser,
			OS:         osName,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Tên đăng nhập hoặc mật khẩu không chính xác",
		})
		return
	}

	// Xác minh mật khẩu bằng Argon2id
	match, err := security.VerifyPassword(req.Password, admin.PasswordHash)
	if err != nil || !match {
		// Ghi nhận Audit Log sai mật khẩu
		_ = h.store.RecordAuditLog(&models.AdminAuditLog{
			AdminID:    &admin.ID,
			Username:   admin.Username,
			Action:     "AUTH_LOGIN_FAILED",
			Resource:   "auth",
			Status:     "failed",
			Details:    map[string]interface{}{"reason": "Mật khẩu sai", "attempted_username": req.Username},
			IPAddress:  clientIP,
			UserAgent:  ua,
			DeviceType: deviceType,
			Browser:    browser,
			OS:         osName,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Tên đăng nhập hoặc mật khẩu không chính xác",
		})
		return
	}

	// Đăng nhập thành công -> Sinh token và lưu session
	rawToken, tokenHash, err := security.GenerateSessionToken()
	if err != nil {
		http.Error(w, "Lỗi tạo phiên đăng nhập", http.StatusInternalServerError)
		return
	}

	// Thời hạn lưu phiên đăng nhập: 180 ngày (Facebook persistent session)
	sessionDuration := 180 * 24 * time.Hour
	expiresAt := time.Now().Add(sessionDuration)

	session := &models.AdminSession{
		AdminID:    admin.ID,
		TokenHash:  tokenHash,
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
		ExpiresAt:  expiresAt,
	}

	if err := h.store.CreateSession(session); err != nil {
		http.Error(w, "Lỗi lưu phiên đăng nhập CSDL", http.StatusInternalServerError)
		return
	}

	// Thiết lập Cookie HttpOnly, SameSite Lax, Max-Age 180 ngày (như Facebook)
	isSecure := r.TLS != nil ||
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") ||
		strings.Contains(r.Header.Get("CF-Visitor"), "https")

	cookie := &http.Cookie{
		Name:     "apex_admin_session",
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	// Ghi nhận Audit Log đăng nhập thành công
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    &admin.ID,
		Username:   admin.Username,
		Action:     "AUTH_LOGIN_SUCCESS",
		Resource:   "auth",
		Status:     "success",
		Details:    map[string]interface{}{"session_id": session.ID, "expires_at": expiresAt.Format(time.RFC3339), "device": deviceType + " - " + browser + " trên " + osName},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.LoginResponse{
		Success:   true,
		Message:   "Đăng nhập thành công",
		Token:     rawToken,
		Admin:     admin,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

// HandleGetMe trả về thông tin Admin hiện tại và phiên đăng nhập
func (h *AuthHandler) HandleGetMe(w http.ResponseWriter, r *http.Request) {
	rawToken := h.ExtractSessionToken(r)
	if rawToken == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	tokenHash := security.HashSessionToken(rawToken)
	sess, admin, err := h.store.GetSessionByTokenHash(tokenHash)
	if err != nil || sess == nil || admin == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	sess.IsCurrent = true

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"authenticated": true,
		"admin":         admin,
		"session":       sess,
	})
}

// HandleLogout xử lý đăng xuất an toàn (thu hồi session hiện tại và xóa cookie)
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	rawToken := h.ExtractSessionToken(r)
	if rawToken != "" {
		tokenHash := security.HashSessionToken(rawToken)
		sess, admin, _ := h.store.GetSessionByTokenHash(tokenHash)
		if sess != nil {
			_ = h.store.RevokeSession(sess.ID)

			clientIP := ExtractClientIP(r)
			ua := r.UserAgent()
			deviceType, browser, osName := security.ParseUserAgent(ua)

			var adminID *int
			var username string
			if admin != nil {
				adminID = &admin.ID
				username = admin.Username
			}

			_ = h.store.RecordAuditLog(&models.AdminAuditLog{
				AdminID:    adminID,
				Username:   username,
				Action:     "AUTH_LOGOUT",
				Resource:   "auth",
				Status:     "success",
				Details:    map[string]interface{}{"session_id": sess.ID, "message": "Quản trị viên đã chủ động đăng xuất"},
				IPAddress:  clientIP,
				UserAgent:  ua,
				DeviceType: deviceType,
				Browser:    browser,
				OS:         osName,
			})
		}
	}

	// Xóa cookie apex_admin_session
	isSecure := r.TLS != nil ||
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") ||
		strings.Contains(r.Header.Get("CF-Visitor"), "https")

	http.SetCookie(w, &http.Cookie{
		Name:     "apex_admin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã đăng xuất an toàn khỏi hệ thống",
	})
}

// HandleChangePassword đổi mật khẩu cho tài khoản Admin duy nhất (hash bằng Argon2id)
func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser)
	if !ok || admin == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if len(req.NewPassword) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Mật khẩu mới phải có độ dài tối thiểu 8 ký tự"})
		return
	}

	// Lấy lại admin từ DB để có password hash mới nhất
	freshAdmin, err := h.store.GetAdminByID(admin.ID)
	if err != nil {
		http.Error(w, "Không tìm thấy tài khoản admin", http.StatusInternalServerError)
		return
	}

	// Xác minh mật khẩu cũ
	match, err := security.VerifyPassword(req.CurrentPassword, freshAdmin.PasswordHash)
	if err != nil || !match {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Mật khẩu hiện tại không chính xác"})
		return
	}

	// Băm mật khẩu mới bằng Argon2id
	newHash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "Lỗi mã hóa mật khẩu mới", http.StatusInternalServerError)
		return
	}

	if err := h.store.UpdateAdminPassword(admin.ID, newHash); err != nil {
		http.Error(w, "Lỗi cập nhật mật khẩu CSDL", http.StatusInternalServerError)
		return
	}

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)

	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    &admin.ID,
		Username:   admin.Username,
		Action:     "UPDATE_PASSWORD",
		Resource:   "auth",
		Status:     "success",
		Details:    map[string]interface{}{"message": "Quản trị viên đã thay đổi mật khẩu hệ thống (Argon2id)"},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã đổi mật khẩu thành công bằng mã hóa Argon2id!",
	})
}

// HandleListSessions danh sách các thiết bị/phiên đang đăng nhập
func (h *AuthHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser)
	currentSess, _ := r.Context().Value(SessionContextKey).(*models.AdminSession)
	if !ok || admin == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var currentID int64
	if currentSess != nil {
		currentID = currentSess.ID
	}

	sessions, err := h.store.ListActiveSessions(admin.ID, currentID)
	if err != nil {
		http.Error(w, "Lỗi lấy danh sách phiên: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"data":     sessions,
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// HandleRevokeSession thu hồi một phiên đăng nhập (đăng xuất từ xa thiết bị khác)
func (h *AuthHandler) HandleRevokeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser)
	if !ok || admin == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		SessionID int64 `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID <= 0 {
		http.Error(w, "Session ID không hợp lệ", http.StatusBadRequest)
		return
	}

	if err := h.store.RevokeSession(req.SessionID); err != nil {
		http.Error(w, "Lỗi thu hồi phiên: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)

	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    &admin.ID,
		Username:   admin.Username,
		Action:     "REVOKE_SESSION",
		Resource:   "auth",
		Status:     "success",
		Details:    map[string]interface{}{"revoked_session_id": req.SessionID},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã đăng xuất thiết bị thành công",
	})
}

// HandleRevokeOtherSessions đăng xuất khỏi tất cả các thiết bị khác (bảo mật Facebook)
func (h *AuthHandler) HandleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser)
	currentSess, _ := r.Context().Value(SessionContextKey).(*models.AdminSession)
	if !ok || admin == nil || currentSess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	count, err := h.store.RevokeAllOtherSessions(admin.ID, currentSess.ID)
	if err != nil {
		http.Error(w, "Lỗi đăng xuất các thiết bị khác: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)

	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    &admin.ID,
		Username:   admin.Username,
		Action:     "REVOKE_ALL_OTHER_SESSIONS",
		Resource:   "auth",
		Status:     "success",
		Details:    map[string]interface{}{"revoked_count": count},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   count,
		"message": "Đã đăng xuất thành công khỏi tất cả các thiết bị khác",
	})
}

// HandleListAuditLogs lấy danh sách nhật ký hoạt động có lọc và phân trang
func (h *AuthHandler) HandleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	action := q.Get("action")
	username := q.Get("username")
	search := q.Get("search")

	logs, total, err := h.store.ListAuditLogs(limit, offset, action, username, search)
	if err != nil {
		http.Error(w, "Lỗi lấy audit logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    logs,
		"logs":    logs,
		"total":   total,
	})
}
