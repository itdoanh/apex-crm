<#
.SYNOPSIS
    Chẩn đoán tổng hợp sức khoẻ hệ thống Apex Fintech trên production server.
.DESCRIPTION
    Thu thập thông tin: tiến trình, port, Task Scheduler, health check, log Caddy,
    dung lượng đĩa, RAM, và trạng thái database.
.EXAMPLE
    powershell -File .apex-session\apex.ps1 "PS1:C:\code\apex_moi\landing\deploy\windows\health_check.ps1"
#>

$ErrorActionPreference = "Continue"
$Sep = "=" * 70

function Section($Title) {
    Write-Output ""
    Write-Output $Sep
    Write-Output "  $Title"
    Write-Output $Sep
}

Section "1. THOI GIAN HE THONG"
Write-Output ("  Server      : " + $env:COMPUTERNAME)
Write-Output ("  User        : " + $env:USERNAME)
Write-Output ("  Time        : " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss"))
Write-Output ("  OS          : " + (Get-CimInstance Win32_OperatingSystem).Caption + " (Build " + (Get-CimInstance Win32_OperatingSystem).BuildNumber + ")")

Section "2. TIEN TRINH (apex-server / caddy / postgres)"
$procs = Get-Process | Where-Object { $_.ProcessName -match 'apex-server|caddy|postgres' }
if ($procs) {
    $procs | Select-Object ProcessName, Id, @{n='MemMB';e={[math]::Round($_.WorkingSet64/1MB,1)}}, StartTime |
        Format-Table -AutoSize | Out-String | Write-Output
} else {
    Write-Output "  [!] KHONG tim thay tien trinh nao dang chay."
}

Section "3. PORT DANG LANG NGHE"
try {
    Get-NetTCPConnection -State Listen -ErrorAction Stop |
        Where-Object { $_.LocalPort -in 80, 443, 8080, 5432 } |
        Select-Object LocalAddress, LocalPort, OwningProcess,
            @{n='Process';e={ (Get-Process -Id $_.OwningProcess -ErrorAction SilentlyContinue).ProcessName }} |
        Sort-Object LocalPort | Format-Table -AutoSize | Out-String | Write-Output
} catch {
    Write-Output "  [!] Khong doc duoc danh sach port: $($_.Exception.Message)"
}

Section "4. TASK SCHEDULER (Apex_Backend_Server / Apex_Caddy_Server)"
foreach ($t in @("Apex_Backend_Server", "Apex_Caddy_Server")) {
    $info = Get-ScheduledTask -TaskName $t -ErrorAction SilentlyContinue
    if ($info) {
        $state = (Get-ScheduledTaskInfo -TaskName $t -ErrorAction SilentlyContinue)
        Write-Output ("  [OK] Task: {0}" -f $t)
        Write-Output ("       State       : {0}" -f $info.State)
        Write-Output ("       Last Run    : {0}" -f $state.LastRunTime)
        Write-Output ("       Last Result : {0}  (0 = thanh cong)" -f $state.LastTaskResult)
        Write-Output ("       Working Dir : {0}" -f $info.Actions[0].WorkingDirectory)
        Write-Output ("       Execute     : {0} {1}" -f $info.Actions[0].Execute, $info.Actions[0].Arguments)
    } else {
        Write-Output ("  [KHONG TON TAI] Task: {0}" -f $t)
    }
}

Section "5. HEALTH CHECK"
try {
    $local = Invoke-WebRequest -Uri "http://127.0.0.1:8080/health" -UseBasicParsing -TimeoutSec 8
    Write-Output ("  [LOCAL ] http://127.0.0.1:8080/health -> HTTP {0} {1}" -f $local.StatusCode, $local.Content)
} catch {
    Write-Output ("  [LOCAL ] FAIL -> {0}" -f $_.Exception.Message)
}
try {
    $pub = Invoke-WebRequest -Uri "https://crm.hanghoaphaisinh.net/health" -UseBasicParsing -TimeoutSec 12
    Write-Output ("  [PUBLIC] https://crm.hanghoaphaisinh.net/health -> HTTP {0} {1}" -f $pub.StatusCode, $pub.Content)
} catch {
    Write-Output ("  [PUBLIC] FAIL -> {0}" -f $_.Exception.Message)
}

Section "6. ON-DEMAND TLS - KIEM TRA DOMAIN DA DANG KY"
try {
    $r = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/internal/check-domain?domain=ads.giaodichhanghoatuonglai.com" -UseBasicParsing -TimeoutSec 8
    Write-Output ("  [OK] ads.giaodichhanghoatuonglai.com -> HTTP {0}" -f $r.StatusCode)
} catch {
    $code = $null
    try { $code = [int]$_.Exception.Response.StatusCode } catch {}
    Write-Output ("  [!] ads.giaodichhanghoatuonglai.com -> HTTP {0} ({1})" -f $code, $_.Exception.Message)
}

Section "7. LOG CADDY - 15 DONG CUOI"
if (Test-Path "C:\caddy\logs\caddy.log") {
    Get-Content "C:\caddy\logs\caddy.log" -Tail 15
} else {
    Write-Output "  [!] Khong tim thay C:\caddy\logs\caddy.log"
}

Section "8. LOG CADDY - 10 LOI GAN NHAT"
if (Test-Path "C:\caddy\logs\caddy.log") {
    $errs = Select-String -Path "C:\caddy\logs\caddy.log" -Pattern "ERROR|error|502|refused" | Select-Object -Last 10
    if ($errs) { $errs | ForEach-Object { Write-Output ("  " + $_.Line) } } else { Write-Output "  [OK] Khong co loi trong log." }
} else {
    Write-Output "  [!] Khong tim thay log Caddy"
}

Section "9. DUNG LUONG DIA"
Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'" | ForEach-Object {
    Write-Output ("  C: {0}GB free / {1}GB total ({2}% used)" -f [math]::Round($_.FreeSpace/1GB,1), [math]::Round($_.Size/1GB,1), [math]::Round((1-$_.FreeSpace/$_.Size)*100,1))
}
if (Test-Path "C:\caddy\logs") {
    Get-ChildItem "C:\caddy\logs" -File | ForEach-Object {
        Write-Output ("  LOG {0} = {1}MB" -f $_.Name, [math]::Round($_.Length/1MB,2))
    }
}

