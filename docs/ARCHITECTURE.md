# Kiến trúc hệ thống — Apex Fintech

## 1. Tổng quan

Apex Fintech là hệ thống **3 tầng** thu thập và quản trị khách hàng tiềm năng cho các chiến dịch quảng cáo tài chính (CPA cao), vận hành cho **Sở Giao dịch Hàng hóa Việt Nam (MXV 080)** — Bộ Công Thương.

```
        Người dùng (Lead)                    Quản trị viên / Telesale
              │                                        │
              ▼                                        ▼
        ┌──────────────┐                    ┌──────────────────┐
        │ Landing Page │                    │   CRM SPA        │
        │ + tracker.js │                    │  /ads  /admin    │
        └──────┬───────┘                    └────────┬─────────┘
               │  HTTPS                              │ HTTPS
               └──────────────┬─────────────────────┘
                              ▼
                 ┌────────────────────────────┐
                 │  CADDY  :80 / :443        │
                 │  • On-Demand TLS (2s)      │
                 │  • ask → check-domain      │
                 │  • HTTP/2, HTTP/3          │
                 │  • Log rotation 10MB       │
                 └─────────────┬──────────────┘
                              │ proxy 127.0.0.1:8080
                              ▼
                 ┌────────────────────────────┐
                 │  GO BACKEND  :8080         │
                 │  • Security middleware     │
                 │  • Multi-domain edge router│
                 │  • RAM cache (landing/dom) │
                 │  • Phone normalizer        │
                 │  • GeoIP + ad placement    │
                 │  • Argon2id auth + audit   │
                 └──────┬──────────────┬──────┘
                        │              │
                        ▼              ▼
            ┌────────────────┐  ┌──────────────────┐
            │ PostgreSQL 16  │  │ Google Sheets v4 │
            │ 12 bảng        │  │ Master SA Bot    │
            │ JSONB tracking │  │ sheet-bot@rinva- │
            │ GIN indexes    │  │ drive-tool       │
            └────────────────┘  └──────────────────┘
```

## 2. Tầng Reverse Proxy (Caddy)

**Vai trò:** TLS termination + reverse proxy + bảo vệ tài nguyên.

```caddyfile
{
    on_demand_tls {
        ask http://127.0.0.1:8080/api/internal/check-domain
        interval 2m
        burst 5
    }
    email admin@hanghoaphaisinh.net
    log { output file C:/caddy/logs/caddy.log { roll_size 10mb; roll_keep 3; roll_keep_for 7d } }
}

crm.hanghoaphaisinh.net {
    reverse_proxy 127.0.0.1:8080 { header_up Host {host}; header_up X-Real-IP {remote_host}; ... }
}

# Môi trường TEST — site block riêng, ưu tiên hơn block catch-all
test.hanghoaphaisinh.net {
    tls { on_demand }
    reverse_proxy 127.0.0.1:8081 { header_up Host {host}; ... }
}

:443 {
    tls { on_demand }
    reverse_proxy 127.0.0.1:8080 { ... }
}
```

> ⚠️ **Caddy 2.8.4 không hỗ trợ `on_demand { ask ... }` lồng trong site block**
> (lỗi `unknown subdirective: {`). Vì vậy **mọi** site block dùng chung **MỘT**
> global `on_demand_tls` → `ask http://127.0.0.1:8080/api/internal/check-domain`
> (hỏi backend **production**).
>
> ⇒ Backend production phải trả `200` cho `test.hanghoaphaisinh.net` (đã có trong
> `onDemandAllowListByEnv` → `EnvProduction`), nếu không Caddy **không cấp được
> chứng chỉ** cho domain test.
>
> Tách biệt vẫn đảm bảo vì Caddy định tuyến theo **HOST**, không theo cổng ask.

### Cơ chế On-Demand TLS

1. Domain lạ trỏ về IP server → Caddy nhận request HTTPS.
2. Caddy gọi `GET /api/internal/check-domain?domain=...` tới backend.
3. Backend kiểm tra RAM cache (`domainCache.HasDomain`):
   - Domain đã đăng ký trong CRM → `200 OK` → Caddy cấp SSL trong ~2s.
   - Domain lạ → `403 Forbidden` → Caddy từ chối cấp chứng chỉ.
