# IP地理位置查询工具

一个轻量级的IP地理位置查询工具，结合太平洋IP和ip-api.com两个免费服务，提供统一的中文地理位置查询接口。

## 特性

- **双服务池化**: 结合太平洋IP（pconline.com.cn）和ip-api.com
- **智能切换**: 优先使用太平洋IP（国内快），失败时自动切换到ip-api.com
- **中文返回**: 统一返回中文国家、省份、城市信息
- **上下文支持**: 支持context超时控制和取消
- **零配置**: 无需API Key，开箱即用

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "internalwallet/common/ipgeo"
)

func main() {
    // 简单调用
    geo, err := ipgeo.GetGeoLocation(context.Background(), "8.8.8.8")
    if err != nil {
        log.Fatalf("查询失败: %v", err)
    }
    
    fmt.Printf("IP: %s\n", geo.IP)
    fmt.Printf("国家: %s\n", geo.Country)
    fmt.Printf("省份: %s\n", geo.Province)
    fmt.Printf("城市: %s\n", geo.City)
    fmt.Printf("运营商: %s\n", geo.ISP)
    fmt.Printf("数据源: %s\n", geo.Source)
}
```

### 带超时控制

```go
import (
    "context"
    "time"
    
    "internalwallet/common/ipgeo"
)

// 方式1: 使用context.WithTimeout
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

geo, err := ipgeo.GetGeoLocation(ctx, "114.114.114.114")
if err != nil {
    log.Printf("查询失败: %v", err)
    return
}

// 方式2: 使用便捷函数
geo, err := ipgeo.GetGeoLocationWithTimeout("114.114.114.114", 3*time.Second)
```

## API 文档

### GetGeoLocation

```go
func GetGeoLocation(ctx context.Context, ip string) (*GeoLocation, error)
```

查询IP地址的地理位置信息。

**参数:**
- `ctx`: 上下文，用于超时控制和取消
- `ip`: 要查询的IP地址（支持IPv4和IPv6）

**返回:**
- `*GeoLocation`: 地理位置信息
- `error`: 错误信息

### GetGeoLocationWithTimeout

```go
func GetGeoLocationWithTimeout(ip string, timeout time.Duration) (*GeoLocation, error)
```

带自定义超时时间的查询方法。

**参数:**
- `ip`: 要查询的IP地址
- `timeout`: 超时时间

### GeoLocation 结构

```go
type GeoLocation struct {
    IP       string // IP地址
    Country  string // 国家（中文）
    Province string // 省份/州（中文）
    City     string // 城市（中文）
    ISP      string // 运营商（可选）
    Source   string // 数据源 (pconline/ip-api)
}
```

## 工作原理

1. **智能路由**: 优先使用太平洋IP服务（国内用户访问快）
2. **自动降级**: 如果太平洋IP失败，自动切换到ip-api.com
3. **统一格式**: 将不同服务的响应统一转换为中文GeoLocation结构
4. **超时保护**: 默认5秒超时，可通过context自定义
5. **编码处理**: 自动处理太平洋IP的GBK编码，转换为UTF-8，避免中文乱码

## 服务对比

| 服务 | 优先级 | 适用场景 | 特点 |
|------|--------|---------|------|
| 太平洋IP | 1 | 国内IP查询 | 国内访问快，中文原生 |
| ip-api.com | 2 | 国际IP查询 | 支持中文参数，数据准确 |

## 测试

运行单元测试：

```bash
# 运行所有测试
go test ./common/ipgeo

# 跳过网络测试（快速模式）
go test -short ./common/ipgeo

# 详细输出
go test -v ./common/ipgeo
```

## 使用示例

### 在登录日志中记录位置

```go
func recordLoginLog(userID int64, ip string) error {
    geo, err := ipgeo.GetGeoLocation(context.Background(), ip)
    if err != nil {
        // 查询失败不影响主流程
        log.Printf("Failed to get geo location for IP %s: %v", ip, err)
        return saveLoginLog(userID, ip, "", "", "")
    }
    
    return saveLoginLog(userID, ip, geo.Country, geo.Province, geo.City)
}
```

### 在API中间件中使用

```go
func IPLocationMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        clientIP := c.ClientIP()
        
        ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
        defer cancel()
        
        geo, err := ipgeo.GetGeoLocation(ctx, clientIP)
        if err == nil {
            c.Set("geo_location", geo)
            c.Set("geo_country", geo.Country)
            c.Set("geo_city", geo.City)
        }
        
        c.Next()
    }
}
```

## 技术细节

### 编码处理

**太平洋IP编码问题及修复**：
- **问题**: 太平洋IP接口返回 **GBK编码** 的JSON，导致中文乱码
  - 示例: `"pro":"�Ϻ���"` (应该是 `"pro":"上海市"`)
- **修复**: 使用 `golang.org/x/text/encoding/simplifiedchinese` 将GBK转UTF-8
- **代码**: 在解析JSON前自动转换编码
  ```go
  decoder := simplifiedchinese.GBK.NewDecoder()
  utf8Body, _ := io.ReadAll(transform.NewReader(strings.NewReader(string(body)), decoder))
  json.Unmarshal(utf8Body, &pcResp)
  ```
- **测试**: 已添加专门的编码测试用例防止回归

## 注意事项

1. **网络依赖**: 需要访问外部API服务，请确保网络连通
2. **速率限制**: 
   - 太平洋IP: 无明确限制，建议合理使用
   - ip-api.com: 45请求/分钟（免费）
3. **准确性**: IP地理位置数据可能不完全准确，仅供参考
4. **缓存建议**: 对于相同IP，建议在应用层缓存结果以减少API调用
5. **编码兼容**: 已处理太平洋IP的GBK编码问题，中文显示正常

## 许可证

本工具为Internal-Wallet项目的一部分。
