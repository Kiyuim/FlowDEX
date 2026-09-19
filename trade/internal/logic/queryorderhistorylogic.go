package logic

import (
	"context"

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

func (l *QueryOrderHistoryLogic) QueryOrderHistory(in *trade.QueryOrderHistoryRequest) (*trade.QueryOrderHistoryResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.QueryOrderHistoryResponse{}, nil
}
