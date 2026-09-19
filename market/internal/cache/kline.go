// Package cache
// File kline.go
package cache

import (
	"context"
	"fmt"
	"time"

	"dex/market/internal/constants"
	"dex/market/internal/svc"
	pb "dex/market/market"

	"github.com/panjf2000/ants/v2"
	"github.com/zeromicro/go-zero/core/logx"
)

type LoadFunc func(chainId constants.ChainId, pairAddr string, interval constants.KlineInterval, limit int64) ([]*pb.Kline, error)

var KlineChan chan *pb.Klines

var KlineCache *klineCache

type klineCache struct {
	m  map[string]*KlineData
	sc *svc.ServiceContext
}

func NewKlineCache(sc *svc.ServiceContext, loadFunc LoadFunc) *klineCache {
	KlineChan = make(chan *pb.Klines, 10000)
	kc := &klineCache{sc: sc}
	m := make(map[string]*KlineData)
	expired := time.Hour * 24
	for _, v := range constants.ChainIds {
		m[kc.key(int(v), string(constants.KlineMin1))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline1m", interval: constants.KlineMin1,
			},
			loadFunc)
		m[kc.key(int(v), string(constants.KlineMin5))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline5m", interval: constants.KlineMin5,
			},
			loadFunc)
		m[kc.key(int(v), string(constants.KlineMin15))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline15m", interval: constants.KlineMin15,
			},
			loadFunc)
		m[kc.key(int(v), string(constants.KlineHour1))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline1h", interval: constants.KlineHour1,
			},
			loadFunc)
		m[kc.key(int(v), string(constants.KlineHour4))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline4h", interval: constants.KlineHour4,
			},
			loadFunc)
		m[kc.key(int(v), string(constants.KlineHour12))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline12h", interval: constants.KlineHour12,
			},
			loadFunc)
		m[kc.key(int(v), string(constants.KlineDay1))] = NewKlineData(sc,
			klineConfig{
				chainId: v, limit: 5000, expire: expired, name: "kline1d", interval: constants.KlineDay1,
			},
			loadFunc)
	}
	kc.m = m
	go kc.Listen()
	return kc
}

// Reload
func (c *klineCache) Reload(chainId constants.ChainId, interval constants.KlineInterval, pairAddresses ...string) error {
	cache := c.m[c.key(int(chainId), string(interval))]
	err := cache.Reload(pairAddresses...)
	return err
}

func (c *klineCache) add(kline *pb.Kline) {
	key := c.key(int(kline.ChainId), kline.Interval)
	cache := c.m[key]
	cache.Add(kline)
}

func (c *klineCache) addFluctuation(kline *pb.Kline, fluctuation *pb.KlineFluctuation) {
	chainId := kline.ChainId
	interval := kline.Interval
	key := c.key(int(chainId), string(interval))
	cache := c.m[key]
	cache.AddFluctuation(kline.PairAddr, kline.CandleTime, kline.TotalCount, fluctuation)
}

func (c *klineCache) Listen() {
	pool, _ := ants.NewPool(10)
	defer pool.Release()

	for klines := range KlineChan {
		klines := klines
		_ = pool.Submit(func() {
			defer func() {
				if r := recover(); r != nil {
					logx.Errorf("panic in kline cache listener: %v", r)
				}
			}()

			if klines == nil || len(klines.List) == 0 {
				return
			}

			startTime := time.Now()
			pairAddr := ""

			for _, kline := range klines.List {
				pairAddr = kline.PairAddr
				c.add(kline)
			}

			c.addFluctuation(klines.List[len(klines.List)-1], klines.Fluctuation)

			if time.Since(startTime) > time.Millisecond*10 {
				logx.Infof("put kline to cache time:%v, pairAddr:%v", time.Since(startTime), pairAddr)
			}
		})
	}
}

func (c *klineCache) key(chainId int, interval string) string {
	return fmt.Sprintf("kline_%d_%s", chainId, interval)
}

func (c *klineCache) Put(kline *pb.Klines) {
	KlineChan <- kline
}

func (c *klineCache) GetFluctuation(ctx context.Context, option KlineQueryOption) *pb.KlineFluctuation {
	//key := c.key(int(option.ChainId), string(option.Interval))
	//cache := c.m[key]
	result := &pb.KlineFluctuation{}

	fluctuation := KlineRedisCache.GetKlineFluctuation(ctx, option)
	switch option.Interval {
	case constants.KlineMin1:
		result.KlineFluctuation_1M = fluctuation
	case constants.KlineMin5:
		result.KlineFluctuation_5M = fluctuation
	case constants.KlineMin15:
		result.KlineFluctuation_15M = fluctuation
	case constants.KlineHour1:
		result.KlineFluctuation_1H = fluctuation
	case constants.KlineHour4:
		result.KlineFluctuation_4H = fluctuation
	case constants.KlineHour12:
		result.KlineFluctuation_12H = fluctuation
	case constants.KlineDay1:
		result.KlineFluctuation_24H = fluctuation
	}

	return result
}

func (c *klineCache) GetKline24Info(ctx context.Context, option KlineQueryOption) *pb.Kline24InfoItem {
	result := &pb.Kline24InfoItem{}

	kline24InfoItem := KlineRedisCache.GetKline24Info(ctx, option)

	result.Change_24 = kline24InfoItem.Change_24
	result.Vol_24H = kline24InfoItem.Vol_24H
	result.Txs_24H = kline24InfoItem.Txs_24H

	return result
}

func (c *klineCache) GetKlineAllFluctuation(ctx context.Context, option KlineQueryOption) *pb.KlineFluctuation {
	result := KlineRedisCache.GetKlineAllFluctuation(ctx, option)
	return result
}

type KlineQueryOption struct {
	ChainId  constants.ChainId
	PairAddr string
	Interval constants.KlineInterval
	From     int64
	To       int64
	Limit    int64
}

// Fetch
func (c *klineCache) Fetch(ctx context.Context, option KlineQueryOption) ([]*pb.Kline, bool) {
	key := c.key(int(option.ChainId), string(option.Interval))
	cache := c.m[key]

	klines, ok := cache.Fetch(ctx, option)
	if !ok {
		if len(klines) > 0 {
			last := klines[len(klines)-1]
			option.To = last.CandleTime
		}

		option.Limit -= int64(len(klines))
		rdsKlines, ok := KlineRedisCache.Fetch(ctx, option)

		return append(klines, rdsKlines...), ok
	}
	return klines, ok
}

func (c *klineCache) GetAll(ctx context.Context, chainId constants.ChainId, interval constants.KlineInterval, pairAddr string) []*pb.Kline {
	key := c.key(int(chainId), string(interval))
	cache := c.m[key]
	klines := cache.GetAll(pairAddr)
	return klines
}

// FindLastAfterTime
func (c *klineCache) FindLastAfterTime(ctx context.Context, chainId constants.ChainId,
	pairAddr string, targetTime int64, interval constants.KlineInterval) (*pb.Kline, bool) {

	klines := c.GetAll(ctx, chainId, interval, pairAddr)
	if len(klines) == 0 {
		return nil, false
	}

	if len(klines) == 1 && klines[0].CandleTime >= targetTime {
		return klines[0], true
	}

	for i := 0; i < len(klines); i++ {
		if (i+1 == len(klines) || klines[i+1].CandleTime < targetTime) && klines[i].CandleTime >= targetTime {
			return klines[i], true
		}
	}

	return nil, false
}
