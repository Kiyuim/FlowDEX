package logic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"dex/market/internal/cache"
	"dex/market/internal/constants"
	"dex/market/internal/data"
	"dex/market/internal/svc"
	"dex/market/market"
	"dex/market/marketclient"
	constants2 "dex/pkg/constants"
	"dex/pkg/xcode"

	"github.com/klen-ygs/gorm-zero/gormc"
	"golang.org/x/sync/singleflight"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetKlineLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetKlineLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetKlineLogic {
	return &GetKlineLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

var klineSingleLight = new(singleflight.Group)

// Get K-line data
func (l *GetKlineLogic) GetKline(in *market.GetKlineRequest) (*marketclient.GetKlineResponse, error) {
	fmt.Println("input is:", in)
	// Honor the caller's actual window/limit — this used to hardcode the
	// last 24h and limit=100 regardless of what was requested, so e.g. a 1d
	// interval could only ever return ~1 candle no matter how much history
	// existed. Only fall back to defaults when the caller didn't specify.
	fromTimestamp := in.FromTimestamp
	toTimestamp := in.ToTimestamp
	limit := in.Limit
	if toTimestamp <= 0 {
		toTimestamp = time.Now().Unix()
	}
	if fromTimestamp <= 0 {
		fromTimestamp = time.Now().Add(-24 * time.Hour).Unix()
	}
	if limit <= 0 {
		limit = 100
	}
	input := &market.GetKlineRequest{
		ChainId:       in.ChainId,
		PairAddress:   in.PairAddress,
		Interval:      in.Interval,
		FromTimestamp: fromTimestamp,
		ToTimestamp:   toTimestamp,
		Limit:         limit,
	}
	klines, err := l.doChanGetKlineData(l.ctx, klineSingleLight, input)
	if err != nil {
		logx.Errorf("1111GetKline doChanGetData error: %v", err)
		return nil, err
	}

	res := &marketclient.GetKlineResponse{}
	for _, kline := range klines.List {
		res.List = append(res.List, &marketclient.NewKline{
			Open:        kline.Open,
			High:        kline.High,
			Low:         kline.Low,
			Close:       kline.Close,
			CandleTime:  kline.CandleTime,
			VolumeToken: kline.VolumeToken,
		})
	}

	return res, nil
}

func (l *GetKlineLogic) doChanGetKlineData(ctx context.Context, g *singleflight.Group, in *marketclient.GetKlineRequest) (*marketclient.Klines, error) {
	fmt.Println("option is:000")

	ch := g.DoChan(fmt.Sprintf("%s_%s_%d_%d", in.PairAddress, in.Interval, in.ToTimestamp, in.Limit), func() (interface{}, error) {
		ret, err := l.getKline(in)
		return ret, err
	})
	select {
	case <-ctx.Done():
		fmt.Println("option is:333")
		return &marketclient.Klines{}, ctx.Err()
	case ret := <-ch:
		fmt.Println("option is:444")
		return ret.Val.(*marketclient.Klines), ret.Err
	}
}

func (l *GetKlineLogic) getKline(in *marketclient.GetKlineRequest) (*marketclient.Klines, error) {
	if _, exist := constants.KlineIntervalMap[in.Interval]; !exist {
		fmt.Println("option is:111")

		return nil, xcode.RequestError
	}

	if in.FromTimestamp > in.ToTimestamp || in.FromTimestamp <= 0 || in.ToTimestamp <= 0 || in.Limit <= 0 {
		fmt.Println("option is:222")

		return nil, xcode.RequestError
	}

	countIntervals := (in.ToTimestamp - in.FromTimestamp) / int64(constants2.KlineIntervalSecondsMap[in.Interval])
	if countIntervals > in.Limit {
		countIntervals = in.Limit
	}

	option := cache.KlineQueryOption{
		ChainId:  constants.ChainId(in.ChainId),
		PairAddr: in.PairAddress,
		Interval: constants.KlineInterval(in.Interval),
		From:     in.FromTimestamp,
		To:       in.ToTimestamp,
		Limit:    countIntervals,
	}

	fmt.Println("option is:", option)

	klines, ok := cache.KlineRedisCache.FetchKline(l.ctx, option)
	if !ok {
		return &marketclient.Klines{
			List: klines,
		}, nil
	}

	if len(klines) == 0 {
		if in.FromTimestamp < 1740758400 {
			in.FromTimestamp = 0
		}
		if in.ToTimestamp < 1740758400 {
			in.ToTimestamp = 0
		}
		klineDb, err := data.NewKlineMysqlRepo(l.svcCtx.DB).QueryKline(l.ctx, constants.KlineInterval(in.Interval), in.ChainId, in.PairAddress, in.FromTimestamp, in.ToTimestamp, int(countIntervals))
		if err != nil && !errors.Is(err, gormc.ErrNotFound) {
			logx.Errorf("Failed to QueryKline: %v", err)
			return nil, err
		}

		if errors.Is(err, gormc.ErrNotFound) || len(klineDb) == 0 {
			klineDb, err = data.NewKlineMysqlRepo(l.svcCtx.DB).QueryKline(l.ctx, constants.KlineInterval(in.Interval), in.ChainId, in.PairAddress, 0, in.ToTimestamp, int(countIntervals))
			if err != nil && !errors.Is(err, gormc.ErrNotFound) {
				logx.Errorf("Failed to QueryKline: %v", err)
				return nil, err
			}

			if len(klineDb) > 0 {
				klineDb = klineDb[:1]
			}
		}

		if len(klineDb) == 0 {
			return &marketclient.Klines{
				List: klines,
			}, nil
		}

		for _, kline := range klineDb {
			klines = append(klines, &market.Kline{
				ChainId:     kline.ChainId,
				Interval:    in.Interval,
				PairAddr:    in.PairAddress,
				Open:        kline.Open,
				High:        kline.High,
				Low:         kline.Low,
				Close:       kline.Close,
				AmountUsd:   kline.Volume,
				VolumeToken: kline.Tokens,
				CandleTime:  kline.CandleTime,
				BuyCount:    kline.BuyCount,
				SellCount:   kline.SellCount,
				TotalCount:  kline.Count,
				AvgPrice:    kline.AvgPrice,
				OpenAt:      kline.OpenAt,
				CloseAt:     kline.CloseAt,
			})
		}
	}

	sort.Slice(klines, func(i, j int) bool {
		return klines[i].CandleTime > klines[j].CandleTime
	})

	if len(klines) < int(countIntervals) {
		klines = l.supplementResult(klines, in.FromTimestamp, in.ToTimestamp, in.Interval, countIntervals)
		sort.Slice(klines, func(i, j int) bool {
			return klines[i].CandleTime > klines[j].CandleTime
		})

		if len(klines) > int(in.Limit) {
			klines = klines[len(klines)-int(in.Limit):]
		}
	}

	l.prettyResult(klines, countIntervals)

	return &marketclient.Klines{
		List: klines,
	}, nil
}

func (l *GetKlineLogic) supplementResult(list []*marketclient.Kline, fromTime, toTime int64, interval string, count int64) []*marketclient.Kline {
	if len(list) <= 0 {
		return nil
	}

	intervalSecond := int64(constants2.KlineIntervalSecondsMap[interval])

	inner := make([]*marketclient.Kline, 0)
	for i := 0; i < len(list)-1; i++ {
		currentKline := list[i]
		nextKline := list[i+1]
		timeDiff := currentKline.CandleTime - nextKline.CandleTime

		// 如果时间间隔不等于预期的 interval，补充缺失的 Kline
		if timeDiff > intervalSecond {
			// 补充缺失的 Kline
			for j := nextKline.CandleTime + intervalSecond; j < currentKline.CandleTime; j += intervalSecond {
				missingKline := &marketclient.Kline{
					ChainId:    currentKline.ChainId,
					Interval:   interval,
					PairAddr:   currentKline.PairAddr,
					Open:       currentKline.Open,
					High:       currentKline.Open,
					Low:        currentKline.Open,
					Close:      currentKline.Open,
					CandleTime: j,
				}
				inner = append(inner, missingKline)
			}
		}
	}

	oldestKline := list[len(list)-1]
	if oldestKline.CandleTime > fromTime {
		num := (oldestKline.CandleTime - fromTime) / intervalSecond
		num = min(num, count)
		for i := 1; i < int(num)+1; i++ {
			list = append(list, &marketclient.Kline{
				ChainId:    oldestKline.ChainId,
				Interval:   oldestKline.Interval,
				PairAddr:   oldestKline.PairAddr,
				Open:       oldestKline.Open,
				High:       oldestKline.Open,
				Low:        oldestKline.Open,
				Close:      oldestKline.Open,
				CandleTime: oldestKline.CandleTime - int64(i)*intervalSecond,
			})
		}
	}

	if list[0].CandleTime < toTime {
		num := (toTime - list[0].CandleTime) / intervalSecond
		for i := 1; i < int(num)+1; i++ {
			list = append(list, &marketclient.Kline{
				ChainId:    list[0].ChainId,
				Interval:   list[0].Interval,
				PairAddr:   list[0].PairAddr,
				Open:       list[0].Close,
				High:       list[0].Close,
				Low:        list[0].Close,
				Close:      list[0].Close,
				CandleTime: list[0].CandleTime + int64(i)*intervalSecond,
			})
		}
	}

	list = append(list, inner...)
	return list
}

func (l *GetKlineLogic) prettyResult(list []*marketclient.Kline, count int64) {
	if len(list) <= 0 {
		return
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CandleTime < list[j].CandleTime
	})

	frontInfo := list[0]
	if len(list) > int(count) {
		list = list[1:]
		list[0].Open = frontInfo.Close
		list[0].McapOpen = frontInfo.McapClose
		frontInfo = list[0]
	}

	for i := 1; i < len(list); i++ {
		list[i].Open = frontInfo.Close
		list[i].McapOpen = frontInfo.McapClose
		frontInfo = list[i]
	}

	for i := 1; i < len(list); i++ {
		list[i].High = max(list[i].High, list[i].Open)
		list[i].Low = min(list[i].Low, list[i].Open)
		list[i].McapHigh = max(list[i].McapHigh, list[i].McapOpen)
		list[i].McapLow = min(list[i].McapLow, list[i].McapOpen)
	}
}
