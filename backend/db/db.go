package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"apex-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

// NewStore khởi tạo kết nối PostgreSQL connection pool và tự động tạo/cập nhật bảng
func NewStore(connString string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("cấu hình chuỗi kết nối PostgreSQL không hợp lệ: %w", err)
	}

	// Tối ưu connection pool
	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối PostgreSQL pool: %w", err)
	}

	// Kiểm tra kết nối
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("không thể ping tới PostgreSQL: %w", err)
	}

	store := &Store{pool: pool}

	// Khởi tạo/cập nhật schema
	if err := store.initSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("lỗi khởi tạo bảng CSDL: %w", err)
	}

	return store, nil
}

// initSchema tạo bảng leads với đầy đủ các cột và index tối ưu
func (s *Store) initSchema(ctx context.Context) error {
	schemaSQL := `
	CREATE TABLE IF NOT EXISTS leads (
		id BIGSERIAL PRIMARY KEY,
		thoi_gian TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		landing_id VARCHAR(100) NOT NULL DEFAULT '',
		nguon VARCHAR(100) NOT NULL DEFAULT '',
		so_dien_thoai VARCHAR(50) NOT NULL DEFAULT '',
		tracking JSONB NOT NULL DEFAULT '{}'::jsonb
	);

	-- Đảm bảo bổ sung cột nếu bảng đã tồn tại trước đó
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS landing_id VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS nguon VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS trang_thai VARCHAR(50) NOT NULL DEFAULT 'moi';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS ghi_chu TEXT NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS nguoi_phu_trach VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS cap_nhat_luc TIMESTAMPTZ NOT NULL DEFAULT NOW();
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS city VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS region VARCHAR(50) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS country VARCHAR(100) NOT NULL DEFAULT 'Việt Nam';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS isp VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS ad_platform VARCHAR(50) NOT NULL DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS ad_placement VARCHAR(100) NOT NULL DEFAULT '';
	-- URL trang khách vừa gửi form (landing + query string). Tách riêng khỏi tracking JSONB
	-- để đồng bộ sang Google Sheets bằng 1 field mapping cố định ("url").
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS url VARCHAR(1000) NOT NULL DEFAULT '';

	CREATE INDEX IF NOT EXISTS idx_leads_landing_id ON leads (landing_id);
	CREATE INDEX IF NOT EXISTS idx_leads_nguon ON leads (nguon);
	CREATE INDEX IF NOT EXISTS idx_leads_so_dien_thoai ON leads (so_dien_thoai);
	CREATE INDEX IF NOT EXISTS idx_leads_thoi_gian ON leads (thoi_gian DESC);
	CREATE INDEX IF NOT EXISTS idx_leads_trang_thai ON leads (trang_thai);
	CREATE INDEX IF NOT EXISTS idx_leads_city ON leads (city);
	CREATE INDEX IF NOT EXISTS idx_leads_region ON leads (region);
	CREATE INDEX IF NOT EXISTS idx_leads_ad_platform ON leads (ad_platform);
	CREATE INDEX IF NOT EXISTS idx_leads_ad_placement ON leads (ad_placement);
	CREATE INDEX IF NOT EXISTS idx_leads_tracking_gin ON leads USING gin (tracking);

	-- Bảng lưu trữ phiên truy cập & toàn bộ hành vi chi tiết (Analytics & Bot Detection)
	CREATE TABLE IF NOT EXISTS analytics_sessions (
		session_id VARCHAR(64) PRIMARY KEY,
		visitor_id VARCHAR(64) NOT NULL DEFAULT '',
		landing_id VARCHAR(100) NOT NULL DEFAULT '',
		nguon VARCHAR(100) NOT NULL DEFAULT '',
		utm_source VARCHAR(100) NOT NULL DEFAULT '',
		utm_medium VARCHAR(100) NOT NULL DEFAULT '',
		utm_campaign VARCHAR(100) NOT NULL DEFAULT '',
		utm_content VARCHAR(100) NOT NULL DEFAULT '',
		utm_term VARCHAR(100) NOT NULL DEFAULT '',
		referrer TEXT NOT NULL DEFAULT '',
		entry_url TEXT NOT NULL DEFAULT '',
		ip_address VARCHAR(50) NOT NULL DEFAULT '',
		city VARCHAR(100) NOT NULL DEFAULT '',
		region VARCHAR(50) NOT NULL DEFAULT '',
		country VARCHAR(100) NOT NULL DEFAULT 'Việt Nam',
		isp VARCHAR(100) NOT NULL DEFAULT '',
		ad_platform VARCHAR(50) NOT NULL DEFAULT '',
		ad_placement VARCHAR(100) NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT '',
		device_type VARCHAR(20) NOT NULL DEFAULT 'desktop',
		browser VARCHAR(50) NOT NULL DEFAULT '',
		os VARCHAR(50) NOT NULL DEFAULT '',
		screen_res VARCHAR(30) NOT NULL DEFAULT '',
		is_bot BOOLEAN NOT NULL DEFAULT FALSE,
		bot_score INT NOT NULL DEFAULT 0,
		bot_reasons TEXT NOT NULL DEFAULT '',
		duration_seconds INT NOT NULL DEFAULT 0,
		max_scroll_percent INT NOT NULL DEFAULT 0,
		total_clicks INT NOT NULL DEFAULT 0,
		has_interacted_form BOOLEAN NOT NULL DEFAULT FALSE,
		converted_lead BOOLEAN NOT NULL DEFAULT FALSE,
		so_dien_thoai VARCHAR(50) NOT NULL DEFAULT '',
		lead_id BIGINT DEFAULT NULL,
		actions JSONB NOT NULL DEFAULT '[]'::jsonb,
		first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS city VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS region VARCHAR(50) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS country VARCHAR(100) NOT NULL DEFAULT 'Việt Nam';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS isp VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS ad_platform VARCHAR(50) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS ad_placement VARCHAR(100) NOT NULL DEFAULT '';

	CREATE INDEX IF NOT EXISTS idx_sessions_landing_id ON analytics_sessions (landing_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_nguon ON analytics_sessions (nguon);
	CREATE INDEX IF NOT EXISTS idx_sessions_is_bot ON analytics_sessions (is_bot);
	CREATE INDEX IF NOT EXISTS idx_sessions_city ON analytics_sessions (city);
	CREATE INDEX IF NOT EXISTS idx_sessions_region ON analytics_sessions (region);
	CREATE INDEX IF NOT EXISTS idx_sessions_ad_platform ON analytics_sessions (ad_platform);
	CREATE INDEX IF NOT EXISTS idx_sessions_ad_placement ON analytics_sessions (ad_placement);
	CREATE INDEX IF NOT EXISTS idx_sessions_converted ON analytics_sessions (converted_lead);
	CREATE INDEX IF NOT EXISTS idx_sessions_first_seen ON analytics_sessions (first_seen DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_phone ON analytics_sessions (so_dien_thoai);
	CREATE INDEX IF NOT EXISTS idx_sessions_utm_campaign ON analytics_sessions (utm_campaign);
	CREATE INDEX IF NOT EXISTS idx_sessions_utm_content ON analytics_sessions (utm_content);
	CREATE INDEX IF NOT EXISTS idx_sessions_actions_gin ON analytics_sessions USING gin (actions);

	-- Bảng cách ly các request rác, spam bot và payload bất thường (Spam Quarantine)
	CREATE TABLE IF NOT EXISTS spam_quarantine (
		id BIGSERIAL PRIMARY KEY,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		ip_address VARCHAR(50) NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT '',
		landing_id VARCHAR(100) NOT NULL DEFAULT '',
		nguon VARCHAR(100) NOT NULL DEFAULT '',
		phone VARCHAR(50) NOT NULL DEFAULT '',
		reason VARCHAR(255) NOT NULL DEFAULT '',
		payload JSONB NOT NULL DEFAULT '{}'::jsonb
	);

	CREATE INDEX IF NOT EXISTS idx_quarantine_created_at ON spam_quarantine (created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_quarantine_ip ON spam_quarantine (ip_address);
	CREATE INDEX IF NOT EXISTS idx_quarantine_phone ON spam_quarantine (phone);
	CREATE INDEX IF NOT EXISTS idx_quarantine_reason ON spam_quarantine (reason);

	-- Bảng lưu cấu hình đồng bộ Google Sheets (hỗ trợ nhiều bảng tính & lọc linh hoạt)
	CREATE TABLE IF NOT EXISTS google_sheet_configs (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL DEFAULT '',
		auth_type VARCHAR(50) NOT NULL DEFAULT 'service_account',
		credentials_json TEXT NOT NULL DEFAULT '',
		spreadsheet_id VARCHAR(255) NOT NULL DEFAULT '',
		spreadsheet_name VARCHAR(255) NOT NULL DEFAULT '',
		sheet_tab_name VARCHAR(255) NOT NULL DEFAULT '',
		field_mappings JSONB NOT NULL DEFAULT '[]'::jsonb,
		filter_landing_id VARCHAR(100) NOT NULL DEFAULT '',
		filter_nguon VARCHAR(100) NOT NULL DEFAULT '',
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		auto_create_header BOOLEAN NOT NULL DEFAULT TRUE,
		last_synced_at TIMESTAMPTZ DEFAULT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_gs_configs_active ON google_sheet_configs (is_active);
	CREATE INDEX IF NOT EXISTS idx_gs_configs_filter_landing ON google_sheet_configs (filter_landing_id);
	CREATE INDEX IF NOT EXISTS idx_gs_configs_filter_nguon ON google_sheet_configs (filter_nguon);

	-- Bảng lưu lịch sử đồng bộ Google Sheets (Audit Logs)
	CREATE TABLE IF NOT EXISTS google_sheet_sync_logs (
		id BIGSERIAL PRIMARY KEY,
		config_id INT NOT NULL REFERENCES google_sheet_configs(id) ON DELETE CASCADE,
		lead_id BIGINT DEFAULT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'success',
		row_range VARCHAR(255) NOT NULL DEFAULT '',
		lead_phone VARCHAR(50) NOT NULL DEFAULT '',
		lead_name VARCHAR(255) NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_gs_sync_logs_config ON google_sheet_sync_logs (config_id);
	CREATE INDEX IF NOT EXISTS idx_gs_sync_logs_synced_at ON google_sheet_sync_logs (synced_at DESC);
	CREATE INDEX IF NOT EXISTS idx_gs_sync_logs_status ON google_sheet_sync_logs (status);
	CREATE INDEX IF NOT EXISTS idx_gs_sync_logs_phone ON google_sheet_sync_logs (lead_phone);

	-- Bảng cấu hình và quản lý Landing Pages (Dynamic Configuration & Feature Flags)
	CREATE TABLE IF NOT EXISTS landing_pages (
		page_id VARCHAR(100) NOT NULL,
		domain VARCHAR(255) NOT NULL DEFAULT '',
		name VARCHAR(255) NOT NULL DEFAULT '',
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		status VARCHAR(50) NOT NULL DEFAULT 'published',
		redirect_url VARCHAR(500) NOT NULL DEFAULT '',
		nguon_default VARCHAR(100) NOT NULL DEFAULT 'landing_chinh',
		settings JSONB NOT NULL DEFAULT '{}'::jsonb,
		code_html TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (domain, page_id)
	);

	CREATE INDEX IF NOT EXISTS idx_landings_page_id ON landing_pages (page_id);
	CREATE INDEX IF NOT EXISTS idx_landings_domain ON landing_pages (domain);
	CREATE INDEX IF NOT EXISTS idx_landings_is_active ON landing_pages (is_active);

	-- Bảng lưu trữ cây thư mục tệp tin (File Manager: index.html, style.css, main.js,...)
	CREATE TABLE IF NOT EXISTS landing_files (
		id SERIAL PRIMARY KEY,
		page_id VARCHAR(100) NOT NULL,
		domain VARCHAR(255) NOT NULL DEFAULT '',
		file_path VARCHAR(255) NOT NULL,
		file_type VARCHAR(50) NOT NULL DEFAULT 'text',
		content TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (domain, page_id, file_path)
	);

	CREATE INDEX IF NOT EXISTS idx_landing_files_page ON landing_files (domain, page_id);

	-- Bảng quản lý Tên Miền (Custom Domains & DNS Management)
	CREATE TABLE IF NOT EXISTS domains (
		id SERIAL PRIMARY KEY,
		domain VARCHAR(255) UNIQUE NOT NULL,
		ns1 VARCHAR(255) NOT NULL DEFAULT 'ns1.hanghoaphaisinh.net',
		ns2 VARCHAR(255) NOT NULL DEFAULT 'ns2.hanghoaphaisinh.net',
		target_ip VARCHAR(100) NOT NULL DEFAULT '103.124.95.232',
		dns_status VARCHAR(50) NOT NULL DEFAULT 'pending',
		ssl_status VARCHAR(50) NOT NULL DEFAULT 'active',
		default_landing_id VARCHAR(100) NOT NULL DEFAULT 'mxv_080_online',
		notes TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_domains_domain ON domains (domain);
	CREATE INDEX IF NOT EXISTS idx_domains_dns_status ON domains (dns_status);

	-- Bảng cấu hình định tuyến thư mục / đường dẫn trên từng tên miền (Multi-Domain Path Routing)
	CREATE TABLE IF NOT EXISTS domain_routes (
		id SERIAL PRIMARY KEY,
		domain_id INT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
		path_prefix VARCHAR(255) NOT NULL DEFAULT '/',
		landing_id VARCHAR(100) NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		redirect_url VARCHAR(500) NOT NULL DEFAULT '',
		deployed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (domain_id, path_prefix)
	);

	CREATE INDEX IF NOT EXISTS idx_domain_routes_lookup ON domain_routes (domain_id, path_prefix);
	CREATE INDEX IF NOT EXISTS idx_domain_routes_landing ON domain_routes (landing_id);

	-- Bảng Quản trị viên Hệ thống (Duy nhất 1 tài khoản với full quyền super_admin)
	CREATE TABLE IF NOT EXISTS admin_users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(100) UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		full_name VARCHAR(150) NOT NULL DEFAULT 'Administrator',
		role VARCHAR(50) NOT NULL DEFAULT 'super_admin',
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_admin_users_username ON admin_users (username);

	-- Bảng Quản lý Phiên Đăng Nhập Lâu Dài & Bảo Mật Cao (như Facebook persistent session)
	CREATE TABLE IF NOT EXISTS admin_sessions (
		id BIGSERIAL PRIMARY KEY,
		admin_id INT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
		token_hash VARCHAR(64) UNIQUE NOT NULL,
		ip_address VARCHAR(50) NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT '',
		device_type VARCHAR(50) NOT NULL DEFAULT 'desktop',
		browser VARCHAR(50) NOT NULL DEFAULT '',
		os VARCHAR(50) NOT NULL DEFAULT '',
		city VARCHAR(100) NOT NULL DEFAULT '',
		country VARCHAR(100) NOT NULL DEFAULT 'Việt Nam',
		is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
		expires_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_admin_sessions_token_hash ON admin_sessions (token_hash);
	CREATE INDEX IF NOT EXISTS idx_admin_sessions_admin_id ON admin_sessions (admin_id);
	CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at ON admin_sessions (expires_at);
	CREATE INDEX IF NOT EXISTS idx_admin_sessions_is_revoked ON admin_sessions (is_revoked);

	-- Bảng Nhật Ký Hoạt Động Hệ Thống (Audit Logs ghi nhận mọi hành vi, thiết bị, IP, thời gian)
	CREATE TABLE IF NOT EXISTS admin_audit_logs (
		id BIGSERIAL PRIMARY KEY,
		admin_id INT DEFAULT NULL,
		username VARCHAR(100) NOT NULL DEFAULT '',
		action VARCHAR(100) NOT NULL,
		resource VARCHAR(100) NOT NULL DEFAULT '',
		resource_id VARCHAR(100) NOT NULL DEFAULT '',
		details JSONB NOT NULL DEFAULT '{}'::jsonb,
		ip_address VARCHAR(50) NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT '',
		device_type VARCHAR(50) NOT NULL DEFAULT 'desktop',
		browser VARCHAR(50) NOT NULL DEFAULT '',
		os VARCHAR(50) NOT NULL DEFAULT '',
		status VARCHAR(20) NOT NULL DEFAULT 'success',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_created_at ON admin_audit_logs (created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_action ON admin_audit_logs (action);
	CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_username ON admin_audit_logs (username);
	CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_ip ON admin_audit_logs (ip_address);
	CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_details_gin ON admin_audit_logs USING gin (details);
	`
	_, err := s.pool.Exec(ctx, schemaSQL)
	return err
}

