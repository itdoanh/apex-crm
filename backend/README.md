# Apex Multi-Landing Backend Server (Go + PostgreSQL)

Hệ thống tiếp nhận thông tin khách hàng từ **nhiều Landing Page khác nhau** và lưu trữ toàn bộ dữ liệu thiết bị / trình duyệt (Full Browser Tracking) sử dụng cơ sở dữ liệu **PostgreSQL** với kiểu dữ liệu **JSONB**.

---

## 1. Thiết Kế & Kiến Trúc CSDL
- **Ngôn ngữ:** Golang 1.26 (tối ưu hóa hiệu năng, chạy dưới dạng 1 file thực thi `.exe` độc lập).
- **CSDL:** **PostgreSQL 16** (triển khai qua Docker hoặc server vật lý/cloud, driver chính thức `pgx/v5` với connection pool tối ưu).
- **Cấu trúc bảng `leads` trong PostgreSQL:**
  - `id`: Mã định danh tăng tự động (`BIGSERIAL PRIMARY KEY`).
  - `thoi_gian`: Thời gian ghi nhận theo chuẩn múi giờ Việt Nam (`TIMESTAMPTZ`, định dạng hiển thị `YYYY-MM-DD HH:mm:ss`).
  - `landing_id`: Định danh landing page gửi lead về (`VARCHAR(100)`).
  - `nguon`: Nguồn traffic được cấu hình riêng cho từng landing (`VARCHAR(100)`).
  - `so_dien_thoai`: Số điện thoại của khách hàng đã chuẩn hóa (`VARCHAR(50)`).
  - `tracking`: Cột lưu toàn bộ thông tin trình duyệt, thiết bị, UTM, session dưới dạng **`JSONB`**.
- **Chỉ mục (Index) tối ưu hóa tốc độ:**
  - `idx_leads_landing_id`: Lọc và phân tích lead theo từng Landing Page.
  - `idx_leads_nguon`: Lọc và phân tích hiệu quả theo từng nguồn traffic.
  - `idx_leads_so_dien_thoai`: Tìm kiếm theo SĐT nhanh chóng.
  - `idx_leads_thoi_gian`: Sắp xếp và thống kê theo ngày cực nhanh.
  - `idx_leads_tracking_gin`: Chỉ mục GIN trên cột JSONB phục vụ query nâng cao vào sâu trong payload tracking.

---

## 2. Cách Cấu Hình Đa Landing Page Qua File `config.json`

Mỗi landing page đi kèm một file **`config.json`** nằm cùng thư mục. Khi trang HTML tải lên, script `tracker.js` sẽ tự động đọc file này để:
1. Gán `landing_id` và `nguon` riêng cho từng landing.
2. Tự động điền thông tin buổi chia sẻ (ngày, giờ, hình thức Zoom...) lên giao diện và đồng bộ đồng hồ đếm ngược.

### Cấu trúc file `config.json`:
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

### Cách gắn dữ liệu lên HTML:
Chỉ cần thêm thuộc tính `data-config="<tên_thuộc_tính>"` vào bất kỳ thẻ HTML nào, ví dụ:
```html
<span data-config="thoi_gian_chia_se.hien_thi">20:00 ngày 03/11/2026</span>
<span data-config="thoi_gian_chia_se.ngay">03/11/2026</span>
<span data-config="thoi_gian_chia_se.gio">20:00</span>
<span data-config="thoi_gian_chia_se.hinh_thuc">Học Online qua Zoom</span>
```
Đồng hồ đếm ngược (Countdown) sẽ tự động lấy `target_date` từ `config.json` để đếm lùi chính xác.

---

## 3. Các API Cung Cấp
| Endpoint | Method | Chức năng |
|---|---|---|
| `/api/leads` | `POST` | Tiếp nhận dữ liệu lead + full browser tracking + landing_id + nguồn |
| `/api/leads` | `GET` | Xem danh sách leads có phân trang & lọc (`?page=1&limit=50&phone=...&landing_id=...&nguon=...`) |
| `/api/leads/export.csv` | `GET` | Xuất CSV chuẩn UTF-8 có BOM (hỗ trợ lọc `?landing_id=...&nguon=...`) |
| `/api/stats` | `GET` | Thống kê số lead và danh sách các `landing_id`, `nguon` phục vụ bộ lọc |
| `/health` | `GET` | Kiểm tra trạng thái máy chủ & CSDL |
| `/admin` | `GET` | Dashboard quản trị đa landing trực quan, bộ lọc dropdown landing & nguồn, xem JSON chi tiết |

---

## 4. Khởi Động Với Docker

