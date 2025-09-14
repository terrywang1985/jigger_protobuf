package main

import (
	"common/redisutil"
	"log/slog"
	"strconv"
	"time"

	pb "proto"
)

// 全局Redis池
var globalRedisPool *redisutil.RedisPool

// 初始化全局Redis池
func initGlobalRedisPool(addr string) {
	globalRedisPool = redisutil.NewRedisPool(addr, "", 0)
}

// 生成全局唯一战斗ID
func GenerateBattleID() string {
	battleID, err := globalRedisPool.GenerateBattleID()
	if err != nil {
		slog.Error("Failed to generate battle ID", "error", err)
		return ""
	}
	return battleID
}

// 存储匹配玩家数据
func StoreMatchPlayer(playerID uint64, data *pb.PlayerInitData) error {
	key := "match:player:" + strconv.FormatUint(playerID, 10)
	return globalRedisPool.SetProto(key, data, 5*time.Minute) // 5分钟过期
}

// 获取匹配玩家数据
func GetMatchPlayer(playerID uint64) (*pb.PlayerInitData, error) {
	key := "match:player:" + strconv.FormatUint(playerID, 10)
	var playerData pb.PlayerInitData
	err := globalRedisPool.GetProto(key, &playerData)
	if err != nil {
		return nil, err
	}
	return &playerData, nil
}

// 删除匹配玩家数据
func DeleteMatchPlayer(playerID uint64) error {
	key := "match:player:" + strconv.FormatUint(playerID, 10)
	return globalRedisPool.Delete(key)
}
