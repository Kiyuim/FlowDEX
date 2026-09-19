package logic

import (
	"context"
	"encoding/json"
	"fmt"

	"dex/market/internal/constants"
	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"
	"dex/pkg/chain"
	"dex/pkg/solprice"

	"github.com/zeromicro/go-zero/core/logx"
)

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

// Get pump token list data
func (l *GetPumpTokenListLogic) GetPumpTokenList(in *market.GetPumpTokenListRequest) (*market.GetPumpTokenListResponse, error) {
	var resultList []*market.PumpTokenItem
	var pairList []solmodel.Pair
	var err error
	pairModel := solmodel.NewPairModel(l.svcCtx.DB)
	redisClient := l.svcCtx.RDS
	pairCacheKey := fmt.Sprint("pump-token-list-", in.PumpStatus)
	solPriceUsd := solprice.GetSolUsdPrice()

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
			List:        resultList,
			Total:       int32(len(resultList)),
			SolPriceUsd: solPriceUsd,
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
		for _, token := range tokenList {
			tokenMap[token.Address] = &token
		}
		tokenHolderMap := FetchHolderCounts(l.ctx, l.svcCtx.DB, in.ChainId, tokanAddresses, tokenMap)

		pairAddresses := make([]string, 0, len(pairList))
		for _, pair := range pairList {
			pairAddresses = append(pairAddresses, pair.Address)
		}
		statsMap, err := Fetch24hStats(l.ctx, l.svcCtx.DB, in.ChainId, pairAddresses)
		if err != nil {
			// Non-fatal: still return the list with zeroed 24h stats rather
			// than failing the whole request over a stats-only aggregation.
			logx.Errorf("GetPumpTokenList: fetch24hStats failed: %v", err)
			statsMap = map[string]PumpToken24hStats{}
		}

		list := make([]*market.PumpTokenItem, 0)
		for _, pair := range pairList {
			token := tokenMap[pair.TokenAddress]
			var tokenIcon, twitterUsername, telegram, program string
			if token != nil {
				tokenIcon = token.Icon
				twitterUsername = token.TwitterUsername
				telegram = token.Telegram
				program = token.Program
			}

			item := &market.PumpTokenItem{
				ChainId:          pair.ChainId,
				ChainIcon:        chain.ChainId2ChainIcon(in.ChainId),
				TokenAddress:     pair.TokenAddress,
				PairAddress:      pair.Address,
				TokenIcon:        tokenIcon,
				TokenName:        pair.TokenSymbol,
				LaunchTime:       pair.BlockTime.Unix(),
				MktCap:           pair.Fdv,
				HoldCount:        tokenHolderMap[pair.TokenAddress],
				DomesticProgress: pair.PumpPoint,
				TwitterUsername:  twitterUsername,
				Telegram:         telegram,
				Program:          program,
			}

			if stats, ok := statsMap[pair.Address]; ok {
				item.Txs_24H = stats.Txs
				item.Vol_24H = stats.Vol
				item.Price = stats.LastPrice
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
			List:        list,
			Total:       50,
			SolPriceUsd: solPriceUsd,
		}, nil
	}
}
