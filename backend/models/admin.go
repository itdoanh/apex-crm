package models

import "time"

// AdminUser đại diện cho tài khoản quản trị duy nhất trong hệ thống
type AdminUser struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Không bao giờ trả hash mật khẩu ra API
	FullName     string    `json:"full_name"`
	Role         string    `json:"role"` // super_admin (full quyền)
	IsActive     bool      `json:"is_active"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// AdminSession đại diện cho phiên đăng nhập lâu dài và bảo mật cao (như Facebook)
type AdminSession struct {
	ID           int64     `json:"id"`
	AdminID      int       `json:"admin_id"`
	TokenHash    string    `json:"-"` // SHA-256 của token, không trả ra API
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	DeviceType   string    `json:"device_type"` // desktop, mobile, tablet
	Browser      string    `json:"browser"`     // Chrome, Safari, Firefox...
	OS           string    `json:"os"`          // Windows 11, iOS, macOS, Android...
	City         string    `json:"city"`
	Country      string    `json:"country"`
	IsRevoked    bool      `json:"is_revoked"`
	IsCurrent    bool      `json:"is_current,omitempty"` // Cờ đánh dấu thiết bị hiện tại của phiên này
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    string    `json:"created_at"`
	LastActiveAt string    `json:"last_active_at"`
}

// AdminAuditLog ghi nhận đầy đủ mọi thao tác quản trị vào hệ thống
type AdminAuditLog struct {
	ID         int64                  `json:"id"`
	AdminID    *int                   `json:"admin_id,omitempty"`
	Username   string                 `json:"username"`
	Action     string                 `json:"action"` // LOGIN_SUCCESS, UPDATE_LEAD, ADD_DOMAIN, ...
	Resource   string                 `json:"resource"` // leads, domains, landings, sheets, auth...
	ResourceID string                 `json:"resource_id,omitempty"`
	Details    map[string]interface{} `json:"details"`
	IPAddress  string                 `json:"ip_address"`
	UserAgent  string                 `json:"user_agent"`
	DeviceType string                 `json:"device_type"`
	Browser    string                 `json:"browser"`
	OS         string                 `json:"os"`
	Status     string                 `json:"status"` // success, failed, warning
	CreatedAt  string                 `json:"created_at"`
}

// LoginRequest dữ liệu gửi lên khi đăng nhập
type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"` // Mặc định true (lưu cookie lâu như Facebook)
}

// LoginResponse phản hồi khi đăng nhập thành công
type LoginResponse struct {
	Success   bool       `json:"success"`
	Message   string     `json:"message"`
	Token     string     `json:"token"` // Session token (lưu cả cookie & token header)
	Admin     *AdminUser `json:"admin"`
	ExpiresAt string     `json:"expires_at"`
}

// ChangePasswordRequest yêu cầu đổi mật khẩu admin
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}
