package model

import "time"

// Web3UserSettingsModel 映射到 web3_user_settings 表（Web3 用户偏好设置）
type Web3UserSettingsModel struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Web3UserID int64  `gorm:"column:web3_user_id;type:bigint;not null;uniqueIndex"`
	DeviceID   string `gorm:"column:device_id;type:varchar(128);not null;index"`

	// 显示设置
	DisplayCurrency string `gorm:"column:display_currency;type:varchar(10);not null;default:'USD'"` // USD/CNY/EUR 等
	DisplayLanguage string `gorm:"column:display_language;type:varchar(20);not null;default:'en'"`  // en/zh-CN/zh-TW 等
	ThemeMode       string `gorm:"column:theme_mode;type:varchar(20);not null;default:'system'"`    // system/light/dark

	// 通知设置
	PushEnabled    bool `gorm:"column:push_enabled;type:tinyint(1);not null;default:1"`     // 推送总开关
	PushSound      bool `gorm:"column:push_sound;type:tinyint(1);not null;default:1"`       // 推送声音
	PushLockScreen bool `gorm:"column:push_lock_screen;type:tinyint(1);not null;default:1"` // 锁屏时推送
	PushBadge      bool `gorm:"column:push_badge;type:tinyint(1);not null;default:1"`       // 标识外观
	PushVibration  bool `gorm:"column:push_vibration;type:tinyint(1);not null;default:1"`   // 震动

	// 网络设置
	ShowTestnet    bool   `gorm:"column:show_testnet;type:tinyint(1);not null;default:0"`       // 显示测试网
	ActiveNetworks string `gorm:"column:active_networks;type:varchar(500);not null;default:''"` // JSON 数组，如 ["Ethereum","Bitcoin","Solana"]

	// 隐私设置
	DataCollection bool `gorm:"column:data_collection;type:tinyint(1);not null;default:1"` // 数据收集
	UserAnalytics  bool `gorm:"column:user_analytics;type:tinyint(1);not null;default:1"`  // 用户分析

	// 时间戳
	CreatedAt *time.Time `gorm:"column:created_at;type:timestamp;index"`
	UpdatedAt *time.Time `gorm:"column:updated_at;type:timestamp"`
}

func (Web3UserSettingsModel) TableName() string { return "web3_user_settings" }

// 主题模式常量
const (
	ThemeModeSystem = "system" // 跟随系统
	ThemeModeLight  = "light"  // 浅色
	ThemeModeDark   = "dark"   // 深色
)

// 货币常量
const (
	CurrencyUSD = "USD" // 美元
	CurrencyCNY = "CNY" // 人民币
	CurrencyEUR = "EUR" // 欧元
	CurrencyJPY = "JPY" // 日元
)

// 语言常量
const (
	LanguageEN   = "en"    // 英语
	LanguageZH   = "zh-CN" // 中文简体
	LanguageZHTW = "zh-TW" // 中文繁体
	LanguageJA   = "ja"    // 日语
	LanguageKO   = "ko"    // 韩语
)
