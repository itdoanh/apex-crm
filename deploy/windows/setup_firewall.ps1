<#
.SYNOPSIS
    Cấu hình tường lửa Windows Defender Firewall cho hệ thống Apex Fintech trên Windows Server 2025.
.DESCRIPTION
    - Mở cổng 80 (HTTP) và 443 (HTTPS) phục vụ truy cập bên ngoài và xác thực ACME On-Demand TLS.
    - Bảo vệ cổng nội bộ 8080 (Go Backend) và 5432 (PostgreSQL), chỉ cho phép kết nối từ Localhost (127.0.0.1).
.NOTES
    Yêu cầu quyền Administrator.
#>

[CmdletBinding()]
param()

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "LỖI: Kịch bản này yêu cầu quyền Administrator. Vui lòng chạy PowerShell với quyền 'Run as Administrator'."
    exit 1
}

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  CẤU HÌNH TƯỜNG LỬA WINDOWS DEFENDER FIREWALL (SECURITY RULES)" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan

# 1. Mở cổng 80 (HTTP) cho Caddy
$Rule80Name = "Apex_Caddy_HTTP_80"
Remove-NetFirewallRule -DisplayName $Rule80Name -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName $Rule80Name `
    -Direction Inbound `
    -LocalPort 80 `
    -Protocol TCP `
    -Action Allow `
    -Profile Any `
    -Description "Mo cong 80 HTTP phuc vu On-Demand TLS va chuyen huong HTTPS" | Out-Null
Write-Host "[OK] Đã mở cổng 80 (HTTP - TCP Inbound)" -ForegroundColor Green

# 2. Mở cổng 443 (HTTPS) cho Caddy
$Rule443Name = "Apex_Caddy_HTTPS_443"
Remove-NetFirewallRule -DisplayName $Rule443Name -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName $Rule443Name `
    -Direction Inbound `
    -LocalPort 443 `
    -Protocol TCP `
    -Action Allow `
    -Profile Any `
    -Description "Mo cong 443 HTTPS bao mat cho Admin CRM va cac Landing Pages" | Out-Null
Write-Host "[OK] Đã mở cổng 443 (HTTPS - TCP Inbound)" -ForegroundColor Green

# 3. Chặn truy cập từ bên ngoài vào cổng 8080 (Apex Backend) - Chỉ cho phép Localhost
$Rule8080Name = "Apex_Block_External_8080"
Remove-NetFirewallRule -DisplayName $Rule8080Name -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName $Rule8080Name `
    -Direction Inbound `
    -LocalPort 8080 `
    -Protocol TCP `
    -Action Block `
    -RemoteAddress NotLocalSubnet `
    -Profile Any `
    -Description "Chan truy cap ngoai vao cong noi bo 8080 cua Go Backend" | Out-Null
Write-Host "[OK] Đã bảo vệ cổng 8080 (Chỉ cho phép Caddy Reverse Proxy kết nối nội bộ)" -ForegroundColor Green

# 4. Chặn truy cập từ bên ngoài vào cổng 5432 (PostgreSQL) - Chỉ cho phép Localhost
$Rule5432Name = "Apex_Block_External_5432"
Remove-NetFirewallRule -DisplayName $Rule5432Name -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName $Rule5432Name `
    -Direction Inbound `
    -LocalPort 5432 `
    -Protocol TCP `
    -Action Block `
    -RemoteAddress NotLocalSubnet `
    -Profile Any `
    -Description "Chan truy cap ngoai vao cong noi bo 5432 cua CSDL PostgreSQL" | Out-Null
Write-Host "[OK] Đã bảo vệ cổng 5432 (Chỉ cho phép Apex Server kết nối CSDL nội bộ)" -ForegroundColor Green

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT THIẾT LẬP TƯỜNG LỬA BẢO MẬT TRÊN WINDOWS SERVER 2025" -ForegroundColor Green
Write-Host "=================================================================" -ForegroundColor Cyan
