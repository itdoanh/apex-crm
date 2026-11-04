@echo off
chcp 65001 >nul
echo =================================================================
echo   DUNG HE THONG APEX FINTECH & CADDY SERVER (WINDOWS 2025)
echo =================================================================

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [CANH BAO] Ban can chay file bat nay voi quyen Run as Administrator de quan ly Services.
    pause
    exit /b 1
)

REM Tu dong phat hien: Task Scheduler (production) hay Windows Service/NSSM
set "USE_TASK=0"
sc query ApexBackend >nul 2>&1
if not %errorlevel% neq 0 set "USE_TASK=1"

if "%USE_TASK%"=="1" (
    echo [*] Phat hien: Windows Task Scheduler ^(Apex_Backend_Server / Apex_Caddy_Server^)
) else (
    echo [*] Phat hien: Windows Service ^(ApexBackend / ApexCaddy^)
)

echo.
echo [*] Dang dung Apex Caddy...
if "%USE_TASK%"=="1" (
    schtasks /End /TN "Apex_Caddy_Server" >nul 2>&1
) else (
    sc stop ApexCaddy >nul 2>&1
)
echo [OK] Apex Caddy da dung.

echo [*] Dang dung Apex Backend...
if "%USE_TASK%"=="1" (
    schtasks /End /TN "Apex_Backend_Server" >nul 2>&1
) else (
    sc stop ApexBackend >nul 2>&1
)
echo [OK] Apex Backend da dung.

echo.
echo =================================================================
echo   HE THONG DA DUNG HOAN TOAN.
echo =================================================================
pause
