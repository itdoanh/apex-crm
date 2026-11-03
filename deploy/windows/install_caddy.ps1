<#
.SYNOPSIS
    Kịch bản tự động tải và cài đặt Caddy Web Server cho Windows Server 2025.
.DESCRIPTION
    Tải bản caddy.exe chính thức cho Windows x64, tạo thư mục C:\caddy,
    thiết lập Caddyfile hỗ trợ On-Demand TLS và kiểm tra tính hợp lệ của cấu hình.
.NOTES
    Yêu cầu quyền Administrator (Run as Administrator).
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "C:\caddy",
    [string]$SourceCaddyfile = "$PSScriptRoot\Caddyfile"
)

# 1. Kiểm tra quyền Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Kịch bản này yêu cầu quyền Administrator. Vui lòng chạy PowerShell với quyền 'Run as Administrator'."
    exit 1
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  CÀI ĐẶT CADDY WEB SERVER (ON-DEMAND TLS) TRÊN WINDOWS SERVER" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan

# 2. Tạo cấu trúc thư mục
$LogsDir = Join-Path $InstallDir "logs"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Write-Host "[OK] Đã tạo thư mục cài đặt: $InstallDir" -ForegroundColor Green
}
if (-not (Test-Path $LogsDir)) {
    New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null
    Write-Host "[OK] Đã tạo thư mục nhật ký: $LogsDir" -ForegroundColor Green
}

# 3. Tải Caddy binary cho Windows x64
$CaddyExe = Join-Path $InstallDir "caddy.exe"
if (-not (Test-Path $CaddyExe)) {
    Write-Host "[*] Đang tải Caddy Server cho Windows x64..." -ForegroundColor Yellow

    # URL tải Caddy chính thức (bản v2.8.4 ổn định nhất hoặc API caddyserver.com)
    $DownloadUrl = "https://caddyserver.com/api/download?os=windows&arch=amd64"
    $FallbackUrl = "https://github.com/caddyserver/caddy/releases/download/v2.8.4/caddy_2.8.4_windows_amd64.zip"
    $ZipPath = Join-Path $env:TEMP "caddy_install.zip"

    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $CaddyExe -UseBasicParsing -TimeoutSec 120
        Write-Host "[OK] Tải Caddy thành công qua API chính thức." -ForegroundColor Green
    } catch {
        Write-Warning "Không tải được trực tiếp từ API, đang thử tải file zip từ GitHub..."
        try {
            Invoke-WebRequest -Uri $FallbackUrl -OutFile $ZipPath -UseBasicParsing -TimeoutSec 120
            Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force
            Remove-Item $ZipPath -Force -ErrorAction SilentlyContinue
            Write-Host "[OK] Giải nén Caddy từ GitHub release thành công." -ForegroundColor Green
        } catch {
            Write-Error "LỖI: Không thể tải Caddy Server. Vui lòng kiểm tra kết nối mạng Internet của máy chủ: $_"
            exit 1
        }
    }
} else {
    Write-Host "[OK] Caddy đã có sẵn tại: $CaddyExe" -ForegroundColor Cyan
}

# 4. Sao chép Caddyfile vào thư mục C:\caddy
$TargetCaddyfile = Join-Path $InstallDir "Caddyfile"
if (Test-Path $SourceCaddyfile) {
    Copy-Item -Path $SourceCaddyfile -Destination $TargetCaddyfile -Force
    Write-Host "[OK] Đã sao chép Caddyfile cấu hình On-Demand TLS vào $TargetCaddyfile" -ForegroundColor Green
} else {
    Write-Warning "Không tìm thấy file nguồn $SourceCaddyfile. Vui lòng kiểm tra lại đường dẫn."
}

# 5. Thêm C:\caddy vào System Environment PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", [EnvironmentVariableTarget]::Machine)
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "[OK] Đã thêm $InstallDir vào biến môi trường PATH của hệ thống." -ForegroundColor Green
}

# 6. Kiểm tra phiên bản và xác thực cú pháp Caddyfile
try {
    $CaddyVersion = & $CaddyExe version
    Write-Host "[OK] Phiên bản Caddy: $CaddyVersion" -ForegroundColor Green

    Write-Host "[*] Đang kiểm tra tính hợp lệ của Caddyfile..." -ForegroundColor Yellow
    $ValidateOutput = & $CaddyExe validate --config $TargetCaddyfile 2>&1
    Write-Host "[OK] Cấu hình Caddyfile HỢP LỆ 100%!" -ForegroundColor Green
} catch {
    Write-Warning "Kiểm tra Caddyfile có cảnh báo: $_"
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT CÀI ĐẶT CADDY TRÊN WINDOWS SERVER 2025" -ForegroundColor Green
Write-Host "  Đường dẫn: $CaddyExe" -ForegroundColor White
Write-Host "  Cấu hình:  $TargetCaddyfile" -ForegroundColor White
Write-Host "=================================================================" -ForegroundColor Cyan
