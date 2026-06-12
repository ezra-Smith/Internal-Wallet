package ipgeo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	// 太平洋IP查询地址（国内快速）
	pconlineURL = "http://whois.pconline.com.cn/ipJson.jsp"

	// ip-api.com查询地址（国际备用）
	ipAPIURL = "http://ip-api.com/json"

	// 默认超时时间
	defaultTimeout = 5 * time.Second

	// 数据源标识
	sourcePconline = "pconline"
	sourceIPAPI    = "ip-api"
)

// GetGeoLocation 查询IP地址的地理位置信息
// 优先使用太平洋IP（国内用户快），失败时自动切换到ip-api.com
func GetGeoLocation(ctx context.Context, ip string) (*GeoLocation, error) {
	// 验证IP地址格式
	if err := validateIP(ip); err != nil {
		return nil, fmt.Errorf("invalid IP address: %w", err)
	}

	// 如果没有设置超时，使用默认超时
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	// 优先尝试太平洋IP（国内用户友好）
	geo, err := queryPconline(ctx, ip)
	if err == nil {
		return geo, nil
	}

	// 太平洋IP失败，尝试ip-api.com作为备用
	geo, err = queryIPAPI(ctx, ip)
	if err == nil {
		return geo, nil
	}

	// 两个服务都失败
	return nil, fmt.Errorf("all IP geolocation services failed: %w", err)
}

// validateIP 验证IP地址格式
func validateIP(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address is empty")
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP format: %s", ip)
	}

	return nil
}

// queryPconline 查询太平洋IP数据库
func queryPconline(ctx context.Context, ip string) (*GeoLocation, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s?ip=%s&json=true", pconlineURL, ip)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create pconline request: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Internal-Wallet/1.0)")

	// 发送请求
	client := &http.Client{
		Timeout: defaultTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pconline request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pconline returned status: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read pconline response: %w", err)
	}

	// 太平洋IP返回GBK编码，需要转换为UTF-8
	// 使用transform.NewReader将GBK解码为UTF-8
	decoder := simplifiedchinese.GBK.NewDecoder()
	utf8Body, err := io.ReadAll(transform.NewReader(strings.NewReader(string(body)), decoder))
	if err != nil {
		// 转码失败，尝试直接解析（可能是纯ASCII或已经是UTF-8）
		utf8Body = body
	}

	// 解析JSON响应
	var pcResp pconlineResponse
	if err := json.Unmarshal(utf8Body, &pcResp); err != nil {
		return nil, fmt.Errorf("failed to parse pconline response: %w", err)
	}

	// 检查是否有错误信息
	if pcResp.Err != "" {
		return nil, fmt.Errorf("pconline error: %s", pcResp.Err)
	}

	// 转换为统一格式
	geo := &GeoLocation{
		IP:       ip,
		Country:  "中国", // 太平洋IP主要服务中国区域
		Province: strings.TrimSpace(pcResp.Pro),
		City:     strings.TrimSpace(pcResp.City),
		ISP:      "", // 太平洋IP不提供ISP信息
		Source:   sourcePconline,
	}

	// 如果Province为空，尝试从Region解析
	if geo.Province == "" && pcResp.Region != "" {
		geo.Province = strings.TrimSpace(pcResp.Region)
	}

	// 对于海外IP，太平洋IP可能返回地址信息
	if strings.Contains(pcResp.Addr, "国外") || strings.Contains(pcResp.Addr, "保留地址") {
		// 如果是海外或保留地址，可以尝试从addr中提取信息
		if pcResp.RegionNames != "" {
			geo.Country = strings.TrimSpace(pcResp.RegionNames)
		}
	}

	return geo, nil
}

// queryIPAPI 查询ip-api.com数据库
func queryIPAPI(ctx context.Context, ip string) (*GeoLocation, error) {
	// 构建请求URL，使用中文语言参数
	url := fmt.Sprintf("%s/%s?lang=zh-CN", ipAPIURL, ip)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ip-api request: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Internal-Wallet/1.0)")

	// 发送请求
	client := &http.Client{
		Timeout: defaultTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ip-api request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ip-api returned status: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ip-api response: %w", err)
	}

	// 解析JSON响应
	var apiResp ipAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse ip-api response: %w", err)
	}

	// 检查是否查询成功
	if apiResp.Status != "success" {
		return nil, fmt.Errorf("ip-api query failed: %s", apiResp.Message)
	}

	// 转换为统一格式
	geo := &GeoLocation{
		IP:       ip,
		Country:  strings.TrimSpace(apiResp.Country),
		Province: strings.TrimSpace(apiResp.RegionName),
		City:     strings.TrimSpace(apiResp.City),
		ISP:      strings.TrimSpace(apiResp.ISP),
		Source:   sourceIPAPI,
	}

	return geo, nil
}

// GetGeoLocationWithTimeout 带自定义超时的查询方法
func GetGeoLocationWithTimeout(ip string, timeout time.Duration) (*GeoLocation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return GetGeoLocation(ctx, ip)
}
