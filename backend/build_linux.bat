@echo off
chcp 65001 > nul
echo ============================================================
echo   Biên dịch Apex Backend cho máy chủ Linux (Ubuntu/Debian)
echo ============================================================

set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o apex-server .

if %ERRORLEVEL% equ 0 (
    echo [THÀNH CÔNG] File thực thi 'apex-server' (Linux 64-bit) đã được tạo!
    echo Bạn có thể upload file 'apex-server' này trực tiếp lên máy chủ Linux.
) else (
    echo [LỖI] Quá trình biên dịch thất bại!
)
pause
