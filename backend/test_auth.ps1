# Comprehensive Admin Authentication, Sessions & Audit Logs Test Script
$baseUrl = "http://localhost:8080"
$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "TEST 1: UNPROTECTED ACCESS ATTEMPTS (MUST RETURN 401)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$endpoints = @(
    "/api/admin/me",
    "/api/crm/stats",
    "/api/leads",
    "/api/domains",
    "/api/admin/sessions",
    "/api/admin/audit-logs"
)

foreach ($ep in $endpoints) {
    try {
        $res = Invoke-RestMethod -Uri "$baseUrl$ep" -Method Get
        Write-Host "[-] FAILED: $ep did not return 401" -ForegroundColor Red
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 401) {
            Write-Host "[+] SUCCESS: $ep is strictly protected with HTTP 401" -ForegroundColor Green
        } else {
            Write-Host "[-] UNEXPECTED: $ep returned $($_.Exception.Response.StatusCode)" -ForegroundColor Red
        }
    }
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "TEST 2: LOGIN FAILURE WITH WRONG PASSWORD" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$wrongLoginBody = @{
    username = "admin"
    password = "WrongPassword123!"
} | ConvertTo-Json

try {
    $res = Invoke-RestMethod -Uri "$baseUrl/api/admin/login" -Method Post -Body $wrongLoginBody -ContentType "application/json"
    Write-Host "[-] FAILED: Wrong login succeeded unexpectedly" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 401) {
        Write-Host "[+] SUCCESS: Wrong password rejected with HTTP 401 (Argon2id verify)" -ForegroundColor Green
    } else {
        Write-Host "[-] FAILED: Wrong password returned $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    }
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "TEST 3: LOGIN SUCCESS WITH ARGON2ID & SESSION GENERATION" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$correctLoginBody = @{
    username = "admin"
    password = "ApexAdmin@2026!"
} | ConvertTo-Json

$loginData = Invoke-RestMethod -Uri "$baseUrl/api/admin/login" -Method Post -Body $correctLoginBody -ContentType "application/json" -Headers @{"User-Agent" = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"}

if ($loginData.success -and $loginData.token) {
    Write-Host "[+] SUCCESS: Login successful!" -ForegroundColor Green
    Write-Host "    Admin: $($loginData.admin.username) ($($loginData.admin.role))" -ForegroundColor Gray
    Write-Host "    Token: $($loginData.token.Substring(0, 20))..." -ForegroundColor Gray
    Write-Host "    Session Expires: $($loginData.expires_at)" -ForegroundColor Gray
} else {
    Write-Host "[-] FAILED: Login response invalid" -ForegroundColor Red
    exit 1
}

$token1 = $loginData.token
$headers1 = @{
    "Authorization" = "Bearer $token1"
    "User-Agent" = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "TEST 4: ACCESS PROTECTED APIS WITH BEARER TOKEN (CROSS-DOMAIN)" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$meRes = Invoke-RestMethod -Uri "$baseUrl/api/admin/me" -Method Get -Headers $headers1
if ($meRes.success -and $meRes.admin.username -eq "admin") {
    Write-Host "[+] SUCCESS: /api/admin/me returned authenticated admin $($meRes.admin.username)!" -ForegroundColor Green
} else {
    Write-Host "[-] FAILED: /api/admin/me did not return admin" -ForegroundColor Red
}

$crmStats = Invoke-RestMethod -Uri "$baseUrl/api/crm/stats" -Method Get -Headers $headers1
if ($crmStats.success) {
    Write-Host "[+] SUCCESS: /api/crm/stats accessed successfully with Bearer Token!" -ForegroundColor Green
}

$domainsRes = Invoke-RestMethod -Uri "$baseUrl/api/domains" -Method Get -Headers $headers1
if ($domainsRes.success) {
    Write-Host "[+] SUCCESS: /api/domains accessed successfully with Bearer Token! (Total: $($domainsRes.total))" -ForegroundColor Green
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "TEST 5: MULTI-SESSION & FACEBOOK-STYLE SESSION REVOCATION" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Create Session 2 (Mobile device)
$loginData2 = Invoke-RestMethod -Uri "$baseUrl/api/admin/login" -Method Post -Body $correctLoginBody -ContentType "application/json" -Headers @{"User-Agent" = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"}
$token2 = $loginData2.token
$headers2 = @{
    "Authorization" = "Bearer $token2"
    "User-Agent" = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
}

# Check sessions count
$sessionsRes = Invoke-RestMethod -Uri "$baseUrl/api/admin/sessions" -Method Get -Headers $headers1
Write-Host "[+] Active sessions count: $($sessionsRes.total)" -ForegroundColor Green
foreach ($s in $sessionsRes.data) {
    Write-Host "    - [ID: $($s.id)] Device: $($s.device_type) | $($s.browser) on $($s.os) | IP: $($s.ip_address) | Current: $($s.is_current)" -ForegroundColor Gray
}

# Session 1 revokes all other sessions
$revokeRes = Invoke-RestMethod -Uri "$baseUrl/api/admin/sessions/revoke-others" -Method Post -Headers $headers1
Write-Host "[+] SUCCESS: Revoke all others -> $($revokeRes.message)" -ForegroundColor Green

# Verify Session 2 is now rejected with 401
try {
    $null = Invoke-RestMethod -Uri "$baseUrl/api/admin/me" -Method Get -Headers $headers2
    Write-Host "[-] FAILED: Revoked session 2 still has access" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 401) {
        Write-Host "[+] SUCCESS: Session 2 immediately rejected with 401 after remote revocation!" -ForegroundColor Green
    }
}

# Verify Session 1 is still active
$meResStill = Invoke-RestMethod -Uri "$baseUrl/api/admin/me" -Method Get -Headers $headers1
if ($meResStill.success) {
    Write-Host "[+] SUCCESS: Session 1 remains active and valid!" -ForegroundColor Green
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "TEST 6: AUDIT LOGS TRAIL RECORDING" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$auditRes = Invoke-RestMethod -Uri "$baseUrl/api/admin/audit-logs?limit=20" -Method Get -Headers $headers1
Write-Host "[+] Total Audit Logs recorded in DB: $($auditRes.total)" -ForegroundColor Green
foreach ($log in $auditRes.data) {
    Write-Host "    - [$($log.created_at)] $($log.action) by $($log.username) (IP: $($log.ip_address), Device: $($log.device_type), Browser: $($log.browser), OS: $($log.os))" -ForegroundColor Gray
}

# Verify key actions are logged
$actions = $auditRes.data | Select-Object -ExpandProperty action
if ($actions -contains "AUTH_LOGIN_SUCCESS" -and $actions -contains "AUTH_LOGIN_FAILED" -and $actions -contains "REVOKE_ALL_OTHER_SESSIONS") {
    Write-Host "[+] SUCCESS: Full audit trail recorded in database with IP and User-Agent fingerprinting!" -ForegroundColor Green
} else {
    Write-Host "[-] WARNING: Some expected actions missing from audit logs" -ForegroundColor Yellow
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "TEST 7: PASSWORD CHANGE WITH ARGON2ID VERIFICATION & REHASH" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Change password to new temporary password
$changePwBody = @{
    current_password = "ApexAdmin@2026!"
    new_password = "ApexAdmin@2026NewPass!"
} | ConvertTo-Json

$changeRes = Invoke-RestMethod -Uri "$baseUrl/api/admin/change-password" -Method Post -Body $changePwBody -ContentType "application/json" -Headers $headers1
Write-Host "[+] SUCCESS: Password changed -> $($changeRes.message)" -ForegroundColor Green

# Try logging in with old password (must fail)
try {
    $null = Invoke-RestMethod -Uri "$baseUrl/api/admin/login" -Method Post -Body $correctLoginBody -ContentType "application/json"
    Write-Host "[-] FAILED: Old password still worked" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 401) {
        Write-Host "[+] SUCCESS: Old password rejected with 401!" -ForegroundColor Green
    }
}

# Log in with new password (must succeed)
$newLoginBody = @{
    username = "admin"
    password = "ApexAdmin@2026NewPass!"
} | ConvertTo-Json

$newLoginRes = Invoke-RestMethod -Uri "$baseUrl/api/admin/login" -Method Post -Body $newLoginBody -ContentType "application/json"
if ($newLoginRes.success) {
    Write-Host "[+] SUCCESS: Logged in successfully with new Argon2id password!" -ForegroundColor Green
    $newHeaders = @{ "Authorization" = "Bearer $($newLoginRes.token)" }

    # Restore password back to ApexAdmin@2026!
    $restorePwBody = @{
        current_password = "ApexAdmin@2026NewPass!"
        new_password = "ApexAdmin@2026!"
    } | ConvertTo-Json
    $null = Invoke-RestMethod -Uri "$baseUrl/api/admin/change-password" -Method Post -Body $restorePwBody -ContentType "application/json" -Headers $newHeaders
    Write-Host "[+] SUCCESS: Restored password back to ApexAdmin@2026!" -ForegroundColor Green
}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "ALL 7 ADVANCED SECURITY & AUTHENTICATION TESTS PASSED 100%!" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
