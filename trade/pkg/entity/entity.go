package entity

import (
	"context"
	"encoding/json"
)

type OrderMessage struct {
	Ctx          context.Context
	TokenCA      string
	CurrentPrice string
	SwapType     int64
	ChainId      int
}

type RedisTrailingStopOrderInfo struct {
	OrderId         int64
	BasePrice       string
	DrawdownPrice   string
	TrailingPercent int
}

func (r *RedisTrailingStopOrderInfo) Serialize() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DeserializeRedisTrailingStopOrderInfo(data string) (*RedisTrailingStopOrderInfo, error) {
	var result RedisTrailingStopOrderInfo
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type RedisTokenPriceLimitOrderInfo struct {
	OrderId   int64
	BasePrice string
}

func (r *RedisTokenPriceLimitOrderInfo) Serialize() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DeserializeRedisTokenPriceLimitOrderInfo(data string) (*RedisTokenPriceLimitOrderInfo, error) {
	var result RedisTokenPriceLimitOrderInfo
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
