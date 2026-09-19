// Package mqs
// File mqs.go
package mqs

import (
	"context"
	"dex/market/internal/config"
	"dex/market/internal/svc"

	"github.com/zeromicro/go-zero/core/service"
)

// TradeConsumers used to start a second Kafka consumer group on the trade
// topic (consumers.NewKafkaGoConsumer / TradeConsumer, in this same package)
// purely for market to build its own K-line cache. That handler only ever
// logged messages and threw them away (its body ended in `// TODO: 根据交易数据生成
// K 线并更新缓存`), while dataflow already owns real K-line generation end-to-end
// (writes MySQL + Redis that market reads via cache.KlineRedisCache). It cost
// market an extra Kafka consumer group and connection for zero output, so it's
// disabled here rather than left running as dead weight. See
// docs/项目已知问题与修复记录.md.
func TradeConsumers(_ config.Config, _ context.Context, _ *svc.ServiceContext) []service.Service {
	return nil
}
