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
| `nguon_default` | Nguồn traffic ghi vào lead. **Mặc định = `page_id`** (mỗi landing 1 nguồn riêng). Giá trị `landing_chinh` bị coi như rỗng → backend tự thay bằng `landing_id` |
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

> ⚠️ **Route prefix khớp cả URL dạng `/<tên>.html`** (fix 2026-11-16):
> `/chucmung.html` PHẢI khớp route prefix `/chucmung` (không chỉ `/chucmung/`).
> Nếu không, trang chúc mừng rơi xuống nhánh **phục vụ file tĩnh từ đĩa** ⇒
> mất `data-landing-id` ⇒ `tracker.js` ghi nhầm landing ⇒ **thống kê chồng chéo**.
> Chi tiết: mục 7.1.

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
  "nguon": "mxv_080_online",
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

#### ⚠️ Meta Pixel: phải dùng `f._fbq` (fix 2026-11-13)

Snippet Meta Pixel **chính thức** phải là:

```js
if (!f._fbq) f._fbq = n;   // ✅ ĐÚNG
```

**KHÔNG** được viết `if (!f._fbq) f._bq = n;` (thiếu chữ `f`).

Meta Pixel chính thức yêu cầu `if(!f._fbq) f._fbq = n;`. Gán sai ⇒ `window._fbq` không
bao giờ được tạo ⇒ `fbevents.js` sau khi nạp sẽ tự tạo **instance thứ hai** ⇒
phát hiện `window.fbq !== window._fbq` ⇒ Meta báo trong console:

```
[Meta Pixel] - Multiple pixels with conflicting versions were detected on this page.
```

⇒ PageView bị đếm sai/lặp, ảnh hưởng trực tiếp tới tối ưu ngân sách Meta Ads.

| Nơi | Cách sửa |
|---|---|
| HTML viết tay (Code Studio) | Sửa trực tiếp trong editor. |
| HTML đã lưu trong DB | `RepairMetaPixelSnippet()` chạy lúc khởi động server ⇒ tự sửa `landing_pages.code_html` + `landing_files.content`. |

> ⚠️ Đừng dùng `LIKE '%_bq%'` để tìm lỗi này — `_` là wildcard trong `LIKE` nên sẽ
> match nhầm. Dùng `strpos()` + `replace()` (literal, không phải regex).

### ⚠️ Dedup PageView: `event_id` dùng chung Pixel ↔ CAPI (fix 2026-11-16)

**Vấn đề:** nếu cả Pixel trên web và CAPI trên server cùng bắn `PageView` mà
**không đồng bộ `event_id`**, Meta sẽ tính **1 lượt vào trang thành 2 lượt
PageView** ⇒ sai lệch toàn bộ chỉ số Landing Page View + phân phối quảng cáo.

**Điều kiện để Meta gộp 2 sự kiện làm 1:**

1. `event_name` giống nhau — đều là `PageView`.
2. `event_id` trùng khớp **100%** — biến `pvEventId` bắn ở `fbq('track', …)`
   phải được gửi kèm lên API server để server truyền vào trường `"event_id"`
   trong payload CAPI.
3. Thứ tự thời gian — trình duyệt gọi `fbq('track', 'PageView')` **trước**
   (để kịp sinh cookie `_fbp`), sau đó mới gửi request kích hoạt CAPI kèm
   cookie `_fbp`, `_fbc`, `client_user_agent` và `pvEventId`.

⚠️ `eventID` là **tham số thứ 4** của `fbq` — KHÔNG phải thuộc tính trong object
tham số 3:

```js
// ✅ ĐÚNG
fbq('track', 'PageView', {}, { eventID: pvEventId });
// ❌ SAI — Meta không đọc được eventID ⇒ không gộp
fbq('track', 'PageView', { eventID: pvEventId });
```

**Cơ chế đã triển khai:**

| Tầng | Thành phần | Nội dung |
|---|---|---|
| Server | `injectPageViewEventIDSnippet()` | Chèn script ở **đầu `<head>`** sinh MỘT `window.APEX_PV_EVENT_ID` (prefix `pv-`). Idempotent. |
| Server | `rewritePageViewWithEventID()` | Ghi đè mọi lệnh `fbq('track','PageView')` **hardcode trong HTML landing** (export LadiPage) để kèm `{ eventID: window.APEX_PV_EVENT_ID }`. Lệnh đã có `eventID` ⇒ giữ nguyên. |
| Server | `InjectDynamicScriptsWithBase()` | Block Meta Pixel auto-inject cũng kèm `{ eventID: window.APEX_PV_EVENT_ID }`. |
| Client | `ensurePageViewEventId()` (`tracker.js`) | Đọc `window.APEX_PV_EVENT_ID` (do server inject) hoặc tự sinh ⇒ **1 giá trị duy nhất / lần tải trang**. |
| Client | `buildAnalyticsPayload()` | Gửi `event_id` lên server trong payload `/api/analytics/track`. |
| Server | `dispatchCAPIPageView()` | Landing **có** Pixel ⇒ chỉ bắn CAPI PageView khi có `event_id` **chung** (từ payload, fallback `analytics_sessions.pv_event_id`). Thiếu ⇒ **bỏ qua server PageView** (chấp nhận thiếu 1 lượt thay vì thừa 1 lượt). |
| DB | `analytics_sessions.pv_event_id` | Lưu `event_id` đầu tiên của phiên (fallback khi ping đầu bị throttle / client cache bản tracker.js cũ). |

**HTML landing viết tay** (Code Studio) nên dùng mẫu sau:

```html
<!-- Meta Pixel Code -->
<script>
  !function (f, b, e, v, n, t, s) { /* …fbevents.js… */ }(window, document, 'script',
      'https://connect.facebook.net/en_US/fbevents.js');

  // Tạo hoặc lấy event_id đồng bộ với request gửi về server CAPI
  window.APEX_PV_EVENT_ID = window.APEX_PV_EVENT_ID ||
      ('pv_' + Date.now() + '_' + Math.random().toString(36).substring(2, 9));
  var pvEventId = window.APEX_PV_EVENT_ID;

  fbq('init', '1558828029597854');
  fbq('track', 'PageView', {}, { eventID: pvEventId });
</script>
```

