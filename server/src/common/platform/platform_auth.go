// common/platform/platform_auth.go
package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PlatformAuthClient 平台认证客户端
type PlatformAuthClient struct {
	BaseURL           string
	InternalAuthToken string
	HTTPClient        *http.Client
}

// NewPlatformAuthClient 创建新的平台认证客户端
func NewPlatformAuthClient(baseURL, internalAuthToken string) *PlatformAuthClient {
	return &PlatformAuthClient{
		BaseURL:           baseURL,
		InternalAuthToken: internalAuthToken,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// 发送验证码请求
type SendCodeRequest struct {
	CountryCode string `json:"country_code"`
	Phone       string `json:"phone"`
	AppID       string `json:"app_id"`
	DeviceID    string `json:"device_id,omitempty"`
}

// 发送验证码响应
type SendCodeResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expires_in"`
	Error     string `json:"error,omitempty"`
}

// 手机号登录请求
type PhoneLoginRequest struct {
	CountryCode string `json:"country_code"`
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	AppID       string `json:"app_id"`
	DeviceID    string `json:"device_id,omitempty"`
}

// 手机号登录响应
type PhoneLoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
	OpenID  string `json:"openid"`
	User    struct {
		ID          uint   `json:"id"`
		Username    string `json:"username"`
		Phone       string `json:"phone"`
		CountryCode string `json:"country_code"`
	} `json:"user"`
	JTI   string `json:"jti"`
	Error string `json:"error,omitempty"`
}

// 用户名密码注册请求
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	AppID    string `json:"app_id"`
}

// 用户名密码注册响应
type RegisterResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
	OpenID  string `json:"openid"`
	User    struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
	JTI   string `json:"jti"`
	Error string `json:"error,omitempty"`
}

// 用户名密码登录请求
type UsernameLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	AppID    string `json:"app_id"`
	DeviceID string `json:"device_id,omitempty"`
}

// 用户名密码登录响应
type UsernameLoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
	OpenID  string `json:"openid"`
	User    struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
	JTI   string `json:"jti"`
	Error string `json:"error,omitempty"`
}

// Token验证请求
type VerifyTokenRequest struct {
	Token string `json:"token"`
	AppID string `json:"app_id"`
}

// Token验证响应（服务端检查接口）
type TokenValidationResponse struct {
	Valid     bool   `json:"valid"`
	OpenID    string `json:"openid"`
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	AppID     string `json:"app_id"`
	SessionID string `json:"session_id"`
	Exp       int64  `json:"exp"`
	IAT       int64  `json:"iat"`
	JTI       string `json:"jti"`
	Reason    string `json:"reason,omitempty"`
}

// 发送验证码
func (c *PlatformAuthClient) SendCode(req *SendCodeRequest) (*SendCodeResponse, error) {
	return c.makeRequest("POST", "/auth/phone/send-code", req, false)
}

// 手机号登录
func (c *PlatformAuthClient) PhoneLogin(req *PhoneLoginRequest) (*PhoneLoginResponse, error) {
	return c.makeRequest("POST", "/auth/phone/login", req, false)
}

// 用户名密码注册
func (c *PlatformAuthClient) Register(req *RegisterRequest) (*RegisterResponse, error) {
	return c.makeRequest("POST", "/auth/register", req, false)
}

// 用户名密码登录
func (c *PlatformAuthClient) UsernameLogin(req *UsernameLoginRequest) (*UsernameLoginResponse, error) {
	return c.makeRequest("POST", "/auth/login", req, false)
}

// 验证Token（服务端专用，需要内部认证）
func (c *PlatformAuthClient) ValidateToken(token, appID string) (*TokenValidationResponse, error) {
	req := &VerifyTokenRequest{
		Token: token,
		AppID: appID,
	}
	return c.makeRequest("POST", "/auth/check-token", req, true)
}

// makeRequest 通用请求方法
func (c *PlatformAuthClient) makeRequest(method, endpoint string, reqBody interface{}, needInternalAuth bool) (interface{}, error) {
	// 序列化请求体
	var reqData []byte
	var err error
	if reqBody != nil {
		reqData, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request failed: %v", err)
		}
	}

	// 创建HTTP请求
	url := c.BaseURL + endpoint
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	if needInternalAuth {
		req.Header.Set("X-Internal-Auth", c.InternalAuthToken)
	}

	// 发送请求
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %v", err)
	}

	// 根据endpoint解析响应
	switch endpoint {
	case "/auth/phone/send-code":
		var result SendCodeResponse
		if err := json.Unmarshal(respData, &result); err != nil {
			return nil, fmt.Errorf("unmarshal response failed: %v", err)
		}
		if result.Error != "" && resp.StatusCode >= 400 {
			return nil, fmt.Errorf("platform error: %s", result.Error)
		}
		return &result, nil

	case "/auth/phone/login":
		var result PhoneLoginResponse
		if err := json.Unmarshal(respData, &result); err != nil {
			return nil, fmt.Errorf("unmarshal response failed: %v", err)
		}
		if result.Error != "" && resp.StatusCode >= 400 {
			return nil, fmt.Errorf("platform error: %s", result.Error)
		}
		return &result, nil

	case "/auth/register":
		var result RegisterResponse
		if err := json.Unmarshal(respData, &result); err != nil {
			return nil, fmt.Errorf("unmarshal response failed: %v", err)
		}
		if result.Error != "" && resp.StatusCode >= 400 {
			return nil, fmt.Errorf("platform error: %s", result.Error)
		}
		return &result, nil

	case "/auth/login":
		var result UsernameLoginResponse
		if err := json.Unmarshal(respData, &result); err != nil {
			return nil, fmt.Errorf("unmarshal response failed: %v", err)
		}
		if result.Error != "" && resp.StatusCode >= 400 {
			return nil, fmt.Errorf("platform error: %s", result.Error)
		}
		return &result, nil

	case "/auth/check-token":
		var result TokenValidationResponse
		if err := json.Unmarshal(respData, &result); err != nil {
			return nil, fmt.Errorf("unmarshal response failed: %v", err)
		}
		return &result, nil

	default:
		return nil, fmt.Errorf("unknown endpoint: %s", endpoint)
	}
}
