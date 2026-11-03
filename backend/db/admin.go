package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"apex-backend/models"
	"apex-backend/security"
)

// SeedDefaultAdmin kiểm tra và khởi tạo duy nhất 1 tài khoản Admin nếu hệ thống chưa có
// Đảm bảo nguyên tắc: Không có đăng ký tự do, chỉ có duy nhất 1 tài khoản full quyền
func (s *Store) SeedDefaultAdmin(ctx context.Context, defaultUser, defaultPass string) error {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count)
	if err != nil {
		return fmt.Errorf("lỗi kiểm tra số lượng admin: %w", err)
	}

	if count > 0 {
		return nil
	}

	if defaultUser == "" {
		defaultUser = "admin"
	}
	if defaultPass == "" {
		defaultPass = "ApexAdmin@2026!"
	}

	// Băm mật khẩu bằng Argon2id chuẩn OWASP
	hash, err := security.HashPassword(defaultPass)
	if err != nil {
		return fmt.Errorf("lỗi băm mật khẩu Argon2id: %w", err)
	}

	query := `
		INSERT INTO admin_users (username, password_hash, full_name, role, is_active)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT (username) DO NOTHING;
	`
	_, err = s.pool.Exec(ctx, query, strings.ToLower(strings.TrimSpace(defaultUser)), hash, "Administrator", "super_admin")
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo tài khoản admin mặc định: %w", err)
	}

	log.Printf("[SECURITY] [ADMIN] Đã khởi tạo duy nhất 1 tài khoản quản trị tối cao '%s' với mật khẩu băm Argon2id", defaultUser)
	return nil
}

