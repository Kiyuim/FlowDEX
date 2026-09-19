package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryTransferLogic {
	return &QueryTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *QueryTransferLogic) QueryTransfer(in *trade.QueryTransferRequest) (*trade.QueryTransferResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.QueryTransferResponse{}, nil
}
