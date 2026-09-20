package logic

import (
	"context"
	"fmt"
	"time"

	"dex/model/solmodel"

	"gorm.io/gorm"
)

// PumpToken24hStats holds trade-derived 24h stats for one pair: tx count,
// USD volume, and enough to compute a % price change (earliest vs latest
// trade price seen in the window). Shared by the request-time handler
// (GetPumpTokenList) and the background cache-warming ticker (PumpTicker) —
// the ticker is what actually serves most live traffic since it keeps the
// Redis cache warm, so both must populate these fields the same way.
type PumpToken24hStats struct {
	Txs        uint32
	Vol        float64
	FirstPrice float64
	LastPrice  float64
}

// Fetch24hStats batch-computes trade count, USD volume, and first/last trade
// price over the last 24h for each pair address.
func Fetch24hStats(ctx context.Context, db *gorm.DB, chainId int64, pairAddresses []string) (map[string]PumpToken24hStats, error) {
	result := make(map[string]PumpToken24hStats, len(pairAddresses))
	if len(pairAddresses) == 0 {
		return result, nil
	}

	type row struct {
		PairAddr   string
		Cnt        uint32
		Vol        float64
		FirstPrice float64
		LastPrice  float64
	}
	var rows []row

	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)
	// The consumer writes daily shards, never the legacy trade table.
	// Query each day separately and merge chronologically across midnight.
	for day := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.UTC); !day.After(now); day = day.AddDate(0, 0, 1) {
		table := fmt.Sprintf("trade_%s", day.Format("2006_01_02"))
		if !db.Migrator().HasTable(table) {
			continue
		}
		rows = nil
		err := db.WithContext(ctx).Table(table).
			Select(`pair_addr,
    COUNT(*) as cnt,
    COALESCE(SUM(ABS(total_usd)), 0) as vol,
    CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time ASC, id ASC), ',', 1) AS DECIMAL(65,18)) as first_price,
    CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time DESC, id DESC), ',', 1) AS DECIMAL(65,18)) as last_price`).
			Where("chain_id = ? AND pair_addr IN ? AND block_time >= ? AND block_time <= ? AND trade_type IN ?", chainId, pairAddresses, since, now, []string{"buy", "sell"}).
			Group("pair_addr").Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			old := result[r.PairAddr]
			if old.Txs == 0 {
				old.FirstPrice = r.FirstPrice
			}
			old.Txs += r.Cnt
			old.Vol += r.Vol
			old.LastPrice = r.LastPrice
			result[r.PairAddr] = old
		}
	}

	return result, nil
}

// FetchHolderCounts batch-computes the current holder count for each token
// address, using each token's own CreatedAt as the "since" cutoff (matching
// CountByTokenAddressWithTime's existing per-token semantics).
func FetchHolderCounts(ctx context.Context, db *gorm.DB, chainId int64, tokenAddresses []string, tokenMap map[string]*solmodel.Token) map[string]int64 {
	result := make(map[string]int64, len(tokenAddresses))
	solTokenAccountModel := solmodel.NewSolTokenAccountModel(db)
	for _, tokenAddress := range tokenAddresses {
		token := tokenMap[tokenAddress]
		if token == nil {
			continue
		}
		holders, err := solTokenAccountModel.CountByTokenAddressWithTime(ctx, chainId, tokenAddress, token.CreatedAt)
		if err != nil {
			result[tokenAddress] = 0
			continue
		}
		result[tokenAddress] = holders
	}
	return result
}
