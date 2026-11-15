# API Reference

Base URL production: `https://crm.hanghoaphaisinh.net` (qua Caddy) hoặc `http://127.0.0.1:8080` (nội bộ).

Tất cả response trả `Content-Type: application/json` trừ khi phục vụ file tĩnh.

---

## 1. Ma trận quyền truy cập

| Nhóm | Endpoint | Auth |
|---|---|---|
| **Public** | `POST /api/leads` | — |
| **Public** | `GET /api/landing/config` | — |
| **Public** | `POST /api/analytics/track` | — |
| **Public** | `GET /api/auth/challenge` | — |
| **Public** | `GET /api/internal/check-domain` | — (chỉ Caddy local) |
| **Public** | `GET /health` | — |
| **Admin** | Tất cả `/api/admin/*`, `/api/leads` (GET), `/api/crm/*`, `/api/analytics/*` (trừ track), `/api/domains/*`, `/api/landings/*`, `/api/integrations/sheets/*`, `/api/facebook-ads/*` | `RequireAdminAuth` |

---

## 2. Health & Auth

### `GET /health`
```json
{ "status": "ok", "time": "2026-11-04T08:53:16Z" }
```

### `POST /api/auth/login` (alias `POST /api/admin/login`)
```json
// Request
{ "username": "admin", "password": "ApexAdmin@2026!" }

// Response 200
{
  "success": true,
  "token": "<bearer-token>",
  "user": { "id": 1, "username": "admin", "role": "super_admin" }
}
```
Set cookie `apex_session` (HttpOnly) **và** trả token để dùng header `Authorization: Bearer <token>`.

### `GET /api/auth/me` (alias `/api/admin/me`)
Trả thông tin admin hiện tại. `401` nếu session hết hạn hoặc bị revoke.

### `POST /api/auth/logout`
Revoke session hiện tại.

### `POST /api/admin/change-password`
```json
{ "current_password": "...", "new_password": "..." }
```
Đổi mật khẩu → hash Argon2id → revoke **mọi** session khác.

### `GET /api/admin/sessions`
Danh sách phiên đang hoạt động (id, ip, device, browser, os, city, last_active_at).

### `POST /api/admin/sessions/revoke`
```json
{ "session_id": 37 }
```

### `POST /api/admin/sessions/revoke-others`
Revoke tất cả trừ session hiện tại.

### `GET /api/admin/audit-logs`
Query: `?page=1&limit=50&action=&username=`

---

## 3. Lead — Public

### `POST /api/leads`

Gửi lead mới. **Đây là endpoint quan trọng nhất của hệ thống.**

```json
{
  "fullName": "Nguyễn Văn A",
  "phone": "0984386538",
  "nguon": "landing_chinh",
  "landing_id": "mxv_080_online",
  "url": "https://hanghoaphaisinh.net/?utm_source=facebook&utm_medium=paid&utm_campaign=KhoaHoc_MXV_T11",
  "session_id": "sess_88573x64_mw1kdrn3",
  "challenge_id": "ch_4585712799a4405c09db91027f8099fd",
  "challenge_salt": "732721a5a7c71469ba38d3a5b126332b",
  "challenge_difficulty": 3,
  "challenge_expires_at": 1793763664,
  "challenge_signature": "a06e93a8...",
  "challenge_solution": "7584",
  "is_autofill": true,
  "max_scroll": 99,
  "clicks_count": 3,
  "time_on_page": 66,
  "utm_source": "facebook",
  "utm_medium": "paid",
  "utm_campaign": "KhoaHoc_MXV_T11"
}
```

**Response 200:**
```json
{
  "success": true,
  "lead_id": 2,
  "message": "Đăng ký thành công",
  "redirect_url": "chucmung.html",
  "url": "https://hanghoaphaisinh.net/?utm_source=facebook&utm_medium=paid"
}
```

#### Trường `url` — URL trang khách đăng ký ⭐

`url` là **URL đầy đủ của trang landing mà khách vừa gửi form** (kèm query string UTM).
Đây là trường **quan trọng** để truy ngược lead về đúng landing page / campaign.

| Nguồn | Thứ tự ưu tiên |
|---|---|
| Payload client | `url` → `page_url` → `entry_url` → `landing_url` |
| Fallback server | `Referer` → `Origin` → `X-Forwarded-Proto` + `Host` + path |

Server lưu vào **cột `leads.url`** (VARCHAR 1000) *và* trong `tracking.url` (JSONB).
Cột riêng cho phép đồng bộ sang Google Sheets bằng **field mapping `url`** và xuất CSV dễ dàng.

> `tracker.js` đã gửi kèm `url: window.location.href` — landing page không cần thay đổi gì.

#### Deduplication & đồng bộ Google Sheets (sửa 2026-11-06)

Backend tự khử trùng ở tầng PostgreSQL (`db.SaveLead` → trả về `isNew`):

| Trường hợp | Kết quả |
|---|---|
| Cùng SĐT + cùng landing, trong 24h | Merge tracking vào lead cũ, **không** tạo dòng mới, **không** đẩy thêm dòng xuống Google Sheets |
| Cùng SĐT, trong 10 phút (bất kể landing) | Như trên |
| Cùng SĐT + cùng `session_id` | Như trên |
| Request song song cùng SĐT | Serialize bằng `pg_advisory_xact_lock` ⇒ chỉ 1 lead được tạo |

⇒ Mỗi lead chỉ tạo **1 dòng** trong Google Sheets. Trước đây mỗi request gửi trùng đều
ghi 1 dòng Sheet ⇒ Sheet hiện nhiều dòng giống nhau cùng thời gian.

#### Chọn sheet nhận lead (sửa 2026-11-13 — lead bắn tràn sang nhiều sheet)

Backend chọn sheet nhận lead theo **2 tầng lọc**:

| Tầng | Cấu hình | Hành vi |
|---|---|---|
| 1 | `landing_pages.settings.google_sheet_ids` + `google_sheet_sync_mode` | `selected` ⇒ chỉ gửi vào đúng các sheet được tick · `all` ⇒ mọi sheet khớp · `disabled` ⇒ không gửi |
| 2 | `google_sheet_configs.filter_landing_id` | rỗng ⇒ nhận mọi landing · có giá trị ⇒ chỉ nhận landing đó |

⚠️ **Khi landing đã cấu hình `google_sheet_ids` (mode `selected`), backend chỉ gửi
vào đúng các sheet đó** — kể cả khi sheet đó có `filter_landing_id = ''` (đang là
"nhận tất cả"). Nếu không, lead của landing này sẽ bị gửi sang sheet của landing
khác.

Backend tra cấu hình theo **domain của URL khách vừa gửi form** (`leads.url`) và
gộp **mọi row** của landing trong `landing_pages` (bản gốc `domain=''` + bản theo
domain) — vì `google_sheet_ids` thường chỉ nằm ở bản theo domain. Chi tiết tại
`docs/DATABASE.md` § "Đồng bộ lead sang Google Sheets".

Lưu ý vận hành: nếu cấu hình `form_config.success_redirect` chứa **domain** (VD
`ads.giaodichhanghoatuonglai.com/chucmung`), `<base href="/landing/">` sẽ phân giải thành
`/landing/ads.giaodichhanghoatuonglai.com/chucmung` — vẫn là **trang có form** ⇒ khách bị
quay lại form và bấm đăng ký lần nữa ⇒ lead trùng.

⇒ Cả 3 nơi đều đã tự chuẩn hoá URL này (`normalizeSuccessRedirectValue` /
`normalizeSuccessRedirect`):

| Nơi | Hành vi |
|---|---|
| `GET /api/landing/config` | `redirect_url` + `settings.form_config.success_redirect` đã chuẩn hoá |
| Render HTML landing (`InjectDynamicScriptsWithBase`) | Config inject xuống client đã chuẩn hoá |
| `tracker.js` (client) | Chuẩn hoá lần nữa + chống trỏ về chính trang đang xem |

Nhưng **vẫn nên cấu hình `success_redirect = "chucmung.html"`** trong CRM để dữ liệu
đúng ngay từ đầu.

**Mã lỗi:**

| Code | Lý do |
|---|---|
| `400` | Thiếu trường bắt buộc / SĐT không hợp lệ (phải 10 số, `03x/05x/07x/08x/09x`) |
| `403` | PoW sai, honeypot bị kích, hoặc IP bị rate-limit → ghi vào `spam_quarantine` |
| `429` | Rate limit |

