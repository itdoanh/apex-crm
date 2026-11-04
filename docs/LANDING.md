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
