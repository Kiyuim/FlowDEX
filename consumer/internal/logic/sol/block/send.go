package block

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/duke-git/lancet/v2/slice"

	"dex/consumer/internal/logic/mq"
	"dex/pkg/types"

	"github.com/zeromicro/go-zero/core/logx"
)

func (s *BlockService) SendTx(_ context.Context, slot int64, trades []*types.TradeWithPair) {
	now := time.Now()
	trades = slice.Filter[*types.TradeWithPair](trades, func(index int, item *types.TradeWithPair) bool {
		if item == nil {
			return false
		}
		if item.Type != types.TradeTypeBuy && item.Type != types.TradeTypeSell {
			return false
		}
		if item.TokenPriceUSD == 0 {
			return false
		}
		item.CreateTime = now
		// Candles chart the pool's spot price after the trade, not the trade's
		// average execution price: on a bonding curve a sell right after a buy
		// executes from the higher spot downward, so its average sits above the
		// buy's average and the sell candle came out green.
		if item.PumpVirtualBaseTokenReserves > 0 && item.PumpVirtualTokenReserves > 0 && item.BaseTokenPriceUSD > 0 {
			item.SpotPriceUSD = item.PumpVirtualBaseTokenReserves / item.PumpVirtualTokenReserves * item.BaseTokenPriceUSD
		}
		return true
	})

	// Flag same-block buy+sell wash trades before they reach Kafka/K-line
	// aggregation downstream (was written but never wired up — see
	// docs/项目已知问题与修复记录.md).
	mq.DetectClaim(trades)

	SolTradeTopic := s.sc.Config.KqSolTrades.Topic
	tradeListJsons, err := json.Marshal(trades)
	if err != nil {
		logx.Errorf("json.Marshal err:%v", err)
		return
	}
	err = mq.SendEventLogKafkaInfoMessage(SolTradeTopic, fmt.Sprintf("%v", slot), tradeListJsons)
	if err != nil {
		logx.Errorf("SendEventLogKafkaInfoMessage err:%v", err)
		return
	}

	fmt.Println("SendTx success")
}

func (s *BlockService) SendPairPriceChange2Kafka(_ context.Context, slot int64, pairAddress string) {

	err := mq.SendEventLogKafkaInfoMessage("sol-pair-price-change", fmt.Sprintf("%v", slot), []byte(pairAddress))
	if err != nil {
		logx.Errorf("SendPairPriceChange2Kafka err:%v", err)
		return
	}
	logx.Infof("sendEventLogKafkaInfoMessage:%v:%v success", slot, pairAddress)
}
