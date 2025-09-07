package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	pb "proto"
)

func main() {
	// 初始化日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// 创建MatchServer
	matchServer := NewOptimizedMatchServer()

	// 启动gRPC服务器
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMatchServiceServer(grpcServer, matchServer)

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		
		// 保存最终状态
		if err := matchServer.saveState(); err != nil {
			slog.Error("Failed to save final state", "error", err)
		}
		
		grpcServer.GracefulStop()
	}()

	slog.Info("MatchServer started", "port", 50052)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}