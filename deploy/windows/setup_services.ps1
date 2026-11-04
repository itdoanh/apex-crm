<#
.SYNOPSIS
    Thiết lập dịch vụ tự khởi động (Windows Service) cho Apex Backend và Caddy Server trên Windows Server 2025.
.DESCRIPTION
    Sử dụng NSSM (Non-Sucking Service Manager) để biến apex-server.exe và caddy.exe thành
    các Windows Services chính thống, tự khởi động cùng máy chủ và tự phục hồi khi gặp sự cố.
.NOTES
    Yêu cầu quyền Administrator.
#>

[CmdletBinding()]
param(
    [string]$ApexBackendDir = "",
    [string]$CaddyDir = "C:\caddy",
    # 'task' = Windows Task Scheduler (KHỚP VỚI PRODUCTION), 'nssm' = Windows Service qua NSSM
    [string]$Method = "task"
)

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Kịch bản này yêu cầu quyền Administrator. Vui lòng chạy PowerShell với quyền 'Run as Administrator'."
    exit 1
}

# Tự động xác định đường dẫn thư mục Apex Backend nếu không truyền vào
if (-not $ApexBackendDir) {
    $PotentialBackend = Resolve-Path (Join-Path $PSScriptRoot "..\..\backend") -ErrorAction SilentlyContinue
    if ($PotentialBackend -and (Test-Path $PotentialBackend)) {
        $ApexBackendDir = $PotentialBackend.Path
    } else {
        $ApexBackendDir = "C:\apex\backend"
    }
}

$ApexExe = Join-Path $ApexBackendDir "apex-server.exe"
$CaddyExe = Join-Path $CaddyDir "caddy.exe"
$Caddyfile = Join-Path $CaddyDir "Caddyfile"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  CẤU HÌNH TỰ ĐỘNG CHẠY NỀN (WINDOWS SERVICES) CHO APEX & CADDY" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "[*] Thư mục Apex Backend: $ApexBackendDir" -ForegroundColor White
Write-Host "[*] Đường dẫn Apex Server: $ApexExe" -ForegroundColor White
Write-Host "[*] Đường dẫn Caddy Server: $CaddyExe" -ForegroundColor White

if (-not (Test-Path $ApexExe)) {
    Write-Error "LỖI: Không tìm thấy file thực thi $ApexExe. Vui lòng kiểm tra lại đường dẫn."
    exit 1
}

if (-not (Test-Path $CaddyExe)) {
    Write-Warning "Chưa tìm thấy $CaddyExe. Đang chạy install_caddy.ps1 để cài đặt Caddy trước..."
    $InstallScript = Join-Path $PSScriptRoot "install_caddy.ps1"
    if (Test-Path $InstallScript) {
        & $InstallScript
    }
}

