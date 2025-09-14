# -*- coding: utf-8 -*-

"""
用户名密码注册登录客户端
实现完整的用户名密码登录流程:
1. 客户端 -> 平台服务：用户名密码注册/登录
2. 平台服务：返回token和openID
3. 客户端 -> 登录服务：使用用户名密码登录获取sessionID
4. 客户端 -> 游戏服务器：使用sessionID认证并开始游戏
"""

import asyncio
import websockets
import struct
import uuid
import time
import sys
import os
import requests
import json

# 添加当前目录到Python路径
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

# 导入protobuf生成的类
import game_pb2 as game_pb
import desktop_pet_pb2 as desktop_pet_pb
import battle_pb2 as battle_pb


class UsernameAuthClient:
    """用户名密码认证客户端"""
    
    def __init__(self):
        # WebSocket连接
        self.ws = None
        self.msg_serial_no = 0
        self.client_id = str(uuid.uuid4())
        self.message_buffer = b''
        
        # 认证状态
        self.authenticated = False
        self.uid = 0
        self.platform_token = None
        self.session_id = None
        self.openid = None
        self.username = None
        
        # 服务地址配置
        self.platform_base_url = "http://localhost:8080"  # API网关地址
        self.login_server_url = "http://localhost:8081"   # 登录服务地址
        self.game_server_url = "ws://127.0.0.1:18080/ws" # 游戏服务器WebSocket地址
        self.app_id = "jigger_game"
        
    def get_next_serial_no(self):
        """获取下一个消息序列号"""
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
        """发送protobuf消息到游戏服务器"""
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
    
    def register_to_platform(self, username, password, email):
        """步骤1a: 向平台服务注册新用户"""
        print(f"\n=== 步骤1a: 平台用户注册 ===")
        print(f"用户名: {username}")
        print(f"邮箱: {email}")
        
        url = f"{self.platform_base_url}/auth/register"
        data = {
            "username": username,
            "password": password,
            "email": email,
            "app_id": self.app_id
        }
        
        try:
            response = requests.post(url, json=data, timeout=10)
            response.raise_for_status()
            
            result = response.json()
            if "error" in result and result["error"]:
                print(f"✗ 平台注册失败: {result['error']}")
                return False
            
            self.platform_token = result.get('token')
            self.openid = result.get('openid')
            user_info = result.get('user', {})
            self.username = user_info.get('username', '')
            
            print(f"✓ 平台注册成功: {result.get('message', '')}")
            print(f"  OpenID: {self.openid}")
            print(f"  用户名: {self.username}")
            print(f"  Token: {self.platform_token[:20]}...")
            
            return True
            
        except requests.exceptions.RequestException as e:
            print(f"✗ 平台注册请求失败: {e}")
            return False
    
    def login_to_platform(self, username, password):
        """步骤1b: 使用用户名密码登录平台获取token"""
        print(f"\n=== 步骤1b: 平台用户登录 ===")
        print(f"用户名: {username}")
        
        url = f"{self.platform_base_url}/auth/login"
        data = {
            "username": username,
            "password": password,
            "app_id": self.app_id,
            "device_id": f"python_client_{uuid.uuid4().hex[:8]}"
        }
        
        try:
            response = requests.post(url, json=data, timeout=10)
            response.raise_for_status()
            
            result = response.json()
            if "error" in result and result["error"]:
                print(f"✗ 平台登录失败: {result['error']}")
                return False
            
            self.platform_token = result.get('token')
            self.openid = result.get('openid')
            user_info = result.get('user', {})
            self.username = user_info.get('username', '')
            
            print(f"✓ 平台登录成功: {result.get('message', '')}")
            print(f"  OpenID: {self.openid}")
            print(f"  用户名: {self.username}")
            print(f"  Token: {self.platform_token[:20]}...")
            
            return True
            
        except requests.exceptions.RequestException as e:
            print(f"✗ 平台登录请求失败: {e}")
            return False
    
    def register_to_game_server(self, username, password, email):
        """步骤2a: 在游戏登录服务器注册"""
        print(f"\n=== 步骤2a: 游戏服务器注册 ===")
        
        url = f"{self.login_server_url}/register"
        data = {
            "username": username,
            "password": password,
            "email": email
        }
        
        try:
            response = requests.post(url, json=data, timeout=10)
            response.raise_for_status()
            
            result = response.json()
            if not result.get('success', False):
                print(f"✗ 游戏服务器注册失败: {result.get('error', '')}")
                return False
            
            self.session_id = result.get('session_id')
            gateway_url = result.get('gateway_url', '')
            
            print(f"✓ 游戏服务器注册成功!")
            print(f"  SessionID: {self.session_id}")
            print(f"  网关地址: {gateway_url}")
            
            return True
            
        except requests.exceptions.RequestException as e:
            print(f"✗ 游戏服务器注册请求失败: {e}")
            return False
    
    def login_to_game_server(self, username, password):
        """步骤2b: 登录到游戏登录服务器获取sessionID"""
        print(f"\n=== 步骤2b: 游戏服务器登录 ===")
        
        url = f"{self.login_server_url}/login"
        data = {
            "username": username,
            "password": password,
            "device_id": f"python_client_{uuid.uuid4().hex[:8]}"
        }
        
        try:
            response = requests.post(url, json=data, timeout=10)
            response.raise_for_status()
            
            result = response.json()
            if not result.get('success', False):
                print(f"✗ 游戏服务器登录失败: {result.get('error', '')}")
                return False
            
            self.session_id = result.get('session_id')
            gateway_url = result.get('gateway_url', '')
            
            print(f"✓ 游戏服务器登录成功!")
            print(f"  SessionID: {self.session_id}")
            print(f"  网关地址: {gateway_url}")
            
            return True
            
        except requests.exceptions.RequestException as e:
            print(f"✗ 游戏服务器登录请求失败: {e}")
            return False
    
    async def connect_to_game_server(self):
        """步骤3: 连接到游戏服务器WebSocket"""
        print(f"\n=== 步骤3: 连接游戏服务器 ===")
        
        try:
            print(f"正在连接到游戏服务器: {self.game_server_url}")
            self.ws = await websockets.connect(self.game_server_url)
            print("✓ 已连接到游戏服务器WebSocket")
            return True
            
        except Exception as e:
            print(f"✗ 连接游戏服务器失败: {e}")
            return False
    
    async def authenticate_with_game_server(self):
        """步骤4: 向游戏服务器发送认证请求"""
        print(f"\n=== 步骤4: 游戏服务器认证 ===")
        
        if not self.session_id:
            print("✗ 没有session_id，无法认证")
            return False
        
        auth_request = game_pb.AuthRequest()
        auth_request.token = self.session_id  # 使用session_id作为token
        auth_request.protocol_version = "1.0"
        auth_request.client_version = "1.0.0"
        auth_request.device_type = "desktop"
        auth_request.device_id = f"python_client_{uuid.uuid4().hex[:8]}"
        auth_request.app_id = self.app_id
        auth_request.nonce = str(uuid.uuid4())
        auth_request.timestamp = int(time.time() * 1000)
        auth_request.signature = ""  # 简化处理，不计算签名
        
        success = await self.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)
        if success:
            print("游戏服务器认证请求已发送，等待响应...")
        return success
    
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
        print("\n=== 游戏服务器认证响应 ===")
        
        auth_response = game_pb.AuthResponse()
        auth_response.ParseFromString(message.data)
        
        if auth_response.ret == game_pb.ErrorCode.OK:
            self.authenticated = True
            self.uid = auth_response.uid
            print("✓ 游戏服务器认证成功!")
            print(f"  游戏UID: {auth_response.uid}")
            print(f"  昵称: {auth_response.nickname}")
            print(f"  金币: {auth_response.gold}")
            print(f"  钻石: {auth_response.diamond}")
            print(f"  等级: {auth_response.level}")
            print(f"  经验: {auth_response.exp}")
            print(f"  连接ID: {auth_response.conn_id}")
            
        else:
            print(f"✗ 游戏服务器认证失败: {game_pb.ErrorCode.Name(auth_response.ret)}")
            if auth_response.error_msg:
                print(f"  错误信息: {auth_response.error_msg}")
    
    async def handle_user_info_response(self, message):
        """处理用户信息响应"""
        print("\n=== 用户信息响应 ===")
        
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
        print("\n=== 抽卡响应 ===")
        
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
            
        print("\n=== 请求用户信息 ===")
        user_info_request = game_pb.GetUserInfoRequest()
        user_info_request.uid = self.uid
        
        return await self.send_message(game_pb.MessageId.GET_USER_INFO_REQUEST, user_info_request)
    
    async def request_draw_card(self, count=1):
        """请求抽卡"""
        if not self.authenticated:
            print("✗ 未认证，无法抽卡")
            return False
            
        print(f"\n=== 请求抽取 {count} 张卡牌 ===")
        draw_card_request = game_pb.DrawCardRequest()
        draw_card_request.uid = self.uid
        draw_card_request.count = count
        
        return await self.send_message(game_pb.MessageId.DRAW_CARD_REQUEST, draw_card_request)
    
    async def close(self):
        """关闭连接"""
        if self.ws:
            await self.ws.close()
            print("WebSocket连接已关闭")


