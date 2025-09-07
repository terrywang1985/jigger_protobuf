// discovery/discovery.go
package discovery

import (
	"context"
	"time"
)

// ServiceInstance 表示一个服务实例
type ServiceInstance struct {
	ServiceName string            // 服务名称 (如 "battle-server")
	InstanceID  string            // 实例唯一ID
	Address     string            // 服务地址 (host:port)
	Metadata    map[string]string // 元数据
	LastSeen    time.Time         // 最后活跃时间
}

// Discovery 服务发现接口
type Discovery interface {
	// 注册服务实例
	Register(ctx context.Context, instance *ServiceInstance) error
	// 注销服务实例
	Deregister(ctx context.Context, instanceID string) error
	// 发现服务实例
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// 监听服务变化
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)
	// 发送心跳
	Heartbeat(ctx context.Context, instanceID string) error
}