**Quy trình PoW bắt buộc:**
1. `GET /api/auth/challenge?difficulty=3` → `{ challenge_id, salt, difficulty, expires_at, signature }`
2. Client brute-force: tìm `solution` sao cho `SHA256(challenge_id + ":" + salt + ":" + solution)` **bắt đầu bằng `"000"`**.
3. Gửi kèm `challenge_solution` + `challenge_signature` khi POST lead.

### `GET /api/leads` — Admin
Query: `?page=1&limit=50&search=&landing_id=&nguon=&status=&from_date=&to_date=&from_hour=&to_hour=`

Response:
```json
{
  "leads": [ { "id": 2, "thoi_gian": "...", "so_dien_thoai": "...", "trang_thai": "moi", "url": "https://...", "tracking": {...} } ],
  "total": 2,
  "page": 1, "limit": 50
}
```

### `GET /api/leads/export.csv` — Admin
Query giống `/api/leads` + `?landing_id=&nguon=`. Trả CSV **UTF-8 có BOM** (để Excel đọc tiếng Việt đúng).

Cột CSV: `ID, Thời Gian, Trạng Thái CRM, Họ Tên, Số Điện Thoại, Nhà Mạng, Landing ID,
**URL Trang Đăng Ký**, Nguồn Traffic, Ghi Chú Sales, Người Phụ Trách, Cập Nhật Lúc,
Thiết Bị, Tự Động Điền, UTM Campaign, Tracking Raw`.

### `POST /api/leads/update` — Admin
```json
{ "id": 2, "trang_thai": "da_goi", "ghi_chu": "Khách hẹn gọi lại 18h", "nguoi_phu_trach": "An" }
```

### `GET /api/crm/stats` — Admin
Trả toàn bộ KPI: `summary`, `hourly_distribution` (24 khung giờ), `daily_trend`, `status_counts`, `carrier_counts`, `source_counts`, `city_counts`, `region_counts`, `ad_placement_counts`, `isp_counts`, `autofill_ratio`.

---

## 4. Landing Page Management — Admin

| Endpoint | Method | Mô tả |
|---|---|---|
| `/api/landings` | GET | Danh sách landing |
| `/api/landings/get?page_id=X&domain=Y` | GET | Chi tiết 1 landing |
| `/api/landings/save` | POST | Tạo/cập nhật landing (settings + code) |
| `/api/landings/code` | POST | Lưu HTML (Studio Ctrl+S) |
| `/api/landings/save-code` | POST | Alias của `/code` |
| `/api/landings/deploy` | POST | Deploy landing lên domain/path |
| `/api/landings/toggle` | POST | Bật/tắt feature flag |
| `/api/landings/duplicate` | POST | Nhân bản landing (**copy đủ cây file + tách biệt định danh** — xem bên dưới) |
| `/api/landings/identity/report` | GET | Báo cáo định danh landing + cảnh báo nguồn dùng chung |
| `/api/landings/identity/repair` | POST | Sửa chữa landing bị chồng chéo định danh |
| `/api/landings/delete` | POST | Xoá landing |
| `/api/landings/files` | GET | Cây file (File Manager) |
| `/api/landings/file/save` | POST | Lưu 1 file |
| `/api/landings/files/save` | POST | Lưu **nhiều file** trong 1 request (batch) |
| `/api/landings/file/upload` | POST | Upload file (ảnh/video) |
| `/api/landings/files/upload` | POST | Upload **nhiều file** từ máy (alias) |
| `/api/landings/file/raw` | GET | Serve file nhị phân |
| `/api/landings/file/rename` | POST | Đổi tên file |
| `/api/landings/folder/create` | POST | Tạo thư mục |
| `/api/landings/folder/rename` | POST | Đổi tên thư mục |
| `/api/landings/folder/delete` | POST | Xoá thư mục |
| `/api/landings/preview` | GET | Xem trước |

### `POST /api/landings/duplicate` — Admin (nhân bản landing)

```json
{
  "source_domain": "lophoc.hanghoaphaisinh.net",
  "source_page_id": "doanh_chung",
  "new_page_id": "doanh_chung_copy",
  "new_name": "Bản sao của doanh_chung"
}
```

**Response:**

```json
{
  "success": true,
  "message": "Đã nhân bản thành công sang 'doanh_chung_copy' (54 tệp tin)",
  "copied_files": 54,
  "nguon": "doanh_chung_copy"
}
```

**Hành vi (fix 2026-11-15):**

| Hành vi | Mô tả |
|---|---|
| Copy **toàn bộ** cây file | CSS, JS, ảnh, video, font, vendor… — không chỉ `index.html` |
| Ghi vào row gốc `domain = ''` | Bản sao có đủ file ngay ⇒ Code Studio hiển thị preview đúng, kể cả khi chưa Deploy |
| **Không** gán domain của landing nguồn | Bản sao là landing riêng; khi Deploy sẽ `SyncLandingFilesToDomain()` sang domain đích |
| Xoá cấu hình riêng của domain nguồn | `settings.path_prefix`, `google_sheet_ids`, `google_sheet_sync_mode`, `nguon`, `landing_id` không bị copy sang |
| `status = 'draft'` | Bản sao mặc định chưa publish |
| 🚨 **Tách biệt định danh** | `nguon_default` = `new_page_id`; `config.json` (`landing_id`+`nguon`) = `new_page_id`; HTML (`data-landing-id`, `data-nguon`, `APEX_LANDING_CONFIG`) = `new_page_id` |
| 🚨 **Validate `page_id`** | Không rỗng, không chứa khoảng trắng / dấu gạch chéo, ≤ 100 ký tự |

> ⚠️ **Lý do tách biệt định danh:** trước fix này, bản sao giữ nguyên
> `data-landing-id` + `nguon_default` của landing nguồn ⇒ `data-landing-id` là
> nguồn ưu tiên **cao nhất** trong `tracker.js` ⇒ **mọi view + lead của bản sao
> bị ghi về landing nguồn** ⇒ thống kê chồng chéo.
> Chi tiết: `docs/LANDING.md` mục "NHÂN BẢN LANDING PHẢI TÁCH BIỆT ĐỊNH DANH".

### `GET /api/landings/identity/report` — Admin (báo cáo định danh landing)

Trả về định danh của **mọi** landing + cảnh báo khi 2 landing dùng **chung 1 nguồn**.

**Response:**

```json
{
  "success": true,
  "report": {
    "xu_huong_1": {
      "nguon": "xu_huong_1",
      "so_domain": "2",
      "canh_bao": "nguồn 'DTN 1' đang dùng chung với: doanh_chung, xu_huong_1"
    },
    "doanh_chung": {
      "nguon": "DTN 1",
      "so_domain": "2"
    }
  }
}
```

### `POST /api/landings/identity/repair` — Admin (sửa chữa landing chồng chéo)

Chạy `store.RepairLandingIdentity()`:
tách nguồn dùng chung + sửa `config.json` / `data-landing-id` / `data-nguon` /
`APEX_LANDING_CONFIG` / `code_html` về đúng landing. Reload cache sau khi sửa.

> ⚠️ **An toàn:** chỉ sửa dữ liệu ĐỊNH DANH — KHÔNG xoá lead, KHÔNG đụng nội dung,
> KHÔNG đụng Google Sheets. **Idempotent** — chạy lại nhiều lần vẫn an toàn.
> Hàm này cũng tự chạy mỗi lần backend khởi động.

**Response:**

```json
{
  "success": true,
  "message": "Đã sửa chữa 12 bản ghi định danh landing",
  "fixed": 12
}
```


### `POST /api/landings/files/save` — Admin (batch save)

Lưu nhiều file của landing trong **1 request** (thay vì N request riêng lẻ).

**Body:**
```json
{
  "domain": "ads.giaodichhanghoatuonglai.com",
  "page_id": "landing_cu",
  "files": [
    { "file_path": "index.html", "file_type": "html", "content": "<!DOCTYPE html>..." },
    { "file_path": "assets/css/style.css", "file_type": "css", "content": "body {...}" }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "saved": 2,
  "failed": [],
  "message": "Đã lưu 2/2 tệp tin"
}
```

> 💡 Ghi chú: nếu `domain` khác rỗng, mỗi file cũng được ghi thêm vào bản gốc (`domain = ''`)
> để khi Deploy sang domain khác không bị thiếu file.
> File nhị phân (ảnh/video) **không** gửi qua endpoint này — chúng đã lưu sẵn trong DB lúc upload.