**Kiểm tra trong CRM:** tab **Quảng cáo Facebook** → bảng *Lịch sử gửi sự kiện* →
cột `event_id` hiển thị đầy đủ + badge **"✓ đã gộp với Pixel"** cho sự kiện
`PageView` có prefix `pv-`.

### ⚠️ Landing CHỈ dùng Pixel — KHÔNG gắn `eventID` (fix 2026-11-18)

> Yêu cầu chủ repo 2026-11-18: `landing_cu` (deploy tại
> `https://ads.giaodichhanghoatuonglai.com/landing`) **KHÔNG dùng CAPI server**,
> chỉ dùng Meta Pixel nhúng trong HTML.

**Vì sao `eventID` phải BỎ khi landing không bật CAPI:**

`eventID` (tham số thứ 4 của `fbq`) chỉ có ý nghĩa khi server CAPI bắn sự kiện
**CÙNG** `event_id` — Meta chỉ gộp browser ↔ server khi trùng khớp 100%.
Landing KHÔNG bật CAPI ⇒ không có sự kiện server để gộp ⇒ gắn `eventID` chỉ là
sửa HTML không cần thiết ⇒ **giữ nguyên block Meta Pixel GỐC**.

**Cơ chế (2 lớp):**

| Lớp | Cơ chế | Vị trí |
|---|---|---|
| 1. Tầng server (chốt chặn tuyệt đối) | `serverCAPIDenyList = ["landing_cu", "landing_lznoew"]` + env `META_CAPI_BLOCKED_LANDINGS` | `backend/integrations/facebook/dispatcher.go` → `resolveConfig()` |
| 2. Dòng DB riêng | `is_active = FALSE` cho `landing_cu` (migration `D3`) | `backend/db/db.go` |

> ⚠️ `landing_lznoew` là trang "Chúc mừng" (thank-you page) — PageView ở đây
> KHÔNG phải lượt xem quảng cáo ⇒ KHÔNG bắn CAPI (tránh nhân đôi PageView).
> Fix dedup 2026-11-18 chỉ áp dụng cho các landing còn lại.

**Hệ quả trong code:**

| Hàm | Hành vi mới |
|---|---|
| `InjectDynamicScriptsWithBase()` | Chỉ gọi `rewritePageViewWithEventID()` + inject `window.APEX_PV_EVENT_ID` **khi** `isCAPIActive(landing_id) == true`. Block Pixel auto-inject cũng chọn bản có/không `eventID` theo cùng điều kiện. |
| `rewritePageViewWithoutEventID()` | Landing **không** bật CAPI ⇒ **GỠ** `{ eventID: window.APEX_PV_EVENT_ID }` khỏi lệnh `fbq('track','PageView')` (kể cả khi template LadiPage đã hardcode sẵn) ⇒ HTML về đúng block Pixel gốc. Không đụng sự kiện khác (`fbq('track','Lead', …)`). |
| `RepairMetaPixelPageViewEventID()` | Landing bật CAPI ⇒ thêm `eventID`. Landing **không** bật CAPI ⇒ **gỡ** `eventID` (đưa HTML về block Pixel gốc) — sửa cả `landing_pages.code_html` lẫn `landing_files.content`. |

> ⚠️ Landing KHÔNG bật CAPI mà vẫn gắn `eventID` ⇒ Meta ghi nhận PageView thiếu
> thông số ⇒ chiến dịch báo lỗi dữ liệu (đúng triệu chứng đã gặp 2026-11-18).

**HTML landing viết tay** (Code Studio) của landing CHỈ dùng Pixel nên dùng mẫu GỐC:

```html
<!-- Meta Pixel Code -->
<script>
!function(f,b,e,v,n,t,s)
{if(f.fbq)return;n=f.fbq=function(){n.callMethod?
n.callMethod.apply(n,arguments):n.queue.push(arguments)};
if(!f._fbq)f._fbq=n;n.push=n;n.loaded=!0;n.version='2.0';
n.queue=[];t=b.createElement(e);t.async=!0;
t.src=v;s=b.getElementsByTagName(e)[0];
s.parentNode.insertBefore(t,s)}(window, document,'script',
'https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '1558828029597854');
fbq('track', 'PageView');
</script>
<noscript><img height="1" width="1" style="display:none"
src="https://www.facebook.com/tr?id=1558828029597854&ev=PageView&noscript=1"
/></noscript>
<!-- End Meta Pixel Code -->
```

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

### 🟢 Chuỗi UTM Meta Ads chuẩn (8 macros — cấu hình 1 lần trong Meta Ads Manager)

Dán vào **URL Parameters** ở cấp độ Quảng cáo (Ad level):

```
utm_medium=paid&utm_source={{site_source_name}}&utm_placement={{placement}}&utm_campaign={{campaign.id}}&utm_content={{adset.id}}&utm_term={{ad.id}}&campaign_name={{campaign.name}}&adset_name={{adset.name}}&ad_name={{ad.name}}
```

| Tham số | Macro Meta | Ý nghĩa | Ghi vào DB |
|---|---|---|---|
| `utm_source` | `{{site_source_name}}` | `fb`/`ig`/`msg`/`an` | `utm_source` (đã chuẩn hoá) |
| `utm_placement` | `{{placement}}` | `Facebook_Reels`, `Facebook_Stories`, `Audience_Network_Rewarded_Videos`… | `utm_placement` + `ad_placement` |
| `utm_campaign` | `{{campaign.id}}` | ID chiến dịch | `utm_campaign` |
| `utm_content` | `{{adset.id}}` | ID nhóm quảng cáo | `utm_content` |
| `utm_term` | `{{ad.id}}` | ID bài quảng cáo | `utm_term` |
| `campaign_name` | `{{campaign.name}}` | Tên chiến dịch | `campaign_name` |
| `adset_name` | `{{adset.name}}` | Tên nhóm (chứa thông tin target) | `adset_name` |
| `ad_name` | `{{ad.name}}` | Tên bài quảng cáo | `ad_name` |

