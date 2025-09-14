@echo off
chcp 65001 > nul

echo === 停止 jigger_protobuf 服务器 ===

REM 停止所有相关进程
echo 🛑 停止 Login Server...
taskkill /f /im login-server.exe 2>nul
if %errorlevel% equ 0 (
    echo ✅ Login Server 已停止
) else (
    echo ℹ️ Login Server 未运行
)

echo 🛑 停止 Game Server...
taskkill /f /im game-server.exe 2>nul
if %errorlevel% equ 0 (
    echo ✅ Game Server 已停止
) else (
    echo ℹ️ Game Server 未运行
)

echo 🛑 停止 Battle Server...
taskkill /f /im battle-server.exe 2>nul
if %errorlevel% equ 0 (
    echo ✅ Battle Server 已停止
) else (
    echo ℹ️ Battle Server 未运行
)

echo.
echo === 所有服务器已停止 ===
echo 💡 使用 start.bat 重新启动所有服务器
pause