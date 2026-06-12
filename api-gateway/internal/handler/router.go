package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"internalwallet/api-gateway/internal/config"
	"internalwallet/api-gateway/internal/storage"
	"internalwallet/common/errcode"
	"internalwallet/common/i18n"
	"internalwallet/common/interceptor"
	"internalwallet/common/middleware"
	"internalwallet/proto/pb"

	gconfig "internalwallet/api-gateway/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ServiceClients 存储所有 RPC 客户端
type ServiceClients struct {
	// Business Service - 业务服务
	BusinessClient pb.BusinessClient

	// ChainRPC Service - 链操作服务
	ChainRPCClient pb.ChainRPCClient

	// ChainSync Service - 扫链服务
	ChainSyncClient pb.ChainSyncClient

	// Admin Service - 后台管理服务
	AdminClient pb.AdminClient
}

// RouterConfig 路由器配置
type RouterConfig struct {
	Mux           *http.ServeMux
	GatewayConfig gconfig.Config
	RouteConfig   *config.RouteConfig
	JWTSecret     string
	// Optional: used to generate/upload QR code images for some endpoints.
	S3Uploader *storage.S3Uploader
	// Optional: used to cache QR code URLs and metadata.
	QRCodeCache QRCodeCache
	// Redis 客户端（用于用户冻结检查等）
	RedisClient *redis.Client
	// 路由级别的IP限流器（内存版 - key: 路由路径）
	IPRateLimiters map[string]*middleware.IPRateLimiter
	// 路由级别的Redis限流器（分布式版 - key: 路由路径）
	RedisRateLimiters map[string]*middleware.RedisRateLimiter
	// 静态路由的方法分发器（同一路径支持多 HTTP Method）
	StaticMethodMuxes map[string]*methodMux
	// 动态路由（支持 {id} 等路径参数）
	DynamicRouter *DynamicRouter
}

func routeKey(route config.Route) string {
	return strings.ToUpper(route.Method) + " " + route.Path
}

// methodMux allows multiple HTTP methods to share the same net/http ServeMux path.
// ServeMux does not allow registering the same pattern twice.
type methodMux struct {
	path     string
	handlers map[string]http.Handler
}

func newMethodMux(path string) *methodMux {
	return &methodMux{path: path, handlers: make(map[string]http.Handler)}
}

func (m *methodMux) Register(method string, handler http.Handler) error {
	method = strings.ToUpper(method)
	if _, exists := m.handlers[method]; exists {
		return fmt.Errorf("duplicate route registration: %s %s", method, m.path)
	}
	m.handlers[method] = handler
	return nil
}

