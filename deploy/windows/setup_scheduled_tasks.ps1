<#
.SYNOPSIS
    Đăng ký các Task Scheduler tự động cho Apex Fintech:
      - Apex_DB_Backup    : Sao lưu PostgreSQL hằng ngày (02:00)
      - Apex_Disk_Cleanup : Dọn file log cũ hằng tuần (Chủ Nhật 03:00)
.DESCRIPTION
    Task `Apex_DB_Backup` chạy `deploy/windows/backup_database.ps1` — bảo vệ dữ liệu lead
    khi ổ đĩa/PostgreSQL gặp sự cố.

    Task `Apex_Disk_Cleanup` chạy `deploy/windows/cleanup_disk_space.ps1` — ngăn ổ đĩa đầy
    do log Caddy và log backend phình to.
.NOTES
    Yêu cầu quyền Administrator.
#>
[CmdletBinding()]
param(
    [string]$ApexDir = "C:\apex",
    [string]$ScriptsDir = "C:\apex\deploy\windows"
)

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Cần quyền Administrator. Chạy với 'Run as Administrator'."
    exit 1
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  ĐĂNG KÝ TASK SCHEDULER TỰ ĐỘNG (Backup DB + Dọn Disk)" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan

$Principal = New-ScheduledTaskPrincipal -UserId "NT AUTHORITY\SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -RestartCount 3 `
    -RestartInterval (New-TimeSpan -Minutes 5) `
    -ExecutionTimeLimit (New-TimeSpan -Hours 2) `
    -MultipleInstances IgnoreNew

# ------------------------------------------------------------------------------
# TASK A: BACKUP DATABASE - HANG NGAY 02:00
# ------------------------------------------------------------------------------
Write-Host "`n[*] Đăng ký task: Apex_DB_Backup (hằng ngày 02:00)" -ForegroundColor Yellow

$backupScript = Join-Path $ScriptsDir "backup_database.ps1"
if (-not (Test-Path $backupScript)) {
    Write-Host "  [!] Không tìm thấy $backupScript - dùng đường dẫn fallback" -ForegroundColor Yellow
    $backupScript = "C:\apex\deploy\windows\backup_database.ps1"
}

Unregister-ScheduledTask -TaskName "Apex_DB_Backup" -Confirm:$false -ErrorAction SilentlyContinue
$ActionBackup = New-ScheduledTaskAction -Execute "powershell.exe" `
    -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$backupScript`"" `
    -WorkingDirectory $ScriptsDir
$TriggerBackup = New-ScheduledTaskTrigger -Daily -At "02:00"

Register-ScheduledTask -TaskName "Apex_DB_Backup" -Action $ActionBackup -Trigger $TriggerBackup -Principal $Principal -Settings $Settings | Out-Null
Write-Host "  [OK] Task 'Apex_DB_Backup' đã đăng ký (hằng ngày 02:00)" -ForegroundColor Green
Write-Host "       Script: $backupScript" -ForegroundColor DarkGray

# ------------------------------------------------------------------------------
# TASK B: DON DIA - HANG TUAN (Chu Nhat 03:00)
# ------------------------------------------------------------------------------
Write-Host "`n[*] Đăng ký task: Apex_Disk_Cleanup (Chủ Nhật 03:00)" -ForegroundColor Yellow

$cleanupScript = Join-Path $ScriptsDir "cleanup_disk_space.ps1"
if (-not (Test-Path $cleanupScript)) {
    Write-Host "  [!] Không tìm thấy $cleanupScript" -ForegroundColor Yellow
    $cleanupScript = "C:\apex\deploy\windows\cleanup_disk_space.ps1"
}

Unregister-ScheduledTask -TaskName "Apex_Disk_Cleanup" -Confirm:$false -ErrorAction SilentlyContinue
$ActionCleanup = New-ScheduledTaskAction -Execute "powershell.exe" `
    -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$cleanupScript`"" `
    -WorkingDirectory $ScriptsDir
$TriggerCleanup = New-ScheduledTaskTrigger -Weekly -DaysOfWeek Sunday -At "03:00"

Register-ScheduledTask -TaskName "Apex_Disk_Cleanup" -Action $ActionCleanup -Trigger $TriggerCleanup -Principal $Principal -Settings $Settings | Out-Null
Write-Host "  [OK] Task 'Apex_Disk_Cleanup' đã đăng ký (Chủ Nhật 03:00)" -ForegroundColor Green
Write-Host "       Script: $cleanupScript" -ForegroundColor DarkGray

# ------------------------------------------------------------------------------
# VERIFY
# ------------------------------------------------------------------------------
Write-Host "`n[*] Kiểm tra các task đã đăng ký:" -ForegroundColor Cyan
foreach ($t in @("Apex_DB_Backup", "Apex_Disk_Cleanup", "Apex_Backend_Server", "Apex_Caddy_Server")) {
    $task = Get-ScheduledTask -TaskName $t -ErrorAction SilentlyContinue
    if ($task) {
        $info = Get-ScheduledTaskInfo -TaskName $t
        Write-Host ("  [OK] {0,-24} State={1,-8} LastResult={2}" -f $t, $task.State, $info.LastTaskResult)
    } else {
        Write-Host ("  [!] {0,-24} KHÔNG TỒN TẠI" -f $t)
    }
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT ĐĂNG KÝ TASK TỰ ĐỘNG" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