// Close đóng kết nối pool an toàn
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

// SaveLead lưu 1 lead mới vào PostgreSQL kèm thông tin landing_id, nguồn, URL trang gửi form và tracking JSONB.
//
// Trả về (lead, isNew, error):
//   - isNew = true  ⇒ đây là lead MỚI được tạo trong lần gọi này.
//   - isNew = false ⇒ lead đã tồn tại (trùng lặp), đã được HỢP NHẤT tracking và trả về bản ghi cũ.
//
// DEDUPLICATION (chống lead trùng):
//  1. TOÀN BỘ thao tác "kiểm tra trùng" + "insert" nằm trong MỘT transaction kèm
//     `pg_advisory_xact_lock(hashtextextended(phone))` ⇒ các request song song cùng
//     1 SĐT bị serialize lại, không thể cùng lúc "thấy chưa có" rồi cùng INSERT
//     (race condition TOCTOU) — đây là nguyên nhân sinh ra nhiều lead trùng nhau
//     cùng thời gian.
//  2. Ba tiêu chí coi là trùng (trong 24h):
//     - cùng SĐT + cùng landing_id  (quy tắc cũ, giữ nguyên)
//     - cùng SĐT trong 10 phút gần nhất bất kể landing (bắt trường hợp gửi 2 landing)
//     - cùng SĐT + cùng session_id (bắt trường hợp double-click / 2 tab cùng phiên)
//  3. Khi là lead trùng, KHÔNG tạo dòng mới ⇒ tầng handler chỉ đồng bộ Google Sheets
//     khi isNew = true, tránh đẩy nhiều dòng giống nhau xuống Sheet.
func (s *Store) SaveLead(landingID string, nguon string, phone string, pageURL string, trackingJSON []byte, city, region, country, isp, adPlatform, adPlacement string) (*models.Lead, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if len(trackingJSON) == 0 {
		trackingJSON = []byte("{}")
	}

	// Trích xuất từ trackingJSON nếu truyền vào rỗng
	var trackMap map[string]interface{}
	sessionID := ""
	if err := json.Unmarshal(trackingJSON, &trackMap); err == nil {
		if city == "" {
			if v, ok := trackMap["city"].(string); ok {
				city = v
			}
		}
		if region == "" {
			if v, ok := trackMap["region"].(string); ok {
				region = v
			}
		}
		if country == "" {
			if v, ok := trackMap["country"].(string); ok {
				country = v
			}
		}
		if isp == "" {
			if v, ok := trackMap["isp"].(string); ok {
				isp = v
			}
		}
		if adPlatform == "" {
			if v, ok := trackMap["ad_platform"].(string); ok {
				adPlatform = v
			}
		}
		if adPlacement == "" {
			if v, ok := trackMap["ad_placement"].(string); ok {
				adPlacement = v
			}
		}
		if pageURL == "" {
			for _, k := range []string{"url", "entry_url", "page_url"} {
				if v, ok := trackMap[k].(string); ok && strings.TrimSpace(v) != "" {
					pageURL = strings.TrimSpace(v)
					break
				}
			}
		}
		for _, k := range []string{"session_id", "sessionId", "apex_session_id"} {
			if v, ok := trackMap[k].(string); ok && strings.TrimSpace(v) != "" {
				sessionID = strings.TrimSpace(v)
				break
			}
		}
	}
	if country == "" {
		country = "Việt Nam"
	}

	// === BẮT ĐẦU TRANSACTION ===
	// Serialize theo SĐT: mọi request cùng 1 SĐT phải xếp hàng, tránh race
	// "cả hai cùng thấy chưa có lead ⇒ cùng INSERT" sinh lead trùng cùng thời gian.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("lỗi bắt đầu transaction lưu lead: %w", err)
	}
	// Rollback là no-op nếu transaction đã Commit thành công.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "apex_lead:"+phone); err != nil {
		return nil, false, fmt.Errorf("lỗi khoá advisory theo SĐT: %w", err)
	}

	// 1. Kiểm tra chống trùng lặp lead (Deduplication) trong vòng 24 giờ
	checkQuery := `
		SELECT id, TO_CHAR(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'), landing_id, nguon, so_dien_thoai,
		       COALESCE(trang_thai, 'moi'), COALESCE(ghi_chu, ''), COALESCE(nguoi_phu_trach, ''),
		       TO_CHAR(COALESCE(cap_nhat_luc, thoi_gian) AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(city, ''), COALESCE(region, ''), COALESCE(country, 'Việt Nam'), COALESCE(isp, ''),
		       COALESCE(ad_platform, ''), COALESCE(ad_placement, ''), COALESCE(url, ''),
		       tracking::text
		FROM leads
		WHERE so_dien_thoai = $1
		  AND thoi_gian >= NOW() - INTERVAL '24 hours'
		  AND (
		        ($2 <> '' AND landing_id = $2)
		     OR thoi_gian >= NOW() - INTERVAL '10 minutes'
		     OR ($3 <> '' AND tracking->>'session_id' = $3)
		      )
		ORDER BY thoi_gian DESC LIMIT 1;
	`
	var existing models.Lead
	var existingTrackingStr string
	err = tx.QueryRow(ctx, checkQuery, phone, landingID, sessionID).Scan(
		&existing.ID,
		&existing.CreatedAt,
		&existing.LandingID,
		&existing.Nguon,
		&existing.Phone,
		&existing.TrangThai,
		&existing.GhiChu,
		&existing.AssignedTo,
		&existing.UpdatedAt,
		&existing.City,
		&existing.Region,
		&existing.Country,
		&existing.ISP,
		&existing.AdPlatform,
		&existing.AdPlacement,
		&existing.URL,
		&existingTrackingStr,
	)
	if err == nil {
		// Đã đăng ký trong 24h qua: hợp nhất tracking data mới nhất vào bản ghi cũ và trả về
		updateQuery := `
			UPDATE leads
			SET tracking = tracking || $1::jsonb,
			    city = CASE WHEN $2 != '' THEN $2 ELSE leads.city END,
			    region = CASE WHEN $3 != '' THEN $3 ELSE leads.region END,
			    country = CASE WHEN $4 != '' THEN $4 ELSE leads.country END,
			    isp = CASE WHEN $5 != '' THEN $5 ELSE leads.isp END,
			    ad_platform = CASE WHEN $6 != '' THEN $6 ELSE leads.ad_platform END,
			    ad_placement = CASE WHEN $7 != '' THEN $7 ELSE leads.ad_placement END,
			    url = CASE WHEN $8 != '' THEN $8 ELSE leads.url END,
			    cap_nhat_luc = NOW()
			WHERE id = $9;
		`
		_, _ = tx.Exec(ctx, updateQuery, string(trackingJSON), city, region, country, isp, adPlatform, adPlacement, pageURL, existing.ID)
		existing.Tracking = json.RawMessage(existingTrackingStr)
		if city != "" {
			existing.City = city
		}
		if region != "" {
			existing.Region = region
		}
		if isp != "" {
			existing.ISP = isp
		}
		if adPlatform != "" {
			existing.AdPlatform = adPlatform
		}
		if adPlacement != "" {
			existing.AdPlacement = adPlacement
		}
		if pageURL != "" {
			existing.URL = pageURL
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("lỗi commit khi hợp nhất lead trùng lặp: %w", err)
		}
		return &existing, false, nil
	}
	if err != pgx.ErrNoRows {
		return nil, false, fmt.Errorf("lỗi kiểm tra lead trùng lặp: %w", err)
	}

	// 2. Lead mới hoàn toàn
	query := `
		INSERT INTO leads (landing_id, nguon, so_dien_thoai, url, tracking, trang_thai, ghi_chu, nguoi_phu_trach, city, region, country, isp, ad_platform, ad_placement, thoi_gian, cap_nhat_luc)
		VALUES ($1, $2, $3, $4, $5::jsonb, 'moi', '', '', $6, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING id, TO_CHAR(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'), landing_id, nguon, so_dien_thoai,
		          COALESCE(trang_thai, 'moi'), COALESCE(ghi_chu, ''), COALESCE(nguoi_phu_trach, ''),
		          TO_CHAR(COALESCE(cap_nhat_luc, thoi_gian) AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		          COALESCE(city, ''), COALESCE(region, ''), COALESCE(country, 'Việt Nam'), COALESCE(isp, ''),
		          COALESCE(ad_platform, ''), COALESCE(ad_placement, ''), COALESCE(url, ''),
		          tracking::text;
	`

	var lead models.Lead
	var trackingStr string

	err = tx.QueryRow(ctx, query, landingID, nguon, phone, pageURL, string(trackingJSON), city, region, country, isp, adPlatform, adPlacement).Scan(
		&lead.ID,
		&lead.CreatedAt,
		&lead.LandingID,
		&lead.Nguon,
		&lead.Phone,
		&lead.TrangThai,
		&lead.GhiChu,
		&lead.AssignedTo,
		&lead.UpdatedAt,
		&lead.City,
		&lead.Region,
		&lead.Country,
		&lead.ISP,
		&lead.AdPlatform,
		&lead.AdPlacement,
		&lead.URL,
		&trackingStr,
	)
	if err != nil {
		return nil, false, fmt.Errorf("lỗi ghi lead vào PostgreSQL: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("lỗi commit lưu lead: %w", err)
	}

	lead.Tracking = json.RawMessage(trackingStr)
	return &lead, true, nil
}

