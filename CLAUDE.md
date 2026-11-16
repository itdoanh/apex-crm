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
11. **Mọi query analytics phải truyền đủ bộ lọc** `landing_id`/`nguon`/`from_date`/`to_date`/`days`/`from_hour`/`to_hour` → `buildAnalyticsWhereClauseFullRange()`. Bộ lọc giờ hỗ trợ **quay vòng qua nửa đêm** (22h→06h).
12. **Tracking tách theo landing** — `tracker.js` namespace hoá cookie/localStorage theo `landing_id`. Landing mới phải khai báo `data-landing-id` + `data-nguon` trùng `config.json`.
13. **Proof-of-Work** — TTL 10 phút, dung sai clock skew 60 giây, hash = `SHA256(challenge_id + ":" + salt + ":" + solution)`. Client gọi `ensureChallengeReady()` trước submit + disable nút ngay khi click.
    - ⚠️ **Cặp (challenge, solution) phải luôn khớp 1-1** (bug 2026-11-15): dùng `challengeState = { challenge, solution }` (1 biến duy nhất) + `challengeFetchPromise` (single-flight) + `verifyPowSolution()`/`ensurePowPairValid()` tự kiểm trước khi gửi. Server có lớp phục hồi `healMismatchedSolution()` (quét challenge đã phát trong 3 phút) — chống bot không bị nới lỏng.
    - Sửa `tracker.js` ⇒ chạy `node scripts/sync-static-mirror.js` (7 bản mirror; `frontend/admin/js/tracker.js` là nguồn chuẩn).
14. **CAPI không chặn luồng chính** — lỗi gửi sự kiện Meta chỉ ghi log `facebook_capi_logs`, không fail `POST /api/leads` / `POST /api/leads/update`. PII băm SHA-256 (SĐT theo **E.164**: `0912345678` → `+84912345678`); `client_ip_address`/`client_user_agent`/`fbc`/`fbp` gửi rõ.
15. **Pipeline CRM 10 trạng thái** — định nghĩa tập trung tại `backend/models/lead_status.go`; frontend gọi `GET /api/facebook-ads/lead-statuses` để render động (Kanban 10 cột), không hardcode.
    - `moi` → `da_goi` → `ket_ban_zalo` → `da_hoc_buoi_chia_se` (miễn phí) → `da_hoc_chuyen_sau` (≈6.000.000đ) → `da_mua_hang` (kết)
    - Kết thúc không mua: `da_hoc_truoc_do` · `khong_quan_tam` · `spam_khong_lien_lac` · `huy`
 - Tương thích ngược: `tiem_nang`/`da_vao_zoom` → `da_hoc_buoi_chia_se`, `chot` → `da_mua_hang`
 - **Ánh xạ CRM → Meta CAPI** (tự bắn khi đổi trạng thái): `moi`→`Lead` (dùng lại `event_id` gốc
   ⇒ dedup với `fbq`) · `da_goi`→`Contact` · `da_hoc_buoi_chia_se`→`StartTrial` ·
   `da_hoc_chuyen_sau`→`Subscribe` (+value) · `da_mua_hang`→`Purchase` (+value).
   Các trạng thái còn lại (`ket_ban_zalo`, kết thúc không mua) **không** bắn.
 - **CAPI chỉ gửi cho lead từ Facebook** (`only_facebook_traffic`, mặc định `TRUE`) —
   `backend/integrations/facebook/attribution.go` (`DetectFacebookTraffic`): `fbc` → `fbclid` →
   `site_source_name`/`utm_source` Meta → referrer Meta + `utm_medium=paid` → `fbp`.
 - **Chuẩn PII Meta** (nâng EMQ): `country` ISO-2 (`vn`), `ct` bỏ dấu (`ha noi`),
   `st` = tỉnh thật (bỎ QUA vùng kinh tế "Miền Bắc"), `ph` E.164. Sự kiện CRM sync
   fallback IP/UA gốc từ `tracking.server_ip` / `tracking.server_user_agent`.
 - CAPI: `da_hoc_chuyen_sau` → `Subscribe`, `da_mua_hang` → `Purchase`
