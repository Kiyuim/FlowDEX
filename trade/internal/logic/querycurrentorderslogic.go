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

	// Best-effort token symbol/icon — an unindexed pair shouldn't hide the
	// order itself, so a lookup failure just leaves these blank.
	var tokenSymbol, tokenIcon string
	if in.TokenCa != "" {
		if pairInfo, perr := l.svcCtx.MarketClient.GetPairInfoByToken(l.ctx, &market.GetPairInfoByTokenRequest{
			ChainId:      in.ChainId,
			TokenAddress: in.TokenCa,
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
			TokenIcon:       tokenIcon,
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

	return &trade.QueryCurrentOrdersResponse{
		List:  list,
		Total: total,
	}, nil
}
