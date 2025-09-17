package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	pb "proto"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// 登录请求结构
type LoginRequest struct {
	DeviceID string `json:"device_id"`
	AppID    string `json:"app_id"`
	IsGuest  bool   `json:"is_guest"`
}

// 登录响应结构
type LoginResponse struct {
	Success    bool   `json:"success"`
	GatewayURL string `json:"gateway_url,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Username   string `json:"username,omitempty"`
	OpenID     string `json:"openid,omitempty"`
	Error      string `json:"error,omitempty"`
	ExpiresIn  int64  `json:"expires_in,omitempty"`
}

func main() {
	// 测试游客登录并创建房间的完整流程
	fmt.Println("开始测试游客登录并创建房间...")

	// 1. 执行真实的游客登录获取token
	token, err := guestLogin()
	if err != nil {
		log.Fatalf("游客登录失败: %v", err)
	}
	fmt.Printf("游客登录成功，token: %s\n", token)

	// 2. 连接到游戏服务器WebSocket
	conn, err := connectToGameServer()
	if err != nil {
		log.Fatalf("连接游戏服务器失败: %v", err)
	}
	defer conn.Close()
	fmt.Println("成功连接到游戏服务器")

	// 3. 发送认证请求
	err = sendAuthRequest(conn, token)
	if err != nil {
		log.Fatalf("发送认证请求失败: %v", err)
	}
	fmt.Println("认证请求已发送")

	// 4. 等待认证响应
	authResp, err := waitForAuthResponse(conn)
	if err != nil {
		log.Fatalf("等待认证响应失败: %v", err)
	}
	fmt.Printf("认证成功，UID: %d, 用户名: %s\n", authResp.GetUid(), authResp.GetNickname())

	// 等待一段时间确保认证完成
	time.Sleep(1 * time.Second)

	// 5. 发送创建房间请求
	err = sendCreateRoomRequest(conn, "测试房间")
	if err != nil {
		log.Fatalf("发送创建房间请求失败: %v", err)
	}
	fmt.Println("创建房间请求已发送")

	// 6. 等待创建房间响应
	roomResp, err := waitForCreateRoomResponse(conn)
	if err != nil {
		log.Fatalf("等待创建房间响应失败: %v", err)
	}

	if roomResp.GetRet() == pb.ErrorCode_OK {
		fmt.Printf("创建房间成功，房间ID: %s, 房间名: %s\n", roomResp.GetRoom().GetId(), roomResp.GetRoom().GetName())
	} else {
		fmt.Printf("创建房间失败，错误码: %v\n", roomResp.GetRet())
	}
}

func guestLogin() (string, error) {
	// 执行真实的游客登录
	loginReq := LoginRequest{
		DeviceID: "test_device_12345",
		AppID:    "desktop_app",
		IsGuest:  true,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return "", err
	}

	resp, err := http.Post("http://127.0.0.1:8081/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}

	if !loginResp.Success {
		return "", fmt.Errorf("登录失败: %s", loginResp.Error)
	}

	return loginResp.SessionID, nil
}

func connectToGameServer() (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: "127.0.0.1:18080", Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	return conn, err
}

func sendAuthRequest(conn *websocket.Conn, token string) error {
	// 创建认证请求
	authRequest := &pb.AuthRequest{
		Token:          token,
		ProtocolVersion: "1.0",
		ClientVersion:   "1.0.0",
		DeviceType:      "test_client",
		DeviceId:        "test_device_12345",
		AppId:           "desktop_app",
		Nonce:           "test_nonce",
		Timestamp:       time.Now().UnixNano() / int64(time.Millisecond),
		Signature:       "",
		IsGuest:         true,
	}

	// 序列化认证请求
	requestData, err := proto.Marshal(authRequest)
	if err != nil {
		return err
	}

	// 创建消息
	message := &pb.Message{
		ClientId:    "test_client_123",
		MsgSerialNo: 1,
		Id:          pb.MessageId_AUTH_REQUEST,
		Data:        requestData,
	}

	// 序列化完整消息
	messageData, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	// 添加4字节长度头
	fullData := make([]byte, 4+len(messageData))
	// 使用小端序写入长度
	fullData[0] = byte(len(messageData))
	fullData[1] = byte(len(messageData) >> 8)
	fullData[2] = byte(len(messageData) >> 16)
	fullData[3] = byte(len(messageData) >> 24)
	copy(fullData[4:], messageData)

	// 发送消息
	return conn.WriteMessage(websocket.BinaryMessage, fullData)
}

func waitForAuthResponse(conn *websocket.Conn) (*pb.AuthResponse, error) {
	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}

		// 解析消息长度（前4字节）
		if len(data) < 4 {
			continue
		}
		length := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24

		// 检查数据是否完整
		if len(data) < 4+length {
			continue
		}

		// 解析消息体
		messageData := data[4 : 4+length]
		var message pb.Message
		if err := proto.Unmarshal(messageData, &message); err != nil {
			continue
		}

		// 检查是否为认证响应
		if message.GetId() == pb.MessageId_AUTH_RESPONSE {
			var authResp pb.AuthResponse
			if err := proto.Unmarshal(message.GetData(), &authResp); err != nil {
				return nil, err
			}
			return &authResp, nil
		}
	}
}

func sendCreateRoomRequest(conn *websocket.Conn, roomName string) error {
	// 创建创建房间请求
	createRoomRequest := &pb.CreateRoomRequest{
		Name: roomName,
	}

	// 序列化创建房间请求
	requestData, err := proto.Marshal(createRoomRequest)
	if err != nil {
		return err
	}

	// 创建消息
	message := &pb.Message{
		ClientId:    "test_client_123",
		MsgSerialNo: 2,
		Id:          pb.MessageId_CREATE_ROOM_REQUEST,
		Data:        requestData,
	}

	// 序列化完整消息
	messageData, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	// 添加4字节长度头
	fullData := make([]byte, 4+len(messageData))
	// 使用小端序写入长度
	fullData[0] = byte(len(messageData))
	fullData[1] = byte(len(messageData) >> 8)
	fullData[2] = byte(len(messageData) >> 16)
	fullData[3] = byte(len(messageData) >> 24)
	copy(fullData[4:], messageData)

	// 发送消息
	return conn.WriteMessage(websocket.BinaryMessage, fullData)
}

func waitForCreateRoomResponse(conn *websocket.Conn) (*pb.CreateRoomResponse, error) {
	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}

		// 解析消息长度（前4字节）
		if len(data) < 4 {
			continue
		}
		length := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24

		// 检查数据是否完整
		if len(data) < 4+length {
			continue
		}

		// 解析消息体
		messageData := data[4 : 4+length]
		var message pb.Message
		if err := proto.Unmarshal(messageData, &message); err != nil {
			continue
		}

		// 检查是否为创建房间响应
		if message.GetId() == pb.MessageId_CREATE_ROOM_RESPONSE {
			var roomResp pb.CreateRoomResponse
			if err := proto.Unmarshal(message.GetData(), &roomResp); err != nil {
				return nil, err
			}
			return &roomResp, nil
		}
	}
}