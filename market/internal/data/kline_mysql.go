package data

import (
	"context"
	"fmt"
	"time"

	"dex/market/internal/constants"

	"gorm.io/gorm"
)

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

func (repo *KlineMysqlRepo) TableName(interval constants.KlineInterval, candleTime int64) (table string) {
	if candleTime == 0 {
		table = fmt.Sprintf("trade_kline_%s", interval)
	} else {
		t := time.Unix(candleTime, 0).UTC()
		table = fmt.Sprintf("trade_kline_%s_%04d%02d", interval, t.Year(), t.Month())
	}
	return
}

func (repo *KlineMysqlRepo) QueryLatestPairKline(ctx context.Context, interval constants.KlineInterval, chainId int64, limit int) ([]Kline, error) {
	candleTime := time.Now().Unix() - int64(constants.KlineIntervalSecondsMap[string(interval)])
	var result []Kline
	err := repo.db.WithContext(ctx).Table(repo.TableName("1m", candleTime)).Select("pair_address").
		Where("chain_id = ? and candle_time > ?", chainId, candleTime).Group("pair_address").Order("sum(v) desc").Limit(limit).Scan(&result).Error
	return result, err
}

func (repo *KlineMysqlRepo) QueryKline(ctx context.Context, interval constants.KlineInterval, chainId int64, pairAddress string, fromTime, toTime int64, limit int) (result []Kline, err error) {
	if repo.TableName(interval, fromTime) != repo.TableName(interval, toTime) {
		var resultFirst, resultLast []Kline
		err = repo.db.WithContext(ctx).Table(repo.TableName(interval, fromTime)).
			Where("chain_id = ? and pair_address = ? and candle_time between ? and ?", chainId, pairAddress, fromTime, toTime).
			Order("id desc").Limit(limit).Scan(&resultFirst).Error

		err = repo.db.WithContext(ctx).Table(repo.TableName(interval, toTime)).
			Where("chain_id = ? and pair_address = ? and candle_time between ? and ?", chainId, pairAddress, fromTime, toTime).
			Order("id desc").Limit(limit).Scan(&resultLast).Error

		result = append(resultLast, resultFirst...)
	} else {
		err = repo.db.WithContext(ctx).Table(repo.TableName(interval, toTime)).
			Where("chain_id = ? and pair_address = ? and candle_time between ? and ?", chainId, pairAddress, fromTime, toTime).
			Order("id desc").Limit(limit).Scan(&result).Error
	}
	return result, err
}
