#!/bin/bash

echo "=== 启动jigger_protobuf游客登录测试环境 ==="

# 检查当前目录
if [ ! -f "docker-compose.yml" ]; then
    echo "错误: 请在jigger_protobuf根目录下运行此脚本"
    exit 1
fi

echo "1. 启动平台认证服务..."
cd ../platform
if [ -f "docker-compose.yml" ]; then
    docker-compose up -d
    echo "平台服务启动中..."
else
    echo "警告: 未找到platform/docker-compose.yml，请手动启动平台服务"
fi

cd ../jigger_protobuf

echo "2. 等待平台服务启动..."
sleep 5

echo "3. 启动登录服务器..."
cd server/src/servers/login
go build -o loginserver . && ./loginserver &
LOGIN_PID=$!
echo "登录服务器已启动 (PID: $LOGIN_PID)"

cd ../../../..

echo "4. 启动游戏服务器..."
cd server/src/servers/game  
go build -o gameserver . && ./gameserver &
GAME_PID=$!
echo "游戏服务器已启动 (PID: $GAME_PID)"

cd ../../../..

echo ""
echo "=== 服务启动完成 ==="
echo "平台认证服务: http://localhost:8080"
echo "登录服务器: http://localhost:8081" 
echo "游戏服务器: ws://localhost:18080/ws"
echo ""
echo "现在可以运行测试客户端:"
echo "cd client && python guest_auth_client.py"
echo ""
echo "按 Ctrl+C 停止所有服务..."

# 捕获中断信号
trap 'echo "正在停止服务..."; kill $LOGIN_PID $GAME_PID 2>/dev/null; cd ../platform && docker-compose down; exit' INT

# 保持脚本运行
while true; do
    sleep 1
done