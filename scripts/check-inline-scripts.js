/**
 * Kiểm tra syntax của các thẻ <script> inline trong file HTML landing.
 * Dùng Node: extract nội dung script → `new Function(...)` để parse.
 *
 * Usage: node scripts/check-inline-scripts.js frontend/landing/index.html frontend/landing/ladi.html
 */
const fs = require('fs');

const files = process.argv.slice(2);
let failed = 0;

for (const file of files) {
  const html = fs.readFileSync(file, 'utf8');
  const re = /<script\b([^>]*)>([\s\S]*?)<\/script>/gi;
  let m, idx = 0;
  while ((m = re.exec(html)) !== null) {
    const attrs = m[1] || '';
    const body = m[2] || '';
    if (/\bsrc\s*=/i.test(attrs)) { idx++; continue; }
    if (!body.trim()) { idx++; continue; }
    const line = html.slice(0, m.index).split('\n').length;
    try {
      new Function(body);
      console.log(`  ✅ script #${idx} (line ~${line}) OK`);
    } catch (e) {
      failed++;
      console.log(`  ❌ script #${idx} (line ~${line}) LỖI: ${e.message}`);
    }
    idx++;
  }
  console.log(`--- ${file}: ${idx} script block ---`);
}

process.exit(failed === 0 ? 0 : 1);
