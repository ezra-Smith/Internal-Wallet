package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"internalwallet/api-gateway/internal/config"
	"internalwallet/api-gateway/internal/docs"
	"internalwallet/api-gateway/internal/handler"
	"internalwallet/api-gateway/internal/storage"
	commoninterceptor "internalwallet/common/interceptor"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/pkg/secrets"
	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
)

var configFile = flag.String("f", "etc/gateway.yaml", "the config file")
var routesFile = flag.String("r", "routes.yaml", "the routes config file")

func main() {
	flag.Parse()

	// ========================================
	// Step 1: 加载密钥配置（优先级最高）
	// ========================================
	ctx := context.Background()

	// 从配置文件 Mode 自动读取（推荐，无需配置环境变量）
	secretConfig, err := secrets.LoadFromConfigMode(ctx, *configFile)
	if err != nil {
		logx.Errorf("Failed to load secrets: %v", err)
		panic(err)
	}
	logx.Infof("Secrets loaded successfully (environment: %s)", secretConfig.Environment)

	// Step 2: 加载网关配置（YAML）
	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// Step 3: 注入密钥到配置（覆盖YAML中的占位符）
	// Redis 配置
	c.Redis.Host = secretConfig.Cache.Redis.Host
	c.Redis.Port = secretConfig.Cache.Redis.Port
	c.Redis.Password = secretConfig.Cache.Redis.Password

	// JWT 配置（普通用户）
	c.Auth.AccessSecret = secretConfig.Security.JWT.AccessSecret
	c.Auth.RefreshSecret = secretConfig.Security.JWT.RefreshSecret

	// Admin JWT 配置
	c.AdminAuth.AccessSecret = secretConfig.Security.AdminJWT.AccessSecret
	c.AdminAuth.RefreshSecret = secretConfig.Security.AdminJWT.RefreshSecret

	// S3 配置（仅配置参数，不含凭据 - 使用 IRSA）
	if secretConfig.S3.Bucket != "" {
		c.S3.Enabled = true
		c.S3.Region = secretConfig.S3.Region
		c.S3.Bucket = secretConfig.S3.Bucket
		c.S3.Endpoint = secretConfig.S3.Endpoint
		c.S3.PublicBaseURL = secretConfig.S3.PublicBaseURL
		c.S3.Prefix = secretConfig.S3.Prefix
		c.S3.UsePathStyle = secretConfig.S3.UsePathStyle
		c.S3.ACL = secretConfig.S3.ACL
		// Note: AccessKeyID/SecretAccessKey are NOT set - using IRSA
		logx.Infof("✓ S3 config loaded from secrets (bucket: %s, region: %s)", c.S3.Bucket, c.S3.Region)
	}

	// Geetest 配置（可选）
	if secretConfig.Geetest.CaptchaID != "" {
		c.Geetest.CaptchaID = secretConfig.Geetest.CaptchaID
	}
	if secretConfig.Geetest.CaptchaKey != "" {
		c.Geetest.CaptchaKey = secretConfig.Geetest.CaptchaKey
	}
	if secretConfig.Geetest.APIServer != "" {
		c.Geetest.APIServer = secretConfig.Geetest.APIServer
	}
	if secretConfig.Geetest.Timeout > 0 {
		c.Geetest.Timeout = secretConfig.Geetest.Timeout
	}
	if secretConfig.Geetest.CaptchaID != "" || secretConfig.Geetest.CaptchaKey != "" || secretConfig.Geetest.APIServer != "" || secretConfig.Geetest.Timeout > 0 {
		c.Geetest.Enabled = secretConfig.Geetest.Enabled
		c.Geetest.FailOpen = secretConfig.Geetest.FailOpen
	}

	// ========================================
	// Step 4: 初始化日志配置（原有逻辑）
	// ========================================
	if err := logx.SetUp(c.Log); err != nil {
		fmt.Printf("Failed to setup log: %v\n", err)
		panic(err)
	}

	logx.Info("Starting API Gateway...")

	// 3. 验证 JWT 密钥（启动时强制检查）
	utils.MustValidateJWTSecretPair(c.Auth.AccessSecret, c.Auth.RefreshSecret)
	// Admin JWT（可选：当启用 Admin 服务路由时建议配置）
	if c.AdminAuth.AccessSecret != "" || c.AdminAuth.RefreshSecret != "" {
		utils.MustValidateJWTSecretPair(c.AdminAuth.AccessSecret, c.AdminAuth.RefreshSecret)
	}

	// 3.1 配置可信代理（用于安全的客户端IP提取）
	// 仅当请求来自这些CIDR范围内的代理时，才信任X-Forwarded-For/X-Real-IP标头
	// 这可以防止攻击者通过伪造代理头绕过IP限流
	if len(c.TrustedProxies) > 0 {
		if err := middleware.SetTrustedProxies(c.TrustedProxies); err != nil {
			logx.Errorf("Failed to set trusted proxies: %v", err)
			panic(err)
		}
		logx.Infof("✓ Configured %d trusted proxy CIDR(s)", len(c.TrustedProxies))
		for _, cidr := range c.TrustedProxies {
			logx.Infof("  - %s", cidr)
		}
	} else {
		logx.Info("⚠ No trusted proxies configured - X-Forwarded-For/X-Real-IP headers will be ignored (secure default)")
	}

	// 4. 加载路由配置
	routeConfig, err := config.LoadRoutesFromFile(*routesFile)
	if err != nil {
		logx.Errorf("Failed to load routes config: %v", err)
		panic(err)
	}

	// 验证路由配置
	if err := routeConfig.Validate(); err != nil {
		logx.Errorf("Invalid routes config: %v", err)
		panic(err)
	}

	logx.Infof("Loaded %d services from routes config", len(routeConfig.Services))

	// 4. 创建 HTTP 路由器
	mux := http.NewServeMux()

	// 4.1 Initialize S3 uploader (shared by router and direct handlers).
	var s3Uploader *storage.S3Uploader
	if c.S3.Enabled {
		uploader, err := storage.NewS3Uploader(ctx, c.S3)
		if err != nil {
			logx.Errorf("Failed to init S3 uploader: %v", err)
		} else {
			s3Uploader = uploader
		}
	}

	// 5. 注册所有路由（从配置文件）
	routerConfig := handler.RouterConfig{
		Mux:           mux,
		GatewayConfig: c,
		RouteConfig:   routeConfig,
		JWTSecret:     c.Auth.AccessSecret,
		S3Uploader:    s3Uploader,
	}

	if err := handler.RegisterRoutes(ctx, routerConfig); err != nil {
		logx.Errorf("Failed to register routes: %v", err)
		panic(err)
	}

	adminJWTSecret := c.Auth.AccessSecret
	if c.AdminAuth.AccessSecret != "" {
		adminJWTSecret = c.AdminAuth.AccessSecret
	}

	// 5.1 Admin direct handlers: require Admin RPC configured.
	var adminClient pb.AdminClient
	if len(c.AdminRpc.Etcd.Hosts) > 0 || len(c.AdminRpc.Endpoints) > 0 {
		adminConn, err := zrpc.NewClient(c.AdminRpc, zrpc.WithUnaryClientInterceptor(commoninterceptor.ClientMetadataUnaryInterceptor()))
		if err != nil {
			logx.Errorf("Failed to connect to AdminRpc: %v", err)
		} else {
			adminClient = pb.NewAdminClient(adminConn.Conn())
		}
	}

	// Transfer import: multipart upload needs a direct gateway handler (not via generic JSON->RPC router).
	mux.Handle("/api/v1/admin/transfer/import", middleware.JWTAuth(adminJWTSecret)(handler.NewAdminTransferImportHandler(adminClient)))
	if adminClient != nil {
		logx.Info("✓ Registered direct handler: POST /api/v1/admin/transfer/import")
	} else {
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/transfer/import (Admin RPC not available)")
	}

	// Transfer import preview: parse/validate without creating batch (multipart upload).
	mux.Handle("/api/v1/admin/transfer/import/preview", middleware.JWTAuth(adminJWTSecret)(handler.NewAdminTransferImportPreviewHandler(adminClient)))
	if adminClient != nil {
		logx.Info("✓ Registered direct handler: POST /api/v1/admin/transfer/import/preview")
	} else {
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/transfer/import/preview (Admin RPC not available)")
	}

	// 5.2 Currency icon upload: multipart upload to S3 (returns public URL).
	mux.Handle("/api/v1/admin/currencies/icon/upload", middleware.JWTAuth(adminJWTSecret)(handler.NewAdminCurrencyIconUploadHandler(s3Uploader, adminClient)))
	switch {
	case s3Uploader != nil && adminClient != nil:
		logx.Info("✓ Registered direct handler: POST /api/v1/admin/currencies/icon/upload")
	case adminClient == nil && !c.S3.Enabled:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/currencies/icon/upload (Admin RPC not available, S3 disabled)")
	case adminClient == nil:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/currencies/icon/upload (Admin RPC not available)")
	case !c.S3.Enabled:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/currencies/icon/upload (S3 disabled)")
	default:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/currencies/icon/upload (S3 not available)")
	}

	// 5.3 Chain icon upload: multipart upload to S3 (returns public URL).
	mux.Handle("/api/v1/admin/chains/icon/upload", middleware.JWTAuth(adminJWTSecret)(handler.NewAdminChainIconUploadHandler(s3Uploader, adminClient)))
	switch {
	case s3Uploader != nil && adminClient != nil:
		logx.Info("✓ Registered direct handler: POST /api/v1/admin/chains/icon/upload")
	case adminClient == nil && !c.S3.Enabled:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/chains/icon/upload (Admin RPC not available, S3 disabled)")
	case adminClient == nil:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/chains/icon/upload (Admin RPC not available)")
	case !c.S3.Enabled:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/chains/icon/upload (S3 disabled)")
	default:
		logx.Info("⚠ Registered direct handler: POST /api/v1/admin/chains/icon/upload (S3 not available)")
	}

	// 5.4 Business avatar upload: multipart upload to S3 (returns public URL).
	mux.Handle("/api/v1/business/upload_avatar", middleware.JWTAuth(c.Auth.AccessSecret)(handler.NewBusinessAvatarUploadHandler(s3Uploader)))
	switch {
	case s3Uploader != nil:
		logx.Info("✓ Registered direct handler: POST /api/v1/business/upload_avatar")
	case !c.S3.Enabled:
		logx.Info("⚠ Registered direct handler: POST /api/v1/business/upload_avatar (S3 disabled)")
	default:
		logx.Info("⚠ Registered direct handler: POST /api/v1/business/upload_avatar (S3 not available)")
	}

	// 6. 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	// 注意: "/" 路由已由 DynamicRouter (router.go) 作为 catch-all 注册
	// 根路径探针由 DynamicRouter.ServeHTTP 处理

	// 6.1 Swagger UI (proxied via gateway)
	docs.Register(mux)

	// 7. 应用全局中间件
	corsConfig := middleware.DefaultCORSConfig()
	if len(c.Cors.AllowOrigins) > 0 {
		corsConfig.AllowOrigins = c.Cors.AllowOrigins
		corsConfig.AllowMethods = c.Cors.AllowMethods
		corsConfig.AllowHeaders = c.Cors.AllowHeaders
		corsConfig.ExposeHeaders = c.Cors.ExposeHeaders
		corsConfig.AllowCredentials = c.Cors.AllowCredentials
		corsConfig.MaxAge = c.Cors.MaxAge
	}

	finalHandler := middleware.Chain(
		middleware.Recovery(),              // 1. 异常恢复（最外层）
		middleware.RequestID(),             // 2. 请求ID
		middleware.Logging(),               // 3. 日志记录
		middleware.ExtractClientMetadata(), // 4. 提取客户端元数据（IP、设备指纹等）
		middleware.CORS(corsConfig),        // 5. CORS
		middleware.RateLimit(10000, 20000), // 6. 全局限流（每秒10000次，突发20000）
	)(mux)

	// 8. 创建 HTTP 服务器
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      finalHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 9. 启动服务
	logx.Infof("API Gateway listening on %s", addr)
	logx.Info("Gateway routes:")
	for serviceName, serviceRoutes := range routeConfig.Services {
		logx.Infof("  [%s]", serviceName)
		for _, route := range serviceRoutes.Routes {
			authStr := ""
			if route.Auth {
				authStr = " [Auth Required]"
			}
			logx.Infof("    %s %s -> %s.%s%s", route.Method, route.Path, route.Service, route.RPC, authStr)
		}
	}

	if err := server.ListenAndServe(); err != nil {
		logx.Errorf("Gateway stopped: %v", err)
	}
}
