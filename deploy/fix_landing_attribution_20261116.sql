-- ==============================================================
-- MIGRATION: SỬA THÔNG SỐ LANDING BỊ CHỒNG CHÉO (2026-11-16)
--
-- BỐI CẢNH (anh itdoanh báo):
--   "lỗi ở phần ql landing thì các landing tôi nhân bản ra vẫn bị chồng chéo
--    dữ liệu thông số như lượt xem, lead trong tab đấy k chính xác"
--
-- NGUYÊN NHÂN GỐC (đã xác minh trên production 2026-11-16):
--
--   [A] `ResolveRoute` không khớp `/chucmung.html` với route prefix `/chucmung`
--       ⇒ trang chúc mừng bị phục vụ TĨNH từ đĩa, KHÔNG có `data-landing-id`
--       ⇒ tracker.js rơi về fallback ⇒ ghi về `mxv_080_online`.
--       ⇒ ĐÃ SỬA Ở CODE (backend/cache/domain_cache.go).
--
--   [B] tracker.js fallback `ladipage_id` (ID nội bộ LadiPage) ⇒ sinh landing
--       ID rác `6adc99f23115ac001247de00` (21 sessions).
--       ⇒ ĐÃ SỬA Ở CODE (frontend/admin/js/tracker.js + 6 bản mirror).
--
--   ⇒ Hệ quả tồn đọng trong DB: dữ liệu LỊCH SỬ đã ghi nhầm landing.
--     Script này sửa lại dữ liệu đã ghi nhầm.
--
-- ⚠️ AN TOÀN:
--   - KHÔNG xoá dữ liệu, chỉ đổi `landing_id`/`nguon` của bản ghi sai.
--   - BẢNG BACKUP được tạo trước khi sửa (`*_backup_20261116`).
--   - Idempotent: chạy lại nhiều lần vẫn an toàn.
--   - CHẠY THỬ TRÊN MÔI TRƯỜNG TEST TRƯỚC KHI ÁP DỤNG PRODUCTION.
-- ==============================================================

BEGIN;

-- ─────────────────────────────────────────────────────────────
-- BƯỚC 0: BACKUP
-- ─────────────────────────────────────────────────────────────
-- ⚠️ Bảng backup PHẢI có PRIMARY KEY, nếu không `ON CONFLICT DO NOTHING`
--    sẽ không có tác dụng (bảng tạo bằng CREATE TABLE AS SELECT không kế
--    thừa constraint) ⇒ chạy lại script lần 2 sẽ nhân bản dòng backup.
--    Dùng `LIKE ... INCLUDING ALL` để cấu trúc + thứ tự cột KHỚP chính xác
--    với bảng gốc ⇒ `SELECT s.*` insert được an toàn.
CREATE TABLE IF NOT EXISTS analytics_sessions_backup_20261116
    (LIKE analytics_sessions INCLUDING ALL);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'analytics_sessions_backup_20261116_pkey'
          AND conrelid = 'analytics_sessions_backup_20261116'::regclass
    ) THEN
        ALTER TABLE analytics_sessions_backup_20261116
            ADD CONSTRAINT analytics_sessions_backup_20261116_pkey
            PRIMARY KEY (session_id);
    END IF;
END $$;

INSERT INTO analytics_sessions_backup_20261116
    SELECT s.* FROM analytics_sessions s
    WHERE s.landing_id IN ('mxv_080_online', 'landing_232', '6adc99f23115ac001247de00', 'default_landing')
    ON CONFLICT (session_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS leads_backup_20261116
    (LIKE leads INCLUDING ALL);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'leads_backup_20261116_pkey'
          AND conrelid = 'leads_backup_20261116'::regclass
    ) THEN
        ALTER TABLE leads_backup_20261116
            ADD CONSTRAINT leads_backup_20261116_pkey
            PRIMARY KEY (id);
    END IF;
END $$;

INSERT INTO leads_backup_20261116
    SELECT l.* FROM leads l
    WHERE l.landing_id IN ('mxv_080_online', 'landing_232', '6adc99f23115ac001247de00', 'default_landing')
    ON CONFLICT (id) DO NOTHING;

