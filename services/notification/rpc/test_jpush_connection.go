//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"time"

	"internalwallet/services/notification/rpc/internal/jpush"
)

func main() {
	fmt.Println("==============================================")
	fmt.Println("   极光推送连接测试工具")
	fmt.Println("==============================================")
	fmt.Println()

	// ⚠️ 请在这里填入您从极光控制台获取的真实值
	appKey := "your_actual_app_key"             // 🔑 替换为真实的 AppKey
	masterSecret := "your_actual_master_secret" // 🔑 替换为真实的 MasterSecret

	// 检查是否已更新配置
	if appKey == "your_actual_app_key" || masterSecret == "your_actual_master_secret" {
		fmt.Println("❌ 错误：请先在代码中填入真实的 AppKey 和 MasterSecret")
		fmt.Println("   打开文件：services/notification/rpc/test_jpush_connection.go")
		fmt.Println("   在第14-15行填入您的真实配置")
		return
	}

	fmt.Println("📋 配置信息：")
	fmt.Printf("   AppKey: %s\n", maskString(appKey))
	fmt.Printf("   MasterSecret: %s\n", maskString(masterSecret))
	fmt.Println()

	// 步骤1: 创建极光客户端
	fmt.Println("步骤 1/3: 创建极光推送客户端...")
	client, err := jpush.NewClient(appKey, masterSecret, &jpush.ClientOptions{
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryInterval:  1 * time.Second,
		ApnsProduction: false, // 测试环境
	})

	if err != nil {
		fmt.Printf("❌ 创建客户端失败: %v\n", err)
		return
	}
	fmt.Println("✅ 客户端创建成功")
	fmt.Println()

	// 步骤2: 测试认证
	fmt.Println("步骤 2/3: 测试认证信息...")
	fmt.Println("   使用假的 Registration ID 测试API调用...")

	testRegistrationID := "test_device_registration_id_12345678"
	notification := &jpush.NotificationPayload{
		Title:   "测试推送",
		Content: "这是一条测试消息，用于验证极光推送配置是否正确",
		Extras: map[string]string{
			"test":   "true",
			"action": "connection_test",
		},
	}

	result, err := client.PushToDevice(testRegistrationID, notification, nil)

	if err != nil {
		// 分析错误类型
		if jpush.IsAuthError(err) {
			fmt.Println("❌ 认证失败！")
			fmt.Println("   可能的原因：")
			fmt.Println("   1. AppKey 不正确")
			fmt.Println("   2. MasterSecret 不正确")
			fmt.Println("   3. 账号被禁用")
			fmt.Printf("   错误详情: %v\n", err)
			return
		}

		if jpush.IsInvalidDeviceError(err) {
			fmt.Println("✅ 认证成功！")
			fmt.Println("   收到了预期的 '无效设备' 错误")
			fmt.Println("   这说明您的 AppKey 和 MasterSecret 配置正确")
			fmt.Println("   只是因为使用了假的 Registration ID")
			fmt.Println()
		} else {
			fmt.Printf("⚠️  收到其他错误: %v\n", err)
			fmt.Println("   但这不一定表示配置错误，可能是网络问题")
			fmt.Println()
		}
	} else if result != nil && result.Success {
		fmt.Println("✅ 推送API调用成功！")
		fmt.Printf("   消息ID: %s\n", result.MsgID)
		fmt.Println()
	}

	// 步骤3: 测试广播推送格式
	fmt.Println("步骤 3/3: 测试广播推送API格式...")
	fmt.Println("   ⚠️  注意：这会发送一条真实的广播推送")
	fmt.Println("   但由于没有设备注册，不会有任何设备收到消息")
	fmt.Print("   继续测试？(输入 y 继续，其他键跳过): ")

	// 简单跳过，避免意外发送
	fmt.Println("跳过")
	fmt.Println()

	// 总结
	fmt.Println("==============================================")
	fmt.Println("   测试完成")
	fmt.Println("==============================================")
	fmt.Println()
	fmt.Println("✨ 下一步：")
	fmt.Println("   1. 如果认证成功，更新配置文件：")
	fmt.Println("      services/notification/rpc/etc/notification.local.yaml")
	fmt.Println()
	fmt.Println("   2. 在配置文件中填入真实的 AppKey 和 MasterSecret")
	fmt.Println()
	fmt.Println("   3. 启动 notification 服务：")
	fmt.Println("      make run-notification")
	fmt.Println()
	fmt.Println("   4. 等待 Flutter APP 上线后，使用真实设备测试")
	fmt.Println()
	fmt.Println("📚 相关文档：")
	fmt.Println("   - Flutter 集成指南: docs/FLUTTER_JPUSH_INTEGRATION_GUIDE.md")
	fmt.Println("   - 服务集成示例: docs/NOTIFICATION_SERVICE_INTEGRATION_EXAMPLE.md")
	fmt.Println()
}

// maskString 遮罩字符串，只显示前后几位
func maskString(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
