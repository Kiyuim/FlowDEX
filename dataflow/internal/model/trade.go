// Package model
// File trade.go
package model

import (
	"time"

	"dex/dataflow/internal/constants"
	"dex/dataflow/pkg/util"

	"github.com/shopspring/decimal"
)

type Trade struct {
	PairAddress       string          `json:"pair_addr" ch:"pair_addr"`
	TxHash            string          `json:"tx_hash" ch:"tx_hash"`
	Maker             string          `json:"maker" ch:"maker"`
	To                string          `json:"to" ch:"to"`
	TradeType         uint8           `json:"trade_type" ch:"trade_type"`
	BaseTokenAmount   decimal.Decimal `json:"base_token_amount" ch:"base_token_amount"`
	TokenAmount       decimal.Decimal `json:"token_amount" ch:"token_amount"`                 // Change in non-base token amount
	BaseTokenPriceUSD decimal.Decimal `json:"base_token_price_usd" ch:"base_token_price_usd"` // Base token price
	TotalUSD          decimal.Decimal `json:"total_usd" ch:"total_usd"`                       // Total value
	TokenPriceUSD     decimal.Decimal `json:"token_price_usd" ch:"token_price_usd"`           // Non-base token price
	BlockNum          uint64          `json:"block_num" ch:"block_num"`                       // Block height
	BlockTimestamp    uint64          `json:"block_timestamp" ch:"block_timestamp"`           // Block timestamp
	// TransactionIndex  int     `json:"transaction_index"`
	// LogIndex          int     `json:"log_index"`            // log index
	Clamp    int8   `json:"clamp_type" ch:"clamp_type"`
	SwapName string `json:"swap_name" ch:"swap_name"`
	Sort     uint64 `ch:"sort"`
}

func (t Trade) GetCols() []any {
	return []any{
		t.PairAddress,
		t.TxHash,
		t.Maker,
		t.To,
		t.TradeType,
		t.BaseTokenAmount,
		t.TokenAmount,
		t.BaseTokenPriceUSD,
		t.TotalUSD,
		t.TokenPriceUSD,
		t.BlockNum,
		t.BlockTimestamp,
		t.Clamp,
		t.SwapName,
		t.Sort,
	}
}

type TradeWithPair struct {
	ChainId  string `json:"chain_id" tag:"true"`  // tag chainId
	PairAddr string `json:"pair_addr" tag:"true"` // tag address
	TxHash   string `json:"tx_hash" tag:"true"`   // tag hash - may cause memory explosion, needs periodic rollup and deletion
	HashId   string `json:"hash_id"`

	Maker             string  `json:"maker"`                // address
	Type              string  `json:"type"`                 // tag: sell/buy/add_position/remove_position
	BaseTokenAmount   float64 `json:"base_token_amount"`    // Change in base token amount
	TokenAmount       float64 `json:"token_amount"`         // Change in non-base token amount
	BaseTokenPriceUSD float64 `json:"base_token_price_usd"` // Base token price
	TotalUSD          float64 `json:"total_usd"`            // Total value
	TokenPriceUSD     float64 `json:"token_price_usd"`      // Non-base token price
	To                string  `json:"to"`                   // Token recipient address
	BlockNum          int64   `json:"block_num"`            // Block height
	BlockTime         int64   `json:"block_time"`           // Block timestamp
	TransactionIndex  int     `json:"transaction_index"`    // Transaction index
	LogIndex          int     `json:"log_index"`            // Log index

	SwapName                     string  `json:"swap_name"`                         // Trading pair version
	CurrentTokenInPoolAmount     float64 `json:"current_token_in_pool_amount"`      // Current token amount
	CurrentBaseTokenInPoolAmount float64 `json:"current_base_token_in_pool_amount"` // Current base token amount

	Fdv  float64 `json:"fdv"`  // Market cap, for websocket push
	Mcap float64 `json:"mcap"` // Liquidity market cap

	PairInfo         Pair      `json:"pair_info"`
	Clamp            bool      `json:"clamp"`         // true: sandwich attack or being sandwiched
	PumpPoint        float64   `json:"pump_point"`    // Pump score
	PumpLaunched     bool      `json:"pump_launched"` // Pump launched
	PumpMarketCap    float64   `json:"pump_market_cap"`
	PumpOwner        string    `json:"pump_owner"`
	PumpSwapPairAddr string    `json:"pump_swap_pair_addr"`
	CreateTime       time.Time `json:"create_time"`
}

