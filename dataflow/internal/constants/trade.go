// Package constants
// File trade.go
package constants

const (
	TradeBuy        = 1
	TradeSell       = 2
	TradeAdd        = 3
	TradeRemove     = 4
	TradePumpCreate = 5
	TradePumpLaunch = 6
)

func MapTradeType(tradeType string) uint8 {
	switch tradeType {
	case "buy":
		return TradeBuy
	case "sell":
		return TradeSell
	case "add_position":
		return TradeAdd
	case "remove_position":
		return TradeRemove
	case "pump_create":
		return TradePumpCreate
	case "pump_launch":
		return TradePumpLaunch
	}

	return 0
}

const (
	TradeTypeSell           = "sell"
	TradeTypeBuy            = "buy"
	TradeTypeAddPosition    = "add_position"
	TradeTypeRemovePosition = "remove_position"
)
