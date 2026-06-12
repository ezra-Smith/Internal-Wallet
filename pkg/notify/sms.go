package notify

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"internalwallet/pkg/secrets"
)

// SMSConfig 短信配置
type SMSConfig struct {
	// 国际短信配置
	InternationalSMSURL       string
	InternationalSMSAppKey    string
	InternationalSMSAppSecret string
	InternationalSMSAppCode   string

	// 国内短信配置（蜂鸟）
	DomesticSMSURL     string
	DomesticSMSMercId  string
	DomesticSMSSecret  string
	DomesticTemplateID string

	// 短信签名
	SMSSign string
}

var defaultSMSConfig atomic.Value // SMSConfig

// SetDefaultSMSConfig 设置默认短信配置（通常来自 yaml），环境变量会在发送时覆盖本配置。
func SetDefaultSMSConfig(cfg SMSConfig) {
	defaultSMSConfig.Store(cfg)
}

// InitFromSecretConfig initializes SMS config from SecretConfig (for AWS environments).
// This should be called during service startup after loading secrets.
func InitFromSecretConfig(sc *secrets.SecretConfig) {
	if sc == nil {
		return
	}

	cfg := SMSConfig{
		InternationalSMSURL:       sc.SMS.InternationalURL,
		InternationalSMSAppKey:    sc.SMS.InternationalAppKey,
		InternationalSMSAppSecret: sc.SMS.InternationalAppSecret,
		InternationalSMSAppCode:   sc.SMS.InternationalAppCode,
		DomesticSMSURL:            sc.SMS.DomesticURL,
		DomesticSMSMercId:         sc.SMS.DomesticMercId,
		DomesticSMSSecret:         sc.SMS.DomesticSecret,
		DomesticTemplateID:        sc.SMS.DomesticTemplateId,
		SMSSign:                   sc.SMS.Sign,
	}

	// Only set if at least one field is non-empty
	if cfg.InternationalSMSAppKey != "" || cfg.DomesticSMSSecret != "" {
		SetDefaultSMSConfig(cfg)
		logx.Info("✓ SMS config initialized from SecretConfig")
	}
}

func getDefaultSMSConfig() SMSConfig {
	if v := defaultSMSConfig.Load(); v != nil {
		if cfg, ok := v.(SMSConfig); ok {
			return cfg
		}
	}
	return SMSConfig{}
}

func loadSMSConfig() SMSConfig {
	cfg := getDefaultSMSConfig()

	// env 覆盖（存在才覆盖）
	// 国际短信配置
	if v := strings.TrimSpace(os.Getenv("SMS_INTERNATIONAL_URL")); v != "" {
		cfg.InternationalSMSURL = v
	}
	if v := strings.TrimSpace(os.Getenv("SMS_INTERNATIONAL_APP_KEY")); v != "" {
		cfg.InternationalSMSAppKey = v
	}
	if v := strings.TrimSpace(os.Getenv("SMS_INTERNATIONAL_APP_SECRET")); v != "" {
		cfg.InternationalSMSAppSecret = v
	}
	if v := strings.TrimSpace(os.Getenv("SMS_INTERNATIONAL_APP_CODE")); v != "" {
		cfg.InternationalSMSAppCode = v
	}

	// 国内短信配置（蜂鸟）
	if v := strings.TrimSpace(os.Getenv("SMS_DOMESTIC_URL")); v != "" {
		cfg.DomesticSMSURL = v
	}
	if v := strings.TrimSpace(os.Getenv("SMS_DOMESTIC_MERC_ID")); v != "" {
		cfg.DomesticSMSMercId = v
	}
	if v := strings.TrimSpace(os.Getenv("SMS_DOMESTIC_SECRET")); v != "" {
		cfg.DomesticSMSSecret = v
	}
	if v := strings.TrimSpace(os.Getenv("SMS_DOMESTIC_TEMPLATE_ID")); v != "" {
		cfg.DomesticTemplateID = v
	}

	// 短信签名
	if v := strings.TrimSpace(os.Getenv("SMS_SIGN")); v != "" {
		cfg.SMSSign = v
	}

	// defaults
	if cfg.InternationalSMSURL == "" {
		cfg.InternationalSMSURL = "http://47.242.85.7:9090/sms/batch/v2"
	}
	if cfg.InternationalSMSAppKey == "" {
		cfg.InternationalSMSAppKey = "Damtta"
	}
	if cfg.InternationalSMSAppSecret == "" {
		cfg.InternationalSMSAppSecret = "xqBmnh"
	}
	if cfg.InternationalSMSAppCode == "" {
		cfg.InternationalSMSAppCode = "1000"
	}
	if cfg.DomesticSMSURL == "" {
		cfg.DomesticSMSURL = "https://pla26.1f8rm.cc/api/sms/sendSms"
	}
	if cfg.DomesticSMSMercId == "" {
		cfg.DomesticSMSMercId = "10240"
	}
	if cfg.DomesticSMSSecret == "" {
		cfg.DomesticSMSSecret = "dafc04ff5a07415d048e3f53e0ef3127"
	}
	if cfg.DomesticTemplateID == "" {
		//cfg.DomesticTemplateID = "688199b81486051d69d3ae27"
		cfg.DomesticTemplateID = "64005daa4d0d6c39420dc226"
	}
	if cfg.SMSSign == "" {
		cfg.SMSSign = "【Zink】"
	}

	return cfg
}

