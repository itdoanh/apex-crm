<#
.SYNOPSIS
    Dựng môi trường TEST cho Apex Fintech trên Windows Server 2025.

.DESCRIPTION
    Môi trường test chạy SONG SONG với production trên cùng server, nhưng
    hoàn toàn tách biệt:

      | Hạng mục        | Production              | Test
      |-----------------|-------------------------|-----------------------------
      | Code            | C:\apex (branch `main`) | C:\apex-test (branch `test`)
      | Backend port    | 8080                    | 8081
      | Database        | apex_leads              | apex_leads_test
      | Task Scheduler  | Apex_Backend_Server     | Apex_Backend_Server_Test
      | Admin domain    | crm.hanghoaphaisinh.net | test.hanghoaphaisinh.net
      | Đồng bộ ngoài   | Meta CAPI + Sheets       | TẮT (cô lập)

    Script này (idempotent — chạy lại được):
      1. Tạo database `apex_leads_test` nếu chưa có.
      2. Clone/cập nhật code tại `C:\apex-test` (branch `test`).
      3. Tạo file `.env` cho môi trường test (KHÔNG đè file đã có).
      4. Copy `service-account.json` sang `C:\apex-test\backend`.
      5. Đăng ký Task Scheduler `Apex_Backend_Server_Test` (WD = C:\apex-test\backend).
      6. Build binary (server ĐÃ CÀI Go 1.27.4 từ 2026-11-15).
      7. Khởi động task + kiểm tra `/health`.

    ⚠️ KHÔNG đụng vào production: không restart `Apex_Backend_Server`, không sửa
       `C:\apex`, không sửa database `apex_leads`.

.PARAMETER SkipBuild
    Bỏ qua bước build binary (dùng khi binary đã có sẵn).

.PARAMETER SkipStart
    Bỏ qua bước khởi động task (dùng khi muốn chỉ setup cấu hình).

.NOTES
    Yêu cầu quyền Administrator.
#>
[CmdletBinding()]
param(
    [string]$TestDir = "C:\apex-test",
    [string]$TestBranch = "test",
    [string]$TestDB = "apex_leads_test",
    [int]$TestPort = 8081,
    [string]$TestDomain = "test.hanghoaphaisinh.net",
    [string]$RepoUrl = "https://github.com/itdoanh/apex-crm.git",
    [switch]$SkipBuild,
    [switch]$SkipStart
)

$ErrorActionPreference = "Continue"

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Cần quyền Administrator. Chạy với 'Run as Administrator'."
    exit 1
}

$TaskName = "Apex_Backend_Server_Test"
$BackendDir = Join-Path $TestDir "backend"
$LogsDir = Join-Path $BackendDir "logs"
$ExePath = Join-Path $BackendDir "apex-server.exe"
$PsqlBin = "C:\pgsql\bin\psql.exe"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  DỰNG MÔI TRƯỜNG TEST — Apex Fintech" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  Code      : $TestDir (branch '$TestBranch')" -ForegroundColor White
Write-Host "  Port      : $TestPort" -ForegroundColor White
Write-Host "  Database  : $TestDB" -ForegroundColor White
Write-Host "  Domain    : https://$TestDomain" -ForegroundColor White
Write-Host "  Task      : $TaskName" -ForegroundColor White

# ------------------------------------------------------------------------------
# 1. TẠO DATABASE apex_leads_test (nếu chưa có)
# ------------------------------------------------------------------------------
Write-Host "`n[1/7] Kiểm tra database '$TestDB'..." -ForegroundColor Yellow

if (-not (Test-Path $PsqlBin)) {
    Write-Host "  [!] Không tìm thấy psql tại $PsqlBin - BỎ QUA bước tạo DB." -ForegroundColor Red
    Write-Host "      Hãy tạo DB thủ công: createdb -U postgres $TestDB" -ForegroundColor Red
} else {
    $env:PGPASSWORD = "postgres"
    $dbExists = & $PsqlBin -U postgres -h localhost -tAc "SELECT 1 FROM pg_database WHERE datname='$TestDB'" 2>$null
    if ($dbExists -eq "1") {
        Write-Host "  [OK] Database '$TestDB' đã tồn tại." -ForegroundColor Green
    } else {
        & $PsqlBin -U postgres -h localhost -c "CREATE DATABASE $TestDB" 2>&1 | Out-String | Write-Host
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  [OK] Đã tạo database '$TestDB'." -ForegroundColor Green
        } else {
            Write-Host "  [LỖI] Không tạo được database '$TestDB'." -ForegroundColor Red
        }
    }
    Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue
}

# ------------------------------------------------------------------------------
# 2. CLONE / CẬP NHẬT CODE
# ------------------------------------------------------------------------------
Write-Host "`n[2/7] Chuẩn bị mã nguồn tại $TestDir..." -ForegroundColor Yellow

