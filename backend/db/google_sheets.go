package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"apex-backend/models"

	"github.com/jackc/pgx/v5"
)

// CreateGoogleSheetConfig tạo một cấu hình kết nối Google Sheet mới
func (s *Store) CreateGoogleSheetConfig(cfg *models.GoogleSheetConfig) (*models.GoogleSheetConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mappingsJSON, err := json.Marshal(cfg.FieldMappings)
	if err != nil {
		mappingsJSON = []byte("[]")
	}

	query := `
		INSERT INTO google_sheet_configs (
			name, auth_type, credentials_json, spreadsheet_id, spreadsheet_name,
			sheet_tab_name, field_mappings, filter_landing_id, filter_nguon,
			is_active, auto_create_header, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, NOW(), NOW()
		) RETURNING id, TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		            TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS');
	`

	err = s.pool.QueryRow(ctx, query,
		cfg.Name,
		cfg.AuthType,
		cfg.CredentialsJSON,
		cfg.SpreadsheetID,
		cfg.SpreadsheetName,
		cfg.SheetTabName,
		mappingsJSON,
		cfg.FilterLandingID,
		cfg.FilterNguon,
		cfg.IsActive,
		cfg.AutoCreateHeader,
	).Scan(&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("lỗi thêm cấu hình Google Sheet: %w", err)
	}

	return cfg, nil
}

// UpdateGoogleSheetConfig cập nhật cấu hình Google Sheet đã có
func (s *Store) UpdateGoogleSheetConfig(cfg *models.GoogleSheetConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mappingsJSON, err := json.Marshal(cfg.FieldMappings)
	if err != nil {
		mappingsJSON = []byte("[]")
	}

	query := `
		UPDATE google_sheet_configs SET
			name = $1,
			auth_type = $2,
			credentials_json = $3,
			spreadsheet_id = $4,
			spreadsheet_name = $5,
			sheet_tab_name = $6,
			field_mappings = $7,
			filter_landing_id = $8,
			filter_nguon = $9,
			is_active = $10,
			auto_create_header = $11,
			updated_at = NOW()
		WHERE id = $12;
	`

	res, err := s.pool.Exec(ctx, query,
		cfg.Name,
		cfg.AuthType,
		cfg.CredentialsJSON,
		cfg.SpreadsheetID,
		cfg.SpreadsheetName,
		cfg.SheetTabName,
		mappingsJSON,
		cfg.FilterLandingID,
		cfg.FilterNguon,
		cfg.IsActive,
		cfg.AutoCreateHeader,
		cfg.ID,
	)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật cấu hình Google Sheet: %w", err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy cấu hình Google Sheet ID=%d", cfg.ID)
	}

	return nil
}

// DeleteGoogleSheetConfig xóa cấu hình Google Sheet (tự động cascade xóa sync logs)
func (s *Store) DeleteGoogleSheetConfig(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "DELETE FROM google_sheet_configs WHERE id = $1", id)
	return err
}

// ToggleGoogleSheetConfig bật/tắt trạng thái hoạt động của cấu hình
func (s *Store) ToggleGoogleSheetConfig(id int, isActive bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "UPDATE google_sheet_configs SET is_active = $1, updated_at = NOW() WHERE id = $2", isActive, id)
	return err
}

