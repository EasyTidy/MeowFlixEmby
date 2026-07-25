@echo off
rem ---------------------------------------------------------------------------
rem MeowFlixEmby - install the per-user login autostart entry (double-click me).
rem Pure ASCII by design; Chinese interaction lives in guide.ps1.
rem ---------------------------------------------------------------------------
chcp 65001 >nul
setlocal
title MeowFlixEmby - Install autostart

call :find_guide || goto :fail
powershell -NoProfile -ExecutionPolicy Bypass -File "%GUIDE%" -Action autostart-install -ShimDir "%~dp0"
set "CODE=%ERRORLEVEL%"

echo.
pause
endlocal
exit /b %CODE%

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
