# Landing Page — Hướng dẫn vận hành

Landing page là **phễu chuyển đổi** — nơi tiền thật được tạo ra. Mọi thay đổi ở đây ảnh hưởng trực tiếp doanh thu và CPA.

---

## 1. Mô hình đa landing

Hệ thống phục vụ **nhiều landing page** trên **nhiều tên miền**, mỗi landing là một row trong `landing_pages`.

```sql
SELECT page_id, domain, name, is_active, nguon_default, redirect_url
FROM landing_pages ORDER BY updated_at DESC;
```

| Khái niệm | Ý nghĩa |
|---|---|
| `page_id` | Slug định danh landing (vd: `mxv_080_online`, `landing_cu`) |
| `domain` | Domain phục vụ landing (rỗng = domain mặc định) |
| `is_active` | Feature flag / kill-switch — tắt thì redirect hoặc báo bảo trì |
| `nguon_default` | Nguồn traffic ghi vào lead (`landing_chinh`, ...) |
| `redirect_url` | Đích redirect khi `is_active = false` |
| `settings` | JSONB: branding, pixel IDs, custom scripts |
| `code_html` | HTML đầy đủ của landing |

### Định tuyến

Mỗi landing được **gắn** vào (domain, path_prefix) qua `domain_routes`:

```sql
SELECT d.domain, r.path_prefix, r.landing_id
FROM domain_routes r JOIN domains d ON d.id = r.domain_id;
```

**Lookup runtime** (`DomainCache.ResolveRoute`) dùng **Longest Prefix Match**:
- `/landing` khớp trước `/`
- `/` luôn ở cuối danh sách

---

## 2. Vòng đời một landing

```
1. Tạo trong CRM: Quản Lý Landing → Khởi Tạo Landing Page Mới
   - page_id (slug), tên, domain, path prefix, nguon
   
2. Upload/soạn code: Code Studio (visual) hoặc paste HTML

3. Bấm "Triển Khai Landing Page Lên Tên Miền"
   → ghi domain_routes + domainCache.Reload()
   
4. Bật feature flag (is_active = true)
   → landingCache.Reload()
   
5. Truy cập https://domain.com/path → test form
   
6. Đăng ký domain trong "Quản Lý Tên Miền" nếu chưa có
   → Caddy tự cấp SSL
```

---

## 3. Cấu trúc landing HTML

### Yêu cầu bắt buộc

```html
<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>...</title>
  <link rel="stylesheet" href="assets/css/style.css">
</head>
<body>
  <!-- Form đăng ký -->
  <form id="leadForm">
    <input type="text" name="fullName" required placeholder="Họ và tên">
    <input type="tel" name="phone" required placeholder="Số điện thoại">

    <!-- HONEYPOT: field ẩn, bot điền, người thật không thấy -->
    <input type="text" name="website" style="display:none" tabindex="-1" autocomplete="off">
    <input type="text" name="fax_company_prefix" style="visibility:hidden" tabindex="-1" autocomplete="off">

    <!-- Checkbox đồng ý -->
    <label><input type="checkbox" name="terms" required> Chấp nhận điều khoản</label>

    <button type="submit">ĐĂNG KÝ</button>
  </form>

  <!-- Tracker: tự động inject bởi backend, nhưng nên include thủ công -->
  <script src="tracker.js"></script>
  <script src="config.js"></script>
</body>
</html>
```

### Bắt buộc

| Yếu tố | Lý do |
|---|---|
| `name="fullName"` | Backend đọc `tracking->>'fullName'` để hiển thị tên |
| `name="phone"` | Trường chính — phải là số VN 10 số |
| Honeypot fields | Không có → bot spam tự do |
| `required` trên field chính | Validation client-side |
| `tracker.js` | Thu thập data + PoW + analytics |

### Không được

| Sai lầm | Hậu quả |
|---|---|
| Xoá honeypot khỏi DOM | Bot spam không bị chặn |
| Bỏ `tracker.js` | Mất tracking, mất PoW → lead bị reject 403 |
| Đặt `autocomplete="off"` trên field chính | Chặn autofill → giảm CR |
| Dùng `<input type="number">` cho SĐT | Mất số 0 đầu, không match validator |
| Nhúng form trong iframe cross-origin | Không đọc được DOM để scan |
| **Gọi `ApexTracker.sendLeadToBackend()` từ NHIỀU handler submit** | Gửi trùng payload → request sau bị ghi vào `spam_quarantine` với lý do "Lỗi Proof-of-Work" ⇒ **lead thật bị gắn nhầm nhãn spam** |

