@echo off
title Apex Backend Server (PostgreSQL)
echo ============================================================
echo   Khoi dong Apex Backend Server (Go + PostgreSQL)
echo ============================================================

REM Kiem tra xem container apex-postgres co dang chay khong
docker ps --filter "name=apex-postgres" --format "{{.Names}}" | findstr /i "apex-postgres" >nul
if %ERRORLEVEL% neq 0 (
    echo [THONG BAO] Dang khoi dong container PostgreSQL (apex-postgres)...
    docker start apex-postgres >nul 2>&1
    if %ERRORLEVEL% neq 0 (
        echo [THONG BAO] Container chua ton tai, dang tao moi bang Docker...
        docker run -d --name apex-postgres -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=apex_leads -e TZ=Asia/Ho_Chi_Minh -p 5432:5432 -v apex_postgres_data:/var/lib/postgresql/data --restart unless-stopped postgres:16-alpine
    )
)

set PORT=8080
set DB_HOST=localhost
set DB_PORT=5432
set DB_USER=postgres
set DB_PASSWORD=postgres
set DB_NAME=apex_leads
set DB_SSLMODE=disable

if exist apex-server.exe (
    apex-server.exe
) else (
    go run .
)
pause
