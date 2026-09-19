package logic

import (
	"context"

	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model"
	"dex/model/solmodel"

	"github.com/zeromicro/go-zero/core/logx"
)

// Portfolio: tokens/pools a wallet created through this app. Token creation
// happens client-side (wallet-adapter + @solana/spl-token, no backend call),
// so there is no way for the backend to learn about a creation on its own —
// RecordUserAsset below is called by the frontend right after a successful
// on-chain create. Before this, user_created_assets was wired (repo + model
// + schema) but never written to by anything, so Portfolio always showed
// nothing; this closes that loop going forward. It can't retroactively
// attribute tokens/pools created before this fix shipped.

type GetUserTokensLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserTokensLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserTokensLogic {
	return &GetUserTokensLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserTokensLogic) GetUserTokens(in *market.GetUserTokensRequest) (*market.GetUserTokensResponse, error) {
	var assets []model.UserCreatedAsset
	err := l.svcCtx.DB.WithContext(l.ctx).
		Where("user_wallet = ? AND chain_id = ? AND asset_type = ?", in.WalletAddress, in.ChainId, "token").
		Order("created_at DESC").
		Find(&assets).Error
	if err != nil {
		return nil, err
	}

	// Best-effort enrichment: if the token was later indexed as a normal
	// sol_token row (e.g. someone traded it), pick up its real icon/desc.
	addresses := make([]string, 0, len(assets))
	for _, a := range assets {
		addresses = append(addresses, a.AssetAddress)
	}
	tokenModel := solmodel.NewTokenModel(l.svcCtx.DB)
	tokenList, err := tokenModel.FindAllByAddresses(l.ctx, in.ChainId, addresses)
	tokenMap := make(map[string]*solmodel.Token, len(tokenList))
	if err == nil {
		for _, t := range tokenList {
			tokenMap[t.Address] = &t
		}
	}

	list := make([]*market.UserTokenItem, 0, len(assets))
	for _, a := range assets {
		item := &market.UserTokenItem{
			Id:            a.ID,
			TokenAddress:  a.AssetAddress,
			TokenName:     a.AssetName,
			TokenSymbol:   a.AssetSymbol,
			TokenDecimals: int32(a.Decimals),
			TokenSupply:   a.TotalSupply,
			CreatedAt:     a.CreatedAt.Unix(),
		}
		if t, ok := tokenMap[a.AssetAddress]; ok {
			item.TokenIcon = t.Icon
			item.Description = t.Description
		}
		list = append(list, item)
	}

	return &market.GetUserTokensResponse{List: list, Total: int32(len(list))}, nil
}

type GetUserPoolsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserPoolsLogic {
	return &GetUserPoolsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserPoolsLogic) GetUserPools(in *market.GetUserPoolsRequest) (*market.GetUserPoolsResponse, error) {
	var assets []model.UserCreatedAsset
	err := l.svcCtx.DB.WithContext(l.ctx).
		Where("user_wallet = ? AND chain_id = ? AND asset_type = ?", in.WalletAddress, in.ChainId, "pool").
		Order("created_at DESC").
		Find(&assets).Error
	if err != nil {
		return nil, err
	}

	list := make([]*market.UserPoolItem, 0, len(assets))
	for _, a := range assets {
		list = append(list, &market.UserPoolItem{
			Id:              a.ID,
			PoolState:       a.AssetAddress,
			InputVaultMint:  a.Token0Address,
			OutputVaultMint: a.Token1Address,
			Token0Symbol:    a.Token0Symbol,
			Token1Symbol:    a.Token1Symbol,
			TradeFeeRate:    int64(a.FeeTier),
			PoolType:        a.PoolType,
			CreatedAt:       a.CreatedAt.Unix(),
		})
	}

	return &market.GetUserPoolsResponse{List: list, Total: int32(len(list))}, nil
}

type RecordUserAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecordUserAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecordUserAssetLogic {
	return &RecordUserAssetLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RecordUserAssetLogic) RecordUserAsset(in *market.RecordUserAssetRequest) (*market.RecordUserAssetResponse, error) {
	asset := model.UserCreatedAsset{
		UserWallet:    in.WalletAddress,
		AssetType:     in.AssetType,
		AssetName:     in.AssetName,
		AssetSymbol:   in.AssetSymbol,
		AssetAddress:  in.AssetAddress,
		ChainID:       in.ChainId,
		Decimals:      int(in.Decimals),
		TotalSupply:   in.TotalSupply,
		Token0Address: in.Token0Address,
		Token1Address: in.Token1Address,
		Token0Symbol:  in.Token0Symbol,
		Token1Symbol:  in.Token1Symbol,
		PoolType:      in.PoolType,
		FeeTier:       int(in.FeeTier),
	}

	// asset_address+chain_id is unique — a wallet re-recording a creation it
	// already reported (e.g. a page refresh retry) should be a no-op, not an
	// error or a duplicate row.
	err := l.svcCtx.DB.WithContext(l.ctx).
		Where(model.UserCreatedAsset{AssetAddress: asset.AssetAddress, ChainID: asset.ChainID}).
		Attrs(asset).
		FirstOrCreate(&asset).Error
	if err != nil {
		logx.Errorf("RecordUserAsset: failed to record wallet=%s address=%s: %v", in.WalletAddress, in.AssetAddress, err)
		return &market.RecordUserAssetResponse{Success: false}, nil
	}

	return &market.RecordUserAssetResponse{Success: true}, nil
}
