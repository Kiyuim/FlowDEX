package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryTradeHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryTradeHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryTradeHistoryLogic {
	return &QueryTradeHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *QueryTradeHistoryLogic) QueryTradeHistory(in *trade.QueryTradeHistoryRequest) (*trade.QueryTradeHistoryResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.QueryTradeHistoryResponse{}, nil
}
