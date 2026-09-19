// Package solprice fetches the real-world SOL/USD price for display purposes.
//
// The project's own on-chain price derivation (consumer's GetBlockSolPrice,
// which watches stablecoin-pool swaps in real blocks) never ran far enough to
// populate block.sol_price — the table has zero non-null rows — because the
// real ingestion pipeline is blocked on the vendored solana-go-sdk
// transaction-deserialization bug (see TODO.md). Seed/demo data has no real
// SOL price either. So anywhere the UI shows "SOL" it needs an independent,
// real source rather than a fabricated one — this package is that source.
package solprice

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

const coingeckoURL = "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd"

var (
	mu        sync.Mutex
	cached    float64
	fetchedAt time.Time
)

const cacheTTL = 60 * time.Second

// GetSolUsdPrice returns the current SOL/USD price, cached for cacheTTL to
// stay well under CoinGecko's free-tier rate limit. Returns the last known
// good price (possibly 0 on the very first call) if the fetch fails, rather
// than erroring the whole request over a price-only lookup.
func GetSolUsdPrice() float64 {
	mu.Lock()
	if time.Since(fetchedAt) < cacheTTL && cached > 0 {
		defer mu.Unlock()
		return cached
	}
	mu.Unlock()

	price, err := fetch()
	if err != nil {
		mu.Lock()
		defer mu.Unlock()
		return cached // stale-if-error
	}

	mu.Lock()
	cached = price
	fetchedAt = time.Now()
	mu.Unlock()
	return price
}

func fetch() (float64, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(coingeckoURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var parsed struct {
		Solana struct {
			Usd float64 `json:"usd"`
		} `json:"solana"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	return parsed.Solana.Usd, nil
}
