package svc

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers     []string `json:",required"`   // Kafka broker列表
	Username    string   `json:",optional"`   // 用户名
	Password    string   `json:",optional"`   // 密码
	Security    string   `json:",optional"`   // 安全协议 (PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL)
	SASLMech    string   `json:",optional"`   // SASL机制 (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)
	ClientID    string   `json:",optional"`   // 客户端ID
	Compression string   `json:",optional"`   // 压缩算法 (none, gzip, snappy, lz4, zstd)
	MaxRetries  int      `json:",default=3"`  // 最大重试次数
	Timeout     int      `json:",default=30"` // 超时时间（秒）
}

// SaramaKafkaProducer 基于sarama的Kafka生产者
type SaramaKafkaProducer struct {
	config        *KafkaConfig
	producer      sarama.SyncProducer
	asyncProducer sarama.AsyncProducer
	isAsync       bool
	mu            sync.RWMutex
	closed        bool
	stats         *ProducerStats
}

// ProducerStats 生产者统计
type ProducerStats struct {
	MessagesSent    uint64 `json:"messages_sent"`
	MessagesFailed  uint64 `json:"messages_failed"`
	BytesSent       uint64 `json:"bytes_sent"`
	LastError       string `json:"last_error"`
	LastSuccessTime int64  `json:"last_success_time"`
	mu              sync.RWMutex
}

// NewSaramaKafkaProducer 创建Kafka生产者
func NewSaramaKafkaProducer(config *KafkaConfig, isAsync bool) (*SaramaKafkaProducer, error) {
	if config == nil || len(config.Brokers) == 0 {
		return nil, fmt.Errorf("kafka config or brokers is empty")
	}

	// 创建sarama配置
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V3_0_0_0 // 使用Kafka 3.0.0版本的协议

	// 设置生产者配置
	if isAsync {
		// 异步生产者配置
		saramaConfig.Producer.Return.Successes = true
		saramaConfig.Producer.Return.Errors = true
		saramaConfig.Producer.Flush.Messages = 100 // 每当消息数量达到100时刷新
		saramaConfig.Producer.Flush.Frequency = 10 * time.Millisecond
	} else {
		// 同步生产者配置
		saramaConfig.Producer.Return.Successes = true
		saramaConfig.Producer.Return.Errors = true
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll // 等待所有副本确认
		saramaConfig.Producer.Retry.Max = config.MaxRetries
		saramaConfig.Producer.Retry.Backoff = 100 * time.Millisecond
		saramaConfig.Producer.Timeout = time.Duration(config.Timeout) * time.Second
	}

	// 设置压缩算法
	switch config.Compression {
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionNone
	}

	// 设置安全认证
	if config.Security != "PLAINTEXT" && (config.Username != "" || config.Password != "") {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.User = config.Username
		saramaConfig.Net.SASL.Password = config.Password

		switch config.SASLMech {
		case "SCRAM-SHA-256":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "SCRAM-SHA-512":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		default:
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}

		if config.Security == "SASL_SSL" || config.Security == "SSL" {
			saramaConfig.Net.TLS.Enable = true
		}
	}

	// 设置客户端ID
	if config.ClientID != "" {
		saramaConfig.ClientID = config.ClientID
	} else {
		saramaConfig.ClientID = "chainsync-producer"
	}

	producer := &SaramaKafkaProducer{
		config:  config,
		isAsync: isAsync,
		stats:   &ProducerStats{},
	}

	var err error
	if isAsync {
		// 创建异步生产者
		producer.asyncProducer, err = sarama.NewAsyncProducer(config.Brokers, saramaConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create async kafka producer: %v", err)
		}

		// 启动消息处理协程
		go producer.handleAsyncMessages()

		logx.Info("Created async kafka producer")
	} else {
		// 创建同步生产者
		producer.producer, err = sarama.NewSyncProducer(config.Brokers, saramaConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create sync kafka producer: %v", err)
		}

		logx.Info("Created sync kafka producer")
	}

	return producer, nil
}

// handleAsyncMessages 处理异步消息的成功和失败回调
func (p *SaramaKafkaProducer) handleAsyncMessages() {
	for {
		select {
		case successMsg, ok := <-p.asyncProducer.Successes():
			if !ok {
				return
			}
			if successMsg == nil {
				continue
			}
			p.updateStats(successMsg, nil)
			// 获取客户端生成的 UUID
			clientUUID := ""
			if successMsg.Metadata != nil {
				if uuidStr, ok := successMsg.Metadata.(string); ok {
					clientUUID = uuidStr
				}
			}
			// 构建 Kafka 返回的真实消息 ID
			kafkaMessageID := fmt.Sprintf("%s-%d-%d", successMsg.Topic, successMsg.Partition, successMsg.Offset)
			logx.Infof("✅ Async message sent successfully: UUID=%s → KafkaID=%s (topic=%s, partition=%d, offset=%d)",
				clientUUID, kafkaMessageID, successMsg.Topic, successMsg.Partition, successMsg.Offset)

		case errMsg, ok := <-p.asyncProducer.Errors():
			if !ok {
				return
			}
			if errMsg == nil {
				continue
			}
			p.updateStats(errMsg.Msg, errMsg.Err)
			clientUUID := ""
			if errMsg.Msg != nil && errMsg.Msg.Metadata != nil {
				if uuidStr, ok := errMsg.Msg.Metadata.(string); ok {
					clientUUID = uuidStr
				}
			}
			logx.Errorf("❌ Failed to send async message (UUID=%s): %v", clientUUID, errMsg.Err)
		}
	}
}

