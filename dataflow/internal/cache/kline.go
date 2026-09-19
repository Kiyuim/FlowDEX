// Package cache
// File kline.go
package cache

import (
	"context"
	pb "dex/dataflow/dataflow"
	"dex/dataflow/internal/constants"
	"dex/dataflow/internal/svc"
)

type LoadFunc func(chainId constants.ChainId, pairAddr string, interval constants.KlineInterval, limit int64) ([]*pb.Kline, error)

var KlineCache *klineCache

type klineCache struct {
	m  map[string]*KlineData
	sc *svc.ServiceContext
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
