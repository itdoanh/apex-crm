# 🚀 HƯỚNG DẪN TRIỂN KHAI APEX FINTECH LÊN SERVER SẢN XUẤT (UBUNTU / DEBIAN)

Tài liệu này cung cấp toàn bộ quy trình chuẩn bị, cấu hình tên miền, DNS, và 2 phương án đưa hệ thống **Apex Fintech Multi-Landing & Enterprise CRM** lên máy chủ sản xuất thực tế.

---

## 📋 THÔNG TIN HỆ THỐNG MẪU

- **Địa chỉ IP máy chủ:** `103.124.95.232` *(IP chính thức máy chủ)*
- **Tên miền quản trị Admin/CRM:** `crm.hanghoaphaisinh.net/ads`
- **NameServers hệ thống:**
  - `ns1.hanghoaphaisinh.net` ➔ `103.124.95.232`
  - `ns2.hanghoaphaisinh.net` ➔ `103.124.95.232`
- **Tài khoản quản trị mặc định ban đầu:**
  - Username: `admin`
  - Mật khẩu: `ApexAdmin@2026!` *(Băm Argon2id - đổi ngay sau khi đăng nhập lần đầu)*

---

## 🌐 BƯỚC 1: CẤU HÌNH BẢN GHI DNS TẠI NHÀ CUNG CẤP TÊN MIỀN

Đăng nhập vào trang quản lý DNS (Cloudflare, Tenten, PA Vietnam, Mat Bao, v.v.) của tên miền `hanghoaphaisinh.net` và thêm các bản ghi sau:

| Loại (Type) | Tên (Host / Name) | Giá Trị (Value / Points to) | Giải thích |
| :--- | :--- | :--- | :--- |
| **A** | `crm` | `103.124.95.232` | Tên miền Admin CRM (`crm.hanghoaphaisinh.net`) |
| **A** | `ns1` | `103.124.95.232` | NameServer 1 phục vụ trỏ tên miền phụ |
| **A** | `ns2` | `103.124.95.232` | NameServer 2 phục vụ trỏ tên miền phụ |
| **A** | `@` (hoặc domain gốc) | `103.124.95.232` | Landing page chính |
| **A** | `*` (Wildcard) | `103.124.95.232` | Đón tất cả subdomain landing tạo tự động |

> **Lưu ý với các tên miền landing riêng biệt khác (ví dụ: `dautuhanghoa.vn`):**
> - **Cách 1 (Khuyên dùng):** Trỏ bản ghi **A** của tên miền đó về IP `103.124.95.232`.
> - **Cách 2:** Hoặc đổi cụm NameServer của tên miền đó sang `ns1.hanghoaphaisinh.net` và `ns2.hanghoaphaisinh.net`.

---

## 📁 BƯỚC 2: UPLOAD MÃ NGUỒN LÊN SERVER

Sử dụng `SCP`, `Rsync` hoặc phần mềm `FileZilla` / `WinSCP` để copy toàn bộ thư mục dự án lên máy chủ (đặt tại `/var/www/apex-landing`):

```bash
# Ví dụ lệnh Rsync từ máy cá nhân lên server:
rsync -avz --exclude 'backend/tests' --exclude '.git' ./ root@103.124.95.232:/var/www/apex-landing/
```

Cấu trúc thư mục tối thiểu cần có trên máy chủ:
```text
/var/www/apex-landing/
├── backend/
│   ├── config/
│   ├── db/
│   ├── handlers/
│   ├── static/          <-- Chứa index.html, studio.html, css, js
│   ├── go.mod, go.sum
│   └── main.go
├── frontend/
│   ├── admin/           <-- CRM Core & Studio Modules
│   └── landing/         <-- File landing gốc ladi.html, assets, tracking
├── deploy/
│   ├── nginx/apex.conf
│   └── systemd/apex.service
├── .env.example
├── docker-compose.yml
└── Dockerfile
```

---

