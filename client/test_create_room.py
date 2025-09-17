#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import asyncio
import websockets
import struct
import uuid
import requests
import json
import time

# 导入protobuf生成的类
import game_pb2 as game_pb

class ProtobufClient:
    """Protobuf通信客户端"""
    def __init__(self):
        self.ws = None
        self.msg_serial_no = 0
        self.client_id = str(uuid.uuid4())
        
    def get_next_serial_no(self):
        self.msg_serial_no += 1
        return self.msg_serial_no
    
    def create_message(self, msg_id, data):
        """创建protobuf消息"""
        message = game_pb.Message()
        message.clientId = self.client_id
        message.msgSerialNo = self.get_next_serial_no()
        message.id = msg_id
        message.data = data
        return message
    
    def pack_message(self, message):
        """打包消息（4字节长度头 + protobuf数据）"""
        data = message.SerializeToString()
        length = struct.pack('<I', len(data))  # 小端序4字节长度
        return length + data
    
    async def send_message(self, msg_id, proto_data):
        """发送protobuf消息"""
        if not self.ws:
            print(f"无法发送消息，WebSocket连接不存在: msg_id={msg_id}")
            return
            
        try:
            # 序列化protobuf数据
            data = proto_data.SerializeToString() if proto_data else b''
            
            # 创建消息
            message = self.create_message(msg_id, data)
            
            # 打包并发送
            packed_data = self.pack_message(message)
            print(f"正在发送消息: ID={msg_id}, 序列号={message.msgSerialNo}, 数据长度={len(data)}")
            await self.ws.send(packed_data)
            
            print(f"消息发送成功: {msg_id}, data length: {len(data)}")
            
        except Exception as e:
            print(f"发送消息失败: msg_id={msg_id}, error={e}")
            import traceback
            traceback.print_exc()

async def guest_login():
    """执行游客登录"""
    url = "http://localhost:8081/login"
    data = {
        "device_id": "python_test_device_12345",
        "app_id": "desktop_app",
        "is_guest": True
    }
    
    try:
        print(f"游客登录中... 设备ID: {data['device_id']}")
        print(f"请求数据: {data}")
        response = requests.post(url, json=data, timeout=10, proxies={"http": None, "https": None})
        
        print(f"HTTP状态码: {response.status_code}")
        print(f"响应内容: {response.text}")
        
        if response.status_code == 200:
            result = response.json()
            if result.get("success"):
                print(f"游客登录成功！用户名: {result.get('username')}")
                return result.get("session_id")
            else:
                error_msg = result.get("error", "游客登录失败")
                raise Exception(error_msg)
        else:
            error_msg = f"HTTP {response.status_code}: {response.text}"
            raise Exception(error_msg)
            
    except Exception as e:
        print(f"请求异常: {str(e)}")
        raise

async def connect_to_game_server():
    """连接到游戏服务器"""
    uri = "ws://127.0.0.1:18080/ws"
    print(f"尝试连接到WebSocket: {uri}")
    ws = await websockets.connect(uri)
    print("成功连接到游戏服务器")
    return ws

async def send_auth_request(protobuf_client, token):
    """发送认证请求"""
    print("准备发送认证请求...")
    # 创建认证请求
    auth_request = game_pb.AuthRequest()
    auth_request.token = token
    auth_request.protocol_version = "1.0"
    auth_request.client_version = "1.0.0"
    auth_request.device_type = "python_test_client"
    auth_request.device_id = "python_test_device_12345"
    auth_request.app_id = "desktop_app"
    auth_request.nonce = str(uuid.uuid4())
    auth_request.timestamp = int(time.time() * 1000)
    auth_request.signature = ""
    auth_request.is_guest = True
    
    # 发送认证请求
    await protobuf_client.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)
    print("认证请求已发送")

