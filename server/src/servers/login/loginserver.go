// loginserver/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

// 服务器向平台认证请求
type PlatformAuthRequest struct {
	Token string `json:"token"`
	AppID string `json:"app_id"`
}

// 平台认证响应
type PlatformAuthResponse struct {
	Valid    bool   `json:"valid"`
	Username string `json:"username"`
	OpenID   string `json:"openid"` // 添加OpenID字段
	Error    string `json:"error,omitempty"`
}

// 客户端登录请求
type LoginRequest struct {
	Token    string `json:"token"` // 移除OpenID字段
	AppID    string `json:"app_id"`
	IsGuest  bool   `json:"is_guest"`  // 是否为游客登录
	DeviceID string `json:"device_id"` // 设备ID（游客登录需要）
}

// 客户端登录响应
type LoginResponse struct {
	Success    bool   `json:"success"`
	GatewayURL string `json:"gateway_url,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Username   string `json:"username,omitempty"`
	OpenID     string `json:"openid,omitempty"` // 添加OpenID到响应
	Error      string `json:"error,omitempty"`
	ExpiresIn  int64  `json:"expires_in,omitempty"`
}

// Redis session 结构
type SessionData struct {
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
	// 从环境变量加载配置
	redisConfig := redisutil.LoadRedisConfigFromEnv()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	platformAPI := os.Getenv("PLATFORM_API")
	if platformAPI == "" {
		platformAPI = "http://localhost:8080/auth/check-token"
	}

	config := &Config{
		Port:         port,
		RedisAddr:    redisConfig.Addr,
		RedisPass:    redisConfig.Password,
		RedisDB:      redisConfig.DB,
		PlatformAPI:  platformAPI,
		GatewayLBURL: "ws://127.0.0.1:18080/ws",
	}

	// 初始化Redis连接池
	redisPool := redisutil.NewRedisPoolFromConfig(redisConfig)

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
	server.router.POST("/login", server.handleLogin) // 统一登录接口，支持普通用户和游客
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

// 处理登录请求（统一接口，支持普通用户和游客）
func (s *LoginServer) handleLogin(c *gin.Context) {
	log.Printf("Received login request")
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// 根据 is_guest 参数判断登录类型
	if req.IsGuest {
		// 游客登录流程
		s.processGuestLogin(c, req)
		return
	}

	// 普通用户登录流程
	s.processNormalLogin(c, req)
}

// 处理普通用户登录逻辑
func (s *LoginServer) processNormalLogin(c *gin.Context, req LoginRequest) {
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
		Username:   userInfo.Username,
		OpenID:     userInfo.OpenID, // 返回OpenID给客户端
		ExpiresIn:  86400,           // 24小时
	})
}
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

	// 添加内部认证头部（用于访问platform的内部API）
	internalToken := os.Getenv("SHARED_INTERNAL_TOKEN")
	if internalToken != "" {
		req.Header.Set("X-Internal-Auth", internalToken)
	}

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

// 处理游客登录逻辑（修改：返回游客认证信息，而不是session token）
func (s *LoginServer) processGuestLogin(c *gin.Context, req LoginRequest) {
	log.Printf("Processing guest login for device: %s", req.DeviceID)

	// 验证设备ID
	if req.DeviceID == "" {
		log.Printf("Guest login failed: empty device ID")
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Error:   "Device ID is required for guest login",
		})
		return
	}

	// 直接在本地创建游客身份，不与平台交互
	guestOpenID := "guest_" + req.DeviceID
	guestUsername := "guest_" + req.DeviceID
	log.Printf("Created guest identity: username=%s, openid=%s", guestUsername, guestOpenID)

	// 生成session ID用于保持流程通用
	sessionID := uuid.New().String()
	log.Printf("Generated session ID: %s", sessionID)

	// 创建游客session数据
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	sessionData := SessionData{
		OpenID:    guestOpenID,
		Username:  guestUsername,
		LoginTime: now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		AppID:     req.AppID,
	}
	log.Printf("Created session data: %+v", sessionData)

	// 存储session到Redis
	log.Printf("Storing session to Redis...")
	if err := s.storeSession(sessionID, sessionData); err != nil {
		log.Printf("Failed to store session: %v", err)
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Error:   "Failed to create session: " + err.Error(),
		})
		return
	}
	log.Printf("Successfully stored session to Redis")

	// 可以在这里做一些游客登录的控制逻辑
	// 比如：分配较少的资源、设置不同的限制等
	log.Printf("Guest login for device: %s, allocated resources: limited", req.DeviceID)

	// 返回游客认证信息（现在包含SessionID以保持流程通用）
	response := LoginResponse{
		Success:    true,
		GatewayURL: s.config.GatewayLBURL,
		SessionID:  sessionID, // 返回sessionID保持流程通用
		Username:   guestUsername,
		OpenID:     guestOpenID,
		ExpiresIn:  86400, // 24小时
	}
	log.Printf("Sending successful response: %+v", response)
	c.JSON(http.StatusOK, response)
}
