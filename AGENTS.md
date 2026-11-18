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
# Build backend (Windows) — chạy ở LOCAL hoặc trên server (server ĐÃ có Go 1.27.4)
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
| Go toolchain | ✅ **ĐÃ CÓ** `C:\Program Files\Go\bin\go.exe` (Go 1.27.4, cài 2026-11-15) |

> 🚨 **Quy tắc vàng:** Sửa code ở local → **test ở môi trường TEST** → merge `main` → deploy production.
> Chi tiết quy trình 2 môi trường: `docs/DEPLOYMENT.md` mục 1.0.

### 4.1. Ba môi trường (local · test · production)

| | **LOCAL** | **TEST** | **PRODUCTION** |
|---|---|---|---|
| `APEX_ENV` | `local` | `test` | `production` |
| Database | `apex_leads` (Docker `apex-postgres`) | `apex_leads_test` | `apex_leads` (native `C:\pgsql`) |
| Port | `8080` | `8081` | `8080` |
| Code trên server | — | `C:\apex-test` (branch `test`) | `C:\apex` (branch `main`) |
| Task Scheduler | — | `Apex_Backend_Server_Test` | `Apex_Backend_Server` |
| Domain quản trị | `localhost` | `test.hanghoaphaisinh.net` | `crm.hanghoaphaisinh.net` |
| Đồng bộ ra ngoài | ❌ TẮT | ❌ TẮT | ✅ BẬT (Meta CAPI + Google Sheets) |

**Luồng chuẩn:**
```
LOCAL → git push origin test → SERVER TEST (deploy_test.ps1) → test OK
      → git merge test → git push origin main → SERVER PROD (deploy_prod.ps1)
```

Môi trường test **cô lập** bằng 3 lớp:
1. Database riêng `apex_leads_test` ⇒ lead thử không lẫn KPI doanh thu thật.
2. `APEX_DISABLE_EXTERNAL_SYNC=true` ⇒ `cfg.IsExternalSyncEnabled()` chặn Google Sheets + Meta CAPI.
3. Caddy định tuyến theo HOST: `test.hanghoaphaisinh.net` → `127.0.0.1:8081`.

> ⚠️ **Caddy 2.8.4 chỉ có MỘT global `on_demand_tls`** (không hỗ trợ `ask` riêng theo
> site block) ⇒ backend production phải trả `200` cho `test.hanghoaphaisinh.net`
> (đã có trong `onDemandAllowListByEnv` → `EnvProduction`), nếu không test domain
> không có HTTPS.

Script: `deploy/windows/setup_test_env.ps1` · `deploy_test.ps1` · `deploy_prod.ps1`.
Mẫu env: `deploy/windows/env.test.example` · `env.production.example` · `.env.example`.

**Local chạy song song 2 môi trường** (khớp 100% server) — Docker `apex-postgres`
chứa cả `apex_leads` lẫn `apex_leads_test`:
```powershell
docker compose up -d postgres
powershell -File deploy\local\run_local.ps1          # :local → DB apex_leads,      :8080
powershell -File deploy\local\run_local.ps1 -Env test # :test  → DB apex_leads_test, :8081
```


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
    - ⚠️ **Cặp (challenge, solution) phải luôn khớp 1-1** — dùng `challengeState = { challenge, solution }` (1 biến duy nhất) + `challengeFetchPromise` (single-flight) + `verifyPowSolution()` tự kiểm trước khi gửi. Tách 2 biến `currentChallenge`/`currentChallengeSolution` ⇒ race condition ⇒ lead thật bị ghi vào `spam_quarantine` "Lỗi Proof-of-Work" (bug 2026-11-15).
    - ⚠️ Server có lớp phục hồi (`healMismatchedSolution()`) quét challenge đã phát trong 3 phút ⇒ quét lại lời giải; chống bot **không** bị nới lỏng (bot vẫn phải giải đúng 1 challenge server đã phát).
    - Sửa `tracker.js` ⇒ **phải** chạy `node scripts/sync-static-mirror.js` (7 bản mirror; `frontend/admin/js/tracker.js` là nguồn chuẩn).
