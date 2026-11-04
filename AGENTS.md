# AGENTS.md — Apex Fintech Multi-Landing & CRM Engine

> **Bắt buộc đọc file này TRƯỚC KHI làm bất kỳ việc gì trong repo này.**

Hệ thống thu thập lead từ landing page đa tên miền cho Sở Giao dịch Hàng hóa Việt Nam (MXV 080), Bộ Công Thương.

---

## 1. Bản đồ repo trong 30 giây

```
  backend/          Go 1.26.3 API server (pgx/v5, PostgreSQL 16)
  main.go         Entry point + HTTP router + multi-domain edge router
  cache/          RAM cache: landing configs + domain routes
  config/         Đọc .env
  db/             pgx/v5 store + schema init
  handlers/       REST handlers (lead, auth, landing, domain, sheet, analytics)
  integrations/   Google Sheets API v4 (Master Service Account Bot)
  models/         Structs dữ liệu
  security/       PoW challenge, honeypot, ratelimit, geoip, argon2, phone
  static/         Bản mirror của frontend (KHÔNG sửa trực tiếp, sửa ở frontend/)
frontend/
  admin/          CRM SPA + Ladi Studio (nguồn chuẩn)
  landing/        Landing page mẫu + tracker.js + config.json
deploy/           Caddyfile, systemd, nginx, windows scripts
```

**Quan trọng:** `backend/static/` và `frontend/admin/` là **hai bản mirror của cùng một nội dung**. Khi sửa UI, sửa ở `frontend/admin/` rồi copy sang `backend/static/`.

---

## 2. Trước khi sửa code, hãy đọc

| Bạn định làm gì | Đọc file này |
|---|---|
| Bất kỳ thứ gì (bắt buộc) | `docs/ARCHITECTURE.md` |
| Viết Go handler / DB query | `docs/API.md` + `docs/DATABASE.md` |
| Sửa giao diện CRM / Studio | `docs/FRONTEND.md` |
| Sửa landing page / tracker | `docs/LANDING.md` |
| Đụng tới auth, PoW, honeypot, rate limit | `docs/SECURITY.md` |
| Deploy lên server | `docs/DEPLOYMENT.md` |
| Lỗi production, 502, mất lead | `docs/RUNBOOK.md` |
| Lần đầu vào dự án | `docs/ONBOARDING.md` |

Quy tắc chi tiết cho AI agent nằm ở `.cursor/rules/` — chúng được auto-load theo loại file bạn đang mở.

---

## 3. Lệnh thường dùng

```bash
# Build backend (Windows)
cd backend && build.bat              # → apex-backend.exe
cd backend && build_linux.bat        # → apex-server (Linux binary)

# Chạy local
cd backend && go run main.go         # http://localhost:8080

# Test E2E (Playwright/Python — cần server chạy sẵn)
node backend/tests/e2e_domain_management_test.js
python backend/tests/test_full_suite.py
```

---

## 4. Production — dữ liệu bất biến

| Hạng mục | Giá trị |
|---|---|
| Server | `103.124.95.232` |
| SSH | `ssh Administrator@103.124.95.232 -p 22899` (key: `~/.ssh/id_ed25519`) |
| Thư mục deploy | `C:\apex` (clone của `github.com/itdoanh/apex-crm.git`) |
| Admin CRM | `https://crm.hanghoaphaisinh.net/ads` |
| Database | `apex_leads` @ `localhost:5432` (user `postgres`) |
| Quản lý tiến trình | **Task Scheduler** (`Apex_Backend_Server`, `Apex_Caddy_Server`) — KHÔNG phải NSSM |
| Backend listen | `:8080` · Caddy `:80/:443` |

> 🚨 **Quy tắc vàng:** Sửa code ở local → commit → push → pull trên server → build → restart task.
> **TUYỆT ĐỐI KHÔNG sửa file trực tiếp trên server** bằng tay (trừ khi là emergency hotfix, và phải ghi lại).

---

## 5. Bất biến kỹ thuật (KHÔNG được phá)

1. **CORS mặc định `*`** — đổi thì landing domain khác sẽ gửi lead không được.
2. **Chuẩn số điện thoại VN 10 số** (`03x/05x/07x/08x/09x`) — `security/phone_validator.go`.
3. **Múi giờ hiển thị là `Asia/Ho_Chi_Minh`** — mọi query dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'`, không dùng `NOW()` trực tiếp cho so sánh ngày.
4. **`landing_pages` PK là `(domain, page_id)`** — KHÔNG phải `page_id` đơn lẻ. Sửa schema phải cập nhật cả `db/db.go` lẫn `cache/landing_cache.go`.
5. **Cache phải được `Reload()` sau khi ghi DB** — nếu không, thay đổi không hiện ra.
6. **Static files phục vụ qua đường dẫn tương đối** `../frontend/landing`, `../frontend/admin` → backend **phải** chạy với working directory là `C:\apex\backend` (giống Task Scheduler đang cấu hình).
7. **On-Demand TLS** dựa vào `/api/internal/check-domain` trả `200` cho domain đã đăng ký, `403` cho domain lạ. Endpoint này **phải** chạy được khi Caddy hỏi.

