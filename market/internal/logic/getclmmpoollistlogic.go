package logic

import (
	"context"
	"time"
	// "encoding/json"
	// "fmt"

	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"
	"dex/pkg/chain"

	"github.com/zeromicro/go-zero/core/logx"
)

// clmm24hStats holds the trade-derived stats for one pool over the last 24h.
// CLMM pool rows carry no reserve/vault-balance snapshot anywhere in this
// pipeline (consumer never indexes vault token balances for CLMM pools), so
// LiquidityUsd and Apr stay 0 below — computing them honestly would need a
// separate on-chain vault-balance indexer, not just a DB join. Vol/Txs are
// real, computed from the trade table.
type clmm24hStats struct {
	Txs uint32
	Vol float64
}

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

	// Trade records store pool_state as pair_addr for CLMM swaps (see
	// consumer/internal/logic/sol/block/raydium_clmm.go), so 24h volume/tx
	// count can be joined straight off the trade table.
	poolStates := make([]string, 0, len(poolList))
	for _, poolInterface := range poolList {
		if in.PoolVersion == 1 {
			poolStates = append(poolStates, poolInterface.(*solmodel.ClmmPoolInfoV1).PoolState)
		} else {
			poolStates = append(poolStates, poolInterface.(*solmodel.ClmmPoolInfoV2).PoolState)
		}
	}
	statsMap, err := l.fetch24hStats(in.ChainId, poolStates)
	if err != nil {
		// Non-fatal: still return the pool list with zeroed 24h stats rather
		// than failing the whole request over a stats-only aggregation.
		logx.Errorf("GetClmmPoolList: fetch24hStats failed: %v", err)
		statsMap = map[string]clmm24hStats{}
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

		if stats, ok := statsMap[poolItem.PoolState]; ok {
			poolItem.Txs_24H = stats.Txs
			poolItem.Vol_24H = stats.Vol
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

// fetch24hStats batch-computes trade count + USD volume over the last 24h for
// each pool address, keyed by pool_state (== trade.pair_addr for CLMM swaps).
func (l *GetClmmPoolListLogic) fetch24hStats(chainId int64, poolStates []string) (map[string]clmm24hStats, error) {
	result := make(map[string]clmm24hStats, len(poolStates))
	if len(poolStates) == 0 {
		return result, nil
	}

	type row struct {
		PairAddr string
		Cnt      uint32
		Vol      float64
	}
	var rows []row

	since := time.Now().Add(-24 * time.Hour)
	err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&solmodel.Trade{}).
		Select("pair_addr as pair_addr, COUNT(*) as cnt, COALESCE(SUM(total_usd), 0) as vol").
		Where("chain_id = ? AND pair_addr IN ? AND block_time >= ?", chainId, poolStates, since).
		Group("pair_addr").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.PairAddr] = clmm24hStats{Txs: r.Cnt, Vol: r.Vol}
	}
	return result, nil
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
		TradeFeeRate: tradeFeeRate,
		LaunchTime:   launchTime,
		// LiquidityUsd/Apr: genuinely not computable from data this pipeline
		// captures — clmm_pool_info_v1/v2 store account addresses only, no
		// vault reserve amounts are ever indexed, so there is no liquidity
		// figure to base either metric on. Left at 0 rather than faked; would
		// need an on-chain vault-balance indexer to do honestly. See
		// docs/项目已知问题与修复记录.md.
		LiquidityUsd: 0.0,
		Apr:          0.0,
		// Txs_24H/Vol_24H are set by the caller from fetch24hStats (real,
		// joined off the trade table).
		Txs_24H:     0,
		Vol_24H:     0.0,
		PoolVersion: poolVersion,
	}
}
