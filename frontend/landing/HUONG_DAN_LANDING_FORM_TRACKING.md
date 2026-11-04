# HƯỚNG DẪN QUY CHUẨN FORM INPUT, TRACKING & TÍCH HỢP LANDING PAGE
### HỆ THỐNG QUẢN TRỊ VÀ ĐỒNG BỘ LEADS APEX FINTECH (MXV 080)

---

## I. TỔNG QUAN KIẾN TRÚC & NGUYÊN TẮC HOẠT ĐỘNG

Tài liệu này quy chuẩn toàn diện về cách thức xây dựng, quản lý mã nguồn, cấu hình Form nhập liệu, cơ chế Tracking hành vi người dùng, và tích hợp tự động cho mọi Landing Page trong hệ sinh thái APEX Fintech.

```
┌────────────────────────────────────────────────────────────────────────┐
│                        LANDING PAGE FRONTEND                           │
│  (Mã HTML tự code, Xuất từ Ladipage, Elementor, Tailwind, Web tĩnh)    │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    │ (Tự động quét Form & Inject Interceptor)
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                     UNIFIED CLIENT TRACKER.JS                          │
│  • Bắt sự kiện form submit (Mọi thẻ <form> trên trang)                 │
│  • Thu thập UTM Tags, Referrer, Fingerprint Visitor ID, Session ID     │
│  • Đo lường: Time on page, Scroll depth, Clicks, Device, Mạng          │
│  • Client-side Anti-Spam: Honeypot trap, Timing check, Phone Validator │
│  • Tự động chuyển hướng trang Chúc Mừng & Lưu Cookie 180 ngày          │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
               ┌────────────────────┴────────────────────┐
               │ AJAX JSON Payload                       │ AJAX JSON Payload
               ▼                                         ▼
   POST /api/leads                          POST /api/analytics/track
┌────────────────────────────────────────────────────────────────────────┐
│                        APEX BACKEND SERVER (GO)                        │
│  • Rate Limiting đa tầng (Token Bucket theo IP + Subnet + Fingerprint) │
│  • Anti-Bot Heuristics & Honeypot Detector                             │
│  • Xác thực số điện thoại 10 số nhà mạng Việt Nam                      │
│  • In-Memory RAM Cache định tuyến đa tên miền                          │
└───────────────────┬────────────────────────────────────┬───────────────┘
                    │                                    │
                    ▼                                    ▼
       ┌─────────────────────────┐          ┌───────────────────────────┐
       │   CSDL POSTGRESQL       │          │   GOOGLE SHEETS ENGINE    │
       │   • Bảng leads          │          │   • Đồng bộ real-time     │
       │   • Bảng tracking_events│          │   • Đa bảng tính/đa tab   │
       │   • Bảng landing_pages  │          │   • Tự động điền cột      │
       └─────────────────────────┘          └───────────────────────────┘
```

---

## II. QUY CHUẨN CÁC TRƯỜNG NHẬP LIỆU FORM (FORM FIELDS SPECIFICATION)

Để đảm bảo tỷ lệ chuyển đổi cao nhất và khả năng tương thích 100% với hệ thống Backend, các form trên Landing Page cần tuân thủ quy chuẩn sau:

### 1. Bảng đặc tả các trường dữ liệu Lead

