@echo off
chcp 65001 > nul

echo === 停止 Jigger Protobuf 服务器 ===

REM 停止所有容器
echo 🛑 停止所有容器...
docker-compose down

REM 清理未使用的容器和镜像（可选）
set /p cleanup="是否清理未使用的Docker资源？(y/N): "
if /i "%cleanup%"=="y" (
    echo 🧹 清理未使用的Docker资源...
    docker system prune -f
)

echo.
echo 🏁 所有服务已停止！
pause