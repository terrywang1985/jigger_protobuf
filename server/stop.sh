#!/bin/bash

echo "=== 停止 jigger_protobuf 服务器 ==="

# 停止Login Server
if [ -f logs/login-server.pid ]; then
    LOGIN_PID=$(cat logs/login-server.pid)
    echo "🛑 停止 Login Server (PID: $LOGIN_PID)..."
    if kill -0 $LOGIN_PID 2>/dev/null; then
        kill $LOGIN_PID
        echo "✅ Login Server 已停止"
    else
        echo "ℹ️ Login Server 未运行"
    fi
    rm -f logs/login-server.pid
else
    echo "ℹ️ Login Server 未运行"
fi

# 停止Game Server
if [ -f logs/game-server.pid ]; then
    GAME_PID=$(cat logs/game-server.pid)
    echo "🛑 停止 Game Server (PID: $GAME_PID)..."
    if kill -0 $GAME_PID 2>/dev/null; then
        kill $GAME_PID
        echo "✅ Game Server 已停止"
    else
        echo "ℹ️ Game Server 未运行"
    fi
    rm -f logs/game-server.pid
else
    echo "ℹ️ Game Server 未运行"
fi

# 停止Battle Server
if [ -f logs/battle-server.pid ]; then
    BATTLE_PID=$(cat logs/battle-server.pid)
    echo "🛑 停止 Battle Server (PID: $BATTLE_PID)..."
    if kill -0 $BATTLE_PID 2>/dev/null; then
        kill $BATTLE_PID
        echo "✅ Battle Server 已停止"
    else
        echo "ℹ️ Battle Server 未运行"
    fi
    rm -f logs/battle-server.pid
else
    echo "ℹ️ Battle Server 未运行"
fi

# 强制清理残留进程
echo "🧹 清理残留进程..."
pkill -f "game-server" 2>/dev/null
pkill -f "battle-server" 2>/dev/null
pkill -f "login-server" 2>/dev/null

echo ""
echo "=== 所有服务器已停止 ==="
echo "💡 使用 start.sh 重新启动所有服务器"