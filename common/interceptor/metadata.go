package interceptor

import (
	"context"
	"fmt"
	"strconv"

	"internalwallet/common/middleware"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MetadataKey 定义 gRPC Metadata 的键名
const (
	// GRPCUserIDKey gRPC元数据中的用户ID键
	GRPCUserIDKey = "x-user-id"
	// GRPCEmailKey gRPC元数据中的邮箱键
	GRPCEmailKey = "x-email"
	// GRPCClientIPKey gRPC元数据中的客户端IP键
	GRPCClientIPKey = "x-client-ip"
	// GRPCUserAgentKey gRPC元数据中的User-Agent键
	GRPCUserAgentKey = "x-user-agent"

	// GRPCHTTPMethodKey gRPC元数据中的 HTTP Method（由网关注入）
	GRPCHTTPMethodKey = "x-http-method"
	// GRPCHTTPPathKey gRPC元数据中的 HTTP Path（由网关注入）
	GRPCHTTPPathKey = "x-http-path"
	// GRPCDeviceFingerprintKey gRPC元数据中的设备指纹键
	GRPCDeviceFingerprintKey = "x-device-fingerprint"
	// GRPCDeviceIDKey gRPC元数据中的设备ID键
	GRPCDeviceIDKey = "x-device-id"
	// GRPCPlatformKey gRPC元数据中的平台键
	GRPCPlatformKey = "x-platform"
	// GRPCAppVersionKey gRPC元数据中的APP版本键
	GRPCAppVersionKey = "x-app-version"
	// GRPCLocaleKey gRPC元数据中的语言偏好键（来自 X-Lang / Accept-Language）
	GRPCLocaleKey = "x-locale"

	// GRPCRequestIDKey gRPC元数据中的请求ID键
	GRPCRequestIDKey = "x-request-id"
	// GRPCIdempotencyKey gRPC元数据中的幂等键
	GRPCIdempotencyKey = "x-idempotency-key"

	// GRPCJWTIDKey gRPC元数据中的JWT jti键
	GRPCJWTIDKey = "x-jwt-id"
	// GRPCJWTExpKey gRPC元数据中的JWT exp键（Unix秒）
	GRPCJWTExpKey = "x-jwt-exp"
	// GRPCJWTVerKey gRPC元数据中的JWT token version键
	GRPCJWTVerKey = "x-jwt-ver"
)

// ClientMetadataUnaryInterceptor 客户端一元拦截器
// 从 Context 中提取元数据并注入到 gRPC Metadata 中
func ClientMetadataUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// 从 Context 提取元数据
		md := ExtractMetadataFromContext(ctx)

		// 如果有元数据，注入到 gRPC Context
		if len(md) > 0 {
			ctx = metadata.NewOutgoingContext(ctx, md)
		}

		// 调用实际的 RPC 方法
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ExtractMetadataFromContext 从 Context 提取元数据（导出函数，供其他包使用）
func ExtractMetadataFromContext(ctx context.Context) metadata.MD {
	md := metadata.MD{}

	// 提取用户ID（最重要！）
	if userID := middleware.GetUserID(ctx); userID != "" {
		md.Set(GRPCUserIDKey, userID)
	}

	// 提取邮箱
	if email := middleware.GetEmail(ctx); email != "" {
		md.Set(GRPCEmailKey, email)
	}

	// 提取客户端IP
	if ip := middleware.GetClientIP(ctx); ip != "" {
		md.Set(GRPCClientIPKey, ip)
	}

	// 提取 User-Agent
	if ua := middleware.GetUserAgent(ctx); ua != "" {
		md.Set(GRPCUserAgentKey, ua)
	}

	// 提取设备指纹
	if fp := middleware.GetDeviceFingerprint(ctx); fp != "" {
		md.Set(GRPCDeviceFingerprintKey, fp)
	}

	// 提取设备ID
	if deviceID := middleware.GetDeviceID(ctx); deviceID != "" {
		md.Set(GRPCDeviceIDKey, deviceID)
	}

	// 提取平台
	if platform := middleware.GetPlatform(ctx); platform != "" {
		md.Set(GRPCPlatformKey, platform)
	}

	// 提取APP版本
	if appVersion := middleware.GetAppVersion(ctx); appVersion != "" {
		md.Set(GRPCAppVersionKey, appVersion)
	}

	// 提取语言偏好
	if locale := middleware.GetLocale(ctx); locale != "" {
		md.Set(GRPCLocaleKey, locale)
	}

	// 提取 HTTP 元信息（由网关注入）
	if method := middleware.GetHTTPMethod(ctx); method != "" {
		md.Set(GRPCHTTPMethodKey, method)
	}
	if path := middleware.GetHTTPPath(ctx); path != "" {
		md.Set(GRPCHTTPPathKey, path)
	}

	// 提取请求ID
	if requestID := middleware.GetRequestID(ctx); requestID != "" {
		md.Set(GRPCRequestIDKey, requestID)
	}

	// 提取幂等键
	if key := middleware.GetIdempotencyKey(ctx); key != "" {
		md.Set(GRPCIdempotencyKey, key)
	}

	// 提取 JWT 元信息
	if jwtID := middleware.GetJWTID(ctx); jwtID != "" {
		md.Set(GRPCJWTIDKey, jwtID)
	}
	if exp := middleware.GetJWTExpiresAt(ctx); exp > 0 {
		md.Set(GRPCJWTExpKey, fmt.Sprintf("%d", exp))
	}
	if ver := middleware.GetJWTTokenVersion(ctx); ver > 0 {
		md.Set(GRPCJWTVerKey, fmt.Sprintf("%d", ver))
	}

	return md
}

// ServerMetadataUnaryInterceptor 服务端一元拦截器
// 从 gRPC Metadata 中提取元数据并存储到 Context 中
// 供后端服务使用
func ServerMetadataUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 从 gRPC Metadata 提取元数据
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			// 没有元数据，直接调用处理器
			return handler(ctx, req)
		}

		// 将元数据注入到 Context
		ctx = injectMetadataToContext(ctx, md)

		// 调用实际的处理器
		return handler(ctx, req)
	}
}