func (m *methodMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Preflight is handled by global CORS middleware; still respond safely.
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if h, ok := m.handlers[strings.ToUpper(r.Method)]; ok {
		h.ServeHTTP(w, r)
		return
	}
	WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func collectRequiredRPCServices(routeConfig *config.RouteConfig) map[string]struct{} {
	required := make(map[string]struct{})
	if routeConfig == nil {
		return required
	}
	for _, serviceRoutes := range routeConfig.Services {
		for _, route := range serviceRoutes.Routes {
			name := strings.TrimSpace(route.Service)
			if name == "" {
				continue
			}
			required[name] = struct{}{}
		}
	}
	return required
}

func isRPCServiceRequired(required map[string]struct{}, service string) bool {
	if len(required) == 0 {
		return false
	}
	_, ok := required[service]
	return ok
}

// RegisterRoutes 从配置文件注册所有路由
func RegisterRoutes(ctx context.Context, cfg RouterConfig) error {
	// 1. 初始化所有 RPC 客户端
	//
	// 重要：如果某个 RPC 服务不可用，而 routes.yaml 里又配置了该服务的路由，
	// 以前会“跳过注册”从而表现为 HTTP 404（非常难排查）。
	// 这里改为启动即失败，避免网关在不可用状态下对外提供服务。
	requiredRPC := collectRequiredRPCServices(cfg.RouteConfig)
	clients, err := initServiceClients(cfg.GatewayConfig, requiredRPC)
	if err != nil {
		return fmt.Errorf("failed to init service clients: %w", err)
	}

	// 2. 根据配置选择限流器类型
	useRedis := cfg.GatewayConfig.RateLimit.Enable && cfg.GatewayConfig.RateLimit.UseRedis

	if useRedis {
		// 使用Redis分布式限流（适合集群部署）
		logx.Info("Using Redis-based distributed rate limiting")

		// 创建Redis客户端
		redisClient, err := createRedisClient(cfg.GatewayConfig)
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}

		// 测试Redis连接
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("failed to connect to Redis: %w", err)
		}

		// 保存 Redis 客户端供其他功能使用（如用户冻结检查）
		cfg.RedisClient = redisClient

		if cfg.QRCodeCache == nil {
			cfg.QRCodeCache = newRedisQRCodeCache(redisClient)
		}

		// 初始化Redis限流器映射
		if cfg.RedisRateLimiters == nil {
			cfg.RedisRateLimiters = make(map[string]*middleware.RedisRateLimiter)
		}

		// 为每个配置了限流的路由创建Redis限流器
		for _, serviceRoutes := range cfg.RouteConfig.Services {
			for _, route := range serviceRoutes.Routes {
				limitType := route.GetRateLimitType()
				if limitType != "none" && route.RateLimit > 0 {
					cfg.RedisRateLimiters[routeKey(route)] = middleware.NewRedisRateLimiter(
						redisClient,
						route.RateLimit,
					)
					logx.Infof("  ✓ Created Redis rate limiter for %s: %d req/s (type: %s)", route.Path, route.RateLimit, limitType)
				}
			}
		}
	} else {
		// 使用内存限流（适合单实例部署）
		logx.Info("Using memory-based rate limiting")

		if cfg.QRCodeCache == nil && strings.TrimSpace(cfg.GatewayConfig.Redis.Host) != "" && cfg.GatewayConfig.Redis.Port > 0 {
			redisClient, err := createRedisClient(cfg.GatewayConfig)
			if err != nil {
				logx.Errorf("Failed to create Redis client for qrcode cache: %v", err)
			} else {
				pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				pingErr := redisClient.Ping(pingCtx).Err()
				cancel()
				if pingErr != nil {
					logx.Errorf("Failed to connect to Redis for qrcode cache: %v", pingErr)
					_ = redisClient.Close()
				} else {
					// 保存 Redis 客户端供其他功能使用（如用户冻结检查）
					cfg.RedisClient = redisClient
					cfg.QRCodeCache = newRedisQRCodeCache(redisClient)
				}
			}
		}

		// 初始化IP限流器映射
		if cfg.IPRateLimiters == nil {
			cfg.IPRateLimiters = make(map[string]*middleware.IPRateLimiter)
		}

		// 为每个配置了限流的路由创建IP限流器
		for _, serviceRoutes := range cfg.RouteConfig.Services {
			for _, route := range serviceRoutes.Routes {
				limitType := route.GetRateLimitType()
				if limitType != "none" && route.RateLimit > 0 {
					cfg.IPRateLimiters[routeKey(route)] = middleware.NewIPRateLimiter(
						route.RateLimit,
						route.RateLimit*2, // burst = rate * 2
						10*time.Minute,    // 10分钟清理一次不活跃的IP限流器
					)
					if limitType == "user" {
						logx.Errorf("  ⚠ Created memory rate limiter for %s: %d req/s (requested: user, fallback: IP)", route.Path, route.RateLimit)
					} else {
						logx.Infof("  ✓ Created memory rate limiter for %s: %d req/s (type: IP)", route.Path, route.RateLimit)
					}
				}
			}
		}
	}

	// 2.5 确保 Redis 客户端可用（用于冻结检查等功能）
	// 如果之前没有初始化 Redis（例如禁用了 Redis 限流），这里尝试初始化
	if cfg.RedisClient == nil && strings.TrimSpace(cfg.GatewayConfig.Redis.Host) != "" && cfg.GatewayConfig.Redis.Port > 0 {
		logx.Info("Initializing Redis client for frozen user check and other features...")
		redisClient, err := createRedisClient(cfg.GatewayConfig)
		if err != nil {
			logx.Errorf("⚠ Failed to create Redis client: %v", err)
			logx.Errorf("⚠ User frozen check will NOT work!")
		} else {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			pingErr := redisClient.Ping(pingCtx).Err()
			cancel()
			if pingErr != nil {
				logx.Errorf("⚠ Failed to connect to Redis: %v", pingErr)
				logx.Errorf("⚠ User frozen check will NOT work!")
				_ = redisClient.Close()
			} else {
				cfg.RedisClient = redisClient
				logx.Info("✓ Redis client initialized successfully for frozen user check")
			}
		}
	}

	// 2.6 检查并记录 Redis 客户端最终状态
	if cfg.RedisClient != nil {
		logx.Info("✓✓✓ Redis client available - frozen user check is ENABLED")
	} else {
		logx.Errorf("✗✗✗ Redis client NOT available - frozen user check is DISABLED!")
		logx.Errorf("✗✗✗ Please ensure Redis is configured correctly in gateway.yaml")
	}

	// 3.0 初始化静态路由方法分发器
	if cfg.StaticMethodMuxes == nil {
		cfg.StaticMethodMuxes = make(map[string]*methodMux)
	}

	// 3. 遍历配置文件中的所有服务
	registeredCount := 0
	skippedCount := 0

	// 3.1 初始化动态路由器（用于支持带路径参数的路由）
	if cfg.DynamicRouter == nil {
		cfg.DynamicRouter = NewDynamicRouter()
	}

	for serviceName, serviceRoutes := range cfg.RouteConfig.Services {
		logx.Infof("Registering routes for service: %s", serviceName)

		// 4. 注册该服务的所有路由
		for _, route := range serviceRoutes.Routes {
			if err := registerRoute(ctx, cfg, clients, serviceName, route); err != nil {
				// 只记录警告,不中断注册流程
				logx.Errorf("  ✗ Skipped route %s %s: %v", route.Method, route.Path, err)
				skippedCount++
			} else {
				logx.Infof("  ✓ Registered: %s %s -> %s.%s", route.Method, route.Path, route.Service, route.RPC)
				registeredCount++
			}
		}
	}

	logx.Infof("Route registration completed: %d registered, %d skipped", registeredCount, skippedCount)

	// 4. Mount a catch-all router at "/" so NotFound responses also follow the unified envelope.
	// ServeMux always picks the longest pattern match, so this will not affect registered static routes.
	if cfg.DynamicRouter != nil {
		cfg.Mux.Handle("/", cfg.DynamicRouter)
		if cfg.DynamicRouter.HasRoutes() {
			logx.Infof("✓ Dynamic router enabled (%d routes)", len(cfg.DynamicRouter.routes))
		} else {
			logx.Info("✓ Catch-all router enabled (no dynamic routes)")
		}
	}
	return nil
}

