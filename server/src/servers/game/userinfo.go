package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"log/slog"
	pb "proto"
	"strconv"
)

//player 与room 之间的映射关系

func (p *Player) HandleGetUserInfoRequest(msg *pb.Message) {
	var req pb.GetUserInfoRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse GetUserInfoRequest Request", "error", err)
		return
	}

	//user:{uid} HashMap	{"name": "user_100001", "level": 1, "exp": 0}	永久	用户信息（JSON 格式），包括昵称、等级、经验等。

	//在redis中查找用户信息，如果没有，则创建一个新的用户
	//如果有，则返回用户信息
	if req.GetUid() == 0 || req.GetUid() != p.Uid {
		p.SendResponse(msg, mustMarshal(&pb.GetUserInfoResponse{
			Ret: pb.ErrorCode_AUTH_FAILED,
		}))
		return
	}

	// 在Redis中查找用户信息
	userKey := fmt.Sprintf("user:%d", req.GetUid())

	slog.Info("get user info request", "uid", p.Uid, "redis_key", userKey)

	// 尝试获取用户信息
	userInfo, err := GlobalRedis.HGetAll(userKey)
	if err != nil && err != redis.ErrNil {
		slog.Error("redis error", "error", err)
		p.SendResponse(msg, mustMarshal(&pb.AuthResponse{
			Ret: pb.ErrorCode_SERVER_ERROR,
		}))
		return
	}

	// 用户不存在则返回错误
	if len(userInfo) == 0 || userInfo["uid"] == "" {
		slog.Info("User not found or incomplete data, creating new user", "key", userKey)
		p.SendResponse(msg, mustMarshal(&pb.GetUserInfoResponse{
			Ret: pb.ErrorCode_AUTH_FAILED,
		}))
		return
	}

	p.SetUserInfoToPlayer(userInfo)

	slog.Info("get user info from redis", "user_info", userInfo)

	// 解析用户信息
	user := &pb.UserInfo{
		Uid:           p.Uid,
		Name:          p.Name,
		Exp:           p.Exp,
		Gold:          p.Gold,
		Diamond:       p.Diamond,
		DrawCardCount: p.DrawCardInfo.DrawCardCount,
		Backpack:      p.Backpack,
	}

	slog.Info("got user info detail", "user", user)

	// 返回新用户信息
	p.SendResponse(msg, mustMarshal(&pb.GetUserInfoResponse{
		Ret:      pb.ErrorCode_OK,
		UserInfo: user,
	}))
}

func (p *Player) BackpackToJsonString() string {
	if p.Backpack == nil {
		return "" // 允许空背包
	}

	// 使用protojson序列化（需导入google.golang.org/protobuf/encoding/protojson）
	jsonData, err := protojson.Marshal(p.Backpack)
	if err != nil {
		slog.Error("protojson序列化失败", "error", err)
		return ""
	}

	p.BackpackJSON = string(jsonData) // 缓存原始JSON

	return p.BackpackJSON
}

func (p *Player) ParseBackpackJSON(jsonStr string) error {
	if jsonStr == "" {
		return nil // 允许空背包
	}

	// 使用protojson解析（需导入google.golang.org/protobuf/encoding/protojson）
	backpack := &pb.BackpackInfo{}
	if err := protojson.Unmarshal([]byte(jsonStr), backpack); err != nil {
		return fmt.Errorf("protojson解析失败: %w", err)
	}

	p.Backpack = backpack
	p.BackpackJSON = jsonStr // 缓存原始JSON
	return nil
}

func (p *Player) SetUserInfoToPlayer(userInfo map[string]string) {
	var err error
	p.Name = userInfo["name"]

	p.Exp, err = strconv.ParseInt(userInfo["exp"], 10, 64)
	if err != nil {
		slog.Error("Failed to parse exp", "error", err)
		p.Exp = 0
	}

	p.Gold, err = strconv.ParseInt(userInfo["gold"], 10, 64)
	if err != nil {
		slog.Error("Failed to parse gold", "error", err)
		p.Gold = 0
	}

	p.Diamond, err = strconv.ParseInt(userInfo["diamond"], 10, 64)
	if err != nil {
		slog.Error("Failed to parse diamond", "error", err)
		p.Diamond = 0
	}

	if p.DrawCardInfo == nil {
		drawCount, _ := strconv.Atoi(userInfo["draw_card_count"])
		lastDrawTime, _ := strconv.ParseInt(userInfo["last_draw_card_time"], 10, 64)
		p.DrawCardInfo = &DrawCardInfo{
			DrawCardCount:    int32(drawCount),
			LastDrawCardTime: lastDrawTime,
		}
	}

	// 解析背包信息
	if err := p.ParseBackpackJSON(userInfo["bag"]); err != nil {
		slog.Error("Failed to parse backpack JSON", "error", err)
	}

	slog.Debug("Parsed user info", "name", p.Name, "exp", p.Exp, "gold", p.Gold, "diamond", p.Diamond, "draw_card_count", p.DrawCardInfo.DrawCardCount)
}