---

## 6. Bảo mật — không được bỏ qua

- `service-account.json`, `.env`, `.env.prod` **không được commit** (đã có trong `.gitignore`).
- Mật khẩu admin hash bằng **Argon2id** (`security/argon2.go`), không dùng MD5/SHA1/bcrypt.
- Session token lưu dạng **SHA-256 hash** trong DB, không lưu plaintext.
- Mọi endpoint admin phải bọc `authHandler.RequireAdminAuth(...)`.
- Endpoint public duy nhất: `POST /api/leads`, `GET /api/landing/config`, `POST /api/analytics/track`, `GET /api/auth/challenge`, `/api/internal/check-domain`, `/health`.

---

## 7. Quy ước commit

```
feat(scope): mô tả ngắn bằng tiếng Việt
fix(scope): mô tả ngắn bằng tiếng Việt
chore(scope): ...
```
Scope dùng: `studio`, `landings`, `domains`, `security`, `leads`, `sheets`, `admin`, `deploy`.

Ví dụ thực tế trong repo:
```
fix(studio): fix preview syntax error, persistent data-ve-id sync, domain deploy 400
feat(landings): add search-replace widget, live attribute sync, multi-file domain deploy
```

**Commit là công sức chung của `itdoanh` (chủ repo) + AI agent.** Git config đã đặt
`user.name = itdoanh` — AI không được tự đổi identity. Luôn kết thúc message bằng:

```
Co-authored-by: itdoanh <itdoanh@users.noreply.github.com>
Co-authored-by: Cursor <cursoragent@cursor.com>
```

---

## 8. Quy luật bắt buộc cho AI agent

Áp dụng cho **mọi** agent làm việc trong repo này — kể cả agent mở từ IDE khác.

### 8.1. Đọc trước — sửa sau — đồng bộ hóa

```
1. ĐỌC    → trước khi làm gì, đọc TẤT CẢ file liên quan (bảng mục 2)
2. LÀM    → sửa code / cấu hình
3. ĐỒNG BỘ → sau khi làm xong, cập nhật lại TẤT CẢ file liên quan cho khớp nhau
```

File liên quan cần đồng bộ (không giới hạn):

| Loại thay đổi | File phải đồng bộ |
|---|---|
| Đổi API | `docs/API.md` + `AGENTS.md` (nếu đổi public endpoint) |
| Đổi schema DB | `docs/DATABASE.md` + `backend/db/db.go` + `backend/cache/` |
| Đổi UI | `frontend/admin/` → copy sang `backend/static/` + `docs/FRONTEND.md` |
| Đổi landing/tracker | `docs/LANDING.md` + `docs/SECURITY.md` (nếu đụng anti-bot) |
| Đổi deploy/vận hành | `docs/DEPLOYMENT.md` + `docs/RUNBOOK.md` + `.cursor/rules/30-deploy.mdc` |
| Đổi quy trình | `AGENTS.md` + `CLAUDE.md` + `.cursorrules` + `.cursor/rules/*.mdc` + `CONTRIBUTING.md` |

> Đồng bộ tài liệu là **một phần của task**, không phải việc làm thêm cho sau.

### 8.2. AI không tự test vận hành

**AI được:** check lỗi code (`go build ./...`, `go vet ./...`, parse PowerShell),
khởi động/dựng lại service để production sẵn sàng, báo cáo trạng thái.

**AI không được** (trừ khi `itdoanh` nói rõ):
- Submit lead thật / đăng ký thử trên landing page.
- Test luồng end-user: đăng nhập CRM, tạo landing, deploy domain, đổi mật khẩu.
- Xoá/sửa dữ liệu nghiệp vụ chỉ để "kiểm tra xem có chạy không".

> Mỗi lần test lead là một lần ghi vào DB production + Google Sheets + KPI doanh thu.
> AI test "cho có" chỉ làm nhiễu dữ liệu thật mà không đem lại giá trị.

Báo cáo phải kết thúc bằng:
`🧪 CHƯA TEST vận hành — anh itdoanh vào kiểm tra giúp em`

### 8.3. Không tự ý sửa lan sang, không xoá dữ liệu

- Vấn đề ngoài phạm vi task → **báo cáo**, không tự ý sửa.
- Không xoá code/dữ liệu production mà không hỏi `itdoanh`.
- Deploy / restart production → **báo trước khi làm**.