## ⚙️ BƯỚC 3: LỰA CHỌN PHƯƠNG ÁN TRIỂN KHAI

### 🔹 PHƯƠNG ÁN A: DÙNG DOCKER COMPOSE (KHUYÊN DÙNG - 1 LỆNH)

Nếu máy chủ đã cài Docker & Docker Compose:

1. **Vào thư mục dự án và tạo file cấu hình `.env`:**
   ```bash
   cd /var/www/apex-landing
   cp .env.example .env
   # Sửa mật khẩu DB_PASSWORD trong file .env nếu cần:
   nano .env
   ```

2. **Khởi chạy toàn bộ hệ thống bằng Docker Compose:**
   ```bash
   docker compose up -d --build
   ```

3. **Kiểm tra trạng thái containers:**
   ```bash
   docker compose ps
   # Xem log vận hành:
   docker compose logs -f app
   ```

---

### 🔹 PHƯƠNG ÁN B: CHẠY TRỰC TIẾP TRÊN LINUX (SYSTEMD SERVICE + POSTGRESQL)

1. **Cài đặt PostgreSQL trên Ubuntu nếu chưa có:**
   ```bash
   sudo apt update
   sudo apt install -y postgresql postgresql-contrib
   sudo systemctl enable postgresql
   sudo systemctl start postgresql

   # Tạo CSDL và phân quyền:
   sudo -u postgres psql -c "CREATE DATABASE apex_leads;"
   sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD 'postgres';"
   ```

2. **Biên dịch binary hoặc dùng binary đã build:**
   - **Cách 1 (Build trên máy Linux):**
     ```bash
     cd /var/www/apex-landing/backend
     go build -ldflags="-w -s" -o /var/www/apex-landing/apex-server .
     ```
   - **Cách 2 (Build từ Windows bằng script có sẵn):**
     Chạy file `backend\build_linux.bat` trên Windows, sau đó upload file `apex-server` lên `/var/www/apex-landing/`.

3. **Cấp quyền thực thi và tạo file `.env`:**
   ```bash
   cd /var/www/apex-landing
   chmod +x apex-server
   cp .env.example .env
   ```

4. **Kích hoạt Systemd Service:**
   ```bash
   sudo cp deploy/systemd/apex.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable apex.service
   sudo systemctl start apex.service
   sudo systemctl status apex.service
   ```

---

## 🔒 BƯỚC 4: CẤU HÌNH NGINX & CÀI ĐẶT SSL HTTPS (CERTBOT)

1. **Cài đặt Nginx & Certbot:**
   ```bash
   sudo apt update
   sudo apt install -y nginx certbot python3-certbot-nginx
   ```

2. **Kích hoạt file cấu hình Apex trong Nginx:**
   ```bash
   sudo cp /var/www/apex-landing/deploy/nginx/apex.conf /etc/nginx/sites-available/apex.conf
   sudo ln -s /etc/nginx/sites-available/apex.conf /etc/nginx/sites-enabled/
   sudo nginx -t
   sudo systemctl reload nginx
   ```