11. **CAPI không được chặn luồng chính** — lỗi gửi sự kiện Meta chỉ ghi log vào `facebook_capi_logs`, tuyệt đối không fail `POST /api/leads` hay `POST /api/leads/update`. PII gửi lên Meta phải băm SHA-256; `client_ip_address` / `client_user_agent` / `fbc` / `fbp` gửi **rõ**.
12. **Danh mục trường Google Sheets có 1 nguồn duy nhất** — `backend/integrations/googlesheets/fields.go` (`leadFieldCatalog`). Thêm/sửa trường phải sửa ở đây (KHÔNG hardcode lại ở handler hay `frontend/admin/js/sheets.js`; frontend nạp từ `GET /api/integrations/sheets/defaults`).
13. **Cột chuẩn mới KHÔNG tự áp dụng cho config đã lưu** — phải bấm "Bổ sung Cột" (`POST /api/integrations/sheets/configs/upgrade-mappings`) ⇒ cột mới thêm vào CUỐI, không phá cột cũ / dữ liệu đã ghi.
14. **Đổi trạng thái CRM phải cập nhật lại dòng Sheet** (không append) — `syncLeadStatusToGoogleSheets()` dùng `Values.BatchUpdate` trên dòng đã ghi (tra qua `google_sheet_sync_logs.row_range`). Ô có giá trị rỗng ⇒ bỏ qua; `ValueInputOption` mặc định `RAW` (chống formula injection).
15. **KHÔNG được ghi đè row template khi sửa landing theo domain** (fix 2026-11-16) — `UpdateLandingCode` chỉ update `WHERE domain = $3 AND page_id = $4`. Dùng `EnsureLandingPageDomainRow()` để clone row gốc (`domain=''`) sang row domain mới trước khi update. `ToggleLandingActiveBase()` chỉ toggle row template (dùng khi deploy). Nếu viết `WHERE (domain = $3 OR domain = '')` ⇒ ghi đè template ⇒ **thống kê chồng chéo giữa các landing đã nhân bản**.
16. **Route prefix phải khớp URL dạng `/<tên>.html`** (fix 2026-11-16) — `DomainCache.ResolveRoute` kiểm tra thêm `strings.TrimSuffix(cleanPath, ".html") == pfx` ⇒ `/chucmung.html` khớp route `/chucmung`. Không có ⇒ trang chúc mừng rơi xuống nhánh phục vụ file tĩnh từ đĩa ⇒ mất `data-landing-id` ⇒ tracker ghi nhầm landing.
17. **`tracker.js` KHÔNG được fallback về template/ID rác** (fix 2026-11-16) — đã bỏ fallback `'mxv_080_online'` (template) và `window.LadiPageScript.runtime.ladipage_id`. Chuỗi resolve duy nhất: `data-landing-id` → `APEX_LANDING_CONFIG.landing_id` → `APEX_LANDING_ID` → URL → `'default_landing'`. Thêm fallback mới = tạo landing ID rác ⇒ thống kê chồng chéo.
18. **File `.html` tĩnh phải có landing context** (fix 2026-11-16) — bước 7 của edge router (`backend/main.go`): nếu không resolve được landing (qua `ResolveRoute(host,"/")` hoặc `ResolveChucMungRoute`) ⇒ **404**, KHÔNG phục vụ HTML thô.
19. **CAPI mặc định phải TẮT/XOÁ được** (fix 2026-11-16) — `POST /api/facebook-ads/config/toggle` và `config/delete` với `landing_id` rỗng ⇒ tác động DÒNG CẤU HÌNH MẶC ĐỊNH (`landing_id = ''` trong `facebook_capi_configs`). Query luôn khớp **CHÍNH XÁC** `landing_id` ⇒ không xoá/toggle nhầm dòng của landing khác. Tắt dòng mặc định ⇒ mọi landing **chưa** có cấu hình riêng ngừng gửi CAPI. Lưu cấu hình RIÊNG cho 1 landing mới phải đọc token từ dòng riêng (`GetFacebookCAPIConfigExact`) — KHÔNG fallback token của config mặc định (sai ⇒ dòng mới có `access_token` rỗng ⇒ CAPI lúc chạy báo "thiếu Access Token").
20. **Chặn SĐT rác — 2 lớp, KHÔNG tốn tiền QC** (fix 2026-11-16) —
    - CLIENT (`frontend/admin/js/tracker.js` → `blockJunkPhone()`): chặn **TRƯỚC** khi gọi
      `fetch` ⇒ lưu cookie `apex_junk_blocked_<landing>` (30 ngày) + `location.replace('chucmung.html')`
      ⇒ **KHÔNG** POST `/api/leads`, **KHÔNG** bắn `fbq('track','Lead')` ⇒ không tạo lead ⇒
      **không tốn tiền QC**. Chokepoint: `sendLeadToBackend()` + `initAutoFormInterceptor()` +
      guard khi tải trang (`checkAutoRedirectJunkBlocked()`).
    - SERVER (`backend/security/junk_phone.go` → `security.IsJunkPhone`, gọi trong
      `lead_handler.go` bước 6.1): không tạo lead, ghi `spam_quarantine`, trả `200 OK` giả.
    - Heuristic **rất chặt** (không chặn nhầm số thật): 14 mẫu cứng · 7 số cuối giống hệt ·
      dãy liên tiếp ±1 **đều toàn bộ** (`0987654322` ❌ không chặn) · lặp mẫu 2/5 số ·
      lặp mẫu 3 số trong 9 số đầu.
    - Sửa `tracker.js` ⇒ `node scripts/sync-static-mirror.js` (7 bản mirror).
