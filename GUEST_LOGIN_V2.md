# 游客登录功能说明 (迭代版本)

## 概述

游客登录功能允许用户在不注册正式账号的情况下，直接体验游戏。本次迭代基于用户反馈，优化了架构设计。

## 关键修改

### 1. 移除GuestLoginRequest
- 不再需要单独的GuestLoginRequest proto消息
- 使用AuthRequest的`is_guest`字段标识游客登录

### 2. LoginServer返回sessionid
- `processGuestLogin`函数现在返回sessionid，保持流程通用
- 便于后续扩展和统一管理

### 3. GameServer支持is_guest字段
- `HandleAuthRequest`检测`is_guest`字段而不是空token
- 支持有token和无token两种游客认证模式

## 新架构流程

```
客户端 
  ↓ 1. 调用/guest-login
LoginServer  
  ↓ 2. 返回sessionid（保持流程通用）
客户端
  ↓ 3. 连接GameServer，AuthRequest.is_guest=true
GameServer
  ↓ 4. 检测is_guest字段，调用handleGuestAuth
游戏中...
```

## 主要代码修改

### LoginServer
- `processGuestLogin`：现在创建并返回sessionid
- 保持与正常登录流程的一致性

### GameServer  
- `HandleAuthRequest`：检测`req.GetIsGuest()`而不是空token
- `handleGuestAuth`：支持有token和无token两种模式

### 客户端
- 使用`is_guest: true`字段而不是单独接口
- 适配有sessionid的返回

## 优势

1. **流程统一**：游客和正常用户都有sessionid，便于管理
2. **扩展性强**：LoginServer可以对游客做资源控制
3. **兼容性好**：支持有token和无token两种游客认证
4. **proto简化**：减少不必要的消息类型

## 测试步骤

1. 启动Redis、LoginServer、GameServer
2. 运行：`python client/guest_auth_client.py`
3. 观察日志确认is_guest字段生效

## 下一步扩展

1. 在游戏内实现账号绑定功能
2. 优化游客资源分配策略
3. 添加游客数据清理机制