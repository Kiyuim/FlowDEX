package data

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm/clause"

	"dex/dataflow/internal/constants"

	"github.com/zeromicro/go-zero/core/logx"

	"gorm.io/gorm"
)

var klineTableLock sync.Mutex

type KlineMysqlRepo struct {
	db *gorm.DB
}

type Kline struct {
	Id          int       `gorm:"primary_key"`
	ChainId     int64     `gorm:"column:chain_id" `    // Chain ID
	PairAddress string    `gorm:"column:pair_address"` // Pair address
	CandleTime  int64     `gorm:"column:candle_time"`  // Candle timestamp in seconds
	OpenAt      int64     `gorm:"column:open_at"`      // First transaction timestamp
	CloseAt     int64     `gorm:"column:close_at"`     // Last transaction timestamp
	Open        float64   `gorm:"column:o"`            // Opening price
	Close       float64   `gorm:"column:c"`            // Closing price
	High        float64   `gorm:"column:h"`            // Highest price
	Low         float64   `gorm:"column:l"`            // Lowest price
	Volume      float64   `gorm:"column:v"`            // Volume (USD)
	Tokens      float64   `gorm:"column:t"`            // Volume (tokens)
	AvgPrice    float64   `gorm:"column:a"`            // Average price
	Count       int64     `gorm:"column:count"`        // Number of transactions
	BuyCount    int64     `gorm:"column:buy_count"`    // Number of buy transactions
	SellCount   int64     `gorm:"column:sell_count"`   // Number of sell transactions
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime:true"`
}

func NewKlineMysqlRepo(db *gorm.DB) *KlineMysqlRepo {
	return &KlineMysqlRepo{db: db}
}

func (repo *KlineMysqlRepo) TableName(interval constants.KlineInterval, candleTime int64) string {
	t := time.Unix(candleTime, 0).UTC()
	return fmt.Sprintf("trade_kline_%s_%04d%02d", interval, t.Year(), t.Month())
}

// createTableIfNotExists creates the kline table if it doesn't exist
func (repo *KlineMysqlRepo) createTableIfNotExists(tableName string) error {
	exists := repo.db.Migrator().HasTable(tableName)
	if !exists {
		klineTableLock.Lock()
		defer klineTableLock.Unlock()

		// Double-check after acquiring lock
		exists := repo.db.Migrator().HasTable(tableName)
		if !exists {
			logx.Infof("Creating kline table: %v", tableName)
			err := repo.db.Table(tableName).Migrator().CreateTable(&Kline{})
			if err != nil {
				logx.Errorf("Failed to create kline table %v: %v", tableName, err)
				return err
			}
			logx.Infof("Successfully created kline table: %v", tableName)
		}
	}
	return nil
}

func (repo *KlineMysqlRepo) SaveKlineWithRetry(ctx context.Context, interval constants.KlineInterval, candleTime int64, klines []*Kline) error {
	maxRetry := 3
	var err error
	for i := 0; i < maxRetry; i++ {
		err = repo.SaveKline(ctx, interval, candleTime, klines)
		if err != nil && strings.Contains(err.Error(), "Deadlock found") {
			time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
			continue
		}
		break
	}
	return err
}

func (repo *KlineMysqlRepo) SaveKline(ctx context.Context, interval constants.KlineInterval, candleTime int64, klines []*Kline) error {
	const batchSize = 50
	tableName := repo.TableName(interval, candleTime)

	// Auto-create table if it doesn't exist
	if err := repo.createTableIfNotExists(tableName); err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	sort.Slice(klines, func(i, j int) bool {
		if klines[i].ChainId != klines[j].ChainId {
			return klines[i].ChainId < klines[j].ChainId
		}
		if klines[i].PairAddress != klines[j].PairAddress {
			return klines[i].PairAddress < klines[j].PairAddress
		}
		return klines[i].CandleTime < klines[j].CandleTime
	})

	for i := 0; i < len(klines); i += batchSize {
		end := i + batchSize
		if end > len(klines) {
			end = len(klines)
		}
		batch := klines[i:end]

		err := repo.db.WithContext(ctx).
			Table(tableName).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "chain_id"}, {Name: "pair_address"}, {Name: "candle_time"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"open_at", "close_at", "o", "c", "h", "l", "v", "t", "a", "count", "buy_count", "sell_count", "updated_at",
				}),
			}).CreateInBatches(klines, len(batch)).Error
		if err != nil {
			if strings.Contains(err.Error(), "Deadlock found") {
				time.Sleep(100 * time.Millisecond)
				return repo.SaveKline(ctx, interval, candleTime, batch)
			}
			return err
		}
	}
	return nil
}