// GetAdminByUsername lấy thông tin tài khoản theo username (bao gồm cả password_hash để đối chiếu khi login)
func (s *Store) GetAdminByUsername(username string) (*models.AdminUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, username, password_hash, full_name, role, is_active,
		       to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
		       to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') AS updated_at
		FROM admin_users
		WHERE LOWER(username) = LOWER($1);
	`
	var u models.AdminUser
	err := s.pool.QueryRow(ctx, query, strings.TrimSpace(username)).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetAdminByID lấy thông tin tài khoản theo ID
func (s *Store) GetAdminByID(id int) (*models.AdminUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, username, password_hash, full_name, role, is_active,
		       to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
		       to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') AS updated_at
		FROM admin_users
		WHERE id = $1;
	`
	var u models.AdminUser
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateAdminPassword cập nhật mật khẩu mới (đã hash bằng Argon2id) cho tài khoản admin
func (s *Store) UpdateAdminPassword(adminID int, newHash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE admin_users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2;
	`
	_, err := s.pool.Exec(ctx, query, newHash, adminID)
	return err
}

// CreateSession tạo phiên đăng nhập mới trong CSDL
func (s *Store) CreateSession(session *models.AdminSession) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO admin_sessions (admin_id, token_hash, ip_address, user_agent, device_type, browser, os, city, country, is_revoked, expires_at, created_at, last_active_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, $10, NOW(), NOW())
		RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'), to_char(last_active_at, 'YYYY-MM-DD HH24:MI:SS');
	`
	return s.pool.QueryRow(ctx, query,
		session.AdminID,
		session.TokenHash,
		session.IPAddress,
		session.UserAgent,
		session.DeviceType,
		session.Browser,
		session.OS,
		session.City,
		session.Country,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt, &session.LastActiveAt)
}

// GetSessionByTokenHash kiểm tra session token hash, trả về session và admin nếu còn hạn và chưa bị thu hồi
func (s *Store) GetSessionByTokenHash(tokenHash string) (*models.AdminSession, *models.AdminUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			s.id, s.admin_id, s.token_hash, s.ip_address, s.user_agent, s.device_type, s.browser, s.os,
			s.city, s.country, s.is_revoked, s.expires_at,
			to_char(s.created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
			to_char(s.last_active_at, 'YYYY-MM-DD HH24:MI:SS') AS last_active_at,
			u.id, u.username, u.full_name, u.role, u.is_active,
			to_char(u.created_at, 'YYYY-MM-DD HH24:MI:SS') AS u_created_at,
			to_char(u.updated_at, 'YYYY-MM-DD HH24:MI:SS') AS u_updated_at
		FROM admin_sessions s
		JOIN admin_users u ON u.id = s.admin_id
		WHERE s.token_hash = $1 AND s.is_revoked = false AND s.expires_at > NOW() AND u.is_active = true;
	`
	var sess models.AdminSession
	var u models.AdminUser
	err := s.pool.QueryRow(ctx, query, tokenHash).Scan(
		&sess.ID,
		&sess.AdminID,
		&sess.TokenHash,
		&sess.IPAddress,
		&sess.UserAgent,
		&sess.DeviceType,
		&sess.Browser,
		&sess.OS,
		&sess.City,
		&sess.Country,
		&sess.IsRevoked,
		&sess.ExpiresAt,
		&sess.CreatedAt,
		&sess.LastActiveAt,
		&u.ID,
		&u.Username,
		&u.FullName,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}
	return &sess, &u, nil
}

// TouchSession cập nhật thời gian hoạt động gần nhất của session
func (s *Store) TouchSession(sessionID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE admin_sessions
		SET last_active_at = NOW()
		WHERE id = $1;
	`
	_, err := s.pool.Exec(ctx, query, sessionID)
	return err
}

// RevokeSession thu hồi/đăng xuất một session cụ thể
func (s *Store) RevokeSession(sessionID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE admin_sessions
		SET is_revoked = true
		WHERE id = $1;
	`
	_, err := s.pool.Exec(ctx, query, sessionID)
	return err
}

// RevokeAllOtherSessions đăng xuất khỏi tất cả các thiết bị khác (như bảo mật Facebook)
func (s *Store) RevokeAllOtherSessions(adminID int, currentSessionID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE admin_sessions
		SET is_revoked = true
		WHERE admin_id = $1 AND id <> $2 AND is_revoked = false;
	`
	ct, err := s.pool.Exec(ctx, query, adminID, currentSessionID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// ListActiveSessions danh sách các phiên đăng nhập đang hoạt động của Admin
func (s *Store) ListActiveSessions(adminID int, currentSessionID int64) ([]models.AdminSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			id, admin_id, ip_address, user_agent, device_type, browser, os,
			city, country, is_revoked, expires_at,
			to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
			to_char(last_active_at, 'YYYY-MM-DD HH24:MI:SS') AS last_active_at
		FROM admin_sessions
		WHERE admin_id = $1 AND is_revoked = false AND expires_at > NOW()
		ORDER BY (id = $2) DESC, last_active_at DESC;
	`
	rows, err := s.pool.Query(ctx, query, adminID, currentSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]models.AdminSession, 0)
	for rows.Next() {
		var sess models.AdminSession
		if err := rows.Scan(
			&sess.ID,
			&sess.AdminID,
			&sess.IPAddress,
			&sess.UserAgent,
			&sess.DeviceType,
			&sess.Browser,
			&sess.OS,
			&sess.City,
			&sess.Country,
			&sess.IsRevoked,
			&sess.ExpiresAt,
			&sess.CreatedAt,
			&sess.LastActiveAt,
		); err != nil {
			return nil, err
		}
		if sess.ID == currentSessionID {
			sess.IsCurrent = true
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// RecordAuditLog ghi nhận 1 hành động quản trị vào nhật ký hệ thống
func (s *Store) RecordAuditLog(logEntry *models.AdminAuditLog) error {
	if logEntry == nil {
		return errors.New("log entry rỗng")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	detailsBytes, err := json.Marshal(logEntry.Details)
	if err != nil {
		detailsBytes = []byte("{}")
	}

	query := `
		INSERT INTO admin_audit_logs (
			admin_id, username, action, resource, resource_id, details,
			ip_address, user_agent, device_type, browser, os, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS');
	`
	return s.pool.QueryRow(ctx, query,
		logEntry.AdminID,
		logEntry.Username,
		logEntry.Action,
		logEntry.Resource,
		logEntry.ResourceID,
		detailsBytes,
		logEntry.IPAddress,
		logEntry.UserAgent,
		logEntry.DeviceType,
		logEntry.Browser,
		logEntry.OS,
		logEntry.Status,
	).Scan(&logEntry.ID, &logEntry.CreatedAt)
}

// ListAuditLogs lấy danh sách nhật ký hoạt động có phân trang và lọc theo hành động, username hoặc từ khóa
func (s *Store) ListAuditLogs(limit, offset int, action, username, search string) ([]models.AdminAuditLog, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	whereClauses := []string{"1=1"}
	args := make([]interface{}, 0)
	argIdx := 1

	if action != "" && action != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, action)
		argIdx++
	}

	if username != "" && username != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("LOWER(username) = LOWER($%d)", argIdx))
		args = append(args, username)
		argIdx++
	}

	if search != "" {
		likeTerm := "%" + strings.ToLower(search) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(LOWER(action) LIKE $%d OR LOWER(resource) LIKE $%d OR LOWER(ip_address) LIKE $%d OR LOWER(browser) LIKE $%d OR details::text ILIKE $%d)", argIdx, argIdx, argIdx, argIdx, argIdx))
		args = append(args, likeTerm)
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Đếm tổng số bản ghi
	var totalCount int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM admin_audit_logs WHERE %s;", whereSQL)
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi đếm tổng audit logs: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			id, admin_id, username, action, resource, resource_id, details,
			ip_address, user_agent, device_type, browser, os, status,
			to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at
		FROM admin_audit_logs
		WHERE %s
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d;
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi truy vấn audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]models.AdminAuditLog, 0)
	for rows.Next() {
		var l models.AdminAuditLog
		var rawDetails []byte
		if err := rows.Scan(
			&l.ID,
			&l.AdminID,
			&l.Username,
			&l.Action,
			&l.Resource,
			&l.ResourceID,
			&rawDetails,
			&l.IPAddress,
			&l.UserAgent,
			&l.DeviceType,
			&l.Browser,
			&l.OS,
			&l.Status,
			&l.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if len(rawDetails) > 0 {
			_ = json.Unmarshal(rawDetails, &l.Details)
		}
		logs = append(logs, l)
	}

	return logs, totalCount, nil
}