func (c SMSConfig) validate() error {
	// 国际短信配置验证
	if c.InternationalSMSURL == "" {
		return errors.New("SMS_INTERNATIONAL_URL is required")
	}
	if c.InternationalSMSAppKey == "" {
		return errors.New("SMS_INTERNATIONAL_APP_KEY is required")
	}
	if c.InternationalSMSAppSecret == "" {
		return errors.New("SMS_INTERNATIONAL_APP_SECRET is required")
	}
	if c.InternationalSMSAppCode == "" {
		return errors.New("SMS_INTERNATIONAL_APP_CODE is required")
	}

	// 国内短信配置验证（蜂鸟）
	if c.DomesticSMSURL == "" {
		return errors.New("SMS_DOMESTIC_URL is required")
	}
	if c.DomesticSMSMercId == "" {
		return errors.New("SMS_DOMESTIC_MERC_ID is required")
	}
	if c.DomesticSMSSecret == "" {
		return errors.New("SMS_DOMESTIC_SECRET is required")
	}

	return nil
}

// 国际短信响应
type InternationalSMSResponse struct {
	Code   string `json:"code"`
	Desc   string `json:"desc"`
	UID    string `json:"uid"`
	Result []struct {
		Status string `json:"status"`
		Phone  string `json:"phone"`
		Desc   string `json:"desc"`
	} `json:"result"`
}

// 国内短信响应（蜂鸟）
type DomesticSMSResponse struct {
	Code int `json:"code"`
	Msg  struct {
		Status string `json:"status"`
		TaskId string `json:"taskId"`
	} `json:"msg"`
	Err string `json:"err"`
}

// SMSResult 短信发送结果
type SMSResult struct {
	Success bool
	Message string
	TaskId  string
}

// SendSMS 发送短信 - 自动判断国际/国内
func SendSMS(phone, content string, code string) (*SMSResult, error) {
	if IsChinaMainlandPhone(phone) {
		return SendDomesticSMS(phone, content, code)
	}
	return SendInternationalSMS(phone, content)
}

// SendVerificationCode 发送验证码短信
func SendVerificationCode(phone, code string) (*SMSResult, error) {
	cfg := loadSMSConfig()
	logx.Infof("send verification code to phone: %s, code: %s", phone, code)
	content := fmt.Sprintf("%s您的验证码是：%s，5分钟内有效，请勿泄露给他人。", cfg.SMSSign, code)
	return SendSMS(phone, content, code)
}

