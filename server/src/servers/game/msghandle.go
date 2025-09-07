package main

import (
	"log/slog"
	pb "proto"
)

// 消息管理器
type MessageManager struct {
	player_handlers map[pb.MessageId]func(player *Player, msg *pb.Message)
	//room_handlers   map[pb.MessageId]func(room *Room, roomMsg *RoomMessage)
}

// 初始化消息管理器
func NewMessageManager() *MessageManager {
	return &MessageManager{
		player_handlers: make(map[pb.MessageId]func(player *Player, msg *pb.Message)),
		//room_handlers:   make(map[pb.MessageId]func(room *Room, roomMsg *RoomMessage)),
	}
}

// 注册消息处理回调
func (m *MessageManager) RegisterHandler(msgId pb.MessageId, handler func(player *Player, msg *pb.Message)) {
	m.player_handlers[msgId] = handler
}

// 处理消息
func (m *MessageManager) HandleMessage(player *Player, msg *pb.Message) {
	if handler, ok := m.player_handlers[msg.GetId()]; ok {
		handler(player, msg)
	} else {
		slog.Info("Message not registered", "msgId", msg.GetId())
	}
}

// 全局消息管理器实例
var MsgHandler = NewMessageManager()

// 注册所有消息回调
func InitMessageHandlers() {
	MsgHandler.RegisterHandler(pb.MessageId_AUTH_REQUEST, (*Player).HandleAuthRequest)
	MsgHandler.RegisterHandler(pb.MessageId_DRAW_CARD_REQUEST, (*Player).HandleDrawCardRequest)
	MsgHandler.RegisterHandler(pb.MessageId_GET_USER_INFO_REQUEST, (*Player).HandleGetUserInfoRequest)

	MsgHandler.RegisterHandler(pb.MessageId_CREATE_ROOM_REQUEST, (*Player).HandleCreateRoomRequest)
	MsgHandler.RegisterHandler(pb.MessageId_JOIN_ROOM_REQUEST, (*Player).HandleJoinRoomRequest)
	MsgHandler.RegisterHandler(pb.MessageId_LEAVE_ROOM_REQUEST, (*Player).HandleLeaveRoomRequest)
	MsgHandler.RegisterHandler(pb.MessageId_GET_READY_REQUEST, (*Player).HandleGetReadyRequest)

	MsgHandler.RegisterHandler(pb.MessageId_GAME_ACTION_REQUEST, (*Player).HandlePlayerActionRequest)

}

func init() {
	InitMessageHandlers()
}
