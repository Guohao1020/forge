@echo off
REM Forge Dev Startup Script (Windows)
REM Usage: dev.bat

echo === Forge Dev Environment ===

REM 1. Start infrastructure
echo [1/4] Starting infrastructure (PostgreSQL + Redis + Temporal)...
docker compose -f docker-compose.dev.yml up -d
echo Waiting for Temporal to be ready...
timeout /t 10 /nobreak >nul

REM 2. Load .env if exists
if exist .env (
    for /f "usebackq tokens=1,* delims==" %%a in (".env") do (
        set "%%a=%%b"
    )
)

REM 3. Start forge-core
echo [2/4] Building and starting forge-core...
cd forge-core
go build -o forge-core.exe ./cmd/forge-core
if errorlevel 1 (
    echo ERROR: forge-core build failed
    cd ..
    exit /b 1
)
start "forge-core" cmd /c "forge-core.exe"
cd ..
timeout /t 3 /nobreak >nul

REM 4. Start forge-portal
echo [3/4] Starting forge-portal...
cd forge-portal
start "forge-portal" cmd /c "npm run dev"
cd ..

echo.
echo [4/4] All services started!
echo.
echo   Frontend:     http://localhost:3000
echo   API Server:   http://localhost:8080
echo   Temporal UI:  http://localhost:8233
echo   PostgreSQL:   localhost:5432
echo   Redis:        localhost:6379
echo.
echo   Login: admin / admin123
echo.
echo Close this window to stop services.
pause
