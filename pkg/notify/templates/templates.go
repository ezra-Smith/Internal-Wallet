package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

//go:embed *.html
var templateFS embed.FS

var (
	loadOnce  sync.Once
	templates map[string]*template.Template
)

// TemplateType 邮件模板类型
type TemplateType string

const (
	TemplateVerificationEmail          TemplateType = "verification_email"
	TemplateWithdrawalSubmitted        TemplateType = "withdrawal_submitted"
	TemplateDepositSuccess             TemplateType = "deposit_success"
	TemplateDepositFailed              TemplateType = "deposit_failed"
	TemplateInternalTransferSuccess    TemplateType = "internal_transfer_success"
	TemplateSecurityAlertAbnormalLogin TemplateType = "security_alert_abnormal_login"
	Template2FAEnabled                 TemplateType = "2fa_enabled"
	Template2FADisabled                TemplateType = "2fa_disabled"
	TemplateEmailChangeSuccess         TemplateType = "email_change_success"
	TemplateTradingPasswordChange      TemplateType = "trading_password_change_success"
	TemplatePhoneChangeSuccess         TemplateType = "phone_change_success"
)

// CommonEmailData 通用邮件模板数据（所有模板共享的字段）
type CommonEmailData struct {
	// 基本信息
	Username    string
	UID         string
	CompanyName string
	Year        int

	// 功能配置
	FreezeAccountURL string
	SupportEmail     string

	// S3 图片 URL
	LogoURL           string
	SocialXURL        string
	SocialTelegramURL string
	SocialTikTokURL   string
	SocialLinkedInURL string
	SocialFacebookURL string
	SocialRedditURL   string
	AppGooglePlayURL  string
	AppAppStoreURL    string

	// 社交媒体账号链接
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

// VerificationEmailData 验证码邮件模板数据
type VerificationEmailData struct {
	CommonEmailData
	Code          string
	ExpiryMinutes int
}

// WithdrawalSubmittedData 提币申请提交邮件模板数据
type WithdrawalSubmittedData struct {
	CommonEmailData
	Currency    string
	Amount      string
	Address     string
	TxHash      string
	Network     string
	ArrivalTime string
}

// DepositSuccessData 充值到账邮件模板数据
type DepositSuccessData struct {
	CommonEmailData
	Currency              string
	Amount                string
	Network               string // 网络/链名称 (ETH/BSC/TRON)
	DepositAddress        string
	SourceAddress         string
	TxHash                string
	ArrivalTime           string
	Confirmations         string
	RequiredConfirmations string
}

// DepositFailedData 充值失败邮件模板数据
type DepositFailedData struct {
	CommonEmailData
	Currency       string
	Amount         string
	Network        string // 网络/链名称 (ETH/BSC/TRON)
	DepositAddress string
	SourceAddress  string
	TxHash         string
	DetectedTime   string
}

// InternalTransferSuccessData 内部转账成功邮件模板数据
type InternalTransferSuccessData struct {
	CommonEmailData
	FromAccount  string
	ToAccount    string
	ReceiverNote string
	Amount       string
	Currency     string
	TransferTime string
	OrderID      string
	OrderNote    string
}

// SecurityAlertAbnormalLoginData 安全警报异常登录邮件模板数据
type SecurityAlertAbnormalLoginData struct {
	CommonEmailData
	LoginTime string
	Device    string
	IPAddress string
	Location  string
}

// TwoFactorAuthEnabledData 2FA启用邮件模板数据
type TwoFactorAuthEnabledData struct {
	CommonEmailData
	ChangeTime string
	IPAddress  string
	Location   string
}

// TwoFactorAuthDisabledData 2FA禁用邮件模板数据
type TwoFactorAuthDisabledData struct {
	CommonEmailData
	DisabledMethod string
	ChangeTime     string
	IPAddress      string
	Location       string
}

// EmailChangeSuccessData 邮箱修改成功邮件模板数据
type EmailChangeSuccessData struct {
	CommonEmailData
	OldEmail   string
	NewEmail   string
	ChangeTime string
	IPAddress  string
	Location   string
}

// TradingPasswordChangeSuccessData 交易密码修改成功邮件模板数据
type TradingPasswordChangeSuccessData struct {
	CommonEmailData
	ChangeTime string
	IPAddress  string
	Location   string
}

// PhoneChangeSuccessData 手机号修改成功邮件模板数据
type PhoneChangeSuccessData struct {
	CommonEmailData
	OldPhone    string
	NewPhone    string
	CountryCode string
	ChangeTime  string
	IPAddress   string
	Location    string
}

// GetTemplate 获取指定类型的邮件模板
func GetTemplate(templateType TemplateType) *template.Template {
	loadOnce.Do(loadTemplates)
	return templates[string(templateType)]
}

// GetVerificationEmailTemplate 获取验证码邮件模板（保持向后兼容）
func GetVerificationEmailTemplate() *template.Template {
	return GetTemplate(TemplateVerificationEmail)
}

// loadTemplates 从 embed.FS 加载所有邮件模板
func loadTemplates() {
	templates = make(map[string]*template.Template)

	templateFiles := []string{
		"verification_email.html",
		"withdrawal_submitted.html",
		"deposit_success.html",
		"deposit_failed.html",
		"internal_transfer_success.html",
		"security_alert_abnormal_login.html",
		"2fa_enabled.html",
		"2fa_disabled.html",
		"email_change_success.html",
		"trading_password_change_success.html",
		"phone_change_success.html",
	}

	for _, filename := range templateFiles {
		// 从 embed.FS 读取模板内容
		content, err := templateFS.ReadFile(filename)
		if err != nil {
			logx.Errorf("failed to read email template %s: %v", filename, err)
			continue
		}

		// 解析模板
		tmpl, err := template.New(filename).Parse(string(content))
		if err != nil {
			logx.Errorf("failed to parse email template %s: %v", filename, err)
			continue
		}

		// 存储模板（去掉 .html 后缀作为key）
		templateKey := filename[:len(filename)-5] // 移除 ".html"
		templates[templateKey] = tmpl
		logx.Infof("email template %s loaded successfully", filename)
	}
}

// RenderTemplate 渲染指定类型的邮件模板
func RenderTemplate(templateType TemplateType, data interface{}) (string, error) {
	tmpl := GetTemplate(templateType)
	if tmpl == nil {
		return "", fmt.Errorf("template %s not loaded", templateType)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template %s: %w", templateType, err)
	}

	return buf.String(), nil
}

// RenderVerificationEmail 渲染验证码邮件模板（保持向后兼容）
func RenderVerificationEmail(data VerificationEmailData) (string, error) {
	return RenderTemplate(TemplateVerificationEmail, data)
}

// RenderWithdrawalSubmittedEmail 渲染提币申请提交邮件模板
func RenderWithdrawalSubmittedEmail(data WithdrawalSubmittedData) (string, error) {
	return RenderTemplate(TemplateWithdrawalSubmitted, data)
}

// RenderDepositSuccessEmail 渲染充值到账邮件模板
func RenderDepositSuccessEmail(data DepositSuccessData) (string, error) {
	return RenderTemplate(TemplateDepositSuccess, data)
}

// RenderDepositFailedEmail 渲染充值失败邮件模板
func RenderDepositFailedEmail(data DepositFailedData) (string, error) {
	return RenderTemplate(TemplateDepositFailed, data)
}

// RenderInternalTransferSuccessEmail 渲染内部转账成功邮件模板
func RenderInternalTransferSuccessEmail(data InternalTransferSuccessData) (string, error) {
	return RenderTemplate(TemplateInternalTransferSuccess, data)
}

// RenderSecurityAlertAbnormalLoginEmail 渲染安全警报异常登录邮件模板
func RenderSecurityAlertAbnormalLoginEmail(data SecurityAlertAbnormalLoginData) (string, error) {
	return RenderTemplate(TemplateSecurityAlertAbnormalLogin, data)
}

// Render2FAEnabledEmail 渲染2FA启用邮件模板
func Render2FAEnabledEmail(data TwoFactorAuthEnabledData) (string, error) {
	return RenderTemplate(Template2FAEnabled, data)
}

// Render2FADisabledEmail 渲染2FA禁用邮件模板
func Render2FADisabledEmail(data TwoFactorAuthDisabledData) (string, error) {
	return RenderTemplate(Template2FADisabled, data)
}

// RenderEmailChangeSuccessEmail 渲染邮箱修改成功邮件模板
func RenderEmailChangeSuccessEmail(data EmailChangeSuccessData) (string, error) {
	return RenderTemplate(TemplateEmailChangeSuccess, data)
}

// RenderTradingPasswordChangeSuccessEmail 渲染交易密码修改成功邮件模板
func RenderTradingPasswordChangeSuccessEmail(data TradingPasswordChangeSuccessData) (string, error) {
	return RenderTemplate(TemplateTradingPasswordChange, data)
}

// RenderPhoneChangeSuccessEmail 渲染手机号修改成功邮件模板
func RenderPhoneChangeSuccessEmail(data PhoneChangeSuccessData) (string, error) {
	return RenderTemplate(TemplatePhoneChangeSuccess, data)
}

// ErrTemplateNotLoaded 模板未加载错误
var ErrTemplateNotLoaded = &templateError{message: "email template not loaded"}

type templateError struct {
	message string
}

func (e *templateError) Error() string {
	return e.message
}
