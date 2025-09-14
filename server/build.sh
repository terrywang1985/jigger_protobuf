#!/bin/bash

echo "=== 构建 jigger_protobuf 服务器 ==="

# 创建 bin 目录（如果不存在）
mkdir -p bin

# 设置环境变量
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# 构建各个服务器
echo "🔨 构建 Game Server..."
cd src/servers/game
go build -o ../../../bin/game-server .
if [ $? -ne 0 ]; then
    echo "❌ Game Server 构建失败"
    exit 1
fi
cd ../../..

echo "🔨 构建 Battle Server..."
cd src/servers/battle
go build -o ../../../bin/battle-server .
if [ $? -ne 0 ]; then
    echo "❌ Battle Server 构建失败"
    exit 1
fi
cd ../../..

echo "🔨 构建 Login Server..."
cd src/servers/login
go build -o ../../../bin/login-server ./loginserver.go
if [ $? -ne 0 ]; then
    echo "❌ Login Server 构建失败"
    exit 1
fi
cd ../../..

echo "✅ 所有服务器构建完成！"

echo ""
echo "=== 构建结果 ==="
ls -la bin/

echo ""
echo "💡 可执行文件位于 bin/ 目录"
echo "💡 配置文件位于 cfg/ 目录"
echo "💡 运行服务器前请确保在 server/ 目录下执行"