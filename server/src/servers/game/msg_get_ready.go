package main

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"log"
	"log/slog"
	pb "proto"
	"time"
)

func (p *Player) HandleGetReadyRequest(msg *pb.Message) {
	var req pb.GetReadyRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse GetReadyRequest Request", "error", err)
		return
	}

	conn, err := grpc.Dial(
		"127.0.0.1:8693",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		log.Printf("连接BattleServer失败: %s, 错误: %v", "127.0.0.1:8693", err)
		return
	}

	defer conn.Close()

	client := pb.NewRoomRpcServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	getReadyRpc := &pb.GetReadyRpcRequest{
		PlayerId: p.Uid,
	}

	resp, err := client.GetReadyRpc(ctx, getReadyRpc)
	if err != nil {
		slog.Error("房间内准备RPC调用失败: ", "error", err)
		return
	}

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("房间内准备失败，错误码: ", "error_code", resp.Ret)
	}

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.GetReadyResponse{
		Ret: resp.Ret,
	}))
}
