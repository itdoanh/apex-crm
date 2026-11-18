# Database Schema — `apex_leads`

PostgreSQL 16, driver `pgx/v5`, connection pool (MaxConns 25, MinConns 2).

**Schema được tạo tự động** bởi `db.NewStore()` → `initSchema()` bằng `CREATE TABLE IF NOT EXISTS` + `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`. Không có migration tool — sửa schema = sửa `backend/db/db.go`.

---

## 1. Danh mục 14 bảng

| Bảng | Mục đích | Số dòng (prod) |
|---|---|---|
| `leads` | Lead đã chuyển đổi | 2 |
| `analytics_sessions` | Phiên truy cập + hành vi | 59 |
| `spam_quarantine` | Request bị chặn | 2 |
| `landing_pages` | Cấu hình landing (PK: domain+page_id) | 4 |
| `landing_files` | Cây file của landing | 32 |
| `domains` | Tên miền đã đăng ký | 2 |
| `domain_routes` | Định tuyến path → landing | 4 |
| `admin_users` | Tài khoản quản trị | 1 |
| `admin_sessions` | Phiên đăng nhập | 37 |
| `admin_audit_logs` | Nhật ký hành động | 55 |
| `google_sheet_configs` | Cấu hình đồng bộ Sheets | 1 |
| `google_sheet_sync_logs` | Lịch sử đồng bộ | 2 |
| `facebook_capi_configs` | Cấu hình Meta Conversions API (theo landing) | 0 |
| `facebook_capi_logs` | Lịch sử gửi sự kiện CAPI | 0 |

---

## 2. `leads` — bảng trung tâm

```sql
CREATE TABLE leads (
    id              BIGSERIAL PRIMARY KEY,
    thoi_gian       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    landing_id      VARCHAR(100) NOT NULL DEFAULT '',
    nguon           VARCHAR(100) NOT NULL DEFAULT '',
    so_dien_thoai   VARCHAR(50)  NOT NULL DEFAULT '',
    tracking        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    trang_thai      VARCHAR(50)  NOT NULL DEFAULT 'moi',
    ghi_chu         TEXT         NOT NULL DEFAULT '',
    nguoi_phu_trach VARCHAR(100) NOT NULL DEFAULT '',
    cap_nhat_luc    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    city            VARCHAR(100) NOT NULL DEFAULT '',
    region          VARCHAR(50)  NOT NULL DEFAULT '',
    country         VARCHAR(100) NOT NULL DEFAULT 'Việt Nam',
    isp             VARCHAR(100) NOT NULL DEFAULT '',
    ad_platform     VARCHAR(50)  NOT NULL DEFAULT '',
    ad_placement    VARCHAR(100) NOT NULL DEFAULT '',
    url             VARCHAR(1000) NOT NULL DEFAULT '',
    is_test         BOOLEAN      NOT NULL DEFAULT FALSE
);
```

> ⭐ **Cột `url`** — URL trang khách đã đăng ký (landing + query string UTM).
> Tách riêng khỏi `tracking` JSONB để:
> 1. Đồng bộ Google Sheets bằng field mapping `url` (1 cột cố định).
> 2. Lọc / xuất CSV nhanh không cần parse JSONB.
> 3. Truy ngược lead về đúng landing page khi campaign bị trộn.
>
> Migration tự động (idempotent) trong `backend/db/db.go`:
> ```sql
> ALTER TABLE leads ADD COLUMN IF NOT EXISTS url VARCHAR(1000) NOT NULL DEFAULT '';
> ```

