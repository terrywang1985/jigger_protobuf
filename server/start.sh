#!/bin/bash

# 参数化启动脚本
# 用法: ./start.sh [service_name] 或 ./start.sh all
# service_name: login, game, battle

SERVICE_NAME="$1"

if [ -z "$SERVICE_NAME" ]; then
    echo "用法: $0 [service_name|all]"
    echo "service_name: login, game, battle"
    echo "例如: $0 game  # 启动游戏服务器"
    echo "例如: $0 all   # 启动所有服务器"
    exit 1
fi

# 创建日志目录
mkdir -p logs

# 启动单个服务的函数（后台模式）
start_service() {
    local service="$1"
    local service_name="$2"
    local port_info="$3"
    
    if [ ! -f "bin/${service}-server" ]; then
        echo "❌ ${service_name} 可执行文件不存在，请先运行 build.sh"
        return 1
    fi
    
    echo "🚀 启动 ${service_name}..."
    cd bin
    nohup "./${service}-server" > "../logs/${service}-server.log" 2>&1 &
    PID=$!
    echo $PID > "../logs/${service}-server.pid"
    cd ..
    sleep 2
    
    if kill -0 $PID 2>/dev/null; then
        echo "✅ ${service_name} 启动成功 (PID: $PID) ${port_info}"
    else
        echo "❌ ${service_name} 启动失败，请检查日志: logs/${service}-server.log"
        return 1
    fi
}

# 启动单个服务的函数（前台模式，用于Docker）
start_service_foreground() {
    local service="$1"
    local service_name="$2"
    local port_info="$3"
    
    if [ ! -f "bin/${service}-server" ]; then
        echo "❌ ${service_name} 可执行文件不存在，请先运行 build.sh"
        exit 1
    fi
    
    echo "🚀 启动 ${service_name}..."
    echo "✅ ${service_name} ${port_info}"
    cd bin
    exec "./${service}-server"
}

# 检查配置文件
if [ ! -f cfg/cfg_tbdrawcard.json ]; then
    echo "❌ 配置文件不存在，请检查 cfg 目录"
    exit 1
fi

case "$SERVICE_NAME" in
    "login")
        # 检查是否在Docker环境中
        if [ -f /.dockerenv ]; then
            start_service_foreground "login" "Login Server" "http://localhost:8081"
        else
            start_service "login" "Login Server" "http://localhost:8081"
        fi
        ;;
    "game")
        if [ -f /.dockerenv ]; then
            start_service_foreground "game" "Game Server" "WebSocket: ws://localhost:18080/ws, TCP: localhost:12345, gRPC: localhost:50051"
        else
            start_service "game" "Game Server" "WebSocket: ws://localhost:18080/ws, TCP: localhost:12345, gRPC: localhost:50051"
        fi
        ;;
    "battle")
        if [ -f /.dockerenv ]; then
            start_service_foreground "battle" "Battle Server" "gRPC: localhost:50053"
        else
            start_service "battle" "Battle Server" "gRPC: localhost:50053"
        fi
        ;;
    "all")
        echo "=== 启动所有 jigger_protobuf 服务器 ==="
        
        start_service "login" "Login Server" "http://localhost:8081"
        start_service "game" "Game Server" "WebSocket: ws://localhost:18080/ws, TCP: localhost:12345, gRPC: localhost:50051"
        start_service "battle" "Battle Server" "gRPC: localhost:50053"
        
        echo ""
        echo "=== 所有服务器启动完成 ==="
        echo "💡 日志文件位于 logs/ 目录"
        echo "💡 使用 stop.sh 停止所有服务器"
        echo "💡 使用 'tail -f logs/服务器名-server.log' 查看实时日志"
        ;;
    *)
        echo "❌ 未知的服务名称: $SERVICE_NAME"
        echo "支持的服务: login, game, battle, all"
        exit 1
        ;;
esac