> `{{account.id}}` **không tồn tại** trong Meta. Muốn phân biệt nhiều tài khoản quảng cáo
> cùng đổ về 1 landing → gõ cứng `&acc_id=tk01` trong ô URL Parameters.

**Định danh người dùng thay thế UID Facebook** (được đọc tự động bởi `tracker.js`):

| Tham số | Nguồn | Ghi vào DB |
|---|---|---|
| `fbclid` | Query string Meta tự gắn khi click | `fbclid` |
| `_fbc` | Cookie Meta Pixel | `fbc` |
| `_fbp` | Cookie Meta Pixel | `fbp` |
| `igshid` | Instagram share link | `igshid` |
| `meta_click_id` | Một số dạng CTA của Meta | `meta_click_id` |

> `_fbc` được **dựng tự động** theo chuẩn Meta (`fb.1.0.<fbclid>`) nếu trình duyệt
> chưa gắn cookie `_fbc` ⇒ CAPI stitch được cả khi Pixel bị chặn.

### 📱 Thu thập thông tin thiết bị & hành vi (2026-11-12)

`tracker.js` thu thập **toàn bộ** những gì web có thể biết về người dùng:

| Nhóm | Trường |
|---|---|
| Trình duyệt | `browser`, `browser_version`, `browser_engine` |
| In-app browser | `is_in_app_browser`, `in_app_type` — phân biệt `Facebook In-App` (`FBAV`/`FBAN`), `Instagram`, `Zalo`, `TikTok`, `Cốc Cốc` vs trình duyệt ngoài |
| Thiết bị | `device_brand`, `device_model`, `device_model_code`, `device_name` — map mã thật (`SM-A525F` → **Samsung Galaxy A52**), iOS UA → **iPhone 15 Pro**… |
| Phần cứng | `device_memory` (RAM MB), `device_cores` (CPU), `device_touchpoints`, `screen_res`, `viewport_res`, `screen_color_depth`, `pixel_ratio` |
| Môi trường | `timezone`, `language`, `platform_info`, `connection_type` (4g/5g), `device_state` |
| VPN | `is_vpn`, `is_proxy`, `is_tor`, `vpn_provider`, `vpn_confidence`, `vpn_reason` |
| Hành vi | `duration_seconds`, `max_scroll_percent` (mốc 25/50/75/90/100%), `total_clicks`, `visible_seconds`, `hidden_seconds`, `has_interacted_form`, `form_submit_attempts`, `video_play`, `zalo_clicks`, `tel_clicks`, `events[]` (timeline) |

### 🔐 Proof-of-Work (PoW) — vòng đời challenge (fix 2026-11-12)

| Mốc | Giá trị |
|---|---|
| TTL server | **600 giây** (10 phút) — `backend/security/challenge.go` |
| Dung sai lệch đồng hồ | **60 giây** (clock skew grace) |
| Biên an toàn client | **90 giây** — client tự lấy challenge mới nếu `expires_at <= now + 90` |
| Hash format | `SHA-256(challenge_id + ":" + salt + ":" + solution)` — phải khớp **chính xác** client & server |
| Difficulty | `3` ⇒ hash bắt đầu bằng `"000"` |

**Luồng submit an toàn (client):**
1. `ensureChallengeReady()` — nếu challenge sắp hết hạn → `GET /api/auth/challenge` lấy challenge mới và giải ngay.
2. `disableSubmitButtons(true)` — **disable nút ngay khi click**, trước khi gọi mạng ⇒ chặn double-click.
3. Khoá idempotency theo số điện thoại trong 1 khoảng thời gian ⇒ 1 số = 1 lần gửi.
4. Server kiểm tra replay bằng `LoadOrStore` trên nonce ⇒ challenge dùng 1 lần là duy nhất.

> ⚠️ **Sai format hash là nguyên nhân #2 gây lỗi PoW.** Client và server **phải** dùng
> cùng công thức nối chuỗi. Xem `docs/SECURITY.md` mục PoW.

### ⚠️ Cặp (challenge, solution) phải LUÔN khớp 1-1 (fix 2026-11-15)

**Triệu chứng:** lead thật #65 (`0368942836`) bị ghi vào `spam_quarantine` với lý do
"Lỗi Proof-of-Work" ⇒ **lead bị rơi**.

**Quy tắc bất biến:** `challenge_id` + `challenge_salt` + `challenge_solution` phải là
**CÙNG MỘT BỘ**. Tách rời ⇒ server băm `SHA256(salt_KHÁC:solution)` ⇒ không khớp ⇒ lead bị cách ly.

**Cấu trúc state ĐÚNG** (`frontend/admin/js/tracker.js`):

```javascript
// ✅ ĐÚNG — 1 biến duy nhất chứa CẶP, luôn khớp 1-1
let challengeState = { challenge: null, solution: '' };
let challengeFetchPromise = null;   // single-flight: gom lời gọi song song
```

```javascript
// ❌ SAI — 2 biến rời rạc ⇒ race condition (đây chính là bug 2026-11-15)
let currentChallenge = null;
let currentChallengeSolution = null;
```

**3 hàm bắt buộc trong `tracker.js`:**

| Hàm | Vai trò |
|---|---|
| `solveProofOfWork(challenge)` | Tìm nonce sao cho `SHA256(id:salt:nonce)` bắt đầu bằng `"000"` |
| `verifyPowSolution(challenge, solution)` | Tự băm lại để xác nhận cặp khớp — **lớp phòng thủ cuối** |
| `ensurePowPairValid()` | Gọi `verifyPowSolution`; lệch ⇒ **giải lại** chứ KHÔNG gửi payload hỏng |

