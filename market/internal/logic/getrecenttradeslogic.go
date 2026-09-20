package logic

import (
	"context"
	"fmt"
	"sort"
	"time"

	"dex/market/internal/svc"
	"dex/market/market"
	"dex/pkg/solprice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRecentTradesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRecentTradesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRecentTradesLogic {
	return &GetRecentTradesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type recentTradeRow struct {
	TxHash          string
	TradeType       string
	BaseTokenAmount float64
	TokenAmount     float64
	TokenPriceUsd   float64
	TotalUsd        float64
	Maker           string
	BlockTimeStamp  int64
}

type pairStatsRow struct {
	Cnt        int64
	Buys       int64
	Sells      int64
	Vol        float64
	Traders    int64
	FirstPrice float64
	LastPrice  float64
}

// GetRecentTrades serves a pair's latest indexed trades and 24h stats from the
// consumer's daily trade shards — the same index the token list is built
// from, so the token page and its list card cannot disagree, and it answers
// in one DB round-trip instead of the client paging getParsedTransaction
// one signature at a time through a rate-limited RPC.
func (l *GetRecentTradesLogic) GetRecentTrades(in *market.GetRecentTradesRequest) (*market.GetRecentTradesResponse, error) {
	if in.PairAddress == "" {
		return nil, fmt.Errorf("pair_address is required")
	}
	limit := int(in.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	db := l.svcCtx.DB
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	var rows []recentTradeRow
	stats := pairStatsRow{}
	firstSet := false
	// Newest shard first so the row limit is hit with the latest trades.
	for day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC); !day.Before(time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.UTC)); day = day.AddDate(0, 0, -1) {
		table := fmt.Sprintf("trade_%s", day.Format("2006_01_02"))
		if !db.Migrator().HasTable(table) {
			continue
		}
		if len(rows) < limit {
			var part []recentTradeRow
			if err := db.WithContext(l.ctx).Table(table).
				Select("tx_hash, trade_type, base_token_amount, token_amount, token_price_usd, total_usd, maker, block_time_stamp").
				Where("chain_id = ? AND pair_addr = ? AND trade_type IN ?", in.ChainId, in.PairAddress, []string{"buy", "sell"}).
				Order("block_time_stamp DESC, id DESC").Limit(limit - len(rows)).Scan(&part).Error; err != nil {
				return nil, err
			}
			rows = append(rows, part...)
		}

		var s pairStatsRow
		if err := db.WithContext(l.ctx).Table(table).
			Select(`COUNT(*) as cnt,
    SUM(trade_type = 'buy') as buys,
    SUM(trade_type = 'sell') as sells,
    COALESCE(SUM(ABS(total_usd)), 0) as vol,
    COUNT(DISTINCT maker) as traders,
    CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time ASC, id ASC), ',', 1) AS DECIMAL(65,18)) as first_price,
    CAST(SUBSTRING_INDEX(GROUP_CONCAT(token_price_usd ORDER BY block_time DESC, id DESC), ',', 1) AS DECIMAL(65,18)) as last_price`).
			Where("chain_id = ? AND pair_addr = ? AND block_time >= ? AND block_time <= ? AND trade_type IN ?", in.ChainId, in.PairAddress, since, now, []string{"buy", "sell"}).
			Scan(&s).Error; err != nil {
			return nil, err
		}
		if s.Cnt == 0 {
			continue
		}
		// Iterating newest day first: the first non-empty day holds the latest
		// price, the last one holds the earliest.
		if !firstSet {
			stats.LastPrice = s.LastPrice
			firstSet = true
		}
		stats.FirstPrice = s.FirstPrice
		stats.Cnt += s.Cnt
		stats.Buys += s.Buys
		stats.Sells += s.Sells
		stats.Vol += s.Vol
		// distinct makers can overlap across days; take the max as a floor
		if s.Traders > stats.Traders {
			stats.Traders = s.Traders
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].BlockTimeStamp > rows[j].BlockTimeStamp })

	list := make([]*market.RecentTrade, 0, len(rows))
	for _, r := range rows {
		list = append(list, &market.RecentTrade{
			TxHash:          r.TxHash,
			TradeType:       r.TradeType,
			BaseTokenAmount: r.BaseTokenAmount,
			TokenAmount:     r.TokenAmount,
			TokenPriceUsd:   r.TokenPriceUsd,
			TotalUsd:        r.TotalUsd,
			Maker:           r.Maker,
			BlockTime:       r.BlockTimeStamp,
		})
	}

	var change float64
	if stats.FirstPrice > 0 {
		change = (stats.LastPrice - stats.FirstPrice) / stats.FirstPrice * 100
	}
	return &market.GetRecentTradesResponse{
		List: list,
		Stats: &market.PairStats24H{
			LastPriceUsd: stats.LastPrice,
			Change_24H:   change,
			Vol_24HUsd:   stats.Vol,
			Txs_24H:      stats.Cnt,
			Buys_24H:     stats.Buys,
			Sells_24H:    stats.Sells,
			Traders_24H:  stats.Traders,
		},
		SolPriceUsd: solprice.GetSolUsdPrice(),
	}, nil
}
