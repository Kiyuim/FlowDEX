package consumers

import (
	"context"
	"dex/market/internal/constants"
	"dex/market/internal/svc"
	"dex/pkg/types"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/core/logx"
)

type TradeConsumer struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTradeConsumer(ctx context.Context, svcCtx *svc.ServiceContext, chainId constants.ChainId) *TradeConsumer {
	ctx, cancel := context.WithCancel(ctx)
	_ = cancel // 保存 cancel 函数以便后续使用
	c := &TradeConsumer{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx).WithFields(logx.Field("chainId", chainId)),
	}
	fmt.Printf("📦 TradeConsumer created for chainId: %d\n", chainId)
	logx.Infof("📦 TradeConsumer created for chainId: %d", chainId)
	return c
}

// Consume 处理从 Kafka 接收到的交易消息
// Consumer 服务发送的是 []*types.TradeWithPair 格式的 JSON
func (t *TradeConsumer) Consume(ctx context.Context, msg kafka.Message) error {
	fmt.Println("🔥 Consume called! Message received from Kafka")
	logx.Infof("📨 Received Kafka message: topic=%s, partition=%d, offset=%d, key=%s, value_len=%d",
		msg.Topic, msg.Partition, msg.Offset, string(msg.Key), len(msg.Value))
	// 解析 Kafka 消息中的交易数据
	// Consumer 服务发送的是 []*types.TradeWithPair 格式
	var tradeMsg []*types.TradeWithPair
	if err := json.Unmarshal(msg.Value, &tradeMsg); err != nil {
		logc.Errorf(ctx, "failed to unmarshal trade message: %+v", err)
		return err
	}

	if len(tradeMsg) == 0 {
		return nil
	}

	logx.Infof("kafka message offset: %d, partition: %d, trades count: %d",
		msg.Offset, msg.Partition, len(tradeMsg))

	// 处理交易数据
	for _, trade := range tradeMsg {
		if trade == nil {
			logc.Errorf(ctx, "trade message is nil")
			continue
		}

		if trade.TokenPriceUSD == 0 {
			logc.Errorf(ctx, "trade TokenPriceUSD is 0, trade: %+v", trade)
			continue
		}

		logx.Infof("processing trade: pair=%s, price=%.6f, amount=%.6f, time=%d, type=%s",
			trade.PairAddr, trade.TokenPriceUSD, trade.TokenAmount, trade.BlockTime, trade.Type)
	}

	logc.Infof(ctx, "processed %d trades at %v", len(tradeMsg), time.Now().Format(time.DateTime))

	// TODO: 根据交易数据生成 K 线并更新缓存
	// 示例：可以调用 cache 相关的方法来更新 K 线数据
	// 例如：cache.KlineRedisCache.Push(ctx, kline)

	return nil
}
