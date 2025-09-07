package main

import (
	"common/rpc"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log/slog"
	"math/rand"
	pb "proto"
	"sync"
	"time"
)

type BattleRoom struct {
	BattleID     string
	Server       *BattleServer
	Game         Game
	GameType     GameType
	ReadyPlayers map[uint64]bool
	CmdChan      chan Command
	Players      map[uint64]*PlayerInfo
	PlayersMutex sync.RWMutex
}

type PlayerInfo struct {
	PlayerID uint64
	Name     string
}

type Command struct {
	PlayerID uint64
	Action   *pb.GameAction
}

func NewBattleRoom(battleID string, server *BattleServer, gameType GameType) *BattleRoom {
	rand.Seed(time.Now().UnixNano())

	room := &BattleRoom{
		BattleID:     battleID,
		Server:       server,
		GameType:     gameType,
		ReadyPlayers: make(map[uint64]bool),
		CmdChan:      make(chan Command, 100),
		Players:      make(map[uint64]*PlayerInfo),
	}

	// 创建游戏实例
	room.Game = GameFactory(gameType)
	return room
}

func (room *BattleRoom) AddPlayer(playerID uint64, name string) {
	room.PlayersMutex.Lock()
	defer room.PlayersMutex.Unlock()

	room.Players[playerID] = &PlayerInfo{
		PlayerID: playerID,
		Name:     name,
	}

	//

}

func (room *BattleRoom) SetPlayerReady(playerID uint64) {
	room.PlayersMutex.Lock()
	defer room.PlayersMutex.Unlock()
	room.ReadyPlayers[playerID] = true
}

func (room *BattleRoom) AllPlayersReady() bool {
	room.PlayersMutex.RLock()
	defer room.PlayersMutex.RUnlock()
	return len(room.ReadyPlayers) == len(room.Players)
}

func (room *BattleRoom) StartGame() {
	// 将房间玩家转换为游戏玩家
	var players []*Player
	for id, info := range room.Players {
		players = append(players, &Player{
			ID:   id,
			Name: info.Name,
		})
	}

	// 初始化并开始游戏
	room.Game.Init(players)
	room.Game.Start()

	// 广播游戏状态
	room.BroadcastGameState()
}

func (room *BattleRoom) BroadcastGameState() {
	state := room.Game.GetState()
	// 实际项目中这里需要通过RPC通知所有玩家
	slog.Info("Broadcasting game state", "state", state)

}

func (room *BattleRoom) Run() {
	go func() {
		gameTicker := time.NewTicker(100 * time.Millisecond)
		defer gameTicker.Stop()

		for {
			select {
			case cmd := <-room.CmdChan:
				room.HandlePlayerCommand(cmd)
			case <-gameTicker.C:
				if room.Game != nil && room.Game.IsGameOver() {
					room.Game.EndGame()
					room.EndGame()
					return
				}
			}
		}
	}()
}

func (room *BattleRoom) HandlePlayerCommand(cmd Command) {
	if room.Game == nil {
		return
	}

	success := room.Game.HandleAction(cmd.PlayerID, cmd.Action)
	if success {
		room.BroadcastGameState()
	}
}

func (room *BattleRoom) EndGame() {
	slog.Info("Game ended", "room", room.BattleID)

	// 清理房间
	room.Server.RoomsMutex.Lock()
	delete(room.Server.BattleRooms, room.BattleID)
	room.Server.RoomsMutex.Unlock()
}

func (room *BattleRoom) BroadcastRoomStatus() {
	// 通知房间内所有玩家
	for playerID := range room.Players {
		room.NotifyRoomStatus(playerID, &pb.RoomDetail{
			Room: &pb.Room{
				Id:   room.BattleID,
				Name: "Battle Room",
			},
			CurrentPlayers: room.GetPlayerList(),
		})
	}
}

func (room *BattleRoom) BroadcastNotifyGameState() {
	state := room.Game.GetState()

	// 通知房间内所有玩家
	for playerID := range room.Players {
		room.NotifyGameState(playerID, &pb.GameStateNotify{
			RoomId:    room.BattleID,
			GameState: state,
		})
	}
}

func (room *BattleRoom) NotifyRoomStatus(playerID uint64, msg *pb.RoomDetail) {
	gameServerAddr := "127.0.0.1:" + fmt.Sprintf(":%d", rpc.GameServiceGRPCPort)
	conn, err := grpc.Dial(gameServerAddr, grpc.WithInsecure())
	if err != nil {
		slog.Error("Failed to connect to game server", "addr", gameServerAddr, "error", err)
		return
	}
	defer conn.Close()

	client := pb.NewGameRpcServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	notify := &pb.RoomDetailNotify{
		BeNotifiedUid: playerID,
		Room:          msg,
	}

	_, err = client.RoomStatusNotifyRpc(ctx, notify)
	if err != nil {
		slog.Error("Failed to send notification", "player_id", playerID, "error", err)
	}
}

func (room *BattleRoom) NotifyGameState(playerId uint64, msg *pb.GameStateNotify) {
	// 获取玩家所在GameServer
	//gameServerAddr, err := room.Server.GetPlayerGameServer(playerId)
	//if err != nil {
	//	slog.Error("Failed to get player's game server", "player_id", playerId, "error", err)
	//	return
	//}

	gameServerAddr := "127.0.0.1:" + fmt.Sprintf(":%d", rpc.GameServiceGRPCPort)
	// 连接GameServer
	conn, err := grpc.Dial(gameServerAddr, grpc.WithInsecure())
	if err != nil {
		slog.Error("Failed to connect to game server", "addr", gameServerAddr, "error", err)
		return
	}
	defer conn.Close()

	client := pb.NewGameRpcServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = client.GameStateNotifyRpc(ctx, msg)
	if err != nil {
		slog.Error("Failed to send game state notification", "player_id", playerId, "error", err)
	}
}

func (room *BattleRoom) GetPlayerList() []*pb.RoomPlayer {
	var players []*pb.RoomPlayer
	for id, info := range room.Players {
		players = append(players, &pb.RoomPlayer{
			Uid:  id,
			Name: info.Name,
		})
	}
	return players
}
