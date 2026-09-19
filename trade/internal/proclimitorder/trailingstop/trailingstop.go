package trailingstop

import (
	"context"
	"dex/pkg/xredis"
	"fmt"

	"dex/trade/internal/logic"

	"dex/trade/internal/svc"
	"dex/trade/pkg/entity"
	"dex/trade/trade"

	"dex/model/trademodel"
	"dex/pkg/constants"
	tradepkg "dex/pkg/trade"
	"dex/pkg/transfer"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Subscriber struct {
	svcCtx *svc.ServiceContext
	// manager *ProcessorManager

}

func NewTrailingStopSubscriber(svcCtx *svc.ServiceContext) *Subscriber {
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
		if err := s.processTrailingStopOrdersFromRedis(orderMsg); err != nil {
			logc.Error(orderMsg.Ctx, err)
		}
	})
	if err != nil {
		logc.Error(orderMsg.Ctx, err)
	}

	// processor := s.manager.getOrCreateProcessor(orderMsg.TokenCA)

	// select {
	// case processor.orderChan <- orderMsg:
	//	logc.Infof(orderMsg.Ctx, "Successfully sent trailing order to processor for token: %s, order chain size: %v", orderMsg.TokenCA, len(processor.orderChan))
	// case <-time.After(time.Second): // Add timeout mechanism
	//	logc.Errorf(orderMsg.Ctx, "Processor channel full for token: %s, message dropped after timeout", orderMsg.TokenCA)
	// }
}

func (s *Subscriber) executeTrailingStopOrder(ctx context.Context, orderId int64) error {
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
		return fmt.Errorf("executeTrailingStopOrder:update order status error: %v, order id: %v", err, order.Id)
	}

	// Execute market order
	txHash, err := logic.NewCreateMarketOrderLogic(ctx, s.svcCtx).CreateMarketTx(order, nil)
	if err != nil {
		logc.Errorf(ctx, "executeTrailingStopOrder:failed to execute market order ID:%d, token:%s, err:%v",
			order.Id, order.TokenCa, err)
		// failed ws
		// if err = pkg.WsPushOrder(ctx, order, s.svcCtx.MarketClient, s.svcCtx.WebsocketClient); err != nil {
		// 	logc.Errorf(ctx, "executeTrailingStopOrder:PushTradeOrder error %v, order: %#v", err, order)
		// }
		return err
	}

	// onChain ws
	// if err = pkg.WsPushOrder(ctx, order, s.svcCtx.MarketClient, s.svcCtx.WebsocketClient); err != nil {
	// 	logc.Errorf(ctx, "executeTrailingStopOrder:PushTradeOrder error %v, order: %#v", err, order)
	// }

	logc.Infof(ctx, "executeTrailingStopOrder:trailing stop order executed: id=%d, token=%s txHash=%s, order: %#v",
		order.Id, order.TokenCa, txHash, order)

	return nil
}

func (s *Subscriber) Consume(lower, upper int64, buffer []*entity.OrderMessage) {
	// Process messages from the buffer in the specified range (lower to upper)
	for ; lower <= upper; lower++ {
		message := buffer[lower%int64(len(buffer))]
		s.doConsumer(message)
	}
}