// buildWhereClause xây dựng câu lệnh WHERE động theo đa chiều các bộ lọc: tìm kiếm, landing, nguồn, trạng thái, ngày, giờ
func buildWhereClause(search string, landingID string, nguon string, status string, fromDate string, toDate string, fromHour int, toHour int) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(search) != "" {
		conditions = append(conditions, fmt.Sprintf("(so_dien_thoai ILIKE '%%' || $%d || '%%' OR ghi_chu ILIKE '%%' || $%d || '%%' OR tracking->>'fullName' ILIKE '%%' || $%d || '%%')", idx, idx, idx))
		args = append(args, strings.TrimSpace(search))
		idx++
	}

	if strings.TrimSpace(landingID) != "" {
		conditions = append(conditions, fmt.Sprintf("landing_id = $%d", idx))
		args = append(args, strings.TrimSpace(landingID))
		idx++
	}

	if strings.TrimSpace(nguon) != "" {
		conditions = append(conditions, fmt.Sprintf("nguon = $%d", idx))
		args = append(args, strings.TrimSpace(nguon))
		idx++
	}

	if strings.TrimSpace(status) != "" {
		conditions = append(conditions, fmt.Sprintf("trang_thai = $%d", idx))
		args = append(args, strings.TrimSpace(status))
		idx++
	}

	if strings.TrimSpace(fromDate) != "" {
		fDate := strings.TrimSpace(fromDate)
		if len(fDate) == 10 {
			fDate += " 00:00:00"
		}
		conditions = append(conditions, fmt.Sprintf("(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh') >= $%d::timestamp", idx))
		args = append(args, fDate)
		idx++
	}

	if strings.TrimSpace(toDate) != "" {
		tDate := strings.TrimSpace(toDate)
		if len(tDate) == 10 {
			tDate += " 23:59:59"
		}
		conditions = append(conditions, fmt.Sprintf("(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh') <= $%d::timestamp", idx))
		args = append(args, tDate)
		idx++
	}

	if fromHour >= 0 && toHour >= 0 && toHour <= 23 && fromHour <= toHour {
		conditions = append(conditions, fmt.Sprintf("EXTRACT(HOUR FROM thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::int BETWEEN $%d AND $%d", idx, idx+1))
		args = append(args, fromHour, toHour)
		idx += 2
	}

	if len(conditions) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// ListLeads lấy danh sách leads có phân trang và lọc đa chiều theo SĐT/Tên, landing_id, nguồn, trạng thái, ngày, giờ
