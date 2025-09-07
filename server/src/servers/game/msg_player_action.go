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

func (p *Player) HandlePlayerActionRequest(msg *pb.Message) {
	var req pb.GameActionRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse GameActionRequest Request", "error", err)
		return
	}

	//创建grp client 并给battleserver阻塞发送,  grpc CreateRoom
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

	actionRpc := &pb.PlayerActionRpcRequest{
		PlayerId: p.Uid,
		Action:   req.Action,
	}

	resp, err := client.PlayerActionRpc(ctx, actionRpc)
	if err != nil {
		slog.Error("玩家Action RPC调用失败: ", "error", err)
		return
	}

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("玩家Action，错误码: ", "error_code", resp.Ret)
	}

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.GameActionResponse{
		Ret: resp.Ret,
	}))
}
