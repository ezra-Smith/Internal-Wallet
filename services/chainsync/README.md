# ChainSync Service - 扫链���务

一个高性能的区块链数据同步服务，支持多链多服务商的自动故障转移和负载均衡。

## 功能特性

### 🔗 多链支持
- **Ethereum (ETH)**: 以太坊主网
- **Binance Smart Chain (BSC)**: 币安智能链
- **TRON**: 波场网络

### 🏢 多服务商支持
- **QuickNode**: 高性能区块链基础设施服务商
- **Infura**: 以太坊和IPFS基础设施
- **Alchemy**: 区块链开发平台
- **Moralis**: Web3开发平台
- **Ankr**: 去中心化基础设施

### 🔄 智能故障转移
- 自动健康检查和监控
- 服务商失败时自动切换
- 加权负载均衡算法
- 冷却时间防止频繁切换

### 📊 实时监控
- 区块链状态实时同步
- 地址余额变更监控
- 交易状态跟踪
- Webhook通知支持

## 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client API    │    │   Web Hook      │    │   Admin Panel   │
└─────────┬───────┘    └─��───────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌─────────────┴─────────────┐
                    │    ChainSync Server       │
                    └─────────────┬─────────────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          │                      │                      │
    ┌─────┴─────┐        ┌─────┴─────┐        ┌─────���─────┐
    │  Sync    │        │ Monitor   │        │ Provider  │
    │ Manager  │        │ Service   │        │   Pool    │
    └─────┬─────┘        └─────┬─────┘        └─────┬─────┘
          │                    │                    │
          └────────────────────┼────────────────────┘
                               │
                    ┌───────────┴───────────┐
                    │   Provider Layer     │
                    └───────────┬───────────┘
                               │
          ┌────────────────────┼──────────────────────┐
          │          │          │          │          │
    ┌─────┴───┐ ┌─────┴───┐ ┌─────┴───┐ ┌─────┴───┐ ┌─────┴───┐
    │QuickNode│ │ Infura  │ │ Alchemy │ │Moralis  │ │  Ankr   │
    └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘
```

## 快速开始

### 1. 配置服务

复制并编辑配置文件：

```bash
cp services/chainsync/rpc/etc/chainsync.yaml.example services/chainsync/rpc/etc/chainsync.yaml
```

主要配置项：

```yaml
# 服务商配置
Providers:
  QuickNode:
    Enabled: true
    Endpoint: "https://YOUR_ENDPOINT.quiknode.pro/YOUR_API_KEY/"
    APIKey: "YOUR_API_KEY"
    Weight: 1.0

# 链配置
Chains:
  Ethereum:
    Enabled: true
    StartBlock: 18800000
    Confirmations: 12
```

### 2. 启动服务

```bash
# 构建服务
cd services/chainsync/rpc
go build -o chainsync .

# 启动服务
./chainsync -f etc/chainsync.yaml
```

### 3. 使用gRPC客户端调用

```go
import (
    "google.golang.org/grpc"
    pb "internal-wallet/proto/pb"
)