### `POST /api/landings/files/upload` — Admin (upload từ máy)

Upload file từ máy lên server vào một thư mục bất kỳ trong cây File Manager.

**Multipart form fields:**

| Field | Bắt buộc | Mô tả |
|---|---|---|
| `page_id` | ✅ | Mã landing |
| `domain` | — | Domain đích (rỗng = bản gốc) |
| `folder` | — | Thư mục đích, vd `assets/img` (rỗng = gốc) |
| `files` | ✅ | File (có thể nhiều file, cùng tên field) |

- Giới hạn 100MB/tệp.
- File nhị phân được lưu dạng `data:<mime>;base64,...` trong `landing_files.content`.
- Cấm các đuôi: `.exe .bat .cmd .sh .ps1 .php .jsp .asp`.
- Tự động lưu cả bản gốc (`domain=''`) nếu `domain` khác rỗng.

### `POST /api/landings/deploy` — Admin

Deploy landing lên domain/path. Body:

| Field | Mô tả |
|---|---|
| `domain` | Domain đích (rỗng = lấy từ route hiện tại) |
| `page_id` | Mã landing |
| `code_html` | Nội dung `index.html` |
| `files` | *(mới)* Mảng file `{file_path, file_type, content}` — **toàn bộ** file trong cây File Manager |

> ⚠️ **Từ 2026-11-04**: `files` cho phép deploy **toàn bộ** file (ảnh, video, css, js...)
> chứ không chỉ `index.html`. Thứ tự trong backend:
> 1. `SyncLandingFilesToDomain("", targetDomain, page_id)` — copy file có sẵn trong DB.
> 2. Ghi đè bằng `files` — nội dung mới nhất.
>
> Response bổ sung field `deployed_files` (số file đã deploy).

> 💡 **Upload nhị phân không đi qua `/files/save`**: file ảnh/video/audio đã lưu sẵn
> trong `landing_files` lúc upload (dạng `data:<mime>;base64,...`), nên frontend
> loại chúng khỏi payload batch (`isBinaryStudioPath()`) để tránh gửi base64
> khổng lồ lên DB. Deploy vẫn giữ được các file này vì
> `SyncLandingFilesToDomain()` copy từ bản gốc sang domain đích.

### `GET /api/landing/config` — **Public**
Trả config của landing mặc định (dùng để inject script & feature flag):
```json
{
  "landing_id": "mxv_080_online",
  "name": "Hội Thảo Giao Dịch Hàng Hóa MXV 080",
  "is_active": false,
  "status": "published",
  "redirect_url": "chucmung.html",
  "settings": { "facebook_pixel_id": "...", "form_mapping": {...} }
}
```

> ⚠️ **Từ 2026-11-04**: endpoint này **không còn** trả các key dùng để ép ghi đè nội dung
> (`hotline`, `zalo_link`, `brand_header`, `tieu_de`, `metric_boxes`, `thoi_gian_chia_se`).
> HTML viết tay trong Code Studio là nguồn sự thật duy nhất — xem `docs/LANDING.md` mục 5.
> Vẫn giữ: `form_mapping` (ánh xạ input → trường lead), `khoa_hoc` (điền trường package),
> `form_config`, `redirect_url`, và toàn bộ `settings` (pixel/SEO/sheets).

### `GET /api/landings/preview` — Admin
Xem trước landing. Query params:

| Param | Mô tả |
|---|---|
| `page_id` | Mã landing (bắt buộc) |
| `domain` | Domain cụ thể — **quan trọng**, ưu tiên landing theo domain thay vì template `domain = ''` |
| `draft` | `1` = lấy bản nháp `index.html` trong `landing_files` |
| `bypass` | `1` = bỏ qua check quyền nội bộ |
| `preview` | `1` = đánh dấu chế độ preview |

---

## 5. Quản lý Tên Miền — Admin

| Endpoint | Method | Mô tả |
|---|---|---|
| `/api/domains` | GET/POST/DELETE | CRUD domain |
| `/api/domains/save` | POST | Thêm/sửa domain |
| `/api/domains/delete` | POST | Xoá domain |
| `/api/domains/check-dns` | GET | Kiểm tra A record & NS |
| `/api/domains/check-ssl` | GET | Kiểm tra trạng thái SSL |
| `/api/domains/deploy` | POST | Gán landing lên path prefix |
| `/api/domains/routes/delete` | POST | Xoá route |

### `POST /api/domains/deploy`
```json
{ "domain_id": 3, "landing_id": "landing_cu", "path_prefix": "/landing", "redirect_url": "" }
```
Ghi `domain_routes` → **bắt buộc** gọi `domainCache.Reload()`.

### `GET /api/internal/check-domain?domain=X` — **Public (Caddy only)**
Điểm thẩm định cho Caddy On-Demand TLS.

| Tình huống | Response |
|---|---|
| Domain đã đăng ký | `200 OK` |
| Domain lạ | `403 Forbidden` |

> ⚠️ **Allow-list phụ thuộc môi trường** (`APEX_ENV` → `cfg.OnDemandAllowList`):
>
> | Môi trường | Domain được trả 200 |
> |---|---|
> | `production` | `crm.hanghoaphaisinh.net`, `hanghoaphaisinh.net`, `ns1/ns2.hanghoaphaisinh.net`, `test.hanghoaphaisinh.net` **+ mọi `*.hanghoaphaisinh.net`** (tương thích ngược với code cũ) |
> | `test` | `test.hanghoaphaisinh.net`, `ns1/ns2.hanghoaphaisinh.net` (**không** dùng suffix matching) |
> | `local` | `localhost`, `127.0.0.1` |
>
> `test.hanghoaphaisinh.net` được production cho phép **cố ý**: Caddy 2.8.4 chỉ có
> MỘT global `on_demand_tls` dùng chung cho mọi site block (hỏi backend
> production) ⇒ thiếu nó thì Caddy không cấp được chứng chỉ cho domain test.
>
> Ngoài ra, domain **đã đăng ký trong CRM** luôn được `200` (tra cứu RAM cache /
> DB) — không phụ thuộc allow-list.
>
> Ghi đè bằng biến `APEX_ON_DEMAND_DOMAINS`.

---

## 6. Analytics — Admin

| Endpoint | Mô tả |
|---|---|
| `POST /api/analytics/track` | **Public.** Thu thập hành vi (session upsert) |
| `GET /api/analytics/overview` | Tổng quan: views, human, bot, CR% |
| `GET /api/analytics/landings` | Hiệu quả từng landing |
| `GET /api/analytics/sources` | Theo nguồn + UTM campaign/content |
| `GET /api/analytics/geo` | Theo tỉnh, vùng miền, ISP |
| `GET /api/analytics/ad-placements` | Chi tiết vị trí quảng cáo |
| `GET /api/analytics/sessions` | Danh sách phiên truy cập |
| `GET /api/analytics/session/{id}` | Chi tiết 1 phiên + timeline |
| `GET /api/analytics/timeseries` | Chuỗi thời gian (giờ/ngày/tuần) — views, human, bot, leads |
| `GET /api/analytics/devices` | Thiết bị chi tiết: brand, model thật, OS, browser, in-app |
| `GET /api/analytics/meta-macros` | Meta macros: campaign/adset/ad/placement + CR% |
| `GET /api/analytics/bots` | Phân tích bot: lý do, score, tỉ lệ |
| `GET /api/analytics/vpn` | Phân tích VPN/Proxy: trạng thái, độ tin cậy, nhà cung cấp |
| `GET /api/analytics/landing-report` | **Báo cáo riêng từng landing page** (toàn bộ KPI + biểu đồ) |

> ⚠️ Tất cả endpoint GET analytics (trừ `track`) đều **bắt buộc** hỗ trợ bộ lọc:
> `landing_id`, `nguon`, `from_date`, `to_date`, `days` (cửa sổ tương đối),
> `from_hour`, `to_hour` (0–23, hỗ trợ **quay vòng qua nửa đêm** ví dụ 22h→06h).
> Trước 2026-11-12 các bộ lọc này bị **bỏ qua** trong SQL → số liệu sai. Xem `docs/DATABASE.md` mục "Bộ lọc analytics".

### 6.1. `GET /api/analytics/landing-report` — báo cáo riêng landing

Query: `?landing_id=doanh_chung&granularity=hour|day|week&from_date=&to_date=&days=&from_hour=&to_hour=`

