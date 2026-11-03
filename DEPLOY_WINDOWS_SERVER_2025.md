# 🪟 HƯỚNG DẪN TRIỂN KHAI APEX FINTECH LÊN WINDOWS SERVER 2025 DATACENTER
### (Tối ưu hóa toàn diện cho máy chủ ổ đĩa ~30GB & Caddy On-Demand TLS)

Tài liệu này cung cấp quy trình chuẩn bị, cài đặt, cấu hình tự động hóa và bảo mật đưa hệ thống **Apex Fintech Multi-Landing & Enterprise CRM** lên hệ điều hành **Windows Server 2025 Datacenter Evaluation**.

---

## 📋 THÔNG TIN HỆ THỐNG MẪU

- **Hệ điều hành:** Windows Server 2025 Datacenter Evaluation (x64)
- **Dung lượng ổ cứng khả dụng:** ~30GB *(Đã tối ưu hóa toàn diện, hệ thống chỉ chiếm <1GB)*
- **Địa chỉ IP máy chủ:** `103.124.95.232` *(IP chính thức máy chủ)*
- **Tên miền quản trị Admin/CRM:** `https://crm.hanghoaphaisinh.net/ads`
- **NameServers hệ thống:**
  - `ns1.hanghoaphaisinh.net` ➔ `103.124.95.232`
  - `ns2.hanghoaphaisinh.net` ➔ `103.124.95.232`
- **Tài khoản quản trị tối cao duy nhất:**
  - **Username:** `admin`
  - **Mật khẩu ban đầu:** `ApexAdmin@2026!` *(Băm Argon2id - đổi ngay sau khi đăng nhập)*
- **Landing Page chuẩn ban đầu:** `mxv_080_online` *(Đầy đủ HTML chuẩn Bộ Công Thương & MXV 080)*

---

## 🌐 BƯỚC 1: CẤU HÌNH BẢN GHI DNS TẠI NHÀ CUNG CẤP TÊN MIỀN

Đăng nhập vào trang quản trị DNS của tên miền `hanghoaphaisinh.net` (Cloudflare, Tenten, PA Vietnam, v.v.) và thêm các bản ghi:

| Loại (Type) | Tên (Host / Name) | Giá Trị (Points to) | Giải thích |
| :--- | :--- | :--- | :--- |
| **A** | `crm` | `103.124.95.232` | Tên miền Admin CRM (`crm.hanghoaphaisinh.net`) |
| **A** | `ns1` | `103.124.95.232` | NameServer 1 phục vụ trỏ tên miền phụ |
| **A** | `ns2` | `103.124.95.232` | NameServer 2 phục vụ trỏ tên miền phụ |
| **A** | `@` | `103.124.95.232` | Landing page chính |
| **A** | `*` (Wildcard) | `103.124.95.232` | Đón tất cả subdomain landing tự động |

> **Đối với bất kỳ tên miền nào khác (ví dụ: `dautuhanghoa.vn`):**
> Chỉ cần trỏ bản ghi **A** về `103.124.95.232` (hoặc đổi cụm NameServer sang `ns1`/`ns2.hanghoaphaisinh.net`).

---

## 📁 BƯỚC 2: SAO CHÉP MÃ NGUỒN VÀO WINDOWS SERVER 2025

1. Đăng nhập Remote Desktop (RDP) vào máy chủ Windows Server 2025.
2. Tạo thư mục làm việc: `C:\apex-landing`
3. Sao chép toàn bộ mã nguồn dự án vào `C:\apex-landing`:
   ```text
   C:\apex-landing\
   ├── backend\
   │   ├── apex-server.exe     <-- Binary Go sản xuất siêu nhẹ (chỉ ~22MB)
   │   ├── config\
   │   ├── db\
   │   ├── handlers\
   │   └── static\
   ├── frontend\
   │   ├── admin\              <-- CRM Core & Studio Modules
   │   └── landing\            <-- Landing page gốc ladi.html & assets
   ├── deploy\
   │   ├── clean_test_data.sql <-- Script dọn sạch dữ liệu test
   │   └── windows\
   │       ├── Caddyfile
   │       ├── install_caddy.ps1
   │       ├── setup_services.ps1
   │       ├── setup_firewall.ps1
   │       ├── cleanup_disk_space.ps1
   │       ├── start_all.bat
   │       ├── stop_all.bat
   │       └── restart_all.bat
   └── .env.example
   ```