Section "10. RAM HE THONG"
$os = Get-CimInstance Win32_OperatingSystem
Write-Output ("  Total RAM : {0}GB" -f [math]::Round($os.TotalVisibleMemorySize/1MB,1))
Write-Output ("  Free RAM  : {0}GB" -f [math]::Round($os.FreePhysicalMemory/1MB,1))

Section "11. DATABASE (PostgreSQL)"
$psql = "C:\pgsql\bin\psql.exe"
if (Test-Path $psql) {
    $env:PGPASSWORD = "postgres"
    $queries = @(
        "SELECT 'leads' AS t, count(*) FROM leads",
        "SELECT 'analytics_sessions', count(*) FROM analytics_sessions",
        "SELECT 'spam_quarantine', count(*) FROM spam_quarantine",
        "SELECT 'landing_pages', count(*) FROM landing_pages",
        "SELECT 'domains', count(*) FROM domains",
        "SELECT 'domain_routes', count(*) FROM domain_routes"
    )
    foreach ($q in $queries) {
        $out = & $psql -U postgres -h localhost -d apex_leads -t -A -c $q 2>&1
        Write-Output ("  {0}" -f ($out -join " ").Trim())
    }
    Write-Output ""
    Write-Output "  --- Trung trang page_id bi trung ---"
    $dup = & $psql -U postgres -h localhost -d apex_leads -t -A -c "SELECT page_id || ' x' || count(*) || ' domains=' || array_agg(domain) FROM landing_pages GROUP BY page_id HAVING count(*) > 1" 2>&1
    if ($dup) { $dup | ForEach-Object { Write-Output ("  [!] " + $_) } } else { Write-Output "  [OK] Khong co page_id bi trung." }
} else {
    Write-Output "  [!] Khong tim thay $psql"
}

Section "12. GIT TREN SERVER"
if (Test-Path "C:\apex\.git") {
    Push-Location "C:\apex"
    $branch = git rev-parse --abbrev-ref HEAD 2>$null
    $commit = git log -1 --pretty=format:"%h %s (%cr)" 2>$null
    $dirty = git status --porcelain 2>$null
    Write-Output ("  Branch : {0}" -f $branch)
    Write-Output ("  Commit : {0}" -f $commit)
    if ($dirty) {
        Write-Output "  [!] Working tree DIRTY (thay doi chua commit):"
        $dirty | Select-Object -First 10 | ForEach-Object { Write-Output ("       " + $_) }
    } else {
        Write-Output "  [OK] Working tree clean."
    }
    Pop-Location
} else {
    Write-Output "  [!] Khong tim thay C:\apex\.git"
}

Section "KET THUC - " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