// GetGoogleSheetConfig lấy thông tin chi tiết một cấu hình theo ID
func (s *Store) GetGoogleSheetConfig(id int) (*models.GoogleSheetConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, name, auth_type, credentials_json, spreadsheet_id, spreadsheet_name,
		       sheet_tab_name, field_mappings::text, filter_landing_id, filter_nguon,
		       is_active, auto_create_header,
		       TO_CHAR(last_synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM google_sheet_configs
		WHERE id = $1;
	`

	var cfg models.GoogleSheetConfig
	var mappingsStr string
	var lastSynced *string

	err := s.pool.QueryRow(ctx, query, id).Scan(
		&cfg.ID, &cfg.Name, &cfg.AuthType, &cfg.CredentialsJSON,
		&cfg.SpreadsheetID, &cfg.SpreadsheetName, &cfg.SheetTabName,
		&mappingsStr, &cfg.FilterLandingID, &cfg.FilterNguon,
		&cfg.IsActive, &cfg.AutoCreateHeader, &lastSynced,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	cfg.LastSyncedAt = lastSynced
	if mappingsStr != "" {
		_ = json.Unmarshal([]byte(mappingsStr), &cfg.FieldMappings)
	}
	if cfg.FieldMappings == nil {
		cfg.FieldMappings = []models.FieldMapping{}
	}

	return &cfg, nil
}

// ListGoogleSheetConfigs trả về toàn bộ danh sách cấu hình kết nối Google Sheet
func (s *Store) ListGoogleSheetConfigs() ([]models.GoogleSheetConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, name, auth_type, credentials_json, spreadsheet_id, spreadsheet_name,
		       sheet_tab_name, field_mappings::text, filter_landing_id, filter_nguon,
		       is_active, auto_create_header,
		       TO_CHAR(last_synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM google_sheet_configs
		ORDER BY id ASC;
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("lỗi đọc danh sách cấu hình Google Sheet: %w", err)
	}
	defer rows.Close()

	configs, err := scanSheetConfigRows(rows)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// scanSheetConfigRows quét danh sách bản ghi GoogleSheetConfig từ pgx.Rows
func scanSheetConfigRows(rows pgx.Rows) ([]models.GoogleSheetConfig, error) {
	configs := make([]models.GoogleSheetConfig, 0)
	for rows.Next() {
		var cfg models.GoogleSheetConfig
		var mappingsStr string
		var lastSynced *string

		if err := rows.Scan(
			&cfg.ID, &cfg.Name, &cfg.AuthType, &cfg.CredentialsJSON,
			&cfg.SpreadsheetID, &cfg.SpreadsheetName, &cfg.SheetTabName,
			&mappingsStr, &cfg.FilterLandingID, &cfg.FilterNguon,
			&cfg.IsActive, &cfg.AutoCreateHeader, &lastSynced,
			&cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, err
		}

		cfg.LastSyncedAt = lastSynced
		if mappingsStr != "" {
			_ = json.Unmarshal([]byte(mappingsStr), &cfg.FieldMappings)
		}
		if cfg.FieldMappings == nil {
			cfg.FieldMappings = []models.FieldMapping{}
		}

		configs = append(configs, cfg)
	}

	return configs, nil
}

// GetActiveGoogleSheetConfigsForLead tìm các cấu hình Google Sheet đang kích hoạt và phù hợp với landingID + nguồn của Lead
// targetSheetIDs: danh sách ID bảng tính cụ thể được gán riêng cho Landing Page (nếu có)
// syncMode: 'selected' (chỉ các sheet trong targetSheetIDs), 'all' (tất cả sheet hợp lệ), 'disabled' (tắt đồng bộ)
func (s *Store) GetActiveGoogleSheetConfigsForLead(landingID, nguon string, targetSheetIDs []int, syncMode string) ([]models.GoogleSheetConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Nếu landing page chủ động tắt đồng bộ Google Sheet
	if syncMode == "disabled" {
		return []models.GoogleSheetConfig{}, nil
	}

	// 2. Nếu landing page chỉ định 1 hoặc nhiều Google Sheet cụ thể (hoặc syncMode == "selected" với targetSheetIDs > 0)
	//
	// ⚠️ CHỈ đồng bộ sang sheet được landing gán — nhưng vẫn phải TÔN TRỌNG bộ
	//    lọc riêng của sheet (`filter_landing_id`). Nếu sheet có `filter_landing_id`
	//    khác landing hiện tại ⇒ KHÔNG gửi (chống lead tràn chéo landing).
	if len(targetSheetIDs) > 0 && syncMode != "all" {
		query := `
			SELECT id, name, auth_type, credentials_json, spreadsheet_id, spreadsheet_name,
			       sheet_tab_name, field_mappings::text, filter_landing_id, filter_nguon,
			       is_active, auto_create_header,
			       TO_CHAR(last_synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
			       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
			       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
			FROM google_sheet_configs
			WHERE is_active = TRUE
			  AND id = ANY($1)
			  AND (filter_landing_id = '' OR filter_landing_id = $2)
			  AND (filter_nguon = '' OR filter_nguon = $3)
			ORDER BY id ASC;
		`
		rows, err := s.pool.Query(ctx, query, targetSheetIDs, landingID, nguon)
		if err != nil {
			return nil, fmt.Errorf("lỗi tìm kiếm cấu hình Google Sheet theo targetSheetIDs: %w", err)
		}
		defer rows.Close()
		return scanSheetConfigRows(rows)
	}

	// 3. Nếu landing page chọn "Theo Sheet Đã Chọn" (selected) nhưng chưa tích chọn sheet nào -> không gửi bừa bãi
	if syncMode == "selected" && len(targetSheetIDs) == 0 {
		return []models.GoogleSheetConfig{}, nil
	}

	// 4. Mặc định hoặc 'all': Lọc theo filter_landing_id của từng sheet (chống gửi tràn lan sang sheet khác)
	query := `
		SELECT id, name, auth_type, credentials_json, spreadsheet_id, spreadsheet_name,
		       sheet_tab_name, field_mappings::text, filter_landing_id, filter_nguon,
		       is_active, auto_create_header,
		       TO_CHAR(last_synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       TO_CHAR(updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM google_sheet_configs
		WHERE is_active = TRUE
		  AND (
		       filter_landing_id = ''
		       OR filter_landing_id = $1
		       OR filter_landing_id LIKE '%' || $1 || '%'
		  )
		  AND (filter_nguon = '' OR filter_nguon = $2)
		ORDER BY id ASC;
	`

	rows, err := s.pool.Query(ctx, query, landingID, nguon)
	if err != nil {
		return nil, fmt.Errorf("lỗi tìm kiếm cấu hình Google Sheet hợp lệ: %w", err)
	}
	defer rows.Close()
	return scanSheetConfigRows(rows)
}

// UpdateConfigLastSynced cập nhật mốc thời gian đồng bộ thành công mới nhất của cấu hình
func (s *Store) UpdateConfigLastSynced(configID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "UPDATE google_sheet_configs SET last_synced_at = NOW() WHERE id = $1", configID)
	return err
}

// CreateSyncLog ghi lại nhật ký đẩy dòng lead sang Google Sheet
func (s *Store) CreateSyncLog(log *models.GoogleSheetSyncLog) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO google_sheet_sync_logs (
			config_id, lead_id, status, row_range, lead_phone, lead_name, error_message, synced_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, NOW()
		) RETURNING id, TO_CHAR(synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS');
	`

	return s.pool.QueryRow(ctx, query,
		log.ConfigID,
		log.LeadID,
		log.Status,
		log.RowRange,
		log.LeadPhone,
		log.LeadName,
		log.ErrorMessage,
	).Scan(&log.ID, &log.SyncedAt)
}

// ListSyncLogs lấy danh sách nhật ký đồng bộ có phân trang
func (s *Store) ListSyncLogs(limit int, offset int, configID int) ([]models.GoogleSheetSyncLog, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var total int64
	var countQuery string
	var listQuery string
	var args []interface{}

	if configID > 0 {
		countQuery = "SELECT COUNT(*) FROM google_sheet_sync_logs WHERE config_id = $1;"
		_ = s.pool.QueryRow(ctx, countQuery, configID).Scan(&total)

		listQuery = `
			SELECT l.id, l.config_id, c.name, COALESCE(l.lead_id, 0), l.status, l.row_range,
			       l.lead_phone, l.lead_name, l.error_message,
			       TO_CHAR(l.synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
			FROM google_sheet_sync_logs l
			JOIN google_sheet_configs c ON l.config_id = c.id
			WHERE l.config_id = $1
			ORDER BY l.id DESC
			LIMIT $2 OFFSET $3;
		`
		args = []interface{}{configID, limit, offset}
	} else {
		countQuery = "SELECT COUNT(*) FROM google_sheet_sync_logs;"
		_ = s.pool.QueryRow(ctx, countQuery).Scan(&total)

		listQuery = `
			SELECT l.id, l.config_id, c.name, COALESCE(l.lead_id, 0), l.status, l.row_range,
			       l.lead_phone, l.lead_name, l.error_message,
			       TO_CHAR(l.synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
			FROM google_sheet_sync_logs l
			JOIN google_sheet_configs c ON l.config_id = c.id
			ORDER BY l.id DESC
			LIMIT $1 OFFSET $2;
		`
		args = []interface{}{limit, offset}
	}

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi truy vấn sync logs: %w", err)
	}
	defer rows.Close()

	logs := make([]models.GoogleSheetSyncLog, 0)
	for rows.Next() {
		var item models.GoogleSheetSyncLog
		if err := rows.Scan(
			&item.ID, &item.ConfigID, &item.ConfigName, &item.LeadID,
			&item.Status, &item.RowRange, &item.LeadPhone, &item.LeadName,
			&item.ErrorMessage, &item.SyncedAt,
		); err != nil {
			return nil, 0, err
		}
		logs = append(logs, item)
	}

	return logs, total, nil
}
