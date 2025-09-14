package main

import (
	"common/discovery"
	"common/redisutil"
	"common/rpc"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	pb "proto"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// BattleServer 结构体
type BattleServer struct {
	pb.UnimplementedRoomRpcServiceServer // 实现gRPC接口

	RedisPool    *redisutil.RedisPool
	BattleRooms  map[string]*BattleRoom
	RoomsMutex   sync.RWMutex
	PlayerInRoom map[uint64]string // 玩家ID到房间ID的映射
	PlayersMutex sync.RWMutex
	Discovery    discovery.Discovery
	InstanceID   string
}

func main() {
	// 初始化日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
	slog.Info("Starting Battle Server...")

	// 初始化Redis连接池
	redisConfig := redisutil.LoadRedisConfigFromEnv()
	redisPool := redisutil.NewRedisPoolFromConfig(redisConfig)
	defer redisPool.Close()

	// 创建BattleServer实例
	server := &BattleServer{
		RedisPool:    redisPool,
		BattleRooms:  make(map[string]*BattleRoom),
		PlayerInRoom: make(map[uint64]string),
	}

	// 注册服务发现
	server.registerServiceDiscovery()

	// 启动gRPC服务器
	server.startRoomGRPCServer(rpc.RoomServiceGRPCPort, server.InstanceID)

	slog.Info("Battle server is running")
	select {} // 阻塞主线程
}

// 注册服务发现
func (s *BattleServer) registerServiceDiscovery() {
	s.InstanceID = generateInstanceID()
	disc := discovery.NewRedisDiscovery(s.RedisPool, "prod_")

	// 获取本机IP
	hostIP, err := getLocalIP()
	if err != nil {
		slog.Error("Failed to get local IP", "error", err)
		hostIP = "127.0.0.1"
	}

	grpcAddr := fmt.Sprintf("%s:%d", hostIP, rpc.RoomServiceGRPCPort)

	instance := &discovery.ServiceInstance{
		ServiceName: "battle-server",
		InstanceID:  s.InstanceID,
		Address:     grpcAddr,
		Metadata: map[string]string{
			"version": "1.0",
		},
	}

	// 注册服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := disc.Register(ctx, instance); err != nil {
		slog.Error("Failed to register service", "error", err)
		os.Exit(1)
	}
	slog.Info("Service registered", "instance", s.InstanceID, "address", grpcAddr)

	s.Discovery = disc

	// 心跳协程
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := disc.Heartbeat(ctx, s.InstanceID); err != nil {
				slog.Error("Heartbeat failed", "error", err)
			} else {
				slog.Debug("Heartbeat sent")
			}
			cancel()
		}
	}()
}

