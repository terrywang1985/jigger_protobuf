#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
桌面宠物游客登录启动脚本
功能：
1. 游客登录到服务器
2. 启动桌面宠物
3. 支持多玩家聊天室
4. 玩家之间可以互相看到
"""

import sys
import os
import tkinter as tk
from tkinter import messagebox
import threading
import time

# 添加当前目录到Python路径
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

# 导入修改后的client11模块
from client11 import JiggerClient, AuthManager, resource_path

class DesktopPetGuestApp:
    def __init__(self):
        self.root = tk.Tk()
        self.root.title("桌面宠物 - 游客登录")
        self.root.geometry("400x300")
        self.root.configure(bg="#2c3e50")
        
        self.auth = AuthManager()
        self.client = None
        
        self.setup_ui()
        self.center_window()
    
    def setup_ui(self):
        """设置用户界面"""
        # 标题
        title_label = tk.Label(
            self.root, 
            text="桌面宠物", 
            font=('Arial', 20, 'bold'),
            bg="#2c3e50", 
            fg="white"
        )
        title_label.pack(pady=20)
        
        # 说明文本
        info_label = tk.Label(
            self.root,
            text="支持游客登录的桌面宠物\n玩家之间可以互相看到并聊天",
            font=('Arial', 12),
            bg="#2c3e50",
            fg="#ecf0f1",
            justify=tk.CENTER
        )
        info_label.pack(pady=10)
        
        # 设备ID显示
        device_frame = tk.Frame(self.root, bg="#2c3e50")
        device_frame.pack(pady=10)
        
        tk.Label(
            device_frame,
            text=f"设备ID: {self.auth.device_id}",
            font=('Arial', 10),
            bg="#2c3e50",
            fg="#bdc3c7"
        ).pack()
        
        # 按钮框架
        button_frame = tk.Frame(self.root, bg="#2c3e50")
        button_frame.pack(pady=30)
        
        # 游客登录按钮
        guest_btn = tk.Button(
            button_frame,
            text="游客登录",
            command=self.guest_login,
            font=('Arial', 14),
            bg="#3498db",
            fg="white",
            width=15,
            height=2,
            cursor="hand2"
        )
        guest_btn.pack(pady=10)
        
        # 手机号登录按钮
        phone_btn = tk.Button(
            button_frame,
            text="手机号登录",
            command=self.phone_login,
            font=('Arial', 14),
            bg="#2ecc71",
            fg="white",
            width=15,
            height=2,
            cursor="hand2"
        )
        phone_btn.pack(pady=10)
        
        # 状态标签
        self.status_label = tk.Label(
            self.root,
            text="",
            font=('Arial', 10),
            bg="#2c3e50",
            fg="#e74c3c"
        )
        self.status_label.pack(pady=10)
    
    def center_window(self):
        """居中窗口"""
        self.root.update_idletasks()
        width = self.root.winfo_width()
        height = self.root.winfo_height()
        x = (self.root.winfo_screenwidth() // 2) - (width // 2)
        y = (self.root.winfo_screenheight() // 2) - (height // 2)
        self.root.geometry(f'{width}x{height}+{x}+{y}')
    
    def guest_login(self):
        """游客登录"""
        self.status_label.config(text="正在游客登录...", fg="#f39c12")
        self.root.update()
        
        # 在后台线程中执行登录
        threading.Thread(target=self._do_guest_login, daemon=True).start()
    
    def _do_guest_login(self):
        """执行游客登录"""
        try:
            success, message, result = self.auth.guest_login()
            
            if success:
                self.root.after(0, lambda: self._on_login_success("游客"))
            else:
                self.root.after(0, lambda: self._on_login_failed(message))
                
        except Exception as e:
            self.root.after(0, lambda: self._on_login_failed(f"登录异常: {e}"))
    
    def phone_login(self):
        """手机号登录（简化版）"""
        messagebox.showinfo(
            "提示", 
            "手机号登录功能请使用主程序中的HomePage\\n这里主要演示游客登录功能"
        )
    
    def _on_login_success(self, login_type):
        """登录成功回调"""
        self.status_label.config(text=f"{login_type}登录成功!", fg="#2ecc71")
        
        # 显示登录信息
        messagebox.showinfo(
            "登录成功",
            f"{login_type}登录成功！\\n"
            f"OpenID: {self.auth.openid}\\n"
            f"设备ID: {self.auth.device_id}\\n\\n"
            f"即将启动桌面宠物..."
        )
        
        # 启动桌面宠物
        self.start_desktop_pet()
    
    def _on_login_failed(self, error_message):
        """登录失败回调"""
        self.status_label.config(text=f"登录失败: {error_message}", fg="#e74c3c")
        messagebox.showerror("登录失败", error_message)
    
    def start_desktop_pet(self):
        """启动桌面宠物"""
        try:
            self.root.withdraw()  # 隐藏登录窗口
            
            # 创建桌面宠物客户端
            sprite_path = resource_path("spritesheet.png")
            if not os.path.exists(sprite_path):
                # 如果没有spritesheet.png，创建一个简单的占位文件提示
                messagebox.showwarning(
                    "提示", 
                    f"未找到spritesheet.png文件\\n"
                    f"请将宠物精灵图放在: {sprite_path}\\n"
                    f"程序将使用默认外观运行"
                )
                # 可以创建一个简单的默认图片
                sprite_path = None
            
            # 创建并启动桌面宠物客户端
            self.client = JiggerClient(sprite_path or "default")
            self.client.auth = self.auth  # 传递认证信息
            
            # 自动连接WebSocket
            if self.auth.token:
                self.client.connect_websocket()
            
            print("桌面宠物启动成功!")
            print("使用说明:")
            print("- 右键宠物可打开菜单")
            print("- 拖动宠物可移动位置")
            print("- 底部输入框可发送聊天消息")
            print("- 鼠标和键盘操作会触发动作同步")
            
        except Exception as e:
            messagebox.showerror("启动失败", f"启动桌面宠物失败: {e}")
            self.root.deiconify()  # 显示登录窗口
    
    def run(self):
        """运行应用"""
        try:
            self.root.mainloop()
        except KeyboardInterrupt:
            print("\\n用户中断程序")
        except Exception as e:
            print(f"程序运行异常: {e}")

def main():
    """主函数"""
    print("桌面宠物游客登录版本")
    print("=" * 40)
    print("功能特性:")
    print("✓ 游客快速登录")
    print("✓ 多玩家在线互动")
    print("✓ 实时聊天功能")
    print("✓ 动作同步显示")
    print("✓ 拖拽移动宠物")
    print()
    
    # 检查是否有必要的依赖
    required_modules = ['websockets', 'requests', 'PIL', 'pynput']
    missing_modules = []
    
    for module in required_modules:
        try:
            __import__(module)
        except ImportError:
            missing_modules.append(module)
    
    if missing_modules:
        print("缺少必要的依赖模块:")
        for module in missing_modules:
            print(f"  - {module}")
        print("\\n请安装缺少的模块:")
        print(f"pip install {' '.join(missing_modules)}")
        return
    
    # 启动应用
    app = DesktopPetGuestApp()
    app.run()

if __name__ == "__main__":
    main()