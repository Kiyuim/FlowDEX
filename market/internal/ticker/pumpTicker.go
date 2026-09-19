package ticker

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
	"dex/pkg/xredis"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

type PumpTicker struct {
	sc     *svc.ServiceContext
	logger logx.Logger
	ctx    context.Context
}

func (t *PumpTicker) Stop() {

}

func NewPumpTicker(sc *svc.ServiceContext) *PumpTicker {
	logger := logx.WithContext(context.Background()).WithFields(logx.Field("service", "pump-ticker"))
	return &PumpTicker{
		sc:     sc,
		logger: logger,
	}
}

func (t *PumpTicker) Start() {
	threading.GoSafe(func() {
		t.logger.Info("tradeTicker:udpatePumpCache")
		go t.StartTicker() // QuickIntel runs in its own goroutine
	})

	threading.GoSafe(func() {
		t.logger.Info("tradeTicker:udpateNewCreationCache")
		go t.StartNewCreationTicker() // QuickIntel runs in its own goroutine
	})
}

// start a ticker to refresh the cache every 5 seconds
// there are three types of pump status: new creation, completing, completed
func (l *PumpTicker) StartTicker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	_, err := xredis.MustLock(context.Background(), l.sc.RDS, "lock:PumpTicker", 5, 5)
	if err != nil {
		logx.Errorf("xredis.MustLock pumpTicker acquiring lock fail:%v", err)
		return
	}

	for {
		select {
		case <-ticker.C:
			threading.RunSafe(func() {
				l.UpdateCache()
			})
		}
	}
}

// UpdateCache updates the cache with the latest pump token data
func (l *PumpTicker) UpdateCache() {
	pairModel := solmodel.NewPairModel(l.sc.DB)
	redisClient := l.sc.RDS
	tokenModel := solmodel.NewTokenModel(l.sc.DB)

	// Define a helper function to update the cache for a specific pump status
	updateCacheForStatus := func(status int, findFunc func(ctx context.Context, pageNo, pageSize int32) ([]solmodel.Pair, error)) {
		// TODO: only return the top 10 pairs based on the pump point
		pairList, err := findFunc(l.ctx, 1, 10)
		if err != nil {
			logx.Errorf("Failed to get latest pump limit for status %d: %v", status, err)
			return
		}

		fmt.Println("pairList length is: , current status is:", status, len(pairList))

		tokanAddresses := make([]string, 0)
		for _, pair := range pairList {
			if pair.TokenAddress != "" {
				tokanAddresses = append(tokanAddresses, pair.TokenAddress)
			}
		}

		chainId := constants.Sol
		tokenList, err := tokenModel.FindAllByAddresses(l.ctx, int64(chainId), tokanAddresses)
		if err != nil {
			fmt.Println("tokenModel.FindAllByAddresse:")

			return
		}

		tokenMap := make(map[string]*solmodel.Token)
		for _, token := range tokenList {
			tokenMap[token.Address] = &token
		}

		list := make([]*market.PumpTokenItem, 0)
		for _, pair := range pairList {
			// if pair.TokenPrice == 0 {
			// 	continue
			// }

			// Get token from map with nil check
			token := tokenMap[pair.TokenAddress]
			var tokenIcon, twitterUsername, telegram string
			if token != nil {
				tokenIcon = token.Icon
				twitterUsername = token.TwitterUsername
				telegram = token.Telegram
			}

			list = append(list, &market.PumpTokenItem{
				ChainId:          pair.ChainId,
				ChainIcon:        chain.ChainId2ChainIcon(100000),
				TokenAddress:     pair.TokenAddress,
				TokenIcon:        tokenIcon,
				TokenName:        pair.TokenSymbol,
				LaunchTime:       pair.BlockTime.Unix(),
				MktCap:           pair.Fdv,
				DomesticProgress: pair.PumpPoint,
				TwitterUsername:  twitterUsername,
				Telegram:         telegram,
				PairAddress:      pair.Address,
			})
		}

		// update the cache with the latest data
		listData, err := json.Marshal(list)
		if err != nil {
			logx.Errorf("Failed to marshal list: %v", err)
			return
		}

		pairCacheKey := fmt.Sprint("pump-token-list-", status)
		err = redisClient.Set(pairCacheKey, string(listData))
		if err != nil {
			logx.Errorf("Failed to set list in Redis: %v", err)
			return
		}

		err = redisClient.Expire(pairCacheKey, 60)
		if err != nil {
			logx.Errorf("Failed to set expiration for list in Redis: %v", err)
			return
		}

		fmt.Println("update cache for status success", status)
	}

	// Update cache for each pump status
	updateCacheForStatus(constants.PumpStatusCompleting, pairModel.FindLatestCompletingPumpLimit)
	updateCacheForStatus(constants.PumpStatusCompleted, pairModel.FindLatestCompletePumpLimit)
}

