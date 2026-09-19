package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"dex/dataflow/internal/cache"
	"dex/dataflow/internal/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"

	datakline "dex/dataflow/dataflow"
	"dex/dataflow/internal/constants"
	"dex/dataflow/internal/data"
	logic "dex/dataflow/internal/logic/kline"
	"dex/dataflow/internal/model"
	"dex/dataflow/internal/svc"

	"github.com/panjf2000/ants/v2"
	"github.com/zeromicro/go-zero/core/logx"
)

type TradeConsumer struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	klineMysqlRepo *data.KlineMysqlRepo
	workerPool     *ants.Pool
	klineCacheMu   sync.RWMutex
	cancel         context.CancelFunc
	chainId        constants.ChainId
	redisClient    *redis.Client
}

// WebSocket message structure for kline updates
type KlineUpdateMessage struct {
	PairAddress string  `json:"pair_address"`
	ChainId     int64   `json:"chain_id"`
	Interval    string  `json:"interval"`
	CandleTime  int64   `json:"candle_time"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	Timestamp   int64   `json:"timestamp"`
}

var DataflowConsumerMessageHander = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "dataflow_kafka_consumer_fetch",
		Help: "dataflow kafka consumer fetch number",
	},
)

func init() {
	prometheus.MustRegister(DataflowConsumerMessageHander)
}

func NewTradeConsumer(ctx context.Context, svcCtx *svc.ServiceContext, chainId constants.ChainId) *TradeConsumer {
	pool, _ := ants.NewPool(10)
	ctx, cancel := context.WithCancel(ctx)

	// Initialize Redis client for pub/sub. go-zero's redis.Redis (svcCtx.RDS)
	// doesn't expose Publish, so we build a plain go-redis client here — but
	// pointed at the SAME configured instance (svcCtx.Config.Redis), not a
	// second hardcoded literal that silently drifts from the real config.
	redisClient := redis.NewClient(&redis.Options{
		Addr:     svcCtx.Config.Redis.Host,
		Password: svcCtx.Config.Redis.Pass,
		DB:       0,
	})

	c := &TradeConsumer{
		ctx:            ctx,
		svcCtx:         svcCtx,
		klineMysqlRepo: data.NewKlineMysqlRepo(svcCtx.DB),
		Logger:         logx.WithContext(ctx).WithFields(logx.Field("chainId", chainId)),
		workerPool:     pool,
		cancel:         cancel,
		chainId:        chainId,
		redisClient:    redisClient,
	}
	go c.ticker()
	return c
}

func (t *TradeConsumer) Consume(ctx context.Context, key kafka.Message) error {
	// Enhanced debugging: Log raw message details
	logc.Infof(ctx, "🔔 Kafka message received - Topic: %s, Partition: %d, Offset: %d, Key: %s, Value length: %d",
		key.Topic, key.Partition, key.Offset, string(key.Key), len(key.Value))

	// Track message processing start time
	startTime := time.Now()

	var tradeMsg []*model.TradeWithPair
	if t.chainId == constants.Sol {
		if err := json.Unmarshal(key.Value, &tradeMsg); err != nil {
			logc.Errorf(ctx, "❌ Failed to unmarshal trade message: %+v", err)
			return err
		}
		logc.Infof(ctx, "✅ Successfully unmarshaled %d SOL trade messages", len(tradeMsg))
	}

	for _, msg := range tradeMsg {
		if msg == nil {
			logc.Errorf(ctx, "trade message is nil")
			continue
		}

		if msg.TokenPriceUSD == 0 {
			logc.Errorf(ctx, "trade TokenPriceUSD is 0, trade: ", msg)
			continue
		}

		logc.Infof(ctx, "tradeMsg,chain id: %v, pair addr: %v, tx hash: %v, price: %v", msg.ChainId, msg.PairAddr, msg.TxHash, msg.TokenPriceUSD)
	}

	if len(tradeMsg) == 0 {
		return nil
	}

	timeStr := time.Now().Format(time.DateTime)
	processingTime := time.Since(startTime)
	logc.Infof(ctx, "📊 tradeConsumer sendTime: %v, blockTime: %d, receiveTime: %v, key partition: %d, chain id: %s, processing time: %v", tradeMsg[0].CreateTime.Format(time.DateTime), tradeMsg[0].BlockTime, timeStr, key.Partition, tradeMsg[0].ChainId, processingTime)
	DataflowConsumerMessageHander.Inc()

	klineMap := logic.GenerateKlinesWithTradesFromTrades(ctx, tradeMsg)
	if len(klineMap) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	for _, klineWithTrade := range klineMap {
		wg.Add(1)
		k := klineWithTrade

		t.workerPool.Submit(func() {
			defer wg.Done()
			if k == nil || len(k.Klines) == 0 {
				logc.Errorf(ctx, "Submit nil data len(k.Klines):%d len(k.Trades):%d", len(k.Klines))
				return
			}
			// 发送到redis
			for i := 0; i < len(k.Klines); i++ {
				klineData := cache.KlineRedisCache.NewPush(ctx, k.Klines[i])
				config.KlineProcessedCh <- klineData
				k.Klines[i] = klineData

				// Publish real-time update to WebSocket clients
				t.publishKlineUpdate(ctx, klineData)
			}
		})
	}
	wg.Wait()

	return nil
}

// publishKlineUpdate publishes kline updates to Redis pub/sub for WebSocket broadcasting
func (t *TradeConsumer) publishKlineUpdate(ctx context.Context, kline *datakline.Kline) {
	if kline == nil {
		logc.Errorf(ctx, "❌ [REDIS PUB] Attempted to publish nil kline")
		return
	}

	logc.Infof(ctx, "🔥 [REDIS PUB] Starting publish for pair: %s, interval: %s, price: %.8f",
		kline.PairAddr, kline.Interval, kline.Close)

	updateMsg := KlineUpdateMessage{
		PairAddress: kline.PairAddr,
		ChainId:     kline.ChainId,
		Interval:    kline.Interval,
		CandleTime:  kline.CandleTime,
		Open:        kline.Open,
		High:        kline.High,
		Low:         kline.Low,
		Close:       kline.Close,
		Volume:      kline.AmountUsd,
		Timestamp:   time.Now().Unix(),
	}

	data, err := json.Marshal(updateMsg)
	if err != nil {
		logc.Errorf(ctx, "❌ [REDIS PUB] Failed to marshal kline update: %v", err)
		return
	}

	logc.Infof(ctx, "📤 [REDIS PUB] Publishing to 'kline:updates', data size: %d bytes", len(data))

	// Publish to Redis pub/sub channel
	result := t.redisClient.Publish(ctx, "kline:updates", data)
	err = result.Err()
	if err != nil {
		logc.Errorf(ctx, "❌ [REDIS PUB] Failed to publish kline update to Redis: %v", err)
		return
	}

	subscribers := result.Val()
	logc.Infof(ctx, "✅ [REDIS PUB] Successfully published kline update for pair %s, interval %s, subscribers: %d",
		kline.PairAddr, kline.Interval, subscribers)
}

// ticker processes klines in batches or when the ticker fires
func (t *TradeConsumer) ticker() {
	const (
		batchSize  = 10000
		tickPeriod = time.Millisecond * 500
	)

	ticker := time.NewTicker(tickPeriod)
	defer ticker.Stop()

	batch := make([]*datakline.Kline, 0, batchSize)

	flush := func() {
		if len(batch) > 0 {
			t.saveKlineToDB(batch)
			batch = make([]*datakline.Kline, 0, batchSize)
		}
	}

	for {
		select {
		case <-t.ctx.Done():
			flush()
			return
		case msg := <-config.KlineProcessedCh:
			if msg == nil {
				return
			}

			batch = append(batch, msg)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (t *TradeConsumer) saveKlineToDB(klines []*datakline.Kline) {
	startTime := time.Now()
	m := make(map[string]*data.Kline)
	for _, kline := range klines {
		if kline == nil {
			continue
		}
		key := fmt.Sprintf("%s:%s:%d", kline.Interval, kline.PairAddr, kline.CandleTime)
		if exist, ok := m[key]; ok {
			if exist.Volume < kline.AmountUsd {
				m[key] = &data.Kline{
					ChainId:     kline.ChainId,
					PairAddress: kline.PairAddr,
					CandleTime:  kline.CandleTime,
					OpenAt:      kline.OpenAt,
					CloseAt:     kline.CloseAt,
					Open:        kline.Open,
					High:        kline.High,
					Low:         kline.Low,
					Close:       kline.Close,
					Volume:      kline.AmountUsd,
					Tokens:      kline.VolumeToken,
					AvgPrice:    kline.AvgPrice,
					Count:       kline.TotalCount,
					BuyCount:    kline.BuyCount,
					SellCount:   kline.SellCount,
					UpdatedAt:   time.Now(),
				}
			}
		} else {
			m[key] = &data.Kline{
				ChainId:     kline.ChainId,
				PairAddress: kline.PairAddr,
				CandleTime:  kline.CandleTime,
				OpenAt:      kline.OpenAt,
				CloseAt:     kline.CloseAt,
				Open:        kline.Open,
				High:        kline.High,
				Low:         kline.Low,
				Close:       kline.Close,
				Volume:      kline.AmountUsd,
				Tokens:      kline.VolumeToken,
				AvgPrice:    kline.AvgPrice,
				Count:       kline.TotalCount,
				BuyCount:    kline.BuyCount,
				SellCount:   kline.SellCount,
				UpdatedAt:   time.Now(),
			}
		}
	}

	// Group by (interval, table month) — NOT just interval — so that every kline
	// in a batch lands in the table its own CandleTime actually belongs to. A
	// single flush can span a month boundary (or, before this fix, pick up an
	// arbitrary CandleTime from Go's randomized map iteration order), which used
	// to route a whole batch of unrelated months into whichever table happened to
	// be picked last. See docs/项目已知问题与修复记录.md.
	insertMap := make(map[string][]*data.Kline)
	representative := make(map[string]int64)
	for key, kline := range m {
		interval := strings.SplitN(key, ":", 2)[0]
		tm := time.Unix(kline.CandleTime, 0).UTC()
		groupKey := fmt.Sprintf("%s:%04d%02d", interval, tm.Year(), tm.Month())
		insertMap[groupKey] = append(insertMap[groupKey], kline)
		representative[groupKey] = kline.CandleTime
	}

	if len(insertMap) > 0 {
		for groupKey, kline := range insertMap {
			interval := strings.SplitN(groupKey, ":", 2)[0]
			candleTime := representative[groupKey]
			if candleTime == 0 {
				candleTime = time.Now().Unix()
			}
			err := t.klineMysqlRepo.SaveKlineWithRetry(context.Background(), constants.KlineInterval(interval), candleTime, kline)
			if err != nil {
				t.Logger.Errorf("failed to save klines for interval %s: %v", interval, err)
			}
			t.Logger.Infof("inserted %d klines, time cost: %v", len(kline), time.Since(startTime))
		}
	} else {
		t.Logger.Errorf("failed to save klines no result")
	}
}

// Close implements the io.Closer interface
func (t *TradeConsumer) Close() error {
	t.cancel()
	t.workerPool.Release()
	return nil
}
