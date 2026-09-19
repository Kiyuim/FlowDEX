package model

import (
	"time"
)

// UserCreatedAsset 表示用户创建的资产（代币或池子）
type UserCreatedAsset struct {
	ID           int64  `db:"id" json:"id"`                       // 主键ID
	UserWallet   string `db:"user_wallet" json:"user_wallet"`     // 用户钱包地址
	AssetType    string `db:"asset_type" json:"asset_type"`       // 资产类型：token（代币）或 pool（池子）
	AssetName    string `db:"asset_name" json:"asset_name"`       // 资产名称
	AssetSymbol  string `db:"asset_symbol" json:"asset_symbol"`   // 资产符号
	AssetAddress string `db:"asset_address" json:"asset_address"` // 资产地址（代币地址或池子地址）
	ChainID      int64  `db:"chain_id" json:"chain_id"`           // 链ID

	// 代币特有字段
	Decimals    int    `db:"decimals" json:"decimals,omitempty"`         // 代币精度
	TotalSupply string `db:"total_supply" json:"total_supply,omitempty"` // 代币总供应量

	// 池子特有字段
	Token0Address string `db:"token0_address" json:"token0_address,omitempty"` // 池子中的代币0地址
	Token1Address string `db:"token1_address" json:"token1_address,omitempty"` // 池子中的代币1地址
	Token0Symbol  string `db:"token0_symbol" json:"token0_symbol,omitempty"`   // 代币0符号
	Token1Symbol  string `db:"token1_symbol" json:"token1_symbol,omitempty"`   // 代币1符号
	PoolType      string `db:"pool_type" json:"pool_type,omitempty"`           // 池子类型（如CLMM, CPMM等）
	FeeTier       int    `db:"fee_tier" json:"fee_tier,omitempty"`             // 费率等级（仅CLMM池子）

	CreatedAt time.Time `db:"created_at" json:"created_at"` // 创建时间
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"` // 更新时间
}

// TableName 返回表名
func (UserCreatedAsset) TableName() string {
	return "user_created_assets"
}