type EvmAddressInfo struct {
	WalletAddress string `json:"wallet_address"`
	AddressTag    string `json:"address_tag"`
	AddressIcon   string `json:"address_icon"`
	TwitterLink   string `json:"twitter_link"`
}

type EvmTradeWithPair struct {
	TraderInfo                   EvmAddressInfo  `json:"trader_info"`             // Trader information
	ChainId                      string          `json:"chain_id" tag:"true"`     // Tag for chain ID
	ChainIdInt                   int             `json:"chain_id_int" tag:"true"` // Tag for chain ID
	PairAddr                     string          `json:"pair_addr" tag:"true"`    // Tag for address
	TxHash                       string          `json:"tx_hash" tag:"true"`      // Tag for transaction hash, may cause memory overflow; needs periodic roll-up and deletion
	HashId                       string          `json:"hash_id"`
	Maker                        string          `json:"maker"` // Address
	From                         string          `json:"from"`
	Type                         string          `json:"type"`                 // Tag: sell/buy/add_position/remove_position
	BaseTokenAmount              decimal.Decimal `json:"base_token_amount"`    // Amount of base token changed
	TokenAmount                  decimal.Decimal `json:"token_amount"`         // Amount of non-base token changed
	BaseTokenPriceUSD            decimal.Decimal `json:"base_token_price_usd"` // Price of the base token in USD
	TotalUSD                     decimal.Decimal `json:"total_usd"`            // Total value in USD
	TokenPriceUSD                decimal.Decimal `json:"token_price_usd"`      // Price of the non-base token in USD
	To                           string          `json:"to"`                   // Token recipient address
	TxTo                         string          `json:"tx_to"`
	BlockNum                     int64           `json:"block_num"`                         // Block height
	BlockTime                    int64           `json:"block_time"`                        // Block time
	TransactionIndex             int             `json:"transaction_index"`                 // Transaction index
	LogIndex                     int             `json:"log_index"`                         // Log index
	SwapName                     string          `json:"swap_name"`                         // Trading pair version
	CurrentTokenInPoolAmount     decimal.Decimal `json:"current_token_in_pool_amount"`      // Current token amount in pool
	CurrentBaseTokenInPoolAmount decimal.Decimal `json:"current_base_token_in_pool_amount"` // Current base token amount in pool

	PairInfo EvmPair `json:"pair_info"`

	KlineUpDown5m  float64         `json:"kline_up_down_5m"`  // 5-minute price change, used for pushing to websocket
	KlineUpDown1h  float64         `json:"kline_up_down_1h"`  // 1-hour price change, used for pushing to websocket
	KlineUpDown4h  float64         `json:"kline_up_down_4h"`  // 4-hour price change, used for pushing to websocket
	KlineUpDown6h  float64         `json:"kline_up_down_6h"`  // 6-hour price change, used for pushing to websocket
	KlineUpDown24h float64         `json:"kline_up_down_24h"` // 24-hour price change, used for pushing to websocket
	Fdv            float64         `json:"fdv"`               // Market cap, used for pushing to websocket
	Mcap           float64         `json:"mcap"`              // Circulating market cap
	Liquidity      decimal.Decimal `json:"liquidity"`         // Liquidity

	TokenAmountInt     int64 `json:"token_amount_int"` // Not divided by decimal
	BaseTokenAmountInt int64 `json:"base_token_amount_int"`
	Clamp              bool  `json:"clamp"` // true: clamped or in a clamp
	Clipper            bool  `json:"-"`     // true: clamp

	// pump
	FourMeme EvmPump `json:"four_meme"`

	CreateTime time.Time `json:"create_time"` // 用于标识 send kafka 消息的时间
}

