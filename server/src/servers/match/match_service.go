package main

import (
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
	redisClient  *redis.Client
}

func NewOptimizedMatchServer(redisAddr string) *OptimizedMatchServer {
	// 初始化Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // 如果有密码
		DB:       0,  // 默认DB
	})

	server := &OptimizedMatchServer{
		matchQueue:   make(map[uint64]*pb.MatchRequest),
		lastActivity: make(map[uint64]time.Time),
		redisClient:  redisClient,
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
