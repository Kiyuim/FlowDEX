package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryHoldingTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryHoldingTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryHoldingTokenLogic {
	return &QueryHoldingTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *QueryHoldingTokenLogic) QueryHoldingToken(in *trade.QueryHoldTokenRequest) (*trade.QueryHoldTokenResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.QueryHoldTokenResponse{}, nil
}
