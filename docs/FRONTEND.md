# Frontend — CRM Admin & Ladi Studio

**Không có build step.** Toàn bộ frontend là HTML/CSS/JS thuần, load Tailwind + Alpine + Chart.js từ CDN. Sửa file → refresh trình duyệt là thấy.

---

## 1. Cấu trúc

```
frontend/
├── admin/
│   ├── index.html          Master SPA — chứa TẤT CẢ panel
│   ├── admin.html          Alias của index.html
│   ├── studio.html         Ladi Studio — visual drag & drop builder
│   ├── config.js           API base URL, target server IP
│   ├── ladihtml.sdk.min.js SDK LadiPage (runtime cho Studio)
│   ├── css/
│   │   ├── admin.css       Dark glassmorphism tài chính
│   │   ├── base.css        Reset + biến CSS chung
│   │   ├── crm.css         Bảng, Kanban, drawer
│   │   ├── layout.css      Grid, sidebar, header
│   │   └── studio.css      Canvas, gizmo, layers, inspector
│   └── js/
│       ├── app.js          Điều phối chính, tab switching
│       ├── auth.js         Login, session, đổi mật khẩu
│       ├── crm.js          Kanban 7 bước, Customer 360° drawer
│       ├── analytics.js    Chart.js — heatmap, funnel, geo, ad placement
│       ├── sheets.js       Google Sheets config + mapping matrix
│       ├── domains.js      DNS, domain, deploy route
│       ├── landings.js     CRUD landing, feature flags
│       ├── security.js     Spam quarantine, security stats
│       ├── tracker.js      Bản mirror (dùng cho preview landing)
│       └── studio/
│           ├── gizmo.js            Khung 8 chốt kéo giãn
│           ├── layers.js           Cây layer Z-Index
│           ├── history.js          Undo/Redo 50 bước
│           ├── context-menu.js     Menu chuột phải 20 hành động
│           ├── inspector.js        Panel thuộc tính 6 tab
│           ├── ast-mapper.js       Ánh xạ DOM ↔ AST
│           └── file-manager.js     Cây file landing
├── landing/
│   ├── index.html         Landing entry (alias của ladi.html)
│   ├── ladi.html          Landing chính (LadiPage format)
│   ├── chucmung.html      Trang cảm ơn
│   ├── tracker.js         Thu thập data + PoW solver
│   ├── config.js          Config client cho landing
│   ├── config.json        Config per-landing
│   ├── ladihtml.sdk.min.js
│   └── assets/            Ảnh (mỗi ảnh có .png + .webp)
```

> 🚨 **`backend/static/` là MIRROR của `frontend/admin/`.**
> Sửa UI phải sửa ở `frontend/admin/` rồi copy sang `backend/static/`.
> Không sửa trực tiếp trong `backend/static/` — sẽ bị mất khi đồng bộ.

---

## 2. `frontend/admin/index.html` — Master SPA

Một file HTML duy nhất chứa toàn bộ panel. Các panel được bật/tắt bằng Alpine.js `x-show`.

### Panel chính

| Panel | Nội dung |
|---|---|
| Quản Trị CRM | Bảng dữ liệu + Kanban 7 bước + Customer 360° drawer |
| Biểu Đồ & Giờ Vàng | Heatmap khung giờ, funnel, geo, ad placement, traffic source |
| Hiệu Quả Landing | So sánh CR% giữa các landing |
| Bảo Mật & Bot | Spam quarantine + security stats |
| Đồng Bộ Google Sheets | Config, mapping matrix, sync logs |

> Trong modal "Thêm kết nối Sheet" → phần **3. Kiểm Tra & Gửi Dòng Thử Nghiệm (Test Row)**:
> nút "Gửi Dòng Thử Nghiệm Ngay" gọi `POST /api/integrations/sheets/test`. Dòng ghi ra luôn có tên
> dạng `Test <3 số ngẫu nhiên>` (vd: `Test 734`) với dữ liệu mô phỏng người dùng thật (SĐT VN,
> nhà mạng, IP, tỉnh/thành, ISP, thiết bị, trình duyệt, UTM, URL landing...). UI hiển thị mã dòng
> để anh dễ tìm & xoá sau khi kiểm tra. Xem `docs/API.md` mục 9.1.
| Quản Lý Landing | CRUD + Code Studio + Live Preview |
| Quản Lý Tên Miền | DNS check, SSL check, deploy route |
| Nhật Ký & Phiên | Active sessions + audit logs |