// initServiceClients 初始化所有 RPC 客户端
func initServiceClients(cfg gconfig.Config, requiredRPC map[string]struct{}) (*ServiceClients, error) {
	clients := &ServiceClients{}

	// 添加客户端拦截器（自动注入元数据）
	clientInterceptor := interceptor.ClientMetadataUnaryInterceptor()

	// ==================== Business Service ====================
	// 业务服务：用户管理、资金账户、内部转账、闪兑等
	// 注意：邮件发送已改为异步，使用默认超时配置即可（通常 5000ms）
	businessConn, err := zrpc.NewClient(cfg.BusinessRpc, zrpc.WithUnaryClientInterceptor(clientInterceptor))
	if err != nil {
		logx.Errorf("Failed to connect to BusinessRpc: %v (service may not be started yet)", err)
		if isRPCServiceRequired(requiredRPC, "Business") {
			return nil, fmt.Errorf("Business RPC is required by routes but unavailable: %w", err)
		}
	} else {
		clients.BusinessClient = pb.NewBusinessClient(businessConn.Conn())
		logx.Info("✓ Connected to Business Service (with metadata interceptor)")
	}

	// ==================== ChainRPC Service ====================
	// 链操作服务：地址生成、交易构建、广播、链上查询等
	chainRpcConn, err := zrpc.NewClient(cfg.ChainRpcRpc, zrpc.WithUnaryClientInterceptor(clientInterceptor))
	if err != nil {
		logx.Errorf("Failed to connect to ChainRpcRpc: %v (service may not be started yet)", err)
		if isRPCServiceRequired(requiredRPC, "ChainRPC") {
			return nil, fmt.Errorf("ChainRPC RPC is required by routes but unavailable: %w", err)
		}
	} else {
		clients.ChainRPCClient = pb.NewChainRPCClient(chainRpcConn.Conn())
		logx.Info("✓ Connected to ChainRPC Service (with metadata interceptor)")
	}

	// ==================== ChainSync Service ====================
	// 扫链服务：监听区块链、充值到账、交易确认等
	chainSyncConn, err := zrpc.NewClient(cfg.ChainSyncRpc, zrpc.WithUnaryClientInterceptor(clientInterceptor))
	if err != nil {
		logx.Errorf("Failed to connect to ChainSyncRpc: %v (service may not be started yet)", err)
		if isRPCServiceRequired(requiredRPC, "ChainSync") {
			return nil, fmt.Errorf("ChainSync RPC is required by routes but unavailable: %w", err)
		}
	} else {
		clients.ChainSyncClient = pb.NewChainSyncClient(chainSyncConn.Conn())
		logx.Info("✓ Connected to ChainSync Service (with metadata interceptor)")
	}

	// ==================== Admin Service ====================
	// 后台管理服务：管理员登录、RBAC、系统配置、审计日志等
	// 注意：即使没配置 AdminRpc，也可能 routes.yaml 中存在 Admin 路由；这种情况应启动失败。
	adminConfigured := len(cfg.AdminRpc.Etcd.Hosts) > 0 || len(cfg.AdminRpc.Endpoints) > 0
	if adminConfigured || isRPCServiceRequired(requiredRPC, "Admin") {
		adminConn, err := zrpc.NewClient(cfg.AdminRpc, zrpc.WithUnaryClientInterceptor(clientInterceptor))
		if err != nil {
			logx.Errorf("Failed to connect to AdminRpc: %v (service may not be started yet)", err)
			if isRPCServiceRequired(requiredRPC, "Admin") {
				return nil, fmt.Errorf("Admin RPC is required by routes but unavailable: %w", err)
			}
		} else {
			clients.AdminClient = pb.NewAdminClient(adminConn.Conn())
			logx.Info("✓ Connected to Admin Service (with metadata interceptor)")
		}
	}

	return clients, nil
}