async def wait_for_auth_response(protobuf_client):
    """等待认证响应"""
    print("等待认证响应...")
    buffer = b''
    
    while True:
        data = await protobuf_client.ws.recv()
        if isinstance(data, str):
            data = data.encode('utf-8')
        buffer += data
        
        while True:
            # 检查是否有足够的数据读取长度
            if len(buffer) < 4:
                break
                
            # 读取包长度
            length = struct.unpack('<I', buffer[:4])[0]
            
            # 检查是否有完整包
            if len(buffer) < 4 + length:
                break
                
            # 读取完整包
            message_data = buffer[4:4+length]
            buffer = buffer[4+length:]  # 移除已处理的数据
            
            # 解析消息
            message = game_pb.Message()
            try:
                message.ParseFromString(message_data)
            except Exception as e:
                print(f"解析消息失败: {e}")
                continue
                
            # 检查是否为认证响应
            if message.id == game_pb.MessageId.AUTH_RESPONSE:
                auth_response = game_pb.AuthResponse()
                auth_response.ParseFromString(message.data)
                print(f"认证成功，UID: {auth_response.uid}, 用户名: {auth_response.nickname}")
                return auth_response

async def send_create_room_request(protobuf_client, room_name):
    """发送创建房间请求"""
    print(f"准备发送创建房间请求: {room_name}")
    # 创建创建房间请求
    create_room_request = game_pb.CreateRoomRequest()
    create_room_request.name = room_name
    
    # 发送创建房间请求
    await protobuf_client.send_message(game_pb.MessageId.CREATE_ROOM_REQUEST, create_room_request)
    print("创建房间请求已发送")

async def wait_for_create_room_response(protobuf_client):
    """等待创建房间响应"""
    print("等待创建房间响应...")
    buffer = b''
    
    while True:
        data = await protobuf_client.ws.recv()
        if isinstance(data, str):
            data = data.encode('utf-8')
        buffer += data
        
        while True:
            # 检查是否有足够的数据读取长度
            if len(buffer) < 4:
                break
                
            # 读取包长度
            length = struct.unpack('<I', buffer[:4])[0]
            
            # 检查是否有完整包
            if len(buffer) < 4 + length:
                break
                
            # 读取完整包
            message_data = buffer[4:4+length]
            buffer = buffer[4+length:]  # 移除已处理的数据
            
            # 解析消息
            message = game_pb.Message()
            try:
                message.ParseFromString(message_data)
            except Exception as e:
                print(f"解析消息失败: {e}")
                continue
                
            # 检查是否为创建房间响应
            if message.id == game_pb.MessageId.CREATE_ROOM_RESPONSE:
                room_response = game_pb.CreateRoomResponse()
                room_response.ParseFromString(message.data)
                if room_response.ret == game_pb.ErrorCode.OK:
                    print(f"创建房间成功，房间ID: {room_response.room.id}, 房间名: {room_response.room.name}")
                else:
                    print(f"创建房间失败，错误码: {room_response.ret}")
                return room_response

async def main():
    """主函数"""
    print("开始测试游客登录并创建房间...")
    
    try:
        # 1. 执行游客登录
        token = await guest_login()
        
        # 2. 连接到游戏服务器
        ws = await connect_to_game_server()
        
        # 3. 创建protobuf客户端
        protobuf_client = ProtobufClient()
        protobuf_client.ws = ws
        
        # 4. 发送认证请求
        await send_auth_request(protobuf_client, token)
        
        # 5. 等待认证响应
        auth_resp = await wait_for_auth_response(protobuf_client)
        
        # 等待一段时间确保认证完成
        await asyncio.sleep(1)
        
        # 6. 发送创建房间请求
        await send_create_room_request(protobuf_client, "Python测试房间")
        
        # 7. 等待创建房间响应
        room_resp = await wait_for_create_room_response(protobuf_client)
        
        # 8. 关闭连接
        await ws.close()
        print("测试完成")
        
    except Exception as e:
        print(f"测试失败: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    asyncio.run(main())