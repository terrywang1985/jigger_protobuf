import tkinter as tk
from tkinter import ttk, messagebox
import PIL
import PIL.Image
import PIL.ImageTk
from PIL import Image, ImageTk
import asyncio
import websockets
import threading
import queue
import json
import time
import sys, os
from pynput import mouse, keyboard
import requests
import uuid
import base64
from io import BytesIO
import logging
import struct

# 清理代理环境变量，确保本地连接不走代理
def clear_proxy_env():
    """清理代理环境变量以确保本地连接"""
    proxy_vars = [
        'HTTP_PROXY', 'HTTPS_PROXY', 'FTP_PROXY', 'SOCKS_PROXY',
        'http_proxy', 'https_proxy', 'ftp_proxy', 'socks_proxy',
        'ALL_PROXY', 'all_proxy'
    ]
    
    cleared_vars = []
    for var in proxy_vars:
        if var in os.environ:
            old_value = os.environ[var]
            del os.environ[var]
            cleared_vars.append(f"{var}={old_value}")
    
    if cleared_vars:
        print(f"[INFO] 已清理代理环境变量: {', '.join(cleared_vars)}")
        print("[INFO] 这确保了本地服务器连接不会被代理")
    else:
        print("[INFO] 未检测到代理环境变量")

# 在模块加载时立即清理代理设置
clear_proxy_env()

# 导入protobuf生成的类
import game_pb2 as game_pb
import desktop_pet_pb2 as desktop_pet_pb
import battle_pb2 as battle_pb

# 平台API地址
PLATFORM_API = "http://localhost:8080"

# 配置日志
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

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
    
    async def send_raw_message(self, msg_id, raw_data):
        """发送原始数据消息（用于发送JSON或其他格式的数据）"""
        if not self.ws:
            return
            
        try:
            # 直接使用原始数据
            data = raw_data if isinstance(raw_data, bytes) else raw_data.encode('utf-8')
            
            # 创建消息
            message = self.create_message(msg_id, data)
            
            # 打包并发送
            packed_data = self.pack_message(message)
            await self.ws.send(packed_data)
            
            print(f"Sent raw message: {msg_id}, data length: {len(data)}")
            
        except Exception as e:
            print(f"Failed to send raw message: {e}")
    
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
            print(f"Failed to parse message: {e}")
            return None, remaining_data
class AuthManager:
    def __init__(self):
        self.token = None
        self.openid = None
        self.app_id = "desktop_app"
        self.device_id = str(uuid.uuid4())[:16]  # 生成设备ID用于游客登录
        self.is_guest = False
        self.session_id = None
    
    def send_sms_code(self, country_code, phone, device_id=""):
        url = f"{PLATFORM_API}/auth/phone/send-code"
        data = {
            "country_code": country_code,
            "phone": phone,
            "app_id": self.app_id,
            "device_id": device_id
        }
        
        try:
            print(f"Sending SMS code to {country_code}{phone}, {url}")
            # 显式禁用代理以确保本地连接
            response = requests.post(url, json=data, proxies={"http": None, "https": None})
            if response.status_code == 200:
                return True, "验证码已发送"
            else:
                error_msg = response.json().get("error", "发送验证码失败")
                return False, error_msg
        except Exception as e:
            return False, f"网络错误: {str(e)}"
    
    def phone_login(self, country_code, phone, code, device_id=""):
        url = f"{PLATFORM_API}/auth/phone/login"
        data = {
            "country_code": country_code,
            "phone": phone,
            "code": code,
            "app_id": self.app_id,
            "device_id": device_id
        }
        
        try:
            # 显式禁用代理以确保本地连接
            response = requests.post(url, json=data, proxies={"http": None, "https": None})
            if response.status_code == 200:
                result = response.json()
                self.token = result.get("token")
                self.openid = result.get("openid")
                return True, "登录成功", result
            else:
                error_msg = response.json().get("error", "登录失败")
                return False, error_msg, None
        except Exception as e:
            return False, f"网络错误: {str(e)}", None
    
    def guest_login(self):
        """游客登录"""
        url = "http://localhost:8081/login"  # 登录服务器地址
        data = {
            "device_id": self.device_id,
            "app_id": self.app_id,
            "is_guest": True
        }
        
        try:
            print(f"游客登录中... 设备ID: {self.device_id}")
            print(f"请求数据: {data}")
            # 显式禁用代理以确保本地连接
            response = requests.post(url, json=data, timeout=10, proxies={"http": None, "https": None})
            
            print(f"HTTP状态码: {response.status_code}")
            print(f"响应内容: {response.text}")
            
            if response.status_code == 200:
                result = response.json()
                if result.get("success"):
                    self.session_id = result.get("session_id")
                    self.token = self.session_id  # 使用session_id作为token
                    self.openid = result.get("openid")
                    self.is_guest = True
                    print(f"游客登录成功！用户名: {result.get('username')}")
                    return True, "游客登录成功", result
                else:
                    error_msg = result.get("error", "游客登录失败")
                    return False, error_msg, None
            else:
                error_msg = f"HTTP {response.status_code}: {response.text}"
                return False, error_msg, None
                
        except Exception as e:
            print(f"请求异常: {str(e)}")
            return False, f"网络错误: {str(e)}", None

def resource_path(filename):
    if getattr(sys, "frozen", False):
        if hasattr(sys, "_MEIPASS"):
            return os.path.join(sys._MEIPASS, filename)
        return os.path.join(os.path.dirname(sys.executable), filename)
    return os.path.join(os.path.dirname(__file__), filename)

