package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"internalwallet/pkg/notify/templates"

	"github.com/zeromicro/go-zero/core/logx"
)

type EmailConfig struct {
	SMTPHost           string
	SMTPPort           string
	FromAddress        string
	FromPassword       string
	FromName           string
	InsecureSkipVerify bool

	// 邮件模板配置
	FreezeAccountURL string
	SupportEmail     string
	CompanyName      string

	// S3 图片资源 URL
	LogoURL           string
	SocialXURL        string
	SocialTelegramURL string
	SocialTikTokURL   string
	SocialLinkedInURL string
	SocialFacebookURL string
	SocialRedditURL   string
	AppGooglePlayURL  string
	AppAppStoreURL    string

	// 社交媒体账号跳转链接
	SocialXLink        string
	SocialTelegramLink string
	SocialTikTokLink   string
	SocialLinkedInLink string
	SocialFacebookLink string
	SocialRedditLink   string

	// 应用商店跳转链接
	AppGooglePlayLink string
	AppAppStoreLink   string
}

var defaultEmailConfig atomic.Value // EmailConfig

// emailTask 邮件发送任务
type emailTask struct {
	to      string
	subject string
	body    string
	isHTML  bool
}

// asyncEmailSender 异步邮件发送器
type asyncEmailSender struct {
	taskChan chan emailTask
	wg       sync.WaitGroup
	once     sync.Once
}

var globalEmailSender *asyncEmailSender

// initAsyncEmailSender 初始化异步邮件发送器
func initAsyncEmailSender() {
	if globalEmailSender == nil {
		globalEmailSender = &asyncEmailSender{
			taskChan: make(chan emailTask, 1000), // 缓冲队列，最多1000个待发送邮件
		}
		// 启动 5 个 worker 并发发送邮件
		for i := 0; i < 5; i++ {
			globalEmailSender.wg.Add(1)
			go globalEmailSender.worker(i)
		}
		logx.Info("异步邮件发送器已启动，worker 数量: 5")
	}
}

// worker 邮件发送工作协程
func (s *asyncEmailSender) worker(id int) {
	defer s.wg.Done()
	logx.Infof("邮件发送 worker #%d 已启动", id)

	for task := range s.taskChan {
		startTime := time.Now()
		_, err := SendEmailWithHTML(task.to, task.subject, task.body, task.isHTML)
		duration := time.Since(startTime)

		if err != nil {
			logx.Errorf("worker #%d 发送邮件失败 [to=%s, subject=%s, duration=%v]: %v",
				id, task.to, task.subject, duration, err)
		} else {
			logx.Infof("worker #%d 发送邮件成功 [to=%s, subject=%s, duration=%v]",
				id, task.to, task.subject, duration)
		}
	}

	logx.Infof("邮件发送 worker #%d 已停止", id)
}

// submitEmailTask 提交邮件发送任务到异步队列
func (s *asyncEmailSender) submitEmailTask(to, subject, body string, isHTML bool) error {
	select {
	case s.taskChan <- emailTask{
		to:      to,
		subject: subject,
		body:    body,
		isHTML:  isHTML,
	}:
		return nil
	default:
		return fmt.Errorf("邮件发送队列已满，请稍后重试")
	}
}

// SetDefaultEmailConfig 设置默认邮件配置（通常来自 yaml），环境变量会在发送时覆盖本配置。
func SetDefaultEmailConfig(cfg EmailConfig) {
	defaultEmailConfig.Store(cfg)
	// 初始化异步发送器（只初始化一次）
	var once sync.Once
	once.Do(initAsyncEmailSender)
}

