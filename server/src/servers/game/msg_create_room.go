package main

import (
	"context"
	"log"
	"log/slog"
	pb "proto"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func (p *Player) HandleCreateRoomRequest(msg *pb.Message) {
	slog.Info("HandleCreateRoomRequest called", "player_id", p.Uid, "message_id", msg.GetId())

	var req pb.CreateRoomRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse CreateRoomRequest", "error", err)
		return
	}

	slog.Info("CreateRoomRequest parsed", "player_id", p.Uid, "room_name", req.GetName())

	//创建grp client 并给battleserver阻塞发送,  grpc CreateRoom
	slog.Info("Attempting to connect to BattleServer", "address", "127.0.0.1:8693")
	conn, err := grpc.Dial(
		"127.0.0.1:8693",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		log.Printf("连接BattleServer失败: %s, 错误: %v", "127.0.0.1:8693", err)
		slog.Error("Failed to connect to BattleServer", "error", err)
		// 即使连接失败，也应该给客户端发送响应
		p.SendResponse(msg, mustMarshal(&pb.CreateRoomResponse{
			Ret: pb.ErrorCode_SERVER_ERROR,
			Room: &pb.Room{
				Id:   "",
				Name: req.GetName(),
			},
		}))
		return
	}

	slog.Info("Connected to BattleServer successfully")
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

	slog.Info("Calling CreateRoomRpc", "player_id", p.Uid)
	resp, err := client.CreateRoomRpc(ctx, createRoomReq)
	if err != nil {
		slog.Error("创建房间RPC调用失败: ", "error", err)
		// 即使RPC调用失败，也应该给客户端发送响应
		p.SendResponse(msg, mustMarshal(&pb.CreateRoomResponse{
			Ret: pb.ErrorCode_SERVER_ERROR,
			Room: &pb.Room{
				Id:   "",
				Name: req.GetName(),
			},
		}))
		return
	}

	slog.Info("CreateRoomRpc response", "player_id", p.Uid, "ret", resp.GetRet(), "room_id", resp.GetRoomId())

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("创建房间失败，错误码: ", "error_code", resp.Ret)
	}

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.CreateRoomResponse{
		Ret: resp.Ret,
		Room: &pb.Room{
			Id:   resp.RoomId,
			Name: req.GetName(), // 使用客户端提供的房间名称
		},
	}))
}
