# 缓存快速使用指南

## 3 步开始使用

### 步骤 1: 在 ServiceContext 中添加缓存

```go
// services/xxx/rpc/internal/svc/servicecontext.go

import (
    "internalwallet/common/cache"
    "github.com/redis/go-redis/v9"
)

type ServiceContext struct {
    Config     config.Config
    YourCache  *cache.RedisCache  // 添加缓存字段
}

func NewServiceContext(c config.Config) *ServiceContext {
    // 创建 Redis 客户端
    redisClient := redis.NewClient(&redis.Options{
        Addr:     c.CacheRedis[0].Host,
        Password: c.CacheRedis[0].Pass,
        DB:       0,
    })

    // 创建缓存实例（使用业务名称作为前缀）
    yourCache := cache.NewRedisCache(redisClient, "your_service_name")

    return &ServiceContext{
        Config:    c,
        YourCache: yourCache,
    }
}
```

### 步骤 2: 在业务逻辑中使用缓存

```go
// services/xxx/rpc/internal/logic/xxxlogic.go

func (l *YourLogic) YourMethod(in *pb.YourRequest) (*pb.YourResponse, error) {
    ctx := l.ctx

    // 使用缓存的 3 种常见场景：

    // 场景1：缓存数据对象
    var data YourData
    err := l.svcCtx.YourCache.GetJSON(ctx, "key", &data)
    if err != nil {
        // 缓存未命中，从数据库加载
        data = loadFromDB()
        // 写入缓存，1小时过期
        l.svcCtx.YourCache.SetJSON(ctx, "key", data, time.Hour)
    }

    // 场景2：计数器（限流、统计）
    count, _ := l.svcCtx.YourCache.Incr(ctx, "counter_key")
    if count > 100 {
        return nil, errors.New("limit exceeded")
    }

    // 场景3：分布式锁（防止重复操作）
    unlock, err := l.svcCtx.YourCache.Lock(ctx, "lock_key", 10*time.Second)
    if err != nil {
        return nil, errors.New("operation in progress")
    }
    defer unlock()

    // 执行临界区代码...

    return &pb.YourResponse{}, nil
}
```

### 步骤 3: 选择合适的方法

#### 字符串操作
```go
// 设置字符串
cache.Set(ctx, "key", "value", 5*time.Minute)

// 获取字符串
value, err := cache.Get(ctx, "key")

// 删除
cache.Delete(ctx, "key")
```

#### JSON 对象
```go
// 保存对象
user := &User{ID: 1, Name: "Alice"}
cache.SetJSON(ctx, "user:1", user, time.Hour)

// 读取对象
var user User
cache.GetJSON(ctx, "user:1", &user)
```

#### 计数器
```go
// 自增
count, _ := cache.Incr(ctx, "visits")

// 增加指定值
cache.IncrBy(ctx, "points", 100)
```

#### 分布式锁
```go
// 获取锁（立即失败）
unlock, err := cache.Lock(ctx, "order:123", 10*time.Second)
if err != nil {
    return errors.New("locked")
}
defer unlock()

// 获取锁（带重试）
unlock, err := cache.LockWithRetry(ctx, "order:123",
    10*time.Second,  // 锁过期时间
    5,               // 重试5次
    100*time.Millisecond) // 每次间隔100ms
```

## 实际示例

notification 服务已经集成了缓存，可以参考：
- `services/notification/rpc/internal/svc/servicecontext.go:46` - 初始化缓存
- `services/notification/rpc/internal/logic/sendemailcodelogic.go:43-111` - 使用缓存

示例包含：
- ✅ 分布式锁防止并发发送 (第43行)
- ✅ 每日发送次数限制 (第56行)
- ✅ 发送历史记录 (第107行)

## 常用场景速查

| 场景 | 方法 | 示例 |
|------|------|------|
| 缓存用户信息 | `SetJSON/GetJSON` | `cache.SetJSON(ctx, "user:123", user, time.Hour)` |
| 限流计数 | `Incr + Expire` | `count, _ := cache.Incr(ctx, "rate:user123")` |
| 防重复提交 | `Lock` | `unlock, err := cache.Lock(ctx, "submit:123", 5*time.Second)` |
| Session管理 | `Set + Expire` | `cache.Set(ctx, "session:abc", data, 30*time.Minute)` |
| 排行榜 | `ZAdd + ZRevRange` | `cache.ZAdd(ctx, "rank", redis.Z{Score: 100, Member: "user1"})` |

## 性能提示

1. **设置合理的过期时间**
   - 热数据：5-30分钟
   - 普通数据：1小时
   - 冷数据：24小时

2. **使用键前缀区分业务**
   ```go
   userCache := cache.NewRedisCache(client, "user")    // user:xxx
   orderCache := cache.NewRedisCache(client, "order")  // order:xxx
   ```

3. **避免缓存穿透**
   ```go
   data, err := loadFromDB(id)
   if err == ErrNotFound {
       // 缓存空值，防止穿透
       cache.Set(ctx, key, "", 5*time.Minute)
   }
   ```

更多详细用法请参考：`common/cache/REDIS_CACHE_USAGE.md`
