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

func (p *Player) HandleJoinRoomRequest(msg *pb.Message) {
	var req pb.JoinRoomRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse JoinRoomRequest Request", "error", err)
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

	playerInitData := &pb.PlayerInitData{
		PlayerId:   p.Uid,
		PlayerName: p.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	joinRoomRpc := &pb.JoinRoomRpcRequest{
		RoomId: req.RoomId,
		Player: playerInitData,
	}

	resp, err := client.JoinRoomRpc(ctx, joinRoomRpc)
	if err != nil {
		slog.Error("加入房间RPC调用失败: ", "error", err)
		return
	}

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("加入房间，错误码: ", "error_code", resp.Ret)
	}

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.JoinRoomResponse{
		Ret: resp.Ret,
	}))
}
