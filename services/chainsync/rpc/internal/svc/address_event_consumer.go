package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/mq"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"
)

func (am *AddressMonitor) startAddressEventConsumer() {
	if am == nil || am.registry == nil || am.config == nil {
		return
	}
	am.wg.Add(1)
	go am.addressEventConsumerLoop()
}

func (am *AddressMonitor) addressEventConsumerLoop() {
	defer am.wg.Done()

	cfg := am.config
	if cfg == nil {
		return
	}
	if !cfg.KafkaConsumer.Enabled {
		logx.Info("KafkaConsumer disabled; skipping address monitor event consumption")
		return
	}

	brokers := cfg.KafkaConsumer.Brokers
	if len(brokers) == 0 {
		brokers = cfg.Kafka.Brokers
	}
	if len(brokers) == 0 {
		logx.Info("KafkaConsumer brokers not configured; skipping address monitor event consumption")
		return
	}

	groupID := strings.TrimSpace(cfg.KafkaConsumer.GroupID)
	if groupID == "" {
		groupID = "chainsync-address-monitor"
	}

	topic := strings.TrimSpace(cfg.Kafka.Topics.AddressMonitorEvent)
	if topic == "" {
		topic = "wallet.address.monitor.events"
	}

	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V3_0_0_0
	saramaCfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaCfg.Consumer.Group.Session.Timeout = time.Duration(cfg.KafkaConsumer.SessionTimeout) * time.Second
	saramaCfg.Consumer.Group.Rebalance.Timeout = time.Duration(cfg.KafkaConsumer.RebalanceTimeout) * time.Second
	saramaCfg.Consumer.MaxProcessingTime = time.Duration(cfg.KafkaConsumer.MaxProcessingTime) * time.Second

	// Prefer consumer-specific credentials; fallback to producer config.
	security := strings.TrimSpace(cfg.KafkaConsumer.Security)
	if security == "" {
		security = strings.TrimSpace(cfg.Kafka.Security)
	}
	username := strings.TrimSpace(cfg.KafkaConsumer.Username)
	password := strings.TrimSpace(cfg.KafkaConsumer.Password)
	saslMech := strings.TrimSpace(cfg.KafkaConsumer.SASLMech)
	if username == "" {
		username = strings.TrimSpace(cfg.Kafka.Username)
	}
	if password == "" {
		password = strings.TrimSpace(cfg.Kafka.Password)
	}
	if saslMech == "" {
		saslMech = strings.TrimSpace(cfg.Kafka.SASLMech)
	}

	if !strings.EqualFold(security, "PLAINTEXT") {
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.User = username
		saramaCfg.Net.SASL.Password = password
		switch strings.ToUpper(saslMech) {
		case "SCRAM-SHA-256":
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "SCRAM-SHA-512":
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		default:
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
		if strings.EqualFold(security, "SASL_SSL") || strings.EqualFold(security, "SSL") {
			saramaCfg.Net.TLS.Enable = true
		}
	}

	logx.Infof("🚀 Starting address monitor Kafka consumer: brokers=%v group=%s topic=%s", brokers, groupID, topic)

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, saramaCfg)
	if err != nil {
		logx.Errorf("Failed to create Kafka consumer group: %v", err)
		return
	}
	defer func() { _ = consumerGroup.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-am.stopCh
		cancel()
	}()

	handler := &addressEventConsumerGroupHandler{am: am}

	for {
		select {
		case <-ctx.Done():
			logx.Info("Address monitor Kafka consumer stopped")
			return
		default:
			if err := consumerGroup.Consume(ctx, []string{topic}, handler); err != nil {
				logx.Errorf("Address monitor Kafka consumer error: %v", err)
				time.Sleep(2 * time.Second)
			}
		}
	}
}

type addressEventConsumerGroupHandler struct {
	am *AddressMonitor
}

func (h *addressEventConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *addressEventConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *addressEventConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if msg == nil {
			continue
		}

		var evt mq.AddressMonitorEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			logx.Errorf("Address monitor event unmarshal failed: topic=%s partition=%d offset=%d err=%v", msg.Topic, msg.Partition, msg.Offset, err)
			session.MarkMessage(msg, "")
			continue
		}

		if h.am == nil || h.am.registry == nil {
			session.MarkMessage(msg, "")
			continue
		}

		res, err := h.am.registry.ApplyEvent(&evt)
		if err != nil {
			logx.Errorf("Address monitor event apply failed: topic=%s partition=%d offset=%d err=%v payload=%s", msg.Topic, msg.Partition, msg.Offset, err, string(msg.Value))
			session.MarkMessage(msg, "")
			continue
		}

		// Keep logs compact; full refresh remains reconciliation fallback.
		if res.monitoredAdd || res.monitoredDrop {
			logx.Infof("📌 Address monitor event applied: chain=%v addr=%s source=%s action=%s before=%d after=%d",
				res.chain, res.address, res.source, res.action, res.before, res.after)
		} else {
			logx.Debugf("Address monitor event noop: chain=%v addr=%s source=%s action=%s", res.chain, res.address, res.source, res.action)
		}

		session.MarkMessage(msg, "")
	}
	return nil
}

func (am *AddressMonitor) publishAddressMonitorEvent(evt mq.AddressMonitorEvent) error {
	if am == nil || am.kafkaProducer == nil || am.config == nil {
		return fmt.Errorf("kafka producer not available")
	}
	topic := strings.TrimSpace(am.config.Kafka.Topics.AddressMonitorEvent)
	if topic == "" {
		topic = "wallet.address.monitor.events"
	}
	key := fmt.Sprintf("%s:%s:%s", evt.Source, strings.ToUpper(strings.TrimSpace(evt.Chain)), strings.TrimSpace(evt.Address))
	_, err := am.kafkaProducer.SendMessage(topic, key, evt)
	return err
}