func (s *Store) ListLeads(page int, limit int, search string, landingID string, nguon string, status string, fromDate string, toDate string, fromHour int, toHour int) ([]models.Lead, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit

	whereClause, args := buildWhereClause(search, landingID, nguon, status, fromDate, toDate, fromHour, toHour)

	// Đếm tổng số bản ghi khớp bộ lọc
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM leads %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("lỗi đếm leads: %w", err)
	}

	// Lấy danh sách leads
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	selectArgs := append(args, limit, offset)

	selectQuery := fmt.Sprintf(`
		SELECT id, TO_CHAR(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'), landing_id, nguon, so_dien_thoai,
		       COALESCE(trang_thai, 'moi'), COALESCE(ghi_chu, ''), COALESCE(nguoi_phu_trach, ''),
		       TO_CHAR(COALESCE(cap_nhat_luc, thoi_gian) AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(city, ''), COALESCE(region, ''), COALESCE(country, 'Việt Nam'), COALESCE(isp, ''),
		       COALESCE(ad_platform, ''), COALESCE(ad_placement, ''), COALESCE(url, ''),
		       tracking::text
		FROM leads
		%s
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, limitIdx, offsetIdx)

	rows, err := s.pool.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi truy vấn danh sách leads: %w", err)
	}
	defer rows.Close()

	leads := make([]models.Lead, 0)
	for rows.Next() {
		var lead models.Lead
		var trackingStr string
		if err := rows.Scan(
			&lead.ID,
			&lead.CreatedAt,
			&lead.LandingID,
			&lead.Nguon,
			&lead.Phone,
			&lead.TrangThai,
			&lead.GhiChu,
			&lead.AssignedTo,
			&lead.UpdatedAt,
			&lead.City,
			&lead.Region,
			&lead.Country,
			&lead.ISP,
			&lead.AdPlatform,
			&lead.AdPlacement,
			&lead.URL,
			&trackingStr,
		); err != nil {
			return nil, 0, fmt.Errorf("lỗi đọc dòng lead: %w", err)
		}
		lead.Tracking = json.RawMessage(trackingStr)
		leads = append(leads, lead)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("lỗi con trỏ dòng: %w", err)
	}

	return leads, total, nil
}

// GetAllLeads lấy toàn bộ leads theo bộ lọc phục vụ xuất file CSV / Excel
func (s *Store) GetAllLeads(search string, landingID string, nguon string, status string, fromDate string, toDate string, fromHour int, toHour int) ([]models.Lead, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	whereClause, args := buildWhereClause(search, landingID, nguon, status, fromDate, toDate, fromHour, toHour)

	query := fmt.Sprintf(`
		SELECT id, TO_CHAR(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'), landing_id, nguon, so_dien_thoai,
		       COALESCE(trang_thai, 'moi'), COALESCE(ghi_chu, ''), COALESCE(nguoi_phu_trach, ''),
		       TO_CHAR(COALESCE(cap_nhat_luc, thoi_gian) AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
		       COALESCE(city, ''), COALESCE(region, ''), COALESCE(country, 'Việt Nam'), COALESCE(isp, ''),
		       COALESCE(ad_platform, ''), COALESCE(ad_placement, ''), COALESCE(url, ''),
		       tracking::text
		FROM leads
		%s
		ORDER BY id DESC;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn toàn bộ leads: %w", err)
	}
	defer rows.Close()

	leads := make([]models.Lead, 0)
	for rows.Next() {
		var lead models.Lead
		var trackingStr string
		if err := rows.Scan(
			&lead.ID,
			&lead.CreatedAt,
			&lead.LandingID,
			&lead.Nguon,
			&lead.Phone,
			&lead.TrangThai,
			&lead.GhiChu,
			&lead.AssignedTo,
			&lead.UpdatedAt,
			&lead.City,
			&lead.Region,
			&lead.Country,
			&lead.ISP,
			&lead.AdPlatform,
			&lead.AdPlacement,
			&lead.URL,
			&trackingStr,
		); err != nil {
			return nil, fmt.Errorf("lỗi đọc dữ liệu export CSV: %w", err)
		}
		lead.Tracking = json.RawMessage(trackingStr)
		leads = append(leads, lead)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lỗi con trỏ: %w", err)
	}

	return leads, nil
}

// UpdateLead cập nhật trạng thái CRM, ghi chú sales và người phụ trách
func (s *Store) UpdateLead(id uint64, status string, note string, assignedTo string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var sets []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(status) != "" {
		sets = append(sets, fmt.Sprintf("trang_thai = $%d", idx))
		args = append(args, strings.TrimSpace(status))
		idx++
	}
	if strings.TrimSpace(note) != "" {
		sets = append(sets, fmt.Sprintf("ghi_chu = $%d", idx))
		args = append(args, strings.TrimSpace(note))
		idx++
	}
	if strings.TrimSpace(assignedTo) != "" {
		sets = append(sets, fmt.Sprintf("nguoi_phu_trach = $%d", idx))
		args = append(args, strings.TrimSpace(assignedTo))
		idx++
	}

	sets = append(sets, "cap_nhat_luc = NOW()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE leads SET %s WHERE id = $%d;", strings.Join(sets, ", "), idx)
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

// GetCRMStats cung cấp toàn bộ chỉ số KPI, phân bổ khung giờ vàng 24h, xu hướng ngày, trạng thái CRM, nhà mạng
func (s *Store) GetCRMStats(search string, landingID string, nguon string, status string, fromDate string, toDate string, fromHour int, toHour int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	whereClause, args := buildWhereClause(search, landingID, nguon, status, fromDate, toDate, fromHour, toHour)

	// 1. Tổng quan KPIs
	summaryQuery := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total,
			COUNT(*) FILTER (WHERE (thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date)::int AS today,
			COUNT(*) FILTER (WHERE (thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - INTERVAL '1 day')::date)::int AS yesterday,
			COUNT(*) FILTER (WHERE (thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh') >= ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh') - INTERVAL '7 days'))::int AS this_week,
			COUNT(*) FILTER (WHERE trang_thai IN ('tiem_nang', 'chot'))::int AS high_intent,
			COUNT(*) FILTER (WHERE trang_thai = 'chot')::int AS closed
		FROM leads
		%s;
	`, whereClause)

	var total, today, yesterday, thisWeek, highIntent, closed int
	_ = s.pool.QueryRow(ctx, summaryQuery, args...).Scan(&total, &today, &yesterday, &thisWeek, &highIntent, &closed)

	// 2. Phân bổ theo 24 khung giờ trong ngày (0h đến 23h)
	hourlyMap := make(map[int]int)
	for i := 0; i < 24; i++ {
		hourlyMap[i] = 0
	}
	hourlyQuery := fmt.Sprintf(`
		SELECT EXTRACT(HOUR FROM thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::int AS hr, COUNT(*)::int
		FROM leads
		%s
		GROUP BY hr
		ORDER BY hr;
	`, whereClause)
	rowsH, errH := s.pool.Query(ctx, hourlyQuery, args...)
	if errH == nil {
		defer rowsH.Close()
		for rowsH.Next() {
			var hr, cnt int
			if err := rowsH.Scan(&hr, &cnt); err == nil && hr >= 0 && hr < 24 {
				hourlyMap[hr] = cnt
			}
		}
	}
	hourlyDistribution := make([]map[string]interface{}, 24)
	peakHour := 0
	peakHourCount := 0
	for i := 0; i < 24; i++ {
		cnt := hourlyMap[i]
		if cnt > peakHourCount {
			peakHourCount = cnt
			peakHour = i
		}
		hourlyDistribution[i] = map[string]interface{}{
			"hour":  i,
			"label": fmt.Sprintf("%02d:00", i),
			"count": cnt,
		}
	}

	// 3. Xu hướng theo ngày (Daily trend - 30 ngày gần nhất)
	dailyQuery := fmt.Sprintf(`
		SELECT TO_CHAR(thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD') AS dt, COUNT(*)::int
		FROM leads
		%s
		GROUP BY dt
		ORDER BY dt ASC
		LIMIT 30;
	`, whereClause)
	dailyTrend := make([]map[string]interface{}, 0)
	rowsD, errD := s.pool.Query(ctx, dailyQuery, args...)
	if errD == nil {
		defer rowsD.Close()
		for rowsD.Next() {
			var dt string
			var cnt int
			if err := rowsD.Scan(&dt, &cnt); err == nil {
				dailyTrend = append(dailyTrend, map[string]interface{}{
					"date":  dt,
					"count": cnt,
				})
			}
		}
	}

	// 4. Phân bổ trạng thái CRM (Pipeline Breakdown)
	statusQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(trang_thai, ''), 'moi') AS st, COUNT(*)::int
		FROM leads
		%s
		GROUP BY st
		ORDER BY COUNT(*) DESC;
	`, whereClause)
	statusCounts := map[string]int{
		"moi":          0,
		"da_goi":       0,
		"ket_ban_zalo": 0,
		"da_vao_zoom":  0,
		"tiem_nang":    0,
		"chot":         0,
		"huy":          0,
	}
	rowsS, errS := s.pool.Query(ctx, statusQuery, args...)
	if errS == nil {
		defer rowsS.Close()
		for rowsS.Next() {
			var st string
			var cnt int
			if err := rowsS.Scan(&st, &cnt); err == nil {
				statusCounts[st] = cnt
			}
		}
	}

	// 5. Phân bổ theo nhà mạng (Carrier breakdown)
	carrierQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(tracking->>'carrier', ''), 'Khác') AS cr, COUNT(*)::int
		FROM leads
		%s
		GROUP BY cr
		ORDER BY COUNT(*) DESC;
	`, whereClause)
	carrierCounts := make(map[string]int)
	rowsC, errC := s.pool.Query(ctx, carrierQuery, args...)
	if errC == nil {
		defer rowsC.Close()
		for rowsC.Next() {
			var cr string
			var cnt int
			if err := rowsC.Scan(&cr, &cnt); err == nil {
				carrierCounts[cr] = cnt
			}
		}
	}

	// 6. Phân bổ theo nguồn (Source breakdown)
	sourceQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(nguon, ''), 'direct') AS src, COUNT(*)::int
		FROM leads
		%s
		GROUP BY src
		ORDER BY COUNT(*) DESC
		LIMIT 10;
	`, whereClause)
	sourceCounts := make([]map[string]interface{}, 0)
	rowsSrc, errSrc := s.pool.Query(ctx, sourceQuery, args...)
	if errSrc == nil {
		defer rowsSrc.Close()
		for rowsSrc.Next() {
			var src string
			var cnt int
			if err := rowsSrc.Scan(&src, &cnt); err == nil {
				sourceCounts = append(sourceCounts, map[string]interface{}{
					"source": src,
					"count":  cnt,
				})
			}
		}
	}

	// 7. Tỷ lệ phương thức điền (Autofill vs Manual)
	autofillQuery := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE tracking->>'is_autofill' IN ('true', '1') OR COALESCE(tracking->>'autofill_source', '') != '')::int AS autofill_count,
			COUNT(*) FILTER (WHERE NOT (tracking->>'is_autofill' IN ('true', '1') OR COALESCE(tracking->>'autofill_source', '') != ''))::int AS manual_count
		FROM leads
		%s;
	`, whereClause)
	var autofillCount, manualCount int
	_ = s.pool.QueryRow(ctx, autofillQuery, args...).Scan(&autofillCount, &manualCount)

	// 8. Phân bổ theo Tỉnh/Thành phố (Top Cities)
	cityQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(city, ''), 'Chưa xác định') AS ct, COUNT(*)::int
		FROM leads
		%s
		GROUP BY ct
		ORDER BY COUNT(*) DESC
		LIMIT 15;
	`, whereClause)
	cityCounts := make([]map[string]interface{}, 0)
	rowsCt, errCt := s.pool.Query(ctx, cityQuery, args...)
	if errCt == nil {
		defer rowsCt.Close()
		for rowsCt.Next() {
			var ct string
			var cnt int
			if err := rowsCt.Scan(&ct, &cnt); err == nil {
				cityCounts = append(cityCounts, map[string]interface{}{
					"city":  ct,
					"count": cnt,
				})
			}
		}
	}

	// 9. Phân bổ theo 3 Vùng miền (Miền Bắc, Miền Trung, Miền Nam, Quốc Tế)
	regionQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(region, ''), 'Miền Bắc') AS rg, COUNT(*)::int
		FROM leads
		%s
		GROUP BY rg
		ORDER BY COUNT(*) DESC;
	`, whereClause)
	regionCounts := map[string]int{
		"Miền Bắc":  0,
		"Miền Trung": 0,
		"Miền Nam":  0,
		"Quốc Tế":   0,
	}
	rowsRg, errRg := s.pool.Query(ctx, regionQuery, args...)
	if errRg == nil {
		defer rowsRg.Close()
		for rowsRg.Next() {
			var rg string
			var cnt int
			if err := rowsRg.Scan(&rg, &cnt); err == nil {
				regionCounts[rg] = cnt
			}
		}
	}

	// 10. Phân bổ theo Vị trí quảng cáo chi tiết (Ad Placements: Instagram, FB Feed, Audience Network, GDN...)
	placementQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(ad_placement, ''), 'Trực Tiếp (Direct)') AS pl, COUNT(*)::int
		FROM leads
		%s
		GROUP BY pl
		ORDER BY COUNT(*) DESC
		LIMIT 12;
	`, whereClause)
	adPlacementCounts := make([]map[string]interface{}, 0)
	rowsPl, errPl := s.pool.Query(ctx, placementQuery, args...)
	if errPl == nil {
		defer rowsPl.Close()
		for rowsPl.Next() {
			var pl string
			var cnt int
			if err := rowsPl.Scan(&pl, &cnt); err == nil {
				adPlacementCounts = append(adPlacementCounts, map[string]interface{}{
					"placement": pl,
					"count":     cnt,
				})
			}
		}
	}

	// 11. Phân bổ theo ISP / Nhà mạng từ IP (VNPT, Viettel, FPT, MobiFone...)
	ispQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(isp, ''), 'Khác') AS isp_name, COUNT(*)::int
		FROM leads
		%s
		GROUP BY isp_name
		ORDER BY COUNT(*) DESC
		LIMIT 8;
	`, whereClause)
	ispCounts := make([]map[string]interface{}, 0)
	rowsIsp, errIsp := s.pool.Query(ctx, ispQuery, args...)
	if errIsp == nil {
		defer rowsIsp.Close()
		for rowsIsp.Next() {
			var iName string
			var cnt int
			if err := rowsIsp.Scan(&iName, &cnt); err == nil {
				ispCounts = append(ispCounts, map[string]interface{}{
					"isp":   iName,
					"count": cnt,
				})
			}
		}
	}

	result := map[string]interface{}{
		"summary": map[string]interface{}{
			"total":           total,
			"today":           today,
			"yesterday":       yesterday,
			"this_week":       thisWeek,
			"high_intent":     highIntent,
			"closed":          closed,
			"peak_hour":       fmt.Sprintf("%02d:00 - %02d:00", peakHour, (peakHour+1)%24),
			"peak_hour_count": peakHourCount,
		},
		"hourly_distribution": hourlyDistribution,
		"daily_trend":         dailyTrend,
		"status_counts":       statusCounts,
		"carrier_counts":      carrierCounts,
		"source_counts":       sourceCounts,
		"city_counts":         cityCounts,
		"region_counts":       regionCounts,
		"ad_placement_counts": adPlacementCounts,
		"isp_counts":          ispCounts,
		"autofill_ratio": map[string]interface{}{
			"autofill": autofillCount,
			"manual":   manualCount,
		},
	}
	return result, nil
}

// CountStats trả về thống kê leads (hỗ trợ lọc theo landing_id và nguồn)
func (s *Store) CountStats(landingID string, nguon string) (total int, today int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whereClause, args := buildWhereClause("", landingID, nguon, "", "", "", -1, -1)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total,
			COUNT(*) FILTER (WHERE (thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date)::int AS today
		FROM leads
		%s;
	`, whereClause)

	err = s.pool.QueryRow(ctx, query, args...).Scan(&total, &today)
	if err != nil {
		return 0, 0, fmt.Errorf("lỗi thống kê leads: %w", err)
	}

	return total, today, nil
}