// SendMessage 发送消息
// 返回值说明：
// - 同步模式：返回 Kafka 分配的真实消息 ID（格式：topic-partition-offset）
// - 异步模式：返回客户端生成的 UUID（消息实际发送成功后可通过回调获取真实 ID）
func (p *SaramaKafkaProducer) SendMessage(topic string, key string, message interface{}) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return "", fmt.Errorf("producer is closed")
	}

	// 序列化消息
	var messageBytes []byte
	var err error

	switch msg := message.(type) {
	case []byte:
		messageBytes = msg
	case string:
		messageBytes = []byte(msg)
	default:
		messageBytes, err = json.Marshal(message)
		if err != nil {
			return "", fmt.Errorf("failed to marshal message: %v", err)
		}
	}

	// 生成唯一消息 ID（用于追踪）
	messageUUID := uuid.New().String()

	// 创建消息
	kafkaMsg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(messageBytes),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("source"),
				Value: []byte("chainsync"),
			},
			{
				Key:   []byte("timestamp"),
				Value: []byte(fmt.Sprintf("%d", time.Now().Unix())),
			},
			{
				Key:   []byte("message_id"),
				Value: []byte(messageUUID),
			},
		},
		Metadata: messageUUID, // 存储 UUID 用于回调时追踪
	}

	if p.isAsync {
		// 异步发送
		// 注意：异步模式下，消息只是放入发送队列，尚未真正发送到 Kafka
		// 因此无法获取 Kafka 返回的 partition 和 offset
		// 返回客户端生成的 UUID 作为临时标识，真实的消息 ID 需要在成功回调中获取
		select {
		case p.asyncProducer.Input() <- kafkaMsg:
			logx.Debugf("📤 Async message queued, UUID: %s, topic: %s", messageUUID, topic)
			return messageUUID, nil
		default:
			return "", fmt.Errorf("async producer input channel is full")
		}
	} else {
		// 同步发送
		// Kafka 返回真实的 partition 和 offset
		partition, offset, err := p.producer.SendMessage(kafkaMsg)
		if err != nil {
			return "", fmt.Errorf("failed to send message: %v", err)
		}

		// 使用 Kafka 返回的 partition 和 offset 组成真实的消息 ID
		messageID := fmt.Sprintf("%s-%d-%d", topic, partition, offset)
		p.updateStats(kafkaMsg, nil)

		logx.Debugf("📤 Sync message sent, ID: %s (UUID: %s)", messageID, messageUUID)
		return messageID, nil
	}
}

// updateStats 更新统计信息
func (p *SaramaKafkaProducer) updateStats(msg *sarama.ProducerMessage, err error) {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()

	if err != nil {
		p.stats.MessagesFailed++
		p.stats.LastError = err.Error()
	} else {
		p.stats.MessagesSent++
		if msg != nil {
			if v, ok := msg.Value.(sarama.ByteEncoder); ok {
				p.stats.BytesSent += uint64(len(v))
			}
		}
		p.stats.LastSuccessTime = time.Now().Unix()
	}
}

// GetStats 获取统计信息
func (p *SaramaKafkaProducer) GetStats() *ProducerStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	return &ProducerStats{
		MessagesSent:    p.stats.MessagesSent,
		MessagesFailed:  p.stats.MessagesFailed,
		BytesSent:       p.stats.BytesSent,
		LastError:       p.stats.LastError,
		LastSuccessTime: p.stats.LastSuccessTime,
	}
}

// Close 关闭生产者
func (p *SaramaKafkaProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	var err error

	if p.isAsync && p.asyncProducer != nil {
		err = p.asyncProducer.Close()
	} else if !p.isAsync && p.producer != nil {
		err = p.producer.Close()
	}

	if err != nil {
		logx.Errorf("Error closing kafka producer: %v", err)
		return err
	}

	logx.Info("Kafka producer closed successfully")
	return nil
}

// NewMockKafkaProducer 创建模拟Kafka生产者（用于测试）
func NewMockKafkaProducer() *MockKafkaProducer {
	return &MockKafkaProducer{}
}

// MockKafkaProducer 模拟Kafka生产者
type MockKafkaProducer struct {
	messages []MockMessage
	mu       sync.RWMutex
}

type MockMessage struct {
	Topic   string
	Key     string
	Message interface{}
	Time    time.Time
}

func (m *MockKafkaProducer) SendMessage(topic string, key string, message interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := MockMessage{
		Topic:   topic,
		Key:     key,
		Message: message,
		Time:    time.Now(),
	}
	m.messages = append(m.messages, msg)

	messageID := fmt.Sprintf("mock_%d_%s", time.Now().UnixNano(), topic)
	logx.Infof("📤 [Mock Kafka] Sent message to topic %s, key: %s, messageID: %s", topic, key, messageID)
	return messageID, nil
}

func (m *MockKafkaProducer) Close() error {
	logx.Info("Mock Kafka producer closed")
	return nil
}

func (m *MockKafkaProducer) GetMessages() []MockMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	messages := make([]MockMessage, len(m.messages))
	copy(messages, m.messages)
	return messages
}

func (m *MockKafkaProducer) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = m.messages[:0]
}

// 默认Kafka配置
func DefaultKafkaConfig() *KafkaConfig {
	return &KafkaConfig{
		Brokers:     []string{"localhost:9092"},
		Security:    "PLAINTEXT",
		Compression: "gzip",
		MaxRetries:  3,
		Timeout:     30,
		ClientID:    "chainsync-producer",
	}
}