// registerRoute 注册单个路由
func registerRoute(ctx context.Context, cfg RouterConfig, clients *ServiceClients, serviceName string, route config.Route) error {
	// 1. 获取 RPC 客户端
	client, err := getClientByService(clients, route.Service)
	if err != nil {
		return err
	}

	// 2. 获取 RPC 方法
	rpcMethod, err := getRPCMethod(client, route.RPC)
	if err != nil {
		return err
	}

	// 3. 创建 HTTP Handler
	handler := createHTTPHandler(ctx, cfg, rpcMethod, route)

	// 4. 应用中间件
	handler = applyMiddlewares(handler, cfg, route)

	// 5. 注册路由
	// 静态路由：直接注册到 ServeMux（高性能、最长前缀匹配）
	// 动态路由：注册到 DynamicRouter（支持 {id} 等路径参数）
	if len(route.PathParams) > 0 || strings.Contains(route.Path, "{") {
		if cfg.DynamicRouter == nil {
			cfg.DynamicRouter = NewDynamicRouter()
		}
		cfg.DynamicRouter.Register(route, handler)
	} else {
		// ServeMux 不允许同一 pattern 注册多次；同一路径不同方法需要做方法分发。
		mm := cfg.StaticMethodMuxes[route.Path]
		if mm == nil {
			mm = newMethodMux(route.Path)
			cfg.StaticMethodMuxes[route.Path] = mm
			cfg.Mux.Handle(route.Path, mm)
		}
		if err := mm.Register(route.Method, handler); err != nil {
			return err
		}
	}

	return nil
}

