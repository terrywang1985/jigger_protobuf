#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
简单的命令行测试客户端
用于测试与服务器的protobuf通信
"""

import asyncio
import websockets
import struct
import uuid
import time
import sys
import os

# 添加当前目录到Python路径
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

# 导入protobuf生成的类
import game_pb2 as game_pb
import desktop_pet_pb2 as desktop_pet_pb
import battle_pb2 as battle_pb


class TestClient:
    """简单的测试客户端"""
    
    def __init__(self):
        self.ws = None
        self.msg_serial_no = 0
        self.client_id = str(uuid.uuid4())
        self.message_buffer = b''
        self.authenticated = False
        self.uid = 0
        
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
            print("WebSocket连接未建立")
            return False
            
        try:
            # 序列化protobuf数据
            data = proto_data.SerializeToString() if proto_data else b''
            
            # 创建消息
            message = self.create_message(msg_id, data)
            
            # 打包并发送
            packed_data = self.pack_message(message)
            await self.ws.send(packed_data)
            
            print(f"✓ 发送消息: {game_pb.MessageId.Name(msg_id)}, 数据长度: {len(data)}")
            return True
            
        except Exception as e:
            print(f"✗ 发送消息失败: {e}")
            return False
    
    def unpack_message(self, data):
        """解包消息"""
        if len(data) < 4:
            return None, data
            
        # 读取长度
        length = struct.unpack('<I', data[:4])[0]
        
        # 检查是否有完整消息
        if len(data) < 4 + length:
            return None, data
            
        # 解析消息
        message_data = data[4:4+length]
        remaining_data = data[4+length:]
        
        try:
            message = game_pb.Message()
            message.ParseFromString(message_data)
            return message, remaining_data
        except Exception as e:
            print(f"✗ 解析消息失败: {e}")
            return None, remaining_data
    
    async def connect(self, uri):
        """连接到服务器"""
        try:
            print(f"正在连接到服务器: {uri}")
            self.ws = await websockets.connect(uri)
            print("✓ 已连接到服务器")
            return True
            
        except Exception as e:
            print(f"✗ 连接失败: {e}")
            return False
    
    async def send_auth_request(self, token="test_token"):
        """发送认证请求"""
        print("\n=== 发送认证请求 ===")
        
        auth_request = game_pb.AuthRequest()
        auth_request.token = token
        auth_request.protocol_version = "1.0"
        auth_request.client_version = "1.0.0"
        auth_request.device_type = "desktop"
        auth_request.device_id = str(uuid.uuid4())[:8]
        auth_request.app_id = "desktop_app"
        auth_request.nonce = str(uuid.uuid4())
        auth_request.timestamp = int(time.time() * 1000)
        auth_request.signature = ""  # 简化处理，不计算签名
        
        success = await self.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)
        if success:
            print("认证请求已发送，等待响应...")
        return success
    
    async def listen_once(self, timeout=5):
        """监听一次消息"""
        try:
            msg = await asyncio.wait_for(self.ws.recv(), timeout=timeout)
            
            if isinstance(msg, bytes):
                self.message_buffer += msg
            else:
                self.message_buffer += msg.encode('utf-8')
            
            # 处理缓冲区中的消息
            messages_processed = 0
            while True:
                message, self.message_buffer = self.unpack_message(self.message_buffer)
                if message is None:
                    break
                await self.handle_message(message)
                messages_processed += 1
            
            return messages_processed > 0
            
        except asyncio.TimeoutError:
            print(f"等待消息超时 ({timeout}秒)")
            return False
        except Exception as e:
            print(f"✗ 接收消息时出错: {e}")
            return False
    
    async def handle_message(self, message):
        """处理收到的消息"""
        msg_id = message.id
        msg_name = game_pb.MessageId.Name(msg_id) if msg_id in game_pb.MessageId.values() else f"Unknown({msg_id})"
        print(f"✓ 收到消息: {msg_name}")
        
        try:
            if msg_id == game_pb.MessageId.AUTH_RESPONSE:
                await self.handle_auth_response(message)
            elif msg_id == game_pb.MessageId.GET_USER_INFO_RESPONSE:
                await self.handle_user_info_response(message)
            elif msg_id == game_pb.MessageId.DRAW_CARD_RESPONSE:
                await self.handle_draw_card_response(message)
            else:
                print(f"  (未处理的消息类型)")
                
        except Exception as e:
            print(f"✗ 处理消息时出错: {e}")
    
    async def handle_auth_response(self, message):
        """处理认证响应"""
        print("\n=== 认证响应 ===\")
        
        auth_response = game_pb.AuthResponse()
        auth_response.ParseFromString(message.data)
        
        if auth_response.ret == game_pb.ErrorCode.OK:
            self.authenticated = True
            self.uid = auth_response.uid
            print("✓ 认证成功!")
            print(f"  UID: {auth_response.uid}")
            print(f"  昵称: {auth_response.nickname}")
            print(f"  金币: {auth_response.gold}")
            print(f"  钻石: {auth_response.diamond}")
            print(f"  等级: {auth_response.level}")
            print(f"  经验: {auth_response.exp}")
            
        else:
            print(f"✗ 认证失败: {game_pb.ErrorCode.Name(auth_response.ret)}")
            if auth_response.error_msg:
                print(f"  错误信息: {auth_response.error_msg}")
    
    async def handle_user_info_response(self, message):
        """处理用户信息响应"""
        print("\n=== 用户信息响应 ===\")
        
        user_info_response = game_pb.GetUserInfoResponse()
        user_info_response.ParseFromString(message.data)
        
        if user_info_response.ret == game_pb.ErrorCode.OK:
            user_info = user_info_response.user_info
            print("✓ 获取用户信息成功:")
            print(f"  姓名: {user_info.name}")
            print(f"  经验: {user_info.exp}")
            print(f"  金币: {user_info.gold}")
            print(f"  钻石: {user_info.diamond}")
            print(f"  抽卡次数: {user_info.draw_card_count}")
            print(f"  背包卡牌数量: {len(user_info.backpack.cards)}")
            
        else:
            print(f"✗ 获取用户信息失败: {game_pb.ErrorCode.Name(user_info_response.ret)}")
    
    async def handle_draw_card_response(self, message):
        """处理抽卡响应"""
        print("\n=== 抽卡响应 ===\")
        
        draw_card_response = game_pb.DrawCardResponse()
        draw_card_response.ParseFromString(message.data)
        
        if draw_card_response.ret == game_pb.ErrorCode.OK:
            print(f"✓ 抽卡成功! 获得 {len(draw_card_response.cards)} 张卡牌:")
            for i, card in enumerate(draw_card_response.cards, 1):
                print(f"  {i}. {card.name} (稀有度: {card.rarity}, ID: {card.id})")
        else:
            print(f"✗ 抽卡失败: {game_pb.ErrorCode.Name(draw_card_response.ret)}")
    
    async def request_user_info(self):
        """请求用户信息"""
        if not self.authenticated:
            print("✗ 未认证，无法请求用户信息")
            return False
            
        print("\n=== 请求用户信息 ===\")
        user_info_request = game_pb.GetUserInfoRequest()
        user_info_request.uid = self.uid
        
        return await self.send_message(game_pb.MessageId.GET_USER_INFO_REQUEST, user_info_request)
    
    async def request_draw_card(self, count=1):
        """请求抽卡"""
        if not self.authenticated:
            print("✗ 未认证，无法抽卡")
            return False
            
        print(f"\n=== 请求抽取 {count} 张卡牌 ===\")
        draw_card_request = game_pb.DrawCardRequest()
        draw_card_request.uid = self.uid
        draw_card_request.count = count
        
        return await self.send_message(game_pb.MessageId.DRAW_CARD_REQUEST, draw_card_request)
    
    async def close(self):
        """关闭连接"""
        if self.ws:
            await self.ws.close()
            print("连接已关闭")


async def main():
    """主测试函数"""
    print("=== 桌面宠物客户端 - Protobuf通信测试 ===\n")
    
    client = TestClient()
    
    # 连接服务器
    if not await client.connect("ws://127.0.0.1:18080/ws"):
        return
    
    try:
        # 1. 发送认证请求
        if await client.send_auth_request():
            # 等待认证响应
            await client.listen_once()
        
        # 2. 如果认证成功，请求用户信息
        if client.authenticated:
            if await client.request_user_info():
                await client.listen_once()
            
            # 3. 尝试抽卡
            if await client.request_draw_card(1):
                await client.listen_once()
                
            # 4. 再次获取用户信息查看变化
            if await client.request_user_info():
                await client.listen_once()
        
    except KeyboardInterrupt:
        print("\n用户中断测试")
    except Exception as e:
        print(f"\n测试过程中出错: {e}")
    finally:
        await client.close()


if __name__ == "__main__":
    asyncio.run(main())