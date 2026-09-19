package types

// StoreUserTokenReq represents a request to store user token information
type StoreUserTokenReq struct {
	UserWalletAddress string `json:"user_wallet_address"`
	ChainId           int64  `json:"chain_id"`
	TokenAddress      string `json:"token_address"`
	TokenProgram      string `json:"token_program"`
	Name              string `json:"name"`
	Symbol            string `json:"symbol"`
	Decimals          int64  `json:"decimals"`
	TotalSupply       int64  `json:"total_supply"`
	Icon              string `json:"icon,optional"`
	Description       string `json:"description,optional"`
	TxHash            string `json:"tx_hash"`
}

// StoreUserTokenResp represents a response after storing user token
type StoreUserTokenResp struct {
	Code int64       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// GetUserTokensReq represents a request to get user tokens
type GetUserTokensReq struct {
	WalletAddress string `form:"wallet_address"`
	ChainId       int64  `form:"chain_id,optional"`
	PageNo        int64  `form:"page_no,optional,default=1"`
	PageSize      int64  `form:"page_size,optional,default=20"`
}

// UserTokenItem represents a single token item
type UserTokenItem struct {
	Id            int64  `json:"id"`
	TokenAddress  string `json:"tokenAddress"`
	TokenProgram  string `json:"tokenProgram"`
	TokenName     string `json:"tokenName"`
	TokenSymbol   string `json:"tokenSymbol"`
	TokenIcon     string `json:"tokenIcon"`
	TokenDecimals int64  `json:"tokenDecimals"`
	TokenSupply   int64  `json:"tokenSupply"`
	Description   string `json:"description"`
	TxHash        string `json:"txHash"`
	CreatedAt     int64  `json:"createdAt"`
}

// GetUserTokensResp represents a response for get user tokens request
type GetUserTokensResp struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List      []UserTokenItem `json:"list"`
		TotalNum  int64           `json:"total_num"`
		PageNo    int64           `json:"page_no"`
		PageSize  int64           `json:"page_size"`
		TotalPage int64           `json:"total_page"`
	} `json:"data"`
}

// StoreUserPoolReq represents a request to store user pool information
type StoreUserPoolReq struct {
	UserWalletAddress string  `json:"user_wallet_address"`
	ChainId           int64   `json:"chain_id"`
	PoolState         string  `json:"pool_state"`
	InputVaultMint    string  `json:"input_vault_mint"`
	OutputVaultMint   string  `json:"output_vault_mint"`
	Token0Symbol      string  `json:"token0_symbol,optional"`
	Token1Symbol      string  `json:"token1_symbol,optional"`
	Token0Decimals    int64   `json:"token0_decimals,optional"`
	Token1Decimals    int64   `json:"token1_decimals,optional"`
	TradeFeeRate      int64   `json:"trade_fee_rate"`
	InitialPrice      float64 `json:"initial_price"`
	TxHash            string  `json:"tx_hash"`
	PoolVersion       string  `json:"pool_version,optional"`
	PoolType          string  `json:"pool_type,optional"`
	AmmConfig         string  `json:"amm_config,optional"`
}

// StoreUserPoolResp represents a response after storing user pool
type StoreUserPoolResp struct {
	Code int64       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// GetUserPoolsReq represents a request to get user pools
type GetUserPoolsReq struct {
	WalletAddress string `form:"wallet_address"`
	ChainId       int64  `form:"chain_id,optional"`
	PoolType      string `form:"pool_type,optional"`
	PoolVersion   string `form:"pool_version,optional"`
	PageNo        int64  `form:"page_no,optional,default=1"`
	PageSize      int64  `form:"page_size,optional,default=20"`
}

// UserPoolItem represents a single pool item
type UserPoolItem struct {
	Id              int64   `json:"id"`
	PoolState       string  `json:"poolState"`
	InputVaultMint  string  `json:"inputVaultMint"`
	OutputVaultMint string  `json:"outputVaultMint"`
	Token0Symbol    string  `json:"token0Symbol"`
	Token1Symbol    string  `json:"token1Symbol"`
	Token0Decimals  int64   `json:"token0Decimals"`
	Token1Decimals  int64   `json:"token1Decimals"`
	Token0Liquidity int64   `json:"token0Liquidity"`
	Token1Liquidity int64   `json:"token1Liquidity"`
	TradeFeeRate    int64   `json:"tradeFeeRate"`
	InitialPrice    float64 `json:"initialPrice"`
	TxHash          string  `json:"txHash"`
	PoolVersion     string  `json:"poolVersion"`
	PoolType        string  `json:"poolType"`
	AmmConfig       string  `json:"ammConfig"`
	CreatedAt       int64   `json:"createdAt"`
}

// GetUserPoolsResp represents a response for get user pools request
type GetUserPoolsResp struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List      []UserPoolItem `json:"list"`
		TotalNum  int64          `json:"total_num"`
		PageNo    int64          `json:"page_no"`
		PageSize  int64          `json:"page_size"`
		TotalPage int64          `json:"total_page"`
	} `json:"data"`
}
