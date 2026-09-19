package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProcTrailingStopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProcTrailingStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProcTrailingStopLogic {
	return &ProcTrailingStopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProcTrailingStopLogic) ProcTrailingStop(in *trade.ProcTokenPriceRequest) (*trade.ProcTokenPriceResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.ProcTokenPriceResponse{}, nil
}