// buildCommonEmailData 构建通用邮件数据（避免重复代码）
func buildCommonEmailData(cfg EmailConfig, username, uid string) templates.CommonEmailData {
	return templates.CommonEmailData{
		Username:          username,
		UID:               uid,
		CompanyName:       cfg.CompanyName,
		Year:              time.Now().Year(),
		FreezeAccountURL:  cfg.FreezeAccountURL,
		SupportEmail:      cfg.SupportEmail,
		LogoURL:           cfg.LogoURL,
		SocialXURL:        cfg.SocialXURL,
		SocialTelegramURL: cfg.SocialTelegramURL,
		SocialTikTokURL:   cfg.SocialTikTokURL,
		SocialLinkedInURL: cfg.SocialLinkedInURL,
		SocialFacebookURL: cfg.SocialFacebookURL,
		SocialRedditURL:   cfg.SocialRedditURL,
		AppGooglePlayURL:  cfg.AppGooglePlayURL,
		AppAppStoreURL:    cfg.AppAppStoreURL,
		// 社交媒体账号链接
		SocialXLink:        cfg.SocialXLink,
		SocialTelegramLink: cfg.SocialTelegramLink,
		SocialTikTokLink:   cfg.SocialTikTokLink,
		SocialLinkedInLink: cfg.SocialLinkedInLink,
		SocialFacebookLink: cfg.SocialFacebookLink,
		SocialRedditLink:   cfg.SocialRedditLink,
		// 应用商店跳转链接
		AppGooglePlayLink: cfg.AppGooglePlayLink,
		AppAppStoreLink:   cfg.AppAppStoreLink,
	}
}

func getDefaultEmailConfig() EmailConfig {
	if v := defaultEmailConfig.Load(); v != nil {
		if cfg, ok := v.(EmailConfig); ok {
			return cfg
		}
	}
	return EmailConfig{}
}

func loadEmailConfig() EmailConfig {
	cfg := getDefaultEmailConfig()

	// env 覆盖（存在才覆盖）
	if v := strings.TrimSpace(os.Getenv("EMAIL_SMTP_HOST")); v != "" {
		cfg.SMTPHost = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SMTP_PORT")); v != "" {
		cfg.SMTPPort = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_FROM_ADDRESS")); v != "" {
		cfg.FromAddress = v
	}
	if v := os.Getenv("EMAIL_FROM_PASSWORD"); strings.TrimSpace(v) != "" {
		// Gmail 应用专用密码可能包含空格，需要去掉
		cfg.FromPassword = strings.ReplaceAll(strings.TrimSpace(v), " ", "")
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_FROM_NAME")); v != "" {
		cfg.FromName = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_INSECURE_SKIP_VERIFY")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.InsecureSkipVerify = b
		}
	}

	// 新增配置字段的环境变量覆盖
	if v := strings.TrimSpace(os.Getenv("EMAIL_FREEZE_ACCOUNT_URL")); v != "" {
		cfg.FreezeAccountURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SUPPORT_EMAIL")); v != "" {
		cfg.SupportEmail = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_COMPANY_NAME")); v != "" {
		cfg.CompanyName = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_LOGO_URL")); v != "" {
		cfg.LogoURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_X_URL")); v != "" {
		cfg.SocialXURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_TELEGRAM_URL")); v != "" {
		cfg.SocialTelegramURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_TIKTOK_URL")); v != "" {
		cfg.SocialTikTokURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_LINKEDIN_URL")); v != "" {
		cfg.SocialLinkedInURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_FACEBOOK_URL")); v != "" {
		cfg.SocialFacebookURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_REDDIT_URL")); v != "" {
		cfg.SocialRedditURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_APP_GOOGLEPLAY_URL")); v != "" {
		cfg.AppGooglePlayURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_APP_APPSTORE_URL")); v != "" {
		cfg.AppAppStoreURL = v
	}

	// 社交媒体账号跳转链接
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_X_LINK")); v != "" {
		cfg.SocialXLink = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_TELEGRAM_LINK")); v != "" {
		cfg.SocialTelegramLink = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_TIKTOK_LINK")); v != "" {
		cfg.SocialTikTokLink = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_LINKEDIN_LINK")); v != "" {
		cfg.SocialLinkedInLink = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_FACEBOOK_LINK")); v != "" {
		cfg.SocialFacebookLink = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_SOCIAL_REDDIT_LINK")); v != "" {
		cfg.SocialRedditLink = v
	}

	// 应用商店跳转链接
	if v := strings.TrimSpace(os.Getenv("EMAIL_APP_GOOGLEPLAY_LINK")); v != "" {
		cfg.AppGooglePlayLink = v
	}
	if v := strings.TrimSpace(os.Getenv("EMAIL_APP_APPSTORE_LINK")); v != "" {
		cfg.AppAppStoreLink = v
	}

	// defaults
	if cfg.SMTPPort == "" {
		cfg.SMTPPort = "587"
	}
	if cfg.FromName == "" {
		cfg.FromName = "Zink Wallet"
	}
	if cfg.CompanyName == "" {
		cfg.CompanyName = "Zink"
	}

	return cfg
}

