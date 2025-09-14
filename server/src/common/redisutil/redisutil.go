package redisutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
	"google.golang.org/protobuf/proto"
)

// RedisConfig Redis连接配置
type RedisConfig struct {
	Addr     string // Redis地址
	Password string // Redis密码
	DB       int    // Redis数据库
}

// LoadRedisConfigFromEnv 从环境变量加载Redis配置
func LoadRedisConfigFromEnv() *RedisConfig {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPass := os.Getenv("REDIS_PASSWORD")

	redisDB := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil {
			redisDB = db
		}
	}

	return &RedisConfig{
		Addr:     redisAddr,
		Password: redisPass,
		DB:       redisDB,
	}
}

// RedisPool 封装了Redis连接池和常用方法
type RedisPool struct {
	pool *redis.Pool
}

// NewRedisPool 创建新的Redis连接池
func NewRedisPool(server, password string, db int) *RedisPool {
	return &RedisPool{
		pool: &redis.Pool{
			MaxIdle:     100,
			MaxActive:   500,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", server)
				if err != nil {
					return nil, err
				}
				if password != "" {
					if _, err := c.Do("AUTH", password); err != nil {
						c.Close()
						return nil, err
					}
				}
				if _, err := c.Do("SELECT", db); err != nil {
					c.Close()
					return nil, err
				}
				return c, nil
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if time.Since(t) < time.Minute {
					return nil
				}
				_, err := c.Do("PING")
				return err
			},
		},
	}
}

// NewRedisPoolFromConfig 从配置创建新的Redis连接池
func NewRedisPoolFromConfig(config *RedisConfig) *RedisPool {
	return NewRedisPool(config.Addr, config.Password, config.DB)
}

// Close 关闭连接池
func (rp *RedisPool) Close() {
	rp.pool.Close()
}

// Get 获取连接
func (rp *RedisPool) Get() redis.Conn {
	return rp.pool.Get()
}

// GetString 获取字符串值
func (rp *RedisPool) GetString(key string) (string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("redis GET failed: %w", err)
	}
	return value, nil
}

// Set 设置字符串值
func (rp *RedisPool) Set(key, value string) error {
	return rp.SetEx(key, value, 0)
}

// SetEx 设置带过期时间的字符串值
func (rp *RedisPool) SetEx(key, value string, expiration time.Duration) error {
	conn := rp.pool.Get()
	defer conn.Close()

	var err error
	if expiration > 0 {
		_, err = conn.Do("SETEX", key, int(expiration.Seconds()), value)
	} else {
		_, err = conn.Do("SET", key, value)
	}

	if err != nil {
		return fmt.Errorf("redis SET failed: %w", err)
	}
	return nil
}

// GetInt64 获取整数值
func (rp *RedisPool) GetInt64(key string) (int64, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	value, err := redis.Int64(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return 0, ErrKeyNotFound
		}
		return 0, fmt.Errorf("redis GET failed: %w", err)
	}
	return value, nil
}

// Incr 自增操作
func (rp *RedisPool) Incr(key string) (int64, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	value, err := redis.Int64(conn.Do("INCR", key))
	if err != nil {
		return 0, fmt.Errorf("redis INCR failed: %w", err)
	}
	return value, nil
}

// IncrEx 自增操作并设置过期时间
func (rp *RedisPool) IncrEx(key string, expiration time.Duration) (int64, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	// 使用事务确保原子性
	conn.Send("MULTI")
	conn.Send("INCR", key)
	if expiration > 0 {
		conn.Send("EXPIRE", key, int(expiration.Seconds()))
	}
	results, err := redis.Values(conn.Do("EXEC"))
	if err != nil {
		return 0, fmt.Errorf("redis transaction failed: %w", err)
	}

	// 解析结果
	if len(results) < 1 {
		return 0, errors.New("invalid transaction results")
	}

	value, ok := results[0].(int64)
	if !ok {
		return 0, errors.New("invalid INCR result type")
	}

	return value, nil
}

// SetJSON 设置JSON值
func (rp *RedisPool) SetJSON(key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("JSON marshal failed: %w", err)
	}
	return rp.SetEx(key, string(jsonData), expiration)
}

