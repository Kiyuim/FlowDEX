package logic

import (
	"context"
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

	since := time.Now().Add(-24 * time.Hour)
	// GROUP_CONCAT + SUBSTRING_INDEX picks the first/last value in each
	// ORDER BY group without relying on window functions.
	err := db.WithContext(ctx).
		Model(&solmodel.Trade{}).
		Select(`pair_addr as pair_addr,
			COUNT(*) as cnt,
			COALESCE(SUM(total_usd), 0) as vol,
			CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time ASC), ',', 1) AS DECIMAL(65,18)) as first_price,
			CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time DESC), ',', 1) AS DECIMAL(65,18)) as last_price`).
		Where("chain_id = ? AND pair_addr IN ? AND block_time >= ?", chainId, pairAddresses, since).
		Group("pair_addr").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.PairAddr] = PumpToken24hStats{
			Txs: r.Cnt, Vol: r.Vol, FirstPrice: r.FirstPrice, LastPrice: r.LastPrice,
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