// 连接服务
conn, err := grpc.Dial("localhost:9001", grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := pb.NewChainSyncClient(conn)

// 启动同步
resp, err := client.StartSync(context.Background(), &pb.StartSyncReq{
    Chain:         pb.ChainType_CHAIN_TYPE_ETHEREUM,
    StartBlock:    18800000,
    EnableRealTime: true,
    MonitorType:   pb.MonitorType_MONITOR_TYPE_ALL,
})
```

## API 接口

### 区块链同步

#### 启动同步
```go
resp, err := client.StartSync(ctx, &pb.StartSyncReq{
    Chain:         pb.ChainType_CHAIN_TYPE_ETHEREUM,
    StartBlock:    18800000,
    EndBlock:      0,  // 0表示持续同步
    EnableRealTime: true,
    Addresses:     []string{"0x742d35Cc6634C0532925a3b8D4C9db96c4b4Db45"},
    MonitorType:   pb.MonitorType_MONITOR_TYPE_ALL,
    Priority:      pb.Priority_PRIORITY_HIGH,
})
```

#### 停止同步
```go
resp, err := client.StopSync(ctx, &pb.StopSyncReq{
    SyncId: "ethereum_1699123456",
})
```

#### 获取同步状态
```go
resp, err := client.GetSyncStatus(ctx, &pb.GetSyncStatusReq{
    Chain: pb.ChainType_CHAIN_TYPE_ETHEREUM,
})
```

### 地址监控

#### 添加地址监控
```go
resp, err := client.AddAddressMonitor(ctx, &pb.AddAddressMonitorReq{
    Chain:        pb.ChainType_CHAIN_TYPE_ETHEREUM,
    Address:      "0x742d35Cc6634C0532925a3b8D4C9db96c4b4Db45",
    MonitorType:  pb.MonitorType_MONITOR_TYPE_ALL,
    Priority:     pb.Priority_PRIORITY_HIGH,
    Tag:          "user_wallet_123",
    WebhookUrl:   "https://your-webhook-url.com/notify",
    Metadata: map[string]string{
        "user_id": "123",
        "type":    "hot_wallet",
    },
})
```

#### 获取监控地址列表
```go
resp, err := client.GetMonitoredAddresses(ctx, &pb.GetMonitoredAddressesReq{
    Chain:       pb.ChainType_CHAIN_TYPE_ETHEREUM,
    MonitorType: pb.MonitorType_MONITOR_TYPE_BALANCE,
    Page:        0,
    PageSize:    10,
})
```

### 数据查询

#### 获取交易详情
```go
resp, err := client.GetTransaction(ctx, &pb.GetTransactionReq{
    Chain:        pb.ChainType_CHAIN_TYPE_ETHEREUM,
    TxHash:       "0x...",
    IncludeTrace: true,
})
```

#### 获取地址余额
```go
resp, err := client.GetAddressBalance(ctx, &pb.GetAddressBalanceReq{
    Chain:   pb.ChainType_CHAIN_TYPE_ETHEREUM,
    Address: "0x742d35Cc6634C0532925a3b8D4C9db96c4b4Db45",
    Tokens:  []string{
        "0xA0b86a33E6417c4c4c4c4c4c4c4c4c4c4c4c4c4c", // USDC
        "0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
    },
})
```

### 服务商管理

#### 获取服务商状态
```go
resp, err := client.GetProviderStatus(ctx, &pb.GetProviderStatusReq{
    Chain: pb.ChainType_CHAIN_TYPE_ETHEREUM,
})
```

#### 测试服务商连接
```go
resp, err := client.TestProvider(ctx, &pb.TestProviderReq{
    ProviderId: "quicknode",
    Chain:      pb.ChainType_CHAIN_TYPE_ETHEREUM,
    Endpoint:   "https://your-endpoint.quiknode.pro/...",
    Credentials: map[string]string{
        "api_key": "your_api_key",
    },
})
```

## 配置说明

### 服务商配置

```yaml
Providers:
  QuickNode:
    Enabled: true           # 是否启用
    Name: "QuickNode"      # 服务商名称
    Endpoint: "https://..." # API端点
    APIKey: "YOUR_KEY"     # API密钥
    Weight: 1.0            # 负载均衡权重
    Timeout: 30            # 请求超时时间（秒）
    RateLimit: 100         # 速率限制（请求/秒）
    MaxRetries: 3          # 最大重试次数
    RetryDelay: 1          # 重试延迟（秒）
    Headers:               # 自定义请求头
      User-Agent: "InternalWallet/1.0"
    HealthCheck:           # 健康检查配置
      Enabled: true
      Interval: 30         # 检查间隔（秒）
      Timeout: 10          # 检查超时（秒）
      FailureCount: 3     # 失败次数阈值
```

### 链配置

```yaml
Chains:
  Ethereum:
    Enabled: true                    # 是否启用
    Name: "Ethereum Mainnet"         # 链名称
    ChainType: "ethereum"            # 链类型
    StartBlock: 18800000            # 起始区块号
    Confirmations: 12                # 确认数
    BlockTime: 12                    # 区块时间（秒）
    DefaultProviders: ["quicknode"]  # 默认服务商列表
    ContractAddresses:               # 常用合约地址
      WETH: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
      USDC: "0xA0b86a33E6417c4c4c4c4c4c4c4c4c4c4c4c4c4c"
    GasConfig:                       # Gas配置
      GasPriceMultiplier: 1.0        # Gas价格乘数
      GasLimitMultiplier: 1.1        # Gas限制乘数
      MaxGasPrice: "200000000000"    # 最大Gas价格
```

### 同步配置

```yaml
Sync:
  MaxConcurrentSyncs: 10            # 最大并发同步数
  DefaultRetryCount: 3              # 默认重试次数
  HealthCheckInterval: 30           # 健康检查间隔（秒）
  FailoverThreshold: 3              # 故障转移阈值
  ProviderSwitchCooldown: 60        # 服务商切换冷却时间（秒）
  BatchSize: 100                    # 批量处理大小
  MaxRetryDelay: 60                 # 最大重试延迟（秒）
```

## 监控和指标

### Prometheus指标

服务内置了Prometheus指标收集，可通过`http://localhost:9090/metrics`访问：

- `chainsync_sync_status`: 同步状态
- `chainsync_blocks_synced_total`: 已同步区块总数
- `chainsync_transactions_processed_total`: 已处理交易总数
- `chainsync_provider_requests_total`: 服务商请求总数
- `chainsync_provider_failures_total`: 服务商失败次数
- `chainsync_response_time_seconds`: 请求响应时间

### 日志格式

使用结构化日志，包含以下字段：
- `level`: 日志级别
- `timestamp`: 时间戳
- `service`: 服务名称
- `chain`: 区块链类型
- `provider`: 服务商ID
- `block_number`: 区块号
- `tx_hash`: 交易哈希
- `address`: 地址
- `error`: 错误信息

## 部署建议

### 生产环境

1. **资源配置**
   - CPU: 4核心以上
   - 内存: 8GB以上
   - 存储: SSD，500GB以上
   - 网络: 高带宽，低延迟

2. **数据库优化**
   - MySQL 8.0以上
   - InnoDB引擎
   - 合理的索引策略
   - 连接池配置

3. **Redis缓存**
   - Redis 6.0以上
   - 持久化配置
   - 内存大小根据数据量确定

4. **负载均衡**
   - 多实例部署
   - Nginx或HAProxy负载均衡
   - 健康检查配置

### 容器化部署

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o chainsync services/chainsync/rpc/chainsync.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/chainsync .
COPY --from=builder /app/services/chainsync/rpc/etc ./etc

EXPOSE 9001
CMD ["./chainsync", "-f", "etc/chainsync.yaml"]
```

### Docker Compose

```yaml
version: '3.8'
services:
  chainsync:
    build: .
    ports:
      - "9001:9001"
    environment:
      - CONFIG_PATH=/app/etc/chainsync.yaml
    volumes:
      - ./logs:/app/logs
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: chainsync
    volumes:
      - mysql_data:/var/lib/mysql

  redis:
    image: redis:6.2-alpine
    volumes:
      - redis_data:/data

volumes:
  mysql_data:
  redis_data:
```

## 故障排除

### 常见问题

1. **服务商连接失败**
   - 检查API密钥是否正确
   - 验证端点URL是否可访问
   - 检查网络连接和防火墙设置

2. **同步进度缓慢**
   - 检查服务商速率限制
   - 调整并发同步数
   - 优化数据库查询性能

3. **内存使用过高**
   - 减少批量处理大小
   - 调整缓存配置
   - 检查内存泄漏

4. **数据库连接问题**
   - 检查数据库服务状态
   - 验证连接字符串
   - 调整连接池大小

### 日志分析

```bash
# 查看错误日志
grep "level=error" logs/chainsync.log

# 查看同步状态
grep "Sync status" logs/chainsync.log

# 查看服务商切换
grep "provider" logs/chainsync.log | grep "switch"
```

## 开发指南

### 添加新的服务商

1. 实现`Provider`接口：
```go
type NewProvider struct {
    *BaseProvider
}

func (p *NewProvider) GetLatestBlock(ctx context.Context) (uint64, error) {
    // 实现具体逻辑
}
```

2. 注册服务商：
```go
provider := providers.NewNewProvider(providers.ProviderConfig{
    ID:       "new_provider",
    Type:     pb.ProviderType_PROVIDER_TYPE_CUSTOM,
    Endpoint: "https://api.newprovider.com",
    // ...
})
pool.AddProvider(provider)
```

### 添加新的区块链

1. 定义链类型：
```protobuf
enum ChainType {
    CHAIN_TYPE_NEW_CHAIN = 4;
}
```

2. 实现链特定的API调用逻辑
3. 更新配置文件
4. 添加对应的测试用例

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](../../LICENSE) 文件。