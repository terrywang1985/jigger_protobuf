#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
完整的平台认证流程脚本
包含: 获取验证码 -> 注册/登录 -> 保存凭据 -> 测试游戏服认证
"""

import requests
import json
import time
import subprocess
import re
import threading
import queue
import sys
import os
from pathlib import Path

class PlatformAuthFlow:
    def __init__(self):
        self.api_gateway_url = "http://localhost:8080"
        self.phone_number = "13800138000"  # 测试手机号
        self.verification_code = None
        self.token = None
        self.openid = None
        self.user_id = None
        self.credentials_file = "platform_credentials.json"
        
    def log(self, message, level="INFO"):
        """日志输出"""
        timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
        print(f"[{timestamp}] [{level}] {message}")
    
    def send_sms_code(self):
        """发送短信验证码"""
        url = f"{self.api_gateway_url}/auth/phone/send-code"
        data = {
            "country_code": "+86",
            "phone": self.phone_number,
            "app_id": "jigger_game"
        }
        
        self.log(f"向 {self.phone_number} 发送验证码...")
        
        try:
            response = requests.post(url, json=data, timeout=10)
            self.log(f"HTTP状态: {response.status_code}, 响应: {response.text}")
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    self.log("验证码发送成功！", "SUCCESS")
                    return True
                else:
                    self.log(f"发送失败: {result.get('message', '未知错误')}", "ERROR")
                    return False
            else:
                self.log(f"HTTP请求失败: {response.status_code}", "ERROR")
                return False
                
        except Exception as e:
            self.log(f"请求异常: {e}", "ERROR")
            return False
    
    def extract_code_from_logs(self, duration=30):
        """从auth-service日志中提取验证码"""
        self.log(f"监控auth-service日志 {duration}秒...")
        
        code_queue = queue.Queue()
        
        def monitor_logs():
            try:
                process = subprocess.Popen(
                    ["docker", "logs", "-f", "platform-auth-service-1"],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    bufsize=1
                )
                
                start_time = time.time()
                
                while time.time() - start_time < duration:
                    line = process.stdout.readline()
                    if line:
                        # 输出日志内容便于调试
                        if "code" in line.lower() or "验证码" in line:
                            self.log(f"[AUTH-LOG] {line.strip()}")
                        
                        # 尝试多种验证码模式
                        patterns = [
                            r'验证码[：:]\s*(\d{4,6})',
                            r'"code"\s*[：:]\s*"?(\d{4,6})"?',
                            r'code[：:]\s*(\d{4,6})',
                            r'SMS.*?(\d{4,6})',
                            r'verification.*?(\d{4,6})',
                            r'短信.*?(\d{4,6})',
                        ]
                        
                        for pattern in patterns:
                            match = re.search(pattern, line, re.IGNORECASE)
                            if match:
                                code = match.group(1)
                                self.log(f"提取到验证码: {code}", "SUCCESS")
                                code_queue.put(code)
                                process.terminate()
                                return
                
                process.terminate()
                self.log("监控超时，未找到验证码", "WARN")
                
            except Exception as e:
                self.log(f"日志监控异常: {e}", "ERROR")
        
        # 启动监控线程
        monitor_thread = threading.Thread(target=monitor_logs)
        monitor_thread.daemon = True
        monitor_thread.start()
        
        # 等待结果
        try:
            code = code_queue.get(timeout=duration + 5)
            self.verification_code = code
            return code
        except queue.Empty:
            self.log("未在规定时间内获取验证码", "ERROR")
            return None
    
    def get_verification_code(self):
        """获取验证码的完整流程"""
        self.log("开始获取验证码流程", "INFO")
        
        if not self.send_sms_code():
            return False
        
        time.sleep(2)  # 等待日志写入
        code = self.extract_code_from_logs(30)
        
        if code:
            self.log(f"验证码获取成功: {code}", "SUCCESS")
            return True
        else:
            self.log("验证码获取失败", "ERROR")
            return False
    
    def register_or_login(self):
        """注册或登录"""
        if not self.verification_code:
            self.log("没有验证码，无法进行注册/登录", "ERROR")
            return False
        
        # 先尝试登录
        if self.try_login():
            return True
        
        # 登录失败则尝试注册
        if self.try_register():
            # 注册成功后再次登录
            return self.try_login()
        
        return False
    
    def try_register(self):
        """尝试注册(手机号登录会自动注册)"""
        # 手机号登录API会自动创建用户，无需单独注册
        self.log("手机号登录会自动注册用户", "INFO")
        return False  # 直接返回False，让流程去尝试登录
        
        self.log("尝试注册...")
        
        try:
            response = requests.post(url, json=data, timeout=10)
            self.log(f"注册响应: {response.status_code} - {response.text}")
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    self.log("注册成功！", "SUCCESS")
                    return True
                else:
                    self.log(f"注册失败: {result.get('message')}", "WARN")
                    return False
            else:
                self.log(f"注册HTTP错误: {response.status_code}", "ERROR")
                return False
                
        except Exception as e:
            self.log(f"注册异常: {e}", "ERROR")
            return False
    
    def try_login(self):
        """尝试登录"""
        url = f"{self.api_gateway_url}/auth/phone/login"
        data = {
            "country_code": "+86",
            "phone": self.phone_number,
            "code": self.verification_code,
            "app_id": "jigger_game"
        }
        
        self.log("尝试登录...")
        
        try:
            response = requests.post(url, json=data, timeout=10)
            self.log(f"登录响应: {response.status_code} - {response.text}")
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success") and result.get("data"):
                    data = result["data"]
                    self.token = data.get("token")
                    self.openid = data.get("openid") or data.get("user_id")
                    self.user_id = data.get("user_id")
                    
                    self.log("登录成功！", "SUCCESS")
                    self.log(f"Token: {self.token[:20]}..." if self.token else "Token: None")
                    self.log(f"OpenID: {self.openid}")
                    self.log(f"UserID: {self.user_id}")
                    return True
                else:
                    self.log(f"登录失败: {result.get('message')}", "ERROR")
                    return False
            else:
                self.log(f"登录HTTP错误: {response.status_code}", "ERROR")
                return False
                
        except Exception as e:
            self.log(f"登录异常: {e}", "ERROR")
            return False
    
    def save_credentials(self):
        """保存认证凭据到本地文件"""
        if not self.token or not self.openid:
            self.log("没有有效的认证凭据可保存", "ERROR")
            return False
        
        credentials = {
            "phone": self.phone_number,
            "token": self.token,
            "openid": self.openid,
            "user_id": self.user_id,
            "timestamp": int(time.time()),
            "expires_at": int(time.time()) + 3600 * 24,  # 假设24小时过期
            "api_gateway": self.api_gateway_url
        }
        
        try:
            with open(self.credentials_file, "w", encoding="utf-8") as f:
                json.dump(credentials, f, indent=2, ensure_ascii=False)
            
            self.log(f"认证凭据已保存到: {self.credentials_file}", "SUCCESS")
            return True
            
        except Exception as e:
            self.log(f"保存凭据失败: {e}", "ERROR")
            return False
    
    def load_credentials(self):
        """从本地文件加载认证凭据"""
        if not os.path.exists(self.credentials_file):
            return False
        
        try:
            with open(self.credentials_file, "r", encoding="utf-8") as f:
                credentials = json.load(f)
            
            # 检查是否过期
            if credentials.get("expires_at", 0) < time.time():
                self.log("本地凭据已过期", "WARN")
                return False
            
            self.token = credentials.get("token")
            self.openid = credentials.get("openid")
            self.user_id = credentials.get("user_id")
            self.phone_number = credentials.get("phone", self.phone_number)
            
            self.log("成功加载本地凭据", "SUCCESS")
            return True
            
        except Exception as e:
            self.log(f"加载凭据失败: {e}", "ERROR")
            return False
    
    def test_jigger_login(self):
        """测试jigger登录服务器认证"""
        if not self.token:
            self.log("没有token，无法测试jigger认证", "ERROR")
            return False
        
        # 这里调用simple_test_client.py或实现gRPC调用
        self.log("准备测试jigger登录服务器...")
        
        # 为了简化，创建一个临时的测试脚本调用
        test_script = f"""
