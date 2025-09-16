@echo off
echo === 启动jigger_protobuf游客登录测试环境 ===

:: 检查当前目录
if not exist "docker-compose.yml" (
    echo 错误: 请在jigger_protobuf根目录下运行此脚本
    pause
    exit /b 1
)

echo 1. 启动平台认证服务...
cd /d "%~dp0..\platform"
if exist "docker-compose.yml" (
    docker-compose up -d
    echo 平台服务启动中...
) else (
    echo 警告: 未找到platform/docker-compose.yml，请手动启动平台服务
)

cd /d "%~dp0"

echo 2. 等待平台服务启动...
timeout /t 5 /nobreak >nul

echo 3. 启动登录服务器...
cd server\src\servers\login
start "LoginServer" cmd /k "go run . || pause"
echo 登录服务器启动中...

cd ..\..\..\..

echo 4. 启动游戏服务器...
cd server\src\servers\game
start "GameServer" cmd /k "go run . || pause"
echo 游戏服务器启动中...

cd ..\..\..\..

echo.
echo === 服务启动完成 ===
echo 平台认证服务: http://localhost:8080
echo 登录服务器: http://localhost:8081
echo 游戏服务器: ws://localhost:18080/ws
echo.
echo 现在可以运行测试客户端:
echo cd client ^&^& python guest_auth_client.py
echo.
echo 按任意键退出...
pause >nul