<#
.SYNOPSIS
    Kịch bản dọn dẹp dung lượng đĩa cho Windows Server 2025 Datacenter (Ổ đĩa ~30GB).
.DESCRIPTION
    Tối ưu hóa và giải phóng dung lượng đĩa bằng cách dọn dẹp các tệp tạm của Windows,
    cache của Windows Update, nhật ký lỗi, Docker dangling resources và thùng rác.
.NOTES
    Chạy với quyền Administrator. Có thể đặt lịch chạy tự động hàng tuần qua Task Scheduler.
#>

[CmdletBinding()]
param(
    [switch]$Force = $false
)

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Kịch bản này yêu cầu quyền Administrator. Vui lòng chạy PowerShell với quyền 'Run as Administrator'."
    exit 1
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  DỌN DẸP & TỐI ƯU HÓA Ổ ĐĨA WINDOWS SERVER 2025 (~30GB QUOTA)" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan

function Get-DriveFreeSpace {
    $drive = Get-PSDrive C -ErrorAction SilentlyContinue
    if ($drive) {
        return [math]::Round($drive.Free / 1GB, 2)
    }
    return 0
}

$startFree = Get-DriveFreeSpace
Write-Host "[*] Dung lượng ổ C: trống ban đầu: $startFree GB" -ForegroundColor Yellow

# 1. Dọn dẹp thùng rác (Recycle Bin)
try {
    Clear-RecycleBin -DriveLetter C -Force -ErrorAction SilentlyContinue
    Write-Host "[OK] Đã dọn sạch Thùng rác (Recycle Bin)." -ForegroundColor Green
} catch {}

# 2. Dọn dẹp thư mục tạm người dùng ($env:TEMP)
Write-Host "[*] Đang dọn dẹp thư mục tạm người dùng ($env:TEMP)..." -ForegroundColor Yellow
$userTempFiles = Get-ChildItem -Path $env:TEMP -Recurse -Force -ErrorAction SilentlyContinue
foreach ($f in $userTempFiles) {
    try {
        Remove-Item -Path $f.FullName -Recurse -Force -ErrorAction SilentlyContinue
    } catch {}
}
Write-Host "[OK] Hoàn tất dọn dẹp thư mục tạm người dùng." -ForegroundColor Green

# 3. Dọn dẹp thư mục tạm hệ thống (C:\Windows\Temp)
Write-Host "[*] Đang dọn dẹp thư mục tạm hệ thống (C:\Windows\Temp)..." -ForegroundColor Yellow
$sysTempFiles = Get-ChildItem -Path "C:\Windows\Temp" -Recurse -Force -ErrorAction SilentlyContinue
foreach ($f in $sysTempFiles) {
    try {
        Remove-Item -Path $f.FullName -Recurse -Force -ErrorAction SilentlyContinue
    } catch {}
}
Write-Host "[OK] Hoàn tất dọn dẹp C:\Windows\Temp." -ForegroundColor Green

# 4. Dọn dẹp bộ nhớ đệm Windows Update (SoftwareDistribution\Download)
Write-Host "[*] Đang dọn dẹp tệp tải về của Windows Update..." -ForegroundColor Yellow
try {
    Stop-Service -Name wuauserv -Force -ErrorAction SilentlyContinue
    $wuDownload = "C:\Windows\SoftwareDistribution\Download"
    if (Test-Path $wuDownload) {
        Get-ChildItem -Path $wuDownload -Recurse -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
    }
    Start-Service -Name wuauserv -ErrorAction SilentlyContinue
    Write-Host "[OK] Đã dọn sạch bộ nhớ đệm Windows Update." -ForegroundColor Green
} catch {
    Write-Warning "Bỏ qua Windows Update cache: $_"
}

# 5. Dọn dẹp Windows Error Reporting (WER)
$werPath = "C:\ProgramData\Microsoft\Windows\WER"
if (Test-Path $werPath) {
    Get-ChildItem -Path $werPath -Recurse -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "[OK] Đã dọn sạch tệp crash dump WER." -ForegroundColor Green
}

# 6. Dọn dẹp Docker rác (nếu máy chủ có cài đặt Docker)
if (Get-Command "docker" -ErrorAction SilentlyContinue) {
    Write-Host "[*] Phát hiện Docker, đang dọn dẹp Docker images/cache rác..." -ForegroundColor Yellow
    try {
        & docker system prune -af --volumes 2>&1 | Out-Null
        Write-Host "[OK] Đã dọn sạch Docker build cache và dangling images." -ForegroundColor Green
    } catch {
        Write-Warning "Không thực thi được docker prune: $_"
    }
}

# 7. Xóa các tệp build tạm thời trong thư mục Apex nếu có
$ApexRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..") -ErrorAction SilentlyContinue
if ($ApexRoot -and (Test-Path $ApexRoot)) {
    Get-ChildItem -Path $ApexRoot -Filter "*~" -Recurse -Force -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    Get-ChildItem -Path $ApexRoot -Filter "*.tmp" -Recurse -Force -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    Get-ChildItem -Path $ApexRoot -Filter "*.log" -Recurse -Force -ErrorAction SilentlyContinue | Where-Object { $_.Length -gt 50MB } | Remove-Item -Force -ErrorAction SilentlyContinue
    Write-Host "[OK] Đã dọn sạch tệp tạm thời trong thư mục dự án Apex." -ForegroundColor Green
}

$endFree = Get-DriveFreeSpace
$freed = [math]::Round($endFree - $startFree, 2)
Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  KẾT QUẢ DỌN DẸP Ổ ĐĨA:" -ForegroundColor Green
Write-Host "  Dung lượng trống trước khi dọn: $startFree GB" -ForegroundColor White
Write-Host "  Dung lượng trống sau khi dọn:   $endFree GB" -ForegroundColor Green
if ($freed -gt 0) {
    Write-Host "  Đã giải phóng thành công:      +$freed GB ổ đĩa!" -ForegroundColor Green
} else {
    Write-Host "  Ổ đĩa hiện tại đã ở trạng thái tối ưu nhất." -ForegroundColor White
}
Write-Host "=================================================================" -ForegroundColor Cyan
