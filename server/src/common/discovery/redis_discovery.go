package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"common/redisutil"
)

const (
	serviceKeyPrefix = "service:"          // 服务实例集合的键前缀
	heartbeatKey     = "service_heartbeat" // 心跳有序集合
	heartbeatExpire  = 15 * time.Second    // 心跳超时时间
)

type RedisDiscovery struct {
	redis  *redisutil.RedisPool
	prefix string // 键前缀，用于区分环境
}

func NewRedisDiscovery(redisPool *redisutil.RedisPool, prefix string) *RedisDiscovery {
	return &RedisDiscovery{
		redis:  redisPool,
		prefix: prefix,
	}
}

func (r *RedisDiscovery) buildKey(key string) string {
	return r.prefix + key
}

func (r *RedisDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	// 1. 序列化实例数据
	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("序列化实例数据失败: %w", err)
	}

	// 2. 设置实例信息（带过期时间）
	instanceKey := r.buildKey(serviceKeyPrefix + instance.ServiceName + ":" + instance.InstanceID)
	if err := r.redis.SetEx(instanceKey, string(data), heartbeatExpire); err != nil {
		return fmt.Errorf("存储实例信息失败: %w", err)
	}
	slog.Info("注册服务实例", "key", instanceKey, "expire", heartbeatExpire)

	// 3. 将实例ID添加到服务实例集合
	serviceSetKey := r.buildKey(serviceKeyPrefix + instance.ServiceName)
	if err := r.redis.SAdd(serviceSetKey, instance.InstanceID); err != nil {
		return fmt.Errorf("添加到服务集合失败: %w", err)
	}

	// 4. 更新心跳时间
	if err := r.Heartbeat(ctx, instance.InstanceID); err != nil {
		return fmt.Errorf("更新心跳失败: %w", err)
	}

	return nil
}

func (r *RedisDiscovery) Deregister(ctx context.Context, instanceID string) error {
	// 1. 获取实例信息以确定服务名
	instance, err := r.getInstanceByID(instanceID)
	if err != nil {
		if errors.Is(err, redisutil.ErrKeyNotFound) {
			return nil // 实例不存在，无需注销
		}
		return fmt.Errorf("获取实例信息失败: %w", err)
	}

	// 2. 从服务集合中移除实例ID
	serviceSetKey := r.buildKey(serviceKeyPrefix + instance.ServiceName)
	if err := r.redis.SRem(serviceSetKey, instanceID); err != nil {
		return fmt.Errorf("从服务集合移除失败: %w", err)
	}

	// 3. 删除实例数据
	instanceKey := r.buildKey(serviceKeyPrefix + instance.ServiceName + ":" + instanceID)
	if err := r.redis.Delete(instanceKey); err != nil {
		return fmt.Errorf("删除实例数据失败: %w", err)
	}

	// 4. 从心跳集合中移除
	if err := r.redis.ZRem(r.buildKey(heartbeatKey), instanceID); err != nil {
		return fmt.Errorf("从心跳集合移除失败: %w", err)
	}

	return nil
}

func (r *RedisDiscovery) getInstanceByID(instanceID string) (*ServiceInstance, error) {
	// 查找所有服务集合
	serviceKeys, err := r.redis.Keys(r.buildKey(serviceKeyPrefix) + "*")
	if err != nil {
		return nil, fmt.Errorf("查找服务集合失败: %w", err)
	}

	// 遍历所有服务集合
	for _, serviceKey := range serviceKeys {
		// 检查实例是否在集合中
		isMember, err := r.redis.SIsMember(serviceKey, instanceID)
		if err != nil {
			continue
		}
		if !isMember {
			continue
		}

		// 获取实例数据
		instanceKey := r.buildKey(serviceKeyPrefix + serviceKey + ":" + instanceID)
		data, err := r.redis.GetString(instanceKey)
		if err != nil {
			return nil, fmt.Errorf("获取实例数据失败: %w", err)
		}

		var instance ServiceInstance
		if err := json.Unmarshal([]byte(data), &instance); err != nil {
			return nil, fmt.Errorf("解析实例数据失败: %w", err)
		}

		return &instance, nil
	}

	return nil, redisutil.ErrKeyNotFound
}