### ⚠️ Quy tắc: MỘT lần bấm = MỘT request `/api/leads`

Landing có **3 nơi** có thể gọi `ApexTracker.sendLeadToBackend()` khi khách bấm nút:

1. capture listener `document.addEventListener('submit', ..., true)` trong HTML
2. inline `onsubmit="handleFormSubmit(event)"` trong HTML
3. interceptor `initAutoFormInterceptor()` của `tracker.js`

Nếu cả 3 cùng chạy ⇒ **cùng 1 payload** (kể cả cùng `challenge_id` + `challenge_solution`)
được POST lên `/api/leads` 3 lần. Request #1 tạo lead (đúng), request #2, #3 dùng lại nonce PoW
đã tiêu thụ ⇒ server ghi vào vùng cách ly ⇒ lead thật hiển thị sai là "Lỗi Proof-of-Work"
trong tab **Bảo mật & Bot** (dù lead vẫn về CRM + Google Sheets).

**Cách làm đúng (đã áp dụng 2026-11-05):**

| Nơi | Trách nhiệm |
|---|---|
| Capture listener trong HTML | **Gửi lead** (nơi duy nhất) + validate + chặn redirect |
| `handleFormSubmit()` trong HTML | Chỉ lo UI (điền modal) + chặn redirect LadiPage — **KHÔNG** gọi `sendLeadToBackend` |
| HTML | Set `window.__REGISTRATION_FORM_HANDLED__ = true` ⇒ interceptor của `tracker.js` tự bỏ qua form `registrationForm` |
| `tracker.js` | Khoá gửi theo SĐT 30 giây (`LEAD_DEDUP_WINDOW_MS`) — lớp bảo vệ thứ hai, tự mở khoá nếu request lỗi mạng |

> 📌 Chi tiết cơ chế fingerprint phía server (phân biệt "gửi trùng do client" với
> "bot replay payload") xem `docs/SECURITY.md` mục 2.

### 🔴 3 nguyên nhân gốc khiến 1 lần bấm sinh N request (fix 2026-11-05)

Ngoài việc "3 handler cùng gọi", còn **3 lỗi hệ thống** khiến lớp bảo vệ phía client
vô hiệu hoá. Cả 3 đều đã được sửa trong backend:

| # | Nguyên nhân | Triệu chứng | Cách sửa |
|---|---|---|---|
| 1 | `GetLandingFile()` không có `ORDER BY` | Trả về bản `domain=''` **cũ** thay vì bản khớp domain ⇒ landing chạy `tracker.js` cũ **không có** `LEAD_DEDUP_WINDOW_MS` | `backend/db/landings.go` — `ORDER BY (CASE WHEN domain = $1 AND $1 <> '' THEN 0 ELSE 1 END) ASC, updated_at DESC` ⇒ ưu tiên domain khớp chính xác, mới nhất |
| 2 | `SyncSharedStaticFilesToDB()` chỉ đọc `landing_pages` | Bỏ sót các cặp `(domain, page_id)` chỉ tồn tại trong `landing_files` (vd `('', 'landing_cu')`, `('hanghoaphaisinh.net','mxv_080_online')`) ⇒ chúng giữ bản `tracker.js` cũ | `backend/db/landings.go` — query `UNION` của `landing_pages` **và** `landing_files` |
| 3 | `<base href>` đặt **SAU** `<script src="tracker.js">` | Theo HTML spec, `<base>` chỉ có hiệu lực với URL phân giải **sau** khi nó được parse ⇒ trình duyệt gọi `/p/tracker.js` (404) thay vì `/landing/tracker.js` ⇒ `window.ApexTracker` **không tồn tại** ⇒ landing tự gửi lead không được, interceptor cũng không chạy | `backend/handlers/landing_handler.go` — `rewriteRelativeAssetPaths()` ghi đè `tracker.js`/`config.json` thành đường dẫn **tuyệt đối** theo `basePath` của URL đang phục vụ |