// 启动gRPC服务器
func (s *BattleServer) startRoomGRPCServer(port int, instanceID string) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		slog.Error("Failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRoomRpcServiceServer(grpcServer, s)
	slog.Info("gRPC server starting", "port", port)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("gRPC server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("gRPC server started", "port", port, "instance_id", instanceID)
}

func (s *BattleServer) CreateRoomRpc(ctx context.Context, req *pb.CreateRoomRpcRequest) (*pb.CreateRoomRpcResponse, error) {

	s.RoomsMutex.Lock()
	defer s.RoomsMutex.Unlock()

	s.PlayersMutex.Lock()
	defer s.PlayersMutex.Unlock()

	//如果玩家已经在房间中，返回错误
	if roomID, exists := s.PlayerInRoom[req.Player.PlayerId]; exists {
		slog.Warn("Player already in room", "player_id", req.Player.PlayerId, "room_id", roomID)
		return &pb.CreateRoomRpcResponse{Ret: pb.ErrorCode_PLAYER_ALREADY_IN_ROOM}, nil
	}

	roomID := strconv.FormatUint(req.Player.PlayerId, 10)

	// 创建新战斗房间
	room := NewBattleRoom(roomID, s, GameType_WordCardGame)
	room.AddPlayer(req.Player.PlayerId, req.Player.PlayerName)
	room.Run()

	s.BattleRooms[roomID] = room
	s.PlayerInRoom[req.Player.PlayerId] = roomID

	slog.Info("Battle room created", "room_id", roomID)

	//todo: room 跟哪个 BattleServer 关联 需要写入到redis里，后续 GameServer收到加入房间请求时可以通过BattleServer的实例ID找到对应的BattleServer

	return &pb.CreateRoomRpcResponse{Ret: pb.ErrorCode_OK, RoomId: roomID}, nil
}

func (s *BattleServer) JoinRoomRpc(ctx context.Context, req *pb.JoinRoomRpcRequest) (*pb.JoinRoomRpcResponse, error) {
	s.RoomsMutex.RLock()
	room, exists := s.BattleRooms[req.RoomId]
	s.RoomsMutex.RUnlock()

	if !exists {
		return &pb.JoinRoomRpcResponse{Ret: pb.ErrorCode_INVALID_ROOM}, nil
	}

	room.AddPlayer(req.Player.PlayerId, req.Player.PlayerName)

	s.PlayersMutex.Lock()
	defer s.PlayersMutex.Unlock()

	s.PlayerInRoom[req.Player.PlayerId] = req.RoomId

	slog.Info("Player joined room", "room_id", req.RoomId, "player", req.Player.PlayerId)

	// 返回当前房间玩家列表
	playerList := make([]*pb.PlayerInitData, 0, len(room.Players))
	for playerID := range room.Players {
		playerList = append(playerList, &pb.PlayerInitData{
			PlayerId: playerID,
		})
	}

	return &pb.JoinRoomRpcResponse{
		Ret:     pb.ErrorCode_OK,
		RoomId:  req.RoomId,
		Players: playerList,
	}, nil

}

func (s *BattleServer) LeaveRoomRpc(ctx context.Context, req *pb.LeaveRoomRpcRequest) (*pb.LeaveRoomRpcResponse, error) {

	return &pb.LeaveRoomRpcResponse{Ret: pb.ErrorCode_OK}, nil
}

func (s *BattleServer) GetReadyRoomRpc(ctx context.Context, req *pb.GetReadyRpcRequest) (*pb.GetReadyRpcResponse, error) {
	//找到玩家在哪个房间
	s.PlayersMutex.RLock()
	roomId, exists := s.PlayerInRoom[req.PlayerId]
	s.PlayersMutex.RUnlock()

	if !exists {
		return &pb.GetReadyRpcResponse{Ret: pb.ErrorCode_INVALID_ROOM}, nil
	}

	//找到房间
	s.RoomsMutex.RLock()
	room, roomExists := s.BattleRooms[roomId]
	s.RoomsMutex.Unlock()

	if !roomExists {
		return &pb.GetReadyRpcResponse{Ret: pb.ErrorCode_INVALID_ROOM}, nil
	}

	//设置状态
	room.SetPlayerReady(req.PlayerId)

	//房间人数大于等于2人，且全部准备，开始游戏
	if len(room.Players) >= 2 && room.AllPlayersReady() {
		slog.Info("All players ready, starting game", "room_id", roomId)
		room.StartGame()
	}

	return &pb.GetReadyRpcResponse{
		Ret:    pb.ErrorCode_OK,
		RoomId: roomId,
	}, nil
}

// PlayerAction 处理玩家操作
func (s *BattleServer) PlayerActionRpc(ctx context.Context, req *pb.PlayerActionRpcRequest) (*pb.PlayerActionRpcResponse, error) {
	s.RoomsMutex.RLock()
	room, exists := s.BattleRooms[req.RoomId]
	s.RoomsMutex.RUnlock()

	if !exists {
		return &pb.PlayerActionRpcResponse{Ret: pb.ErrorCode_INVALID_ROOM}, nil
	}

	// 将操作发送到房间的命令通道
	room.CmdChan <- Command{
		PlayerID: req.PlayerId,
		Action:   req.Action,
	}

	return &pb.PlayerActionRpcResponse{Ret: pb.ErrorCode_OK}, nil
}

// 获取本机IP
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no valid local IP found")
}

// 生成唯一实例ID
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())
}
