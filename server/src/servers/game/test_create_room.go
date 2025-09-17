package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func main() {
	// 首先测试连接到BattleServer并调用CreateRoomRpc
	fmt.Println("测试连接到BattleServer并创建房间...")

	// 创建gRPC客户端连接到BattleServer
	conn, err := grpc.Dial(
		"127.0.0.1:8693",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatalf("无法连接到BattleServer: %v", err)
	}
	defer conn.Close()

	fmt.Println("成功连接到BattleServer")

	// 创建RoomRpcService客户端
	client := pb.NewRoomRpcServiceClient(conn)

	// 创建玩家数据
	player := &pb.PlayerInitData{
		PlayerId:   12345,
		PlayerName: "TestPlayer",
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用CreateRoomRpc
	fmt.Println("调用CreateRoomRpc...")
	createRoomReq := &pb.CreateRoomRpcRequest{
		Player: player,
	}

	resp, err := client.CreateRoomRpc(ctx, createRoomReq)
	if err != nil {
		log.Fatalf("CreateRoomRpc调用失败: %v", err)
	}

	fmt.Printf("CreateRoomRpc响应: Ret=%v, RoomId=%s\n", resp.GetRet(), resp.GetRoomId())

	// 现在测试打包和发送protobuf消息
	fmt.Println("\n测试打包和发送protobuf消息...")

	// 创建CreateRoomRequest消息
	createRoomRequest := &pb.CreateRoomRequest{
		Name: "TestRoom",
	}

	// 序列化CreateRoomRequest
	requestData, err := proto.Marshal(createRoomRequest)
	if err != nil {
		log.Fatalf("序列化CreateRoomRequest失败: %v", err)
	}

	// 创建游戏消息
	message := &pb.Message{
		ClientId:     "test_client",
		MsgSerialNo:  1,
		Id:           pb.MessageId_CREATE_ROOM_REQUEST,
		Data:         requestData,
	}

	// 序列化完整消息
	messageData, err := proto.Marshal(message)
	if err != nil {
		log.Fatalf("序列化完整消息失败: %v", err)
	}

	fmt.Printf("消息ID: %v\n", message.GetId())
	fmt.Printf("消息序列号: %v\n", message.GetMsgSerialNo())
	fmt.Printf("客户端ID: %v\n", message.GetClientId())
	fmt.Printf("数据长度: %v\n", len(message.GetData()))
	fmt.Printf("完整消息长度: %v\n", len(messageData))

	fmt.Println("测试完成")
}