class DesktopPet:
    def __init__(self, root, sprite_path, player_id, client=None, is_self=True):
        self.root = root
        self.client = client  # 修改为传入client对象
        self.player_id = player_id
        self.is_self = is_self

        self.root.overrideredirect(True)
        self.root.attributes("-topmost", True)
        self.root.attributes("-transparentcolor", "white")

        # 加载图片
        self.current_skin = None
        self.idle_sprites = self.load_sprites(sprite_path, 3, scale=0.5)
        self.action_sprites = list(self.idle_sprites[::-1])
        self.current_image = self.idle_sprites[0]

        self.label = tk.Label(root, image=self.current_image, bg="white")
        self.label.pack()
        self.root.geometry(f"{self.idle_sprites[0].width()}x{self.idle_sprites[0].height()}")

        self.frame = 0
        self.events = []  # 最近1秒的动作事件
        self.chat_text = None
        self.chat_label = None

        # 拖动支持
        self.label.bind("<Button-1>", self.start_move)
        self.label.bind("<B1-Motion>", self.on_move)

        if is_self:
            self.root.bind("<Button-3>", self.show_menu)
            self.chat_entry = tk.Entry(self.root)
            self.chat_entry.pack(side=tk.BOTTOM, fill=tk.X)
            self.chat_entry.bind("<Return>", self.send_chat)
            self.start_listeners()
        else:
            self.chat_entry = None

        self.animate()

    def load_sprites(self, path, frame_count, scale=1.0):
        sheet = Image.open(path)
        w, h = sheet.size
        frame_width = w // frame_count
        frames = []
        for i in range(frame_count):
            frame = sheet.crop((i*frame_width,0,(i+1)*frame_width,h))
            if scale != 1.0:
                frame = frame.resize((int(frame_width*scale), int(h*scale)), Image.Resampling.LANCZOS)
            frames.append(ImageTk.PhotoImage(frame))
        return frames

    def update_skin(self, skin_image):
        """更新皮肤"""
        if not skin_image:
            return
            
        try:
            # 将PIL图像转换为PhotoImage
            photo_image = ImageTk.PhotoImage(skin_image)
            
            # 更新精灵列表
            self.idle_sprites = [photo_image] * 3
            self.action_sprites = list(self.idle_sprites[::-1])
            self.current_image = self.idle_sprites[0]
            self.label.config(image=self.current_image)
            
            # 保存引用防止垃圾回收
            self.current_skin = photo_image
            
        except Exception as e:
            print(f"更新皮肤错误: {e}")

    def show_menu(self, event):
        menu = tk.Menu(self.root, tearoff=0)
        menu.add_command(label="主页", command=self.show_home)
        menu.add_command(label="退出", command=self.root.quit)
        menu.add_command(label="聊天", command=lambda: self.chat_entry.focus_set() if self.chat_entry else None)
        menu.tk_popup(event.x_root, event.y_root)
    
    def show_home(self):
        HomePage(self.root)

    def send_chat(self, event=None):
        text = self.chat_entry.get()
        if text.strip() and self.client and self.client.ws:
            # 使用新的protobuf协议发送聊天消息
            asyncio.run_coroutine_threadsafe(
                self.send_chat_message(text),
                asyncio.get_event_loop()
            )
            self.chat_entry.delete(0, tk.END)
    
    async def send_chat_message(self, text):
        """发送聊天消息（使用protobuf格式）"""
        try:
            # 创建聊天数据
            chat_data = {
                "type": "pet_chat",
                "player_id": str(self.player_id),
                "player_name": f"用户{self.player_id}",
                "chat_text": text,
                "position_x": self.root.winfo_x(),
                "position_y": self.root.winfo_y(),
                "timestamp": int(time.time() * 1000)
            }
            
            # 将聊天数据序列化为JSON字符串，然后转为bytes
            chat_json = json.dumps(chat_data)
            chat_bytes = chat_json.encode('utf-8')
            
            # 使用GAME_ACTION_NOTIFICATION消息ID发送（暂时方案）
            await self.client.protobuf_client.send_raw_message(
                game_pb.MessageId.GAME_ACTION_NOTIFICATION, 
                chat_bytes
            )
            print(f"发送聊天消息: {text}")
            
        except Exception as e:
            print(f"发送聊天消息失败: {e}")

    def animate(self):
        now = time.time()
        self.events = [t for t in self.events if now - t < 1]
        action_count = len(self.events)

        if action_count > 0:
            delay = max(50, 200 - action_count*20)
            self.frame = (self.frame + 1) % len(self.action_sprites)
            self.current_image = self.action_sprites[self.frame]
            self.label.config(image=self.current_image)
        else:
            self.current_image = self.idle_sprites[0]
            self.label.config(image=self.current_image)
            delay = 200

        if self.chat_text:
            if not self.chat_label:
                self.chat_label = tk.Label(self.root, text=self.chat_text, bg="yellow")
            self.chat_label.place(x=0, y=-20)
            if now - self.chat_start > 1:
                self.chat_label.place_forget()
                self.chat_text = None

        self.root.after(delay, self.animate)

    def start_move(self, event):
        self.x = event.x
        self.y = event.y

    def on_move(self, event):
        dx = event.x - self.x
        dy = event.y - self.y
        x = self.root.winfo_x() + dx
        y = self.root.winfo_y() + dy
        self.root.geometry(f"+{x}+{y}")
        self.trigger_action()

    def trigger_action(self):
        self.events.append(time.time())
        if self.client and self.client.ws:
            # 使用新的protobuf协议发送动作消息
            asyncio.run_coroutine_threadsafe(
                self.send_action_message(),
                asyncio.get_event_loop()
            )
    
    async def send_action_message(self):
        """发送动作消息（使用protobuf格式）"""
        try:
            # 创建GameAction消息（利用现有的协议）
            # 这里需要根据实际的GameAction定义来构造
            # 暂时发送一个简单的通知，包含位置信息
            
            # 创建动作数据
            action_data = {
                "type": "pet_move",
                "player_id": str(self.player_id),
                "position_x": self.root.winfo_x(),
                "position_y": self.root.winfo_y(),
                "timestamp": int(time.time() * 1000)
            }
            
            # 将动作数据序列化为JSON字符串，然后转为bytes
            action_json = json.dumps(action_data)
            action_bytes = action_json.encode('utf-8')
            
            # 使用GAME_ACTION_NOTIFICATION消息ID发送
            await self.client.protobuf_client.send_raw_message(
                game_pb.MessageId.GAME_ACTION_NOTIFICATION, 
                action_bytes
            )
            
        except Exception as e:
            print(f"发送动作消息失败: {e}")

    def receive_action(self):
        self.events.append(time.time())

    def receive_chat(self, text):
        self.chat_text = text
        self.chat_start = time.time()

    def start_listeners(self):
        def on_click(x, y, button, pressed):
            if pressed:
                self.trigger_action()
        def on_key_press(key):
            self.trigger_action()
        mouse.Listener(on_click=on_click).start()
        keyboard.Listener(on_press=on_key_press).start()