if (-not (Test-Path (Join-Path $TestDir ".git"))) {
    if (Test-Path $TestDir) {
        Write-Host "  [*] Thư mục đã tồn tại nhưng chưa phải git clone - backup rồi clone lại." -ForegroundColor Yellow
        $backupDir = "$TestDir.bak_$(Get-Date -Format 'yyyyMMdd_HHmmss')"
        Rename-Item $TestDir $backupDir -Force -ErrorAction SilentlyContinue
    }
    Write-Host "  [*] Đang clone từ $RepoUrl (branch '$TestBranch')..." -ForegroundColor Yellow
    & git clone --branch $TestBranch $RepoUrl $TestDir 2>&1 | Out-String | Write-Host
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [LỖI] Clone thất bại. Kiểm tra mạng GitHub." -ForegroundColor Red
        exit 1
    }
    Write-Host "  [OK] Đã clone branch '$TestBranch'." -ForegroundColor Green
} else {
    Write-Host "  [*] Đã có git clone - đang fetch + checkout '$TestBranch'..." -ForegroundColor Yellow
    Push-Location $TestDir
    & git fetch origin 2>&1 | Out-String | Write-Host
    & git checkout $TestBranch 2>&1 | Out-String | Write-Host
    & git pull origin $TestBranch 2>&1 | Out-String | Write-Host
    Pop-Location
    Write-Host "  [OK] Đã cập nhật code branch '$TestBranch'." -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 3. TẠO FILE .env CHO MÔI TRƯỜNG TEST
# ------------------------------------------------------------------------------
Write-Host "`n[3/7] Cấu hình file .env..." -ForegroundColor Yellow

$envFile = Join-Path $TestDir ".env"
if (Test-Path $envFile) {
    Write-Host "  [OK] File .env đã tồn tại - giữ nguyên (KHÔNG đè)." -ForegroundColor Green
} else {
    $envTemplate = Join-Path $TestDir "deploy\windows\env.test.example"
    if (-not (Test-Path $envTemplate)) {
        Write-Host "  [!] Không tìm thấy template $envTemplate - tạo .env từ nội dung mặc định." -ForegroundColor Yellow
        $envContent = @"
# AUTO-GENERATED cho môi trường test — $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
APEX_ENV=test
PORT=$TestPort
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=$TestDB
DB_SSLMODE=disable
ALLOW_ORIGINS=*
ADMIN_KEY=apex080
CHALLENGE_SECRET=apex_fintech_test_secret_2026_do_not_use_in_prod
APEX_ADMIN_HOSTS=$TestDomain,localhost,127.0.0.1
APEX_ON_DEMAND_DOMAINS=$TestDomain,ns1.hanghoaphaisinh.net,ns2.hanghoaphaisinh.net
APEX_DISABLE_EXTERNAL_SYNC=true
GOOGLE_SERVICE_ACCOUNT_FILE=service-account.json
GOOGLE_BOT_EMAIL=sheet-bot@rinva-drive-tool.iam.gserviceaccount.com
"@
    } else {
        $envContent = Get-Content $envTemplate -Raw -Encoding UTF8
        # Thay thế giá trị theo tham số script
        $envContent = $envContent -replace '(?m)^PORT=.*$', "PORT=$TestPort"
        $envContent = $envContent -replace '(?m)^DB_NAME=.*$', "DB_NAME=$TestDB"
    }
    # Ghi file UTF-8 KHÔNG BOM (Go đọc .env bằng bufio → BOM sẽ hỏng key đầu tiên)
    [System.IO.File]::WriteAllText($envFile, $envContent, (New-Object System.Text.UTF8Encoding $false))
    Write-Host "  [OK] Đã tạo $envFile" -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 4. COPY SERVICE ACCOUNT (Google Sheets bot)
# ------------------------------------------------------------------------------
Write-Host "`n[4/7] Copy service-account.json..." -ForegroundColor Yellow

$srcSA = "C:\apex\backend\service-account.json"
$dstSA = Join-Path $BackendDir "service-account.json"
if ((Test-Path $srcSA) -and (-not (Test-Path $dstSA))) {
    Copy-Item $srcSA $dstSA -Force
    Write-Host "  [OK] Đã copy service-account.json sang môi trường test." -ForegroundColor Green
} elseif (Test-Path $dstSA) {
    Write-Host "  [OK] service-account.json đã tồn tại - giữ nguyên." -ForegroundColor Green
} else {
    Write-Host "  [!] Không tìm thấy service-account.json (bỏ qua - Sheets sẽ không dùng được)." -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------
# 5. ĐĂNG KÝ TASK SCHEDULER
# ------------------------------------------------------------------------------
Write-Host "`n[5/7] Đăng ký Task Scheduler '$TaskName'..." -ForegroundColor Yellow

New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

$Principal = New-ScheduledTaskPrincipal -UserId "NT AUTHORITY\SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -RestartCount 5 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit (New-TimeSpan -Days 365) `
    -MultipleInstances IgnoreNew
$Trigger = New-ScheduledTaskTrigger -AtStartup

# ⚠️ Bọc bằng `cmd /c ... >> log 2>&1` — Task Scheduler nuốt stdout/stderr
#    ⇒ mọi lỗi Go sẽ biến mất hoàn toàn.
$Action = New-ScheduledTaskAction -Execute "cmd.exe" `
    -Argument "/c `"`"$ExePath`" >> `"$LogsDir\server.log`" 2>&1`"" `
    -WorkingDirectory $BackendDir

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings | Out-Null
Write-Host "  [OK] Task '$TaskName' đã đăng ký (WD=$BackendDir)." -ForegroundColor Green

# ------------------------------------------------------------------------------
# 6. BUILD BINARY (server đã có Go 1.27.4)
# ------------------------------------------------------------------------------
if (-not $SkipBuild) {
    Write-Host "`n[6/7] Build binary..." -ForegroundColor Yellow

    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        Write-Host "  [!] Không tìm thấy lệnh 'go' trong PATH - BỎ QUA build." -ForegroundColor Red
        Write-Host "      Cài Go 1.27+ hoặc build ở LOCAL rồi copy binary sang." -ForegroundColor Red
    } else {
        Write-Host "  [*] go version: $(& go version)" -ForegroundColor DarkGray
        Push-Location $BackendDir
        $env:GOMAXPROCS = "1"
        $env:GOGC = "25"
        & go build -p 1 -ldflags="-s -w" -trimpath -o $ExePath . 2>&1 | Out-String | Write-Host
        $buildExit = $LASTEXITCODE
        Pop-Location
        Remove-Item Env:\GOMAXPROCS -ErrorAction SilentlyContinue
        Remove-Item Env:\GOGC -ErrorAction SilentlyContinue

        if ($buildExit -eq 0) {
            $size = [math]::Round((Get-Item $ExePath).Length / 1MB, 1)
            Write-Host "  [OK] Build thành công: $ExePath ($size MB)" -ForegroundColor Green
        } else {
            Write-Host "  [LỖI] Build thất bại (exit=$buildExit)." -ForegroundColor Red
        }
    }
} else {
    Write-Host "`n[6/7] Bỏ qua build (-SkipBuild)." -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------
# 7. KHỞI ĐỘNG + KIỂM TRA
# ------------------------------------------------------------------------------
if (-not $SkipStart) {
    Write-Host "`n[7/7] Khởi động task '$TaskName'..." -ForegroundColor Yellow

    Get-Process -Name "apex-server" -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $ExePath } |
        Stop-Process -Force -ErrorAction SilentlyContinue

    Start-ScheduledTask -TaskName $TaskName
    Start-Sleep -Seconds 5

    $health = $null
    try {
        $health = Invoke-WebRequest -Uri "http://127.0.0.1:$TestPort/health" -UseBasicParsing -TimeoutSec 10
    } catch {
        Write-Host "  [!] Health check chưa OK: $($_.Exception.Message)" -ForegroundColor Yellow
    }

    if ($health -and $health.StatusCode -eq 200) {
        Write-Host "  [OK] Backend test đang chạy: http://127.0.0.1:$TestPort/health" -ForegroundColor Green
    } else {
        Write-Host "  [!] Chưa phản hồi. Xem log: $LogsDir\server.log" -ForegroundColor Yellow
    }

    Write-Host "`n  Log môi trường test: $LogsDir\server.log" -ForegroundColor DarkGray
}

Write-Host "`n=================================================================" -ForegroundColor Cyan
Write-Host "  MÔI TRƯỜNG TEST ĐÃ SẴN SÀNG" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  Admin CRM : https://$TestDomain/ads" -ForegroundColor White
Write-Host "  Health    : http://127.0.0.1:$TestPort/health" -ForegroundColor White
Write-Host "  Log       : $LogsDir\server.log" -ForegroundColor White
Write-Host ""
Write-Host "  CÀN LÀM TIẾP (nếu chưa có trong Caddyfile):" -ForegroundColor Yellow
Write-Host "    Thêm site block test.$TestDomain -> reverse_proxy 127.0.0.1:$TestPort" -ForegroundColor Yellow
Write-Host "    rồi: C:\caddy\caddy.exe validate --config C:\caddy\Caddyfile" -ForegroundColor Yellow
Write-Host "          C:\caddy\caddy.exe reload --config C:\caddy\Caddyfile" -ForegroundColor Yellow
Write-Host "=================================================================" -ForegroundColor Cyan