func (c EmailConfig) validate() error {
	if c.SMTPHost == "" {
		return errors.New("EMAIL_SMTP_HOST is required")
	}
	if c.SMTPPort == "" {
		return errors.New("EMAIL_SMTP_PORT is required")
	}
	if c.FromAddress == "" {
		return errors.New("EMAIL_FROM_ADDRESS is required")
	}
	// FromPassword 可为空（部分 SMTP 支持匿名/基于 IP 白名单），但大多数环境需要配置。
	return nil
}

// EmailResult 邮件发送结果
type EmailResult struct {
	Success bool
	Message string
}

// SendEmail 发送邮件
func SendEmail(to, subject, body string) (*EmailResult, error) {
	return SendEmailWithHTML(to, subject, body, false)
}

// SendEmailWithHTML 发送邮件
func SendEmailWithHTML(to, subject, body string, isHTML bool) (*EmailResult, error) {
	cfg := loadEmailConfig()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// 构建邮件头
	contentType := "text/plain"
	if isHTML {
		contentType = "text/html"
	}

	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromAddress)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = fmt.Sprintf("%s; charset=UTF-8", contentType)

	// 构建邮件内容
	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	// 连接SMTP服务器
	var auth smtp.Auth
	password := strings.TrimSpace(cfg.FromPassword)
	// Gmail 应用专用密码可能包含空格，需要去掉
	password = strings.ReplaceAll(password, " ", "")
	if password != "" {
		auth = smtp.PlainAuth("", cfg.FromAddress, password, cfg.SMTPHost)
	}

	// 使用TLS连接
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.SMTPHost,
	}

	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)

	// 发送邮件
	err := sendMailWithTLS(addr, auth, cfg.FromAddress, []string{to}, []byte(message.String()), tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("发送邮件失败: %w", err)
	}

	return &EmailResult{
		Success: true,
		Message: "邮件发送成功",
	}, nil
}

// SendVerificationEmail 发送验证码邮件（使用 Zink 品牌模板）- 同步发送
func SendVerificationEmail(to, code string) (*EmailResult, error) {
	cfg := loadEmailConfig()

	// 提前验证配置，提供更清晰的错误信息
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return nil, fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 尝试使用新的模板系统
	tmpl := templates.GetVerificationEmailTemplate()
	if tmpl != nil {
		// 准备模板数据（验证码邮件使用邮箱作为用户名）
		data := templates.VerificationEmailData{
			CommonEmailData: templates.CommonEmailData{
				Username:           to, // 未注册用户使用邮箱作为用户名
				CompanyName:        cfg.CompanyName,
				Year:               time.Now().Year(),
				FreezeAccountURL:   cfg.FreezeAccountURL,
				SupportEmail:       cfg.SupportEmail,
				LogoURL:            cfg.LogoURL,
				SocialXURL:         cfg.SocialXURL,
				SocialTelegramURL:  cfg.SocialTelegramURL,
				SocialTikTokURL:    cfg.SocialTikTokURL,
				SocialLinkedInURL:  cfg.SocialLinkedInURL,
				SocialFacebookURL:  cfg.SocialFacebookURL,
				SocialRedditURL:    cfg.SocialRedditURL,
				AppGooglePlayURL:   cfg.AppGooglePlayURL,
				AppAppStoreURL:     cfg.AppAppStoreURL,
				SocialXLink:        cfg.SocialXLink,
				SocialTelegramLink: cfg.SocialTelegramLink,
				SocialTikTokLink:   cfg.SocialTikTokLink,
				SocialLinkedInLink: cfg.SocialLinkedInLink,
				SocialFacebookLink: cfg.SocialFacebookLink,
				SocialRedditLink:   cfg.SocialRedditLink,
				AppGooglePlayLink:  cfg.AppGooglePlayLink,
				AppAppStoreLink:    cfg.AppAppStoreLink,
			},
			Code:          code,
			ExpiryMinutes: 5, // 5分钟有效期
		}

		// 渲染模板
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			logx.Errorf("failed to render email template: %v, falling back to simple template", err)
		} else {
			// 模板渲染成功，使用新模板
			subject := fmt.Sprintf("验证码 - %s", cfg.CompanyName)
			return SendEmailWithHTML(to, subject, buf.String(), true)
		}
	}

	// 降级方案：使用简单模板
	return sendFallbackVerificationEmail(to, code)
}

