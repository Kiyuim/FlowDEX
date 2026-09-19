package logic

import (
	"context"
	"time"

	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNativeTokenPriceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNativeTokenPriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNativeTokenPriceLogic {
	return &GetNativeTokenPriceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Get native token price
func (l *GetNativeTokenPriceLogic) GetNativeTokenPrice(in *market.GetNativeTokenPriceRequest) (*market.GetNativeTokenPriceResponse, error) {
	resp := &market.GetNativeTokenPriceResponse{
		BaseTokenPriceUsd: 0,
	}

	searchTime, err := time.Parse(time.DateTime, in.SearchTime)
	if err != nil {
		return nil, nil
	}

	tradeModel := solmodel.NewTradeModel(l.svcCtx.DB)
	price, err := tradeModel.GetNativeTokenPrice(l.ctx, in.ChainId, searchTime)
	if err != nil {
		return resp, err
	}
	resp.BaseTokenPriceUsd = price
	return resp, nil
}
