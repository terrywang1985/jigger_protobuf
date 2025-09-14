@echo off
chcp 65001 > nul

echo === 启动 jigger_protobuf 服务器 ===

REM 检查二进制文件是否存在
if not exist bin\game-server.exe (
    echo ❌ Game Server 可执行文件不存在，请先运行 build.bat
    pause
    exit /b 1
)

if not exist bin\battle-server.exe (
    echo ❌ Battle Server 可执行文件不存在，请先运行 build.bat
    pause
    exit /b 1
)

if not exist bin\login-server.exe (
    echo ❌ Login Server 可执行文件不存在，请先运行 build.bat
    pause
    exit /b 1
)

REM 检查配置文件
if not exist cfg\cfg_tbdrawcard.json (
    echo ❌ 配置文件不存在，请检查 cfg 目录
    pause
    exit /b 1
)

echo 🚀 启动 Login Server...
cd bin
start "Login Server" login-server.exe
timeout /t 2 > nul

echo 🚀 启动 Game Server...
start "Game Server" game-server.exe
timeout /t 2 > nul

echo 🚀 启动 Battle Server...
start "Battle Server" battle-server.exe
timeout /t 2 > nul

cd ..

echo.
echo === 服务器启动完成 ===
echo 💡 Login Server:  http://localhost:8081
echo 💡 Game Server:   WebSocket: ws://localhost:18080/ws, TCP: localhost:12345, gRPC: localhost:50051
echo 💡 Battle Server: gRPC: localhost:50053
echo.
echo 💡 使用 stop.bat 停止所有服务器
echo 💡 查看服务器窗口以监控运行状态
pause