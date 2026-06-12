package interceptor

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// IPWhitelistConfig IP白名单配置
type IPWhitelistConfig struct {
	Enabled      bool     // 是否启用白名单
	AllowedIPs   []string // 允许的IP列表（精确匹配）
	AllowedCIDRs []string // 允许的CIDR列表（网段匹配）
}

// IPWhitelistInterceptor IP白名单拦截器
type IPWhitelistInterceptor struct {
	config      *IPWhitelistConfig
	allowedIPs  map[string]bool // IP精确匹配缓存
	allowedNets []*net.IPNet    // CIDR网络列表
	logger      logx.Logger
	serviceName string
}

// NewIPWhitelistInterceptor 创建IP白名单拦截器
func NewIPWhitelistInterceptor(config *IPWhitelistConfig, serviceName string) (*IPWhitelistInterceptor, error) {
	interceptor := &IPWhitelistInterceptor{
		config:      config,
		allowedIPs:  make(map[string]bool),
		allowedNets: make([]*net.IPNet, 0),
		logger:      logx.WithContext(context.Background()),
		serviceName: serviceName,
	}

	// 如果未启用，直接返回
	if !config.Enabled {
		interceptor.logger.Infof("[%s] IP whitelist is disabled", serviceName)
		return interceptor, nil
	}

	// 解析IP列表
	for _, ip := range config.AllowedIPs {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		// 验证IP格式
		if net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("invalid IP address: %s", ip)
		}
		interceptor.allowedIPs[ip] = true
	}

	// 解析CIDR列表
	for _, cidr := range config.AllowedCIDRs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %s, error: %w", cidr, err)
		}
		interceptor.allowedNets = append(interceptor.allowedNets, ipNet)
	}

	interceptor.logger.Infof("[%s] IP whitelist enabled: %d IPs, %d CIDRs",
		serviceName, len(interceptor.allowedIPs), len(interceptor.allowedNets))

	return interceptor, nil
}

// UnaryServerInterceptor gRPC一元拦截器
func (i *IPWhitelistInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 如果未启用白名单，直接放行
		if !i.config.Enabled {
			return handler(ctx, req)
		}

		// 获取客户端IP
		clientIP, err := i.getClientIP(ctx)
		if err != nil {
			i.logger.Errorf("[%s] Failed to get client IP: %v", i.serviceName, err)
			return nil, status.Errorf(codes.Internal, "failed to determine client IP")
		}

		// 检查IP是否在白名单中
		if !i.isIPAllowed(clientIP) {
			i.logger.Errorf("[%s] IP not in whitelist: %s, method: %s",
				i.serviceName, clientIP, info.FullMethod)
			return nil, status.Errorf(codes.PermissionDenied,
				"access denied: IP %s is not in whitelist", clientIP)
		}

		// IP验证通过，记录日志
		i.logger.Infof("[%s] IP whitelist check passed: %s, method: %s",
			i.serviceName, clientIP, info.FullMethod)

		// 调用实际的处理器
		return handler(ctx, req)
	}
}

// getClientIP 从gRPC上下文中获取客户端IP
func (i *IPWhitelistInterceptor) getClientIP(ctx context.Context) (string, error) {
	// 从peer中获取客户端地址
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("failed to get peer from context")
	}

	// 解析地址，去除端口号
	addr := p.Addr.String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// 如果没有端口号，直接使用原地址
		host = addr
	}

	return host, nil
}

// isIPAllowed 检查IP是否在白名单中
func (i *IPWhitelistInterceptor) isIPAllowed(ip string) bool {
	// 1. 精确IP匹配
	if i.allowedIPs[ip] {
		return true
	}

	// 2. CIDR网段匹配
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		i.logger.Errorf("Invalid IP format: %s", ip)
		return false
	}

	for _, ipNet := range i.allowedNets {
		if ipNet.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// GetAllowedIPs 获取允许的IP列表（用于调试）
func (i *IPWhitelistInterceptor) GetAllowedIPs() []string {
	ips := make([]string, 0, len(i.allowedIPs))
	for ip := range i.allowedIPs {
		ips = append(ips, ip)
	}
	return ips
}

// GetAllowedCIDRs 获取允许的CIDR列表（用于调试）
func (i *IPWhitelistInterceptor) GetAllowedCIDRs() []string {
	cidrs := make([]string, 0, len(i.allowedNets))
	for _, ipNet := range i.allowedNets {
		cidrs = append(cidrs, ipNet.String())
	}
	return cidrs
}
