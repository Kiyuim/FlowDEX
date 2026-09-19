package cache

import (
	"context"
	"time"

	"dex/market/internal/constants"
)

type KlineIntervalMin5 struct {
	interval constants.KlineInterval
}

func NewKlineIntervalMin5() *KlineIntervalMin5 {
	return &KlineIntervalMin5{
		interval: constants.KlineMin1,
	}
}

func (k *KlineIntervalMin5) GetOpenClose(chainId constants.ChainId, pairAddr string) (float64, float64) {
	option := KlineQueryOption{
		ChainId:  chainId,
		PairAddr: pairAddr,
		Interval: k.interval,
		To:       0,
		Limit:    70,
	}
	klines, ok := KlineCache.Fetch(context.TODO(), option)
	if klines == nil || !ok {

	}
	var openPx, closePx float64
	if len(klines) > 0 {
		lastTime := time.Unix(klines[0].CandleTime, 0)
		firstTime := lastTime.Add(-time.Second * 3600).Unix()
		for _, kline := range klines {
			if kline.Close > 0 && closePx == 0 {
				closePx = kline.Close
			}
			if kline.CandleTime <= firstTime && kline.Open > 0 {
				openPx = kline.Open
				break
			}
		}
	}
	return openPx, closePx
}

func (k *KlineIntervalMin5) GetUpDown(chainId constants.ChainId, pairAddr string) float64 {
	openPx, closePx := k.GetOpenClose(chainId, pairAddr)
	return closePx/openPx - 1
}
