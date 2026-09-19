// Package model
// File kline.go
package model

import (
	datakline "dex/dataflow/dataflow"
)

type Kline struct {
	PairAddress string  `json:"pair_address" ch:"pair_address"`
	CandleTime  int64   `json:"candle_time" ch:"candle_time"`
	Open        float64 `json:"open" ch:"open"`
	High        float64 `json:"high" ch:"high"`
	Low         float64 `json:"low" ch:"low"`
	Close       float64 `json:"close" ch:"close"`
	McapOpen    float64 `json:"mcap_open" ch:"mcap_open"`
	McapHigh    float64 `json:"mcap_high" ch:"mcap_high"`
	McapLow     float64 `json:"mcap_low" ch:"mcap_low"`
	McapClose   float64 `json:"mcap_close" ch:"mcap_close"`
	AmountUsd   float64 `json:"amount_usd" ch:"amount_usd"`
	VolumeToken float64 `json:"volume_token" ch:"volume_token"`
	OpenAt      int64   `json:"open_at" ch:"open_at"`
	CloseAt     int64   `json:"close_at" ch:"close_at"`
	AvgPrice    float64 `json:"avg_price" ch:"avg_price"`
	TotalCount  int64   `json:"total_count" ch:"total_count"`
	BuyCount    int64   `json:"buy_count" ch:"buy_count"`
	SellCount   int64   `json:"sell_count" ch:"sell_count"`
}

func (k Kline) GetCols() []any {
	return []any{
		k.PairAddress,
		k.CandleTime,
		k.Open,
		k.High,
		k.Low,
		k.Close,
		k.McapOpen,
		k.McapHigh,
		k.McapLow,
		k.McapClose,
		k.AmountUsd,
		k.VolumeToken,
		k.OpenAt,
		k.CloseAt,
		k.AvgPrice,
		k.TotalCount,
		k.BuyCount,
		k.SellCount,
	}
}

func (k Kline) GetUpdateCols() []any {
	return []any{
		k.Open,
		k.High,
		k.Low,
		k.Close,
		k.McapOpen,
		k.McapHigh,
		k.McapLow,
		k.McapClose,
		k.AmountUsd,
		k.VolumeToken,
		k.OpenAt,
		k.CloseAt,
		k.AvgPrice,
		k.TotalCount,
		k.BuyCount,
		k.SellCount,
		k.PairAddress, // WHERE
		k.CandleTime,  // WHERE
	}
}

type KlineWithTrade struct {
	Klines []*datakline.Kline
}
