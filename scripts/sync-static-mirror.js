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
  ['js/app.js', 'js/app.js'],
  ['js/landings.js', 'js/landings.js'],
  ['js/tracker.js', 'js/tracker.js'],
  ['tracker.js', 'tracker.js'],
  ['js/studio/ast-mapper.js', 'js/studio/ast-mapper.js'],
  ['js/studio/file-manager.js', 'js/studio/file-manager.js'],
  ['js/studio/form-scanner.js', 'js/studio/form-scanner.js'],
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
console.log('done');