> ⚠️ **Không được dùng `fetch` challenge trực tiếp trong nhiều nơi.** Mọi lời gọi phải đi qua
> `ensureChallengeReady()` ⇒ dùng chung `challengeFetchPromise` (single-flight).
>
> ⚠️ **Server có lớp phục hồi riêng** (`healMismatchedSolution()`) vì trình duyệt có thể
> đang dùng `tracker.js` **bản cũ trong cache**. Xem `docs/SECURITY.md` mục "Nguyên nhân #5".
>
> ⇒ Khi sửa `tracker.js` phải chạy `node scripts/sync-static-mirror.js` để đồng bộ **7 bản mirror**
> (`frontend/admin/js/tracker.js` là nguồn chuẩn).

### 🔒 Cách ly tracking giữa các landing (bắt buộc)

`tracker.js` namespace hoá **toàn bộ** storage theo `landing_id`:

| Storage | Key |
|---|---|
| visitor ID | `apex_vid_<landing_id>` |
| session ID | `apex_session_id_<landing_id>` |
| cờ đã đăng ký | `apex_lead_registered_<landing_id>` |
| cookie dedup | `apex_dedup_<landing_id>` |

⇒ Không bao giờ có chuyện visitor của landing A bị ghi nhận sang landing B.
Landing mới **bắt buộc** khai báo:

```html
<script src="/p/doanh_chung/tracker.js"
        data-landing-id="doanh_chung"
        data-nguon="doanh_chung"></script>
```
và `config.json` phải có `landing_id` + `nguon` **khớp chính xác**.

> ⚠️ **Quy tắc nguồn theo từng landing** (sửa 2026-11-13):
> `nguon` mặc định = `landing_id`. Giá trị `landing_chinh` được backend coi như
> "rỗng" và tự thay bằng `landing_id` (`lead_handler.go`, `analytics_handler.go`,
> `tracker.js`). Nếu landing cố ý khai báo `data-nguon` riêng thì vẫn được tôn trọng.
>
> **Vì sao**: trước đây mọi landing ghi chung `nguon = "landing_chinh"` ⇒ 4 landing
> dùng chung 1 nguồn ⇒ không lọc tách được, thống kê bị chồng chéo giữa các landing.
>
> ⚠️ **Landing "chỉ xem" (view-only)** — vd `landing_lznoew` là trang "chúc mừng"
> được điều hướng từ landing, không có form gửi lead. Trang này:
> - **KHÔNG** được set `converted_lead` (client không gửi cờ này nữa — chỉ backend
>   set khi có lead thật trong bảng `leads`).
> - Vẫn ghi view/analytics như bình thường.
> - Session attribution là **first-touch** ⇒ phiên tính cho landing gốc (landing có
>   form), không bị "chuyển" sang landing chúc mừng.

### URL lead (`url`)

Tracker gửi kèm `url: window.location.href` — **URL đầy đủ của trang đang đăng ký** kèm query string.
Server lưu vào cột `leads.url` và đồng bộ sang Google Sheets bằng field mapping `url`.

| Nguồn | Thứ tự ưu tiên |
|---|---|
| Payload client | `url` → `page_url` → `entry_url` → `landing_url` |
| Fallback server | `Referer` → `Origin` → `X-Forwarded-Proto` + `Host` + path |

> Landing page mới **không cần làm gì thêm** — `tracker.js` đã gửi sẵn.

### ⛔ Chặn SĐT rác — tự động chuyển trang "chúc mừng" (thêm 2026-11-16)

**Áp dụng cho TẤT CẢ landing** (kể cả `doanh_chung`, `xu_huong_1`, `xu_huong_2`,
`xu_huong_3`) vì tất cả dùng chung 1 file `tracker.js`.

Khi khách nhập SĐT rác (`0987654321`, `0909090909`, 7 số cuối giống hệt…) và bấm
"Đăng ký":

```
tracker.js (client)
  └─ blockJunkPhone()
       ├─ setJunkBlocked()   → cookie + localStorage `apex_junk_blocked_<landing>` (30 ngày)
       ├─ redirectAfterJunk() → window.location.replace('chucmung.html')
       └─ return true ⇒ DỪNG: KHÔNG POST /api/leads, KHÔNG ping analytics,
                          KHÔNG bắn fbq('track','Lead')
```

⇒ **Không tạo lead ⇒ không bắn Meta CAPI ⇒ không tốn tiền QC.**

Khi tải lại trang form mà cookie `apex_junk_blocked_<landing>` còn hiệu lực ⇒ tự
động chuyển sang `chucmung.html` (khách không nhập lại được).

Lớp phòng thủ thứ 2 phía server: `backend/security/junk_phone.go`
(`security.IsJunkPhone`) gọi trong `lead_handler.go` bước 6.1 — nếu client bị
bypass ⇒ không tạo lead, ghi `spam_quarantine`, trả `200 OK` giả.

| Nơi | Hành vi |
|---|---|
| `sendLeadToBackend()` | ⛔ chặn TRƯỚC khi gọi `fetch` (chokepoint chính) |
| `initAutoFormInterceptor()` | ⛔ chặn ngay khi bấm nút (landing tự xử lý form riêng) |
| Tải trang | ⛔ tự chuyển `chucmung.html` nếu cookie chặn còn hiệu lực |

> ⚠️ Landing tự viết handler submit riêng (vd `doanh_chung` dùng
> `assets/js/apex-adapter.js` → `ApexTracker.sendLeadToBackend()`) **vẫn được bảo
> vệ** vì chặn nằm ngay trong `sendLeadToBackend()`.
>
> ⚠️ Sửa `tracker.js` ⇒ chạy `node scripts/sync-static-mirror.js` (7 bản mirror)
> + deploy ⇒ backend tự đồng bộ `tracker.js` vào `landing_files` cho mọi landing
> (`SyncSharedStaticFilesToDB`).
>
> Chi tiết heuristic + bảng mẫu rác: `docs/SECURITY.md` mục 6.1.

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

### 🐛 Preview Code Studio — không load giao diện & assets (fix 2026-11-15)