> 🚨 **Lưu ý #3 rất dễ tái phát**: HTML export từ LadiPage luôn đặt `<base>` ở giữa
> `<head>`, còn thẻ `<script src="tracker.js">` nằm ở đầu. Vì vậy **không được** viết
> `<script src="tracker.js">` trong HTML và tin rằng trình duyệt sẽ tự resolve đúng.
> Backend đã tự ghi đè — nhưng nếu bạn thêm script asset khác dùng đường dẫn tương đối
> (`style.css`, `main.js`…) thì hãy dùng đường dẫn **tuyệt đối** (`/landing/style.css`).

**Đường dẫn `tracker.js` đúng theo từng kiểu phục vụ:**

| Kiểu phục vụ | `basePath` | `tracker.js` được trỏ tới |
|---|---|---|
| Domain route có prefix | `/landing` | `/landing/tracker.js` |
| Domain route gốc | `/` | `/tracker.js` |
| Multi-landing public | `/p/<page_id>` | `/p/<page_id>/tracker.js` |

### 🧪 Kiểm thử tự động

```bash
# 1 lần bấm = 1 request /api/leads, 0 bản ghi spam_quarantine
python backend/tests/e2e_browser_single_submit_test.py

# tracker.js nạp đúng ở mọi kiểu phục vụ (dùng Host header giả lập)
python backend/tests/e2e_browser_edge_router_test.py
```

Cả 2 script dùng **Playwright Chromium thật** (không phải jsdom) — chỉ bằng cách đó
mới bắt được lỗi phân giải `<base>`.

---

## 4. `config.json` — cấu hình per-landing

Đặt cạnh file HTML landing. `tracker.js` đọc file này để biết landing này là gì.

```json
{
  "landing_id": "mxv_080_online",
  "nguon": "landing_chinh",
  "thoi_gian_chia_se": {
    "target_date": "2026-11-03T20:00:00",
    "ngay": "03/11/2026",
    "gio": "20:00",
    "thoi_luong": "2 giờ",
    "hien_thi": "20:00 - Thứ Ba, ngày 03/11/2026",
    "hinh_thuc": "Học Online qua Zoom"
  },
  "khoa_hoc": {
    "ten": "Giải Mã Đầu Tư Hàng Hóa",
    "suat_hoc": "Chỉ dành cho 100 suất",
    "da_dang_ky": "Đã có 79 suất đăng ký"
  },
  "backend_url": "/api/leads"
}
```

### Binding tự động

```html
<span data-config="thoi_gian_chia_se.hien_thi">20:00 ngày 03/11/2026</span>
<span data-config="khoa_hoc.ten">Giải Mã Đầu Tư Hàng Hóa</span>
```

`tracker.js` tự điền tất cả element có `data-config`. Nhờ vậy khi đổi ngày/giờ chỉ cần sửa `config.json`, không cần sửa HTML.

### Countdown

```html
<div id="countdown" data-target="2026-11-03T20:00:00"></div>
```
`tracker.js` đếm ngược theo `target_date` (ISO 8601, giờ Việt Nam).

---

## 5. Tích hợp tracking

### Pixel / Analytics

Đặt trong `settings` của landing trong CRM (không hardcode vào HTML):
- `facebook_pixel_id`
- `google_analytics_id`
- `tiktok_pixel_id`
- `custom_head_scripts`
- `custom_body_scripts`

Backend tự inject vào HTML khi render (`InjectDynamicScripts`).

#### ⚠️ Meta Pixel KHÔNG được chèn trùng (fix 2026-11-06)

Nhiều landing (đặc biệt là landing export từ **LadiPage**) đã có **sẵn** block
Meta Pixel hardcode trong HTML:

```html
<script>...fbevents.js...; fbq('init', '1558828029597854'); fbq('track', 'PageView');</script>
```

Nếu backend **vẫn inject thêm** 1 block nữa (từ `settings.facebook_pixel_id`) thì:

