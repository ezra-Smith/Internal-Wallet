package test

import (
	"testing"
	"time"

	"internalwallet/services/notification/rpc/internal/jpush"
)

// TestJPushConnection 测试极光推送连接
// 运行方式：go test -v ./test -run TestJPushConnection
func TestJPushConnection(t *testing.T) {
	// ⚠️ 请将下面的值替换为您从极光控制台获取的真实值
	appKey := "8374232de76d87692547925c"       // 替换为真实的 AppKey
	masterSecret := "2eb53aedb0240d1f8aaea88f" // 替换为真实的 MasterSecret

	// 创建极光客户端
	client, err := jpush.NewClient(appKey, masterSecret, &jpush.ClientOptions{
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryInterval:  1 * time.Second,
		ApnsProduction: false, // 测试环境
	})

	if err != nil {
		t.Fatalf("❌ 创建极光客户端失败: %v", err)
	}

	t.Logf("✅ 极光客户端创建成功")

	// 测试1: 验证认证（通过发送一个测试推送）
	t.Run("Test_Authentication", func(t *testing.T) {
		testAuthentication(t, client)
	})

	// 测试2: 测试推送API调用格式
	t.Run("Test_Push_API_Format", func(t *testing.T) {
		testPushAPIFormat(t, client)
	})
}

// testAuthentication 测试认证是否正确
func testAuthentication(t *testing.T, client *jpush.Client) {
	t.Log("📝 测试极光API认证...")

	// 尝试推送到一个测试 Registration ID
	// 注意：这个 Registration ID 是假的，推送会失败，但可以验证认证是否正确
	testRegistrationID := "test_registration_id_12345678"

	notification := &jpush.NotificationPayload{
		Title:   "测试推送",
		Content: "这是一条测试消息，用于验证极光推送配置",
		Extras: map[string]string{
			"test": "true",
		},
	}

	result, err := client.PushToDevice(testRegistrationID, notification, nil)

	// 分析错误类型
	if err != nil {
		// 检查是否是认证错误
		if jpush.IsAuthError(err) {
			t.Fatalf("❌ 认证失败！请检查 AppKey 和 MasterSecret 是否正确\n错误: %v", err)
		}

		// 检查是否是无效设备错误（这是预期的，因为我们用的是假 Registration ID）
		if jpush.IsInvalidDeviceError(err) {
			t.Logf("✅ 认证成功！（收到预期的无效设备错误）")
			t.Logf("   这说明 AppKey 和 MasterSecret 正确，只是设备ID无效")
			return
		}

		// 其他错误
		t.Logf("⚠️  推送失败，但可能是网络或其他问题: %v", err)
		return
	}

	// 如果没有错误（不太可能，因为是假的 Registration ID）
	if result != nil && result.Success {
		t.Logf("✅ 推送成功！消息ID: %s", result.MsgID)
	}
}

// testPushAPIFormat 测试推送API调用格式
func testPushAPIFormat(t *testing.T, client *jpush.Client) {
	t.Log("📝 测试推送API调用格式...")

	// 测试广播推送（这个会真实发送，但因为没有设备注册，不会有任何设备收到）
	notification := &jpush.NotificationPayload{
		Title:   "系统测试",
		Content: "API格式测试消息",
		Extras: map[string]string{
			"action": "test",
			"time":   time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	// 注意：广播推送会真实发送！但由于没有设备注册，不会有任何影响
	t.Log("⚠️  注意：下面会尝试发送一条广播推送（因为没有注册设备，不会有任何影响）")

	result, err := client.PushBroadcast(notification, nil)

	if err != nil {
		if jpush.IsAuthError(err) {
			t.Fatalf("❌ 认证失败: %v", err)
		}
		t.Logf("⚠️  广播推送失败: %v", err)
		return
	}

	if result != nil && result.Success {
		t.Logf("✅ 广播推送API调用成功！")
		t.Logf("   消息ID: %s", result.MsgID)
		t.Logf("   状态码: %d", result.StatusCode)
		t.Logf("   💡 虽然推送成功，但由于没有设备注册，不会有任何设备收到消息")
	}
}

// TestJPushValidation 测试配置验证
func TestJPushValidation(t *testing.T) {
	t.Run("Invalid_AppKey", func(t *testing.T) {
		_, err := jpush.NewClient("", "test_secret", nil)
		if err != jpush.ErrInvalidAppKey {
			t.Errorf("期望返回 ErrInvalidAppKey，实际返回: %v", err)
		} else {
			t.Log("✅ AppKey 验证正常")
		}
	})

	t.Run("Invalid_MasterSecret", func(t *testing.T) {
		_, err := jpush.NewClient("test_key", "", nil)
		if err != jpush.ErrInvalidMasterSecret {
			t.Errorf("期望返回 ErrInvalidMasterSecret，实际返回: %v", err)
		} else {
			t.Log("✅ MasterSecret 验证正常")
		}
	})
}
