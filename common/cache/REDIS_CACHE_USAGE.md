# Redis Cache 使用指南

本指南说明如何使用基于 go-redis/v9 的缓存适配器 `RedisCache`。

## 1. 初始化

```go
import (
    "internalwallet/common/cache"
    "github.com/redis/go-redis/v9"
)

// 创建 Redis 客户端
redisClient := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

// 创建缓存实例（带业务前缀）
userCache := cache.NewRedisCache(redisClient, "user")       // 键前缀：user:
orderCache := cache.NewRedisCache(redisClient, "order")     // 键前缀：order:
marketCache := cache.NewRedisCache(redisClient, "market")   // 键前缀：market:
```

## 2. 基本字符串操作

```go
ctx := context.Background()

// 设置字符串值（5分钟过期）
err := userCache.Set(ctx, "token:user123", "eyJhbGc...", 5*time.Minute)

// 获取字符串值
token, err := userCache.Get(ctx, "token:user123")

// 删除键
err = userCache.Delete(ctx, "token:user123")

// 检查键是否存在
exists, err := userCache.Exists(ctx, "token:user123")

// 设置过期时间
err = userCache.Expire(ctx, "session:abc", 30*time.Minute)

// 获取剩余过期时间
ttl, err := userCache.TTL(ctx, "session:abc")
```

## 3. JSON 对象缓存

```go
type User struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
}

// 缓存用户对象（1小时过期）
user := &User{ID: 123, Username: "alice", Email: "alice@example.com"}
err := userCache.SetJSON(ctx, "info:123", user, time.Hour)

// 获取用户对象
var cachedUser User
err = userCache.GetJSON(ctx, "info:123", &cachedUser)
if err != nil {
    // 缓存未命中，从数据库加载
    user, err = userRepo.FindByID(ctx, 123)
    if err == nil {
        // 写入缓存
        userCache.SetJSON(ctx, "info:123", user, time.Hour)
    }
}
```

## 4. 计数器操作

```go
// 自增（返回自增后的值）
count, err := userCache.Incr(ctx, "login_count:user123")

// 增加指定值
count, err = userCache.IncrBy(ctx, "points:user123", 100)

// 自减
count, err = userCache.Decr(ctx, "stock:item456")

// 减少指定值
count, err = userCache.DecrBy(ctx, "stock:item456", 5)

// 示例：限流计数器
// 每分钟最多100次请求
key := fmt.Sprintf("rate_limit:%s:%d", userID, time.Now().Unix()/60)
count, err := userCache.Incr(ctx, key)
if count == 1 {
    // 第一次访问，设置过期时间
    userCache.Expire(ctx, key, time.Minute)
}
if count > 100 {
    return errors.New("rate limit exceeded")
}
```

## 5. 分布式锁

```go
// 方式1：基本锁（立即失败）
unlock, err := orderCache.Lock(ctx, "order:create:user123", 10*time.Second)
if err != nil {
    return errors.New("another order is being created")
}
defer unlock()

// 执行临界区代码
err = createOrder(...)

// 方式2：带重试的锁
// 参数：lockKey, 过期时间, 最大重试次数, 重试间隔
unlock, err := orderCache.LockWithRetry(
    ctx,
    "order:create:user123",
    10*time.Second,  // 锁过期时间
    5,               // 最多重试5次
    100*time.Millisecond, // 每次重试间隔100ms
)
if err != nil {
    return errors.New("failed to acquire lock")
}
defer unlock()

// 执行临界区代码
err = createOrder(...)
```

## 6. 使用场景示例

### 场景1：用户信息缓存

```go
// services/account/rpc/internal/logic/getuserinfologic.go

func (l *GetUserInfoLogic) GetUserInfo(in *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
    ctx := l.ctx

    // 1. 尝试从缓存获取
    var user model.User
    cacheKey := fmt.Sprintf("info:%d", in.UserId)
    err := l.svcCtx.UserCache.GetJSON(ctx, cacheKey, &user)

    if err == nil {
        // 缓存命中
        logx.Infof("User %d info loaded from cache", in.UserId)
        return &pb.GetUserInfoResponse{
            UserId:   user.ID,
            Username: user.Username,
            Email:    user.Email,
        }, nil
    }

    // 2. 缓存未命中，从数据库加载
    user, err = l.svcCtx.UserModel.FindOne(ctx, in.UserId)
    if err != nil {
        return nil, err
    }

    // 3. 写入缓存（1小时过期）
    l.svcCtx.UserCache.SetJSON(ctx, cacheKey, &user, time.Hour)

    return &pb.GetUserInfoResponse{
        UserId:   user.ID,
        Username: user.Username,
        Email:    user.Email,
    }, nil
}
```

