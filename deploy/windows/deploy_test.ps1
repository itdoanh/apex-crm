<#
.SYNOPSIS
    Deploy code lên môi trường TEST trên Windows Server 2025.

.DESCRIPTION
    Quy trình: git pull branch `test` → build binary trên server → restart Task
    `Apex_Backend_Server_Test` → verify /health.

    ⚠️ KHÔNG đụng production. Script này chỉ đụng `C:\apex-test` và task
       `Apex_Backend_Server_Test`.

.PARAMETER Branch
    Nhánh git cần deploy (mặc định `test`).

.PARAMETER SkipBuild
    Bỏ qua build (dùng khi binary đã được copy lên sẵn).

.EXAMPLE
    powershell -File deploy/windows/deploy_test.ps1
#>
[CmdletBinding()]
param(
    [string]$TestDir = "C:\apex-test",
    [string]$Branch = "test",
    [int]$TestPort = 8081,
    [switch]$SkipBuild,
    [switch]$SkipHealth
)

$ErrorActionPreference = "Continue"

$TaskName = "Apex_Backend_Server_Test"
$BackendDir = Join-Path $TestDir "backend"
$LogsDir = Join-Path $BackendDir "logs"
$ExePath = Join-Path $BackendDir "apex-server.exe"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  DEPLOY → MÔI TRƯỜNG TEST" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  Dir    : $TestDir (branch '$Branch')" -ForegroundColor White
Write-Host "  Task   : $TaskName" -ForegroundColor White
Write-Host "  Port   : $TestPort" -ForegroundColor White

# ------------------------------------------------------------------------------
# 1. GIT PULL
# ------------------------------------------------------------------------------
Write-Host "`n[1/4] Git pull branch '$Branch'..." -ForegroundColor Yellow
Push-Location $TestDir
& git fetch origin 2>&1 | Out-String | Write-Host
& git checkout $Branch 2>&1 | Out-String | Write-Host
& git pull origin $Branch 2>&1 | Out-String | Write-Host
$gitExit = $LASTEXITCODE
Pop-Location

if ($gitExit -ne 0) {
    Write-Host "  [LỖI] Git pull thất bại (exit=$gitExit) - dừng deploy." -ForegroundColor Red
    exit 1
}
Write-Host "  [OK] Code đã cập nhật." -ForegroundColor Green

# ------------------------------------------------------------------------------
# 2. BUILD
# ------------------------------------------------------------------------------
if (-not $SkipBuild) {
    Write-Host "`n[2/4] Build binary..." -ForegroundColor Yellow
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "  [LỖI] Không tìm thấy 'go' - cần Go 1.27+ trên server." -ForegroundColor Red
        exit 1
    }
    Push-Location $BackendDir
    $env:GOMAXPROCS = "1"
    & go build -p 1 -ldflags="-s -w" -trimpath -o $ExePath . 2>&1 | Out-String | Write-Host
    $buildExit = $LASTEXITCODE
    Pop-Location
    Remove-Item Env:\GOMAXPROCS -ErrorAction SilentlyContinue

    if ($buildExit -ne 0) {
        Write-Host "  [LỖI] Build thất bại (exit=$buildExit) - dừng deploy." -ForegroundColor Red
        exit 1
    }
    Write-Host "  [OK] Build thành công." -ForegroundColor Green
} else {
    Write-Host "`n[2/4] Bỏ qua build (-SkipBuild)." -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------
# 3. RESTART TASK
# ------------------------------------------------------------------------------
Write-Host "`n[3/4] Restart task '$TaskName'..." -ForegroundColor Yellow

Get-Process -Name "apex-server" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $ExePath } |
    Stop-Process -Force -ErrorAction SilentlyContinue

Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 5

Write-Host "  [OK] Task đã khởi động lại." -ForegroundColor Green

# ------------------------------------------------------------------------------
# 4. VERIFY
# ------------------------------------------------------------------------------
if (-not $SkipHealth) {
    Write-Host "`n[4/4] Kiểm tra health..." -ForegroundColor Yellow
    $ok = $false
    for ($i = 0; $i -lt 10; $i++) {
        try {
            $r = Invoke-WebRequest -Uri "http://127.0.0.1:$TestPort/health" -UseBasicParsing -TimeoutSec 5
            if ($r.StatusCode -eq 200) { $ok = $true; break }
        } catch {}
        Start-Sleep -Seconds 2
    }
    if ($ok) {
        Write-Host "  [OK] http://127.0.0.1:$TestPort/health = 200" -ForegroundColor Green
    } else {
        Write-Host "  [!] Health check thất bại. Xem log: $LogsDir\server.log" -ForegroundColor Red
        Write-Host "  Last 30 dòng log:" -ForegroundColor DarkGray
        if (Test-Path "$LogsDir\server.log") {
            Get-Content "$LogsDir\server.log" -Tail 30 | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
        }
    }
}

Write-Host "`n=================================================================" -ForegroundColor Cyan
Write-Host "  DEPLOY TEST HOÀN TẤT" -ForegroundColor Green
Write-Host "  https://test.hanghoaphaisinh.net/ads" -ForegroundColor White
Write-Host "=================================================================" -ForegroundColor Cyan