-- ─────────────────────────────────────────────────────────────
-- BƯỚC 1: `landing_232` → `landing_cu`
--
--   BẰNG CHỨNG: `landing_pages.code_html` của `landing_232` và `landing_cu`
--   có CÙNG md5 (`31d23cbec24ad026aa03a3f3e794cf07`) và cùng độ dài (265594)
--   ⇒ `landing_232` là bản NHÂN BẢN của `landing_cu` (copy nguyên si).
--
--   ⚠️ SỬA LẠI sau khi đối chiếu dữ liệu production thật (2026-11-16):
--   `landing_232` KHÔNG phải bản đổi tên — nó được tạo **2026-11-11 16:56**
--   (SAU `landing_cu` 2026-11-04 10:35) ⇒ đây là lần NHÂN BẢN từ CRM.
--   ⇒ **KHÔNG được gộp vào `landing_cu`** (sẽ làm thống kê chồng chéo NGƯỢC lại).
--
--   Phân bổ thật của 47 sessions + 5 leads của `landing_232`:
--     · 31 sessions `ads.giaodichhanghoatuonglai.com/chucmung*` (11-11 → 11-12)
--         ⇒ trang chúc mừng ⇒ `landing_lznoew`
--     · 14 sessions + 4 leads `lophoc.hanghoaphaisinh.net/` (route `/`)
--         ⇒ `doanh_chung`
--     · 1 session + 1 lead `ads.giaodichhanghoatuonglai.com/` (root)
--         ⇒ `landing_232` (giữ nguyên — landing riêng, đang deploy route `/`)
--     · 1 session URL rỗng ⇒ `default_landing`
-- ─────────────────────────────────────────────────────────────

-- (1a) `landing_232` trên `ads.../chucmung*` ⇒ `landing_lznoew`
UPDATE analytics_sessions
SET landing_id = 'landing_lznoew',
    nguon      = 'landing_lznoew'
WHERE landing_id = 'landing_232'
  AND entry_url LIKE 'https://ads.giaodichhanghoatuonglai.com/chucmung%';

-- (1b) `landing_232` trên `lophoc.hanghoaphaisinh.net` ⇒ `doanh_chung`
UPDATE analytics_sessions
SET landing_id = 'doanh_chung',
    nguon      = 'doanh_chung'
WHERE landing_id = 'landing_232'
  AND entry_url LIKE 'https://lophoc.hanghoaphaisinh.net%';

-- (1c) `landing_232` URL rỗng / about:srcdoc ⇒ `default_landing`
UPDATE analytics_sessions
SET landing_id = 'default_landing',
    nguon      = 'default_landing'
WHERE landing_id = 'landing_232'
  AND (entry_url = '' OR entry_url LIKE 'about:%');

-- (1d) LEADS của `landing_232` theo URL thật
UPDATE leads
SET landing_id = 'doanh_chung',
    nguon      = 'doanh_chung'
WHERE landing_id = 'landing_232'
  AND url LIKE 'https://lophoc.hanghoaphaisinh.net%';

UPDATE leads
SET landing_id = 'landing_232',
    nguon      = 'landing_232'
WHERE landing_id = 'landing_232'
  AND url LIKE 'https://ads.giaodichhanghoatuonglai.com%';

-- ⚠️ `landing_232` (row domain `lophoc.hanghoaphaisinh.net`, `is_active = false`)
--    là landing RIÊNG của anh itdoanh ⇒ GIỮ NGUYÊN, không xoá, không gộp.
--    Route `ads.../` → `landing_232` vẫn active trong `domain_routes`.

-- ─────────────────────────────────────────────────────────────
-- BƯỚC 2: `mxv_080_online` → landing thật, theo URL thực tế
--
--   Đây là phần dữ liệu RỜI NHẤT. `mxv_080_online` là TEMPLATE dùng chung,
--   KHÔNG phải landing đang chạy ⇒ mọi bản ghi gán cho nó là sai.
--
--   Quy tắc quyết định từ `entry_url` (URL khách thực sự truy cập):
--
--   a) host `ads.giaodichhanghoatuonglai.com`, path bắt đầu `/chucmung`
--      ⇒ trang CHÚC MỪNG của landing `landing_lznoew` (route `/chucmung`).
--
--   b) host `ads.giaodichhanghoatuonglai.com`, path bắt đầu `/landing`
--      ⇒ landing `landing_cu` (route `/landing`).
--
--   c) host `lophoc.hanghoaphaisinh.net` ⇒ `doanh_chung` (route `/`).
--
--   d) host `xuhuong.hanghoaphaisinh.net` / `daotao.*` ⇒ `xu_huong_1`.
--
--   e) còn lại (kể cả `about:srcdoc` = preview trong CRM) ⇒ `default_landing`
--      (nhóm riêng, dễ nhận diện — KHÔNG gán nhầm cho landing nào).
-- ─────────────────────────────────────────────────────────────

-- a) Trang chúc mừng ⇒ landing_lznoew
UPDATE analytics_sessions
SET landing_id = 'landing_lznoew',
    nguon      = 'landing_lznoew'
WHERE landing_id = 'mxv_080_online'
  AND entry_url LIKE 'https://ads.giaodichhanghoatuonglai.com/chucmung%';

