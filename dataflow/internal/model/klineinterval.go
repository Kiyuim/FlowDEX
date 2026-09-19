package model

type KlineInterval struct {
	Kline5M  float64
	Kline30M float64
	Kline1H  float64
	Kline4H  float64
	Kline6H  float64
	Kline24H float64

	Volume5M  float64
	Volume30M float64
	Volume1H  float64
	Volume4H  float64
	Volume6H  float64
	Volume24H float64

	SellCount5M float64
	BuyCount5M  float64

	SellCount30M float64
	BuyCount30M  float64

	SellCount1H float64
	BuyCount1H  float64

	SellCount4H float64
	BuyCount4H  float64

	SellCount6H float64
	BuyCount6H  float64

	BuyCount24H  float64
	SellCount24H float64
}
