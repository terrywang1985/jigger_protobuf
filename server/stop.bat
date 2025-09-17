@echo off

echo ================================
echo    Stop jigger_protobuf Servers
echo ================================

REM Stop all related processes
echo.
echo [1/3] Stopping Login Server...
taskkill /f /im login-server.exe 2>nul
if %errorlevel% equ 0 (
    echo OK: Login Server stopped
) else (
    echo Info: Login Server not running
)

echo.
echo [2/3] Stopping Game Server...
taskkill /f /im game-server.exe 2>nul
if %errorlevel% equ 0 (
    echo OK: Game Server stopped
) else (
    echo Info: Game Server not running
)

echo.
echo [3/3] Stopping Battle Server...
taskkill /f /im battle-server.exe 2>nul
if %errorlevel% equ 0 (
    echo OK: Battle Server stopped
) else (
    echo Info: Battle Server not running
)

echo.
echo ================================
echo    All servers have been stopped
echo ================================
echo Tip: Use start.bat to restart all servers
pause