package trade

const (
	RedisLimitOrderBuyPrefix         = "dex:token_price_buy"
	RedisLimitOrderSellPrefix        = "dex:token_price_sell"
	RedisLimitCapBuyPrefix           = "dex:token_cap_buy"
	RedisLimitCapSellPrefix          = "dex:token_cap_sell"
	RedisMovingStopLossProfitChannel = "dex:moving_stop_loss_profit"
	RedisTrailingStopPrefix          = "dex:trailing_stop"
	RedisTradeOrderInfoPrefix        = "dex:trade_order_info"
)
