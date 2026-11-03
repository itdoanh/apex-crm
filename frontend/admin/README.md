# HƯỚNG DẪN DEPLOY ADMIN DASHBOARD ĐỘC LẬP
Domain mục tiêu: `crm.hanghoaphaisinh.net/ads`

Thư mục này (`frontend/admin/`) chứa toàn bộ giao diện quản trị Admin CRM & Landing Analytics chạy độc lập, tách biệt 100% với backend.

---

## 1. Cấu Trúc Thư Mục (Kiến Trúc Module Tách Biệt)
```
frontend/admin/
├── index.html                 # Giao diện HTML Master (Sạch sẽ, chỉ chứa markup cấu trúc SPA)
├── config.js                  # Cấu hình API Backend Endpoint và môi trường
├── README.md                  # Hướng dẫn cấu hình & deploy
├── css/
│   └── admin.css              # Stylesheet chung: Scrollbar, Dark Glassmorphism, Gizmo Handles, Inspector
└── js/
    ├── app.js                 # Điều phối chung: Router chuyển tab chính, auto-refresh, global toast
    ├── auth.js                # Quản lý xác thực, token bearer, phiên đăng nhập & nhật ký kiểm toán (Audit Logs)
    ├── crm.js                 # Quản lý Leads CRM: Table view, Kanban board, Drawer 360, bộ lọc, xuất CSV
    ├── analytics.js           # Biểu đồ Chart.js: Traffic, thiết bị, nguồn, vị trí địa lý & Session Replay
    ├── security.js            # Phân hệ Bảo Mật & Anti-Bot: Thống kê tấn công, Honeypot & Vùng cách ly
    ├── sheets.js              # Đồng bộ Google Sheets đa bảng tính: Mapping cột, kiểm thử & nhật ký sync
    ├── landings.js            # Quản lý danh sách Landing Pages: Lưới thẻ, sao chép, xóa, modal cài đặt
    ├── domains.js             # Quản lý Tên Miền & DNS: Kiểm tra bản ghi, định tuyến đường dẫn, modal cấu hình
    └── studio/                # Phân Hệ Visual Web Builder & Ladi Studio:
        ├── ast-mapper.js      # ApexASTSourceMapper: Phân tích cú pháp AST không phá hủy mã nguồn
        ├── history.js         # Hệ thống Lịch sử Photoshop 50 bước: Undo, Redo & Dropdown Time-Travel
        ├── gizmo.js           # Khung điều khiển Gizmo: 8 chốt co giãn, kéo di chuyển, overlay isolation
        ├── inspector.js       # 6 Tab thuộc tính Inspector: Layout, Typography, Style, Spacing, Classes, Attributes
        ├── form-scanner.js    # Bộ quét Form tự động, ánh xạ trường input & tạo mã nhúng tracker
        ├── context-menu.js    # Menu chuột phải chuyên nghiệp 20 hành động (xuyên iframe, chống tràn mép)
        ├── layers.js          # Cây phân cấp Layer DOM: 7 nhóm trực quan, nhận diện Stacking Context (ABS/Z-Index)
        └── file-manager.js    # Quản lý tệp tin Studio, cây thư mục, đồng bộ code editor ↔ preview iframe
```

---

## 2. Cách Cấu Hình API Endpoint Cross-Domain
Khi Admin chạy trên domain `crm.hanghoaphaisinh.net/ads` và Backend API chạy trên `api.hanghoaphaisinh.net` hoặc server khác:

1. **Cách 1: Qua file `config.js`**:
   Sửa `defaultApiUrl` trong `frontend/admin/config.js`:
   ```javascript
   defaultApiUrl = 'https://api.hanghoaphaisinh.net';
   ```

2. **Cách 2: Cấu hình trực tiếp trên giao diện Admin**:
   Bấm vào badge **"● API Server"** ở góc trên thanh công cụ của Admin -> Nhập URL Backend API -> Bấm Lưu & Kết Nối. URL sẽ được lưu tự động vào `localStorage` của trình duyệt.

---

## 3. Cấu Hình Nginx Reverse Proxy / Web Server Cho `crm.hanghoaphaisinh.net/ads`

Nếu bạn dùng Nginx phục vụ thư mục `/ads`:

```nginx
server {
    server_name crm.hanghoaphaisinh.net;
    
    # Phục vụ Admin Frontend tại đường dẫn /ads
    location /ads {
        alias /var/www/apex/frontend/admin;
        index index.html;
        try_files $uri $uri/ /ads/index.html;
    }

    # Proxy các API call về Backend Go (nếu proxy chung domain)
    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