- `fbq('init')` chạy 2 lần ⇒ reset pixel ID, mất cơ chế event dedup của Meta
- `fbq('track', 'PageView')` chạy 2 lần ⇒ **PageView nhân đôi** ⇒ Meta Ads Manager
  đếm sai lượt hiển thị, quy ngữ quy đổi (nhận diện sai)

⇒ Backend đã chuyển sang cơ chế **kiểm tra trước khi inject**
(`htmlHasMetaPixel()` trong `backend/handlers/landing_handler.go`):

| HTML thô chứa | Backend có inject thêm? |
|---|---|
| `fbevents.js` | ❌ Không |
| `fbq('init', '<đúng pixel id>')` | ❌ Không |
| `fbq('init', '<pixel id khác>')` | ❌ Không (tránh 2 pixel cùng chạy) |
| `fbq('init', ID)` với ID là biến | ❌ Không |
| Không có gì | ✅ Có (hành vi cũ) |

`tracker.js` cũng có lớp chặn thứ hai phía client (`htmlHasMetaPixel()` trong
`frontend/landing/tracker.js`): chỉ inject khi `window.fbq` chưa tồn tại **và**
DOM không còn script chứa `fbevents.js` / `fbq('init'…)` ⇒ không bao giờ có
3 nguồn pixel chạy cùng lúc.

> 📌 Nguồn chân lý (sự thật) của pixel là **HTML viết tay trong Code Studio**.
> Nếu HTML đã có pixel ⇒ backend + tracker đều tự động bỏ qua.
> Nếu muốn landing **không** chạy Meta Pixel: để trống `settings.facebook_pixel_id`
> **và** xoá block pixel hardcode trong HTML (Code Studio).

### ⚠️ `success_redirect` không được chứa domain (fix 2026-11-06)

`form_config.success_redirect` trong CSDL của `landing_cu` từng được cấu hình là:

```
ads.giaodichhanghoatuonglai.com/chucmung      ← SAI
```

Khi `tracker.js` gán giá trị này vào `window.location.href`, trình duyệt phân giải
**tương đối** theo `<base href="/landing/">` ⇒ URL thành:

```
/landing/ads.giaodichhanghoatuonglai.com/chucmung
```

Đường dẫn này **vẫn nằm trong prefix của landing** ⇒ edge router phục vụ **lại trang có
form đăng ký** ⇒ khách bị quay lại form, bấm "Đăng ký" lần nữa ⇒ **sinh thêm lead trùng
SĐT cùng thời gian** (hiện tượng "bấm đăng ký bị lặp mấy cái lead giống nhau").

**Cách làm đúng:**

| Nên dùng | Sai |
|---|---|
| `chucmung.html` | `ads.giaodichhanghoatuonglai.com/chucmung` |
| `chucmung.html` | `/landing/ads.giaodichhanghoatuonglai.com/chucmung` |

`normalizeSuccessRedirect()` (client) + `normalizeSuccessRedirectValue()` (server) đã tự
chuẩn hoá (bỏ domain, thêm `.html`, tránh trỏ về chính trang đang xem) ở cả 3 nơi:

| Nơi | Hàm |
|---|---|
| `GET /api/landing/config` | `normalizeSuccessRedirectValue()` (server) |
| Render HTML landing | `normalizeSuccessRedirectValue()` (server) |
| `tracker.js` | `normalizeSuccessRedirect()` (client) |

Nhưng **vẫn nên sửa cấu hình trong CRM** để dữ liệu đúng ngay từ đầu.

### ❌ Hotline & Zalo link — KHÔNG còn ép ghi đè (từ 2026-11-04)

