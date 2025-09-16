package main

import (
	"common/redisutil"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	pb "proto"

	"google.golang.org/protobuf/proto"
)

// DrawCardInfo 抽卡信息
type DrawCardInfo struct {
	DrawCardCount    int32
	LastDrawCardTime int64
}

// Player 玩家结构体
type Player struct {
	// 连接信息
	ConnUUID string // 连接唯一标识
	Uid      uint64 // 玩家唯一标识
	OpenId   string // 平台唯一标识
	//Conn       net.Conn
	Conn       Connection       //同时支持tcp和websocket
	RecvChan   chan *pb.Message // 玩家收消息管道
	SendChan   chan *pb.Message // 玩家发消息管道
	NotiChan   chan *pb.Message // 给玩家发送通知的管道
	ctx        context.Context
	cancelFunc context.CancelFunc

	// 认证相关字段
	SessionID     string    // LoginServer 返回的 session_id
	SessionExpiry time.Time // session 过期时间
	Authenticated bool      // 是否已认证

	// 基础信息
	Name    string
	Exp     int64
	Gold    int64
	Diamond int64

	// 抽卡信息
	DrawCardInfo *DrawCardInfo

	// 背包信息
	Backpack      *pb.BackpackInfo
	BackpackJSON  string
	LastSavedTime int64 // 最后保存时间戳
}

// SessionData 会话数据结构（与登录服务器一致）
type SessionData struct {
	UserID    uint64 `json:"user_id"`
	OpenID    string `json:"openid"`
	Username  string `json:"username"`
	LoginTime int64  `json:"login_time"`
	ExpiresAt int64  `json:"expires_at"`
	AppID     string `json:"app_id"`
}

// UserData 用户数据
type UserData struct {
	exp      int64
	gold     int64
	diamond  int64
	nickname string
}

// 全局 Redis 连接池

// NewPlayer 创建玩家
// func NewPlayer(connUUID string, conn net.Conn) *Player {
func NewPlayer(connUUID string, conn Connection) *Player {
	ctx, cancel := context.WithCancel(context.Background())
	return &Player{
		ConnUUID:     connUUID,
		Conn:         conn,
		RecvChan:     make(chan *pb.Message, 1000),
		SendChan:     make(chan *pb.Message, 1000),
		NotiChan:     make(chan *pb.Message, 1000),
		ctx:          ctx,
		cancelFunc:   cancel,
		Backpack:     &pb.BackpackInfo{},
		DrawCardInfo: &DrawCardInfo{},
	}
}

// GetVIPLevel 获取VIP等级
func (p *Player) GetVIPLevel() int {
	return 1
}

// GetDrawCount 获取抽卡次数
func (p *Player) GetDrawCount() int32 {
	if p.DrawCardInfo == nil {
		return 0
	}
	return p.DrawCardInfo.DrawCardCount
}

// GetPlayerID 获取玩家ID
func (p *Player) GetPlayerID() uint64 {
	return p.Uid
}

// SetDrawCount 设置抽卡次数
func (p *Player) SetDrawCount(count int32) {
	if p.DrawCardInfo == nil {
		p.DrawCardInfo = &DrawCardInfo{}
	}
	p.DrawCardInfo.DrawCardCount = count
}

// SaveDrawCardResults 保存抽卡结果
func (p *Player) SaveDrawCardResults(cards []*pb.Card) {
	drawCardCount := len(cards)
	p.Backpack.Cards = append(p.Backpack.Cards, cards...)
	p.BackpackToJsonString()

	// 使用 HMSet 方法
	fields := map[string]interface{}{
		"draw_card_count":     p.DrawCardInfo.DrawCardCount + int32(drawCardCount),
		"last_draw_card_time": time.Now().Unix(),
		"bag":                 p.BackpackJSON,
	}

	if err := GlobalRedis.HMSet(fmt.Sprintf("user:%d", p.Uid), fields); err != nil {
		slog.Error("Failed to save draw card results to Redis", "error", err)
		return
	}

	slog.Info("Saved draw card results to Redis", "uid", p.Uid, "cards", cards)
}

