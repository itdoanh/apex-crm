<#
.SYNOPSIS
    Sao lưu tự động PostgreSQL database `apex_leads` bằng pg_dump.
.DESCRIPTION
    Tạo file backup dạng custom-format (.dump) trong thư mục backup,
    tự xoay vòng giữ lại N bản gần nhất, và ghi log kết quả.

    Dùng cho Task Scheduler `Apex_DB_Backup` (chạy hằng ngày).
.NOTES
    Yêu cầu quyền Administrator (để đọc PGPASSWORD từ file .env).
#>
[CmdletBinding()]
param(
    [string]$ApexDir = "C:\apex",
    [string]$BackupDir = "C:\backup\apex_leads",
    [string]$DbName = "apex_leads",
    [string]$DbUser = "postgres",
    [string]$DbHost = "localhost",
    [int]$DbPort = 5432,
    [int]$KeepCount = 7,
    [string]$LogFile = "C:\backup\backup.log"
)

$ErrorActionPreference = "Continue"
$Sep = "=" * 60

function Write-Log([string]$Msg, [string]$Level = "INFO") {
    $line = "[{0}] [{1}] {2}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Level, $Msg
    Write-Output $line
    try {
        if (-not (Test-Path (Split-Path $LogFile -Parent))) {
            New-Item -ItemType Directory -Force -Path (Split-Path $LogFile -Parent) | Out-Null
        }
        Add-Content -Path $LogFile -Value $line -Encoding UTF8
    } catch {}
}

Write-Log "=== BAT DAU BACKUP $DbName ===" "INFO"

# 1. Xac dinh pg_dump
$pgDump = "C:\pgsql\bin\pg_dump.exe"
if (-not (Test-Path $pgDump)) {
    Write-Log "Khong tim thay pg_dump.exe tai $pgDump" "ERROR"
    exit 1
}

# 2. Doc mat khau DB tu file .env (khong hardcode)
$envFile = Join-Path $ApexDir ".env"
$dbPassword = "postgres"
if (Test-Path $envFile) {
    foreach ($line in (Get-Content $envFile -Encoding UTF8)) {
        $t = $line.Trim()
        if ($t -match '^DB_PASSWORD\s*=\s*(.*)$') {
            $dbPassword = $Matches[1].Trim().Trim('"').Trim("'")
        }
        if ($t -match '^DB_NAME\s*=\s*(.*)$')       { $DbName   = $Matches[1].Trim().Trim('"').Trim("'") }
        if ($t -match '^DB_USER\s*=\s*(.*)$')       { $DbUser   = $Matches[1].Trim().Trim('"').Trim("'") }
        if ($t -match '^DB_HOST\s*=\s*(.*)$')       { $DbHost   = $Matches[1].Trim().Trim('"').Trim("'") }
        if ($t -match '^DB_PORT\s*=\s*(.*)$')       { $DbPort   = [int]$Matches[1].Trim().Trim('"').Trim("'") }
    }
}
$env:PGPASSWORD = $dbPassword

# 3. Tao thu muc backup
if (-not (Test-Path $BackupDir)) {
    New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null
    Write-Log "Da tao thu muc backup: $BackupDir" "INFO"
}

# 4. Ten file theo ngay
$stamp = Get-Date -Format "yyyyMMdd_HHmm"
$outFile = Join-Path $BackupDir "${DbName}_${stamp}.dump"

# 5. Chay pg_dump (custom format - nen + restore duoc)
Write-Log "Chay pg_dump -> $outFile" "INFO"
$dumpOut = & $pgDump -U $DbUser -h $DbHost -p $DbPort -d $DbName -F p -v -f $outFile 2>&1 | Out-String
$dumpExit = $LASTEXITCODE

if ($dumpExit -ne 0 -or -not (Test-Path $outFile)) {
    Write-Log "pg_dump THAT BAI (exit=$dumpExit): $dumpOut" "ERROR"
    exit 1
}

$sizeMB = [math]::Round((Get-Item $outFile).Length / 1MB, 2)
Write-Log "Backup THANH CONG: $outFile ($sizeMB MB)" "INFO"

# 6. Xoay vong: giu lai $KeepCount ban moi nhat
$all = Get-ChildItem $BackupDir -Filter "${DbName}_*.dump" -File -ErrorAction SilentlyContinue |
       Sort-Object LastWriteTime -Descending
if ($all.Count -gt $KeepCount) {
    $toDelete = $all | Select-Object -Skip $KeepCount
    foreach ($f in $toDelete) {
        Remove-Item $f.FullName -Force -ErrorAction SilentlyContinue
        Write-Log "Da xoa backup cu: $($f.Name)" "INFO"
    }
}

$remaining = (Get-ChildItem $BackupDir -Filter "${DbName}_*.dump" -File).Count
Write-Log "Xoay vong xong. Con $remaining/$KeepCount ban backup" "INFO"
Write-Log "=== KET THUC BACKUP ===" "INFO"
exit 0