| Tên trường (Key) | Kiểu dữ liệu | Bắt buộc | Gợi ý `name` / `id` | Mô tả & Quy tắc Validation |
| :--- | :--- | :---: | :--- | :--- |
| **`fullname`** | String (2-100 ký tự) | **Có** | `fullname`, `hoten`, `ho_ten`, `name` | Họ và tên khách hàng. Tối thiểu 2 ký tự, tự động chuẩn hóa viết hoa chữ cái đầu. |
| **`phone`** | String (10 chữ số) | **Có** | `phone`, `sdt`, `so_dien_thoai`, `tel` | **Số điện thoại chuẩn Việt Nam**. 10 chữ số, bắt đầu bằng các đầu số hợp lệ: `03`, `05`, `07`, `08`, `09`. Tự động loại bỏ dấu cách, gạch nối, dấu chấm, tiền tố `+84` chuyển về `0`. |
| **`email`** | String (Email RFC) | Tùy chọn | `email`, `mail` | Địa chỉ email của khách hàng. Phải đúng định dạng `user@domain.ext`. |
| **`note`** | String (Tối đa 500 ký tự) | Tùy chọn | `note`, `ghi_chu`, `message`, `nhu_cau` | Ghi chú nhu cầu tư vấn (Ví dụ: "Muốn học mở tài khoản phái sinh", "Cần tư vấn vốn 50tr"). |
| **`package`** | String | Tùy chọn | `package`, `khoa_hoc`, `goi_dich_vu` | Gói đăng ký hoặc tên hội thảo (Ví dụ: `Giải Mã Đầu Tư Hàng Hóa`). Thường là input ẩn (`type="hidden"`) hoặc radio/select. |
| **`source`** | String | Tùy chọn | `source`, `nguon` | Nguồn chiến dịch quảng cáo (Ví dụ: `fb_ads`, `tiktok_ads`, `zalo_oa`, `google_search`). Nếu để trống, hệ thống tự lấy `nguon_default` của Landing Page. |
| **`landing_id`** | String | Tùy chọn | `landing_id`, `page_id` | Mã định danh của Landing Page trong CSDL (Ví dụ: `mxv_080_online`). |

---

### 2. Các trường bảo mật & chống Spam tự động (Hidden Security Fields)

Các trường này được script `tracker.js` tự động tạo và nhúng ngầm vào form nếu form chưa có:

* **`_hp_check` (Honeypot Trap)**:
  * Thẻ input văn bản ẩn hoàn toàn bằng CSS đối với người dùng thật (`opacity: 0; position: absolute; left: -9999px;`).
  * Người dùng thật sẽ không thấy và không điền.
  * Spambot hoặc auto-fill bot quét form sẽ điền trường này -> Server phát hiện có dữ liệu trong `_hp_check` sẽ **hủy ngay lập tức** mà không tạo lead.
* **`_client_ts` (Form Timing Check)**:
  * Timestamp thời điểm khách bắt đầu mở trang hoặc focus vào form.
  * Nếu thời gian từ lúc mở form đến lúc submit nhỏ hơn 1.5 giây -> Server nhận diện hành vi gửi tự động của máy và chặn spam.
* **`_challenge_token` (Proof-of-Work Token)**:
  * Chuỗi hash sinh ngẫu nhiên kèm bài toán tính toán nhẹ phía client JS để xác minh trình duyệt thật.

---

### 3. Ví dụ mã HTML Form chuẩn tối ưu

```html
<form id="leadForm" class="apex-lead-form" method="POST" action="/api/leads">
    <!-- Trường Họ và tên -->
    <div class="form-group mb-3">
        <label for="fullname" class="block text-sm font-semibold text-slate-800 mb-1">
            Họ và tên <span class="text-rose-500">*</span>
        </label>
        <input type="text" id="fullname" name="fullname" required
               placeholder="Nguyễn Văn A" autocomplete="name"
               class="w-full px-4 py-3 rounded-xl border border-slate-300 focus:ring-2 focus:ring-emerald-500">
    </div>

    <!-- Trường Số điện thoại -->
    <div class="form-group mb-3">
        <label for="phone" class="block text-sm font-semibold text-slate-800 mb-1">
            Số điện thoại (Zalo) <span class="text-rose-500">*</span>
        </label>
        <input type="tel" id="phone" name="phone" required maxlength="12"
               placeholder="0984 386 538" autocomplete="tel"
               class="w-full px-4 py-3 rounded-xl border border-slate-300 focus:ring-2 focus:ring-emerald-500">
    </div>

    <!-- Trường Email (tùy chọn) -->
    <div class="form-group mb-3">
        <label for="email" class="block text-sm font-semibold text-slate-800 mb-1">Email</label>
        <input type="email" id="email" name="email"
               placeholder="email@example.com" autocomplete="email"
               class="w-full px-4 py-3 rounded-xl border border-slate-300 focus:ring-2 focus:ring-emerald-500">
    </div>

    <!-- Honeypot chống Bot Spam (Ẩn hoàn toàn) -->
    <div style="position: absolute; left: -9999px; opacity: 0; pointer-events: none;" aria-hidden="true">
        <input type="text" name="_hp_check" tabindex="-1" autocomplete="off">
    </div>

    <!-- Nút Submit -->
    <button type="submit" id="btnSubmitLead" class="w-full py-4 rounded-xl bg-orange-600 text-white font-bold hover:bg-orange-700">
        ĐĂNG KÝ THAM GIA NGAY
    </button>
</form>
```

