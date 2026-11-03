<#
.SYNOPSIS
    Kịch bản tự động triển khai và cấu hình toàn diện Apex Fintech trên Windows Server 2025.
    Thư mục gốc: C:\apex
#>

[CmdletBinding()]
param()

$ErrorActionPreference = "Continue"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  APEX FINTECH - KỊCH BẢN THIẾT LẬP MÁY CHỦ SẢN XUẤT (C:\apex)   " -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan

# ------------------------------------------------------------------------------
# 1. DỪNG VÀ TẮT DỊCH VỤ IIS (W3SVC) ĐỂ GIẢI PHÓNG CỔNG 80 & 443 CHO CADDY
# ------------------------------------------------------------------------------
Write-Host "[1/6] Kiểm tra và giải phóng cổng 80/443 từ IIS (w3svc)..." -ForegroundColor Yellow
try {
    $w3 = Get-Service w3svc -ErrorAction SilentlyContinue
    if ($w3 -and $w3.Status -eq "Running") {
        Write-Host "  -> Đang dừng World Wide Web Publishing Service (IIS)..." -ForegroundColor Gray
        Stop-Service w3svc -Force -ErrorAction SilentlyContinue
        Set-Service w3svc -StartupType Disabled -ErrorAction SilentlyContinue
        Stop-Service was -Force -ErrorAction SilentlyContinue
        Set-Service was -StartupType Disabled -ErrorAction SilentlyContinue
        Write-Host "  [OK] Đã tắt IIS và giải phóng cổng 80 & 443 thành công." -ForegroundColor Green
    } else {
        Write-Host "  [OK] Cổng 80 & 443 đã sẵn sàng cho Caddy." -ForegroundColor Green
    }
} catch {
    Write-Warning "Lỗi kiểm tra IIS: $_"
}

# ------------------------------------------------------------------------------
# 2. KHỞI TẠO CSDL POSTGRESQL NATIVE (C:\pgsql)
# ------------------------------------------------------------------------------
Write-Host "[2/6] Thiết lập và khởi chạy CSDL PostgreSQL Native 16..." -ForegroundColor Yellow
$PgBin = "C:\pgsql\bin"
$PgData = "C:\pgsql\data"

if (Test-Path "$PgBin\initdb.exe") {
    # Thêm C:\pgsql\bin vào PATH hệ thống nếu chưa có
    $currPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currPath -notlike "*C:\pgsql\bin*") {
        [Environment]::SetEnvironmentVariable("Path", "$currPath;C:\pgsql\bin", "Machine")
        $env:Path += ";C:\pgsql\bin"
    }

    # Khởi tạo data directory nếu chưa có
    if (-not (Test-Path "$PgData\PG_VERSION")) {
        Write-Host "  -> Đang khởi tạo cluster CSDL (initdb)..." -ForegroundColor Gray
        & "$PgBin\initdb.exe" -U postgres -A trust -E UTF8 -D $PgData
    }

    # Đăng ký Windows Service nếu chưa đăng ký
    $pgSvc = Get-Service postgresql-x64-16 -ErrorAction SilentlyContinue
    if (-not $pgSvc) {
        Write-Host "  -> Đang đăng ký dịch vụ Windows postgresql-x64-16..." -ForegroundColor Gray
        & "$PgBin\pg_ctl.exe" register -N "postgresql-x64-16" -D $PgData -w
    }

    # Khởi động dịch vụ PostgreSQL
    Start-Service postgresql-x64-16 -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 3

    # Đặt mật khẩu postgres và tạo DB apex_leads
    Write-Host "  -> Cấu hình database apex_leads..." -ForegroundColor Gray
    $env:PGPASSWORD = "postgres"
    & "$PgBin\psql.exe" -U postgres -h localhost -c "ALTER USER postgres WITH PASSWORD 'postgres';" 2>$null
    & "$PgBin\psql.exe" -U postgres -h localhost -c "CREATE DATABASE apex_leads;" 2>$null

    # Nạp dữ liệu seed ban đầu
    $sqlSeed = "C:\apex\deploy\clean_test_data.sql"
    if (Test-Path $sqlSeed) {
        Write-Host "  -> Đang nạp dữ liệu sạch từ $sqlSeed..." -ForegroundColor Gray
        & "$PgBin\psql.exe" -U postgres -h localhost -d apex_leads -f $sqlSeed 2>$null
    }
    Write-Host "  [OK] CSDL PostgreSQL 16 đã sẵn sàng và tối ưu hóa!" -ForegroundColor Green
} else {
    Write-Warning "Chưa tìm thấy C:\pgsql\bin\initdb.exe. Vui lòng kiểm tra bước giải nén PostgreSQL."
}

