<div align="center">

# 🏛️ APEX FINTECH MULTI-LANDING & ENTERPRISE CRM ENGINE
### Hệ Thống Quản Trị Đa Landing Page, Marketing Automation & CRM Khách Hàng Tiềm Năng Chuyên Sâu Ngành Hàng Hóa Phái Sinh (MXV 080)

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16%20JSONB-4169E1?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Caddy](https://img.shields.io/badge/Caddy-On--Demand%20TLS-1F88C0?style=for-the-badge&logo=caddy&logoColor=white)](https://caddyserver.com)
[![Windows Server](https://img.shields.io/badge/Windows%20Server-2025%20Ready-0078D4?style=for-the-badge&logo=windows&logoColor=white)](https://microsoft.com)
[![Argon2id](https://img.shields.io/badge/Security-Argon2id%20Hash-10B981?style=for-the-badge&logo=shield&logoColor=white)](#)
[![Google Sheets](https://img.shields.io/badge/Integration-Master%20Service%20Account-34A853?style=for-the-badge&logo=googlesheets&logoColor=white)](#)



**Apex Fintech Multi-Landing & CRM Engine** là nền tảng quản trị khách hàng tiềm năng thế hệ mới được thiết kế may đo cho **Thành viên Kinh doanh số 080 - Sở Giao dịch Hàng hóa Việt Nam (MXV)** thuộc quản lý của **Bộ Công Thương**. Hệ thống tích hợp toàn diện từ phễu chuyển đổi cao (Landing Page), bộ thiết kế trực quan (Visual Drag & Drop Studio), cơ chế tự động cấp phát SSL 2 giây cho mọi tên miền, đến ma trận chống bot/spam đa tầng và luồng tự động đồng bộ Google Sheets qua Master Service Account Bot.

</div>

---

> ## 📚 TÀI LIỆU DỰ ÁN — BẮT BUỘC ĐỌC TRƯỚC KHI LÀM VIỆC
>
> | Bạn là | Đọc file này |
> |---|---|
> | **Người mới / AI agent** | [`AGENTS.md`](AGENTS.md) → [`docs/ONBOARDING.md`](docs/ONBOARDING.md) |
> | **AI agent (Cursor)** | Rules auto-load từ `.cursor/rules/`, skills trong `.cursor/skills/` |
> | **AI agent (Claude Code)** | [`CLAUDE.md`](CLAUDE.md) |
> | **Tổng quan tài liệu** | [`docs/README.md`](docs/README.md) |
>
> **Tài liệu chuyên đề:** [`ARCHITECTURE`](docs/ARCHITECTURE.md) · [`DATABASE`](docs/DATABASE.md) · [`API`](docs/API.md) · [`FRONTEND`](docs/FRONTEND.md) · [`LANDING`](docs/LANDING.md) · [`SECURITY`](docs/SECURITY.md) · [`DEPLOYMENT`](docs/DEPLOYMENT.md) · [`RUNBOOK`](docs/RUNBOOK.md)

---

## 🌟 ĐIỂM ĐẶC BIỆT & ĐỘT PHÁ CÔNG NGHỆ

### 1. 🤖 Tự Động Hóa Đồng Bộ Google Sheets Qua Master Email Bot (Zero-Config)
Quy trình tích hợp Google Sheets thông thường đòi hỏi người dùng tạo dự án trên Google Cloud, tải file JSON Service Account phức tạp và dán key thủ công. **Apex CRM đã cách mạng hóa quy trình này:**
- **Email Bot Hệ Thống (Master Service Account):**
  ```text
  sheet-bot@rinva-drive-tool.iam.gserviceaccount.com
  ```
- **Luồng kết nối 1-Click chuẩn SaaS:**
  1. Người dùng mở file Google Sheet của họ ➔ Nhấn **Chia sẻ (Share)** ➔ Dán email bot và cấp quyền **Người chỉnh sửa (Editor)**.
  2. Dán link Google Sheet vào CRM ➔ Nhấn **Kết Nối & Tải Cấu Trúc**.
- **Backend Go tự động hóa 100%:**
  - Tự động bóc tách `SpreadsheetID` từ mọi biến thể link URL qua Regular Expression.
  - Tự động nạp cấu trúc file: Tiêu đề trang tính, danh sách các Tab (Sheets), số hàng x số cột và toàn bộ danh sách tiêu đề cột.
  - Tự động ghi nhận Lead mới theo thời gian thực (Real-time Append) vào đúng hàng, đúng cột theo ma trận ánh xạ linh hoạt.
  - **Đa cấu hình độc lập:** Mỗi Landing Page có thể gửi Lead về một hoặc nhiều bảng tính Google Sheet riêng biệt.

---

### 2. ⚡ Kiến Trúc Caddy On-Demand TLS: Cấp SSL Tức Thì Trong 2 Giây
- **Tự động cấp chứng chỉ SSL (Let's Encrypt / ZeroSSL) cho MỌI TÊN MIỀN:** Bất kỳ tên miền mới nào của khách hàng hoặc chiến dịch quảng cáo trỏ về địa chỉ IP hoặc cặp NameServers (`ns1`/`ns2.hanghoaphaisinh.net`) đều được Caddy tự động cấp SSL trong **2 giây** khi có lượt truy cập đầu tiên.
- **Thẩm định tên miền nội bộ (Internal Domain Whitelist):** Caddy phối hợp chặt chẽ với Go Backend qua endpoint bảo vệ `/api/internal/check-domain` để ngăn chặn các cuộc tấn công lạm dụng cấp phát chứng chỉ (Denial-of-Service / Certificate Rate Limiting).
- **Không bao giờ cần khởi động lại Server** khi thêm hoặc gỡ bỏ tên miền.

---

### 3. 🎨 Ladi Studio: Visual Drag & Drop Web Builder (Bộ Thiết Kế Web Kéo Thả)
Tích hợp trực tiếp trong Admin CRM, biến bất kỳ Landing Page tĩnh nào thành một không gian chỉnh sửa trực quan:
- **Gizmo Resizer 8 Chốt Thông Minh:** Kéo giãn, phóng to thu nhỏ các khối nội dung, hình ảnh, văn bản mượt mà; thuật toán chống trôi chuột và hỗ trợ khóa tỷ lệ khung hình (Aspect Ratio Lock).
- **Hệ Thống Lịch Sử 50 Bước (Photoshop-style Time Travel):** Hỗ trợ hoàn tác (`Ctrl+Z`), làm lại (`Ctrl+Y`) và dropdown lịch sử trực quan.
- **Quản Lý Layer Phân Tầng Chuyên Nghiệp:** Cây phân cấp Layer nhận diện 7 nhóm phần tử (Text, Image, Button, Form, Section, Box, Icon), hiển thị thứ tự Z-Index, hỗ trợ ẩn/hiện, khóa và sắp xếp layer.
- **Menu Chuột Phải Ngữ Cảnh (Context Menu 20 Hành Động):** Hoạt động xuyên thấu iframe, chống tràn mép màn hình: Nhân bản, Đưa lên trước, Đưa xuống sau, Căn lề tự động, Khóa phần tử, Xóa.
- **Bộ Quét Biểu Mẫu Tự Động (Form Auto-Scanner):** Tự động phân tích mã nguồn HTML/JS của trang để nhận diện các trường input (Họ tên, SĐT, Email, Tỉnh thành...), tự động chèn bộ thu thập dữ liệu `tracker.js` và bảo vệ chống bot.

---

### 4. 🛡️ Ma Trận Phòng Thủ & Chống Spam Đa Tầng (Enterprise Anti-Bot & Anti-Spam)
Được thiết kế đặc thù cho các chiến dịch quảng cáo tài chính có chi phí mỗi lượt chuyển đổi (CPA) cao:
- **Thử Thách Proof-of-Work (SHA-256 Client Challenge):** Bắt buộc trình duyệt giải một bài toán băm mã hóa trước khi gửi form, vô hiệu hóa 100% các script spam tự động (cURL, Python Requests, Botnet).
- **Bẫy Honeypot Ẩn:** Các trường input giả lập vô hình với mắt người thật nhưng thu hút bot tự động điền; kích hoạt cơ chế cách ly lead rác ngay lập tức.
- **Phân Tích Hành Vi Người Thật (Behavioral Metrics):** Kiểm tra thời gian tối thiểu tương tác trên trang (>3 giây), phát hiện di chuyển chuột (mouse movement), hành vi cuộn trang (scroll depth), gõ phím.
- **Nhận Diện Nhà Mạng Viễn Thông Việt Nam:** Tự động chuẩn hóa số điện thoại theo chuẩn E.164, nhận diện chính xác 5 nhà mạng lớn: **Viettel**, **Vinaphone**, **Mobifone**, **Vietnamobile**, **Wintel**.
- **Định Vị Địa Lý IP (GeoIP) & Bóc Tách Nguồn Quảng Cáo Chuyên Sâu:** Tự động phát hiện tỉnh/thành phố và phân tích nguồn traffic từ UTM parameters: Facebook Feed, Stories, Reels, Audience Network, Google Search, TikTok Ads...
- **Cơ Chế Cookie Chuyển Đổi 180 Ngày:** Khách hàng đã gửi thông tin thành công 1 lần sẽ tự động chuyển hướng sang trang `chucmung.html` khi vào lại trong 180 ngày, tránh trùng lặp lead và tối ưu hóa chuyển đổi.
- **An Toàn Tuyệt Đối Cho Tính Năng Tự Động Điền (Autofill-Safe):** Thuật toán bảo mật được tinh chỉnh để người dùng sử dụng tính năng tự động điền của Google Chrome / Apple Keychain vẫn đăng ký mượt mà không bị nhận diện nhầm là bot.

---

### 5. 📊 Enterprise CRM Single Page Application (SPA)
- **Kiến trúc SPA hiện đại:** Xây dựng trên nền tảng HTML5, Tailwind CSS và Alpine.js với độ trễ phản hồi <50ms.
- **Chế Độ Xem Linh Hoạt:** Chuyển đổi tức thì giữa **Bảng Danh Sách (Table View)** lọc đa chiều và **Quy Trình Kanban (Pipeline 7 Giai Đoạn)**: Mới tiếp nhận ➔ Đang liên hệ ➔ Hẹn gọi lại ➔ Quan tâm ➔ Đã mở tài khoản ➔ Đã nạp vốn ➔ Không có nhu cầu.
- **Customer 360° Drawer:** Mở xem toàn cảnh chi tiết một khách hàng chỉ với 1 click: Thông tin thiết bị, độ phân giải màn hình, địa chỉ IP, vị trí địa lý, nguồn chiến dịch UTM, lịch sử cuộc gọi và ghi chú nội bộ.
- **Biểu Đồ Khung Giờ Vàng (Golden Hour Heatmap):** Phân tích mật độ tiếp nhận lead theo từng khung giờ trong ngày và từng ngày trong tuần, giúp đội ngũ Telesale trực chiến hiệu quả nhất.

---

### 6. 🪟 Tối Ưu Hóa Tuyệt Đối Cho Windows Server 2025 Datacenter (<1GB Disk Footprint)
Được cấu hình sẵn cho các máy chủ doanh nghiệp có tài nguyên ổ cứng giới hạn (~30GB SSD khả dụng):
- **Binary Go duy nhất:** Dung lượng siêu nhẹ chỉ **~22MB**, chạy Native Windows Service không cần cài đặt Runtime rườm rà.
- **PostgreSQL Native Service:** Tiết kiệm hơn **15GB ổ cứng** so với việc chạy Docker Desktop/WSL2 trên Windows Server.
- **Kịch bản tự động hóa dọn dẹp ổ đĩa (`cleanup_disk_space.ps1`):** Tự động dọn file tạm hệ thống hàng tuần qua Windows Task Scheduler, bảo đảm máy chủ luôn dư dả từ **25GB - 28GB trống**.
- **Dịch Vụ Chạy Ngầm 24/7:** Thiết lập qua NSSM với cơ chế tự khởi động lại sau 5 giây nếu gặp sự cố và tự động xoay vòng file log (Log Rotation 10MB).

---

## 📐 SƠ ĐỒ KIẾN TRÚC HỆ THỐNG

```
[ Người Dùng / Khách Hàng Tiềm Năng ]       [ Quản Trị Viên / Leader / Telesale ]
                  │                                             │
                  ▼                                             ▼
       localhost             https://crm.domain/ads
                  │                                             │
                  └──────────────────────┬──────────────────────┘
                                         ▼
                 ┌──────────────────────────────────────────────┐
                 │          CADDY REVERSE PROXY & TLS           │
                 │   - Tự động cấp SSL On-Demand trong 2 giây   │
                 │   - Chặn truy cập trực tiếp file nhạy cảm    │
                 │   - Giao thức HTTP/2 & HTTP/3 siêu tốc       │
                 └──────────────────────┬───────────────────────┘
                                        │ (Proxy nội bộ 127.0.0.1:8080)
                                        ▼
                 ┌──────────────────────────────────────────────┐
                 │           APEX BACKEND ENGINE (GO)           │
                 │ ──────────────────────────────────────────── │
                 │ • Security Middleware (PoW, Honeypot, Limit) │
                 │ • Dynamic Multi-Landing Router & Caching     │
                 │ • Phone Normalizer & Vietnamese Carrier      │
                 │ • GeoIP & Deep Ad Placement Parser (FB/GG)   │
                 │ • Argon2id Auth & Audit Session Logger       │
                 └──────────────┬───────────────────────────────┘
                                │
        ┌───────────────────────┴───────────────────────┐
        ▼                                               ▼
┌───────────────────────────────┐       ┌───────────────────────────────┐
│     POSTGRESQL 16 NATIVE      │       │      GOOGLE SHEETS API v4     │
│ - Lưu trữ Leads & Status CRM  │       │  (Master Service Account Bot) │
│ - Cột Tracking JSONB tối ưu   │       │ - sheet-bot@rinva-drive-tool  │
│ - Tên miền, Cấu hình Landing  │       │ - Realtime Lead Streaming     │
│ - Tài khoản Admin & Audit Log │       │ - Tự động nạp Tab & Cột       │
└───────────────────────────────┘       └───────────────────────────────┘
```

---

## 📁 CẤU TRÚC THƯ MỤC DỰ ÁN

```text
apex-crm/
├── backend/                            # Mã nguồn Go Backend hiệu năng cao
│   ├── cache/                          # In-Memory Cache (Domain, Landing configs)
│   ├── config/                         # Đọc cấu hình môi trường (.env)
│   ├── db/                             # Tầng truy xuất PostgreSQL (pgx/v5)
│   ├── handlers/                       # Bộ xử lý RESTful API (Lead, Sheet, Admin, Auth)
│   ├── integrations/googlesheets/      # Module tích hợp Google Sheets & Drive API v4
│   ├── models/                         # Khai báo Struct dữ liệu (Lead, Admin, Domain)
│   ├── security/                       # Ma trận bảo mật (PoW, Honeypot, GeoIP, Argon2id)
│   ├── static/                         # Thư mục tệp tĩnh đồng bộ phục vụ Web Server
│   ├── tests/                          # Bộ kiểm thử tự động E2E (Playwright & Python)
│   ├── go.mod / go.sum                 # Quản lý phụ thuộc Go Modules
│   └── main.go                         # Entrypoint máy chủ Go
│
├── frontend/                           # Giao diện người dùng phân tách độc lập
│   ├── admin/                          # Apex Enterprise CRM Single Page Application
│   │   ├── index.html                  # Giao diện Master SPA
│   │   ├── config.js                   # Cấu hình API Backend linh hoạt
│   │   ├── css/admin.css               # Phong cách Dark Glassmorphism tài chính
│   │   └── js/                         # Bộ mô-đun điều khiển SPA
│   │       ├── app.js / auth.js        # Điều phối chính & Xác thực phiên
│   │       ├── crm.js / analytics.js   # Quản lý khách hàng, Kanban & Biểu đồ Chart.js
│   │       ├── sheets.js               # Đồng bộ Google Sheets qua Master Email Bot
│   │       ├── domains.js / landings.js# Quản lý tên miền DNS & Danh mục landing
│   │       └── studio/                 # Bộ công cụ Ladi Studio Drag & Drop
│   │           ├── gizmo.js            # Khung 8 chốt kéo thả kích thước
│   │           ├── history.js          # Hệ thống Undo/Redo 50 bước
│   │           ├── layers.js           # Quản lý cây Layer Z-Index
│   │           ├── context-menu.js     # Menu chuột phải 20 chức năng
│   │           └── form-scanner.js     # Bộ quét biểu mẫu tự động
│   └── landing/                        # Landing page chuẩn MXV 080 Bộ Công Thương
│       ├── index.html / ladi.html      # Mã nguồn HTML tối ưu chuyển đổi
│       ├── chucmung.html               # Trang cảm ơn sau khi đăng ký
│       ├── tracker.js                  # Thư viện thu thập dữ liệu & PoW Solver
│       ├── config.json                 # Cấu hình buổi chia sẻ, ngày giờ, Zoom
│       └── assets/img/                 # Hình ảnh, chứng nhận MXV, biểu đồ thị phần
│
├── deploy/                             # Bộ công cụ đóng gói & triển khai máy chủ
│   ├── windows/                        # Kịch bản tự động hóa cho Windows Server 2025
│   │   ├── Caddyfile                   # Cấu hình Caddy Reverse Proxy & On-Demand TLS
│   │   ├── install_caddy.ps1           # Kịch bản tải và cài đặt Caddy tự động
│   │   ├── install_postgres_native.ps1 # Kịch bản cài đặt PostgreSQL Native nhẹ máy
│   │   ├── setup_services.ps1          # Tạo Windows Services chạy ngầm 24/7 (NSSM)
│   │   ├── setup_firewall.ps1          # Khóa cổng nội bộ, mở cổng Web an toàn
│   │   ├── cleanup_disk_space.ps1      # Kịch bản dọn dẹp rác bảo vệ ổ đĩa 30GB
│   │   ├── start_all.bat               # Phím tắt 1-click khởi động toàn bộ hệ thống
│   │   ├── stop_all.bat                # Phím tắt 1-click dừng toàn bộ hệ thống
│   │   └── restart_all.bat             # Phím tắt 1-click khởi động lại hệ thống
│   └── clean_test_data.sql             # SQL Script dọn sạch dữ liệu test khi bàn giao
│
├── .env.example                        # Mẫu cấu hình môi trường chuẩn
├── .gitignore                          # Loại trừ triệt để tệp nhạy cảm và binary
├── DEPLOY_WINDOWS_SERVER_2025.md       # Sổ tay triển khai thực tế trên Windows Server 2025
└── README.md                           # Tài liệu tổng quan dự án
```

---

## 📋 MA TRẬN ÁNH XẠ DỮ LIỆU GOOGLE SHEETS (15 TRƯỜNG CHUẨN)

Hệ thống hỗ trợ ánh xạ tự động hoặc tùy biến 15 trường thông tin chuẩn từ Lead vào các cột tương ứng trên Google Sheet:

| STT | Khóa Dữ Liệu | Tiêu Đề Cột Mẫu | Ví Dụ Dữ Liệu Thực Tế | Mô Tả Chi Tiết |
| :---: | :--- | :--- | :--- | :--- |
| **1** | `thoi_gian` | Thời Gian Ghi Nhận | `2026-11-03 20:15:30` | Thời điểm khách gửi form (Giờ Việt Nam UTC+7) |
| **2** | `ho_ten` | Họ Và Tên | `Nguyễn Văn Doanh` | Họ tên khách hàng đăng ký |
| **3** | `so_dien_thoai` | Số Điện Thoại | `0984386538` | Số điện thoại đã được chuẩn hóa |
| **4** | `nha_mang` | Nhà Mạng | `Viettel` | Viettel, Vinaphone, Mobifone, Vietnamobile... |
| **5** | `landing_id` | Mã Landing | `mxv_080_online` | Định danh trang landing page phát sinh lead |
| **6** ⭐ | `url` | URL Trang Đăng Ký | `https://hanghoaphaisinh.net/?utm_source=facebook&utm_campaign=KhoaHoc_MXV_T11` | **URL đầy đủ của trang khách vừa gửi form** (kèm query string UTM) — truy ngược lead về đúng landing page / campaign |
| **7** | `nguon` | Nguồn Chiến Dịch | `facebook_ads` | Nguồn lưu lượng truy cập |
| **8** | `vi_tri_qc` | Vị Trí Quảng Cáo | `Facebook_Reels` | Feed, Story, Reels, Audience Network, Search |
| **9** | `ip` | Địa Chỉ IP | `113.161.72.45` | Địa chỉ IP của người dùng |
| **10** | `tinh_thanh` | Tỉnh / Thành Phố | `Hà Nội` | Vị trí địa lý giải mã qua GeoIP |
| **11** | `thiet_bi` | Loại Thiết Bị | `Mobile (iPhone)` | Phân loại thiết bị truy cập |
| **12** | `trinh_duyet`| Trình Duyệt | `Safari Mobile 18.2` | Tên và phiên bản trình duyệt |
| **13** | `utm_campaign`| Chiến Dịch UTM | `KhoaHoc_MXV_T11` | Tên chiến dịch Marketing |
| **14** | `thoi_gian_xem`| Thời Gian Xem Trang| `45 giây` | Tổng thời gian khách đọc nội dung trước khi gửi |
| **15** | `ghi_chu` | Ghi Chú / Nhu Cầu | `Cần tư vấn hợp đồng Ngô & Dầu đậu tương` | Nội dung tùy chọn của khách |

> Ngoài 15 trường chuẩn còn có thể ánh xạ: `fill_duration_ms`, `max_scroll`, `utm_source`,
> `utm_medium`, `utm_content`, `utm_term`, `session_id`, `referrer`, `tracking_json`, `custom_text`.

---


---

## 🚀 HƯỚNG DẪN CÀI ĐẶT & CHẠY THỬ NGHIỆM

### 1. Yêu Cầu Môi Trường
- **Go:** Phiên bản 1.24 trở lên.
- **PostgreSQL:** Phiên bản 15 hoặc 16 (Hỗ trợ tốt kiểu dữ liệu `JSONB` và chỉ mục `GIN`).
- **Trình duyệt hiện đại:** Chrome, Edge, Safari hoặc Firefox.

### 2. Khởi Động Nhanh Cục Bộ (Local Development)

```bash
# 1. Clone mã nguồn
git clone https://github.com/itdoanh/apex-crm.git
cd apex-crm

# 2. Tạo file cấu hình từ mẫu
cp .env.example .env

# 3. Tạo cơ sở dữ liệu PostgreSQL
psql -U postgres -c "CREATE DATABASE apex_leads;"

# 4. Chạy Backend Go
cd backend
go run main.go
```

Truy cập hệ thống:
- **Landing Page Mẫu:** `http://localhost:8080/` hoặc `http://localhost:8080/ladi.html`
- **Admin CRM & Studio:** `http://localhost:8080/ads` hoặc `http://localhost:8080/admin`
  - *Tài khoản:* `admin`
  - *Mật khẩu mặc định:* `ApexAdmin@2026!`

---

## 🪟 QUY TRÌNH TRIỂN KHAI WINDOWS SERVER 2025 (TỐI ƯU Ổ ĐĨA 30GB)

Toàn bộ quy trình triển khai máy chủ sản xuất đã được tự động hóa qua thư mục `deploy/windows/`:

1. **Bước 1: Trỏ DNS**
   - Trỏ bản ghi A `crm.hanghoaphaisinh.net` và `*` về IP máy chủ: `103.124.95.232`.
   - Trỏ 2 bản ghi NameServer `ns1` và `ns2` về IP máy chủ.
2. **Bước 2: Cài PostgreSQL Native**
   - Chạy `install_postgres_native.ps1` hoặc cài đặt bản PostgreSQL 16 x64 cho Windows.
   - Nạp dữ liệu sạch qua script: `psql -U postgres -d apex_leads -f deploy/clean_test_data.sql`.
3. **Bước 3: Cài đặt Caddy On-Demand TLS**
   - Mở PowerShell Administrator tại `deploy/windows/` và chạy `.\install_caddy.ps1`.
4. **Bước 4: Cấu hình Tường Lửa Tự Động**
   - Chạy `.\setup_firewall.ps1` (Mở cổng 80, 443; khóa cổng nội bộ 8080 và 5432).
5. **Bước 5: Kích Hoạt Dịch Vụ Tự Khởi Động 24/7**
   - Chạy `.\setup_services.ps1` để đăng ký `ApexBackend` và `ApexCaddy` vào Windows Services.

> 📖 *Xem hướng dẫn chi tiết từng bước tại tài liệu:* [DEPLOY_WINDOWS_SERVER_2025.md](DEPLOY_WINDOWS_SERVER_2025.md)

---

## 🔒 BẢO MẬT & BẢO VỆ DỮ LIỆU BÍ MẬT

Hệ thống tuân thủ nghiêm ngặt các tiêu chuẩn an toàn thông tin:
- **Khóa Service Account & Biến Môi Trường:** Các tệp `service-account.json`, `rinva-*.json`, `.env` được bảo vệ nghiêm ngặt qua `.gitignore` và không bao giờ được commit lên repository công khai.
- **Tường Lửa Web Handler:** Backend Go tích hợp cơ chế lọc trực tiếp, trả về mã lỗi `404 Not Found` ngay lập tức đối với mọi yêu cầu truy cập file chứa phần mở rộng `.json`, `.env`, `.key`, `.sql`, `.exe`.
- **Mật Khẩu Argon2id:** Toàn bộ thông tin xác thực quản trị được mã hóa bằng thuật toán băm Argon2id với độ phức tạp cao, chống tấn công dò quét từ điển và bẻ khóa bằng GPU.

---

## 👤 TÁC GIẢ & BẢN QUYỀN

- **Tác giả phát triển:** **itdoanh** ([GitHub Profile](https://github.com/itdoanh))
- **Giấy phép:** Bản quyền phần mềm nội bộ thuộc sở hữu của itdoanhq. Nghiêm cấm mọi hành vi sao chép và phân phối trái phép khi chưa có sự đồng ý bằng văn bản.


