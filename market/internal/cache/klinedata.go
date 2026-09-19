package cache

import (
	"context"
	"sync"
	"time"

	"dex/market/internal/constants"
	"dex/market/internal/svc"
	pb "dex/market/market"
	"dex/market/pkg/util"

	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mathx"
)

type KlineData struct {
	klines       *collection.Cache // pairAddr -> ring
	fluctuations *collection.Cache // pairAddr -> KlineFluctuation
	sc           *svc.ServiceContext
	limit        int64
	chainId      constants.ChainId
	interval     constants.KlineInterval
	loadFunc     LoadFunc
	lock         sync.RWMutex
}

const (
	RingSize = 501
)

type klineConfig struct {
	chainId  constants.ChainId
	interval constants.KlineInterval
	name     string
	limit    int
	expire   time.Duration
}

type cacheFluctuation struct {
	latestTime  int64
	tradeCount  int64
	fluctuation *pb.KlineFluctuation
}

func NewKlineData(sc *svc.ServiceContext, config klineConfig, f LoadFunc) *KlineData {
	klines, _ := collection.NewCache(config.expire,
		collection.WithLimit(config.limit),
		collection.WithName(config.name))

	fluctuations, _ := collection.NewCache(config.expire,
		collection.WithLimit(config.limit),
		collection.WithName(config.name+"_fluctuations"))

	return &KlineData{
		klines:       klines,
		fluctuations: fluctuations,
		sc:           sc,
		limit:        int64(config.limit),
		chainId:      config.chainId,
		interval:     config.interval,
		loadFunc:     f,
	}
}

func (k *KlineData) Reload(pairAddress ...string) error {
	for _, addr := range pairAddress {
		_, err := k.load(addr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *KlineData) load(pairAddress string) (*util.Ring[*pb.Kline], error) {
	klines, err := k.loadFunc(k.chainId, pairAddress, k.interval, k.limit)

	ring := util.NewRing[*pb.Kline](RingSize)
	if len(klines) > 0 {
		ring.AddBatch(klines...)
	}

	k.klines.Set(pairAddress, ring)
	return ring, err
}

func (k *KlineData) Add(kline *pb.Kline) {
	c, ok := k.klines.Get(kline.PairAddr)
	var r *util.Ring[*pb.Kline]
	if !ok {
		// If not exists, initialize
		k.lock.Lock()
		r = util.NewRing[*pb.Kline](RingSize)
		r.Add(kline)
		k.lock.Unlock()
		k.klines.Set(kline.PairAddr, r)
		return
	}

	r = c.(*util.Ring[*pb.Kline])
	k.lock.Lock()
	defer k.lock.Unlock()

	last := r.Peek()
	if last == nil {
		r.Add(kline)
		return
	}
	// Reduce cache usage
	if kline.CandleTime == last.CandleTime && kline.TotalCount > last.TotalCount {
		r.Replace(kline)
	} else if kline.CandleTime > last.CandleTime {
		r.Add(kline)
	} else {
		//fmt.Println("kline.candletime", kline.CandleTime)
		//todo Handle data from same period but older or from previous period
	}
}

func (k *KlineData) AddFluctuation(pairAddr string, latestTime int64, tradeCount int64, fluctuation *pb.KlineFluctuation) {
	if fluctuation == nil {
		logx.Error("received nil fluctuation data")
		return
	}
	k.lock.Lock()
	defer k.lock.Unlock()

	c, ok := k.fluctuations.Get(pairAddr)
	var v *cacheFluctuation
	if !ok {
		v = &cacheFluctuation{
			latestTime:  latestTime,
			tradeCount:  tradeCount,
			fluctuation: fluctuation,
		}
		k.fluctuations.Set(pairAddr, v)
		return
	}
	v = c.(*cacheFluctuation)
	if v.latestTime < latestTime || (v.latestTime == latestTime && v.tradeCount < tradeCount) {
		v.latestTime = latestTime
		v.tradeCount = tradeCount
		v.fluctuation = fluctuation
		k.fluctuations.Set(pairAddr, v)
	} else {
		// todo
		//fmt.Println("fluctuation is old")
	}
}

func (k *KlineData) getRing(pairAddr string) *util.Ring[*pb.Kline] {
	c, ok := k.klines.Get(pairAddr)
	var r *util.Ring[*pb.Kline]
	if ok {
		r = c.(*util.Ring[*pb.Kline])
	} else {
		r, _ = k.load(pairAddr)
	}
	return r
}

func (k *KlineData) GetAll(pairAddr string) []*pb.Kline {
	r := k.getRing(pairAddr)
	return r.Take()
}

func (k *KlineData) Fetch(ctx context.Context, option KlineQueryOption) ([]*pb.Kline, bool) {
	ring := k.getRing(option.PairAddr)

	if option.To != 0 {

		k.lock.RLock()
		list := ring.Take()
		ringSize := ring.Size()
		k.lock.RUnlock()

		size := mathx.MinInt(ringSize, int(option.Limit))

		res := make([]*pb.Kline, 0, size)
		for _, kline := range list {
			if kline.CandleTime < option.To {
				res = append(res, kline)
			}
		}

		// Cache is full and not enough, remaining goes to the database
		return res, !(ringSize == ring.Cap() && len(res) < int(option.Limit))
	} else {
		k.lock.RLock()
		list := ring.TakeN(int(option.Limit))
		ringSize := ring.Size()
		k.lock.RUnlock()

		// Cache is full and exceeds capacity, remaining goes to the database
		return list, !(ringSize == ring.Cap() && int(option.Limit) > ring.Cap())
	}
}

func (k *KlineData) GetFluctuation(pairAddr string) *pb.KlineFluctuation {
	c, ok := k.fluctuations.Get(pairAddr)
	if ok {
		return c.(*cacheFluctuation).fluctuation
	}
	return nil
}
