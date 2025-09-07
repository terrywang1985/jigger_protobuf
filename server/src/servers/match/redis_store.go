package main

import (
	"common/db"
	"github.com/garyburd/redigo/redis"
	"google.golang.org/protobuf/encoding/protojson"
	"log/slog"
	pb "proto"
	"time"
)

// 生成全局唯一战斗ID
func GenerateBattleID() string {
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

// 存储匹配玩家数据
func StoreMatchPlayer(playerID uint64, data *pb.PlayerInitData) error {
	conn := db.Pool.Get()
	defer conn.Close()

	key := "match:player:" + string(playerID)
	jsonData, err := protojson.Marshal(data)
	if err != nil {
		return err
	}

	_, err = conn.Do("SETEX", key, 300, jsonData) // 5分钟过期
	return err
}

// 获取匹配玩家数据
func GetMatchPlayer(playerID uint64) (*pb.PlayerInitData, error) {
	conn := db.Pool.Get()
	defer conn.Close()

	key := "match:player:" + string(playerID)
	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return nil, err
	}

	var playerData pb.PlayerInitData
	if err := protojson.Unmarshal(data, &playerData); err != nil {
		return nil, err
	}

	return &playerData, nil
}

// 删除匹配玩家数据
func DeleteMatchPlayer(playerID uint64) error {
	conn := Pool.Get()
	defer conn.Close()

	key := "match:player:" + string(playerID)
	_, err := conn.Do("DEL", key)
	return err
}
