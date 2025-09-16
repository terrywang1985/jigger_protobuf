# 游客登录功能说明 (新架构版)

## 概述

游客登录功能允许用户在不注册正式账号的情况下，直接体验游戏。后续用户可以选择注册正式账号并绑定游客数据。

## 架构设计

### 设计理念
1. **先体验，后绑定**：用户可以先体验游戏，然后再选择是否绑定正式账号
2. **LoginServer控制**：游客登录仍然经过LoginServer，便于进行资源分配和控制
3. **无token认证**：GameServer基于设备ID直接认证，不依赖token
4. **平台独立**：游客登录完全绕过平台认证服务

### 登录流程

```
客户端 -> LoginServer (游客登录) -> 返回游客信息(无token) -> 客户端直接连GameServer -> 基于设备ID认证
```

## 主要修改

### LoginServer
- `processGuestLogin` 函数不返回 session_id
- 可以在此处控制游客资源分配

### GameServer  
- `HandleAuthRequest` 检测空token，调用游客认证
- `handleGuestAuth` 基于设备ID直接认证

### 客户端
- 游客登录后直接连接GameServer
- 认证时不发送token，只发送设备ID

## 测试步骤

1. 启动服务：Redis, LoginServer, GameServer
2. 运行: `python client/guest_auth_client.py`

## 优势

1. 经过LoginServer可以做控制和资源分配
2. GameServer无token认证更简单直接
3. 为后续扩展留出空间