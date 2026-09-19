package migration

import (
	"database/sql"
)

// CreateUserCreatedAssetsTable 创建用户创建的资产表
func CreateUserCreatedAssetsTable(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS user_created_assets (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_wallet VARCHAR(255) NOT NULL,
    asset_type VARCHAR(50) NOT NULL,
    asset_name VARCHAR(255) NOT NULL,
    asset_symbol VARCHAR(50) NOT NULL,
    asset_address VARCHAR(255) NOT NULL,
    chain_id BIGINT NOT NULL,
    
    decimals INT,
    total_supply VARCHAR(255),
    
    token0_address VARCHAR(255),
    token1_address VARCHAR(255),
    token0_symbol VARCHAR(50),
    token1_symbol VARCHAR(50),
    pool_type VARCHAR(50),
    fee_tier INT,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_user_wallet (user_wallet),
    INDEX idx_asset_type (asset_type),
    INDEX idx_asset_address (asset_address),
    INDEX idx_chain_id (chain_id),
    UNIQUE INDEX uniq_asset_address_chain_id (asset_address, chain_id)
);
`)
	return err
}

// RegisterMigration 注册迁移
func init() {
	registerMigration("CreateUserCreatedAssetsTable", CreateUserCreatedAssetsTable)
}
