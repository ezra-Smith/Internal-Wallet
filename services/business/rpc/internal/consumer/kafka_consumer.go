package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"internalwallet/common/mq"
	"internalwallet/services/business/rpc/internal/config"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"
)

type topicKind string

const (
	topicKindTransactionConfirm topicKind = "transaction_confirm"
	topicKindBalanceChange      topicKind = "balance_change"
)

type topicConsumer struct {
	name    string
	groupID string
	topic   string
	kind    topicKind

	group   sarama.ConsumerGroup
	handler sarama.ConsumerGroupHandler
}

// KafkaConsumer Kafka 消费者服务
type KafkaConsumer struct {
	svcCtx    *svc.ServiceContext
	config    config.Config
	consumers []topicConsumer
	workers   int
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

// NewKafkaConsumer 创建 Kafka 消费者
func NewKafkaConsumer(svcCtx *svc.ServiceContext, cfg config.Config) (*KafkaConsumer, error) {
	if len(cfg.KafkaConsumer.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers not configured")
	}

	// 创建 Sarama 配置
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_8_0_0
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Group.Session.Timeout = time.Duration(cfg.KafkaConsumer.SessionTimeout) * time.Second
	saramaConfig.Consumer.Group.Rebalance.Timeout = time.Duration(cfg.KafkaConsumer.RebalanceTimeout) * time.Second
	saramaConfig.Consumer.MaxProcessingTime = time.Duration(cfg.KafkaConsumer.MaxProcessingTime) * time.Second

	// SASL 配置
	if cfg.KafkaConsumer.Security != "PLAINTEXT" {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		if cfg.KafkaConsumer.Username != "" {
			saramaConfig.Net.SASL.User = cfg.KafkaConsumer.Username
		}
		if cfg.KafkaConsumer.Password != "" {
			saramaConfig.Net.SASL.Password = cfg.KafkaConsumer.Password
		}
	}

	// 创建消费者组
	baseGroupID := strings.TrimSpace(cfg.KafkaConsumer.GroupID)
	if baseGroupID == "" {
		return nil, fmt.Errorf("kafka consumer group id not configured")
	}

	var consumers []topicConsumer
	for _, spec := range []struct {
		name   string
		suffix string
		topic  string
		kind   topicKind
	}{
		{
			name:   "tx-deposit",
			suffix: "tx-deposit",
			topic:  strings.TrimSpace(cfg.KafkaConsumer.Topics.TransactionDeposit),
			kind:   topicKindTransactionConfirm,
		},
		{
			name:   "tx-web3",
			suffix: "tx-web3",
			topic:  strings.TrimSpace(cfg.KafkaConsumer.Topics.TransactionWeb3),
			kind:   topicKindTransactionConfirm,
		},
		{
			name:   "tx-vault",
			suffix: "tx-vault",
			topic:  strings.TrimSpace(cfg.KafkaConsumer.Topics.TransactionVault),
			kind:   topicKindTransactionConfirm,
		},
		{
			name:   "tx-manual",
			suffix: "tx-manual",
			topic:  strings.TrimSpace(cfg.KafkaConsumer.Topics.TransactionManual),
			kind:   topicKindTransactionConfirm,
		},
		{
			name:   "tx-unknown",
			suffix: "tx-unknown",
			topic:  strings.TrimSpace(cfg.KafkaConsumer.Topics.TransactionUnknown),
			kind:   topicKindTransactionConfirm,
		},
		{
			name:   "balance-change",
			suffix: "balance-change",
			topic:  strings.TrimSpace(cfg.KafkaConsumer.Topics.BalanceChange),
			kind:   topicKindBalanceChange,
		},
	} {
		if spec.topic == "" {
			continue
		}

		groupID := fmt.Sprintf("%s-%s", baseGroupID, spec.suffix)

		consumerGroup, err := sarama.NewConsumerGroup(cfg.KafkaConsumer.Brokers, groupID, saramaConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create consumer group (name=%s group_id=%s): %w", spec.name, groupID, err)
		}

		handler := &consumerGroupHandler{
			svcCtx:  svcCtx,
			config:  cfg,
			workers: cfg.KafkaConsumer.Workers,
			kind:    spec.kind,
		}

		consumers = append(consumers, topicConsumer{
			name:    spec.name,
			groupID: groupID,
			topic:   spec.topic,
			kind:    spec.kind,
			group:   consumerGroup,
			handler: handler,
		})
	}

	if len(consumers) == 0 {
		return nil, fmt.Errorf("no kafka topics configured for consumer")
	}

	return &KafkaConsumer{
		svcCtx:    svcCtx,
		config:    cfg,
		consumers: consumers,
		workers:   cfg.KafkaConsumer.Workers,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start 启动消费者
func (kc *KafkaConsumer) Start() error {
	logx.Info("🚀 Starting Kafka consumers...")

	for _, consumer := range kc.consumers {
		consumer := consumer
		topics := []string{consumer.topic}

		kc.wg.Add(1)
		go func() {
			defer kc.wg.Done()
			for {
				select {
				case <-kc.stopCh:
					logx.Infof("Kafka consumer stopping: name=%s group_id=%s topics=%v", consumer.name, consumer.groupID, topics)
					return
				default:
					if err := consumer.group.Consume(context.Background(), topics, consumer.handler); err != nil {
						logx.Errorf("Error from consumer: name=%s group_id=%s topics=%v err=%v", consumer.name, consumer.groupID, topics, err)
						time.Sleep(5 * time.Second) // 等待后重试
					}
				}
			}
		}()

		logx.Infof("✅ Kafka consumer started: name=%s group_id=%s topics=%v", consumer.name, consumer.groupID, topics)
	}
	return nil
}

// Stop 停止消费者
func (kc *KafkaConsumer) Stop() error {
	logx.Info("Stopping Kafka consumers...")
	close(kc.stopCh)
	kc.wg.Wait()

	var closeErr error
	for _, consumer := range kc.consumers {
		if err := consumer.group.Close(); err != nil && closeErr == nil {
			closeErr = fmt.Errorf("failed to close consumer group (name=%s group_id=%s): %w", consumer.name, consumer.groupID, err)
		}
	}
	if closeErr != nil {
		return closeErr
	}
	logx.Info("✅ Kafka consumers stopped")
	return nil
}

// consumerGroupHandler 消费者组处理器
type consumerGroupHandler struct {
	svcCtx  *svc.ServiceContext
	config  config.Config
	workers int
	kind    topicKind
}

// Setup 在消费者组会话开始时调用
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	logx.Info("Kafka consumer group session started")
	return nil
}

// Cleanup 在消费者组会话结束时调用
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	logx.Info("Kafka consumer group session ended")
	return nil
}

// ConsumeClaim 处理消息
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// NOTE:
	// ConsumeClaim is already invoked per-partition by Sarama.
	// Process messages sequentially to preserve ordering and avoid committing offsets past failures.
	for msg := range claim.Messages() {
		if msg == nil {
			continue
		}
		if err := h.processMessage(msg); err != nil {
			// Returning error will abort this claim; the outer Consume loop will retry.
			return err
		}
		session.MarkMessage(msg, "")
	}
	return nil
}

