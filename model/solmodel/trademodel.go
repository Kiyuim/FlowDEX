package solmodel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	. "github.com/klen-ygs/gorm-zero/gormc/sql"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// avoid unused err
var _ = InitField
var _ TradeModel = (*customTradeModel)(nil)

var tradeModelLock sync.Mutex
var tableLock sync.Mutex

type (
	// TradeModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTradeModel.
	TradeModel interface {
		tradeModel
		customTradeLogicModel
	}

	customTradeLogicModel interface {
		WithSession(tx *gorm.DB) TradeModel
		BatchInsertTrades(ctx context.Context, trades []*Trade) error
		GetNativeTokenPrice(ctx context.Context, chainId int64, searchTime time.Time) (float64, error)
		FindOneByTxHashWithTime(ctx context.Context, txHash string, createdAt time.Time) (*Trade, error)
	}

	customTradeModel struct {
		*defaultTradeModel
	}
)

func (c customTradeModel) WithSession(tx *gorm.DB) TradeModel {
	newModel := *c.defaultTradeModel
	c.defaultTradeModel = &newModel
	c.conn = tx
	return c
}

// NewTradeModel returns a model for the database table.
func NewTradeModel(conn *gorm.DB) TradeModel {
	return &customTradeModel{
		defaultTradeModel: newTradeModel(conn),
	}
}

func (m *defaultTradeModel) customCacheKeys(data *Trade) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

func (m *defaultTradeModel) GetNativeTokenPrice(ctx context.Context, chainId int64, searchTime time.Time) (float64, error) {
	table := getShardTableNameByTime("trade", searchTime)

	var resp Trade
	// Query the current shard (table)
	err := m.conn.WithContext(ctx).Table(table).Where("chain_id = ? AND block_time > ?", chainId, searchTime).
		Order("block_time ASC").First(&resp).Error

	if err != nil {
		// Return any other error
		return 0, err
	}

	// Return the base token price if a record is found
	return resp.BaseTokenPriceUsd, nil
}

// FindOneByTxHashWithTime looks up an on-chain fill by tx hash in the daily
// shard tables around the order's creation time — the order's day and the next
// (a tx submitted at 23:59 can land in a block after midnight, and created_at /
// block_time may sit in different timezones). Shard tables that don't exist
// (days with no trades at all) are skipped, not errors. Returns
// gorm.ErrRecordNotFound when the tx isn't indexed yet.
func (m *defaultTradeModel) FindOneByTxHashWithTime(ctx context.Context, txHash string, createdAt time.Time) (*Trade, error) {
	for i := 0; i <= 1; i++ {
		table := getShardTableNameByTime("trade", createdAt.AddDate(0, 0, i))
		if !m.conn.Migrator().HasTable(table) {
			continue
		}
		var resp Trade
		err := m.conn.WithContext(ctx).Table(table).Where("tx_hash = ?", txHash).First(&resp).Error
		if err == nil {
			return &resp, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// getShardTableNameByTime generates a table name based on the given timestamp (30-minute interval)
func getShardTableNameByTime(baseTable string, t time.Time) string {
	// Format the table name as: baseTable_<year>_<month>_<day>
	return fmt.Sprintf("%s_%d_%02d_%02d",
		baseTable,
		t.Year(),  // Year
		t.Month(), // Month
		t.Day())   // Day
}

// createShardTableIfNotExists 自动创建表（如果不存在）
func (m *defaultTradeModel) createShardTableIfNotExists(table string) error {

	exists := m.conn.Migrator().HasTable(table)
	if !exists {
		tableLock.Lock()
		defer tableLock.Unlock()
		exists := m.conn.Migrator().HasTable(table)
		if !exists {
			logx.Infof("createShardTableIfNotExists will create table: %v", table)
			err := m.conn.Table(table).Migrator().CreateTable(&Trade{})
			if err != nil {
				return err
			}
			logx.Infof("createShardTableIfNotExists:table success: %v", table)
		}
	}
	return nil
}

func (m *defaultTradeModel) BatchInsertTrades(ctx context.Context, trades []*Trade) error {
	if len(trades) == 0 {
		return nil
	}

	// Group data by table name
	tableGroups := make(map[string][]*Trade)
	for _, trade := range trades {
		table := getShardTableNameByTime("trade", trade.BlockTime)
		tableGroups[table] = append(tableGroups[table], trade)
	}

	// tradeModelLock.Lock()
	// defer tradeModelLock.Unlock()
	// Insert data into each table
	for table, group := range tableGroups {
		// Ensure the table exists
		if err := m.createShardTableIfNotExists(table); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table, err)
		}

		// Batch insert data into the table
		// err := m.conn.WithContext(ctx).Table(table).Clauses(clause.OnConflict{
		// 	Columns:   []clause.Column{{Name: "hash_id_index"}}, // Update on conflict using unique index
		// 	UpdateAll: true,
		// }).CreateInBatches(group, 1024).Error
		// now := time.Now()
		err := m.conn.WithContext(ctx).Table(table).CreateInBatches(group, 1024).Error
		// logx.Infof("BatchInsertTrades size: %v, cost: %v", len(group), time.Since(now))
		if err != nil && strings.Contains(err.Error(), "Deadlock found when trying to get lock") {
			tradeModelLock.Lock()
			err := m.conn.WithContext(ctx).Table(table).CreateInBatches(group, 1024).Error
			tradeModelLock.Unlock()
			return errors.Wrap(err, fmt.Sprintf("BatchInsertTrades error after get lock: %v", len(group)))
		}

		if err != nil {
			return fmt.Errorf("failed to batch insert into table %s: %w", table, err)
		}
	}

	return nil
}
