# Runbook — Xử lý sự cố Production

Tài liệu này dành cho người trực hệ thống production. Mỗi triệu chứng → nguyên nhân → cách xử lý.

**Kết nối:**
```bash
ssh Administrator@103.124.95.232 -p 22899
```

---

## 0. Triệu chứng → tra cứu nhanh

| Triệu chứng | Mục |
|---|---|
| Website trả 502 | [1. 502 Bad Gateway](#1-502-bad-gateway) |
| Website trả 404 trên landing | [2. 404 Not Found](#2-404-not-found-trên-landing) |
| HTTPS lỗi "trust relationship" | [3. Lỗi SSL](#3-lỗi-ssl--tls) |
| Landing không nhận lead | [4. Không nhận được lead](#4-không-nhận-được-lead) |
| Lead vào CRM nhưng không có trong Google Sheets | [5. Sheet không nhận lead](#5-lead-không-đến-google-sheets) |
| Đổi landing nhưng không thấy thay đổi | [6. Cache không invalidate](#6-cache-không-invalidate) |
| Domain mới không lên SSL | [7. On-Demand TLS không cấp được](#7-on-demand-tls-không-cấp-được-chứng-chỉ) |
| Máy chủ chậm / đầy đĩa | [8. Tài nguyên](#8-tài-nguyên--hiệu-năng) |
| Không đăng nhập được CRM | [9. Auth](#9-không-đăng-nhập-được-crm) |
| Tưởng mất dữ liệu lead | [10. Khôi phục dữ liệu](#10-khôi-phục-dữ-liệu) |

---

## 1. 502 Bad Gateway

**Triệu chứng:** Caddy trả 502. Log Caddy có:
```
dial tcp 127.0.0.1:8080: connectex: No connection could be made because the target machine actively refused it.
```

**Nguyên nhân:** Backend Go đang chết (crash) hoặc đang restart.

### Chẩn đoán
```cmd
sc query "Apex_Backend_Server"
schtasks /Query /TN "Apex_Backend_Server" /V /FO LIST
powershell -NoProfile -Command "Get-Process apex-server -ErrorAction SilentlyContinue | Select-Object Id,StartTime"
curl http://127.0.0.1:8080/health
```

### Xử lý
```cmd
schtasks /End /TN "Apex_Backend_Server"
schtasks /Run /TN "Apex_Backend_Server"
```
Chờ 3–5 giây rồi `curl http://127.0.0.1:8080/health`.

### Nếu backend crash ngay khi start
Backend **đã có log** tại `C:\apex\backend\logs\server.log` (Task Scheduler được bọc
`cmd /c ... >> logs\server.log 2>&1` từ 2026-11-04). Đọc log trước:
```powershell
Get-Content C:\apex\backend\logs\server.log -Tail 50 -Encoding UTF8
```
Lỗi thường gặp: `không thể khởi tạo database PostgreSQL` → kiểm tra PostgreSQL service đã chạy chưa, `.env` đúng chưa.
Lỗi `bind: Only one usage of each socket address` → process cũ chưa chết, kill rồi start lại:
```powershell
Get-Process -Name "apex-server" | Stop-Process -Force
```

### Phòng ngừa ✅ (đã áp dụng)
`C:\caddy\Caddyfile` đã có health check + retry:
```caddyfile
reverse_proxy 127.0.0.1:8080 {
    header_up Host {host}
    health_uri /health
    health_interval 30s
    health_timeout 5s
    lb_try_duration 10s
    lb_try_interval 500ms
}
```
> ⚠️ **Không dùng `lb_try_count`** — subdirective này chỉ có từ Caddy 2.9+,
> production chạy 2.8.4 ⇒ `caddy validate` fail ⇒ Caddy không khởi động được.

> ⚠️ **Server KHÔNG có Go toolchain.** Không chạy `build.bat` qua SSH (có `pause` ⇒ treo
> phiên). Build ở local rồi `scp` binary lên. Xem `docs/DEPLOYMENT.md` mục 1.1.

---

## 2. 404 Not Found trên landing

**Triệu chứng:** Domain trả 404, hoặc trang trắng, hoặc file JS/CSS trả 404.

**Nguyên nhân thường gặp:**

| Nguyên nhân | Cách kiểm tra | Cách sửa |
|---|---|---|
| Working Directory sai | Task WD phải là `C:\apex\backend` | Sửa lại Task Scheduler |
| Thiếu thư mục `frontend/` | `dir C:\apex\frontend` | Pull lại từ git |
| Không có `domain_routes` | Query DB | Deploy route trong CRM |
| Domain chưa đăng ký | `GET /api/internal/check-domain?domain=X` → 403 | Đăng ký trong CRM |

### Chẩn đoán
```cmd
curl -I http://127.0.0.1:8080/
curl -I -H "Host: ads.giaodichhanghoatuonglai.com" http://127.0.0.1:8080/landing
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads -c "SELECT * FROM domain_routes;"
```

> Nhớ: `main.go` phục vụ static qua đường dẫn tương đối `../frontend/landing`. Nếu WD ≠ `C:\apex\backend` → không tìm thấy file nào.

---

## 3. Lỗi SSL / TLS

**Triệu chứng:**
```
The underlying connection was closed: Could not establish trust relationship for the SSL/TLS secure channel.
```

**Nguyên nhân:** Domain chưa có chứng chỉ, hoặc DNS chưa trỏ đúng.

### Kiểm tra
```powershell
Resolve-DnsName hanghoaphaisinh.net
Resolve-DnsName crm.hanghoaphaisinh.net
Get-ChildItem "C:\Users\Administrator\AppData\Roaming\Caddy\acme" -Recurse -Filter "*.json" | Select-Object Name
```

### Xử lý
1. DNS phải trỏ `103.124.95.232` (cả A record lẫn wildcard).
2. Domain phải có trong bảng `domains`.
3. Truy cập `https://domain.com` → Caddy tự cấp SSL trong ~2s.
4. Nếu vẫn lỗi → kiểm tra log Caddy có dòng `failed to get permission for on-demand certificate` không.

---

## 4. Không nhận được lead

### Flow kiểm tra (theo thứ tự)

```
1. Form có gửi request không?        → DevTools Network tab, xem POST /api/leads
2. Có bị 403 không?                  → Xem lý do trong response body
3. Có bị 429 không?                  → Rate limit
4. Có bị 400 không?                  → Validation (thiếu trường / SĐT sai)
5. Lead có vào DB không?              → SELECT * FROM leads ORDER BY id DESC LIMIT 5
6. Bị quarantine không?              → SELECT * FROM spam_quarantine ORDER BY id DESC LIMIT 5
```

### Kiểm tra PoW
Nếu 403 với lý do `pow_invalid`, kiểm tra:
- Challenge hết hạn (>5 phút)?
- Client gửi đúng `challenge_solution` + `challenge_signature`?
- `challenge_difficulty` trong request có khớp challenge server cấp không?

### Kiểm tra honeypot
Nếu lý do chặn là honeypot → kiểm tra form có chứa field ẩn mà script test vô tình điền (vd: `website`, `fax_company_prefix`).

### Dữ liệu test trong DB
```sql
SELECT id, thoi_gian, so_dien_thoai, landing_id, nguon, trang_thai
FROM leads ORDER BY id DESC LIMIT 10;
```

---

## 5. Lead không đến Google Sheets

### Kiểm tra
```sql
SELECT * FROM google_sheet_sync_logs ORDER BY id DESC LIMIT 10;
-- status = 'success' | 'error'
-- error_message ghi lý do
```

### Checklist
1. Sheet có chia sẻ cho `sheet-bot@rinva-drive-tool.iam.gserviceaccount.com` (quyền Editor) chưa?
2. `google_sheet_configs.is_active = TRUE` chưa?
3. `filter_landing_id` / `filter_nguon` có khớp landing của lead không? (lọc sai → lead bị bỏ qua)
4. File `C:\apex\backend\service-account.json` còn tồn tại & đúng?
5. Landing có bật sync không (trong CRM: Cấu Hình Bảng Tính Google Sheets Nhận Lead)?

### ⚠️ Lead bắn SANG NHIỀU SHEET (triệu chứng: test landing A lại vào cả sheet của landing B)

**Nguyên nhân thường gặp (đã fix 2026-11-13):** landing có **nhiều row** trong
`landing_pages` (bản gốc `domain=''` + bản theo domain) mà cấu hình
`google_sheet_ids` chỉ nằm ở bản theo domain. Đọc nhầm bản gốc ⇒ không có sheet
IDs ⇒ backend rơi vào nhánh "đồng bộ mặc định" ⇒ khớp **mọi** sheet active có
`filter_landing_id = ''`.

**Kiểm tra:**
```sql
-- 1. Landing có nhiều row không? Cấu hình sheet nằm ở row nào?
SELECT page_id, domain, settings->'google_sheet_ids' AS sheet_ids,
       settings->>'google_sheet_sync_mode' AS mode,
       updated_at AT TIME ZONE 'Asia/Ho_Chi_Minh' AS updated
FROM landing_pages
WHERE page_id = 'doanh_chung'
ORDER BY updated_at DESC;

-- 2. Lead gần nhất có bắn vào mấy sheet?
SELECT l.lead_id, ld.landing_id, l.config_id, c.name, c.filter_landing_id,
       TO_CHAR(l.synced_at AT TIME ZONE 'Asia/Ho_Chi_Minh','YYYY-MM-DD HH24:MI:SS') AS synced
FROM google_sheet_sync_logs l
JOIN google_sheet_configs c ON c.id = l.config_id
JOIN leads ld ON ld.id = l.lead_id
ORDER BY l.id DESC LIMIT 20;

-- 3. Sheet nào đang "nhận tất cả" (filter rỗng) — thủ phạm gửi tràn
SELECT id, name, filter_landing_id, filter_nguon, is_active
FROM google_sheet_configs ORDER BY id;
```

**Cách xử lý:**
- Cấu hình `google_sheet_ids` cho TẤT CẢ landing (mode `selected`) trong CRM.
- Đặt `filter_landing_id` cho sheet nếu muốn giới hạn theo landing.
- Restart backend sau khi sửa ⇒ `RepairDuplicateLandingSettings` đồng bộ 2 chiều
  cấu hình sheet giữa bản gốc và bản domain.

### Kiểm tra service account
```cmd
curl http://127.0.0.1:8080/api/integrations/sheets/bot-info -H "Authorization: Bearer <token>"
```

---

## 6. Cache không invalidate

**Triệu chứng:** Sửa landing trong Studio → Ctrl+S → truy cập vẫn thấy bản cũ. Hoặc deploy route mới → truy cập vẫn route cũ.

**Nguyên nhân:** Handler ghi DB nhưng không gọi `Reload()`.

**Cách kiểm tra:** grep trong `backend/handlers/`:
```bash
grep -rn "Reload(ctx)" backend/handlers/
```
Mỗi handler ghi `landing_pages` / `domains` / `domain_routes` phải có:
```go
landingCache.Reload(ctx)
domainCache.Reload(ctx)
```

**Workaround (restart):**
```cmd
schtasks /End /TN "Apex_Backend_Server"
schtasks /Run /TN "Apex_Backend_Server"
```

---

## 7. On-Demand TLS không cấp được chứng chỉ

**Triệu chứng trong log Caddy:**
```
tls: failed to get permission for on-demand certificate
{"domain": "xuananhheathcare.com", "error": "checking http://127.0.0.1:8080/api/internal/check-domain ..."}
```

**Nguyên nhân:** Domain không có trong bảng `domains` → backend trả `403`.

### Kiểm tra
```cmd
curl "http://127.0.0.1:8080/api/internal/check-domain?domain=example.com"
-- 403 = chưa đăng ký (đúng hành vi)
-- 200 = đã đăng ký
```

### Xử lý
Đăng ký domain trong CRM → **Quản Lý Tên Miền** → thêm domain → deploy route.

> Nếu backend chết → `ask` endpoint không phản hồi → **không domain nào cấp được SSL mới**. Luôn giữ backend sống.

---

## 8. Tài nguyên & hiệu năng

### Đĩa đầy
```powershell
Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'" |
  ForEach-Object { "Free {0}GB / {1}GB" -f [math]::Round($_.FreeSpace/1GB,1), [math]::Round($_.Size/1GB,1) }
```

Nguồn phình to nhất thường là log Caddy:
```powershell
Get-ChildItem C:\caddy\logs -File | ForEach-Object { "{0} {1}MB" -f $_.Name, [math]::Round($_.Length/1MB,1) }
```
Task `Apex_Disk_Cleanup` (Chủ Nhật 03:00) đã tự dọn: temp, Windows Update cache,
WER, file `.log` > 50MB, và cắt bớt `server.log` còn 2000 dòng cuối.
Chạy tay: `powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\cleanup_disk_space.ps1`

### RAM / process
```powershell
Get-Process | Where-Object { $_.ProcessName -match 'apex-server|caddy|postgres' } |
  Select-Object ProcessName, Id, @{n='MemMB';e={[math]::Round($_.WorkingSet64/1MB,1)}}, StartTime
```

### PostgreSQL
```cmd
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads -c "SELECT count(*) FROM pg_stat_activity;"
```

---

## 9. Không đăng nhập được CRM

### Triệu chứng: `POST /api/auth/login` trả 401.

### Kiểm tra
```sql
SELECT id, username, is_active, updated_at FROM admin_users;
```
Nếu `is_active = false` → bật lại:
```sql
UPDATE admin_users SET is_active = TRUE WHERE username = 'admin';
```

### Đặt lại mật khẩu (không cần mật khẩu cũ)
Backend tự seed admin mặc định `admin` / `ApexAdmin@2026!` **nếu chưa có user nào**. Xoá user cũ rồi restart backend sẽ re-seed:
```sql
DELETE FROM admin_sessions WHERE admin_id = 1;
DELETE FROM admin_users WHERE username = 'admin';
```
```cmd
schtasks /End /TN "Apex_Backend_Server"
schtasks /Run /TN "Apex_Backend_Server"
```
→ Đăng nhập `admin` / `ApexAdmin@2026!` → **đổi mật khẩu ngay**.

### Session hết hạn / bị revoke
```sql
SELECT id, admin_id, is_revoked, expires_at, last_active_at FROM admin_sessions ORDER BY id DESC LIMIT 10;
```
Xoá session cũ:
```sql
DELETE FROM admin_sessions WHERE expires_at < NOW() OR is_revoked = TRUE;
```

---

## 10. Khôi phục dữ liệu

### Backup tự động ✅
Task `Apex_DB_Backup` chạy hằng ngày 02:00 → `C:\backup\apex_leads\apex_leads_YYYYMMDD_HHmm.dump`,
giữ 7 bản gần nhất. Log: `C:\backup\backup.log`.

```cmd
dir C:\backup\apex_leads
powershell -c "Get-Content C:\backup\backup.log -Tail 20"
```

Chạy tay (không chờ lịch):
```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\backup_database.ps1
```

### Backup thủ công
```cmd
set PGPASSWORD=postgres
C:\pgsql\bin\pg_dump.exe -U postgres -h localhost -d apex_leads -F p -f C:\backup\apex_leads.dump
```

### Restore
```cmd
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
C:\pgsql\bin\pg_restore.exe -U postgres -h localhost -d apex_leads --clean --if-exists C:\backup\apex_leads.dump
```
Restart backend sau khi restore.

> 🚨 **Restore ghi đè dữ liệu hiện tại.** Luôn `pg_dump` bản hiện tại trước khi restore.

### Lead bị xoá nhầm (chưa có backup)
Không có cách nào khô ngoài backup — `leads` không dùng soft-delete. Nếu cần, dựng lại từ `analytics_sessions` (chỉ có session đã convert, không có tên/SĐT đầy đủ).

---

## 11. Công cụ chẩn đoán tổng hợp

Repo có sẵn script đầy đủ:
```powershell
powershell -File .apex-session\apex.ps1 "PS1:C:\code\apex_moi\landing\deploy\windows\health_check.ps1"
```

Thu thập: tiến trình · port · Task Scheduler + Working Directory · health local/public · check-domain · log Caddy (15 dòng + 10 lỗi) · đĩa/RAM · số dòng DB + page_id trùng · git status trên server.

Kiểm tra nhanh:
```cmd
schtasks /Query /TN "Apex_Backend_Server" /V /FO LIST | findstr Status
curl http://127.0.0.1:8080/health
curl https://crm.hanghoaphaisinh.net/health
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads -t -c "SELECT count(*) FROM leads;"
```

---

## 12. Danh sách lệnh nhanh

| Mục đích | Lệnh |
|---|---|
| Restart backend | `schtasks /End /TN "Apex_Backend_Server" & schtasks /Run /TN "Apex_Backend_Server"` |
| Restart Caddy | `schtasks /End /TN "Apex_Caddy_Server" & schtasks /Run /TN "Apex_Caddy_Server"` |
| Health local | `curl http://127.0.0.1:8080/health` |
| Health public | `curl https://crm.hanghoaphaisinh.net/health` |
| Pull code | `cd C:\apex & git pull` |
| Build (LOCAL, không phải server) | `cd backend & go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .` |
| Log backend | `powershell -c "Get-Content C:\apex\backend\logs\server.log -Tail 50 -Encoding UTF8"` |
| Log Caddy | `powershell -c "Get-Content C:\caddy\logs\caddy.log -Tail 50"` |
| Lỗi Caddy | `powershell -c "Select-String -Path C:\caddy\logs\caddy.log -Pattern ERROR | Select -Last 10"` |
| Query DB | `set PGPASSWORD=postgres & C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads` |
| Disk | `powershell -c "Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='C:'\""` |
| Validate Caddyfile | `C:\caddy\caddy.exe validate --config C:\caddy\Caddyfile` |
| Backup DB (tay) | `powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\backup_database.ps1` |
| Dọn disk (tay) | `powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\cleanup_disk_space.ps1` |
| Task list | `schtasks /Query /FO LIST /V | findstr /C:"Apex_"` |
