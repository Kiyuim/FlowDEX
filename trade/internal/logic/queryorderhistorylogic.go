package logic

import (
	"context"

	"dex/model/trademodel"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryOrderHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryOrderHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryOrderHistoryLogic {
	return &QueryOrderHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QueryOrderHistory lists every order for a token, any status, newest first.
func (l *QueryOrderHistoryLogic) QueryOrderHistory(in *trade.QueryOrderHistoryRequest) (*trade.QueryOrderHistoryResponse, error) {
	orderModel := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	orders, _, err := orderModel.FindOrderHistory(l.ctx, in.ChainId, in.TokenCa, int64(in.TradeType), int64(in.SwapType), in.PageNo, in.PageSize)
	if err != nil {
		return nil, err
	}
	return &trade.QueryOrderHistoryResponse{
		List: ordersToInfo(l.ctx, l.svcCtx, in.ChainId, in.TokenCa, orders),
	}, nil
}