**Triệu chứng:** mở Code Studio với landing `doanh_chung` ⇒ preview trắng, không
có CSS/JS/ảnh. Network tab đầy lỗi 404:
`/admin/assets/css/style.css`, `/admin/assets/vendor/tailwind/tailwind.css`…

**Nguyên nhân gốc (2 lỗi riêng biệt):**

**1) `<base>` + helper script bị chèn NHẦM vào trong HTML comment.**

`ast-mapper.js` dùng `result.toLowerCase().indexOf('<head>')` để tìm thẻ `<head>`:

- Chỉ khớp đúng chuỗi `<head>` **không có thuộc tính** ⇒ thẻ `<head>` sau khi
  instrument (`<head data-ve-id="node-2">`) **không** khớp.
- Khớp nhầm **text `<head>` nằm trong HTML comment** — landing `doanh_chung` có
  comment `(inject style vào cuối <head>)` ở `frontend/landing/doanh_chung/chung.html:39`.

⇒ `<base href="…">` + helper script bị chèn **bên trong comment** ⇒ trình duyệt coi
như không tồn tại ⇒ `document.querySelector('base')` = `null` ⇒ URL tương đối
(`assets/css/style.css`) phân giải theo `/admin/studio.html` ⇒ `/admin/assets/…` ⇒ **404**.

> ✅ **Cách sửa:** hàm `findRealTagInsertIndex(html, tagName)` trong `ast-mapper.js`
> quét HTML **bỏ qua comment `<!-- … -->`**, khớp đúng **tên thẻ** (không nhầm
> `<head>` với `<header>`), bỏ qua dấu `>` nằm trong thuộc tính có quote, và trả về
> index **ngay sau** thẻ mở ⇒ chèn đúng chỗ, vào `<head>` thật.

**2) `<base href>` trỏ sai ⇒ tài nguyên tương đối gọi nhầm host.**

`<base href="/">` ⇒ `assets/css/style.css` phân giải thành
`https://crm.hanghoaphaisinh.net/assets/css/style.css` — nhưng assets của landing
nằm trong bảng `landing_files` **theo `page_id`**, không nằm ở root của CRM host.

> ✅ **Cách sửa:** `getPreviewBaseHref()` trong `ast-mapper.js` trả
> `/p/<page_id>/` — đúng route `ServePublicLanding` (`backend/main.go`), vốn đọc
> `landing_files` theo `page_id` ⇒ tải được CSS/JS/ảnh/video của **đúng landing**.

| Trước | Sau |
|---|---|
| `indexOf('<head>')` (khớp nhầm comment) | `findRealTagInsertIndex(html, 'head')` (bỏ qua comment) |
| `<base href="/">` | `<base href="/p/<page_id>/">` |

### 🖥️ Preview PC — khung hình & zoom (fix 2026-11-15)

**Triệu chứng:** ở chế độ PC/Laptop, khung preview bị tràn (nhìn thấy khung bị cắt
cụt) và **zoom in/out không làm thay đổi khung hình web**.

**Nguyên nhân:** `gizmo.js` ép `min-width: 780px` cho wrapper và
`viewportW = Math.max(boxW / zoom, 780)`:

- Panel split hẹp chỉ ~566px ⇒ wrapper ≥ 780px ⇒ **tràn khung**.
- `Math.max(…, 780)` **clamp** viewport nội bộ ⇒ đổi zoom **không** làm viewport
  đổi ⇒ media query không đổi ⇒ bố cục web **không co giãn** ⇒ zoom "vô hiệu".

> ✅ **Cách sửa:**
> - `STUDIO_PC_MIN_WIDTH` = `320` (bỏ clamp 780), wrapper `min-width: 0`.
> - `computeCanvasViewportWidth(boxW, zoom, isMobile)`: PC ⇒ `boxW / zoom`
>   (mô phỏng đúng zoom trình duyệt), Mobile ⇒ cố định 390px.
> - Vì viewport có thể < 769px khi zoom vào ⇒ PC styles (`@media (min-width:769px)`)
>   sẽ tắt. `ast-mapper.js` sinh thêm style **ép PC** (chỉ trong preview, selector
>   `html[data-apex-preview-device="pc"]`) + `syncPreviewDeviceToIframe()` gửi
>   `APEX_CMD_SET_PREVIEW_DEVICE` xuống iframe. **Style này không bao giờ được ghi
>   vào `code_html`/`landing_files`** ⇒ production vẫn dùng media query thật.

### 📋 Nhân bản landing phải copy đủ file (fix 2026-11-15)

**Triệu chứng:** nhân bản landing ⇒ bản sao chỉ có HTML, không có CSS/JS/ảnh.

**Nguyên nhân:** `DuplicateLandingPage()` lưu file vào `newPage.Domain` (domain
của landing **nguồn**) thay vì bản gốc `domain = ''` ⇒ bản sao không có row file
hợp lệ ⇒ `ListLandingFiles()` trả về rỗng ⇒ preview bản sao trắng.

> ✅ **Cách sửa:** `DuplicateLandingPage()` giờ:
> - Trả về **số file đã copy** (`copied_files` trong response API).
> - Copy **toàn bộ** file của landing nguồn vào row gốc `domain = ''`.
> - Bản sao **không** gán domain của landing nguồn (tránh tranh chấp route).
> - Xoá cấu hình riêng của domain nguồn khỏi `settings` (`path_prefix`,
>   `google_sheet_ids`, `google_sheet_sync_mode`).
> - Khi user Deploy, `HandleDeployLanding` gọi `SyncLandingFilesToDomain()` để
>   đồng bộ cây file sang domain đích ⇒ không mất file.

### 🚨 NHÂN BẢN LANDING PHẢI TÁCH BIỆT ĐỊNH DANH (fix 2026-11-15)

**Triệu chứng:** nhân bản landing `xu_huong_1` từ `doanh_chung` ⇒ landing mới
vẫn chạy, vẫn nhận khách, nhưng **mọi view + lead đều bị ghi về `doanh_chung`**
⇒ thống kê chồng chéo, không tách được, không bao giờ phân biệt được.

