package ipgeo

import (
	"context"
	"testing"
	"time"
)

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			ip:      "43.212.169.167",
			wantErr: false,
		},
		{
			name:    "valid IPv4 - Chinese IP",
			ip:      "43.212.169.167",
			wantErr: false,
		},
		{
			name:    "valid IPv6",
			ip:      "103.219.195.219",
			wantErr: false,
		},
		{
			name:    "empty IP",
			ip:      "",
			wantErr: true,
		},
		{
			name:    "invalid IP format",
			ip:      "155.117.85.115",
			wantErr: true,
		},
		{
			name:    "invalid IP string",
			ip:      "not-an-ip",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetGeoLocation_ValidIP(t *testing.T) {
	// 这个测试需要网络连接，可能会失败如果服务不可用
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "Google DNS - should work with ip-api",
			ip:      "155.117.85.115",
			wantErr: false,
		},
		{
			name:    "Chinese DNS - should work with pconline",
			ip:      "66.92.32.44",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			geo, err := GetGeoLocation(ctx, tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGeoLocation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// 验证返回的数据
				if geo.IP != tt.ip {
					t.Errorf("GetGeoLocation() IP = %v, want %v", geo.IP, tt.ip)
				}
				if geo.Source == "" {
					t.Errorf("GetGeoLocation() Source is empty")
				}
				if geo.Country == "" {
					t.Errorf("GetGeoLocation() Country is empty")
				}

				t.Logf("IP: %s, Country: %s, Province: %s, City: %s, ISP: %s, Source: %s",
					geo.IP, geo.Country, geo.Province, geo.City, geo.ISP, geo.Source)
			}
		})
	}
}

func TestGetGeoLocation_InvalidIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "empty IP",
			ip:      "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			ip:      "not-an-ip",
			wantErr: true,
		},
		{
			name:    "out of range",
			ip:      "999.999.999.999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := GetGeoLocation(ctx, tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGeoLocation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetGeoLocation_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	// 创建一个非常短的超时context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// 等待context超时
	time.Sleep(10 * time.Millisecond)

	_, err := GetGeoLocation(ctx, "8.8.8.8")
	if err == nil {
		t.Error("GetGeoLocation() expected timeout error, got nil")
	}
}

func TestGetGeoLocationWithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	geo, err := GetGeoLocationWithTimeout("8.8.8.8", 10*time.Second)
	if err != nil {
		t.Logf("GetGeoLocationWithTimeout() warning: %v (may fail if network unavailable)", err)
		return
	}

	if geo.IP != "8.8.8.8" {
		t.Errorf("GetGeoLocationWithTimeout() IP = %v, want %v", geo.IP, "8.8.8.8")
	}

	t.Logf("Result: %+v", geo)
}

func TestQueryPconline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试中国IP
	geo, err := queryPconline(ctx, "114.114.114.114")
	if err != nil {
		t.Logf("queryPconline() warning: %v (may fail if service unavailable)", err)
		return
	}

	if geo.Source != sourcePconline {
		t.Errorf("queryPconline() Source = %v, want %v", geo.Source, sourcePconline)
	}

	t.Logf("Pconline Result: %+v", geo)
}

func TestQueryIPAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试Google DNS IP
	geo, err := queryIPAPI(ctx, "8.8.8.8")
	if err != nil {
		t.Logf("queryIPAPI() warning: %v (may fail if service unavailable)", err)
		return
	}

	if geo.Source != sourceIPAPI {
		t.Errorf("queryIPAPI() Source = %v, want %v", geo.Source, sourceIPAPI)
	}

	// 验证中文返回
	if geo.Country == "" {
		t.Error("queryIPAPI() Country is empty")
	}

	t.Logf("IP-API Result: %+v", geo)
}

// Example_getGeoLocation 演示如何使用GetGeoLocation
func Example_getGeoLocation() {
	// 简单调用
	geo, err := GetGeoLocation(context.Background(), "8.8.8.8")
	if err != nil {
		panic(err)
	}
	println("Country:", geo.Country)
	println("Province:", geo.Province)
	println("City:", geo.City)
}

// Example_getGeoLocationWithTimeout 演示如何使用自定义超时
func Example_getGeoLocationWithTimeout() {
	// 带超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	geo, err := GetGeoLocation(ctx, "114.114.114.114")
	if err != nil {
		panic(err)
	}
	println("Location:", geo.Country, geo.Province, geo.City)
}
