package cache

import (
	"context"
	"dex/market/internal/config"
	"dex/market/internal/model"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"dex/market/internal/constants"
	"dex/market/internal/data"

	datakline "dex/market/market"
	rds "dex/market/pkg/redis"
	"dex/market/pkg/util"

	"github.com/panjf2000/ants/v2"
	"github.com/zeromicro/go-zero/core/logc"
	goredis "github.com/zeromicro/go-zero/core/stores/redis"
)

var KlineRedisCache *klineRedisCache

type klineRedisCache struct {
	expired int //seconds
	db      *gorm.DB
}

func NewKlineRedisCache(db *gorm.DB) *klineRedisCache {
	return &klineRedisCache{
		expired: 3600 * 24 * 30,
		db:      db,
	}
}

// Fetch
func (ks *klineRedisCache) Fetch(ctx context.Context, option KlineQueryOption) ([]*datakline.Kline, bool) {
	key := ks.key(option.ChainId, option.PairAddr, option.Interval)

	v, err := rds.Client.LindexCtx(ctx, key, -1)
	if errors.Is(err, goredis.Nil) {
		klines, err := ks.loadToRedis(ctx, key, KlineQueryOption{
			ChainId:  option.ChainId,
			PairAddr: option.PairAddr,
			Interval: option.Interval,
			To:       option.To,
			Limit:    1500,
		})
		if err != nil {
			return []*datakline.Kline{}, true
		}
		res := make([]*datakline.Kline, 0, option.Limit)
		for _, kline := range klines {
			if kline.CandleTime < option.To {
				res = append(res, kline)
			}
			if len(res) >= int(option.Limit) {
				break
			}
		}
		return res, true
	}

	oldestKline, err := ks.decompress(v)
	if err != nil || option.To <= oldestKline.CandleTime {
		logc.Errorf(ctx, "lindex redismodel kline  err:%v", err)
		return nil, false
	}
	kline, err := ks.decompress(v)
	if err != nil {
		return nil, false
	}
	if kline.CandleTime >= option.To {
		return nil, false
	}

	list, err := rds.Client.LrangeCtx(ctx, key, 0, -1)
	if err != nil {
		return nil, false
	}
	res := make([]*datakline.Kline, 0, option.Limit)
	for _, str := range list {
		kline, err := ks.decompress(str)
		if err != nil {
			logc.Errorf(ctx, "decompress err:%v", err)
			continue
		}
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
		return nil, fmt.Errorf("redismodel kline invalid string len,need:15 get:%d ", len(parts))
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

// queryKlineList
func (ks *klineRedisCache) queryKlineList(ctx context.Context, option KlineQueryOption) []*datakline.Kline {
	// Use MySQL instead of ClickHouse
	mysqlRepo := data.NewKlineMysqlRepo(ks.db)

	// Convert time to Unix timestamp (ClickHouse used nanoseconds, MySQL uses seconds)
	toTime := option.To
	if toTime == 0 {
		toTime = time.Now().Unix()
	}

	// Query from MySQL - we need to calculate fromTime based on limit and interval
	fromTime := toTime - int64(option.Limit)*int64(constants.KlineIntervalSecondsMap[string(option.Interval)])

	dbKlines, err := mysqlRepo.QueryKline(ctx, option.Interval, int64(option.ChainId), option.PairAddr, fromTime, toTime, int(option.Limit))
	if err != nil {
		logc.Errorf(ctx, "QueryKline from MySQL err:%v", err)
	}

	klines := make([]*datakline.Kline, 0, len(dbKlines))
	for _, kline := range dbKlines {
		klines = append(klines, &datakline.Kline{
			PairAddr:    kline.PairAddress,
			CandleTime:  kline.CandleTime,
			Open:        kline.Open,
			Close:       kline.Close,
			High:        kline.High,
			Low:         kline.Low,
			McapOpen:    0,            // MySQL model doesn't have mcap fields, set to 0
			McapHigh:    0,            // MySQL model doesn't have mcap fields, set to 0
			McapLow:     0,            // MySQL model doesn't have mcap fields, set to 0
			McapClose:   0,            // MySQL model doesn't have mcap fields, set to 0
			AmountUsd:   kline.Volume, // MySQL uses 'Volume' field for USD amount
			VolumeToken: kline.Tokens, // MySQL uses 'Tokens' field for token volume
			BuyCount:    kline.BuyCount,
			SellCount:   kline.SellCount,
		})
	}
	return klines
}

// saveToRedis
func (ks *klineRedisCache) saveToRedis(ctx context.Context, key string, klines []*datakline.Kline) (int, error) {
	values := make([]any, 0, len(klines))

	for _, kline := range klines {
		values = append(values, ks.compress(kline))
	}
	count, err := rds.RPushWithExpired(ctx, key, ks.expired, values...)
	if err != nil {
		logc.Errorf(ctx, "lpush kline to redismodel err:%v", err)
	}
	return count, nil
}

func (ks *klineRedisCache) loadToRedis(ctx context.Context, key string, option KlineQueryOption) ([]*datakline.Kline, error) {
	cacheKlines := KlineCache.GetAll(ctx, option.ChainId, option.Interval, option.PairAddr)
	if len(cacheKlines) == 0 {
		return nil, errors.New("database no kline")
	}
	oldest := cacheKlines[len(cacheKlines)-1]
	klines := ks.queryKlineList(ctx, KlineQueryOption{
		ChainId:  option.ChainId,
		PairAddr: option.PairAddr,
		Interval: option.Interval,
		To:       oldest.CandleTime,
		Limit:    int64(1500 - len(cacheKlines)),
	})
	klines = append(cacheKlines, klines...)

	locker, err := rds.MustLock(ctx, rds.Client, fmt.Sprintf("klinelock:%v", key), 10, 3)
	if err != nil {
		return nil, err
	}
	defer locker.Release()

	_, err = rds.Client.LindexCtx(ctx, key, -1)
	if errors.Is(err, goredis.Nil) {
		_, err = ks.saveToRedis(ctx, key, klines)
	}
	return klines, err
}

// push
func (ks *klineRedisCache) push(ctx context.Context, key string, kline *datakline.Kline) int {
	locker, err := rds.MustLock(ctx, rds.Client, fmt.Sprintf("klinelock:%v", key), 10, 3)
	if err != nil {
		logc.Errorf(ctx, "lpush kline to redismodel err,get Lock:%v", err)
		return 0
	}
	v, err := rds.Client.LpopCtx(ctx, key)
	if err != nil {
		logc.Errorf(ctx, "LpopCtx found  %v err:%v", kline.PairAddr, err)
		locker.Release()
		_, err := ks.loadToRedis(ctx, key, KlineQueryOption{
			ChainId:  constants.ChainId(kline.ChainId),
			PairAddr: kline.PairAddr,
			Interval: constants.KlineInterval(kline.Interval),
			To:       0,
			Limit:    1500,
		})
		if err != nil {
			logc.Errorf(ctx, "load %v kline to redismodel err:%v", kline.PairAddr, err)
		}
		return 0
	}
	defer locker.Release()
	last, err := ks.decompress(v)
	if err != nil {
		logc.Errorf(ctx, "decompress err:%v", err)
		ks.remove(ctx, key)
		count, _ := rds.LPushWithExpired(ctx, key, ks.expired, ks.compress(kline))
		return count
	}
	if kline.CandleTime < last.CandleTime {
		logc.Errorf(ctx, "kline.CandleTime< last.CandleTime %s", kline.PairAddr)
	}
	if errors.Is(err, goredis.Nil) || kline.CloseAt <= last.CloseAt || kline.CandleTime < last.CandleTime {
		count, _ := rds.LPushWithExpired(ctx, key, ks.expired, ks.compress(last))
		return count
	} else if kline.CandleTime == last.CandleTime {
		count, _ := rds.LPushWithExpired(ctx, key, ks.expired, ks.compress(kline))
		return count
	} else {
		count, _ := rds.LPushWithExpired(ctx, key, ks.expired, ks.compress(last), ks.compress(kline))
		return count
	}
}

func (ks *klineRedisCache) trim(ctx context.Context, key string) {
	err := rds.Client.LtrimCtx(ctx, key, 0, 1499)
	if err != nil {
		logc.Errorf(ctx, "Rpop kline err:%v", err)
	}
}

func (ks *klineRedisCache) remove(ctx context.Context, key string) {
	rds.Client.DelCtx(ctx, key)
}

func (ks *klineRedisCache) Push(ctx context.Context, kline *datakline.Kline) {
	key := ks.key(constants.ChainId(kline.ChainId), kline.PairAddr, constants.KlineInterval(kline.Interval))

	count := ks.push(ctx, key, kline)
	logc.Infof(ctx, "LPush %v kline count:%v", kline.PairAddr, count)
	if count > 1500 {
		ks.trim(ctx, key)
	}
}

var KlineRedisConsumer = NewKlineRedisConsumer()

type klineRedisConsumer struct {
	ch chan *model.KlineWithTrade
}

func NewKlineRedisConsumer() *klineRedisConsumer {
	ksc := klineRedisConsumer{
		ch: make(chan *model.KlineWithTrade, 200000),
	}
	//go ksc.consume()
	return &ksc
}

func (krc *klineRedisConsumer) Put(kline *model.KlineWithTrade) {
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
			for _, k := range kline.Klines {
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
	locker, err := rds.MustLock(ctx, rds.Client, fmt.Sprintf("klinelock:%v", key), 10, 10)
	if err != nil {
		logc.Errorf(ctx, "lpush kline to redismodel err,get Lock:%v", err)
		return
	}
	defer locker.Release()

	newKline = kline
	v, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, kline.CandleTime, kline.CandleTime)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			_, err = rds.Client.ZaddCtx(ctx, key, kline.CandleTime, ks.compress(newKline))
			if err != nil {
				logc.Errorf(ctx, "ZaddCtx MergeKline key:%v err:%v", key, err)
			}
			return
		}
		logc.Errorf(ctx, "Zrangebyscore found %v err:%v", kline.PairAddr, err)
		return
	}

	if len(v) == 0 {
		_, err = rds.Client.ZaddCtx(ctx, key, kline.CandleTime, ks.compress(newKline))
		if err != nil {
			logc.Errorf(ctx, "ZaddCtx MergeKline key:%v err:%v", key, err)
		}
	} else {
		oldKline, err := ks.decompress(v[0].Key)
		if err != nil {
			logc.Errorf(ctx, "decompress err:%v", err)
			return
		}

		_, err = rds.Client.ZremrangebyscoreCtx(ctx, key, kline.CandleTime, kline.CandleTime)
		if err != nil {
			logc.Errorf(ctx, "ZremrangebyscoreCtx key:%v err:%v", key, err)
		}

		newKline = mergeKline(oldKline, kline)
		_, err = rds.Client.ZaddCtx(ctx, key, kline.CandleTime, ks.compress(newKline))
		if err != nil {
			logc.Errorf(ctx, "ZaddCtx MergeKline key:%v err:%v", key, err)
		}
	}

	return
}