// SendVerificationEmailAsync 发送验证码邮件（异步）- 立即返回，后台发送
func SendVerificationEmailAsync(to, code string) (*EmailResult, error) {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 提前验证配置，提供更清晰的错误信息
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return nil, fmt.Errorf("邮件服务配置错误: %w", err)
	}

	var subject, body string
	var isHTML bool

	// 尝试使用新的模板系统
	tmpl := templates.GetVerificationEmailTemplate()
	if tmpl != nil {
		// 准备模板数据（验证码邮件使用邮箱作为用户名）
		data := templates.VerificationEmailData{
			CommonEmailData: templates.CommonEmailData{
				Username:           to, // 未注册用户使用邮箱作为用户名
				CompanyName:        cfg.CompanyName,
				Year:               time.Now().Year(),
				FreezeAccountURL:   cfg.FreezeAccountURL,
				SupportEmail:       cfg.SupportEmail,
				LogoURL:            cfg.LogoURL,
				SocialXURL:         cfg.SocialXURL,
				SocialTelegramURL:  cfg.SocialTelegramURL,
				SocialTikTokURL:    cfg.SocialTikTokURL,
				SocialLinkedInURL:  cfg.SocialLinkedInURL,
				SocialFacebookURL:  cfg.SocialFacebookURL,
				SocialRedditURL:    cfg.SocialRedditURL,
				AppGooglePlayURL:   cfg.AppGooglePlayURL,
				AppAppStoreURL:     cfg.AppAppStoreURL,
				SocialXLink:        cfg.SocialXLink,
				SocialTelegramLink: cfg.SocialTelegramLink,
				SocialTikTokLink:   cfg.SocialTikTokLink,
				SocialLinkedInLink: cfg.SocialLinkedInLink,
				SocialFacebookLink: cfg.SocialFacebookLink,
				SocialRedditLink:   cfg.SocialRedditLink,
				AppGooglePlayLink:  cfg.AppGooglePlayLink,
				AppAppStoreLink:    cfg.AppAppStoreLink,
			},
			Code:          code,
			ExpiryMinutes: 5, // 5分钟有效期
		}

		// 渲染模板
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			logx.Errorf("failed to render email template: %v, falling back to simple template", err)
			// 使用降级方案
			subject, body, isHTML = getFallbackVerificationEmailContent(code)
		} else {
			// 模板渲染成功
			subject = fmt.Sprintf("验证码 - %s", cfg.CompanyName)
			body = buf.String()
			isHTML = true
		}
	} else {
		// 使用降级方案
		subject, body, isHTML = getFallbackVerificationEmailContent(code)
	}

	// 提交到异步队列
	if err := globalEmailSender.submitEmailTask(to, subject, body, isHTML); err != nil {
		return nil, err
	}

	logx.Infof("验证码邮件已提交到异步队列 [to=%s]", to)
	return &EmailResult{
		Success: true,
		Message: "验证码邮件已提交发送",
	}, nil
}

