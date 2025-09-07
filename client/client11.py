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

# 平台API地址
PLATFORM_API = "http://localhost:8080"

# 配置日志
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class AuthManager:
    def __init__(self):
        self.token = None
        self.openid = None
        self.app_id = "desktop_app"
    
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
            response = requests.post(url, json=data)
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
            response = requests.post(url, json=data)
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

def resource_path(filename):
    if getattr(sys, "frozen", False):
        if hasattr(sys, "_MEIPASS"):
            return os.path.join(sys._MEIPASS, filename)
        return os.path.join(os.path.dirname(sys.executable), filename)
    return os.path.join(os.path.dirname(__file__), filename)

class DesktopPet:
    def __init__(self, root, sprite_path, player_id, ws=None, is_self=True):
        self.root = root
        self.ws = ws
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
        if text.strip() and self.ws:
            asyncio.run_coroutine_threadsafe(self.ws.send(json.dumps({
                "type":"chat","player_id":self.player_id,"text":text
            })), asyncio.get_event_loop())
            self.chat_entry.delete(0, tk.END)

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
        if self.ws:
            asyncio.run_coroutine_threadsafe(self.ws.send(json.dumps({
                "type":"action","player_id":self.player_id
            })), asyncio.get_event_loop())

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
            return
        
        window.destroy()
        messagebox.showinfo("成功", f"聊天室 '{name}' 创建成功!")
    
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
        title_label = ttk.Label(self.content_frame, text="手机号登录/注册", 
                               font=('Arial', 16, 'bold'), background='#ecf0f1')
        title_label.pack(pady=10)
        
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
        
        login_btn = ttk.Button(self.content_frame, text="登录/注册", command=self.do_login)
        login_btn.pack(pady=20)
        
        self.status_label = ttk.Label(self.content_frame, text="", background='#ecf0f1', foreground='#e74c3c')
        self.status_label.pack(pady=10)
        
        self.countdown = 0
        self.countdown_timer = None
    
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
    
    def on_login_success(self, result):
        self.status_label.config(text="登录成功!", foreground="#2ecc71")
        
        self.client.connect_websocket()
        
        messagebox.showinfo("登录成功", f"欢迎使用桌面宠物!\n您的OpenID: {self.auth.openid}")
        
        self.show_profile()
    
    def is_logged_in(self):
        return self.auth.token is not None and self.auth.openid is not None
    
    def is_connected(self):
        return True

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

        self.root = tk.Tk()
        self.root.withdraw()

        self.start_pet(self.player_id, None, is_self=True)

        self.root.after(50, self.process_queue)
        self.root.mainloop()

    def connect_websocket(self):
        if not self.auth.token or not self.auth.openid:
            print("未登录，无法连接WebSocket")
            return
            
        threading.Thread(target=self.ws_loop, daemon=True).start()

    def start_pet(self, player_id, ws, is_self):
        window = tk.Toplevel(self.root)
        pet = DesktopPet(window, self.sprite_path, player_id, ws, is_self)
        self.players[player_id] = pet

    def process_queue(self):
        while not self.event_queue.empty():
            event = self.event_queue.get()
            pid = event.get("player_id")
            if pid not in self.players:
                self.start_pet(pid, self.ws, is_self=False)
            pet = self.players[pid]
            if event["type"] == "action":
                pet.receive_action()
            elif event["type"] == "chat":
                pet.receive_chat(event["text"])
        self.root.after(50, self.process_queue)

    def ws_loop(self):
        asyncio.run(self.ws_main())

    async def ws_main(self):
        uri = "ws://127.0.0.1:8765"
        try:
            self.ws = await asyncio.wait_for(websockets.connect(uri), timeout=1)
            self.online = True
            print("联网模式")
            
            auth_msg = {
                "type": "auth",
                "token": self.auth.token,
                "openid": self.auth.openid,
                "room": "room1"
            }
            await self.ws.send(json.dumps(auth_msg))
            
            response = await asyncio.wait_for(self.ws.recv(), timeout=5)
            auth_result = json.loads(response)
            
            if auth_result.get("type") == "auth_success":
                print("服务器认证成功")
                await self.ws.send(json.dumps({"type":"join","room":"room1","password":None}))
                
                await self.ws.send(json.dumps({"type": "get_backpack"}))
                
                await self.ws.send(json.dumps({"type": "get_equipped_skin"}))
            else:
                print("服务器认证失败:", auth_result.get("reason", "未知错误"))
                self.online = False
                await self.ws.close()
                return
                
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

    async def handle_websocket_message(self, msg):
        try:
            event = json.loads(msg)
            msg_type = event.get("type")
            
            if msg_type in ["action", "chat"]:
                self.event_queue.put(event)
            elif msg_type == "backpack_info":
                self.handle_backpack_info(event)
            elif msg_type == "market_info":
                self.handle_market_info(event)
            elif msg_type == "purchase_success":
                self.handle_purchase_success(event)
            elif msg_type == "equip_success":
                self.handle_equip_success(event)
            elif msg_type == "skin_data":
                self.handle_skin_data(event)
            elif msg_type == "error":
                self.handle_error(event)
                
        except Exception as e:
            print(f"处理WebSocket消息错误: {e}")
    
    def handle_backpack_info(self, event):
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