package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGasInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGasInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGasInfoLogic {
	return &GetGasInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGasInfoLogic) GetGasInfo(in *trade.GetGasInfoRequest) (*trade.GetGasInfoResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.GetGasInfoResponse{}, nil
}
