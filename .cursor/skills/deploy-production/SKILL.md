---
name: deploy-production
description: Deploy dự án Apex Fintech lên Windows Server 2025 production qua SSH. Dùng khi người dùng yêu cầu deploy, cập nhật server, đổi binary Go, restart dịch vụ, hoặc kiểm tra trạng thái production.
---

# Deploy Production — Apex Fintech

Deploy an toàn theo quy trình git-first. Không bao giờ sửa file trực tiếp trên server.

---

## Thông tin kết nối

```bash
ssh Administrator@103.124.95.232 -p 22899
```

| Hạng mục | Giá trị |
|---|---|
| User | `Administrator` (**không phải** `root` — server là Windows) |
| Port | `22899` (**không phải** 22) |
| Key | `C:\Users\doanh\.ssh\id_ed25519` (tự dùng, không cần `-i`) |
| Alias | `apex-prod` |
| Remote shell | `cmd.exe` (không phải PowerShell) |

### Phiên persistent (cho task nhiều bước)

`ControlMaster` không hoạt động trên Windows. Dùng worker:

```powershell
powershell -File .apex-session\apex.ps1 "hostname"
powershell -File .apex-session\apex.ps1 "PS1:C:\path\to\script.ps1"
powershell -File .apex-session\apex.ps1 "PUT:C:\local.txt:C:/Windows/Temp/remote.txt"
powershell -File .apex-session\apex.ps1 "GET:C:/Windows/Temp/remote.txt:C:\local.txt"
```

Worker hỗ trợ: lệnh cmd thường · `PS1:<file>` (upload + execute) · `PUT` · `GET`.

---

## Quy trình deploy

> ⭐ **Luôn deploy qua môi trường TEST trước** (thiết lập 2026-11-15).
> Chi tiết: `docs/DEPLOYMENT.md` mục 1.0.

```
Bước 0 — Local:  commit + push branch `test`
Bước 1 — Server TEST:  deploy_test.ps1 (pull → build → restart → health)
Bước 2 — Kiểm thử tại https://test.hanghoaphaisinh.net/ads
Bước 3 — Local:  git merge test → push main
Bước 4 — Server PROD:  deploy_prod.ps1 (pull → build → restart → health)
Bước 5 — Verify:  /health + log Caddy
```

### Bước 0 — Commit & push lên branch `test`

```powershell
cd c:\code\apex_moi\landing
git checkout test
git status
git diff --cached
git add <file cụ thể>
git commit -m "fix(scope): mô tả tiếng Việt"
git push origin test
```

Format: `feat(scope):` / `fix(scope):` / `chore(scope):`
Scope: `studio` `landings` `domains` `security` `leads` `sheets` `admin` `deploy`

### Bước 1 — Deploy lên môi trường TEST (server)

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex-test\deploy\windows\deploy_test.ps1
```

Script tự làm: `git pull origin test` → build (Go 1.27.4 trên server) →
restart Task `Apex_Backend_Server_Test` → verify `http://127.0.0.1:8081/health`.

### Bước 2 — Kiểm thử

```powershell
Invoke-WebRequest https://test.hanghoaphaisinh.net/health -UseBasicParsing
Invoke-WebRequest https://test.hanghoaphaisinh.net/ads -UseBasicParsing
```

### Bước 3 — Merge lên `main`

```powershell
git checkout main
git merge test
git push origin main
```

### Bước 4 — Deploy lên production (server)

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\deploy_prod.ps1
```

> ⚠️ `deploy_prod.ps1` **từ chối chạy** nếu branch `test` chưa merge vào `main`.
> Dùng `-Force` để bỏ qua (chỉ khi deploy khẩn cấp).

### Bước 5 — Verify

```cmd
curl http://127.0.0.1:8080/health
curl https://crm.hanghoaphaisinh.net/health
powershell -NoProfile -Command "Select-String -Path C:\caddy\logs\caddy.log -Pattern ERROR | Select-Object -Last 5"
```

**Đạt** khi: cả 2 health trả `{"status":"ok"}` và không có 502 mới trong log Caddy.

---

## Phương án dự phòng — build ở LOCAL rồi `scp` binary lên

Dùng khi server mất mạng hoặc cần deploy khẩn cấp.

### Bước 1 — Build & test local

> ℹ️ Server **ĐÃ có Go toolchain** (`C:\Program Files\Go\bin\go.exe`, Go 1.27.4, cài 2026-11-15)
> ⇒ có thể build trực tiếp trên server. Các bước dưới đây là phương án dự phòng.

```powershell
cd c:\code\apex_moi\landing\backend
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:GOMAXPROCS="1"; $env:GOGC="25"
go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .
go vet ./...
```

### Bước 3 — Upload binary lên server

> ⚠️ Đường dẫn local có dấu `:` (`C:\...`) **không** dùng được với worker
> `apex.ps1` (nó phân tách bằng `:`). Dùng `scp` trực tiếp.

```powershell
scp -i C:\Users\doanh\.ssh\id_ed25519 -P 22899 `
    -o BatchMode=yes -o StrictHostKeyChecking=no `
    "C:\code\apex_moi\landing\backend\apex-server.exe" `
    "Administrator@103.124.95.232:C:/Windows/Temp/apex-server-new.exe"
```

