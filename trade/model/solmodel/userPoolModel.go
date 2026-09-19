package solmodel

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserPoolModel = (*customUserPoolModel)(nil)

type (
	// UserPoolModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserPoolModel.
	UserPoolModel interface {
		userPoolModel
		Insert(ctx context.Context, data *UserPool) error
		Update(ctx context.Context, data *UserPool) error
		FindOneByWalletAddressPoolState(ctx context.Context, userWalletAddress, poolState string) (*UserPool, error)
		Find(ctx context.Context, userWalletAddress string, chainId int64, poolType, poolVersion string, offset, limit int64) ([]*UserPool, error)
		Count(ctx context.Context, userWalletAddress string, chainId int64, poolType, poolVersion string) (int64, error)
	}

	customUserPoolModel struct {
		*defaultUserPoolModel
	}

	// UserPool represents a user-created pool record
	UserPool struct {
		Id                int64     `db:"id"`
		UserWalletAddress string    `db:"user_wallet_address"`
		ChainId           int64     `db:"chain_id"`
		PoolState         string    `db:"pool_state"`
		InputVaultMint    string    `db:"input_vault_mint"`
		OutputVaultMint   string    `db:"output_vault_mint"`
		Token0Symbol      string    `db:"token0_symbol"`
		Token1Symbol      string    `db:"token1_symbol"`
		Token0Decimals    int64     `db:"token0_decimals"`
		Token1Decimals    int64     `db:"token1_decimals"`
		Token0Liquidity   int64     `db:"token0_liquidity"`
		Token1Liquidity   int64     `db:"token1_liquidity"`
		InitialPrice      float64   `db:"initial_price"`
		TradeFeeRate      int64     `db:"trade_fee_rate"`
		TxHash            string    `db:"tx_hash"`
		PoolVersion       string    `db:"pool_version"`
		PoolType          string    `db:"pool_type"`
		AmmConfig         string    `db:"amm_config"`
		CreatedAt         time.Time `db:"created_at"`
		UpdatedAt         time.Time `db:"updated_at"`
		DeletedAt         time.Time `db:"deleted_at"`
	}
)

// NewUserPoolModel returns a model for the user_pools table
func NewUserPoolModel(conn sqlx.SqlConn) UserPoolModel {
	return &customUserPoolModel{
		defaultUserPoolModel: newDefaultUserPoolModel(conn),
	}
}

// userPoolModel is the base model interface for user_pools table
type userPoolModel interface {
	// Define base model interface methods
	Insert(ctx context.Context, data *UserPool) error
	FindOne(ctx context.Context, id int64) (*UserPool, error)
	Update(ctx context.Context, data *UserPool) error
	Delete(ctx context.Context, id int64) error
}

// defaultUserPoolModel is the default implementation of userPoolModel
type defaultUserPoolModel struct {
	conn  sqlx.SqlConn
	table string
}

// newDefaultUserPoolModel creates a new defaultUserPoolModel
func newDefaultUserPoolModel(conn sqlx.SqlConn) *defaultUserPoolModel {
	return &defaultUserPoolModel{
		conn:  conn,
		table: "user_pools",
	}
}

