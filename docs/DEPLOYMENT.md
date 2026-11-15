# Deployment & Operations — Production + Test

Production: **Windows Server 2025 Datacenter** tại `103.124.95.232`.

> 🏗️ **Kiến trúc 2 môi trường trên CÙNG một server** (thiết lập 2026-11-15):
>
> | | **PRODUCTION** | **TEST** |
> |---|---|---|
> | Mục đích | Nhận lead thật, đồng bộ Sheets, gửi Meta CAPI | Up code & kiểm thử trước khi deploy |
> | Code | `C:\apex` (branch `main`) | `C:\apex-test` (branch `test`) |
> | Port | `8080` | `8081` |
> | Database | `apex_leads` | `apex_leads_test` |
> | Task Scheduler | `Apex_Backend_Server` | `Apex_Backend_Server_Test` |
> | Domain quản trị | `crm.hanghoaphaisinh.net` | `test.hanghoaphaisinh.net` |
> | Đồng bộ ra ngoài | ✅ CAPI + Google Sheets | ❌ TẮT (`APEX_DISABLE_EXTERNAL_SYNC=true`) |
>
> Chi tiết: mục **1.0**.

---

## 1.0. Kiến trúc 2 môi trường

### 1.0.1. Sơ đồ tổng thể

```
                          Internet
                              │
                 :80 / :443  ▼
                    ┌──────────────────────┐
                    │  Caddy (2.8.4)       │
                    │  1 tiến trình, 2 site │
                    └───────┬──────────────┘
                            │ reverse_proxy theo HOST
              ┌─────────────┴─────────────┐
              ▼                           ▼
   crm.hanghoaphaisinh.net      test.hanghoaphaisinh.net
   + domain landing khác         (site block riêng)
              │                           │
              ▼                           ▼
      :8080 (apex-server.exe)    :8081 (apex-server.exe)
      Task: Apex_Backend_Server  Task: Apex_Backend_Server_Test
      WD: C:\apex\backend         WD: C:\apex-test\backend
      Code: C:\apex (main)        Code: C:\apex-test (test)
              │                           │
              ▼                           ▼
      DB: apex_leads              DB: apex_leads_test
              │                           │
              ▼                           ▼
      ✅ Meta CAPI                 ❌ KHÔNG gửi CAPI
      ✅ Google Sheets              ❌ KHÔNG ghi Sheets
```

> ⚠️ **Caddy chỉ có MỘT global `on_demand_tls`** (Caddy 2.8.4 **không** hỗ trợ
> `on_demand { ask ... }` lồng trong site block — đã kiểm tra 2026-11-15, báo lỗi
> `unknown subdirective: {`). Vì vậy:
>
> - `ask http://127.0.0.1:8080/api/internal/check-domain` — hỏi **backend production**.
> - Backend production **phải** trả `200` cho `test.hanghoaphaisinh.net` ⇒ đã thêm
>   domain này vào `APEX_ON_DEMAND_DOMAINS` của production
>   (xem `backend/config/config.go` → `onDemandAllowListByEnv`).
> - **Tách biệt thật sự vẫn đảm bảo** vì Caddy định tuyến theo **HOST**:
>   site block `test.hanghoaphaisinh.net` → `127.0.0.1:8081`,
>   site block `crm.hanghoaphaisinh.net` → `127.0.0.1:8080`.

### 1.0.2. Luồng làm việc (branch flow)

```
   LOCAL (C:\code\apex_moi\landing)
      │  git checkout test && git commit && git push origin test
      ▼
   TEST SERVER (C:\apex-test, branch test, port 8081)
      │  https://test.hanghoaphaisinh.net/ads
      │  ← kiểm thử đầy đủ (lead thử, CRM, landing, tracker)
      │
      │  OK ⇒ git checkout main && git merge test && git push origin main
      ▼
   PRODUCTION SERVER (C:\apex, branch main, port 8080)
         https://crm.hanghoaphaisinh.net/ads
```

> ⚠️ **Test KHÔNG được ghi dữ liệu ra ngoài.** Nếu không, mỗi lần test lead là một
> lần ghi vào Google Sheets production + tính vào KPI doanh thu thật.
> Cơ chế chặn: `APEX_DISABLE_EXTERNAL_SYNC=true` → `cfg.IsExternalSyncEnabled()`
> (chặn cả `syncLeadToGoogleSheets` lẫn `facebook.Dispatcher`).

### 1.0.3. Biến môi trường

| Biến | Giá trị | Ý nghĩa |
|---|---|---|
| `APEX_ENV` | `local` \| `test` \| `production` | Định danh môi trường (suy ra từ `DB_NAME` nếu để trống) |
| `APEX_ADMIN_HOSTS` | danh sách host | Host được phục vụ giao diện CRM Admin (`/ads`, `/admin`) |
| `APEX_ON_DEMAND_DOMAINS` | danh sách domain | Domain được phép cấp SSL on-demand |
| `APEX_DISABLE_EXTERNAL_SYNC` | `true` \| `false` | Chặn Meta CAPI + Google Sheets |

Nguồn: `backend/config/config.go`.

### 1.0.4. Script trong repo

| Script | Mục đích |
|---|---|
| `deploy/windows/setup_test_env.ps1` | Dựng môi trường test trên server (DB, clone, `.env`, Task, build, start) |
| `deploy/windows/deploy_test.ps1` | Deploy branch `test` → môi trường test (pull → build → restart task → health) |
| `deploy/windows/deploy_prod.ps1` | Deploy branch `main` → production (kiểm tra merge → pull → build → restart task → health) |
| `deploy/windows/env.test.example` | Mẫu `.env` cho môi trường test |
| `deploy/windows/env.production.example` | Mẫu `.env` cho production |
| `deploy/windows/Caddyfile` | Caddyfile (đã thêm site block `test.hanghoaphaisinh.net` → 8081) |
| `deploy/local/run_local.ps1` | Chạy backend ở máy dev (môi trường `local` hoặc `test`) — tự sinh `.env`, tự tạo DB |
| `deploy/docker/init/01-create-test-db.sql` | Init script Docker — tạo `apex_leads_test` lần đầu |
| `.gitattributes` | Ép line-ending + giữ UTF-8 BOM cho `.ps1` (PowerShell 5.1 cần BOM) |

