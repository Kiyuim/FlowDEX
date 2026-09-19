package logic

import (
	"context"
	"dex/trade/internal/svc"
	"dex/trade/trade"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestRpcLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTestRpcLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestRpcLogic {
	return &TestRpcLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TestRpcLogic) TestRpc(in *trade.TestRpcRequest) (*trade.TestRpcResponse, error) {
	// 记录请求信息
	l.Infof("Received TestRpc request with message: %s, value: %d", in.Message, in.Value)

	// 创建响应
	resp := &trade.TestRpcResponse{
		Message:   "Hello! Your message was: " + in.Message,
		Success:   true,
		Timestamp: time.Now().Unix(),
	}

	l.Infof("Responding with: message=%s, success=%v, timestamp=%d",
		resp.Message, resp.Success, resp.Timestamp)

	return resp, nil
}