// Run 启动玩家逻辑协程
func (p *Player) Run() {
	var wg sync.WaitGroup
	wg.Add(3) // 有三个goroutine需要等待

	defer func() {
		// 清理玩家退出时的资源
		GlobalManager.DeletePlayer(p.ConnUUID) // 从全局管理器中移除玩家
		p.cancelFunc()                         // 取消上下文以停止所有goroutine
		close(p.RecvChan)
		//close(p.SendChan) //为了避免grpc 收到消息，拿到layer后的瞬间，这里关闭了发送管道，导致的panic ,这里就直接不关闭了，等待垃圾回收
		defer p.Conn.Close()

		slog.Info("Player exited", "conn_uuid", p.ConnUUID)
	}()

	// 处理接收消息的goroutine
	go func() {
		defer wg.Done()
		buffer := make([]byte, 0, 4096) // 初始缓冲区
		for {
			select {
			case <-p.ctx.Done():
				slog.Info("Context done, exiting read loop", "conn_uuid", p.ConnUUID)
				return
			default:
				slog.Info("Waiting to read from connection...", "conn_uuid", p.ConnUUID)

				// Temporary buffer
				tempBuf := make([]byte, 1024)
				n, err := p.Conn.Read(tempBuf)
				if err != nil {
					slog.Info("Connection closed", "conn_uuid", p.ConnUUID, "reason", err)
					p.cancelFunc() // 取消上下文以停止所有goroutine
					return
				}

				// 将读取的数据追加到缓冲区
				buffer = append(buffer, tempBuf[:n]...)

				// 处理缓冲区中的数据
				for {
					slog.Debug("Buffer content", "length", len(buffer), "buffer", buffer)
					// Check if there is enough data to read the packet length
					if len(buffer) < 4 {
						slog.Debug("Not enough data to read the packet length, breaking out of inner loop")
						break
					}

					// 读取包长度
					length := int(binary.LittleEndian.Uint32(buffer[:4]))

					// 检查是否有足够的数据读取完整包
					if len(buffer) < 4+length {
						break // 没有足够的数据用于完整包
					}

					// 读取完整包
					messageBuf := buffer[4 : 4+length]
					buffer = buffer[4+length:] // 移除已处理的数据

					var parsedMsg pb.Message
					if err := proto.Unmarshal(messageBuf, &parsedMsg); err != nil {
						slog.Error("Failed to unmarshal message", "error", err)
						continue
					}

					// 尝试将消息发送到RecvChan
					select {
					case p.RecvChan <- &parsedMsg:
						// 成功入队
					default:
						// 如果通道已满，则丢弃消息
						slog.Error("RecvChan full, dropping message", "message", parsedMsg)
					}
				}
			}
		}
	}()

	// 处理发送消息的协程
	go func() {
		defer wg.Done()
		for {
			select {
			case rspMsg := <-p.SendChan:
				data, err := proto.Marshal(rspMsg)
				if err != nil {
					slog.Error("Failed to marshal response", "error", err)
					continue
				}

				length := make([]byte, 4)
				binary.LittleEndian.PutUint32(length, uint32(len(data)))

				slog.Info("In Send chan coroutine, Sending response", "messageLength", len(data), "message", rspMsg)
				packet := append(length, data...)
				if _, err := p.Conn.Write(packet); err != nil {
					p.cancelFunc() // 取消上下文以停止所有goroutine
					slog.Error("Failed to write response", "error", err)
					return
				}
			case <-p.ctx.Done():
				slog.Info("Context done, exiting send loop", "conn_uuid", p.ConnUUID)
				return
			}
		}
	}()

	// 处理从RecvChan接收消息的goroutine
	go func() {
		defer wg.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case msg := <-p.RecvChan:
				slog.Info("Received message", "message", msg)
				if msg.GetId() != pb.MessageId_AUTH_REQUEST && !p.Authenticated {
					slog.Warn("Unauthenticated player attempted to send message",
						"conn_uuid", p.ConnUUID, "msg_id", msg.GetId())
					return
				}
				MsgHandler.HandleMessage(p, msg)
			}
		}
	}()

	wg.Wait() // 等待所有goroutine完成
}

// Done 返回一个通道，当玩家退出时会关闭该通道
func (p *Player) Done() <-chan struct{} {
	return p.ctx.Done()
}

// SendMessage 发送消息
func (p *Player) SendMessage(msg *pb.Message) {
	p.SendChan <- msg
}

