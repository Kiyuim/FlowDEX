// Package model
// File pair.go
package model

type Pair struct {
	ChainId string `json:"chain_id"` //tag
	Addr    string `json:"addr"`     //tag

	BaseTokenAddr          string `json:"base_token_addr"`
	TokenAddr              string `json:"token_addr"`
	TokenName              string `json:"token_name"`
	BaseTokenSymbol        string `json:"base_token_symbol"`
	TokenSymbol            string `json:"token_symbol"`
	BaseTokenDecimal       uint8  `json:"base_token_decimal"`
	TokenDecimal           uint8  `json:"token_decimal"`
	BaseTokenIsNativeToken bool   `json:"base_token_is_native_token"`
	BaseTokenIsToken0      bool   `json:"base_token_is_token_0"`

	TokenTotalSupply    float64 `json:"token_total_supply"`
	InitTokenAmount     float64 `json:"init_token_amount"`
	InitBaseTokenAmount float64 `json:"init_base_token_amount"`

	Name string `json:"name"`

	BlockNum  int64 `json:"block_num"`
	BlockTime int64 `json:"block_time"`
}