### Panel chuyển động
- Sidebar trái — icon + label, active state
- Header — tên user, nút đổi mật khẩu, đăng xuất
- Auto-refresh: dropdown `10s / 30s / 60s / Tắt` + nút refresh thủ công
- Filter bar: search, landing, nguồn, trạng thái, nhà mạng, ngày, khung giờ

### Cấu hình API
`frontend/admin/config.js`:
```javascript
window.APEX_ADMIN_CONFIG = {
  API_BASE_URL: '',        // rỗng = same-origin ( qua Caddy )
  BASE_PATH: '/ads',
  TARGET_SERVER_IP: '103.124.95.232'
};
```

---

## 3. Ladi Studio (`frontend/admin/studio.html`)

Visual drag & drop builder cho landing page.

### Kiến trúc

```
studio.html
  ├── iframe preview  ← render landing HTML thật
  ├── Canvas controls (zoom, pan, hand mode)
  ├── Layers panel   ← cây layer, Z-Index, ẩn/khoá
  ├── Inspector      ← 6 tab thuộc tính
  ├── Content tab    ← sửa nhanh text/ảnh/CTA
  ├── CSS tab        ← CSS 2 chiều + effect library
  └── Code tab       ← editor HTML/JS/CSS + search-replace
```

### Tính năng chính

| Tính năng | Chi tiết |
|---|---|
| Gizmo 8 chốt | Kéo giãn chốt từ 4 góc + 4 cạnh, khóa tỷ lệ khung hình |
| History 50 bước | Undo `Ctrl+Z` / Redo `Ctrl+Y`, dropdown lịch sử |
| Layers | 7 nhóm: Text, Image, Button, Form, Section, Box, Icon |
| Context menu | 20 hành động, xuyên thấu iframe, chống tràn mép màn hình |
| Form scanner | Quét DOM tìm input → gợi ý mapping vào CSDL |
| Hand mode | Nhấn giữ `Space` để pan canvas |
| AST mapper | Đồng bộ DOM ↔ code, giữ `data-ve-id` xuyên suốt |

### Keyboard shortcuts

| Phím | Hành động |
|---|---|
| `Ctrl+Z` | Undo |
| `Ctrl+Y` | Redo |
| `Ctrl+S` | Lưu code (POST `/api/landings/code`) |
| `Space` (giữ) | Hand mode / pan |
| `Delete` | Xoá element đang chọn |
| `Esc` | Bỏ chọn |

### Bảo toàn `data-ve-id`

`data-ve-id` là khoá định danh element giữa DOM và AST. **Phải duy trì xuyên suốt** — mất nó thì inspector/layers/code sync mất liên kết.

---

## 4. Landing page (`frontend/landing/`)

### `config.json` — config per-landing

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

### Binding trong HTML
```html
<span data-config="thoi_gian_chia_se.hien_thi">20:00 ngày 03/11/2026</span>
<span data-config="thoi_gian_chia_se.ngay">03/11/2026</span>
<span data-config="khoa_hoc.da_dang_ky">Đã có 79 suất đăng ký</span>
```
`tracker.js` đọc `config.json` → tự động điền vào mọi element có `data-config`.

### Countdown
Đếm ngược tới `target_date` (ISO 8601).

---

## 5. `tracker.js` — thu thập dữ liệu

### Trách nhiệm
1. Đọc `config.json` → gắn `landing_id` + `nguon`.
2. Gửi `POST /api/analytics/track` (heartbeat) — thu thập scroll, click, mouse.
3. `GET /api/auth/challenge` → giải PoW.
4. Submit form → `POST /api/leads` kèm tracking + PoW solution.
5. Set cookie 180 ngày → redirect `chucmung.html` nếu đã đăng ký.

### Data thu thập

