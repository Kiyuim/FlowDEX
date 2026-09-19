package trade

type CreateMarketTx struct {
	UserId            uint64
	ChainId           uint64
	UserWalletId      uint32
	UserWalletAddress string
	AmountIn          string
	IsAntiMev         bool
	IsAutoSlippage    bool
	Slippage          uint32
	GasType           int32
	TradePoolName     string
	InDecimal         uint8
	OutDecimal        uint8
	InTokenCa         string
	OutTokenCa        string
	PairAddr          string
	Price             string
	UsePriceLimit     bool
	InTokenProgram    string
	OutTokenProgram   string
}

type CreatePoolTx struct {
	ChainId           int64
	TokenMint0        string
	TokenMint1        string
	InitialPrice      string
	FeeTier           int32
	OpenTime          int64
	UserWalletAddress string
}

type AddLiquidityTx struct {
	ChainId           int64
	PoolId            string
	TickLower         int64
	TickUpper         int64
	BaseToken         int32 // 0 for token A, 1 for token B
	BaseAmount        string
	OtherAmountMax    string
	UserWalletAddress string
	TokenAAddress     string
	TokenBAddress     string
}
