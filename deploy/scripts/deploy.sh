#!/bin/bash
# ==============================================================
# SCRIPT CÀI ĐẶT & CẬP NHẬT TỰ ĐỘNG APEX FINTECH TRÊN LINUX SERVER
# ==============================================================
set -e

echo "=========================================================="
echo "  APEX FINTECH - SERVER DEPLOYMENT & UPDATE SCRIPT"
echo "=========================================================="

APP_DIR="/var/www/apex-landing"

# 1. Kiểm tra quyền root
if [ "$EUID" -ne 0 ]; then
  echo "[LỖI] Vui lòng chạy script với quyền root: sudo bash deploy.sh"
  exit 1
fi

# 2. Tạo thư mục làm việc nếu chưa có
mkdir -p "$APP_DIR"
cd "$APP_DIR"

# 3. Tạo file .env nếu chưa tồn tại
if [ ! -f ".env" ]; then
  echo "[INFO] Tạo file .env mẫu từ .env.example..."
  if [ -f ".env.example" ]; then
    cp .env.example .env
  else
    cat <<EOF > .env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=apex_leads
DB_SSLMODE=disable
ALLOW_ORIGINS=*
ADMIN_KEY=apex080
EOF
  fi
  echo "[CHÚ Ý] Vui lòng chỉnh sửa file /var/www/apex-landing/.env theo thông tin CSDL của bạn!"
fi

# 4. Biên dịch mã nguồn Go cho Linux (nếu có Go cài đặt)
if command -v go &> /dev/null; then
  echo "[INFO] Đang biên dịch Go binary từ thư mục backend..."
  cd backend
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o "$APP_DIR/apex-server" .
  cd "$APP_DIR"
  chmod +x "$APP_DIR/apex-server"
  echo "[OK] Biên dịch thành công: $APP_DIR/apex-server"
else
  if [ -f "apex-server" ]; then
    chmod +x "apex-server"
    echo "[INFO] Đã tìm thấy binary apex-server có sẵn."
  else
    echo "[LỖI] Chưa có file apex-server và máy chủ chưa cài đặt Go. Hãy cài đặt Go hoặc upload binary đã build sẵn!"
    exit 1
  fi
fi

# 5. Cài đặt Systemd Service
if [ -f "deploy/systemd/apex.service" ]; then
  cp deploy/systemd/apex.service /etc/systemd/system/apex.service
  systemctl daemon-reload
  systemctl enable apex.service
  systemctl restart apex.service
  echo "[OK] Dịch vụ apex.service đã được khởi động!"
  systemctl status apex.service --no-pager || true
fi

echo "=========================================================="
echo "  [HOÀN TẤT] HỆ THỐNG ĐÃ SẴN SÀNG HOẠT ĐỘNG!"
echo "  Kiểm tra: curl http://localhost:8080/health"
echo "  Xem log: journalctl -u apex.service -f"
echo "=========================================================="
