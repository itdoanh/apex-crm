---
name: debug-production-issue
description: Chẩn đoán và xử lý sự cố production của dự án Apex Fintech — 502, 404 landing, lỗi SSL, mất lead, cache không invalidate, không đăng nhập được. Dùng khi người dùng báo lỗi trên server thật, website không hoạt động, hoặc cần chẩn đoán bằng SSH.
---

# Chẩn đoán Sự cố Production — Apex Fintech

Tài liệu đầy đủ: `docs/RUNBOOK.md`.

---

## Bước 0 — Thu thập thông tin (luôn làm trước)

```cmd
schtasks /Query /TN "Apex_Backend_Server" /V /FO LIST
powershell -NoProfile -Command "Get-Process | Where-Object {$_.ProcessName -match 'apex-server|caddy'} | Select ProcessName,Id,StartTime"
curl http://127.0.0.1:8080/health
curl https://crm.hanghoaphaisinh.net/health
powershell -NoProfile -Command "Get-Content C:\caddy\logs\caddy.log -Tail 30"
powershell -NoProfile -Command "Get-NetTCPConnection -State Listen | Where-Object {$_.LocalPort -in 80,443,8080,5432}"
```

**Không đoán — thu thập bằng chứng trước.**

---

## Tra cứu nhanh triệu chứng