// GetFilterOptions lấy danh sách các landing_id và nguon đã có trong CSDL phục vụ cho dropdown lọc
func (s *Store) GetFilterOptions() (landingIDs []string, sources []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Landing IDs
	rowsLanding, err := s.pool.Query(ctx, "SELECT DISTINCT landing_id FROM leads WHERE landing_id != '' ORDER BY landing_id ASC;")
	if err == nil {
		defer rowsLanding.Close()
		for rowsLanding.Next() {
			var lid string
			if err := rowsLanding.Scan(&lid); err == nil {
				landingIDs = append(landingIDs, lid)
			}
		}
	}

	// 2. Sources (Nguồn)
	rowsSource, err := s.pool.Query(ctx, "SELECT DISTINCT nguon FROM leads WHERE nguon != '' ORDER BY nguon ASC;")
	if err == nil {
		defer rowsSource.Close()
		for rowsSource.Next() {
			var src string
			if err := rowsSource.Scan(&src); err == nil {
				sources = append(sources, src)
			}
		}
	}

	return landingIDs, sources, nil
}

// ============================================================
// HỆ THỐNG TRACKING CHI TIẾT & PHÂN TÍCH HÀNH VI (ANALYTICS)
// ============================================================

// UpsertAnalyticsSession cập nhật hoặc thêm mới một phiên truy cập kèm theo các sự kiện hành vi
func (s *Store) UpsertAnalyticsSession(p *models.AnalyticsEventPayload, ip string, userAgent string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var actionsJSON []byte
	if len(p.Events) > 0 {
		actionsJSON, _ = json.Marshal(p.Events)
	}
	if len(actionsJSON) == 0 || string(actionsJSON) == "null" {
		actionsJSON = []byte("[]")
	}

	if p.Country == "" {
		p.Country = "Việt Nam"
	}

	query := `
		INSERT INTO analytics_sessions (
			session_id, visitor_id, landing_id, nguon,
			utm_source, utm_medium, utm_campaign, utm_content, utm_term,
			referrer, entry_url, ip_address, city, region, country, isp, ad_platform, ad_placement,
			user_agent, device_type, browser, os, screen_res,
			is_bot, bot_score, bot_reasons, duration_seconds, max_scroll_percent,
			total_clicks, has_interacted_form, converted_lead, so_dien_thoai, lead_id,
			actions, first_seen, last_seen
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23,
			$24, $25, $26, $27, $28,
			$29, $30, $31, $32, $33,
			COALESCE($34::jsonb, '[]'::jsonb), NOW(), NOW()
		)
		ON CONFLICT (session_id) DO UPDATE SET
			last_seen = NOW(),
			landing_id = CASE WHEN EXCLUDED.landing_id != '' THEN EXCLUDED.landing_id ELSE analytics_sessions.landing_id END,
			nguon = CASE WHEN EXCLUDED.nguon != '' THEN EXCLUDED.nguon ELSE analytics_sessions.nguon END,
			city = CASE WHEN EXCLUDED.city != '' THEN EXCLUDED.city ELSE analytics_sessions.city END,
			region = CASE WHEN EXCLUDED.region != '' THEN EXCLUDED.region ELSE analytics_sessions.region END,
			country = CASE WHEN EXCLUDED.country != '' THEN EXCLUDED.country ELSE analytics_sessions.country END,
			isp = CASE WHEN EXCLUDED.isp != '' THEN EXCLUDED.isp ELSE analytics_sessions.isp END,
			ad_platform = CASE WHEN EXCLUDED.ad_platform != '' THEN EXCLUDED.ad_platform ELSE analytics_sessions.ad_platform END,
			ad_placement = CASE WHEN EXCLUDED.ad_placement != '' THEN EXCLUDED.ad_placement ELSE analytics_sessions.ad_placement END,
			duration_seconds = GREATEST(analytics_sessions.duration_seconds, EXCLUDED.duration_seconds),
			max_scroll_percent = GREATEST(analytics_sessions.max_scroll_percent, EXCLUDED.max_scroll_percent),
			total_clicks = analytics_sessions.total_clicks + EXCLUDED.total_clicks,
			has_interacted_form = analytics_sessions.has_interacted_form OR EXCLUDED.has_interacted_form,
			converted_lead = analytics_sessions.converted_lead OR EXCLUDED.converted_lead,
			so_dien_thoai = CASE WHEN EXCLUDED.so_dien_thoai != '' THEN EXCLUDED.so_dien_thoai ELSE analytics_sessions.so_dien_thoai END,
			lead_id = COALESCE(EXCLUDED.lead_id, analytics_sessions.lead_id),
			is_bot = analytics_sessions.is_bot OR EXCLUDED.is_bot,
			bot_score = GREATEST(analytics_sessions.bot_score, EXCLUDED.bot_score),
			bot_reasons = CASE
				WHEN EXCLUDED.bot_reasons != '' AND analytics_sessions.bot_reasons = '' THEN EXCLUDED.bot_reasons
				WHEN EXCLUDED.bot_reasons != '' AND analytics_sessions.bot_reasons NOT LIKE '%' || EXCLUDED.bot_reasons || '%' THEN analytics_sessions.bot_reasons || '; ' || EXCLUDED.bot_reasons
				ELSE analytics_sessions.bot_reasons
			END,
			actions = CASE
				WHEN jsonb_typeof($34::jsonb) = 'array' AND jsonb_array_length($34::jsonb) > 0
				THEN COALESCE(analytics_sessions.actions, '[]'::jsonb) || $34::jsonb
				ELSE COALESCE(analytics_sessions.actions, '[]'::jsonb)
			END;
	`

	var leadIDVal interface{}
	if p.LeadID != nil {
		leadIDVal = int64(*p.LeadID)
	}

	_, err := s.pool.Exec(ctx, query,
		p.SessionID, p.VisitorID, p.LandingID, p.Nguon,
		p.UTMSource, p.UTMMedium, p.UTMCampaign, p.UTMContent, p.UTMTerm,
		p.Referrer, p.EntryURL, ip, p.City, p.Region, p.Country, p.ISP, p.AdPlatform, p.AdPlacement,
		userAgent, p.DeviceType, p.Browser, p.OS, p.ScreenRes,
		p.IsBot, p.BotScore, p.BotReasons, p.DurationSeconds, p.MaxScrollPercent,
		p.ClicksCount, p.HasInteractedForm, p.ConvertedLead, p.Phone, leadIDVal,
		string(actionsJSON),
	)
	return err
}

