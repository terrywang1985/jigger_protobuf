// common/platform/config.go
package platform

import (
	"os"
)

// PlatformConfig 平台配置
type PlatformConfig struct {
	BaseURL           string // 平台服务基础URL
	InternalAuthToken string // 内部认证Token
	AppID             string // 应用ID
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *PlatformConfig {
	baseURL := os.Getenv("PLATFORM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // 默认API网关地址
	}

	internalAuthToken := os.Getenv("PLATFORM_INTERNAL_TOKEN")
	if internalAuthToken == "" {
		internalAuthToken = "default_internal_token_change_in_production" // 生产环境应修改
	}

	appID := os.Getenv("PLATFORM_APP_ID")
	if appID == "" {
		appID = "jigger_game" // 默认应用ID
	}

	return &PlatformConfig{
		BaseURL:           baseURL,
		InternalAuthToken: internalAuthToken,
		AppID:             appID,
	}
}

// NewAuthClient 创建认证客户端
func (c *PlatformConfig) NewAuthClient() *PlatformAuthClient {
	return NewPlatformAuthClient(c.BaseURL, c.InternalAuthToken)
}
