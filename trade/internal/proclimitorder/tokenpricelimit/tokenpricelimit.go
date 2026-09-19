package tokenpricelimit

import (
	"context"
	"dex/model/trademodel"
	"dex/pkg/constants"
	"dex/pkg/transfer"
	"dex/trade/internal/svc"
	"dex/trade/pkg/entity"
	"dex/trade/trade"
	"fmt"

	tradepkg "dex/pkg/trade"

	"dex/pkg/xredis"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logc"
)

type Subscriber struct {
	svcCtx *svc.ServiceContext
	//
}

func NewLimitSubscriber(svcCtx *svc.ServiceContext) *Subscriber {
	subscriber := &Subscriber{
		svcCtx: svcCtx,
	}

	// subscriber.manager = NewProcessorManager(subscriber)
	return subscriber
}

// doConsumer processes the order message
func (s *Subscriber) doConsumer(orderMsg *entity.OrderMessage) {
	// logc.Infof(orderMsg.Ctx, "Processing order - Token: %s, SwapType: %d, Price: %v",
	//	orderMsg.TokenCA, orderMsg.SwapType, orderMsg.CurrentPrice)

	err := s.svcCtx.Pool.Submit(func() {
		swapTypes := []trade.SwapType{trade.SwapType_Buy, trade.SwapType_Sell}
		slice.ForEach[trade.SwapType](swapTypes, func(i int, swapType trade.SwapType) {
			orderMsg.SwapType = int64(swapType)
			if err := s.processTokenPriceLimitOrdersFromRedis(orderMsg); err != nil {
				logc.Error(orderMsg.Ctx, err)
			}
		})
	})
	if err != nil {
		logc.Error(orderMsg.Ctx, err)
	}
}

func (s *Subscriber) Consume(lower, upper int64, buffer []*entity.OrderMessage) {
	// Process messages from the buffer in the specified range (lower to upper)
	for ; lower <= upper; lower++ {
		message := buffer[lower%int64(len(buffer))]
		// logc.Infof(message.Ctx, "token price limit received message: %v at sequence: %d", message, lower)
		s.doConsumer(message)
	}
}