16. **Ánh xạ URL Meta Ads** — `utm_campaign` = `campaign.id`, `utm_content` = `adset.id`, `utm_term` = `ad.id`; `campaign_name`/`adset_name`/`ad_name` là tên hiển thị. KHÔNG theo quy ước Google Ads.
17. **Dedupe browser ↔ server** — `eventID` là **tham số thứ 4** của `fbq('track', …)`: `fbq('track','Lead',{ content_name:'…' },{ eventID: <cùng event_id> })`. Đặt trong object tham số 3 ⇒ Meta không đọc được ⇒ đếm đôi.
18. **Dataset Quality v27.0** — `web` trả về là **MẢNG** (mỗi phần tử = 1 `event_name` kèm chỉ số riêng); `dataset_id` là tham số **bắt buộc**; `agent_name` tuỳ chọn.
19. **Bật/tắt CAPI theo từng landing** — `POST /api/facebook-ads/config/toggle` (`landing_id` + `is_active`). `landing_id` rỗng ⇒ toggle DÒNG CẤU HÌNH MẶC ĐỊNH (tắt ⇒ mọi landing CHƯA có cấu hình riêng ngừng gửi CAPI; landing đã có config riêng không bị ảnh hưởng). Query luôn khớp CHÍNH XÁC `landing_id` ⇒ không đụng nhầm dòng khác.
20. **Danh mục trường Google Sheets có 1 nguồn duy nhất** — `backend/integrations/googlesheets/fields.go` (`leadFieldCatalog`, ~90 trường / 9 nhóm). Handler + frontend đều lấy từ đây (`GET /api/integrations/sheets/defaults`); frontend **không** hardcode danh sách trường.
21. **Cột chuẩn mới không tự áp dụng cho config đã lưu** — Admin bấm "Bổ sung Cột" (`POST /api/integrations/sheets/configs/upgrade-mappings`): giữ nguyên cột cũ, thêm cột mới vào CUỐI, ghi tiêu đề mới vào dòng 1.
22. **Đổi trạng thái CRM cập nhật lại dòng Sheet** (không append) — `syncLeadStatusToGoogleSheets()` tra `google_sheet_sync_logs.row_range` rồi `Values.BatchUpdate` đúng dòng. Ô rỗng ⇒ bỏ qua; `RAW` (không diễn giải `=` là công thức).
23. **KHÔNG ghi đè row template khi sửa landing theo domain** (fix 2026-11-16) — `UpdateLandingCode` chỉ update `WHERE domain = $3 AND page_id = $4`; dùng `EnsureLandingPageDomainRow()` clone row gốc (`domain=''`) trước khi update; `ToggleLandingActiveBase()` chỉ toggle row template (dùng khi deploy). Sai ⇒ **thống kê chồng chéo giữa các landing đã nhân bản**.
24. **Route prefix khớp URL dạng `/<tên>.html`** (fix 2026-11-16) — `DomainCache.ResolveRoute` kiểm tra thêm `strings.TrimSuffix(cleanPath, ".html") == pfx` ⇒ `/chucmung.html` khớp route `/chucmung`. Thiếu ⇒ trang chúc mừng rơi xuống nhánh file tĩnh từ đĩa ⇒ mất `data-landing-id` ⇒ tracker ghi nhầm landing.
25. **`tracker.js` KHÔNG fallback về template/ID rác** (fix 2026-11-16) — đã bỏ fallback `'mxv_080_online'` và `window.LadiPageScript.runtime.ladipage_id`. Chuỗi resolve duy nhất: `data-landing-id` → `APEX_LANDING_CONFIG.landing_id` → `APEX_LANDING_ID` → URL → `'default_landing'`.
26. **File `.html` tĩnh phải có landing context** (fix 2026-11-16) — bước 7 edge router (`backend/main.go`): không resolve được landing ⇒ **404**, không phục vụ HTML thô.
27. **Thống kê landing tách human/bot** (fix 2026-11-16) — `ListLandingPages()` trả `views_count` (tổng) · `human_views_count` (`FILTER (WHERE NOT is_bot)`) · `bot_views_count`; CR% trong tab Quản lý Landing dùng `human_views_count` (khớp tab Trung Tâm Báo Cáo); `domains []string` lấy từ `domain_routes`.
28. **CAPI mặc định tắt/xoá được** (fix 2026-11-16) — `POST /api/facebook-ads/config/toggle` & `config/delete` với `landing_id` rỗng ⇒ tác động DÒNG CẤU HÌNH MẶC ĐỊNH (`landing_id = ''`). Query khớp CHÍNH XÁC `landing_id` ⇒ không đụng nhầm dòng khác. Lưu cấu hình mới cho 1 landing riêng phải đọc token từ dòng RIÊNG (`GetFacebookCAPIConfigExact`), KHÔNG fallback token của config mặc định.
29. **Chặn SĐT rác — 2 lớp, KHÔNG tốn tiền QC** (fix 2026-11-16) —
    - CLIENT (`frontend/admin/js/tracker.js` → `blockJunkPhone()`): chặn **TRƯỚC** khi gọi `fetch` ⇒ lưu cookie `apex_junk_blocked_<landing>` (30 ngày) + `location.replace('chucmung.html')` ⇒ **KHÔNG** POST `/api/leads`, **KHÔNG** bắn `fbq('track','Lead')` ⇒ không tạo lead ⇒ **không tốn tiền QC**. Chokepoint: `sendLeadToBackend()` + `initAutoFormInterceptor()` + guard khi tải trang.
    - SERVER (`backend/security/junk_phone.go` → `security.IsJunkPhone`, `lead_handler.go` bước 6.1): không tạo lead, ghi `spam_quarantine`, trả `200 OK` giả.
    - Heuristic **rất chặt** (không chặn nhầm số thật): 14 mẫu cứng · 7 số cuối giống hệt · dãy liên tiếp ±1 **đều toàn bộ** (`0987654322` ❌) · lặp mẫu 2/5 số · lặp mẫu 3 số trong 9 số đầu.
    - Sửa `tracker.js` ⇒ `node scripts/sync-static-mirror.js` (7 bản mirror).
