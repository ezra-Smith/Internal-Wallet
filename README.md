# Internal Wallet（企业内部钱包）

基于 Go-Zero 的企业内部钱包系统，提供资金账户、充值/提现、内部转账、链上签名、扫链入账、资金归集、会计账本、行情与后台管理能力。

## 核心能力

- 用户侧：登录鉴权、资产总览、充值/提现、交易记录、地址管理、通知。
- 钱包侧：多链（ETH/BSC/TRON）交易构建、签名、广播、状态追踪。
- 风控与运营：后台 RBAC、资产配置、地址初始化、黑名单/审计相关管理。
- 资金中台：会计双分录账本（`acct_*`）与资产主数据（`asset`）统一治理。
- 归集与运维：充值地址监控、链上确认、归集任务调度、可观测性与部署脚本。

## 架构总览

`api-gateway` 作为统一 HTTP 入口，按 `api-gateway/routes.yaml` 将请求转发到后端 RPC 服务；核心服务通过 ETCD 做服务发现，依赖 MariaDB / Redis / Kafka。

主要服务如下：

- `api-gateway`：HTTP 网关、鉴权、限流、Swagger 入口。
- `services/business/rpc`：用户业务主流程（资产、充值、提现、转账、Swap 等）。
- `services/admin/rpc`：后台管理 RPC（菜单权限、运营管理、资产配置等）。
- `services/accounting/rpc`：会计账本与资产主数据真源。
- `services/chainrpc/rpc`：链交互封装（构建交易、费估算、广播、查询）。
- `services/chainsync/rpc`：扫链与充值入账链路。
- `services/signer/rpc`：签名与地址派生服务。
- `services/consolidation/rpc`：用户充值地址资金归集。
- `services/swap/rpc`：Swap 聚合与交易编排。
- `services/market/rpc`：行情与汇率采集（Binance ticker/fiat -> Redis）。
- `services/notification/rpc`：站内/推送通知相关能力。

## 目录结构

```text
.
├── api-gateway/               # HTTP 网关与路由配置
├── services/*/rpc/            # 各业务微服务（Go-Zero gRPC）
├── proto/                     # protobuf 契约与生成产物
├── common/ pkg/               # 跨服务公共能力
├── deploy/
│   ├── docker/                # 本地基础设施与初始化 SQL
│   ├── admin/                 # 全栈 docker-compose（含业务服务）
│   └── k8s/                   # Kubernetes 清单
├── scripts/                   # 开发、迁移、校验、发布脚本
├── docs/swagger/              # OpenAPI/Swagger 产物
└── admin/                     # Admin 前端代码（当前工作区可见时）
```

## 技术栈

- Go 1.25.4
- Go-Zero
- MariaDB 10.11
- Redis
- Kafka
- ETCD
- OpenTelemetry / Jaeger
- Admin 前端：Umi + Ant Design Pro + Bun

## 本地开发（推荐路径）

### 1) 准备依赖

- Go 1.25.4
- Docker / Docker Compose
- `protoc` + `goctl`
- `bun`（前端）

安装 `goctl`：

```bash
go install github.com/zeromicro/go-zero/tools/goctl@latest
```

### 2) 启动基础设施

```bash
docker compose -f deploy/docker/docker-compose.yml up -d
```

该 compose 会启动 MariaDB / Redis / Kafka / ETCD / Jaeger / Prometheus / Grafana / Swagger UI（基础设施层）。

### 3) 初始化或补跑数据库脚本

```bash
bash scripts/migrate-db.sh
```

### 4) 代码生成（改 proto 或路由后必做）

```bash
make proto
bash scripts/sync-swagger-http-yaml.sh
```

如需要更新管理端 API 类型：

```bash
cd admin
bun run generate:api
bun run lint
```

## 常用命令

### 后端

```bash
go test ./...
golangci-lint run
make proto
```

```bash
make run-market-alert
```

### 前端（admin）

```bash
cd admin
bun install
bun run dev
bun run lint
bun run test
```

如果你的本地是 polyrepo 布局（前端目录为 `web/admin`），把上面的 `cd admin` 替换为 `cd web/admin`。

如果执行 shell 脚本出现 `$'\r': command not found`，先统一换行符：

```bash
find scripts -name '*.sh' -type f -exec perl -pi -e 's/\r$//' {} +
```

## 关键约定（必须遵守）

### 1) 统一时间字段与软删除

- 所有表统一：`created_at` / `updated_at` / `deleted_at`
- 软删除语义：`deleted_at IS NULL` 为未删，`deleted_at IS NOT NULL` 为已删
- 禁止继续引入：`created_time` / `updated_time` / `deleted` 标志位

### 2) Accounting 数据所有权

`services/accounting` 是以下表的唯一 owner，其他服务不得直连读写：

- `asset`
- `acct_account_types`
- `acct_account_type_assets`
- `acct_accounts`
- `acct_balances`
- `acct_ledger_tx`
- `acct_ledger_postings`

其他服务获取资产主数据必须通过 Accounting RPC。

### 3) 地址边界（充值地址 vs 提币地址簿）

- `wallet_user_chain_addresses`：系统分配的用户充值地址（扫链、归集、内部地址识别使用）
- `member_wallet_address`：用户保存的提币地址簿（仅便捷管理）
- 提币下单允许任意 `to_address`，不得强依赖 `member_wallet_address` 存在

### 4) 契约变更同步链路

当修改 `proto/*.proto` 或 `api-gateway/routes.yaml` 时：

1. `make proto`
2. `bash scripts/sync-swagger-http-yaml.sh`
3. `cd admin && bun run generate:api`
4. `cd admin && bun run lint`
5. `bash scripts/check-db-conventions.sh`

## 端口参考（常见）

- 基础设施（`deploy/docker/docker-compose.yml`）：
  - MariaDB `15306`
  - Redis `16380`
  - ETCD `15279`
  - Kafka `9092`
  - Prometheus `9090`
  - Grafana `3000`
  - Jaeger UI `16686`
  - Swagger UI `18080`
- 业务服务监听（默认配置文件）：
  - Gateway `8080`
  - Business RPC `8081`
  - Admin RPC `9013`
  - Swap RPC `9014`
  - Accounting RPC `9015`
  - Consolidation RPC `9016`
  - Signer RPC `9012`
  - ChainRPC `9005`
  - ChainSync `9001`
  - Notification `8087`
  - Market Health `18080`

## 部署

- Docker 相关：`deploy/docker/`, `deploy/admin/`
- Kubernetes 清单：`deploy/k8s/`
- 生产 overlay：`deploy/k8s/overlays/production`

## 安全提示

- 不要提交任何密钥、助记词、私钥或生产环境凭据。
- 本地配置请使用 `.env` / `deploy/admin/.env`（模板见 `deploy/admin/env.example`）。
- 生产环境建议通过 Secrets Manager / K8s Secret 注入配置。
