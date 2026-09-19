package main

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"math"
// 	"math/rand"
// 	"sync"
// 	"time"

// 	"github.com/redis/go-redis/v9"
// )

// type KlineUpdateMessage struct {
// 	PairAddress string  `json:"pair_address"`
// 	ChainId     int64   `json:"chain_id"`
// 	Interval    string  `json:"interval"`
// 	CandleTime  int64   `json:"candle_time"`
// 	Open        float64 `json:"open"`
// 	High        float64 `json:"high"`
// 	Low         float64 `json:"low"`
// 	Close       float64 `json:"close"`
// 	Volume      float64 `json:"volume"`
// 	Timestamp   int64   `json:"timestamp"`
// }

// // PairState maintains the state for each trading pair
// type PairState struct {
// 	CurrentPrice  float64
// 	Trend         float64 // -1 to 1, negative for downtrend, positive for uptrend
// 	TrendStrength float64 // 0 to 1, how strong the trend is
// 	Volatility    float64 // current volatility level
// 	BaseVolume    float64 // base volume level
// 	LastUpdate    time.Time
// 	PriceHistory  []float64 // keep last 20 prices for trend analysis
// 	mu            sync.RWMutex
// }

// // Global state for all pairs
// var pairStates = make(map[string]*PairState)
// var statesMutex sync.RWMutex

// func main() {
// 	// Connect to Redis (using the same config as dataflow)
// 	rdb := redis.NewClient(&redis.Options{
// 		Addr:     "r-uf6ka4mjfgkkgbqlhepd.redis.rds.aliyuncs.com:6379",
// 		Password: "Web3itefun",
// 		DB:       10,
// 	})

// 	ctx := context.Background()

// 	// Test connection
// 	_, err := rdb.Ping(ctx).Result()
// 	if err != nil {
// 		log.Fatalf("Failed to connect to Redis: %v", err)
// 	}

// 	fmt.Println("🔗 Connected to Redis successfully!")
// 	fmt.Println("📡 Starting realistic mock kline data generator...")
// 	fmt.Println("Press Ctrl+C to stop\n")

// 	// Configuration
// 	pairs := []string{
// 		"4AQyBkYzGprZcZfbvNsbJWSD93gFiTUed4Dvodqi67kR",
// 	}

// 	intervals := []string{"1m", "5m", "15m", "1h", "4h", "1d"}
// 	intervalSeconds := map[string]int64{
// 		"1m":  60,
// 		"5m":  300,
// 		"15m": 900,
// 		"1h":  3600,
// 		"4h":  14400,
// 		"1d":  86400,
// 	}

// 	// Initialize pair states
// 	initializePairStates(pairs)

// 	// Start continuous price updates (every 1 second)
// 	go continuousPriceUpdater(ctx)

// 	// Main ticker for publishing kline data
// 	ticker := time.NewTicker(5 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			// Generate and publish kline data for each pair and interval
// 			for _, pair := range pairs {
// 				for _, interval := range intervals {
// 					mockData := generateRealisticKlineData(pair, interval, intervalSeconds[interval])

// 					// Publish to Redis
// 					data, err := json.Marshal(mockData)
// 					if err != nil {
// 						log.Printf("❌ Failed to marshal mock data: %v", err)
// 						continue
// 					}

// 					result := rdb.Publish(ctx, "kline:updates", data)
// 					if err := result.Err(); err != nil {
// 						log.Printf("❌ Failed to publish to Redis: %v", err)
// 						continue
// 					}

// 					subscribers := result.Val()

// 					// Get current state for logging
// 					statesMutex.RLock()
// 					state := pairStates[pair]
// 					trendIndicator := "📈"
// 					if state.Trend < -0.1 {
// 						trendIndicator = "📉"
// 					} else if state.Trend > 0.1 {
// 						trendIndicator = "📈"
// 					} else {
// 						trendIndicator = "➡️"
// 					}
// 					statesMutex.RUnlock()

// 					fmt.Printf("%s %s-%s: price=%.8f, vol=%.0f, trend=%.2f, subs=%d\n",
// 						trendIndicator, pair[:8], interval, mockData.Close, mockData.Volume, state.Trend, subscribers)
// 				}
// 			}

// 		case <-ctx.Done():
// 			return
// 		}
// 	}
// }

// func initializePairStates(pairs []string) {
// 	statesMutex.Lock()
// 	defer statesMutex.Unlock()

// 	for _, pair := range pairs {
// 		pairStates[pair] = &PairState{
// 			CurrentPrice:  0.000005 + rand.Float64()*0.000010, // Random starting price
// 			Trend:         (rand.Float64() - 0.5) * 0.5,       // Initial trend
// 			TrendStrength: rand.Float64() * 0.7,               // Initial trend strength
// 			Volatility:    0.02 + rand.Float64()*0.08,         // 2-10% volatility
// 			BaseVolume:    5000 + rand.Float64()*15000,        // Base volume 5k-20k
// 			LastUpdate:    time.Now(),
// 			PriceHistory:  make([]float64, 0, 20),
// 		}
// 	}
// }

// func continuousPriceUpdater(ctx context.Context) {
// 	ticker := time.NewTicker(1 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			updatePriceStates()
// 		case <-ctx.Done():
// 			return
// 		}
// 	}
// }

