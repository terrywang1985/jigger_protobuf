package main

//player 与room 之间的映射关系
import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"sync"
	"sync/atomic"
)

// 自增的连接计数器
var connCounter uint64

// GenerateConnID 生成连接的唯一 ID
func GenerateConnID(conn Connection) string {
	remoteAddr := conn.RemoteAddr().String() // 获取远程地址 (IP:Port)
	connID := atomic.AddUint64(&connCounter, 1)
	hash := md5.Sum([]byte(remoteAddr + strconv.FormatUint(connID, 10))) // 基于地址和计数生成哈希
	return hex.EncodeToString(hash[:])                                   // 返回字符串格式的哈希值
}

// GenerateShortUUID 生成短版 UUID
func GenerateShortUUID() string {
	uuid := make([]byte, 16)  // 16字节随机数
	_, err := rand.Read(uuid) // 生成随机数
	if err != nil {
		panic("Failed to generate UUID")
	}
	return base64.URLEncoding.EncodeToString(uuid) // 转为 Base64 编码
}

// 全局映射表
var uuidToConn sync.Map

// BindUUIDToConn 绑定 UUID 和连接 ID
func BindUUIDToConn(uuid string, connID string) {
	uuidToConn.Store(connID, uuid) // 存储映射关系
}

// GetUUIDByConn 根据连接 ID 获取 UUID
func GetUUIDByConn(connID string) string {
	value, ok := uuidToConn.Load(connID)
	if ok {
		return value.(string) // 返回绑定的 UUID
	}
	return "" // 未找到
}

// 示例：清理无效映射
func RemoveConnID(connID string) {
	uuidToConn.Delete(connID)
}