> **Thay đổi kiến trúc**: HTML viết tay trong **Code Studio là nguồn sự thật duy nhất**.
> Hệ thống **không còn** ghi đè bất kỳ nội dung nào trên landing từ cấu hình CSDL.
> Các module sau đã bị **xoá hoàn toàn** khỏi UI lẫn runtime:
>
> | Đã xoá | Nơi xoá |
> |---|---|
> | `Hotline` / tự động cập nhật link `tel:` | UI tab Cài đặt + `tracker.js applyConfigToDOM` |
> | `Zalo link` (ghi đè `<a href*="zalo.me">`) | UI + `tracker.js applyConfigToDOM` |
> | `Trang chuyển hướng sau đăng ký` (`cfgSuccessRedirect`) | UI + save-time rewrite |
> | Form Auto-Scanner & Mapping Engine (`btnScanForm`, `btnInjectScript`) | UI + `js/studio/form-scanner.js` |
> | Nhận diện cơ quan / logo thương hiệu (`cfgBrand*`) | UI + save-time rewrite |
> | Tiêu đề hero & huy hiệu (`cfgBadge*`, `cfgTitle*`, `cfgSubtitle`) | UI + save-time rewrite |
> | 3 khối thông tin dưới form (`cfgBox*`, `cfgTargetDate`, `cfgEventQuota`, `cfgEventTitle`) | UI + save-time rewrite |
> | Nội dung động (Quét Nội Dung Từ HTML) | UI + `landings.js` |
> | Tab "Form" và tab "Sự Kiện" trong modal Cài đặt | `index.html` / `studio.html` |
>
> **Cách sửa bây giờ**: mở CRM → Code Studio → sửa trực tiếp HTML → Lưu.
> Không có bước "áp dụng cấu hình" nữa.

Các key `settings.hotline`, `settings.zalo_link`, `settings.brand_header`, `settings.tieu_de`,
`settings.metric_boxes`, `settings.thoi_gian_chia_se` vẫn còn trong DB (không xoá để tương
thích ngược) nhưng **backend không còn inject chúng xuống client** và `tracker.js` **không còn
đọc chúng** để sửa DOM.

Link nhóm Zalo (`https://zalo.me/g/...`) giờ chỉ được đặt trực tiếp trong HTML.

> 💡 Muốn đổi trang chuyển hướng sau đăng ký? Đặt biến JS trong HTML:
> ```html
> <script>window.APEX_SUCCESS_REDIRECT = 'chucmung.html';</script>
> ```
> `tracker.js` ưu tiên biến này, sau đó mới fallback về `settings.form_config.success_redirect`.

### Nguồn phục vụ file: `landing_files` (DB) TRƯỚC, đĩa SAU

Edge Router phục vụ tài nguyên tĩnh của landing theo thứ tự:

```
1. Bảng `landing_files` (PostgreSQL)   ← ưu tiên cao nhất
2. Fallback tới thư mục tĩnh trên đĩa (frontend/landing, backend/static, ...)
```

Khi landing được deploy, toàn bộ file của landing (kể cả `tracker.js`) được **copy vào
bảng `landing_files`**. Vì vậy sửa `tracker.js` trong git rồi `git pull` trên server
**chưa đủ** — bản trong DB vẫn cũ và tiếp tục được serve.

> ⚠️ **Đã xử lý (2026-11-04)**: `SyncSharedStaticFilesToDB()` trong
> `backend/db/landings.go` chạy mỗi lần backend khởi động, đọc `tracker.js` từ đĩa
> rồi ghi đè vào `landing_files` cho **mọi landing**. Nhờ vậy code trong git là
> nguồn sự thật duy nhất, không còn tình trạng "sửa code mà không có tác dụng".
>
> 🚨 **`config.json` KHÔNG được đồng bộ chung** (fix 2026-11-05): `config.json` chứa
> `landing_id` của landing mẫu. Copy nó cho mọi landing sẽ làm mọi landing đều có
> `landing_id: "mxv_080_online"` ⇒ **analytics/lead gửi về nhầm landing**.
> `config.json` là file RIÊNG của từng landing, do người dùng quản lý trong
> Code Studio. Hàm `EnsureLandingConfigFile()` chạy lúc backend khởi động sẽ tự
> sửa `landing_id` trong `config.json` cho đúng `page_id` của landing đó.
>
> **Vẫn cần làm khi sửa file khác** (vd: `assets/css/style.css`, `chucmung.html`):
> dùng CRM → File Manager để sửa (ghi thẳng vào `landing_files`), **hoặc** chạy lệnh
> đồng bộ thủ công:
>
> ```sql
> UPDATE landing_files
> SET content = pg_read_file('C:/apex/frontend/landing/<file>', true), updated_at = NOW()
> WHERE file_path = '<file>';
> ```

### 🧩 Cách ly giữa các landing (fix 2026-11-05)

