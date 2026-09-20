package logic

import (
	"context"

	"dex/market/market"
	"dex/model/trademodel"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryCurrentOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryCurrentOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryCurrentOrdersLogic {
	return &QueryCurrentOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *QueryCurrentOrdersLogic) QueryCurrentOrders(in *trade.QueryCurrentOrdersRequest) (*trade.QueryCurrentOrdersResponse, error) {
	orderModel := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	orders, total, err := orderModel.FindOpenOrders(l.ctx, in.ChainId, in.TokenCa, in.TradeType, in.SwapType, in.PageNo, in.PageSize)
	if err != nil {
		return nil, err
	}

	return &trade.QueryCurrentOrdersResponse{
		List:  ordersToInfo(l.ctx, l.svcCtx, in.ChainId, in.TokenCa, orders),
		Total: total,
	}, nil
}

// ordersToInfo maps DB rows to the wire shape shared by the current-orders and
// order-history RPCs. Token symbol is a best-effort lookup — an unindexed pair
// shouldn't hide the orders themselves, so a failure just leaves it blank.
func ordersToInfo(ctx context.Context, svcCtx *svc.ServiceContext, chainId int64, tokenCa string, orders []trademodel.TradeOrder) []*trade.QueryOrderInfo {
	var tokenSymbol string
	if tokenCa != "" {
		if pairInfo, perr := svcCtx.MarketClient.GetPairInfoByToken(ctx, &market.GetPairInfoByTokenRequest{
			ChainId:      chainId,
			TokenAddress: tokenCa,
		}); perr == nil {
			tokenSymbol = pairInfo.TokenSymbol
		}
	}

	list := make([]*trade.QueryOrderInfo, 0, len(orders))
	for _, o := range orders {
		priceUsd := o.OrderPriceBase.Mul(o.OrderBasePrice)
		valueUsd := o.OrderValueBase.Mul(o.OrderBasePrice)
		list = append(list, &trade.QueryOrderInfo{
			Id:              o.Id,
			ChainId:         o.ChainId,
			TokenCa:         o.TokenCa,
			TokenSymbol:     tokenSymbol,
			TradeType:       o.TradeType,
			SwapType:        o.SwapType,
			Cap:             o.OrderCap.String(),
			Amount:          o.OrderAmount.String(),
			Price:           o.OrderPriceBase.String(),
			PriceUsd:        priceUsd.String(),
			Value:           o.OrderValueBase.String(),
			ValueUsd:        valueUsd.String(),
			Status:          o.Status,
			TxHash:          o.TxHash,
			CreateTime:      o.CreatedAt.Unix(),
			UpdateTime:      o.UpdatedAt.Unix(),
			DoubleOut:       o.DoubleOut,
			TrailingPercent: o.TrailingPercent,
		})
	}
	return list
}
