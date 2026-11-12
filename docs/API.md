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
| **Admin** | Tất cả `/api/admin/*`, `/api/leads` (GET), `/api/crm/*`, `/api/analytics/*` (trừ track), `/api/domains/*`, `/api/landings/*`, `/api/integrations/sheets/*` | `RequireAdminAuth` |

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
| `/api/landings/duplicate` | POST | Nhân bản landing |
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
  "events": [ { "time": 12, "type": "scroll", "target": "hero", "text": "..." } ]
}
```

> **Meta CAPI stitching**: `_fbc` được dựng tự động từ `fbclid` theo chuẩn Meta
> (`fb.1.0.<fbclid>`) nếu trình duyệt không gắn sẵn cookie. `utm_source` được chuẩn hoá
> về `fb` / `ig` / `msg` / `an` theo `{{site_source_name}}`.
>
> **Tách landing**: `landing_id` + `nguon` là bắt buộc; `tracker.js` namespace hoá
> cookie/localStorage theo `landing_id` ⇒ không bao giờ lẫn thông số giữa các landing.

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
| `GET /api/integrations/sheets/defaults` | 14 mapping mặc định |

---

## 8. Security

| Endpoint | Auth | Mô tả |
|---|---|---|
| `GET /api/auth/challenge?difficulty=3` | Public | Lấy PoW challenge |
| `GET /api/security/stats` | Admin | Thống kê spam bị chặn |
| `GET /api/security/quarantine` | Admin | Danh sách vùng cách ly |

---

## 9. Ma trận ánh xạ 15 trường Google Sheets

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

**Ngoài 15 trường chuẩn** còn map được: `fill_duration_ms`, `max_scroll`, `utm_source`,
`utm_medium`, `utm_content`, `utm_term`, `session_id`, `referrer`, `tracking_json`, `custom_text`.

Danh sách đầy đủ trả về từ `GET /api/integrations/sheets/defaults` (`available_fields`).

### 9.0. Bản đồ mặc định mở rộng (2026-11-12) — 45 cột A→AS

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
