package logic

import (
	"context"

	"dex/consumer/consumer"
	"dex/consumer/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PingLogic {
	return &PingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Ping
func (l *PingLogic) Ping(in *consumer.PingRequest) (*consumer.PingResponse, error) {
	// todo: add your logic here and delete this line

	return &consumer.PingResponse{}, nil
}