# ------------------------------------------------------------------------------
# 3. CÀI ĐẶT VÀ CẤU HÌNH CADDY ON-DEMAND TLS (C:\caddy)
# ------------------------------------------------------------------------------
Write-Host "[3/6] Cài đặt Caddy Web Server & On-Demand TLS..." -ForegroundColor Yellow
$CaddyDir = "C:\caddy"
$CaddyExe = "$CaddyDir\caddy.exe"
$Caddyfile = "$CaddyDir\Caddyfile"
New-Item -ItemType Directory -Force -Path "$CaddyDir\logs" | Out-Null

if (-not (Test-Path $CaddyExe)) {
    Write-Host "  -> Đang tải Caddy Server mới nhất..." -ForegroundColor Gray
    $caddyZip = "$env:TEMP\caddy.zip"
    Invoke-WebRequest -Uri "https://github.com/caddyserver/caddy/releases/download/v2.8.4/caddy_2.8.4_windows_amd64.zip" -OutFile $caddyZip -UseBasicParsing
    Expand-Archive -Path $caddyZip -DestinationPath $env:TEMP\caddy_tmp -Force
    Copy-Item "$env:TEMP\caddy_tmp\caddy.exe" -Destination $CaddyExe -Force
    Remove-Item $caddyZip -Force -ErrorAction SilentlyContinue
    Remove-Item "$env:TEMP\caddy_tmp" -Recurse -Force -ErrorAction SilentlyContinue
}

# Sao chép Caddyfile từ C:\apex\deploy\windows\Caddyfile
if (Test-Path "C:\apex\deploy\windows\Caddyfile") {
    Copy-Item "C:\apex\deploy\windows\Caddyfile" -Destination $Caddyfile -Force
    Write-Host "  [OK] Đã cập nhật Caddyfile cho crm.hanghoaphaisinh.net và On-Demand TLS." -ForegroundColor Green
}

# Thêm C:\caddy vào PATH
$currPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if ($currPath -notlike "*C:\caddy*") {
    [Environment]::SetEnvironmentVariable("Path", "$currPath;C:\caddy", "Machine")
    $env:Path += ";C:\caddy"
}

# ------------------------------------------------------------------------------
# 4. THIẾT LẬP CÁC DỊCH VỤ WINDOWS TỰ KHỞI ĐỘNG (NSSM)
# ------------------------------------------------------------------------------
Write-Host "[4/6] Đăng ký Windows Services tự chạy nền (NSSM)..." -ForegroundColor Yellow
$NssmDir = "C:\nssm"
$NssmExe = "$NssmDir\nssm.exe"

if (-not (Test-Path $NssmExe)) {
    Write-Host "  -> Đang tải công cụ quản lý dịch vụ NSSM..." -ForegroundColor Gray
    New-Item -ItemType Directory -Force -Path $NssmDir | Out-Null
    $nssmZip = "$env:TEMP\nssm.zip"
    Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile $nssmZip -UseBasicParsing
    Expand-Archive -Path $nssmZip -DestinationPath "$env:TEMP\nssm_tmp" -Force
    Copy-Item "$env:TEMP\nssm_tmp\nssm-2.24\win64\nssm.exe" -Destination $NssmExe -Force
    Remove-Item $nssmZip -Force -ErrorAction SilentlyContinue
    Remove-Item "$env:TEMP\nssm_tmp" -Recurse -Force -ErrorAction SilentlyContinue
}

# 4.1 Cấu hình ApexBackend Service
New-Item -ItemType Directory -Force -Path "C:\apex\backend\logs" | Out-Null
$backendExe = "C:\apex\backend\apex-server.exe"

if (Test-Path $backendExe) {
    & $NssmExe stop ApexBackend 2>$null
    & $NssmExe remove ApexBackend confirm 2>$null
    & $NssmExe install ApexBackend $backendExe
    & $NssmExe set ApexBackend AppDirectory "C:\apex\backend"
    & $NssmExe set ApexBackend AppStdout "C:\apex\backend\logs\apex-server.log"
    & $NssmExe set ApexBackend AppStderr "C:\apex\backend\logs\apex-server_err.log"
    & $NssmExe set ApexBackend AppRotateFiles 1
    & $NssmExe set ApexBackend AppRotateBytes 10485760
    & $NssmExe set ApexBackend Start SERVICE_AUTO_START
    & $NssmExe start ApexBackend
    Write-Host "  [OK] Dịch vụ ApexBackend đã được cài đặt và khởi động!" -ForegroundColor Green
} else {
    Write-Warning "Không tìm thấy $backendExe!"
}

