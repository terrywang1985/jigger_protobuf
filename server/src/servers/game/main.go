package main

import (
	"common/redisutil"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"
)

// 全局变量
var (
	GlobalRedis *redisutil.RedisPool
)

// 处理新连接
func handleConnection(conn net.Conn) {
	connID := GenerateConnID(conn)
	defer GlobalManager.DeletePlayer(connID)

	player := GlobalManager.GetOrCreatePlayer(connID, conn)

	// 设置认证超时为3秒
	authTimeout := time.NewTimer(3 * time.Second)
	defer authTimeout.Stop()

	select {
	case <-player.Done():
		// 玩家主动断开连接
		return
	case <-authTimeout.C:
		// 认证超时
		if !player.Authenticated {
			slog.Info("Authentication timeout (3s), closing connection", "conn_uuid", connID)
			player.Conn.Close()
			return
		}
	}

	// 等待玩家退出信号
	<-player.Done()
}

func main() {
	// 初始化全局Logger（JSON格式，级别为Debug）
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger) // 设为全局默认Logger

	// 初始化Redis连接池
	GlobalRedis = redisutil.NewRedisPool("localhost:6379", "", 0)

	// 测试Redis连接
	if err := testRedisConnection(); err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	//启动 grpc
	service := &GameGRPCService{}

	// 启动gRPC服务器
	go service.StartGameGRPCService()

	// 启动服务器
	listener, err := net.Listen("tcp", ":12345")
	if err != nil {
		slog.Error("Failed to start server", "error", err)
	}
	defer listener.Close()
	//fmt.Println("Server started at :12345")

	// 其他代码直接调用slog的方法即可
	slog.Info("Server started", "port", 12345)

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("Failed to accept connection", "error", err)
			continue
		}
		go handleConnection(conn)
	}
}

// 测试Redis连接
func testRedisConnection() error {
	// 使用Exists命令测试连接
	_, err := GlobalRedis.Exists("test_connection")
	if err != nil {
		return fmt.Errorf("redis connection failed: %v", err)
	}
	return nil
}