-- b) Route /landing ⇒ landing_cu
UPDATE analytics_sessions
SET landing_id = 'landing_cu',
    nguon      = 'landing_cu'
WHERE landing_id = 'mxv_080_online'
  AND entry_url LIKE 'https://ads.giaodichhanghoatuonglai.com/landing%';

-- c) lophoc.hanghoaphaisinh.net ⇒ doanh_chung
UPDATE analytics_sessions
SET landing_id = 'doanh_chung',
    nguon      = 'doanh_chung'
WHERE landing_id = 'mxv_080_online'
  AND entry_url LIKE 'https://lophoc.hanghoaphaisinh.net%';

-- d) Các domain của xu_huong_1 ⇒ xu_huong_1
UPDATE analytics_sessions
SET landing_id = 'xu_huong_1',
    nguon      = 'xu_huong_1'
WHERE landing_id = 'mxv_080_online'
  AND (
    entry_url LIKE 'https://xuhuong.hanghoaphaisinh.net%'
    OR entry_url LIKE 'https://daotao.%'
  );

-- e) Còn lại (preview CRM, ads.hanghoaphaisinh.net, ...) ⇒ default_landing
UPDATE analytics_sessions
SET landing_id = 'default_landing',
    nguon      = 'default_landing'
WHERE landing_id = 'mxv_080_online';

-- ─────────────────────────────────────────────────────────────
-- BƯỚC 3: landing ID rác `6adc99f23115ac001247de00` (ladipage_id)
-- ─────────────────────────────────────────────────────────────
UPDATE analytics_sessions
SET landing_id = 'landing_lznoew',
    nguon      = 'landing_lznoew'
WHERE landing_id = '6adc99f23115ac001247de00'
  AND entry_url LIKE 'https://ads.giaodichhanghoatuonglai.com/chucmung%';

UPDATE analytics_sessions
SET landing_id = 'default_landing',
    nguon      = 'default_landing'
WHERE landing_id = '6adc99f23115ac001247de00';

-- ─────────────────────────────────────────────────────────────
-- BƯỚC 4: LEADS ghi nhầm — sửa theo `url` thực tế
--
--   3 lead của `mxv_080_online` đều có URL `ads.giaodichhanghoatuonglai.com/landing`
--   ⇒ thuộc `landing_cu`.
--   5 lead của `landing_232` đã xử lý ở BƯỚC 1 (4 → doanh_chung, 1 → landing_232).
-- ─────────────────────────────────────────────────────────────
UPDATE leads
SET landing_id = 'landing_cu',
    nguon      = 'landing_cu'
WHERE landing_id = 'mxv_080_online'
  AND url LIKE '%ads.giaodichhanghoatuonglai.com/landing%';

UPDATE leads
SET landing_id = 'default_landing',
    nguon      = 'default_landing'
WHERE landing_id = 'mxv_080_online';

UPDATE leads
SET landing_id = 'default_landing',
    nguon      = 'default_landing'
WHERE landing_id = '6adc99f23115ac001247de00';

-- ─────────────────────────────────────────────────────────────
-- BƯỚC 5: Đồng bộ `nguon` còn sót (VD `nguon = 'an'`)
--
--   Rule: `nguon` mặc định = `landing_id` (docs/LANDING.md mục 5).
--   ⇒ mọi `nguon` lạc (`an`, `landing_chinh`, ...) trên landing đã biết
--     được chuẩn hoá về `landing_id`.
-- ─────────────────────────────────────────────────────────────
UPDATE analytics_sessions
SET nguon = landing_id
WHERE nguon IS DISTINCT FROM landing_id
  AND nguon NOT IN ('', 'landing_chinh')
  AND landing_id IN (
      'landing_cu', 'landing_lznoew', 'doanh_chung', 'landing_232',
      'xu_huong_1', 'xu_huong_2', 'xu_huong_3'
  );

UPDATE leads
SET nguon = landing_id
WHERE nguon IS DISTINCT FROM landing_id
  AND nguon NOT IN ('', 'landing_chinh')
  AND landing_id IN (
      'landing_cu', 'landing_lznoew', 'doanh_chung', 'landing_232',
      'xu_huong_1', 'xu_huong_2', 'xu_huong_3'
  );

COMMIT;

-- ─────────────────────────────────────────────────────────────
-- BÁO CÁO KẾT QUẢ
-- ─────────────────────────────────────────────────────────────
SELECT landing_id,
       count(*) AS sessions,
       count(*) FILTER (WHERE NOT is_bot) AS humans,
       count(*) FILTER (WHERE converted_lead) AS converted
FROM analytics_sessions
GROUP BY landing_id
ORDER BY sessions DESC;

SELECT landing_id, count(*) AS leads
FROM leads
GROUP BY landing_id
ORDER BY leads DESC;
