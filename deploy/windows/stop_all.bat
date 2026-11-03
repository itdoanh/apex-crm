@echo off
chcp 65001 >nul
echo =================================================================
echo   DỪNG HỆ THỐNG APEX FINTECH & CADDY SERVER (WINDOWS 2025)
echo =================================================================

net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [CANH BAO] Ban can chay file bat nay voi quyen Run as Administrator de quan ly Services.
    pause
    exit /b 1
)

echo [*] Dang dung dich vu ApexCaddy...
sc stop ApexCaddy >nul 2>&1
echo [OK] ApexCaddy da dung.

echo [*] Dang dung dich vu ApexBackend...
sc stop ApexBackend >nul 2>&1
echo [OK] ApexBackend da dung.

echo =================================================================
echo   HE THONG DA DUNG HOAN TOAN.
echo =================================================================
pause
