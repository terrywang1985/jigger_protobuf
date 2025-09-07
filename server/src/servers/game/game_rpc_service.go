package main

import (
	"common/rpc"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log/slog"
	"net"
	"os"
	pb "proto"
)

type GameGRPCService struct {
	pb.UnimplementedGameRpcServiceServer // 实现gRPC接口
}

func (s *GameGRPCService) StartGameGRPCService() {
	port := rpc.GameServiceGRPCPort
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		slog.Error("Failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterGameRpcServiceServer(grpcServer, s)
	slog.Info("game grpc service starting", "port", port)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("game grpc service failed", "error", err)
		os.Exit(1)
	}
	slog.Info("gRPC server started", "port", port)
}

func (s *GameGRPCService) RoomStatusNotifyRpc(ctx context.Context, req *pb.RoomDetailNotify) (*pb.NotifyResponse, error) {
	// 处理房间状态通知逻辑
	slog.Info("Received RoomStatusNotifyRpc", "room_id", req.Room.Room.Id, "room_name", req.Room.Room.Name)

	//通过 被通知者id 找到玩家的连接
	player, ok := GlobalManager.GetPlayerByUin(req.BeNotifiedUid)
	if !ok {
		slog.Error("Player not found for notification", "player_id", req.BeNotifiedUid)
		return &pb.NotifyResponse{
			Ret: int32(pb.ErrorCode_NOT_FOUND),
		}, nil
	}

	noti := &pb.Message{
		Id:          pb.MessageId_ROOM_STATE_NOTIFICATION,
		MsgSerialNo: -1,
		ClientId:    "",
		Data:        mustMarshal(req),
	}

	player.SendMessage(noti)

	slog.Info("RoomStatusNotifyRpc processed", "be_notified_uid", req.BeNotifiedUid)

	// 这里可以添加具体的业务逻辑处理
	return &pb.NotifyResponse{
		Ret: int32(pb.ErrorCode_OK),
	}, nil
}

func (s *GameGRPCService) GameStateNotifyRpc(ctx context.Context, req *pb.GameStateNotify) (*pb.NotifyResponse, error) {
	// 处理游戏状态通知逻辑
	slog.Info("Received GameStateNotifyRpc", "room_id", req.RoomId, "game_state", req.GameState)

	//通过 被通知者id 找到玩家的连接
	player, ok := GlobalManager.GetPlayerByUin(req.BeNotifiedUid)
	if !ok {
		slog.Error("Player not found for notification", "player_id", req.BeNotifiedUid)
		return &pb.NotifyResponse{
			Ret: int32(pb.ErrorCode_NOT_FOUND),
		}, nil
	}

	noti := &pb.Message{
		Id:          pb.MessageId_GAME_STATE_NOTIFICATION,
		MsgSerialNo: -1,
		ClientId:    "",
		Data:        mustMarshal(req),
	}

	player.SendMessage(noti)

	slog.Info("GameStateNotifyRpc processed", "be_notified_uid", req.BeNotifiedUid)

	return &pb.NotifyResponse{
		Ret: int32(pb.ErrorCode_OK),
	}, nil
}

func (s *GameGRPCService) PlayerActionNotifyRpc(ctx context.Context, req *pb.PlayerActionNotify) (*pb.NotifyResponse, error) {
	// 处理玩家动作通知逻辑
	slog.Info("Received PlayerActionNotifyRpc", "room_id", req.RoomId, "action", req.Action)

	//通过 被通知者id 找到玩家的连接
	player, ok := GlobalManager.GetPlayerByUin(req.BeNotifiedUid)
	if !ok {
		slog.Error("Player not found for notification", "player_id", req.BeNotifiedUid)
		return &pb.NotifyResponse{
			Ret: int32(pb.ErrorCode_NOT_FOUND),
		}, nil
	}

	noti := &pb.Message{
		Id:          pb.MessageId_GAME_ACTION_NOTIFICATION,
		MsgSerialNo: -1,
		ClientId:    "",
		Data:        mustMarshal(req),
	}

	player.SendMessage(noti)

	slog.Info("PlayerActionNotifyRpc processed", "be_notified_uid", req.BeNotifiedUid)

	return &pb.NotifyResponse{
		Ret: int32(pb.ErrorCode_OK),
	}, nil
}
