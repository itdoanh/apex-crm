# CLAUDE.md — Apex Fintech Multi-Landing & CRM Platform

> File này được Claude Code đọc tự động. **Đọc `AGENTS.md` trước khi làm bất kỳ việc gì.**

## Bắt buộc đọc

1. `AGENTS.md` — bản đồ repo + bất biến kỹ thuật
2. `docs/ARCHITECTURE.md` — kiến trúc 3 tầng
3. Tài liệu theo task:

| Task | Tài liệu |
|---|---|
| Go handler / DB | `docs/API.md`, `docs/DATABASE.md` |
| UI CRM / Studio | `docs/FRONTEND.md` |
| Landing / tracker | `docs/LANDING.md` |
| Auth / anti-bot | `docs/SECURITY.md` |
| Deploy / server | `docs/DEPLOYMENT.md` |
| Sự cố production | `docs/RUNBOOK.md` |
| Lần đầu dự án | `docs/ONBOARDING.md` |

## Bất biến kỹ thuật — KHÔNG được phá

1. CORS mặc định `ALLOW_ORIGINS=*`
2. SĐT Việt Nam 10 số (`03x/05x/07x/08x/09x`)
3. Mọi query thời gian dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'` — **không** dùng `NOW()` trực tiếp cho so sánh ngày
4. `landing_pages` PK composite `(domain, page_id)`
5. Ghi DB → **phải** `Reload()` cache (`landingCache` / `domainCache`)
6. Backend chạy với working directory `C:\apex\backend`
7. `/api/internal/check-domain` trả `200` (domain đã đăng ký) / `403` (lạ) — Caddy On-Demand TLS phụ thuộc
8. `backend/static/` là **mirror** của `frontend/admin/` — sửa ở `frontend/admin/` rồi copy sang
9. Không phá logic autofill-safe trong `tracker.js`
10. Không xoá field honeypot khỏi DOM

## Production

```bash
ssh Administrator@103.124.95.232 -p 22899
```
- Code: `C:\apex` · Binary: `C:\apex\backend\apex-server.exe`
- Quản lý tiến trình: **Task Scheduler** (`Apex_Backend_Server`, `Apex_Caddy_Server`) — **KHÔNG phải** NSSM
- Deploy: sửa local → test → commit → push → `git pull` server → build → restart task → verify `/health`
- 🚨 **Không sửa file trực tiếp trên server** — mọi thay đổi qua git để rollback được

## Quy ước

- Ngôn ngữ trao đổi: **tiếng Việt**
- Commit: `feat(scope):` / `fix(scope):` / `chore(scope):` — scope: `studio` `landings` `domains` `security` `leads` `sheets` `admin` `deploy`
- **Commit chung với `itdoanh`** — luôn kết thúc message bằng:
  ```
  Co-authored-by: itdoanh <itdoanh@users.noreply.github.com>
  Co-authored-by: Cursor <cursoragent@cursor.com>
  ```
- Build: `cd backend && build.bat`
- Cập nhật docs liên quan khi thay đổi API/schema/quy trình

## Quy luật bắt buộc cho AI agent

### Đọc trước — sửa sau — đồng bộ

1. **Trước khi làm gì cũng phải đọc các file liên quan** (bảng trên).
2. **Làm xong phải cập nhật lại và đồng bộ với các file liên quan** — `docs/`, `AGENTS.md`,
   `CLAUDE.md`, `.cursorrules`, `.cursor/rules/*.mdc`, `CONTRIBUTING.md`, `.env.example`.
   Đồng bộ tài liệu là **một phần của task**, không phải việc làm thêm cho sau.

### AI không tự test vận hành

- **Được**: check lỗi code (`go build ./...`, `go vet ./...`, parse PowerShell), khởi động
  lại service, báo cáo trạng thái.
- **Không được** (trừ khi `itdoanh` nói rõ): submit lead thật, test luồng end-user
  (đăng nhập CRM, tạo landing, deploy domain, đổi mật khẩu), sửa dữ liệu nghiệp vụ
  chỉ để "kiểm tra".

> Mỗi lần test lead là một lần ghi vào DB production + Google Sheets + KPI doanh thu.
> AI test "cho có" chỉ làm nhiễu dữ liệu thật.

Báo cáo phải kết thúc bằng:
`🧪 CHƯA TEST vận hành — anh itdoanh vào kiểm tra giúp em`

### Không tự ý sửa lan sang

- Vấn đề ngoài phạm vi task → **báo cáo**, không tự sửa.
- Không xoá code/dữ liệu production mà không hỏi `itdoanh`.
- Deploy / restart production → **báo trước khi làm**.
