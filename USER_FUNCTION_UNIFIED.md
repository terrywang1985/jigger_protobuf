# 用户创建函数统一优化

## 问题分析

原先的代码中存在两个几乎相同的函数：
- `findOrCreateUserByOpenID`: 用于正常用户
- `findOrCreateGuestUserLocal`: 用于游客用户

经过仔细分析，发现这两个函数的核心逻辑完全相同，只有很细微的差别，这违反了DRY（Don't Repeat Yourself）原则。

## 统一的好处

### 1. **减少代码重复**
- 消除了几乎完全相同的代码逻辑
- 减少了代码维护成本
- 降低了出错的可能性

### 2. **逻辑一致性**
- 所有用户类型使用相同的创建逻辑
- 统一的Redis键模式
- 统一的错误处理

### 3. **扩展性更好**
- 如果需要添加新的用户类型，只需在一个地方修改
- 更容易进行全局的用户数据调整

### 4. **维护更简单**
- 只需要维护一个函数的逻辑
- Bug修复只需要在一个地方进行
- 代码审查更容易

## 统一后的架构

```go
// 统一的用户创建函数
func findOrCreateUserByOpenID(openid string, username string) (*UserData, uint64, error) {
    // 1. 查找现有用户
    // 2. 创建新用户（如果不存在）
    // 3. 根据openid前缀自动判断用户类型
    // 4. 可以为不同类型用户设置不同的初始资源
}
```

## 灵活性保留

虽然统一了函数，但仍然保留了为不同用户类型设置不同初始资源的能力：

```go
// 判断是否为游客（根据openid前缀）
isGuest := len(openid) >= 6 && openid[:6] == "guest_"

// 游客可以给更少的初始资源（如果需要区别对待）
if isGuest {
    // 可以在这里调整游客的初始资源
    // initialUserData.gold = 50  // 游客给更少金币
    // initialUserData.diamond = 5 // 游客给更少钻石
}
```

## 调用简化

现在所有地方都调用同一个函数：

```go
// 之前需要判断用户类型
if isGuest {
    gameUserData, gameUid, err = findOrCreateGuestUserLocal(openID, username)
} else {
    gameUserData, gameUid, err = findOrCreateUserByOpenID(openID, username)
}

// 现在统一调用
gameUserData, gameUid, err = findOrCreateUserByOpenID(openID, username)
```

## 总结

这次优化体现了"统一认证架构"的设计理念：
- ✅ 减少代码重复
- ✅ 提高代码质量
- ✅ 简化维护工作
- ✅ 保持功能完整性

通过统一函数，我们实现了真正的"一套逻辑处理所有用户类型"，这与之前统一token认证的思路是一致的。