package main

import (
	"encoding/json"
	"log/slog"
	"time"

	"common/db"
	pb "proto"
	"google.golang.org/protobuf/proto"
)

// 服务器状态结构
type ServerState struct {
	MatchQueue   map[uint64]*pb.MatchRequest
	LastActivity map[uint64]time.Time
}

// 保存服务器状态到Redis
func saveServerState(state ServerState) error {
	conn := db.Pool.Get()
	defer conn.Close()

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = conn.Do("SETEX", "match:server:state", 900, data) // 15分钟过期
	return err
}

// 从Redis恢复服务器状态
func restoreServerState() (ServerState, error) {
	conn := db.Pool.Get()
	defer conn.Close()

	data, err := redis.Bytes(conn.Do("GET", "match:server:state"))
	if err != nil {
		return ServerState{}, err
	}

	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return ServerState{}, err
	}

	return state, nil
}

// 存储匹配玩家数据
func storeMatchPlayer(playerID uint64, data *pb.PlayerInitData) error {
	conn := db.Pool.Get()
	defer conn.Close()

	key := "match:player:" + string(rune(playerID))

	protoData, err := proto.Marshal(data)
	if err != nil {
		return err
	}

	_, err = conn.Do("SETEX", key, 300, protoData) // 5分钟过期
	return err
}

// 获取匹配玩家数据
func getMatchPlayer(playerID uint64) (*pb.PlayerInitData, error) {
	conn := db.Pool.Get()
	defer conn.Close()

	key := "match:player:" + string(rune(playerID))

	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return nil, err
	}

	var playerData pb.PlayerInitData
	if err := proto.Unmarshal(data, &playerData); err != nil {
		return nil, err
	}

	return &playerData, nil
}

// 删除匹配玩家数据
func deleteMatchPlayer(playerID uint64) error {
	conn := db.Pool.Get()
	defer conn.Close()

	key := "match:player:" + string(rune(playerID))
	_, err := conn.Do("DEL", key)
	return err
}

// 生成唯一战斗ID
func generateBattleID() string {
	conn := db.Pool.Get()
	defer conn.Close()

	// 使用日期前缀 + 序列号
	today := time.Now().Format("20060102")
	key := "global:battle_id:" + today

	// 初始化或递增序列号
	_, err := conn.Do("SETNX", key, 0)
	if err != nil {
		slog.Error("Failed to initialize battle ID sequence", "error", err)
		return ""
	}

	id, err := redis.Int64(conn.Do("INCR", key))
	if err != nil {
		slog.Error("Failed to generate battle ID", "error", err)
		return ""
	}

	return today + "-" + string(id)
}