Response (rút gọn):
```json
{
  "landing_id": "doanh_chung",
  "granularity": "day",
  "summary": {
    "total_views": 1520, "unique_visitors": 1310, "human_visitors": 1180,
    "bot_visitors": 340, "total_leads": 87, "conversion_rate": 5.72,
    "avg_duration_seconds": 74, "avg_scroll_percent": 63,
    "vpn_visitors": 92, "vpn_rate": 6.1, "in_app_visitors": 410, "meta_clicks": 505
  },
  "timeseries": [ { "bucket": "2026-11-12", "views": 210, "human": 180, "bot": 30, "leads": 12 } ],
  "devices": [ { "device_name": "Samsung Galaxy A52", "brand": "Samsung", "model_code": "SM-A525F", "views": 220, "leads": 14, "conversion_rate": 6.4, "avg_duration": 81, "avg_scroll": 71 } ],
  "in_app": [ { "in_app_type": "Facebook In-App", "views": 180, "leads": 9 } ],
  "browsers": [ { "browser": "Chrome Mobile 131", "views": 700 } ],
  "bots": [ { "reason": "no_interaction", "count": 180 } ],
  "vpn": [ { "provider": "Cloudflare WARP", "count": 34 } ],
  "meta_macros": [ { "campaign_name": "Nam_25-45_HN_HCM_DauTu", "adset_name": "...", "ad_name": "...", "placement": "Facebook_Reels", "views": 120, "leads": 8, "conversion_rate": 6.7 } ]
}
```

> `summary` là số liệu **thật** từ `analytics_sessions` (không còn suy ra từ `totalLeads * 8`).

### 6.2. Trung Tâm Báo Cáo — `/api/analytics/report/*` (thiết kế lại 2026-11-13)

Toàn bộ endpoint dưới đây dùng **CHUNG một bộ lọc toàn cục** (`db.ReportFilter`)
⇒ mọi thẻ KPI, biểu đồ và bảng luôn thống nhất với nhau.
Handler: `backend/handlers/report_handler.go` · Store: `backend/db/analytics_report.go`.

| Endpoint | Mô tả | Dùng cho |
|---|---|---|
| `GET /api/analytics/report/summary` | Thẻ KPI + % so sánh kỳ trước | Tab 1 |
| `GET /api/analytics/report/timeseries` | Biểu đồ 1 — combo bar + line (`granularity=hour\|day\|week`) | Tab 1 |
| `GET /api/analytics/report/landings` | Bảng 1 — leaderboard đa landing | Tab 1 |
| `GET /api/analytics/report/meta` | Biểu đồ 2 (nền tảng + placement) & 3 (in-app vs ngoài) | Tab 2 |
| `GET /api/analytics/report/ad-hierarchy` | Bảng 2 — campaign → adset → ad | Tab 2 |

> ⚠️ **Bug 2026-11-15 — lỗi 500 `SQLSTATE 42803` ở `ad-hierarchy`:**
> PostgreSQL resolve `GROUP BY <tên>` theo **TÊN CỘT THẬT** của bảng trước, rồi mới tới
> output alias. Query cũ dùng `SELECT COALESCE(NULLIF(campaign_name,''), NULLIF(utm_campaign,''), 'none') AS campaign_name`
> + `GROUP BY campaign_name` ⇒ PostgreSQL hiểu là group theo cột thật `analytics_sessions.campaign_name`
> ⇒ `utm_campaign` không nằm trong `GROUP BY` ⇒ lỗi 42803 ⇒ tab "Quảng cáo" trả HTTP 500.
>
> ⇒ **Fix:** dùng CTE tính sẵn giá trị hiển thị (`grp_campaign`/`grp_adset`/`grp_ad` — tên KHÔNG
> trùng cột thật) rồi `GROUP BY` trên cột của CTE.
> ⇒ **Quy tắc cho mọi query sau:** nếu `SELECT` có biểu thức `COALESCE` fallback sang cột KHÁC,
> **phải** tính sẵn trong CTE/subquery rồi group trên alias đã "sạch" (không trùng tên cột thật).
> ⇒ Regression test: `backend/db/analytics_report_groupby_test.go`
> (`TestGetReportAdHierarchyNoGroupByError` + `TestAllReportQueriesRunWithoutSQLError` — gọi TẤT CẢ
> hàm báo cáo với bộ lọc rộng nhất để bắt lỗi SQL còn sót).
>
> ⚠️ **Bug 2026-11-15 — lỗi 500 `cannot scan NULL into *int` ở `report/security`:**
> `MAX(bot_score)::int` trả NULL khi không có phiên bot nào ⇒ `Scan` lỗi ⇒ tab "Bảo mật & Bot" trả 500.
> ⇒ **Fix:** `COALESCE(MAX(bot_score), 0)::int` (cả ở `analytics_report.go` lẫn `analytics_ext.go`).
| `GET /api/analytics/report/heatmap` | Biểu đồ 4 — 7 ngày × 24 giờ (`metric=leads\|sessions\|cr`) | Tab 2 |
| `GET /api/analytics/report/behavior` | Biểu đồ 5, 6, 7 + Bảng 3 (top điểm chạm) | Tab 3 |
| `GET /api/analytics/report/form` | Biểu đồ 8 (phễu form) & 9 (nguồn autofill) | Tab 4 |
| `GET /api/analytics/report/devices` | Biểu đồ 10, 11 + Bảng 4 (mã thiết bị & viewport) | Tab 5 |
| `GET /api/analytics/report/security` | Biểu đồ 12 — KPI bảo mật + lý do chặn bot | Tab 6 |
| `GET /api/analytics/report/quarantine` | Bảng 5 — nhật ký vùng cách ly | Tab 6 |
| `POST /api/analytics/report/quarantine/approve` | Duyệt thủ công lead bị chặn nhầm → CRM | Tab 6 |
| `GET /api/analytics/report/journey` | Hành trình khách hàng (hồ sơ + timeline từng giây) | CRM |
| `GET /api/analytics/report/filters` | Danh sách giá trị distinct cho dropdown bộ lọc | Bộ lọc |

#### Bộ lọc toàn cục (dùng chung cho MỌI endpoint trên)

| Tham số | Giá trị | Ý nghĩa |
|---|---|---|
| `from_date` / `to_date` | `YYYY-MM-DD` | Khoảng ngày (múi giờ `Asia/Ho_Chi_Minh`) |
| `days` | int | Cửa sổ tương đối "N ngày gần nhất" (dùng khi không có from/to) |
| `from_hour` / `to_hour` | 0–23 | Khung giờ — **hỗ trợ quay vòng qua nửa đêm** (22h→06h) |
| `landing_ids` | CSV | Multi-select landing (`landing_id` đơn cũng OK) |
| `nguon` / `landing_path` | string | Nhánh vận hành / đường dẫn |
| `ad_account_id` | string | Tài khoản quảng cáo |
| `campaign_name` / `adset_name` / `ad_name` | string | Attribution Meta Ads |
| `site_source_name` | `fb`\|`ig`\|`msg`\|`an` | Nền tảng |
| `utm_placement` | `Facebook_Reels`… | Vị trí hiển thị |
| `visitor_type` | `all`\|`new`\|`returning` | Khách mới (`is_new_visitor`) vs quay lại (`session_count > 1`) |
| `device_type` / `os` | string | Thiết bị & hệ điều hành |
| `is_in_app_browser` | `0`\|`1` | Trình duyệt in-app vs ngoài |
| `in_app_type` | string | Facebook In-App, Zalo In-App, TikTok… |
| `traffic_scope` | `human`\|`all`\|`bot` | **Mặc định `human`** (chỉ người thật) |
| `compare` | `1` | Bật % so sánh kỳ trước trên thẻ KPI |

> ⚠️ `traffic_scope=human` là mặc định **cố ý**: tính CR trên toàn bộ traffic sẽ bị
> lệch vì bot không bao giờ chuyển đổi. Bật `all` khi cần soi traffic thô.
>
> ⚠️ Bảng leaderboard (`/report/landings`) **bỏ qua** bộ lọc `landing_id` (vì cần
> nhóm theo `landing_id`) nhưng **giữ nguyên** mọi bộ lọc còn lại.

#### Ví dụ response

