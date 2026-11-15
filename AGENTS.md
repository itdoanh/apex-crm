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
# Build backend (Windows) — chạy ở LOCAL, server KHÔNG có Go
cd backend && build.bat              # → apex-backend.exe
cd backend && build_linux.bat        # → apex-server (Linux binary)

# Build chuẩn cho production (khuyến nghị — có -trimpath + -ldflags)
cd backend && go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .

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

> 🚨 **Quy tắc vàng:** Sửa code ở local → test → commit → push → **build binary ở LOCAL**
> (⚠️ **server KHÔNG có Go toolchain** — đã kiểm tra 2026-11-15: không `C:\Go`,
> không `C:\Program Files\Go`, `where go` không ra) → `scp` binary lên server →
> pull code trên server → swap binary → restart task.
> **TUYỆT ĐỐI KHÔNG sửa file trực tiếp trên server** bằng tay (trừ khi là emergency hotfix, và phải ghi lại).
> Chi tiết: `docs/DEPLOYMENT.md` mục 1.1.

---

## 5. Bất biến kỹ thuật (KHÔNG được phá)

1. **CORS mặc định `*`** — đổi thì landing domain khác sẽ gửi lead không được.
2. **Chuẩn số điện thoại VN 10 số** (`03x/05x/07x/08x/09x`) — `security/phone_validator.go`.
3. **Múi giờ hiển thị là `Asia/Ho_Chi_Minh`** — mọi query dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'`, không dùng `NOW()` trực tiếp cho so sánh ngày.
4. **`landing_pages` PK là `(domain, page_id)`** — KHÔNG phải `page_id` đơn lẻ. Sửa schema phải cập nhật cả `db/db.go` lẫn `cache/landing_cache.go`.
5. **Cache phải được `Reload()` sau khi ghi DB** — nếu không, thay đổi không hiện ra.
6. **Static files phục vụ qua đường dẫn tương đối** `../frontend/landing`, `../frontend/admin` → backend **phải** chạy với working directory là `C:\apex\backend` (giống Task Scheduler đang cấu hình).
7. **On-Demand TLS** dựa vào `/api/internal/check-domain` trả `200` cho domain đã đăng ký, `403` cho domain lạ. Endpoint này **phải** chạy được khi Caddy hỏi.
8. **Mọi query analytics phải truyền đủ bộ lọc** `landing_id`, `nguon`, `from_date`, `to_date`, `days`, `from_hour`, `to_hour` → dùng `buildAnalyticsWhereClauseFullRange()` trong `backend/db/db.go`. Bộ lọc giờ hỗ trợ **quay vòng qua nửa đêm** (22h→06h). Không bỏ qua bộ lọc — số liệu sẽ sai.
9. **Tracking phải tách theo landing**: `tracker.js` namespace hoá cookie/localStorage theo `landing_id` (`apex_vid_<landing>`). Landing mới **phải** khai báo `data-landing-id` + `data-nguon` trùng `landing_id` trong `config.json`.
10. **Proof-of-Work**: TTL 10 phút, dung sai clock skew 60 giây. Client phải gọi `ensureChallengeReady()` **trước khi submit** và disable nút ngay khi click. Hash format: `SHA256(challenge_id + ":" + salt + ":" + solution)`.
11. **CAPI không được chặn luồng chính** — lỗi gửi sự kiện Meta chỉ ghi log vào `facebook_capi_logs`, tuyệt đối không fail `POST /api/leads` hay `POST /api/leads/update`. PII gửi lên Meta phải băm SHA-256; `client_ip_address` / `client_user_agent` / `fbc` / `fbp` gửi **rõ**.

---

## 6. Bảo mật — không được bỏ qua

- `service-account.json`, `.env`, `.env.prod` **không được commit** (đã có trong `.gitignore`).
- Mật khẩu admin hash bằng **Argon2id** (`security/argon2.go`), không dùng MD5/SHA1/bcrypt.
- Session token lưu dạng **SHA-256 hash** trong DB, không lưu plaintext.
- Mọi endpoint admin phải bọc `authHandler.RequireAdminAuth(...)`.
- Endpoint public duy nhất: `POST /api/leads`, `GET /api/landing/config`, `POST /api/analytics/track`, `GET /api/auth/challenge`, `/api/internal/check-domain`, `/health`.

### 6.1. Endpoint Analytics (admin — `RequireAdminAuth`)