---

## III. QUY CHUẨN TRACKING HÀNH VI KHÁCH HÀNG (BEHAVIORAL TRACKING)

Toàn bộ quá trình theo dõi được thực thi ngầm bởi `tracker.js`, không gây ảnh hưởng đến tốc độ tải trang (PageSpeed > 95/100).

### 1. Danh mục tham số Tracking tự động

| Tham số Tracking | Ý nghĩa | Phương thức thu thập |
| :--- | :--- | :--- |
| **`visitor_id`** | Mã định danh khách truy cập duy nhất | Sinh qua Fingerprint UUID, lưu vĩnh viễn vào `localStorage` và `Cookie` 180 ngày. |
| **`session_id`** | Mã định danh phiên truy cập hiện tại | Sinh ngẫu nhiên mỗi lần mở tab hoặc sau 30 phút không hoạt động. |
| **`utm_source`** | Nguồn truy cập (Facebook, Google, TikTok, ...) | Bóc tách từ Query Parameter trên URL. |
| **`utm_medium`** | Hình thức quảng cáo (cpc, banner, reels, email) | Bóc tách từ Query Parameter trên URL. |
| **`utm_campaign`** | Tên chiến dịch quảng cáo | Bóc tách từ Query Parameter trên URL. |
| **`utm_content`** | Mẫu nội dung / video / hình ảnh A/B test | Bóc tách từ Query Parameter trên URL. |
| **`utm_term`** | Từ khóa tìm kiếm | Bóc tách từ Query Parameter trên URL. |
| **`referrer`** | Đường dẫn trang trước chuyển tiếp sang | `document.referrer` |
| **`url`** | **URL đầy đủ của trang khách đang đăng ký** (kèm query string UTM) — trường quan trọng để truy ngược lead về đúng landing page | `window.location.href` |
| **`time_on_page`** | Tổng thời gian khách đã ở lại trên trang (giây) | `performance.now()` hoặc bộ đếm thời gian thực. |
| **`scroll_depth`** | Độ sâu cuộn trang tối đa (25%, 50%, 75%, 100%) | Lắng nghe sự kiện `window.addEventListener('scroll')` throttling. |
| **`clicks_count`** | Tổng số lượt click/tương tác trên trang | Lắng nghe sự kiện `document.addEventListener('click')`. |
| **`device_type`** | Phân loại thiết bị (`mobile`, `tablet`, `desktop`) | Phân tích qua User-Agent và chiều rộng màn hình `window.innerWidth`. |
| **`screen_resolution`** | Độ phân giải màn hình (Ví dụ: `390x844`) | `window.screen.width + 'x' + window.screen.height`. |

---

### 2. Cơ chế Cookie 180 Ngày & Điều Hướng Trang Chúc Mừng

Để tối ưu hóa chi phí quảng cáo và trải nghiệm người dùng trong ngành tài chính (khách chỉ cần đăng ký 1 lần):
1. **Sau khi gửi Form thành công**:
   * Hệ thống ghi Cookie: `apex_submitted_180d=true; Max-Age=15552000; Path=/; SameSite=Lax` (180 ngày = 6 tháng).
   * Đồng thời lưu vào `localStorage.setItem('apex_lead_submitted', 'true')`.
   * Tự động điều hướng ngay sang trang `chucmung.html`.
