//go:build ignore

// This file implements a REST layer (UserTokens/UserPools) that is never wired
// into trade.go's main() (trade only starts a gRPC server — see trade/trade.go).
// It also doesn't compile as-is (undefined xcode.ParamsError, missing
// svcCtx.UserPoolsModel/UserTokensModel, wrong xcode.New signature). Excluded
// from the build rather than silently left broken; see
// docs/项目已知问题与修复记录.md for what's needed to finish it.

package v1

import (
	"context"
	"dex/pkg/xcode"
	"dex/trade/internal/svc"
	"dex/trade/internal/types"
	"dex/trade/model/solmodel"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type StoreUserPoolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStoreUserPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StoreUserPoolLogic {
	return &StoreUserPoolLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StoreUserPoolLogic) StoreUserPool(req *types.StoreUserPoolReq) (resp *types.StoreUserPoolResp, err error) {
	l.Infof("StoreUserPool req: %+v", req)

	// Create model for insertion
	userPool := &solmodel.UserPools{
		UserWalletAddress: req.UserWalletAddress,
		ChainId:           req.ChainId,
		PoolState:         req.PoolState,
		InputVaultMint:    req.InputVaultMint,
		OutputVaultMint:   req.OutputVaultMint,
		Token0Symbol:      req.Token0Symbol,
		Token1Symbol:      req.Token1Symbol,
		Token0Decimals:    req.Token0Decimals,
		Token1Decimals:    req.Token1Decimals,
		InitialPrice:      req.InitialPrice,
		TradeFeeRate:      req.TradeFeeRate,
		TxHash:            req.TxHash,
		PoolVersion:       req.PoolVersion,
		PoolType:          req.PoolType,
		AmmConfig:         req.AmmConfig,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Check if the pool already exists
	existingPool, err := l.svcCtx.UserPoolsModel.FindOneByUserWalletAddressPoolState(
		l.ctx,
		req.UserWalletAddress,
		req.PoolState,
	)

	if err == nil {
		// Pool already exists, update it
		l.Infof("Pool already exists with ID %d, updating", existingPool.Id)
		userPool.Id = existingPool.Id
		userPool.UpdatedAt = time.Now()
		err = l.svcCtx.UserPoolsModel.Update(l.ctx, userPool)
		if err != nil {
			l.Errorf("Failed to update pool: %v", err)
			return nil, xcode.New(500, err.Error())
		}
	} else {
		// Pool doesn't exist, insert it
		l.Infof("Pool doesn't exist, creating new record")
		err = l.svcCtx.UserPoolsModel.Insert(l.ctx, userPool)
		if err != nil {
			l.Errorf("Failed to insert pool: %v", err)
			return nil, xcode.New(500, err.Error())
		}
	}

	return &types.StoreUserPoolResp{
		Code: 0,
		Msg:  "success",
		Data: map[string]interface{}{
			"id": userPool.Id,
		},
	}, nil
}
