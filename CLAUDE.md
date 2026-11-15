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
19. **Bật/tắt CAPI theo từng landing** — `POST /api/facebook-ads/config/toggle` (`landing_id` + `is_active`). Chỉ toggle được dòng có cấu hình riêng; **không** toggle cấu hình mặc định (`landing_id = ''`).
20. **Danh mục trường Google Sheets có 1 nguồn duy nhất** — `backend/integrations/googlesheets/fields.go` (`leadFieldCatalog`, ~90 trường / 9 nhóm). Handler + frontend đều lấy từ đây (`GET /api/integrations/sheets/defaults`); frontend **không** hardcode danh sách trường.
21. **Cột chuẩn mới không tự áp dụng cho config đã lưu** — Admin bấm "Bổ sung Cột" (`POST /api/integrations/sheets/configs/upgrade-mappings`): giữ nguyên cột cũ, thêm cột mới vào CUỐI, ghi tiêu đề mới vào dòng 1.
22. **Đổi trạng thái CRM cập nhật lại dòng Sheet** (không append) — `syncLeadStatusToGoogleSheets()` tra `google_sheet_sync_logs.row_range` rồi `Values.BatchUpdate` đúng dòng. Ô rỗng ⇒ bỏ qua; `RAW` (không diễn giải `=` là công thức).

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