// AssociateLeadToSession liên kết thông tin Lead (SĐT, LeadID) vào Session tương ứng
func (s *Store) AssociateLeadToSession(sessionID string, phone string, leadID uint64) error {
	if sessionID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE analytics_sessions
		SET converted_lead = TRUE,
		    so_dien_thoai = $1::varchar,
		    lead_id = $2::bigint,
		    actions = COALESCE(actions, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
		        'time', duration_seconds,
		        'type', 'converted',
		        'target', 'registrationForm',
		        'text', 'Hoàn tất đăng ký',
		        'detail', jsonb_build_object('lead_id', $2::bigint, 'phone', $1::varchar)
		    )),
		    last_seen = NOW()
		WHERE session_id = $3::varchar;
	`
	_, err := s.pool.Exec(ctx, query, phone, int64(leadID), sessionID)
	return err
}

// buildAnalyticsWhereClause hỗ trợ bộ lọc chung cho các bảng analytics
func buildAnalyticsWhereClause(landingID string, nguon string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(landingID) != "" {
		conditions = append(conditions, fmt.Sprintf("landing_id = $%d", idx))
		args = append(args, strings.TrimSpace(landingID))
		idx++
	}

	if strings.TrimSpace(nguon) != "" {
		conditions = append(conditions, fmt.Sprintf("nguon = $%d", idx))
		args = append(args, strings.TrimSpace(nguon))
		idx++
	}

	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// GetAnalyticsOverview trả về báo cáo tổng quan lượt xem, người thật, bot, CR%
func (s *Store) GetAnalyticsOverview(landingID string, nguon string) (*models.AnalyticsOverview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClause(landingID, nguon)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS total_leads,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration,
			COALESCE(AVG(max_scroll_percent) FILTER (WHERE NOT is_bot), 0)::float AS avg_scroll,
			COUNT(*) FILTER (WHERE device_type = 'mobile')::int AS mobile_views,
			COUNT(*) FILTER (WHERE device_type = 'desktop')::int AS desktop_views,
			COUNT(*) FILTER (WHERE device_type = 'tablet')::int AS tablet_views
		FROM analytics_sessions
		%s;
	`, whereClause)

	var ov models.AnalyticsOverview
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&ov.TotalViews,
		&ov.HumanVisitors,
		&ov.BotVisitors,
		&ov.TotalLeads,
		&ov.AvgDurationSec,
		&ov.AvgMaxScrollPct,
		&ov.MobileViews,
		&ov.DesktopViews,
		&ov.TabletViews,
	)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn tổng quan analytics: %w", err)
	}

	if ov.TotalViews > 0 {
		ov.BotRatePercent = float64(ov.BotVisitors) / float64(ov.TotalViews) * 100
	}
	if ov.HumanVisitors > 0 {
		ov.ConversionRate = float64(ov.TotalLeads) / float64(ov.HumanVisitors) * 100
	}

	return &ov, nil
}

