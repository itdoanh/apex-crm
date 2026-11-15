<#
.SYNOPSIS
    Promote (deploy) code đã được kiểm thử từ môi trường TEST sang PRODUCTION.

.DESCRIPTION
    Quy trình chuẩn — KHÔNG tự động merge branch, chỉ deploy code đã merge:

      1. Kiểm tra branch `main` đã chứa code của branch `test` (git branch --contains)
      2. `git pull origin main` trên `C:\apex`
      3. Build binary production
      4. Restart Task `Apex_Backend_Server`
      5. Verify `/health` + `crm.hanghoaphaisinh.net/health`

    ⚠️ Script này ĐỤNG production — chỉ chạy khi anh itdoanh đã xác nhận.

.PARAMETER SkipBuild
    Bỏ qua build (dùng khi binary đã được copy lên sẵn).

.PARAMETER Force
    Bỏ qua bước kiểm tra branch `test` đã merge vào `main` chưa.

.EXAMPLE
    powershell -File deploy/windows/deploy_prod.ps1
#>
[CmdletBinding()]
param(
    [string]$ProdDir = "C:\apex",
    [string]$MainBranch = "main",
    [string]$TestBranch = "test",
    [int]$ProdPort = 8080,
    [switch]$SkipBuild,
    [switch]$SkipHealth,
    [switch]$Force
)

$ErrorActionPreference = "Continue"

$TaskName = "Apex_Backend_Server"
$BackendDir = Join-Path $ProdDir "backend"
$LogsDir = Join-Path $BackendDir "logs"
$ExePath = Join-Path $BackendDir "apex-server.exe"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  DEPLOY → PRODUCTION  ⚠️" -ForegroundColor Yellow
Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  Dir  : $ProdDir (branch '$MainBranch')" -ForegroundColor White
Write-Host "  Task : $TaskName" -ForegroundColor White
Write-Host "  Port : $ProdPort" -ForegroundColor White

# ------------------------------------------------------------------------------
# 1. KIỂM TRA CODE ĐÃ ĐƯỢC TEST & MERGE CHƯA
# ------------------------------------------------------------------------------
Write-Host "`n[1/5] Kiểm tra trạng thái merge..." -ForegroundColor Yellow

Push-Location $ProdDir
& git fetch origin 2>&1 | Out-Null
$merged = & git branch -r --contains "origin/$TestBranch" 2>&1
Pop-Location

$isMerged = $false
if ($merged -match [regex]::Escape("origin/$MainBranch")) {
    $isMerged = $true
}

if ($isMerged) {
    Write-Host "  [OK] Branch '$TestBranch' đã được merge vào '$MainBranch'." -ForegroundColor Green
} elseif ($Force) {
    Write-Host "  [!] Chưa merge nhưng -Force được bật - tiếp tục." -ForegroundColor Yellow
} else {
    Write-Host "  [LỖI] Branch '$TestBranch' CHƯA được merge vào '$MainBranch'." -ForegroundColor Red
    Write-Host "         Test OK rồi thì merge trước trên máy local:" -ForegroundColor Yellow
    Write-Host "           git checkout main && git merge test && git push origin main" -ForegroundColor Yellow
    exit 1
}

# ------------------------------------------------------------------------------
# 2. GIT PULL
# ------------------------------------------------------------------------------
Write-Host "`n[2/5] Git pull branch '$MainBranch'..." -ForegroundColor Yellow
Push-Location $ProdDir
& git pull origin $MainBranch 2>&1 | Out-String | Write-Host
$gitExit = $LASTEXITCODE
Pop-Location

if ($gitExit -ne 0) {
    Write-Host "  [LỖI] Git pull thất bại - dừng deploy." -ForegroundColor Red
    exit 1
}
Write-Host "  [OK] Code đã cập nhật." -ForegroundColor Green

# ------------------------------------------------------------------------------
# 3. BUILD
# ------------------------------------------------------------------------------
if (-not $SkipBuild) {
    Write-Host "`n[3/5] Build binary..." -ForegroundColor Yellow
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
        Write-Host "  [LỖI] Build thất bại - dừng deploy (production giữ nguyên binary cũ)." -ForegroundColor Red
        exit 1
    }
    Write-Host "  [OK] Build thành công." -ForegroundColor Green
} else {
    Write-Host "`n[3/5] Bỏ qua build (-SkipBuild)." -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------
# 4. RESTART TASK
# ------------------------------------------------------------------------------
Write-Host "`n[4/5] Restart task '$TaskName'..." -ForegroundColor Yellow

Get-Process -Name "apex-server" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $ExePath } |
    Stop-Process -Force -ErrorAction SilentlyContinue

Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 5

Write-Host "  [OK] Task đã khởi động lại." -ForegroundColor Green

# ------------------------------------------------------------------------------
# 5. VERIFY
# ------------------------------------------------------------------------------
if (-not $SkipHealth) {
    Write-Host "`n[5/5] Kiểm tra health..." -ForegroundColor Yellow
    $ok = $false
    for ($i = 0; $i -lt 10; $i++) {
        try {
            $r = Invoke-WebRequest -Uri "http://127.0.0.1:$ProdPort/health" -UseBasicParsing -TimeoutSec 5
            if ($r.StatusCode -eq 200) { $ok = $true; break }
        } catch {}
        Start-Sleep -Seconds 2
    }
    if ($ok) {
        Write-Host "  [OK] http://127.0.0.1:$ProdPort/health = 200" -ForegroundColor Green
    } else {
        Write-Host "  [!] Health check thất bại. Xem log: $LogsDir\server.log" -ForegroundColor Red
        if (Test-Path "$LogsDir\server.log") {
            Get-Content "$LogsDir\server.log" -Tail 30 | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
        }
    }

    # Verify public domain
    try {
        $r = Invoke-WebRequest -Uri "https://crm.hanghoaphaisinh.net/health" -UseBasicParsing -TimeoutSec 15
        Write-Host "  [OK] https://crm.hanghoaphaisinh.net/health = $($r.StatusCode)" -ForegroundColor Green
    } catch {
        Write-Host "  [!] https://crm.hanghoaphaisinh.net/health lỗi: $($_.Exception.Message)" -ForegroundColor Yellow
    }
}

Write-Host "`n=================================================================" -ForegroundColor Cyan
Write-Host "  DEPLOY PRODUCTION HOÀN TẤT" -ForegroundColor Green
Write-Host "  https://crm.hanghoaphaisinh.net/ads" -ForegroundColor White
Write-Host "=================================================================" -ForegroundColor Cyan