import sys
sys.path.append('.')
import grpc
import proto.login_pb2 as login_pb2
import proto.login_pb2_grpc as login_pb2_grpc

def test_login():
    channel = grpc.insecure_channel('localhost:8081')
    stub = login_pb2_grpc.LoginServiceStub(channel)
    
    request = login_pb2.LoginRequest(
        token="{self.token}",
        openid="{self.openid}"
    )
    
    try:
        response = stub.Login(request)
        print(f"Login Response: {{response}}")
        return True
    except Exception as e:
        print(f"Login Error: {{e}}")
        return False

if __name__ == "__main__":
    test_login()
"""
        
        # 写入临时文件
        temp_file = "temp_jigger_test.py"
        try:
            with open(temp_file, "w") as f:
                f.write(test_script)
            
            # 执行测试
            result = subprocess.run([sys.executable, temp_file], 
                                  capture_output=True, text=True, timeout=30)
            
            self.log(f"Jigger测试输出: {result.stdout}")
            if result.stderr:
                self.log(f"Jigger测试错误: {result.stderr}")
            
            # 清理临时文件
            os.remove(temp_file)
            
            return result.returncode == 0
            
        except Exception as e:
            self.log(f"Jigger测试异常: {e}", "ERROR")
            return False
    
    def run_complete_flow(self):
        """运行完整的认证流程"""
        self.log("=" * 60)
        self.log("开始完整的平台认证流程")
        self.log("=" * 60)
        
        # 1. 尝试加载现有凭据
        if self.load_credentials():
            self.log("使用现有凭据，跳过认证步骤")
        else:
            # 2. 获取验证码
            if not self.get_verification_code():
                self.log("获取验证码失败，流程终止", "ERROR")
                return False
            
            # 3. 注册或登录
            if not self.register_or_login():
                self.log("注册/登录失败，流程终止", "ERROR")
                return False
            
            # 4. 保存凭据
            if not self.save_credentials():
                self.log("保存凭据失败", "WARN")
        
        # 5. 测试jigger认证
        self.log("\n开始测试jigger服务器认证...")
        if self.test_jigger_login():
            self.log("jigger认证测试成功！", "SUCCESS")
        else:
            self.log("jigger认证测试失败", "ERROR")
        
        self.log("\n认证流程完成！")
        self.log(f"Token: {self.token}")
        self.log(f"OpenID: {self.openid}")
        self.log(f"凭据文件: {self.credentials_file}")
        
        return True

def main():
    """主函数"""
    print("平台认证完整流程")
    print("=" * 40)
    
    flow = PlatformAuthFlow()
    
    # 检查服务状态
    try:
        response = requests.get(f"{flow.api_gateway_url}/health", timeout=5)
        print(f"✓ API网关状态正常: {response.status_code}")
    except:
        print("✗ 无法连接API网关，请确认服务运行状态")
        return
    
    # 运行完整流程
    success = flow.run_complete_flow()
    
    if success:
        print("\n🎉 认证流程成功完成！")
    else:
        print("\n❌ 认证流程失败！")
        sys.exit(1)

if __name__ == "__main__":
    main()