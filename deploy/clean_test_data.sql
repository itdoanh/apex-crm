-- ==============================================================
-- KỊCH BẢN DỌN DẸP SẠCH DỮ LIỆU THỬ NGHIỆM (CLEAN TEST DATA)
-- Khởi tạo hệ thống sẵn sàng đưa vào vận hành sản xuất (Production)
--
-- RÀNG BUỘC AN TOÀN TUYỆT ĐỐI (BẢO VỆ NGUYÊN VẸN):
-- 1. Giữ nguyên Landing Page chính: 'mxv_080_online' (Toàn bộ HTML & tệp tin)
-- 2. Giữ nguyên Tài khoản Admin duy nhất: 'admin' (Mật khẩu Argon2id: ApexAdmin@2026!)
-- 3. Giữ nguyên Cấu hình tên miền chuẩn: 'hanghoaphaisinh.net' và 'dautuhanghoa.vn'
-- ==============================================================

BEGIN;

-- 1. Xóa sạch dữ liệu thử nghiệm phát sinh trong quá trình phát triển & test
TRUNCATE TABLE leads RESTART IDENTITY CASCADE;
TRUNCATE TABLE analytics_sessions RESTART IDENTITY CASCADE;
TRUNCATE TABLE spam_quarantine RESTART IDENTITY CASCADE;
TRUNCATE TABLE google_sheet_sync_logs RESTART IDENTITY CASCADE;
TRUNCATE TABLE google_sheet_configs RESTART IDENTITY CASCADE;
TRUNCATE TABLE admin_audit_logs RESTART IDENTITY CASCADE;
TRUNCATE TABLE admin_sessions RESTART IDENTITY CASCADE;

-- 2. Dọn dẹp các tên miền và route thử nghiệm (chỉ giữ lại 2 tên miền chuẩn)
DELETE FROM domain_routes
WHERE domain_id NOT IN (
    SELECT id FROM domains WHERE domain IN ('hanghoaphaisinh.net', 'dautuhanghoa.vn')
) OR landing_id != 'mxv_080_online';

DELETE FROM domains
WHERE domain NOT IN ('hanghoaphaisinh.net', 'dautuhanghoa.vn');

-- Cập nhật trạng thái tên miền chuẩn
UPDATE domains
SET dns_status = 'pending',
    ssl_status = 'active',
    default_landing_id = 'mxv_080_online',
    updated_at = NOW()
WHERE domain IN ('hanghoaphaisinh.net', 'dautuhanghoa.vn');

-- Đảm bảo các route chuẩn cho 2 tên miền tồn tại
INSERT INTO domain_routes (domain_id, path_prefix, landing_id, redirect_url, is_active, deployed_at)
SELECT d.id, '/', 'mxv_080_online', '', true, NOW()
FROM domains d
WHERE d.domain = 'hanghoaphaisinh.net'
  AND NOT EXISTS (
      SELECT 1 FROM domain_routes r WHERE r.domain_id = d.id AND r.path_prefix = '/'
  );

INSERT INTO domain_routes (domain_id, path_prefix, landing_id, redirect_url, is_active, deployed_at)
SELECT d.id, '/', 'mxv_080_online', '', true, NOW()
FROM domains d
WHERE d.domain = 'dautuhanghoa.vn'
  AND NOT EXISTS (
      SELECT 1 FROM domain_routes r WHERE r.domain_id = d.id AND r.path_prefix = '/'
  );

INSERT INTO domain_routes (domain_id, path_prefix, landing_id, redirect_url, is_active, deployed_at)
SELECT d.id, '/ads', 'mxv_080_online', '', true, NOW()
FROM domains d
WHERE d.domain = 'dautuhanghoa.vn'
  AND NOT EXISTS (
      SELECT 1 FROM domain_routes r WHERE r.domain_id = d.id AND r.path_prefix = '/ads'
  );

-- 3. Đảm bảo chỉ giữ lại landing page chuẩn 'mxv_080_online'
DELETE FROM landing_pages WHERE page_id != 'mxv_080_online';
DELETE FROM landing_files WHERE page_id != 'mxv_080_online';

-- 4. Đảm bảo chỉ giữ duy nhất tài khoản admin
DELETE FROM admin_users WHERE username != 'admin';

-- 5. Ghi nhận log khởi tạo hệ thống sản xuất
INSERT INTO admin_audit_logs (
    admin_id, username, action, resource, resource_id,
    status, details, ip_address, user_agent,
    device_type, browser, os, created_at
) VALUES (
    (SELECT id FROM admin_users WHERE username = 'admin' LIMIT 1),
    'admin',
    'SYSTEM_CLEAN_INIT',
    'system',
    'production',
    'success',
    '{"message": "Dọn dẹp sạch dữ liệu thử nghiệm thành công. Hệ thống Apex sẵn sàng cho Production."}'::jsonb,
    '127.0.0.1',
    'System Initializer (Windows Server 2025 Deployer)',
    'server',
    'internal',
    'Windows Server 2025',
    NOW()
);

COMMIT;

-- Giải phóng dung lượng đĩa và tối ưu hóa chỉ mục
VACUUM FULL;
