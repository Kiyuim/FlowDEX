package logic

import (
	"context"
	"fmt"

	"dex/market/market"
	"dex/model/trademodel"
	"dex/pkg/constants"
	tradepkg "dex/pkg/trade"
	"dex/pkg/util"
	"dex/pkg/xcode"
	"dex/pkg/xredis"
	"dex/trade/internal/svc"
	"dex/trade/pkg/entity"
	"dex/trade/trade"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTrailingStopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTrailingStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTrailingStopLogic {
	return &CreateTrailingStopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateTrailingStop places a trailing stop (移动止盈止损) sell order: when the
// price falls N% from the highest price seen after placement, the order is
// executed as a market sell by the trailing-stop price consumer. The trigger
// line only ratchets upward (BasePrice = highest seen), so the same line acts
// as both take-profit and stop-loss.
func (l *CreateTrailingStopLogic) CreateTrailingStop(in *trade.CreateTrailingStopRequest) (*trade.CreateTrailingStopResponse, error) {
	// N outside (0,100) produces a trigger price ≤ 0 or ≥ current price: either
	// a zombie order that never fires or one that fires on placement. Reject at
	// the door — an illegal order in Redis costs every future price tick.
	if in.TrailingPercent <= 0 || in.TrailingPercent >= 100 {
		return nil, xcode.AmountErr
	}

	amountDecimal, err := decimal.NewFromString(in.Amount)
	if err != nil {
		return nil, fmt.Errorf("amount:%s parse err:%s", in.Amount, err.Error())
	}
	if !amountDecimal.IsPositive() {
		return nil, xcode.AmountErr
	}

	pairInfo, err := l.svcCtx.MarketClient.GetPairInfoByToken(l.ctx, &market.GetPairInfoByTokenRequest{
		ChainId:      in.ChainId,
		TokenAddress: in.TokenCa,
	})
	if err != nil {
		return nil, err
	}

	// Same unit convention as the consumer's price ticks (tokenPriceUSD /
	// solPrice): the anchor must be SOL-denominated or the ratchet comparison
	// is meaningless. Devnet pair rows often carry base_token_price=0 — fall
	// back to the nominal SOL price the consumer uses.
	baseTokenPrice := pairInfo.BaseTokenPrice
	if baseTokenPrice <= 0 {
		baseTokenPrice = constants.NominalSolPriceUsd
	}
	basePriceDecimal := decimal.NewFromFloat(baseTokenPrice)

	// The current price is the initial BasePrice (highest-seen anchor). Without
	// it there is nothing to trail — a token that has never traded can't take a
	// trailing stop.
	if pairInfo.TokenPrice <= 0 {
		return nil, fmt.Errorf("token %s has no current price (no trades yet), cannot place trailing stop", in.TokenCa)
	}
	currentPriceBase := decimal.NewFromFloat(pairInfo.TokenPrice).Div(basePriceDecimal)
	drawdownPrice := CalculateDrawDownPrice(currentPriceBase, int(in.TrailingPercent))

	order := &trademodel.TradeOrder{
		ChainId:        in.ChainId,
		TradeType:      int64(trade.TradeType_TrailingStop),
		GasType:        1,
		IsAutoSlippage: util.BoolToInt64(in.IsAutoSlippage),
		Slippage:       1000, // 10%, same as limit orders
		IsAntiMev:      0,
		TokenCa:        in.TokenCa,
		// A trailing stop protects an existing position — it is always a sell.
		SwapType:        int64(trade.SwapType_Sell),
		OrderCap:        decimal.NewFromFloat(pairInfo.Fdv),
		OrderAmount:     amountDecimal,
		OrderPriceBase:  currentPriceBase,
		OrderValueBase:  currentPriceBase.Mul(amountDecimal),
		OrderBasePrice:  basePriceDecimal,
		DrawdownPrice:   drawdownPrice,
		TrailingPercent: int64(in.TrailingPercent),
		Status:          int64(trade.OrderStatus_Waiting),
	}

	if err := l.CreateTrailingStopOrder(order); err != nil {
		return nil, err
	}

	return &trade.CreateTrailingStopResponse{
		OrderId: uint64(order.Id),
	}, nil
}

// CreateTrailingStopOrder is the shared entrance for trailing stop orders —
// hand-placed (RPC above) or auto-attached to a confirmed buy (ticker):
// MySQL insert + trigger note into the per-token Redis list. The caller sets
// OrderPriceBase (anchor) and DrawdownPrice.
func (l *CreateTrailingStopLogic) CreateTrailingStopOrder(order *trademodel.TradeOrder) error {
	model := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	if err := model.InsertWithLog(l.ctx, order); err != nil {
		return fmt.Errorf("CreateTrailingStop Insert err:%s", err.Error())
	}

	if err := l.addOrderToRedis(order); err != nil {
		// No zombie Waiting rows: an order that never reached Redis can never
		// trigger, so flip it to Fail rather than leaving it live in the DB.
		order.Status = int64(trade.OrderStatus_Fail)
		if updErr := model.UpdateOrderBySelect(l.ctx, order, "status"); updErr != nil {
			l.Errorf("CreateTrailingStop mark fail err:%s", updErr.Error())
		}
		return err
	}
	return nil
}

// addOrderToRedis appends the 4-field trigger note to the per-token trailing
// stop list. It takes the same distributed lock as the price consumer: the
// consumer's Del+RPush full rewrite works on a snapshot, and an unlocked RPUSH
// landing between its LRANGE and DEL would be silently erased (the order would
// stay Waiting in MySQL forever but never exist in Redis).
func (l *CreateTrailingStopLogic) addOrderToRedis(order *trademodel.TradeOrder) error {
	key := fmt.Sprintf("%v:%v:%v", tradepkg.RedisTrailingStopPrefix, order.TokenCa, order.ChainId)
	if order.ChainId == constants.SolChainIdInt {
		key = fmt.Sprintf("%v:%v", tradepkg.RedisTrailingStopPrefix, order.TokenCa)
	}

	info := &entity.RedisTrailingStopOrderInfo{
		OrderId:         order.Id,
		BasePrice:       order.OrderPriceBase.String(),
		DrawdownPrice:   order.DrawdownPrice.String(),
		TrailingPercent: int(order.TrailingPercent),
	}
	serializedInfo, err := info.Serialize()
	if err != nil {
		return err
	}

	lockKey := fmt.Sprintf("%v:%v", key, "lock")
	lock, err := xredis.MustLock(l.ctx, l.svcCtx.Redis, lockKey, 10, 10)
	if err != nil {
		return fmt.Errorf("addOrderToRedis MustLock err: %v, lockKey: %v", err, lockKey)
	}
	defer xredis.ReleaseLock(lock)

	_, err = l.svcCtx.Redis.RpushCtx(l.ctx, key, serializedInfo)
	return err
}

// CalculateDrawDownPrice mirrors the consumer's ratchet formula:
// trigger = price × (100 - N) / 100.
func CalculateDrawDownPrice(currentPrice decimal.Decimal, percent int) decimal.Decimal {
	return currentPrice.Mul(decimal.NewFromInt(int64(100 - percent))).Div(decimal.NewFromInt(100))
}