### 场景2：防止重复下单

```go
// services/trade/rpc/internal/logic/placeorderlogic.go

func (l *PlaceOrderLogic) PlaceOrder(in *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
    ctx := l.ctx

    // 1. 使用分布式锁防止重复下单
    lockKey := fmt.Sprintf("place_order:%d:%s", in.UserId, in.Symbol)
    unlock, err := l.svcCtx.OrderCache.LockWithRetry(
        ctx,
        lockKey,
        5*time.Second,  // 锁5秒
        3,              // 重试3次
        100*time.Millisecond,
    )
    if err != nil {
        return nil, errors.New("order is being processed, please wait")
    }
    defer unlock()

    // 2. 检查24小时内下单次数（防刷单）
    countKey := fmt.Sprintf("order_count:%d:%s", in.UserId, time.Now().Format("2006-01-02"))
    count, _ := l.svcCtx.OrderCache.Incr(ctx, countKey)
    if count == 1 {
        // 第一次下单，设置24小时过期
        l.svcCtx.OrderCache.Expire(ctx, countKey, 24*time.Hour)
    }
    if count > 1000 {
        return nil, errors.New("daily order limit exceeded")
    }

    // 3. 创建订单
    order, err := l.createOrder(in)
    if err != nil {
        return nil, err
    }

    // 4. 缓存订单信息（10分钟）
    orderKey := fmt.Sprintf("info:%s", order.OrderID)
    l.svcCtx.OrderCache.SetJSON(ctx, orderKey, order, 10*time.Minute)

    return &pb.PlaceOrderResponse{
        OrderId: order.OrderID,
        Status:  order.Status,
    }, nil
}
```

### 场景3：行情数据缓存

```go
// services/market/rpc/internal/logic/gettickerlogic.go

func (l *GetTickerLogic) GetTicker(in *pb.GetTickerRequest) (*pb.GetTickerResponse, error) {
    ctx := l.ctx

    // 1. 从缓存获取最新行情（行情数据通常缓存1-5秒）
    var ticker model.Ticker
    cacheKey := fmt.Sprintf("ticker:%s", in.Symbol)
    err := l.svcCtx.MarketCache.GetJSON(ctx, cacheKey, &ticker)

    if err == nil {
        return &pb.GetTickerResponse{
            Symbol:    ticker.Symbol,
            LastPrice: ticker.LastPrice,
            Volume:    ticker.Volume,
            // ...
        }, nil
    }

    // 2. 缓存未命中，从数据源获取
    ticker, err = l.fetchTickerFromSource(in.Symbol)
    if err != nil {
        return nil, err
    }

    // 3. 写入缓存（3秒过期，行情数据更新频繁）
    l.svcCtx.MarketCache.SetJSON(ctx, cacheKey, &ticker, 3*time.Second)

    return &pb.GetTickerResponse{
        Symbol:    ticker.Symbol,
        LastPrice: ticker.LastPrice,
        Volume:    ticker.Volume,
    }, nil
}
```

### 场景4：Session 管理

```go
// api-gateway/internal/handler/sessionhandler.go

type SessionManager struct {
    cache *cache.RedisCache
}

// 创建 Session
func (s *SessionManager) CreateSession(ctx context.Context, userID int64) (string, error) {
    sessionID := uuid.New().String()

    sessionData := map[string]interface{}{
        "user_id":    userID,
        "created_at": time.Now().Unix(),
    }

    // 存储 Session（30分钟过期）
    key := fmt.Sprintf("session:%s", sessionID)
    err := s.cache.SetJSON(ctx, key, sessionData, 30*time.Minute)
    if err != nil {
        return "", err
    }

    return sessionID, nil
}

// 验证 Session
func (s *SessionManager) ValidateSession(ctx context.Context, sessionID string) (int64, error) {
    var sessionData map[string]interface{}
    key := fmt.Sprintf("session:%s", sessionID)

    err := s.cache.GetJSON(ctx, key, &sessionData)
    if err != nil {
        return 0, errors.New("session not found or expired")
    }

    // 刷新 Session 过期时间
    s.cache.Expire(ctx, key, 30*time.Minute)

    userID := int64(sessionData["user_id"].(float64))
    return userID, nil
}

// 销毁 Session
func (s *SessionManager) DestroySession(ctx context.Context, sessionID string) error {
    key := fmt.Sprintf("session:%s", sessionID)
    return s.cache.Delete(ctx, key)
}
```

