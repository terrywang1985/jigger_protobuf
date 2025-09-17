# 桌面宠物消息协议修正说明

## 问题发现

在之前的实现中，我错误地使用了JSON格式直接发送WebSocket消息，但实际上服务器只接受protobuf格式的消息。

## 服务器协议分析

根据服务器代码分析：

1. **WebSocket处理器**: `ws_handler.go` 中的 `handleWebSocket` 函数最终调用的是与TCP相同的 `handleConnection` 
2. **消息格式**: 所有消息都必须是 `4字节长度头 + protobuf Message` 格式
3. **消息注册**: 服务器在 `msghandle.go` 中只注册了特定的protobuf消息处理器
4. **无JSON支持**: 服务器代码中没有任何JSON消息解析逻辑

## 修正方案

### 原来的错误实现（已修正）
```python
# 错误：直接发送JSON
await self.client.ws.send(json.dumps({
    "type": "pet_action", 
    "data": action_data
}))
```

### 正确的实现
```python
# 正确：使用protobuf Message格式
action_data = {
    "type": "pet_move",
    "player_id": str(self.player_id),
    "position_x": self.root.winfo_x(),
    "position_y": self.root.winfo_y(),
    "timestamp": int(time.time() * 1000)
}

# 转换为bytes并通过protobuf Message发送
action_bytes = json.dumps(action_data).encode('utf-8')
await self.client.protobuf_client.send_raw_message(
    game_pb.MessageId.GAME_ACTION_NOTIFICATION, 
    action_bytes
)
```

## 技术细节

### 1. 消息封装
- 使用 `GAME_ACTION_NOTIFICATION` 消息ID
- 将JSON格式的动作数据作为bytes放入protobuf Message的data字段
- 保持与现有服务器协议的兼容性

### 2. 新增方法
在 `ProtobufClient` 类中添加了 `send_raw_message` 方法：
```python
async def send_raw_message(self, msg_id, raw_data):
    """发送原始数据消息（用于发送JSON或其他格式的数据）"""
    # 直接使用原始数据
    data = raw_data if isinstance(raw_data, bytes) else raw_data.encode('utf-8')
    
    # 创建消息
    message = self.create_message(msg_id, data)
    
    # 打包并发送
    packed_data = self.pack_message(message)
    await self.ws.send(packed_data)
```

### 3. 消息处理
在客户端添加了对 `GAME_ACTION_NOTIFICATION` 的处理：
```python
async def handle_game_action_notification(self, message):
    # 解析JSON数据
    action_json = message.data.decode('utf-8')
    action_data = json.loads(action_json)
    
    action_type = action_data.get("type")
    if action_type == "pet_chat":
        # 处理聊天消息
    elif action_type == "pet_move":
        # 处理移动动作
```

## 数据格式

### 动作消息格式
```json
{
    "type": "pet_move",
    "player_id": "12345",
    "position_x": 100,
    "position_y": 200,
    "timestamp": 1609459200000
}
```

### 聊天消息格式
```json
{
    "type": "pet_chat",
    "player_id": "12345",
    "player_name": "用户12345",
    "chat_text": "Hello World!",
    "position_x": 100,
    "position_y": 200,
    "timestamp": 1609459200000
}
```

## 服务器端支持

要让服务器支持桌面宠物同步，需要在服务器的消息处理器中添加对 `GAME_ACTION_NOTIFICATION` 的处理逻辑：

```go
func (p *Player) HandleGameActionNotification(msg *pb.Message) {
    // 解析JSON数据
    var actionData map[string]interface{}
    if err := json.Unmarshal(msg.GetData(), &actionData); err != nil {
        return
    }
    
    actionType := actionData["type"].(string)
    
    switch actionType {
    case "pet_move":
        // 广播移动动作给其他玩家
        p.broadcastToOthers(msg)
    case "pet_chat":
        // 广播聊天消息给其他玩家
        p.broadcastToOthers(msg)
    }
}
```

## 优势

1. **协议兼容**: 与现有服务器架构完全兼容
2. **扩展性好**: 可以在JSON数据中添加更多字段
3. **类型安全**: 利用了protobuf的消息验证机制
4. **性能优化**: 避免了协议解析的复杂性

## 注意事项

1. **服务器支持**: 服务器需要添加对 `GAME_ACTION_NOTIFICATION` 的处理逻辑
2. **消息广播**: 服务器需要实现消息广播机制，将动作/聊天消息转发给其他在线玩家
3. **错误处理**: 需要处理JSON解析错误和消息格式错误
4. **性能考虑**: 频繁的动作消息可能需要限流机制

这次修正确保了客户端完全遵循服务器的protobuf协议要求，同时保持了桌面宠物多人互动功能的完整性。