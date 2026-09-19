package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProcTokenCapLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProcTokenCapLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProcTokenCapLogic {
	return &ProcTokenCapLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProcTokenCapLogic) ProcTokenCap(in *trade.ProcTokenCapRequest) (*trade.ProcTokenCapResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.ProcTokenCapResponse{}, nil
}
