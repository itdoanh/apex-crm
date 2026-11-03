@echo off
chcp 65001 >nul
echo =================================================================
echo   KHỞI ĐỘNG LẠI HỆ THỐNG APEX FINTECH & CADDY (WINDOWS 2025)
echo =================================================================

net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [CANH BAO] Ban can chay file bat nay voi quyen Run as Administrator de quan ly Services.
    pause
    exit /b 1
)

echo [*] Dang khoi dong lai ApexBackend...
sc stop ApexBackend >nul 2>&1
timeout /t 2 /nobreak >nul
sc start ApexBackend >nul 2>&1
echo [OK] ApexBackend da khoi dong lai.

echo [*] Dang khoi dong lai ApexCaddy...
sc stop ApexCaddy >nul 2>&1
timeout /t 2 /nobreak >nul
sc start ApexCaddy >nul 2>&1
echo [OK] ApexCaddy da khoi dong lai.

echo =================================================================
echo   TRANG THAI HIEN TAI:
echo =================================================================
sc query ApexBackend | findstr "STATE"
sc query ApexCaddy | findstr "STATE"
echo =================================================================
pause