2. **Khi khách truy cập lại bất kỳ Landing Page nào của hệ thống**:
   * Ngay khi trang HTML bắt đầu nạp, script kiểm tra:
     ```javascript
     if (document.cookie.includes('apex_submitted_180d=true') || localStorage.getItem('apex_lead_submitted') === 'true') {
         window.location.replace('/chucmung.html');
     }
     ```
   * Trình duyệt tự động chuyển hướng tức thì sang `chucmung.html`, không hiển thị lại form để tránh gửi trùng lặp.

---

## IV. ĐẶC TẢ API GIAO TIẾP VỚI BACKEND SERVER

### 1. API Tiếp Nhận Lead: `POST /api/leads`

* **Endpoint**: `POST http://localhost:8080/api/leads` (hoặc domain production `https://crm.hanghoaphaisinh.net/api/leads`)
* **Headers**: `Content-Type: application/json; charset=utf-8`

#### Request Payload JSON Mẫu:
```json
{
  "fullname": "Nguyễn Hoàng Nam",
  "phone": "0984386538",
  "email": "nam.nguyen@gmail.com",
  "note": "Cần mở tài khoản giao dịch dầu thô và cà phê",
  "package": "Giải Mã Đầu Tư Hàng Hóa",
  "nguon": "fb_ads_chien_dich_080",
  "landing_id": "mxv_080_online",
  "domain": "hanghoaphaisinh.net",
  "visitor_id": "vis_8a7f9c2d-3e4b-4a5c-8d1e-2f3a4b5c6d7e",
  "session_id": "ses_1a2b3c4d",
  "time_on_page": 48,
  "scroll_depth": 90,
  "utm_source": "facebook",
  "utm_medium": "cpc",
  "utm_campaign": "dau_tu_hang_hoa_t11",
  "utm_content": "video_chuyen_gia_mxv",
  "utm_term": "phai_sinh_hang_hoa",
  "referrer": "https://m.facebook.com/",
  "url": "https://hanghoaphaisinh.net/?utm_source=facebook&utm_medium=cpc&utm_campaign=dau_tu_hang_hoa_t11",
  "_hp_check": "",
  "_client_ts": 1730554800000
}
```

> ⭐ Trường **`url`** là URL trang đăng ký. Backend lưu vào cột `leads.url` và đồng bộ
> sang Google Sheets bằng field mapping `url`. Nếu thiếu, server fallback theo
> `Referer` → `Origin` → `Host`.

#### Response Thành Công (`200 OK`):
```json
{
  "success": true,
  "lead_id": 128,
  "message": "Đăng ký thành công! Chuyên viên APEX sẽ liên hệ sớm nhất.",
  "redirect_url": "/chucmung.html"
}
```

#### Response Thất Bại (`400 Bad Request` hoặc `429 Too Many Requests`):
```json
{
  "success": false,
  "error": "Số điện thoại không hợp lệ. Vui lòng nhập đúng 10 số di động Việt Nam."
}
```

---

### 2. API Tracking Hành Vi: `POST /api/analytics/track`

* **Endpoint**: `POST /api/analytics/track`
* **Mục đích**: Ghi nhận các mốc hành vi (Heatmap/Funnel Analysis) phục vụ trang Báo cáo Thống kê.

#### Request Payload Mẫu:
```json
{
  "event": "scroll_75",
  "landing_id": "mxv_080_online",
  "domain": "hanghoaphaisinh.net",
  "visitor_id": "vis_8a7f9c2d...",
  "session_id": "ses_1a2b3c4d",
  "time_spent": 35,
  "scroll_depth": 75,
  "device": "mobile",
  "screen": "390x844"
}
```

---

## V. HỆ THỐNG QUÉT FORM TỰ ĐỘNG & ÁNH XẠ DỮ LIỆU (FORM AUTO-SCANNER & MAPPING ENGINE)

Khi thêm một Landing Page mới (cho dù thiết kế bằng Ladipage, tải từ ThemeForest hay code tay thuần), hệ thống cung cấp công cụ tự động hóa không cần viết lại code:

