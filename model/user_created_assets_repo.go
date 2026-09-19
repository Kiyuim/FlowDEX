package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// UserCreatedAssetsRepo 用户创建资产的存储库
type UserCreatedAssetsRepo struct {
	conn sqlx.SqlConn
}

// NewUserCreatedAssetsRepo 创建新的用户资产存储库
func NewUserCreatedAssetsRepo(conn sqlx.SqlConn) *UserCreatedAssetsRepo {
	return &UserCreatedAssetsRepo{
		conn: conn,
	}
}

// Create 创建新的用户资产记录
func (r *UserCreatedAssetsRepo) Create(ctx context.Context, asset *UserCreatedAsset) error {
	asset.CreatedAt = time.Now()
	asset.UpdatedAt = time.Now()

	query := `
INSERT INTO user_created_assets (
    user_wallet, asset_type, asset_name, asset_symbol, asset_address, chain_id,
    decimals, total_supply, token0_address, token1_address, token0_symbol, token1_symbol,
    pool_type, fee_tier, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.conn.ExecCtx(ctx, query,
		asset.UserWallet, asset.AssetType, asset.AssetName, asset.AssetSymbol, asset.AssetAddress, asset.ChainID,
		asset.Decimals, asset.TotalSupply, asset.Token0Address, asset.Token1Address, asset.Token0Symbol, asset.Token1Symbol,
		asset.PoolType, asset.FeeTier, asset.CreatedAt, asset.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user created asset: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	asset.ID = id
	return nil
}

// GetByID 通过ID获取用户资产
func (r *UserCreatedAssetsRepo) GetByID(ctx context.Context, id int64) (*UserCreatedAsset, error) {
	var asset UserCreatedAsset
	query := "SELECT * FROM user_created_assets WHERE id = ?"
	err := r.conn.QueryRowCtx(ctx, &asset, query, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, nil // 未找到记录
		}
		return nil, fmt.Errorf("failed to get user created asset by id: %w", err)
	}
	return &asset, nil
}

// GetByUserWallet 获取用户创建的所有资产
func (r *UserCreatedAssetsRepo) GetByUserWallet(ctx context.Context, userWallet string) ([]*UserCreatedAsset, error) {
	var assets []*UserCreatedAsset
	query := "SELECT * FROM user_created_assets WHERE user_wallet = ? ORDER BY created_at DESC"
	err := r.conn.QueryRowsCtx(ctx, &assets, query, userWallet)
	if err != nil {
		return nil, fmt.Errorf("failed to get user created assets by wallet: %w", err)
	}
	return assets, nil
}

// GetByUserWalletAndType 获取用户创建的特定类型资产
func (r *UserCreatedAssetsRepo) GetByUserWalletAndType(ctx context.Context, userWallet, assetType string) ([]*UserCreatedAsset, error) {
	var assets []*UserCreatedAsset
	query := "SELECT * FROM user_created_assets WHERE user_wallet = ? AND asset_type = ? ORDER BY created_at DESC"
	err := r.conn.QueryRowsCtx(ctx, &assets, query, userWallet, assetType)
	if err != nil {
		return nil, fmt.Errorf("failed to get user created assets by wallet and type: %w", err)
	}
	return assets, nil
}

// GetByAssetAddress 通过资产地址获取资产
func (r *UserCreatedAssetsRepo) GetByAssetAddress(ctx context.Context, assetAddress string, chainID int64) (*UserCreatedAsset, error) {
	var asset UserCreatedAsset
	query := "SELECT * FROM user_created_assets WHERE asset_address = ? AND chain_id = ?"
	err := r.conn.QueryRowCtx(ctx, &asset, query, assetAddress, chainID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, nil // 未找到记录
		}
		return nil, fmt.Errorf("failed to get user created asset by address: %w", err)
	}
	return &asset, nil
}

// Update 更新用户资产
func (r *UserCreatedAssetsRepo) Update(ctx context.Context, asset *UserCreatedAsset) error {
	asset.UpdatedAt = time.Now()

	query := `
UPDATE user_created_assets SET
    asset_name = ?,
    asset_symbol = ?,
    decimals = ?,
    total_supply = ?,
    token0_symbol = ?,
    token1_symbol = ?,
    updated_at = ?
WHERE id = ?`

	_, err := r.conn.ExecCtx(ctx, query,
		asset.AssetName, asset.AssetSymbol, asset.Decimals, asset.TotalSupply,
		asset.Token0Symbol, asset.Token1Symbol, asset.UpdatedAt, asset.ID)
	if err != nil {
		return fmt.Errorf("failed to update user created asset: %w", err)
	}
	return nil
}

// Delete 删除用户资产
func (r *UserCreatedAssetsRepo) Delete(ctx context.Context, id int64) error {
	query := "DELETE FROM user_created_assets WHERE id = ?"
	_, err := r.conn.ExecCtx(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user created asset: %w", err)
	}
	return nil
}