Mỗi landing là **một đơn vị độc lập**: khác `page_id`, khác `domain`, khác cây file,
khác số liệu view/lead. Để điều này đúng, 4 điểm sau phải được giữ:

| # | Điểm | Cách đảm bảo |
|---|---|---|
| 1 | `config.json` của mỗi landing có `landing_id` đúng | `EnsureLandingConfigFile()` sửa lúc khởi động backend; **không** copy chung từ đĩa |
| 2 | `window.APEX_LANDING_CONFIG` được set **trước** `tracker.js` | `InjectDynamicScripts()` cài property trap ở **đầu** `<head>` |
| 3 | `tracker.js` biết chắc landing nào đang phục vụ | Thẻ `<script src="tracker.js" data-landing-id="<page_id>">` — backend luôn gắn |
| 4 | `chucmung.html` cũng có ngữ cảnh landing | Serve qua `InjectDynamicScripts()` (không serve thẳng từ đĩa) |
| 5 | Trình duyệt không cache HTML cũ | Mọi response HTML landing có `Cache-Control: no-cache, no-store, must-revalidate` |
| 6 | `tracker.js` được nạp đúng bản CỦA landing này | `GetLandingFile()` ưu tiên domain khớp chính xác + mới nhất (fix 2026-11-05) |
| 7 | `tracker.js` được nạp đúng **đường dẫn** | `rewriteRelativeAssetPaths()` ghi đè thành đường dẫn tuyệt đối theo `basePath` (fix 2026-11-05) |

> ⚠️ **Đừng bao giờ** để `window.APEX_LANDING_CONFIG` của template ghi đè giá trị
> backend inject. Luôn dùng property trap (`Object.defineProperty` + setter merge).
> Xem `docs/FRONTEND.md` mục "Code Studio preview — cách ly".

> 🚨 **Vì sao #5 quan trọng:** HTML landing chứa `data-landing-id` quyết định analytics
> ghi về landing nào. Backend trước đây **không** set `Cache-Control` cho HTML ⇒ trình
> duyệt cache theo heuristic `Last-Modified` ⇒ khách vẫn chạy HTML cũ ⇒ tracker gửi
> analytics về nhầm landing **dù code đã sửa đúng**. Đã xử lý 2026-11-05.

> ⚠️ **Route dangling**: nếu route gốc `/` của domain trỏ tới landing không tồn tại
> (vd: `landing2`), `/chucmung.html` sẽ không có landing context. `DomainCache.ResolveChucMungRoute()`
> fallback sang route có `path_prefix` chứa `chucmung` (vd: `/chucmung` → `landing_lznoew`).

**Thứ tự ưu tiên `landing_id` trong `tracker.js`:**

```
1. data-landing-id trên thẻ <script src="tracker.js">   ← chắc chắn đúng
2. window.APEX_LANDING_CONFIG.landing_id                ← backend inject trước tracker
3. window.APEX_LANDING_ID                               ← biến JS do landing khai báo
4. URL /p/<page_id>                                     ← chỉ dùng cho route /p/
5. default_landing
```

> ⚠️ `tracker.js` có **nhiều bản** trong repo: `frontend/admin/js/tracker.js` (nguồn
> chuẩn), `frontend/admin/tracker.js`, `frontend/landing/tracker.js`, `tracker.js` (gốc),
> `backend/static/tracker.js`, `backend/static/js/tracker.js`. Chạy
> `node scripts/sync-static-mirror.js` sau khi sửa — script đã tự đồng bộ TẤT CẢ.

### 📁 Cây file & Upload từ máy (từ 2026-11-04)

Mỗi landing có một cây file riêng trong bảng `landing_files`, quản lý từ
**Code Studio → File Explorer** (panel bên trái).

| Thao tác | Cách làm | Endpoint |
|---|---|---|
| Tạo thư mục | Nút "Mục" hoặc hover vào thư mục → `fa-folder-plus` | `POST /api/landings/folder/create` |
| Tạo file | Nút "Tệp" | `POST /api/landings/file` |
| **Upload file từ máy** | Nút **"Tải lên"** (header File Explorer) hoặc hover thư mục → icon cloud | `POST /api/landings/files/upload` |
| Đổi tên | Hover file/thư mục → icon bút | `POST /api/landings/file/rename` / `folder/rename` |
| Xoá | Hover file/thư mục → icon thùng | `DELETE /api/landings/file` / `folder/delete` |

