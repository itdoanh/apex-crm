<#
.SYNOPSIS
    Kịch bản hướng dẫn và tự động thiết lập CSDL PostgreSQL Native trên Windows Server 2025.
.DESCRIPTION
    Tối ưu hóa dung lượng ổ đĩa tối đa (chỉ tốn ~200MB thay vì cài Docker Desktop tốn 10-15GB).
    Khởi tạo cơ sở dữ liệu 'apex_leads' và nạp cấu hình sạch (Clean Seed).
.NOTES
    Yêu cầu quyền Administrator.
#>

[CmdletBinding()]
param(
    [string]$DbUser = "postgres",
    [string]$DbPassword = "postgres",
    [string]$DbName = "apex_leads",
    [string]$DbPort = "5432"
)

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Kịch bản này yêu cầu quyền Administrator. Vui lòng chạy PowerShell với quyền 'Run as Administrator'."
    exit 1
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  THIẾT LẬP POSTGRESQL NATIVE CHO WINDOWS SERVER 2025" -ForegroundColor Green
Write-Host "  (Giải pháp tối ưu siêu nhẹ ~200MB cho ổ đĩa 30GB)" -ForegroundColor White
Write-Host "=================================================================" -ForegroundColor Cyan

# 1. Kiểm tra xem PostgreSQL đã được cài đặt trên máy chủ chưa
$PsqlCmd = Get-Command "psql" -ErrorAction SilentlyContinue
$PgService = Get-Service -Name "postgresql*" -ErrorAction SilentlyContinue

if (-not $PsqlCmd -and -not $PgService) {
    Write-Host "[*] PostgreSQL chưa có trên máy. Bạn có 2 cách cài đặt cực nhanh:" -ForegroundColor Yellow
    Write-Host "  - CÁCH 1 (Tải trực tiếp qua trình duyệt):" -ForegroundColor White
    Write-Host "    Vào https://www.enterprisedb.com/downloads/postgres-postgresql-downloads" -ForegroundColor Cyan
    Write-Host "    Tải bản PostgreSQL 16 cho Windows x86-64, chạy bộ cài đặt, đặt mật khẩu là '$DbPassword'." -ForegroundColor White
    Write-Host "  - CÁCH 2 (Dùng lệnh winget nếu có):" -ForegroundColor White
    Write-Host "    winget install PostgreSQL.PostgreSQL.16 --silent" -ForegroundColor Cyan
    Write-Host "-----------------------------------------------------------------" -ForegroundColor Gray

    $Choice = Read-Host "Bạn có muốn tải tự động bộ cài đặt PostgreSQL 16 installer (khoảng 300MB) về máy không? (Y/N)"
    if ($Choice -eq 'Y' -or $Choice -eq 'y') {
        $InstallerPath = Join-Path $env:TEMP "postgresql-16-installer.exe"
        $DownloadUrl = "https://sbp.enterprisedb.com/getfile.jsp?fileid=1258897"
        Write-Host "[*] Đang tải PostgreSQL 16 installer vào $InstallerPath..." -ForegroundColor Yellow
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $DownloadUrl -OutFile $InstallerPath -UseBasicParsing -TimeoutSec 300
            Write-Host "[OK] Tải hoàn tất! Đang khởi chạy trình cài đặt..." -ForegroundColor Green
            Start-Process -FilePath $InstallerPath
            Write-Host "[*] Vui lòng hoàn tất các bước trên màn hình cài đặt của PostgreSQL (đặt mật khẩu là '$DbPassword')." -ForegroundColor Yellow
        } catch {
            Write-Warning "Không tải được file installer: $_. Vui lòng tải thủ công qua trình duyệt."
        }
    }
} else {
    Write-Host "[OK] Đã phát hiện PostgreSQL trên máy chủ!" -ForegroundColor Green
}

# 2. Khởi tạo Database và nạp dữ liệu sạch
Write-Host "[*] Đang tạo cơ sở dữ liệu '$DbName'..." -ForegroundColor Yellow

$env:PGPASSWORD = $DbPassword
try {
    # Tạo database apex_leads nếu chưa có
    & psql -U $DbUser -h localhost -p $DbPort -c "CREATE DATABASE $DbName;" 2>$null
    Write-Host "[OK] Đã khởi tạo CSDL $DbName." -ForegroundColor Green
} catch {
    Write-Warning "Database có thể đã tồn tại: $_"
}

# 3. Nạp kịch bản làm sạch dữ liệu và cấu hình chuẩn
$CleanSqlPath = Resolve-Path (Join-Path $PSScriptRoot "..\clean_test_data.sql") -ErrorAction SilentlyContinue
if ($CleanSqlPath -and (Test-Path $CleanSqlPath)) {
    Write-Host "[*] Đang nạp cấu hình và làm sạch dữ liệu từ clean_test_data.sql..." -ForegroundColor Yellow
    try {
        & psql -U $DbUser -h localhost -p $DbPort -d $DbName -f $CleanSqlPath.Path
        Write-Host "[OK] Nạp dữ liệu sạch (Clean Seed) thành công!" -ForegroundColor Green
    } catch {
        Write-Warning "Không thực thi được clean_test_data.sql: $_"
    }
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT THIẾT LẬP POSTGRESQL NATIVE!" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
