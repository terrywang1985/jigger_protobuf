// loginserver/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"common/redisutil" // 根据实际路径修改
)

// 配置信息
type Config struct {
	Port         string `json:"port"`
	RedisAddr    string `json:"redis_addr"`
	RedisPass    string `json:"redis_pass"`
	RedisDB      int    `json:"redis_db"`
	PlatformAPI  string `json:"platform_api"`
	GatewayLBURL string `json:"gateway_lb_url"`
}

// 平台认证请求
type PlatformAuthRequest struct {
	Token string `json:"token"`
	AppID string `json:"app_id"`
}

// 平台认证响应
type PlatformAuthResponse struct {
	Valid    bool   `json:"valid"`
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	OpenID   string `json:"openid"` // 添加OpenID字段
	Error    string `json:"error,omitempty"`
}

// 登录请求
type LoginRequest struct {
	Token string `json:"token"` // 移除OpenID字段
	AppID string `json:"app_id"`
}

// 登录响应
type LoginResponse struct {
	Success    bool   `json:"success"`
	GatewayURL string `json:"gateway_url,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	UserID     uint64 `json:"user_id,omitempty"`
	Username   string `json:"username,omitempty"`
	OpenID     string `json:"openid,omitempty"` // 添加OpenID到响应
	Error      string `json:"error,omitempty"`
	ExpiresIn  int64  `json:"expires_in,omitempty"`
}

// Redis session 结构
type SessionData struct {
	UserID    uint64 `json:"user_id"`
	OpenID    string `json:"openid"`
	Username  string `json:"username"`
	LoginTime int64  `json:"login_time"`
	ExpiresAt int64  `json:"expires_at"`
	AppID     string `json:"app_id"`
}

// LoginServer 结构
type LoginServer struct {
	config *Config
	redis  *redisutil.RedisPool
	router *gin.Engine
}

func main() {
	// 加载配置
	config := &Config{
		Port:         "8081",
		RedisAddr:    "localhost:6379",
		RedisPass:    "",
		RedisDB:      0,
		PlatformAPI:  "http://localhost:8080/auth/check-token", // 修正为连字符
		GatewayLBURL: "gateway.example.com:9000",
	}

	// 初始化Redis连接池
	redisPool := redisutil.NewRedisPool(config.RedisAddr, config.RedisPass, config.RedisDB)

	// 测试Redis连接
	if err := testRedisConnection(redisPool); err != nil {
		log.Fatalf("Redis connection test failed: %v", err)
	}

	// 创建LoginServer
	server := &LoginServer{
		config: config,
		redis:  redisPool,
		router: gin.Default(),
	}

	// 设置路由
	server.router.POST("/login", server.handleLogin)
	server.router.GET("/health", server.handleHealthCheck)

	// 启动服务器
	log.Printf("LoginServer starting on port %s", config.Port)
	if err := server.router.Run(":" + config.Port); err != nil {
		log.Fatalf("Failed to start LoginServer: %v", err)
	}
}

// 测试Redis连接
func testRedisConnection(redis *redisutil.RedisPool) error {
	// 使用Exists命令测试连接
	_, err := redis.Exists("test_connection")
	if err != nil {
		return fmt.Errorf("redis connection failed: %v", err)
	}
	return nil
}

// 处理登录请求
func (s *LoginServer) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// 验证平台token
	userInfo, err := s.validatePlatformToken(req.Token, req.AppID)
	if err != nil || userInfo == nil || !userInfo.Valid {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Error:   "Platform token validation failed: " + err.Error(),
		})
		return
	}

	// 生成session ID
	sessionID := uuid.New().String()

	// 创建session数据
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	sessionData := SessionData{
		UserID:    userInfo.UserID,
		OpenID:    userInfo.OpenID, // 使用从平台返回的OpenID
		Username:  userInfo.Username,
		LoginTime: now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		AppID:     req.AppID,
	}

	// 存储session到Redis
	if err := s.storeSession(sessionID, sessionData); err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Error:   "Failed to create session: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, LoginResponse{
		Success:    true,
		GatewayURL: s.config.GatewayLBURL,
		SessionID:  sessionID,
		UserID:     userInfo.UserID,
		Username:   userInfo.Username,
		OpenID:     userInfo.OpenID, // 返回OpenID给客户端
		ExpiresIn:  86400,           // 24小时
	})
}

// 验证平台token
func (s *LoginServer) validatePlatformToken(token, appid string) (*PlatformAuthResponse, error) {
	// 创建HTTP客户端
	client := &http.Client{Timeout: 5 * time.Second}

	// 准备请求数据
	authReq := PlatformAuthRequest{
		Token: token,
		AppID: appid,
	}

	// 序列化请求数据
	jsonData, err := json.Marshal(authReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth request: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", s.config.PlatformAPI, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth service returned status: %s", resp.Status)
	}

	// 解析响应
	var authResp PlatformAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if !authResp.Valid {
		return &authResp, fmt.Errorf("invalid token: %s", authResp.Error)
	}

	return &authResp, nil
}

// 存储session到Redis
func (s *LoginServer) storeSession(sessionID string, data SessionData) error {
	// 使用RedisPool的SetJSON方法存储session数据
	key := fmt.Sprintf("session:%s", sessionID)
	expiration := 24 * time.Hour

	if err := s.redis.SetJSON(key, data, expiration); err != nil {
		return fmt.Errorf("failed to store session in Redis: %v", err)
	}

	return nil
}

// 健康检查处理
func (s *LoginServer) handleHealthCheck(c *gin.Context) {
	// 检查Redis连接 - 使用Exists方法
	_, err := s.redis.Exists("health_check_test")
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"error":   "Redis connection failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"redis":  "connected",
	})
}

// 添加一个优雅关闭的函数
func (s *LoginServer) shutdown() {
	log.Println("Shutting down LoginServer...")

	// 关闭Redis连接池
	if s.redis != nil {
		s.redis.Close()
	}

	log.Println("LoginServer shutdown complete")
}
