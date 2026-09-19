package logic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &trade.QueryCurrentOrdersResponse{}, nil
}