// GetLandingPerformance so sánh hiệu quả giữa các Landing Page
func (s *Store) GetLandingPerformance(landingID string, nguon string) ([]models.LandingStatItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClause(landingID, nguon)

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(landing_id, ''), 'default_landing') AS lid,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_visitors,
			COUNT(*) FILTER (WHERE has_interacted_form)::int AS form_interactions,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration,
			COALESCE(AVG(max_scroll_percent) FILTER (WHERE NOT is_bot), 0)::float AS avg_scroll
		FROM analytics_sessions
		%s
		GROUP BY lid
		ORDER BY total_views DESC;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê hiệu quả landing: %w", err)
	}
	defer rows.Close()

	items := make([]models.LandingStatItem, 0)
	for rows.Next() {
		var it models.LandingStatItem
		if err := rows.Scan(
			&it.LandingID,
			&it.TotalViews,
			&it.HumanVisitors,
			&it.BotVisitors,
			&it.FormInteractions,
			&it.Leads,
			&it.ConversionRate,
			&it.AvgDurationSec,
			&it.AvgScrollPct,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// GetSourcePerformance thống kê theo Nguồn traffic & Content quảng cáo
func (s *Store) GetSourcePerformance(landingID string, nguon string) ([]models.SourceStatItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClause(landingID, nguon)

	query := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(nguon, ''), 'direct') AS src,
			COALESCE(NULLIF(utm_campaign, ''), 'none') AS campaign,
			COALESCE(NULLIF(utm_content, ''), 'none') AS content,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE is_bot)::int AS bot_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration
		FROM analytics_sessions
		%s
		GROUP BY src, campaign, content
		ORDER BY total_views DESC
		LIMIT 50;
	`, whereClause)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lỗi thống kê nguồn & content: %w", err)
	}
	defer rows.Close()

	items := make([]models.SourceStatItem, 0)
	for rows.Next() {
		var it models.SourceStatItem
		if err := rows.Scan(
			&it.Nguon,
			&it.UTMCampaign,
			&it.UTMContent,
			&it.TotalViews,
			&it.HumanVisitors,
			&it.BotVisitors,
			&it.Leads,
			&it.ConversionRate,
			&it.AvgDurationSec,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// ListAnalyticsSessions danh sách chi tiết các phiên truy cập của khách hàng
func (s *Store) ListAnalyticsSessions(page, limit int, landingID, nguon, filterBot, filterConverted, searchPhone string) ([]models.AnalyticsSession, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 30
	}
	offset := (page - 1) * limit

	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(landingID) != "" {
		conditions = append(conditions, fmt.Sprintf("landing_id = $%d", idx))
		args = append(args, strings.TrimSpace(landingID))
		idx++
	}

	if strings.TrimSpace(nguon) != "" {
		conditions = append(conditions, fmt.Sprintf("nguon = $%d", idx))
		args = append(args, strings.TrimSpace(nguon))
		idx++
	}

	if filterBot == "human" {
		conditions = append(conditions, "is_bot = FALSE")
	} else if filterBot == "bot" {
		conditions = append(conditions, "is_bot = TRUE")
	}

	if filterConverted == "converted" {
		conditions = append(conditions, "converted_lead = TRUE")
	} else if filterConverted == "not_converted" {
		conditions = append(conditions, "converted_lead = FALSE")
	}

	if strings.TrimSpace(searchPhone) != "" {
		conditions = append(conditions, fmt.Sprintf("so_dien_thoai ILIKE '%%' || $%d || '%%'", idx))
		args = append(args, strings.TrimSpace(searchPhone))
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Đếm tổng số bản ghi
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM analytics_sessions %s", whereClause)
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	selectArgs := append(args, limit, offset)

	query := fmt.Sprintf(`
		SELECT
			session_id, visitor_id, landing_id, nguon,
			utm_source, utm_medium, utm_campaign, utm_content, utm_term,
			referrer, entry_url, ip_address,
			COALESCE(city, ''), COALESCE(region, ''), COALESCE(country, 'Việt Nam'), COALESCE(isp, ''),
			COALESCE(ad_platform, ''), COALESCE(ad_placement, ''),
			user_agent, device_type, browser, os, screen_res,
			is_bot, bot_score, bot_reasons, duration_seconds, max_scroll_percent,
			total_clicks, has_interacted_form, converted_lead, so_dien_thoai, lead_id,
			actions::text,
			TO_CHAR(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
			TO_CHAR(last_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM analytics_sessions
		%s
		ORDER BY last_seen DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, limitIdx, offsetIdx)

	rows, err := s.pool.Query(ctx, query, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi truy vấn sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]models.AnalyticsSession, 0)
	for rows.Next() {
		var sess models.AnalyticsSession
		var actionsStr string
		err := rows.Scan(
			&sess.SessionID, &sess.VisitorID, &sess.LandingID, &sess.Nguon,
			&sess.UTMSource, &sess.UTMMedium, &sess.UTMCampaign, &sess.UTMContent, &sess.UTMTerm,
			&sess.Referrer, &sess.EntryURL, &sess.IPAddress,
			&sess.City, &sess.Region, &sess.Country, &sess.ISP,
			&sess.AdPlatform, &sess.AdPlacement,
			&sess.UserAgent, &sess.DeviceType, &sess.Browser, &sess.OS, &sess.ScreenRes,
			&sess.IsBot, &sess.BotScore, &sess.BotReasons, &sess.DurationSeconds, &sess.MaxScrollPercent,
			&sess.TotalClicks, &sess.HasInteractedForm, &sess.ConvertedLead, &sess.Phone, &sess.LeadID,
			&actionsStr,
			&sess.FirstSeen,
			&sess.LastSeen,
		)
		if err != nil {
			return nil, 0, err
		}
		sess.Actions = json.RawMessage(actionsStr)
		sessions = append(sessions, sess)
	}

	return sessions, total, nil
}