21. **Lead test (tên CHỨA chuỗi `test`) — tự đánh dấu `is_test`** (fix 2026-11-16, mở rộng 2026-11-16) —
   `leads.is_test BOOLEAN` (`security.IsTestLeadName()` = `strings.Contains(strings.ToLower(name), "test")`).
   - ⚠️ Trước đây chỉ khớp `"(test)"` CÓ NGOẶC ⇒ tên `"test 1"` bị bỏ sót ⇒ lead test VẪN tính vào thống kê (bug anh itdoanh, lead #425).
   - Migration backfill (idempotent) trong `backend/db/db.go` (bước `D1`) đánh dấu lead cũ đã lỡ bị tính.
    - Bảng CRM + export CSV (cột "Lead Test") ⇒ **VẪN hiện** (badge "TEST").
    - Google Sheets ⇒ **VẪN đồng bộ bình thường** (field `is_test` ⇒ "Có"/"").
    - Biểu đồ / thống kê / CR% / phễu / `ListLandingPages` / `GetMetaCampaignBreakdown` ⇒
      **KHÔNG tính** (`buildWhereClause()` lọc mặc định; `buildWhereClauseEx(..., includeTest=true)`
      cho bảng CRM + export CSV).
    - `AssociateLeadToSession(..., isTest)` ⇒ KHÔNG set `converted_lead` / action `converted`.
    - Meta CAPI (`dispatchCAPILead` + `dispatchCAPICRMSync`) ⇒ **KHÔNG bắn**.
22. **Duyệt cách ly ⇒ đồng bộ Google Sheets** (fix 2026-11-16) —
 `POST /api/analytics/report/quarantine/approve` do `LeadHandler.HandleApproveQuarantine`
 xử lý (không phải `ReportHandler` — vì cần `syncLeadToGoogleSheets()`).
 `store.ApproveQuarantineLead(id)` trả về `(result, lead, rawData, error)` ⇒ handler gọi
 `go h.syncLeadToGoogleSheets(lead, rawData)` ⇒ lead được duyệt **vào CRM + vào sheet**
 theo đúng cấu hình `google_sheet_configs` / `google_sheet_sync_mode` của landing.
23. **Dedup PageView Pixel ↔ CAPI — `event_id` dùng chung** (fix 2026-11-16) —
 Meta CHỈ gộp sự kiện browser (`fbq`) với server (CAPI) khi cả hai dùng CHUNG
 `event_id`. Không đồng bộ ⇒ **1 lượt xem bị đếm 2 lượt PageView** ⇒ sai lệch toàn
 bộ chỉ số Landing Page View + phân phồi quảng cáo.
 - `eventID` là **tham số THỨ 4** của `fbq` — KHÔNG phải thuộc tính trong object
   tham số 3. `fbq('track','PageView',{},{eventID: pvEventId})` ✅
 - Backend: `injectPageViewEventIDSnippet()` chèn script ở ĐẦU `<head>` sinh MỘT
   `window.APEX_PV_EVENT_ID` (prefix `pv-`) · `rewritePageViewWithEventID()` ghi đè
   mọi `fbq('track','PageView')` hardcode trong HTML landing để kèm eventID ·
   block Meta Pixel auto-inject cũng kèm eventID.
 - Client (`tracker.js`): `ensurePageViewEventId()` đọc `window.APEX_PV_EVENT_ID` ·
   `buildAnalyticsPayload()` gửi `event_id` lên server trong payload analytics.
 - Server (`dispatchCAPIPageView`): landing CÓ Pixel ⇒ chỉ bắn CAPI PageView khi có
   `event_id` CHUNG (payload, fallback `analytics_sessions.pv_event_id`) · thiếu ⇒
   **KHÔNG bắn** (chấp nhận thiếu 1 lượt thay vì thừa 1 lượt).
 - DB: `analytics_sessions.pv_event_id` — giữ giá trị ĐẦU TIÊN của phiên.
 - Thứ tự thời gian: Pixel bắn TRƯỚC (sinh cookie `_fbp`) ⇒ sau đó mới ping analytics.
29. **Landing CHỈ dùng Meta Pixel — TẮT CAPI tầng server** (fix 2026-11-18) —
 `landing_cu` (deploy tại `https://ads.giaodichhanghoatuonglai.com/landing`) KHÔNG
 dùng CAPI server, chỉ dùng Pixel nhúng trong HTML.
 - ⚠️ NGUYÊN NHÂN: `GetFacebookCAPIConfig(landingID)` FALLBACK sang dòng cấu hình MẶC ĐỊNH (`landing_id = ''`) khi landing chưa có dòng riêng ⇒ tạo 1 cấu hình mặc định là MỌI landing (kể cả landing không dùng CAPI) bắn sự kiện lên Meta ⇒ chiến dịch Meta báo lỗi dữ liệu (thiếu currency/value).
 - CHỐT CHẶN 2 LỚP: ① `serverCAPIDenyList = ["landing_cu", "landing_lznoew"]` + env `META_CAPI_BLOCKED_LANDINGS` ⇒ `resolveConfig()` trả `nil` TRƯỚC khi query DB (tuyệt đối, không thể vô hiệu hoá) · ② migration `D3` trong `backend/db/db.go` tạo dòng `landing_cu` với `is_active = FALSE`.
 - ⇒ HỆ QUẢ: `eventID` chỉ gắn khi landing THỰC SỰ bật CAPI (`isCAPIActive(landing_id)`) — landing không bật CAPI ⇒ giữ nguyên block Meta Pixel GỐC (`fbq('track','PageView')`).
 - Sửa `InjectDynamicScriptsWithBase()` (`backend/handlers/landing_handler.go`) + `RepairMetaPixelPageViewEventID()` (`backend/db/landing_pixel_repair.go`).
30. **Adapter `apex-adapter.js` PHẢI resolve động `landing_id`/`nguon`** (fix 2026-11-18 — `xu_huong_2` tại `/apex` gửi lead về nhầm `doanh_chung`) — hardcode `LANDING_ID='doanh_chung'` + `nguon:'doanh_chung'` ⇒ mọi bản nhân bản dùng chung adapter đều ghi sai landing. Runtime resolve qua `ApexTracker.getLandingConfig()` → `data-landing-id` → `APEX_LANDING_CONFIG` → `APEX_LANDING_ID`, backend ép lại `data-landing-id`/`data-nguon` cũ trong HTML nhân bản (`reStaleDataLandingID*`) + `<base href>` theo `basePath` route đang phục vụ (không dùng `p.PathPrefix`). Đồng bộ adapter đĩa → DB lúc khởi động (`SyncSharedStaticFilesToDB`); bằng chứng production 2026-11-18: lead #482 lúc 19:20:26 vẫn sai (adapter cũ) → adapter DB update 19:54:14 → lead #484 lúc 20:08:52 đã đúng `xu_huong_2`.
31. **`ServeBinaryFile` KHÔNG cache JS/CSS/JSON/HTML** (fix 2026-11-18) — file trong `landing_files` quyết định landing quy kết ⇒ phải `Cache-Control: no-cache, must-revalidate` (kẻo trình duyệt giữ adapter cũ hardcode sau deploy). Ảnh/font/media giữ `public, max-age=86400`.

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
| `POST /api/facebook-ads/config/delete` | Xoá cấu hình (`landing_id` rỗng ⇒ xoá cấu hình **mặc định**) |
| `POST /api/facebook-ads/config/toggle` | Bật/tắt nhanh CAPI theo `landing_id` (`landing_id` rỗng ⇒ toggle cấu hình **mặc định**) |
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
- **Ánh xạ CRM → Meta CAPI** (tự bắn khi sales đổi trạng thái):
  `moi`→`Lead` (dùng lại `event_id` gốc ⇒ dedup với `fbq`) · `da_goi`→`Contact` ·
  `da_hoc_buoi_chia_se`→`StartTrial` · `da_hoc_chuyen_sau`→`Subscribe` (+value) ·
  `da_mua_hang`→`Purchase` (+value). Các trạng thái còn lại **không** bắn.
- **CAPI chỉ gửi cho lead từ Facebook** (`only_facebook_traffic`, mặc định `TRUE`):
  `DetectFacebookTraffic()` trong `backend/integrations/facebook/attribution.go` — bằng chứng
  `fbc` → `fbclid` → `site_source_name`/`utm_source` Meta → referrer Meta + `utm_medium=paid` → `fbp`.
- **Chuẩn PII Meta (quyết định điểm EMQ)**: `country` = ISO-2 (`vn`), `ct` = bỏ dấu
  (`ha noi`), `st` = tỉnh thật (BỎ QUA vùng kinh tế "Miền Bắc"), `ph` = E.164.
  IP/UA gốc cho sự kiện CRM sync fallback từ `tracking.server_ip`/`server_user_agent`.
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