### 1.0.4.1. Môi trường LOCAL (máy dev)

| Hạng mục | Giá trị |
|---|---|
| Code | `C:\code\apex_moi\landing` (branch `test` khi đang phát triển) |
| DB | PostgreSQL trong Docker — container `apex-postgres` (port 5432) |
| Database | `apex_leads` (môi trường `local`) · `apex_leads_test` (môi trường `test`) |
| Port | `8080` (local) · `8081` (test) |
| Admin | `http://localhost:8080/ads` |

**Chạy:**

```powershell
# 1. Bật PostgreSQL Docker (tạo sẵn CẢ 2 database)
docker compose up -d postgres

# 2. Chạy backend môi trường local (DB apex_leads, port 8080)
powershell -NoProfile -ExecutionPolicy Bypass -File deploy\local\run_local.ps1 -Build

# 3. (Tùy chọn) Chạy song song môi trường test ở local (DB apex_leads_test, port 8081)
powershell -NoProfile -ExecutionPolicy Bypass -File deploy\local\run_local.ps1 -Env test
```

Script `run_local.ps1` sẽ:
1. Kiểm tra container `apex-postgres` đang chạy, tạo DB nếu thiếu.
2. **Sinh lại file `.env`** ở thư mục gốc (UTF-8 **không** BOM — Go đọc `.env`
   bằng `bufio` nên BOM sẽ hỏng key đầu tiên).
3. Build binary (nếu `-Build`) và chạy `apex-server.exe` với `WD = backend\`
   (bắt buộc — static files dùng đường dẫn tương đối `../frontend/landing`).

> ⚠️ **Cả `local` và `test` đều TẮT đồng bộ ra ngoài** (`APEX_DISABLE_EXTERNAL_SYNC=true`)
> ⇒ lead thử ở local **không** ghi vào Google Sheets production và **không** gửi
> Meta CAPI (không làm nhiễu KPI doanh thu thật).

> ⚠️ **Biến môi trường trong session shell CÓ THỂ ghi đè `.env`.** `config.Load()`
> chỉ nạp biến từ `.env` khi biến đó **chưa có** trong môi trường. Nếu anh có
> đặt `DB_NAME`/`PORT`/`APEX_ENV` ở cấp User/Machine ⇒ file `.env` bị bỏ qua.
> Kiểm tra: `Get-ChildItem Env: | Where-Object { $_.Name -match "APEX|DB_|PORT" }`
> → nếu thấy, xoá bằng `Remove-Item Env:\DB_NAME` hoặc gỡ khỏi User/Machine.
>
> ⚠️ **Xoá `apex-server.exe` cũ khi build lại** (Windows giữ handle tới file đang chạy):
> ```powershell
> Get-Process -Name apex-server -ErrorAction SilentlyContinue | Stop-Process -Force
> Remove-Item backend\apex-server.exe -Force -ErrorAction SilentlyContinue
> ```


### 1.0.5. Dựng môi trường test lần đầu (trên server)

```powershell
# Chạy 1 lần duy nhất, quyền Administrator
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex-test\deploy\windows\setup_test_env.ps1
```

Script sẽ (idempotent — chạy lại được):
1. Tạo database `apex_leads_test` nếu chưa có.
2. Clone `C:\apex-test` từ GitHub (branch `test`) hoặc `git pull` nếu đã có.
3. Tạo `C:\apex-test\.env` từ `deploy/windows/env.test.example` (KHÔNG đè file đã có).
4. Copy `service-account.json` từ `C:\apex\backend`.
5. Đăng ký Task `Apex_Backend_Server_Test` (WD = `C:\apex-test\backend`).
6. Build binary (server đã có Go 1.27.4).
7. Khởi động task + kiểm tra `http://127.0.0.1:8081/health`.

Sau đó **copy Caddyfile** lên server và reload:

```powershell
copy /Y C:\apex-test\deploy\windows\Caddyfile C:\caddy\Caddyfile
C:\caddy\caddy.exe validate --config C:\caddy\Caddyfile
C:\caddy\caddy.exe reload  --config C:\caddy\Caddyfile
```

### 1.0.6. Deploy hằng ngày

```powershell
# LOCAL → test
git checkout test
git add <file cụ thể> && git commit -m "feat(scope): ..."
git push origin test

# SERVER → pull + build + restart task test
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex-test\deploy\windows\deploy_test.ps1

# Kiểm tra
Invoke-WebRequest https://test.hanghoaphaisinh.net/health -UseBasicParsing
```

Sau khi test OK → merge và deploy production:

```powershell
# LOCAL
git checkout main && git merge test && git push origin main

# SERVER
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\deploy_prod.ps1
```

> ⚠️ `deploy_prod.ps1` **từ chối chạy** nếu branch `test` chưa merge vào `main`
> (kiểm tra bằng `git branch -r --contains origin/test`). Dùng `-Force` để bỏ qua.

### 1.0.7. Firewall

⚠️ **Port 8081 (backend test) cũng phải bị chặn từ internet** — giống port 8080.
Caddy proxy qua `127.0.0.1` (loopback) nên không bị ảnh hưởng.

```powershell
New-NetFirewallRule -DisplayName "Apex_Block_External_8081" `
    -Direction Inbound -LocalPort 8081 -Protocol TCP `
    -RemoteAddress Any -Action Block -Profile Public,Private
```

### 1.0.8. Database test

```cmd
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads_test
```

Backup DB test (nên có, vì test vẫn cần dữ liệu landing/config):

```powershell
$env:PGPASSWORD = "postgres"
C:\pgsql\bin\pg_dump.exe -U postgres -h localhost -d apex_leads_test -F p `
    -f C:\backup\apex_leads_test\apex_leads_test_test.dump