// func updatePriceStates() {
// 	statesMutex.Lock()
// 	defer statesMutex.Unlock()

// 	for pair, state := range pairStates {
// 		state.mu.Lock()

// 		now := time.Now()
// 		timeDelta := now.Sub(state.LastUpdate).Seconds()

// 		// Update trend occasionally (every 30-120 seconds)
// 		if rand.Float64() < 0.02 { // 2% chance per second
// 			// Trend can shift
// 			state.Trend += (rand.Float64() - 0.5) * 0.3
// 			state.Trend = math.Max(-1, math.Min(1, state.Trend))

// 			// Trend strength changes
// 			state.TrendStrength += (rand.Float64() - 0.5) * 0.2
// 			state.TrendStrength = math.Max(0, math.Min(1, state.TrendStrength))

// 			// Volatility changes over time
// 			state.Volatility += (rand.Float64() - 0.5) * 0.01
// 			state.Volatility = math.Max(0.005, math.Min(0.15, state.Volatility))
// 		}

// 		// Calculate price change based on trend and volatility
// 		trendComponent := state.Trend * state.TrendStrength * 0.001 * timeDelta
// 		randomComponent := (rand.Float64() - 0.5) * state.Volatility * 0.1 * math.Sqrt(timeDelta)

// 		// Mean reversion component (prevents prices from going too extreme)
// 		meanReversionTarget := 0.000008 // Target price
// 		meanReversionComponent := (meanReversionTarget - state.CurrentPrice) * 0.0001 * timeDelta

// 		// Apply price change
// 		priceChange := trendComponent + randomComponent + meanReversionComponent
// 		newPrice := state.CurrentPrice * (1 + priceChange)

// 		// Prevent negative prices and extreme values
// 		newPrice = math.Max(0.000001, math.Min(0.001, newPrice))

// 		state.CurrentPrice = newPrice
// 		state.LastUpdate = now

// 		// Update price history
// 		state.PriceHistory = append(state.PriceHistory, newPrice)
// 		if len(state.PriceHistory) > 20 {
// 			state.PriceHistory = state.PriceHistory[1:]
// 		}

// 		state.mu.Unlock()

// 		_ = pair // Use pair variable to avoid unused variable warning
// 	}
// }

// func generateRealisticKlineData(pairAddress, interval string, intervalSec int64) KlineUpdateMessage {
// 	now := time.Now().Unix()
// 	candleTime := (now / intervalSec) * intervalSec

// 	statesMutex.RLock()
// 	state := pairStates[pairAddress]
// 	statesMutex.RUnlock()

// 	state.mu.RLock()
// 	defer state.mu.RUnlock()

// 	// Generate OHLC data based on current price and recent history
// 	currentPrice := state.CurrentPrice

// 	// For realistic candles, we need to simulate price movement within the interval
// 	volatilityFactor := getVolatilityFactor(interval)
// 	intervalVolatility := state.Volatility * volatilityFactor

// 	// Generate open price (slightly different from current)
// 	openPrice := currentPrice * (1 + (rand.Float64()-0.5)*intervalVolatility*0.3)

// 	// Generate close price with trend bias
// 	trendBias := state.Trend * state.TrendStrength * intervalVolatility * 0.5
// 	closePrice := openPrice * (1 + trendBias + (rand.Float64()-0.5)*intervalVolatility*0.7)

// 	// Generate high/low based on volatility
// 	priceRange := math.Max(openPrice, closePrice) * intervalVolatility * (0.5 + rand.Float64()*1.5)
// 	highPrice := math.Max(openPrice, closePrice) + priceRange*rand.Float64()
// 	lowPrice := math.Min(openPrice, closePrice) - priceRange*rand.Float64()

// 	// Ensure logical order
// 	highPrice = math.Max(highPrice, math.Max(openPrice, closePrice))
// 	lowPrice = math.Min(lowPrice, math.Min(openPrice, closePrice))

// 	// Generate volume based on price movement and volatility
// 	priceMovement := math.Abs(closePrice-openPrice) / openPrice
// 	volumeMultiplier := 1 + priceMovement*10 // Higher volume with bigger moves
// 	volume := state.BaseVolume * volumeMultiplier * (0.5 + rand.Float64()*1.5)

// 	// Add some volume spikes occasionally
// 	if rand.Float64() < 0.1 { // 10% chance of volume spike
// 		volume *= 2 + rand.Float64()*3
// 	}

// 	return KlineUpdateMessage{
// 		PairAddress: pairAddress,
// 		ChainId:     100000,
// 		Interval:    interval,
// 		CandleTime:  candleTime,
// 		Open:        openPrice,
// 		High:        highPrice,
// 		Low:         lowPrice,
// 		Close:       closePrice,
// 		Volume:      volume,
// 		Timestamp:   now,
// 	}
// }

// func getVolatilityFactor(interval string) float64 {
// 	// Longer intervals tend to have higher volatility
// 	switch interval {
// 	case "1m":
// 		return 0.3
// 	case "5m":
// 		return 0.5
// 	case "15m":
// 		return 0.7
// 	case "1h":
// 		return 1.0
// 	case "4h":
// 		return 1.5
// 	case "1d":
// 		return 2.0
// 	default:
// 		return 1.0
// 	}
// }
