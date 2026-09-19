package logic

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"

	"github.com/shopspring/decimal"

	"dex/dataflow/pkg/util"

	datakline "dex/dataflow/dataflow"
	"dex/dataflow/internal/constants"
	"dex/dataflow/internal/model"
)

func GenerateKlinesWithTradesFromTrades(ctx context.Context, trades []*model.TradeWithPair) map[string]*model.KlineWithTrade {
	if len(trades) == 0 {
		return nil
	}

	tradeMap := make(map[string][]*model.TradeWithPair, len(trades)/10)
	for _, trade := range trades {
		tradeMap[trade.PairAddr] = append(tradeMap[trade.PairAddr], trade)
	}

	// Generate klines for each pair and interval
	klineMap := make(map[string]*model.KlineWithTrade, len(tradeMap))

	for pairAddress, _ := range tradeMap {
		if _, ok := klineMap[pairAddress]; !ok {
			klineMap[pairAddress] = &model.KlineWithTrade{
				Klines: []*datakline.Kline{},
			}
		}
	}

	for _, interval := range constants.KlineIntervalMinutes {
		for pairAddress, pairTrades := range tradeMap {
			klineMap[pairAddress].Klines = append(klineMap[pairAddress].Klines, aggregateTradesIntoKlines(pairTrades, interval)...)
		}
	}
	return klineMap
}

// aggregateTradesIntoKlines aggregates trades into klines for a specific interval
func aggregateTradesIntoKlines(trades []*model.TradeWithPair, intervalMinute int) []*datakline.Kline {
	if len(trades) == 0 {
		return nil
	}

	klineMap := make(map[string]*datakline.Kline, len(trades)/2)
	// candleTime will be the same within a batch of trades
	// as they were produced from the same block
	candleTime := util.GetCandleTime(trades[0].BlockTime, intervalMinute)
	pair := trades[0].PairAddr
	key := fmt.Sprintf("%v:%v", pair, candleTime)

	//result := &datakline.ConsumerKlines{
	//	List: make([]*datakline.Kline, 0, len(trades)/2),
	//}
	result := make([]*datakline.Kline, 0, len(trades)/2)
	for _, trade := range trades {
		if kline, exists := klineMap[key]; exists {
			if kline.CandleTime == candleTime {
				updateKlineByTrade(kline, trade)
			} else {
				if newKline := newKlineByTrade(trade, candleTime, intervalMinute); newKline != nil {
					klineMap[key] = newKline
				}
			}
		} else {
			if newKline := newKlineByTrade(trade, candleTime, intervalMinute); newKline != nil {
				klineMap[key] = newKline
			}
		}
	}

	// Convert map to sorted array
	for _, kline := range klineMap {
		result = append(result, kline)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CandleTime < result[j].CandleTime
	})

	return result
}

// updateKlineByTrade updates an existing kline with new trade data
func updateKlineByTrade(kline *datakline.Kline, trade *model.TradeWithPair) {
	if trade == nil || kline == nil {
		return
	}

	if !trade.Clamp {
		// 去除夹子带来的k线毛刺
		// 纠正历史k线的开盘价、收盘价和市值相关
		if trade.BlockTime < kline.OpenAt {
			kline.OpenAt = trade.BlockTime
			kline.Open = trade.TokenPriceUSD
			kline.McapOpen = trade.Mcap
		}
		if trade.BlockTime > kline.CloseAt {
			kline.CloseAt = trade.BlockTime
			kline.Close = trade.TokenPriceUSD
			kline.McapClose = trade.Mcap
			kline.PumpPoint = trade.PumpPoint
			kline.MktCap = trade.Mcap
		}

		// 计算当前k线的价格和市值相关
		kline.High = max(kline.High, trade.TokenPriceUSD)
		kline.Low = min(kline.Low, trade.TokenPriceUSD)
		kline.McapHigh = max(kline.McapHigh, trade.Mcap)
		kline.McapLow = min(kline.McapLow, trade.Mcap)

		// Update weighted average price
		totalPrice := kline.AvgPrice * float64(kline.TotalCount)
		kline.AvgPrice = (totalPrice + trade.TokenPriceUSD) / float64(kline.TotalCount+1)
	}

	// Update trade statistics
	kline.AmountUsd += trade.TotalUSD
	kline.VolumeToken += trade.TokenAmount
	kline.TotalCount++

	switch trade.Type {
	case constants.TradeTypeBuy:
		kline.BuyCount++
	case constants.TradeTypeSell:
		kline.SellCount++
	}
}

