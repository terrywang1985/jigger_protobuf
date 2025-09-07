package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	pb "proto"
	"google.golang.org/grpc"
)

type OptimizedMatchServer struct {
	pb.UnimplementedMatchServiceServer
	mu           sync.RWMutex
	matchQueue   map[uint64]*pb.MatchRequest // 玩家ID到匹配请求的映射
	lastActivity map[uint64]time.Time        // 玩家最后活动时间
}

func NewOptimizedMatchServer() *OptimizedMatchServer {
	server := &OptimizedMatchServer{
		matchQueue:   make(map[uint64]*pb.MatchRequest),
		lastActivity: make(map[uint64]time.Time),
	}

	// 启动后台匹配协程
	go server.backgroundMatcher()

	// 启动状态保存协程
	go server.periodicStateSaver()

	// 恢复状态
	server.restoreState()

	return server
}

// 处理匹配请求
func (s *OptimizedMatchServer) StartMatch(ctx context.Context, req *pb.MatchRequest) (*pb.MatchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerID := req.GetPlayerId()
	slog.Info("Player joined match queue", "player_id", playerID)

	// 如果已经在匹配队列中，则忽略
	if _, exists := s.matchQueue[playerID]; exists {
		return &pb.MatchResponse{Ret: pb.ErrorCode_OK}, nil
	}

	// 添加到匹配队列
	s.matchQueue[playerID] = req
	s.lastActivity[playerID] = time.Now()

	// 存储玩家数据到Redis
	if err := storeMatchPlayer(playerID, req.GetPlayerData()); err != nil {
		slog.Error("Failed to store player data in Redis", "player_id", playerID, "error", err)
	}

	return &pb.MatchResponse{Ret: pb.ErrorCode_OK}, nil
}

// 保存状态到Redis
func (s *OptimizedMatchServer) saveState() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := struct {
		MatchQueue   map[uint64]*pb.MatchRequest
		LastActivity map[uint64]time.Time
	}{
		MatchQueue:   s.matchQueue,
		LastActivity: s.lastActivity,
	}

	return saveServerState(state)
}

// 从Redis恢复状态
func (s *OptimizedMatchServer) restoreState() {
	state, err := restoreServerState()
	if err != nil {
		slog.Info("No saved state found in Redis", "error", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.matchQueue = state.MatchQueue
	s.lastActivity = state.LastActivity

	slog.Info("State restored from Redis", "players", len(s.matchQueue))
}

// 定期保存状态
func (s *OptimizedMatchServer) periodicStateSaver() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.saveState(); err != nil {
			slog.Error("Failed to save state to Redis", "error", err)
		} else {
			slog.Debug("State saved to Redis")
		}
	}
}