package cache

import (
	"context"
	"time"

	"dex/market/internal/constants"
)

type KlineIntervalHour24 struct {
	interval constants.KlineInterval
	duration time.Duration
}

func NewKlineIntervalHour24() *KlineIntervalHour24 {
	return &KlineIntervalHour24{
		interval: constants.KlineMin5,
		duration: time.Second * 3600 * 24,
	}
}

func (k *KlineIntervalHour24) GetOpenClose(chainId constants.ChainId, pairAddr string) (float64, float64) {
	option := KlineQueryOption{
		ChainId:  chainId,
		PairAddr: pairAddr,
		Interval: k.interval,
		To:       0,
		Limit:    300,
	}
	klines, ok := KlineCache.Fetch(context.TODO(), option)
	if klines == nil || !ok {

	}
	var openPx, closePx float64
	if len(klines) > 0 {
		lastTime := time.Unix(klines[0].CandleTime, 0)
		firstTime := lastTime.Add(-k.duration).Unix()
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

func (k *KlineIntervalHour24) GetUpDown(chainId constants.ChainId, pairAddr string) float64 {
	openPx, closePx := k.GetOpenClose(chainId, pairAddr)
	return closePx/openPx - 1
}