| Triệu chứng | Mục |
|---|---|
| 502 Bad Gateway | [A](#a-502-bad-gateway) |
| Landing trả 404 / trang trắng / file JS 404 | [B](#b-404-trên-landing) |
| Lỗi SSL "trust relationship" | [C](#c-lỗi-ssl--tls) |
| Form không gửi được lead | [D](#d-không-nhận-được-lead) |
| Lead vào CRM nhưng không có trong Sheets | [E](#e-lead-không-đến-google-sheets) |
| Đổi landing nhưng không thấy thay đổi | [F](#f-cache-không-invalidate) |
| Domain mới không lên SSL | [G](#g-on-demand-tls-không-cấp-được-chứng-chỉ) |
| Máy chậm / đĩa đầy | [H](#h-tài-nguyên) |
| Không đăng nhập được CRM | [I](#i-không-đăng-nhập-được-crm) |

---

## A. 502 Bad Gateway

Log Caddy: `dial tcp 127.0.0.1:8080: connectex: ... actively refused`

→ **Backend Go đang chết.**

```cmd
schtasks /End /TN "Apex_Backend_Server"
schtasks /Run /TN "Apex_Backend_Server"
```
Chờ 3–5s → `curl http://127.0.0.1:8080/health`.

### Backend crash ngay khi start
Task Scheduler nuốt stdout/stderr → phải chạy tay để xem lỗi:
```cmd
cd C:\apex\backend
apex-server.exe
```
Lỗi thường gặp: không kết nối được PostgreSQL (service chưa chạy, `.env` sai).

### Phòng ngừa
Thêm vào `C:\caddy\Caddyfile`:
```caddyfile
reverse_proxy 127.0.0.1:8080 {
    header_up Host {host}
    lb_try_duration 10s
    lb_try_interval 500ms
    lb_try_count 5
    health_uri /health
    health_interval 30s
}
```

---

## B. 404 trên landing

| Nguyên nhân | Kiểm tra | Sửa |
|---|---|---|
| Working Directory sai | Task WD phải `C:\apex\backend` | Sửa Task Scheduler |
| Thiếu thư mục `frontend/` | `dir C:\apex\frontend` | `git pull` lại |
| Không có route | Query `domain_routes` | Deploy route trong CRM |
| Domain chưa đăng ký | `curl ".../check-domain?domain=X"` → 403 | Đăng ký trong CRM |

```cmd
curl -I -H "Host: ads.giaodichhanghoatuonglai.com" http://127.0.0.1:8080/landing
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads -c "SELECT * FROM domain_routes;"
```

⚠️ `main.go` phục vụ static qua đường dẫn tương đối `../frontend/landing`. WD ≠ `C:\apex\backend` → không tìm thấy file nào.

---

## C. Lỗi SSL / TLS

```
Could not establish trust relationship for the SSL/TLS secure channel
```

```powershell
Resolve-DnsName domain.com
curl "http://127.0.0.1:8080/api/internal/check-domain?domain=domain.com"
```

| `check-domain` | DNS | Nguyên nhân | Xử lý |
|---|---|---|---|
| 403 | OK | Domain chưa đăng ký | Đăng ký trong CRM |
| 200 | Sai | DNS trỏ sai IP | Sửa A record |
| timeout | OK | Backend chết | Restart backend |

Caddy không thể cấp SSL cho domain mới nếu backend chết (ask endpoint chết).

---

## D. Không nhận được lead

### Flow
```
1. Form có POST /api/leads không?    → DevTools Network
2. Status code?                       → 400 / 403 / 429 / 500
3. Lead vào DB chưa?                  → SELECT * FROM leads ORDER BY id DESC LIMIT 5
4. Bị quarantine chưa?                → SELECT * FROM spam_quarantine ORDER BY id DESC LIMIT 5
```

| Code | Nguyên nhân | Xử lý |
|---|---|---|
| 400 | Thiếu field / SĐT sai định dạng | Sửa form, kiểm tra `name="phone"` |
| 403 | PoW sai / honeypot / domain | Đọc `error` trong response body |
| 403 | "Replay attack detected" | Client dùng lại solution → tạo challenge mới |
| 429 | Rate limit | Chờ, hoặc giảm tần suất test |
| 500 | Lỗi server | Xem log backend (nếu có) |

### Kiểm tra PoW thủ công
```
SHA256(challenge_id + ":" + salt + ":" + solution) phải bắt đầu bằng "000"
```

### Kiểm tra honeypot
Lý do chặn là honeypot → form có field ẩn bị test script điền (vd `website`).

---

## E. Lead không đến Google Sheets

```sql
SELECT * FROM google_sheet_sync_logs ORDER BY id DESC LIMIT 10;
-- status: 'success' | 'error'
-- error_message: lý do
```

### Checklist
1. Sheet đã share cho `sheet-bot@rinva-drive-tool.iam.gserviceaccount.com` (Editor)?
2. `google_sheet_configs.is_active = TRUE`?
3. `filter_landing_id` / `filter_nguon` có khớp lead không? (lọc sai → bị bỏ qua)
4. `C:\apex\backend\service-account.json` còn tồn tại & đúng?
5. Landing có bật sync trong CRM không?

```cmd
curl http://127.0.0.1:8080/api/integrations/sheets/bot-info -H "Authorization: Bearer <token>"
```

---

## F. Cache không invalidate

**Triệu chứng:** Sửa landing/deploy route → vẫn thấy bản cũ.

**Nguyên nhân:** Handler ghi DB mà không gọi `Reload()`.

```bash
grep -rn "Reload(ctx)" backend/handlers/
```

Mỗi handler ghi `landing_pages`/`domains`/`domain_routes` phải có:
```go
landingCache.Reload(ctx)
domainCache.Reload(ctx)
```

**Workaround:** restart backend.

---

## G. On-Demand TLS không cấp được chứng chỉ

Log Caddy:
```
tls: failed to get permission for on-demand certificate
```

```cmd
curl "http://127.0.0.1:8080/api/internal/check-domain?domain=xuananhheathcare.com"
-- 403 = chưa đăng ký (đúng hành vi) → đăng ký trong CRM
-- 200 = đã đăng ký → xem log Caddy chi tiết
```

Nếu backend chết → không domain nào cấp được SSL mới.

---

## H. Tài nguyên

### Đĩa
```powershell
Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'" |
  ForEach-Object { "Free {0}GB / {1}GB" -f [math]::Round($_.FreeSpace/1GB,1), [math]::Round($_.Size/1GB,1) }
Get-ChildItem C:\caddy\logs -File | ForEach-Object { "{0} {1}MB" -f $_.Name, [math]::Round($_.Length/1MB,1) }
```
Nguồn phình to: log Caddy → chạy `deploy/windows/cleanup_disk_space.ps1`.

### RAM
```powershell
Get-Process | Where-Object { $_.ProcessName -match 'apex-server|caddy|postgres' } |
  Select-Object ProcessName, Id, @{n='MemMB';e={[math]::Round($_.WorkingSet64/1MB,1)}}, StartTime
```

---

## I. Không đăng nhập được CRM

```sql
SELECT id, username, is_active, updated_at FROM admin_users;
```
`is_active = false` → `UPDATE admin_users SET is_active = TRUE WHERE username = 'admin';`

### Reset mật khẩu
```sql
DELETE FROM admin_sessions WHERE admin_id = 1;
DELETE FROM admin_users WHERE username = 'admin';
```
```cmd
schtasks /End /TN "Apex_Backend_Server" && schtasks /Run /TN "Apex_Backend_Server"
```
→ Backend re-seed `admin` / `ApexAdmin@2026!` → **đổi mật khẩu ngay**.

### Session
```sql
SELECT id, admin_id, is_revoked, expires_at, last_active_at FROM admin_sessions ORDER BY id DESC LIMIT 10;
DELETE FROM admin_sessions WHERE expires_at < NOW() OR is_revoked = TRUE;
```

---

## Sau khi xử lý — báo cáo

```
1. Triệu chứng đã gặp
2. Bằng chứng thu thập (log, query, output)
3. Nguyên nhân gốc
4. Đã làm gì
5. Đã verify chưa (bằng chứng)
6. Còn lại gì chưa xử lý / rủi ro
```

Nếu là bug code → commit fix theo format `fix(scope): mô tả`.
