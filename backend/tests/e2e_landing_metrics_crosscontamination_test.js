/**
 * Regression test cho bug 2026-11-16: "các landing nhân bản ra vẫn bị chồng
 * chéo dữ liệu thông số như lượt xem, lead" (anh itdoanh báo).
 *
 * NGUYÊN NHÂN (3 tầng, đều dẫn tới ghi analytics/lead về SAI landing):
 *
 *   [1] SERVER — `ResolveRoute` KHÔNG khớp `/chucmung.html` với route prefix
 *       `/chucmung` ⇒ `found = false` ⇒ Edge Router phục vụ file HTML TĨNH
 *       từ đĩa ⇒ KHÔNG có `data-landing-id` / `window.APEX_LANDING_CONFIG`.
 *
 *   [2] CLIENT — `tracker.js` có 2 fallback nguy hiểm:
 *       (a) `loadConfigFile()` fallback cứng `'mxv_080_online'` (template dùng
 *           chung) ⇒ mọi trang không có định danh đều ghi về template.
 *       (b) `getLandingConfig()` fallback `window.LadiPageScript.runtime.ladipage_id`
 *           ⇒ sinh landing ID RÁC (vd `6adc99f23115ac001247de00`) không khớp
 *           landing nào trong CRM.
 *
 *   [3] METRICS — tab Quản lý Landing tính CR theo TỔNG sessions (gồm bot),
 *       còn Trung Tâm Báo Cáo tính theo `human_visitors` ⇒ 2 tab lệch nhau.
 *
 * Chạy: node backend/tests/e2e_landing_metrics_crosscontamination_test.js
 */

const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..', '..');
const TRACKER_PATH = path.join(ROOT, 'frontend', 'admin', 'js', 'tracker.js');
const DOMAIN_CACHE_PATH = path.join(ROOT, 'backend', 'cache', 'domain_cache.go');
const LANDINGS_GO_PATH = path.join(ROOT, 'backend', 'db', 'landings.go');
const LANDING_MODEL_PATH = path.join(ROOT, 'backend', 'models', 'landing.go');
const LANDINGS_JS_PATH = path.join(ROOT, 'frontend', 'admin', 'js', 'landings.js');

const trackerSrc = fs.readFileSync(TRACKER_PATH, 'utf8');
const domainCacheSrc = fs.readFileSync(DOMAIN_CACHE_PATH, 'utf8');
const landingsGoSrc = fs.readFileSync(LANDINGS_GO_PATH, 'utf8');
const landingModelSrc = fs.readFileSync(LANDING_MODEL_PATH, 'utf8');
const landingsJsSrc = fs.readFileSync(LANDINGS_JS_PATH, 'utf8');

let failures = 0;
function check(name, cond, detail) {
  if (cond) {
    console.log('  [OK]   ' + name);
  } else {
    failures++;
    console.log('  [FAIL] ' + name + (detail ? ' — ' + detail : ''));
  }
}

console.log('=== Regression test: chồng chéo thông số landing (bug 2026-11-16) ===\n');

console.log('[1] SERVER — ResolveRoute khớp URL dạng `/<tên>.html` với route prefix `/<tên>`:');
check('domain_cache.go có xử lý hậu tố `.html`',
  /strings\.HasSuffix\(cleanPath, "\.html"\)/.test(domainCacheSrc));
check('có so khớp `htmlBase == pfx`',
  /htmlBase\s*==\s*pfx/.test(domainCacheSrc));
check('trả về subPath kèm đuôi .html để Edge Router vào nhánh có inject',
  /return r\.LandingID, pfx, strings\.TrimPrefix\(cleanPath, "\/"\), r\.RedirectURL, true/.test(domainCacheSrc));

console.log('\n[2] SERVER — fallback tĩnh KHÔNG được phục vụ HTML không có landing context:');
const mainGoSrc = fs.readFileSync(path.join(ROOT, 'backend', 'main.go'), 'utf8');
check('main.go có chặn phục vụ HTML tĩnh thô',
  /Từ chối phục vụ HTML tĩnh không có landing context/.test(mainGoSrc));
