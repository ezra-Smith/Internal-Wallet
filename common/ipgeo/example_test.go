package ipgeo_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"internalwallet/common/ipgeo"
)

// 示例：基本使用
func ExampleGetGeoLocation_basic() {
	// 查询Google DNS的地理位置
	geo, err := ipgeo.GetGeoLocation(context.Background(), "8.8.8.8")
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("国家: %s\n", geo.Country)
	fmt.Printf("数据源: %s\n", geo.Source)
	// 由于网络返回结果可能不同，这里只演示代码结构
}

// 示例：在登录日志中使用
func ExampleGetGeoLocation_loginLog() {
	userIP := "114.114.114.114"

	// 设置3秒超时
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	geo, err := ipgeo.GetGeoLocation(ctx, userIP)
	if err != nil {
		// 查询失败不影响主流程，使用默认值
		log.Printf("IP地理位置查询失败: %v", err)
		saveLoginLog(userIP, "未知", "未知", "未知")
		return
	}

	// 保存登录日志
	saveLoginLog(userIP, geo.Country, geo.Province, geo.City)
	fmt.Printf("登录位置: %s %s %s\n", geo.Country, geo.Province, geo.City)
}

// 示例：带重试逻辑
func ExampleGetGeoLocation_withRetry() {
	ip := "1.1.1.1"
	maxRetries := 3

	var geo *ipgeo.GeoLocation
	var err error

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		geo, err = ipgeo.GetGeoLocation(ctx, ip)
		cancel()

		if err == nil {
			break
		}

		log.Printf("尝试 %d/%d 失败: %v", i+1, maxRetries, err)
		time.Sleep(time.Second)
	}

	if err != nil {
		log.Fatalf("所有重试都失败: %v", err)
	}

	fmt.Printf("位置: %s %s\n", geo.Country, geo.City)
}

// 示例：使用便捷函数
func ExampleGetGeoLocationWithTimeout() {
	// 直接指定超时时间
	geo, err := ipgeo.GetGeoLocationWithTimeout("8.8.8.8", 5*time.Second)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}

	fmt.Printf("IP: %s, 位置: %s %s %s\n",
		geo.IP, geo.Country, geo.Province, geo.City)
}

// 示例：批量查询
func ExampleGetGeoLocation_batch() {
	ips := []string{"8.8.8.8", "114.114.114.114", "1.1.1.1"}

	type result struct {
		IP  string
		Geo *ipgeo.GeoLocation
		Err error
	}

	results := make(chan result, len(ips))

	// 并发查询
	for _, ip := range ips {
		go func(ip string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			geo, err := ipgeo.GetGeoLocation(ctx, ip)
			results <- result{IP: ip, Geo: geo, Err: err}
		}(ip)
	}

	// 收集结果
	for i := 0; i < len(ips); i++ {
		res := <-results
		if res.Err != nil {
			fmt.Printf("%s: 查询失败 - %v\n", res.IP, res.Err)
			continue
		}
		fmt.Printf("%s: %s %s %s (来源: %s)\n",
			res.IP, res.Geo.Country, res.Geo.Province,
			res.Geo.City, res.Geo.Source)
	}
}

// 模拟保存登录日志的函数
func saveLoginLog(ip, country, province, city string) {
	// 实际实现中会保存到数据库
	log.Printf("保存登录日志: IP=%s, 位置=%s %s %s", ip, country, province, city)
}