// IsChinaMainlandPhone 判断是否为中国大陆手机号
func IsChinaMainlandPhone(phone string) bool {
	// 去除前缀
	phone = strings.TrimPrefix(phone, "+")

	// 检查是否以86开头
	if strings.HasPrefix(phone, "86") {
		phone = strings.TrimPrefix(phone, "86")
	} else {
		return false
	}

	// 中国大陆手机号: 11位数字，第一位是1，第二位是3-9
	if len(phone) != 11 {
		return false
	}
	if !strings.HasPrefix(phone, "1") {
		return false
	}

	// 验证第二位是否是有效的运营商号段 (3-9)
	secondDigit := phone[1]
	if secondDigit >= '3' && secondDigit <= '9' {
		return true
	}
	return false
}

// SendInternationalSMS 发送国际短信
func SendInternationalSMS(phone, content string) (*SMSResult, error) {
	cfg := loadSMSConfig()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// 格式化手机号：去掉 + 前缀，API要求格式为 "国家代码+手机号"（如 841234567890）
	phone = strings.TrimPrefix(phone, "+")

	// 构建请求URL
	params := url.Values{}
	params.Set("appkey", cfg.InternationalSMSAppKey)
	params.Set("appcode", cfg.InternationalSMSAppCode)
	params.Set("appsecret", cfg.InternationalSMSAppSecret)
	params.Set("phone", phone)
	params.Set("msg", content)

	reqURL := fmt.Sprintf("%s?%s", cfg.InternationalSMSURL, params.Encode())
	logx.Infof("sending international SMS to %s", phone)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("发送国际短信失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	logx.Infof("international SMS response: %s", string(body))

	var result InternationalSMSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != "00000" {
		return &SMSResult{
			Success: false,
			Message: result.Desc,
			TaskId:  result.UID,
		}, nil
	}

	return &SMSResult{
		Success: true,
		Message: result.Desc,
		TaskId:  result.UID,
	}, nil
}

// SendDomesticSMS 发送国内短信（蜂鸟）
func SendDomesticSMS(phone, content string, code string) (*SMSResult, error) {
	cfg := loadSMSConfig()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.DomesticSMSURL == "" {
		return &SMSResult{
			Success: false,
			Message: "国内短信接口未配置",
		}, nil
	}

	// 格式化手机号：蜂鸟支持 +86xxx 或 xxx 格式
	// 去掉 +86 前缀，只保留11位手机号
	phone = strings.TrimPrefix(phone, "+")
	phone = strings.TrimPrefix(phone, "86")

	// 构建签名: md5(mercId=${}&templateId=${}&smsCode=${}&mobile=${}&secret=${})
	signStr := fmt.Sprintf("mercId=%s&templateId=%s&smsCode=%s&mobile=%s&secret=%s",
		cfg.DomesticSMSMercId, cfg.DomesticTemplateID, code, phone, cfg.DomesticSMSSecret)
	sign := md5Hash(signStr)

	// 构建请求体
	reqBody := map[string]string{
		"mercId": cfg.DomesticSMSMercId,
		//"content":  content,
		"mobile":     phone,
		"signType":   "md5",
		"sign":       sign,
		"templateId": cfg.DomesticTemplateID,
		"smsCode":    code,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	logx.Infof("sending domestic SMS to %s, url: %s, body: %s", phone, cfg.DomesticSMSURL, string(jsonBody))

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(cfg.DomesticSMSURL, "application/json", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("发送国内短信失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	logx.Infof("domestic SMS response (status=%d): %s", resp.StatusCode, string(body))

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return &SMSResult{
			Success: false,
			Message: fmt.Sprintf("HTTP错误: %d, body: %s", resp.StatusCode, string(body)),
		}, nil
	}

	// 检查响应体是否为空
	if len(body) == 0 {
		return &SMSResult{
			Success: false,
			Message: "服务器返回空响应",
		}, nil
	}

	var result DomesticSMSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败 (raw: %s): %w", string(body), err)
	}

	if result.Code != 200 {
		return &SMSResult{
			Success: false,
			Message: result.Err,
		}, nil
	}

	return &SMSResult{
		Success: result.Msg.Status == "success",
		Message: result.Msg.Status,
		TaskId:  result.Msg.TaskId,
	}, nil
}

// md5Hash 计算MD5哈希
func md5Hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