// getFallbackVerificationEmailContent 获取降级方案的邮件内容
func getFallbackVerificationEmailContent(code string) (subject, body string, isHTML bool) {
	subject = "验证码 - Zink Wallet"
	body = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #f9f9f9; padding: 30px; border-radius: 10px;">
        <h2 style="color: #333;">验证码</h2>
        <p style="color: #666;">您的验证码是：</p>
        <div style="background: #007bff; color: white; font-size: 32px; padding: 15px 30px; display: inline-block; border-radius: 5px; letter-spacing: 5px;">
            %s
        </div>
        <p style="color: #666; margin-top: 20px;">验证码5分钟内有效，请勿泄露给他人。</p>
        <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
        <p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
    </div>
</body>
</html>
`, code)
	isHTML = true
	return
}

// sendFallbackVerificationEmail 降级方案：发送简单验证码邮件
func sendFallbackVerificationEmail(to, code string) (*EmailResult, error) {
	subject := "验证码 - Zink Wallet"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #f9f9f9; padding: 30px; border-radius: 10px;">
        <h2 style="color: #333;">验证码</h2>
        <p style="color: #666;">您的验证码是：</p>
        <div style="background: #007bff; color: white; font-size: 32px; padding: 15px 30px; display: inline-block; border-radius: 5px; letter-spacing: 5px;">
            %s
        </div>
        <p style="color: #666; margin-top: 20px;">验证码5分钟内有效，请勿泄露给他人。</p>
        <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
        <p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
    </div>
</body>
</html>
`, code)

	return SendEmailWithHTML(to, subject, body, true)
}