// Insert inserts a new record into the user_pools table
func (m *defaultUserPoolModel) Insert(ctx context.Context, data *UserPool) error {
	query := `insert into ` + m.table + ` (user_wallet_address, chain_id, pool_state, input_vault_mint, output_vault_mint, 
		token0_symbol, token1_symbol, token0_decimals, token1_decimals, token0_liquidity, token1_liquidity, 
		initial_price, trade_fee_rate, tx_hash, pool_version, pool_type, amm_config) 
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := m.conn.ExecCtx(ctx, query,
		data.UserWalletAddress, data.ChainId, data.PoolState, data.InputVaultMint, data.OutputVaultMint,
		data.Token0Symbol, data.Token1Symbol, data.Token0Decimals, data.Token1Decimals, data.Token0Liquidity, data.Token1Liquidity,
		data.InitialPrice, data.TradeFeeRate, data.TxHash, data.PoolVersion, data.PoolType, data.AmmConfig)

	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	data.Id = id
	return nil
}

// FindOne finds a user pool by ID
func (m *defaultUserPoolModel) FindOne(ctx context.Context, id int64) (*UserPool, error) {
	query := `select * from ` + m.table + ` where id = ? and deleted_at is null limit 1`
	var resp UserPool
	err := m.conn.QueryRowCtx(ctx, &resp, query, id)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Update updates a user pool record
func (m *defaultUserPoolModel) Update(ctx context.Context, data *UserPool) error {
	query := `update ` + m.table + ` set user_wallet_address=?, chain_id=?, pool_state=?, input_vault_mint=?, output_vault_mint=?, 
		token0_symbol=?, token1_symbol=?, token0_decimals=?, token1_decimals=?, token0_liquidity=?, token1_liquidity=?, 
		initial_price=?, trade_fee_rate=?, tx_hash=?, pool_version=?, pool_type=?, amm_config=?, updated_at=? where id=?`

	_, err := m.conn.ExecCtx(ctx, query,
		data.UserWalletAddress, data.ChainId, data.PoolState, data.InputVaultMint, data.OutputVaultMint,
		data.Token0Symbol, data.Token1Symbol, data.Token0Decimals, data.Token1Decimals, data.Token0Liquidity, data.Token1Liquidity,
		data.InitialPrice, data.TradeFeeRate, data.TxHash, data.PoolVersion, data.PoolType, data.AmmConfig, data.UpdatedAt, data.Id)

	return err
}

// Delete deletes a user pool record
func (m *defaultUserPoolModel) Delete(ctx context.Context, id int64) error {
	query := `update ` + m.table + ` set deleted_at = ? where id = ?`
	_, err := m.conn.ExecCtx(ctx, query, time.Now(), id)
	return err
}

// FindOneByWalletAddressPoolState finds a user pool by wallet address and pool state
func (m *customUserPoolModel) FindOneByWalletAddressPoolState(ctx context.Context, userWalletAddress, poolState string) (*UserPool, error) {
	query := `select * from ` + m.table + ` where user_wallet_address = ? and pool_state = ? and deleted_at is null limit 1`
	var resp UserPool
	err := m.conn.QueryRowCtx(ctx, &resp, query, userWalletAddress, poolState)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Find finds user pools by wallet address with pagination
func (m *customUserPoolModel) Find(ctx context.Context, userWalletAddress string, chainId int64, poolType, poolVersion string, offset, limit int64) ([]*UserPool, error) {
	var query string
	var args []interface{}

	// Base query
	queryBase := `select * from ` + m.table + ` where user_wallet_address = ? and deleted_at is null`
	args = append(args, userWalletAddress)

	// Add filters
	if chainId > 0 {
		queryBase += ` and chain_id = ?`
		args = append(args, chainId)
	}

	if poolType != "" {
		queryBase += ` and pool_type = ?`
		args = append(args, poolType)
	}

	if poolVersion != "" {
		queryBase += ` and pool_version = ?`
		args = append(args, poolVersion)
	}

	// Add order and limits
	query = queryBase + ` order by id desc limit ?, ?`
	args = append(args, offset, limit)

	var resp []*UserPool
	err := m.conn.QueryRowsCtx(ctx, &resp, query, args...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Count counts user pools by wallet address
func (m *customUserPoolModel) Count(ctx context.Context, userWalletAddress string, chainId int64, poolType, poolVersion string) (int64, error) {
	var query string
	var args []interface{}

	// Base query
	queryBase := `select count(id) from ` + m.table + ` where user_wallet_address = ? and deleted_at is null`
	args = append(args, userWalletAddress)

	// Add filters
	if chainId > 0 {
		queryBase += ` and chain_id = ?`
		args = append(args, chainId)
	}

	if poolType != "" {
		queryBase += ` and pool_type = ?`
		args = append(args, poolType)
	}

	if poolVersion != "" {
		queryBase += ` and pool_version = ?`
		args = append(args, poolVersion)
	}

	query = queryBase

	var count int64
	err := m.conn.QueryRowCtx(ctx, &count, query, args...)
	if err != nil {
		return 0, err
	}
	return count, nil
}