// getClientByService 根据服务名获取客户端
func getClientByService(clients *ServiceClients, serviceName string) (interface{}, error) {
	var client interface{}

	switch serviceName {
	// ==================== Business Service ====================
	case "Business":
		client = clients.BusinessClient

	// ==================== ChainRPC Service ====================
	case "ChainRPC":
		client = clients.ChainRPCClient

	// ==================== ChainSync Service ====================
	case "ChainSync":
		client = clients.ChainSyncClient

	// ==================== Admin Service ====================
	case "Admin":
		client = clients.AdminClient

	default:
		return nil, fmt.Errorf("unknown service: %s", serviceName)
	}

	// 检查客户端是否已初始化
	if client == nil {
		return nil, fmt.Errorf("service %s is not available (RPC service may not be started)", serviceName)
	}

	return client, nil
}

// getRPCMethod 通过反射获取 RPC 方法
func getRPCMethod(client interface{}, methodName string) (interface{}, error) {
	clientValue := reflect.ValueOf(client)
	method := clientValue.MethodByName(methodName)

	if !method.IsValid() {
		return nil, fmt.Errorf("method %s not found", methodName)
	}

	return method.Interface(), nil
}

// createHTTPHandler 创建 HTTP 处理器
func createHTTPHandler(ctx context.Context, cfg RouterConfig, rpcMethod interface{}, route config.Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 HTTP Method
		if r.Method != route.Method && r.Method != http.MethodOptions {
			WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 使用反射调用 RPC 方法
		// 这里简化实现，实际可以根据 proto 定义生成更精确的代码
		methodValue := reflect.ValueOf(rpcMethod)
		methodType := methodValue.Type()

		// RPC 方法签名支持两种:
		// 1. func(ctx context.Context, req *Request) (*Response, error)
		// 2. func(ctx context.Context, req *Request, opts ...grpc.CallOption) (*Response, error)
		numIn := methodType.NumIn()
		if numIn < 2 {
			WriteError(w, "Invalid RPC method signature: need at least 2 parameters", http.StatusInternalServerError)
			return
		}

		// 创建请求对象
		reqType := methodType.In(1).Elem()
		reqValue := reflect.New(reqType)

		// 解析请求参数
		if err := parseRequest(r, reqValue.Interface(), route); err != nil {
			WriteError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		// 注入 HTTP 元信息到 context，供后端审计/追踪透传
		// 创建一个独立的 30 秒超时 context 用于 RPC 调用
		// 避免继承 HTTP request context 的默认超时（go-zero 默认 2 秒）
		rpcCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rpcCtx = context.WithValue(rpcCtx, middleware.HTTPMethodKey, r.Method)
		rpcCtx = context.WithValue(rpcCtx, middleware.HTTPPathKey, r.URL.Path)

		// 从 HTTP request context 复制 JWT 相关信息到 RPC context
		// 这些值由 JWTAuth 中间件设置，需要传递给 RPC 服务进行鉴权
		httpCtx := r.Context()
		if userID := middleware.GetUserID(httpCtx); userID != "" {
			rpcCtx = context.WithValue(rpcCtx, "user_id", userID)
		}
		if email := middleware.GetEmail(httpCtx); email != "" {
			rpcCtx = context.WithValue(rpcCtx, "email", email)
		}
		if jwtID := middleware.GetJWTID(httpCtx); jwtID != "" {
			rpcCtx = context.WithValue(rpcCtx, middleware.JWTIDKey, jwtID)
		}
		if jwtExp := middleware.GetJWTExpiresAt(httpCtx); jwtExp > 0 {
			rpcCtx = context.WithValue(rpcCtx, middleware.JWTExpiresAtKey, jwtExp)
		}
		if jwtVer := middleware.GetJWTTokenVersion(httpCtx); jwtVer > 0 {
			rpcCtx = context.WithValue(rpcCtx, middleware.JWTTokenVersionKey, jwtVer)
		}
		// 复制客户端元数据（IP、设备指纹等）
		if clientIP := middleware.GetClientIP(httpCtx); clientIP != "" {
			rpcCtx = context.WithValue(rpcCtx, middleware.ClientIPKey, clientIP)
		}
		if userAgent := middleware.GetUserAgent(httpCtx); userAgent != "" {
			rpcCtx = context.WithValue(rpcCtx, middleware.UserAgentKey, userAgent)
		}
		if reqID := middleware.GetRequestID(httpCtx); reqID != "" {
			rpcCtx = context.WithValue(rpcCtx, middleware.RequestIDKey, reqID)
		}
		if locale := middleware.GetLocale(httpCtx); locale != "" {
			rpcCtx = context.WithValue(rpcCtx, middleware.LocaleKey, locale)
		}
		if idempotencyKey := middleware.GetIdempotencyKey(httpCtx); idempotencyKey != "" {
			rpcCtx = context.WithValue(rpcCtx, middleware.IdempotencyKeyKey, idempotencyKey)
		}

		// 调用 RPC 方法
		var results []reflect.Value
		args := []reflect.Value{
			reflect.ValueOf(rpcCtx),
			reqValue,
		}

		// 如果是可变参数方法（如 gRPC 方法），使用 CallSlice
		if methodType.IsVariadic() {
			// 获取可变参数的类型（例如 []grpc.CallOption）
			variadicType := methodType.In(methodType.NumIn() - 1)
			// 创建一个空切片作为可变参数
			emptySlice := reflect.Zero(variadicType)
			args = append(args, emptySlice)
			results = methodValue.CallSlice(args)
		} else {
			results = methodValue.Call(args)
		}

		// 处理返回值
		if len(results) != 2 {
			WriteError(w, "Invalid RPC method return", http.StatusInternalServerError)
			return
		}

		// 检查错误
		if !results[1].IsNil() {
			err := results[1].Interface().(error)
			WriteRPCError(w, r, err)
			return
		}

		// 返回响应
		resp := results[0].Interface()

		if depositResp, ok := resp.(*pb.CreateDepositResp); ok {
			var uploader QRCodeUploader
			if cfg.S3Uploader != nil {
				uploader = cfg.S3Uploader
			}
			enhanceCreateDepositQRCode(rpcCtx, uploader, cfg.QRCodeCache, depositResp)
		}

		WriteJSON(w, resp)
	})
}

