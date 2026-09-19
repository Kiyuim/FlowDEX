// Package cache
// File cache.go
package cache

import "dex/market/internal/svc"

var (
// var klineCache *kline.KlineCache
)

func Init(sc *svc.ServiceContext, loadFunc LoadFunc) {
	KlineCache = NewKlineCache(sc, loadFunc)
	KlineRedisCache = NewKlineRedisCache(sc.DB)
}
