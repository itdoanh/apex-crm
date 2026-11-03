@echo off
echo ============================================================
echo   Bien dich Apex Backend (Go + PostgreSQL) cho Windows x64
echo ============================================================

set GOOS=windows
set GOARCH=amd64
set GOMAXPROCS=1
set GOGC=25
go build -p 1 -ldflags="-s -w" -trimpath -o apex-server.exe .

if %ERRORLEVEL% equ 0 (
    echo [THANH CONG] File thuc thi apex-server.exe da duoc tao thanh cong!
) else (
    echo [LOI] Bien dich that bai!
)
pause
