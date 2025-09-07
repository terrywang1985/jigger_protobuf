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

func (p *Player) HandleCreateRoomRequest(msg *pb.Message) {
	var req pb.StartGameBattleRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse GetUserInfoRequest Request", "error", err)
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

	player := &pb.PlayerInitData{
		PlayerId:   p.Uid,
		PlayerName: p.Name,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	createRoomReq := &pb.CreateRoomRpcRequest{
		Player: player,
	}

	resp, err := client.CreateRoomRpc(ctx, createRoomReq)
	if err != nil {
		slog.Error("创建房间RPC调用失败: ", "error", err)
		return
	}

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("创建房间失败，错误码: ", "error_code", resp.Ret)
	}

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.CreateRoomResponse{
		Ret: resp.Ret,
		Room: &pb.Room{
			Id:   resp.RoomId,
			Name: "房间名称", // 这里可以根据实际情况设置房间名称
		},
	}))
}
