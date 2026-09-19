package consumers

import (
	"context"
	"dex/market/internal/config"
	"dex/market/internal/constants"
	"dex/market/internal/svc"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

// KafkaGoConsumer 使用 kafka-go 库直接实现的 Consumer
// 绕过 go-queue/kq 库的 TLS 限制
type KafkaGoConsumer struct {
	reader  *kafka.Reader
	handler *TradeConsumer
	ctx     context.Context
	cancel  context.CancelFunc
	svcCtx  *svc.ServiceContext
	config  config.Config
	chainId constants.ChainId
}

// NewKafkaGoConsumer 创建新的 Kafka Consumer（使用 kafka-go 库直接实现）
func NewKafkaGoConsumer(c config.Config, svcCtx *svc.ServiceContext, chainId constants.ChainId) service.Service {
	ctx, cancel := context.WithCancel(context.Background())

	// 配置 SASL
	mechanism := plain.Mechanism{
		Username: c.KqSolTrades.Username,
		Password: c.KqSolTrades.Password,
	}

	// 创建 dialer - 不设置 TLS（因为 broker 使用 SASL_PLAINTEXT）
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: mechanism,
		// 注意：不设置 TLS，因为 broker 使用 SASL_PLAINTEXT
	}

	// 确定 offset
	var startOffset int64
	if c.KqSolTrades.Offset == "first" {
		startOffset = kafka.FirstOffset
	} else {
		startOffset = kafka.LastOffset
	}

	// 创建 Reader
	readerConfig := kafka.ReaderConfig{
		Brokers:     c.KqSolTrades.Brokers,
		Topic:       c.KqSolTrades.Topic,
		GroupID:     c.KqSolTrades.Group,
		StartOffset: startOffset,
		MinBytes:    c.KqSolTrades.MinBytes,
		MaxBytes:    c.KqSolTrades.MaxBytes,
		Dialer:      dialer,
	}

	reader := kafka.NewReader(readerConfig)

	// 创建 TradeConsumer handler
	handler := NewTradeConsumer(ctx, svcCtx, chainId)

	consumer := &KafkaGoConsumer{
		reader:  reader,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
		svcCtx:  svcCtx,
		config:  c,
		chainId: chainId,
	}

	fmt.Printf("✅ KafkaGoConsumer created: topic=%s, group=%s\n", c.KqSolTrades.Topic, c.KqSolTrades.Group)
	return consumer
}

// Start 实现 service.Service 接口
func (k *KafkaGoConsumer) Start() {
	logx.Infof("🚀 Starting KafkaGoConsumer: topic=%s, group=%s", k.config.KqSolTrades.Topic, k.config.KqSolTrades.Group)

	// 先测试连接
	if err := k.testConnection(); err != nil {
		logx.Errorf("❌ Failed to connect to Kafka broker: %v", err)
		logx.Errorf("💡 Please check:")
		logx.Errorf("   1. Kafka broker is running: %v", k.config.KqSolTrades.Brokers)
		logx.Errorf("   2. Network connectivity")
		logx.Errorf("   3. SASL credentials are correct")
		return
	}
	logx.Infof("✅ Successfully connected to Kafka broker")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("❌ PANIC in KafkaGoConsumer: %v", r)
			}
		}()

		for {
			select {
			case <-k.ctx.Done():
				logx.Infof("KafkaGoConsumer context cancelled, stopping...")
				return
			default:
				// 读取消息（带超时，避免阻塞）
				ctx, cancel := context.WithTimeout(k.ctx, 5*time.Second)
				msg, err := k.reader.ReadMessage(ctx)
				cancel()

				if err != nil {
					if err == context.DeadlineExceeded || err == context.Canceled {
						// 超时或取消，继续循环
						continue
					}
					logx.Errorf("❌ Error reading Kafka message: %v", err)
					time.Sleep(1 * time.Second) // 等待后重试
					continue
				}

				// 处理消息
				if err := k.handler.Consume(k.ctx, msg); err != nil {
					logx.Errorf("❌ Error processing message: %v", err)
					// 继续处理下一条消息
				}
			}
		}
	}()

	logx.Infof("✅ KafkaGoConsumer started successfully")
}

// testConnection 测试 Kafka broker 连接
func (k *KafkaGoConsumer) testConnection() error {
	// 配置 SASL
	mechanism := plain.Mechanism{
		Username: k.config.KqSolTrades.Username,
		Password: k.config.KqSolTrades.Password,
	}

	// 创建 dialer - 不设置 TLS
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: mechanism,
	}

	// 尝试连接第一个 broker
	if len(k.config.KqSolTrades.Brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}

	broker := k.config.KqSolTrades.Brokers[0]
	conn, err := dialer.DialContext(context.Background(), "tcp", broker)
	if err != nil {
		return fmt.Errorf("failed to dial broker %s: %w", broker, err)
	}
	defer conn.Close()

	// 测试获取 controller
	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}
	logx.Infof("✅ Connected to Kafka broker, controller: %s:%d", controller.Host, controller.Port)

	return nil
}

// Stop 实现 service.Service 接口
func (k *KafkaGoConsumer) Stop() {
	logx.Infof("🛑 Stopping KafkaGoConsumer...")
	k.cancel()
	if err := k.reader.Close(); err != nil {
		logx.Errorf("❌ Error closing Kafka reader: %v", err)
	}
	logx.Infof("✅ KafkaGoConsumer stopped")
}