**Nguyên nhân:** `DuplicateLandingPage()` copy **nguyên si** "dấu vân tay" định danh
của landing nguồn sang bản sao:

| Nơi | Giá trị bị copy | Hậu quả |
|---|---|---|
| `landing_pages.nguon_default` | `"DTN 1"` | Lead ghi nhận dưới nguồn của landing nguồn |
| `landing_files/config.json` | `landing_id` + `nguon` của landing nguồn | `tracker.js` nạp config sai |
| `landing_files/index.html` | `data-landing-id` + `data-nguon` của landing nguồn | **`data-landing-id` là ưu tiên CAO NHẤT** ⇒ view/lead về landing nguồn |
| `landing_pages.code_html` | `window.APEX_LANDING_CONFIG` của landing nguồn | Config inject sai |
| URL trong HTML | `https://lophoc.hanghoaphaisinh.net/` | `og:url` / `canonical` trỏ domain cũ ⇒ crawler đánh giá sai trang |

> ✅ **Cách sửa:** mọi dấu vân tay định danh được **viết lại** theo `page_id` bản sao:
> - `nguon_default` = `newPageID` (quy tắc mục 5: `nguon` mặc định = `landing_id`).
> - `config.json`: `landing_id` + `nguon` = `newPageID`.
> - HTML: `data-landing-id`, `data-nguon`, `APEX_LANDING_CONFIG` = `newPageID`.
> - `settings` bản sao **xoá** `nguon`, `landing_id`, `google_sheet_ids`,
>   `google_sheet_sync_mode` (tránh lead bắn sang sheet của landing nguồn).
> - Validate `page_id` (không rỗng, không khoảng trắng, ≤ 100 ký tự).
>
> **Helper:** `backend/db/landing_identity.go`
> - `RewriteLandingIdentityHTML(content, landingID, nguon, oldDomain, newDomain)`
> - `RewriteLandingConfigJSON(content, landingID, nguon)`
> - `ValidateLandingIdentity(pageID, nguonDefault)`
>
> **Test:** `backend/db/landing_identity_test.go` (8 test, PASS).

### 🔧 File landing phải KHÔNG CÓ BOM (fix 2026-11-15)

**Triệu chứng:** `config.json` / `index.html` upload từ Windows (Notepad, VS Code)
thường có BOM (`EF BB BF`) ở đầu ⇒:

- `tracker.js` gọi `res.json()` ⇒ **throw** `Unexpected token ﻿` ⇒ fallback
  `config.json` **không bao giờ nạp được** ⇒ landing mất cấu hình (title,
  countdown, form mapping).
- Backend `json.Unmarshal` ⇒ **fail** ⇒ `EnsureLandingConfigFile` /
  `RepairLandingIdentity` **không tự sửa được** file BOM ⇒ bug nhân bản landing
  chồng chéo định danh có thể **tái diễn**.

> ⚠️ JSON chuẩn (RFC 8259) **KHÔNG cho phép BOM**.

✅ **Cách sửa:**
- `StripUTF8BOM(content)` (`backend/db/landing_identity.go`) — cắt BOM ở đầu.
- `RewriteLandingConfigJSON()` cắt BOM **trước khi parse** ⇒ kết quả marshal
  lại **không có BOM** ⇒ JSON chuẩn.
- `EnsureLandingConfigFile()` + `setConfigJSONNguon()` cắt BOM trước parse.
- `SaveLandingFile()` + `UpdateLandingCode()` **strip BOM lúc GHI** ⇒ file upload
  từ Code Studio luôn sạch ⇒ không còn BOM trong DB.

> **Vệ sinh dữ liệu cũ:** `UPDATE landing_files SET content = substr(content, 4)
> WHERE content LIKE (chr(65279) || '%');` (+ tương tự cho `landing_pages.code_html`).
>
> ⚠️ **LƯU Ý — `substr` trên `text` đếm KÝ TỰ, không phải BYTE!**
> `substr(content, 4)` cắt 3 **ký tự** ⇒ mất cả `{` đầu JSON / `<!` đầu HTML ⇒
> file hỏng. Phải cắt theo **byte**:
> ```sql
> UPDATE landing_files
> SET content = convert_from(
>       substring(convert_to(content,'UTF8') from 4 for octet_length(content)-3),'UTF8')
> WHERE substring(content,1,1) = chr(65279);
> ```
> ⇒ **Luôn BACKUP trước** (`CREATE TABLE ... AS SELECT`) rồi verify lại đầu file.

### 🔧 Tự sửa chữa landing bị chồng chéo (fix 2026-11-15)

**Vấn đề:** các landing đã nhân bản **trước khi có fix** vẫn còn dấu vân tay
cũ trong DB ⇒ không tự lành.

> ✅ **Cách sửa:** `store.RepairLandingIdentity(ctx)` chạy **mỗi lần backend khởi
> động** (sau `EnsureLandingConfigFile`):
> 1. Gom `nguon_default` theo landing ⇒ landing nào **dùng chung nguồn** với
>    landing khác ⇒ tách ra theo `page_id` (giữ 1 landing gốc).
> 2. Sửa `config.json` (`landing_id` + `nguon`).
> 3. Sửa HTML (`data-landing-id`, `data-nguon`, `APEX_LANDING_CONFIG`).
> 4. Sửa `landing_pages.code_html`.
>
> **Idempotent** — chỉ sửa dữ liệu ĐỊNH DANH, KHÔNG xoá lead, KHÔNG đụng nội dung,
> KHÔNG đụng Google Sheets.

**API admin (bọc `RequireAdminAuth`):**

| Endpoint | Method | Mô tả |
|---|---|---|
| `/api/landings/identity/report` | GET | Báo cáo định danh mọi landing + cảnh báo "nguồn dùng chung" |
| `/api/landings/identity/repair` | POST | Chạy `RepairLandingIdentity()` thủ công + reload cache |


### 🔧 `syncPreviewDeviceToIframe()` phải ở scope GLOBAL (fix 2026-11-15)