4. **Không bao giờ cần restart server** khi thêm/gỡ domain.

> ⚠️ **Phụ thuộc chặt chẽ:** nếu backend chết, Caddy **không cấp được SSL cho domain mới** (vì `ask` endpoint chết). Backend phải luôn sống.

## 3. Tầng Backend (Go)

### 3.1. Khởi tạo (`main.go`)

Thứ tự khởi động **rất quan trọng**:

```
config.Load()              → đọc .env (PORT, DB, Google SA)
db.NewStore()              → connect PostgreSQL + initSchema() (CREATE TABLE IF NOT EXISTS)
store.SeedDefaultLanding() → tạo landing mặc định nếu chưa có
store.SeedDefaultDomains() → tạo domain mặc địnhh
store.SeedDefaultAdmin()   → tạo admin/admin nếu chưa có
cache.NewLandingCache().Warmup()  → nạp landing configs vào RAM
cache.NewDomainCache().Warmup()   → nạp domain routes vào RAM (Longest Prefix Match)
→ đăng ký routes → ListenAndServe(":8080")
```

### 3.2. Multi-Domain Edge Router

`landingEdgeRouter` trong `main.go` xử lý **mọi** request không khớp route cụ thể:

```
landingEdgeRouter(r):
  1. Chặn path traversal (".." hoặc "//")
  2. domainCache.ResolveRoute(host, path)   → landingID, pathPrefix, subPath, redirectURL
  3. Nếu match:
     a. redirectURL != ""  → 302 redirect
     b. subPath là file tĩnh (.js/.css/.png/...) → phục vụ từ landing_files DB, rồi fallback disk
     c. subPath == "chucmung.html" → serve file chucmung.html
     d. Không có subPath → render landing HTML từ RAM cache + InjectDynamicScripts()
  4. reqPath == "/" → redirect /admin
  5. Chặn file ẩn (tên bắt đầu bằng ".")
  6. Whitelist đuôi file được phép phục vụ
  7. Fallback: tìm file trong frontend/landing, frontend/admin, static/
```

**Điểm quan trọng:** `subPath` có đuôi file (`.js`, `.css`) nhưng không tìm thấy → trả `404` với MIME đúng, **tuyệt đối không fallback về HTML landing** (tránh trả HTML sai kiểu cho file JS).

### 3.3. RAM Cache

| Cache | Nội dung | API |
|---|---|---|
| `LandingCache` | `page_id` → landing config + HTML | `Get(host, pageID)`, `GetByPageID(pageID)`, `Reload()` |
| `DomainCache` | `domain` → sorted routes (prefix dài → ngắn) | `ResolveRoute(host, path)`, `HasDomain(host)`, `Reload()` |

`DomainCache` dùng **Longest Prefix Match**: `/landing` được ưu tiên hơn `/`, và `/` luôn ở cuối.

**Bất biến:** mọi handler ghi vào `landing_pages` / `domains` / `domain_routes` **phải gọi `Reload()`** sau khi ghi, nếu không thay đổi sẽ không hiện ra cho người dùng.

### 3.4. InjectDynamicScripts

Khi render landing HTML, backend chèn tự động:
- `tracker.js` (thu thập data + PoW solver)
- Facebook Pixel / Google Analytics / TikTok Pixel (nếu cấu hình trong `settings`)
- Custom head/body scripts

Ngoài ra backend chuẩn hoá HTML trả về:

| Việc | Hàm | Mục đích |
|---|---|---|
| Gắn `data-landing-id` vào thẻ `<script src="tracker.js">` | `InjectDynamicScripts()` | `tracker.js` biết chắc landing nào đang phục vụ (chống chồng chéo landing) |
| Đặt cờ `window.__REGISTRATION_FORM_HANDLED__ = true` | `injectRegistrationFormGuard()` | Landing tự gửi lead ⇒ interceptor của `tracker.js` bỏ qua ⇒ **1 lần bấm = 1 request** |
| Ghi đè đường dẫn tài nguyên tương đối → **tuyệt đối** | `rewriteRelativeAssetPaths()` | `<base href>` đặt sai chỗ trong HTML khiến trình duyệt gọi `/p/tracker.js` (404) ⇒ mất toàn bộ tracking |

