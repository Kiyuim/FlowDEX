package logic

import (
	"context"
	// "encoding/json"
	// "fmt"

	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"
	"dex/pkg/chain"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClmmPoolListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClmmPoolListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClmmPoolListLogic {
	return &GetClmmPoolListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetClmmPoolListLogic) GetClmmPoolList(in *market.GetClmmPoolListRequest) (*market.GetClmmPoolListResponse, error) {
	var resultList []*market.ClmmPoolItem
	// redisClient := l.svcCtx.RDS
	// cacheKey := fmt.Sprintf("clmm-pool-list-v%d", in.PoolVersion)

	// // Try to get from cache first
	// cachedData, err := redisClient.Get(cacheKey)
	// if err == nil && cachedData != "" {
	// 	err = json.Unmarshal([]byte(cachedData), &resultList)
	// 	if err != nil {
	// 		logx.Errorf("Failed to unmarshal cached CLMM data: %v", err)
	// 	} else {
	// 		return &market.GetClmmPoolListResponse{
	// 			List:  resultList,
	// 			Total: int32(len(resultList)),
	// 		}, nil
	// 	}
	// }

	// Fetch from database
	var poolList []interface{}
	var tokenAddresses []string

	if in.PoolVersion == 1 {
		// Fetch CLMM V1 pools
		clmmV1Model := solmodel.NewClmmPoolInfoV1Model(l.svcCtx.DB)
		pools, err := l.fetchClmmV1Pools(clmmV1Model, in.PageNo, in.PageSize)
		if err != nil {
			return nil, err
		}

		for _, pool := range pools {
			poolList = append(poolList, pool)
			tokenAddresses = append(tokenAddresses, pool.InputVaultMint, pool.OutputVaultMint)
		}
	} else {
		// Fetch CLMM V2 pools
		clmmV2Model := solmodel.NewClmmPoolInfoV2Model(l.svcCtx.DB)
		pools, err := l.fetchClmmV2Pools(clmmV2Model, in.PageNo, in.PageSize)
		if err != nil {
			return nil, err
		}

		for _, pool := range pools {
			poolList = append(poolList, pool)
			tokenAddresses = append(tokenAddresses, pool.InputVaultMint, pool.OutputVaultMint)
		}
	}

	// Get token information
	tokenModel := solmodel.NewTokenModel(l.svcCtx.DB)
	tokenList, err := tokenModel.FindAllByAddresses(l.ctx, in.ChainId, tokenAddresses)
	if err != nil {
		logx.Errorf("Failed to get token info: %v", err)
		return nil, err
	}

	// Create token map for quick lookup
	tokenMap := make(map[string]*solmodel.Token)
	for _, token := range tokenList {
		tokenMap[token.Address] = &token
	}

	// Build response
	resultList = make([]*market.ClmmPoolItem, 0)
	for _, poolInterface := range poolList {
		var poolItem *market.ClmmPoolItem

		if in.PoolVersion == 1 {
			pool := poolInterface.(*solmodel.ClmmPoolInfoV1)
			poolItem = l.buildClmmPoolItem(pool.PoolState, pool.InputVaultMint, pool.OutputVaultMint,
				pool.TradeFeeRate, pool.CreatedAt.Unix(), tokenMap, 1)
		} else {
			pool := poolInterface.(*solmodel.ClmmPoolInfoV2)
			poolItem = l.buildClmmPoolItem(pool.PoolState, pool.InputVaultMint, pool.OutputVaultMint,
				pool.TradeFeeRate, pool.CreatedAt.Unix(), tokenMap, 2)
		}

		poolItem.ChainId = in.ChainId
		poolItem.ChainIcon = chain.ChainId2ChainIcon(in.ChainId)

		resultList = append(resultList, poolItem)
	}

	// Cache the result
	// listData, err := json.Marshal(resultList)
	// if err != nil {
	// 	logx.Errorf("Failed to marshal CLMM pool list: %v", err)
	// } else {
	// 	err = redisClient.Set(cacheKey, string(listData))
	// 	if err != nil {
	// 		logx.Errorf("Failed to cache CLMM pool list: %v", err)
	// 	}

	// 	// Set expiration (1 hour)
	// 	err = redisClient.Expire(cacheKey, 60*60)
	// 	if err != nil {
	// 		logx.Errorf("Failed to set expiration for CLMM pool list: %v", err)
	// 	}
	// }

	return &market.GetClmmPoolListResponse{
		List:  resultList,
		Total: int32(len(resultList)),
	}, nil
}

func (l *GetClmmPoolListLogic) fetchClmmV1Pools(model solmodel.ClmmPoolInfoV1Model, pageNo, pageSize int32) ([]*solmodel.ClmmPoolInfoV1, error) {
	// For now, return latest 10 pools. In production, implement proper pagination
	offset := (pageNo - 1) * pageSize

	var pools []*solmodel.ClmmPoolInfoV1
	err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&solmodel.ClmmPoolInfoV1{}).
		Order("created_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&pools).Error

	return pools, err
}

func (l *GetClmmPoolListLogic) fetchClmmV2Pools(model solmodel.ClmmPoolInfoV2Model, pageNo, pageSize int32) ([]*solmodel.ClmmPoolInfoV2, error) {
	// For now, return latest 10 pools. In production, implement proper pagination
	offset := (pageNo - 1) * pageSize

	var pools []*solmodel.ClmmPoolInfoV2
	err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&solmodel.ClmmPoolInfoV2{}).
		Order("created_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&pools).Error

	return pools, err
}

func (l *GetClmmPoolListLogic) buildClmmPoolItem(poolState, inputMint, outputMint string,
	tradeFeeRate int64, launchTime int64, tokenMap map[string]*solmodel.Token, poolVersion int32) *market.ClmmPoolItem {

	inputToken := tokenMap[inputMint]
	outputToken := tokenMap[outputMint]

	var inputSymbol, outputSymbol, inputIcon, outputIcon string

	if inputToken != nil {
		inputSymbol = inputToken.Symbol
		inputIcon = inputToken.Icon
	} else {
		inputSymbol = "Unknown"
		inputIcon = ""
	}

	if outputToken != nil {
		outputSymbol = outputToken.Symbol
		outputIcon = outputToken.Icon
	} else {
		outputSymbol = "Unknown"
		outputIcon = ""
	}

	return &market.ClmmPoolItem{
		PoolState:         poolState,
		InputVaultMint:    inputMint,
		OutputVaultMint:   outputMint,
		InputTokenSymbol:  inputSymbol,
		OutputTokenSymbol: outputSymbol,
		InputTokenIcon:    inputIcon,
		OutputTokenIcon:   outputIcon,
		TradeFeeRate:      tradeFeeRate,
		LaunchTime:        launchTime,
		LiquidityUsd:      0.0, // TODO: Calculate from pool data
		Txs_24H:           0,   // TODO: Calculate from trade data
		Vol_24H:           0.0, // TODO: Calculate from trade data
		Apr:               0.0, // TODO: Calculate APR
		PoolVersion:       poolVersion,
	}
}