// parseRequest 解析请求参数
func parseRequest(r *http.Request, req interface{}, route config.Route) error {
	// 根据 HTTP Method 解析参数
	if route.Method == http.MethodGet || route.Method == http.MethodDelete {
		// GET/DELETE: apply path params first, then merge query params.
		pathVars := ExtractPathParams(r.Context())
		if len(pathVars) > 0 {
			if err := SetStructFields(req, pathVars); err != nil {
				return err
			}
		}
		return ParseQuery(r, req)
	} else {
		// POST/PUT/PATCH: parse body first (protojson may reset message), then override with path params.
		if err := parseBody(r, req); err != nil {
			return err
		}
		pathVars := ExtractPathParams(r.Context())
		if len(pathVars) > 0 {
			if err := SetStructFields(req, pathVars); err != nil {
				return err
			}
		}
		return nil
	}
}

// parseBody 解析请求体
func parseBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	// Prefer protojson for protobuf request messages:
	// - supports google.protobuf.* wrappers as scalar JSON values (e.g. {"pid":1})
	// - accepts both snake_case (proto name) and lowerCamelCase (json name)
	if msg, ok := v.(proto.Message); ok {
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, msg)
	}

	decoder := jsonDecoder(bytes.NewReader(body))
	if err := decoder.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