**Quy tắc lưu & deploy (quan trọng):**

1. Bấm **"Lưu Code"** (`Ctrl+S`) → gửi **toàn bộ** file văn bản trong cây lên
   `POST /api/landings/files/save`.
2. Bấm **"Deploy"** (`Ctrl+Enter`) → gửi lại toàn bộ file lên
   `POST /api/landings/deploy` (payload `files`), backend đồng bộ **tất cả** file
   (ảnh, video, css, js…) sang domain đích — không chỉ `index.html`.

> ⚠️ File nhị phân (ảnh/video/audio/font/pdf) được lưu thẳng vào `landing_files` lúc upload
> (dạng `data:<mime>;base64,...`), **không** gửi lại qua JSON mỗi lần lưu (vì có thể
> tới 100MB/tệp). Backend tự đồng bộ chúng sang domain đích khi deploy.

> 💡 Upload vào domain rỗng (bản gốc) sẽ tự động tạo bản sao cho domain hiện tại.
> Deploy sang domain mới sẽ copy toàn bộ cây file sang domain đó.

> ⚠️ **Lỗi đã gặp & đã fix (2026-11-05)**:
> - `populateFolderDropdowns()` bỏ sót `studioUploadFolder` ⇒ dropdown thư mục đích
>   chỉ có "(Thư mục gốc /)" ⇒ **không upload được vào thư mục con**. Đã sửa ở
>   `frontend/admin/js/studio/file-manager.js` (commit `5185228`).
> - `reloadStudioFilesAfterMutation()` không gán lại `currentStudioFiles` ⇒ cây file
>   hiển thị dữ liệu cũ sau upload. Đã sửa (commit `b006f59`).
> - Preview Code Studio hiện sai landing (`mxv_080_online` thay vì landing đang mở)
>   do script template gán `window.APEX_LANDING_CONFIG` **sau** helper script của
>   Studio trong `<head>`. Đã sửa bằng `Object.defineProperty` setter
>   (commit `897f57a`, xem `docs/FRONTEND.md` mục "Code Studio preview — cách ly").

### UTM chuẩn

| Tham số | Ý nghĩa | Ví dụ |
|---|---|---|
| `utm_source` | Nền tảng | `facebook`, `google`, `tiktok` |
| `utm_medium` | Loại | `paid`, `organic` |
| `utm_campaign` | Chiến dịch | `KhoaHoc_MXV_T11` |
| `utm_content` | Nội dung/creative | `reels_v3` |
| `utm_term` | Từ khoá | `dau tu hang hoa` |

Backend tự parse `ad_platform` / `ad_placement` từ UTM + referrer.

### URL lead (`url`)

Tracker gửi kèm `url: window.location.href` — **URL đầy đủ của trang đang đăng ký** kèm query string.
Server lưu vào cột `leads.url` và đồng bộ sang Google Sheets bằng field mapping `url`.

| Nguồn | Thứ tự ưu tiên |
|---|---|
| Payload client | `url` → `page_url` → `entry_url` → `landing_url` |
| Fallback server | `Referer` → `Origin` → `X-Forwarded-Proto` + `Host` + path |

> Landing page mới **không cần làm gì thêm** — `tracker.js` đã gửi sẵn.

---

## 6. Quy trình tạo landing mới (checklist)

```
□  Chọn page_id (slug ngắn, không ký tự đặc biệt)
□  Chọn domain đã đăng ký trong CRM
□  Chọn path prefix (vd: /landing, /khoa-hoc-dau-tu)
□  Tạo config.json với landing_id + nguon
□  Soạn HTML: form đúng field + honeypot + tracker.js
□  Test: mở form, submit, kiểm tra lead vào CRM
□  Kiểm tra tracking đầy đủ trong CRM (Customer 360°)
□  Kiểm tra trong tab "Đồng Bộ Google Sheets" nếu cần gửi sheet
□  Deploy route trong "Triển Khai Landing Page Lên Tên Miền"
□  Bật feature flag
□  Test HTTPS trên domain thật
□  Test trên mobile (viewport)
```