30. **Lead test (tên chứa `(test)`) — `leads.is_test`** (fix 2026-11-16) — `security.IsTestLeadName()`.
    - Bảng CRM + export CSV ⇒ **VẪN hiện** (badge "TEST").
    - Google Sheets ⇒ **VẪN đồng bộ bình thường** (field `is_test`).
    - Biểu đồ / thống kê / CR% / phễu / landing / campaign breakdown ⇒ **KHÔNG tính** (`buildWhereClause()` lọc mặc định; `buildWhereClauseEx(..., includeTest=true)` cho CRM + CSV).
    - `AssociateLeadToSession(..., isTest)` ⇒ KHÔNG set `converted_lead`.
    - Meta CAPI ⇒ **KHÔNG bắn**.
31. **Duyệt cách ly ⇒ đồng bộ Google Sheets** (fix 2026-11-16) — route `POST /api/analytics/report/quarantine/approve` do `LeadHandler.HandleApproveQuarantine` xử lý (cần `syncLeadToGoogleSheets()`). `store.ApproveQuarantineLead(id)` trả `(result, lead, rawData, error)` ⇒ handler gọi `go h.syncLeadToGoogleSheets(lead, rawData)`.
32. **Dedup PageView Pixel ↔ CAPI — `event_id` dùng chung** (fix 2026-11-16) —
 Meta CHỈ gộp sự kiện browser (`fbq`) với server (CAPI) khi cả hai dùng CHUNG
 `event_id`. Không đồng bộ ⇒ **1 lượt xem bị đếm 2 lượt PageView**.
 - `eventID` là **tham số THỨ 4** của `fbq` — KHÔNG phải thuộc tính trong object
   tham số 3. `fbq('track','PageView',{},{eventID: pvEventId})` ✅
 - Backend `backend/handlers/landing_handler.go`: `injectPageViewEventIDSnippet()` (đầu
   `<head>`, sinh MỘT `window.APEX_PV_EVENT_ID` prefix `pv-`) ·
   `rewritePageViewWithEventID()` (ghi đè mọi `fbq('track','PageView')` hardcode) ·
   block Meta Pixel auto-inject kèm eventID.
 - Client `tracker.js`: `ensurePageViewEventId()` + `buildAnalyticsPayload()` gửi
   `event_id` lên server trong payload analytics.
 - Server `dispatchCAPIPageView()`: landing CÓ Pixel ⇒ chỉ bắn CAPI PageView khi có
   `event_id` CHUNG (payload, fallback `analytics_sessions.pv_event_id`) · thiếu ⇒ KHÔNG bắn.
 - DB: `analytics_sessions.pv_event_id` — giữ giá trị ĐẦU TIÊN của phiên.

## Production

```bash
ssh Administrator@103.124.95.232 -p 22899
```
- Code: `C:\apex` · Binary: `C:\apex\backend\apex-server.exe`
- Quản lý tiến trình: **Task Scheduler** (`Apex_Backend_Server`, `Apex_Caddy_Server`) — **KHÔNG phải** NSSM
- Deploy: sửa local → **test ở môi trường TEST** → merge `main` → deploy production
- ⚠️ **Server ĐÃ có Go toolchain** (`C:\Program Files\Go\bin\go.exe`, Go 1.27.4, cài 2026-11-15) ⇒ **có thể build trực tiếp trên server**.
  Vẫn có phương án dự phòng: build ở LOCAL rồi `scp` binary lên. Chi tiết: `docs/DEPLOYMENT.md` mục 1.1.
- 🚨 **Không sửa file trực tiếp trên server** — mọi thay đổi qua git để rollback được

### Ba môi trường

| | **LOCAL** | **TEST** | **PRODUCTION** |
|---|---|---|---|
| `APEX_ENV` | `local` | `test` | `production` |
| Database | `apex_leads` (Docker) | `apex_leads_test` | `apex_leads` |
| Port | `8080` | `8081` | `8080` |
| Code trên server | — | `C:\apex-test` (branch `test`) | `C:\apex` (branch `main`) |
| Task | — | `Apex_Backend_Server_Test` | `Apex_Backend_Server` |
| Domain quản trị | `localhost` | `test.hanghoaphaisinh.net` | `crm.hanghoaphaisinh.net` |
| Đồng bộ ngoài | ❌ | ❌ | ✅ CAPI + Sheets |

**Luồng chuẩn:** `local → git push origin test → deploy_test.ps1 → test OK → git merge test → git push origin main → deploy_prod.ps1`

Môi trường test cô lập bằng: DB riêng + `APEX_DISABLE_EXTERNAL_SYNC=true` + Caddy định tuyến theo HOST.
Chi tiết: `docs/DEPLOYMENT.md` mục 1.0.

**Local chạy song song 2 môi trường** (khớp 100% server) — Docker `apex-postgres` có cả
`apex_leads` + `apex_leads_test`:
```powershell
docker compose up -d postgres
powershell -File deploy\local\run_local.ps1           # :local → DB apex_leads,      :8080
powershell -File deploy\local\run_local.ps1 -Env test # :test  → DB apex_leads_test, :8081
```


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
