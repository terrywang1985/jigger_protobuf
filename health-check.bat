@echo off
chcp 65001 > nul

echo === Jigger Protobuf 服务器健康检查 ===
echo.

REM 检查Docker容器状态
echo 📊 检查容器状态...
docker-compose ps
echo.

REM 检查Redis健康状态
echo 🔍 检查 Redis 健康状态...
docker exec jigger-redis redis-cli ping 2>nul
if %errorlevel% equ 0 (
    echo ✅ Redis: 健康
) else (
    echo ❌ Redis: 不健康或未运行
)

REM 检查Login Server健康状态
echo 🔍 检查 Login Server 健康状态...
curl -s -o nul http://localhost:8081/health 2>nul
if %errorlevel% equ 0 (
    echo ✅ Login Server: 健康
) else (
    echo ❌ Login Server: 不健康或未运行
)

REM 检查Game Server WebSocket端口
echo 🔍 检查 Game Server WebSocket 端口...
netstat -an | findstr :18080 >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Game Server WebSocket: 端口18080开放
) else (
    echo ❌ Game Server WebSocket: 端口18080未开放
)

REM 检查Game Server TCP端口
echo 🔍 检查 Game Server TCP 端口...
netstat -an | findstr :12345 >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Game Server TCP: 端口12345开放
) else (
    echo ❌ Game Server TCP: 端口12345未开放
)

REM 检查Battle Server gRPC端口
echo 🔍 检查 Battle Server gRPC 端口...
netstat -an | findstr :50053 >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Battle Server gRPC: 端口50053开放
) else (
    echo ❌ Battle Server gRPC: 端口50053未开放
)

REM 检查Match Server gRPC端口
echo 🔍 检查 Match Server gRPC 端口...
netstat -an | findstr :50052 >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Match Server gRPC: 端口50052开放
) else (
    echo ❌ Match Server gRPC: 端口50052未开放
)

echo.
echo === Redis 连接测试 ===
docker exec jigger-redis redis-cli info replication 2>nul | findstr "role:master" 
if %errorlevel% equ 0 (
    echo ✅ Redis 作为主节点运行正常
) else (
    echo ❌ Redis 状态异常
)

echo.
echo === 内存使用情况 ===
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"

echo.
echo === 网络连通性测试 ===
REM 从game-server容器测试Redis连接
docker exec jigger-game-server sh -c "echo 'ping' | nc redis 6379" 2>nul | findstr "PONG" >nul
if %errorlevel% equ 0 (
    echo ✅ Game Server -> Redis: 连接正常
) else (
    echo ❌ Game Server -> Redis: 连接异常
)

echo.
echo 💡 如果发现问题，可以使用以下命令查看详细日志:
echo    docker-compose logs [service-name]
echo 💡 重启有问题的服务:
echo    docker-compose restart [service-name]

pause