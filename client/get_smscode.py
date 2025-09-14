#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
获取短信验证码的客户端
通过调用platform API获取验证码，并从auth-service日志中提取真实验证码
"""

import requests
import json
import time
import subprocess
import re
import threading
import queue
import sys

class SMSCodeGetter:
    def __init__(self):
        self.api_gateway_url = "http://localhost:8080"
        self.phone_number = "13800138000"  # 测试手机号
        self.verification_code = None
        self.code_queue = queue.Queue()
        
    def send_sms_code(self):
        """发送短信验证码请求"""
        url = f"{self.api_gateway_url}/auth/send-sms-code"
        data = {
            "phone": self.phone_number
        }
        
        print(f"正在向 {self.phone_number} 发送验证码...")
        
        try:
            response = requests.post(url, json=data, timeout=10)
            print(f"HTTP状态码: {response.status_code}")
            print(f"响应内容: {response.text}")
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    print("✓ 验证码发送成功！")
                    return True
                else:
                    print(f"✗ 验证码发送失败: {result.get('message', '未知错误')}")
                    return False
            else:
                print(f"✗ HTTP请求失败: {response.status_code}")
                return False
                
        except requests.exceptions.RequestException as e:
            print(f"✗ 请求异常: {e}")
            return False
    
    def monitor_auth_logs(self, duration=30):
        """监控auth-service日志，提取验证码"""
        print(f"开始监控auth-service日志，持续{duration}秒...")
        
        def log_monitor():
            try:
                # 启动docker logs命令
                process = subprocess.Popen(
                    ["docker", "logs", "-f", "platform-auth-service-1"],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    bufsize=1,
                    universal_newlines=True
                )
                
                start_time = time.time()
                
                while time.time() - start_time < duration:
                    line = process.stdout.readline()
                    if line:
                        print(f"[LOG] {line.strip()}")
                        
                        # 查找验证码模式
                        # 可能的模式: "验证码: 123456", "code: 123456", "sms code: 123456" 等
                        patterns = [
                            r'验证码[：:]\s*(\d{4,6})',
                            r'code[：:]\s*(\d{4,6})',
                            r'sms\s*code[：:]\s*(\d{4,6})',
                            r'verification\s*code[：:]\s*(\d{4,6})',
                            r'"code"\s*[:：]\s*"?(\d{4,6})"?',
                            r'短信验证码[：:]\s*(\d{4,6})',
                            r'SMS.*?(\d{4,6})',
                        ]
                        
                        for pattern in patterns:
                            match = re.search(pattern, line, re.IGNORECASE)
                            if match:
                                code = match.group(1)
                                print(f"🎉 找到验证码: {code}")
                                self.code_queue.put(code)
                                process.terminate()
                                return
                
                process.terminate()
                print("⚠️ 监控超时，未找到验证码")
                
            except Exception as e:
                print(f"✗ 日志监控异常: {e}")
        
        # 启动日志监控线程
        monitor_thread = threading.Thread(target=log_monitor)
        monitor_thread.daemon = True
        monitor_thread.start()
        
        # 等待验证码或超时
        try:
            code = self.code_queue.get(timeout=duration + 5)
            self.verification_code = code
            return code
        except queue.Empty:
            print("⚠️ 未在规定时间内获取到验证码")
            return None
    
    def get_verification_code(self):
        """完整的获取验证码流程"""
        print("=" * 50)
        print("开始获取短信验证码")
        print("=" * 50)
        
        # 1. 发送验证码请求
        if not self.send_sms_code():
            return None
        
        # 2. 监控日志获取验证码
        print("\n等待2秒后开始监控日志...")
        time.sleep(2)
        
        code = self.monitor_auth_logs(30)
        
        if code:
            print(f"\n✓ 成功获取验证码: {code}")
            print(f"手机号: {self.phone_number}")
            return code
        else:
            print("\n✗ 未能获取验证码")
            return None

def main():
    """主函数"""
    getter = SMSCodeGetter()
    
    print("短信验证码获取工具")
    print("-" * 30)
    print(f"目标手机号: {getter.phone_number}")
    print(f"API网关: {getter.api_gateway_url}")
    print()
    
    # 检查服务状态
    try:
        response = requests.get(f"{getter.api_gateway_url}/health", timeout=5)
        if response.status_code == 200:
            print("✓ API网关连接正常")
        else:
            print("⚠️ API网关状态异常")
    except:
        print("✗ 无法连接到API网关，请确认服务是否运行")
        return
    
    # 获取验证码
    code = getter.get_verification_code()
    
    if code:
        # 保存验证码到文件
        with open("verification_code.txt", "w") as f:
            f.write(f"phone: {getter.phone_number}\n")
            f.write(f"code: {code}\n")
            f.write(f"timestamp: {int(time.time())}\n")
        
        print(f"\n验证码已保存到 verification_code.txt 文件")
        
        # 输出给其他脚本使用
        print(f"\n--- 输出信息 ---")
        print(f"PHONE: {getter.phone_number}")
        print(f"CODE: {code}")
    else:
        print("\n获取验证码失败！")
        sys.exit(1)

if __name__ == "__main__":
    main()