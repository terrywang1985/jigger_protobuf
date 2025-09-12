# 桌面宠物客户端 - Protobuf版本

## 项目概述

这是一个基于Protobuf3的桌面宠物游戏客户端，与Go语言编写的游戏服务器进行通信。

## 主要改进

### 1. 协议统一
- **之前**: 客户端使用JSON格式与WebSocket通信
- **现在**: 使用标准的Protobuf二进制协议，与服务器保持一致

### 2. 消息格式规范
- 使用4字节小端序长度头 + Protobuf消息体
- 统一的消息ID和序列号管理
- 标准的错误码处理

### 3. 认证流程
- 使用protobuf的`AuthRequest`和`AuthResponse`
- 支持token验证和session管理
- 完整的用户信息同步

## 文件说明

### 核心文件

1. **test_client.py** - 简单的命令行测试客户端
   - 用于测试与服务器的基本通信
   - 包含认证、用户信息获取、抽卡等功能

2. **protobuf_client_example.py** - 完整的客户端示例
   - 包含GUI界面
   - 演示完整的客户端架构

3. **client11.py** - 原始的客户端（已部分改造）
   - 桌面宠物的完整实现
   - 正在迁移到Protobuf协议

### Protobuf生成文件

- `game_pb2.py` - 游戏基础协议
- `battle_pb2.py` - 战斗相关协议  
- `desktop_pet_pb2.py` - 桌面宠物协议
- `match_pb2.py` - 匹配服务协议
- `room_service_pb2.py` - 房间服务协议
- `game_service_pb2.py` - 游戏服务协议

## 使用方法

### 1. 安装依赖

```bash
pip install websockets protobuf pillow pynput requests
```

### 2. 运行测试客户端

```bash
cd client
python test_client.py
```

### 3. 运行桌面宠物客户端

```bash
cd client  
python client11.py
```

## 协议对比

### 旧协议 (JSON)
```json
{
  "type": "auth",
  "token": "xxx",
  "openid": "xxx"
}
```

### 新协议 (Protobuf)
```protobuf
message AuthRequest {
  string token = 1;
  string protocol_version = 2;
  string client_version = 3;
  string device_type = 4;
  string device_id = 5;
  string app_id = 6;
  string nonce = 7;
  int64 timestamp = 8;
  string signature = 9;
}
```

## 消息流程

### 1. 连接认证
```
客户端 -> 服务器: AuthRequest
服务器 -> 客户端: AuthResponse (包含用户基本信息)
```

### 2. 获取用户详情
```
客户端 -> 服务器: GetUserInfoRequest
服务器 -> 客户端: GetUserInfoResponse (包含完整用户数据)
```

### 3. 游戏操作
```
客户端 -> 服务器: DrawCardRequest
服务器 -> 客户端: DrawCardResponse (返回抽到的卡牌)
```

## 关键类说明

### ProtobufClient
负责Protobuf协议的封装，包括：
- 消息打包/解包
- WebSocket通信管理
- 消息序列号管理

### TestClient  
简单的测试客户端，用于验证：
- 基本连接功能
- 认证流程
- 消息收发

## 服务器兼容性

客户端完全兼容现有的Go服务器：
- 使用相同的Protobuf定义文件
- 遵循相同的消息格式
- 支持相同的业务流程

## 调试建议

1. **连接问题**: 确认服务器运行在`127.0.0.1:18080`
2. **认证问题**: 检查token格式和认证逻辑
3. **协议问题**: 对比protobuf定义文件确保一致性

## 下一步计划

1. 完成`client11.py`的完整迁移
2. 实现桌面宠物特有的功能（皮肤、商城等）
3. 添加更完善的错误处理和重连机制
4. 优化GUI界面和用户体验

## 常见问题

**Q: 为什么要从JSON改为Protobuf？**
A: 
- 更高的性能（二进制格式）
- 更强的类型安全
- 与服务器协议保持一致
- 更好的跨语言支持

**Q: 如何生成新的protobuf文件？**
A:
```bash
cd proto
protoc --python_out=../client *.proto
```

**Q: 如何处理连接断开？**
A: 客户端会自动检测连接状态，可以实现重连机制。