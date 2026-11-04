# Onboarding — Bắt đầu từ đâu

Checklist cho người **mới** tham gia dự án, và cho AI agent lần đầu đọc repo.

---

## 1. Đọc theo thứ tự (30–60 phút)

| # | Tài liệu | Đọc khi nào |
|---|---|---|
| 1 | `AGENTS.md` | Luôn — bản đồ repo + bất biến kỹ thuật |
| 2 | `docs/ARCHITECTURE.md` | Hiểu hệ thống chạy như thế nào |
| 3 | `docs/DATABASE.md` | Trước khi đụng DB |
| 4 | `docs/API.md` | Trước khi thêm/sửa endpoint |
| 5 | `docs/FRONTEND.md` | Trước khi sửa UI |
| 6 | `docs/LANDING.md` | Trước khi sửa landing/tracker |
| 7 | `docs/SECURITY.md` | Trước khi đụng auth/anti-bot |
| 8 | `docs/DEPLOYMENT.md` | Trước khi deploy |
| 9 | `docs/RUNBOOK.md` | Khi có sự cố production |

---

## 2. Setup môi trường local

### Yêu cầu

| Công cụ | Phiên bản | Ghi chú |
|---|---|---|
| Go | 1.26.3+ | `backend/go.mod` khai báo `go 1.26.3` |
| PostgreSQL | 15 hoặc 16 | Cần extension `JSONB` + index `GIN` |
| Browser | Chrome/Edge/Safari/Firefox | DevTools dùng nhiều |

### Cài đặt

```bash
# 1. Clone
git clone https://github.com/itdoanh/apex-crm.git
cd apex-crm

# 2. Cấu hình
cp .env.example .env
#   Sửa DB_PASSWORD nếu cần

# 3. Tạo database
psql -U postgres -c "CREATE DATABASE apex_leads;"

# 4. Chạy backend
cd backend
go run main.go
```

### Truy cập

| URL | Nội dung |
|---|---|
| `http://localhost:8080/` | Redirect → /admin |
| `http://localhost:8080/ads` | CRM Admin |
| `http://localhost:8080/admin` | CRM Admin (alias) |
| `http://localhost:8080/ladi.html` | Landing mẫu |
| `http://localhost:8080/p/mxv_080_online` | Landing theo page_id |
| `http://localhost:8080/health` | Health check |

**Đăng nhập:** `admin` / `ApexAdmin@2026!` → **đổi mật khẩu ngay** trong UI.

> 💡 Schema tự tạo khi backend khởi động (`initSchema` với `CREATE TABLE IF NOT EXISTS`). Không cần chạy SQL migration thủ công.

---

## 3. Điều hướng codebase

```bash
# Tìm handler của một endpoint
grep -rn "HandleSubmitLead" backend/handlers/

# Tìm nơi sửa HTML landing
grep -rn "landing_pages" backend/db/

# Tìm code frontend tương ứng
grep -rn "api/leads" frontend/admin/js/

# Xem tất cả route đăng ký
grep -n "mux.HandleFunc" backend/main.go
```

### Điểm vào theo tác vụ

| Tôi muốn... | Xem file |
|---|---|
| Thêm API endpoint | `backend/main.go` (mux) → `backend/handlers/` → `backend/db/` |
| Đổi cấu trúc DB | `backend/db/db.go` (`initSchema`) + `backend/models/` |
| Sửa giao diện CRM | `frontend/admin/index.html` + `frontend/admin/js/` → copy sang `backend/static/` |
| Sửa Studio | `frontend/admin/studio.html` + `frontend/admin/js/studio/` |
| Sửa form landing | `frontend/landing/ladi.html` + `frontend/landing/tracker.js` |
| Đổi anti-bot | `backend/security/` |
| Đổi Google Sheets | `backend/integrations/googlesheets/` |
| Đổi routing domain | `backend/cache/domain_cache.go` + `backend/db/domains.go` |

---

## 4. Quy trình làm việc chuẩn

```
1. Đọc tài liệu liên quan (mục 1)
2. Tạo branch: git checkout -b feat/ten-task
3. Sửa code
4. Build: cd backend && build.bat
5. Test local (E2E nếu có)
6. Commit: git commit -m "feat(scope): mô tả tiếng Việt"
7. Push + PR
8. Deploy: docs/DEPLOYMENT.md mục 1
9. Verify: /health + kiểm tra log Caddy
```

### Quy tắc commit
```
feat(scope): mô tả
fix(scope): mô tả
chore(scope): mô tả
```
Scope: `studio` `landings` `domains` `security` `leads` `sheets` `admin` `deploy`

**Commit chung với `itdoanh`** — luôn kết thúc message bằng:
```
Co-authored-by: itdoanh <itdoanh@users.noreply.github.com>
Co-authored-by: Cursor <cursoragent@cursor.com>
```

### Ba quy luật bắt buộc cho AI agent

**1. Đọc trước — sửa sau — đồng bộ**

- Trước khi làm gì cũng phải đọc các file liên quan (mục 1).
- Làm xong phải cập nhật lại và đồng bộ với các file liên quan: `docs/`, `AGENTS.md`,
  `CLAUDE.md`, `.cursorrules`, `.cursor/rules/*.mdc`, `CONTRIBUTING.md`, `.env.example`.
  Đồng bộ tài liệu là **một phần của task**, không phải việc làm thêm cho sau.