3. **Cài đặt chứng chỉ SSL miễn phí (Let's Encrypt):**
   ```bash
   # Cấp chứng chỉ cho Admin CRM và các domain landing:
   sudo certbot --nginx -d crm.hanghoaphaisinh.net -d dautuhanghoa.vn -d www.dautuhanghoa.vn
   ```
   *Certbot sẽ tự động cấu hình gia hạn tự động và bật chuyển hướng HTTPS an toàn.*

4. **Mở cổng tường lửa UFW (nếu có bật tường lửa):**
   ```bash
   sudo ufw allow 'Nginx Full'
   sudo ufw allow OpenSSH
   sudo ufw enable
   ```

---

## ✅ BƯỚC 5: NGHIỆM THU & TRUY CẬP HỆ THỐNG

1. **Truy cập Giao Diện Admin CRM:**
   - URL: `https://crm.hanghoaphaisinh.net/ads` (hoặc `http://<IP_SERVER>:8080/ads`)
   - Đăng nhập: `admin` / `ApexAdmin@2026!`
   - Vào mục **Hồ sơ Admin** (Góc phải trên) để đổi mật khẩu quản trị cá nhân.

2. **Truy cập Visual Web Builder (Studio):**
   - Click vào thẻ Landing Page `mxv_080_online` -> Bấm nút **"Code Studio"**.
   - Workspace toàn màn hình sẽ mở ra tại `https://crm.hanghoaphaisinh.net/studio.html?page_id=mxv_080_online`.
   - Có thể trực tiếp kéo dãn (Gizmo 8 handles), chỉnh sửa thuộc tính trong 6 tab Inspector, xem cây Layers, xem lịch sử Photoshop 50 bước.
   - Nhấn **"Lưu Code (Ctrl+S)"** để lưu trực tiếp vào CSDL PostgreSQL và cập nhật Live Cache tức thì.

3. **Kiểm tra Landing Page công khai:**
   - Truy cập: `https://crm.hanghoaphaisinh.net/p/mxv_080_online` hoặc `https://crm.hanghoaphaisinh.net/ladi.html`.
   - Điền form thử nghiệm để kiểm tra lead lưu về CRM, bắn về Google Sheets và cookie 180 ngày chuyển hướng sang `chucmung.html`.

---

## 🔒 GIẢI PHÁP SSL TỰ ĐỘNG CHO MỌI TÊN MIỀN TRỎ VỀ IP / NS (ON-DEMAND TLS)

Nếu hệ thống của bạn có hàng chục hoặc hàng trăm tên miền landing page được thêm động trong CRM, việc gõ lệnh `certbot` thủ công cho từng tên miền là bất khả thi.

### 🌟 GIẢI PHÁP TỐT NHẤT: CADDY SERVER VỚI ON-DEMAND TLS (BẬT LÀ CHẠY)

Caddy là máy chủ web hiện đại được viết bằng Golang (sử dụng thư viện `CertMagic`). Caddy có tính năng **On-Demand TLS** độc nhất vô nhị:
- Khi bất kỳ tên miền nào (vd: `dautuhanghoa.vn`, `domain-khach.com`) vừa trỏ về IP / NameServer của bạn, ngay lần đầu tiên có người truy cập qua HTTPS `https://domain-khach.com`, Caddy sẽ:
  1. Tự động gọi endpoint nội bộ `http://127.0.0.1:8080/api/internal/check-domain?domain=domain-khach.com`.
  2. Go Backend kiểm tra RAM Cache: Nếu tên miền này đã được bạn thêm trong Admin CRM -> Trả về `200 OK`.
  3. Caddy kết nối Let's Encrypt / ZeroSSL, xin cấp chứng chỉ SSL miễn phí chỉ trong 2 giây!
  4. Trình duyệt hiển thị ổ khóa xanh HTTPS bảo mật 100% mà bạn **không cần thao tác bất cứ dòng lệnh nào**!
  5. Nếu tên miền lạ của kẻ xấu trỏ bừa về IP -> Go Backend trả về `403` -> Caddy từ chối cấp SSL, bảo vệ tuyệt đối hạn mức rate-limit của bạn.

#### Hướng dẫn cài đặt Caddy thay thế Nginx (Chỉ 3 bước):
```bash
# 1. Cài đặt Caddy trên Ubuntu/Debian:
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy

# 2. Sử dụng file cấu hình Caddyfile có sẵn trong thư mục deploy/caddy/:
sudo cp /var/www/apex-landing/deploy/caddy/Caddyfile /etc/caddy/Caddyfile

# 3. Khởi động Caddy:
sudo systemctl reload caddy
```
Từ thời điểm này, **mọi tên miền** bạn thêm vào hệ thống và trỏ về server đều có **SSL HTTPS tự động 100%**!