| Endpoint | Mô tả |
|---|---|
| `GET /api/analytics/overview` | Tổng quan mở rộng (views, human, bot, leads, CR, VPN, in-app, scroll funnel) |
| `GET /api/analytics/timeseries` | Chuỗi thời gian giờ/ngày/tuần |
| `GET /api/analytics/devices` | Thiết bị / trình duyệt / in-app / viewport |
| `GET /api/analytics/meta-macros` | Meta macros (campaign/adset/ad/placement) + CR% |
| `GET /api/analytics/bots` | Phân tích bot (lý do, score, tỉ lệ) |
| `GET /api/analytics/vpn` | VPN / Proxy / Tor (trạng thái, độ tin cậy, nhà cung cấp) |
| `GET /api/analytics/landing-report` | **Báo cáo riêng từng landing page** (KPI + 6 biểu đồ + 2 bảng) |
| `GET /api/analytics/landings` · `sources` · `geo` · `ad-placements` · `sessions` · `session/{id}` | Các báo cáo còn lại |

Tất cả đều hỗ trợ `landing_id`, `nguon`, `from_date`, `to_date`, `days`, `from_hour`, `to_hour`.

### 6.2. Endpoint Facebook Ads / Conversions API (admin — `RequireAdminAuth`)

| Endpoint | Mô tả |
|---|---|
| `GET /api/facebook-ads/configs` | Danh sách cấu hình CAPI (token đã mask) |
| `GET/POST /api/facebook-ads/config` | Chi tiết / lưu cấu hình theo `landing_id` |
| `POST /api/facebook-ads/config/delete` | Xoá cấu hình |
| `POST /api/facebook-ads/config/toggle` | Bật/tắt nhanh CAPI theo `landing_id` |
| `POST /api/facebook-ads/test-event` | Gửi 1 sự kiện test (có `test_event_code`) |
| `GET /api/facebook-ads/dataset-quality` | EMQ, coverage, dedupe, freshness, delay, value |
| `GET /api/facebook-ads/logs` | Lịch sử gửi sự kiện CAPI |
| `GET /api/facebook-ads/stats` | Thống kê tổng hợp CAPI |
| `GET /api/facebook-ads/campaigns` | Breakdown theo chiến dịch Meta |
| `POST /api/facebook-ads/parse-url` | Phân tích URL quảng cáo Meta → campaign/adset/ad |
| `GET /api/facebook-ads/lead-statuses` | Danh mục 10 trạng thái CRM (UI render động) |

**Pipeline trạng thái CRM (10 cột)** — định nghĩa tập trung tại `backend/models/lead_status.go`:

`moi` → `da_goi` → `ket_ban_zalo` → `da_hoc_buoi_chia_se` → `da_hoc_chuyen_sau` → `da_mua_hang`,
kèm kết thúc không mua: `da_hoc_truoc_do` · `khong_quan_tam` · `spam_khong_lien_lac` · `huy`.

- `da_hoc_buoi_chia_se` = bước **miễn phí**; `da_hoc_chuyen_sau` = **≈6.000.000đ**; `da_mua_hang` = chuyển đổi cuối.
- Giá trị cũ tự chuẩn hoá: `tiem_nang`/`da_vao_zoom` → `da_hoc_buoi_chia_se`, `chot` → `da_mua_hang`.
- CAPI: `da_hoc_chuyen_sau` → `Subscribe`, `da_mua_hang` → `Purchase` (kèm `value` = `course_value`).
- **Ánh xạ URL Meta Ads KHÔNG theo Google Ads**: `utm_campaign` = `campaign.id`,
  `utm_content` = `adset.id`, `utm_term` = `ad.id`; `campaign_name`/`adset_name`/`ad_name` là tên hiển thị.
- **Dedupe browser ↔ server**: `eventID` là **tham số thứ 4** của `fbq('track', …)` —
  `fbq('track','Lead',{ content_name:'…' },{ eventID: <cùng event_id> })`.
  Đặt trong object tham số 3 ⇒ Meta không đọc được ⇒ đếm đôi.
- **SĐT băm theo E.164**: `0912345678` → `+84912345678` (Meta chỉ khớp E.164).
- **Dataset Quality v27.0**: `web` là **MẢNG** (mỗi phần tử = 1 `event_name`),
  `dataset_id` là tham số **bắt buộc**.

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