// applyMiddlewares 应用中间件
func applyMiddlewares(handler http.Handler, cfg RouterConfig, route config.Route) http.Handler {
	// 应用顺序很重要！

	// 1. JWT 认证（如果需要）
	if route.Auth {
		secret := cfg.JWTSecret
		// Admin 服务使用独立密钥对（若配置了）
		if route.Service == "Admin" && cfg.GatewayConfig.AdminAuth.AccessSecret != "" {
			secret = cfg.GatewayConfig.AdminAuth.AccessSecret
		}

		// 1.1 用户冻结检查（在 JWT 认证之后立即检查）
		// 注意：中间件的应用顺序和执行顺序是相反的！
		// 后应用的中间件会先执行，所以要先应用冻结检查，后应用 JWT 认证
		// 这样执行顺序才是：JWT 认证 → 冻结检查
		// 仅对 Business 服务的接口进行冻结检查（Admin 服务的管理员不受此限制）
		redisAvailable := cfg.RedisClient != nil
		shouldApplyCheck := route.Service == "Business" && redisAvailable

		// 【调试日志】检查中间件应用条件
		logx.Infof("→ [FrozenCheck] Middleware application check: path=%s, service=%s, auth=%v, redis_available=%v, will_apply=%v",
			route.Path, route.Service, route.Auth, redisAvailable, shouldApplyCheck)

		if shouldApplyCheck {
			logx.Infof("→ [FrozenCheck] ✓ Applying frozen check middleware to: %s %s (will execute AFTER JWT auth)", route.Method, route.Path)
			handler = applyUserFrozenCheck(handler, cfg.RedisClient)
		} else {
			if route.Service != "Business" {
				logx.Infof("→ [FrozenCheck] ⊗ Skipping frozen check (not Business service): %s %s (service=%s)",
					route.Method, route.Path, route.Service)
			} else if !redisAvailable {
				logx.Errorf("→ [FrozenCheck] ⚠ Skipping frozen check (Redis not available): %s %s",
					route.Method, route.Path)
			}
		}

		// 最后应用 JWT 认证（这样它会先执行）
		handler = middleware.JWTAuth(secret)(handler)
	}

	// 2. 限流（如果配置了）
	limitType := route.GetRateLimitType()
	if limitType != "none" && route.RateLimit > 0 {
		key := routeKey(route)
		// 优先使用Redis限流器（分布式）
		if redisLimiter, exists := cfg.RedisRateLimiters[key]; exists {
			// 根据限流类型调用不同的中间件
			handler = redisLimiter.MiddlewareWithType(key, limitType)(handler)
			logx.Infof("Applied Redis rate limiter for %s: type=%s, rate=%d", route.Path, limitType, route.RateLimit)
		} else if ipLimiter, exists := cfg.IPRateLimiters[key]; exists {
			// 降级到内存限流器（只支持IP限流）
			if limitType == "user" {
				logx.Errorf("Memory rate limiter does not support user-based limiting, falling back to IP for route %s", route.Path)
			}
			handler = ipLimiter.Middleware()(handler)
		} else {
			// 如果都没有找到，使用全局限流（最后的兜底）
			logx.Errorf("No rate limiter found for route %s, using global rate limit", route.Path)
			handler = middleware.RateLimit(route.RateLimit, route.RateLimit*2)(handler)
		}
	}

	// 3. 路径参数解析（如果有路径参数）
	if len(route.PathParams) > 0 || strings.Contains(route.Path, "{") {
		handler = PathParamMiddleware(route.Path)(handler)
	}

	return handler
}

// jsonDecoder 返回 JSON 解码器
func jsonDecoder(r interface{ Read([]byte) (int, error) }) interface{ Decode(interface{}) error } {
	// 使用标准库的 json.Decoder
	return json.NewDecoder(r.(io.Reader))
}

// createRedisClient 创建Redis客户端
func createRedisClient(cfg gconfig.Config) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     100,             // 连接池大小
		MinIdleConns: 10,              // 最小空闲连接数
		MaxRetries:   3,               // 最大重试次数
		DialTimeout:  5 * time.Second, // 连接超时
		ReadTimeout:  3 * time.Second, // 读取超时
		WriteTimeout: 3 * time.Second, // 写入超时
		PoolTimeout:  4 * time.Second, // 连接池超时
	})

	logx.Infof("Created Redis client: %s (DB: %d)", addr, cfg.Redis.DB)
	return client, nil
}

