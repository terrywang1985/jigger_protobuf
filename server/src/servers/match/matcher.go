package main

import (
	"context"
	"log/slog"
	pb "proto"
	"time"
)

func (s *OptimizedMatchServer) backgroundMatcher() {
	ticker := time.NewTicker(100 * time.Millisecond) // 每100毫秒匹配一次
	defer ticker.Stop()

	for range ticker.C {
		s.tryMatch()
	}
}

func (s *OptimizedMatchServer) tryMatch() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查超时（5分钟）
	now := time.Now()
	for playerID, lastTime := range s.lastActivity {
		if now.Sub(lastTime) > 5*time.Minute {
			delete(s.matchQueue, playerID)
			delete(s.lastActivity, playerID)
		}
	}

	// 简单匹配：凑够2人就开房
	var matchedPlayers []*pb.MatchRequest
	for playerID, req := range s.matchQueue {
		matchedPlayers = append(matchedPlayers, req)
		delete(s.matchQueue, playerID)
		delete(s.lastActivity, playerID)

		// 凑够2人
		if len(matchedPlayers) == 2 {
			// 创建房间
			go s.createBattleRoom(matchedPlayers)
			matchedPlayers = nil // 清空，继续匹配
		}
	}

	// 如果匹配后还有剩余玩家，放回队列（但上面是循环整个队列，所以这里不需要放回）
}

func (s *OptimizedMatchServer) createBattleRoom(players []*pb.MatchRequest) {
	// 调用BattleCommandService的CreateRoom
	// 这里需要实现gRPC调用

	// 假设我们有一个battleCommandClient
	// 创建房间请求
	roomReq := &pb.CreateRoomRequest{
		BattleId: generateBattleID(), // 生成唯一战斗ID
		Players:  make([]*pb.PlayerInitData, 0, len(players)),
	}

	for _, playerReq := range players {
		roomReq.Players = append(roomReq.Players, playerReq.PlayerData)
	}

	// 设置战场（这里简化，实际应该从配置中获取）
	roomReq.Battlefield = &pb.Battlefield{
		Width:       1000,
		Height:      1000,
		Player1Base: &pb.Position{X: 100, Y: 500},
		Player2Base: &pb.Position{X: 900, Y: 500},
		BaseRadius:  50,
	}

	// 调用BattleCommandService
	// 注意：这里需要处理错误和重试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := battleCommandClient.CreateRoom(ctx, roomReq)
	if err != nil {
		slog.Error("failed to create battle room", "error", err)
		// 如果创建房间失败，需要将玩家重新放回匹配队列？
		// 这里可以根据业务逻辑处理，比如重试或者通知玩家匹配失败
		return
	}

	if resp.Ret != pb.ErrorCode_OK {
		slog.Error("create room failed", "error", resp.Ret)
		return
	}

	// 通知玩家匹配成功（这里通过GameServer转发）
	for _, playerReq := range players {
		s.notifyPlayerMatched(playerReq.PlayerId, roomReq.BattleId)
	}
}

func (s *OptimizedMatchServer) notifyPlayerMatched(playerID uint64, battleID string) {
	// 这里需要将匹配结果通知到GameServer，由GameServer通知玩家
	// 我们可以通过gRPC调用GameServer的接口，或者通过消息队列
	// 由于您现有的架构中，GameServer和MatchServer都是gRPC服务，这里直接调用GameServer的接口

	// 假设我们有一个gameServerClient
	notifyReq := &pb.MatchResultNotify{
		PlayerId:   playerID,
		BattleId:   battleID,
		ServerAddr: "battle-server-address:50051", // 战斗服务器地址
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := gameServerClient.OnMatchSuccess(ctx, notifyReq)
	if err != nil {
		slog.Error("failed to notify player", "player_id", playerID, "error", err)
	}
}

func generateBattleID() string {
	// 生成唯一战斗ID，可以用UUID或者时间戳+随机数
	return "battle-" + time.Now().Format("20060102150405") + "-" + randStr(6)
}
