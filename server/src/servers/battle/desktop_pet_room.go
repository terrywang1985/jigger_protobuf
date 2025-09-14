package main

import (
	"log/slog"
	"sync"
	"time"

	pb "proto"
)

// DesktopPetRoom 桌面宠物房间
type DesktopPetRoom struct {
	RoomID      string
	Players     map[uint64]*DesktopPetPlayer
	PlayersMutex sync.RWMutex
	CmdChan     chan DesktopPetCommand
	IsRunning   bool
	Server      *BattleServer
}

// DesktopPetPlayer 桌面宠物玩家
type DesktopPetPlayer struct {
	PlayerID   uint64
	PlayerName string
	PetSkin    string
	PositionX  float32
	PositionY  float32
	Action     string
	ChatText   string
	LastActive time.Time
}

// DesktopPetCommand 桌面宠物命令
type DesktopPetCommand struct {
	PlayerID uint64
	Type     string // "move", "action", "chat"
	Data     interface{}
}

// NewDesktopPetRoom 创建新的桌面宠物房间
func NewDesktopPetRoom(roomID string, server *BattleServer) *DesktopPetRoom {
	return &DesktopPetRoom{
		RoomID:    roomID,
		Players:   make(map[uint64]*DesktopPetPlayer),
		CmdChan:   make(chan DesktopPetCommand, 100),
		IsRunning: false,
		Server:    server,
	}
}

// AddPlayer 添加玩家到房间
func (r *DesktopPetRoom) AddPlayer(playerID uint64, playerName string) {
	r.PlayersMutex.Lock()
	defer r.PlayersMutex.Unlock()

	r.Players[playerID] = &DesktopPetPlayer{
		PlayerID:   playerID,
		PlayerName: playerName,
		PositionX:  0.5, // 默认位置
		PositionY:  0.5,
		LastActive: time.Now(),
	}

	slog.Info("Player added to desktop pet room", "room_id", r.RoomID, "player_id", playerID)
}

// RemovePlayer 从房间移除玩家
func (r *DesktopPetRoom) RemovePlayer(playerID uint64) {
	r.PlayersMutex.Lock()
	defer r.PlayersMutex.Unlock()

	delete(r.Players, playerID)
	slog.Info("Player removed from desktop pet room", "room_id", r.RoomID, "player_id", playerID)
}

// Run 运行房间逻辑
func (r *DesktopPetRoom) Run() {
	r.IsRunning = true
	ticker := time.NewTicker(100 * time.Millisecond) // 10fps

	go func() {
		for r.IsRunning {
			select {
			case cmd := <-r.CmdChan:
				r.handleCommand(cmd)
			case <-ticker.C:
				r.broadcastRoomState()
			}
		}
	}()

	slog.Info("Desktop pet room started", "room_id", r.RoomID)
}

// Stop 停止房间
func (r *DesktopPetRoom) Stop() {
	r.IsRunning = false
	slog.Info("Desktop pet room stopped", "room_id", r.RoomID)
}

// HandleCommand 处理玩家命令
func (r *DesktopPetRoom) handleCommand(cmd DesktopPetCommand) {
	r.PlayersMutex.Lock()
	defer r.PlayersMutex.Unlock()

	player, exists := r.Players[cmd.PlayerID]
	if !exists {
		return
	}

	player.LastActive = time.Now()

	switch cmd.Type {
	case "move":
		if data, ok := cmd.Data.(map[string]float32); ok {
			if x, exists := data["x"]; exists {
				player.PositionX = x
			}
			if y, exists := data["y"]; exists {
				player.PositionY = y
			}
		}
	case "action":
		if action, ok := cmd.Data.(string); ok {
			player.Action = action
			// 动作持续一段时间后自动清除
			go func() {
				time.Sleep(1 * time.Second)
				r.PlayersMutex.Lock()
				if p, exists := r.Players[cmd.PlayerID]; exists {
					p.Action = ""
				}
				r.PlayersMutex.Unlock()
			}()
		}
	case "chat":
		if chat, ok := cmd.Data.(string); ok {
			player.ChatText = chat
			// 聊天消息持续一段时间后自动清除
			go func() {
				time.Sleep(3 * time.Second)
				r.PlayersMutex.Lock()
				if p, exists := r.Players[cmd.PlayerID]; exists {
					p.ChatText = ""
				}
				r.PlayersMutex.Unlock()
			}()
		}
	}
}

// BroadcastRoomState 广播房间状态
func (r *DesktopPetRoom) broadcastRoomState() {
	r.PlayersMutex.RLock()
	defer r.PlayersMutex.RUnlock()

	// 构建房间状态
	roomState := &pb.DesktopPetRoomState{
		RoomId:  r.RoomID,
		Players: make([]*pb.DesktopPetPlayerState, 0, len(r.Players)),
	}

	for _, player := range r.Players {
		roomState.Players = append(roomState.Players, &pb.DesktopPetPlayerState{
			PlayerId:   player.PlayerID,
			PlayerName: player.PlayerName,
			PositionX:  player.PositionX,
			PositionY:  player.PositionY,
			Action:     player.Action,
			ChatText:   player.ChatText,
			PetSkin:    player.PetSkin,
		})
	}

	// 广播给所有玩家
	for playerID := range r.Players {
		r.notifyPlayer(playerID, roomState)
	}
}

// NotifyPlayer 通知单个玩家
func (r *DesktopPetRoom) notifyPlayer(playerID uint64, state *pb.DesktopPetRoomState) {
	// 这里需要实现通过RPC通知玩家的逻辑
	// 实际实现会根据您的RPC框架有所不同
	slog.Debug("Notifying player", "player_id", playerID, "room_id", r.RoomID)
}