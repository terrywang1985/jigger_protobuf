# 🚀 Jigger Protobuf Server 一键部署指南

## 📋 概述

这个Docker Compose配置可以一键启动完整的Jigger Protobuf游戏服务器集群，包括所有必需的服务和依赖。

## 🏗️ 服务架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Python Client │───▶│   Login Server  │───▶│  Platform Auth  │
│                 │    │     :8081       │    │     :8080       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                        │                       │
         │              ┌─────────▼─────────┐            │
         └─────────────▶│   Game Server     │◀───────────┘
                        │  :18080 :12345    │
                        │     :50051        │
                        └─────────┬─────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Battle Server  │    │      Redis      │    │  Match Server   │
│     :50053      │◀──▶│     :6379       │◀──▶│     :50052      │  
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🛠️ 预备条件

### 必需软件
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) 4.0+
- Windows 10/11 或 Linux/macOS
- 至少 4GB 可用内存
- 至少 2GB 可用磁盘空间

### 端口要求
确保以下端口未被占用：
- `6379` - Redis
- `8081` - Login Server 
- `18080` - Game Server WebSocket
- `12345` - Game Server TCP
- `50051` - Game Server gRPC
- `50052` - Match Server gRPC
- `50053` - Battle Server gRPC

## 🚀 快速开始

### 1. 启动所有服务

#### Windows 用户
```cmd
# 双击运行或在命令行执行
start-all.bat
```

#### Linux/macOS 用户
```bash
chmod +x start-all.sh
./start-all.sh
```

#### 手动启动
```bash
docker-compose up -d --build
```

### 2. 验证服务启动

运行健康检查：
```cmd
health-check.bat
```

或查看服务状态：
```bash
docker-compose ps
```

### 3. 查看服务日志

#### 使用日志查看器 (Windows)
```cmd
logs.bat
```

#### 手动查看日志
```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f game-server
docker-compose logs -f login-server
```

## 📊 服务详情

| 服务名 | 容器名 | 端口映射 | 健康检查 | 描述 |
|--------|--------|----------|----------|------|
| Redis | jigger-redis | 6379:6379 | ✅ | 数据存储 |
| Login Server | jigger-login-server | 8081:8081 | ✅ | 登录认证 |
| Game Server | jigger-game-server | 18080:18080<br>12345:12345<br>50051:50051 | ✅ | 游戏主服务 |
| Battle Server | jigger-battle-server | 50053:50053 | ❌ | 战斗逻辑 |
| Match Server | jigger-match-server | 50052:50052 | ❌ | 匹配服务 |

## 🔧 环境配置

### 环境变量

#### Login Server
```yaml
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
PLATFORM_API=http://host.docker.internal:8080/auth/check-token
PORT=8081
```

#### Game Server
```yaml
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
PLATFORM_BASE_URL=http://host.docker.internal:8080
PLATFORM_INTERNAL_TOKEN=default_internal_token_change_in_production
PLATFORM_APP_ID=jigger_game
```

#### Battle/Match Server
```yaml
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
GRPC_PORT=50053/50052
```

### 自定义配置

编辑 `docker-compose.yml` 文件中的 `environment` 部分：

```yaml
environment:
  - REDIS_ADDR=redis:6379
  - CUSTOM_SETTING=your_value
```

## 🧪 测试客户端

### 1. 手机号验证码登录
```bash
cd client
python phone_auth_client.py
```

### 2. 用户名密码登录
```bash
cd client  
python username_auth_client.py
```

### 测试流程
1. 启动平台服务（platform目录）
2. 启动游戏服务器（本docker-compose）
3. 运行Python客户端进行测试

## 🚨 故障排除

### 常见问题

#### 1. 端口冲突
**症状**: 服务启动失败，提示端口已被占用
**解决**: 
```bash
# 查看端口占用
netstat -an | findstr :8081
# 停止占用端口的进程或修改docker-compose.yml中的端口映射
```

#### 2. Redis连接失败
**症状**: 应用日志显示Redis连接错误
**解决**:
```bash
# 检查Redis容器状态
docker logs jigger-redis
# 重启Redis服务
docker-compose restart redis
```

#### 3. 内存不足
**症状**: 容器频繁重启或OOM错误
**解决**:
```bash
# 查看资源使用
docker stats
# 增加Docker内存限制或关闭其他应用
```

#### 4. 镜像构建失败
**症状**: 构建过程中出现Go编译错误
**解决**:
```bash
# 清理Docker缓存
docker system prune -a
# 重新构建
docker-compose build --no-cache
```

### 调试命令

```bash
# 进入容器调试
docker exec -it jigger-game-server sh

# 查看容器日志
docker logs jigger-game-server -f

# 检查网络连接
docker network inspect jigger_jigger-network

# 测试服务间连通性
docker exec jigger-game-server ping redis
```

## 📈 性能监控

### 实时监控
```bash
# 查看资源使用情况
docker stats

# 监控Redis性能
docker exec jigger-redis redis-cli info stats

# 查看网络连接
docker exec jigger-game-server netstat -tulpn
```

### 日志分析
```bash
# 查看错误日志
docker-compose logs | grep -i error

# 统计连接数
docker exec jigger-redis redis-cli info clients

# 监控游戏服务器连接
docker logs jigger-game-server | grep -i "connection"
```

## 🔒 安全建议

### 生产环境配置

1. **修改默认密码和Token**
```yaml
environment:
  - REDIS_PASSWORD=your_secure_password
  - PLATFORM_INTERNAL_TOKEN=your_secure_token
```

2. **限制网络访问**
```yaml
networks:
  jigger-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

3. **资源限制**
```yaml
deploy:
  resources:
    limits:
      cpus: '0.50'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 256M
```

## 🛠️ 维护操作

### 日常维护
```bash
# 停止所有服务
docker-compose down

# 更新服务
docker-compose pull
docker-compose up -d

# 清理未使用的资源
docker system prune -f

# 备份Redis数据
docker exec jigger-redis redis-cli save
```

### 数据备份
```bash
# 创建Redis数据备份
docker run --rm --volumes-from jigger-redis -v $(pwd):/backup alpine tar czf /backup/redis-backup.tar.gz /data

# 恢复Redis数据
docker run --rm --volumes-from jigger-redis -v $(pwd):/backup alpine tar xzf /backup/redis-backup.tar.gz
```

## 📚 相关文档

- [Docker Compose 官方文档](https://docs.docker.com/compose/)
- [Redis 配置文档](https://redis.io/topics/config)
- [Go 应用容器化最佳实践](https://docs.docker.com/language/golang/)

## 🤝 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📝 更新日志

### v1.0.0 (2025-09-13)
- ✨ 初始版本发布
- 🚀 支持一键启动所有服务
- 📊 完整的健康检查机制
- 🔧 环境变量配置支持
- 📚 详细的文档和故障排除指南

---

**需要帮助？** 请查看故障排除部分或提交Issue！