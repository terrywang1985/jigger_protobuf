package main

import (
	"common/redisutil"
	"context"
	"sync"
	"time"

	pb "proto"
)

type OptimizedMatchServer struct {
	pb.UnimplementedMatchServiceServer

	mu           sync.RWMutex
	matchQueue   map[uint64]*pb.MatchRequest // 玩家ID到匹配请求的映射
	lastActivity map[uint64]time.Time        // 玩家最后活动时间（用于超时）
	redisPool    *redisutil.RedisPool
}

func NewOptimizedMatchServer(redisAddr string) *OptimizedMatchServer {
	// 初始化Redis连接池
	redisPool := redisutil.NewRedisPool(redisAddr, "", 0)

	server := &OptimizedMatchServer{
		matchQueue:   make(map[uint64]*pb.MatchRequest),
		lastActivity: make(map[uint64]time.Time),
		redisPool:    redisPool,
	}

	// 启动后台匹配协程
	go server.backgroundMatcher()

	// 启动状态保存协程
	go server.periodicStateSaver()

	// 恢复状态
	server.restoreState()

	return server
}

func (s *OptimizedMatchServer) StartMatch(ctx context.Context, req *pb.MatchRequest) (*pb.MatchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerID := req.GetPlayerId()

	// 如果已经在匹配队列中，则忽略
	if _, exists := s.matchQueue[playerID]; exists {
		return &pb.MatchResponse{Ret: pb.ErrorCode_OK}, nil
	}

	// 添加到匹配队列
	s.matchQueue[playerID] = req
	s.lastActivity[playerID] = time.Now()

	return &pb.MatchResponse{Ret: pb.ErrorCode_OK}, nil
}

func (s *OptimizedMatchServer) CancelMatch(ctx context.Context, req *pb.CancelMatchRequest) (*pb.ErrorCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerID := req.GetPlayerId()
	if _, exists := s.matchQueue[playerID]; exists {
		delete(s.matchQueue, playerID)
		delete(s.lastActivity, playerID)
	}

	ret := pb.ErrorCode_OK
	return &ret, nil
}

// 定期保存状态到Redis
func (s *OptimizedMatchServer) periodicStateSaver() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒保存一次
	defer ticker.Stop()

	for range ticker.C {
		s.saveState()
	}
}

// 保存状态到Redis
func (s *OptimizedMatchServer) saveState() {
	// TODO: 实现状态保存逻辑
	// 可以将当前匹配队列保存到Redis
}

// 从 Redis 恢复状态
func (s *OptimizedMatchServer) restoreState() {
	// TODO: 实现状态恢复逻辑
	// 可以从 Redis 恢复之前保存的匹配队列
}
