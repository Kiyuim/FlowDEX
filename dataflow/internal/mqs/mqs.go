// Package mqs
// File mqs.go
package mqs

import (
	"context"

	"dex/dataflow/internal/config"
	"dex/dataflow/internal/constants"
	"dex/dataflow/internal/mqs/consumers"
	"dex/dataflow/internal/svc"

	"github.com/zeromicro/go-zero/core/service"
)

func TradeConsumers(c config.Config, ctx context.Context, svcContext *svc.ServiceContext) []service.Service {
	var res []service.Service
	for _, chainId := range constants.ChainIds {
		if chainId == constants.Sol {
			// Create TradeConsumer
			tradeConsumer := consumers.NewTradeConsumer(ctx, svcContext, chainId)

			// Create adapter to bridge Sarama and kafka-go message formats
			adapter := consumers.NewTradeConsumerAdapter(tradeConsumer)

			// Use our custom Sarama-based consumer (same config as working producer)
			saramaConsumer := NewSaramaKafkaConsumer(c.KqSolConf, adapter)

			res = append(res, saramaConsumer)
		}
	}
	return res
}