---

## 7. Test landing trước khi deploy

```bash
# 1. Server chạy local
cd backend && go run main.go

# 2. Landing đã đăng ký trong DB
# → http://localhost:8080/p/<page_id>

# 3. Hoặc theo domain
curl -H "Host: ads.giaodichhanghoatuonglai.com" http://localhost:8080/landing

# 4. Kiểm tra lead đã tạo
curl http://localhost:8080/api/leads -H "Authorization: Bearer <token>"
```

### Kiểm tra bằng browser
1. Mở landing → F12 → Network
2. Submit form
3. Kiểm tra `POST /api/leads` trả 200
4. Kiểm tra `POST /api/analytics/track` có gửi
5. Vào CRM → kiểm tra lead có trong danh sách + đủ thông tin tracking

### 🔒 Code Studio preview — cách ly (từ 2026-11-04)

Khi xem trước trong **Code Studio**, `tracker.js` phải **không** dùng cấu hình của landing
khác. Cơ chế cách ly gồm 3 lớp:

| Lớp | Cơ chế | Mục đích |
|---|---|---|
| 1. `tracker.js` | `isStudioBuilder()` / `isAnyPreview()` | Bỏ qua `loadConfigFile()` + `applyConfigToDOM()`; bỏ qua `initAutoFormInterceptor()` (không chặn form); bỏ qua `sendAnalyticsPing()` (không gửi analytics) |
| 2. `ast-mapper.js` | `getHelperScript()` inject `__APEX_STUDIO_LANDING_ID__` / `__APEX_STUDIO_DOMAIN__` + ghi đè `window.APEX_LANDING_CONFIG` | Đảm bảo preview dùng đúng landing đang mở, không phải template gốc (vd `mxv_080_online`) |
| 3. `db/landings.go` | `ListLandingFiles()` dùng `DISTINCT ON (file_path)`, ưu tiên `domain` cụ thể; `GetLandingPage()` ưu tiên `domain` khớp chính xác | Domain cụ thể được ưu tiên hơn bản `domain = ''` (template) |

Nếu thêm bất kỳ module DOM-mutating nào mới vào `tracker.js`, **phải** guard bằng:

```javascript
if (isAnyPreview()) return;
```

---

## 8. Tối ưu chuyển đổi

| Yếu tố | Gợi ý |
|---|---|
| Tốc độ tải | Ảnh dùng `.webp` (repo có cả `.png` và `.webp` — dùng `.webp`) |
| Mobile-first | Phần lớn traffic từ Facebook/TikTok ads trên mobile |
| Form ngắn | Chỉ hỏi tên + SĐT. Mọi thông tin khác hỏi sau |
| CTA rõ ràng | Nút submit nêu rõ giá trị ("ĐĂNG KÝ BUỔI HỌC 2 GIỜ MIỄN PHÍ") |
| Social proof | Số suất + đã đăng ký (binding từ `config.json`) |
| Không chặn autofill | Autofill là nguồn lead lớn — xem `docs/SECURITY.md` mục 4 |
| Redirect sau đăng ký | `redirect_url` → `chucmung.html` |

---

## 9. Bảo trì

### Đổi nội dung hàng loạt
Sửa `config.json` → refresh. Không cần sửa HTML.

### Đổi code HTML
Code Studio (`Ctrl+S`) hoặc upload file qua File Manager.

### Tạm ngưng landing (dừng nhận lead)
- `is_active = false` → khách thấy trang bảo trì (hoặc redirect nếu có `redirect_url`)
- Hoặc dùng Feature Flag trong CRM

> ⚠️ Khi tắt `is_active`, nhớ `landingCache.Reload()` (CRM tự làm, nhưng nếu sửa trực tiếp DB thì phải restart backend).

---

## 10. Tài liệu liên quan

- `frontend/landing/HUONG_DAN_LANDING_FORM_TRACKING.md` — hướng dẫn chi tiết form tracking
- `docs/SECURITY.md` — PoW, honeypot, autofill-safe
- `docs/DATABASE.md` — schema `landing_pages`, `domain_routes`
- `docs/ARCHITECTURE.md` — edge router, cache
