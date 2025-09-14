@echo off
chcp 65001 > nul

echo === Jigger Protobuf 服务器启动脚本 ===

REM 检查Docker是否运行
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Docker 未运行，请先启动 Docker Desktop
    pause
    exit /b 1
)

echo ✅ Docker 运行正常

REM 停止现有容器（如果存在）
echo 🛑 停止现有容器（如果存在）...
docker-compose down

REM 构建并启动所有服务
echo 🔨 构建并启动所有服务...
docker-compose up -d --build

REM 等待服务启动
echo ⏳ 等待服务启动...
timeout /t 10 > nul

REM 检查服务状态
echo 📊 检查服务状态...
docker-compose ps

echo.
echo === 服务地址 ===
echo 🔗 Redis:          localhost:6379
echo 🔗 Login Server:   localhost:8081
echo 🔗 Game Server:    localhost:18080 (WebSocket), localhost:12345 (TCP)
echo 🔗 Battle Server:  localhost:50053 (gRPC)
echo 💡 Match Server:   已暂时禁用

echo.
echo === 查看日志命令 ===
echo docker-compose logs -f [service-name]
echo 例如: docker-compose logs -f game-server

echo.
echo === 健康检查 ===
echo 正在检查服务健康状态...

REM 等待健康检查
timeout /t 5 > nul

REM 检查Redis
docker exec jigger-redis redis-cli ping | find "PONG" >nul
if %errorlevel% equ 0 (
    echo ✅ Redis: 健康
) else (
    echo ❌ Redis: 不健康
)

REM 检查Login Server
curl -s http://localhost:8081/health >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Login Server: 健康
) else (
    echo ❌ Login Server: 不健康
)

echo.
echo 🚀 所有服务已启动完成！
echo 💡 使用 'docker-compose logs -f' 查看所有服务日志
echo 💡 使用 'docker-compose down' 停止所有服务
pause