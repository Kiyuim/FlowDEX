package logic

import (
	"context"
	"fmt"

	"dex/trade/internal/svc"
	"dex/trade/pkg/entity"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/trace"
)

type ProcTokenPriceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProcTokenPriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProcTokenPriceLogic {
	return &ProcTokenPriceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProcTokenPriceLogic) ProcTokenPrice(in *trade.ProcTokenPriceRequest) (*trade.ProcTokenPriceResponse, error) {
	ctx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(l.ctx))

	fmt.Printf("=== RECEIVED PROC TOKEN PRICE === chain id: %v, token address: %v, price: %v", in.ChainId, in.TokenCa, in.Price)

	message := &entity.OrderMessage{
		Ctx:          ctx,
		TokenCA:      in.TokenCa,
		CurrentPrice: in.Price,
		ChainId:      int(in.ChainId),
	}

	l.svcCtx.DisruptorWrapper.Publish(message)

	return nil, nil
}
