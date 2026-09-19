package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dex/market/internal/constants"
	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"
	"dex/pkg/chain"

	"github.com/zeromicro/go-zero/core/logx"
)

// pumpToken24hStats holds trade-derived 24h stats for one pair: tx count,
// USD volume, and enough to compute a % price change (earliest vs latest
// trade price seen in the window).
type pumpToken24hStats struct {
	Txs        uint32
	Vol        float64
	FirstPrice float64
	LastPrice  float64
}

type GetPumpTokenListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPumpTokenListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPumpTokenListLogic {
	return &GetPumpTokenListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// fetch24hStats batch-computes trade count, USD volume, and first/last trade
// price over the last 24h for each pair address. Txs_24H/Vol_24H/Change24 on
// PumpTokenItem were previously never set (always 0) — this is what backs them.
func (l *GetPumpTokenListLogic) fetch24hStats(chainId int64, pairAddresses []string) (map[string]pumpToken24hStats, error) {
	result := make(map[string]pumpToken24hStats, len(pairAddresses))
	if len(pairAddresses) == 0 {
		return result, nil
	}

	type row struct {
		PairAddr   string
		Cnt        uint32
		Vol        float64
		FirstPrice float64
		LastPrice  float64
	}
	var rows []row

	since := time.Now().Add(-24 * time.Hour)
	// GROUP_CONCAT + SUBSTRING_INDEX picks the first/last value in each
	// ORDER BY group without relying on window functions.
	err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&solmodel.Trade{}).
		Select(`pair_addr as pair_addr,
			COUNT(*) as cnt,
			COALESCE(SUM(total_usd), 0) as vol,
			CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time ASC), ',', 1) AS DECIMAL(65,18)) as first_price,
			CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time DESC), ',', 1) AS DECIMAL(65,18)) as last_price`).
		Where("chain_id = ? AND pair_addr IN ? AND block_time >= ?", chainId, pairAddresses, since).
		Group("pair_addr").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.PairAddr] = pumpToken24hStats{
			Txs: r.Cnt, Vol: r.Vol, FirstPrice: r.FirstPrice, LastPrice: r.LastPrice,
		}
	}
	return result, nil
}