func MergeModelKline(klines []model.Kline) (newKline model.Kline) {
	if len(klines) == 0 || klines == nil {
		return
	}

	newKline = klines[0]
	for index, kline := range klines {
		if index == 0 {
			continue
		}
		if newKline.OpenAt > kline.OpenAt {
			newKline.OpenAt = kline.OpenAt
			newKline.Open = kline.Open
			newKline.McapOpen = kline.McapOpen
		}

		if newKline.CloseAt < kline.CloseAt {
			newKline.CloseAt = kline.CloseAt
			newKline.Close = kline.Close
			newKline.McapClose = kline.McapClose
		}

		if newKline.High < kline.High {
			newKline.High = kline.High
			newKline.McapHigh = kline.McapHigh
		}

		if newKline.Low > kline.Low {
			newKline.Low = kline.Low
			newKline.McapLow = kline.McapLow
		}

		newTotalPrice := decimal.NewFromFloat(newKline.AvgPrice).Mul(decimal.NewFromInt(newKline.TotalCount))
		klineTotalPrice := decimal.NewFromFloat(kline.AvgPrice).Mul(decimal.NewFromInt(kline.TotalCount))
		newKline.AvgPrice, _ = newTotalPrice.Add(klineTotalPrice).Div(decimal.NewFromInt(newKline.TotalCount).Add(decimal.NewFromInt(kline.TotalCount))).Float64()

		newKline.AmountUsd += kline.AmountUsd
		newKline.VolumeToken += kline.VolumeToken
		newKline.TotalCount += kline.TotalCount
		newKline.BuyCount += kline.BuyCount
		newKline.SellCount += kline.SellCount
	}

	return
}

// newKlineByTrade creates a new kline from a trade
func newKlineByTrade(trade *model.TradeWithPair, candleTime int64, intervalMinute int) *datakline.Kline {
	if trade == nil {
		return nil
	}
	mcap := trade.Mcap

	chainId, err := strconv.ParseInt(trade.ChainId, 10, 64)
	if err != nil {
		log.Printf("invalid chain id: %s", trade.ChainId)
		return nil
	}

	openAt, closeAt := trade.BlockTime, trade.BlockTime
	if trade.Clamp {
		openAt = candleTime + 24*60*60
		closeAt = candleTime
	}

	kline := &datakline.Kline{
		ChainId:     chainId,
		PairAddr:    trade.PairAddr,
		CandleTime:  candleTime,
		OpenAt:      openAt,
		CloseAt:     closeAt,
		Open:        trade.TokenPriceUSD,
		Close:       trade.TokenPriceUSD,
		High:        trade.TokenPriceUSD,
		Low:         trade.TokenPriceUSD,
		McapOpen:    mcap,
		McapClose:   mcap,
		McapHigh:    mcap,
		McapLow:     mcap,
		AmountUsd:   trade.TotalUSD,
		VolumeToken: trade.TokenAmount,
		AvgPrice:    trade.TokenPriceUSD,
		TotalCount:  1,
		Interval:    string(constants.MinuteToInterval(intervalMinute)),
		MktCap:      trade.Mcap,
		PumpPoint:   trade.PumpPoint,
		TokenAddr:   trade.PairInfo.TokenAddr,
	}

	switch trade.Type {
	case constants.TradeTypeBuy:
		kline.BuyCount = 1
	case constants.TradeTypeSell:
		kline.SellCount = 1
	}

	return kline
}
