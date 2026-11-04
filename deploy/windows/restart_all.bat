@echo off
chcp 65001 >nul
echo =================================================================
echo   KHOI DONG LAI HE THONG APEX FINTECH & CADDY (WINDOWS 2025)
echo =================================================================

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [CANH BAO] Ban can chay file bat nay voi quyen Run as Administrator de quan ly Services.
    pause
    exit /b 1
)

REM ---------------------------------------------------------------------------
REM Tu dong phat hien phuong an quan ly: Task Scheduler (production) hay NSSM (Windows Service)
REM ---------------------------------------------------------------------------
set "USE_TASK=0"
sc query ApexBackend >nul 2>&1
if not %errorlevel% neq 0 set "USE_TASK=1"

if "%USE_TASK%"=="1" (
    echo [*] Phat hien: Windows Task Scheduler ^(Apex_Backend_Server / Apex_Caddy_Server^)
) else (
    echo [*] Phat hien: Windows Service ^(ApexBackend / ApexCaddy^)
)

echo.
echo [*] Dang khoi dong lai Apex Backend...
if "%USE_TASK%"=="1" (
    schtasks /End /TN "Apex_Backend_Server" >nul 2>&1
    timeout /t 2 /nobreak >nul
    schtasks /Run /TN "Apex_Backend_Server" >nul 2>&1
) else (
    sc stop ApexBackend >nul 2>&1
    timeout /t 2 /nobreak >nul
    sc start ApexBackend >nul 2>&1
)
echo [OK] Apex Backend da khoi dong lai.

echo [*] Dang khoi dong lai Apex Caddy...
if "%USE_TASK%"=="1" (
    schtasks /End /TN "Apex_Caddy_Server" >nul 2>&1
    timeout /t 2 /nobreak >nul
    schtasks /Run /TN "Apex_Caddy_Server" >nul 2>&1
) else (
    sc stop ApexCaddy >nul 2>&1
    timeout /t 2 /nobreak >nul
    sc start ApexCaddy >nul 2>&1
)
echo [OK] Apex Caddy da khoi dong lai.

echo.
echo [*] Dang kiem tra health endpoint...
timeout /t 3 /nobreak >nul
curl -s -m 5 http://127.0.0.1:8080/health
echo.

echo.
echo =================================================================
echo   TRANG THAI HIEN TAI:
echo =================================================================
if "%USE_TASK%"=="1" (
    schtasks /Query /TN "Apex_Backend_Server" /FO LIST /V | findstr /I "Status TaskName"
    schtasks /Query /TN "Apex_Caddy_Server" /FO LIST /V | findstr /I "Status TaskName"
) else (
    sc query ApexBackend | findstr "STATE"
    sc query ApexCaddy | findstr "STATE"
)
echo =================================================================
pause
