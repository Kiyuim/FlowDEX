package logic

import (
	"context"

	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferLogic {
	return &TransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// internal
func (l *TransferLogic) Transfer(in *trade.TransferRequest) (*trade.TransferResponse, error) {
	// todo: add your logic here and delete this line

	return &trade.TransferResponse{}, nil
}