func (r *RedisDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	// 1. 获取服务所有实例ID
	serviceSetKey := r.buildKey(serviceKeyPrefix + serviceName)
	instanceIDs, err := r.redis.SMembers(serviceSetKey)
	if err != nil {
		return nil, fmt.Errorf("获取服务实例ID失败: %w", err)
	}

	// 2. 清理过期心跳
	if err := r.cleanExpiredHeartbeats(); err != nil {
		return nil, fmt.Errorf("清理过期心跳失败: %w", err)
	}

	// 3. 获取有效实例ID
	validInstanceIDs, err := r.getValidInstanceIDs()
	if err != nil {
		return nil, fmt.Errorf("获取有效实例ID失败: %w", err)
	}

	// 4. 获取实例数据
	var instances []*ServiceInstance
	for _, id := range instanceIDs {
		if !validInstanceIDs[id] {
			continue // 跳过失效实例
		}

		instanceKey := r.buildKey(serviceKeyPrefix + serviceName + ":" + id)
		data, err := r.redis.GetString(instanceKey)
		if err != nil {
			if errors.Is(err, redisutil.ErrKeyNotFound) {
				continue // 实例数据已过期
			}
			return nil, fmt.Errorf("获取实例数据失败: %w", err)
		}

		var instance ServiceInstance
		if err := json.Unmarshal([]byte(data), &instance); err != nil {
			return nil, fmt.Errorf("解析实例数据失败: %w", err)
		}

		instances = append(instances, &instance)
	}

	return instances, nil
}

func (r *RedisDiscovery) cleanExpiredHeartbeats() error {
	now := time.Now().Unix()
	expireTime := now - int64(heartbeatExpire.Seconds())
	return r.redis.ZRemRangeByScore(r.buildKey(heartbeatKey), 0, expireTime)
}

func (r *RedisDiscovery) getValidInstanceIDs() (map[string]bool, error) {
	now := time.Now().Unix()
	expireTime := now - int64(heartbeatExpire.Seconds())

	validIDs, err := r.redis.ZRangeByScore(r.buildKey(heartbeatKey), expireTime, now)
	if err != nil {
		return nil, err
	}

	validInstanceIDs := make(map[string]bool)
	for _, id := range validIDs {
		validInstanceIDs[id] = true
	}

	return validInstanceIDs, nil
}

func (r *RedisDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	ch := make(chan []*ServiceInstance, 10)

	go func() {
		defer close(ch)

		// 初始状态
		lastInstances, _ := r.Discover(ctx, serviceName)
		ch <- lastInstances

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentInstances, err := r.Discover(ctx, serviceName)
				if err != nil {
					continue
				}

				// 检查是否有变化
				if !instancesEqual(lastInstances, currentInstances) {
					ch <- currentInstances
					lastInstances = currentInstances
				}
			}
		}
	}()

	return ch, nil
}

func instancesEqual(a, b []*ServiceInstance) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]*ServiceInstance)
	for _, ins := range a {
		aMap[ins.InstanceID] = ins
	}

	for _, ins := range b {
		if aIns, ok := aMap[ins.InstanceID]; !ok || !instanceEqual(aIns, ins) {
			return false
		}
	}

	return true
}

func instanceEqual(a, b *ServiceInstance) bool {
	return a.InstanceID == b.InstanceID &&
		a.ServiceName == b.ServiceName &&
		a.Address == b.Address
}

func (r *RedisDiscovery) Heartbeat(ctx context.Context, instanceID string) error {
	return r.redis.ZAdd(r.buildKey(heartbeatKey), time.Now().Unix(), instanceID)
}
