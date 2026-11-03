@echo off
chcp 65001 >nul
echo =================================================================
echo   KHỞI ĐỘNG HỆ THỐNG APEX FINTECH & CADDY SERVER (WINDOWS 2025)
echo =================================================================

net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [CANH BAO] Ban can chay file bat nay voi quyen Run as Administrator de quan ly Services.
    pause
    exit /b 1
)

echo [*] Dang khoi dong cac dich vu...
sc start ApexBackend >nul 2>&1
if %errorLevel% equ 0 (
    echo [OK] ApexBackend: Khoi dong thanh cong.
) else (
    echo [!] ApexBackend da dang chay hoac chua duoc tao service.
)

sc start ApexCaddy >nul 2>&1
if %errorLevel% equ 0 (
    echo [OK] ApexCaddy: Khoi dong thanh cong.
) else (
    echo [!] ApexCaddy da dang chay hoac chua duoc tao service.
)

echo =================================================================
echo   TRANG THAI DICH VU:
echo =================================================================
sc query ApexBackend | findstr "STATE"
sc query ApexCaddy | findstr "STATE"
echo =================================================================
pause
