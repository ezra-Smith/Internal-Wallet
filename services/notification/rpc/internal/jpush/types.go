package jpush

// NotificationPayload 推送消息载荷
type NotificationPayload struct {
	Title   string            // 标题
	Content string            // 内容
	Extras  map[string]string // 扩展数据
}

// PushTarget 推送目标
type PushTarget struct {
	RegistrationIDs []string // Registration ID列表
	Aliases         []string // 别名列表
	Tags            []string // 标签列表
	All             bool     // 是否推送给所有用户
}

// PushOptions 推送选项
type PushOptions struct {
	ApnsProduction bool // iOS推送环境：false=开发 true=生产
	TimeToLive     int  // 离线消息保留时长（秒），默认86400秒（1天）
	Priority       int  // 优先级：0=普通 1=高
}

// PushResult 推送结果
type PushResult struct {
	Success    bool   // 是否成功
	MsgID      string // 消息ID
	SendNo     string // 发送编号
	Error      error  // 错误信息
	StatusCode int    // HTTP状态码
}

// PushStatus 推送状态查询结果
type PushStatus struct {
	MsgID           string                 // 消息ID
	AndroidReceived int                    // Android接收数
	IOSReceived     int                    // iOS接收数
	IOSAPNSSent     int                    // iOS APNS发送数
	Extra           map[string]interface{} // 额外信息
}

// 常量定义
const (
	// 推送平台
	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformHMOS    = "hmos" // 鸿蒙系统 (HarmonyOS)
	PlatformAll     = "all"

	// 推送类型
	PushTypeNotification = "notification" // 通知栏消息
	PushTypeMessage      = "message"      // 自定义消息（透传）

	// 优先级
	PriorityNormal = 0
	PriorityHigh   = 1

	// 默认配置
	DefaultTimeToLive = 86400 // 1天
	DefaultTimeout    = 5000  // 5秒
	DefaultMaxRetries = 3
)
