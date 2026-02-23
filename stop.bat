@echo off
setlocal enabledelayedexpansion

set GREEN=[92m
set YELLOW=[93m
set RED=[91m
set RESET=[0m

set LOGFILE=%~dp0stop.log
echo [%date% %time%] Stopping Server Manager > "%LOGFILE%"

echo %YELLOW%Stopping Server Manager...%RESET%
echo.

REM ── Backend ───────────────────────────────────────────────────────────────
echo Stopping backend...
taskkill /IM "server-manager.exe" /F >nul 2>&1
if %errorlevel% equ 0 (
    echo %GREEN%  Backend stopped%RESET%
    echo [%date% %time%] Backend: stopped >> "%LOGFILE%"
) else (
    echo %YELLOW%  Backend was not running%RESET%
    echo [%date% %time%] Backend: was not running >> "%LOGFILE%"
)

REM ── Frontend ──────────────────────────────────────────────────────────────
echo Stopping frontend...
powershell -NoProfile -Command "Get-WmiObject Win32_Process -Filter \"name='node.exe'\" | Where-Object { $_.CommandLine -like '*vite*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }"
echo %GREEN%  Frontend stopped%RESET%
echo [%date% %time%] Frontend: stopped >> "%LOGFILE%"

REM ── Close any lingering terminal windows opened by start.bat ───────────────
taskkill /FI "WINDOWTITLE eq Server Manager Backend" /F /T >nul 2>&1
taskkill /FI "WINDOWTITLE eq Server Manager Frontend" /F /T >nul 2>&1

echo. >> "%LOGFILE%"
echo.
echo %GREEN%==================================%RESET%
echo %GREEN%Server Manager stopped%RESET%
echo %GREEN%==================================%RESET%
echo Log written to: %LOGFILE%
pause