// SendNotificationEmail 发送通知邮件
func SendNotificationEmail(to, title, content string) (*EmailResult, error) {
	subject := fmt.Sprintf("%s - Zink Wallet", title)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: #f9f9f9; padding: 30px; border-radius: 10px;">
        <h2 style="color: #333;">%s</h2>
        <div style="color: #666; line-height: 1.8;">
            %s
        </div>
        <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
        <p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
    </div>
</body>
</html>
`, title, content)

	return SendEmailWithHTML(to, subject, body, true)
}

// SendAdminAccountCreatedEmail 发送管理员账户创建通知邮件（不包含 2FA secret）
func SendAdminAccountCreatedEmail(to, systemName, username, tempPassword string, requirePasswordChange bool, requireTwoFA bool) (*EmailResult, error) {
	if strings.TrimSpace(systemName) == "" {
		systemName = "Zink Wallet"
	}
	requireChangeText := "否"
	if requirePasswordChange {
		requireChangeText = "是（首次登录会被要求修改密码）"
	}
	requireTwoFAText := "否"
	if requireTwoFA {
		requireTwoFAText = "是（登录后需完成绑定）"
	}
	passwordLine := "<p><b>初始密码</b>：请联系管理员获取</p>"
	if strings.TrimSpace(tempPassword) != "" {
		passwordLine = fmt.Sprintf(`<p><b>临时密码</b>：<span style="font-size: 18px; letter-spacing: 1px;">%s</span></p>`, tempPassword)
	}
	content := fmt.Sprintf(`
<p>您好，您的后台管理员账户已创建。</p>
<p><b>系统</b>：%s</p>
<p><b>账号（邮箱）</b>：%s</p>
%s
<p><b>首次登录需改密</b>：%s</p>
<p><b>首次登录需绑定 2FA</b>：%s</p>
<p style="color:#666;">建议您尽快登录并完成改密/2FA 绑定。请勿将密码泄露给他人。</p>
`, systemName, username, passwordLine, requireChangeText, requireTwoFAText)
	return SendNotificationEmail(to, "管理员账户已创建", content)
}

// SendAdminPasswordResetEmail 发送管理员密码重置通知邮件
func SendAdminPasswordResetEmail(to, systemName, tempPassword string, requirePasswordChange bool) (*EmailResult, error) {
	if strings.TrimSpace(systemName) == "" {
		systemName = "Zink Wallet"
	}
	requireChangeText := "否"
	if requirePasswordChange {
		requireChangeText = "是（首次登录会被要求修改密码）"
	}
	content := fmt.Sprintf(`
<p>您好，您的后台管理员密码已被重置。</p>
<p><b>系统</b>：%s</p>
<p><b>临时密码</b>：<span style="font-size: 18px; letter-spacing: 1px;">%s</span></p>
<p><b>首次登录需改密</b>：%s</p>
<p style="color:#666;">请尽快登录并修改密码。如非本人操作，请立即联系管理员。</p>
`, systemName, tempPassword, requireChangeText)
	return SendNotificationEmail(to, "管理员密码已重置", content)
}

// SendTradingPasswordChangeEmail 发送交易密码修改成功通知邮件
func SendTradingPasswordChangeEmail(to, username, uid, changeTime, ipAddress, location string) (*EmailResult, error) {
	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return nil, fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.TradingPasswordChangeSuccessData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		ChangeTime:      changeTime,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.RenderTradingPasswordChangeSuccessEmail(data)
	if err != nil {
		logx.Errorf("failed to render trading password change email template: %v", err)
		return nil, fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 发送邮件
	subject := fmt.Sprintf("交易密码修改成功 - %s", cfg.CompanyName)
	return SendEmailWithHTML(to, subject, body, true)
}

// SendTradingPasswordChangeEmailAsync 异步发送交易密码修改成功通知邮件
func SendTradingPasswordChangeEmailAsync(to, username, uid, changeTime, ipAddress, location string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.TradingPasswordChangeSuccessData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		ChangeTime:      changeTime,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.RenderTradingPasswordChangeSuccessEmail(data)
	if err != nil {
		logx.Errorf("failed to render trading password change email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("交易密码修改成功 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("交易密码修改通知邮件已提交到异步队列 [to=%s]", to)
	return nil
}

// Send2FAEnabledEmailAsync 异步发送2FA启用通知邮件
func Send2FAEnabledEmailAsync(to, username, uid, changeTime, ipAddress, location string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.TwoFactorAuthEnabledData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		ChangeTime:      changeTime,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.Render2FAEnabledEmail(data)
	if err != nil {
		logx.Errorf("failed to render 2FA enabled email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("重要设置变更通知 - 2FA已启用 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("2FA启用通知邮件已提交到异步队列 [to=%s]", to)
	return nil
}

// Send2FADisabledEmailAsync 异步发送2FA禁用通知邮件
func Send2FADisabledEmailAsync(to, username, uid, disabledMethod, changeTime, ipAddress, location string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.TwoFactorAuthDisabledData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		DisabledMethod:  disabledMethod,
		ChangeTime:      changeTime,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.Render2FADisabledEmail(data)
	if err != nil {
		logx.Errorf("failed to render 2FA disabled email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("重要设置变更通知 - 2FA已关闭 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("2FA关闭通知邮件已提交到异步队列 [to=%s]", to)
	return nil
}

// SendPhoneChangeEmailAsync 异步发送手机号修改成功通知邮件
func SendPhoneChangeEmailAsync(to, username, uid, oldPhone, newPhone, countryCode, changeTime, ipAddress, location string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.PhoneChangeSuccessData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		OldPhone:        oldPhone,
		NewPhone:        newPhone,
		CountryCode:     countryCode,
		ChangeTime:      changeTime,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.RenderPhoneChangeSuccessEmail(data)
	if err != nil {
		logx.Errorf("failed to render phone change email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("手机号修改成功 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("手机号修改通知邮件已提交到异步队列 [to=%s]", to)
	return nil
}

// SendEmailChangeEmailAsync 异步发送邮箱修改成功通知邮件
func SendEmailChangeEmailAsync(to, username, uid, oldEmail, newEmail, changeTime, ipAddress, location string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.EmailChangeSuccessData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		OldEmail:        oldEmail,
		NewEmail:        newEmail,
		ChangeTime:      changeTime,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.RenderEmailChangeSuccessEmail(data)
	if err != nil {
		logx.Errorf("failed to render email change email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("邮箱修改成功 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("邮箱修改通知邮件已提交到异步队列 [to=%s]", to)
	return nil
}

// SendDepositSuccessEmailAsync 异步发送充值成功通知邮件
func SendDepositSuccessEmailAsync(to, username, uid, assetCode, chainCode, amount, depositTime, txHash, fromAddress, depositAddress, confirmations, requiredConfirmations string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据（使用原有字段名）
	data := templates.DepositSuccessData{
		CommonEmailData:       buildCommonEmailData(cfg, username, uid),
		Currency:              assetCode,
		Amount:                amount,
		Network:               chainCode,
		DepositAddress:        MaskAddress(depositAddress), // 对地址进行隐私处理
		SourceAddress:         MaskAddress(fromAddress),    // 对地址进行隐私处理
		TxHash:                txHash,
		ArrivalTime:           depositTime,
		Confirmations:         confirmations,
		RequiredConfirmations: requiredConfirmations,
	}

	// 渲染模板
	body, err := templates.RenderDepositSuccessEmail(data)
	if err != nil {
		logx.Errorf("failed to render deposit success email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("充值到账通知 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("充值成功通知邮件已提交到异步队列 [to=%s, asset=%s, amount=%s]", to, assetCode, amount)
	return nil
}

// SendDepositFailedEmailAsync 异步发送充值失败通知邮件
func SendDepositFailedEmailAsync(to, username, uid, assetCode, chainCode, amount, failedTime, txHash, depositAddress, sourceAddress string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据（使用原有字段名）
	data := templates.DepositFailedData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		Currency:        assetCode,
		Amount:          amount,
		Network:         chainCode,
		DepositAddress:  MaskAddress(depositAddress), // 对地址进行隐私处理
		SourceAddress:   MaskAddress(sourceAddress),  // 对地址进行隐私处理
		TxHash:          txHash,
		DetectedTime:    failedTime,
	}

	// 渲染模板
	body, err := templates.RenderDepositFailedEmail(data)
	if err != nil {
		logx.Errorf("failed to render deposit failed email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("充值失败通知 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("充值失败通知邮件已提交到异步队列 [to=%s, asset=%s, amount=%s]", to, assetCode, amount)
	return nil
}

// SendSecurityAlertAbnormalLoginEmailAsync 异步发送异常登录安全警报邮件
func SendSecurityAlertAbnormalLoginEmailAsync(to, username, uid, loginTime, device, ipAddress, location string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据
	data := templates.SecurityAlertAbnormalLoginData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		LoginTime:       loginTime,
		Device:          device,
		IPAddress:       ipAddress,
		Location:        location,
	}

	// 渲染模板
	body, err := templates.RenderSecurityAlertAbnormalLoginEmail(data)
	if err != nil {
		logx.Errorf("failed to render security alert abnormal login email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("安全警报 - 异常登录 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("异常登录安全警报邮件已提交到异步队列 [to=%s, ip=%s, device=%s]", to, ipAddress, device)
	return nil
}

// SendWithdrawalSuccessEmailAsync 异步发送提现成功通知邮件
func SendWithdrawalSuccessEmailAsync(to, username, uid, assetCode, amount, network, withdrawalTime, txHash, toAddress string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据（使用原有字段名）
	data := templates.WithdrawalSubmittedData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		Currency:        assetCode,
		Amount:          amount,
		Address:         MaskAddress(toAddress), // 对地址进行隐私处理
		TxHash:          txHash,
		Network:         network,
		ArrivalTime:     withdrawalTime,
	}

	// 渲染模板
	body, err := templates.RenderWithdrawalSubmittedEmail(data)
	if err != nil {
		logx.Errorf("failed to render withdrawal success email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("提币成功通知 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("提现成功通知邮件已提交到异步队列 [to=%s, asset=%s, amount=%s]", to, assetCode, amount)
	return nil
}

// SendInternalTransferSuccessEmailAsync 异步发送内部转账成功通知邮件
func SendInternalTransferSuccessEmailAsync(to, username, uid, assetCode, amount, transferTime, fromUser, toUser, receiverNote, orderID, orderNote string) error {
	// 确保异步发送器已初始化
	if globalEmailSender == nil {
		initAsyncEmailSender()
	}

	cfg := loadEmailConfig()

	// 验证配置
	if err := cfg.validate(); err != nil {
		logx.Errorf("email config validation failed: %v", err)
		return fmt.Errorf("邮件服务配置错误: %w", err)
	}

	// 准备模板数据（使用原有字段名）
	data := templates.InternalTransferSuccessData{
		CommonEmailData: buildCommonEmailData(cfg, username, uid),
		FromAccount:     fromUser,
		ToAccount:       toUser,
		ReceiverNote:    receiverNote, // 昵称/备注
		Amount:          amount,
		Currency:        assetCode,
		TransferTime:    transferTime,
		OrderID:         orderID,
		OrderNote:       orderNote,
	}

	// 渲染模板
	body, err := templates.RenderInternalTransferSuccessEmail(data)
	if err != nil {
		logx.Errorf("failed to render internal transfer success email template: %v", err)
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 提交到异步队列
	subject := fmt.Sprintf("内部转账通知 - %s", cfg.CompanyName)
	if err := globalEmailSender.submitEmailTask(to, subject, body, true); err != nil {
		return err
	}

	logx.Infof("内部转账成功通知邮件已提交到异步队列 [to=%s, asset=%s, amount=%s, orderID=%s]", to, assetCode, amount, orderID)
	return nil
}

// sendMailWithTLS 使用TLS发送邮件（带超时控制）
func sendMailWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, tlsConfig *tls.Config) error {
	// 设置总超时时间为 25 秒（略小于 API Gateway 的 30 秒超时，留出缓冲）
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// 使用 channel 来在 goroutine 中执行邮件发送，并支持超时取消
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		err := sendMailWithTLSSync(addr, auth, from, to, msg, tlsConfig)
		resultCh <- result{err: err}
	}()

	select {
	case res := <-resultCh:
		return res.err
	case <-ctx.Done():
		return fmt.Errorf("发送邮件超时（超过 25 秒），可能是 SMTP 服务器响应慢或网络问题")
	}
}

// sendMailWithTLSSync 同步执行 SMTP 发送（不带超时）
func sendMailWithTLSSync(addr string, auth smtp.Auth, from string, to []string, msg []byte, tlsConfig *tls.Config) error {
	// 提取主机名（去掉端口部分）
	host := addr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		host = addr[:idx]
	}

	// 使用带超时的 Dialer 创建连接（连接超时 10 秒）
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	// 建立 TCP 连接
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("连接SMTP服务器失败: %w", err)
	}
	defer conn.Close()

	// 创建 SMTP 客户端（第二个参数是主机名，用于 EHLO/HELO 命令）
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer c.Close()

	// 发送EHLO
	if err = c.Hello("localhost"); err != nil {
		return fmt.Errorf("SMTP握手失败: %w", err)
	}

	// 启动TLS
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("启动TLS失败: %w", err)
		}
	}

	// 认证
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(auth); err != nil {
				return fmt.Errorf("SMTP认证失败，请检查邮箱地址和密码是否正确: %w", err)
			}
		} else {
			return fmt.Errorf("SMTP服务器不支持AUTH认证")
		}
	} else {
		// Gmail 需要认证
		if strings.Contains(addr, "gmail.com") {
			return fmt.Errorf("Gmail需要认证，但未提供密码")
		}
	}

	// 设置发件人
	if err = c.Mail(from); err != nil {
		return err
	}

	// 设置收件人
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}

	// 发送邮件内容
	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return c.Quit()
}
