#!/bin/bash

echo "=== Jigger Protobuf 服务器启动脚本 ==="

# 检查Docker是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker 未运行，请先启动 Docker Desktop"
    exit 1
fi

echo "✅ Docker 运行正常"

# 停止现有容器（如果存在）
echo "🛑 停止现有容器（如果存在）..."
docker-compose down

# 构建并启动所有服务
echo "🔨 构建并启动所有服务..."
docker-compose up -d --build

# 等待服务启动
echo "⏳ 等待服务启动..."
sleep 10

# 检查服务状态
echo "📊 检查服务状态..."
docker-compose ps

echo ""
echo "=== 服务地址 ==="
echo "🔗 Redis:          localhost:6379"
echo "🔗 Login Server:   localhost:8081"
echo "🔗 Game Server:    localhost:18080 (WebSocket), localhost:12345 (TCP)"
echo "🔗 Battle Server:  localhost:50053 (gRPC)"
echo "🔗 Match Server:   localhost:50052 (gRPC)"

echo ""
echo "=== 查看日志命令 ==="
echo "docker-compose logs -f [service-name]"
echo "例如: docker-compose logs -f game-server"

echo ""
echo "=== 健康检查 ==="
echo "正在检查服务健康状态..."

# 等待健康检查
sleep 5

# 检查Redis
if docker exec jigger-redis redis-cli ping | grep -q PONG; then
    echo "✅ Redis: 健康"
else
    echo "❌ Redis: 不健康"
fi

# 检查Login Server
if curl -s http://localhost:8081/health > /dev/null; then
    echo "✅ Login Server: 健康"
else
    echo "❌ Login Server: 不健康"
fi

echo ""
echo "🚀 所有服务已启动完成！"
echo "💡 使用 'docker-compose logs -f' 查看所有服务日志"
echo "💡 使用 'docker-compose down' 停止所有服务"