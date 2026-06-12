package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RouteConfig 路由配置
type RouteConfig struct {
	Services map[string]ServiceRoutes `yaml:"services"`
}

// ServiceRoutes 服务路由配置
type ServiceRoutes struct {
	ServiceName string  `yaml:"service_name"`
	Routes      []Route `yaml:"routes"`
}

// Route 路由定义
type Route struct {
	Path          string   `yaml:"path"`
	Method        string   `yaml:"method"`
	RPC           string   `yaml:"rpc"`
	Service       string   `yaml:"service"`
	Auth          bool     `yaml:"auth"`
	RateLimit     int      `yaml:"rate_limit"`
	RateLimitType string   `yaml:"rate_limit_type"` // ip, user, none (默认：auth=false用ip，auth=true用user)
	PathParams    []string `yaml:"path_params"`
	Description   string   `yaml:"description"`
}

// LoadRoutesFromFile 从文件加载路由配置
func LoadRoutesFromFile(filePath string) (*RouteConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes file: %w", err)
	}

	var config RouteConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse routes file: %w", err)
	}

	for name, sr := range config.Services {
		if sr.ServiceName == "" {
			sr.ServiceName = name
			config.Services[name] = sr
		}
	}

	return &config, nil
}

// GetServiceRoutes 获取指定服务的路由
func (c *RouteConfig) GetServiceRoutes(serviceName string) (ServiceRoutes, bool) {
	routes, ok := c.Services[serviceName]
	return routes, ok
}

// GetAllServices 获取所有服务名称
func (c *RouteConfig) GetAllServices() []string {
	services := make([]string, 0, len(c.Services))
	for name := range c.Services {
		services = append(services, name)
	}
	return services
}

// GetRateLimitType 获取限流类型（处理默认值）
func (r *Route) GetRateLimitType() string {
	if r.RateLimit <= 0 {
		return "none"
	}

	// 如果明确指定了类型，使用指定的类型
	if r.RateLimitType != "" {
		return r.RateLimitType
	}

	// 默认规则：
	// - auth: false -> ip限流（公开接口）
	// - auth: true  -> user限流（业务接口）
	if r.Auth {
		return "user"
	}
	return "ip"
}

// Validate 验证路由配置
func (c *RouteConfig) Validate() error {
	for serviceName, serviceRoutes := range c.Services {
		if serviceRoutes.ServiceName == "" {
			serviceRoutes.ServiceName = serviceName
			c.Services[serviceName] = serviceRoutes
		}

		for i, route := range serviceRoutes.Routes {
			if route.Path == "" {
				return fmt.Errorf("service %s, route %d: path is required", serviceName, i)
			}
			if route.Method == "" {
				return fmt.Errorf("service %s, route %d: method is required", serviceName, i)
			}
			if route.RPC == "" {
				return fmt.Errorf("service %s, route %d: rpc is required", serviceName, i)
			}
			if route.Service == "" {
				return fmt.Errorf("service %s, route %d: service is required", serviceName, i)
			}

			// 验证限流类型
			limitType := route.GetRateLimitType()
			if limitType != "none" && limitType != "ip" && limitType != "user" {
				return fmt.Errorf("service %s, route %d: invalid rate_limit_type '%s' (must be: ip, user, or none)", serviceName, i, route.RateLimitType)
			}

			// 验证：user限流必须开启认证
			if limitType == "user" && !route.Auth {
				return fmt.Errorf("service %s, route %d: rate_limit_type 'user' requires auth: true", serviceName, i)
			}
		}
	}

	return nil
}