```jsonc
// GET /api/analytics/report/summary?compare=1&traffic_scope=human
{
  "success": true,
  "data": {
    "kpis": [
      { "key": "sessions", "label": "Tổng lượt truy cập", "value": 1520,
        "unit": "phiên", "previous": 1310, "change_percent": 16.03,
        "direction": "up", "hint": "Số session hợp lệ" }
    ],
    "compare": true, "from_date": "2026-10-14", "to_date": "2026-11-12", "days": 0
  }
}
```

```jsonc
// GET /api/analytics/report/journey?phone=0368123456
{
  "success": true,
  "data": {
    "profile": { "visitor_id": "v_...", "session_count": 2, "campaign_name": "...",
                 "device_model_code": "SM-A525F", "visible_seconds": 52,
                 "max_scroll_percent": 75 },
    "sessions": [ /* ... */ ],
    "timeline": [ { "time": 4, "type": "scroll", "target": "hero",
                    "text": "Cuộn tới 25%", "at": "2026-11-12 21:04:11" } ]
  }
}
```

> `journey` nhận `session_id`, `visitor_id` **hoặc** `phone`. Với `phone`, backend tự
> tra `visitor_id` gần nhất trong `analytics_sessions`.

### `POST /api/analytics/track` — payload
```json
{
  "session_id": "sess_...", "visitor_id": "v_...",
  "landing_id": "doanh_chung", "nguon": "doanh_chung",
  "utm_source": "fb", "utm_medium": "paid",
  "utm_campaign": "12022...", "utm_content": "12022...", "utm_term": "12022...",
  "utm_placement": "Facebook_Reels",
  "campaign_name": "Nam_25-45_HN_HCM_DauTu",
  "adset_name": "VN_25-45_HN_HCM",
  "ad_name": "Video_A52",
  "referrer": "", "entry_url": "https://...",
  "city": "Hà Nội", "region": "Miền Bắc", "isp": "Viettel Telecom",
  "ad_platform": "Meta Ads", "ad_placement": "Facebook_Reels",
  "device_type": "mobile", "browser": "Chrome Mobile", "browser_version": "131.0.0.0",
  "browser_engine": "Blink", "os": "Android", "os_version": "13",
  "device_brand": "Samsung", "device_model": "Galaxy A52",
  "device_model_code": "SM-A525F", "device_name": "Samsung Galaxy A52",
  "platform_info": "Linux armv8l",
  "device_memory": 4096, "device_cores": 8, "device_touchpoints": 5,
  "screen_res": "390x844", "viewport_res": "390x844", "screen_color_depth": 24,
  "pixel_ratio": "3", "timezone": "Asia/Ho_Chi_Minh", "language": "vi-VN",
  "connection_type": "4g", "device_state": "online",
  "is_in_app_browser": true, "in_app_type": "Facebook In-App",
  "is_vpn": false, "is_proxy": false, "is_tor": false,
  "vpn_provider": "", "vpn_confidence": 0, "vpn_reason": "",
  "fbclid": "IwAR...", "fbc": "fb.1.1794189884.IwAR...", "fbp": "fb.1.1794189884.123456789",
  "igshid": "", "meta_click_id": "",
  "is_bot": false, "bot_score": 0, "bot_reasons": "",
  "duration_seconds": 66, "max_scroll_percent": 99, "total_clicks": 3,
  "has_interacted_form": true, "converted_lead": false,
  "is_autofill": true, "autofill_source": "url_params",
  "fill_duration_ms": 14000, "validation_passed": true, "form_focus_ms": 9500,
  "events": [ { "time": 12, "type": "scroll", "target": "hero", "text": "..." } ]
}
```

> **Phễu biểu mẫu ở mức PHIÊN** (2026-11-13): `is_autofill`, `autofill_source`,
> `fill_duration_ms`, `validation_passed`, `form_focus_ms` được gửi KÈM trong payload
> analytics (không chỉ trong payload `/api/leads`) ⇒ có thể đo tỷ lệ bỏ ngang form cho
> cả khách KHÔNG submit. Xem `docs/DATABASE.md` mục "Phễu biểu mẫu".

> **Meta CAPI stitching**: `_fbc` được dựng tự động từ `fbclid` theo chuẩn Meta
> (`fb.1.0.<fbclid>`) nếu trình duyệt không gắn sẵn cookie. `utm_source` được chuẩn hoá
> về `fb` / `ig` / `msg` / `an` theo `{{site_source_name}}`.
>
> **Tách landing**: `landing_id` + `nguon` là bắt buộc; `tracker.js` namespace hoá
> cookie/localStorage theo `landing_id` ⇒ không bao giờ lẫn thông số giữa các landing.

> ⚠️ **`converted_lead` KHÔNG được tin từ client** (sửa 2026-11-13).
>
> Client **không** được gửi `converted_lead: true` trong payload (tracker.js đã bỏ
> trường này). Cờ chỉ được backend set `TRUE` khi `lead_id` thực sự tồn tại trong
> bảng `leads` (xem `AssociateLeadToSession`).
>
> **Vì sao**: `isRegistered()` đọc cookie 180 ngày ⇒ một phiên MỚI của khách đã
> đăng ký trước đó cũng bị đánh dấu converted ⇒ đo trên production có 581 session
> converted nhưng bảng `leads` chỉ có 285 lead ⇒ số liệu chồng chéo, CR sai
> (trang "Chúc mừng" hiện CR = 100%).
>
> ⚠️ **`nguon` mặc định = `landing_id`** (sửa 2026-11-13). Trước đây mọi landing
> ghi chung `nguon = "landing_chinh"` ⇒ 4 landing dùng chung 1 nguồn, không lọc
> tách được. Nay mỗi landing có nguồn riêng (tên landing). Giá trị `landing_chinh`
> được backend coi như "rỗng" và tự động thay bằng `landing_id`.

---

## 7. Google Sheets — Admin

| Endpoint | Mô tả |
|---|---|
| `GET /api/integrations/sheets/bot-info` | Email bot service account |
| `POST /api/integrations/sheets/meta` | Đọc metadata (tabs, dims) từ link Sheet |
| `GET /api/integrations/sheets/headers` | Headers của tab |
| `POST /api/integrations/sheets/create-headers` | Tạo dòng tiêu đề |
| `POST /api/integrations/sheets/test` | Gửi dòng test (tên dòng luôn có dạng `Test <3 số ngẫu nhiên>`) |
| `GET /api/integrations/sheets/drive-files` | List file Drive |
| `GET/POST/DELETE /api/integrations/sheets/configs` | CRUD cấu hình |
| `GET /api/integrations/sheets/configs/get` | Chi tiết config |
| `POST /api/integrations/sheets/configs/toggle` | Bật/tắt |
| `GET /api/integrations/sheets/logs` | Lịch sử sync |
| `GET /api/integrations/sheets/defaults` | Danh mục trường + bộ cột chuẩn mặc định (nguồn sự thật duy nhất) |
| `POST /api/integrations/sheets/configs/upgrade-mappings` | **Bổ sung các trường chuẩn MỚI** vào 1 config đã lưu (an toàn — không phá cột cũ) |

---

## 8. Security

| Endpoint | Auth | Mô tả |
|---|---|---|
| `GET /api/auth/challenge?difficulty=3` | Public | Lấy PoW challenge |
| `GET /api/security/stats` | Admin | Thống kê spam bị chặn |
| `GET /api/security/quarantine` | Admin | Danh sách vùng cách ly |

---

## 9. Ma trận ánh xạ trường Google Sheets (2026-11-15)

> ⭐ **NGUỒN SỰ THẬT DUY NHẤT:** `backend/integrations/googlesheets/fields.go`
> (`leadFieldCatalog`).
>
> - `GetStandardDefaultMappings()` → bộ cột chuẩn (sinh từ danh mục).
> - `AvailableLeadFields()` → danh sách trường cho UI (`GET .../sheets/defaults`).
> - `ExtractLeadValue()` (`sync.go`) → cách lấy giá trị cho từng key.
>
> ⇒ Trước đây danh sách bị lặp ở 3 nơi (handler + `sync.go` + `sheets.js`)
> ⇒ dễ lệch. Nay frontend **nạp từ API**, không hardcode.

### 9.0.0. Danh mục trường (~90 trường, 9 nhóm)

