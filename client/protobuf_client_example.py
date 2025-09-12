#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
桌面宠物客户端 - Protobuf版本
与服务器使用相同的Protobuf协议进行通信
"""

import asyncio
import websockets
import struct
import uuid
import time
import tkinter as tk
from tkinter import messagebox

# 导入protobuf生成的类
import game_pb2 as game_pb
import desktop_pet_pb2 as desktop_pet_pb
import battle_pb2 as battle_pb


class ProtobufClient:
    """Protobuf通信客户端"""
    
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
            return
            
        try:
            # 序列化protobuf数据
            data = proto_data.SerializeToString() if proto_data else b''
            
            # 创建消息
            message = self.create_message(msg_id, data)
            
            # 打包并发送
            packed_data = self.pack_message(message)
            await self.ws.send(packed_data)
            
            print(f"发送消息: {msg_id}, 数据长度: {len(data)}")
            
        except Exception as e:
            print(f"发送消息失败: {e}")
    
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
            print(f"解析消息失败: {e}")
            return None, remaining_data
    
    async def connect(self, uri, token):
        """连接到服务器"""
        try:
            self.ws = await websockets.connect(uri)
            print(f"已连接到服务器: {uri}")
            
            # 发送认证请求
            await self.send_auth_request(token)
            
            # 开始监听消息
            await self.listen_messages()
            
        except Exception as e:
            print(f"连接失败: {e}")
    
    async def send_auth_request(self, token):
        """发送认证请求"""
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
        
        await self.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)
        print("已发送认证请求")
    
    async def listen_messages(self):
        """监听服务器消息"""
        try:
            async for msg in self.ws:
                if isinstance(msg, bytes):
                    self.message_buffer += msg
                else:
                    self.message_buffer += msg.encode('utf-8')
                
                # 处理缓冲区中的所有完整消息
                while True:
                    message, self.message_buffer = self.unpack_message(self.message_buffer)
                    if message is None:
                        break
                    await self.handle_message(message)
                    
        except websockets.exceptions.ConnectionClosed:
            print("服务器连接已关闭")
        except Exception as e:
            print(f"监听消息时出错: {e}")
    
    async def handle_message(self, message):
        """处理收到的消息"""
        msg_id = message.id
        print(f"收到消息 ID: {msg_id}")
        
        try:
            if msg_id == game_pb.MessageId.AUTH_RESPONSE:
                await self.handle_auth_response(message)
            elif msg_id == game_pb.MessageId.GET_USER_INFO_RESPONSE:
                await self.handle_user_info_response(message)
            elif msg_id == game_pb.MessageId.DRAW_CARD_RESPONSE:
                await self.handle_draw_card_response(message)
            else:
                print(f"未处理的消息类型: {msg_id}")
                
        except Exception as e:
            print(f"处理消息时出错: {e}")
    
    async def handle_auth_response(self, message):
        """处理认证响应"""
        auth_response = game_pb.AuthResponse()
        auth_response.ParseFromString(message.data)
        
        if auth_response.ret == game_pb.ErrorCode.OK:
            self.authenticated = True
            self.uid = auth_response.uid
            print(f"认证成功!")
            print(f"UID: {auth_response.uid}")
            print(f"昵称: {auth_response.nickname}")
            print(f"金币: {auth_response.gold}")
            print(f"钻石: {auth_response.diamond}")
            
            # 认证成功后请求用户信息
            await self.request_user_info()
            
        else:
            print(f"认证失败: {auth_response.error_msg}")
    
    async def handle_user_info_response(self, message):
        """处理用户信息响应"""
        user_info_response = game_pb.GetUserInfoResponse()
        user_info_response.ParseFromString(message.data)
        
        if user_info_response.ret == game_pb.ErrorCode.OK:
            user_info = user_info_response.user_info
            print(f"用户信息:")
            print(f"  姓名: {user_info.name}")
            print(f"  经验: {user_info.exp}")
            print(f"  金币: {user_info.gold}")
            print(f"  钻石: {user_info.diamond}")
            print(f"  抽卡次数: {user_info.draw_card_count}")
            print(f"  背包卡牌数量: {len(user_info.backpack.cards)}")
            
        else:
            print("获取用户信息失败")
    
    async def handle_draw_card_response(self, message):
        """处理抽卡响应"""
        draw_card_response = game_pb.DrawCardResponse()
        draw_card_response.ParseFromString(message.data)
        
        if draw_card_response.ret == game_pb.ErrorCode.OK:
            print(f"抽卡成功! 获得 {len(draw_card_response.cards)} 张卡牌:")
            for card in draw_card_response.cards:
                print(f"  - {card.name} (稀有度: {card.rarity})")
        else:
            print("抽卡失败")
    
    async def request_user_info(self):
        """请求用户信息"""
        if not self.authenticated:
            print("未认证，无法请求用户信息")
            return
            
        user_info_request = game_pb.GetUserInfoRequest()
        user_info_request.uid = self.uid
        
        await self.send_message(game_pb.MessageId.GET_USER_INFO_REQUEST, user_info_request)
        print("已请求用户信息")
    
    async def request_draw_card(self, count=1):
        """请求抽卡"""
        if not self.authenticated:
            print("未认证，无法抽卡")
            return
            
        draw_card_request = game_pb.DrawCardRequest()
        draw_card_request.uid = self.uid
        draw_card_request.count = count
        
        await self.send_message(game_pb.MessageId.DRAW_CARD_REQUEST, draw_card_request)
        print(f"已请求抽取 {count} 张卡牌")


class SimpleAuthManager:
    """简单的认证管理器"""
    
    def __init__(self):
        self.token = None
        self.openid = None
    
    def get_test_token(self):
        """获取测试用的token（实际项目中需要通过登录服务器获取）"""
        # 这里返回一个测试token，实际项目中需要通过登录API获取
        return "test_token_12345"


class DesktopPetClientGUI:
    """桌面宠物客户端GUI"""
    
    def __init__(self):
        self.root = tk.Tk()
        self.root.title("桌面宠物客户端 - Protobuf版本")
        self.root.geometry("400x300")
        
        self.client = ProtobufClient()
        self.auth = SimpleAuthManager()
        
        self.setup_ui()
    
    def setup_ui(self):
        """设置用户界面"""
        # 连接按钮
        tk.Button(self.root, text="连接服务器", command=self.connect_server).pack(pady=10)
        
        # 抽卡按钮
        tk.Button(self.root, text="抽卡 (1张)", command=lambda: self.draw_card(1)).pack(pady=5)
        tk.Button(self.root, text="抽卡 (10张)", command=lambda: self.draw_card(10)).pack(pady=5)
        
        # 用户信息按钮
        tk.Button(self.root, text="获取用户信息", command=self.get_user_info).pack(pady=5)
        
        # 状态标签
        self.status_label = tk.Label(self.root, text="未连接", fg="red")
        self.status_label.pack(pady=10)
    
    def connect_server(self):
        """连接服务器"""
        token = self.auth.get_test_token()
        
        # 在异步线程中连接
        asyncio.create_task(self.client.connect("ws://127.0.0.1:18080/ws", token))
        
        self.status_label.config(text="正在连接...", fg="orange")
    
    def draw_card(self, count):
        """抽卡"""
        asyncio.create_task(self.client.request_draw_card(count))
    
    def get_user_info(self):
        """获取用户信息"""
        asyncio.create_task(self.client.request_user_info())
    
    def run(self):
        """运行GUI"""
        self.root.mainloop()


async def main():
    """主函数"""
    print("桌面宠物客户端 - Protobuf版本")
    print("正在连接服务器...")
    
    # 创建客户端
    client = ProtobufClient()
    auth = SimpleAuthManager()
    
    # 获取token（实际项目中需要通过登录API获取）
    token = auth.get_test_token()
    
    # 连接服务器
    await client.connect("ws://127.0.0.1:18080/ws", token)


if __name__ == "__main__":
    # 可以选择使用命令行版本或GUI版本
    
    # 命令行版本
    # asyncio.run(main())
    
    # GUI版本
    app = DesktopPetClientGUI()
    app.run()