func (ks *klineRedisCache) FetchKline(ctx context.Context, option KlineQueryOption) ([]*datakline.Kline, bool) {
	key := ks.key(option.ChainId, option.PairAddr, option.Interval)
	list, err := rds.Client.ZrevrangebyscoreWithScoresCtx(ctx, key, option.From, option.To)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false
		}
		logc.Errorf(ctx, "FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return nil, false
	}

	res := make([]*datakline.Kline, 0, option.Limit)
	for _, str := range list {
		kline, err := ks.decompress(str.Key)
		if err != nil {
			logc.Errorf(ctx, "decompress err:%v", err)
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
		logc.Errorf(ctx, "FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return 0
	}

	if len(list) == 0 {
		return 0
	}

	latestKline, err := ks.decompress(list[len(list)-1].Key)
	if err != nil {
		logc.Errorf(ctx, "GetKlineFluctuation decompress key:%v err:%v", key, err)
		return 0
	}

	oldestKline, err := ks.decompress(list[0].Key)
	if err != nil {
		logc.Errorf(ctx, "GetKlineFluctuation decompress key:%v err:%v", key, err)
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
		logc.Errorf(ctx, "FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return
	}

	if len(list) == 0 {
		return
	}

	latestKline, err := ks.decompress(list[len(list)-1].Key)
	if err != nil {
		logc.Errorf(ctx, "GetKlineFluctuation decompress key:%v err:%v", key, err)
		return
	}

	oldestKline, err := ks.decompress(list[0].Key)
	if err != nil {
		logc.Errorf(ctx, "GetKlineFluctuation decompress key:%v err:%v", key, err)
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
		logc.Errorf(ctx, "FetchKline ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return
	}

	if len(list) == 0 {
		return
	}

	latestKline, err := ks.decompress(list[len(list)-1].Key)
	if err != nil {
		logc.Errorf(ctx, "GetKlineFluctuation decompress key:%v err:%v", key, err)
		return
	}

	oldestKline, err := ks.decompress(list[0].Key)
	if err != nil {
		logc.Errorf(ctx, "GetKlineFluctuation decompress key:%v err:%v", key, err)
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

		if latestKline.CandleTime-kline.CandleTime <= int64(constants.IntervalHour12) && result.KlineFluctuation_12H == 0 {
			if kline.Close > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_12H = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime <= int64(constants.IntervalHour4) && result.KlineFluctuation_4H == 0 {
			if kline.Close > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_4H = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime <= int64(constants.IntervalHour1) && result.KlineFluctuation_1H == 0 {
			if kline.Close > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_1H = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime <= int64(constants.IntervalMin15) && result.KlineFluctuation_15M == 0 {
			if kline.Close > 0 {
				flu, _ = decimal.NewFromFloat(latestKline.Close).Sub(decimal.NewFromFloat(kline.Open)).Div(decimal.NewFromFloat(kline.Open)).Float64()
				if flu > 999.99 {
					flu = 999.99
				}
				result.KlineFluctuation_15M = flu
			}
			continue
		}

		if latestKline.CandleTime-kline.CandleTime <= int64(constants.IntervalMin5) && result.KlineFluctuation_5M == 0 {
			if kline.Close == 0 {
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

func (ks *klineRedisCache) GetPairMcapHigh(ctx context.Context, option KlineQueryOption) (result float64) {
	key := ks.key(option.ChainId, option.PairAddr, "1m")
	list, err := rds.Client.ZrangebyscoreWithScoresCtx(ctx, key, option.From, time.Now().Unix())
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		logc.Errorf(ctx, "GetPairMcapHigh ZrangebyscoreWithScoresCtx key:%v err:%v", key, err)
		return
	}

	if len(list) == 0 {
		return
	}

	for _, info := range list {
		kline, _ := ks.decompress(info.Key)
		if result < kline.McapHigh {
			result = kline.McapHigh
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

	//newTotalPrice := decimal.NewFromFloat(newKline.AvgPrice).Mul(decimal.NewFromInt(newKline.TotalCount))
	//klineTotalPrice := decimal.NewFromFloat(oldKline.AvgPrice).Mul(decimal.NewFromInt(oldKline.TotalCount))
	//newKline.AvgPrice, _ = newTotalPrice.Add(klineTotalPrice).Div(decimal.NewFromInt(newKline.TotalCount).Add(decimal.NewFromInt(oldKline.TotalCount))).Float64()

	newKline.AmountUsd += oldKline.AmountUsd
	newKline.VolumeToken += oldKline.VolumeToken
	newKline.TotalCount += oldKline.TotalCount
	newKline.BuyCount += oldKline.BuyCount
	newKline.SellCount += oldKline.SellCount
	return
}