**Triệu chứng:** sửa style PC trong Inspector ⇒ không thấy thay đổi khi zoom
làm viewport nội bộ < 769px. Attribute `data-apex-preview-device` không được set.

**Nguyên nhân:** hàm `syncPreviewDeviceToIframe()` bị khai báo **bên trong**
`setStudioDevice()` ⇒ không có scope global ⇒ `inspector.js` (`applyAstStyleChange`)
và `refreshStudioPreview()` gọi `typeof syncPreviewDeviceToIframe === 'function'`
luôn ra `false` ⇒ không sync device xuống iframe.

> ✅ **Cách sửa:** chuyển hàm ra **scope global** (cùng cấp với `setStudioDevice`).

**Bổ sung:** style block `#__apex_preview_pc_force_styles__` giờ **luôn được tạo**
(kể cả rỗng) và handler `APEX_CMD_SET_PREVIEW_DEVICE` **tự tạo** element nếu thiếu
⇒ CSS ép PC luôn áp dụng được ngay từ lần đầu mở Studio.

### ⚡ Throttle analytics ping (fix 2026-11-13)

`ApexTracker.send(type, value)` (được landing gọi cho mọi tương tác: `click`,
`input`, `modal_open`, `phone_input`…) gọi `sendAnalyticsPing()` **ngay lập tức**.

Landing `doanh_chung` gọi `trackEvent()` ở **mỗi keystroke** trong ô tên/SĐT ⇒
mỗi phím gõ = 1 POST `/api/analytics/track`. Người dùng gõ nhanh trên trang dài
⇒ **hàng chục request/giây** ⇒ server bị nghiêng, log đầy lỗi 500, và dữ liệu
analytics bị ghi đè liên tục (`ON CONFLICT` chỉ giữ giá trị mới nhất) ⇒ **mất event thật**.

**Cơ chế throttle (3 giây):**

| Hàm | Khi nào dùng | Hành vi |
|---|---|---|
| `sendAnalyticsPing(extraEvents)` | pageview · heartbeat · `exit` (tab hidden / window close) · `converted` | Gửi **NGAY**, không throttle |
| `sendAnalyticsPingThrottled(extraEvents)` | `ApexTracker.send()` — event tương tác | Gom các lời gọi trong cửa sổ 3s thành **1 request**; events đang chờ được giữ lại (`analyticsPendingEvents`) và gửi kèm lần flush |
| `flushAnalyticsPending()` | Gọi trước `sendAnalyticsPing()` trong handler `visibilitychange` / `beforeunload` | Flush ngay ⇒ **không mất** keystroke/click đang chờ |

> ⚠️ Đừng đổi `ApexTracker.send()` lại về `sendAnalyticsPing()` — sẽ quay lại lỗi spam request.
> Thêm 1 kiểu event mới thì dùng `sendAnalyticsPing()` (gửi ngay) nếu event đó quan trọng.

---

## 7.1 🚨 Sửa lỗi thống kê landing CHỒNG CHÉO (fix 2026-11-16)

**Triệu chứng (anh itdoanh báo):**
> *"lỗi ở phần ql landing thì các landing tôi nhân bản ra vẫn bị chồng chéo dữ
> liệu thông số như lượt xem, lead trong tab đấy k chính xác"*

⇒ Trong tab **Quản lý Landing**, các landing đã nhân bản hiển thị **cùng số lượt
xem / lead** (không tách được), dù nội dung & domain hoàn toàn khác nhau.

### Nguyên nhân gốc (3 tầng, đã xác minh trên production)

| # | Tầng | Nguyên nhân | Hậu quả |
|---|---|---|---|
| **[A]** | Edge router | `ResolveRoute` **không** khớp `/chucmung.html` với route prefix `/chucmung` (`strings.HasPrefix("/chucmung.html", "/chucmung/")` = `false`) | Trang chúc mừng rơi xuống nhánh **phục vụ file tĩnh từ đĩa** ⇒ không có `data-landing-id`, không có `window.APEX_LANDING_CONFIG` |
| **[B]** | `tracker.js` | `loadConfigFile()` hardcode fallback `'mxv_080_online'`; `getLandingConfig()` fallback `window.LadiPageScript.runtime.ladipage_id` | Sinh landing ID rác `6adc99f23115ac001247de00` (21 sessions) + ghi mọi thứ về template `mxv_080_online` |
| **[C]** | DB | `UpdateLandingCode` dùng `WHERE (domain = $3 OR domain = '') AND page_id = $4` ⇒ sửa code trên domain đích **ghi đè luôn** template gốc | Toàn bộ landing cùng `page_id` bị gộp làm 1 ⇒ thống kê chồng chéo |

> 🔍 **Bằng chứng production:** `landing_pages.code_html` của `landing_232` và
> `landing_cu` có **cùng md5** (`31d23cbec24ad026aa03a3f3e794cf07`) và cùng độ dài
> (265594) ⇒ `landing_cu` chính là bản **đổi tên** của `landing_232`. 47 sessions +
> 5 leads của `landing_232` thuộc về `landing_cu`.

### Đã sửa ở code