### Bước 4-5 — Server: pull + swap + restart

```cmd
cd C:\apex
git pull
```

```powershell
# Swap (chạy qua apex.ps1 "PS1:<file>")
schtasks /End /TN "Apex_Backend_Server"
Start-Sleep -Seconds 4
Get-Process -Name "apex-server" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 3
Rename-Item C:\apex\backend\apex-server.exe "apex-server.exe.old_<ts>" -Force
Copy-Item C:\Windows\Temp\apex-server-new.exe C:\apex\backend\apex-server.exe -Force
schtasks /Run /TN "Apex_Backend_Server"
```

Chờ 3–5 giây.

---

## ⚠️ Bẫy vận hành

| Bẫy | Hậu quả | Cách tránh |
|---|---|---|
| **Chạy `build.bat` trên server** | Script có `pause` ⇒ treo phiên SSH vô thời hạn | Dùng `go build` trực tiếp, hoặc `deploy_test.ps1` / `deploy_prod.ps1` |
| **Linker flag gõ trực tiếp qua SSH cmd.exe** | `flag provided but not defined: -w\` (cmd xử lý sai dấu nháy kép) | Bọc lệnh build trong file `.ps1` rồi thực thi file |
| **PUT qua worker `apex.ps1` với path có `:`** | `scp` bị cắt path → "stat local \"C\": No such file" | Dùng `scp` trực tiếp cho path local |
| **Copy binary cũ đè khi process đang giữ handle** | "file đang được sử dụng" | `Rename-Item` binary cũ trước rồi `Copy-Item` mới |
| Working directory sai | Static files 404 → landing chết | Task Scheduler WD phải là `C:\apex\backend` |
| Copy thiếu `frontend/` | Toàn bộ landing/CSS/JS 404 | Deploy luôn cả `frontend/` |
| Restart nhầm Task | Không restart được | Task đúng: `Apex_Backend_Server`, `Apex_Caddy_Server` |
| Dùng NSSM | Service trùng, xung đột port | Production dùng **Task Scheduler** |
| Deploy khi `git status` dirty trên server | Mất thay đổi khi pull | Kiểm tra `git status` trước khi pull |

---

## Kiểm tra trạng thái (không deploy)

```cmd
schtasks /Query /TN "Apex_Backend_Server" /V /FO LIST
powershell -NoProfile -Command "Get-Process | Where-Object {$_.ProcessName -match 'apex-server|caddy'} | Select ProcessName,Id,StartTime"
powershell -NoProfile -Command "Get-NetTCPConnection -State Listen | Where-Object {$_.LocalPort -in 80,443,8080,5432}"
```

### Script chẩn đoán đầy đủ

Repo có sẵn script tổng hợp:
```powershell
powershell -File .apex-session\apex.ps1 "PS1:C:\code\apex_moi\landing\deploy\windows\health_check.ps1"
```

Script thu thập: tiến trình · port · Task Scheduler + Working Directory · health local/public · check-domain · log Caddy (15 dòng + 10 lỗi) · đĩa/RAM · số dòng DB + page_id trùng · git status trên server.

---

## Rollback

> ℹ️ Server ĐÃ có Go ⇒ có thể build lại tại chỗ. Vẫn **không** `build.bat` trên server
> (có `pause` ⇒ treo phiên). Rollback nhanh nhất: copy lại file `.bak` (đã lưu ở mỗi lần deploy).

```cmd
cd C:\apex
git log --oneline -5
git reset --hard <commit-hash>
REM copy lại binary backup
dir C:\apex\backend\apex-server.exe.bak_*
copy C:\apex\backend\apex-server.exe.bak_<timestamp> C:\apex\backend\apex-server.exe -Force
schtasks /End /TN "Apex_Backend_Server" && schtasks /Run /TN "Apex_Backend_Server"
```

Rollback môi trường test: thay `C:\apex` → `C:\apex-test`, task `Apex_Backend_Server` → `Apex_Backend_Server_Test`.

---

## Sau khi deploy — báo cáo

Nêu rõ:
1. Commit đã deploy (hash + message)
2. Health check local + public
3. Trạng thái Task Scheduler
4. Lỗi Caddy mới (nếu có)
5. Việc chưa làm / rủi ro còn lại
