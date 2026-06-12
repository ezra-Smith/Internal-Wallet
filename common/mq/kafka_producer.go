package mq

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

// KafkaProducer provides a minimal interface used across services.
type KafkaProducer interface {
	SendMessage(topic string, key string, message interface{}) (string, error)
	Close() error
}

// KafkaProducerConfig defines producer settings shared across services.
type KafkaProducerConfig struct {
	Brokers     []string
	Username    string
	Password    string
	Security    string // PLAINTEXT, SASL_PLAINTEXT, SASL_SSL, SSL
	SASLMech    string // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	ClientID    string
	Compression string // none, gzip, snappy, lz4, zstd
	MaxRetries  int
	Timeout     time.Duration
	UseAsync    bool
}

// SaramaProducer is a sarama-based Kafka producer.
type SaramaProducer struct {
	syncProducer  sarama.SyncProducer
	asyncProducer sarama.AsyncProducer
	isAsync       bool
}

func NewSaramaProducer(cfg KafkaProducerConfig) (*SaramaProducer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers is empty")
	}

	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V3_0_0_0

	// Producer config
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.Return.Errors = true
	saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
	if cfg.MaxRetries > 0 {
		saramaCfg.Producer.Retry.Max = cfg.MaxRetries
	}
	if cfg.Timeout > 0 {
		saramaCfg.Producer.Timeout = cfg.Timeout
	}

	// Compression
	switch cfg.Compression {
	case "gzip":
		saramaCfg.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaCfg.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaCfg.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaCfg.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaCfg.Producer.Compression = sarama.CompressionNone
	}

	// SASL/TLS
	security := cfg.Security
	if security == "" {
		security = "PLAINTEXT"
	}
	if security != "PLAINTEXT" && (cfg.Username != "" || cfg.Password != "") {
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.User = cfg.Username
		saramaCfg.Net.SASL.Password = cfg.Password
		switch cfg.SASLMech {
		case "SCRAM-SHA-256":
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "SCRAM-SHA-512":
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		default:
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
		if security == "SASL_SSL" || security == "SSL" {
			saramaCfg.Net.TLS.Enable = true
		}
	}

	if cfg.ClientID != "" {
		saramaCfg.ClientID = cfg.ClientID
	}

	p := &SaramaProducer{isAsync: cfg.UseAsync}
	var err error
	if cfg.UseAsync {
		p.asyncProducer, err = sarama.NewAsyncProducer(cfg.Brokers, saramaCfg)
		if err != nil {
			return nil, fmt.Errorf("create async producer: %w", err)
		}
	} else {
		p.syncProducer, err = sarama.NewSyncProducer(cfg.Brokers, saramaCfg)
		if err != nil {
			return nil, fmt.Errorf("create sync producer: %w", err)
		}
	}
	return p, nil
}

func (p *SaramaProducer) SendMessage(topic string, key string, message interface{}) (string, error) {
	if p == nil {
		return "", fmt.Errorf("producer is nil")
	}
	if topic == "" {
		return "", fmt.Errorf("topic is empty")
	}

	var payload []byte
	switch v := message.(type) {
	case []byte:
		payload = v
	case string:
		payload = []byte(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal message: %w", err)
		}
		payload = b
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(payload),
	}

	if p.isAsync {
		if p.asyncProducer == nil {
			return "", fmt.Errorf("async producer not initialized")
		}
		id := uuid.New().String()
		msg.Metadata = id
		p.asyncProducer.Input() <- msg
		return id, nil
	}

	if p.syncProducer == nil {
		return "", fmt.Errorf("sync producer not initialized")
	}
	partition, offset, err := p.syncProducer.SendMessage(msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%d", topic, partition, offset), nil
}

func (p *SaramaProducer) Close() error {
	if p == nil {
		return nil
	}
	if p.asyncProducer != nil {
		return p.asyncProducer.Close()
	}
	if p.syncProducer != nil {
		return p.syncProducer.Close()
	}
	return nil
}