// applyUserFrozenCheck 应用用户冻结检查中间件
func applyUserFrozenCheck(handler http.Handler, redisClient *redis.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 【调试日志】进入冻结检查中间件
		logx.WithContext(ctx).Infof("→ [FrozenCheck] Entering frozen check middleware: path=%s", r.URL.Path)

		// 获取用户 ID（由 JWTAuth 中间件设置）
		userID := middleware.GetUserID(ctx)
		if userID == "" {
			// 没有 user_id，直接放行
			logx.WithContext(ctx).Infof("→ [FrozenCheck] No user_id found, skipping check: path=%s", r.URL.Path)
			handler.ServeHTTP(w, r)
			return
		}

		// 【调试日志】找到用户 ID
		logx.WithContext(ctx).Infof("→ [FrozenCheck] Checking user: user_id=%s, path=%s, redis_client_nil=%v",
			userID, r.URL.Path, redisClient == nil)

		// 检查用户是否被冻结
		frozen, err := isUserFrozenInRedis(ctx, redisClient, userID)
		if err != nil {
			// 检查失败，记录日志但不影响请求（降级处理）
			logx.WithContext(ctx).Errorf("→ [FrozenCheck] Failed to check frozen status: user_id=%s, err=%v", userID, err)
			handler.ServeHTTP(w, r)
			return
		}

		// 【调试日志】检查结果
		logx.WithContext(ctx).Infof("→ [FrozenCheck] Check result: user_id=%s, frozen=%v, path=%s", userID, frozen, r.URL.Path)

		if frozen {
			// 用户已被冻结，返回特定错误码供前端处理（如跳转到登录页或显示冻结提示）
			logx.WithContext(ctx).Infof("→ [FrozenCheck] ❌ User is FROZEN, rejecting request: user_id=%s, path=%s", userID, r.URL.Path)

			// 使用国际化消息（支持中英文）
			message := i18n.T(ctx, "ACCOUNT_FROZEN", nil)
			if message == "" || message == "ACCOUNT_FROZEN" {
				// 如果国际化消息不可用，使用默认中文消息
				message = "您的账户已被冻结，请联系客服"
			}
			WriteErrorWithDetails(
				w,
				int(errcode.CodeUserFrozen), // 特定错误码: 12014
				message,
				http.StatusMethodNotAllowed, // 与前端约定405
				nil,
			)
			return
		}

		// 用户未被冻结，继续处理请求
		logx.WithContext(ctx).Infof("→ [FrozenCheck] ✓ User is NOT frozen, allowing request: user_id=%s, path=%s", userID, r.URL.Path)
		handler.ServeHTTP(w, r)
	})
}

// isUserFrozenInRedis 检查用户是否在 Redis 中被标记为冻结
func isUserFrozenInRedis(ctx context.Context, redisClient *redis.Client, userID string) (bool, error) {
	if redisClient == nil {
		logx.WithContext(ctx).Errorf("→ [FrozenCheck] RedisClient is nil, cannot check frozen status")
		return false, nil
	}

	// 检查 Redis 中的冻结标记
	key := fmt.Sprintf("user:frozen:%s", userID)

	// 【调试日志】查询 Redis
	logx.WithContext(ctx).Infof("→ [FrozenCheck] Checking Redis key: %s", key)

	exists, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		logx.WithContext(ctx).Errorf("→ [FrozenCheck] Redis query failed: key=%s, err=%v", key, err)
		return false, fmt.Errorf("failed to check frozen flag in redis: %w", err)
	}

	// 【调试日志】Redis 查询结果
	logx.WithContext(ctx).Infof("→ [FrozenCheck] Redis EXISTS result: key=%s, exists=%d, is_frozen=%v",
		key, exists, exists > 0)

	return exists > 0, nil
}