| Nhóm (`group`) | Trường chính |
|---|---|
| **Thông tin lead** | `thoi_gian`, `cap_nhat_luc`, `fullName`, `so_dien_thoai`, `carrier`, `landing_id`, `url`, `nguon`, `nguoi_phu_trach`, `ghi_chu`, `lead_id` |
| **CRM Pipeline** | `trang_thai` (mã), `trang_thai_label` (nhãn tiếng Việt), `trang_thai_stage` (giai đoạn), `capi_event` (sự kiện Meta), `capi_has_value` |
| **Địa lý & Nhà mạng** | `city`, `region_name` (tỉnh gốc GeoIP), `region` (vùng kinh tế), `country`, `country_code` (ISO-2), `isp` |
| **Nguồn quảng cáo** | `ad_platform`, `ad_placement`, `ad_source_tag`, `utm_source/medium/campaign/content/term`, `utm_placement`, `campaign_name`, `adset_name`, `ad_name`, `ad_account_id`, `site_source_name` |
| **Định danh lượt nhấp Meta** | `fbclid`, `fbc`, `fbp`, `igshid`, `meta_click_id`, `event_id` |
| **Thiết bị & Trình duyệt** | `device_type`, `device_brand`, `device_name`, `device_model`, `device_model_code`, `browser`, `browser_version`, `browser_engine`, `os`, `os_version`, `screen_res`, `viewport_res`, `screen_color_depth`, `pixel_ratio`, `device_memory`, `device_cores`, `device_touchpoints`, `timezone`, `language`, `platform_info`, `connection_type`, `device_state`, `is_in_app_browser`, `in_app_type` |
| **Hành vi người dùng** | `time_on_page`, `max_scroll`, `max_scroll_percent`, `clicks_count`, `visible_seconds`, `hidden_seconds`, `session_count`, `session_id`, `visitor_id`, `is_new_visitor`, `scroll_depth_25/50/75/90/100`, `has_video_play`, `has_zalo_click`, `has_tel_click`, `referrer`, `referrer_host`, `landing_path`, `entry_url` |
| **Form & Autofill** | `is_autofill`, `autofill_source`, `fill_duration_ms`, `validation_passed`, `form_focus_ms` |
| **VPN / Proxy / Bot** | `is_vpn`, `is_proxy`, `is_tor`, `vpn_provider`, `vpn_confidence`, `vpn_reason`, `is_bot`, `bot_score`, `bot_reasons` |
| **Khác** | `ip_address`, `server_received_at`, `tracking_json`, `custom_text` |

> ⚠️ `trang_thai_label` / `trang_thai_stage` / `capi_event` được suy ra từ
> `models.GetLeadStatus()` / `models.LeadStatusCAPIEvent()` ⇒ tự động khớp
> pipeline 10 trạng thái (không hardcode chuỗi).

### 9.0.1. Bộ cột chuẩn mặc định

`GetStandardDefaultMappings()` sinh cột A, B, C… **theo đúng thứ tự danh mục**
(trước 2026-11-15 chỉ có 45 cột A→AS).

> ⚠️ Cấu hình đã lưu trong DB **KHÔNG tự đổi** ⇒ sheet cũ giữ mapping cũ
> cho tới khi Admin bấm **"Bổ sung Cột"** (UI) hoặc gọi
> `POST /api/integrations/sheets/configs/upgrade-mappings`.

**Cơ chế "Bổ sung Cột" (an toàn):**
1. Giữ nguyên thứ tự + tiêu đề + mapping của các cột **đang có**.
2. Thêm các trường chuẩn **còn thiếu** vào **cuối** (đánh số cột liên tiếp).
3. Ghi tiêu đề của các cột mới vào **đúng ô dòng 1** trên Google Sheet.
4. Lưu lại `field_mappings` trong DB.

⇒ Dữ liệu lead đã ghi **không bị xáo trộn** (cột mới luôn nằm sau cột cũ).

### 9.0.2. Cập nhật trạng thái CRM trên Sheet (2026-11-15)

⚠️ Trước đây Sheet chỉ nhận **1 dòng** lúc lead mới tạo ⇒ cột "Trạng Thái"
kẹt vĩnh viễn ở `moi` dù sales đã đổi trạng thái.

⇒ Nay `POST /api/leads/update` (đổi trạng thái / ghi chú / người phụ trách)
gọi thêm `syncLeadStatusToGoogleSheets()`:

1. Tra `google_sheet_sync_logs` lấy `row_range` của lần ghi **thành công**
   gần nhất (theo từng config) ⇒ `GetLatestSyncRowRanges(leadID)`.
2. `Values.BatchUpdate` ghi đè **CHỈ** các ô trạng thái
   (`trang_thai`, `trang_thai_label`, `trang_thai_stage`, `capi_event`,
   `capi_has_value`, `ghi_chu`, `nguoi_phu_trach`, `cap_nhat_luc`).

| Đặc điểm | Chi tiết |
|---|---|
| Không append | dùng `BatchUpdate` trên đúng dòng ⇒ **không sinh dòng trùng** |
| Không ghi đè dữ liệu tay | chỉ ghi các ô trạng thái; ô có giá trị rỗng ⇒ bỏ qua |
| `ValueInputOption` | `RAW` (mặc định) ⇒ không diễn giải `=` là công thức (chống formula injection) |
| Không chặn CRM | chạy goroutine riêng; lỗi chỉ log |
| Môi trường TEST | bị chặn bởi `IsExternalSyncEnabled()` |

### 9.0.3. Ma trận 15 trường chuẩn (legacy — vẫn hỗ trợ)

| # | Key | Tiêu đề cột | Ví dụ |
|---|---|---|---|
| 1 | `thoi_gian` | Thời Gian Ghi Nhận | `2026-11-03 20:15:30` |
| 2 | `ho_ten` | Họ Và Tên | `Nguyễn Văn Doanh` |
| 3 | `so_dien_thoai` | Số Điện Thoại | `0984386538` |
| 4 | `nha_mang` | Nhà Mạng | `Viettel` |
| 5 | `landing_id` | Mã Landing | `mxv_080_online` |
| 6 ⭐ | `url` | URL Trang Đăng Ký | `https://hanghoaphaisinh.net/?utm_source=facebook&utm_campaign=KhoaHoc_MXV_T11` |
| 7 | `nguon` | Nguồn Chiến Dịch | `facebook_ads` |
| 8 | `vi_tri_qc` | Vị Trí Quảng Cáo | `Facebook_Reels` |
| 9 | `ip` | Địa Chỉ IP | `113.161.72.45` |
| 10 | `tinh_thanh` | Tỉnh / Thành Phố | `Hà Nội` |
| 11 | `thiet_bi` | Loại Thiết Bị | `Mobile (iPhone)` |
| 12 | `trinh_duyet` | Trình Duyệt | `Safari Mobile 18.2` |
| 13 | `utm_campaign` | Chiến Dịch UTM | `KhoaHoc_MXV_T11` |
| 14 | `thoi_gian_xem` | Thời Gian Xem Trang | `45 giây` |
| 15 | `ghi_chu` | Ghi Chú / Nhu Cầu | `Cần tư vấn hợp đồng` |

### 9.0.4. Bản đồ mặc định 45 cột A→AS (2026-11-12 — đã bị thay bằng danh mục 2026-11-15)

`backend/integrations/googlesheets/sync.go` → `defaultColumnMappings()`:

| Cột | Header | Field |
|---|---|---|
| A–O | 15 trường chuẩn (xem bảng trên) | `thoi_gian` … `ghi_chu` |
| P | Meta UTM Source | `utm_source` |
| Q | Meta Placement | `utm_placement` |
| R | Meta Adset ID (utm_content) | `utm_content` |
| S | Meta Ad ID (utm_term) | `utm_term` |
| T | Meta Campaign Name | `campaign_name` |
| U | Meta Adset Name | `adset_name` |
| V | Meta Ad Name | `ad_name` |
| W | Meta Account ID | `ad_account_id` |
| X | Meta Site Source | `site_source_name` |
| Y | Meta Click ID (fbclid) | `fbclid` |
| Z | Meta _fbc | `fbc` |
| AA | Meta _fbp | `fbp` |
| AB | Meta Click ID tổng | `meta_click_id` |
| AC | Hãng Thiết Bị | `device_brand` |
| AD | Tên Thiết Bị | `device_name` |
| AE | Mã Model | `device_model_code` |
| AF | Trình Duyệt | `browser` |
| AG | Phiên Bản Trình Duyệt | `browser_version` |
| AH | Trình Duyệt Trong App | `is_in_app_browser` |
| AI | Loại App | `in_app_type` |
| AJ | RAM (GB) | `device_memory` |
| AK | Số Lõi CPU | `device_cores` |
| AL | Múi Giờ | `timezone` |
| AM | Loại Kết Nối | `connection_type` |
| AN | Trạng Thái Thiết Bị | `device_state` |
| AO | Dùng VPN | `is_vpn` |
| AP | Nhà Cung Cấp VPN | `vpn_provider` |
| AQ | Thời Gian Hiển Thị | `visible_seconds` |
| AR | Thời Gian Ẩn | `hidden_seconds` |
| AS | Số Phiên | `session_count` |

