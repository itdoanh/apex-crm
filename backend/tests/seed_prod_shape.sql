-- ============================================================================
-- ⚠️⚠️⚠️ CẢNH BÁO — FILE NÀY XOÁ DỮ LIỆU THẬT ⚠️⚠️⚠️
--
-- Script này chạy `DELETE FROM domain_routes / landing_files / landing_pages`
-- rồi tạo lại dữ liệu GIẢ LẬP (landing_cu, landing_lznoew, mxv_080_online).
--
-- CHỈ CHẠY TRÊN DATABASE LOCAL / DB TEST. TUYỆT ĐỐI KHÔNG chạy trên
-- production (apex_leads @ 103.124.95.232) — sẽ mất toàn bộ landing page,
-- file tĩnh và định tuyến domain đang chạy quảng cáo thật.
--
-- Dùng cho: mô phỏng lại hình dạng dữ liệu production để test lỗi
-- tracker.js 404 + <base href> đặt sai chỗ.
-- ============================================================================

-- Dựng lại SỐ LƯỢNG GIỐNG production để test lỗi tracker.js 404 + base path.
-- Mục tiêu: landing HTML có <base href="/landing/"> ĐẶT SAU <script src="tracker.js">,
-- phục vụ tại route /p/landing_cu ⇒ trước fix sẽ gọi /p/tracker.js (404).

DELETE FROM domain_routes;
DELETE FROM landing_files;
DELETE FROM landing_pages;

-- Landing chính (giống production landing_cu)
INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES (
  'landing_cu', 'ads.giaodichhanghoatuonglai.com', 'Landing CU Test', true, 'published', '', 'landing_chinh',
  '{}',
  $html$
<!DOCTYPE html><html lang="vi"><head>
<title>Landing CU Test</title>
<script src="tracker.js" data-landing-id="landing_cu"></script>
<link href="https://fonts.googleapis.com/css2?family=Be+Vietnam+Pro&display=swap" rel="stylesheet">
<script>
window.APEX_LANDING_CONFIG = { landing_id: 'landing_cu', nguon: 'landing_chinh' };
</script>
<base href="/landing/">
</head><body>
<h1>Đăng ký lớp học</h1>
<form id="registrationForm" onsubmit="handleFormSubmit(event)">
  <input id="fullName" type="text" required placeholder="Họ và tên">
  <input id="phoneNumber" type="tel" required placeholder="Số điện thoại">
  <button type="submit">ĐĂNG KÝ</button>
</form>
<script>
function validateRegistrationForm() {
  var n = document.getElementById('fullName');
  var p = document.getElementById('phoneNumber');
  if (!n || !n.value.trim()) return false;
  if (!p || !/^0\d{9}$/.test(p.value.replace(/\D/g,''))) return false;
  return true;
}
document.addEventListener('submit', function (e) {
  var form = e.target;
  if (!form || form.id !== 'registrationForm') return;
  if (!validateRegistrationForm()) { e.preventDefault(); return; }
  e.preventDefault();
  if (window.ApexTracker && typeof window.ApexTracker.sendLeadToBackend === 'function') {
    var nameInp = document.getElementById('fullName');
    var phoneInp = document.getElementById('phoneNumber');
    window.ApexTracker.sendLeadToBackend({
      fullName: nameInp ? nameInp.value.trim() : '',
      phoneNumber: phoneInp ? phoneInp.value.trim() : ''
    });
  }
}, true);
function handleFormSubmit(e) { return true; }
</script>
</body></html>
$html$,
  NOW(), NOW()
);

INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES (
  'landing_lznoew', 'ads.giaodichhanghoatuonglai.com', 'Landing LZ Test', true, 'published', '', 'landing_chinh',
  '{}',
  $html2$
<!DOCTYPE html><html lang="vi"><head>
<title>Landing LZ Test</title>
<script src="tracker.js" data-landing-id="landing_lznoew"></script>
<script>
window.APEX_LANDING_CONFIG = { landing_id: 'landing_lznoew', nguon: 'landing_chinh' };
</script>
<base href="/chucmung/">
</head><body>
<h1>Đăng ký lớp học (LZ)</h1>
<form id="registrationForm" onsubmit="handleFormSubmit(event)">
  <input id="fullName" type="text" required placeholder="Họ và tên">
  <input id="phoneNumber" type="tel" required placeholder="Số điện thoại">
  <button type="submit">ĐĂNG KÝ</button>
</form>
<script>
function validateRegistrationForm() {
  var n = document.getElementById('fullName');
  var p = document.getElementById('phoneNumber');
  if (!n || !n.value.trim()) return false;
  if (!p || !/^0\d{9}$/.test(p.value.replace(/\D/g,''))) return false;
  return true;
}
document.addEventListener('submit', function (e) {
  var form = e.target;
  if (!form || form.id !== 'registrationForm') return;
  if (!validateRegistrationForm()) { e.preventDefault(); return; }
  e.preventDefault();
  if (window.ApexTracker && typeof window.ApexTracker.sendLeadToBackend === 'function') {
    var nameInp = document.getElementById('fullName');
    var phoneInp = document.getElementById('phoneNumber');
    window.ApexTracker.sendLeadToBackend({
      fullName: nameInp ? nameInp.value.trim() : '',
      phoneNumber: phoneInp ? phoneInp.value.trim() : ''
    });
  }
}, true);
function handleFormSubmit(e) { return true; }
</script>
</body></html>
$html2$,
  NOW(), NOW()
);

INSERT INTO landing_pages (page_id, domain, name, is_active, status, redirect_url, nguon_default, settings, code_html, created_at, updated_at)
VALUES (
  'mxv_080_online', 'hanghoaphaisinh.net', 'MXV 080', true, 'published', '', 'landing_chinh',
  '{}', '<!DOCTYPE html><html><head><title>MXV</title></head><body>x</body></html>', NOW(), NOW()
);

-- Domain + routes
INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id, notes, created_at, updated_at)
VALUES ('ads.giaodichhanghoatuonglai.com', 'ns1.hanghoaphaisinh.net', 'ns2.hanghoaphaisinh.net', '127.0.0.1', 'active', 'pending', 'landing_lznoew', '', NOW(), NOW())
ON CONFLICT (domain) DO NOTHING;

INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/', 'landing_lznoew', true, '', NOW() FROM domains d WHERE d.domain = 'ads.giaodichhanghoatuonglai.com';

INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/landing', 'landing_cu', true, '', NOW() FROM domains d WHERE d.domain = 'ads.giaodichhanghoatuonglai.com';

INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
SELECT d.id, '/', 'mxv_080_online', true, '', NOW() FROM domains d WHERE d.domain = 'hanghoaphaisinh.net';