// processTrailingStopOrdersFromRedis processes orders from Redis For SOL and ETH
func (s *Subscriber) processTrailingStopOrdersFromRedis(msg *entity.OrderMessage) error {
	ctx := msg.Ctx
	tokenCA := msg.TokenCA
	currentPrice := msg.CurrentPrice
	key := fmt.Sprintf("%v:%v:%v", tradepkg.RedisTrailingStopPrefix, tokenCA, msg.ChainId)

	logc.Infof(ctx, "processTrailingStopOrdersFromRedis:key:%v,price: %v", key, currentPrice)

	if msg.ChainId == constants.SolChainIdInt {
		key = fmt.Sprintf("%v:%v", tradepkg.RedisTrailingStopPrefix, tokenCA)
	}

	lockKey := fmt.Sprintf("%v:%v", key, "lock")
	lock, err := xredis.MustLock(ctx, s.svcCtx.Redis, lockKey, 10, 10)
	if err != nil {
		logc.Errorf(ctx, "processTrailingStopOrdersFromRedis:MustLock err: %v, lockKey: %v", err, lockKey)
		return err
	}
	defer xredis.ReleaseLock(lock)

	infos, err := s.svcCtx.Redis.LrangeCtx(ctx, key, 0, -1)
	if err != nil {
		logc.Errorf(ctx, "get redis key error: %v,key : %#v", err, key)
		return err
	}

	stopOrderInfos := slice.FilterMap[string, *entity.RedisTrailingStopOrderInfo](infos, func(_ int, item string) (*entity.RedisTrailingStopOrderInfo, bool) {
		trailingStopOrderInfo, err := transfer.String2Struct[*entity.RedisTrailingStopOrderInfo](item)
		if err != nil {
			logc.Errorf(ctx, "transfer error: %v,value : %#v", err, item)
			return nil, false
		}
		return trailingStopOrderInfo, true
	})
	if len(stopOrderInfos) == 0 {
		return nil
	}
	logc.Debugf(ctx, "stopOrderInfos:%s len is %d, infos: %v", key, len(stopOrderInfos), infos)

	decimalCurrentPrice, err := decimal.NewFromString(currentPrice)
	if err != nil {
		logc.Errorf(ctx, "decimal error: %v,value : %#v,msg: %#v", err, currentPrice, msg)
		return err
	}

	// Get orders with trigger price less than current price - these orders don't need to be triggered
	// but may need price updates
	graterTrailingStopOrderInfos := slice.Filter[*entity.RedisTrailingStopOrderInfo](stopOrderInfos, func(_ int, item *entity.RedisTrailingStopOrderInfo) bool {
		drawdownPrice, err := decimal.NewFromString(item.DrawdownPrice)
		if err != nil {
			logc.Errorf(ctx, "decimal drawdownPrice error: %v,item : %#v", err, item)
			return false
		}
		if drawdownPrice.LessThan(decimalCurrentPrice) {
			return true
		}
		return false
	})

	slice.ForEach[*entity.RedisTrailingStopOrderInfo](graterTrailingStopOrderInfos, func(_ int, info *entity.RedisTrailingStopOrderInfo) {
		basePrice, err := decimal.NewFromString(info.BasePrice)
		if err != nil {
			logc.Errorf(ctx, "decimal basePrice error: %v,item : %#v", err, info)
			return
		}
		if decimalCurrentPrice.GreaterThan(basePrice) {
			newDrawDownPrice := calculateNewDrawDownPrice(decimalCurrentPrice, info.TrailingPercent)
			info.BasePrice = decimalCurrentPrice.String()
			info.DrawdownPrice = newDrawDownPrice.String()
		}
	})

	// Get orders that need to be triggered as market orders
	lessTrailingStopOrderInfos := slice.Filter[*entity.RedisTrailingStopOrderInfo](stopOrderInfos, func(_ int, item *entity.RedisTrailingStopOrderInfo) bool {
		drawdownPrice, err := decimal.NewFromString(item.DrawdownPrice)
		if err != nil {
			logc.Errorf(ctx, "decimal drawdownPrice error: %v,item : %#v", err, item)
			return false
		}
		if drawdownPrice.GreaterThanOrEqual(decimalCurrentPrice) {
			return true
		}
		return false
	})
	if len(lessTrailingStopOrderInfos) == 0 {
		return nil
	}
	for _, v := range lessTrailingStopOrderInfos {
		logc.Debugf(ctx, "lessTrailingStopOrderInfos:%s is %#v, currentPrice:%v", key, v, currentPrice)
	}

	slice.ForEach[*entity.RedisTrailingStopOrderInfo](lessTrailingStopOrderInfos, func(_ int, info *entity.RedisTrailingStopOrderInfo) {
		err := s.executeTrailingStopOrder(ctx, info.OrderId)
		if err != nil {
			logc.Errorf(ctx, "failed to execute trailing stop order error: %v, order id: %v", err, info.OrderId)
		}
		if err == nil {
			logc.Infof(ctx, "trailing stop order executed: order id: %d", info.OrderId)
		}
	})

	slice.SortBy[*entity.RedisTrailingStopOrderInfo](graterTrailingStopOrderInfos, func(a, b *entity.RedisTrailingStopOrderInfo) bool {
		return a.TrailingPercent < b.TrailingPercent
	})

	err = s.svcCtx.Redis.PipelinedCtx(ctx, func(pipeline redis.Pipeliner) error {
		pipeline.Del(ctx, key)
		for _, info := range graterTrailingStopOrderInfos {
			infoStr, _ := info.Serialize()
			pipeline.RPush(ctx, key, infoStr)
		}
		return nil
	})

	return err
}

// calculateNewDrawDownPrice calculates the new drawdown price based on current price and trailing percentage
func calculateNewDrawDownPrice(currentPrice decimal.Decimal, percent int) decimal.Decimal {
	return currentPrice.Mul(decimal.NewFromInt(int64(100 - percent))).Div(decimal.NewFromInt(100))
}
