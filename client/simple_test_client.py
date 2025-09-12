#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
Simple test client for protobuf communication
"""

import asyncio
import websockets
import struct
import uuid
import time
import sys
import os

# Add current directory to Python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

# Import protobuf generated classes
import game_pb2 as game_pb


class SimpleTestClient:
    """Simple test client"""
    
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
        """Create protobuf message"""
        message = game_pb.Message()
        message.clientId = self.client_id
        message.msgSerialNo = self.get_next_serial_no()
        message.id = msg_id
        message.data = data
        return message
    
    def pack_message(self, message):
        """Pack message (4-byte length header + protobuf data)"""
        data = message.SerializeToString()
        length = struct.pack('<I', len(data))  # Little-endian 4-byte length
        return length + data
    
    async def send_message(self, msg_id, proto_data):
        """Send protobuf message"""
        if not self.ws:
            print("WebSocket connection not established")
            return False
            
        try:
            # Serialize protobuf data
            data = proto_data.SerializeToString() if proto_data else b''
            
            # Create message
            message = self.create_message(msg_id, data)
            
            # Pack and send
            packed_data = self.pack_message(message)
            await self.ws.send(packed_data)
            
            print(f"[SEND] Message: {msg_id}, Data length: {len(data)}")
            return True
            
        except Exception as e:
            print(f"[ERROR] Failed to send message: {e}")
            return False
    
    def unpack_message(self, data):
        """Unpack message"""
        if len(data) < 4:
            return None, data
            
        # Read length
        length = struct.unpack('<I', data[:4])[0]
        
        # Check if we have complete message
        if len(data) < 4 + length:
            return None, data
            
        # Parse message
        message_data = data[4:4+length]
        remaining_data = data[4+length:]
        
        try:
            message = game_pb.Message()
            message.ParseFromString(message_data)
            return message, remaining_data
        except Exception as e:
            print(f"[ERROR] Failed to parse message: {e}")
            return None, remaining_data
    
    async def connect(self, uri):
        """Connect to server"""
        try:
            print(f"[INFO] Connecting to server: {uri}")
            self.ws = await websockets.connect(uri)
            print("[INFO] Connected to server")
            return True
            
        except Exception as e:
            print(f"[ERROR] Connection failed: {e}")
            return False
    
    async def send_auth_request(self, token="test_token"):
        """Send authentication request"""
        print("[INFO] Sending authentication request")
        
        auth_request = game_pb.AuthRequest()
        auth_request.token = token
        auth_request.protocol_version = "1.0"
        auth_request.client_version = "1.0.0"
        auth_request.device_type = "desktop"
        auth_request.device_id = str(uuid.uuid4())[:8]
        auth_request.app_id = "desktop_app"
        auth_request.nonce = str(uuid.uuid4())
        auth_request.timestamp = int(time.time() * 1000)
        auth_request.signature = ""  # Simplified, no signature calculation
        
        success = await self.send_message(game_pb.MessageId.AUTH_REQUEST, auth_request)
        if success:
            print("[INFO] Authentication request sent, waiting for response...")
        return success
    
    async def listen_once(self, timeout=5):
        """Listen for one message"""
        try:
            msg = await asyncio.wait_for(self.ws.recv(), timeout=timeout)
            
            if isinstance(msg, bytes):
                self.message_buffer += msg
            else:
                self.message_buffer += msg.encode('utf-8')
            
            # Process messages in buffer
            messages_processed = 0
            while True:
                message, self.message_buffer = self.unpack_message(self.message_buffer)
                if message is None:
                    break
                await self.handle_message(message)
                messages_processed += 1
            
            return messages_processed > 0
            
        except asyncio.TimeoutError:
            print(f"[WARN] Message timeout ({timeout}s)")
            return False
        except Exception as e:
            print(f"[ERROR] Error receiving message: {e}")
            return False
    
    async def handle_message(self, message):
        """Handle received message"""
        msg_id = message.id
        print(f"[RECV] Message ID: {msg_id}")
        
        try:
            if msg_id == game_pb.MessageId.AUTH_RESPONSE:
                await self.handle_auth_response(message)
            elif msg_id == game_pb.MessageId.GET_USER_INFO_RESPONSE:
                await self.handle_user_info_response(message)
            elif msg_id == game_pb.MessageId.DRAW_CARD_RESPONSE:
                await self.handle_draw_card_response(message)
            else:
                print(f"[INFO] Unhandled message type: {msg_id}")
                
        except Exception as e:
            print(f"[ERROR] Error handling message: {e}")
    
    async def handle_auth_response(self, message):
        """Handle authentication response"""
        print("[INFO] Authentication response received")
        
        auth_response = game_pb.AuthResponse()
        auth_response.ParseFromString(message.data)
        
        if auth_response.ret == game_pb.ErrorCode.OK:
            self.authenticated = True
            self.uid = auth_response.uid
            print("[SUCCESS] Authentication successful!")
            print(f"  UID: {auth_response.uid}")
            print(f"  Nickname: {auth_response.nickname}")
            print(f"  Gold: {auth_response.gold}")
            print(f"  Diamond: {auth_response.diamond}")
            print(f"  Level: {auth_response.level}")
            print(f"  Exp: {auth_response.exp}")
            
        else:
            print(f"[ERROR] Authentication failed: {auth_response.ret}")
            if auth_response.error_msg:
                print(f"  Error message: {auth_response.error_msg}")
    
    async def handle_user_info_response(self, message):
        """Handle user info response"""
        print("[INFO] User info response received")
        
        user_info_response = game_pb.GetUserInfoResponse()
        user_info_response.ParseFromString(message.data)
        
        if user_info_response.ret == game_pb.ErrorCode.OK:
            user_info = user_info_response.user_info
            print("[SUCCESS] User info retrieved:")
            print(f"  Name: {user_info.name}")
            print(f"  Exp: {user_info.exp}")
            print(f"  Gold: {user_info.gold}")
            print(f"  Diamond: {user_info.diamond}")
            print(f"  Draw card count: {user_info.draw_card_count}")
            print(f"  Backpack cards: {len(user_info.backpack.cards)}")
            
        else:
            print(f"[ERROR] Failed to get user info: {user_info_response.ret}")
    
    async def handle_draw_card_response(self, message):
        """Handle draw card response"""
        print("[INFO] Draw card response received")
        
        draw_card_response = game_pb.DrawCardResponse()
        draw_card_response.ParseFromString(message.data)
        
        if draw_card_response.ret == game_pb.ErrorCode.OK:
            print(f"[SUCCESS] Draw card successful! Got {len(draw_card_response.cards)} cards:")
            for i, card in enumerate(draw_card_response.cards, 1):
                print(f"  {i}. {card.name} (Rarity: {card.rarity}, ID: {card.id})")
        else:
            print(f"[ERROR] Draw card failed: {draw_card_response.ret}")
    
    async def request_user_info(self):
        """Request user info"""
        if not self.authenticated:
            print("[ERROR] Not authenticated, cannot request user info")
            return False
            
        print("[INFO] Requesting user info")
        user_info_request = game_pb.GetUserInfoRequest()
        user_info_request.uid = self.uid
        
        return await self.send_message(game_pb.MessageId.GET_USER_INFO_REQUEST, user_info_request)
    
    async def request_draw_card(self, count=1):
        """Request draw card"""
        if not self.authenticated:
            print("[ERROR] Not authenticated, cannot draw card")
            return False
            
        print(f"[INFO] Requesting to draw {count} card(s)")
        draw_card_request = game_pb.DrawCardRequest()
        draw_card_request.uid = self.uid
        draw_card_request.count = count
        
        return await self.send_message(game_pb.MessageId.DRAW_CARD_REQUEST, draw_card_request)
    
    async def close(self):
        """Close connection"""
        if self.ws:
            await self.ws.close()
            print("[INFO] Connection closed")


async def main():
    """Main test function"""
    print("=== Desktop Pet Client - Protobuf Communication Test ===\n")
    
    client = SimpleTestClient()
    
    # Connect to server
    if not await client.connect("ws://127.0.0.1:18080/ws"):
        return
    
    try:
        # 1. Send authentication request
        if await client.send_auth_request():
            # Wait for authentication response
            await client.listen_once()
        
        # 2. If authentication successful, request user info
        if client.authenticated:
            if await client.request_user_info():
                await client.listen_once()
            
            # 3. Try to draw card
            if await client.request_draw_card(1):
                await client.listen_once()
                
            # 4. Get user info again to see changes
            if await client.request_user_info():
                await client.listen_once()
        
    except KeyboardInterrupt:
        print("\n[INFO] Test interrupted by user")
    except Exception as e:
        print(f"\n[ERROR] Error during test: {e}")
    finally:
        await client.close()


if __name__ == "__main__":
    asyncio.run(main())