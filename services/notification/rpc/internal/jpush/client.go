package jpush

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// JPush API地址
	JPushAPIBaseURL = "https://api.jpush.cn/v3"
	// Push API路径
	JPushPushPath = "/push"
	// 查询推送状态路径
	JPushStatusPath = "/reports/messages"
)

// Client 极光推送客户端
type Client struct {
	appKey       string
	masterSecret string
	baseURL      string
	httpClient   *http.Client
	options      *ClientOptions
	logger       logx.Logger
}

// ClientOptions 客户端选项
type ClientOptions struct {
	Timeout        time.Duration // 超时时间
	MaxRetries     int           // 最大重试次数
	RetryInterval  time.Duration // 重试间隔
	ApnsProduction bool          // iOS推送环境
}

// NewClient 创建极光推送客户端
func NewClient(appKey, masterSecret string, options *ClientOptions) (*Client, error) {
	if appKey == "" {
		return nil, ErrInvalidAppKey
	}
	if masterSecret == "" {
		return nil, ErrInvalidMasterSecret
	}

	if options == nil {
		options = &ClientOptions{
			Timeout:        5 * time.Second,
			MaxRetries:     DefaultMaxRetries,
			RetryInterval:  1 * time.Second,
			ApnsProduction: true,
		}
	}

	return &Client{
		appKey:       appKey,
		masterSecret: masterSecret,
		baseURL:      JPushAPIBaseURL,
		httpClient: &http.Client{
			Timeout: options.Timeout,
		},
		options: options,
		logger:  logx.WithContext(nil),
	}, nil
}

// SetLogger 设置日志器
func (c *Client) SetLogger(logger logx.Logger) {
	c.logger = logger
}

// buildAuthHeader 构建认证头
func (c *Client) buildAuthHeader() string {
	auth := c.appKey + ":" + c.masterSecret
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// doRequest 执行HTTP请求（带重试）
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var lastErr error

	for attempt := 0; attempt <= c.options.MaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Infof("Retrying JPush request (attempt %d/%d)", attempt, c.options.MaxRetries)
			time.Sleep(c.options.RetryInterval * time.Duration(attempt)) // 指数退避
		}

		respBody, statusCode, err := c.doSingleRequest(method, path, body)
		if err == nil {
			return respBody, statusCode, nil
		}

		lastErr = err

		// 如果不是可重试错误，直接返回
		if !IsRetryableError(err) {
			break
		}

		c.logger.Errorf("JPush request failed (attempt %d): %v", attempt+1, err)
	}

	return nil, 0, lastErr
}

// doSingleRequest 执行单次HTTP请求
func (c *Client) doSingleRequest(method, path string, body interface{}) ([]byte, int, error) {
	url := c.baseURL + path

	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.buildAuthHeader())

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, ErrNetworkTimeout
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	// 处理HTTP状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBody, resp.StatusCode, nil
	}

	// 解析错误响应
	var errResp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &errResp); err != nil {
		return nil, resp.StatusCode, NewJPushError(0, resp.StatusCode, string(respBody))
	}

	// 记录解析后的错误码和消息
	c.logger.Errorf("JPush error: code=%d, message=%s", errResp.Error.Code, errResp.Error.Message)

	// 特殊错误处理
	if resp.StatusCode == 401 {
		return nil, resp.StatusCode, ErrAuthenticationFailed
	}
	if resp.StatusCode == 429 {
		return nil, resp.StatusCode, ErrRateLimitExceeded
	}
	if errResp.Error.Code == 1011 {
		// 错误码1011: 没有满足条件的推送目标
		// 可能原因：别名未绑定、别名格式错误、设备超过255天不活跃等
		return nil, resp.StatusCode, fmt.Errorf("no valid push targets (code 1011): %s - %w", errResp.Error.Message, ErrInvalidDevice)
	}

	return nil, resp.StatusCode, NewJPushError(errResp.Error.Code, resp.StatusCode, errResp.Error.Message)
}

// buildPushPayload 构建推送请求载荷
// 根据极光推送 API v3 官方文档构建
// 文档：https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push
func (c *Client) buildPushPayload(target *PushTarget, notification *NotificationPayload, options *PushOptions) map[string]interface{} {
	payload := make(map[string]interface{})

	// 1. 设置平台 (必填)
	// 支持: "all", "android", "ios", "hmos" 或数组 ["android", "ios"]
	payload["platform"] = "all"

	// 2. 设置推送目标 (必填)
	var audience interface{}
	if target.All {
		// 广播模式：audience 为字符串 "all"
		audience = "all"
	} else {
		// 指定目标模式：audience 为 map
		audienceMap := make(map[string]interface{})
		if len(target.RegistrationIDs) > 0 {
			audienceMap["registration_id"] = target.RegistrationIDs
		}
		if len(target.Aliases) > 0 {
			audienceMap["alias"] = target.Aliases
		}
		if len(target.Tags) > 0 {
			audienceMap["tag"] = target.Tags
		}

		// 检查是否至少有一个推送目标
		if len(audienceMap) == 0 {
			c.logger.Error("buildPushPayload: audience is empty, no target specified")
		}

		audience = audienceMap
	}
	payload["audience"] = audience

	// 3. 设置通知内容 (必填)
	notificationContent := make(map[string]interface{})

	// 3.1 全平台通用 alert（使用content作为通用alert）
	notificationContent["alert"] = notification.Content

	// 3.2 Android 平台通知
	androidNotif := make(map[string]interface{})
	// Android: alert=content, title=title
	androidNotif["alert"] = notification.Content
	if notification.Title != "" {
		androidNotif["title"] = notification.Title
	}
	// extras 只在有内容时添加
	if len(notification.Extras) > 0 {
		androidNotif["extras"] = notification.Extras
	}
	notificationContent["android"] = androidNotif

	// 3.3 iOS 平台通知
	iosNotif := make(map[string]interface{})
	iosAlert := notification.Content
	if notification.Title != "" {
		iosAlert = notification.Title
	}
	iosNotif["alert"] = iosAlert
	iosNotif["sound"] = "default"
	iosNotif["badge"] = 1
	// extras 只在有内容时添加
	if len(notification.Extras) > 0 {
		iosNotif["extras"] = notification.Extras
	}
	notificationContent["ios"] = iosNotif

	// 3.4 鸿蒙平台通知 (HarmonyOS)
	hmosNotif := make(map[string]interface{})
	hmosNotif["alert"] = notification.Content
	if notification.Title != "" {
		hmosNotif["title"] = notification.Title
	}
	if len(notification.Extras) > 0 {
		hmosNotif["extras"] = notification.Extras
	}
	notificationContent["hmos"] = hmosNotif
	payload["notification"] = notificationContent

	// 4. 设置可选参数
	opts := make(map[string]interface{})

	// iOS 推送环境 (false=开发环境, true=生产环境)
	if options != nil && options.ApnsProduction {
		opts["apns_production"] = true
	} else {
		opts["apns_production"] = c.options.ApnsProduction
	}

	// 离线消息保留时长（秒），默认86400秒（1天）
	if options != nil && options.TimeToLive > 0 {
		opts["time_to_live"] = options.TimeToLive
	} else {
		opts["time_to_live"] = DefaultTimeToLive
	}

	payload["options"] = opts

	return payload
}
