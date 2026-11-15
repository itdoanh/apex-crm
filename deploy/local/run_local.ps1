<#
.SYNOPSIS
    Chạy backend Apex Fintech ở môi trường LOCAL (máy dev).

.DESCRIPTION
    Môi trường local dùng PostgreSQL trong Docker (container `apex-postgres`).
    Có 2 DB sẵn có (khớp với server):
      - `apex_leads`      ← môi trường "local"  (APEX_ENV=local)
      - `apex_leads_test` ← môi trường "test"   (APEX_ENV=test, port 8081)

    Script này chỉ chạy backend (binary Go) — KHÔNG chạy Caddy (Caddy chỉ
    chạy trên server production).

.PARAMETER Env
    `local` (mặc định) hoặc `test`.

    - `local`: DB `apex_leads`, PORT 8080, admin host `localhost`.
    - `test` : DB `apex_leads_test`, PORT 8081, admin host `localhost`.
      Cả 2 đều TẮT đồng bộ ra ngoài (Meta CAPI / Google Sheets) ⇒ lead thử
      ở local KHÔNG làm nhiễu KPI doanh thu thật.

.EXAMPLE
    powershell -File deploy/local/run_local.ps1
    powershell -File deploy/local/run_local.ps1 -Env test

.NOTES
    Yêu cầu Docker đang chạy (container `apex-postgres`).
#>
[CmdletBinding()]
param(
    [ValidateSet("local", "test")]
    [string]$Env = "local",
    [switch]$Build,
    [switch]$NoSync
)

$ErrorActionPreference = "Continue"

$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$BackendDir = Join-Path $Root "backend"
$ExePath = Join-Path $BackendDir "apex-server.exe"
$LogsDir = Join-Path $BackendDir "logs"
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

if ($Env -eq "test") {
    $Port = 8081
    $DbName = "apex_leads_test"
    $ApexEnv = "test"
    $Domain = "test.hanghoaphaisinh.net"
} else {
    $Port = 8080
    $DbName = "apex_leads"
    $ApexEnv = "local"
    $Domain = "localhost"
}

Write-Host "=============================================================" -ForegroundColor Cyan
Write-Host "  CHẠY LOCAL — Apex Fintech (môi trường '$Env')" -ForegroundColor Green
Write-Host "=============================================================" -ForegroundColor Cyan
Write-Host "  Port      : $Port" -ForegroundColor White
Write-Host "  Database  : $DbName (Docker `apex-postgres` @ localhost:5432)" -ForegroundColor White
Write-Host "  Admin host: $Domain" -ForegroundColor White

# ------------------------------------------------------------------------------
# 1. KIỂM TRA DATABASE
# ------------------------------------------------------------------------------
Write-Host "`n[1/4] Kiểm tra database '$DbName'..." -ForegroundColor Yellow

$container = docker ps --filter "name=apex-postgres" --format "{{.Names}}" 2>$null
if (-not $container) {
    Write-Host "  [LỖI] Container 'apex-postgres' chưa chạy." -ForegroundColor Red
    Write-Host "         Khởi động bằng: docker compose up -d postgres" -ForegroundColor Red
    exit 1
}

$dbExists = docker exec apex-postgres psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$DbName'" 2>$null
if ($dbExists -ne "1") {
    Write-Host "  [!] Database '$DbName' chưa tồn tại - đang tạo..." -ForegroundColor Yellow
    docker exec apex-postgres psql -U postgres -c "CREATE DATABASE $DbName" 2>&1 | Out-String | Write-Host
} else {
    Write-Host "  [OK] Database '$DbName' đã tồn tại." -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 2. GHI FILE .env CHO LOCAL
# ------------------------------------------------------------------------------
Write-Host "`n[2/4] Cập nhật file .env..." -ForegroundColor Yellow

$envFile = Join-Path $Root ".env"
$envContent = @"
# ==============================================================
# APEX FINTECH — CẤU HÌNH LOCAL (máy dev)
# Sinh tự động bởi deploy/local/run_local.ps1 — môi trường '$Env'
# KHÔNG commit (đã có trong .gitignore).
# ==============================================================
APEX_ENV=$ApexEnv
PORT=$Port

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=$DbName
DB_SSLMODE=disable

ALLOW_ORIGINS=*
ADMIN_KEY=apex080
CHALLENGE_SECRET=apex_fintech_local_secret_do_not_use_in_prod

APEX_ADMIN_HOSTS=localhost,127.0.0.1
APEX_ON_DEMAND_DOMAINS=localhost,127.0.0.1

# Local KHÔNG đẩy dữ liệu lên Meta CAPI / Google Sheets production.
APEX_DISABLE_EXTERNAL_SYNC=true

GOOGLE_SERVICE_ACCOUNT_FILE=service-account.json
GOOGLE_BOT_EMAIL=sheet-bot@rinva-drive-tool.iam.gserviceaccount.com
"@

# Ghi UTF-8 KHÔNG BOM — Go đọc .env bằng bufio ⇒ BOM sẽ hỏng key đầu tiên.
[System.IO.File]::WriteAllText($envFile, $envContent, (New-Object System.Text.UTF8Encoding $false))
Write-Host "  [OK] Đã ghi $envFile" -ForegroundColor Green

# ------------------------------------------------------------------------------
# 3. BUILD (nếu yêu cầu)
# ------------------------------------------------------------------------------
if ($Build) {
    Write-Host "`n[3/4] Build binary..." -ForegroundColor Yellow
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "  [LỖI] Không tìm thấy lệnh 'go' trong PATH." -ForegroundColor Red
        exit 1
    }
    Push-Location $BackendDir
    $env:GOMAXPROCS = "1"
    & go build -p 1 -ldflags="-s -w" -trimpath -o $ExePath . 2>&1 | Out-String | Write-Host
    $buildExit = $LASTEXITCODE
    Pop-Location
    Remove-Item Env:\GOMAXPROCS -ErrorAction SilentlyContinue
    if ($buildExit -ne 0) {
        Write-Host "  [LỖI] Build thất bại (exit=$buildExit)." -ForegroundColor Red
        exit 1
    }
    Write-Host "  [OK] Build thành công." -ForegroundColor Green
} else {
    Write-Host "`n[3/4] Bỏ qua build (dùng -Build nếu muốn build lại)." -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------
# 4. CHẠY BACKEND
# ------------------------------------------------------------------------------
Write-Host "`n[4/4] Khởi động backend..." -ForegroundColor Yellow

# Dừng instance local cũ (nếu có) để không tranh port.
Get-Process -Name "apex-server" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $ExePath } |
    Stop-Process -Force -ErrorAction SilentlyContinue

$env:PORT = "$Port"
$env:DB_NAME = $DbName
$env:APEX_ENV = $ApexEnv

Write-Host "  Đang chạy `apex-server.exe` — nhấn Ctrl+C để dừng." -ForegroundColor Yellow
Write-Host "  Log file: $BackendDir\logs\server.log" -ForegroundColor DarkGray

Push-Location $BackendDir
try {
    & $ExePath 2>&1 | Tee-Object -FilePath (Join-Path $BackendDir "logs\server.log")
} finally {
    Pop-Location
}