// SendResponse 发送响应
func (p *Player) SendResponse(srcMsg *pb.Message, responseData []byte) {
	// 响应
	response := &pb.Message{
		Id:          srcMsg.GetId() + 1,      // 响应ID是请求ID + 1
		MsgSerialNo: srcMsg.GetMsgSerialNo(), // 使用相同的消息序列号
		ClientId:    srcMsg.GetClientId(),    // 使用相同的客户端ID
		Data:        responseData,
	}

	p.SendMessage(response)
}

// HandleAuthRequest 处理认证请求（统一流程：游客和正常用户都有token）
func (p *Player) HandleAuthRequest(msg *pb.Message) {
	var req pb.AuthRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse Auth Request", "error", err)
		p.sendAuthErrorResponse(msg, pb.ErrorCode_INVALID_PARAM, "Invalid request format")
		return
	}

	// 统一验证token（游客和正常用户都必须有token）
	if req.GetToken() == "" {
		p.sendAuthErrorResponse(msg, pb.ErrorCode_INVALID_PARAM, "Token is required")
		return
	}

	// 验证session/token
	isValid, sessionData, err := validateSession(req.GetToken())
	if err != nil {
		slog.Error("Session validation error", "error", err)
		p.sendAuthErrorResponse(msg, pb.ErrorCode_SERVER_ERROR, "Internal server error")
		return
	}

	if !isValid {
		slog.Info("Invalid session/token", "token", req.GetToken())
		p.sendAuthErrorResponse(msg, pb.ErrorCode_AUTH_FAILED, "Invalid token or session expired")
		return
	}

	// 根据is_guest字段或者session中的openid判断是否为游客
	isGuest := req.GetIsGuest() || (len(sessionData.OpenID) >= 6 && sessionData.OpenID[:6] == "guest_")

	// 统一处理流程：使用 openid 查找或创建游戏内用户数据
	var gameUserData *UserData
	var gameUid uint64
	// 统一使用一个函数处理所有用户类型（游客和正常用户）
	gameUserData, gameUid, err = findOrCreateUserByOpenID(sessionData.OpenID, sessionData.Username)

	if err != nil {
		p.sendAuthErrorResponse(msg, pb.ErrorCode_SERVER_ERROR, "Failed to load user data")
		return
	}

	// 设置玩家信息
	p.Uid = gameUid
	p.SessionID = req.GetToken()
	p.Authenticated = true
	p.Name = gameUserData.nickname
	p.OpenId = sessionData.OpenID

	// 从Redis加载用户完整信息
	userData, err := loadUserDataFromRedis(p.Uid)
	if err != nil {
		p.sendAuthErrorResponse(msg, pb.ErrorCode_SERVER_ERROR, "Failed to load user data")
		return
	}

	// 设置玩家属性
	p.Exp = userData.exp
	p.Gold = userData.gold
	p.Diamond = userData.diamond

	// 加入到manager里面
	GlobalManager.OnPlayerUinSet(p.ConnUUID)

	slog.Info("User authenticated", "uid", gameUid, "openid", sessionData.OpenID, "is_guest", isGuest)

	// 返回认证成功响应
	p.sendAuthSuccessResponse(msg, userData, isGuest)
}

// 辅助函数：发送认证错误响应
func (p *Player) sendAuthErrorResponse(srcMsg *pb.Message, errorCode pb.ErrorCode, errorMsg string) {
	response := &pb.AuthResponse{
		Ret:      errorCode,
		Uid:      p.Uid,
		ErrorMsg: errorMsg,
	}
	p.SendResponse(srcMsg, mustMarshal(response))
}

// 辅助函数：发送认证成功响应
func (p *Player) sendAuthSuccessResponse(srcMsg *pb.Message, userData *UserData, isGuest bool) {
	response := &pb.AuthResponse{
		Ret:           pb.ErrorCode_OK,
		Uid:           p.Uid,
		ConnId:        p.ConnUUID,
		ServerTime:    time.Now().Format(time.RFC3339),
		SessionExpiry: time.Now().Add(24 * time.Hour).Unix(),
		Nickname:      p.Name,
		Level:         calculateLevel(p.Exp),
		Exp:           p.Exp,
		Gold:          p.Gold,
		Diamond:       p.Diamond,
		IsGuest:       isGuest, // TODO: 等protobuf重新生成后开启
	}
	p.SendResponse(srcMsg, mustMarshal(response))
}

