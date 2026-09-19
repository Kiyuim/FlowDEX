package solmodel

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserTokenModel = (*customUserTokenModel)(nil)

type (
	// UserTokenModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserTokenModel.
	UserTokenModel interface {
		userTokenModel
		Insert(ctx context.Context, data *UserToken) error
		Update(ctx context.Context, data *UserToken) error
		FindOneByWalletAddressTokenAddress(ctx context.Context, userWalletAddress, tokenAddress string) (*UserToken, error)
		Find(ctx context.Context, userWalletAddress string, chainId int64, offset, limit int64) ([]*UserToken, error)
		Count(ctx context.Context, userWalletAddress string, chainId int64) (int64, error)
	}

	customUserTokenModel struct {
		*defaultUserTokenModel
	}

	// UserToken represents a user-created token record
	UserToken struct {
		Id                int64     `db:"id"`
		UserWalletAddress string    `db:"user_wallet_address"`
		ChainId           int64     `db:"chain_id"`
		TokenAddress      string    `db:"token_address"`
		TokenProgram      string    `db:"token_program"`
		Name              string    `db:"name"`
		Symbol            string    `db:"symbol"`
		Decimals          int64     `db:"decimals"`
		TotalSupply       int64     `db:"total_supply"`
		Icon              string    `db:"icon"`
		Description       string    `db:"description"`
		TxHash            string    `db:"tx_hash"`
		CreatedAt         time.Time `db:"created_at"`
		UpdatedAt         time.Time `db:"updated_at"`
		DeletedAt         time.Time `db:"deleted_at"`
	}
)

// NewUserTokenModel returns a model for the user_tokens table
func NewUserTokenModel(conn sqlx.SqlConn) UserTokenModel {
	return &customUserTokenModel{
		defaultUserTokenModel: newDefaultUserTokenModel(conn),
	}
}

// userTokenModel is the base model interface for user_tokens table
type userTokenModel interface {
	// Define base model interface methods
	Insert(ctx context.Context, data *UserToken) error
	FindOne(ctx context.Context, id int64) (*UserToken, error)
	Update(ctx context.Context, data *UserToken) error
	Delete(ctx context.Context, id int64) error
}

// defaultUserTokenModel is the default implementation of userTokenModel
type defaultUserTokenModel struct {
	conn  sqlx.SqlConn
	table string
}

// newDefaultUserTokenModel creates a new defaultUserTokenModel
func newDefaultUserTokenModel(conn sqlx.SqlConn) *defaultUserTokenModel {
	return &defaultUserTokenModel{
		conn:  conn,
		table: "user_tokens",
	}
}

// Insert inserts a new record into the user_tokens table
func (m *defaultUserTokenModel) Insert(ctx context.Context, data *UserToken) error {
	query := `insert into ` + m.table + ` (user_wallet_address, chain_id, token_address, token_program, name, symbol, decimals, total_supply, icon, description, tx_hash) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := m.conn.ExecCtx(ctx, query, data.UserWalletAddress, data.ChainId, data.TokenAddress, data.TokenProgram, data.Name, data.Symbol, data.Decimals, data.TotalSupply, data.Icon, data.Description, data.TxHash)
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

// FindOne finds a user token by ID
func (m *defaultUserTokenModel) FindOne(ctx context.Context, id int64) (*UserToken, error) {
	query := `select * from ` + m.table + ` where id = ? and deleted_at is null limit 1`
	var resp UserToken
	err := m.conn.QueryRowCtx(ctx, &resp, query, id)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Update updates a user token record
func (m *defaultUserTokenModel) Update(ctx context.Context, data *UserToken) error {
	query := `update ` + m.table + ` set user_wallet_address=?, chain_id=?, token_address=?, token_program=?, name=?, symbol=?, decimals=?, total_supply=?, icon=?, description=?, tx_hash=?, updated_at=? where id=?`
	_, err := m.conn.ExecCtx(ctx, query, data.UserWalletAddress, data.ChainId, data.TokenAddress, data.TokenProgram, data.Name, data.Symbol, data.Decimals, data.TotalSupply, data.Icon, data.Description, data.TxHash, data.UpdatedAt, data.Id)
	return err
}

// Delete deletes a user token record
func (m *defaultUserTokenModel) Delete(ctx context.Context, id int64) error {
	query := `update ` + m.table + ` set deleted_at = ? where id = ?`
	_, err := m.conn.ExecCtx(ctx, query, time.Now(), id)
	return err
}

// FindOneByWalletAddressTokenAddress finds a user token by wallet address and token address
func (m *customUserTokenModel) FindOneByWalletAddressTokenAddress(ctx context.Context, userWalletAddress, tokenAddress string) (*UserToken, error) {
	query := `select * from ` + m.table + ` where user_wallet_address = ? and token_address = ? and deleted_at is null limit 1`
	var resp UserToken
	err := m.conn.QueryRowCtx(ctx, &resp, query, userWalletAddress, tokenAddress)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Find finds user tokens by wallet address with pagination
func (m *customUserTokenModel) Find(ctx context.Context, userWalletAddress string, chainId int64, offset, limit int64) ([]*UserToken, error) {
	var query string
	var args []interface{}

	if chainId > 0 {
		query = `select * from ` + m.table + ` where user_wallet_address = ? and chain_id = ? and deleted_at is null order by id desc limit ?, ?`
		args = append(args, userWalletAddress, chainId, offset, limit)
	} else {
		query = `select * from ` + m.table + ` where user_wallet_address = ? and deleted_at is null order by id desc limit ?, ?`
		args = append(args, userWalletAddress, offset, limit)
	}

	var resp []*UserToken
	err := m.conn.QueryRowsCtx(ctx, &resp, query, args...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Count counts user tokens by wallet address
func (m *customUserTokenModel) Count(ctx context.Context, userWalletAddress string, chainId int64) (int64, error) {
	var query string
	var args []interface{}

	if chainId > 0 {
		query = `select count(id) from ` + m.table + ` where user_wallet_address = ? and chain_id = ? and deleted_at is null`
		args = append(args, userWalletAddress, chainId)
	} else {
		query = `select count(id) from ` + m.table + ` where user_wallet_address = ? and deleted_at is null`
		args = append(args, userWalletAddress)
	}

	var count int64
	err := m.conn.QueryRowCtx(ctx, &count, query, args...)
	if err != nil {
		return 0, err
	}
	return count, nil
}
