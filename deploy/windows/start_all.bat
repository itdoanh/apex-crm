@echo off
chcp 65001 >nul
echo =================================================================
echo   KHOI DONG HE THONG APEX FINTECH & CADDY SERVER (WINDOWS 2025)
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
echo [*] Dang khoi dong cac dich vu...
if "%USE_TASK%"=="1" (
    schtasks /Run /TN "Apex_Backend_Server" >nul 2>&1
    schtasks /Run /TN "Apex_Caddy_Server" >nul 2>&1
) else (
    sc start ApexBackend >nul 2>&1
    sc start ApexCaddy >nul 2>&1
)

timeout /t 3 /nobreak >nul

echo.
echo =================================================================
echo   TRANG THAI DICH VU:
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
