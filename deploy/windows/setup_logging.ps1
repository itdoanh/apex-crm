<#
.SYNOPSIS
    Cấu hình Task Scheduler cho Apex Backend để ghi log stdout/stderr vào file.
.DESCRIPTION
    Task Scheduler mặc định NUỐT toàn bộ stdout/stderr của tiến trình.
    Hậu quả: mọi lỗi Go (panic, DB disconnect,...) đều bị mất hoàn toàn,
    không có dấu vết để chẩn đoán.

    Script này cấu hình lại action của task `Apex_Backend_Server` để chạy qua
    `cmd /c "... >> logs\server.log 2>&1"` — khiến toàn bộ output được ghi vào
    file log có xoay vòng.

    Cũng tạo task `Apex_Backend_Server` nếu chưa có.
.NOTES
    Yêu cầu quyền Administrator.
#>
[CmdletBinding()]
param(
    [string]$ApexBackendDir = "C:\apex\backend",
    [string]$TaskName = "Apex_Backend_Server"
)

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Cần quyền Administrator. Chạy với 'Run as Administrator'."
    exit 1
}

$ApexExe = Join-Path $ApexBackendDir "apex-server.exe"
$ApexLogsDir = Join-Path $ApexBackendDir "logs"
$ApexLogFile = Join-Path $ApexLogsDir "server.log"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  CẤU HÌNH LOGGING CHO TASK SCHEDULER: $TaskName" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan

if (-not (Test-Path $ApexExe)) {
    Write-Error "LỖI: Không tìm thấy $ApexExe"
    exit 1
}

# 1. Tạo thư mục logs nếu chưa có
if (-not (Test-Path $ApexLogsDir)) {
    New-Item -ItemType Directory -Force -Path $ApexLogsDir | Out-Null
    Write-Host "[OK] Đã tạo thư mục log: $ApexLogsDir" -ForegroundColor Green
} else {
    Write-Host "[OK] Thư mục log đã tồn tại: $ApexLogsDir" -ForegroundColor DarkGray
}

# 2. Ghi file log rỗng nếu chưa có (để đảm bảo redirect không lỗi)
if (-not (Test-Path $ApexLogFile)) {
    New-Item -ItemType File -Force -Path $ApexLogFile | Out-Null
}

# 3. Cấu hình lại action để bọc cmd /c kèm redirect
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

$escapedExe = $ApexExe.Replace('"', '\"')
$escapedLog = $ApexLogFile.Replace('"', '\"')
$actionArg = "/c `"`"$escapedExe`" >> `"$escapedLog`" 2>&1`""

$Action = New-ScheduledTaskAction -Execute "cmd.exe" -Argument $actionArg -WorkingDirectory $ApexBackendDir
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Principal = New-ScheduledTaskPrincipal -UserId "NT AUTHORITY\SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -RestartCount 5 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit (New-TimeSpan -Days 365) `
    -MultipleInstances IgnoreNew

Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings | Out-Null
Write-Host "[OK] Đã (re)đăng ký task: $TaskName" -ForegroundColor Green
Write-Host "     Execute    : cmd.exe" -ForegroundColor DarkGray
Write-Host "     Arguments  : $actionArg" -ForegroundColor DarkGray
Write-Host "     WorkingDir : $ApexBackendDir" -ForegroundColor DarkGray

# 4. Khởi động lại task để áp dụng
Write-Host "[*] Đang khởi động lại task..." -ForegroundColor Yellow
Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 4

# 5. Verify
$state = (Get-ScheduledTask -TaskName $TaskName).State
Write-Host "[OK] Task state: $state" -ForegroundColor Green

try {
    $h = Invoke-WebRequest -Uri "http://127.0.0.1:8080/health" -UseBasicParsing -TimeoutSec 8
    Write-Host "[OK] Health check: HTTP $($h.StatusCode) $($h.Content)" -ForegroundColor Green
} catch {
    Write-Warning "Health check thất bại: $($_.Exception.Message)"
}

# 6. Kiểm tra log đã ghi được chưa
if (Test-Path $ApexLogFile) {
    $size = (Get-Item $ApexLogFile).Length
    Write-Host "[OK] File log: $ApexLogFile ($size bytes)" -ForegroundColor Green
    if ($size -gt 0) {
        Write-Host "--- 10 dòng log gần nhất ---" -ForegroundColor DarkGray
        Get-Content $ApexLogFile -Tail 10 | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
    }
} else {
    Write-Warning "Chưa thấy file log."
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT CẤU HÌNH LOGGING" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