| Nhóm | Fields |
|---|---|
| Form | `fullName`, `phone`, `email`, `note`, `package` |
| Hành vi | `time_on_page`, `max_scroll`, `clicks_count`, `fill_duration_ms` |
| Autofill | `is_autofill`, `autofill_source` |
| UTM | `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term` |
| URL | `url` (URL đầy đủ trang đăng ký kèm query string), `entry_url`, `referrer` |
| Fingerprint | `visitor_id`, `session_id`, `device_type`, `browser`, `os`, `screen_res` |
| PoW | `challenge_id`, `challenge_salt`, `challenge_difficulty`, `challenge_signature`, `challenge_solution` |

> ⭐ **`url` là trường quan trọng** — server lưu vào cột `leads.url` và đồng bộ Google Sheets
> bằng field mapping `url`. Xem `docs/API.md` mục 3 & `docs/DATABASE.md` mục 2.

### Bất biến
- **Không** được gỡ field honeypot (`website`, `fax_company_prefix`, `user_email_verify`, `company_website_url`) khỏi DOM.
- **Không** được phá autofill-safe — xem `docs/SECURITY.md` mục 4.
- **Không** được thêm logic sửa DOM từ cấu hình CSDL (hotline, zalo, tiêu đề hero, metric box...).
  HTML viết tay trong Code Studio là **nguồn sự thật duy nhất**. Xem `docs/LANDING.md` mục 5.
- **Mọi** module mới có side-effect lên DOM phải guard bằng `if (isAnyPreview()) return;`
  để Code Studio preview không bị nhiễu.

### Code Studio preview — cách ly

`tracker.js` có 3 helper để nhận biết đang chạy trong preview/Studio:

| Helper | Ý nghĩa |
|---|---|
| `isStudioBuilder()` | `window.__APEX_STUDIO__` / `__APEX_BUILDER_ACTIVE__` / `__APEX_FREEZE_MODE__` |
| `isAnyPreview()` | Studio builder **hoặc** `__APEX_PREVIEW__` / `IS_PREVIEW` **hoặc** đang nằm trong iframe |

Khi preview:
- Không nạp `config.json` từ network.
- Không chạy `applyConfigToDOM()` (chỉ set `document.title` + Pixel ngoài preview).
- Không chặn form (`initAutoFormInterceptor` return sớm).
- Không gửi analytics (`sendAnalyticsPing` return sớm).

Thêm vào `ast-mapper.getHelperScript()` (inject vào preview iframe):
`__APEX_STUDIO_LANDING_ID__`, `__APEX_STUDIO_DOMAIN__`, `__APEX_STUDIO_LANDING_NAME__` —
dùng để ghi đè `window.APEX_LANDING_CONFIG` của template gốc.

---

## 6. Quy trình sửa UI (quan trọng)

```
1. Sửa file trong frontend/admin/  (nguồn chuẩn)
2. Copy sang backend/static/        (mirror)
   Copy-Item frontend\admin\js\crm.js backend\static\js\crm.js
3. Nếu đang chạy local: refresh trình duyệt (không cần build)
4. Nếu production: commit + push + git pull + restart task
```

### Kiểm tra mirror có đồng bộ
```powershell
git diff --stat frontend/admin backend/static
```

---

## 7. Test E2E

```bash
# Node (Playwright)
node backend/tests/e2e_domain_management_test.js
node backend/tests/e2e_ad_geo_test.js

# Python
python backend/tests/test_full_suite.py
python backend/tests/test_visual_web_builder.py
```

Test cần server đang chạy + admin credentials:
```json
{ "username": "admin", "password": "ApexAdmin@2026!" }
```

---

## 8. Lưu ý về encoding

Một số file cũ trong repo lưu UTF-8 nhưng hiển thị mojibake khi đọc bằng `cmd /c type` (console codepage). Đây là **vấn đề hiển thị console**, không phải lỗi file. Dùng `Get-Content -Encoding UTF8` trong PowerShell hoặc đọc bằng editor.

File server copy từ git giữ đúng encoding UTF-8 — kiểm tra bằng:
```powershell
[System.IO.File]::ReadAllText("C:\apex\backend\README.md", [System.Text.Encoding]::UTF8).Substring(0,100)
```