**2. AI không tự test vận hành**

- *Được*: check lỗi code (`go build ./...`, `go vet ./...`, parse PowerShell), khởi động
  lại service, báo cáo trạng thái.
- *Không được* (trừ khi `itdoanh` nói rõ): submit lead thật, test luồng end-user
  (đăng nhập CRM, tạo landing, deploy domain, đổi mật khẩu), sửa dữ liệu nghiệp vụ
  chỉ để "kiểm tra xem có chạy không".
- Báo cáo phải kết thúc bằng: `🧪 CHƯA TEST vận hành — anh itdoanh vào kiểm tra giúp em`

**3. Không tự ý sửa lan sang**

- Vấn đề ngoài phạm vi task → báo cáo, không tự sửa.
- Không xoá code/dữ liệu production mà không hỏi `itdoanh`.
- Deploy / restart production → báo trước khi làm.

---

## 5. Bất biến — không được phá

1. CORS `ALLOW_ORIGINS=*` mặc định.
2. SĐT VN 10 số (`03x/05x/07x/08x/09x`).
3. Múi giờ `Asia/Ho_Chi_Minh` cho mọi so sánh ngày/giờ.
4. `landing_pages` PK = `(domain, page_id)`.
5. Ghi DB → phải `Reload()` cache.
6. Backend chạy với WD = `C:\apex\backend` (production).
7. `/api/internal/check-domain` phải phân biệt domain đã đăng ký (200) vs lạ (403).

Chi tiết: `AGENTS.md` mục 5.

---

## 6. Kiến thức nền tảng cần có

### Bắt buộc
- Go: goroutine, context, error handling, `net/http`
- PostgreSQL: JSONB, index, parameterized query
- HTTP: REST, status code, CORS, cookie
- JavaScript ES6+: async/await, Promise, DOM API

### Nên có
- Docker (compose cho môi trường dev)
- Nginx/Caddy (reverse proxy, TLS)
- Windows Server / Task Scheduler / NSSM
- Chart.js, Alpine.js, Tailwind CSS
- Google Sheets API v4 + Service Account
- Playwright / Python (test E2E)

### Kiến thức ngành
- Hệ thống CRM: pipeline lead, trạng thái, phân quyền
- Quảng cáo Facebook/TikTok: UTM, ad placement, chuyển đổi
- Anti-bot: proof-of-work, honeypot, rate limiting, device fingerprint
- GeoIP & nhà mạng Việt Nam

---

## 7. Môi trường production (chỉ đọc, không sửa tay)

```bash
ssh Administrator@103.124.95.232 -p 22899
```

| Hạng mục | Giá trị |
|---|---|
| Code | `C:\apex` (git clone) |
| Binary | `C:\apex\backend\apex-server.exe` |
| Caddy | `C:\caddy\` (v2.8.4 — không dùng `lb_try_count`) |
| PostgreSQL | `C:\pgsql\bin\psql.exe` |
| Logs | `C:\caddy\logs\` + `C:\apex\backend\logs\server.log` |
| Backup DB | `C:\backup\apex_leads\` (task hằng ngày 02:00) |
| Task | `Apex_Backend_Server`, `Apex_Caddy_Server`, `Apex_DB_Backup`, `Apex_Disk_Cleanup` |

> ⚠️ **Server KHÔNG có Go toolchain.** Build binary ở LOCAL rồi `scp` lên server.
> Chi tiết: `docs/DEPLOYMENT.md` mục 1.1.

Chi tiết: `docs/DEPLOYMENT.md`.

> 🚨 Không sửa file trực tiếp trên production. Mọi thay đổi qua git.

---

## 8. Tài liệu cũ (root repo)

| File | Nội dung |
|---|---|
| `README.md` | Tổng quan tính năng, kiến trúc, ma trận 14 trường Sheets |
| `SECURITY_PLAN.md` | Kế hoạch bảo mật đầy đủ |
| `DEPLOY_WINDOWS_SERVER_2025.md` | Deploy Windows chi tiết |
| `DEPLOY_SERVER_GUIDE.md` | Deploy Ubuntu/Docker/Nginx |
| `backend/README.md` | Tài liệu backend gốc |
| `frontend/landing/HUONG_DAN_LANDING_FORM_TRACKING.md` | Hướng dẫn form tracking |

---

## 9. Danh sách kiểm tra định kỳ

### Hàng tuần
- [ ] Disk C: còn > 20GB trống
- [ ] Log Caddy không vượt 10MB/10MB
- [ ] Không có 502 mới trong log Caddy
- [ ] `spam_quarantine` không tăng bất thường
- [ ] `google_sheet_sync_logs` không có `status='error'`

### Hàng tháng
- [ ] Đổi mật khẩu admin nếu có nhân sự mới
- [ ] Review audit log — phát hiện hành vi bất thường
- [ ] Backup DB tự động còn hoạt động
- [ ] Cập nhật dependency Go (`go get -u`)
- [ ] Kiểm tra SSL các domain sắp hết hạn
