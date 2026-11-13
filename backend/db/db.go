package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	// Timeout cho phần KẾT NỐI pool (ping).
	connCtx, connCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connCancel()

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("cấu hình chuỗi kết nối PostgreSQL không hợp lệ: %w", err)
	}

	// Tối ưu connection pool
	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(connCtx, config)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối PostgreSQL pool: %w", err)
	}

	// Kiểm tra kết nối
	if err := pool.Ping(connCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("không thể ping tới PostgreSQL: %w", err)
	}

	store := &Store{pool: pool}

	// Khởi tạo/cập nhật schema (DDL).
	//
	// ⚠️ Dùng context RIÊNG với timeout 60 giây (không dùng chung context 10s
	// của phần kết nối). Lý do: DDL gồm nhiều câu CREATE TABLE IF NOT EXISTS +
	// ALTER TABLE ADD COLUMN IF NOT EXISTS + CREATE INDEX; trên máy chậm hoặc
	// khi bảng lớn có thể vượt 10 giây ⇒ backend FATAL lúc khởi động ⇒
	// production 503. Đã xảy ra ngày 13-11-2026 (do migration dữ liệu nằm
	// chung DDL), nay đã tách migration data ra riêng + timeout riêng cho DDL.
	schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer schemaCancel()

	if err := store.initSchema(schemaCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("lỗi khởi tạo bảng CSDL: %w", err)
	}

	// Migration DỮ LIỆU analytics (đồng bộ attribution + cờ converted_lead).
	//
	// ⚠️ Chạy SAU initSchema, trong GOROUTINE riêng, với context RIÊNG
	// (timeout 2 phút) — không dùng chung context 10s của NewStore, vì
	// migration quét bảng leads + analytics_sessions nên vượt quá thời gian
	// đó ⇒ đã gây FATAL lúc khởi động server (503) ngày 13-11-2026.
	// Lỗi migration KHÔNG chặn backend khởi động (chỉ log cảnh báo).
	go store.migrateAnalyticsAttribution()

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

	-- ══════════════════════════════════════════════════════════════
	-- MỞ RỘNG THỐNG KÊ (landing doanh_chung — 2026-11-12)
	-- ══════════════════════════════════════════════════════════════
	-- Thiết bị chi tiết (model thật, không chỉ "mobile")
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_brand VARCHAR(50) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_model VARCHAR(120) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_model_code VARCHAR(60) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_name VARCHAR(160) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS os_version VARCHAR(30) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS browser_version VARCHAR(30) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS browser_engine VARCHAR(20) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS is_in_app_browser BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS in_app_type VARCHAR(30) NOT NULL DEFAULT '';

	-- Trạng thái thiết bị / môi trường thực thi
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_memory INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_cores INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_touchpoints INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS viewport_res VARCHAR(30) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS screen_color_depth INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS pixel_ratio VARCHAR(20) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS timezone VARCHAR(60) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS language VARCHAR(40) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS platform_info VARCHAR(80) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS connection_type VARCHAR(30) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS device_state VARCHAR(120) NOT NULL DEFAULT '';

	-- VPN / Proxy / Tor
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS is_vpn BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS is_proxy BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS is_tor BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS vpn_provider VARCHAR(100) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS vpn_confidence INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS vpn_reason VARCHAR(255) NOT NULL DEFAULT '';

	-- Meta Click ID (Conversions API) & định danh lượt nhấp
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS fbclid VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS fbc VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS fbp VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS igshid VARCHAR(120) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS meta_click_id VARCHAR(255) NOT NULL DEFAULT '';

	-- Meta Ads Macros (campaign.id / adset.id / ad.id / tên...)
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS utm_placement VARCHAR(120) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS campaign_name VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS adset_name VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS ad_name VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS ad_account_id VARCHAR(60) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS site_source_name VARCHAR(40) NOT NULL DEFAULT '';

	-- Thời gian thực & hành vi (tách khỏi "duration_seconds" tổng)
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS visible_seconds INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS hidden_seconds INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS session_count INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS landing_path VARCHAR(255) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS referrer_host VARCHAR(160) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS is_new_visitor BOOLEAN NOT NULL DEFAULT TRUE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS has_video_play BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS has_zalo_click BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS has_tel_click BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS form_submit_attempts INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS form_validation_errors INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS scroll_depth_25 BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS scroll_depth_50 BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS scroll_depth_75 BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS scroll_depth_90 BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS scroll_depth_100 BOOLEAN NOT NULL DEFAULT FALSE;

	-- ══════════════════════════════════════════════════════════════
	-- PHỄU BIỂU MẪU & AUTOFILL (thiết kế lại báo cáo — 2026-11-13)
	-- ══════════════════════════════════════════════════════════════
	-- Trước đây is_autofill / autofill_source / fill_duration_ms chỉ nằm
	-- trong leads.tracking (JSONB) ⇒ chỉ thống kê được trên lead ĐÃ tạo,
	-- không tính được cho phiên bỏ form. Báo cáo "Phễu Biểu Mẫu" cần số liệu
	-- ở MỨC PHIÊN ⇒ đưa các cột này vào analytics_sessions.
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS is_autofill BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS autofill_source VARCHAR(40) NOT NULL DEFAULT '';
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS fill_duration_ms INTEGER NOT NULL DEFAULT 0;
	-- Phiên đã bấm submit ≥ 1 lần VÀ không dính lỗi validate ở lần cuối
	-- ⇒ đã vượt qua bước kiểm tra dữ liệu (khác với form_submit_attempts
	-- chỉ đếm số lần bấm).
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS validation_passed BOOLEAN NOT NULL DEFAULT FALSE;
	-- Tổng thời gian user đã focus vào form (ms) — dùng cho "Avg Fill Duration".
	ALTER TABLE analytics_sessions ADD COLUMN IF NOT EXISTS form_focus_ms INTEGER NOT NULL DEFAULT 0;

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
	CREATE INDEX IF NOT EXISTS idx_sessions_device_model ON analytics_sessions (device_model);
	CREATE INDEX IF NOT EXISTS idx_sessions_device_brand ON analytics_sessions (device_brand);
	CREATE INDEX IF NOT EXISTS idx_sessions_fbclid ON analytics_sessions (fbclid);
	CREATE INDEX IF NOT EXISTS idx_sessions_meta_click ON analytics_sessions (meta_click_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_is_vpn ON analytics_sessions (is_vpn);
	CREATE INDEX IF NOT EXISTS idx_sessions_utm_placement ON analytics_sessions (utm_placement);
	CREATE INDEX IF NOT EXISTS idx_sessions_campaign_name ON analytics_sessions (campaign_name);
	CREATE INDEX IF NOT EXISTS idx_sessions_landing_first_seen ON analytics_sessions (landing_id, first_seen DESC);
	-- ──────────────────────────────────────────────────────────────────────
	-- ĐỒNG BỘ CỜ converted_lead VỚI BẢNG leads (fix 2026-11-13)
	-- ──────────────────────────────────────────────────────────────────────
	--
	-- ⚠️ QUAN TRỌNG: migration DỮ LIỆU (B1→B5) KHÔNG nằm ở đây.
	-- Lý do: initSchema dùng CHUNG context với NewStore (timeout 10 giây).
	-- Migration dữ liệu cần thời gian lâu hơn (quét bảng leads +
	-- analytics_sessions) ⇒ nếu để trong DDL sẽ làm backend FATAL lúc
	-- khởi động (timeout: context deadline exceeded) — đã xảy ra trên
	-- production 2026-11-13 05:34.
	--
	-- ⇒ Migration data được tách sang hàm migrateAnalyticsAttribution()
	--   chạy với context RIÊNG (timeout 2 phút) SAU khi DDL hoàn tất.
	--   Xem backend/db/db.go → migrateAnalyticsAttribution.
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

// migrateAnalyticsAttribution chạy migration DỮ LIỆU cho bảng analytics.
//
// ⚠️ TÁCH RIÊNG khỏi `initSchema` (fix sự cố 2026-11-13):
//
// VÌ SAO TÁCH:
// `initSchema` dùng CHUNG context với `NewStore` — timeout chỉ 10 giây. Migration
// dữ liệu phải quét bảng `leads` + `analytics_sessions` nên vượt quá 10 giây ⇒
// backend bị FATAL lúc khởi động ("timeout: context deadline exceeded") ⇒
// production trả 503. Đã xảy ra thật lúc 05:34 ngày 13-11-2026.
//
// ⇒ Hàm này chạy với context RIÊNG (timeout 2 phút) và chạy SAU khi DDL xong.
//   Lỗi migration KHÔNG làm backend chết — chỉ log cảnh báo, backend vẫn phục vụ.
//
// THỨ TỰ (phải đúng):
//   B1. Tạo dòng session tối thiểu cho lead chưa có session.
//   B2. Gắn lead_id cho session qua tracking.session_id.
//   B3. Gắn lead_id cho session qua action 'converted' (SĐT trong action).
//   B4. Khôi phục landing_id/nguon của session theo landing của lead.
//   B5. CUỐI CÙNG: gỡ converted_lead cho mọi session không có lead thật.
//
// Tất cả đều IDEMPOTENT — chạy lại nhiều lần vẫn cho cùng kết quả.
func (s *Store) migrateAnalyticsAttribution() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	steps := []struct {
		name string
		sql  string
	}{
		{
			// B1. BACKFILL SESSION THIẾU CHO LEAD ĐÃ GHI
			//
			// VẤN ĐỀ: nhiều lead có tracking.session_id nhưng KHÔNG có dòng tương ứng
			// trong analytics_sessions (analytics ping bị throttle / mất / server
			// restart giữa lúc ghi lead). Đo trên production ngày 13-11: 8 lead, chỉ
			// 2 có session ⇒ biểu đồ "lượt xem" và "lead" thấp hơn thực tế.
			//
			// CÁCH SỬA: tự tạo dòng session tối thiểu cho các lead còn thiếu, gắn
			// converted_lead = TRUE + lead_id ⇒ số liệu lịch sử khớp với CRM.
			name: "B1 backfill session cho lead",
			sql: `
				INSERT INTO analytics_sessions (
				    session_id, visitor_id, landing_id, nguon,
				    so_dien_thoai, lead_id, converted_lead, has_interacted_form,
				    duration_seconds, actions, first_seen, last_seen
				)
				SELECT
				    l.tracking->>'session_id',
				    COALESCE(NULLIF(l.tracking->>'visitor_id', ''), l.tracking->>'session_id'),
				    COALESCE(NULLIF(l.landing_id, ''), 'default_landing'),
				    COALESCE(NULLIF(l.nguon, ''), l.landing_id),
				    l.so_dien_thoai,
				    l.id,
				    TRUE, TRUE,
				    0,
				    jsonb_build_array(jsonb_build_object(
				        'time', 0,
				        'type', 'converted',
				        'target', 'registrationForm',
				        'text', 'Hoàn tất đăng ký',
				        'detail', jsonb_build_object('lead_id', l.id, 'phone', l.so_dien_thoai)
				    )),
				    l.thoi_gian, l.thoi_gian
				FROM leads l
				WHERE COALESCE(l.tracking->>'session_id', '') != ''
				  AND NOT EXISTS (
				    SELECT 1 FROM analytics_sessions s WHERE s.session_id = l.tracking->>'session_id'
				  )
				ON CONFLICT (session_id) DO NOTHING;
			`,
		},
		{
			// B2. Gắn lead_id cho các session đã có nhưng chưa trỏ tới lead nào.
			name: "B2 gắn lead_id qua tracking.session_id",
			sql: `
				UPDATE analytics_sessions s
				SET converted_lead = TRUE,
				    lead_id = l.id,
				    so_dien_thoai = COALESCE(NULLIF(s.so_dien_thoai, ''), l.so_dien_thoai)
				FROM leads l
				WHERE s.session_id = l.tracking->>'session_id'
				  AND (s.lead_id IS NULL OR s.lead_id <> l.id);
			`,
		},
		{
			// B3. Gắn session ↔ lead bằng SĐT trong action 'converted'.
			//
			// ⚠️ TỐI ƯU: bản đầu dùng correlated subquery MIN(first_seen) ⇒ quét
			// lại toàn bảng `analytics_sessions` cho MỖI dòng ⇒ O(n²) ⇒ timeout.
			// Nay dùng CTE gom SĐT → session đầu tiên (ROW_NUMBER) rồi join 1 lần.
			name: "B3 gắn lead_id qua action converted",
			sql: `
				WITH first_conv AS (
				    SELECT
				        COALESCE(e->'detail'->>'phone', '') AS phone,
				        s.session_id,
				        ROW_NUMBER() OVER (PARTITION BY COALESCE(e->'detail'->>'phone', '') ORDER BY s.first_seen ASC) AS rn
				    FROM analytics_sessions s
				    CROSS JOIN LATERAL jsonb_array_elements(COALESCE(s.actions, '[]'::jsonb)) e
				    WHERE e->>'type' = 'converted'
				      AND COALESCE(e->'detail'->>'phone', '') != ''
				),
				picked AS (
				    SELECT phone, session_id FROM first_conv WHERE rn = 1
				)
				UPDATE analytics_sessions s
				SET lead_id = l.id,
				    converted_lead = TRUE,
				    so_dien_thoai = COALESCE(NULLIF(s.so_dien_thoai, ''), l.so_dien_thoai)
				FROM picked p
				JOIN leads l ON l.so_dien_thoai = p.phone
				WHERE s.session_id = p.session_id
				  AND s.lead_id IS NULL;
			`,
		},
		{
			// B4. Khôi phục landing_id/nguon của session theo landing CỦA LEAD.
			//
			// VẤN ĐỀ: cookie phiên dùng CHUNG giữa landing và trang "chúc mừng"
			// ⇒ cùng 1 session_id được gửi lên lần lượt với landing_id = landing_cu
			// rồi landing_lznoew ⇒ phiên gốc của landing_cu bị "chuyển" sang lznoew.
			// Đo trên production: landing_lznoew có 270 view + 270 converted (CR 100%)
			// trong khi đó chỉ là trang CHỈ ĐỂ XEM; landing_cu mất lead.
			name: "B4 khôi phục landing_id/nguon theo lead",
			sql: `
				UPDATE analytics_sessions s
				SET landing_id = l.landing_id,
				    nguon = COALESCE(NULLIF(l.nguon, ''), l.landing_id)
				FROM leads l
				WHERE s.lead_id = l.id
				  AND COALESCE(l.landing_id, '') != ''
				  AND s.landing_id <> l.landing_id;
			`,
		},
		{
			// B5. GỠ cờ converted cho mọi session KHÔNG có lead thật.
			//
			// ⚠️ Chạy SAU CÙNG để backend là nguồn sự thật duy nhất: cờ converted
			// chỉ TRUE khi session liên kết được tới 1 lead thật trong bảng leads.
			name: "B5 gỡ cờ converted không có lead thật",
			sql: `
				UPDATE analytics_sessions s
				SET converted_lead = FALSE
				WHERE s.converted_lead = TRUE
				  AND (
				    s.lead_id IS NULL
				    OR NOT EXISTS (SELECT 1 FROM leads l WHERE l.id = s.lead_id)
				  );
			`,
		},
		{
			// CHUẨN HOÁ NGUỒN THEO TỪNG LANDING.
			//
			// VẤN ĐỀ: nguồn mặc định cũ là landing_chinh cho MỌI landing
			// (landing_cu, landing_lznoew, landing_232, mxv_080_online đều ghi chung
			// 1 nguồn) ⇒ bộ lọc "Nguồn" không tách được, số liệu chồng chéo.
			name: "C chuẩn hoá nguon theo landing",
			sql: `
				UPDATE leads
				SET nguon = landing_id
				WHERE (nguon = '' OR nguon = 'landing_chinh')
				  AND COALESCE(landing_id, '') != '';

				UPDATE analytics_sessions
				SET nguon = landing_id
				WHERE (nguon = '' OR nguon = 'landing_chinh')
				  AND COALESCE(landing_id, '') != '';
			`,
		},
	}

	for _, step := range steps {
		start := time.Now()
		if _, err := s.pool.Exec(ctx, step.sql); err != nil {
			if ctx.Err() != nil {
				log.Printf("[MIGRATION][WARN] Migration '%s' da timeout — bo qua (backend van chay)", step.name)
				return
			}
			log.Printf("[MIGRATION][WARN] Migration '%s' that bai: %v", step.name, err)
			continue
		}
		log.Printf("[MIGRATION] OK %s (%v)", step.name, time.Since(start).Round(time.Millisecond))
	}
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

	// ⚠️ 2026-11-12: hỗ trợ khoảng giờ VÒNG QUA nửa đêm (vd "22h-06h").
	// Điều kiện cũ `fromHour <= toHour` ⇒ khoảng đêm bị BỎ QUA hoàn toàn
	// ⇒ bộ lọc "Buổi Đêm" không lọc gì cả.
	if fromHour >= 0 && toHour >= 0 && toHour <= 23 {
		if fromHour <= toHour {
			conditions = append(conditions, fmt.Sprintf("EXTRACT(HOUR FROM thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::int BETWEEN $%d AND $%d", idx, idx+1))
		} else {
			// Khoảng vòng qua nửa đêm: giờ >= fromHour HOẶC giờ <= toHour
			conditions = append(conditions, fmt.Sprintf("(EXTRACT(HOUR FROM thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::int >= $%d OR EXTRACT(HOUR FROM thoi_gian AT TIME ZONE 'Asia/Ho_Chi_Minh')::int <= $%d)", idx, idx+1))
		}
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

	// 12. PHỄU CHUYỂN ĐỔI THẬT (2026-11-12)
	//
	// ⚠️ TRƯỚC ĐÂY frontend tự ước lượng `totalLeads * 8` ⇒ phễu hiển thị sai
	// hoàn toàn so với dữ liệu thật. Nay lấy số liệu thật từ
	// `analytics_sessions` (lượt xem / cuộn / tương tác form / chuyển đổi).
	funnelWhere, funnelArgs := buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate, 0, fromHour, toHour)
	funnelQuery := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE NOT is_bot)::int AS views,
			COUNT(*) FILTER (WHERE NOT is_bot AND max_scroll_percent >= 50)::int AS scrolled,
			COUNT(*) FILTER (WHERE NOT is_bot AND (has_interacted_form OR form_submit_attempts > 0))::int AS interacted,
			COUNT(*) FILTER (WHERE NOT is_bot AND converted_lead)::int AS converted
		FROM analytics_sessions
		%s;
	`, funnelWhere)

	var funnelViews, funnelScrolled, funnelInteracted, funnelConverted int
	_ = s.pool.QueryRow(ctx, funnelQuery, funnelArgs...).Scan(&funnelViews, &funnelScrolled, &funnelInteracted, &funnelConverted)

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
		"funnel": map[string]interface{}{
			"views":       funnelViews,
			"scrolled":    funnelScrolled,
			"interacted":  funnelInteracted,
			"converted":   funnelConverted,
			"high_intent": highIntent,
			"closed":      closed,
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
			actions, first_seen, last_seen,
			device_brand, device_model, device_model_code, device_name,
			os_version, browser_version, browser_engine, is_in_app_browser, in_app_type,
			device_memory, device_cores, device_touchpoints, viewport_res,
			screen_color_depth, pixel_ratio, timezone, language, platform_info,
			connection_type, device_state,
			is_vpn, is_proxy, is_tor, vpn_provider, vpn_confidence, vpn_reason,
			fbclid, fbc, fbp, igshid, meta_click_id,
			utm_placement, campaign_name, adset_name, ad_name, ad_account_id, site_source_name,
			visible_seconds, hidden_seconds, session_count, landing_path, referrer_host,
			is_new_visitor, has_video_play, has_zalo_click, has_tel_click,
			form_submit_attempts, form_validation_errors,
			scroll_depth_25, scroll_depth_50, scroll_depth_75, scroll_depth_90, scroll_depth_100,
			is_autofill, autofill_source, fill_duration_ms, validation_passed, form_focus_ms
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23,
			$24, $25, $26, $27, $28,
			$29, $30, $31, $32, $33,
			COALESCE($34::jsonb, '[]'::jsonb), NOW(), NOW(),
			$35, $36, $37, $38,
			$39, $40, $41, $42, $43,
			$44, $45, $46, $47,
			$48, $49, $50, $51, $52,
			$53, $54,
			$55, $56, $57, $58, $59, $60,
			$61, $62, $63, $64, $65,
			$66, $67, $68, $69, $70, $71,
			$72, $73, $74, $75, $76, $77,
			$78, $79, $80, $81, $82, $83,
			$84, $85, $86, $87,
			$88, $89, $90, $91, $92
		)
		ON CONFLICT (session_id) DO UPDATE SET
			last_seen = NOW(),
			-- ⚠️ FIX 2026-11-13: GIỮ NGUYÊN landing_id/nguon CŨ (landing đầu tiên
			-- khách thấy), KHÔNG ghi đè bằng landing của request sau.
			--
			-- VÌ SAO: cookie phiên dùng CHUNG giữa landing và trang "chúc mừng"
			-- ⇒ cùng 1 session_id sẽ được gửi lên lần lượt với landing_id =
			-- landing_cu rồi landing_lznoew. Trước đây EXCLUDED.landing_id
			-- ghi đè ⇒ phiên gốc của landing_cu bị "chuyển" sang lznoew ⇒
			-- landing_cu mất lead, lznoew có 270 view + 270 lead (CR = 100%) ⇒
			-- THÔNG SỐ CHỒNG CHÉO giữa các landing.
			--
			-- ⇒ Landing đầu tiên là nguồn attribution đúng. Chỉ ghi landing mới
			--   khi bản cũ rỗng (trường hợp lần ping đầu tiên thiếu landing_id).
			landing_id = CASE WHEN analytics_sessions.landing_id != '' THEN analytics_sessions.landing_id ELSE EXCLUDED.landing_id END,
			nguon = CASE WHEN analytics_sessions.nguon != '' THEN analytics_sessions.nguon ELSE EXCLUDED.nguon END,
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
			END,
			device_brand = CASE WHEN EXCLUDED.device_brand != '' THEN EXCLUDED.device_brand ELSE analytics_sessions.device_brand END,
			device_model = CASE WHEN EXCLUDED.device_model != '' THEN EXCLUDED.device_model ELSE analytics_sessions.device_model END,
			device_model_code = CASE WHEN EXCLUDED.device_model_code != '' THEN EXCLUDED.device_model_code ELSE analytics_sessions.device_model_code END,
			device_name = CASE WHEN EXCLUDED.device_name != '' THEN EXCLUDED.device_name ELSE analytics_sessions.device_name END,
			os_version = CASE WHEN EXCLUDED.os_version != '' THEN EXCLUDED.os_version ELSE analytics_sessions.os_version END,
			browser_version = CASE WHEN EXCLUDED.browser_version != '' THEN EXCLUDED.browser_version ELSE analytics_sessions.browser_version END,
			browser_engine = CASE WHEN EXCLUDED.browser_engine != '' THEN EXCLUDED.browser_engine ELSE analytics_sessions.browser_engine END,
			is_in_app_browser = analytics_sessions.is_in_app_browser OR EXCLUDED.is_in_app_browser,
			in_app_type = CASE WHEN EXCLUDED.in_app_type != '' THEN EXCLUDED.in_app_type ELSE analytics_sessions.in_app_type END,
			device_memory = GREATEST(analytics_sessions.device_memory, EXCLUDED.device_memory),
			device_cores = GREATEST(analytics_sessions.device_cores, EXCLUDED.device_cores),
			device_touchpoints = GREATEST(analytics_sessions.device_touchpoints, EXCLUDED.device_touchpoints),
			viewport_res = CASE WHEN EXCLUDED.viewport_res != '' THEN EXCLUDED.viewport_res ELSE analytics_sessions.viewport_res END,
			screen_color_depth = GREATEST(analytics_sessions.screen_color_depth, EXCLUDED.screen_color_depth),
			pixel_ratio = CASE WHEN EXCLUDED.pixel_ratio != '' THEN EXCLUDED.pixel_ratio ELSE analytics_sessions.pixel_ratio END,
			timezone = CASE WHEN EXCLUDED.timezone != '' THEN EXCLUDED.timezone ELSE analytics_sessions.timezone END,
			language = CASE WHEN EXCLUDED.language != '' THEN EXCLUDED.language ELSE analytics_sessions.language END,
			platform_info = CASE WHEN EXCLUDED.platform_info != '' THEN EXCLUDED.platform_info ELSE analytics_sessions.platform_info END,
			connection_type = CASE WHEN EXCLUDED.connection_type != '' THEN EXCLUDED.connection_type ELSE analytics_sessions.connection_type END,
			device_state = CASE WHEN EXCLUDED.device_state != '' THEN EXCLUDED.device_state ELSE analytics_sessions.device_state END,
			is_vpn = analytics_sessions.is_vpn OR EXCLUDED.is_vpn,
			is_proxy = analytics_sessions.is_proxy OR EXCLUDED.is_proxy,
			is_tor = analytics_sessions.is_tor OR EXCLUDED.is_tor,
			vpn_provider = CASE WHEN EXCLUDED.vpn_provider != '' THEN EXCLUDED.vpn_provider ELSE analytics_sessions.vpn_provider END,
			vpn_confidence = GREATEST(analytics_sessions.vpn_confidence, EXCLUDED.vpn_confidence),
			vpn_reason = CASE WHEN EXCLUDED.vpn_reason != '' THEN EXCLUDED.vpn_reason ELSE analytics_sessions.vpn_reason END,
			fbclid = CASE WHEN EXCLUDED.fbclid != '' THEN EXCLUDED.fbclid ELSE analytics_sessions.fbclid END,
			fbc = CASE WHEN EXCLUDED.fbc != '' THEN EXCLUDED.fbc ELSE analytics_sessions.fbc END,
			fbp = CASE WHEN EXCLUDED.fbp != '' THEN EXCLUDED.fbp ELSE analytics_sessions.fbp END,
			igshid = CASE WHEN EXCLUDED.igshid != '' THEN EXCLUDED.igshid ELSE analytics_sessions.igshid END,
			meta_click_id = CASE WHEN EXCLUDED.meta_click_id != '' THEN EXCLUDED.meta_click_id ELSE analytics_sessions.meta_click_id END,
			utm_placement = CASE WHEN EXCLUDED.utm_placement != '' THEN EXCLUDED.utm_placement ELSE analytics_sessions.utm_placement END,
			campaign_name = CASE WHEN EXCLUDED.campaign_name != '' THEN EXCLUDED.campaign_name ELSE analytics_sessions.campaign_name END,
			adset_name = CASE WHEN EXCLUDED.adset_name != '' THEN EXCLUDED.adset_name ELSE analytics_sessions.adset_name END,
			ad_name = CASE WHEN EXCLUDED.ad_name != '' THEN EXCLUDED.ad_name ELSE analytics_sessions.ad_name END,
			ad_account_id = CASE WHEN EXCLUDED.ad_account_id != '' THEN EXCLUDED.ad_account_id ELSE analytics_sessions.ad_account_id END,
			site_source_name = CASE WHEN EXCLUDED.site_source_name != '' THEN EXCLUDED.site_source_name ELSE analytics_sessions.site_source_name END,
			visible_seconds = GREATEST(analytics_sessions.visible_seconds, EXCLUDED.visible_seconds),
			hidden_seconds = GREATEST(analytics_sessions.hidden_seconds, EXCLUDED.hidden_seconds),
			session_count = GREATEST(analytics_sessions.session_count, EXCLUDED.session_count),
			landing_path = CASE WHEN EXCLUDED.landing_path != '' THEN EXCLUDED.landing_path ELSE analytics_sessions.landing_path END,
			referrer_host = CASE WHEN EXCLUDED.referrer_host != '' THEN EXCLUDED.referrer_host ELSE analytics_sessions.referrer_host END,
			is_new_visitor = analytics_sessions.is_new_visitor AND EXCLUDED.is_new_visitor,
			has_video_play = analytics_sessions.has_video_play OR EXCLUDED.has_video_play,
			has_zalo_click = analytics_sessions.has_zalo_click OR EXCLUDED.has_zalo_click,
			has_tel_click = analytics_sessions.has_tel_click OR EXCLUDED.has_tel_click,
			form_submit_attempts = GREATEST(analytics_sessions.form_submit_attempts, EXCLUDED.form_submit_attempts),
			form_validation_errors = GREATEST(analytics_sessions.form_validation_errors, EXCLUDED.form_validation_errors),
			scroll_depth_25 = analytics_sessions.scroll_depth_25 OR EXCLUDED.scroll_depth_25,
			scroll_depth_50 = analytics_sessions.scroll_depth_50 OR EXCLUDED.scroll_depth_50,
			scroll_depth_75 = analytics_sessions.scroll_depth_75 OR EXCLUDED.scroll_depth_75,
			scroll_depth_90 = analytics_sessions.scroll_depth_90 OR EXCLUDED.scroll_depth_90,
			scroll_depth_100 = analytics_sessions.scroll_depth_100 OR EXCLUDED.scroll_depth_100,
			is_autofill = analytics_sessions.is_autofill OR EXCLUDED.is_autofill,
			autofill_source = CASE WHEN EXCLUDED.autofill_source != '' THEN EXCLUDED.autofill_source ELSE analytics_sessions.autofill_source END,
			fill_duration_ms = GREATEST(analytics_sessions.fill_duration_ms, EXCLUDED.fill_duration_ms),
			validation_passed = analytics_sessions.validation_passed OR EXCLUDED.validation_passed,
			form_focus_ms = GREATEST(analytics_sessions.form_focus_ms, EXCLUDED.form_focus_ms);
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
		p.DeviceBrand, p.DeviceModel, p.DeviceModelCode, p.DeviceName,
		p.OSVersion, p.BrowserVersion, p.BrowserEngine, p.IsInAppBrowser, p.InAppType,
		p.DeviceMemory, p.DeviceCores, p.DeviceTouchpoints, p.ViewportRes,
		p.ScreenColorDepth, p.PixelRatio, p.Timezone, p.Language, p.PlatformInfo,
		p.ConnectionType, p.DeviceState,
		p.IsVPN, p.IsProxy, p.IsTor, p.VPNProvider, p.VPNConfidence, p.VPNReason,
		p.Fbclid, p.Fbc, p.Fbp, p.Igshid, p.MetaClickID,
		p.UTMPlacement, p.CampaignName, p.AdsetName, p.AdName, p.AdAccountID, p.SiteSourceName,
		p.VisibleSeconds, p.HiddenSeconds, p.SessionCount, p.LandingPath, p.ReferrerHost,
		p.IsNewVisitor, p.HasVideoPlay, p.HasZaloClick, p.HasTelClick,
		p.FormSubmitAttempts, p.FormValidationError,
		p.ScrollDepth25, p.ScrollDepth50, p.ScrollDepth75, p.ScrollDepth90, p.ScrollDepth100,
		p.IsAutofill, p.AutofillSource, p.FillDurationMs, p.ValidationPassed, p.FormFocusMs,
	)
	return err
}

// AssociateLeadToSession liên kết thông tin Lead (SĐT, LeadID) vào Session tương ứng.
//
// ⚠️ FIX 2026-11-13 — "Hôm nay có nhiều lead nhưng biểu đồ hiện rất ít":
//
// VẤN ĐỀ: hàm này UPDATE theo `session_id`. Nếu phiên chưa tồn tại trong
// `analytics_sessions` (khách đăng ký rất nhanh, analytics ping bị throttle
// hoặc bị mất) ⇒ UPDATE khớp 0 dòng ⇒ lead có trong bảng `leads` nhưng
// BIỂU ĐỒ KHÔNG THẤY. Trên production: 8 lead hôm nay chỉ 2 khớp session.
//
// CÁCH SỬA: dùng `INSERT ... ON CONFLICT (session_id) DO UPDATE` ⇒ luôn tạo
// dòng session nếu chưa có ⇒ mọi lead đều xuất hiện trong biểu đồ.
func (s *Store) AssociateLeadToSession(sessionID string, phone string, leadID uint64) error {
	if sessionID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Lấy landing_id/nguon của session (nếu có) để dựng dòng mới cho đúng.
	var landingID, nguon, visitorID string
	_ = s.pool.QueryRow(ctx,
		`SELECT landing_id, nguon, visitor_id FROM analytics_sessions WHERE session_id = $1`,
		sessionID).Scan(&landingID, &nguon, &visitorID)

	if visitorID == "" {
		visitorID = sessionID
	}
	if landingID == "" {
		landingID = "default_landing"
	}
	if nguon == "" {
		// ⚠️ Nguồn mặc định = landing_id (fix 2026-11-13) để mỗi landing có
		// nguồn riêng, không bị gộp chung thành "landing_chinh".
		nguon = landingID
	}

	query := `
		INSERT INTO analytics_sessions (
			session_id, visitor_id, landing_id, nguon,
			so_dien_thoai, lead_id, converted_lead, has_interacted_form,
			duration_seconds, actions, first_seen, last_seen
		) VALUES (
			$1, $2, $3, $4,
			$5, $6::bigint, TRUE, TRUE,
			0, jsonb_build_array(jsonb_build_object(
			    'time', 0,
			    'type', 'converted',
			    'target', 'registrationForm',
			    'text', 'Hoàn tất đăng ký',
			    'detail', jsonb_build_object('lead_id', $6::bigint, 'phone', $5::varchar)
			)), NOW(), NOW()
		)
		ON CONFLICT (session_id) DO UPDATE SET
			converted_lead = TRUE,
			so_dien_thoai = EXCLUDED.so_dien_thoai,
			lead_id = EXCLUDED.lead_id,
			has_interacted_form = TRUE,
			landing_id = CASE WHEN EXCLUDED.landing_id != '' THEN EXCLUDED.landing_id ELSE analytics_sessions.landing_id END,
			nguon = CASE WHEN EXCLUDED.nguon != '' THEN EXCLUDED.nguon ELSE analytics_sessions.nguon END,
			actions = COALESCE(analytics_sessions.actions, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
			    'time', analytics_sessions.duration_seconds,
			    'type', 'converted',
			    'target', 'registrationForm',
			    'text', 'Hoàn tất đăng ký',
			    'detail', jsonb_build_object('lead_id', EXCLUDED.lead_id, 'phone', EXCLUDED.so_dien_thoai)
			)),
			last_seen = NOW()
	`
	_, err := s.pool.Exec(ctx, query, sessionID, visitorID, landingID, nguon, phone, int64(leadID))
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

// buildAnalyticsWhereClauseFull bộ lọc analytics ĐẦY ĐỦ (landing + nguồn +
// khoảng ngày theo múi giờ Asia/Ho_Chi_Minh).
//
// ⚠️ Lý do tồn tại (2026-11-12): các hàm thống kê cũ chỉ dùng
// `buildAnalyticsWhereClause` ⇒ bộ lọc "Từ ngày / Đến ngày" trên CRM bị
// BỎ QUA hoàn toàn ⇒ số liệu biểu đồ không khớp với bảng lead ⇒ "thống kê sai".
//
// fromDate/toDate: định dạng YYYY-MM-DD (rỗng = không giới hạn).
// days: số ngày gần nhất; <= 0 = không giới hạn. Chỉ dùng khi KHÔNG có
// from/to date cụ thể để tránh cắt bớt dữ liệu ngoài khoảng đã chọn.
func buildAnalyticsWhereClauseFull(landingID, nguon, fromDate, toDate string, days int) (string, []interface{}) {
	return buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate, days, -1, -1)
}

// buildAnalyticsWhereClauseFullRange như buildAnalyticsWhereClauseFull nhưng
// hỗ trợ thêm lọc theo khung giờ trong ngày (fromHour/toHour, 0-23).
// -1 = không lọc.
func buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate string, days int, fromHour int, toHour int) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	if s := strings.TrimSpace(landingID); s != "" {
		conditions = append(conditions, fmt.Sprintf("landing_id = $%d", idx))
		args = append(args, s)
		idx++
	}
	if s := strings.TrimSpace(nguon); s != "" {
		conditions = append(conditions, fmt.Sprintf("nguon = $%d", idx))
		args = append(args, s)
		idx++
	}
	if s := strings.TrimSpace(fromDate); s != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ($%d)::date", idx))
		args = append(args, s)
		idx++
	}
	if s := strings.TrimSpace(toDate); s != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date <= ($%d)::date", idx))
		args = append(args, s)
		idx++
	}
	if days > 0 && strings.TrimSpace(fromDate) == "" && strings.TrimSpace(toDate) == "" {
		conditions = append(conditions, fmt.Sprintf(
			"(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - ($%d::int))", idx))
		args = append(args, days)
		idx++
	}
	if fromHour >= 0 && toHour >= 0 && toHour <= 23 {
		if fromHour <= toHour {
			conditions = append(conditions, fmt.Sprintf(
				"EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int BETWEEN $%d AND $%d", idx, idx+1))
		} else {
			// Khoảng vòng qua nửa đêm (vd 22h-06h)
			conditions = append(conditions, fmt.Sprintf(
				"(EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int >= $%d OR EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int <= $%d)", idx, idx+1))
		}
		args = append(args, fromHour, toHour)
		idx += 2
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

	whereClause, args := buildAnalyticsWhereClauseFull(landingID, nguon, "", "", 0)

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
//
// ⚠️ KHÔNG lọc theo landing_id (bảng này dùng để SO SÁNH các landing),
// chỉ lọc theo nguồn + khoảng ngày.
func (s *Store) GetLandingPerformance(landingID string, nguon string, fromDate string, toDate string, fromHour int, toHour int) ([]models.LandingStatItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClauseFullRange("", nguon, fromDate, toDate, 0, fromHour, toHour)

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
func (s *Store) GetSourcePerformance(landingID string, nguon string, fromDate string, toDate string, fromHour int, toHour int) ([]models.SourceStatItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate, 0, fromHour, toHour)

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
			COALESCE(device_brand, ''), COALESCE(device_model, ''), COALESCE(device_model_code, ''), COALESCE(device_name, ''),
			COALESCE(os_version, ''), COALESCE(browser_version, ''), COALESCE(browser_engine, ''),
			is_in_app_browser, COALESCE(in_app_type, ''),
			device_memory, device_cores, device_touchpoints, COALESCE(viewport_res, ''),
			screen_color_depth, COALESCE(pixel_ratio, ''), COALESCE(timezone, ''), COALESCE(language, ''),
			COALESCE(platform_info, ''), COALESCE(connection_type, ''), COALESCE(device_state, ''),
			is_vpn, is_proxy, is_tor, COALESCE(vpn_provider, ''), vpn_confidence, COALESCE(vpn_reason, ''),
			COALESCE(fbclid, ''), COALESCE(fbc, ''), COALESCE(fbp, ''), COALESCE(igshid, ''), COALESCE(meta_click_id, ''),
			COALESCE(utm_placement, ''), COALESCE(campaign_name, ''), COALESCE(adset_name, ''),
			COALESCE(ad_name, ''), COALESCE(ad_account_id, ''), COALESCE(site_source_name, ''),
			visible_seconds, hidden_seconds, session_count, COALESCE(landing_path, ''), COALESCE(referrer_host, ''),
			is_new_visitor, has_video_play, has_zalo_click, has_tel_click,
			form_submit_attempts, form_validation_errors,
			scroll_depth_25, scroll_depth_50, scroll_depth_75, scroll_depth_90, scroll_depth_100,
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
			&sess.DeviceBrand, &sess.DeviceModel, &sess.DeviceModelCode, &sess.DeviceName,
			&sess.OSVersion, &sess.BrowserVersion, &sess.BrowserEngine,
			&sess.IsInAppBrowser, &sess.InAppType,
			&sess.DeviceMemory, &sess.DeviceCores, &sess.DeviceTouchpoints, &sess.ViewportRes,
			&sess.ScreenColorDepth, &sess.PixelRatio, &sess.Timezone, &sess.Language,
			&sess.PlatformInfo, &sess.ConnectionType, &sess.DeviceState,
			&sess.IsVPN, &sess.IsProxy, &sess.IsTor, &sess.VPNProvider, &sess.VPNConfidence, &sess.VPNReason,
			&sess.Fbclid, &sess.Fbc, &sess.Fbp, &sess.Igshid, &sess.MetaClickID,
			&sess.UTMPlacement, &sess.CampaignName, &sess.AdsetName,
			&sess.AdName, &sess.AdAccountID, &sess.SiteSourceName,
			&sess.VisibleSeconds, &sess.HiddenSeconds, &sess.SessionCount, &sess.LandingPath, &sess.ReferrerHost,
			&sess.IsNewVisitor, &sess.HasVideoPlay, &sess.HasZaloClick, &sess.HasTelClick,
			&sess.FormSubmitAttempts, &sess.FormValidationError,
			&sess.ScrollDepth25, &sess.ScrollDepth50, &sess.ScrollDepth75, &sess.ScrollDepth90, &sess.ScrollDepth100,
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
			COALESCE(device_brand, ''), COALESCE(device_model, ''), COALESCE(device_model_code, ''), COALESCE(device_name, ''),
			COALESCE(os_version, ''), COALESCE(browser_version, ''), COALESCE(browser_engine, ''),
			is_in_app_browser, COALESCE(in_app_type, ''),
			device_memory, device_cores, device_touchpoints, COALESCE(viewport_res, ''),
			screen_color_depth, COALESCE(pixel_ratio, ''), COALESCE(timezone, ''), COALESCE(language, ''),
			COALESCE(platform_info, ''), COALESCE(connection_type, ''), COALESCE(device_state, ''),
			is_vpn, is_proxy, is_tor, COALESCE(vpn_provider, ''), vpn_confidence, COALESCE(vpn_reason, ''),
			COALESCE(fbclid, ''), COALESCE(fbc, ''), COALESCE(fbp, ''), COALESCE(igshid, ''), COALESCE(meta_click_id, ''),
			COALESCE(utm_placement, ''), COALESCE(campaign_name, ''), COALESCE(adset_name, ''),
			COALESCE(ad_name, ''), COALESCE(ad_account_id, ''), COALESCE(site_source_name, ''),
			visible_seconds, hidden_seconds, session_count, COALESCE(landing_path, ''), COALESCE(referrer_host, ''),
			is_new_visitor, has_video_play, has_zalo_click, has_tel_click,
			form_submit_attempts, form_validation_errors,
			scroll_depth_25, scroll_depth_50, scroll_depth_75, scroll_depth_90, scroll_depth_100,
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
		&sess.DeviceBrand, &sess.DeviceModel, &sess.DeviceModelCode, &sess.DeviceName,
		&sess.OSVersion, &sess.BrowserVersion, &sess.BrowserEngine,
		&sess.IsInAppBrowser, &sess.InAppType,
		&sess.DeviceMemory, &sess.DeviceCores, &sess.DeviceTouchpoints, &sess.ViewportRes,
		&sess.ScreenColorDepth, &sess.PixelRatio, &sess.Timezone, &sess.Language,
		&sess.PlatformInfo, &sess.ConnectionType, &sess.DeviceState,
		&sess.IsVPN, &sess.IsProxy, &sess.IsTor, &sess.VPNProvider, &sess.VPNConfidence, &sess.VPNReason,
		&sess.Fbclid, &sess.Fbc, &sess.Fbp, &sess.Igshid, &sess.MetaClickID,
		&sess.UTMPlacement, &sess.CampaignName, &sess.AdsetName,
		&sess.AdName, &sess.AdAccountID, &sess.SiteSourceName,
		&sess.VisibleSeconds, &sess.HiddenSeconds, &sess.SessionCount, &sess.LandingPath, &sess.ReferrerHost,
		&sess.IsNewVisitor, &sess.HasVideoPlay, &sess.HasZaloClick, &sess.HasTelClick,
		&sess.FormSubmitAttempts, &sess.FormValidationError,
		&sess.ScrollDepth25, &sess.ScrollDepth50, &sess.ScrollDepth75, &sess.ScrollDepth90, &sess.ScrollDepth100,
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
func (s *Store) GetGeoAnalytics(landingID string, nguon string, fromDate string, toDate string, fromHour int, toHour int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate, 0, fromHour, toHour)

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
func (s *Store) GetAdPlacementAnalytics(landingID string, nguon string, fromDate string, toDate string, fromHour int, toHour int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	whereClause, args := buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate, 0, fromHour, toHour)

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