check('main.go inject dynamic scripts cho HTML tĩnh khi có landing context',
  /InjectDynamicScriptsWithBase\(string\(data\), cmLanding, "\/"\)/.test(mainGoSrc));

console.log('\n[3] CLIENT — tracker.js KHÔNG fallback sang landing ID của template:');
check("không còn fallback cứng 'mxv_080_online' trong loadConfigFile",
  !/currentLandingId\s*=\s*'mxv_080_online'/.test(trackerSrc));
check('loadConfigFile dừng lại khi không xác định được landing',
  /if \(!currentLandingId\) \{\s*return window\.APEX_LANDING_CONFIG \|\| \{\};/.test(trackerSrc));

console.log('\n[4] CLIENT — tracker.js KHÔNG dùng `ladipage_id` (ID nội bộ LadiPage) làm landing_id:');
check('không còn fallback window.LadiPageScript.runtime.ladipage_id',
  !/window\.LadiPageScript && window\.LadiPageScript\.runtime && window\.LadiPageScript\.runtime\.ladipage_id/.test(trackerSrc));
check('fallback cuối là default_landing',
  /urlLandingId \|\|\s*'default_landing'/.test(trackerSrc));

console.log('\n[5] MIRROR — 7 bản tracker.js đã đồng bộ:');
const TRACKER_MIRRORS = [
  'frontend/admin/js/tracker.js',
  'frontend/admin/tracker.js',
  'frontend/landing/tracker.js',
  'tracker.js',
  'backend/static/tracker.js',
  'backend/static/js/tracker.js',
  'backend/tracker.js',
];
const trackerHash = cryptoHash(trackerSrc);
for (const rel of TRACKER_MIRRORS) {
  const p = path.join(ROOT, rel);
  let ok = false;
  try {
    ok = cryptoHash(fs.readFileSync(p, 'utf8')) === trackerHash;
  } catch (_) {
    ok = false;
  }
  check('mirror khớp: ' + rel, ok);
}

console.log('\n[6] METRICS — tách human/bot views + danh sách domain:');
check('models.LandingPage có HumanViewsCount', /HumanViewsCount\s+int/.test(landingModelSrc));
check('models.LandingPage có BotViewsCount', /BotViewsCount\s+int/.test(landingModelSrc));
check('models.LandingPage có Domains []string', /Domains\s+\[\]string/.test(landingModelSrc));
check('ListLandingPages query có human_views_count',
  /human_views_count/.test(landingsGoSrc));
check('ListLandingPages query có bot_views_count',
  /bot_views_count/.test(landingsGoSrc));
check('có hàm lấy domain deploy theo landing', /GetLandingDeployDomains/.test(landingsGoSrc));

console.log('\n[7] UI — CR tính theo lượt xem người thật (khớp Trung Tâm Báo Cáo):');
check('landings.js dùng human_views_count cho CR',
  /p\.human_views_count/.test(landingsJsSrc));
check('landings.js hiển thị danh sách domains',
  /p\.domains/.test(landingsJsSrc));

console.log('\n[8] METRICS — UpdateLandingCode / ToggleLandingActive KHÔNG đè bản gốc:');
check('UpdateLandingCode dùng `WHERE domain = $3 AND page_id = $4` (không `OR domain = \'\'`)',
  /WHERE domain = \$3 AND page_id = \$4/.test(landingsGoSrc));
check('có EnsureLandingPageDomainRow tạo row riêng theo domain',
  /EnsureLandingPageDomainRow/.test(landingsGoSrc));

console.log('\n' + (failures === 0
  ? '=== TẤT CẢ PASS ==='
  : `=== CÓ ${failures} LỖI ===`));
process.exit(failures === 0 ? 0 : 1);

// ─── helper ────────────────────────────────────────────────────────────────
function cryptoHash(str) {
  const crypto = require('crypto');
  return crypto.createHash('sha256').update(str).digest('hex');
}