async def main():
    """主函数：完整的用户名密码登录流程"""
    print("=== 用户名密码注册登录客户端 ===\n")
    
    client = UsernameAuthClient()
    
    try:
        # 获取用户选择
        choice = input("选择操作 (1: 注册新用户, 2: 登录已有用户): ").strip()
        
        if choice == "1":
            # 注册流程
            print("\n=== 用户注册流程 ===")
            username = input("请输入用户名: ").strip()
            password = input("请输入密码: ").strip()
            email = input("请输入邮箱: ").strip()
            
            # 步骤1a: 平台注册
            if not client.register_to_platform(username, password, email):
                print("平台注册失败，退出")
                return
            
            # 步骤2a: 游戏服务器注册
            if not client.register_to_game_server(username, password, email):
                print("游戏服务器注册失败，退出")
                return
                
        elif choice == "2":
            # 登录流程
            print("\n=== 用户登录流程 ===")
            username = input("请输入用户名: ").strip()
            password = input("请输入密码: ").strip()
            
            # 步骤1b: 平台登录
            if not client.login_to_platform(username, password):
                print("平台登录失败，退出")
                return
            
            # 步骤2b: 游戏服务器登录
            if not client.login_to_game_server(username, password):
                print("游戏服务器登录失败，退出")
                return
        else:
            print("无效选择，退出")
            return
        
        # 步骤3: 连接游戏服务器
        if not await client.connect_to_game_server():
            print("连接游戏服务器失败，退出")
            return
        
        # 步骤4: 游戏服务器认证
        if await client.authenticate_with_game_server():
            # 等待认证响应
            await client.listen_once()
        
        # 认证成功后进行游戏操作
        if client.authenticated:
            print("\n=== 开始游戏操作 ===")
            
            # 获取用户信息
            if await client.request_user_info():
                await client.listen_once()
            
            # 尝试抽卡
            draw_choice = input("\n是否要抽卡？(y/n): ").strip().lower()
            if draw_choice == 'y':
                if await client.request_draw_card(1):
                    await client.listen_once()
                
                # 再次获取用户信息查看变化
                if await client.request_user_info():
                    await client.listen_once()
            
            print("\n=== 完整流程测试完成 ===")
            print("您已成功通过用户名密码登录并连接到游戏服务器！")
        
    except KeyboardInterrupt:
        print("\n用户中断测试")
    except Exception as e:
        print(f"\n测试过程中出错: {e}")
        import traceback
        traceback.print_exc()
    finally:
        await client.close()


if __name__ == "__main__":
    asyncio.run(main())