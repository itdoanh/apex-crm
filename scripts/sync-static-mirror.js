/**
 * Đồng bộ `backend/static/` từ `frontend/admin/` — nguồn chuẩn theo Rule #8.
 * Dùng Node (Buffer) để giữ nguyên encoding UTF-8, tránh mojibake do
 * PowerShell 5.1 Get-Content/Set-Content.
 *
 * Usage: node scripts/sync-static-mirror.js
 */
const fs = require('fs');
const path = require('path');

const SRC = 'frontend/admin';
const DST = 'backend/static';

/** Bảng ánh xạ file nguồn → file đích trong backend/static. */
const MAP = [
  ['index.html', 'index.html'],
  ['index.html', 'admin.html'],
  ['studio.html', 'studio.html'],
  ['config.js', 'config.js'],
  ['ladihtml.sdk.min.js', 'ladihtml.sdk.min.js'],
  ['css/admin.css', 'css/admin.css'],
  ['css/base.css', 'css/base.css'],
  ['css/crm.css', 'css/crm.css'],
  ['css/layout.css', 'css/layout.css'],
  ['css/studio.css', 'css/studio.css'],
  ['js/analytics.js', 'js/analytics.js'],
  ['js/app.js', 'js/app.js'],
  ['js/auth.js', 'js/auth.js'],
  ['js/crm.js', 'js/crm.js'],
  ['js/domains.js', 'js/domains.js'],
  ['js/landings.js', 'js/landings.js'],
  ['js/security.js', 'js/security.js'],
  ['js/sheets.js', 'js/sheets.js'],
  ['js/tracker.js', 'js/tracker.js'],
  ['tracker.js', 'tracker.js'],
  ['js/studio/ast-mapper.js', 'js/studio/ast-mapper.js'],
  ['js/studio/context-menu.js', 'js/studio/context-menu.js'],
  ['js/studio/file-manager.js', 'js/studio/file-manager.js'],
  ['js/studio/form-scanner.js', 'js/studio/form-scanner.js'],
  ['js/studio/gizmo.js', 'js/studio/gizmo.js'],
  ['js/studio/history.js', 'js/studio/history.js'],
  ['js/studio/inspector.js', 'js/studio/inspector.js'],
  ['js/studio/layers.js', 'js/studio/layers.js'],
];

for (const [from, to] of MAP) {
  const src = path.join(SRC, from);
  const dst = path.join(DST, to);
  if (!fs.existsSync(src)) {
    console.error(`SKIP (missing source): ${src}`);
    continue;
  }
  const content = fs.readFileSync(src);
  fs.mkdirSync(path.dirname(dst), { recursive: true });
  fs.writeFileSync(dst, content);
  console.log(`${src} -> ${dst}`);
}

// tracker.js là file ĐƯỢC SERVE cho landing page (Edge Router + /p/:page_id).
// Nguồn chuẩn duy nhất: frontend/admin/js/tracker.js.
// Phải đồng bộ sang TẤT CẢ các bản còn lại, nếu không landing sẽ chạy code cũ
// và fix landing_id sẽ "không có tác dụng" trên production.
//
// LƯU Ý: file `frontend/landing/tracker.js` và `tracker.js` ở thư mục gốc cũng
// được `SyncSharedStaticFilesToDB()` nạp vào DB lúc backend khởi động.
const TRACKER_SRC = path.join(SRC, 'js', 'tracker.js');
const TRACKER_COPIES = [
  path.join(SRC, 'tracker.js'),
  path.join('frontend', 'landing', 'tracker.js'),
  'tracker.js',
];

if (fs.existsSync(TRACKER_SRC)) {
  const trackerContent = fs.readFileSync(TRACKER_SRC);
  for (const dst of TRACKER_COPIES) {
    fs.mkdirSync(path.dirname(dst), { recursive: true });
    fs.writeFileSync(dst, trackerContent);
    console.log(`${TRACKER_SRC} -> ${dst}`);
  }
}

console.log('done');
