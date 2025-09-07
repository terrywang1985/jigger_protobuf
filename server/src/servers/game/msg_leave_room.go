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

func (p *Player) HandleLeaveRoomRequest(msg *pb.Message) {
	var req pb.LeaveRoomRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse LeaveRoomRequest Request", "error", err)
		return
	}

	//暂时连接到固定的 BattleServer地址，后续通过redis做服务发现，获得一个空闲的 BattleServer地址
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

	leaveRoomRpc := &pb.LeaveRoomRpcRequest{}

	resp, err := client.LeaveRoomRpc(ctx, leaveRoomRpc)
	if err != nil {
		slog.Error("离开房间RPC调用失败: ", "error", err)
		return
	}

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("离开房间失败，错误码: ", "error_code", resp.Ret)
	}

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.LeaveRoomResponse{
		Ret: resp.Ret,
	}))
}