// start a ticker to refresh the cache every 5 seconds
// there are three types of pump status: new creation, completing, completed
func (l *PumpTicker) StartNewCreationTicker() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	_, err := xredis.MustLock(context.Background(), l.sc.RDS, "lock:PumpTickerNewCreation", 5, 1)
	if err != nil {
		logx.Errorf("xredis.MustLock pumpTicker acquiring lock fail:%v", err)
		return
	}

	for {
		select {
		case <-ticker.C:
			l.UpdateNewCreationCache()
		}
	}
}

// UpdateCache updates the cache with the latest pump token data
func (l *PumpTicker) UpdateNewCreationCache() {
	pairModel := solmodel.NewPairModel(l.sc.DB)
	redisClient := l.sc.RDS
	tokenModel := solmodel.NewTokenModel(l.sc.DB)

	// Define a helper function to update the cache for a specific pump status
	updateCacheForStatus := func(status int, findFunc func(ctx context.Context, pageNo, pageSize int32) ([]solmodel.Pair, error)) {
		// TODO: only return the top 10 pairs based on the pump point
		pairList, err := findFunc(l.ctx, 1, 10)
		if err != nil {
			logx.Errorf("Failed to get latest pump limit for status %d: %v", status, err)
			return
		}

		tokanAddresses := make([]string, 0)
		for _, pair := range pairList {
			if pair.TokenAddress != "" {
				tokanAddresses = append(tokanAddresses, pair.TokenAddress)
			}
		}

		chainId := constants.Sol
		tokenList, err := tokenModel.FindAllByAddresses(l.ctx, int64(chainId), tokanAddresses)
		if err != nil {
			return
		}

		tokenMap := make(map[string]*solmodel.Token)
		for _, token := range tokenList {
			tokenMap[token.Address] = &token
		}

		list := make([]*market.PumpTokenItem, 0)
		for _, pair := range pairList {
			// if pair.TokenPrice == 0 {
			// 	continue
			// }

			fmt.Println("pair value is:", pair)

			// Get token from map with nil check
			token := tokenMap[pair.TokenAddress]
			var tokenIcon, twitterUsername, telegram string
			if token != nil {
				tokenIcon = token.Icon
				twitterUsername = token.TwitterUsername
				telegram = token.Telegram
			}

			list = append(list, &market.PumpTokenItem{
				ChainId:          pair.ChainId,
				ChainIcon:        chain.ChainId2ChainIcon(100000),
				TokenAddress:     pair.TokenAddress,
				TokenIcon:        tokenIcon,
				TokenName:        pair.TokenSymbol,
				LaunchTime:       pair.BlockTime.Unix(),
				MktCap:           pair.Fdv,
				DomesticProgress: pair.PumpPoint,
				TwitterUsername:  twitterUsername,
				Telegram:         telegram,
				PairAddress:      pair.Address,
			})
		}

		fmt.Println("list length is:", len(list))

		// update the cache with the latest data
		listData, err := json.Marshal(list)
		if err != nil {
			logx.Errorf("Failed to marshal list: %v", err)
			return
		}

		pairCacheKey := fmt.Sprint("pump-token-list-", status)
		err = redisClient.Set(pairCacheKey, string(listData))
		if err != nil {
			logx.Errorf("Failed to set list in Redis: %v", err)
			return
		}

		err = redisClient.Expire(pairCacheKey, 60*60*24*7)
		if err != nil {
			logx.Errorf("Failed to set expiration for list in Redis: %v", err)
			return
		}
		fmt.Println("update cache for new creationstatus success", status)

	}

	// Update cache for each pump status
	updateCacheForStatus(constants.PumpStatusNewCreation, pairModel.FindLatestPumpLimit)
}
