package main

import (
	"log/slog"
	"sync"
	"time"

	pb "proto"
)

// 后台匹配处理器
func (s *OptimizedMatchServer) backgroundMatcher() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		s.tryMatch()
	}
}

// 尝试匹配玩家
func (s *OptimizedMatchServer) tryMatch() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查超时（5分钟）
	now := time.Now()
	for playerID, lastTime := range s.lastActivity {
		if now.Sub(lastTime) > 5*time.Minute {
			slog.Info("Player removed due to timeout", "player_id", playerID)
			delete(s.matchQueue, playerID)
			delete(s.lastActivity, playerID)
			deleteMatchPlayer(playerID)
		}
	}

	// 简单匹配：凑够2人就开房
	var matchedPlayers []*pb.MatchRequest
	for playerID, req := range s.matchQueue {
		matchedPlayers = append(matchedPlayers, req)
		delete(s.matchQueue, playerID)
		delete(s.lastActivity, playerID)
		deleteMatchPlayer(playerID)

		// 凑够2人
		if len(matchedPlayers) == 2 {
			slog.Info("Matched 2 players, creating battle room")
			go s.createBattleRoom(matchedPlayers)
			matchedPlayers = nil // 清空，继续匹配
		}
	}
}

// 创建战斗房间
func (s *OptimizedMatchServer) createBattleRoom(players []*pb.MatchRequest) {
	battleID := generateBattleID()
	slog.Info("Creating battle room", "battle_id", battleID, "players", len(players))

	// 构建创建房间请求
	roomReq := &pb.CreateRoomRequest{
		BattleId: battleID,
		Players:  make([]*pb.PlayerInitData, 0, len(players)),
	}

	for _, playerReq := range players {
		roomReq.Players = append(roomReq.Players, playerReq.PlayerData)
	}

	// 设置战场
	roomReq.Battlefield = &pb.Battlefield{
		Width:       1000,
		Height:      1000,
		Player1Base: &pb.Position{X: 100, Y: 500},
		Player2Base: &pb.Position{X: 900, Y: 500},
		BaseRadius:  50,
	}

	// 在实际应用中，这里会调用BattleCommandService
	slog.Info("Battle room created", "battle_id", battleID, "players", len(players))

	// 通知玩家匹配成功
	for _, playerReq := range players {
		s.notifyPlayerMatched(playerReq.PlayerId, battleID)
	}
}

// 通知玩家匹配成功
func (s *OptimizedMatchServer) notifyPlayerMatched(playerID uint64, battleID string) {
	slog.Info("Notifying player matched", "player_id", playerID, "battle_id", battleID)
	// 在实际应用中，这里会调用GameServer通知玩家
}