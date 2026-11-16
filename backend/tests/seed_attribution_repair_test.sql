-- ============================================================================
-- Dữ liệu GIẢ LẬP mô phỏng đúng hình dạng production (để test migration).
-- ⚠️ CHỈ CHẠY TRÊN DB LOCAL/TEST — KHÔNG chạy trên production.
-- ============================================================================

DELETE FROM domain_routes;
DELETE FROM landing_files;
DELETE FROM landing_pages;
DELETE FROM analytics_sessions;
DELETE FROM leads;

-- 1) landing_cu (nội dung giống landing_232 — cùng md5)
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES ('landing_cu', 'ads.giaodichhanghoatuonglai.com', 'Landing CU', true, 'published', '', 'landing_cu', '{}',
  '<!DOCTYPE html><html><head><title>CU</title><script src="tracker.js" data-landing-id="landing_cu"></script></head><body>CU</body></html>', NOW(), NOW());

-- 2) landing_232 (BẢN NHÂN BẢN tạo 2026-11-11 — KHÔNG phải bản đổi tên)
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES ('landing_232', 'lophoc.hanghoaphaisinh.net', 'Landing 232', false, 'published', '', 'landing_232', '{}',
  '<!DOCTYPE html><html><head><title>CU</title><script src="tracker.js" data-landing-id="landing_cu"></script></head><body>CU</body></html>', NOW(), NOW());

-- 3) landing_lznoew (route /chucmung)
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES ('landing_lznoew', 'ads.giaodichhanghoatuonglai.com', 'Landing LZ', true, 'published', '', 'landing_lznoew', '{}',
  '<!DOCTYPE html><html><head><title>LZ</title><script src="tracker.js" data-landing-id="landing_lznoew"></script></head><body>LZ</body></html>', NOW(), NOW());

-- 4) doanh_chung (route / của lophoc)
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES ('doanh_chung', '', 'Doanh Chung', true, 'published', '', 'doanh_chung', '{}',
  '<!DOCTYPE html><html><head><title>DC</title><script src="tracker.js" data-landing-id="doanh_chung"></script></head><body>DC</body></html>', NOW(), NOW());

-- 5) mxv_080_online (TEMPLATE — KHÔNG active)
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES ('mxv_080_online', '', 'MXV 080', false, 'published', '', 'mxv_080_online', '{}',
  '<!DOCTYPE html><html><head><title>MXV</title></head><body>MXV</body></html>', NOW(), NOW());

-- 6) xu_huong_1
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES ('xu_huong_1', '', 'Xu Huong 1', true, 'published', '', 'xu_huong_1', '{}',
  '<!DOCTYPE html><html><head><title>XH1</title><script src="tracker.js" data-landing-id="xu_huong_1"></script></head><body>XH1</body></html>', NOW(), NOW());

-- Domains + routes
INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id, notes, created_at, updated_at)
VALUES ('ads.giaodichhanghoatuonglai.com', 'ns1.hanghoaphaisinh.net', 'ns2.hanghoaphaisinh.net', '127.0.0.1', 'active', 'pending', 'mxv_080_online', '', NOW(), NOW())
ON CONFLICT (domain) DO NOTHING;
INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id, notes, created_at, updated_at)
VALUES ('lophoc.hanghoaphaisinh.net', 'ns1.hanghoaphaisinh.net', 'ns2.hanghoaphaisinh.net', '127.0.0.1', 'active', 'pending', 'mxv_080_online', '', NOW(), NOW())
ON CONFLICT (domain) DO NOTHING;
INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id, notes, created_at, updated_at)
VALUES ('xuhuong.hanghoaphaisinh.net', 'ns1.hanghoaphaisinh.net', 'ns2.hanghoaphaisinh.net', '127.0.0.1', 'active', 'pending', 'mxv_080_online', '', NOW(), NOW())
ON CONFLICT (domain) DO NOTHING;

INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/chucmung', 'landing_lznoew', true, '', NOW() FROM domains d WHERE d.domain = 'ads.giaodichhanghoatuonglai.com';
INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/landing', 'landing_cu', true, '', NOW() FROM domains d WHERE d.domain = 'ads.giaodichhanghoatuonglai.com';
INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/', 'doanh_chung', true, '', NOW() FROM domains d WHERE d.domain = 'lophoc.hanghoaphaisinh.net';
INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/', 'xu_huong_1', true, '', NOW() FROM domains d WHERE d.domain = 'xuhuong.hanghoaphaisinh.net';

-- ─────────────────────────────────────────────────────────────
-- Sessions mô phỏng đúng dữ liệu production
-- ─────────────────────────────────────────────────────────────
INSERT INTO analytics_sessions (session_id, visitor_id, landing_id, nguon, entry_url, is_bot, converted_lead, first_seen, last_seen)
VALUES
 ('s1','v1','mxv_080_online','mxv_080_online','https://ads.giaodichhanghoatuonglai.com/chucmung.html?utm_medium=paid&utm_source=an',false,false, NOW(), NOW()),
 ('s2','v2','mxv_080_online','mxv_080_online','https://ads.giaodichhanghoatuonglai.com/chucmung.html',false,false, NOW(), NOW()),
 ('s3','v3','mxv_080_online','mxv_080_online','https://ads.giaodichhanghoatuonglai.com/landing?utm_source=zalo',false,true, NOW(), NOW()),
 ('s4','v4','mxv_080_online','mxv_080_online','https://ads.giaodichhanghoatuonglai.com/landing/chucmung.html',false,false, NOW(), NOW()),
 ('s5','v5','mxv_080_online','mxv_080_online','about:srcdoc',false,false, NOW(), NOW()),
 ('s6','v6','mxv_080_online','mxv_080_online','https://lophoc.hanghoaphaisinh.net/landing',false,false, NOW(), NOW()),
 ('s7','v7','6adc99f23115ac001247de00','6adc99f23115ac001247de00','https://ads.giaodichhanghoatuonglai.com/chucmung.html',false,false, NOW(), NOW()),
 ('s8','v8','landing_232','landing_232','https://ads.giaodichhanghoatuonglai.com/chucmung.html',false,false, NOW(), NOW()),
 ('s9','v9','landing_cu','an','https://ads.giaodichhanghoatuonglai.com/landing',false,false, NOW(), NOW()),
 ('s10','v10','default_landing','default_landing','https://x.com/',false,false, NOW(), NOW()),
 ('s11','v11','landing_232','landing_232','https://lophoc.hanghoaphaisinh.net/',false,true, NOW(), NOW()),
 ('s12','v12','landing_232','landing_232','',false,false, NOW(), NOW());

INSERT INTO leads (thoi_gian, landing_id, nguon, so_dien_thoai, tracking, url)
VALUES
 (NOW(), 'mxv_080_online','mxv_080_online','0900000001','{}','https://ads.giaodichhanghoatuonglai.com/landing?zarsrc=31'),
 (NOW(), 'mxv_080_online','mxv_080_online','0900000002','{}','https://ads.giaodichhanghoatuonglai.com/landing/ads.giaodichhanghoatuonglai.com/chucmung'),
 (NOW(), 'landing_232','landing_232','0900000003','{}','https://ads.giaodichhanghoatuonglai.com/'),
 (NOW(), 'landing_232','landing_232','0900000005','{}','https://lophoc.hanghoaphaisinh.net/'),
 (NOW(), 'landing_cu','landing_cu','0900000004','{}','https://ads.giaodichhanghoatuonglai.com/landing');
