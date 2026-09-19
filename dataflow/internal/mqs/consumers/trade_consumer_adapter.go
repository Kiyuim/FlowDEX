package consumers

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/segmentio/kafka-go"
)

// TradeConsumerAdapter adapts TradeConsumer to work with Sarama messages
type TradeConsumerAdapter struct {
	tradeConsumer *TradeConsumer
}

func NewTradeConsumerAdapter(tradeConsumer *TradeConsumer) *TradeConsumerAdapter {
	return &TradeConsumerAdapter{
		tradeConsumer: tradeConsumer,
	}
}

// Consume implements SaramaConsumerHandler interface
func (adapter *TradeConsumerAdapter) Consume(ctx context.Context, message sarama.ConsumerMessage) error {
	// Convert Sarama message to kafka-go message format (what TradeConsumer expects)
	kafkaGoMessage := kafka.Message{
		Topic:     message.Topic,
		Partition: int(message.Partition),
		Offset:    message.Offset,
		Key:       message.Key,
		Value:     message.Value,
		Time:      message.Timestamp,
	}

	// Call the existing TradeConsumer.Consume method
	return adapter.tradeConsumer.Consume(ctx, kafkaGoMessage)
}
