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

		list := make([]*market.PumpTokenItem, 0)
		for _, pair := range pairList {
			token := tokenMap[pair.TokenAddress]
			var tokenIcon, twitterUsername, telegram string
			if token != nil {
				tokenIcon = token.Icon
				twitterUsername = token.TwitterUsername
				telegram = token.Telegram
			}

			list = append(list, &market.PumpTokenItem{
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
			})
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