`InjectDynamicScriptsWithBase(rawHTML, p, basePath)` nhận `basePath` = prefix của URL
đang phục vụ (`/p/<page_id>`, `/landing`, `/`) để sinh đúng đường dẫn tuyệt đối:

| Kiểu phục vụ | `basePath` | `tracker.js` |
|---|---|---|
| Multi-landing public | `/p/<page_id>` | `/p/<page_id>/tracker.js` |
| Domain route có prefix | `/landing` | `/landing/tracker.js` |
| Domain route gốc | `/` | `/tracker.js` |

## 4. Tầng Frontend

### 4.1. CRM Admin (`frontend/admin/`)

SPA thuần HTML + Tailwind CDN + Alpine.js + Chart.js. **Không có build step** — sửa file là thấy ngay.

| File | Vai trò |
|---|---|
| `index.html` | Master SPA — chứa toàn bộ panel (CRM, Analytics, Sheets, Studio, Domains, Audit) |
| `studio.html` | Ladi Studio — visual drag & drop builder |
| `js/app.js` | Điều phối chính, tab switching, khởi tạo |
| `js/auth.js` | Login, session, đổi mật khẩu |
| `js/crm.js` | Kanban 7 bước, Customer 360° drawer, lead CRUD |
| `js/analytics.js` | Charts: heatmap giờ vàng, funnel, geo, ad placement |
| `js/sheets.js` | Kết nối Google Sheets, mapping matrix |
| `js/domains.js` | Quản lý domain/DNS, deploy route |
| `js/landings.js` | CRUD landing, feature flags |
| `js/studio/*` | Gizmo 8 chốt, layers, history 50 bước, context menu, form scanner, AST mapper |

### 4.2. Landing (`frontend/landing/`)

| File | Vai trò |
|---|---|
| `ladi.html` | Landing chính (LadiPage format) |
| `chucmung.html` | Trang cảm ơn sau khi đăng ký |
| `tracker.js` | Thu thập data, PoW solver, cookie 180 ngày, UTM parsing |
| `config.json` | Cấu hình per-landing (ngày giờ, số suất, backend_url) |

## 5. Luồng dữ liệu Lead

```
Người dùng điền form
  └→ tracker.js thu thập: name, phone, UTM, url (URL trang đăng ký), device, fingerprint, time_on_page
      └→ GET /api/auth/challenge → nhận challenge_id, salt, difficulty
          └→ client brute-force SHA-256 tới khi hash bắt đầu bằng "000"
              └→ POST /api/leads (kèm challenge_solution + signature)
                  ├→ security.VerifyChallenge()  (fail → spam_quarantine, trả 403)
                  ├→ Honeypot check
                  ├→ Rate limit check
                  ├→ Phone normalize + carrier detect (Viettel/Vinaphone/...)
                  ├→ GeoIP resolve (city/region/isp)
                  ├→ Trích xuất `url` (fallback: Referer → Origin → Host)
                  ├→ db.SaveLead()  (dedupe 24h + advisory lock: cùng SĐT → merge tracking,
                  │                 trả về isNew để chỉ đồng bộ Sheet 1 lần)
                  ├→ analytics_sessions upsert + converted_lead = TRUE
                  └→ Google Sheets append (async, không chặn response)
```

## 6. Google Sheets Integration

- **Service Account:** `sheet-bot@rinva-drive-tool.iam.gserviceaccount.com`
- File `service-account.json` được auto-detect qua `config.Load()` (nhiều candidate path).
- Quy trình 1-click: user share Sheet cho bot → dán link → hệ thống tự parse `spreadsheet_id` bằng regex → đọc tabs + headers → lưu mapping.
- Mỗi landing có thể gửi lead tới nhiều sheet, có filter theo `landing_id` / `nguon`.
- 15 trường chuẩn ánh xạ (trong đó có `url` — URL trang khách đăng ký): xem bảng trong `README.md`.