// GetJSON 获取JSON值
func (rp *RedisPool) GetJSON(key string, dest interface{}) error {
	jsonData, err := rp.GetString(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(jsonData), dest)
}

// SetProto 设置Protobuf值
func (rp *RedisPool) SetProto(key string, value proto.Message, expiration time.Duration) error {
	data, err := proto.Marshal(value)
	if err != nil {
		return fmt.Errorf("protobuf marshal failed: %w", err)
	}
	return rp.SetEx(key, string(data), expiration)
}

// GetProto 获取Protobuf值
func (rp *RedisPool) GetProto(key string, dest proto.Message) error {
	data, err := rp.GetString(key)
	if err != nil {
		return err
	}
	return proto.Unmarshal([]byte(data), dest)
}

// Delete 删除键
func (rp *RedisPool) Delete(key string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", key)
	if err != nil {
		return fmt.Errorf("redis DEL failed: %w", err)
	}
	return nil
}

// Exists 检查键是否存在
func (rp *RedisPool) Exists(key string) (bool, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	count, err := redis.Int(conn.Do("EXISTS", key))
	if err != nil {
		return false, fmt.Errorf("redis EXISTS failed: %w", err)
	}
	return count > 0, nil
}

// Expire 设置键的过期时间
func (rp *RedisPool) Expire(key string, expiration time.Duration) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("EXPIRE", key, int(expiration.Seconds()))
	if err != nil {
		return fmt.Errorf("redis EXPIRE failed: %w", err)
	}
	return nil
}

// HSet 设置哈希字段
func (rp *RedisPool) HSet(key, field, value string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("HSET", key, field, value)
	if err != nil {
		return fmt.Errorf("redis HSET failed: %w", err)
	}
	return nil
}

// HGet 获取哈希字段
func (rp *RedisPool) HGet(key, field string) (string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	value, err := redis.String(conn.Do("HGET", key, field))
	if err != nil {
		if err == redis.ErrNil {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("redis HGET failed: %w", err)
	}
	return value, nil
}

// HGetAll 获取所有哈希字段
func (rp *RedisPool) HGetAll(key string) (map[string]string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	values, err := redis.StringMap(conn.Do("HGETALL", key))
	if err != nil {
		return nil, fmt.Errorf("redis HGETALL failed: %w", err)
	}
	return values, nil
}

// HMSet 设置多个哈希字段
func (rp *RedisPool) HMSet(key string, fields map[string]interface{}) error {
	conn := rp.pool.Get()
	defer conn.Close()

	args := redis.Args{}.Add(key)
	for field, value := range fields {
		args = args.Add(field, value)
	}

	_, err := conn.Do("HMSET", args...)
	if err != nil {
		return fmt.Errorf("redis HMSET failed: %w", err)
	}
	return nil
}

// Publish 发布消息
func (rp *RedisPool) Publish(channel, message string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("PUBLISH", channel, message)
	if err != nil {
		return fmt.Errorf("redis PUBLISH failed: %w", err)
	}
	return nil
}

// Subscribe 订阅频道
func (rp *RedisPool) Subscribe(channel string, handler func(string)) error {
	conn := rp.pool.Get()
	defer conn.Close()

	psc := redis.PubSubConn{Conn: conn}
	if err := psc.Subscribe(channel); err != nil {
		return fmt.Errorf("redis SUBSCRIBE failed: %w", err)
	}

	go func() {
		defer psc.Close()
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				handler(string(v.Data))
			case redis.Subscription:
				// 订阅状态变化
			case error:
				return
			}
		}
	}()

	return nil
}

// GenerateBattleID 生成全局唯一战斗ID
func (rp *RedisPool) GenerateBattleID() (string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	// 使用日期前缀 + 序列号
	today := time.Now().Format("20060102")
	key := "global:battle_id:" + today

	// 初始化或递增序列号
	_, err := conn.Do("SETNX", key, 0)
	if err != nil {
		return "", fmt.Errorf("SETNX failed: %w", err)
	}

	id, err := redis.Int64(conn.Do("INCR", key))
	if err != nil {
		return "", fmt.Errorf("INCR failed: %w", err)
	}

	return fmt.Sprintf("%s-%d", today, id), nil
}