// Get pump token list data
func (l *GetPumpTokenListLogic) GetPumpTokenList(in *market.GetPumpTokenListRequest) (*market.GetPumpTokenListResponse, error) {
	var resultList []*market.PumpTokenItem
	var pairList []solmodel.Pair
	var err error
	pairModel := solmodel.NewPairModel(l.svcCtx.DB)
	redisClient := l.svcCtx.RDS
	pairCacheKey := fmt.Sprint("pump-token-list-", in.PumpStatus)

	//list
	fmt.Println("pairCacheKey is:", pairCacheKey)

	// get result list from cache
	cachedData, err := redisClient.Get(pairCacheKey)
	if err == nil && cachedData != "" {
		err = json.Unmarshal([]byte(cachedData), &resultList)
		if err != nil {
			logx.Errorf("Failed to unmarshal cached data: %v", err)
			return nil, err
		}

		fmt.Println("resultList length is:", len(resultList))
		return &market.GetPumpTokenListResponse{
			List:  resultList,
			Total: int32(len(resultList)),
		}, nil
	} else {
		in.PageNo = 1
		in.PageSize = 10
		if len(pairList) <= 0 {
			switch in.PumpStatus {
			case constants.PumpStatusNewCreation:
				pairList, err = pairModel.FindLatestPumpLimit(l.ctx, in.PageNo, in.PageSize)
			case constants.PumpStatusCompleting:
				pairList, err = pairModel.FindLatestCompletingPumpLimit(l.ctx, in.PageNo, in.PageSize)
			case constants.PumpStatusCompleted:
				pairList, err = pairModel.FindLatestCompletePumpLimit(l.ctx, in.PageNo, in.PageSize)
			}

			if err != nil {
				return nil, err
			}
			if len(pairList) == 0 {
				return &market.GetPumpTokenListResponse{
					List:  []*market.PumpTokenItem{},
					Total: 0,
				}, nil
			}
		}

		tokanAddresses := make([]string, 0)
		for _, pair := range pairList {
			if pair.TokenAddress != "" {
				tokanAddresses = append(tokanAddresses, pair.TokenAddress)
			}
		}

		tokenModel := solmodel.NewTokenModel(l.svcCtx.DB)
		tokenList, err := tokenModel.FindAllByAddresses(l.ctx, in.ChainId, tokanAddresses)
		if err != nil {
			fmt.Println("FindAllByAddresses:", err)
			return nil, err
		}

		tokenMap := make(map[string]*solmodel.Token)
		tokenHolderMap := make(map[string]int64)
		for _, token := range tokenList {
			tokenMap[token.Address] = &token
		}

		solTokenAccountModel := solmodel.NewSolTokenAccountModel(l.svcCtx.DB)
		for _, tokenAddress := range tokanAddresses {
			token := tokenMap[tokenAddress]
			if token != nil {
				holders, err := solTokenAccountModel.CountByTokenAddressWithTime(l.ctx, in.ChainId, tokenAddress, token.CreatedAt)
				if err != nil {
					l.Errorf("GetPumpTokenList: countByTokenAddressWithTime failed: %v token address: %v, createTime: %v", err, tokenAddress, token.CreatedAt)
					tokenHolderMap[tokenAddress] = 0
					continue
				}
				tokenHolderMap[tokenAddress] = holders
			}
		}

		pairAddresses := make([]string, 0, len(pairList))
		for _, pair := range pairList {
			pairAddresses = append(pairAddresses, pair.Address)
		}
		statsMap, err := l.fetch24hStats(in.ChainId, pairAddresses)
		if err != nil {
			// Non-fatal: still return the list with zeroed 24h stats rather
			// than failing the whole request over a stats-only aggregation.
			logx.Errorf("GetPumpTokenList: fetch24hStats failed: %v", err)
			statsMap = map[string]pumpToken24hStats{}
		}

		list := make([]*market.PumpTokenItem, 0)
		for _, pair := range pairList {
			token := tokenMap[pair.TokenAddress]
			var tokenIcon, twitterUsername, telegram string
			if token != nil {
				tokenIcon = token.Icon
				twitterUsername = token.TwitterUsername
				telegram = token.Telegram
			}

			item := &market.PumpTokenItem{
				ChainId:          pair.ChainId,
				ChainIcon:        chain.ChainId2ChainIcon(in.ChainId),
				TokenAddress:     pair.TokenAddress,
				TokenIcon:        tokenIcon,
				TokenName:        pair.TokenSymbol,
				LaunchTime:       pair.BlockTime.Unix(),
				MktCap:           pair.Fdv,
				HoldCount:        tokenHolderMap[pair.TokenAddress],
				DomesticProgress: pair.PumpPoint,
				TwitterUsername:  twitterUsername,
				Telegram:         telegram,
			}

			if stats, ok := statsMap[pair.Address]; ok {
				item.Txs_24H = stats.Txs
				item.Vol_24H = stats.Vol
				if stats.FirstPrice > 0 {
					item.Change24 = (stats.LastPrice - stats.FirstPrice) / stats.FirstPrice * 100
				}
			}

			list = append(list, item)
		}

		fmt.Println("list:", list)

		// cache the list in redismodel
		listData, err := json.Marshal(list)
		if err != nil {
			logx.Errorf("Failed to marshal list: %v", err)
			return nil, err
		}

		err = redisClient.Set(pairCacheKey, string(listData))
		if err != nil {
			logx.Errorf("Failed to set list in Redis: %v", err)
			return nil, err
		}

		// DEL pump-token-list-2
		err = redisClient.Expire(pairCacheKey, 5) // 5 seconds for development
		if err != nil {
			logx.Errorf("Failed to set expiration for list in Redis: %v", err)
			return nil, err
		}

		return &market.GetPumpTokenListResponse{
			List:  list,
			Total: 50,
		}, nil
	}
}
