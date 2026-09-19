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
	"math"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserPoolsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserPoolsLogic {
	return &GetUserPoolsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserPoolsLogic) GetUserPools(req *types.GetUserPoolsReq) (resp *types.GetUserPoolsResp, err error) {
	l.Infof("GetUserPools req: %+v", req)

	if req.WalletAddress == "" {
		return nil, xcode.New(400, "wallet_address is required")
	}

	// Create response object
	resp = &types.GetUserPoolsResp{
		Code: 0,
		Msg:  "success",
		Data: struct {
			List      []types.UserPoolItem `json:"list"`
			TotalNum  int64                `json:"total_num"`
			PageNo    int64                `json:"page_no"`
			PageSize  int64                `json:"page_size"`
			TotalPage int64                `json:"total_page"`
		}{
			PageNo:   req.PageNo,
			PageSize: req.PageSize,
		},
	}

	// Calculate pagination
	offset := (req.PageNo - 1) * req.PageSize

	// Build query conditions
	db := l.svcCtx.DB
	q := db.Model(&solmodel.UserPools{}).
		Where("user_wallet_address = ?", req.WalletAddress)

	if req.ChainId > 0 {
		q = q.Where("chain_id = ?", req.ChainId)
	}

	if req.PoolType != "" {
		q = q.Where("pool_type = ?", req.PoolType)
	}

	if req.PoolVersion != "" {
		q = q.Where("pool_version = ?", req.PoolVersion)
	}

	// Count total records
	var totalCount int64
	if err := q.Count(&totalCount).Error; err != nil {
		l.Errorf("Failed to get pool count: %v", err)
		return nil, xcode.New(500, "database error")
	}

	// Calculate total pages
	totalPages := int64(math.Ceil(float64(totalCount) / float64(req.PageSize)))
	resp.Data.TotalNum = totalCount
	resp.Data.TotalPage = totalPages

	// If no records, return early
	if totalCount == 0 {
		return resp, nil
	}

	// Query pools with pagination
	var pools []*solmodel.UserPools
	if err := q.Limit(int(req.PageSize)).Offset(int(offset)).
		Order("id DESC").Find(&pools).Error; err != nil {
		l.Errorf("Failed to get pools: %v", err)
		return nil, xcode.New(500, "database error")
	}

	// Convert to response items
	items := make([]types.UserPoolItem, len(pools))
	for i, pool := range pools {
		createdAt := int64(0)
		if !pool.CreatedAt.IsZero() {
			createdAt = pool.CreatedAt.Unix()
		}

		items[i] = types.UserPoolItem{
			Id:              pool.Id,
			PoolState:       pool.PoolState,
			InputVaultMint:  pool.InputVaultMint,
			OutputVaultMint: pool.OutputVaultMint,
			Token0Symbol:    pool.Token0Symbol,
			Token1Symbol:    pool.Token1Symbol,
			Token0Decimals:  pool.Token0Decimals,
			Token1Decimals:  pool.Token1Decimals,
			Token0Liquidity: int64(pool.Token0Liquidity),
			Token1Liquidity: int64(pool.Token1Liquidity),
			TradeFeeRate:    pool.TradeFeeRate,
			InitialPrice:    pool.InitialPrice,
			TxHash:          pool.TxHash,
			PoolVersion:     pool.PoolVersion,
			PoolType:        pool.PoolType,
			AmmConfig:       pool.AmmConfig,
			CreatedAt:       createdAt,
		}
	}

	resp.Data.List = items
	return resp, nil
}