// 辅助函数：生成新的用户ID
func generateNewUserId() (uint64, error) {
	// 获取全局自增UID计数器
	uid, err := GlobalRedis.Incr("global:user_uid")
	if err != nil {
		return 0, err
	}
	return uint64(uid), nil
}

func mustMarshal(pb proto.Message) []byte {
	data, err := proto.Marshal(pb)
	if err != nil {
		slog.Error("Failed to marshal protobuf message", "error", err)
	}
	return data
}

// validateSession 验证session/token的有效性
func validateSession(token string) (bool, *SessionData, error) {
	// 从Redis中获取session信息
	var sessionData SessionData
	err := GlobalRedis.GetJSON("session:"+token, &sessionData)
	if err != nil {
		if err == redisutil.ErrKeyNotFound {
			return false, nil, nil
		}
		return false, nil, err
	}

	// 检查session是否过期
	if time.Now().Unix() > sessionData.ExpiresAt {
		return false, nil, nil
	}

	return true, &sessionData, nil
}

// loadUserDataFromRedis 从Redis加载用户数据
func loadUserDataFromRedis(uid uint64) (*UserData, error) {
	values, err := GlobalRedis.HGetAll(fmt.Sprintf("user:%d", uid))
	if err != nil {
		return nil, err
	}

	// 解析values
	exp, _ := strconv.ParseInt(values["exp"], 10, 64)
	gold, _ := strconv.ParseInt(values["gold"], 10, 64)
	diamond, _ := strconv.ParseInt(values["diamond"], 10, 64)
	name, _ := values["nickname"]

	return &UserData{
		exp:      exp,
		gold:     gold,
		diamond:  diamond,
		nickname: name,
	}, nil
}

func saveUserDataToRedis(uid uint64, userData *UserData) error {
	fields := map[string]interface{}{
		"exp":      userData.exp,
		"gold":     userData.gold,
		"diamond":  userData.diamond,
		"nickname": userData.nickname,
	}
	return GlobalRedis.HMSet(fmt.Sprintf("user:%d", uid), fields)
}

// calculateLevel 根据经验值计算等级
func calculateLevel(exp int64) int32 {
	// 简单的等级计算公式，可根据需要调整
	return int32(exp / 1000)
}

// findOrCreateUserByOpenID 根据openid查找或创建用户（统一函数，支持正常用户和游客）
func findOrCreateUserByOpenID(openid string, username string) (*UserData, uint64, error) {
	// 1. 尝试从Redis中根据openid查找是否已存在游戏用户
	existingUid, err := GlobalRedis.GetString("openid_to_uid:" + openid)
	if err == nil && existingUid != "" {
		// 用户已存在，直接返回uid和用户数据
		uid, _ := strconv.ParseUint(existingUid, 10, 64)
		userData, err := loadUserDataFromRedis(uid)
		return userData, uid, err
	}

	// 2. 用户不存在，创建新用户
	newUid, err := generateNewUserId()
	if err != nil {
		return nil, 0, err
	}

	// 建立 openid 到 game_uid 的映射关系
	err = GlobalRedis.Set("openid_to_uid:"+openid, strconv.FormatUint(newUid, 10))
	if err != nil {
		return nil, 0, err
	}

	// 判断是否为游客（根据openid前缀）
	isGuest := len(openid) >= 6 && openid[:6] == "guest_"

	// 初始化用户数据（游客和正常用户可以有不同的初始资源）
	initialUserData := &UserData{
		exp:      0,
		gold:     100, // 初始金币
		diamond:  10,  // 初始钻石
		nickname: username,
	}

	// 游客可以给更少的初始资源（如果需要区别对待）
	if isGuest {
		// 可以在这里调整游客的初始资源
		// initialUserData.gold = 50  // 游客给更少金币
		// initialUserData.diamond = 5 // 游客给更少钻石
	}

	// 保存初始用户数据到 Redis
	err = saveUserDataToRedis(newUid, initialUserData)
	if err != nil {
		return nil, 0, err
	}

	userType := "normal"
	if isGuest {
		userType = "guest"
	}
	slog.Info("Created new user", "type", userType, "openid", openid, "username", username, "uid", newUid)

	return initialUserData, newUid, nil
}
