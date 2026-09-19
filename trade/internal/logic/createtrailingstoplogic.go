package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTrailingStopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTrailingStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTrailingStopLogic {
	return &CreateTrailingStopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateTrailingStopLogic) CreateTrailingStop(in *trade.CreateTrailingStopRequest) (*trade.CreateTrailingStopResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.CreateTrailingStopResponse{}, nil
}