### Cách 1: Sử dụng Docker Compose (Khuyên dùng)
Tại thư mục gốc của dự án `landing`:
```bash
docker compose up -d
```

### Cách 2: Sử dụng Docker CLI trực tiếp
```bash
docker run -d --name apex-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=apex_leads \
  -e TZ=Asia/Ho_Chi_Minh \
  -p 5432:5432 \
  -v apex_postgres_data:/var/lib/postgresql/data \
  --restart unless-stopped \
  postgres:16-alpine
```

---

## 5. Chạy Backend Server Trên Windows

1. Chạy file `run.bat` (tự động kiểm tra và khởi động container `apex-postgres` nếu chưa chạy).
2. Hoặc chạy trực tiếp từ dòng lệnh:
   ```cmd
   apex-backend.exe
   ```
3. Mặc định server chạy tại:
   - Dashboard Quản trị: `http://localhost:8080/admin`
   - API Endpoint: `http://localhost:8080/api/leads`

### Các biến môi trường tùy chỉnh:
- `PORT`: Cổng máy chủ HTTP (mặc định: `8080`)
- `DB_HOST`: Host PostgreSQL (mặc định: `localhost`)
- `DB_PORT`: Port PostgreSQL (mặc định: `5432`)
- `DB_USER`: Tên người dùng CSDL (mặc định: `postgres`)
- `DB_PASSWORD`: Mật khẩu CSDL (mặc định: `postgres`)
- `DB_NAME`: Tên CSDL (mặc định: `apex_leads`)
- `DB_SSLMODE`: Chế độ SSL (`disable` khi chạy local)
- `DATABASE_URL`: Hoặc truyền trực tiếp chuỗi kết nối:
  `postgres://user:pass@host:5432/dbname?sslmode=disable`

---

---

## 7. Hệ Thống Thống Kê Hành Vi, Phát Hiện Bot & Đánh Giá Landing/Content

Hệ thống theo dõi chi tiết toàn bộ hành vi của người dùng trên mọi Landing Page, phân tách rành mạch giữa **Người dùng thật** và **Bot/Crawler**, lưu trữ vào bảng `analytics_sessions` tối ưu cao của PostgreSQL 16:

1. **Phân Tách Bot vs Người Dùng Thật (Bot Detection Engine):**
   - **Client Heuristics:** Kiểm tra `navigator.webdriver`, headless globals (`callPhantom`, `__nightmare`), headless User-Agent, kích thước màn hình `0x0`, độ hỗn loạn tương tác vật lý (mouse, touch, keypress, scroll entropy).
   - **Server Crawler Inspection:** Nhận diện các bot mạng xã hội và công cụ cào dữ liệu (`Googlebot`, `Bingbot`, `ByteSpider/TikTok`, `FacebookExternalHit`, `PetalBot`, headless browsers, `curl/requests`).
2. **Đo Lường Hành Vi Thực Tế:**
   - **Thời gian trên trang (Dwell Time):** Tính toán chính xác theo giây với Visibility-aware Heartbeat (chỉ tính thời gian tab đang hiển thị thực tế).
   - **Độ cuộn sâu (Scroll Depth):** Ghi nhận các mốc quan trọng `25%`, `50%`, `75%`, `90%`, `100%`.
   - **Tương tác click & form:** Ghi nhận mọi cú nhấp chuột (CTA, nút nhận vé, link Zalo, mở form, điền form).
   - **Timeline hành vi từng giây:** Lưu chuỗi hành động có mốc thời gian `+0s`, `+12s`, `+25s`... dưới dạng mảng JSONB.
3. **Liên Kết Danh Tính (Lead-to-Session Identity Stitching):**
   - Khi khách hàng gửi biểu mẫu thành công, phiên ẩn danh lập tức được cập nhật `converted_lead = TRUE`, gắn số điện thoại và `lead_id`, chèn sự kiện `converted` vào Timeline hành vi.
4. **Báo Cáo & So Sánh Đa Chiều Trên Dashboard Admin (`/admin`):**
   - **KPIs Tổng Quan:** Tổng views, Khách thật, Bot, Tỷ lệ Bot %, Leads, CR %, Thời gian TB, Độ cuộn TB.
   - **So Sánh Từng Landing Page:** Đánh giá trang nào giữ chân khách tốt nhất và đạt CR% cao nhất.
   - **Đo Lường Nguồn & Content Quảng Cáo:** So sánh chi tiết theo `nguon`, `utm_campaign`, `utm_content` để biết bài viết/video nào mang lại nhiều người thật và chuyển đổi cao nhất.
   - **Danh Sách Phiên & Timeline Modal:** Bấm "Timeline" để xem lại toàn bộ hành trình từng giây của khách hàng.


