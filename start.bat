@echo off
setlocal enabledelayedexpansion

REM Colors and formatting
set GREEN=[92m
set YELLOW=[93m
set RED=[91m
set RESET=[0m

echo %YELLOW%Starting Server Manager...%RESET%

REM Get the directory where the script is located
set SCRIPT_DIR=%~dp0

REM Check if backend executable exists
if not exist "%SCRIPT_DIR%backend\server-manager.exe" (
    echo %RED%Error: Backend executable not found at %SCRIPT_DIR%backend\server-manager.exe%RESET%
    exit /b 1
)

REM Check if frontend dependencies are installed
if not exist "%SCRIPT_DIR%frontend\node_modules" (
    echo %YELLOW%Installing frontend dependencies...%RESET%
    cd /d "%SCRIPT_DIR%frontend"
    call npm install
    if errorlevel 1 (
        echo %RED%Failed to install frontend dependencies%RESET%
        exit /b 1
    )
)

REM Start backend
echo %GREEN%Starting backend...%RESET%
cd /d "%SCRIPT_DIR%backend"
start "Server Manager Backend" cmd /k "server-manager.exe"
echo %GREEN%Backend started%RESET%

REM Wait a moment for backend to start
timeout /t 2 /nobreak

REM Start frontend
echo %GREEN%Starting frontend...%RESET%
cd /d "%SCRIPT_DIR%frontend"
start "Server Manager Frontend" cmd /k "npm run dev"
echo %GREEN%Frontend started%RESET%

echo %GREEN%==================================%RESET%
echo %GREEN%Server Manager is running!%RESET%
echo %GREEN%==================================%RESET%
echo Backend:  %YELLOW%http://localhost:8090%RESET%
echo Frontend: %YELLOW%http://localhost:5173%RESET%
echo.
echo Close the terminal windows to stop the services
echo %GREEN%==================================%RESET%

exit /b 0
