package main

import (
	"log/slog"
	"sync"
)

type Manager struct {
	players    sync.Map // 使用 sync.Map 来存储玩家  conn_uuid_player
	uin_player sync.Map // 使用 sync.Map 来存储玩家 uin_player
}

// 创建玩家, connUUID 类似于 166034fe2377ce4f5b4a8c09d7dfbc6f  这样的字符串，标记连接, 后续等待玩家登陆绑定uid
func (rm *Manager) GetOrCreatePlayer(connUUID string, conn Connection) *Player {
	player, loaded := rm.players.LoadOrStore(connUUID, NewPlayer(connUUID, conn)) // 从 sync.Map 获取玩家，如果不存在则创建

	// 如果玩家是新创建的，则启动其协程
	if !loaded {
		slog.Info("Player created", "conn_uuid", connUUID)
		go player.(*Player).Run() // 启动玩家逻辑协程
	} else {
		slog.Info("Player already exists", "conn_uuid", connUUID)
	}

	return player.(*Player)
}

func (m *Manager) OnPlayerUinSet(connUUID string) {
	player, ok := m.players.Load(connUUID)
	if !ok {
		slog.Info("Player connection uuid not found", "conn_uuid", connUUID)
		return
	}

	// 将玩家的 Uid 存储到 uin_player 中
	m.uin_player.Store(player.(*Player).Uid, player)
}

// 获取玩家
func (rm *Manager) GetPlayer(id string) (*Player, bool) {
	player, ok := rm.players.Load(id) // 从 sync.Map 获取玩家
	if !ok {
		slog.Info("Player not found", "conn_uuid", id)
		return nil, false
	}
	return player.(*Player), true
}

// 获取玩家
func (rm *Manager) GetPlayerByUin(uin uint64) (*Player, bool) {
	player, ok := rm.uin_player.Load(uin) // 从 sync.Map 获取玩家
	if !ok {
		slog.Info("Player not found", "uin", uin)
		return nil, false
	}
	return player.(*Player), true
}

// 删除玩家
func (rm *Manager) DeletePlayer(id string) {
	if player, ok := rm.GetPlayer(id); ok {
		rm.players.Delete(id) // 删除玩家
		rm.uin_player.Delete(player.Uid)
		slog.Info("Player deleted", "conn_uuid", id)
	}
}

// 获取所有玩家
func (rm *Manager) GetAllPlayers() []*Player {
	var players []*Player
	rm.players.Range(func(key, value interface{}) bool {
		players = append(players, value.(*Player)) // 将所有玩家收集到数组中
		return true                                // 继续遍历
	})
	return players
}

// 全局管理器实例
var GlobalManager = &Manager{}

func init() {
	// 在程序启动时，如果需要做初始化工作，可以在这里进行
	slog.Info("Manager initialized")
}
