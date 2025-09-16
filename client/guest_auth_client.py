#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
游客登录客户端示例 (统一接口版)
演示如何通过游客模式登录游戏服务器

游客登录流程：
1. 调用统一的/login接口，设置is_guest=true
2. LoginServer根据is_guest参数路由到游客登录逻辑
3. 返回sessionid保持流程通用性
4. GameServer使用AuthRequest.is_guest字段进行认证
5. 后续可以在游戏内绑定正式账号

优劢：
- 接口统一，减少复杂度
- 通过参数区分登录类型更灵活
- 保持流程一致性
"""

import asyncio
import websockets
import json
import struct
import uuid
import time
import random
from datetime import datetime

class GuestAuthClient:
    def __init__(self):
        self.device_id = self.generate_device_id()
        self.app_id = "jigger_game"
        self.session_id = None
        self.gateway_url = None
        self.websocket = None
        self.user_info = {}
        
    def generate_device_id(self):
        """生成设备ID"""
        return f"device_{uuid.uuid4().hex[:16]}"
    
    async def guest_login(self):
        """游客登录流程（使用统一的/login接口）"""
        print(f"开始游客登录流程...")
        print(f"设备ID: {self.device_id}")
        print("注意：使用统一的/login接口，通过is_guest参数区分")
        
        # 使用统一的/login接口，通过is_guest参数标识游客登录
        login_url = "http://localhost:8081/login"  # 统一接口
        
        login_data = {
            "device_id": self.device_id,
            "app_id": self.app_id,
            "is_guest": True  # 关键参数：标识为游客登录
        }
        
        try:
            import requests
            response = requests.post(login_url, json=login_data, timeout=10)
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    # 游客登录现在返回SessionID保持流程通用
                    self.session_id = result.get("session_id")
                    self.gateway_url = result.get("gateway_url", "localhost:18080")
                    self.user_info = {
                        "username": result.get("username"),
                        "openid": result.get("openid")
                    }
                    print(f"游客登录成功！")
                    print(f"Session ID: {self.session_id}")
                    print(f"用户名: {self.user_info['username']}")
                    print(f"OpenID: {self.user_info['openid']}")
                    return True
                else:
                    print(f"游客登录失败: {result.get('error')}")
                    return False
            else:
                print(f"登录请求失败: HTTP {response.status_code}")
                return False
                
        except Exception as e:
            print(f"游客登录异常: {e}")
            return False
    
    async def connect_game_server(self):
        """连接游戏服务器（支持有无session_id两种模式）"""
        # 支持两种模式：有session_id和无session_id
        
        # 连接WebSocket
        ws_url = f"ws://localhost:18080/ws"
        print(f"连接游戏服务器: {ws_url}")
        print(f"Session ID: {self.session_id or '无'}")
        
        try:
            self.websocket = await websockets.connect(ws_url)
            print("WebSocket连接成功！")
            
            # 发送认证请求
            await self.send_auth_request()
            
            # 开始接收消息
            await self.message_loop()
            
        except Exception as e:
            print(f"连接游戏服务器失败: {e}")
            return False
    
    async def send_auth_request(self):
        """发送认证请求（使用AuthRequest的is_guest字段）"""
        print("发送游客认证请求...")
        
        # 构造认证消息 (简化版，实际应使用protobuf)
        # 使用AuthRequest的is_guest字段来标识游客登录
        auth_data = {
            "token": self.session_id or "",  # 游客可能有或没有token
            "protocol_version": "1.0",
            "client_version": "1.0.0",
            "device_type": "PC",
            "device_id": self.device_id,  # 关键信息：设备ID
            "app_id": self.app_id,
            "nonce": str(random.randint(1000000, 9999999)),
            "timestamp": int(time.time()),
            "signature": "guest_signature",
            "is_guest": True  # 使用新proto中的is_guest字段
        }
        
        message = {
            "clientId": f"guest_client_{self.device_id}",
            "msgSerialNo": 1,
            "id": 2,  # AUTH_REQUEST
            "data": auth_data
        }
        
        # 这里应该使用protobuf序列化，现在用JSON模拟
        json_data = json.dumps(message).encode('utf-8')
        
        # 发送消息长度 + 消息内容 (模拟二进制协议)
        length = struct.pack('<I', len(json_data))
        await self.websocket.send(length + json_data)
        
        print(f"游客认证请求已发送（is_guest=True, has_token={self.session_id is not None}）")
    
    async def message_loop(self):
        """消息处理循环"""
        print("开始接收服务器消息...")
        
        try:
            while True:
                # 接收消息
                message = await self.websocket.recv()
                await self.handle_message(message)
                
        except websockets.exceptions.ConnectionClosed:
            print("与服务器连接已断开")
        except Exception as e:
            print(f"消息处理异常: {e}")
    
    async def handle_message(self, message):
        """处理服务器消息"""
        try:
            # 解析消息长度
            if len(message) < 4:
                return
                
            length = struct.unpack('<I', message[:4])[0]
            json_data = message[4:4+length].decode('utf-8')
            msg = json.loads(json_data)
            
            msg_id = msg.get("id")
            
            if msg_id == 3:  # AUTH_RESPONSE
                await self.handle_auth_response(msg.get("data", {}))
            else:
                print(f"收到消息 ID: {msg_id}, 内容: {msg}")
                
        except Exception as e:
            print(f"解析消息失败: {e}")
    
    async def handle_auth_response(self, data):
        """处理认证响应"""
        ret = data.get("ret", 1)
        
        if ret == 0:  # 成功
            print("游戏服务器认证成功！")
            print(f"用户ID: {data.get('uid')}")
            print(f"昵称: {data.get('nickname')}")
            print(f"等级: {data.get('level')}")
            print(f"金币: {data.get('gold')}")
            print(f"钻石: {data.get('diamond')}")
            print(f"是否游客: {data.get('is_guest', '未知')}")
            
            # 认证成功后可以发送其他请求
            await self.test_game_features()
        else:
            print(f"游戏服务器认证失败: {data.get('error_msg', '未知错误')}")
    
    async def test_game_features(self):
        """测试游戏功能"""
        print("\n开始测试游戏功能...")
        
        # 等待一会
        await asyncio.sleep(1)
        
        # 获取用户信息
        await self.send_get_user_info()
        
        # 等待一会
        await asyncio.sleep(1)
        
        # 抽卡测试
        await self.send_draw_card()
    
    async def send_get_user_info(self):
        """发送获取用户信息请求"""
        print("发送获取用户信息请求...")
        
        message = {
            "clientId": f"guest_client_{self.device_id}",
            "msgSerialNo": 2,
            "id": 4,  # GET_USER_INFO_REQUEST
            "data": {
                "uid": 0  # 服务器会使用当前认证用户的ID
            }
        }
        
        json_data = json.dumps(message).encode('utf-8')
        length = struct.pack('<I', len(json_data))
        await self.websocket.send(length + json_data)
    
    async def send_draw_card(self):
        """发送抽卡请求"""
        print("发送抽卡请求...")
        
        message = {
            "clientId": f"guest_client_{self.device_id}",
            "msgSerialNo": 3,
            "id": 16,  # DRAW_CARD_REQUEST
            "data": {
                "uid": 0,  # 服务器会使用当前认证用户的ID
                "count": 1
            }
        }
        
        json_data = json.dumps(message).encode('utf-8')
        length = struct.pack('<I', len(json_data))
        await self.websocket.send(length + json_data)
    
    async def run(self):
        """运行游客登录测试"""
        print("=== 游客登录测试开始 ===")
        print(f"时间: {datetime.now()}")
        print(f"设备ID: {self.device_id}")
        print()
        
        # 步骤1: 游客登录
        if not await self.guest_login():
            print("游客登录失败，测试结束")
            return
        
        print()
        
        # 步骤2: 连接游戏服务器
        await self.connect_game_server()

async def main():
    """主函数"""
    client = GuestAuthClient()
    await client.run()

if __name__ == "__main__":
    print("游客登录客户端 (统一认证版)")
    print("确保以下服务正在运行:")
    print("- 登录服务器 (端口 8081)")
    print("- 游戏服务器 (端口 18080)")
    print("游客登录架构: 统一token认证 + is_guest参数区分")
    print()
    
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\n用户中断测试")
    except Exception as e:
        print(f"测试异常: {e}")