| File | Sửa gì |
|---|---|
| `backend/cache/domain_cache.go` | `ResolveRoute` khớp thêm URL dạng `/<tên>.html` với prefix `/<tên>` |
| `backend/main.go` (bước 7 — static fallback) | File `.html` tĩnh **không** được phục vụ thô nếu không resolve được landing context ⇒ **404** thay vì phục vụ sai (inject `data-landing-id` khi có context) |
| `frontend/admin/js/tracker.js` (+ 6 bản mirror) | Bỏ fallback `'mxv_080_online'` **và** `ladipage_id` ⇒ chỉ còn `data-landing-id` → `APEX_LANDING_CONFIG.landing_id` → `APEX_LANDING_ID` → URL → `default_landing` |
| `backend/db/landings.go` | Thêm `EnsureLandingPageDomainRow()`; `UpdateLandingCode` chỉ update `WHERE domain = $3 AND page_id = $4`; `ToggleLandingActiveBase()` chỉ toggle template (`domain = ''`) |
| `backend/handlers/landing_handler.go` | `HandleDeployLanding` dùng `ToggleLandingActiveBase()` (không toggle nhầm row domain) |
| `backend/db/domains.go` | `GetAllActiveDomainRoutes()` dùng `SELECT DISTINCT ON (dr.id)` ⇒ không trả trùng route khi 1 landing có nhiều row domain |
| `backend/models/landing.go` | Thêm `HumanViewsCount`, `BotViewsCount`, `Domains []string` |
| `backend/db/landings.go` (`ListLandingPages`) | Tách `views_count` (tổng) / `human_views_count` (`COUNT(*) FILTER (WHERE NOT is_bot)`) / `bot_views_count`; gắn danh sách domain deploy từ `domain_routes` |
| `frontend/admin/js/landings.js` + `index.html` | CR% tính trên `human_views_count` (khớp công thức tab Trung Tâm Báo Cáo); hiển thị badge domain (tối đa 3 + "+N") |

> ⚠️ **Sửa `tracker.js` ⇒ BẮT BUỘC chạy `node scripts/sync-static-mirror.js`** (7 bản mirror).

### Migrations dữ liệu lịch sử

`deploy/fix_landing_attribution_20261116.sql` — sửa `landing_id`/`nguon` của các bản ghi
đã ghi nhầm:

- Bảng backup `analytics_sessions_backup_20261116` / `leads_backup_20261116`
  (tạo bằng `LIKE ... INCLUDING ALL` + **PRIMARY KEY** ⇒ `ON CONFLICT DO NOTHING` có tác dụng ⇒ **idempotent**).
- `landing_232` → `landing_cu`; `mxv_080_online` → landing theo `entry_url` thực tế
  (`/chucmung%` → `landing_lznoew`, `/landing%` → `landing_cu`, `lophoc.*` → `doanh_chung`,
  `xuhuong.*`/`daotao.*` → `xu_huong_1`, còn lại → `default_landing`).
- `6adc99f23115ac001247de00` (ladipage_id) → `landing_lznoew` / `default_landing`.
- Chuẩn hoá `nguon` lạ (`an`, ...) về `landing_id`.

> 🧪 Đã chạy thử trên DB test (`apex_leads_test`) với dữ liệu mô phỏng đúng hình dạng
> production (`backend/tests/seed_attribution_repair_test.sql`) ⇒ chạy 2 lần cho kết quả
> **giống hệt**, backup **không nhân bản**.

### Test tự động

| File | Kiểm tra |
|---|---|
| `backend/cache/domain_cache_route_test.go` | `ResolveRoute` khớp `.html` không phá route thường, không che route dài hơn |
| `backend/db/landings_dedupe_domains_test.go` | `dedupeLandingPagesByPageID` gộp/dedupe danh sách domain |
| `backend/tests/e2e_landing_metrics_crosscontamination_test.js` | Quét pattern code (Go + JS + mirror) để chống tái phát |

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

## 9.1 Landing `doanh_chung` (thêm 2026-11-12)

| Hạng mục | Giá trị |
|---|---|
| `landing_id` / `nguon` | `doanh_chung` |
| Nguồn mã | `C:\code\apex_moi\apex\chungkhoan\chung` → copy vào `frontend/landing/doanh_chung/` |
| File chính | `chung.html` |
| Script | `assets/js/protect.js`, `assets/js/apex-adapter.js` |

### Cấu trúc

```
frontend/landing/doanh_chung/
├── chung.html
├── config.json          ← landing_id + nguon = doanh_chung
├── assets/
│   ├── css/…
│   ├── img/…
│   └── js/
│       ├── protect.js       ← chống chèn trùng, autofill-safe
│       └── apex-adapter.js  ← bridge form → ApexTracker.sendLeadToBackend()
```

### Điểm khác biệt so với landing cũ

| Điểm | `landing_cu` / `mxv_080_online` | `doanh_chung` |
|---|---|---|
| Storage key | namespace theo `landing_id` | namespace theo `landing_id` (tách riêng hoàn toàn) |
| Tracker | `tracker.js` (dùng chung) | `tracker.js` (dùng chung) — namespace tự động theo `landing_id` |
| Bridge form | inline script | `assets/js/apex-adapter.js` — 1 nơi duy nhất gọi `sendLeadToBackend()` |
| Meta Pixel | trong `settings` CRM | trong `settings` CRM (không hardcode trong HTML) |
| Google Sheets | qua API `/api/leads` | qua API `/api/leads` — **không** hardcode secret trong file |

### ⚠️ Quy tắc

1. **KHÔNG** hardcode Google Sheets API key / script URL vào HTML của landing.
   `apex-adapter.js` chỉ expose `window.ApexSheetsDirect` làm shim tương thích, nhưng
   dữ liệu vẫn đi qua `POST /api/leads` ⇒ không rò rỉ secret.
2. **MỘT** handler gọi `ApexTracker.sendLeadToBackend()` (xem mục 3 mục 5).
3. `data-landing-id` / `data-nguon` trong HTML gốc có thể vẫn là `doanh_chung`
   (do nhân bản), nhưng runtime backend **tự ghi đè về đúng landing đang phục vụ**
   (`InjectDynamicScriptsWithBase` + regex `reStaleDataLandingID*` — fix 2026-11-18),
   còn `apex-adapter.js` resolve động qua `ApexTracker.getLandingConfig()`
   ⇒ lead/view không bao giờ ghi nhầm về `doanh_chung` nữa.

---

## 10. Tài liệu liên quan

- `frontend/landing/HUONG_DAN_LANDING_FORM_TRACKING.md` — hướng dẫn chi tiết form tracking
- `docs/SECURITY.md` — PoW, honeypot, autofill-safe
- `docs/DATABASE.md` — schema `landing_pages`, `domain_routes`
- `docs/ARCHITECTURE.md` — edge router, cache
