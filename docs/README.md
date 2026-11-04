# 📚 Tài liệu dự án Apex Fintech

> **Đây là điểm khởi đầu cho mọi người (kể cả AI agent) làm việc với dự án này.**
> Đọc `AGENTS.md` trước tiên.

---

## 🚀 Bắt đầu nhanh

| Bạn là | Đọc |
|---|---|
| **Người mới / AI agent lần đầu** | [`AGENTS.md`](AGENTS.md) → [`docs/ONBOARDING.md`](docs/ONBOARDING.md) |
| **Người đóng góp** | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| **AI agent (Cursor)** | Rules tự động load từ `.cursor/rules/`. Skills trong `.cursor/skills/` |
| **AI agent (Claude Code)** | [`CLAUDE.md`](CLAUDE.md) |
| **AI agent (Cursor legacy)** | [`.cursorrules`](.cursorrules) |
| **Dev backend** | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) → [`docs/API.md`](docs/API.md) → [`docs/DATABASE.md`](docs/DATABASE.md) |
| **Dev frontend** | [`docs/FRONTEND.md`](docs/FRONTEND.md) → [`docs/LANDING.md`](docs/LANDING.md) |
| **Dev/DevOps** | [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) → [`docs/RUNBOOK.md`](docs/RUNBOOK.md) |
| **Security review** | [`docs/SECURITY.md`](docs/SECURITY.md) |

---

## 📁 Cấu trúc tài liệu

```
AGENTS.md                      ← Entry point chính (bắt buộc đọc)
CLAUDE.md                      ← Claude Code pointer
.cursorrules                   ← Cursor legacy rules pointer
docs/
├── ARCHITECTURE.md            ← Kiến trúc 3 tầng, luồng dữ liệu, cache
├── DATABASE.md                ← 12 bảng, schema, index, quy tắc SQL
├── API.md                     ← Toàn bộ REST endpoints + examples
├── FRONTEND.md                ← CRM SPA, Ladi Studio, tracker.js
├── LANDING.md                 ← Hướng dẫn vận hành landing page
├── SECURITY.md                ← PoW, honeypot, rate limit, Argon2id, auth
├── DEPLOYMENT.md              ← Quy trình deploy, Windows Server, firewall
├── RUNBOOK.md                 ← Xử lý sự cố production
└── ONBOARDING.md              ← Checklist cho người mới
.cursor/
├── rules/                     ← Cursor rules (auto-load theo file type)
│   ├── 00-core-project-rules.mdc   ← alwaysApply — bất biến kỹ thuật
│   ├── 10-go-backend.mdc            ← Go conventions
│   ├── 20-frontend.mdc              ← JS/HTML conventions
│   ├── 30-deploy.mdc                ← Deploy & ops
│   ├── 40-security.mdc              ← Security conventions
│   └── 50-workflow.mdc              ← Quy trình làm việc
└── skills/                    ← Cursor Agent Skills (workflow chuẩn hoá)
    ├── deploy-production/SKILL.md
    ├── add-api-endpoint/SKILL.md
    ├── manage-domains-and-landings/SKILL.md
    ├── debug-production-issue/SKILL.md
    └── modify-landing-tracker/SKILL.md
```

---

## 🎯 Dự án này là gì

**Apex Fintech Multi-Landing & CRM Platform** — hệ thống thu thập lead từ landing page đa tên miền cho Sở Giao dịch Hàng hóa Việt Nam (MXV 080), Bộ Công Thương.

| Tầng | Công nghệ |
|---|---|
| Reverse proxy / TLS | Caddy (On-Demand TLS, cấp chứng chỉ ~2s) |
| Backend | Go 1.26+ · `net/http` · `pgx/v5` |
| Database | PostgreSQL 16 (12 bảng, JSONB tracking, GIN index) |
| Frontend | Alpine.js + Tailwind + Chart.js + Ladi Studio (không build step) |
| Anti-bot | Proof-of-Work SHA-256 + Honeypot + Rate limit + Behavioral |
| Tích hợp | Google Sheets API v4 (Master Service Account Bot) |

---

## ⚠️ Bất biến cốt lõi (đọc trước khi sửa code)

1. CORS mặc định `ALLOW_ORIGINS=*`
2. SĐT Việt Nam 10 số (`03x/05x/07x/08x/09x`)
3. Múi giờ hiển thị `Asia/Ho_Chi_Minh` — mọi query dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'`
4. `landing_pages` PK composite `(domain, page_id)`
5. Ghi DB → **phải** `Reload()` cache
6. Backend chạy với working directory `C:\apex\backend`
7. `/api/internal/check-domain` trả `200` (đã đăng ký) / `403` (lạ) — Caddy phụ thuộc
8. `backend/static/` là **mirror** của `frontend/admin/`

Chi tiết: [`AGENTS.md`](AGENTS.md) mục 5.

---

## 🚀 Quick start

```bash
git clone https://github.com/itdoanh/apex-crm.git
cd apex-crm
cp .env.example .env
cd backend && go run main.go
```

| URL | Nội dung |
|---|---|
| `http://localhost:8080/ads` | CRM Admin |
| `http://localhost:8080/ladi.html` | Landing mẫu |
| `http://localhost:8080/health` | Health check |

Đăng nhập: `admin` / `ApexAdmin@2026!` → **đổi mật khẩu ngay**.

---

## 📄 Tài liệu gốc (legacy)

| File | Nội dung |
|---|---|
| [`README.md`](README.md) | Tổng quan tính năng, ma trận 14 trường Google Sheets |
| [`SECURITY_PLAN.md`](SECURITY_PLAN.md) | Kế hoạch bảo mật đầy đủ |
| [`DEPLOY_WINDOWS_SERVER_2025.md`](DEPLOY_WINDOWS_SERVER_2025.md) | Deploy Windows chi tiết |
| [`DEPLOY_SERVER_GUIDE.md`](DEPLOY_SERVER_GUIDE.md) | Deploy Ubuntu/Docker/Nginx |
| [`backend/README.md`](backend/README.md) | Tài liệu backend gốc |
| [`frontend/landing/HUONG_DAN_LANDING_FORM_TRACKING.md`](frontend/landing/HUONG_DAN_LANDING_FORM_TRACKING.md) | Hướng dẫn form tracking |

---

## 🖥️ Production

```
SSH : ssh Administrator@103.124.95.232 -p 22899
Code: C:\apex    Binary: C:\apex\backend\apex-server.exe
Task: Apex_Backend_Server, Apex_Caddy_Server (Task Scheduler)
CRM : https://crm.hanghoaphaisinh.net/ads
```

Chi tiết: [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).
