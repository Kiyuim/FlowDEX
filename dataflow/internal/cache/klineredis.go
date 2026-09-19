package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	constants2 "dex/pkg/constants"

	"dex/dataflow/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	datakline "dex/dataflow/dataflow"
	"dex/dataflow/internal/constants"
	rds "dex/dataflow/pkg/redis"
	"dex/dataflow/pkg/util"

	"github.com/panjf2000/ants/v2"
	"github.com/zeromicro/go-zero/core/logx"
)

var KlineRedisCache = NewKlineRedisCache()

type klineRedisCache struct {
	expired int //seconds
}

func NewKlineRedisCache() *klineRedisCache {
	return &klineRedisCache{
		expired: 3600 * 24 * 30,
	}
}

func (ks *klineRedisCache) key(chainId constants.ChainId, pairAddr string, interval constants.KlineInterval) string {
	return fmt.Sprintf("kline:%v:%s:%s", chainId, pairAddr, interval)
}

func (ks *klineRedisCache) compress(pb *datakline.Kline) string {
	return strings.Join(
		[]string{
			pb.PairAddr,
			util.Int64ToString(pb.CandleTime),
			util.Float64ToString(pb.Open),
			util.Float64ToString(pb.Close),
			util.Float64ToString(pb.High),
			util.Float64ToString(pb.Low),
			util.Float64ToString(pb.McapOpen),
			util.Float64ToString(pb.McapHigh),
			util.Float64ToString(pb.McapLow),
			util.Float64ToString(pb.McapClose),
			util.Float64ToString(pb.AmountUsd),
			util.Float64ToString(pb.VolumeToken),
			strconv.Itoa(int(pb.BuyCount)),
			strconv.Itoa(int(pb.SellCount)),
			strconv.Itoa(int(pb.TotalCount)),
		},
		",")
}

func (ks *klineRedisCache) decompress(str string) (*datakline.Kline, error) {
	parts := strings.Split(str, ",")
	if len(parts) != 15 {
		logx.Errorf("redismodel kline invalid string")
		return nil, fmt.Errorf("redismodel kline invalid string")
	}

	return &datakline.Kline{
		PairAddr:    parts[0],
		CandleTime:  util.StringToInt64(parts[1]),
		Open:        util.StringToFloat64(parts[2]),
		Close:       util.StringToFloat64(parts[3]),
		High:        util.StringToFloat64(parts[4]),
		Low:         util.StringToFloat64(parts[5]),
		McapOpen:    util.StringToFloat64(parts[6]),
		McapHigh:    util.StringToFloat64(parts[7]),
		McapLow:     util.StringToFloat64(parts[8]),
		McapClose:   util.StringToFloat64(parts[9]),
		AmountUsd:   util.StringToFloat64(parts[10]),
		VolumeToken: util.StringToFloat64(parts[11]),
		BuyCount:    util.StringToInt64(parts[12]),
		SellCount:   util.StringToInt64(parts[13]),
		TotalCount:  util.StringToInt64(parts[14]),
	}, nil
}

var KlineRedisConsumer = NewKlineRedisConsumer()

type klineRedisConsumer struct {
	ch chan *datakline.Klines
}

func NewKlineRedisConsumer() *klineRedisConsumer {
	ksc := klineRedisConsumer{
		ch: make(chan *datakline.Klines, 200000),
	}
	go ksc.consume()
	return &ksc
}

func (krc *klineRedisConsumer) Put(kline *datakline.Klines) {
	krc.ch <- kline
}

func (krc *klineRedisConsumer) consume() {
	ticker := time.NewTicker(time.Millisecond * 100)
	m := make(map[string][]*datakline.Kline)
	pool, _ := ants.NewPool(30)
	for {
		select {
		case <-ticker.C:
			list := make([][]*datakline.Kline, 0)
			for _, v := range m {
				list = append(list, v)
			}
			m = make(map[string][]*datakline.Kline)

			for _, klines := range list {
				_ = pool.Submit(func() {
					for _, k := range klines {
						if k != nil {
							config.KlineProcessedCh <- KlineRedisCache.NewPush(context.TODO(), k)
						}
					}
				})
			}

		case kline := <-krc.ch:
			for _, k := range kline.List {
				if k.CloseAt == 0 {
					logx.Debugf("kline closeAt is zero %v candleTime:%v", k.PairAddr, k.CandleTime)
				}

				key := k.PairAddr + strconv.Itoa(int(k.CandleTime))
				m[key] = append(m[key], k)
			}
		}
	}
}