> ⚠️ `igshid` chưa có cột riêng — dùng `meta_click_id` (fallback `fbclid` → `igshid`).

### 9.1. Dòng thử nghiệm (Test Row)

`POST /api/integrations/sheets/test` ghi 1 dòng mẫu lên Google Sheet để kiểm tra kết nối Service
Account + ánh xạ cột. Đặc điểm:

- **Tên dòng luôn có dạng `Test <3 chữ số ngẫu nhiên>`** (vd: `Test 734`, `Test 089`) — sinh bằng
  `crypto/rand` trong `backend/handlers/google_sheet_handler.go` (`randomTestCode()`), để dễ nhận biết
  & xoá sau khi kiểm tra. Nếu client truyền `test_lead.fullName` thì tên sẽ là `Test 734 - <tên>`.
- **Các trường còn lại mô phỏng người dùng thật**: số điện thoại VN 10 số, nhà mạng, IP, tỉnh/thành,
  ISP, thiết bị, trình duyệt, hệ điều hành, viewport, UTM đầy đủ, thời gian xem trang, tỷ lệ cuộn,
  số click, session/visitor id, referrer và URL landing kèm UTM.
- Response trả thêm `test_code` để UI hiển thị mã dòng vừa ghi.

```json
{
  "success": true,
  "message": "Gửi dòng dữ liệu thử nghiệm thành công! (Test 734)",
  "test_code": "734",
  "updated_range": "Sheet1!A100:O100",
  "row_values": ["...", "Test 734", "..."]
}
```

---

## 9.2. Meta Ads / Conversions API (CAPI) — Admin

> Thêm 2026-11-15. Toàn bộ endpoint dưới đây yêu cầu `RequireAdminAuth`.

| Endpoint | Method | Mô tả |
|---|---|---|
| `/api/facebook-ads/configs` | GET | Danh sách cấu hình CAPI (token đã mask) |
| `/api/facebook-ads/config` | GET/POST | Chi tiết / lưu cấu hình theo `landing_id` |
| `/api/facebook-ads/config/delete` | POST | Xoá cấu hình |
| `/api/facebook-ads/config/toggle` | POST | Bật/tắt nhanh CAPI theo `landing_id` (không cần gửi lại config) |
| `/api/facebook-ads/test-event` | POST | Gửi 1 sự kiện test (có `test_event_code`) |
| `/api/facebook-ads/dataset-quality` | GET | Chỉ số Dataset Quality (EMQ, coverage, dedupe…) |
| `/api/facebook-ads/logs` | GET | Lịch sử gửi sự kiện CAPI |
| `/api/facebook-ads/stats` | GET | Thống kê tổng hợp CAPI từ DB |
| `/api/facebook-ads/campaigns` | GET | Báo cáo breakdown theo chiến dịch Meta |
| `/api/facebook-ads/parse-url` | POST | Phân tích URL quảng cáo Meta → thông số campaign/adset/ad |
| `/api/facebook-ads/lead-statuses` | GET | Danh mục 10 trạng thái CRM (dùng để render UI động) |

### 9.2.1. `POST /api/facebook-ads/config`

```json
{
  "landing_id": "mxv_080_online",
  "is_active": true,
  "pixel_id": "1234567890",
  "access_token": "EAAG...",
  "api_version": "v27.0",
  "gateway_enabled": false,
  "gateway_endpoint": "https://apac.gateway.facebook.com",
  "gateway_access_key": "...",
  "send_page_view": true,
  "send_lead": true,
  "send_purchase": true,
  "send_crm_sync": true,
  "only_facebook_traffic": true,
  "course_value": 6000000,
  "currency": "VND",
  "send_user_data": true,
  "hash_email": true, "hash_phone": true, "hash_name": true,
  "hash_address": true, "hash_state": true, "hash_zip": true,
  "hash_country": true, "hash_external_id": true
}
```

- `landing_id = ""` ⇒ cấu hình **mặc định**, áp dụng cho mọi landing chưa có config riêng.
- Token gửi lên có thể là dạng đã mask (`EAAG****abcd`) → backend giữ nguyên token cũ trong DB.
  Muốn đổi token thì gửi giá trị đầy đủ.
- `only_facebook_traffic` (mặc định `TRUE`) ⇒ **chỉ gửi CAPI cho lead đến từ
  Facebook/Instagram**. Lead organic / Google Ads / TikTok / truy cập thẳng bị bỏ qua
  (không có lượt nhấp Meta để so khớp ⇒ gửi lên chỉ làm nhiễu chỉ số quảng cáo).
  Nếu admin **không** gửi field này ⇒ backend giữ nguyên giá trị đang lưu trong DB.
- Ghi DB xong **phải** `Reload()` cache → handler đã gọi sẵn.

#### Ánh xạ trạng thái CRM → sự kiện Meta (2026-11-15)

Khi sales đổi trạng thái lead trong CRM, backend tự bắn sự kiện tương ứng
(`triggered_by = crm_status_change`):

| Trạng thái CRM | Sự kiện Meta | `custom_data.value` |
|---|---|---|
| `moi` | `Lead` | ❌ — dùng lại `event_id` gốc ⇒ dedup với `fbq` |
| `da_goi` | `Contact` | ❌ |
| `da_hoc_buoi_chia_se` | `StartTrial` | ❌ |
| `da_hoc_chuyen_sau` | `Subscribe` | ✅ `course_value` (6.000.000đ) |
| `da_mua_hang` | `Purchase` | ✅ `course_value` (6.000.000đ) |
| `ket_ban_zalo`, `da_hoc_truoc_do`, `khong_quan_tam`, `spam_khong_lien_lac`, `huy` | *(không bắn)* | — |

Nguồn định nghĩa: `backend/models/lead_status.go` (`LeadStatusDefs.CAPIEvent`).

#### Gate traffic Facebook

`DetectFacebookTraffic()` (`backend/integrations/facebook/attribution.go`) kiểm tra
theo thứ tự mạnh → yếu:

1. `fbc` (cookie `_fbc`) — bằng chứng định danh lượt nhấp Meta ★
2. `fbclid` / `meta_click_id` — click ID trong URL
3. `site_source_name` / `utm_source` ∈ {`fb`,`ig`,`msg`,`an`,`facebook`,`instagram`,…}
4. Referrer thuộc domain Meta + `utm_medium` ∈ {`paid`,`cpc`,`ads`}
5. `fbp` (cookie `_fbp`) — bằng chứng yếu nhất (cuối cùng)

Không có bằng chứng nào ⇒ sự kiện bị **skip** (ghi log `facebook_capi_logs.status='skipped'`).

#### Chuẩn băm PII (theo Meta Conversions API)

| Thông số | Chuẩn hoá trước khi băm SHA-256 |
|---|---|
| `em` | trim + lowercase |
| `ph` | **E.164** — số VN 10 số `0912345678` ⇒ `+84912345678` |
| `fn` / `ln` | trim + lowercase + gộp khoảng trắng |
| `ct` | bỏ dấu tiếng Việt + lowercase (`"Hà Nội"` ⇒ `ha noi`) |
| `st` | bỏ dấu + lowercase; **bỏ qua vùng kinh tế** (`"Miền Bắc"` ⇒ *trống*) |
| `zp` | trim + lowercase + gộp khoảng trắng |
| `country` | **ISO-2** lowercase (`"Việt Nam"` ⇒ `vn`) |
| `external_id` | chuẩn hoá như số điện thoại, hash thẳng |

**KHÔNG băm** (Meta yêu cầu gửi rõ): `client_ip_address` · `client_user_agent` ·
`fbc` · `fbp`.

> 📌 Chuẩn E.164 cho `ph` giúp khoá so khớp `ph` có hiệu quả cao hơn trong
> Advanced Matching ⇒ EMQ tốt hơn. Số dạng local `0912345678` bị Meta tính là
> khoá khác với `+84912345678` ⇒ không khớp được với dữ liệu Meta.
>
> 📌 `country` ISO-2 + `ct` bỏ dấu + `st` là tỉnh thật (không phải vùng kinh tế)
> là 3 nguyên nhân chính khiến EMQ chỉ ~5-6/10 (2026-11-15). Sự kiện CRM sync
> còn fallback IP/UA gốc từ `tracking.server_ip` / `tracking.server_user_agent`
> ⇒ có đủ dữ liệu để Meta so khớp.

