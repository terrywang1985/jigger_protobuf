// ws_handler.go
package main

import (
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境应根据需要检查Origin
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	// 创建WSConnection包装器
	conn := NewWSConnection(wsConn)

	// 使用与TCP相同的连接处理逻辑
	go handleConnection(conn)
}