```

---

## 1. Quy trình deploy (chuẩn — bắt buộc tuân thủ)

```
1. Sửa code ở LOCAL (C:\code\apex_moi\landing)
2. Test local (go build + go vet + go test)
3. git commit + git push
4. ⚠️ ƯU TIÊN: deploy lên môi trường TEST (mục 1.0.6) → kiểm thử → merge main
5. Build binary (LOCAL hoặc SERVER — server đã có Go 1.27.4, mục 1.1)
5b. Upload binary lên server (scp → C:/Windows/Temp/apex-server-new.exe) — nếu build ở LOCAL
6. SSH vào server: cd C:\apex && git pull
7. Swap binary + restart Task Scheduler:
   schtasks /End /TN "Apex_Backend_Server"
   schtasks /Run /TN "Apex_Backend_Server"
8. Verify: curl http://127.0.0.1:8080/health
9. Kiểm tra log Caddy không có 502 mới
```

> 🚨 **TUYỆT ĐỐI KHÔNG sửa file trực tiếp trên server.** Mọi thay đổi phải đi qua git để có thể rollback. Chỉ sửa tay trong trường hợp emergency hotfix — và phải ghi lại vào commit sau.

---

## 1.1. Build binary — server ĐÃ CÓ Go toolchain (từ 2026-11-15)

✅ **Cập nhật 2026-11-15:** `itdoanh` đã cài **Go 1.27.4** trên production server tại
`C:\Program Files\Go\bin\go.exe`. Đã xác minh `go version` chạy được và tải được
module dependency (`pgx/v5`, `google.golang.org/api`, …) qua mạng.

⇒ Có **2 phương án build** — xem bên dưới.

**Phương án A — build trực tiếp trên server (khuyến nghị cho deploy hằng ngày):**

```powershell
# SERVER (PowerShell)
cd C:\apex\backend
$env:GOMAXPROCS = "1"; $env:GOGC = "25"
go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .
```

**Phương án B — build ở LOCAL rồi upload (dự phòng / dùng khi server mất mạng):**

```powershell
# LOCAL
cd C:\code\apex_moi\landing\backend
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:GOMAXPROCS="1"; $env:GOGC="25"
go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .
# → apex-server.exe (~23 MB)
```

```powershell
# Upload — GHI ĐÈ biến `$Target` bằng đường dẫn KHÔNG có dấu `:` trước tên file.
# ⚠️ Worker `apex.ps1` dùng `:` làm dấu phân tách PUT ⇒ đường dẫn `C:\...` bị
#    cắt sai. Dùng `scp` trực tiếp cho đường dẫn local có `:`.
scp -i C:\Users\doanh\.ssh\id_ed25519 -P 22899 `
    -o BatchMode=yes -o StrictHostKeyChecking=no `
    "C:\code\apex_moi\landing\backend\apex-server.exe" `
    "Administrator@103.124.95.232:C:/Windows/Temp/apex-server-new.exe"
```

```powershell
# SERVER — swap + restart
schtasks /End /TN "Apex_Backend_Server"
Start-Sleep -Seconds 4
Get-Process -Name "apex-server" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 3
Copy-Item C:\apex\backend\apex-server.exe C:\apex\backend\apex-server.exe.bak_<timestamp>
Copy-Item C:\Windows\Temp\apex-server-new.exe C:\apex\backend\apex-server.exe -Force
schtasks /Run /TN "Apex_Backend_Server"
```

> 💡 **Nên rename binary cũ trước khi copy** (thay vì chỉ copy):
> ```powershell
> Rename-Item C:\apex\backend\apex-server.exe "apex-server.exe.old_<timestamp>" -Force
> ```
> Windows có thể giữ handle tới file đang chạy ⇒ `Copy-Item -Force` đôi lúc báo
> "file đang được sử dụng". Rename luôn thành công và giữ lại bản backup.

> 🚨 **Không chạy `build.bat` qua SSH.** Script có lệnh `pause` ở cuối ⇒ sẽ treo
> phiên SSH vô thời hạn chờ nhấn phím. Dùng `go build` trực tiếp.