// injectMetadataToContext 将 gRPC Metadata 注入到 Context
func injectMetadataToContext(ctx context.Context, md metadata.MD) context.Context {
	// 提取用户ID
	if userIDs := md.Get(GRPCUserIDKey); len(userIDs) > 0 {
		ctx = context.WithValue(ctx, "user_id", userIDs[0])
	}

	// 提取邮箱
	if emails := md.Get(GRPCEmailKey); len(emails) > 0 {
		ctx = context.WithValue(ctx, "email", emails[0])
	}

	// 提取客户端IP
	if ips := md.Get(GRPCClientIPKey); len(ips) > 0 {
		ctx = context.WithValue(ctx, middleware.ClientIPKey, ips[0])
	}

	// 提取 User-Agent
	if uas := md.Get(GRPCUserAgentKey); len(uas) > 0 {
		ctx = context.WithValue(ctx, middleware.UserAgentKey, uas[0])
	}

	// 提取设备指纹
	if fps := md.Get(GRPCDeviceFingerprintKey); len(fps) > 0 {
		ctx = context.WithValue(ctx, middleware.DeviceFingerprintKey, fps[0])
	}

	// 提取设备ID
	if deviceIDs := md.Get(GRPCDeviceIDKey); len(deviceIDs) > 0 {
		ctx = context.WithValue(ctx, middleware.DeviceIDKey, deviceIDs[0])
	}

	// 提取平台
	if platforms := md.Get(GRPCPlatformKey); len(platforms) > 0 {
		ctx = context.WithValue(ctx, middleware.PlatformKey, platforms[0])
	}

	// 提取APP版本
	if appVersions := md.Get(GRPCAppVersionKey); len(appVersions) > 0 {
		ctx = context.WithValue(ctx, middleware.AppVersionKey, appVersions[0])
	}

	// 提取语言偏好
	if locales := md.Get(GRPCLocaleKey); len(locales) > 0 {
		ctx = context.WithValue(ctx, middleware.LocaleKey, locales[0])
	}

	// 提取 HTTP 元信息
	if methods := md.Get(GRPCHTTPMethodKey); len(methods) > 0 {
		ctx = context.WithValue(ctx, middleware.HTTPMethodKey, methods[0])
	}
	if paths := md.Get(GRPCHTTPPathKey); len(paths) > 0 {
		ctx = context.WithValue(ctx, middleware.HTTPPathKey, paths[0])
	}

	// 提取请求ID
	if ids := md.Get(GRPCRequestIDKey); len(ids) > 0 {
		ctx = context.WithValue(ctx, middleware.RequestIDKey, ids[0])
	}

	// 提取幂等键
	if keys := md.Get(GRPCIdempotencyKey); len(keys) > 0 {
		ctx = context.WithValue(ctx, middleware.IdempotencyKeyKey, keys[0])
	}

	// 提取 JWT 元信息
	if jtids := md.Get(GRPCJWTIDKey); len(jtids) > 0 {
		ctx = context.WithValue(ctx, middleware.JWTIDKey, jtids[0])
	}
	if exps := md.Get(GRPCJWTExpKey); len(exps) > 0 {
		if v, err := strconv.ParseInt(exps[0], 10, 64); err == nil {
			ctx = context.WithValue(ctx, middleware.JWTExpiresAtKey, v)
		}
	}
	if vers := md.Get(GRPCJWTVerKey); len(vers) > 0 {
		if v, err := strconv.ParseInt(vers[0], 10, 64); err == nil {
			ctx = context.WithValue(ctx, middleware.JWTTokenVersionKey, v)
		}
	}

	return ctx
}