# ------------------------------------------------------------------------------
# PHƯƠNG ÁN 1: DÙNG NSSM (KHUYÊN DÙNG - ỔN ĐỊNH VÀ CHUYÊN NGHIỆP NHẤT)
# ------------------------------------------------------------------------------
if ($Method -eq "nssm") {
    $NssmDir = "C:\nssm"
    $NssmExe = Join-Path $NssmDir "nssm.exe"

    if (-not (Test-Path $NssmExe)) {
        Write-Host "[*] Đang tải công cụ quản lý dịch vụ Windows NSSM (chỉ ~300KB)..." -ForegroundColor Yellow
        New-Item -ItemType Directory -Force -Path $NssmDir | Out-Null
        $ZipPath = Join-Path $env:TEMP "nssm.zip"
        $NssmUrl = "https://nssm.cc/release/nssm-2.24.zip"

        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $NssmUrl -OutFile $ZipPath -UseBasicParsing -TimeoutSec 60
            Expand-Archive -Path $ZipPath -DestinationPath $env:TEMP\nssm_temp -Force
            Copy-Item "$env:TEMP\nssm_temp\nssm-2.24\win64\nssm.exe" -Destination $NssmExe -Force
            Remove-Item $ZipPath -Force -ErrorAction SilentlyContinue
            Remove-Item "$env:TEMP\nssm_temp" -Recurse -Force -ErrorAction SilentlyContinue
            Write-Host "[OK] Đã tải và cài đặt NSSM thành công tại $NssmExe" -ForegroundColor Green
        } catch {
            Write-Warning "Không thể tải NSSM tự động từ nssm.cc: $_"
            Write-Warning "Tự động chuyển sang phương án Windows Task Scheduler nội bộ..."
            $Method = "task"
        }
    }

    if ($Method -eq "nssm") {
        # 1. Cài đặt Windows Service cho Apex Backend
        Write-Host "[*] Đang cài đặt Windows Service: ApexBackend..." -ForegroundColor Yellow
        $ApexLogsDir = Join-Path $ApexBackendDir "logs"
        if (-not (Test-Path $ApexLogsDir)) { New-Item -ItemType Directory -Force -Path $ApexLogsDir | Out-Null }

        # Dừng và gỡ bỏ service cũ nếu tồn tại
        & $NssmExe stop ApexBackend 2>$null
        & $NssmExe remove ApexBackend confirm 2>$null

        & $NssmExe install ApexBackend "$ApexExe"
        & $NssmExe set ApexBackend AppDirectory "$ApexBackendDir"
        & $NssmExe set ApexBackend AppStdout "$ApexLogsDir\apex-server.log"
        & $NssmExe set ApexBackend AppStderr "$ApexLogsDir\apex-server_err.log"
        & $NssmExe set ApexBackend AppRotateFiles 1
        & $NssmExe set ApexBackend AppRotateBytes 10485760 # 10MB xoay vòng log
        & $NssmExe set ApexBackend Start SERVICE_AUTO_START
        & $NssmExe set ApexBackend AppRestartDelay 5000 # Tự restart sau 5s nếu crash
        & $NssmExe start ApexBackend
        Write-Host "[OK] Dịch vụ ApexBackend đã cài đặt và KHỞI ĐỘNG thành công!" -ForegroundColor Green

        # 2. Cài đặt Windows Service cho Caddy Server
        Write-Host "[*] Đang cài đặt Windows Service: ApexCaddy..." -ForegroundColor Yellow
        $CaddyLogsDir = Join-Path $CaddyDir "logs"
        if (-not (Test-Path $CaddyLogsDir)) { New-Item -ItemType Directory -Force -Path $CaddyLogsDir | Out-Null }

        & $NssmExe stop ApexCaddy 2>$null
        & $NssmExe remove ApexCaddy confirm 2>$null

        & $NssmExe install ApexCaddy "$CaddyExe" "run --config `"$Caddyfile`""
        & $NssmExe set ApexCaddy AppDirectory "$CaddyDir"
        & $NssmExe set ApexCaddy AppStdout "$CaddyLogsDir\caddy_service.log"
        & $NssmExe set ApexCaddy AppStderr "$CaddyLogsDir\caddy_service_err.log"
        & $NssmExe set ApexCaddy AppRotateFiles 1
        & $NssmExe set ApexCaddy AppRotateBytes 10485760 # 10MB xoay vòng log
        & $NssmExe set ApexCaddy Start SERVICE_AUTO_START
        & $NssmExe set ApexCaddy AppRestartDelay 5000
        & $NssmExe start ApexCaddy
        Write-Host "[OK] Dịch vụ ApexCaddy đã cài đặt và KHỞI ĐỘNG thành công!" -ForegroundColor Green
    }
}

# ------------------------------------------------------------------------------
# PHƯƠNG ÁN 2: DÙNG WINDOWS TASK SCHEDULER (100% NATIVE KHÔNG CẦN CÔNG CỤ NGOÀI)
# ------------------------------------------------------------------------------
if ($Method -eq "task") {
    Write-Host "[*] Đang cấu hình tự khởi động qua Windows Task Scheduler..." -ForegroundColor Yellow

    # 1. Scheduled Task cho Apex Backend
    # Action bọc qua cmd /c để stdout/stderr được ghi vào file log
    # (Task Scheduler mặc định nuốt toàn bộ output -> mất log khi lỗi)
    $ApexLogsDir = Join-Path $ApexBackendDir "logs"
    if (-not (Test-Path $ApexLogsDir)) { New-Item -ItemType Directory -Force -Path $ApexLogsDir | Out-Null }
    $ApexLogFile = Join-Path $ApexLogsDir "server.log"

    Unregister-ScheduledTask -TaskName "Apex_Backend_Server" -Confirm:$false -ErrorAction SilentlyContinue
    $ActionApex = New-ScheduledTaskAction -Execute "cmd.exe" -Argument "/c `"`"$ApexExe`" >> `"$ApexLogFile`" 2>&1`"" -WorkingDirectory $ApexBackendDir
    $TriggerApex = New-ScheduledTaskTrigger -AtStartup
    $PrincipalApex = New-ScheduledTaskPrincipal -UserId "NT AUTHORITY\SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    $SettingsApex = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 5 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Days 365) -MultipleInstances IgnoreNew
    Register-ScheduledTask -TaskName "Apex_Backend_Server" -Action $ActionApex -Trigger $TriggerApex -Principal $PrincipalApex -Settings $SettingsApex | Out-Null
    Start-ScheduledTask -TaskName "Apex_Backend_Server"
    Write-Host "[OK] Đã tạo Task Scheduler: Apex_Backend_Server (WD=$ApexBackendDir, log=$ApexLogFile)" -ForegroundColor Green

    # 2. Scheduled Task cho Caddy Server
    $CaddyLogsDir = Join-Path $CaddyDir "logs"
    if (-not (Test-Path $CaddyLogsDir)) { New-Item -ItemType Directory -Force -Path $CaddyLogsDir | Out-Null }

    Unregister-ScheduledTask -TaskName "Apex_Caddy_Server" -Confirm:$false -ErrorAction SilentlyContinue
    $ActionCaddy = New-ScheduledTaskAction -Execute $CaddyExe -Argument "run --config `"$Caddyfile`"" -WorkingDirectory $CaddyDir
    $TriggerCaddy = New-ScheduledTaskTrigger -AtStartup
    $PrincipalCaddy = New-ScheduledTaskPrincipal -UserId "NT AUTHORITY\SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    $SettingsCaddy = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 5 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Days 365) -MultipleInstances IgnoreNew
    Register-ScheduledTask -TaskName "Apex_Caddy_Server" -Action $ActionCaddy -Trigger $TriggerCaddy -Principal $PrincipalCaddy -Settings $SettingsCaddy | Out-Null
    Start-ScheduledTask -TaskName "Apex_Caddy_Server"
    Write-Host "[OK] Đã tạo Task Scheduler: Apex_Caddy_Server (WD=$CaddyDir)" -ForegroundColor Green
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  KIỂM TRA TRẠNG THÁI HOẠT ĐỘNG:" -ForegroundColor Green
Start-Sleep -Seconds 2

# Kiểm tra port 8080 và 80/443
$port8080 = Test-NetConnection -ComputerName 127.0.0.1 -Port 8080 -WarningAction SilentlyContinue
$port80 = Test-NetConnection -ComputerName 127.0.0.1 -Port 80 -WarningAction SilentlyContinue

if ($port8080.TcpTestSucceeded) {
    Write-Host "[OK] Apex Backend (Port 8080): ĐANG HOẠT ĐỘNG (Listening)" -ForegroundColor Green
} else {
    Write-Warning "Apex Backend (Port 8080) chưa phản hồi. Vui lòng kiểm tra log tại $ApexBackendDir\logs"
}

if ($port80.TcpTestSucceeded) {
    Write-Host "[OK] Caddy Server (Port 80/443): ĐANG HOẠT ĐỘNG (Listening)" -ForegroundColor Green
} else {
    Write-Warning "Caddy Server (Port 80) chưa phản hồi. Vui lòng kiểm tra log tại $CaddyDir\logs"
}
Write-Host "=================================================================" -ForegroundColor Cyan
