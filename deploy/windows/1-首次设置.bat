@echo off
rem ---------------------------------------------------------------------------
rem MeowFlixEmby - first-run setup (double-click me).
rem
rem This shim is deliberately pure ASCII: under chcp 65001 cmd.exe tracks batch
rem file offsets in characters rather than bytes, so multi-line Chinese text in
rem a .bat desynchronises the parser and lines get executed as garbage commands.
rem All Chinese interaction therefore lives in deploy\windows\guide.ps1.
rem ---------------------------------------------------------------------------
chcp 65001 >nul
setlocal
title MeowFlixEmby - Setup

call :find_guide || goto :fail
powershell -NoProfile -ExecutionPolicy Bypass -File "%GUIDE%" -Action setup -ShimDir "%~dp0"
if errorlevel 1 goto :fail

endlocal
exit /b 0

rem --- Locate guide.ps1: release zip root, or a git checkout. -----------------
:find_guide
if exist "%~dp0deploy\windows\guide.ps1" (
    set "GUIDE=%~dp0deploy\windows\guide.ps1"
    exit /b 0
)
if exist "%~dp0guide.ps1" (
    set "GUIDE=%~dp0guide.ps1"
    exit /b 0
)
echo.
echo   [ERROR] guide.ps1 not found.
echo.
echo   Extract the whole .zip first, then double-click from the extracted folder.
exit /b 1

:fail
echo.
pause
endlocal
exit /b 1