## 7. Cơ chế phân tầng bảo mật

```
Request vào
  ├─ CORS middleware
  ├─ Rate limit (theo IP)
  ├─ PoW challenge verify  ← chặn script tự động
  ├─ Honeypot check         ← chặn bot điền form
  ├─ Behavioral analysis    ← time_on_page, mouse, scroll, clicks
  ├─ GeoIP + phone carrier  ← enrich data
  └─ RequireAdminAuth (nếu endpoint admin)
        └─ Argon2id verify → session token → audit log
```

Chi tiết: `docs/SECURITY.md`.

## 8. Môi trường

### 8.1. Bảng tổng quan

| Môi trường | `APEX_ENV` | Database | Port | Code trên server | Domain quản trị | Đồng bộ ra ngoài |
|---|---|---|---|---|---|---|
| **Local** | `local` | `apex_leads` (Docker `apex-postgres`) | `8080` | — (chạy trên máy dev) | `localhost` | ❌ TẮT |
| **Test** | `test` | `apex_leads_test` | `8081` | `C:\apex-test` (branch `test`) | `test.hanghoaphaisinh.net` | ❌ TẮT |
| **Production** | `production` | `apex_leads` (native `C:\pgsql`) | `8080` | `C:\apex` (branch `main`) | `crm.hanghoaphaisinh.net` | ✅ BẬT |

### 8.2. Cơ chế cô lập

Môi trường `test` được cô lập khỏi production bằng **3 lớp**:

1. **Database riêng** — `apex_leads_test` ⇒ lead thử nghiệm không lẫn vào KPI doanh thu thật.
2. **`APEX_DISABLE_EXTERNAL_SYNC=true`** — chặn ở tầng code:
   - `cfg.IsExternalSyncEnabled()` → `syncLeadToGoogleSheets()` bỏ qua.
   - `facebook.NewDispatcherWithSync(store, false)` → `resolveConfig()` trả `nil, nil` ⇒ không gửi Meta CAPI.
3. **Caddy định tuyến theo HOST** — site block `test.hanghoaphaisinh.net` → `127.0.0.1:8081`
   (backend test), site block `crm.hanghoaphaisinh.net` → `127.0.0.1:8080` (backend production).

### 8.3. Cấu hình `.env`

Cấu hình qua `.env` (xem `.env.example`, `deploy/windows/env.test.example`,
`deploy/windows/env.production.example`). `config.Load()` tự tìm file `.env` ở:
`.env`, `../.env`, `backend/.env`, cạnh executable, `../` cạnh executable.

Biến môi trường chính (định nghĩa trong `backend/config/config.go`):

| Biến | Giá trị hợp lệ | Ý nghĩa |
|---|---|---|
| `APEX_ENV` | `local` \| `test` \| `production` | Định danh môi trường. Nếu để trống ⇒ suy ra từ `DB_NAME` (`apex_leads_test` ⇒ test, `apex_leads` ⇒ production). |
| `APEX_ADMIN_HOSTS` | danh sách host, phân tách bởi `,` | Host được phục vụ CRM Admin (`/ads`, `/admin`). Host khác bị chặn ⇒ CRM không lộ ra internet. |
| `APEX_ON_DEMAND_DOMAINS` | danh sách domain, phân tách bởi `,` | Domain được phép cấp SSL on-demand (dùng bởi `check-domain`). |
| `APEX_DISABLE_EXTERNAL_SYNC` | `true` \| `false` | Chặn Meta CAPI + Google Sheets. Mặc định: TẮT cho `test`, BẬT cho `local`/`production`. |

### 8.4. Nhánh git

| Nhánh | Môi trường | Quy trình |
|---|---|---|
| `test` | Test | commit từ local → push → server test `git pull` → build → restart task |
| `main` | Production | merge từ `test` sau khi test OK → push → server production `git pull` → build → restart task |

Chi tiết script: `docs/DEPLOYMENT.md` mục 1.0.
