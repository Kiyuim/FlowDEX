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

type StoreUserTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStoreUserTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StoreUserTokenLogic {
	return &StoreUserTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StoreUserTokenLogic) StoreUserToken(req *types.StoreUserTokenReq) (resp *types.StoreUserTokenResp, err error) {
	l.Infof("StoreUserToken req: %+v", req)

	// Create model for insertion
	userToken := &solmodel.UserTokens{
		UserWalletAddress: req.UserWalletAddress,
		ChainId:           req.ChainId,
		TokenAddress:      req.TokenAddress,
		TokenProgram:      req.TokenProgram,
		Name:              req.Name,
		Symbol:            req.Symbol,
		Decimals:          req.Decimals,
		TotalSupply:       float64(req.TotalSupply),
		Icon:              req.Icon,
		Description:       req.Description,
		TxHash:            req.TxHash,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Check if the token already exists
	existingToken, err := l.svcCtx.UserTokensModel.FindOneByUserWalletAddressTokenAddress(
		l.ctx,
		req.UserWalletAddress,
		req.TokenAddress,
	)

	if err == nil {
		// Token already exists, update it
		l.Infof("Token already exists with ID %d, updating", existingToken.Id)
		userToken.Id = existingToken.Id
		userToken.UpdatedAt = time.Now()
		err = l.svcCtx.UserTokensModel.Update(l.ctx, userToken)
		if err != nil {
			l.Errorf("Failed to update token: %v", err)
			return nil, xcode.New(xcode.InternalError, err.Error())
		}
	} else {
		// Token doesn't exist, insert it
		l.Infof("Token doesn't exist, creating new record")
		err = l.svcCtx.UserTokensModel.Insert(l.ctx, userToken)
		if err != nil {
			l.Errorf("Failed to insert token: %v", err)
			return nil, xcode.New(xcode.InternalError, err.Error())
		}
	}

	return &types.StoreUserTokenResp{
		Code: 0,
		Msg:  "success",
		Data: map[string]interface{}{
			"id": userToken.Id,
		},
	}, nil
}
