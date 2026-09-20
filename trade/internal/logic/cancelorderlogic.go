package logic

import (
	"context"
	"fmt"
	"time"

	"dex/model/trademodel"
	"dex/pkg/constants"
	tradepkg "dex/pkg/trade"
	"dex/trade/internal/svc"
	"dex/trade/pkg/entity"
	"dex/trade/trade"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// stuckOrderAge is how long a market buy/sell can sit at OrderStatus_Proc
// before it's treated as abandoned and becomes user-cancellable. Devnet
// transactions confirm in seconds; anything still Proc after this either had
// its ConfirmMarketOrder call lost (network blip) or predates that mechanism
// entirely — either way, nothing is ever going to move it out of Proc on its
// own, so it should not be stuck in Open Orders forever with no way out.
const stuckOrderAge = 2 * time.Minute

type CancelOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelOrderLogic {
	return &CancelOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelOrderLogic) CancelOrder(in *trade.CancelOrderRequest) (*trade.CancelOrderResponse, error) {
	orderModel := trademodel.NewTradeOrderModel(l.svcCtx.DB)

	order, err := orderModel.FindOne(l.ctx, in.OrderId)
	if err != nil {
		return nil, fmt.Errorf("order %d not found: %w", in.OrderId, err)
	}

	fromStatus := int64(trade.OrderStatus_Waiting)
	switch order.Status {
	case int64(trade.OrderStatus_Waiting):
	case int64(trade.OrderStatus_Proc):
		if time.Since(order.CreatedAt) < stuckOrderAge {
			return nil, fmt.Errorf("order %d is still being submitted, try again shortly", in.OrderId)
		}
		fromStatus = int64(trade.OrderStatus_Proc)
	default:
		return nil, fmt.Errorf("order %d is not open (status %d), cannot cancel", in.OrderId, order.Status)
	}

	rows, err := orderModel.UpdateOrderStatus(l.ctx, order, fromStatus, int64(trade.OrderStatus_Cancel))
	if err != nil {
		return nil, fmt.Errorf("CancelOrder update err: %w", err)
	}
	if rows == 0 {
		return nil, fmt.Errorf("order %d already changed status, cannot cancel", in.OrderId)
	}

	// The order is only queued for triggering while it's still in Redis's
	// price-trigger list — remove it there too, or a cancelled order could
	// still fire once the price crosses its line.
	if err := l.removeFromRedisTriggerList(order); err != nil {
		l.Errorf("CancelOrder: failed to remove order %d from redis trigger list: %v", order.Id, err)
	}

	return &trade.CancelOrderResponse{}, nil
}

// removeFromRedisTriggerList drops the order from the buy/sell price-trigger
// list it was pushed onto at creation (mirrors the rebuild pattern the
// trigger executor itself uses: no LREM matching, just DEL + RPUSH the rest).
func (l *CancelOrderLogic) removeFromRedisTriggerList(order *trademodel.TradeOrder) error {
	var key string
	switch trade.SwapType(order.SwapType) {
	case trade.SwapType_Buy:
		key = fmt.Sprintf("%v:%v:%v", tradepkg.RedisLimitOrderBuyPrefix, order.TokenCa, order.ChainId)
		if order.ChainId == constants.SolChainIdInt {
			key = fmt.Sprintf("%v:%v", tradepkg.RedisLimitOrderBuyPrefix, order.TokenCa)
		}
	case trade.SwapType_Sell:
		key = fmt.Sprintf("%v:%v:%v", tradepkg.RedisLimitOrderSellPrefix, order.TokenCa, order.ChainId)
		if order.ChainId == constants.SolChainIdInt {
			key = fmt.Sprintf("%v:%v", tradepkg.RedisLimitOrderSellPrefix, order.TokenCa)
		}
	default:
		return fmt.Errorf("invalid swap type: %v", order.SwapType)
	}

	entries, err := l.svcCtx.Redis.LrangeCtx(l.ctx, key, 0, -1)
	if err != nil {
		return err
	}

	kept := make([]string, 0, len(entries))
	for _, e := range entries {
		info, derr := entity.DeserializeRedisTokenPriceLimitOrderInfo(e)
		if derr != nil || info.OrderId == order.Id {
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == len(entries) {
		return nil
	}

	return l.svcCtx.Redis.PipelinedCtx(l.ctx, func(pipeline redis.Pipeliner) error {
		pipeline.Del(l.ctx, key)
		for _, e := range kept {
			pipeline.RPush(l.ctx, key, e)
		}
		return nil
	})
}
