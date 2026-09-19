package mq

import "dex/pkg/types"

func DetectClaim(tradeListMsg []*types.TradeWithPair) {
	var tradePairMap = make(map[string][]*types.TradeWithPair)
	for _, trade := range tradeListMsg {
		tradePairMap[trade.PairAddr] = append(tradePairMap[trade.PairAddr], trade)
	}
	for _, tradeListMsg := range tradePairMap {
		makerFirstTrade := make(map[string]int)
		makerTradeType := make(map[string]int)
		for i, trade := range tradeListMsg {
			if trade.Maker == "" {
				continue
			}
			if trade.Type == "buy" {
				makerTradeType[trade.Maker] |= 1 << 0
			} else if trade.Type == "sell" {
				makerTradeType[trade.Maker] |= 1 << 1
			}
			if lastIndex, ok := makerFirstTrade[trade.Maker]; ok {
				last := tradeListMsg[lastIndex]
				if last.BlockNum == trade.BlockNum {
					if makerTradeType[trade.Maker] >= (1<<0 | 1<<1) {
						for j := lastIndex; j <= i; j++ {
							if tradeListMsg[j].PairAddr == trade.PairAddr {
								tradeListMsg[j].Clamp = true
								//logx.Errorf("claim detected,pair:%v,make:%v, price:%v, totalUsd:%v, txhash:%v,", tradeListMsg[j].PairAddr, tradeListMsg[j].Maker, tradeListMsg[j].TokenPriceUSD, tradeListMsg[j].TotalUSD, tradeListMsg[j].TxHash)
							}
						}
					}
				} else {
					makerFirstTrade[trade.Maker] = i
				}
			} else {
				makerFirstTrade[trade.Maker] = i
			}
		}
	}

}
