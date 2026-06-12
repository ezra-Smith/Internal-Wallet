package model

import (
	"gorm.io/gorm"
	"internalwallet/common/model"
	"time"
)

// KafkaFailedMessage Kafka发送失败消息模型
type KafkaFailedMessage struct {
	model.BaseModel
	Topic        string     `gorm:"type:varchar(128);not null;index" json:"topic"`       // Kafka主题
	Key          string     `gorm:"type:varchar(256);not null" json:"key"`               // 消息Key
	Message      string     `gorm:"type:text;not null" json:"message"`                   // 消息内容(JSON)
	MessageType  string     `gorm:"type:varchar(64);not null;index" json:"message_type"` // 消息类型: transaction, balance_validation等
	TxHash       string     `gorm:"type:varchar(128);index" json:"tx_hash"`              // 关联的交易哈希(可选)
	Chain        string     `gorm:"type:varchar(50);index" json:"chain"`                 // 区块链类型
	Status       uint8      `gorm:"not null;default:0;index" json:"status"`              // 状态: 0-待重试, 1-成功, 2-失败(超过重试次数)
	RetryCount   int32      `gorm:"not null;default:0" json:"retry_count"`               // 重试次数
	MaxRetries   int32      `gorm:"not null;default:5" json:"max_retries"`               // 最大重试次数
	NextRetryAt  *time.Time `gorm:"type:datetime;index" json:"next_retry_at"`            // 下次重试时间
	LastError    string     `gorm:"type:text" json:"last_error"`                         // 最后一次错误信息
	LastRetryAt  *time.Time `gorm:"type:datetime" json:"last_retry_at"`                  // 最后一次重试时间
	SuccessAt    *time.Time `gorm:"type:datetime" json:"success_at"`                     // 成功发送时间
	MessageID    string     `gorm:"type:varchar(128)" json:"message_id"`                 // Kafka消息ID(发送成功后填充)
	Priority     int32      `gorm:"not null;default:0;index" json:"priority"`            // 优先级: 0-普通, 1-高优先级
	SourceModule string     `gorm:"type:varchar(64);index" json:"source_module"`         // 来源模块
}

// TableName 指定表名
func (KafkaFailedMessage) TableName() string {
	return "kafka_failed_message"
}

// BeforeCreate 创建前回调
func (m *KafkaFailedMessage) BeforeCreate(tx *gorm.DB) error {
	if err := m.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}
	if m.NextRetryAt == nil {
		now := time.Now().Local()
		m.NextRetryAt = &now
	}
	return nil
}

// BeforeUpdate 更新前回调
func (m *KafkaFailedMessage) BeforeUpdate(tx *gorm.DB) error {
	return m.BaseModel.BeforeUpdate(tx)
}

// IsPending 检查是否待重试
func (m *KafkaFailedMessage) IsPending() bool {
	return m.Status == 0
}

// IsSuccess 检查是否成功
func (m *KafkaFailedMessage) IsSuccess() bool {
	return m.Status == 1
}

// IsFailed 检查是否失败(超过重试次数)
func (m *KafkaFailedMessage) IsFailed() bool {
	return m.Status == 2
}

// CanRetry 检查是否可以重试
func (m *KafkaFailedMessage) CanRetry() bool {
	return m.Status == 0 && m.RetryCount < m.MaxRetries
}

// ShouldRetryNow 检查是否应该立即重试
func (m *KafkaFailedMessage) ShouldRetryNow() bool {
	if !m.CanRetry() {
		return false
	}
	if m.NextRetryAt == nil {
		return true
	}
	return time.Now().After(*m.NextRetryAt)
}

// IncrementRetry 增加重试次数并计算下次重试时间
func (m *KafkaFailedMessage) IncrementRetry(baseDelay time.Duration, maxDelay time.Duration, backoffMultiplier float64) {
	m.RetryCount++
	now := time.Now()
	m.LastRetryAt = &now

	// 计算下次重试延迟(指数退避)
	delay := baseDelay
	for i := int32(1); i < m.RetryCount; i++ {
		delay = time.Duration(float64(delay) * backoffMultiplier)
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}

	nextRetry := now.Add(delay)
	m.NextRetryAt = &nextRetry
}

// MarkSuccess 标记为成功
func (m *KafkaFailedMessage) MarkSuccess(messageID string) {
	m.Status = 1
	now := time.Now()
	m.SuccessAt = &now
	m.MessageID = messageID
	m.LastError = ""
}

// MarkFailed 标记为失败(超过重试次数)
func (m *KafkaFailedMessage) MarkFailed(errorMessage string) {
	m.Status = 2
	m.LastError = errorMessage
}