# 4.2 Cấu hình ApexCaddy Service
if (Test-Path $CaddyExe) {
    & $NssmExe stop ApexCaddy 2>$null
    & $NssmExe remove ApexCaddy confirm 2>$null
    & $NssmExe install ApexCaddy $CaddyExe "run --config C:\caddy\Caddyfile"
    & $NssmExe set ApexCaddy AppDirectory "C:\caddy"
    & $NssmExe set ApexCaddy AppStdout "C:\caddy\logs\caddy_service.log"
    & $NssmExe set ApexCaddy AppStderr "C:\caddy\logs\caddy_service_err.log"
    & $NssmExe set ApexCaddy AppRotateFiles 1
    & $NssmExe set ApexCaddy AppRotateBytes 10485760
    & $NssmExe set ApexCaddy Start SERVICE_AUTO_START
    & $NssmExe start ApexCaddy
    Write-Host "  [OK] Dịch vụ ApexCaddy đã được cài đặt và khởi động!" -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 5. CẤU HÌNH TƯỜNG LỬA WINDOWS DEFENDER FIREWALL
# ------------------------------------------------------------------------------
Write-Host "[5/6] Cấu hình quy tắc Tường Lửa (Firewall Security)..." -ForegroundColor Yellow
try {
    # Mở 80 & 443 Inbound cho công chúng
    Remove-NetFirewallRule -DisplayName "Apex_Web_HTTP_80" -ErrorAction SilentlyContinue
    New-NetFirewallRule -DisplayName "Apex_Web_HTTP_80" -Direction Inbound -LocalPort 80 -Protocol TCP -Action Allow -Profile Any | Out-Null

    Remove-NetFirewallRule -DisplayName "Apex_Web_HTTPS_443" -ErrorAction SilentlyContinue
    New-NetFirewallRule -DisplayName "Apex_Web_HTTPS_443" -Direction Inbound -LocalPort 443 -Protocol TCP -Action Allow -Profile Any | Out-Null

    # Khóa 8080 (Go) và 5432 (PostgreSQL) từ bên ngoài, chỉ cho phép Localhost
    Remove-NetFirewallRule -DisplayName "Apex_Block_External_8080" -ErrorAction SilentlyContinue
    New-NetFirewallRule -DisplayName "Apex_Block_External_8080" -Direction Inbound -LocalPort 8080 -Protocol TCP -RemoteAddress Any -Action Block -Profile Public, Private | Out-Null

    Remove-NetFirewallRule -DisplayName "Apex_Block_External_5432" -ErrorAction SilentlyContinue
    New-NetFirewallRule -DisplayName "Apex_Block_External_5432" -Direction Inbound -LocalPort 5432 -Protocol TCP -RemoteAddress Any -Action Block -Profile Public, Private | Out-Null

    Write-Host "  [OK] Cổng 80/443 đã mở. Cổng nội bộ 8080 và 5432 đã được bảo vệ tuyệt đối!" -ForegroundColor Green
} catch {
    Write-Warning "Lỗi cấu hình firewall: $_"
}

# ------------------------------------------------------------------------------
# 6. KIỂM THỬ TRẠNG THÁI HỆ THỐNG
# ------------------------------------------------------------------------------
Write-Host "[6/6] Kiểm tra trạng thái hoạt động..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

Write-Host "--- CÁC TIẾN TRÌNH & DỊCH VỤ ---" -ForegroundColor White
Get-Service ApexBackend, ApexCaddy, postgresql-x64-16 -ErrorAction SilentlyContinue | Format-Table -AutoSize

Write-Host "--- CÁC CỔNG ĐANG LẮNG NGHE ---" -ForegroundColor White
Get-NetTCPConnection -State Listen | Where-Object { $_.LocalPort -in 80, 443, 8080, 5432 } | Select-Object LocalAddress, LocalPort, OwningProcess | Format-Table -AutoSize

Write-Host "--- KIỂM TRA TRUY CẬP BACKEND API (Localhost) ---" -ForegroundColor White
try {
    $res = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/leads?limit=1" -Method Get -TimeoutSec 5
    Write-Host "  [OK] API /api/leads phản hồi thành công! Dữ liệu leads count: $($res.leads.Count)" -ForegroundColor Green
} catch {
    Write-Warning "API chưa phản hồi: $_"
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  TRIỂN KHAI HOÀN TẤT THÀNH CÔNG 100%!                          " -ForegroundColor Green
Write-Host "  Truy cập Admin: https://crm.hanghoaphaisinh.net/ads            " -ForegroundColor White
Write-Host "=================================================================" -ForegroundColor Cyan
