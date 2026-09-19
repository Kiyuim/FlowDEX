package cache

import (
	"context"

	"dex/market/internal/constants"
)

type KlineInterval interface {
	GetOpenClose(interval constants.KlineInterval, chainId constants.ChainId, pairAddr string) (float64, float64)
	GetUpDown(chainId constants.ChainId, pairAddr string) float64
}

type klineInterval struct {
}

func GetKlineInterval(ctx context.Context, interval constants.KlineInterval, chainId constants.ChainId, pairAddr string) float64 {
	return 0
}
