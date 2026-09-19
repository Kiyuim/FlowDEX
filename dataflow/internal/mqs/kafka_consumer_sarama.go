package mqs

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/chengfield/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

// SaramaKafkaConsumer uses the same configuration pattern as the working producer
type SaramaKafkaConsumer struct {
	config   kq.KqConf
	handler  SaramaConsumerHandler
	consumer sarama.ConsumerGroup
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan bool
}

type SaramaConsumerHandler interface {
	Consume(ctx context.Context, message sarama.ConsumerMessage) error
}

// ConsumerGroupHandler implements sarama.ConsumerGroupHandler
type SaramaConsumerGroupHandler struct {
	handler SaramaConsumerHandler
}

func (h *SaramaConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	logx.Info("🔄 Consumer group rebalanced")
	return nil
}

func (h *SaramaConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	logx.Info("🧹 Consumer group cleanup")
	return nil
}

func (h *SaramaConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			ctx := session.Context()
			if err := h.handler.Consume(ctx, *message); err != nil {
				logx.Errorf("❌ Error processing message: %v", err)
				continue
			}

			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

func NewSaramaKafkaConsumer(conf kq.KqConf, handler SaramaConsumerHandler) *SaramaKafkaConsumer {
	ctx, cancel := context.WithCancel(context.Background())

	return &SaramaKafkaConsumer{
		config:  conf,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan bool),
	}
}

func (c *SaramaKafkaConsumer) Start() {
	// Use EXACT same configuration as the working producer
	config := sarama.NewConfig()

	// Network related configurations (same as producer)
	config.Net.DialTimeout = 30 * time.Second
	config.Net.ReadTimeout = 30 * time.Second
	config.Net.WriteTimeout = 30 * time.Second
	config.Net.KeepAlive = 30 * time.Second

	// Consumer specific configurations
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin

	// Retry configurations (same as producer)
	config.Consumer.Retry.Backoff = 100 * time.Millisecond

	// CRITICAL: Use EXACT same TLS/SASL config as working producer
	config.Net.TLS.Enable = false // ← Same as producer
	// Don't set TLS.Config when TLS is disabled

	config.Net.MaxOpenRequests = 1024
	config.ChannelBufferSize = 256

	// SASL configurations (EXACT same as producer)
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	config.Net.SASL.User = c.config.Username
	config.Net.SASL.Password = c.config.Password

	config.ClientID = "dataflow-consumer"

	// Enable logging (same as producer)
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	// Create consumer group
	consumer, err := sarama.NewConsumerGroup(c.config.Brokers, c.config.Group, config)
	if err != nil {
		logx.Errorf("❌ Failed to create consumer group: %v", err)
		return
	}
	c.consumer = consumer

	// Handle consumer errors
	go func() {
		for err := range consumer.Errors() {
			logx.Errorf("❌ Consumer error: %v", err)
		}
	}()

	// Start consuming
	handler := &SaramaConsumerGroupHandler{handler: c.handler}

	go func() {
		defer close(c.done)
		for {
			if err := consumer.Consume(c.ctx, []string{c.config.Topic}, handler); err != nil {
				logx.Errorf("❌ Error from consumer: %v", err)
				return
			}
			if c.ctx.Err() != nil {
				return
			}
		}
	}()

	logx.Infof("🚀 Sarama Kafka consumer started - Topic: %s, Group: %s", c.config.Topic, c.config.Group)
	logx.Info("✅ Using same TLS configuration as working producer")
}

func (c *SaramaKafkaConsumer) Stop() {
	logx.Info("🛑 Stopping Sarama Kafka consumer...")

	if c.cancel != nil {
		c.cancel()
	}

	if c.consumer != nil {
		if err := c.consumer.Close(); err != nil {
			logx.Errorf("❌ Error closing consumer: %v", err)
		}
	}

	// Wait for consumer to finish
	<-c.done

	logx.Info("✅ Sarama Kafka consumer stopped")
}
