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

```
Bước 1 — Local:  build + test
Bước 2 — Commit + push
Bước 3 — Server:  git pull
Bước 4 — Server:  build binary
Bước 5 — Server:  restart Task Scheduler
Bước 6 — Verify:  /health + log Caddy
```

### Bước 1 — Build & test local

```powershell
cd c:\code\apex_moi\landing\backend
.\build.bat
.\build_linux.bat    # nếu server chạy Linux
```

### Bước 2 — Commit & push

```powershell
cd c:\code\apex_moi\landing
git status
git diff --cached
git add <file cụ thể>
git commit -m "fix(scope): mô tả tiếng Việt"
git push
```

Format: `feat(scope):` / `fix(scope):` / `chore(scope):`
Scope: `studio` `landings` `domains` `security` `leads` `sheets` `admin` `deploy`

### Bước 3-5 — Server

```cmd
cd C:\apex
git pull
cd backend
build.bat
schtasks /End /TN "Apex_Backend_Server"
schtasks /Run /TN "Apex_Backend_Server"
```

Chờ 3–5 giây.

### Bước 6 — Verify

```cmd
curl http://127.0.0.1:8080/health
curl https://crm.hanghoaphaisinh.net/health
powershell -NoProfile -Command "Select-String -Path C:\caddy\logs\caddy.log -Pattern ERROR | Select-Object -Last 5"
```

**Đạt** khi: cả 2 health trả `{"status":"ok"}` và không có 502 mới trong log Caddy.

---

## ⚠️ Bẫy vận hành

| Bẫy | Hậu quả | Cách tránh |
|---|---|---|
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

```cmd
cd C:\apex
git log --oneline -5
git reset --hard <commit-hash>
cd backend && build.bat
schtasks /End /TN "Apex_Backend_Server" && schtasks /Run /TN "Apex_Backend_Server"
```

---

## Sau khi deploy — báo cáo

Nêu rõ:
1. Commit đã deploy (hash + message)
2. Health check local + public
3. Trạng thái Task Scheduler
4. Lỗi Caddy mới (nếu có)
5. Việc chưa làm / rủi ro còn lại