// GetSessionDetail lấy chi tiết 1 phiên và toàn bộ Timeline hành vi
func (s *Store) GetSessionDetail(sessionID string) (*models.AnalyticsSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			session_id, visitor_id, landing_id, nguon,
			utm_source, utm_medium, utm_campaign, utm_content, utm_term,
			referrer, entry_url, ip_address,
			COALESCE(city, ''), COALESCE(region, ''), COALESCE(country, 'Việt Nam'), COALESCE(isp, ''),
			COALESCE(ad_platform, ''), COALESCE(ad_placement, ''),
			user_agent, device_type, browser, os, screen_res,
			is_bot, bot_score, bot_reasons, duration_seconds, max_scroll_percent,
			total_clicks, has_interacted_form, converted_lead, so_dien_thoai, lead_id,
			actions::text,
			TO_CHAR(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
			TO_CHAR(last_seen AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS')
		FROM analytics_sessions
		WHERE session_id = $1;
	`

	var sess models.AnalyticsSession
	var actionsStr string
	err := s.pool.QueryRow(ctx, query, sessionID).Scan(
		&sess.SessionID, &sess.VisitorID, &sess.LandingID, &sess.Nguon,
		&sess.UTMSource, &sess.UTMMedium, &sess.UTMCampaign, &sess.UTMContent, &sess.UTMTerm,
		&sess.Referrer, &sess.EntryURL, &sess.IPAddress,
		&sess.City, &sess.Region, &sess.Country, &sess.ISP,
		&sess.AdPlatform, &sess.AdPlacement,
		&sess.UserAgent, &sess.DeviceType, &sess.Browser, &sess.OS, &sess.ScreenRes,
		&sess.IsBot, &sess.BotScore, &sess.BotReasons, &sess.DurationSeconds, &sess.MaxScrollPercent,
		&sess.TotalClicks, &sess.HasInteractedForm, &sess.ConvertedLead, &sess.Phone, &sess.LeadID,
		&actionsStr,
		&sess.FirstSeen,
		&sess.LastSeen,
	)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy phiên truy cập: %w", err)
	}
	sess.Actions = json.RawMessage(actionsStr)
	return &sess, nil
}

// GetGeoAnalytics trả về thống kê lượt truy cập, leads và tỷ lệ chuyển đổi CR% theo tỉnh thành, vùng miền và ISP
func (s *Store) GetGeoAnalytics(landingID string, nguon string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClause(landingID, nguon)

	// 1. Top Tỉnh / Thành phố
	cityQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(city, ''), 'Khác / Chưa rõ') AS city_name,
			COALESCE(NULLIF(region, ''), 'Miền Bắc') AS region_name,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr
		FROM analytics_sessions
		%s
		GROUP BY city_name, region_name
		ORDER BY total_views DESC
		LIMIT 20;
	`, whereClause)

	rowsC, errC := s.pool.Query(ctx, cityQuery, args...)
	cities := make([]map[string]interface{}, 0)
	if errC == nil {
		defer rowsC.Close()
		for rowsC.Next() {
			var cName, rName string
			var views, humans, leads int
			var cr float64
			if err := rowsC.Scan(&cName, &rName, &views, &humans, &leads, &cr); err == nil {
				cities = append(cities, map[string]interface{}{
					"name":            cName,
					"region":          rName,
					"total_views":     views,
					"human_visitors":  humans,
					"leads":           leads,
					"conversion_rate": cr,
				})
			}
		}
	}

	// 2. Phân bổ 3 Vùng miền (Miền Bắc, Miền Trung, Miền Nam, Quốc Tế)
	regionQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(region, ''), 'Miền Bắc') AS region_name,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr
		FROM analytics_sessions
		%s
		GROUP BY region_name
		ORDER BY total_views DESC;
	`, whereClause)

	rowsR, errR := s.pool.Query(ctx, regionQuery, args...)
	regions := make([]map[string]interface{}, 0)
	if errR == nil {
		defer rowsR.Close()
		for rowsR.Next() {
			var rName string
			var views, humans, leads int
			var cr float64
			if err := rowsR.Scan(&rName, &views, &humans, &leads, &cr); err == nil {
				regions = append(regions, map[string]interface{}{
					"name":            rName,
					"total_views":     views,
					"human_visitors":  humans,
					"leads":           leads,
					"conversion_rate": cr,
				})
			}
		}
	}

	// 3. Phân bổ theo Nhà mạng / ISP (VNPT, Viettel, FPT, MobiFone,...)
	ispQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(isp, ''), 'Khác') AS isp_name,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr
		FROM analytics_sessions
		%s
		GROUP BY isp_name
		ORDER BY total_views DESC
		LIMIT 10;
	`, whereClause)

	rowsI, errI := s.pool.Query(ctx, ispQuery, args...)
	isps := make([]map[string]interface{}, 0)
	if errI == nil {
		defer rowsI.Close()
		for rowsI.Next() {
			var iName string
			var views, humans, leads int
			var cr float64
			if err := rowsI.Scan(&iName, &views, &humans, &leads, &cr); err == nil {
				isps = append(isps, map[string]interface{}{
					"name":            iName,
					"total_views":     views,
					"human_visitors":  humans,
					"leads":           leads,
					"conversion_rate": cr,
				})
			}
		}
	}

	return map[string]interface{}{
		"cities":  cities,
		"regions": regions,
		"isps":    isps,
	}, nil
}

// GetAdPlacementAnalytics trả về thống kê lượt truy cập, leads và tỷ lệ chuyển đổi chi tiết theo Nền tảng & Vị trí quảng cáo
func (s *Store) GetAdPlacementAnalytics(landingID string, nguon string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClause(landingID, nguon)

	// 1. Phân bổ theo Nền tảng (Meta Ads, Google Ads, TikTok Ads, Zalo Ads, Direct, Referral...)
	platformQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(ad_platform, ''), 'Trực Tiếp') AS pf,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration
		FROM analytics_sessions
		%s
		GROUP BY pf
		ORDER BY total_views DESC;
	`, whereClause)

	rowsP, errP := s.pool.Query(ctx, platformQuery, args...)
	platforms := make([]map[string]interface{}, 0)
	if errP == nil {
		defer rowsP.Close()
		for rowsP.Next() {
			var pf string
			var views, humans, leads int
			var cr, duration float64
			if err := rowsP.Scan(&pf, &views, &humans, &leads, &cr, &duration); err == nil {
				platforms = append(platforms, map[string]interface{}{
					"platform":             pf,
					"total_views":          views,
					"human_visitors":       humans,
					"leads":                leads,
					"conversion_rate":      cr,
					"avg_duration_seconds": duration,
				})
			}
		}
	}

	// 2. Chi tiết vị trí quảng cáo (Ad Placement: Instagram, FB Feed, Audience Network, GDN, YouTube, PMax...)
	placementQuery := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(ad_platform, ''), 'Trực Tiếp') AS pf,
			COALESCE(NULLIF(ad_placement, ''), 'Trực Tiếp (Direct)') AS pl,
			COUNT(*)::int AS total_views,
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS human_visitors,
			COUNT(*) FILTER (WHERE converted_lead)::int AS leads,
			CASE
				WHEN COUNT(*) FILTER (WHERE NOT is_bot) > 0
				THEN ROUND((COUNT(*) FILTER (WHERE converted_lead)::numeric / COUNT(*) FILTER (WHERE NOT is_bot)::numeric) * 100, 2)::float
				ELSE 0
			END AS cr,
			COALESCE(AVG(duration_seconds) FILTER (WHERE NOT is_bot), 0)::float AS avg_duration
		FROM analytics_sessions
		%s
		GROUP BY pf, pl
		ORDER BY total_views DESC
		LIMIT 30;
	`, whereClause)

	rowsPl, errPl := s.pool.Query(ctx, placementQuery, args...)
	placements := make([]map[string]interface{}, 0)
	metaPlacements := make([]map[string]interface{}, 0)
	googlePlacements := make([]map[string]interface{}, 0)

	if errPl == nil {
		defer rowsPl.Close()
		for rowsPl.Next() {
			var pf, pl string
			var views, humans, leads int
			var cr, duration float64
			if err := rowsPl.Scan(&pf, &pl, &views, &humans, &leads, &cr, &duration); err == nil {
				item := map[string]interface{}{
					"platform":             pf,
					"placement":            pl,
					"total_views":          views,
					"human_visitors":       humans,
					"leads":                leads,
					"conversion_rate":      cr,
					"avg_duration_seconds": duration,
				}
				placements = append(placements, item)
				if pf == "Meta Ads" {
					metaPlacements = append(metaPlacements, item)
				} else if pf == "Google Ads" {
					googlePlacements = append(googlePlacements, item)
				}
			}
		}
	}

	return map[string]interface{}{
		"platforms":         platforms,
		"placements":        placements,
		"meta_placements":   metaPlacements,
		"google_placements": googlePlacements,
	}, nil
}

// SaveSpamQuarantine lưu thông tin request spam vào vùng cách ly
func (s *Store) SaveSpamQuarantine(ip, userAgent, landingID, nguon, phone, reason string, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if len(payload) == 0 {
		payload = []byte("{}")
	}

	query := `
		INSERT INTO spam_quarantine (ip_address, user_agent, landing_id, nguon, phone, reason, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, NOW());
	`
	_, err := s.pool.Exec(ctx, query, ip, userAgent, landingID, nguon, phone, reason, string(payload))
	return err
}

// ListSpamQuarantine lấy danh sách các bản ghi spam bị chặn
func (s *Store) ListSpamQuarantine(limit, offset int) ([]models.SpamQuarantineItem, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM spam_quarantine;`
	var total int64
	if err := s.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id,
			TO_CHAR(created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD HH24:MI:SS'),
			ip_address, user_agent, landing_id, nguon, phone, reason, payload::text
		FROM spam_quarantine
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`
	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.SpamQuarantineItem
	for rows.Next() {
		var item models.SpamQuarantineItem
		var payloadStr string
		err := rows.Scan(
			&item.ID,
			&item.CreatedAt,
			&item.IPAddress,
			&item.UserAgent,
			&item.LandingID,
			&item.Nguon,
			&item.Phone,
			&item.Reason,
			&payloadStr,
		)
		if err != nil {
			return nil, 0, err
		}
		item.Payload = json.RawMessage(payloadStr)
		items = append(items, item)
	}

	return items, total, nil
}

// GetSecurityStats lấy thống kê tổng quan về các cuộc tấn công và spam bị chặn
func (s *Store) GetSecurityStats() (*models.SecurityStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats := &models.SecurityStats{
		ReasonsBreakdown: make(map[string]int64),
	}

	// 1. Tổng số spam bị chặn
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM spam_quarantine;`).Scan(&stats.TotalSpamBlocked)

	// 2. Số spam bị chặn trong ngày hôm nay (giờ VN)
	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM spam_quarantine
		WHERE created_at >= (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Ho_Chi_Minh')::date;
	`).Scan(&stats.TodaySpamBlocked)

	// 3. Phân loại theo lý do chặn
	rows, err := s.pool.Query(ctx, `
		SELECT reason, COUNT(*) as cnt
		FROM spam_quarantine
		GROUP BY reason
		ORDER BY cnt DESC
		LIMIT 10;
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var reason string
			var count int64
			if err := rows.Scan(&reason, &count); err == nil {
				stats.ReasonsBreakdown[reason] = count
			}
		}
	}

	return stats, nil
}


