@echo off
chcp 65001 > nul

echo === Jigger Protobuf 服务器日志查看器 ===
echo.
echo 可用服务:
echo 1. redis
echo 2. login-server
echo 3. game-server
echo 4. battle-server
echo 5. match-server
echo 6. 查看所有服务日志
echo.

set /p choice="请选择要查看的服务 (1-6): "

if "%choice%"=="1" (
    echo 查看 Redis 日志...
    docker-compose logs -f redis
) else if "%choice%"=="2" (
    echo 查看 Login Server 日志...
    docker-compose logs -f login-server
) else if "%choice%"=="3" (
    echo 查看 Game Server 日志...
    docker-compose logs -f game-server
) else if "%choice%"=="4" (
    echo 查看 Battle Server 日志...
    docker-compose logs -f battle-server
) else if "%choice%"=="5" (
    echo 查看 Match Server 日志...
    docker-compose logs -f match-server
) else if "%choice%"=="6" (
    echo 查看所有服务日志...
    docker-compose logs -f
) else (
    echo 无效选择！
    pause
)