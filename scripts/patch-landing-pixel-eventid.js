/**
 * Bổ sung `eventID` dùng chung Pixel ↔ CAPI cho các block Meta Pixel hardcode
 * trong HTML landing (fix 2026-11-16 — dedup PageView).
 *
 * Meta CHỈ gộp sự kiện browser (`fbq`) với sự kiện server (CAPI) khi cả hai
 * dùng CHUNG `event_id`. ⇒ `fbq('track','PageView',{},{eventID: pvEventId})`
 * và `pvEventId` phải được gửi lên server (tracker.js đọc
 * `window.APEX_PV_EVENT_ID`).
 */
const fs = require('fs');
const path = require('path');

const FILES = [
  'frontend/landing/index.html',
  'frontend/landing/ladi.html',
  'frontend/landing/chucmung.html',
  'frontend/landing/doanh_chung/chung.html',
  'ladi.html',
  'chucmung.html',
];

// Khối sinh event_id — chèn NGAY TRƯỚC lệnh `fbq('init', ...)`.
const GENERATOR = `
        // === [APEX PV EVENT ID] — event_id dùng chung Pixel ↔ CAPI (dedup PageView) ===
        // Meta chỉ gộp sự kiện trình duyệt (fbq) với sự kiện máy chủ (CAPI)
        // khi cả hai dùng CHUNG event_id. Giá trị này được tracker.js gửi lên
        // server (payload analytics "event_id") ⇒ server truyền nguyên văn vào
        // trường "event_id" của payload CAPI ⇒ Meta gộp 2 sự kiện thành 1.
        window.APEX_PV_EVENT_ID = window.APEX_PV_EVENT_ID || ('pv_' + Date.now() + '_' + Math.random().toString(36).substring(2, 9));
        var pvEventId = window.APEX_PV_EVENT_ID;
`;

let changed = 0;
for (const f of FILES) {
  const p = path.join(__dirname, '..', f);
  if (!fs.existsSync(p)) {
    console.error('SKIP (missing): ' + f);
    continue;
  }
  let src = fs.readFileSync(p, 'utf8');
  if (!/fbq\(\s*['"]track['"]\s*,\s*['"]PageView['"]\s*\)/.test(src)) {
    console.log('SKIP (no PageView fbq): ' + f);
    continue;
  }
  if (src.includes('APEX_PV_EVENT_ID')) {
    console.log('SKIP (already patched): ' + f);
    continue;
  }

  // 1) Ghi đè lệnh track PageView để kèm eventID (tham số thứ 4).
  src = src.replace(
    /fbq\(\s*(['"])track\1?\s*,\s*(['"])PageView\2?\s*\)/g,
    "fbq('track', 'PageView', {}, { eventID: window.APEX_PV_EVENT_ID })"
  );

  // 2) Chèn khối sinh event_id trước lệnh `fbq('init', ...)`.
  src = src.replace(
    /([ \t]*)fbq\((['"])init\2/,
    (m, indent) => indent + GENERATOR.replace(/^\n/, '\n') + indent + "fbq('init'"
  );

  fs.writeFileSync(p, src, 'utf8');
  console.log('PATCHED: ' + f);
  changed++;
}
console.log('done, changed=' + changed);
