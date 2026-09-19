package logic

import (
	"context"
	"fmt"
	"time"

	"dex/market/internal/constants"
	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPairInfoByTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPairInfoByTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPairInfoByTokenLogic {
	return &GetPairInfoByTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Get pair information by token address
func (l *GetPairInfoByTokenLogic) GetPairInfoByToken(in *market.GetPairInfoByTokenRequest) (*market.GetPairInfoByTokenResponse, error) {
	pair := &solmodel.Pair{}
	var err error
	fmt.Println("chain id is:", in.ChainId)
	if in.ChainId == int64(constants.Sol) || in.ChainId == 100000 {
		pairModel := solmodel.NewPairModel(l.svcCtx.DB)
		//output chain id and token address
		fmt.Println("****************chain id and token address**************", in.ChainId, in.TokenAddress)
		pair, err = pairModel.FindOneByChainIdTokenAddress(l.ctx, in.ChainId, in.TokenAddress)
		if err != nil {
			fmt.Println("******1111111111111*****", in.ChainId, in.TokenAddress)
			return nil, err
		}
	}

	fmt.Println("****************pair**************", pair.BaseTokenPrice)
	fmt.Println("****************pair**************", pair.Fdv)

	if pair == nil {
		return nil, fmt.Errorf("pairInfo is nil for token: %s", in.TokenAddress)
	}

	// Helper function to safely convert time.Time to Unix timestamp
	// Returns 0 if time is zero value instead of -62135596800
	safeUnixTime := func(t time.Time) int64 {
		if t.IsZero() {
			return 0
		}
		return t.Unix()
	}

	return &market.GetPairInfoByTokenResponse{
		ChainId:                in.ChainId,
		Address:                pair.Address,
		Name:                   pair.Name,
		FactoryAddress:         pair.FactoryAddress,
		BaseTokenAddress:       pair.BaseTokenAddress,
		TokenAddress:           pair.TokenAddress,
		BaseTokenSymbol:        pair.BaseTokenSymbol,
		TokenSymbol:            pair.TokenSymbol,
		BaseTokenDecimal:       pair.BaseTokenDecimal,
		TokenDecimal:           pair.TokenDecimal,
		BaseTokenIsNativeToken: pair.BaseTokenIsNativeToken == 1,
		BaseTokenIsToken0:      pair.BaseTokenIsToken0 == 1,
		InitBaseTokenAmount:    pair.InitBaseTokenAmount,
		InitTokenAmount:        pair.InitTokenAmount,
		CurrentBaseTokenAmount: pair.CurrentBaseTokenAmount,
		CurrentTokenAmount:     pair.CurrentTokenAmount,
		Fdv:                    pair.Fdv,
		// 对内显示 不改成fdv
		MktCap:            pair.MktCap,
		BaseTokenPrice:    pair.BaseTokenPrice,
		TokenPrice:        pair.TokenPrice,
		BlockNum:          pair.BlockNum,
		BlockTime:         safeUnixTime(pair.BlockTime),
		HighestTokenPrice: pair.HighestTokenPrice,
		LatestTradeTime:   safeUnixTime(pair.LatestTradeTime),
	}, nil
}