4. Đổi tên file `C:\apex-landing\.env.example` thành `C:\apex-landing\.env` (hoặc đặt cạnh `backend\apex-server.exe`).

---

## 🗄️ BƯỚC 3: CÀI ĐẶT CSDL POSTGRESQL NATIVE (TIẾT KIỆM Ổ ĐĨA 30GB)

> 💡 **TẠI SAO DÙNG POSTGRESQL NATIVE THAY VÌ DOCKER DESKTOP TRÊN WINDOWS SERVER?**
> - **Docker Desktop:** Yêu cầu cài WSL2 hoặc Hyper-V VM, tạo file ổ đĩa ảo `ext4.vhdx` ngốn ngay **10GB - 15GB** (chiếm 50% tổng dung lượng 30GB còn lại của bạn!).
> - **PostgreSQL Native Windows:** Cài trực tiếp thành Windows Service, chỉ tốn **~200MB ổ đĩa** và **~50MB RAM**, hoạt động ổn định 24/7.

### Cách thực hiện (Chỉ 2 bước):

1. **Tải và cài đặt PostgreSQL 16 cho Windows:**
   - Mở trình duyệt trên máy chủ tải từ trang chính thức: [EnterpriseDB PostgreSQL Downloads](https://www.enterprisedb.com/downloads/postgres-postgresql-downloads).
   - Chọn bản **PostgreSQL 16 (Windows x86-64)**.
   - Chạy trình cài đặt:
     - Thư mục cài đặt: Để mặc định (`C:\Program Files\PostgreSQL\16`).
     - Mật khẩu tài khoản `postgres`: Đặt là `postgres` *(khớp với cấu hình .env)*.
     - Cổng (Port): Để mặc định `5432`.
   - Hoàn tất cài đặt (bỏ tích chọn Stack Builder).

2. **Khởi tạo Database và Nạp Dữ Liệu Sạch (Clean Seed):**
   Mở **PowerShell (Run as Administrator)** và chạy:
   ```powershell
   # Tạo database apex_leads
   $env:PGPASSWORD = "postgres"
   & "C:\Program Files\PostgreSQL\16\bin\psql.exe" -U postgres -h localhost -c "CREATE DATABASE apex_leads;"

   # Nạp dữ liệu sạch và tối ưu hóa
   & "C:\Program Files\PostgreSQL\16\bin\psql.exe" -U postgres -h localhost -d apex_leads -f "C:\apex-landing\deploy\clean_test_data.sql"
   ```

---

## 🚀 BƯỚC 4: CÀI ĐẶT CADDY WEB SERVER (ON-DEMAND TLS)

Caddy là web server hiện đại, tự động cấp chứng chỉ SSL miễn phí (Let's Encrypt / ZeroSSL) cho **MỌI TÊN MIỀN** trỏ về server thông qua On-Demand TLS.

Mở **PowerShell (Run as Administrator)** và chạy lệnh:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process -Force
cd C:\apex-landing\deploy\windows
.\install_caddy.ps1
```

*Kịch bản sẽ tự động:*
- Tải `caddy.exe` mới nhất bản Windows x64.
- Đặt tại `C:\caddy\caddy.exe` và tạo thư mục logs `C:\caddy\logs`.
- Sao chép `Caddyfile` đã cấu hình On-Demand TLS và Log Rotation.
- Thêm `C:\caddy` vào biến môi trường PATH của hệ thống.
- Xác thực cú pháp cấu hình thành công 100%.

---

## 🛡️ BƯỚC 5: CẤU HÌNH TƯỜNG LỬA WINDOWS DEFENDER FIREWALL

Bảo vệ máy chủ tối đa: Mở cổng web công khai, chặn truy cập ngoài vào cổng nội bộ.

Trong PowerShell Administrator, chạy:
```powershell
cd C:\apex-landing\deploy\windows
.\setup_firewall.ps1
```

*Kết quả bảo mật:*
- ✅ **Cổng 80 (HTTP):** Mở Inbound cho tất cả để phục vụ chuyển hướng HTTPS và xác thực ACME Let's Encrypt.
- ✅ **Cổng 443 (HTTPS):** Mở Inbound cho tất cả để phục vụ Admin CRM và Landing Pages an toàn.
- 🔒 **Cổng 8080 (Go Backend):** Khóa chặn toàn bộ kết nối bên ngoài, chỉ cho phép Localhost (127.0.0.1) từ Caddy Reverse Proxy.
- 🔒 **Cổng 5432 (PostgreSQL):** Khóa chặn toàn bộ kết nối bên ngoài, ngăn chặn mọi cuộc tấn công dò quét CSDL.

---

## ⚙️ BƯỚC 6: CÀI ĐẶT WINDOWS SERVICES TỰ KHỞI ĐỘNG CÙNG MÁY CHỦ

Để hệ thống hoạt động liên tục 24/7, tự khởi động lại khi máy chủ restart hoặc khi gặp sự cố, ta thiết lập 2 Windows Services chính thức:
1. `ApexBackend` (Chạy `apex-server.exe`)
2. `ApexCaddy` (Chạy `caddy.exe`)

Trong PowerShell Administrator, chạy:
```powershell
cd C:\apex-landing\deploy\windows
.\setup_services.ps1
```

*Kịch bản tự động tải NSSM (chỉ 300KB), đăng ký cả 2 dịch vụ ở chế độ `Automatic`, thiết lập tự khởi động lại sau 5 giây nếu gặp sự cố và kích hoạt tính năng Xoay vòng file Log (Log Rotation 10MB).*

### Các phím tắt quản trị nhanh 1-Click:
Trong thư mục `C:\apex-landing\deploy\windows\`:
- `start_all.bat`: Khởi động toàn bộ hệ thống.
- `stop_all.bat`: Dừng toàn bộ hệ thống.
- `restart_all.bat`: Khởi động lại toàn bộ hệ thống sau khi cập nhật code hoặc sửa cấu hình.

---

## 🧹 BƯỚC 7: TỐI ƯU HÓA VÀ DỌN DẸP ĐỊNH KỲ Ổ ĐĨA 30GB

Để bảo đảm phân vùng đĩa ~30GB luôn có từ **25GB - 28GB trống**, kịch bản `cleanup_disk_space.ps1` sẽ tự động dọn dẹp các tệp rác hệ thống:

```powershell
cd C:\apex-landing\deploy\windows
.\cleanup_disk_space.ps1
```

### Thiết lập tự động dọn dẹp hàng tuần qua Task Scheduler:
Chạy lệnh PowerShell sau để máy chủ tự động dọn đĩa vào 03:00 sáng Chủ Nhật hàng tuần:
```powershell
$action = New-ScheduledTaskAction -Execute "PowerShell.exe" -Argument "-NoProfile -ExecutionPolicy Bypass -File C:\apex-landing\deploy\windows\cleanup_disk_space.ps1"
$trigger = New-ScheduledTaskTrigger -Weekly -DaysOfWeek Sunday -At 3am
Register-ScheduledTask -TaskName "Apex_Weekly_Disk_Cleanup" -Action $action -Trigger $trigger -User "NT AUTHORITY\SYSTEM" -RunLevel Highest
```

---

## ✅ BƯỚC 8: NGHIỆM THU VÀ KIỂM TRA HỆ THỐNG THỰC TẾ

### 1. Đăng nhập Admin CRM:
- Mở trình duyệt truy cập: `https://crm.hanghoaphaisinh.net/ads`
- Đăng nhập:
  - **Username:** `admin`
  - **Password:** `ApexAdmin@2026!`
- Ngay sau khi đăng nhập: Nhấn vào biểu tượng Hồ Sơ ở góc phải trên để đổi mật khẩu quản trị cá nhân.

### 2. Kiểm tra Landing Page:
- Truy cập: `https://crm.hanghoaphaisinh.net/p/mxv_080_online` hoặc `https://dautuhanghoa.vn/`
- Trang web tải nhanh siêu tốc, hiển thị ổ khóa xanh HTTPS bảo mật 100%.
- Thử điền form để kiểm tra lưu Lead về CRM và chuyển hướng sang `chucmung.html`.

### 3. Kiểm tra cấp SSL On-Demand cho Tên Miền Mới Bất Kỳ:
- Vào mục **Quản Lý Tên Miền** trong Admin CRM.
- Nhập một tên miền mới (ví dụ: `khoahocmoi.vn`).
- Trỏ bản ghi A của `khoahocmoi.vn` về IP `103.124.95.232`.
- Mở trình duyệt vào `https://khoahocmoi.vn`:
  - Caddy tự động gọi Go Backend thẩm định: Domain hợp lệ ➔ Let's Encrypt cấp SSL trong **2 giây**!
  - Trang web hiển thị ngay lập tức với chứng chỉ SSL bảo mật mà không cần gõ bất kỳ dòng lệnh nào!

### 4. Kiểm tra Tự Động Đồng Bộ Google Sheets (Email Bot):
- Vào tab **Đồng Bộ Google Sheets** trong Admin CRM.
- Hệ thống hiển thị Email Bot Service Account sẵn sàng:
  ```text
  sheet-bot@rinva-drive-tool.iam.gserviceaccount.com
  ```
- Bấm nút **Copy** để sao chép email bot.
- Mở bảng tính Google Sheet (Private) của bạn ➔ Bấm nút **Chia sẻ (Share)** ở góc phải ➔ Dán email bot và chọn quyền **Người chỉnh sửa (Editor)** ➔ Bấm **Gửi**.
- Bấm **Thêm Kết Nối Google Sheet Mới** trên CRM ➔ Dán link Google Sheet ➔ Bấm **Kết Nối & Tải Cấu Trúc**:
  - Hệ thống tự động phân tích URL, trích xuất Spreadsheet ID và nạp danh sách các Tab (Trang tính), số dòng x số cột tức thì!
- Bấm **Gửi Dòng Thử Nghiệm Ngay** để kiểm tra ghi dữ liệu ➔ Bấm **Lưu Cấu Hình Kết Nối**.
- Kể từ thời điểm này, mỗi khi có khách đăng ký mới trên Landing Page, hệ thống tự động đẩy dữ liệu sang Google Sheet theo thời gian thực mà người dùng không cần đăng nhập Google!

---

## 🔍 XỬ LÝ SỰ CỐ NHANH (TROUBLESHOOTING)

| Vấn đề | Nguyên nhân | Cách khắc phục |
| :--- | :--- | :--- |
| **Không mở được trang web qua cổng 80/443** | Chưa chạy Caddy hoặc tường lửa đang chặn | Mở PowerShell Administrator, chạy `sc query ApexCaddy` và chạy lại `.\setup_firewall.ps1`. |
| **Lỗi 502 Bad Gateway khi truy cập** | ApexBackend chưa chạy | Chạy `restart_all.bat` hoặc kiểm tra log tại `C:\apex-landing\backend\logs\apex-server_err.log`. |
| **Không kết nối được CSDL PostgreSQL** | Service PostgreSQL chưa bật hoặc sai mật khẩu | Mở Services (`services.msc`), đảm bảo `postgresql-x64-16` đang ở trạng thái `Running`. |
| **Tên miền mới trỏ về nhưng bị lỗi SSL** | Tên miền chưa được thêm vào CRM hoặc DNS chưa lan truyền | Vào Admin CRM thêm tên miền vào danh sách và kiểm tra DNS bằng nút "Kiểm tra DNS". |
| **Lỗi Google Sheet 403 / 404 khi kết nối** | Chưa cấp quyền Editor cho Email Bot | Mở Google Sheet ➔ Bấm Chia sẻ ➔ Thêm email `sheet-bot@rinva-drive-tool.iam.gserviceaccount.com` với quyền Editor. |