// ====================== 新增方法 ====================== //

// ZAdd 向有序集合添加成员
func (rp *RedisPool) ZAdd(key string, score int64, member string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("ZADD", key, score, member)
	if err != nil {
		return fmt.Errorf("redis ZADD failed: %w", err)
	}
	return nil
}

// ZRem 从有序集合移除成员
func (rp *RedisPool) ZRem(key string, member string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("ZREM", key, member)
	if err != nil {
		return fmt.Errorf("redis ZREM failed: %w", err)
	}
	return nil
}

// ZRemRangeByScore 按分数范围移除有序集合成员
func (rp *RedisPool) ZRemRangeByScore(key string, min, max int64) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("ZREMRANGEBYSCORE", key, min, max)
	if err != nil {
		return fmt.Errorf("redis ZREMRANGEBYSCORE failed: %w", err)
	}
	return nil
}

// ZRangeByScore 按分数范围获取有序集合成员
func (rp *RedisPool) ZRangeByScore(key string, min, max int64) ([]string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	members, err := redis.Strings(conn.Do("ZRANGEBYSCORE", key, min, max))
	if err != nil {
		if err == redis.ErrNil {
			return []string{}, nil
		}
		return nil, fmt.Errorf("redis ZRANGEBYSCORE failed: %w", err)
	}
	return members, nil
}

// ZRangeByScoreWithScores 按分数范围获取有序集合成员及其分数
func (rp *RedisPool) ZRangeByScoreWithScores(key string, min, max int64) (map[string]int64, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	values, err := redis.Values(conn.Do("ZRANGEBYSCORE", key, min, max, "WITHSCORES"))
	if err != nil {
		if err == redis.ErrNil {
			return map[string]int64{}, nil
		}
		return nil, fmt.Errorf("redis ZRANGEBYSCORE WITHSCORES failed: %w", err)
	}

	result := make(map[string]int64)
	for i := 0; i < len(values); i += 2 {
		member, ok := values[i].([]byte)
		if !ok {
			continue
		}

		score, err := strconv.ParseInt(string(values[i+1].([]byte)), 10, 64)
		if err != nil {
			continue
		}

		result[string(member)] = score
	}

	return result, nil
}

// SMembers 获取集合所有成员
func (rp *RedisPool) SMembers(key string) ([]string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	members, err := redis.Strings(conn.Do("SMEMBERS", key))
	if err != nil {
		if err == redis.ErrNil {
			return []string{}, nil
		}
		return nil, fmt.Errorf("redis SMEMBERS failed: %w", err)
	}
	return members, nil
}

// SIsMember 检查成员是否在集合中
func (rp *RedisPool) SIsMember(key string, member string) (bool, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	isMember, err := redis.Bool(conn.Do("SISMEMBER", key, member))
	if err != nil {
		return false, fmt.Errorf("redis SISMEMBER failed: %w", err)
	}
	return isMember, nil
}

// SAdd 向集合添加成员
func (rp *RedisPool) SAdd(key string, member string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SADD", key, member)
	if err != nil {
		return fmt.Errorf("redis SADD failed: %w", err)
	}
	return nil
}

// SRem 从集合移除成员
func (rp *RedisPool) SRem(key string, member string) error {
	conn := rp.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SREM", key, member)
	if err != nil {
		return fmt.Errorf("redis SREM failed: %w", err)
	}
	return nil
}

// Keys 查找匹配模式的键
func (rp *RedisPool) Keys(pattern string) ([]string, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	keys, err := redis.Strings(conn.Do("KEYS", pattern))
	if err != nil {
		if err == redis.ErrNil {
			return []string{}, nil
		}
		return nil, fmt.Errorf("redis KEYS failed: %w", err)
	}
	return keys, nil
}

// SetNX 设置键值，仅当键不存在时
func (rp *RedisPool) SetNX(key, value string) (bool, error) {
	conn := rp.pool.Get()
	defer conn.Close()

	result, err := redis.Int(conn.Do("SETNX", key, value))
	if err != nil {
		return false, fmt.Errorf("redis SETNX failed: %w", err)
	}
	return result == 1, nil
}

// 错误定义
var (
	ErrKeyNotFound = errors.New("key not found")
)
