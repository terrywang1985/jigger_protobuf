#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
测试完整的认证流程：平台手机号登录 -> jigger登录服务器 -> 游戏服务器
"""

import requests
import json
import time
import sys
import os
import asyncio
import websockets
import struct
from typing import Optional

class JiggerAuthTestClient:
    def __init__(self):
        self.platform_credentials = None
        self.jigger_session = None
        self.ws: Optional[websockets.WebSocketServerProtocol] = None
        
        # 服务地址
        self.platform_api = "http://localhost:8080"
        self.jigger_login_api = "http://localhost:8081"
        self.gameserver_ws_url = "ws://localhost:18080/ws"
        self.app_id = "jigger_game"
        
    def load_platform_credentials(self):
        """加载平台认证凭据"""
        try:
            with open("platform_credentials.json", "r", encoding="utf-8-sig") as f:
                content = f.read().strip()
                if not content:
                    print("✗ 凭据文件为空")
                    return False
                self.platform_credentials = json.loads(content)
            
            print("✓ 成功加载平台凭据:")
            print(f"  手机号: {self.platform_credentials.get('phone', 'N/A')}")
            print(f"  OpenID: {self.platform_credentials.get('openid', 'N/A')}")
            token = self.platform_credentials.get('token', '')
            print(f"  Token: {token[:20] if token else 'N/A'}...")
            return True
            
        except FileNotFoundError:
            print("✗ 找不到 platform_credentials.json 文件")
            print("  请先运行 complete_auth_flow.py 获取平台认证凭据")
            return False
        except json.JSONDecodeError as e:
            print(f"✗ JSON解析错误: {e}")
            print("请检查 platform_credentials.json 文件格式")
            return False
        except Exception as e:
            print(f"✗ 加载平台凭据失败: {e}")
            return False
    
    
    def login_to_jigger(self):
        """登录到jigger登录服务器"""
        print(f"\n=== 登录到jigger登录服务器 ===")
        
        url = f"{self.jigger_login_api}/login"
        data = {
            "token": self.platform_credentials['token'],
            "app_id": self.app_id
        }
        
        try:
            response = requests.post(url, json=data, timeout=10)
            
            print(f"HTTP状态: {response.status_code}")
            print(f"响应内容: {response.text}")
            
            if response.status_code == 200:
                result = response.json()
                
                if result.get("success"):
                    self.jigger_session = result
                    print("✓ jigger登录成功!")
                    print(f"  SessionID: {result.get('session_id')}")
                    print(f"  用户名: {result.get('username')}")
                    print(f"  OpenID: {result.get('openid')}")
                    print(f"  网关地址: {result.get('gateway_url')}")
                    print(f"  过期时间: {result.get('expires_in')}秒")
                    return True
                else:
                    print(f"✗ jigger登录失败: {result.get('error')}")
                    return False
            else:
                print(f"✗ jigger登录HTTP错误: {response.status_code}")
                print(f"  错误内容: {response.text}")
                return False
                
        except Exception as e:
            print(f"✗ jigger登录异常: {e}")
            return False
    
    def save_jigger_session(self):
        """保存jigger会话信息"""
        if not self.jigger_session:
            print("✗ 没有jigger会话信息可保存")
            return False
        
        try:
            session_data = {
                "jigger_session": self.jigger_session,
                "platform_credentials": self.platform_credentials,
                "timestamp": int(time.time()),
                "app_id": self.app_id
            }
            
            with open("jigger_session.json", "w") as f:
                json.dump(session_data, f, indent=2)
            
            print("✓ jigger会话信息已保存到 jigger_session.json")
            return True
            
        except Exception as e:
            print(f"✗ 保存jigger会话失败: {e}")
            return False
    
    def test_gameserver_connection(self):
        """测试游戏服务器连接（简单的HTTP健康检查）"""
        print(f"\n=== 测试游戏服务器连接 ===")
        
        # 测试游戏服务器的HTTP健康检查端点
        gameserver_health_url = "http://localhost:18080/health"
        
        try:
            response = requests.get(gameserver_health_url, timeout=5)
            
            if response.status_code == 200:
                print("✓ 游戏服务器连接正常")
                return True
            else:
                print(f"✗ 游戏服务器健康检查失败: {response.status_code}")
                return False
                
        except Exception as e:
            print(f"ℹ 游戏服务器HTTP健康检查不可用: {e}")
            print("  这是正常的，游戏服务器可能只支持WebSocket")
            return True  # 不算失败，因为游戏服务器可能只支持WebSocket
    
    def pack_message(self, msg_type: int, data: bytes) -> bytes:
        """打包消息"""
        # 消息格式: [长度(4字节)] [类型(4字节)] [数据]
        length = 4 + len(data)  # 类型字段长度 + 数据长度
        return struct.pack('<II', length, msg_type) + data
    
    def unpack_message(self, data: bytes) -> tuple:
        """解包消息"""
        if len(data) < 8:
            return None, None
        
        length, msg_type = struct.unpack('<II', data[:8])
        message_data = data[8:8+length-4]  # 减去类型字段的4字节
        return msg_type, message_data
    
    async def send_message(self, msg_type: int, data: bytes):
        """发送消息到gameserver"""
        if not self.ws:
            print("✗ WebSocket连接未建立")
            return False
        
        try:
            message = self.pack_message(msg_type, data)
            await self.ws.send(message)
            print(f"✓ 发送消息: 类型={msg_type}, 长度={len(data)}")
            return True
        except Exception as e:
            print(f"✗ 发送消息失败: {e}")
            return False
    
    async def receive_message(self, timeout=10):
        """接收gameserver消息"""
        if not self.ws:
            print("✗ WebSocket连接未建立")
            return None, None
        
        try:
            data = await asyncio.wait_for(self.ws.recv(), timeout=timeout)
            msg_type, message_data = self.unpack_message(data)
            print(f"✓ 接收消息: 类型={msg_type}, 长度={len(message_data) if message_data else 0}")
            return msg_type, message_data
        except asyncio.TimeoutError:
            print(f"✗ 接收消息超时 ({timeout}秒)")
            return None, None
        except Exception as e:
            print(f"✗ 接收消息失败: {e}")
            return None, None
    
    async def send_auth_request(self, session_id: str):
        """发送认证请求到gameserver"""
        # 认证消息格式 (简化的JSON格式)
        auth_data = {
            "session_id": session_id,
            "openid": self.platform_credentials.get('openid'),
            "username": self.platform_credentials.get('username'),
            "app_id": self.app_id
        }
        
        auth_json = json.dumps(auth_data).encode('utf-8')
        
        # 假设认证消息类型为 1
        AUTH_MESSAGE_TYPE = 1
        
        print(f"发送认证请求: {auth_data}")
        success = await self.send_message(AUTH_MESSAGE_TYPE, auth_json)
        
        if success:
            # 等待认证响应
            msg_type, response_data = await self.receive_message(timeout=15)
            
            if msg_type is not None:
                try:
                    response = json.loads(response_data.decode('utf-8'))
                    print(f"认证响应: {response}")
                    
                    if response.get('success') or response.get('status') == 'success':
                        print("✓ gameserver认证成功！")
                        return True
                    else:
                        print(f"✗ gameserver认证失败: {response.get('message', '未知错误')}")
                        return False
                except json.JSONDecodeError:
                    print(f"✗ 认证响应JSON解析失败: {response_data}")
                    return False
            else:
                print("✗ 未收到gameserver认证响应")
                return False
        else:
            return False
    
    async def connect_to_gameserver(self):
        """连接到gameserver并进行session校验"""
        if not self.jigger_session:
            print("✗ 没有jigger会话信息")
            return False
        
        session_id = self.jigger_session.get('session_id')
        if not session_id:
            print("✗ 没有找到session_id")
            return False
        
        print(f"连接到gameserver: {self.gameserver_ws_url}")
        print(f"使用session_id: {session_id}")
        
        try:
            # 建立WebSocket连接
            self.ws = await websockets.connect(self.gameserver_ws_url)
            print("✓ WebSocket连接建立成功")
            
            # 发送认证请求
            auth_success = await self.send_auth_request(session_id)
            
            if auth_success:
                # 可以进行一些基本的游戏通信测试
                await self.test_game_communication()
            
            # 关闭连接
            await self.ws.close()
            print("✓ WebSocket连接已关闭")
            
            return auth_success
            
        except ConnectionRefusedError:
            print(f"✗ 无法连接到gameserver: {self.gameserver_ws_url}")
            print("  请确认gameserver正在运行")
            return False
        except Exception as e:
            print(f"✗ gameserver连接异常: {e}")
            return False
    
    async def test_game_communication(self):
        """测试基本的游戏通信"""
        print("\n--- 测试游戏通信 ---")
        
        # 发送一个简单的心跳或状态查询消息
        HEARTBEAT_MESSAGE_TYPE = 2
        heartbeat_data = json.dumps({"type": "heartbeat", "timestamp": int(time.time())}).encode('utf-8')
        
        print("发送心跳消息...")
        success = await self.send_message(HEARTBEAT_MESSAGE_TYPE, heartbeat_data)
        
        if success:
            # 等待心跳响应
            msg_type, response_data = await self.receive_message(timeout=10)
            
            if msg_type is not None:
                try:
                    response = json.loads(response_data.decode('utf-8'))
                    print(f"心跳响应: {response}")
                except json.JSONDecodeError:
                    print(f"心跳响应(原始): {response_data}")
            else:
                print("未收到心跳响应")
        
        print("--- 游戏通信测试完成 ---")
    
    def run_complete_test(self):
        """运行完整的认证测试流程"""
        print("=== Jigger认证完整测试流程 ===")
        print(f"时间: {time.strftime('%Y-%m-%d %H:%M:%S')}")
        
        # 1. 加载平台凭据
        if not self.load_platform_credentials():
            return False
        
        
        # 3. 登录到jigger
        if not self.login_to_jigger():
            return False
        
        # 4. 保存jigger会话
        if not self.save_jigger_session():
            return False
        
        # 5. 测试游戏服务器连接
        if not self.test_gameserver_connection():
            return False
        
        # 6. 连接gameserver进行session校验
        print(f"\n=== 连接gameserver进行session校验 ===")
        try:
            success = asyncio.run(self.connect_to_gameserver())
            if not success:
                return False
        except Exception as e:
            print(f"✗ gameserver连接异常: {e}")
            return False
        
        print(f"\n🎉 认证测试流程全部完成！")
        print(f"✓ 平台认证: 成功")
        print(f"✓ Jigger登录: 成功")
        print(f"✓ 会话保存: 成功")
        print(f"✓ 游戏服务器: 可访问")
        print(f"✓ Gameserver认证: 成功")
        
        print(f"\n📝 下一步:")
        print(f"  1. 已完成完整认证链路验证")
        print(f"  2. 可以开始正式的游戏通信")
        print(f"  3. 使用 simple_test_client.py 进行更多游戏功能测试")
        
        return True

def main():
    """主函数"""
    client = JiggerAuthTestClient()
    
    # 检查当前目录
    if not os.path.exists("platform_credentials.json"):
        print("✗ 找不到 platform_credentials.json 文件")
        print("  请先运行 complete_auth_flow.py 获取平台认证凭据")
        return
    
    # 运行完整测试
    success = client.run_complete_test()
    
    if success:
        print("\\n🚀 所有测试通过！可以开始游戏通信了。")
    else:
        print("\\n❌ 测试失败，请检查错误信息。")
        sys.exit(1)

if __name__ == "__main__":
    main()