// processTrailingStopOrdersFromRedis processes orders from Redis, For SOL and ETH
func (s *Subscriber) processTokenPriceLimitOrdersFromRedis(in *entity.OrderMessage) error {
	ctx := in.Ctx

	var (
		orders             []*entity.RedisTokenPriceLimitOrderInfo
		delLimitOrderInfos []*entity.RedisTokenPriceLimitOrderInfo
		key                string
		lockKey            string
	)

	switch trade.SwapType(in.SwapType) {
	case trade.SwapType_Buy:
		key = fmt.Sprintf("%v:%v:%v", tradepkg.RedisLimitOrderBuyPrefix, in.TokenCA, in.ChainId)
		if in.ChainId == constants.SolChainIdInt {
			key = fmt.Sprintf("%v:%v", tradepkg.RedisLimitOrderBuyPrefix, in.TokenCA)
		}
	case trade.SwapType_Sell:
		key = fmt.Sprintf("%v:%v:%v", tradepkg.RedisLimitOrderSellPrefix, in.TokenCA, in.ChainId)
		if in.ChainId == constants.SolChainIdInt {
			key = fmt.Sprintf("%v:%v", tradepkg.RedisLimitOrderSellPrefix, in.TokenCA)
		}
	default:
		return fmt.Errorf("invalid swap type: %v", in.SwapType)
	}
	lockKey = fmt.Sprintf("%v:%v", key, "lock")
	lock, err := xredis.MustLock(ctx, s.svcCtx.Redis, lockKey, 10, 10)
	if err != nil {
		logc.Errorf(ctx, "processTokenPriceLimitOrdersFromRedis:MustLock err: %v, lockKey: %v", err, lockKey)
		return err
	}
	defer xredis.ReleaseLock(lock)

	decimalCurrentPrice, err := decimal.NewFromString(in.CurrentPrice)
	if err != nil {
		return err
	}

	switch trade.SwapType(in.SwapType) {
	case trade.SwapType_Buy:
		ordersStr, err := s.svcCtx.Redis.LrangeCtx(ctx, key, 0, -1)
		if err != nil {
			return err
		}

		// Parse and filter valid order information
		orders = slice.FilterMap[string, *entity.RedisTokenPriceLimitOrderInfo](ordersStr, func(_ int, str string) (*entity.RedisTokenPriceLimitOrderInfo, bool) {
			priceLimitOrderInfo, err := transfer.String2Struct[*entity.RedisTokenPriceLimitOrderInfo](str)
			if err != nil {
				return nil, false
			}
			return priceLimitOrderInfo, true
		})
		if len(orders) == 0 {
			return nil
		}
		logc.Debugf(ctx, "in:%#v, orders:%s len is %d", in, key, len(orders))

		delLimitOrderInfos = slice.Filter[*entity.RedisTokenPriceLimitOrderInfo](orders, func(_ int, info *entity.RedisTokenPriceLimitOrderInfo) bool {
			decimalBasePrice, err := decimal.NewFromString(info.BasePrice)
			if err != nil {
				return false
			}
			if decimalBasePrice.GreaterThanOrEqual(decimalCurrentPrice) {
				return true
			}
			return false
		})
		if len(delLimitOrderInfos) == 0 {
			return nil
		}
		for _, v := range delLimitOrderInfos {
			logc.Debugf(ctx, "delLimitOrderInfos:%s is %#v, currentPrice:%v", key, v, decimalCurrentPrice.String())
		}

		slice.ForEach[*entity.RedisTokenPriceLimitOrderInfo](delLimitOrderInfos, func(_ int, info *entity.RedisTokenPriceLimitOrderInfo) {
			err := s.executeLimitTokenPriceOrder(ctx, info.OrderId)
			if err != nil {
				logc.Errorf(ctx, "failed to execute limit token price order: %v, order id: %v", err, info.OrderId)
			}
			if err == nil {
				logc.Infof(ctx, "limit token price order executed: order id: %d", info.OrderId)
			}
		})

	case trade.SwapType_Sell:
		ordersStr, err := s.svcCtx.Redis.LrangeCtx(ctx, key, 0, -1)
		if err != nil {
			return err
		}

		// Parse and filter valid order information
		orders = slice.FilterMap[string, *entity.RedisTokenPriceLimitOrderInfo](ordersStr, func(_ int, str string) (*entity.RedisTokenPriceLimitOrderInfo, bool) {
			priceLimitOrderInfo, err := transfer.String2Struct[*entity.RedisTokenPriceLimitOrderInfo](str)
			if err != nil {
				return nil, false
			}
			return priceLimitOrderInfo, true
		})
		if len(orders) == 0 {
			return nil
		}
		logc.Debugf(ctx, "in:%#v orders:%s len is %d", in, key, len(orders))

		delLimitOrderInfos = slice.Filter[*entity.RedisTokenPriceLimitOrderInfo](orders, func(_ int, info *entity.RedisTokenPriceLimitOrderInfo) bool {
			decimalBasePrice, err := decimal.NewFromString(info.BasePrice)
			if err != nil {
				return false
			}
			if decimalBasePrice.LessThanOrEqual(decimalCurrentPrice) {
				return true
			}
			return false
		})
		if len(delLimitOrderInfos) == 0 {
			return nil
		}
		for _, v := range delLimitOrderInfos {
			logc.Debugf(ctx, "delLimitOrderInfos:%s is %#v, currentPrice:%v", key, v, decimalCurrentPrice.String())
		}

		slice.ForEach[*entity.RedisTokenPriceLimitOrderInfo](delLimitOrderInfos, func(_ int, info *entity.RedisTokenPriceLimitOrderInfo) {
			err := s.executeLimitTokenPriceOrder(ctx, info.OrderId)
			if err != nil {
				logc.Errorf(ctx, "failed to execute limit token price order error: %v, order id: %v", err, info.OrderId)
			}
			if err == nil {
				logc.Infof(ctx, "limit token price order executed: order id: %d", info.OrderId)
			}
		})

	default:
		return fmt.Errorf("invalid swap type: %v", in.SwapType)
	}

	for _, info := range delLimitOrderInfos {
		logc.Infof(ctx, "execute token price limit order id: %v", info.OrderId)
	}

	if len(delLimitOrderInfos) > 0 {
		_ = s.svcCtx.Redis.PipelinedCtx(ctx, func(pipeline redis.Pipeliner) error {
			pipeline.Del(ctx, key)
			for _, info := range slice.Difference(orders, delLimitOrderInfos) {
				infoStr, _ := info.Serialize()
				pipeline.RPush(ctx, key, infoStr)
			}
			return nil
		})
	}

	return nil
}

func (s *Subscriber) executeLimitTokenPriceOrder(ctx context.Context, orderId int64) error {
	model := trademodel.NewTradeOrderModel(s.svcCtx.DB)

	order, err := model.FindOne(ctx, orderId)
	if err != nil {
		return err
	}

	if slice.Contain[trade.OrderStatus]([]trade.OrderStatus{
		trade.OrderStatus_Cancel,
		trade.OrderStatus_Fail,
		trade.OrderStatus_Suc,
		trade.OrderStatus_FailTimeout,
	}, trade.OrderStatus(order.Status)) {
		return nil
	}

	rowsAffected, err := model.UpdateOrderStatus(ctx, order,
		int64(trade.OrderStatus_Waiting),
		int64(trade.OrderStatus_Proc))
	if err != nil || rowsAffected == 0 {
		return fmt.Errorf("executeLimitTokenPriceOrder:update order status error: %v, order id: %v", err, order.Id)
	}

	return nil
}
