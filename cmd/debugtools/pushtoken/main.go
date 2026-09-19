package main

import (
	"context"
	"fmt"
	"log"

	"dex/market/market"
	"dex/market/marketclient"

	"github.com/zeromicro/go-zero/zrpc"
)

func main() {
	// Create market service client
	marketClient := marketclient.NewMarket(zrpc.MustNewClient(zrpc.RpcClientConf{
		Target: "localhost:8083",
	}))

	// Example: Push a new token creation
	req := &market.PushTokenInfoRequest{
		ChainId:      100000,
		TokenAddress: "7GCihgDB8fe6KNjn2MYtkzZcRjQy3t9GHdC8uHYmW2hr", // Example token
		PairAddress:  "5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1", // Example pair
		TokenPrice:   0.000001234,                                    // Example price
		MktCap:       50000.0,                                        // Example market cap
	}

	// Call PushTokenInfo
	resp, err := marketClient.PushTokenInfo(context.Background(), req)
	if err != nil {
		log.Fatalf("❌ Failed to push token info: %v", err)
	}

	fmt.Printf("✅ Successfully pushed token info!\n")
	fmt.Printf("Response: %+v\n", resp)
}