func (ks *klineRedisCache) NewPush(ctx context.Context, kline *datakline.Kline) *datakline.Kline {
	key := ks.key(constants.ChainId(kline.ChainId), kline.PairAddr, constants.KlineInterval(kline.Interval))
	return ks.newPush(ctx, key, kline)
}

func (ks *klineRedisCache) newPush(ctx context.Context, key string, kline *datakline.Kline) (newKline *datakline.Kline) {
	locker, err := rds.MustLock(ctx, rds.Client, fmt.Sprintf("klinelock:%v", key), 10, 3)
	if err != nil {
		logx.Errorf("lpush kline to redismodel err,get Lock:%v", err)
		return
	}
	defer locker.Release()
	expire := constants2.SevenDaysSeconds
	if kline.Interval == string(constants.KlineMin1) {
		expire = constants2.ThreeDaysSeconds
	}
	defer rds.Client.ExpireCtx(ctx, key, expire)

	newKline = kline
	v, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, kline.CandleTime, kline.CandleTime)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			_, err = rds.Client.ZaddCtx(ctx, key, kline.CandleTime, ks.compress(newKline))
			if err != nil {
				logx.Errorf("ZaddCtx MergeKline key:%v err:%v", key, err)
			}
			return
		}
		logx.Errorf("Zrangebyscore found %v err:%v", kline.PairAddr, err)
		return
	}

	if len(v) == 0 {
		_, err = rds.Client.ZaddCtx(ctx, key, kline.CandleTime, ks.compress(newKline))
		if err != nil {
			logx.Errorf("ZaddCtx MergeKline key:%v err:%v", key, err)
		}
	} else {
		oldKline, err := ks.decompress(v[0].Key)
		if err != nil {
			logx.Errorf("decompress err:%v", err)
			return
		}

		_, err = rds.Client.ZremrangebyscoreCtx(ctx, key, kline.CandleTime, kline.CandleTime)
		if err != nil {
			logx.Errorf("ZremrangebyscoreCtx key:%v err:%v", key, err)
		}

		newKline = mergeKline(oldKline, kline)
		_, err = rds.Client.ZaddCtx(ctx, key, kline.CandleTime, ks.compress(newKline))
		if err != nil {
			logx.Errorf("ZaddCtx MergeKline key:%v err:%v", key, err)
		}
	}

	daysBefore := time.Now().Unix() - int64(constants2.SevenDaysSeconds)
	if kline.Interval == string(constants.KlineMin1) {
		daysBefore = time.Now().Unix() - int64(constants2.ThreeDaysSeconds)
	}

	count, _ := rds.Client.ZcountCtx(ctx, key, 0, daysBefore)
	if count > 300 {
		_, err = rds.Client.ZremrangebyscoreCtx(ctx, key, 0, daysBefore)
		logx.Infof("ZremrangebyscoreCtx key: %v, count: %v", key, count)
	}

	return
}

func (ks *klineRedisCache) FetchKline(ctx context.Context, option KlineQueryOption) ([]*datakline.Kline, bool) {
	key := ks.key(option.ChainId, option.PairAddr, option.Interval)
	list, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, option.From, option.To)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false
		}
		logx.Errorf("FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return nil, false
	}

	res := make([]*datakline.Kline, 0, option.Limit)
	for _, str := range list {
		kline, err := ks.decompress(str.Key)
		if err != nil {
			logx.Errorf("decompress err:%v", err)
			continue
		}
		kline.ChainId = int64(option.ChainId)
		kline.Interval = string(option.Interval)
		if kline.CandleTime < option.To {
			res = append(res, kline)
		}
		if len(res) >= int(option.Limit) {
			break
		}
	}
	return res, true
}

func (ks *klineRedisCache) GetKlineFluctuation(ctx context.Context, option KlineQueryOption) float64 {
	key := ks.key(option.ChainId, option.PairAddr, "1m")
	list, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, option.From, option.To)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0
		}
		logx.Errorf("FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return 0
	}

	if len(list) == 0 {
		return 0
	}

	latestKline, err := ks.decompress(list[len(list)-1].Key)
	if err != nil {
		logx.Errorf("GetKlineFluctuation decompress key:%v err:%v", key, err)
		return 0
	}

	oldestKline, err := ks.decompress(list[0].Key)
	if err != nil {
		logx.Errorf("GetKlineFluctuation decompress key:%v err:%v", key, err)
		return 0
	}

	if oldestKline.Open == 0 {
		return 0
	}

	result, _ := decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(oldestKline.Open)).Div(decimal.NewFromFloat(oldestKline.Open)).Float64()
	if result > 999.99 {
		result = 999.99
	}

	return result
}

