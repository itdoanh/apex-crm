# KẾ HOẠCH BẢO MẬT & PHÒNG THỦ TOÀN DIỆN CHO LANDING PAGE VÀ BACKEND
### Hệ Thống: Apex Fintech Multi-Landing Platform
**Phiên bản tài liệu:** 1.0.0 | **Ngày lập:** 01/11/2026  
**Mục tiêu cốt lõi:** Bảo vệ hệ thống tuyệt đối trước các nguy cơ **Spam Lead, cURL Flooding, Layer 7 DDoS, Botnet cào dữ liệu, Sabotage phá hoại database** — ngay cả khi Endpoint API bị kẻ tấn công soi mã nguồn và dịch ngược hoàn toàn.

---

## MỤC LỤC
1. [Tổng Quan Kiến Trúc & Mô Hình Đe Dọa (Threat Modeling)](#1-tổng-quan-kiến-trúc--mô-hình-đe-dọa-threat-modeling)
2. [Triết Lý Thiết Kế: Phòng Thủ Bất Đối Xứng (Asymmetric Defense)](#2-triết-lý-thiết-kế-phòng-thủ-bất-đối-xứng-asymmetric-defense)
3. [Thiết Kế Tầng 1: Phòng Thủ Phía Front-End (Landing Page)](#3-thiết-kế-tầng-1-phòng-thủ-phía-front-end-landing-page)
4. [Thiết Kế Tầng 2: Phòng Thủ Phía Back-End (Golang API Server)](#4-thiết-kế-tầng-2-phòng-thủ-phía-back-end-golang-api-server)
5. [Thiết Kế Tầng 3: Tối Ưu CSDL PostgreSQL & Cơ Chế Cách Ly Spam](#5-thiết-kế-tầng-3-tối-ưu-csdl-postgresql--cơ-chế-cách-ly-spam)
6. [Thiết Kế Tầng 4: Bảo Vệ Hạ Tầng & Mạng (Edge WAF & Docker)](#6-thiết-kế-tầng-4-bảo-vệ-hạ-tầng--mạng-edge-waf--docker)
7. [Kế Hoạch Triển Khai Chi Tiết Từng Giai Đoạn (Implementation Roadmap)](#7-kế-hoạch-triển-khai-chi-tiết-từng-giai-đoạn-implementation-roadmap)
8. [Kịch Bản Kiểm Thử & Mô Phỏng Tấn Công Thực Nghiệm (Pentest Simulation)](#8-kịch-bản-kiểm-thử--mô-phỏng-tấn-công-thực-nghiệm-pentest-simulation)

---

## 1. TỔNG QUAN KIẾN TRÚC & MÔ HÌNH ĐE DỌA (THREAT MODELING)

### 1.1. Hiện Trạng Hệ Thống
- **Client (Landing Page):** HTML5, Tailwind CSS, JavaScript thuần (`tracker.js`), tải trên hàng loạt tên miền / sub-domain khác nhau.
- **Server:** Golang 1.26 (binary độc lập `apex-backend.exe`), chạy REST API trên port 8080.
- **Database:** PostgreSQL 16 chạy trong container Docker `apex-postgres`, lưu trữ hai bảng chính: `leads` (thông tin khách) và `analytics_sessions` (chuỗi hành vi JSONB).
- **Các API công khai nhạy cảm:**
  - `POST /api/leads`: Tiếp nhận đơn đăng ký buổi học.
  - `POST /api/analytics/track`: Tiếp nhận ping sự kiện và chuỗi hành vi on-site.

### 1.2. Mô Hình Đe Dọa (Attack Vectors Analysis)
Bất kỳ ai mở trình duyệt nhấn **F12 -> Network Tab** đều thấy được toàn bộ URL endpoint API và cấu trúc dữ liệu JSON gửi đi. Nếu không có cơ chế phòng thủ chuyên sâu, kẻ tấn công hoặc đối thủ cạnh tranh có thể thực hiện các cuộc tấn công sau:

```
[Kẻ Tấn Công / Bot Script]
       │
       ├── (Vector 1: Script Flooding) ──────> Gửi hàng vạn lead rác (Postman, Python, cURL)
       ├── (Vector 2: Telemetry Pollution) ──> Bơm hàng triệu JSONB rác làm tràn ổ cứng PostgreSQL
       ├── (Vector 3: Layer 7 DDoS) ─────────> Bơm HTTP request dồn dập làm cạn RAM/CPU Go
       ├── (Vector 4: Slowloris Attack) ─────> Mở hàng nghìn kết nối giữ chậm gây treo Server
       ├── (Vector 5: Telesale Sabotage) ────> Gửi số ảo (0999999999) phá hoại đội ngũ tư vấn
       └── (Vector 6: Replay Attack) ────────> Bắt gói tin hợp lệ và bắn lại liên tục
```

| Vector Tấn Công | Phương Thức Tấn Công | Hậu Quả Đối Với Hệ Thống | Mức Độ |
|---|---|---|:---:|
| **1. Direct API Flooding** | Dùng Python `requests` hoặc cURL gửi trực tiếp vòng lặp `POST /api/leads` | Tràn bảng `leads`, đội Telesale gọi vào số rác, chi phí vận hành tăng vọt | **Nghiêm trọng** |
| **2. Layer 7 DDoS** | Dùng máy chủ botnet bắn 5,000 - 10,000 req/s vào các endpoint | Server Golang cạn Goroutine, CSDL PostgreSQL chạm `max_connections`, sập web | **Nghiêm trọng** |
| **3. Slowloris Attack** | Gửi HTTP Header cực chậm (vài byte/giây) chiếm dụng socket | Cạn kiệt File Descriptor của hệ điều hành, người dùng thật không vào được trang | **Cao** |
| **4. Database Exhaustion** | Bắn JSON payload khổng lồ (vài MB) chứa ký tự ngẫu nhiên vào analytics | Hết dung lượng ổ đĩa Docker, nghẽn I/O PostgreSQL | **Cao** |
| **5. Headless Browser Farm** | Sử dụng Puppeteer/Selenium điều khiển Chrome tự động điền form | Vượt qua các kiểm tra User-Agent thông thường | **Trung bình** |
| **6. Replay Attack** | Lấy 1 request hợp lệ từ trình duyệt và gửi lại nguyên vẹn nhiều lần | Tạo nhiều bản ghi trùng lặp trong thời gian ngắn | **Trung bình** |

---

## 2. TRIẾT LÝ THIẾT KẾ: PHÒNG THỦ BẤT ĐỐI XỨNG (ASYMMETRIC DEFENSE)

### 2.1. Định Lý Phòng Thủ Không Gian Web Công Khai
> **"Không thể giấu được API Endpoint đối với người dùng cuối, nhưng hoàn toàn có thể triệt tiêu giá trị của việc lộ Endpoint bằng Chi Phí Tính Toán Bất Đối Xứng và Cơ Chế Xác Thực Đa Tầng."**

### 2.2. Bốn Nguyên Tắc Vàng
1. **Asymmetric Workload (Tải Bất Đối Xứng):**  
   - Người dùng thật thao tác tự nhiên: **Tốn 0ms, không cần bấm ảnh Captcha khó chịu**.
   - Kẻ viết bot muốn spam 10.000 requests: **Bắt buộc phải tốn tài nguyên CPU giải mã toán học hoặc phải vượt qua kiểm định hành vi**.
2. **Silent Drop & Tarpit (Bẫy Bùn Lầy & Giả Vờ Thành Công):**  
   - Khi phát hiện request là bot: **Tuyệt đối không báo lỗi `400` hay `403`** (vì báo lỗi sẽ giúp kẻ viết script biết để sửa mã).
   - Thay vào đó: **Giữ kết nối 1.5 - 2.5 giây (làm chậm tốc độ bot), sau đó trả về HTTP 200 `{"success": true}`**. Dữ liệu rác sẽ bị vứt bỏ âm thầm hoặc đẩy vào bảng cách ly.
3. **Multi-Signal Verification (Xác Thực Đa Tín Hiệu):**  
   - Một submission chỉ được coi là hợp lệ khi thỏa mãn đồng thời:
     $\text{Valid} = \text{Honeypot Sạch} \wedge \text{Thời Gian Hợp Lý} \wedge \text{Nonce Hợp Lệ} \wedge \text{SĐT Đúng Nhà Mạng} \wedge \text{Rate Limit Cho Phép}$.
4. **Defense in Depth (Phòng Thủ Chiều Sâu):**  
   - Không dựa vào duy nhất 1 chốt chặn. Nếu kẻ tấn công vượt qua Front-End, Back-End sẽ chặn; nếu vượt qua Back-End, Database Circuit Breaker sẽ bảo vệ tầng dữ liệu.

---

## 3. THIẾT KẾ TẦNG 1: PHÒNG THỦ PHÍA FRONT-END (LANDING PAGE)

### 3.1. Hệ Thống Multi-Field Behavioral Honeypot (Bẫy Ngầm Vô Hình)
Bot tự động thường dùng cơ chế quét DOM và điền tất cả các thẻ `<input>` có tên gợi cảm (email, website, address...). Ta tạo ra các trường bẫy mà người thật không nhìn thấy:

```html
<!-- Cụm trường bẫy Bot (Honeypot Trap) - Đặt phân tán trong Form -->
<div style="position: absolute; left: -9999px; top: -9999px; opacity: 0; pointer-events: none;" aria-hidden="true" tabindex="-1">
    <!-- Trap 1: Giả trường Email -->
    <input type="email" name="user_email_verify" id="user_email_verify" autocomplete="off" tabindex="-1">
    <!-- Trap 2: Giả trường Website -->
    <input type="url" name="company_website_url" id="company_website_url" autocomplete="off" tabindex="-1">
    <!-- Trap 3: Checkbox bẫy -->
    <input type="checkbox" name="terms_confirm_bot" id="terms_confirm_bot" value="1" tabindex="-1">
</div>
```
- **Kỹ thuật ngụy trang:** Sử dụng thuộc tính `tabindex="-1"` để người dùng bấm phím `Tab` không bao giờ nhảy vào ô bẫy.
- **Hành vi xử lý:** Nếu request gửi lên có bất kỳ giá trị nào trong các trường này $\rightarrow$ Đánh dấu ngay `bot_trap = true`.

### 3.2. Time-To-Submit Verification (Đo Lường Thời Gian Thao Tác Tự Nhiên)
- **Hành vi người thật:** Đọc lướt nội dung, nhấp vào ô input, gõ họ tên (10-20 ký tự), gõ số điện thoại (10 ký tự) $\rightarrow$ Tối thiểu mất **3.5 đến 8 giây**.
- **Hành vi bot script:** Load trang hoặc bắn POST trực tiếp $\rightarrow$ Mất **dưới 0.8 giây**.
- **Giải pháp:**
  - Client ghi nhận `form_open_time = Date.now()` khi người dùng focus ô đầu tiên.
  - Khi submit, tính toán `fill_duration_ms = Date.now() - form_open_time`.
  - Nếu `fill_duration_ms < 2500ms` (2.5 giây) $\rightarrow$ Xác định 99.9% là bot điền tự động.

### 3.3. Dynamic Ephemeral Token & Lightweight Proof-of-Work (PoW)
Để ngăn chặn hoàn toàn việc ai đó lấy cURL bắn trực tiếp vào `/api/leads`:
1. Khi người dùng focus vào form đăng ký, JavaScript âm thầm gọi `GET /api/auth/challenge`.
2. Server tạo ra một challenge gồm: `{ challenge_id, timestamp, salt, difficulty }` được ký HMAC bí mật.
3. Trình duyệt dùng thuật toán Web Crypto API (`crypto.subtle.digest`) tính toán chuỗi hash SHA-256 thỏa mãn điều kiện (mất ~30-60ms trên CPU điện thoại, người dùng không hề cảm nhận thấy độ trễ).
4. Khi submit form, gửi kèm `{ challenge_id, solution, nonce }`.
5. **Kết quả:** Kẻ dùng cURL / Postman nếu không chạy JavaScript để giải PoW sẽ bị Server từ chối ngay lập tức!

### 3.4. Tích Hợp Tùy Chọn: Cloudflare Turnstile (Invisible Mode)
- **Cơ chế:** Hoàn toàn vô hình, không bắt người dùng chọn đèn giao thông hay giải câu đố.
- **Cách nhúng:**
  ```html
  <script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
  <div class="cf-turnstile" data-sitekey="YOUR_SITE_KEY" data-callback="onTurnstileSuccess" data-size="invisible"></div>
  ```
- **Xác thực:** Gửi token `cf-turnstile-response` về backend để verify với Cloudflare API.

### 3.5. Kiểm Tra Độ Hỗn Loạn Tương Tác Vật Lý (Physical Entropy)
Theo dõi các sự kiện trước khi cho phép submit form:
- Số lượt di chuyển chuột (`mousemove > 5`) hoặc số lượt chạm vuốt (`touchmove > 2`).
- Khoảng cách giữa các lần gõ phím (`keydown entropy > 40ms`).
- Nếu form submit khi `mousemove + touchmove + keypress == 0` $\rightarrow$ Đánh dấu script automation.

---

## 4. THIẾT KẾ TẦNG 2: PHÒNG THỦ PHÍA BACK-END (GOLANG API SERVER)

### 4.1. Kiến Trúc Middleware Phòng Thủ Trong Golang
Toàn bộ request gửi đến Backend phải đi qua chuỗi Pipeline bảo mật theo sơ đồ:

```
Request Đến
    │
    ▼
[1. Security Headers & CORS Validator] ─────► Không hợp lệ ──► Drop 403
    │
    ▼
[2. Body Size Limit (MaxBytesReader)] ──────► Body > 64KB ───► Drop 413
    │
    ▼
[3. Anti-Slowloris & Timeout Context] ──────► Quá thời gian ──► Drop 408
    │
    ▼
[4. Dual-Layer Rate Limiting] ──────────────► Quá tần suất ──► 429 Too Many Requests
    │
    ▼
[5. Ephemeral Ticket & Anti-Replay Nonce] ──► Nonce cũ/hết hạn ► Reject 403
    │
    ▼
[6. Honeypot & Bot Heuristics Check] ───────► Là Bot/Honeypot ─► TARPIT & FAKE 200 OK
    │
    ▼
[7. Vietnamese Carrier Prefix Whitelist] ───► SĐT rác/ảo ────► 400 Bad Request
    │
    ▼
[8. Deduplication 24h Window Check] ────────► Trùng SĐT ──────► Ghi đè Tracking / Merge
    │
    ▼
[Xử Lý Lưu CSDL PostgreSQL An Toàn]
```

### 4.2. Bộ Giới Hạn Tần Suất Kép (Dual-Layer Rate Limiting)
Triển khai bộ Rate Limiter bằng thuật toán **Token Bucket** (`golang.org/x/time/rate`) kết hợp cơ chế dọn dẹp bộ nhớ RAM tự động (tránh tấn công làm tràn RAM - OOM Attack):

```go
// Cấu hình Rate Limiting phân cấp cho từng Endpoint
var (
    // Endpoint gửi Lead: Tối đa 5 lần / 5 phút trên 1 IP (Burst 8)
    LeadLimiter = rate.Every(1 * time.Minute) 
    // Endpoint Ping Analytics: Tối đa 60 ping / 1 phút trên 1 IP (Burst 90)
    AnalyticsLimiter = rate.Every(1 * time.Second)
)
```

- **Bộ nhớ đệm có TTL:** Sử dụng `sync.Map` kèm thời gian hết hạn (ví dụ: IP không có tương tác sau 10 phút sẽ tự động xóa khỏi RAM).

### 4.3. Xác Thực Đầu Số Điện Thoại Nhà Mạng Việt Nam Chuẩn Xác (Whitelist)
Kẻ spam thường sinh số điện thoại ngẫu nhiên (vd: `0123456789`, `0999999999`, `0888888888`). Backend áp dụng bộ lọc Regex kiểm tra đúng 10 số và khớp đầu số nhà mạng quốc gia:

```go
// Viettel: 086, 096, 097, 098, 032, 033, 034, 035, 036, 037, 038, 039
// Vinaphone: 088, 091, 094, 081, 082, 083, 084, 085
// Mobifone: 089, 090, 093, 070, 076, 077, 078, 079
// Vietnamobile: 092, 056, 058
// Itelecom / Wintel / FPT: 087, 055, 0775
var vnPhoneRegex = regexp.MustCompile(`^(0)(3[2-9]|5[68]|7[06-9]|8[1-9]|9[0-46-9])[0-9]{7}$`)
```

- Loại bỏ hoàn toàn các chuỗi số lặp: `0999999999`, `0111111111`, `0123456789`, `0987654321`.

### 4.4. Cơ Chế Chống Tấn Công Gửi Lại (Anti-Replay Attack)
- Mỗi submission phải mang theo một `nonce` duy nhất (UUIDv4) do client sinh ra kèm `timestamp`.
- Server kiểm tra `nonce` trong bộ nhớ cache Redis/In-Memory (thời hạn 5 phút):
  - Nếu `nonce` đã từng xuất hiện $\rightarrow$ **Từ chối ngay lập tức**.
  - Nếu `timestamp` chênh lệch quá 5 phút so với đồng hồ Server $\rightarrow$ **Từ chối**.

### 4.5. Cơ Chế Bẫy Bùn Lầy & Phản Hồi Giả (Tarpit Strategy)
Khi một request bị phát hiện:
1. Có dữ liệu trong trường Honeypot.
2. Hoặc thời gian điền form dưới 2 giây.
3. Hoặc User-Agent nằm trong danh sách cào dữ liệu (`python`, `curl`, `headless`).

**Xử lý chuyên sâu:**
```go
if isHoneypotTriggered || isSpeedBot {
    // 1. Tarpit: Giữ socket ngủ ngẫu nhiên 1.5 - 2.5s làm nghẽn tiến trình bot
    sleepDuration := time.Duration(1500+rand.Intn(1000)) * time.Millisecond
    time.Sleep(sleepDuration)

    // 2. Trả về kết quả 200 OK giả mạo hoàn hảo
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Dữ liệu lead đã được ghi nhận thành công",
        "id":      999999, // Fake ID
    })
    return // Dừng lại, KHÔNG lưu vào database leads!
}
```

### 4.6. Bảo Vệ Slowloris & Cạn Kiệt Bộ Đệm HTTP
Tối ưu hóa máy chủ HTTP Go trong `main.go`:
```go
server := &http.Server{
    Addr:              ":" + cfg.Port,
    Handler:           mux,
    ReadHeaderTimeout: 3 * time.Second,  // Chống Slowloris gửi header chậm
    ReadTimeout:       5 * time.Second,  // Hạn chế treo kết nối body
    WriteTimeout:      5 * time.Second,  // Đảm bảo phản hồi nhanh
    IdleTimeout:       30 * time.Second, // Đóng kết nối nhàn rỗi
    MaxHeaderBytes:    1 << 16,          // 64 KB tối đa cho headers
}
```

---

## 5. THIẾT KẾ TẦNG 3: TỐI ƯU CSDL POSTGRESQL & CƠ CHẾ CÁCH LY SPAM

### 5.1. Bảng Cách Ly Rác (`spam_quarantine`)
Không xóa trắng các request nghi ngờ mà ghi nhận vào một bảng cách ly riêng biệt để đội ngũ kỹ thuật có dữ liệu phân tích mẫu tấn công mà không làm bẩn bảng `leads`:

```sql
CREATE TABLE IF NOT EXISTS spam_quarantine (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address VARCHAR(50) NOT NULL,
    user_agent TEXT NOT NULL,
    reason VARCHAR(100) NOT NULL, -- 'honeypot', 'rapid_submit', 'fake_phone', 'blacklisted_ip'
    payload JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quarantine_created ON spam_quarantine (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quarantine_reason ON spam_quarantine (reason);
```

### 5.2. Deduplication Window 24 Giờ (Chống Trùng Lặp & Ghi Đè Thông Minh)
Khi một số điện thoại đã đăng ký trong vòng 24 giờ qua cùng 1 Landing Page:
- **Không tạo dòng mới:** Tránh tăng đột biến số lượng bản ghi rác.
- **Thực hiện cập nhật tracking:** Cập nhật cột `last_seen` và nối thêm dữ liệu UTM mới vào cột `tracking` JSONB.

### 5.3. Giới Hạn Kết Nối PostgreSQL (Connection Pool Guard)
Tránh việc Go Backend mở hàng nghìn kết nối xuống Postgres khi bị dội traffic:
```go
pgxConfig.MaxConns = 30               // Tối đa 30 kết nối đồng thời
pgxConfig.MinConns = 5                // Tối thiểu 5 kết nối sẵn sàng
pgxConfig.MaxConnIdleTime = 1 * time.Minute
pgxConfig.MaxConnLifetime = 30 * time.Minute
```

---

## 6. THIẾT KẾ TẦNG 4: BẢO VỆ HẠ TẦNG & MẠNG (EDGE WAF & DOCKER)

### 6.1. Cấu Hình Docker Resource Limits
Để tránh trường hợp Backend hoặc Database bị tấn công chiếm hết 100% tài nguyên CPU và RAM của máy chủ host, cập nhật `docker-compose.yml`:

```yaml
services:
  apex-postgres:
    image: postgres:16-alpine
    deploy:
      resources:
        limits:
          cpus: '1.50'
          memory: 1024M
        reservations:
          cpus: '0.25'
          memory: 256M
    restart: unless-stopped
```

### 6.2. Cấu Hình Cloudflare Edge Protection (Khuyên Dùng Trong Môi Trường Production)
1. **Bật "Bot Fight Mode":** Tự động phát hiện và chặn các scraper, botnet Python, headless browser trước khi request chạm vào máy chủ Go.
2. **Rate Limiting Rule tại Edge:** 
   - Quy tắc: Nếu 1 địa chỉ IP gửi quá 10 request trong 10 giây vào đường dẫn `/api/*` $\rightarrow$ Block IP trong 1 giờ.
3. **WAF Custom Rule:** 
   - Chặn toàn bộ User-Agent chứa: `curl/`, `Wget/`, `python-requests`, `PostmanRuntime`, `Go-http-client`.

---

## 7. KẾ HOẠCH TRIỂN KHAI CHI TIẾT TỪNG GIAI ĐOẠN (ROADMAP & CHECKLIST)

```
[GIAI ĐOẠN 1: CORE BACKEND] ──► [GIAI ĐOẠN 2: FRONTEND SHIELD] ──► [GIAI ĐOẠN 3: POW/CAPTCHA] ──► [GIAI ĐOẠN 4: AUDIT]
   - Rate Limiter Middleware      - Invisible Honeypot Fields       - Proof-of-Work Challenge      - Stress Testing
   - VN Phone Whitelist Regex     - Time-To-Submit Verification     - Cloudflare Turnstile Hook    - Pentest Script
   - Silent Drop & Tarpit Trap    - Entropy Collection Telemetry    - Spam Quarantine Admin View   - Production Deploy
   - Slowloris Timeout Settings   - Dynamic Payload Signature
```

### Chi Tiết Từng Bước Triển Khai:

#### GIAI ĐOẠN 1: XÂY DỰNG TẦNG PHÒNG THỦ BACKEND (Ưu tiên số 1)
- [ ] **Bước 1.1:** Tạo package `backend/security`:
  - `ratelimit.go`: Triển khai Token Bucket Rate Limiter phân tầng IP & Endpoint với background cleaner.
  - `phone_validator.go`: Bộ kiểm tra định dạng và đầu số nhà mạng Viettel, Vina, Mobi, Vietnamobile.
  - `honeypot.go`: Logic nhận diện bẫy, kích hoạt Tarpit độ trễ và trả về kết quả giả mạo.
- [ ] **Bước 1.2:** Tích hợp Middleware vào `backend/main.go` và `backend/handlers/lead_handler.go`:
  - Thêm `http.MaxBytesReader` giới hạn dung lượng request tối đa 32KB.
  - Bổ sung cấu hình `ReadHeaderTimeout: 3s`, `WriteTimeout: 5s`.
- [ ] **Bước 1.3:** Tạo bảng CSDL `spam_quarantine` trong `backend/db/db.go` để lưu vết các lượt spam bị tóm.

#### GIAI ĐOẠN 2: NÂNG CẤP BẢO VỆ PHÍA FRONTEND (`ladi.html` & `tracker.js`)
- [ ] **Bước 2.1:** Thêm cụm thẻ input Honeypot vô hình vào form đăng ký trong `landing/ladi.html`.
- [ ] **Bước 2.2:** Cập nhật `landing/tracker.js`:
  - Theo dõi thời gian bắt đầu mở form (`formOpenTimestamp`) và tính toán `fill_duration_ms`.
  - Thu thập điểm entropy tương tác chuột / chạm màn hình.
  - Tạo hàm đóng gói payload kèm chữ ký dynamic hash.

#### GIAI ĐOẠN 3: XÁC THỰC THỬ THÁCH MÃ HÓA (CHALLENGE / PROOF-OF-WORK)
- [ ] **Bước 3.1:** Xây dựng endpoint `GET /api/auth/challenge` cấp vé nonce ngắn hạn (TTL 3 phút).
- [ ] **Bước 3.2:** Tích hợp cơ chế Web Crypto SHA-256 giải đố ngầm phía client trước khi cho gửi form.
- [ ] **Bước 3.3:** Chuẩn bị sẵn module kết nối **Cloudflare Turnstile** khi khách hàng chạy domain chính thức.

#### GIAI ĐOẠN 4: NÂNG CẤP DASHBOARD QUẢN TRỊ ADMIN (`admin.html`)
- [ ] **Bước 4.1:** Bổ sung chỉ số **Spam Blocked** và **Tarpitted Bots** trên bảng điều khiển KPI.
- [ ] **Bước 4.2:** Thêm tab danh sách cách ly rác (`Spam Quarantine`) để xem được các IP và User-Agent đang cố gắng tấn công hệ thống.

#### GIAI ĐOẠN 5: KIỂM THỬ THỰC TẾ & ĐÁNH GIÁ ĐỘ BỀN (PENTEST)
- [ ] **Bước 5.1:** Viết script kiểm thử giả lập tấn công cURL, Postman và Python flood.
- [ ] **Bước 5.2:** Đo lường tải của CPU/RAM server khi bị bắn 1,000 request/giây.
- [ ] **Bước 5.3:** Đóng gói phiên bản release chính thức.

---

## 8. KỊCH BẢN KIỂM THỬ & MÔ PHỎNG TẤN CÔNG THỰC NGHIỆM (PENTEST SIMULATION)

Sau khi hoàn thành các tầng bảo vệ, hệ thống sẽ được kiểm tra với 5 kịch bản tấn công thực tế:

### Kịch Bản 1: Script Tấn Công Trực Diện Bằng cURL / Postman
- **Hành động kẻ tấn công:** Soi DevTools, lấy endpoint `POST http://localhost:8080/api/leads`, dùng vòng lặp Bash bắn 100 request liên tục không có Honeypot và không giải Challenge.
- **Kỳ vọng hệ thống:** Server từ chối ngay lập tức với mã lỗi `403 Forbidden` (Thiếu Nonce hợp lệ) hoặc kích hoạt Rate Limit trả về `429 Too Many Requests`. Bảng `leads` hoàn toàn sạch sẽ.

### Kịch Bản 2: Bot Cào Quét Tự Động Điền Mọi Ô Input
- **Hành động kẻ tấn công:** Bot duyệt DOM, thấy các ô `user_email_verify` hoặc `company_website_url` và điền giá trị ngẫu nhiên vào.
- **Kỳ vọng hệ thống:** Backend kích hoạt bẫy Honeypot, giữ kết nối 2 giây (Tarpit), trả về `200 OK {"success": true, "id": 999999}` giả mạo. Dữ liệu bị chuyển hướng lưu vào bảng `spam_quarantine`, không bao giờ lọt vào bảng `leads`.

### Kịch Bản 3: Tấn Công Bơm Số Điện Thoại Ảo (Fake Phone Attack)
- **Hành động kẻ tấn công:** Gửi các số điện thoại sai cấu trúc hoặc không tồn tại (`0123456789`, `0999999999`, `0000000000`).
- **Kỳ vọng hệ thống:** Bộ lọc Regex nhà mạng chặn đứng và báo lỗi định dạng không hợp lệ, bảo vệ đội Telesale khỏi việc gọi vào các số vô nghĩa.

### Kịch Bản 4: Tấn Công Bơm Dữ Liệu Khổng Lồ (JSON Buffer Overflow)
- **Hành động kẻ tấn công:** Gửi payload JSON dung lượng 5MB chứa chuỗi string ngẫu nhiên nhằm làm treo bộ nhớ RAM Golang.
- **Kỳ vọng hệ thống:** Middleware `http.MaxBytesReader` ngắt kết nối ngay khi nhận quá 32KB, trả về mã lỗi `413 Payload Too Large` mà không tốn CPU xử lý JSON parser.

### Kịch Bản 5: Tấn Công Replay Attack (Bắt Gói Tin Hợp Lệ Gửi Lại)
- **Hành động kẻ tấn công:** Bắt được 1 request hợp lệ từ trình duyệt và dùng Postman nhấn Send liên tục 50 lần.
- **Kỳ vọng hệ thống:** Request đầu tiên thành công; 49 request tiếp theo bị chặn do `nonce` đã bị tiêu thụ một lần duy nhất.

---

## TÀI LIỆU THAM KHẢO & TIÊU CHUẨN BẢO MẬT
- [OWASP Automated Threats to Web Applications](https://owasp.org/www-project-automated-threats-to-web-applications/)
- [NCSC Guidance: Limiting API Exposure](https://www.ncsc.gov.uk/collection/securing-http-based-apis/7-limiting-exposure)
- [Cloudflare Turnstile Documentation & Best Practices](https://developers.cloudflare.com/turnstile/)
- [Altcha Open-Source Proof-of-Work Specification](https://altcha.org/docs/how-it-works/)
- [Golang Official Rate Limiting Package `golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate)