> ⭐ **Cột `is_test`** (thêm 2026-11-16) — cờ LEAD TEST.
> Lead mà **tên chứa chuỗi `test`** (không phân biệt hoa thường) ⇒ tự động
> đánh dấu `is_test = TRUE`.
>
> ⚠️ **Mở rộng 2026-11-16** — trước đây quy tắc chỉ khớp `"(test)"` CÓ DẤU
> NGOẶC ⇒ tên `"test 1"` bị bỏ sót ⇒ lead test **vẫn được tính vào thống kê**
> (bug anh itdoanh phát hiện — lead #425). Nay bất kỳ tên nào chứa `test` đều
> là lead test: `test` · `test 1` · `TEST` · `(test)` · `Tester Nguyễn` · `contest`.
>
> | Nơi | Lead test |
> |---|---|
> | Bảng CRM (`GET /api/leads`) + export CSV | ✅ VẪN hiện (badge "TEST") |
> | Google Sheets (`syncLeadToGoogleSheets`) | ✅ VẪN đồng bộ bình thường |
> | Biểu đồ / thống kê (`GET /api/leads/stats`, `/api/crm/stats`) | ❌ KHÔNG tính |
> | Thống kê landing (`ListLandingPages`) | ❌ KHÔNG tính |
> | Break down chiến dịch Meta (`GET /api/facebook-ads/campaigns`) | ❌ KHÔNG tính |
> | `analytics_sessions.converted_lead` (biểu đồ analytics / CR% / phễu) | ❌ KHÔNG đánh dấu |
> | Meta Conversions API (CAPI) | ❌ KHÔNG bắn (không tốn tiền QC) |
>
> Nhận diện: `security.IsTestLeadName()` — `strings.Contains(strings.ToLower(name), "test")`.
>
> Migration tự động (idempotent) trong `backend/db/db.go`:
> ```sql
> ALTER TABLE leads ADD COLUMN IF NOT EXISTS is_test BOOLEAN NOT NULL DEFAULT FALSE;
> CREATE INDEX IF NOT EXISTS idx_leads_is_test ON leads (is_test);
> ```
>
> **Backfill lead cũ** (migration `D1`, idempotent) — tự đánh dấu các lead đã
> lỡ bị tính trước khi quy tắc được mở rộng:
> ```sql
> UPDATE leads SET is_test = TRUE
> WHERE COALESCE(is_test, FALSE) = FALSE
>   AND LOWER(COALESCE(tracking->>'fullName', '')) LIKE '%test%';
> -- (tương tự cho name / ho_ten / customer_name)
> ```
>
> **Dọn session** (migration `D2`, idempotent, chạy SAU `D1`) — gỡ cờ
> `converted_lead` + xoá action `"converted"` trong timeline của các session gắn
> lead test (biểu đồ analytics / CR% / phễu chuyển đổi):
> ```sql
> UPDATE analytics_sessions s SET converted_lead = FALSE
> FROM leads l
> WHERE s.lead_id = l.id AND COALESCE(l.is_test, FALSE) = TRUE
>   AND s.converted_lead = TRUE;
> ```
>
> ⚠️ Cơ chế lọc: `buildWhereClause()` (mặc định) **LOẠI** lead test;
> `buildWhereClauseEx(..., includeTest=true)` **GIỮ** lead test (dùng cho
> bảng CRM + export CSV).

### Trạng thái CRM (`trang_thai`)

> ⭐ **Pipeline 10 trạng thái (thêm 2026-11-15).** Định nghĩa tập trung ở
> `backend/models/lead_status.go` (`LeadStatusDefs` / `LeadStatusOrder`).
> Frontend gọi `GET /api/facebook-ads/lead-statuses` để render động — không hardcode.

| Giá trị (`key`) | Nhãn UI | Giai đoạn | CAPI |
|---|---|---|---|
| `moi` | 📥 Mới tiếp nhận | `new` | — |
| `da_goi` | 📞 Đã gọi điện | `contacted` | — |
| `ket_ban_zalo` | 💬 Đã kết bạn Zalo | `contacted` | — |
| `da_hoc_buoi_chia_se` | 💻 Đã học buổi chia sẻ | `free` | — |
| `da_hoc_chuyen_sau` | 📘 Đã học chuyên sâu (≈6.000.000đ) | `paid` | `Subscribe` |
| `da_mua_hang` | 🏆 Khách hàng đã mua hàng | `won` | `Purchase` |
| `da_hoc_truoc_do` | 🔁 Đã học trước đó rồi | `lost` | — |
| `khong_quan_tam` | 🙅 Không quan tâm | `lost` | — |
| `spam_khong_lien_lac` | 🚫 Spam / Không liên lạc được | `lost` | — |
| `huy` | ❌ Hủy / Sai số | `lost` | — |

**Tương thích ngược** (lead cũ trong DB vẫn đọc được, tự chuẩn hoá khi ghi):
`tiem_nang` → `da_hoc_buoi_chia_se` · `chot` → `da_mua_hang` · `da_vao_zoom` → `da_hoc_buoi_chia_se`.

> Ngành "Giải Mã Đầu Tư Hàng Hóa" chỉ cần khách mua **1 lần** ⇒ không có trạng thái
> "mua lại" / "khách cũ quay lại". `da_hoc_truoc_do` là trạng thái kết thúc không mua.

### Cấu trúc `tracking` JSONB

```json
{
  "fullName": "Nguyễn Văn A",
  "phone": "0984386538",
  "carrier": "Viettel",
  "isp": "Viettel Telecom",
  "city": "Hà Nội",
  "region": "Miền Bắc",
  "country": "Việt Nam",
  "server_ip": "113.161.72.45",
  "server_user_agent": "Mozilla/5.0 ...",
  "server_referer": "https://...",
  "entry_url": "https://ads.giaodichhanghoatuonglai.com/landing",
  "url": "https://ads.giaodichhanghoatuonglai.com/landing?utm_source=facebook&utm_campaign=KhoaHoc_MXV_T11",
  "referrer": "https://www.facebook.com/",
  "session_id": "sess_88573x64_mw1kdrn3",
  "visitor_id": "v_bh2k8njl_mw1kdrn2",
  "ad_platform": "Meta Ads",
  "ad_placement": "Facebook_Reels",
  "ad_source_tag": "landing_chinh",
  "utm_source": "", "utm_medium": "", "utm_campaign": "", "utm_content": "", "utm_term": "",
  "max_scroll": 99,
  "clicks_count": 3,
  "time_on_page": 66,
  "is_autofill": true,
  "autofill_source": "browser_autofill",
  "fill_duration_ms": 43756,
  "challenge_id": "ch_4585712799a4405c09db91027f8099fd",
  "challenge_difficulty": 3,
  "challenge_signature": "a06e93a8..."
}
```

### Index

```sql
idx_leads_landing_id     ON leads (landing_id)
idx_leads_nguon         ON leads (nguon)
idx_leads_so_dien_thoai ON leads (so_dien_thoai)
idx_leads_thoi_gian     ON leads (thoi_gian DESC)
idx_leads_trang_thai    ON leads (trang_thai)
idx_leads_city          ON leads (city)
idx_leads_region        ON leads (region)
idx_leads_ad_platform   ON leads (ad_platform)
idx_leads_ad_placement  ON leads (ad_placement)
idx_leads_tracking_gin  ON leads USING gin (tracking)
idx_leads_is_test       ON leads (is_test)
```

### Deduplication 24h

`SaveLead()` trả về `(*models.Lead, isNew bool, err error)`:

- `isNew = true`  → lead MỚI được tạo trong lần gọi này.
- `isNew = false` → lead đã tồn tại, đã được **merge tracking** và trả về bản ghi cũ.

Toàn bộ thao tác "kiểm tra trùng" + "insert" nằm trong **một transaction** kèm
`SELECT pg_advisory_xact_lock(hashtextextended('apex_lead:'||phone, 0))` ⇒ các request
song song cùng 1 SĐT bị serialize lại, không thể cùng lúc "thấy chưa có lead" rồi
cùng `INSERT` (race condition TOCTOU).

Lead được coi là **trùng lặp** (trong 24h) khi thỏa **một** trong 3 tiêu chí:

| # | Tiêu chí | Mục đích |
|---|----------|---------|
| 1 | cùng `so_dien_thoai` + cùng `landing_id` | quy tắc cũ (giữ nguyên) |
| 2 | cùng `so_dien_thoai` trong **10 phút** gần nhất (bất kể landing) | bắt trường hợp gửi 2 landing liên tiếp (VD: `landing_cu` → `mxv_080_online`) |
| 3 | cùng `so_dien_thoai` + cùng `tracking->>'session_id'` | bắt trường hợp double-click / 2 tab cùng 1 phiên đăng ký |

Khi merge, cột `url` cũng được cập nhật nếu lead mới có URL khác (`url = CASE WHEN $8 != '' THEN $8 ELSE leads.url END`).

> ⚠️ **Chỉ đồng bộ Google Sheets khi `isNew = true`**
> (`backend/handlers/lead_handler.go` → `HandleSubmitLead`).
> Nếu đồng bộ mọi lần thì 1 lead bị gửi trùng 2–3 lần sẽ đẩy **2–3 dòng giống
> nhau** xuống Sheet ⇒ báo cáo bán hàng thấy "mấy cái lead giống nhau cùng thời gian".

---

## 3. `landing_pages` — PK composite

```sql
CREATE TABLE landing_pages (
    page_id        VARCHAR(100) NOT NULL,
    domain         VARCHAR(255) NOT NULL DEFAULT '',
    name           VARCHAR(255) NOT NULL DEFAULT '',
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    status         VARCHAR(50)  NOT NULL DEFAULT 'published',
    redirect_url   VARCHAR(500) NOT NULL DEFAULT '',
    nguon_default  VARCHAR(100) NOT NULL DEFAULT 'landing_chinh',
    settings       JSONB NOT NULL DEFAULT '{}'::jsonb,
    code_html      TEXT   NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (domain, page_id)
);
```

> ⚠️ **Lưu ý vận hành:** production hiện có **2 row cùng `page_id = 'mxv_080_online'`** (một row `domain=''`, một row `domain='ads.giaodichhanghoatuonglai.com'`). Đây là hợp lệ với PK composite nhưng gây rối khi `LandingCache.GetByPageID()` tra theo `page_id` đơn lẻ — hàm này có thể trả về row không mong muốn.

### ⚠️ Chống landing trùng (fix 2026-11-13)

Vì PK là `(domain, page_id)`, cùng 1 `page_id` có thể tồn tại **nhiều row**: bản GỐC
(`domain = ''`) + bản theo domain đích. Trước đây mỗi lần bấm "Lưu cài đặt" trong
CRM (hoặc deploy landing) sẽ `INSERT` thêm 1 row `(page_id, domain)` ⇒ **CRM hiển
thị landing bị nhân đôi** sau mỗi lần thêm miền / deploy landing.

| Hàm store | Quy tắc bắt buộc |
|---|---|
| `SaveLandingPage()` | Nếu đã có bản GỐC (`domain=''`) ⇒ **cập nhật bản gốc, KHÔNG tạo row domain mới**. Nếu bản domain đích đã tồn tại ⇒ cập nhật **cả hai** để 2 bản đồng bộ. Nếu chỉ có bản domain-specific ⇒ cập nhật bản đó (không tạo bản gốc). |
| `ListLandingPages()` | Gọi `dedupeLandingPagesByPageID()` ⇒ gom theo `page_id`, ưu tiên dòng có `domain` khác rỗng, `settings` được **trộn** (giá trị khác rỗng ưu tiên) để không mất cấu hình. Dòng bị gộp **không xoá khỏi DB** — chỉ ẩn khỏi danh sách. |
| `LandingCache.Warmup()` | `byPageID` chỉ gán khi **chưa có** ⇒ giữ bản `updated_at` mới nhất. Gán đè vô điều kiện khiến bản CŨ thắng (query sort `updated_at DESC` ⇒ phần tử cuối là bản cũ) ⇒ server phục vụ code cũ, mất chỉnh sửa trong Code Studio. |

> Việc gắn landing ↔ domain đã do `domain_routes` đảm nhiệm (PK `(domain_id, path_prefix)`)
> ⇒ **không cần** row riêng trong `landing_pages` cho mỗi domain.

### ⚠️ Sửa typo Meta Pixel (fix 2026-11-13)

Một số landing có block Meta Pixel hardcode viết sai: `if(!f._fbq) f._bq = n;` — **thiếu chữ `f`**.

Meta Pixel chính thức yêu cầu `if(!f._fbq) f._fbq = n;`. Gán sai ⇒ `window._fbq` không
bao giờ được tạo ⇒ `fbevents.js` tự tạo instance thứ hai ⇒ phát hiện
`window.fbq !== window._fbq` ⇒ Meta báo:

```
[Meta Pixel] - Multiple pixels with conflicting versions were detected on this page.
```

⇒ PageView bị đếm sai/lặp, ảnh hưởng tối ưu ngân sách Meta Ads.

| Hàm store | Quy tắc |
|---|---|
| `RepairMetaPixelSnippet()` | Chạy lúc khởi động server. Thay literal `f._bq=n` → `f._fbq=n` trong `landing_pages.code_html` + `landing_files.content`. Dùng `strpos()` (KHÔNG dùng `LIKE` vì `_` là wildcard) và `replace()` (literal, không phải regex). |

### ⚠️ Đồng bộ `settings` bản gốc ↔ bản domain (fix 2026-11-13)

Bản domain (`domain <> ''`) được tạo ra từ lúc deploy landing và **chỉ chứa một phần
cấu hình** — các trường SEO (`title`, `og_image`, `meta_description`) bị để trống.

`GetLandingPage(domain, page_id)` **ưu tiên bản domain** ⇒ landing chạy thật trên
domain đích mất tiêu đề/description. Đây là lý do ô "Tiêu đề SEO" trong CRM trông
trống sau khi deploy.

| Hàm store | Quy tắc |
|---|---|
| `RepairDuplicateLandingSettings()` | Chạy lúc khởi động server. 2 bước:<br>① `repairDomainFromBase()` — với mỗi `page_id` có cả bản gốc và bản domain, **điền các khoá TRỐNG** ở bản domain từ bản gốc. **KHÔNG** ghi đè giá trị đã có ⇒ `path_prefix`, `google_sheet_ids`, `google_sheet_sync_mode` riêng của domain được giữ nguyên.<br>② `repairBaseFromDomain()` — đồng bộ **NGƯỢC** `google_sheet_sync_mode` + `google_sheet_ids` từ bản domain (mới nhất) về bản gốc **khi bản gốc chưa có**. Bản gốc là nơi các đường đọc `domain = ''` lấy cấu hình ⇒ nếu bản gốc thiếu ⇒ lead bắn tràn sang mọi sheet (xem mục Google Sheets bên dưới). |
| `SaveLandingPage()` → `mergeDomainSettings()` | Trước khi upsert bản domain, gộp `settings` mới với `settings` đang có của bản domain. Upsert thẳng sẽ **xoá mất** các khoá domain-specific. |
| `SaveLandingPage()` → `mergeBaseSheetSettings()` | Khi ghi bản gốc, gộp thêm `google_sheet_sync_mode` + `google_sheet_ids` từ bản domain mới nhất (chỉ khi bản gốc chưa có). |
| `GetLandingSheetSyncSettings(domain, page_id)` | Đọc cấu hình sheet gộp từ **MỌI row** của landing. Ưu tiên: row khớp domain → bản gốc → row mới nhất. Dùng làm fallback khi RAM cache miss. |

> Bản gốc = nguồn **mặc định** của landing · Bản domain = **override theo domain**.
> Quy tắc chung: giá trị rỗng = "chưa cấu hình" ⇒ kế thừa từ bản gốc.
> **Ngoại lệ cấu hình sheet:** `google_sheet_sync_mode` + `google_sheet_ids` là cấu
> hình ở **MỨC LANDING** ⇒ phải đồng bộ 2 chiều (bản gốc ↔ bản domain).

### 🚨 Cách ly row theo domain — KHÔNG ghi đè template (fix 2026-11-16)

**Triệu chứng:** các landing đã nhân bản có **thống kê chồng chéo** (lượt xem / lead
gộp chung) trong tab Quản lý Landing.

**Nguyên nhân:** `UpdateLandingCode` dùng:

```sql
WHERE (domain = $3 OR domain = '') AND page_id = $4   -- ❌ SAI
```

⇒ Sửa code HTML trên domain đích **ghi đè LUÔN** row template (`domain = ''`) ⇒
mọi landing cùng `page_id` bị gộp làm 1 ⇒ số liệu chồng chéo. Tương tự,
`ToggleLandingActive` khi deploy cũng tắt/mở nhầm row domain khác.

| Hàm store | Quy tắc bắt buộc (fix 2026-11-16) |
|---|---|
| `EnsureLandingPageDomainRow(ctx, domain, pageID, codeHTML)` | Trước khi update row theo domain: nếu chưa có row `(domain, page_id)` ⇒ **clone từ row gốc** (`domain=''`) sang row domain mới. Không có hàm này thì update sẽ **không ăn** (0 row) ⇒ mất code. |
| `UpdateLandingCode()` | Update **chỉ** `WHERE domain = $3 AND page_id = $4`. Không dùng `OR domain = ''`. |
| `ToggleLandingActive(domain, pageID, isActive)` | `domain != ''` ⇒ chỉ toggle row đó. `domain = ''` ⇒ toggle **mọi** row của `page_id` (dùng khi bật/tắt toàn bộ landing). |
| `ToggleLandingActiveBase(pageID, isActive)` | Toggle **chỉ** row template (`domain = ''`). Dùng trong `HandleDeployLanding` để deploy **không** làm đổi trạng thái active của các row domain khác. |
| `GetAllActiveDomainRoutes()` | Dùng `SELECT DISTINCT ON (dr.id)` + `EXISTS (... landing_pages lp2 WHERE lp2.page_id = dr.landing_id AND lp2.is_active = true)` ⇒ 1 landing có nhiều row domain **không** sinh route trùng. |
| `ListLandingPages()` | Tách `views_count` (tổng) · `human_views_count` (`COUNT(*) FILTER (WHERE NOT is_bot)`) · `bot_views_count`; gắn `domains []string` từ `domain_routes` (`GetLandingDeployDomains`); `dedupeLandingPagesByPageID` **gộp** danh sách domain (`mergeDomainLists`). |

> 📌 `models.LandingPage` giờ có thêm: `HumanViewsCount int`, `BotViewsCount int`,
> `Domains []string` — frontend dùng `human_views_count` làm mẫu số CR% (khớp công
> thức tab Trung Tâm Báo Cáo).

> 🧪 Migration dữ liệu lịch sử: `deploy/fix_landing_attribution_20261116.sql`
> (đã test trên `apex_leads_test` — idempotent). Chi tiết: `docs/LANDING.md` mục 7.1.

### `settings` JSONB — feature flags & branding

```json
{
  "brand_header": {
    "agency_1": "Bộ Công Thương",
    "agency_2": "Sở giao dịch hàng hóa Việt Nam MXV",
    "logo_url": "assets/img/logo_APEX_K_NEN.webp",
    "member_text": "Thành viên 080 MXV"
  },
  "facebook_pixel_id": "15588280295...",
  "google_analytics_id": "",
  "tiktok_pixel_id": "",
  "custom_head_scripts": "",
  "custom_body_scripts": ""
}
```

---

## 3.1 `landing_files` — cây file của landing

```sql
CREATE TABLE landing_files (
    id          SERIAL PRIMARY KEY,
    page_id     VARCHAR(100) NOT NULL,
    domain      VARCHAR(255) NOT NULL DEFAULT '',
    file_path   VARCHAR(500) NOT NULL DEFAULT '',
    file_type   VARCHAR(50)  NOT NULL DEFAULT 'html',
    content     TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

| Hàm store | Quy tắc |
|---|---|
| `GetLandingFile(domain, pageID, filePath)` | `WHERE (domain = $1 OR domain = '')` + `ORDER BY (CASE WHEN domain = $1 AND $1 <> '' THEN 0 ELSE 1 END) ASC, updated_at DESC` ⇒ **bản domain khớp chính xác** thắng bản `domain = ''` dù chung; cùng nhóm thì lấy bản cập nhật gần nhất. Không có `ORDER BY` này ⇒ server serve **bản `tracker.js` cũ** ⇒ mất lớp chống submit trùng phía client. |
| `SyncSharedStaticFilesToDB()` | `SELECT domain, page_id FROM landing_pages UNION SELECT domain, page_id FROM landing_files` ⇒ file tĩnh (orphan trong `landing_files` mà không còn trong `landing_pages`) vẫn được đồng bộ từ đĩa lúc khởi động server. |

> ⚠️ Vì `landing_files` cho phép `domain = ''` (bản dùng chung), cùng một `file_path`
> có thể tồn tại 2 row. Quy tắc ưu tiên domain khớp chính xác là **bất biến** —
> không được bỏ `ORDER BY`.

---

## 4. `domains` & `domain_routes`

```sql
CREATE TABLE domains (
    id                  SERIAL PRIMARY KEY,
    domain              VARCHAR(255) UNIQUE NOT NULL,
    ns1                 VARCHAR(255) NOT NULL DEFAULT 'ns1.hanghoaphaisinh.net',
    ns2                 VARCHAR(255) NOT NULL DEFAULT 'ns2.hanghoaphaisinh.net',
    target_ip           VARCHAR(100) NOT NULL DEFAULT '103.124.95.232',
    dns_status          VARCHAR(50)  NOT NULL DEFAULT 'pending',
    ssl_status          VARCHAR(50)  NOT NULL DEFAULT 'active',
    default_landing_id  VARCHAR(100) NOT NULL DEFAULT 'mxv_080_online',
    notes               TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE domain_routes (
    id            SERIAL PRIMARY KEY,
    domain_id     INT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path_prefix   VARCHAR(255) NOT NULL DEFAULT '/',
    landing_id    VARCHAR(100) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    redirect_url  VARCHAR(500) NOT NULL DEFAULT '',
    deployed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (domain_id, path_prefix)
);
```

**Quan hệ:** 1 domain có N route. Mỗi route ánh xạ `path_prefix` → `landing_id`.

**Lookup runtime:** `DomainCache.ResolveRoute(host, path)` — Longest Prefix Match, `/` luôn match cuối.

**Cache invalidation:** sau khi deploy route → `domainCache.Reload()` bắt buộc.

> ⚠️ **Route "dangling" không được nạp vào cache** (sửa 2026-11-13).
>
> `GetAllActiveDomainRoutes()` chỉ nạp route thỏa:
> `(dr.redirect_url IS NOT NULL AND dr.redirect_url <> '') OR lp.is_active = true`.
>
> **Vì sao**: route `/` của `ads.giaodichhanghoatuonglai.com` trỏ tới
> `landing_232` đã bị deactivate ⇒ landing không được phục vụ ⇒ request `/` không
> tìm được landing ⇒ backend redirect về chính nó ⇒ **vòng lặp redirect 302**
> (đã xác nhận trên production). Nay route tới landing đã tắt bị bỏ qua ⇒ domain
> root trả 404 rõ ràng thay vì loop.
>
> Dữ liệu route **không bị xoá** — chỉ không nạp vào cache. Nếu muốn dọn hẳn thì
> tắt `domain_routes.is_active = false` cho route đó.

---

## 5. `analytics_sessions`

Lưu toàn bộ hành vi người dùng, dùng cho:
- Phân biệt bot vs người thật (`is_bot`, `bot_score`, `bot_reasons`)
- Heatmap khung giờ vàng
- Funnel chuyển đổi
- Ad placement analytics
- Geo analytics

Cột quan trọng: `session_id` (PK), `visitor_id`, `landing_id`, `utm_*`, `ip_address`, `city`/`region`/`isp`, `ad_platform`/`ad_placement`, `duration_seconds`, `max_scroll_percent`, `total_clicks`, `has_interacted_form`, `converted_lead`, `lead_id`, `actions JSONB` (timeline sự kiện).

### 5.1. Cột mở rộng 2026-11-12 (landing `doanh_chung`)

Migration chạy tự động trong `initSchema` (`backend/db/db.go`) — `ADD COLUMN IF NOT EXISTS`, an toàn khi chạy lại nhiều lần.

**Meta Ads Macros (8 biến động Meta cung cấp + account id):**
| Cột | Nguồn |
|---|---|
| `utm_source` | `{{site_source_name}}` — chuẩn hoá `fb`/`ig`/`msg`/`an` |
| `utm_placement` | `{{placement}}` — `Facebook_Reels`, `Facebook_Stories`, `Audience_Network_Rewarded_Videos`… |
| `utm_campaign` | `{{campaign.id}}` |
| `utm_content` | `{{adset.id}}` |
| `utm_term` | `{{ad.id}}` |
| `campaign_name` | `{{campaign.name}}` |
| `adset_name` | `{{adset.name}}` |
| `ad_name` | `{{ad.name}}` |
| `acc_id` | gõ cứng trong Meta Ads (không có macro tự động) |

**Meta Click ID (CAPI):** `fbclid`, `fbc`, `fbp`, `igshid`, `meta_click_id`

**Thiết bị chi tiết:** `device_brand`, `device_model`, `device_model_code` (`SM-A525F`), `device_name` (`Samsung Galaxy A52`), `os_version`, `browser_version`, `browser_engine`, `is_in_app_browser`, `in_app_type` (`Facebook In-App`, `Instagram`, `Zalo`, `Cốc Cốc`), `device_memory`, `device_cores`, `device_touchpoints`, `viewport_res`, `screen_color_depth`, `pixel_ratio`, `timezone`, `language`, `platform_info`, `connection_type`, `device_state`

**VPN/Proxy/Tor:** `is_vpn`, `is_proxy`, `is_tor`, `vpn_provider`, `vpn_confidence`, `vpn_reason`

### 5.2. Bộ lọc analytics — BẮT BUỘC dùng đúng

Tất cả query analytics dùng 2 helper trong `backend/db/db.go`:

```go
buildAnalyticsWhereClauseFull(landingID, nguon, fromDate, toDate string, days int) (string, []interface{})
buildAnalyticsWhereClauseFullRange(landingID, nguon, fromDate, toDate string, days, fromHour, toHour int) (string, []interface{})
```

| Điều kiện | SQL sinh ra |
|---|---|
| `landing_id` | `landing_id = $n` |
| `nguon` | `nguon = $n` |
| `from_date` | `(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date >= ($n)::date` |
| `to_date` | `(first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::date <= ($n)::date` |
| `days` | `...::date >= ((NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date - ($n::int))` |
| `from_hour`/`to_hour` | `EXTRACT(HOUR FROM first_seen AT TIME ZONE 'Asia/Ho_Chi_Minh')::int BETWEEN $n AND $n+1` |
| **quay vòng qua nửa đêm** (`from_hour > to_hour`, vd 22→06) | `(EXTRACT(HOUR ...) >= $n OR EXTRACT(HOUR ...) <= $n+1)` |

> ⚠️ **Bug đã sửa 2026-11-12**: trước đó các bộ lọc `from_date`/`to_date`/`days` **bị bỏ qua hoàn toàn** và bộ lọc giờ đêm bị bỏ qua vì điều kiện `fromHour <= toHour` ⇒ số liệu thống kê sai. **Mọi handler analytics mới phải truyền đủ 6 tham số.**

> ⚠️ `NOW()` **không** được dùng trực tiếp cho so sánh ngày — luôn bọc `AT TIME ZONE 'Asia/Ho_Chi_Minh'`.

### 5.3. Funnel — số liệu thật

`GetCRMStats()` (`backend/db/db.go`) tính funnel trực tiếp trên `analytics_sessions`:

```sql
SELECT
    COUNT(*) FILTER (WHERE NOT is_bot)::int                          AS views,
    COUNT(*) FILTER (WHERE NOT is_bot AND max_scroll_percent >= 50)::int AS scrolled,
    COUNT(*) FILTER (WHERE NOT is_bot AND (has_interacted_form OR form_submit_attempts > 0))::int AS interacted,
    COUNT(*) FILTER (WHERE NOT is_bot AND converted_lead)::int        AS converted
FROM analytics_sessions <where>;
```

> ⚠️ **Bug đã sửa 2026-11-12**: trước đó funnel **bịa** số liệu (`views = totalLeads * 8`) ⇒ không khớp với báo cáo riêng landing.

### 5.4. Phễu biểu mẫu ở mức PHIÊN (2026-11-13)

Bổ sung 5 cột vào `analytics_sessions` (migration tự động trong `initSchema`, `ADD COLUMN IF NOT EXISTS`):

| Cột | Kiểu | Ý nghĩa |
|---|---|---|
| `is_autofill` | `BOOLEAN NOT NULL DEFAULT FALSE` | Form được điền tự động (trình duyệt) hay gõ tay |
| `autofill_source` | `VARCHAR(40) NOT NULL DEFAULT ''` | Nguồn autofill: `url_params` \| `browser_suggest` \| `password_manager` \| `manual` |
| `fill_duration_ms` | `INTEGER NOT NULL DEFAULT 0` | Thời gian từ focus đầu tiên → submit attempt |
| `validation_passed` | `BOOLEAN NOT NULL DEFAULT FALSE` | Lần submit đầu đã vượt qua validate |
| `form_focus_ms` | `INTEGER NOT NULL DEFAULT 0` | Tổng thời gian form nằm trong viewport (focus + nhìn) |

> **Vì sao lưu ở mức session chứ không phải mức lead?** Vì phễu form cần đo cả
> khách **bỏ ngang** (không submit) — nếu chỉ ghi khi có lead thì không thể tính
> "tỷ lệ bỏ ngang form". `tracker.js` gửi 5 trường này **kèm trong payload
> analytics** (`POST /api/analytics/track`) chứ không chỉ trong payload lead.

> ⚠️ `UpsertAnalyticsSession` dùng **93 tham số**. Nếu thêm cột mới phải cập nhật
> **cả** danh sách `$1..$n` **và** `ON CONFLICT ... DO UPDATE SET` — nếu quên,
> Postgres sẽ báo lỗi "bind message supplies N parameters, but prepared statement requires M".

### 5.5. Bộ lọc toàn cục của Trung Tâm Báo Cáo (2026-11-13)

`db.ReportFilter` (`backend/db/analytics_report.go`) là bộ lọc **dùng chung** cho
14 endpoint `/api/analytics/report/*`. SQL được sinh động bằng builder có tham số
hoá (`add(...)` cho biến, `addConst(...)` cho hằng số) ⇒ không lệch chỉ số `$n`.

| Nhóm | Tham số |
|---|---|
| Thời gian | `from_date`, `to_date`, `days`, `from_hour`, `to_hour` |
| Landing | `landing_id`, `landing_ids` (CSV, multi-select), `nguon`, `landing_path` |
| Meta Ads | `ad_account_id`, `campaign_name`, `adset_name`, `ad_name`, `site_source_name`, `utm_placement` |
| Khách | `visitor_type` (`new` → `is_new_visitor`, `returning` → `session_count > 1`) |
| Thiết bị | `device_type`, `os`, `is_in_app_browser`, `in_app_type` |
| Chất lượng | `traffic_scope` (`human` mặc định \| `all` \| `bot`) |
| So sánh | `compare=1` ⇒ `previousPeriod()` dựng from/to lùi về đúng số ngày |

> ⚠️ `previousPeriod()` **phải** chuyển `Days` thành `FromDate`/`ToDate` tường minh.
> Nếu chỉ giữ `Days = N` thì SQL vẫn là "N ngày gần nhất" ⇒ kỳ trước trùng kỳ
> hiện tại ⇒ **% thay đổi luôn = 0%**. Đã sửa 2026-11-13.

> ⚠️ `traffic_scope` mặc định là `human` (`is_bot = FALSE`) vì bot không bao giờ
> chuyển đổi — tính CR trên toàn bộ traffic sẽ ra tỷ lệ sai lệch.

### 5.6. Đồng bộ attribution & cờ `converted_lead` (migration 2026-11-13)

> ⚠️ **Migration DỮ LIỆU KHÔNG được đặt trong `initSchema`** (sự cố 2026-11-13).
>
> `initSchema` dùng chung context với `NewStore`. Migration dữ liệu quét bảng
> `leads` + `analytics_sessions` nên vượt quá thời gian cho phép ⇒ backend **FATAL
> lúc khởi động** ⇒ production trả **503**. Đã xảy ra thật lúc 05:34 ngày
> 13-11-2026 (sau khi deploy bản đầu) ⇒ phải rollback binary cũ.
>
> ⇒ Kiến trúc đúng:
> | Phần | Context | Timeout | Chạy ở |
> |---|---|---|---|
> | Kết nối pool + ping | `connCtx` | 10 giây | đồng bộ |
> | DDL (`initSchema`) | `schemaCtx` | 60 giây | đồng bộ |
> | Migration dữ liệu (`migrateAnalyticsAttribution`) | context riêng | 2 phút | **goroutine** |
>
> Lỗi migration **KHÔNG** chặn backend khởi động — chỉ log `[MIGRATION][WARN]`.

Migration chạy **tự động** trong `migrateAnalyticsAttribution()`, theo đúng thứ tự
B1 → B5 (idempotent, chạy lại nhiều lần vẫn an toàn):

| Bước | Việc | Vì sao |
|---|---|---|
| **B1** | `INSERT` session tối thiểu cho lead có `tracking.session_id` nhưng chưa có dòng session | Lead đã ghi vào bảng `leads` nhưng biểu đồ không thấy (analytics ping bị throttle/mất). Đo production 13-11: 8 lead chỉ 2 khớp session |
| **B2** | `UPDATE` gắn `lead_id` + `converted_lead = TRUE` cho session đã có nhưng `lead_id` NULL | Khớp qua `l.tracking->>'session_id'` |
| **B3** | Gắn `lead_id` qua action `type = 'converted'` (khớp SĐT trong `detail->>'phone'`) | Session có dấu vết đăng ký nhưng chưa gắn lead. Dùng CTE `ROW_NUMBER() OVER (PARTITION BY phone ORDER BY first_seen)` ⇒ chọn session đầu tiên, **không nhân bản** lead. Bản đầu dùng correlated subquery `MIN(first_seen)` ⇒ O(n²) ⇒ timeout |
| **B4** | Khôi phục `landing_id` / `nguon` của session **theo landing của lead** | Session gốc bị ghi đè khi khách đi từ landing sang trang "chúc mừng" (dùng chung cookie phiên) |
| **B5** | `converted_lead = FALSE` cho mọi session **không có lead thật** | Cờ do client gửi lên dựa trên cookie 180 ngày ⇒ tín sai. Chạy CUỐI CÙNG để backend là nguồn sự thật duy nhất |

Kèm 2 câu chuẩn hoá nguồn:

```sql
UPDATE leads SET nguon = landing_id
 WHERE (nguon = '' OR nguon = 'landing_chinh') AND COALESCE(landing_id,'') != '';
UPDATE analytics_sessions SET nguon = landing_id
 WHERE (nguon = '' OR nguon = 'landing_chinh') AND COALESCE(landing_id,'') != '';
```

**Quy tắc attribution "first-touch"** (sửa 2026-11-13) trong
`UpsertAnalyticsSession`:

```sql
landing_id = CASE WHEN analytics_sessions.landing_id != '' THEN analytics_sessions.landing_id ELSE EXCLUDED.landing_id END
```

> **Vì sao**: cookie phiên dùng CHUNG giữa landing và trang "chúc mừng" ⇒ cùng
> 1 `session_id` được gửi lên lần lượt với `landing_id` khác nhau. Trước đây
> `EXCLUDED.landing_id` ghi đè ⇒ phiên gốc của `landing_cu` bị "chuyển" sang
> `landing_lznoew` ⇒ `landing_lznoew` có 270 view + 270 lead (CR = 100%) trong khi
> đó là trang CHỈ ĐỂ XEM, còn `landing_cu` mất lead. ⇒ **Landing đầu tiên khách
> thấy là nguồn attribution đúng.**

> ⚠️ `AssociateLeadToSession` đã đổi từ `UPDATE` sang
> `INSERT ... ON CONFLICT (session_id) DO UPDATE` ⇒ luôn tạo dòng session nếu
> chưa có ⇒ **mọi lead đều xuất hiện trong biểu đồ**.

### 5.7. Cột `pv_event_id` — dedup PageView Pixel ↔ CAPI (2026-11-16)

Migration tự động trong `initSchema` (`backend/db/db.go`):

```sql
ALTER TABLE analytics_sessions
  ADD COLUMN IF NOT EXISTS pv_event_id VARCHAR(100) NOT NULL DEFAULT '';
```

| Cột | Kiểu | Ý nghĩa |
|---|---|---|
| `pv_event_id` | `VARCHAR(100) NOT NULL DEFAULT ''` | `event_id` của PageView đầu tiên của phiên (prefix `pv-`) |

> **Vì sao?** Meta CHỈ gộp (deduplicate) sự kiện trình duyệt (`fbq`) với sự kiện máy
> chủ (CAPI) khi cả hai dùng **CHUNG `event_id`**. Nếu Pixel bắn
> `fbq('track','PageView')` **không có** eventID mà server CAPI bắn `PageView` với
> `event_id` khác ⇒ Meta tính **1 lượt xem thành 2 lượt PageView** ⇒ sai lệch toàn
> bộ chỉ số Landing Page View + phân phối quảng cáo.
>
> ⇒ Client sinh MỘT `window.APEX_PV_EVENT_ID` cho 1 lần tải trang, bắn Pixel với
> eventID đó, rồi gửi kèm lên server (payload analytics `event_id`).
> ⇒ Server truyền **nguyên văn** vào `event_id` của payload CAPI ⇒ Meta gộp 2 sự
> kiện thành 1.

**Quy tắc ghi (`UpsertAnalyticsSession`)** — giữ giá trị **đầu tiên** của phiên:

```sql
pv_event_id = CASE
  WHEN analytics_sessions.pv_event_id != '' THEN analytics_sessions.pv_event_id
  ELSE EXCLUDED.pv_event_id
END;
```

> `tracker.js` gửi ping nhiều lần (pageview, heartbeat, exit…) nhưng `pvEventId` là
> **HẰNG SỐ** trong 1 lần tải trang ⇒ mọi ping mang cùng giá trị ⇒ giữ giá trị
> đầu tiên, không ghi đè.

**Fallback phía server**: `GetSessionPvEventID(sessionID)` đọc cột này khi client
không gửi `event_id` trong payload (trình duyệt cache bản `tracker.js` cũ, hoặc ping
đầu bị throttle). Landing **có** Meta Pixel mà không có event_id chung ⇒ server
**KHÔNG bắn** PageView server-side (chấp nhận thiếu 1 lượt thay vì thừa 1 lượt).

⚠️ `UpsertAnalyticsSession` dùng **93 tham số** sau thay đổi này.

---

## 6. `spam_quarantine`

| Cột | Ý nghĩa |
|---|---|
| `reason` | Lý do chặn, ví dụ `pow_invalid: Thử thách này đã được sử dụng (Replay attack detected)` |
| `payload` | Toàn bộ body request (JSONB) để phân tích |
| `ip_address`, `user_agent` | Định danh bot |

---

## 7. Auth & Audit

```sql
admin_users    (id, username UNIQUE, password_hash, full_name, role, is_active, ...)
admin_sessions (id, admin_id FK, token_hash UNIQUE, ip_address, user_agent,
                device_type, browser, os, city, country,
                is_revoked, expires_at, created_at, last_active_at)
admin_audit_logs (id, admin_id, username, action, resource, resource_id,
                  details JSONB, ip_address, user_agent,
                  device_type, browser, os, status, created_at)
```

- `password_hash` = **Argon2id**.
- `token_hash` = SHA-256 của session token, **không** lưu plaintext.
- `admin_sessions.expires_at` — session dài hạn, có thể revoke từ xa.

### Action audit phổ biến

`AUTH_LOGIN_SUCCESS`, `AUTH_LOGIN_FAILED`, `AUTH_LOGOUT`, `AUTH_PASSWORD_CHANGED`, `DEPLOY_DOMAIN_ROUTE`, `SAVE_LANDING`, `UPDATE_LEAD`, `EXPORT_LEADS`, `SESSION_REVOKED`.

---

## 8. Google Sheets

```sql
google_sheet_configs (
    id, name, auth_type, credentials_json, spreadsheet_id, spreadsheet_name,
    sheet_tab_name, field_mappings JSONB,
    filter_landing_id, filter_nguon,
    is_active, auto_create_header, last_synced_at, created_at, updated_at
)

google_sheet_sync_logs (
    id, config_id FK ON DELETE CASCADE, lead_id, status,
    row_range, lead_phone, lead_name, error_message, synced_at
)
```

### ⚠️ Đồng bộ lead sang Google Sheets — cách chọn sheet (fix 2026-11-13)

Có **2 tầng lọc** độc lập. Landing quyết định sheet nào được nhận lead; sheet
quyết định có nhận landing nào.

| Tầng | Nguồn cấu hình | Giá trị | Ý nghĩa |
|---|---|---|---|
| 1. Landing → sheet | `landing_pages.settings.google_sheet_ids` + `google_sheet_sync_mode` | `selected` | Chỉ gửi vào đúng các sheet được tick |
| | | `all` | Gửi vào mọi sheet khớp bộ lọc của sheet |
| | | `disabled` | Không gửi sheet nào |
| 2. Sheet → landing | `google_sheet_configs.filter_landing_id` | rỗng | Nhận lead của **mọi** landing |
| | | `landing_id` | Chỉ nhận lead của landing đó |

**Quy tắc bất biến:** khi landing đã cấu hình `google_sheet_ids` (mode `selected`),
backend **chỉ** gửi vào đúng các sheet đó — kể cả khi sheet đó có
`filter_landing_id = ''` (đang là "nhận tất cả"). Ngược lại, lead sẽ bị tràn sang
sheet của landing khác.

**Root cause lỗi "lead bắn về cả 2 sheet" (2026-11-13):**
`landing_pages` có PK composite `(domain, page_id)` ⇒ 1 landing tồn tại ở NHIỀU row
(bản gốc `domain=''` + bản theo domain). Cấu hình `google_sheet_ids` chỉ nằm ở
**bản theo domain**. Handler đọc `landingCache.Get("", page_id)` ⇒ luôn ra **bản
gốc** ⇒ không có `google_sheet_ids` ⇒ rơi vào nhánh "đồng bộ mặc định" ⇒ khớp
**mọi** sheet active có `filter_landing_id = ''`.

⇒ Fix đã áp dụng:
- `cache.LandingCache.GetSheetSyncSettings(domain, page_id)` — gộp **mọi row** của
  landing, ưu tiên row khớp domain của lead.
- `db.Store.GetLandingSheetSyncSettings(domain, page_id)` — fallback tương tự khi
  cache miss.
- Handler tra domain từ `lead.URL` để chọn đúng bản cấu hình.
- `RepairDuplicateLandingSettings` đồng bộ 2 chiều cấu hình sheet giữa bản gốc
  và bản domain.
- Nhánh `targetSheetIDs` trong `GetActiveGoogleSheetConfigsForLead` cũng tôn trọng
  `filter_landing_id` (chống tràn chéo landing).

### 8.1. Danh mục trường ánh xạ & cập nhật trạng thái (2026-11-15)

#### Danh mục trường (nguồn sự thật duy nhất)

`backend/integrations/googlesheets/fields.go` → `leadFieldCatalog` (~90 trường, 9 nhóm).

| Nhàm | Nội dung |
|---|---|
| `GetStandardDefaultMappings()` | Bộ cột chuẩn A, B, C… theo đúng thứ tự danh mục |
| `AvailableLeadFields()` | Danh sách trường cho UI (`GET /api/integrations/sheets/defaults`) |
| `MergeStandardMappings(existing)` | Bổ sung trường chuẩn còn thiếu vào **cuối** config cũ (không phá cột cũ) |
| `ExtractLeadValue(field, ...)` | Lấy giá trị cho 1 key (`sync.go`) |

> ⚠️ Trước 2026-11-15 danh sách trường bị lặp ở 3 nơi (handler + `sync.go`
> + `frontend/admin/js/sheets.js`) ⇒ dễ lệch. Nay frontend **nạp từ API**.

#### `google_sheet_sync_logs` — dùng để cập nhật dòng đã ghi

Khi CRM đổi trạng thái, backend cần biết dòng nào trong Sheet chứa lead đó:

```sql
SELECT DISTINCT ON (config_id) config_id, row_range
FROM google_sheet_sync_logs
WHERE lead_id = $1 AND status = 'success' AND row_range <> ''
ORDER BY config_id, id DESC;
```

`row_range` có dạng `Data!A123:BS123` ⇒ `ParseRowNumber()` trích ra `123`.

| Hàm store | Mục đích |
|---|---|
| `GetLatestSyncRowRanges(leadID)` | Map `{configID: row_range}` của lần ghi thành công gần nhất |

#### Cập nhật trạng thái CRM trên Sheet

`POST /api/leads/update` → `syncLeadStatusToGoogleSheets()`:

| Bước | Chi tiết |
|---|---|
| 1 | `GetLatestSyncRowRanges(leadID)` → tra dòng đã ghi |
| 2 | `StatusColumnIndexes(mappings)` → tìm chỉ số cột của 8 trường trạng thái |
| 3 | `BuildStatusRowValues(...)` → dựng giá trị (ô rỗng ⇒ bỏ qua) |
| 4 | `UpdateRowCells(...)` → `Values.BatchUpdate` trên **đúng dòng** (không append) |

> ⚠️ Dùng `ValueInputOption` mặc định (`RAW`) ⇒ không diễn giải `=` là công thức
> (chống formula injection qua ghi chú sales).
>
> ⚠️ Cấu hình đã lưu KHÔNG tự đổi ⇒ Admin phải bấm **"Bổ sung Cột"**
> (`POST /api/integrations/sheets/configs/upgrade-mappings`) để có đủ cột mới.

---

## 8.5. Meta Conversions API — `facebook_capi_configs` & `facebook_capi_logs`

> Thêm 2026-11-15. DDL trong `backend/db/db.go` (`initSchema`), idempotent.

```sql
CREATE TABLE IF NOT EXISTS facebook_capi_configs (
    id                 BIGSERIAL PRIMARY KEY,
    landing_id         VARCHAR(100) NOT NULL DEFAULT '',  -- '' = cấu hình mặc định
    is_active          BOOLEAN NOT NULL DEFAULT FALSE,
    pixel_id           VARCHAR(100) NOT NULL DEFAULT '',
    access_token       TEXT NOT NULL DEFAULT '',
    api_version        VARCHAR(20)  NOT NULL DEFAULT 'v27.0',
    gateway_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    gateway_endpoint   TEXT NOT NULL DEFAULT '',
    gateway_access_key TEXT NOT NULL DEFAULT '',
    send_page_view     BOOLEAN NOT NULL DEFAULT TRUE,
    send_lead          BOOLEAN NOT NULL DEFAULT TRUE,
    send_purchase      BOOLEAN NOT NULL DEFAULT TRUE,
    send_crm_sync      BOOLEAN NOT NULL DEFAULT TRUE,
    -- Chỉ gửi CAPI cho lead đến từ Facebook/Instagram (2026-11-15).
    -- TRUE (mặc định) ⇒ lead organic / Google Ads / TikTok / truy cập
    -- thẳng bị BỎ QUA (không có lượt nhấp Meta để so khớp).
    only_facebook_traffic BOOLEAN NOT NULL DEFAULT TRUE,
    course_value       DOUBLE PRECISION NOT NULL DEFAULT 6000000,
    currency           VARCHAR(10) NOT NULL DEFAULT 'VND',
    hash_email         BOOLEAN NOT NULL DEFAULT TRUE,
    hash_phone         BOOLEAN NOT NULL DEFAULT TRUE,
    hash_name          BOOLEAN NOT NULL DEFAULT TRUE,
    hash_address       BOOLEAN NOT NULL DEFAULT TRUE,
    hash_external_id   BOOLEAN NOT NULL DEFAULT TRUE,
    hash_city          BOOLEAN NOT NULL DEFAULT TRUE,
    hash_state         BOOLEAN NOT NULL DEFAULT TRUE,
    hash_zip           BOOLEAN NOT NULL DEFAULT TRUE,
    hash_country       BOOLEAN NOT NULL DEFAULT TRUE,
    send_user_data     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_facebook_capi_configs_landing
    ON facebook_capi_configs (landing_id);

-- Migration (bảng đã tồn tại thì CREATE TABLE IF NOT EXISTS KHÔNG thêm cột):
ALTER TABLE facebook_capi_configs
    ADD COLUMN IF NOT EXISTS only_facebook_traffic BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE IF NOT EXISTS facebook_capi_logs (
    id             BIGSERIAL PRIMARY KEY,
    lead_id        BIGINT DEFAULT NULL,
    landing_id     VARCHAR(100) NOT NULL DEFAULT '',
    event_name     VARCHAR(50)  NOT NULL DEFAULT '',   -- Lead | Purchase | Subscribe | PageView
    event_id       VARCHAR(100) NOT NULL DEFAULT '',   -- khoá dedup browser <-> server
    action_source  VARCHAR(30)  NOT NULL DEFAULT 'website',
    pixel_id       VARCHAR(100) NOT NULL DEFAULT '',
    status         VARCHAR(20)  NOT NULL DEFAULT '',   -- success | failed | skipped
    http_status    INT NOT NULL DEFAULT 0,
    response       JSONB NOT NULL DEFAULT '{}'::jsonb,
    error          TEXT NOT NULL DEFAULT '',
    triggered_by   VARCHAR(50)  NOT NULL DEFAULT '',   -- lead_submit | crm_status_change | page_view | manual_test
    trang_thai_crm VARCHAR(50)  NOT NULL DEFAULT '',
    session_id     VARCHAR(120) NOT NULL DEFAULT '',   -- dedup PageView server-side (1 phiên = 1 PageView)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Index: `idx_facebook_capi_logs_created_at` (DESC), `_lead_id`, `_status`, `_event_name`, `_landing_id`, `_session_id`.

**Quy tắc dữ liệu:**

- `landing_id = ''` ⇒ cấu hình **mặc định**, áp dụng cho mọi landing chưa có config riêng
  (`GetFacebookCAPIConfig` fallback từ landing_id → `''`).
- ⚠️ **Dòng mặc định tắt/xoá được** (fix 2026-11-16) —
  `POST /api/facebook-ads/config/toggle` và `config/delete` với `landing_id` **rỗng**
  ⇒ tác động đúng dòng `landing_id = ''` (trước đây handler trả `400 "Thiếu landing_id"`
  ⇒ Admin không tắt/xoá được cấu hình mặc định).
  - `UPDATE`/`DELETE` luôn khớp **CHÍNH XÁC** `landing_id` ⇒ không bao giờ đụng
    nhầm dòng của landing khác.
  - Tắt dòng mặc định (`is_active = false`) ⇒ mọi landing **chưa** có config riêng
    ngừng gửi CAPI; landing **đã** có config riêng không bị ảnh hưởng.
  - Xoá dòng mặc định ⇒ các landing đó không còn config nào ⇒ ngừng gửi CAPI
    (`facebook_capi_logs` **không** bị xoá ⇒ vẫn đối soát được).
- `GetFacebookCAPIConfigExact(landingID)` — đọc **đúng dòng** (không fallback).
  Dùng khi Admin lưu config riêng cho 1 landing mới: nếu fallback sang config
  mặc định thì `existingToken` lấy nhầm token ⇒ validate "có token" pass ⇒ insert
  dòng mới với `access_token` RỖNG ⇒ CAPI lúc chạy báo "thiếu Access Token".
- `access_token` / `gateway_access_key` lưu **rõ** trong DB; API trả về client đã mask
  (`MaskAccessToken()`). Khi client gửi lại giá trị đã mask (`****`) ⇒ backend giữ token cũ.
- `status = 'skipped'` ghi lại lý do bỏ qua (CAPI tắt, thiếu token, trạng thái không có giá trị…).
- `course_value` mặc định `6000000` (VND) — giá khoá học chuyên sâu.
- Meta chỉ nhận sự kiện server trong **7 ngày** gần nhất ⇒ log cũ không thể gửi lại.
- `session_id` dùng cho **dedup PageView server-side**: `tracker.js` gửi nhiều ping
  `POST /api/analytics/track` trong 1 phiên (pageview, heartbeat, exit, scroll…) ⇒ không
  dedup thì 1 lượt xem bị đếm 5–10 lần trên Meta Ads Manager.
  `HasFacebookCAPISessionEvent(session_id, event_name)` kiểm tra đã gửi chưa (cửa sổ 24h).

### 8.5.1 Landing CHỈ dùng Meta Pixel — TẮT CAPI tầng server (fix 2026-11-18)

> Yêu cầu chủ repo 2026-11-18: `landing_cu` (deploy tại
> `https://ads.giaodichhanghoatuonglai.com/landing`) **KHÔNG dùng CAPI server**,
> chỉ dùng Meta Pixel nhúng trong HTML.

**Vấn đề gốc:** `GetFacebookCAPIConfig(landingID)` **fallback** sang dòng cấu hình
MẶC ĐỊNH (`landing_id = ''`) khi landing chưa có dòng riêng ⇒ chỉ cần admin tạo
1 cấu hình CAPI mặc định là **mọi** landing (kể cả landing không dùng CAPI) bắt
đầu bắn sự kiện lên Meta ⇒ chiến dịch Meta báo lỗi dữ liệu (thiếu currency/value).

**Cách sửa (2 lớp):**

| Lớp | Cơ chế | Vị trí |
|---|---|---|
| 1. Tầng server (chốt chặn tuyệt đối) | `serverCAPIDenyList = ["landing_cu"]` + env `META_CAPI_BLOCKED_LANDINGS` (bổ sung, ngăn cách dấu phẩy) ⇒ `resolveConfig()` trả `nil` **trước khi** query DB | `backend/integrations/facebook/dispatcher.go` |
| 2. Dòng DB riêng | `INSERT ... ('landing_cu', is_active = FALSE, ...)` ⇒ không fallback sang cấu hình mặc định | migration `D3` trong `backend/db/db.go` |

Migration `D3` (idempotent, chạy trong `migrateAnalyticsAttribution()`):

```sql
INSERT INTO facebook_capi_configs
    (landing_id, is_active, pixel_id, access_token, api_version,
     gateway_enabled, gateway_endpoint, gateway_access_key,
     send_page_view, send_lead, send_purchase, send_crm_sync,
     only_facebook_traffic, course_value, currency,
     hash_email, hash_phone, hash_name, hash_address, hash_external_id,
     hash_city, hash_state, hash_zip, hash_country, send_user_data)
VALUES
    ('landing_cu', FALSE, '1558828029597854', '', 'v27.0',
     FALSE, '', '',
     FALSE, FALSE, FALSE, FALSE,
     TRUE, 6000000, 'VND',
     TRUE, TRUE, TRUE, TRUE, TRUE,
     TRUE, TRUE, TRUE, TRUE, TRUE)
ON CONFLICT (landing_id) DO UPDATE
SET is_active = FALSE, send_page_view = FALSE, send_lead = FALSE,
    send_purchase = FALSE, send_crm_sync = FALSE, updated_at = NOW();
```

> ⚠️ Lớp 1 là **chốt chặn tuyệt đối** — không thể bị vô hiệu hoá kể cả khi Admin
> xoá dòng cấu hình trong DB (vì `GetFacebookCAPIConfig` sẽ fallback sang dòng
> mặc định đang bật).

**Hệ quả ⇒ `eventID` chỉ gắn khi landing THỰC SỰ bật CAPI:**

- `eventID` (tham số thứ 4 của `fbq`) chỉ có ý nghĩa khi server CAPI bắn sự kiện
  **CÙNG** `event_id` (Meta chỉ gộp browser ↔ server khi trùng khớp).
- Landing KHÔNG bật CAPI ⇒ gắn `eventID` vô nghĩa ⇒ giữ nguyên **block Meta Pixel GỐC**
  (`fbq('track', 'PageView')`), đúng như thiết kế ban đầu.
- Xem `backend/handlers/landing_handler.go` (`InjectDynamicScriptsWithBase`) và
  `backend/db/landing_pixel_repair.go` (`RepairMetaPixelPageViewEventID`).

### 8.6. Ánh xạ trạng thái CRM → sự kiện Meta CAPI (2026-11-15)

Định nghĩa tập trung: `backend/models/lead_status.go` (`LeadStatusDefs.CAPIEvent`).

| Trạng thái CRM | Sự kiện Meta | Kèm `value` |
|---|---|---|
| `moi` | `Lead` | ❌ (dùng lại `event_id` gốc ⇒ dedup với `fbq`) |
| `da_goi` | `Contact` | ❌ |
| `da_hoc_buoi_chia_se` | `StartTrial` | ❌ |
| `da_hoc_chuyen_sau` | `Subscribe` | ✅ `course_value` (6.000.000đ) |
| `da_mua_hang` | `Purchase` | ✅ `course_value` (6.000.000đ) |
| `ket_ban_zalo`, `da_hoc_truoc_do`, `khong_quan_tam`, `spam_khong_lien_lac`, `huy` | *(không bắn)* | — |

**Gate chỉ gửi cho traffic Facebook** (`only_facebook_traffic`, mặc định `TRUE`):
`DetectFacebookTraffic()` trong `backend/integrations/facebook/attribution.go` kiểm tra
theo thứ tự mạnh → yếu: `fbc` → `fbclid`/`meta_click_id` → `site_source_name`/`utm_source`
∈ tập nguồn Meta → referrer Meta + `utm_medium=paid` → `fbp`.

**Chuẩn hoá PII theo chuẩn Meta** (quyết định trực tiếp điểm EMQ):

| Trường | Quy tắc | Ví dụ |
|---|---|---|
| `country` | ISO-2 lowercase | `"Việt Nam"` → `vn` |
| `ct` | bỏ dấu + lowercase | `"Hà Nội"` → `ha noi` |
| `st` | bỏ dấu + lowercase; **bỏ qua vùng kinh tế** | `"Miền Bắc"` → *(bỏ trống)* |
| `ph` | E.164 | `0912345678` → `+84912345678` |
| `client_ip_address`, `client_user_agent`, `fbc`, `fbp` | **không băm**, gửi rõ | — |

`st` lấy từ `tracking.region_name` (tỉnh gốc từ GeoIP), fallback `lead.region`
(vùng kinh tế ⇒ bị loại). `country` lấy từ `tracking.country_code`.

**IP/UA gốc cho sự kiện CRM sync**: `buildCAPILeadContext` fallback về
`tracking.server_ip` / `tracking.server_user_agent` (đã lưu lúc tạo lead) ⇒ sự kiện
đổi trạng thái vẫn có IP + UA để Meta so khớp.

---

## 9. Truy vấn DB từ máy local (dev)

```bash
# Qua SSH vào production
ssh Administrator@103.124.95.232 -p 22899

# psql client đã cài ở C:\pgsql\bin
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads -c "SELECT count(*) FROM leads;"
```

> ⚠️ **Luôn dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'`** khi so sánh ngày/giờ. `NOW()` trả UTC — dùng trực tiếp sẽ lệch 7 tiếng so với giờ Việt Nam mà UI hiển thị.

---

## 10. Sửa schema — checklist

1. Sửa `backend/db/db.go` (`initSchema`).
2. Nếu đổi cấu trúc `landing_pages` → cập nhật `backend/cache/landing_cache.go` và `backend/db/landings.go`.
3. Nếu thêm cột vào `leads` → cập nhật `backend/models/lead.go` + tất cả `Scan()` trong `backend/db/db.go`.
4. Nếu đổi tên cột `leads` → grep toàn repo (`handlers/`, `static/js/`) vì frontend đọc JSON trực tiếp.
5. Rebuild binary → restart task `Apex_Backend_Server`.