## 7. ServiceContext 集成

```go
// services/xxx/rpc/internal/svc/servicecontext.go

import (
    "internalwallet/common/cache"
    "github.com/redis/go-redis/v9"
)

type ServiceContext struct {
    Config    config.Config
    UserCache *cache.RedisCache
    // ... 其他字段
}

func NewServiceContext(c config.Config) *ServiceContext {
    // 创建 Redis 客户端
    redisClient := redis.NewClient(&redis.Options{
        Addr:     c.CacheRedis[0].Host,
        Password: c.CacheRedis[0].Pass,
        DB:       0,
    })

    // 创建缓存实例
    userCache := cache.NewRedisCache(redisClient, "user")

    return &ServiceContext{
        Config:    c,
        UserCache: userCache,
    }
}
```

## 8. 高级操作

### Hash 操作（存储对象字段）

```go
// 设置用户字段
userCache.HSet(ctx, "profile:123", "username", "alice")
userCache.HSet(ctx, "profile:123", "email", "alice@example.com")

// 获取单个字段
username, err := userCache.HGet(ctx, "profile:123", "username")

// 获取所有字段
fields, err := userCache.HGetAll(ctx, "profile:123")
// fields = map[string]string{"username": "alice", "email": "alice@example.com"}

// 删除字段
err = userCache.HDel(ctx, "profile:123", "email")
```

### List 操作（队列）

```go
// 推送任务到队列
orderCache.RPush(ctx, "pending_orders", orderID1, orderID2, orderID3)

// 从队列取出任务（FIFO）
orderID, err := orderCache.LPop(ctx, "pending_orders")

// 获取队列长度
length, err := orderCache.LLen(ctx, "pending_orders")

// 获取队列内容（不删除）
orders, err := orderCache.LRange(ctx, "pending_orders", 0, 9) // 前10个
```

### Set 操作（去重集合）

```go
// 添加用户到在线列表
userCache.SAdd(ctx, "online_users", "user123", "user456")

// 检查用户是否在线
isOnline, err := userCache.SIsMember(ctx, "online_users", "user123")

// 获取所有在线用户
users, err := userCache.SMembers(ctx, "online_users")

// 移除离线用户
userCache.SRem(ctx, "online_users", "user123")

// 获取在线用户数
count, err := userCache.SCard(ctx, "online_users")
```

### Sorted Set 操作（排行榜）

```go
// 添加用户分数
userCache.ZAdd(ctx, "leaderboard", redis.Z{Score: 1000, Member: "user123"})
userCache.ZAdd(ctx, "leaderboard", redis.Z{Score: 2000, Member: "user456"})

// 获取排行榜前10名（分数从高到低）
topUsers, err := userCache.ZRevRange(ctx, "leaderboard", 0, 9)

// 获取指定用户的分数
score, err := userCache.ZScore(ctx, "leaderboard", "user123")

// 获取排行榜总人数
count, err := userCache.ZCard(ctx, "leaderboard")
```

## 9. 性能优化建议

1. **合理设置过期时间**
   - 用户信息：1小时
   - Session：30分钟
   - 行情数据：3-5秒
   - 验证码：5分钟

2. **使用 Pipeline 批量操作**
   ```go
   client := userCache.GetClient()
   pipe := client.Pipeline()

   pipe.Set(ctx, "key1", "value1", time.Hour)
   pipe.Set(ctx, "key2", "value2", time.Hour)
   pipe.Set(ctx, "key3", "value3", time.Hour)

   _, err := pipe.Exec(ctx)
   ```

3. **缓存穿透防护**
   ```go
   user, err := getUserFromDB(userID)
   if err == ErrNotFound {
       // 缓存空值，防止缓存穿透
       userCache.Set(ctx, fmt.Sprintf("info:%d", userID), "", 5*time.Minute)
       return nil, err
   }
   ```

4. **使用键前缀区分业务**
   - 每个服务使用独立的缓存实例和前缀
   - 避免不同业务的键冲突

5. **监控缓存命中率**
   ```go
   // 记录缓存命中
   metrics.IncrCounter("cache.hit", tags)

   // 记录缓存未命中
   metrics.IncrCounter("cache.miss", tags)
   ```