> ⚠️ **Linker flag qua SSH phải bọc trong file `.ps1`.** Khi gõ trực tiếp trong
> `cmd.exe` qua SSH, dấu nháy kép bị `cmd` xử lý sai ⇒ lỗi
> `flag provided but not defined: -w\`. Luôn đặt lệnh build vào file `.ps1`
> rồi thực thi file đó (xem `deploy/windows/deploy_test.ps1` / `deploy_prod.ps1`).

> ✅ **Verify binary đúng phiên bản** (tránh deploy nhầm bản cũ):
> ```powershell
> # LOCAL
> (Get-FileHash C:\code\apex_moi\landing\backend\apex-server.exe -Algorithm SHA256).Hash
> # SERVER (chạy qua apex.ps1 "PS1:...")
> (Get-FileHash C:\apex\backend\apex-server.exe -Algorithm SHA256).Hash
> ```
> Hai giá trị phải **giống hệt**.

### 1.2. Deploy static files (admin UI) — phải copy vào **2 vị trí**

`backend/main.go` phục vụ file tĩnh theo thứ tự ưu tiên (xem `main.go` khoảng dòng 285):

```
../frontend/admin/<path>   ← ưu tiên 1
frontend/admin/<path>      ← ưu tiên 2
...
static/<path>              ← ưu tiên 5
../backend/static/<path>   ← ưu tiên 6
```

⚠️ Vì `../frontend/admin` được tra **trước**, nếu chỉ copy vào `backend/static/` thì
production vẫn serve file cũ ⇒ thay đổi UI **không hiện ra**.

```cmd
REM SERVER — sau khi git pull thì cả 2 vị trí đã đúng tự động.
REM Nếu deploy thủ công bằng SCP thì phải copy vào cả hai:
copy /Y C:\Windows\Temp\index.html C:\apex\frontend\admin\index.html
copy /Y C:\Windows\Temp\index.html C:\apex\backend\static\index.html
copy /Y C:\Windows\Temp\sheets.js C:\apex\frontend\admin\js\sheets.js
copy /Y C:\Windows\Temp\sheets.js C:\apex\backend\static\js\sheets.js
```

**Cách kiểm tra nhanh** (so kích thước file local vs server):

```cmd
powershell -Command "(Get-Item 'C:\apex\frontend\admin\js\sheets.js').Length"
powershell -Command "(Get-Item 'C:\apex\backend\static\js\sheets.js').Length"
```

> ⚠️ Nếu dùng `git checkout --` trên server để dọn working tree cũ, static files sẽ bị
> revert về bản cũ. Phải `git pull origin main` lại ngay sau đó (mạng GitHub đôi lúc
> bị "Empty reply from server" → retry 1-2 lần).

### 1.3. Landing resources (`tracker.js`, `chucmung.html`, ...) — nguồn serve là DB

⚠️ **Khác với admin UI**: tài nguyên của landing page được phục vụ theo thứ tự:

```
1. Bảng `landing_files` (PostgreSQL)   ← ưu tiên cao nhất
2. Thư mục tĩnh trên đĩa (frontend/landing, backend/static, ...)
```

Khi landing được deploy, toàn bộ file (kể cả `tracker.js`) được **copy vào
`landing_files`**. ⇒ `git pull` xong **chưa đủ**: bản trong DB vẫn cũ và tiếp tục
được serve ⇒ "sửa code xong mà không có tác dụng".

> ✅ **Đã xử lý (2026-11-04)**: `SyncSharedStaticFilesToDB()` trong
> `backend/db/landings.go` chạy mỗi lần backend khởi động, đọc `tracker.js` +
> `config.json` từ đĩa rồi ghi đè vào `landing_files` cho **mọi landing**.
> ⇒ restart backend sau khi deploy là đủ cho 2 file này.

**Với các file khác** (`chucmung.html`, `assets/**`): sửa qua CRM → File Manager
(ghi thẳng vào `landing_files`), hoặc đồng bộ thủ công trên server:

```cmd
C:\pgsql\bin\psql.exe -U postgres -d apex_leads -c ^
"UPDATE landing_files SET content = pg_read_file('C:/apex/frontend/landing/chucmung.html', true), updated_at = NOW() WHERE file_path = 'chucmung.html';"
```

**Kiểm tra nhanh** file nào đang được serve (DB hay đĩa):

```cmd
C:\Windows\System32\curl.exe -s -o C:\Windows\Temp\chk.js http://127.0.0.1:8080/tracker.js
powershell -Command "(Get-Item C:\Windows\Temp\chk.js).Length"
REM Nếu khác kích thước file trên đĩa ⇒ đang serve bản DB cũ
```

---

## 2. Kết nối SSH

```bash
ssh Administrator@103.124.95.232 -p 22899
```

| Hạng mục | Giá trị |
|---|---|
| Host | `103.124.95.232` |
| Port | `22899` (KHÔNG phải 22) |
| User | `Administrator` (**không phải** `root` — server là Windows) |
| Key | `C:\Users\doanh\.ssh\id_ed25519` |
| Alias | `apex-prod` (trong `~/.ssh/config`) |

Remote shell là **cmd.exe**, không phải PowerShell. Để chạy PowerShell:
```cmd
powershell -NoProfile -ExecutionPolicy Bypass -Command "..."
```

### Phiên persistent (khuyến nghị cho phiên làm việc dài)

`ControlMaster` của OpenSSH **không hoạt động trên Windows** (`getsockname failed: Not a socket`). Dùng worker script:

```powershell
# Chạy lệnh cmd
powershell -File .apex-session\apex.ps1 "hostname & dir C:\apex"

# Chạy script PowerShell (tự upload rồi execute)
powershell -File .apex-session\apex.ps1 "PS1:C:\path\to\script.ps1"

# Upload / download file
powershell -File .apex-session\apex.ps1 "PUT:C:\local.txt:C:/Windows/Temp/remote.txt"
powershell -File .apex-session\apex.ps1 "GET:C:/Windows/Temp/remote.txt:C:\local.txt"
```

Worker hỗ trợ 4 lệnh: lệnh cmd thường, `PS1:<file>`, `PUT:<local>:<remote>`, `GET:<remote>:<local>`.

---

## 3. Kiến trúc production + test

> Sơ đồ đầy đủ 2 môi trường: xem **mục 1.0.1**. Phần này mô tả chi tiết thành phần
> của production; môi trường test dùng **cùng bộ thành phần** nhưng khác code
> (`C:\apex-test`), port (`8081`), DB (`apex_leads_test`) và Task
> (`Apex_Backend_Server_Test`).

```
Internet
   │
   ├── :80  ──┐
   └── :443 ──┴──▶ Caddy (caddy.exe) ──proxy──▶ apex-server.exe (:8080)
                                                       │
                                                       ▼
                                              PostgreSQL 16 (:5432, localhost only)
```

| Thành phần | Đường dẫn | Quản lý |
|---|---|---|
| Mã nguồn | `C:\apex` (git clone) | `git pull` |
| Binary | `C:\apex\backend\apex-server.exe` | **build trên server** (Go 1.27.4) hoặc build ở LOCAL rồi upload (mục 1.1) |
| Caddy | `C:\caddy\caddy.exe` + `Caddyfile` | Task `Apex_Caddy_Server` |
| PostgreSQL | `C:\pgsql` (native, 844MB) | Windows Service `postgresql-x64-16` |
| Log Caddy | `C:\caddy\logs\` | rotation 10MB × 3 |
| ACME store | `C:\Users\Administrator\AppData\Roaming\Caddy\acme\` | Caddy tự quản lý |

### Task Scheduler (KHÔNG phải NSSM)

Production chạy bằng **Task Scheduler**, không phải NSSM (như `deploy_remote_setup.ps1` mô tả).

| Task | Action | Working Directory |
|---|---|---|
| `Apex_Backend_Server` | `cmd /c "C:\apex\backend\apex-server.exe >> C:\apex\backend\logs\server.log 2>&1"` | `C:\apex\backend` |
| `Apex_Caddy_Server` | `C:\caddy\caddy.exe run --config "C:\caddy\Caddyfile"` | `C:\caddy` |
| `Apex_DB_Backup` | `powershell -File C:\apex\deploy\windows\backup_database.ps1` | `C:\apex\deploy\windows` |
| `Apex_Disk_Cleanup` | `powershell -File C:\apex\deploy\windows\cleanup_disk_space.ps1` | `C:\apex\deploy\windows` |
| `Apex_Backend_Server_Test` | `cmd /c "C:\apex-test\backend\apex-server.exe >> C:\apex-test\backend\logs\server.log 2>&1"` | `C:\apex-test\backend` |

Cấu hình chung: RunAs `SYSTEM`, trigger `MSFT_TaskBootTrigger`, RestartCount 5 / 1 phút, ExecTimeLimit P365D, MultipleInstances `IgnoreNew`.

> ⚠️ **Working Directory là bắt buộc.** `main.go` phục vụ file tĩnh qua đường dẫn tương đối `../frontend/landing`. Đổi WD → static files biến mất → landing trả 404.

> ⚠️ **`deploy_remote_setup.ps1` chạy lại sẽ tạo service NSSM trùng.** Nếu dùng script đó, phải xoá Task Scheduler trước.

> ⚠️ **Backend phải được bọc `cmd /c ... >> log 2>&1`.** Task Scheduler nuốt stdout/stderr ⇒
> mọi lỗi Go (panic, DB disconnect) biến mất hoàn toàn, không có dấu vết để chẩn đoán.
> Script `deploy/windows/setup_logging.ps1` làm việc này (idempotent). Xem mục 9.3.

### Quản lý task

```cmd
schtasks /Query /TN "Apex_Backend_Server" /V /FO LIST
schtasks /End   /TN "Apex_Backend_Server"
schtasks /Run   /TN "Apex_Backend_Server"
```

---

## 4. Database production

```cmd
set PGPASSWORD=postgres
C:\pgsql\bin\psql.exe -U postgres -h localhost -d apex_leads
```

| Thuộc tính | Giá trị |
|---|---|
| Host | `localhost` (chỉ listen `127.0.0.1` + `::1`) |
| Port | `5432` |
| User / Password | `postgres` / `postgres` ⚠️ |
| Database | `apex_leads` |

⚠️ **Đổi mật khẩu DB** trước khi production lâu dài:
```sql
ALTER USER postgres WITH PASSWORD '<mật khẩu-mới>';
```
rồi cập nhật `C:\apex\.env` (`DB_PASSWORD`) và restart backend.

---

## 5. Firewall

| Port | Trạng thái | Ghi chú |
|---|---|---|
| 80 | ✅ Allow Any | HTTP → Caddy redirect HTTPS |
| 443 | ✅ Allow Any | HTTPS → Caddy |
| 8080 | ✅ **Đã chặn** | Rule `Apex_Block_External_8080` (Block, RemoteAddress=Any, Profile Public,Private) |
| 8081 | ⚠️ **Cần chặn** (backend môi trường test) | Rule `Apex_Block_External_8081` — xem mục 1.0.7 |
| 5432 | ✅ Đã chặn + localhost only | Rule `Apex_Block_External_5432`; PostgreSQL chỉ listen loopback |

### Chặn port 8080 (đã áp dụng 2026-11-04)

```powershell
New-NetFirewallRule -DisplayName "Apex_Block_External_8080" -Direction Inbound -LocalPort 8080 -Protocol TCP -RemoteAddress Any -Action Block -Profile Public,Private
```

Sau khi chặn, Caddy vẫn proxy được (vì nó gọi `127.0.0.1:8080` — loopback không bị rule Public/Private chặn).

> ℹ️ **Loopback không bị chặn.** Windows Firewall *Inbound Block* với `RemoteAddress=Any`
> không áp dụng cho traffic qua `127.0.0.1` ⇒ Caddy → backend vẫn hoạt động bình thường.

**Kiểm tra lại:**
```powershell
Invoke-WebRequest -Uri "http://103.124.95.232:8080/health" -UseBasicParsing -TimeoutSec 10
# Phải FAIL sau khi chặn
```

Script: `deploy/windows/setup_firewall.ps1` (idempotent — chạy lại được).

---

## 6. DNS

```
A     crm      → 103.124.95.232
A     test     → 103.124.95.232   (môi trường TEST — đã trỏ 2026-11-15)
A     ns1      → 103.124.95.232
A     ns2      → 103.124.95.232
A     @        → 103.124.95.232
A     *        → 103.124.95.232   (wildcard)
```

Domain landing riêng: trỏ A record về `103.124.95.232`, **hoặc** đổi NS sang `ns1.hanghoaphaisinh.net` / `ns2.hanghoaphaisinh.net`.

Sau khi đổi DNS → đăng ký domain trong CRM (`/ads` → Quản lý Tên Miền) → SSL tự cấp trong ~2s.

---

## 7. Thêm tên miền mới — checklist

1. Trỏ A record (hoặc NS) về `103.124.95.232`.
2. Vào CRM → **Quản Lý Tên Miền** → thêm domain (nhập `ns1`, `ns2`, IP, landing mặc định).
3. Bấm **Kiểm Tra DNS** → chờ `dns_status = active`.
4. Truy cập `https://domain-moi.com` → Caddy tự cấp SSL.
5. **Triển Khai Landing Page Lên Tên Miền** → chọn path prefix + landing.
6. Kiểm tra `domain_routes` trong DB và `domainCache.Reload()` đã được gọi.

---

## 8. Monitoring

### Health check
```powershell
# Production
Invoke-WebRequest -Uri "http://127.0.0.1:8080/health" -UseBasicParsing
Invoke-WebRequest -Uri "https://crm.hanghoaphaisinh.net/health" -UseBasicParsing

# Môi trường test
Invoke-WebRequest -Uri "http://127.0.0.1:8081/health" -UseBasicParsing
Invoke-WebRequest -Uri "https://test.hanghoaphaisinh.net/health" -UseBasicParsing
```

### Xác nhận đúng môi trường đang chạy

```powershell
# Log khởi động ghi rõ môi trường — kiểm tra dòng đầu tiên
Get-Content C:\apex\backend\logs\server.log -Tail 200 | Select-String "MÔI TRƯỜNG"
Get-Content C:\apex-test\backend\logs\server.log -Tail 200 | Select-String "MÔI TRƯỜNG"
# Kỳ vọng:
#   [INFO] MÔI TRƯỜNG: PRODUCTION | DB=apex_leads | ... | Đồng bộ ra ngoài: BẬT
#   [INFO] MÔI TRƯỜNG: TEST | DB=apex_leads_test | ... | Đồng bộ ra ngoài: TẮT
```

### Đọc log Caddy
```powershell
Get-Content C:\caddy\logs\caddy.log -Tail 50
Select-String -Path C:\caddy\logs\caddy.log -Pattern 'ERROR' | Select-Object -Last 10
```

### Kiểm tra tiến trình
```powershell
Get-Process | Where-Object { $_.ProcessName -match 'apex-server|caddy' } |
  Select-Object ProcessName, Id, @{n='MemMB';e={[math]::Round($_.WorkingSet64/1MB,1)}}, StartTime
Get-NetTCPConnection -State Listen | Where-Object { $_.LocalPort -in 80,443,8080,5432 }
```

### Dung lượng đĩa
```powershell
Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'" |
  ForEach-Object { "Free {0}GB / {1}GB" -f [math]::Round($_.FreeSpace/1GB,1), [math]::Round($_.Size/1GB,1) }
```

---

## 9. ⚠️ Vấn đề đã biết — TRẠNG THÁI & CÁCH XỬ LÝ

| # | Vấn đề | Ảnh hưởng | Cách sửa | Trạng thái |
|---|---|---|---|---|
| 1 | **Backend không ghi log** — Task Scheduler nuốt stdout/stderr | Mọi lỗi Go bị mất | Bọc action bằng `cmd /c "apex-server.exe >> C:\apex\backend\logs\server.log 2>&1"` | ✅ **Đã xử lý** — `deploy/windows/setup_logging.ps1` |
| 2 | **Port 8080 mở internet** | Bypass Caddy/TLS | Thêm firewall rule (mục 5) | ✅ **Đã xử lý** — rule `Apex_Block_External_8080` |
| 3 | **Không có health check Caddy** | 502 khi backend restart | Thêm `health_uri` + `lb_try_duration` vào `reverse_proxy` | ✅ **Đã xử lý** — xem `deploy/windows/Caddyfile` |
| 4 | `hanghoaphaisinh.net` chưa có SSL | Truy cập HTTPS fail | Kiểm tra DNS + đăng ký lại domain trong CRM | ⚠️ **Cần user đổi DNS** — xem mục 9.1 |
| 5 | Duplicate `page_id` trong `landing_pages` | Cache có thể load sai landing | Xoá row thừa, giữ 1 row/domain | ✅ **Đã xử lý** — xem mục 9.2 |
| 6 | `deploy_remote_setup.ps1` dùng NSSM | Lệch với thực tế | Cập nhật script dùng Task Scheduler | ✅ **Đã xử lý** |
| 7 | Chưa có task dọn disk | Ổ đĩa đầy dần theo thời gian | Task `Apex_Disk_Cleanup` (Chủ Nhật 03:00) | ✅ **Đã xử lý** — xem mục 9.3 |
| 8 | Chưa có backup DB tự động | Mất dữ liệu khi hỏng ổ đĩa/PostgreSQL | Task `Apex_DB_Backup` (hằng ngày 02:00) | ✅ **Đã xử lý** — xem mục 9.3 |

> ✅ **Đã xử lý trong repo:**
> - `deploy/windows/setup_services.ps1` → mặc định `-Method task` (khớp production), action đã bọc `cmd /c ... >> logs\server.log 2>&1` để có log.
> - `deploy/windows/start_all.bat` / `stop_all.bat` / `restart_all.bat` → tự phát hiện Task Scheduler vs Windows Service.
> - `deploy/windows/health_check.ps1` → script chẩn đoán tổng hợp.
> - `deploy/windows/setup_logging.ps1` → cấu hình lại action Task Scheduler để ghi log.
> - `deploy/windows/setup_scheduled_tasks.ps1` → đăng ký task backup DB + dọn disk.
> - `deploy/windows/backup_database.ps1` → `pg_dump` + xoay vòng giữ 7 bản.

---

## 9.1. `hanghoaphaisinh.net` — cần user đổi DNS

**Nguyên nhân gốc (đã chẩn đoán 2026-11-04):**

| Mục | Giá trị thực tế | Vấn đề |
|---|---|---|
| A record `@` | `103.97.126.29` | ❌ Trỏ sang **máy chủ khác** (openresty — trang "Kiểm tra trình duyệt") |
| NS | `ns1-free02.luutruso.com` / `ns2-free02.luutruso.com` | ❌ Không phải NS của hệ thống |
| IP máy chủ Apex | `103.124.95.232` | — |
| `crm.hanghoaphaisinh.net` | `103.124.95.232` | ✅ Đúng |

⇒ **Không thể cấp SSL cho `hanghoaphaisinh.net`** vì Let's Encrypt xác thực HTTP-01
trên `103.97.126.29` — máy chủ đó không chạy Caddy của Apex. Backend `/api/internal/check-domain`
trả `200` (domain đã đăng ký trong CRM) nên Caddy **sẵn sàng** cấp cert, chỉ là DNS chưa trỏ đúng.

**Việc user (`itdoanh`) cần làm ở panel DNS `luutruso.com`:**

```
A     @     → 103.124.95.232        (xoá bản ghi 103.97.126.29)
A     crm   → 103.124.95.232        ✅ đã đúng, giữ nguyên
A     ns1   → 103.124.95.232
A     ns2   → 103.124.95.232
A     *     → 103.124.95.232        (wildcard — nên có cho sub-domain)
```

**Hoặc** đổi NS của zone `hanghoaphaisinh.net` sang:
```
NS1  ns1.hanghoaphaisinh.net   → 103.124.95.232
NS2  ns2.hanghoaphaisinh.net   → 103.124.95.232
```

Sau khi DNS lan truyền (5–15 phút):
1. Vào CRM → **Quản lý Tên Miện** → **Kiểm Tra DNS** → `dns_status = active`.
2. Truy cập `https://hanghoaphaisinh.net` → Caddy tự cấp SSL (~2s).
3. Trong CRM, `ssl_status` sẽ chuyển `dns_error` → `pending` → `active`.

> ⚠️ **Lưu ý:** `hanghoaphaisinh.net` đang trỏ về máy chủ khác — có thể đó là môi trường
> **cũ** hoặc **staging** của bạn. **Đừng xoá bản ghi A cũ trước khi xác nhận** — nếu
> `103.97.126.29` đang phục vụ nội dung riêng, cần chuyển nội dung đó sang server Apex trước.

---

## 9.2. Duplicate `page_id` trong `landing_pages`

**Nguyên nhân gốc:** `landing_pages` có PK composite `(domain, page_id)` — cùng một `page_id`
có thể tồn tại ở nhiều `domain`. Nhưng `landing_cache.go` xây map `byPageID` chỉ theo
`page_id` (không theo domain) ⇒ **dòng nào được nạp sau cùng sẽ đè dòng trước**.

Tình trạng trước khi sửa (2026-11-04):

| page_id | domain | is_active | form_mapping.fields | path_prefix |
|---|---|---|---|---|
| `mxv_080_online` | *(rỗng)* | `false` | 0 | — |
| `mxv_080_online` | `ads.giaodichhanghoatuonglai.com` | `false` | 5 | `/landing` |

⇒ `byPageID["mxv_080_online"]` bị đè không xác định → `GetByPageID()` có thể trả về
landing không đúng, `path_prefix` bị lẫn giữa 2 domain.

**Cách xử lý (đã áp dụng — ở mức dữ liệu):**
- Backup trước khi sửa: `C:\backup\apex_leads\apex_leads_predup_20261104_093407.dump`.
- Xoá dòng trùng `(domain='ads.giaodichhanghoatuonglai.com', page_id='mxv_080_online')`
  kèm file trong `landing_files` — dòng này là bản sao, không phải bản phục vụ.
- Giữ dòng `domain=''` làm **template/fallback toàn hệ thống** (route
  `hanghoaphaisinh.net /` trỏ tới).
- Kết quả: mỗi `page_id` hiện chỉ còn **1 dòng** → `byPageID` không còn bị đè.

> ⚠️ **Giới hạn còn lại (cần code fix khi tới lúc):** `backend/cache/landing_cache.go`
> vẫn xây map `byPageID` **chỉ theo `page_id`** (không theo domain) nên nếu sau này
> cố ý tạo cùng một `page_id` trên 2 domain, dòng nạp sau sẽ đè dòng trước.
> `GetByPageID()` hiện đã ưu tiên `byDomainPage` với `domain=''` trước khi fallback
> sang `byPageID`, nên fallback toàn hệ thống vẫn đúng. Việc sửa triệt để đòi hỏi
> đổi API `GetByPageID` → `GetByDomainPage(domain, pageID)` — **chưa thực hiện** vì
> nằm ngoài phạm vi task production hardening.

**Kiểm tra duplicate (chạy định kỳ):**
```sql
SELECT page_id, count(*), array_agg(domain)
FROM landing_pages
GROUP BY page_id
HAVING count(*) > 1;
```
> Lưu ý: query này **luôn trả về ≥1 dòng** nếu hệ thống dùng mô hình multi-domain
> (cùng `page_id` ở nhiều domain là hợp lệ). Điều cần kiểm là **trùng lặp trong cùng
> một `(domain, page_id)`** — điều mà PK đã chặn. Vấn đề thật là map cache `byPageID`
> bị đè, đã xử lý ở `backend/cache/landing_cache.go`.

---

## 9.2.1. ⚠️ Route dangling `landing2` — cần user quyết định

Sau khi dọn duplicate, `domain_routes` còn 1 route trỏ tới landing **không tồn tại**:

| domain | path_prefix | landing_id | landing có trong `landing_pages`? |
|---|---|---|---|
| `ads.giaodichhanghoatuonglai.com` | `/` | `landing2` | ❌ **KHÔNG** |

Route này là **root domain** của `ads.giaodichhanghoatuonglai.com` ⇒ truy cập
`https://ads.giaodichhanghoatuonglai.com/` sẽ rơi vào trang lỗi / landing mặc định.
Hai route con (`/landing` → `landing_cu`, `/chucmung` → `landing_lznoew`) vẫn hoạt động bình thường.

**Các landing hiện có:**
| page_id | domain | is_active |
|---|---|---|
| `landing_cu` | `ads.giaodichhanghoatuonglai.com` | ✅ true |
| `landing_lznoew` | `ads.giaodichhanghoatuonglai.com` | ✅ true |
| `mxv_080_online` | *(rỗng — template toàn hệ thống)* | ⚠️ false |

**Việc user (`itdoanh`) cần quyết định** — 3 lựa chọn:
1. Trỏ `/` → `landing_cu` (landing chính, giống `/landing`).
2. Trỏ `/` → `mxv_080_online` (template toàn hệ thống, dùng cho `hanghoaphaisinh.net`).
3. Tạo landing mới `landing2` trong CRM → Studio rồi deploy.

> Em **không tự sửa** route này vì nó là dữ liệu nghiệp vụ — đổi hướng landing root
> có thể ảnh hưởng campaign đang chạy.

---

## 9.3. Task tự động — backup DB & dọn disk

### Task đã đăng ký trên production

| Task | Lịch | Script | Tác dụng |
|---|---|---|---|
| `Apex_DB_Backup` | Hằng ngày **02:00** | `deploy/windows/backup_database.ps1` | `pg_dump -F p` → `C:\backup\apex_leads\`, giữ 7 bản gần nhất |
| `Apex_Disk_Cleanup` | Chủ Nhật **03:00** | `deploy/windows/cleanup_disk_space.ps1` | Dọn temp, Windows Update cache, WER, log > 50MB, cắt bớt `server.log` |

Cả hai chạy bằng `SYSTEM`, `RestartCount 3`, `ExecutionTimeLimit 2 giờ`.

### Đăng ký lại (idempotent)

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\setup_scheduled_tasks.ps1
```

### Chạy thử backup ngay

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\backup_database.ps1
Get-ChildItem C:\backup\apex_leads\
Get-Content C:\backup\backup.log -Tail 20
```

### Khôi phục DB từ backup

```powershell
$env:PGPASSWORD = "postgres"
C:\pgsql\bin\pg_restore.exe -U postgres -h localhost -d apex_leads --clean --if-exists `
    "C:\backup\apex_leads\apex_leads_20261104_0200.dump"
```

> 🚨 **Restore sẽ ghi đè dữ liệu hiện tại.** Luôn `pg_dump` bản hiện tại trước khi restore.

---

## 9.4. Script deploy trong repo

| Script | Nền tảng | Trạng thái |
|---|---|---|
| `deploy/windows/setup_services.ps1` | Windows | ✅ Mặc định `task` (khớp production). Truyền `-Method nssm` nếu muốn Windows Service. |
| `deploy/windows/setup_logging.ps1` | Windows | ✅ Cấu hình lại action Task Scheduler để ghi `logs\server.log` |
| `deploy/windows/setup_scheduled_tasks.ps1` | Windows | ✅ Đăng ký `Apex_DB_Backup` + `Apex_Disk_Cleanup` |
| `deploy/windows/backup_database.ps1` | Windows | ✅ `pg_dump` + xoay vòng giữ 7 bản |
| `deploy/windows/install_postgres_native.ps1` | Windows | ✅ OK |
| `deploy/windows/install_caddy.ps1` | Windows | ✅ OK |
| `deploy/windows/setup_firewall.ps1` | Windows | ✅ OK — port 8080 đã chặn |
| `deploy/windows/cleanup_disk_space.ps1` | Windows | ✅ OK — có thêm xoay vòng `server.log` |
| `deploy/windows/health_check.ps1` | Windows | ✅ **Chẩn đoán tổng hợp** |
| `deploy/windows/setup_test_env.ps1` | Windows | ✅ **Dựng môi trường TEST** (DB + clone + `.env` + Task + build) — mục 1.0.5 |
| `deploy/windows/deploy_test.ps1` | Windows | ✅ **Deploy branch `test`** → môi trường test — mục 1.0.6 |
| `deploy/windows/deploy_prod.ps1` | Windows | ✅ **Deploy branch `main`** → production (có kiểm tra merge) — mục 1.0.6 |
| `deploy/windows/start_all.bat` / `stop_all.bat` / `restart_all.bat` | Windows | ✅ Tự phát hiện Task vs Service |
| `deploy/scripts/deploy.sh` | Linux | ✅ OK |
| `deploy/systemd/apex.service` | Linux | ✅ OK |

---

## 10. Thêm PostgreSQL vào Windows PATH

```powershell
[Environment]::SetEnvironmentVariable("Path", "$([Environment]::GetEnvironmentVariable('Path','Machine'));C:\pgsql\bin", "Machine")
```

---

## 11. Backup

**Đã tự động hoá** — task `Apex_DB_Backup` chạy hằng ngày 02:00 (xem mục 9.3).

### Chạy tay một lần

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\apex\deploy\windows\backup_database.ps1
```

### Thủ công (nếu cần ngay)

```cmd
set PGPASSWORD=postgres
C:\pgsql\bin\pg_dump.exe -U postgres -h localhost -d apex_leads -F p -f C:\backup\apex_leads_%DATE:~10,4%%DATE:~4,2%%DATE:~7,2%.dump
```

### Khôi phục

```powershell
$env:PGPASSWORD = "postgres"
C:\pgsql\bin\pg_restore.exe -U postgres -h localhost -d apex_leads --clean --if-exists `
    "C:\backup\apex_leads\apex_leads_20261104_0200.dump"
```

> 🚨 **Restore ghi đè dữ liệu hiện tại.** Luôn `pg_dump` bản hiện tại trước khi restore.

---

## 12. Rollback

### Production

```cmd
cd C:\apex
git log --oneline -5
git reset --hard <commit-hash>
REM ⚠️ Server ĐÃ có Go 1.27.4 ⇒ có thể build lại tại chỗ (mục 1.1 phương án A),
REM hoặc copy từ file .bak đã lưu.
cd C:\apex\backend
go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .
REM Nếu không build được:
dir C:\apex\backend\apex-server.exe.bak_*
copy C:\apex\backend\apex-server.exe.bak_<timestamp> C:\apex\backend\apex-server.exe -Force
schtasks /End /TN "Apex_Backend_Server" && schtasks /Run /TN "Apex_Backend_Server"
```

### Môi trường test

```cmd
cd C:\apex-test
git log --oneline -5
git reset --hard <commit-hash>
cd C:\apex-test\backend
go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .
schtasks /End /TN "Apex_Backend_Server_Test" && schtasks /Run /TN "Apex_Backend_Server_Test"
```

---

## 13. Tài liệu liên quan

- `DEPLOY_WINDOWS_SERVER_2025.md` — hướng dẫn deploy Windows chi tiết (root repo)
- `DEPLOY_SERVER_GUIDE.md` — hướng dẫn deploy Ubuntu/Docker
- `deploy/windows/` — script cài đặt (bao gồm `setup_test_env.ps1`, `deploy_test.ps1`, `deploy_prod.ps1`)
- `deploy/windows/env.test.example` · `deploy/windows/env.production.example` — mẫu `.env` từng môi trường
- `deploy/caddy/Caddyfile` — Caddyfile cho Linux
- `deploy/systemd/apex.service` — service systemd cho Linux
- `docs/RUNBOOK.md` — xử lý sự cố production
- `docs/ARCHITECTURE.md` — kiến trúc tổng thể (3 tầng + 3 môi trường)