### 9.2.2. `POST /api/facebook-ads/parse-url`

```json
{ "url": "https://lophoc.hanghoaphaisinh.net/?utm_medium=paid&utm_source=fb&utm_campaign=120253099537400306&utm_content=120253099537410306&utm_term=120253099537420306&campaign_name=Anh+1&adset_name=Anh&ad_name=%E1%BA%A2nh+-+Chung&fbclid=IwZXh0bgNhZW0" }
```

```json
{
  "success": true,
  "data": {
    "utm_source": "fb", "utm_medium": "paid",
    "campaign_id": "120253099537400306",
    "adset_id": "120253099537410306",
    "ad_id": "120253099537420306",
    "placement": "", "utm_id": "",
    "campaign_name": "Anh 1", "adset_name": "Anh", "ad_name": "Ảnh - Chung",
    "fbclid": "IwZXh0bgNhZW0"
  }
}
```

> ⚠️ Ánh xạ **KHÔNG** theo quy ước Google Ads:
> `utm_campaign` = **campaign.id**, `utm_content` = **adset.id**, `utm_term` = **ad.id**.
> `campaign_name` / `adset_name` / `ad_name` là tên hiển thị (chỉ có khi bật
> "Include names in URL" trong Ads Manager).

### 9.2.3. Trigger gửi sự kiện CAPI

| Trigger | Sự kiện Meta | Khi nào |
|---|---|---|
| `lead_submit` | `Lead` | Lead **mới** vừa submit form landing (`isNew = true`) |
| `crm_status_change` | `Subscribe` | Sales đổi trạng thái → `da_hoc_chuyen_sau` (≈6.000.000đ) |
| `crm_status_change` | `Purchase` | Sales đổi trạng thái → `da_mua_hang` (chuyển đổi cuối) |
| `page_view` | `PageView` | Tắt mặc định (bật qua `send_page_view`) |
| `manual_test` | tuỳ chọn | Nút "Gửi test event" trong tab Facebook Ads |

- Mọi trạng thái còn lại (`moi`, `da_goi`, `ket_ban_zalo`, `da_hoc_buoi_chia_se`,
  `da_hoc_truoc_do`, `khong_quan_tam`, `spam_khong_lien_lac`, `huy`) **không** sinh sự kiện Meta
  (không có giá trị tiền ⇒ không làm nhiễu chỉ số EMQ).
- **Deduplication browser ↔ server**: client gửi `event_id` trong payload
  `POST /api/leads`; server dùng đúng `event_id` đó. `tracker.js` cũng gọi
  `fbq('track','Lead',{ content_name: '…' }, { eventID: <cùng id> })` ⇒ Meta chỉ tính 1 lần.

  > 📌 Theo tài liệu Meta ("Xử lý sự kiện Pixel và sự kiện API Chuyển đổi trùng lặp"),
  > `eventID` là **tham số thứ 4** của lệnh gọi `fbq`, KHÔNG phải thuộc tính nằm trong
  > object tham số thứ 3. Đặt sai chỗ ⇒ Meta không đọc được `eventID` ⇒ không gộp ⇒
  > mỗi lần đăng ký bị đếm 2 lần (1 từ Pixel + 1 từ CAPI).
  >
  > ```js
  > ✅ fbq('track', 'Lead', { content_name: '…' }, { eventID: 'EVENT_ID' })
  > ❌ fbq('track', 'Lead', { content_name: '…', eventID: 'EVENT_ID' })
  > ```

- Lỗi CAPI **không bao giờ** làm fail request chính — chỉ ghi log vào `facebook_capi_logs`.

### 9.2.4. `GET /api/facebook-ads/dataset-quality`

Proxy tới Meta Graph API `GET /{version}/dataset_quality`:

```
GET https://graph.facebook.com/v27.0/dataset_quality
    ?dataset_id=<PIXEL_ID>          ← BẮT BUỘC
    &agent_name=apex_crm            ← tuỳ chọn (lọc theo partner_agent)
    &access_token=<TOKEN>
    &fields=web{event_name,event_match_quality{...},acr{...},event_coverage{...},
                dedupe_key_feedback{...},data_freshness{...},event_spend{...},
                event_ad_sets{...},value_diagnostics{...},event_delay{...}}
```

**Response đã chuẩn hoá** (`quality`):

```json
{
  "success": true,
  "pixel_id": "123456",
  "fetched_at": "2026-11-15 14:00:00",
  "quality": {
    "pixel_id": "123456",
    "events": [
      {
        "event_name": "Purchase",
        "event_match_quality": {
          "composite_score": 7.2,
          "match_key_feedback": [
            { "identifier": "email", "coverage": { "percentage": 100 },
              "potential_aly_acr_increase": { "percentage": 58.96 } }
          ],
          "diagnostics": [
            { "name": "…", "description": "…", "solution": "…",
              "percentage": 59.5, "affected_event_count": 18930, "total_event_count": 31830 }
          ]
        },
        "acr": { "percentage": 37.9 },
        "event_potential_aly_acr_increase": { "percentage": 32.9 },
        "event_coverage": { "percentage": 34.1, "goal_percentage": 75 },
        "dedupe_key_feedback": [
          { "dedupe_key": "event_id",
            "browser_events_with_dedupe_key": { "percentage": 100 },
            "server_events_with_dedupe_key": { "percentage": 100 },
            "overall_browser_coverage_from_dedupe_key": { "percentage": 14.8 } }
        ],
        "data_freshness": { "upload_frequency": "real_time" },
        "event_spend": { "amount": 108947.38, "currency_code": "VND" },
        "event_ad_sets": [ { "ad_set_id": "…", "ad_set_name": "…", "objective": "Sales",
          "spend_amount": 20619.35, "spend_currency_code": "USD", "event_use": "Optimization" } ],
        "value_diagnostics": [ { "issue_type": "MISSING_OR_INVALID_VALUE",
          "issue_count": 250, "event_count": 1000, "issue_rate": 0.25 } ],
        "event_delay": { "events_delayed_over_2h": 100, "events_delayed_over_24h": 10,
          "total_events": 1000, "pct_delayed_over_2h": 10, "pct_delayed_over_24h": 1 }
      }
    ],
    "aggregate": {
      "event_count": 4, "avg_emq": 6.35, "avg_coverage": 34.1,
      "total_spend": 108947.38, "spend_currency_code": "VND", "critical_issues": 2
    }
  }
}
```

> 📌 **Lưu ý format Meta v27.0**: `web` là **MẢNG**, mỗi phần tử = 1 `event_name`
> kèm toàn bộ chỉ số riêng. Bản cũ parse `web` như 1 object ⇒ mọi thẻ đều hiển thị "—".

> ⏱️ Meta cần ≥ 20 phút sau khi gửi sự kiện mới có số liệu, và thường trễ vài giờ.
> Endpoint này chỉ được gọi thủ công từ Admin UI (nút "Làm mới"), **không** gọi tự động khi submit lead.

### 9.2.5. `POST /api/facebook-ads/config/toggle`

Bật/tắt nhanh CAPI cho 1 landing mà **không** cần gửi lại toàn bộ cấu hình
(token, pixel, gateway…).

```json
{ "landing_id": "mxv_080_online", "is_active": false }
```

- Landing phải có cấu hình riêng trong DB ⇒ nếu chưa có, trả `404` kèm thông báo
  yêu cầu tạo cấu hình trước.
- **Không** đụng vào cấu hình mặc định (`landing_id = ""`) — đó là fallback áp dụng
  cho mọi landing chưa có cấu hình riêng, tắt nhầm sẽ ảnh hưởng toàn hệ thống.

---

## 10. Quy ước response

**Thành công:**
```json
{ "success": true, "data": {...} }
```

**Lỗi:**
```json
{ "success": false, "error": "Thông báo lỗi cho người dùng" }
```

**Auth:** `401 Unauthorized` · `403 Forbidden` (không đủ quyền hoặc bị chặn) · `429 Too Many Requests`

Tất cả endpoint admin đi qua `CORSMiddleware` → header `Access-Control-Allow-Origin` theo `ALLOW_ORIGINS` (mặc định `*`).
