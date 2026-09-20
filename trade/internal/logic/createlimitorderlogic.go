package logic

import (
	"context"
	"fmt"

	"dex/market/market"
	"dex/pkg/constants"
	"dex/pkg/util"
	"dex/pkg/xcode"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	"dex/model/trademodel"

	tradepkg "dex/pkg/trade"

	"dex/trade/pkg/entity"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateLimitOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateLimitOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLimitOrderLogic {
	return &CreateLimitOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateLimitOrderLogic) CreateLimitOrder(in *trade.CreateLimitOrderRequest) (*trade.CreateLimitOrderResponse, error) {
	// Validate swap type, must be either Buy or Sell
	var (
		pairInfo *market.GetPairInfoByTokenResponse
		err      error
	)

	if in.SwapType != trade.SwapType_Buy && in.SwapType != trade.SwapType_Sell {
		return nil, fmt.Errorf("invalid swap type: %v", in.SwapType)
	}

	// 如果是买单，数量就是Sol的数量，如果是卖单，数量就是MeMe的数量
	amountDecimal, err := decimal.NewFromString(in.Amount)
	if err != nil {
		return nil, fmt.Errorf("amount:%s parse err:%s", in.Amount, err.Error())
	}
	if !amountDecimal.IsPositive() {
		return nil, xcode.AmountErr
	}

	if pairInfo, err = l.svcCtx.MarketClient.GetPairInfoByToken(l.ctx, &market.GetPairInfoByTokenRequest{
		ChainId:      in.ChainId,
		TokenAddress: in.TokenCa,
	}); err != nil {
		return nil, err
	}

	// Base对U的价格。Devnet pair rows often carry base_token_price=0 (no SOL/USDC
	// pool to derive it from); fall back to the same nominal price the consumer
	// uses when pushing trigger prices, so both sides of the comparison stay in
	// the same unit. Also guards the Div below against a zero divisor.
	baseTokenPrice := pairInfo.BaseTokenPrice
	if baseTokenPrice <= 0 {
		baseTokenPrice = constants.NominalSolPriceUsd
	}
	basePriceDecimal := decimal.NewFromFloat(baseTokenPrice)

	// 流通市值
	capDecimal := decimal.NewFromFloat(pairInfo.Fdv)
	order := &trademodel.TradeOrder{
		ChainId:        in.GetChainId(),
		TradeType:      int64(trade.TradeType_Limit),
		GasType:        1,
		IsAutoSlippage: util.BoolToInt64(in.IsAutoSlippage),
		Slippage:       1000, // 10% slippage instead of 0.1%
		IsAntiMev:      0,
		TokenCa:        in.TokenCa,
		SwapType:       int64(in.SwapType),
		OrderCap:       capDecimal,
		OrderAmount:    amountDecimal,
		OrderBasePrice: basePriceDecimal,
		Status:         int64(trade.OrderStatus_Waiting),
		DoubleOut:      util.BoolToInt64(in.DoubleOut),
	}

	// 按照价格挂单
	if len(in.TokenCap) == 0 && len(in.PriceUsd) > 0 {
		// in.PriceUsd 前端传入的价格，单位是USD,买单：MeMe对U的价格，卖单：MeMe对U的价格 需要转换为Base的价格
		priceUsdDecimal, err := decimal.NewFromString(in.PriceUsd)
		if err != nil {
			return nil, fmt.Errorf("price:%s parse err:%s", in.PriceUsd, err.Error())
		}

		// 转换为MeMe币对Base价格
		priceBaseDecimal := priceUsdDecimal.Div(basePriceDecimal)

		// 如果是买单，传入的数量是需要买多少sol, 就是挂单总价
		order.OrderValueBase = amountDecimal

		// 如果是卖单，传入的数量是MeMe的数量，需要x MeMe的价格
		if in.SwapType == trade.SwapType_Sell {
			order.OrderValueBase = priceBaseDecimal.Mul(amountDecimal)
		}

		order.OrderPriceBase = priceBaseDecimal
	}

	// 按照市值挂单 TODO: 市值 = 代币总共的发行量*代币价格
	if len(in.TokenCap) > 0 {
		// 市值反推价格需要真实的 Fdv 和 TokenPrice，缺一不可
		if pairInfo.Fdv == 0 || pairInfo.TokenPrice == 0 {
			return nil, fmt.Errorf("%s pairInfo %s fdv/token_price is 0, cannot place cap order", pairInfo.Name, pairInfo.Address)
		}
		tokenCapDecimal, err := decimal.NewFromString(in.TokenCap)
		if err != nil {
			return nil, fmt.Errorf("price:%s parse err:%s", in.TokenCap, err.Error())
		}
		// 计算token 总供应量
		totalSupply := decimal.NewFromFloat(pairInfo.Fdv).Div(decimal.NewFromFloat(pairInfo.TokenPrice))
		// 当时计算出来的价格
		priceUsdDecimal := tokenCapDecimal.Div(totalSupply)
		// 转换为MeMe币对Base价格
		priceBaseDecimal := priceUsdDecimal.Div(basePriceDecimal)

		order.TradeType = int64(trade.TradeType_TokenCapLimit)

		order.OrderValueBase = amountDecimal

		if in.SwapType == trade.SwapType_Sell {
			order.OrderValueBase = priceBaseDecimal.Mul(amountDecimal)
		}

		order.OrderPriceBase = priceBaseDecimal
	}

	if err := l.CreateTradeOrder(order); err != nil {
		return nil, err
	}

	return &trade.CreateLimitOrderResponse{
		OrderId: uint64(order.Id),
	}, nil
}

// CreateTradeOrder 创建订单记录
func (l *CreateLimitOrderLogic) CreateTradeOrder(order *trademodel.TradeOrder) error {
	// 插入订单记录
	model := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	if err := model.InsertWithLog(l.ctx, order); err != nil {
		return fmt.Errorf("CreateLimitOrder Insert err:%s", err.Error())
	}

	switch order.TradeType {
	// 普通限价单 市值
	case int64(trade.TradeType_Limit), int64(trade.TradeType_TokenCapLimit):
		if err := l.addOrderToRedis(order); err != nil {
			// 更新订单状态为失败
			order.Status = int64(trade.OrderStatus_Fail)
			if err := model.UpdateOrderBySelect(l.ctx, order, "status"); err != nil {
				l.Errorf("addOrderToRedis err:&s", err.Error())
				return err
			}
			return err
		}
	default:
		return fmt.Errorf("err tradetype:%v", order.TradeType)
	}

	return nil
}

func (l *CreateLimitOrderLogic) addOrderToRedis(order *trademodel.TradeOrder) error {

	var (
		key string
	)

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
	}

	info := &entity.RedisTokenPriceLimitOrderInfo{
		OrderId:   order.Id,
		BasePrice: order.OrderPriceBase.String(),
	}

	serializedInfo, err := info.Serialize()
	if err != nil {
		return err
	}
	_, err = l.svcCtx.Redis.RpushCtx(l.ctx, key, serializedInfo)
	if err != nil {
		return err
	}
	return err
}