// processMessage 处理单条消息
func (h *consumerGroupHandler) processMessage(msg *sarama.ConsumerMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logx.Debugf("Received message from topic %s, partition %d, offset %d", msg.Topic, msg.Partition, msg.Offset)

	// 根据 kind 路由到不同的处理器
	switch h.kind {
	case topicKindTransactionConfirm:
		return h.handleTransactionConfirm(ctx, msg)
	case topicKindBalanceChange:
		return h.handleBalanceChange(ctx, msg)
	default:
		logx.Infof("Unknown kafka message kind: %s (topic=%s)", h.kind, msg.Topic)
		return nil
	}
}

// handleTransactionConfirm 处理交易确认消息
func (h *consumerGroupHandler) handleTransactionConfirm(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var txMsg mq.TransactionConfirmMessage
	if err := json.Unmarshal(msg.Value, &txMsg); err != nil {
		logx.Errorf("Failed to unmarshal transaction confirm message: %v", err)
		return err
	}

	logx.Infof("Processing transaction confirm: tx_hash=%s, chain=%s, monitored_address=%s",
		txMsg.TxHash, txMsg.Chain, txMsg.MonitoredAddress)

	// 调用交易确认处理器
	processor := NewTransactionConfirmProcessor(h.svcCtx)
	if err := processor.Process(ctx, &txMsg); err != nil {
		logx.Errorf("Failed to process transaction confirm: tx_hash=%s, error=%v", txMsg.TxHash, err)
		// IMPORTANT:
		// Returning error here will prevent offset commits and can permanently stall the consumer group
		// on a single "poison pill" message (e.g. unknown token mapping). Until we have a DLQ/retry
		// queue, we must not block the whole stream.
		return nil
	}

	logx.Infof("✅ Successfully processed transaction confirm: tx_hash=%s", txMsg.TxHash)
	return nil
}

// handleBalanceChange 处理余额变动消息
func (h *consumerGroupHandler) handleBalanceChange(ctx context.Context, msg *sarama.ConsumerMessage) error {
	// TODO: 实现余额变动消息处理
	// 当前策略：仅记录日志，不处理
	//
	// 设计考虑：
	// 1. BalanceChange 消息可能与 TransactionConfirm 消息重复（同一笔交易同时触发两个事件）
	// 2. 当前余额同步逻辑已在 TransactionConfirm 中处理，可能不需要单独处理 BalanceChange
	// 3. 如果需要独立处理，建议实现：
	//    - 解析 BalanceChange 消息（格式待定）
	//    - 调用 syncWeb3AddressBalanceFromChain 同步余额
	//    - 去重机制（避免与 TransactionConfirm 冲突）
	//
	// 当前行为：消息被成功消费但不处理（避免 Kafka offset 卡住）
	logx.Infof("⚠️  BalanceChange message received but not implemented, skipping: topic=%s partition=%d offset=%d",
		msg.Topic, msg.Partition, msg.Offset)
	return nil
}