class HomePage:
    def __init__(self, parent):
        self.window = tk.Toplevel(parent)
        self.window.title("桌面宠物主页")
        self.window.geometry("900x650")
        self.window.resizable(False, False)
        self.window.configure(bg="#2c3e50")
        
        self.center_window(self.window, 900, 650)
        
        self.style = ttk.Style()
        self.style.theme_use('clam')
        
        self.style.configure('TFrame', background='#2c3e50')
        self.style.configure('Header.TLabel', background='#2c3e50', foreground='white', font=('Arial', 18, 'bold'))
        self.style.configure('Content.TFrame', background='#ecf0f1')
        self.style.configure('Menu.TButton', background='#34495e', foreground='white', 
                            font=('Arial', 12), width=15, padding=10)
        self.style.map('Menu.TButton', background=[('active', '#3498db')])
        self.style.configure('Action.TButton', background='#3498db', foreground='white', 
                            font=('Arial', 10), padding=5)
        self.style.map('Action.TButton', background=[('active', '#2980b9')])
        
        main_frame = ttk.Frame(self.window, style='TFrame')
        main_frame.pack(fill=tk.BOTH, expand=True, padx=20, pady=20)
        
        header = ttk.Frame(main_frame, style='TFrame')
        header.pack(fill=tk.X, pady=(0, 20))
        
        title_label = ttk.Label(header, text="桌面宠物管理中心", style='Header.TLabel')
        title_label.pack(side=tk.LEFT)
        
        content_frame = ttk.Frame(main_frame)
        content_frame.pack(fill=tk.BOTH, expand=True)
        
        self.menu_frame = ttk.Frame(content_frame, width=180, style='TFrame')
        self.menu_frame.pack(side=tk.LEFT, fill=tk.Y)
        self.menu_frame.pack_propagate(False)
        
        self.content_frame = ttk.Frame(content_frame, style='Content.TFrame')
        self.content_frame.pack(side=tk.RIGHT, expand=True, fill=tk.BOTH, padx=(20, 0))
        
        self.create_menu()
        
        self.auth = AuthManager()
        self.backpack_items = []
        self.market_items = []
        
        self.show_marketplace()
    
    def create_menu(self):
        marketplace_btn = ttk.Button(self.menu_frame, text="商城", style='Menu.TButton',
                                    command=self.show_marketplace)
        marketplace_btn.pack(pady=10)
        
        chatroom_btn = ttk.Button(self.menu_frame, text="聊天室", style='Menu.TButton',
                                 command=self.show_chatroom)
        chatroom_btn.pack(pady=10)
        
        friends_btn = ttk.Button(self.menu_frame, text="好友", style='Menu.TButton',
                                command=self.show_friends)
        friends_btn.pack(pady=10)
        
        profile_btn = ttk.Button(self.menu_frame, text="我", style='Menu.TButton',
                                command=self.show_profile)
        profile_btn.pack(pady=10)
    
    def clear_content(self):
        for widget in self.content_frame.winfo_children():
            widget.destroy()
    
    def center_window(self, window, width, height):
        window.update_idletasks()
        x = (window.winfo_screenwidth() // 2) - (width // 2)
        y = (window.winfo_screenheight() // 2) - (height // 2)
        window.geometry(f'{width}x{height}+{x}+{y}')
    
    def show_marketplace(self):
        self.clear_content()
        
        title_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        title_frame.pack(fill=tk.X, pady=10)
        
        title_label = ttk.Label(title_frame, text="皮肤商城", 
                               font=('Arial', 16, 'bold'), background='#ecf0f1')
        title_label.pack()
        
        if not self.is_logged_in():
            ttk.Label(self.content_frame, text="请先登录才能查看商城", 
                     background='#ecf0f1').pack(pady=20)
            return
        
        if not self.market_items:
            self.fetch_market_items()
            return
        
        container = ttk.Frame(self.content_frame, style='Content.TFrame')
        container.pack(fill=tk.BOTH, expand=True, padx=10, pady=10)
        
        canvas = tk.Canvas(container, bg='#ecf0f1', highlightthickness=0)
        scrollbar = ttk.Scrollbar(container, orient="vertical", command=canvas.yview)
        scrollable_frame = ttk.Frame(canvas, style='Content.TFrame')
        
        scrollable_frame.bind(
            "<Configure>",
            lambda e: canvas.configure(scrollregion=canvas.bbox("all"))
        )
        
        canvas.create_window((0, 0), window=scrollable_frame, anchor="nw")
        canvas.configure(yscrollcommand=scrollbar.set)
        
        for i in range(4):
            scrollable_frame.columnconfigure(i, weight=1)
        
        for i, item in enumerate(self.market_items):
            row = i // 4
            col = i % 4
            
            skin_frame = ttk.Frame(scrollable_frame, style='Content.TFrame', 
                                  relief='raised', padding=10)
            skin_frame.grid(row=row, column=col, padx=10, pady=10, sticky='nsew')
            
            preview_frame = ttk.Frame(skin_frame, height=100, width=120, style='Content.TFrame')
            preview_frame.pack_propagate(False)
            preview_frame.pack(pady=5, fill=tk.X, expand=True)
            
            preview = ttk.Label(preview_frame, text=item.get("name", "未知皮肤"), 
                               background='#bdc3c7',
                               anchor='center', font=('Arial', 10))
            preview.pack(fill=tk.BOTH, expand=True)
            
            price_label = ttk.Label(skin_frame, text=f"价格: {item.get('price', 0)}金币", 
                                   background='#ecf0f1', font=('Arial', 9))
            price_label.pack(pady=2)
            
            btn_frame = ttk.Frame(skin_frame, style='Content.TFrame')
            btn_frame.pack(pady=5, fill=tk.X)
            
            preview_btn = ttk.Button(btn_frame, text="预览", style='Action.TButton',
                                    command=lambda s=item: self.preview_skin(s))
            preview_btn.pack(side=tk.LEFT, padx=2, fill=tk.X, expand=True)
            
            buy_btn = ttk.Button(btn_frame, text="购买", style='Action.TButton',
                                command=lambda s=item: self.buy_skin(s))
            buy_btn.pack(side=tk.LEFT, padx=2, fill=tk.X, expand=True)
        
        canvas.pack(side="left", fill="both", expand=True)
        scrollbar.pack(side="right", fill="y")
    
    def fetch_market_items(self):
        if not self.auth.token:
            messagebox.showerror("错误", "未登录，无法获取商城信息")
            return
            
        if not hasattr(self, 'client') or not self.client.ws:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        # 这里需要根据实际的desktop_pet协议来实现
        # 暂时使用JSON格式作为过渡
        message = {
            "type": "get_market"
        }
        
        asyncio.run_coroutine_threadsafe(
            self.client.ws.send(json.dumps(message)),
            asyncio.get_event_loop()
        )
    
    def preview_skin(self, skin_item):
        preview_window = tk.Toplevel(self.window)
        preview_window.title(f"预览 - {skin_item.get('name', '未知皮肤')}")
        preview_window.geometry("350x400")
        preview_window.configure(bg='#ecf0f1')
        preview_window.transient(self.window)
        preview_window.grab_set()
        self.center_window(preview_window, 350, 400)
        
        main_frame = ttk.Frame(preview_window, style='Content.TFrame')
        main_frame.pack(fill=tk.BOTH, expand=True, padx=20, pady=20)
        
        ttk.Label(main_frame, text=f"正在预览: {skin_item.get('name', '未知皮肤')}", 
                 font=('Arial', 14), background='#ecf0f1').pack(pady=20)
        
        preview_area = ttk.Frame(main_frame, width=200, height=200, 
                                style='Content.TFrame')
        preview_area.pack(pady=10)
        preview_area.pack_propagate(False)
        
        ttk.Label(preview_area, text="皮肤动画预览", 
                 background='#bdc3c7', anchor='center').pack(fill=tk.BOTH, expand=True)
        
        desc_frame = ttk.Frame(main_frame, style='Content.TFrame')
        desc_frame.pack(fill=tk.X, pady=10)
        
        ttk.Label(desc_frame, text="描述:", 
                 font=('Arial', 12, 'bold'), background='#ecf0f1').pack(anchor='w')
        ttk.Label(desc_frame, text=skin_item.get('description', '暂无描述'), 
                 background='#ecf0f1', wraplength=300).pack(anchor='w', pady=5)
        
        ttk.Button(main_frame, text="关闭", style='Action.TButton',
                  command=preview_window.destroy).pack(pady=10)
    
    def buy_skin(self, skin_item):
        if not self.is_logged_in():
            messagebox.showinfo("提示", "请先登录才能购买皮肤")
            return
        
        if not hasattr(self, 'client') or not self.client.ws:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        message = {
            "type": "purchase",
            "item_id": skin_item.get("id")
        }
        
        asyncio.run_coroutine_threadsafe(
            self.client.ws.send(json.dumps(message)),
            asyncio.get_event_loop()
        )
    
    def show_chatroom(self):
        self.clear_content()
        
        header_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        header_frame.pack(fill=tk.X, pady=10)
        
        title_label = ttk.Label(header_frame, text="聊天室", 
                               font=('Arial', 16, 'bold'), background='#ecf0f1')
        title_label.pack(side=tk.LEFT, padx=10)
        
        create_btn = ttk.Button(header_frame, text="创建聊天室", style='Action.TButton',
                               command=self.create_chatroom)
        create_btn.pack(side=tk.RIGHT, padx=10)
        
        if not self.is_connected():
            ttk.Label(self.content_frame, text="无法连接到服务器，请检查网络连接", 
                     background='#ecf0f1').pack(pady=20)
            return
        
        container = ttk.Frame(self.content_frame, style='Content.TFrame')
        container.pack(fill=tk.BOTH, expand=True, padx=10, pady=10)
        
        canvas = tk.Canvas(container, bg='#ecf0f1', highlightthickness=0)
        scrollbar = ttk.Scrollbar(container, orient="vertical", command=canvas.yview)
        scrollable_frame = ttk.Frame(canvas, style='Content.TFrame')
        
        scrollable_frame.bind(
            "<Configure>",
            lambda e: canvas.configure(scrollregion=canvas.bbox("all"))
        )
        
        canvas.create_window((0, 0), window=scrollable_frame, anchor="nw")
        canvas.configure(yscrollcommand=scrollbar.set)
        
        chatrooms = [f"聊天室 {i+1}" for i in range(15)]
        
        for i, room in enumerate(chatrooms):
            room_frame = ttk.Frame(scrollable_frame, style='Content.TFrame', 
                                  relief='raised', padding=10)
            room_frame.pack(fill=tk.X, padx=10, pady=5)
            
            ttk.Label(room_frame, text=room, background='#ecf0f1', 
                     font=('Arial', 12)).pack(side=tk.LEFT)
            
            join_btn = ttk.Button(room_frame, text="加入", style='Action.TButton',
                                 command=lambda r=room: self.join_chatroom(r))
            join_btn.pack(side=tk.RIGHT)
        
        canvas.pack(side="left", fill="both", expand=True)
        scrollbar.pack(side="right", fill="y")
    
    def create_chatroom(self):
        create_window = tk.Toplevel(self.window)
        create_window.title("创建聊天室")
        create_window.geometry("400x200")
        create_window.configure(bg='#ecf0f1')
        create_window.transient(self.window)
        create_window.grab_set()
        self.center_window(create_window, 400, 200)
        
        main_frame = ttk.Frame(create_window, style='Content.TFrame')
        main_frame.pack(fill=tk.BOTH, expand=True, padx=20, pady=20)
        
        ttk.Label(main_frame, text="聊天室名称:", background='#ecf0f1').grid(row=0, column=0, padx=10, pady=10, sticky='e')
        name_entry = ttk.Entry(main_frame, width=20)
        name_entry.grid(row=0, column=1, padx=10, pady=10, sticky='w')
        
        ttk.Label(main_frame, text="密码(可选):", background='#ecf0f1').grid(row=1, column=0, padx=10, pady=10, sticky='e')
        password_entry = ttk.Entry(main_frame, width=20, show="*")
        password_entry.grid(row=1, column=1, padx=10, pady=10, sticky='w')
        
        btn_frame = ttk.Frame(main_frame, style='Content.TFrame')
        btn_frame.grid(row=2, column=0, columnspan=2, pady=10)
        
        confirm_btn = ttk.Button(btn_frame, text="创建", style='Action.TButton',
                                command=lambda: self.confirm_create_chatroom(
                                    name_entry.get(), password_entry.get(), create_window))
        confirm_btn.pack(side=tk.LEFT, padx=5)
        
        cancel_btn = ttk.Button(btn_frame, text="取消", style='Action.TButton',
                               command=create_window.destroy)
        cancel_btn.pack(side=tk.LEFT, padx=5)
    
    def confirm_create_chatroom(self, name, password, window):
        if not name:
            messagebox.showerror("错误", "聊天室名称不能为空")
            window.destroy()
            return
            
        # 使用protobuf协议发送创建房间请求
        if hasattr(self, 'client') and self.client.ws and self.client.protobuf_client:
            try:
                print(f"准备发送创建房间请求: {name}")
                print(f"websocket连接状态: {self.client.ws.open if hasattr(self.client.ws, 'open') else 'unknown'}")
                
                # 检查连接状态
                is_open = False
                if hasattr(self.client.ws, 'open'):
                    is_open = self.client.ws.open
                elif hasattr(self.client.ws, 'closed'):
                    is_open = not self.client.ws.closed
                else:
                    # 如果无法确定状态，默认认为是打开的
                    is_open = True
                    
                if not is_open:
                    print("WebSocket连接未打开，无法发送消息")
                    messagebox.showerror("错误", "WebSocket连接未打开")
                    window.destroy()
                    return
                
                # 使用protobuf客户端发送创建房间请求
                print("开始发送创建房间请求...")
                # 确保使用正确的事件循环
                try:
                    # 获取当前线程的事件循环
                    loop = asyncio.get_event_loop()
                except RuntimeError:
                    # 如果没有事件循环，使用默认循环
                    loop = asyncio.get_event_loop_policy().get_event_loop()
                
                # 在事件循环中执行协程
                future = asyncio.run_coroutine_threadsafe(
                    self.client.send_create_room_request(name),
                    loop
                )
                
                # 等待结果
                try:
                    success = future.result(timeout=5)  # 5秒超时
                    if success:
                        print(f"创建房间请求发送成功: {name}")
                    else:
                        print(f"创建房间请求发送失败: {name}")
                        messagebox.showerror("错误", "创建房间请求发送失败")
                except Exception as e:
                    print(f"等待创建房间请求发送结果超时: {e}")
                    messagebox.showerror("错误", f"创建房间请求发送超时: {e}")
                    
            except Exception as e:
                print(f"发送创建房间请求失败: {e}")
                import traceback
                traceback.print_exc()
                messagebox.showerror("错误", f"创建房间请求发送失败: {e}")
        else:
            print("未连接到服务器或protobuf客户端未初始化")
            print(f"client: {hasattr(self, 'client')}")
            if hasattr(self, 'client'):
                print(f"client.ws: {self.client.ws}")
                print(f"client.protobuf_client: {self.client.protobuf_client if hasattr(self.client, 'protobuf_client') else 'None'}")
            messagebox.showerror("错误", "未连接到服务器")
        
        window.destroy()

    def join_chatroom(self, room_name):
        messagebox.showinfo("加入聊天室", f"已加入 {room_name}")
    
    def show_friends(self):
        self.clear_content()
        ttk.Label(self.content_frame, text="好友列表", 
                 font=('Arial', 16, 'bold'), background='#ecf0f1').pack(pady=10)
        
        if not self.is_logged_in():
            ttk.Label(self.content_frame, text="请先登录才能查看好友", 
                     background='#ecf0f1').pack(pady=20)
            return
        
        friends = [("好友1", "在线"), ("好友2", "离线"), ("好友3", "游戏中")]
        
        for name, status in friends:
            friend_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
            friend_frame.pack(fill=tk.X, padx=20, pady=5)
            
            status_color = "green" if status == "在线" else "gray"
            ttk.Label(friend_frame, text=name, width=10, background='#ecf0f1').pack(side=tk.LEFT)
            ttk.Label(friend_frame, text=status, foreground=status_color, 
                     background='#ecf0f1').pack(side=tk.LEFT)
    
    def show_profile(self):
        self.clear_content()
        ttk.Label(self.content_frame, text="个人信息", 
                 font=('Arial', 16, 'bold'), background='#ecf0f1').pack(pady=10)
        
        if not self.is_logged_in():
            self.show_phone_login()
            return
        
        self.fetch_backpack_items()
        
        user_info = {
            "用户名": "testuser",
            "等级": "10", 
            "金币": "1000",
            "拥有的皮肤": str(len(self.backpack_items))
        }
        
        info_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        info_frame.pack(pady=20)
        
        for i, (key, value) in enumerate(user_info.items()):
            ttk.Label(info_frame, text=f"{key}:", background='#ecf0f1', 
                     font=('Arial', 12, 'bold')).grid(row=i, column=0, sticky=tk.W, pady=5, padx=10)
            ttk.Label(info_frame, text=value, background='#ecf0f1').grid(row=i, column=1, sticky=tk.W, pady=5)
        
        ttk.Label(self.content_frame, text="我的背包", 
                 font=('Arial', 14, 'bold'), background='#ecf0f1').pack(pady=(30, 10))
        
        if self.backpack_items:
            backpack_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
            backpack_frame.pack(fill=tk.BOTH, expand=True, padx=20, pady=10)
            
            canvas = tk.Canvas(backpack_frame, bg='#ecf0f1', highlightthickness=0)
            scrollbar = ttk.Scrollbar(backpack_frame, orient="vertical", command=canvas.yview)
            scrollable_frame = ttk.Frame(canvas, style='Content.TFrame')
            
            scrollable_frame.bind(
                "<Configure>",
                lambda e: canvas.configure(scrollregion=canvas.bbox("all"))
            )
            
            canvas.create_window((0, 0), window=scrollable_frame, anchor="nw")
            canvas.configure(yscrollcommand=scrollbar.set)
            
            for i, item in enumerate(self.backpack_items):
                item_frame = ttk.Frame(scrollable_frame, style='Content.TFrame', relief='raised', padding=5)
                item_frame.pack(fill=tk.X, pady=2)
                
                ttk.Label(item_frame, text=item.get("name", "未知物品"), 
                         background='#ecf0f1').pack(side=tk.LEFT)
                
                if item.get("equipped", False):
                    status_label = ttk.Label(item_frame, text="已装备", 
                                           background='#ecf0f1', foreground='green')
                    status_label.pack(side=tk.LEFT, padx=5)
                
                if item.get("type") == "skin" and not item.get("equipped", False):
                    use_btn = ttk.Button(item_frame, text="使用", style='Action.TButton',
                                        command=lambda i=item: self.use_item(i))
                    use_btn.pack(side=tk.RIGHT, padx=5)
                
                ttk.Label(item_frame, text=f"获得时间: {item.get('acquired_time', '未知')}", 
                         background='#ecf0f1', font=('Arial', 9)).pack(side=tk.RIGHT)
            
            canvas.pack(side="left", fill="both", expand=True)
            scrollbar.pack(side="right", fill="y")
        else:
            ttk.Label(self.content_frame, text="背包为空", 
                     background='#ecf0f1').pack(pady=20)
    
    def use_item(self, item):
        if not hasattr(self, 'client') or not self.client.ws:
            messagebox.showerror("错误", "未连接到服务器")
            return
            
        message = {
            "type": "equip",
            "item_id": item.get("id")
        }
        
        asyncio.run_coroutine_threadsafe(
            self.client.ws.send(json.dumps(message)),
            asyncio.get_event_loop()
        )
        
        messagebox.showinfo("成功", f"已装备 {item.get('name')}")
        self.show_profile()
    
    def fetch_backpack_items(self):
        if not self.auth.token:
            messagebox.showerror("错误", "未登录，无法获取背包信息")
            return
            
        if not hasattr(self, 'client') or not self.client.ws:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        message = {
            "type": "get_backpack"
        }
        
        asyncio.run_coroutine_threadsafe(
            self.client.ws.send(json.dumps(message)),
            asyncio.get_event_loop()
        )
    
    def show_phone_login(self):
        title_label = ttk.Label(self.content_frame, text="登录方式选择", 
                               font=('Arial', 16, 'bold'), background='#ecf0f1')
        title_label.pack(pady=10)
        
        # 添加游客登录按钮
        guest_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        guest_frame.pack(fill=tk.X, pady=20, padx=20)
        
        guest_btn = ttk.Button(guest_frame, text="游客登录（快速体验）", 
                              command=self.do_guest_login, 
                              style='Action.TButton')
        guest_btn.pack(pady=10)
        
        ttk.Label(guest_frame, text="或者使用手机号登录", 
                 background='#ecf0f1', font=('Arial', 12)).pack(pady=10)
        
        # 手机号登录部分
        phone_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        phone_frame.pack(fill=tk.X, pady=10, padx=20)
        
        ttk.Label(phone_frame, text="国家代码:", background='#ecf0f1').pack(side=tk.LEFT)
        self.country_code = ttk.Combobox(phone_frame, width=5, values=["+86", "+1", "+81", "+82", "+852", "+853", "+886"])
        self.country_code.set("+86")
        self.country_code.pack(side=tk.LEFT, padx=5)
        
        ttk.Label(phone_frame, text="手机号:", background='#ecf0f1').pack(side=tk.LEFT, padx=(10, 0))
        self.phone = ttk.Entry(phone_frame, width=15)
        self.phone.pack(side=tk.LEFT, padx=5)
        
        self.send_code_btn = ttk.Button(phone_frame, text="发送验证码", command=self.send_verification_code)
        self.send_code_btn.pack(side=tk.LEFT, padx=5)
        
        code_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        code_frame.pack(fill=tk.X, pady=10, padx=20)
        
        ttk.Label(code_frame, text="验证码:", background='#ecf0f1').pack(side=tk.LEFT)
        self.code = ttk.Entry(code_frame, width=10)
        self.code.pack(side=tk.LEFT, padx=5)
        
        device_frame = ttk.Frame(self.content_frame, style='Content.TFrame')
        device_frame.pack(fill=tk.X, pady=10, padx=20)
        
        ttk.Label(device_frame, text="设备ID:", background='#ecf0f1').pack(side=tk.LEFT)
        self.device_id = ttk.Entry(device_frame, width=20)
        self.device_id.pack(side=tk.LEFT, padx=5)
        self.device_id.insert(0, str(uuid.uuid4())[:8])
        
        login_btn = ttk.Button(self.content_frame, text="手机号登录/注册", command=self.do_login)
        login_btn.pack(pady=20)
        
        self.status_label = ttk.Label(self.content_frame, text="", background='#ecf0f1', foreground='#e74c3c')
        self.status_label.pack(pady=10)
        
        self.countdown = 0
        self.countdown_timer = None
    
    def do_guest_login(self):
        """执行游客登录"""
        self.status_label.config(text="正在游客登录...")
        threading.Thread(target=self._guest_login_thread, daemon=True).start()
    
    def _guest_login_thread(self):
        """游客登录线程"""
        success, message, result = self.auth.guest_login()
        
        if success:
            self.window.after(0, lambda: self.on_login_success(result, is_guest=True))
        else:
            self.window.after(0, lambda: self.status_label.config(text=message))

    def send_verification_code(self):
        country_code = self.country_code.get().strip()
        phone = self.phone.get().strip()
        device_id = self.device_id.get().strip()
        
        if not country_code or not phone:
            self.status_label.config(text="请填写国家代码和手机号")
            return
        
        self.send_code_btn.config(state="disabled")
        self.countdown = 60
        self.update_countdown()
        
        threading.Thread(target=self._send_code_thread, args=(country_code, phone, device_id), daemon=True).start()
    
    def _send_code_thread(self, country_code, phone, device_id):
        success, message = self.auth.send_sms_code(country_code, phone, device_id)
        self.window.after(0, lambda: self.status_label.config(text=message))
    
    def update_countdown(self):
        if self.countdown > 0:
            self.send_code_btn.config(text=f"重新发送({self.countdown})")
            self.countdown -= 1
            self.countdown_timer = self.window.after(1000, self.update_countdown)
        else:
            self.send_code_btn.config(text="发送验证码", state="normal")
    
    def do_login(self):
        country_code = self.country_code.get().strip()
        phone = self.phone.get().strip()
        code = self.code.get().strip()
        device_id = self.device_id.get().strip()
        
        if not all([country_code, phone, code]):
            self.status_label.config(text="请填写完整信息")
            return
        
        threading.Thread(target=self._login_thread, args=(country_code, phone, code, device_id), daemon=True).start()
    
    def _login_thread(self, country_code, phone, code, device_id):
        success, message, result = self.auth.phone_login(country_code, phone, code, device_id)
        
        if success:
            self.window.after(0, lambda: self.on_login_success(result))
        else:
            self.window.after(0, lambda: self.status_label.config(text=message))
    
    def on_login_success(self, result, is_guest=False):
        self.status_label.config(text="登录成功!", foreground="#2ecc71")
        
        # 设置client对象到HomePage中
        if not hasattr(self, 'client'):
            self.client = JiggerClientForHome()
            self.client.auth = self.auth
        
        self.client.connect_websocket()
        
        login_type = "游客" if is_guest else "用户"
        messagebox.showinfo("登录成功", f"欢迎使用桌面宠物!\n{login_type}登录成功\n您的OpenID: {self.auth.openid}")
        
        self.show_profile()
    
    def is_logged_in(self):
        return self.auth.token is not None and self.auth.openid is not None
    
    def is_connected(self):
        return True

class JiggerClientForHome:
    """简化的客户端类，用于HomePage中的WebSocket连接"""
    def __init__(self):
        self.auth = None
        self.ws = None
        self.online = False
        self.protobuf_client = ProtobufClient()
        self.message_buffer = b''
    
    def connect_websocket(self):
        if not self.auth or not self.auth.token:
            print("未登录，无法连接WebSocket")
            return
        print("开始连接WebSocket...")
        threading.Thread(target=self.ws_loop, daemon=True).start()
    
    def ws_loop(self):
        print("WebSocket循环启动...")
        asyncio.run(self.ws_main())
    
    async def ws_main(self):
        uri = "ws://127.0.0.1:18080/ws"
        print(f"尝试连接到WebSocket: {uri}")
        try:
            self.ws = await asyncio.wait_for(websockets.connect(uri), timeout=5)
            self.protobuf_client.ws = self.ws
            self.online = True
            print("连接游戏服务器成功")
            
            # 发送认证请求
            await self.send_auth_request()
            
            # 处理消息
            print("开始接收消息...")
            async for msg in self.ws:
                await self.handle_websocket_message(msg)
                
        except Exception as e:
            print(f"连接服务器失败: {e}")
            import traceback
            traceback.print_exc()
            self.online = False
    
    async def send_auth_request(self):
        """发送认证请求"""
        if not self.auth.token:
            print("未登录，无法认证")
            return
            
        print("准备发送认证请求...")
        # 创建认证请求
        auth_request = game_pb.AuthRequest()
        auth_request.token = self.auth.token
        auth_request.protocol_version = "1.0"
        auth_request.client_version = "1.0.0"
        auth_request.device_type = "desktop"
        auth_request.device_id = self.auth.device_id
        auth_request.app_id = self.auth.app_id
        auth_request.nonce = str(uuid.uuid4())
        auth_request.timestamp = int(time.time() * 1000)
        auth_request.signature = ""
        auth_request.is_guest = self.auth.is_guest
        
        # 发送认证请求
        await self.protobuf_client.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)
        print("认证请求已发送")
    
    async def send_create_room_request(self, room_name):
        """发送创建房间请求"""
        if not self.ws or not self.protobuf_client:
            print("WebSocket连接或protobuf客户端未初始化")
            return False
            
        try:
            print(f"准备发送创建房间请求: {room_name}")
            # 创建创建房间请求
            create_room_request = game_pb.CreateRoomRequest()
            create_room_request.name = room_name
            
            # 发送创建房间请求
            await self.protobuf_client.send_message(game_pb.MessageId.CREATE_ROOM_REQUEST, create_room_request)
            print("创建房间请求已发送")
            return True
        except Exception as e:
            print(f"发送创建房间请求失败: {e}")
            import traceback
            traceback.print_exc()
            return False
    
    async def handle_websocket_message(self, msg):
        """处理WebSocket消息"""
        try:
            if isinstance(msg, bytes):
                self.message_buffer += msg
            else:
                self.message_buffer += msg.encode('utf-8')
            
            while True:
                message, self.message_buffer = self.protobuf_client.unpack_message(self.message_buffer)
                if message is None:
                    break
                await self.handle_protobuf_message(message)
                
        except Exception as e:
            print(f"处理WebSocket消息错误: {e}")
            import traceback
            traceback.print_exc()
    
    async def handle_protobuf_message(self, message):
        """处理protobuf消息"""
        try:
            msg_id = message.id
            print(f"Received message ID: {msg_id}")
            
            if msg_id == game_pb.MessageId.AUTH_RESPONSE:
                await self.handle_auth_response(message)
            elif msg_id == game_pb.MessageId.GAME_ACTION_NOTIFICATION:
                await self.handle_game_action_notification(message)
            elif msg_id == game_pb.MessageId.CREATE_ROOM_RESPONSE:
                await self.handle_create_room_response(message)
            else:
                print(f"Unhandled message type: {msg_id}")
                
        except Exception as e:
            print(f"处理protobuf消息错误: {e}")
            import traceback
            traceback.print_exc()
    
    async def handle_game_action_notification(self, message):
        """处理游戏动作通知"""
        try:
            # 解析JSON数据
            action_json = message.data.decode('utf-8')
            action_data = json.loads(action_json)
            
            action_type = action_data.get("type")
            print(f"收到动作通知: {action_type}")
            
        except Exception as e:
            print(f"处理游戏动作通知错误: {e}")
    
    async def handle_create_room_response(self, message):
        """处理创建房间响应"""
        try:
            create_room_response = game_pb.CreateRoomResponse()
            create_room_response.ParseFromString(message.data)
            
            if create_room_response.ret == game_pb.ErrorCode.OK:
                room = create_room_response.room
                print(f"创建房间成功: {room.name} (ID: {room.id})")
                # 这里可以添加创建成功的UI反馈
            else:
                print(f"创建房间失败: 错误码 {create_room_response.ret}")
                # 这里可以添加创建失败的UI反馈
                
        except Exception as e:
            print(f"处理创建房间响应错误: {e}")
    
    async def handle_auth_response(self, message):
        """处理认证响应"""
        try:
            auth_response = game_pb.AuthResponse()
            auth_response.ParseFromString(message.data)
            
            if auth_response.ret == game_pb.ErrorCode.OK:
                guest_info = "（游客）" if auth_response.is_guest else ""
                print(f"认证成功{guest_info}! UID: {auth_response.uid}, 昵称: {auth_response.nickname}")
                print(f"金币: {auth_response.gold}, 钻石: {auth_response.diamond}")
            else:
                print(f"认证失败: {auth_response.error_msg}")
                
        except Exception as e:
            print(f"处理认证响应错误: {e}")

class JiggerClient:
    def __init__(self, sprite_path):
        self.sprite_path = sprite_path
        self.players = {}
        self.player_id = id(self)
        self.ws = None
        self.online = False
        self.event_queue = queue.Queue()
        self.auth = AuthManager()
        self.current_skin = None
        self.protobuf_client = ProtobufClient()  # 添加Protobuf客户端
        self.message_buffer = b''  # 消息缓冲区

        self.root = tk.Tk()
        self.root.withdraw()

        self.start_pet(self.player_id, self, is_self=True)

        self.root.after(50, self.process_queue)
        self.root.mainloop()

    def connect_websocket(self):
        if not self.auth.token or not self.auth.openid:
            print("未登录，无法连接WebSocket")
            return
            
        threading.Thread(target=self.ws_loop, daemon=True).start()

    def start_pet(self, player_id, client, is_self):
        window = tk.Toplevel(self.root)
        pet = DesktopPet(window, self.sprite_path, player_id, client, is_self)
        self.players[player_id] = pet

    def process_queue(self):
        while not self.event_queue.empty():
            event = self.event_queue.get()
            pid = event.get("player_id")
            if pid not in self.players:
                self.start_pet(pid, self, is_self=False)
            pet = self.players[pid]
            if event["type"] == "action":
                pet.receive_action()
            elif event["type"] == "chat":
                pet.receive_chat(event["text"])
        self.root.after(50, self.process_queue)

    def ws_loop(self):
        asyncio.run(self.ws_main())

    async def ws_main(self):
        uri = "ws://127.0.0.1:18080/ws"  # 使用正确的WebSocket地址
        try:
            self.ws = await asyncio.wait_for(websockets.connect(uri), timeout=5)
            self.protobuf_client.ws = self.ws  # 设置protobuf客户端的websocket连接
            self.online = True
            print("联网模式")
            
            # 发送认证请求
            await self.send_auth_request()
            
        except Exception as e:
            self.ws = None
            self.online = False
            print(f"连接服务器失败: {str(e)}")
            print("单机模式")

        if self.online:
            try:
                async for msg in self.ws:
                    await self.handle_websocket_message(msg)
            except Exception as e:
                self.online = False
                print(f"断开连接: {str(e)}，切换单机模式")
                
    async def send_auth_request(self):
        """发送认证请求"""
        if not self.auth.token:
            print("未登录，无法认证")
            return
            
        # 创建认证请求
        auth_request = game_pb.AuthRequest()
        auth_request.token = self.auth.token
        auth_request.protocol_version = "1.0"
        auth_request.client_version = "1.0.0"
        auth_request.device_type = "desktop"
        auth_request.device_id = self.auth.device_id
        auth_request.app_id = self.auth.app_id
        auth_request.nonce = str(uuid.uuid4())
        auth_request.timestamp = int(time.time() * 1000)
        auth_request.signature = ""  # 简化处理，不计算签名
        auth_request.is_guest = self.auth.is_guest  # 添加游客标识
        
        # 发送认证请求
        await self.protobuf_client.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)

    async def handle_websocket_message(self, msg):
        """处理WebSocket消息（二进制protobuf数据）"""
        try:
            # 将收到的数据添加到缓冲区
            if isinstance(msg, bytes):
                self.message_buffer += msg
            else:
                # 如果收到的是文本消息，转为字节
                self.message_buffer += msg.encode('utf-8')
            
            # 处理缓冲区中的所有完整消息
            while True:
                message, self.message_buffer = self.protobuf_client.unpack_message(self.message_buffer)
                if message is None:
                    break  # 没有完整消息
                    
                await self.handle_protobuf_message(message)
                
        except Exception as e:
            print(f"处理WebSocket消息错误: {e}")
    async def handle_protobuf_message(self, message):
        """处理protobuf消息"""
        try:
            msg_id = message.id
            print(f"Received message ID: {msg_id}")
            
            if msg_id == game_pb.MessageId.AUTH_RESPONSE:
                await self.handle_auth_response(message)
            elif msg_id == game_pb.MessageId.GET_USER_INFO_RESPONSE:
                await self.handle_user_info_response(message)
            elif msg_id == game_pb.MessageId.GAME_ACTION_NOTIFICATION:
                await self.handle_game_action_notification(message)
            # 可以添加更多消息处理
            else:
                print(f"Unhandled message type: {msg_id}")
                
        except Exception as e:
            print(f"处理protobuf消息错误: {e}")
    
    async def handle_auth_response(self, message):
        """处理认证响应"""
        try:
            auth_response = game_pb.AuthResponse()
            auth_response.ParseFromString(message.data)
            
            if auth_response.ret == game_pb.ErrorCode.OK:
                guest_info = "（游客）" if auth_response.is_guest else ""
                print(f"认证成功{guest_info}! UID: {auth_response.uid}, 昵称: {auth_response.nickname}")
                print(f"金币: {auth_response.gold}, 钻石: {auth_response.diamond}")
                
                # 认证成功后可以发送其他请求
                await self.request_user_info(auth_response.uid)
            else:
                print(f"认证失败: {auth_response.error_msg}")
                self.online = False
                await self.ws.close()
                
        except Exception as e:
            print(f"处理认证响应错误: {e}")
    
    async def handle_game_action_notification(self, message):
        """处理游戏动作通知（用于桌面宠物同步）"""
        try:
            # 解析JSON数据
            action_json = message.data.decode('utf-8')
            action_data = json.loads(action_json)
            
            action_type = action_data.get("type")
            player_id = action_data.get("player_id")
            
            if action_type == "pet_chat":
                # 处理聊天消息
                self.event_queue.put({
                    "type": "chat",
                    "player_id": player_id,
                    "text": action_data.get("chat_text")
                })
                print(f"收到聊天消息: {action_data.get('chat_text')}")
                
            elif action_type == "pet_move":
                # 处理移动动作
                self.event_queue.put({
                    "type": "action",
                    "player_id": player_id
                })
                print(f"收到移动动作: {player_id}")
                
        except Exception as e:
            print(f"处理游戏动作通知错误: {e}")
    
    async def handle_user_info_response(self, message):
        """处理用户信息响应"""
        try:
            user_info_response = game_pb.GetUserInfoResponse()
            user_info_response.ParseFromString(message.data)
            
            if user_info_response.ret == game_pb.ErrorCode.OK:
                user_info = user_info_response.user_info
                print(f"用户信息: {user_info.name}, 等级: {user_info.exp}, 金币: {user_info.gold}")
                # 更新界面显示
            else:
                print("获取用户信息失败")
                
        except Exception as e:
            print(f"处理用户信息响应错误: {e}")
    
    async def request_user_info(self, uid):
        """请求用户信息"""
        user_info_request = game_pb.GetUserInfoRequest()
        user_info_request.uid = uid
        
        await self.protobuf_client.send_message(
            game_pb.MessageId.GET_USER_INFO_REQUEST, 
            user_info_request
        )
    def handle_backpack_info(self, event):
        """暂时保留兼容性处理"""
        items = event.get("items", [])
        if hasattr(self, 'home_page') and self.home_page:
            self.home_page.backpack_items = items
            if hasattr(self.home_page, 'show_profile'):
                self.home_page.show_profile()
    
    def handle_market_info(self, event):
        items = event.get("items", [])
        if hasattr(self, 'home_page') and self.home_page:
            self.home_page.market_items = items
            if hasattr(self.home_page, 'show_marketplace'):
                self.home_page.show_marketplace()
    
    def handle_purchase_success(self, event):
        item_id = event.get("item_id")
        messagebox.showinfo("购买成功", f"购买成功! 物品ID: {item_id}")
        asyncio.run_coroutine_threadsafe(
            self.ws.send(json.dumps({"type": "get_backpack"})),
            asyncio.get_event_loop()
        )
    
    def handle_equip_success(self, event):
        item_id = event.get("item_id")
        asyncio.run_coroutine_threadsafe(
            self.ws.send(json.dumps({"type": "get_skin_data", "skin_id": item_id})),
            asyncio.get_event_loop()
        )
    
    def handle_skin_data(self, event):
        skin_id = event.get("skin_id")
        skin_data = event.get("data")
        
        try:
            # 这里应该添加解密逻辑
            # 简化处理，直接使用base64解码
            image_data = base64.b64decode(skin_data)
            
            image = Image.open(BytesIO(image_data))
            
            self.current_skin = image
            
            if self.players and self.player_id in self.players:
                pet = self.players[self.player_id]
                pet.update_skin(self.current_skin)
                
        except Exception as e:
            print(f"处理皮肤数据错误: {e}")
    
    def handle_error(self, event):
        error_type = event.get("error")
        error_messages = {
            "invalid_item_id": "无效的物品ID",
            "item_not_found": "物品不存在",
            "server_error": "服务器错误",
            "skin_not_owned": "未拥有此皮肤",
            "skin_data_not_found": "皮肤数据不存在",
            "no_equipped_skin": "没有装备皮肤"
        }
        
        message = error_messages.get(error_type, f"未知错误: {error_type}")
        messagebox.showerror("错误", message)

if __name__=="__main__":
    sprite_path = resource_path("spritesheet.png")
    client = JiggerClient(sprite_path)
    
    # 设置主页的客户端引用
    if hasattr(client, 'players') and client.player_id in client.players:
        pet = client.players[client.player_id]
        if hasattr(pet, 'home_page'):
            pet.home_page.client = client