type EvmPair struct {
	ChainId string `json:"chain_id"`
	Addr    string `json:"addr"`

	FactoryAddress string `json:"factory_address"` // Factory address
	Token0         string `json:"token0"`          // Token0 address
	Token1         string `json:"token1"`          // Token1 address
	Fee            int64  `json:"fee"`             // Pool fee
	TickSpacing    int64  `json:"tick_spacing"`    // Pool tick spacing

	BaseTokenAddr          string `json:"base_token_addr"`
	TokenAddr              string `json:"token_addr"`
	BaseTokenSymbol        string `json:"base_token_symbol"`
	TokenSymbol            string `json:"token_symbol"`
	BaseTokenDecimal       uint8  `json:"base_token_decimal"`
	TokenDecimal           uint8  `json:"token_decimal"`
	BaseTokenIsNativeToken bool   `json:"base_token_is_native_token"`
	BaseTokenIsToken0      bool   `json:"base_token_is_token_0"`

	TokenTotalSupply    float64         `json:"token_total_supply"`     // 代币总供应量
	InitTokenAmount     decimal.Decimal `json:"init_token_amount"`      // 初始化代币数量
	InitBaseTokenAmount decimal.Decimal `json:"init_base_token_amount"` // 初始化基础代币数量

	Name         string `json:"name"`
	ExchangeType int64  `json:"exchange_type"`

	BlockNum  int64 `json:"block_num"`  // 池子创建Slot
	BlockTime int64 `json:"block_time"` // 池子创建时间

	CurrentBaseTokenAmount decimal.Decimal `json:"current_base_token_amount"` // 当前base流动性数量
	CurrentTokenAmount     decimal.Decimal `json:"current_token_amount"`      // 当前token流动性数量

	LatestTradeTime time.Time
}

type EvmPump struct {
	PumpPoint                    decimal.Decimal `json:"pump_point"`    // Pump score
	PumpLaunched                 bool            `json:"pump_launched"` // Pump launched
	PumpMarketCap                decimal.Decimal `json:"pump_market_cap"`
	PumpOwner                    string          `json:"pump_owner"`
	PumpSwapPairAddr             string          `json:"pump_swap_pair_addr"`
	PumpVirtualBaseTokenReserves decimal.Decimal `json:"pump_virtual_base_token_reserves,omitempty"`
	PumpVirtualTokenReserves     decimal.Decimal `json:"pump_virtual_token_reserves,omitempty"`
	PumpStatus                   int             `json:"pump_status"`
	PumpPairAddr                 string          `json:"pump_pair_addr"`
}

func (tp *TradeWithPair) ToTrade() Trade {
	sort := time.Unix(tp.BlockTime, 0).UTC().
		Add(time.Microsecond * time.Duration(tp.TransactionIndex)).
		Add(time.Nanosecond * time.Duration(tp.LogIndex)).
		UnixNano()

	var clamp int
	if tp.Clamp {
		clamp = 1
	}

	t := Trade{
		PairAddress:       tp.PairAddr,
		TxHash:            tp.TxHash,
		Maker:             tp.Maker,
		To:                tp.To,
		TradeType:         constants.MapTradeType(tp.Type),
		BaseTokenAmount:   util.Float642Decimal(tp.BaseTokenAmount),
		TokenAmount:       util.Float642Decimal(tp.TokenAmount),
		BaseTokenPriceUSD: util.Float642Decimal(tp.BaseTokenPriceUSD),
		TotalUSD:          util.Float642Decimal(tp.TotalUSD),
		TokenPriceUSD:     util.Float642Decimal(tp.TokenPriceUSD),
		BlockNum:          uint64(tp.BlockNum),
		BlockTimestamp:    uint64(tp.BlockTime),
		Clamp:             int8(clamp),
		SwapName:          tp.SwapName,
		Sort:              uint64(sort),
	}

	return t
}