### 1. Cơ chế Quét Form Tự Động (Auto-Scanner)
1. Trong Admin **Quản Lý Landing (Ladi Studio)**:
   * Mở landing page cần cấu hình -> Chọn tab **"Cấu hình Form & Quét Input"**.
   * Nhấn nút **"Quét Form Tự Động Từ Mã HTML"**.
2. Bộ Parser sẽ phân tích toàn bộ thẻ `<form>`, `<input>`, `<select>`, `<textarea>`:
   * Trích xuất các thuộc tính: `Tag`, `Type`, `Name`, `ID`, `Placeholder`, `Class`, Nhãn `<label>` đi kèm.
3. **Thuật toán Gợi ý Ánh xạ Thông minh (Smart Heuristics)**:
   * Phát hiện `type="tel"` hoặc tên/placeholder chứa `sdt`, `phone`, `điện thoại`, `di động` -> Tự động chọn map sang **`phone` (Số điện thoại)**.
   * Phát hiện tên/placeholder chứa `name`, `họ tên`, `tên`, `fullname` -> Tự động chọn map sang **`fullname` (Họ và tên)**.
   * Phát hiện `type="email"` hoặc tên chứa `email`, `thư điện tử` -> Tự động chọn map sang **`email` (Email)**.
   * Phát hiện tên/placeholder chứa `note`, `ghi chú`, `nhu cầu`, `tin nhắn` -> Tự động chọn map sang **`note` (Ghi chú)**.
   * Phát hiện tên/placeholder chứa `gói`, `khóa học`, `package` -> Tự động chọn map sang **`package` (Gói quan tâm)**.

### 2. Bảng Cấu Hình Ánh Xạ Động (Stored in JSONB)
Cấu hình sau khi quét được lưu vào trường `settings.form_mapping` của Landing Page trong CSDL PostgreSQL:

```json
{
  "form_selector": "form#leadForm, form.ladi-form",
  "mapping": {
    "phone": "#phone, input[name='phone'], input[name='ladi-phone']",
    "fullname": "#fullname, input[name='name'], input[name='ladi-name']",
    "email": "#email, input[name='email']",
    "note": "#note, textarea[name='message']",
    "package": "#package, input[name='course']"
  },
  "success_redirect": "/chucmung.html",
  "auto_inject_script": true
}
```

---

## VI. HƯỚNG DẪN 3 BƯỚC NHÚNG VÀO BẤT KỲ LANDING PAGE MỚI

Để tích hợp một Landing Page hoàn toàn mới vào hệ thống:

### Bước 1: Đặt file vào thư mục quản trị
* Đặt thư mục Landing Page mới vào `frontend/landing/[ten_landing_moi]/` hoặc import mã HTML vào Ladi Studio.

### Bước 2: Nhúng 1 dòng Script duy nhất
Chèn đoạn mã sau vào trước thẻ đóng `</body>` của file HTML:
```html
<script src="/tracker.js" data-landing-id="MA_LANDING_CUA_BAN"></script>
```
*(Nếu truy cập qua route `/p/:page_id` của hệ thống, Backend Server đã tự động inject script này vào HTML mà bạn không cần sửa code).*

### Bước 3: Kiểm tra hoạt động
1. Mở trang Landing trên trình duyệt.
2. Thử nhập một số điện thoại sai định dạng (ví dụ `12345` hoặc `0123456789`) -> Kiểm tra thông báo cảnh báo màu đỏ xuất hiện dưới ô input.
3. Nhập số điện thoại đúng 10 số (ví dụ `0984386538`) và bấm Đăng ký.
4. Xác nhận:
   * Trình duyệt tự động chuyển hướng sang trang `chucmung.html`.
   * Lead mới xuất hiện ngay lập tức trong bảng Leads của CRM Admin (`/admin`).
   * Dữ liệu tự động đồng bộ sang Google Sheets tương ứng.
   * Cookie 180 ngày đã được lưu thành công.

---
*Tài liệu được biên soạn và chuẩn hóa bởi APEX Fintech Development Team.*