func (ks *klineRedisCache) GetKline24Info(ctx context.Context, option KlineQueryOption) (result *datakline.Kline24InfoItem) {
	result = &datakline.Kline24InfoItem{}
	key := ks.key(option.ChainId, option.PairAddr, "1m")
	list, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, option.From, option.To)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		logx.Errorf("FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return
	}

	if len(list) == 0 {
		return
	}

	latestKline, err := ks.decompress(list[len(list)-1].Key)
	if err != nil {
		logx.Errorf("GetKlineFluctuation decompress key:%v err:%v", key, err)
		return
	}

	oldestKline, err := ks.decompress(list[0].Key)
	if err != nil {
		logx.Errorf("GetKlineFluctuation decompress key:%v err:%v", key, err)
		return
	}

	if oldestKline.Open == 0 {
		return
	}

	change, _ := decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(oldestKline.Open)).Div(decimal.NewFromFloat(oldestKline.Open)).Float64()
	if change > 999.99 {
		change = 999.99
	}

	result.Change_24 = change

	for _, info := range list {
		kline, _ := ks.decompress(info.Key)
		result.Vol_24H += kline.AmountUsd
		result.Txs_24H += uint32(kline.TotalCount)
	}

	return result
}

func (ks *klineRedisCache) GetKlineAllFluctuation(ctx context.Context, option KlineQueryOption) (result *datakline.KlineFluctuation) {
	result = &datakline.KlineFluctuation{}
	key := ks.key(option.ChainId, option.PairAddr, "1m")
	list, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, option.From, option.To)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		logx.Errorf("FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return
	}

	if len(list) == 0 {
		return
	}

	latestKline, err := ks.decompress(list[len(list)-1].Key)
	if err != nil {
		logx.Errorf("GetKlineFluctuation decompress key:%v err:%v", key, err)
		return
	}

	oldestKline, err := ks.decompress(list[0].Key)
	if err != nil {
		logx.Errorf("GetKlineFluctuation decompress key:%v err:%v", key, err)
		return
	}

	if oldestKline.Open == 0 {
		return
	}

	flu, _ := decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(oldestKline.Open)).Div(decimal.NewFromFloat(oldestKline.Open)).Float64()
	if flu > 999.99 {
		flu = 999.99
	}

	result.KlineFluctuation_24H = flu

	if latestKline.Open == 0 {
		return result
	}
	flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(latestKline.Open)).Div(decimal.NewFromFloat(latestKline.Open)).Float64()
	if flu > 999.99 {
		flu = 999.99
	}
	result.KlineFluctuation_1M = flu

	for _, info := range list {
		kline, _ := ks.decompress(info.Key)

		if latestKline.CandleTime-kline.CandleTime == int64(constants.IntervalHour12) {
			if kline.Close > 0 && kline.Open > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_12H = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime == int64(constants.IntervalHour4) {
			if kline.Close > 0 && kline.Open > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_4H = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime == int64(constants.IntervalHour1) {
			if kline.Close > 0 && kline.Open > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_1H = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime == int64(constants.IntervalMin15) {
			if kline.Close > 0 && kline.Open > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_15M = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime == int64(constants.IntervalMin5) {
			if kline.Close == 0 && kline.Open > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
			}
			result.KlineFluctuation_5M = flu
			continue
		}
	}

	return result
}

func mergeKline(oldKline, kline *datakline.Kline) (newKline *datakline.Kline) {
	if oldKline == nil || kline == nil {
		return
	}

	if oldKline.CandleTime != kline.CandleTime {
		return
	}

	newKline = kline
	if newKline.OpenAt > oldKline.OpenAt {
		newKline.OpenAt = oldKline.OpenAt
		newKline.Open = oldKline.Open
		newKline.McapOpen = oldKline.McapOpen
	}

	if newKline.CloseAt < oldKline.CloseAt {
		newKline.CloseAt = oldKline.CloseAt
		newKline.Close = oldKline.Close
		newKline.McapClose = oldKline.McapClose
	}

	if newKline.High < oldKline.High {
		newKline.High = oldKline.High
		newKline.McapHigh = oldKline.McapHigh
	}

	if newKline.Low > oldKline.Low {
		newKline.Low = oldKline.Low
		newKline.McapLow = oldKline.McapLow
	}

	newKline.AmountUsd += oldKline.AmountUsd
	newKline.VolumeToken += oldKline.VolumeToken
	newKline.TotalCount += oldKline.TotalCount
	newKline.BuyCount += oldKline.BuyCount
	newKline.SellCount += oldKline.SellCount
	return
}
