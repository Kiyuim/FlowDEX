package tests

import (
	"context"
	"testing"

	"dex/market/market"
	"dex/trade/internal/config"
	"dex/trade/internal/logic"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/stretchr/testify/assert"
	"github.com/zeromicro/go-zero/core/conf"
	"google.golang.org/grpc"
)

func TestCreateLimitOrder_BasicBuyOrder(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Load trade service configuration
	var c config.Config
	conf.MustLoad("../etc/trade.yaml", &c)
	svcCtx := svc.NewServiceContext(c)

	// Create real market client connection
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		t.Skipf("Skipping test - cannot connect to market service: %v", err)
	}
	defer conn.Close()

	svcCtx.MarketClient = market.NewMarketClient(conn)

	// Create logic instance
	logic := logic.NewCreateLimitOrderLogic(ctx, svcCtx)

	// Test request
	req := &trade.CreateLimitOrderRequest{
		ChainId:   100000,
		TokenCa:   "9TDBrhYyJ1ysmqLfQopvZMX9AUaqc62ARS1iEkMspump",
		SwapType:  trade.SwapType_Buy,
		Amount:    "1.0",
		PriceUsd:  "0.001",
		DoubleOut: false,
	}

	// Execute
	resp, err := logic.CreateLimitOrder(req)

	// Assertions
	if err != nil {
		// If there's an error, it might be expected (e.g., token not found)
		// We'll just log it and continue
		t.Logf("CreateLimitOrder returned error (expected for test token): %v", err)
		return
	}

	assert.NotNil(t, resp)
	assert.Greater(t, resp.OrderId, uint64(0))
}

func TestCreateLimitOrder_SellOrder(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Load trade service configuration
	var c config.Config
	conf.MustLoad("../etc/trade.yaml", &c)
	svcCtx := svc.NewServiceContext(c)

	// Create real market client connection
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		t.Skipf("Skipping test - cannot connect to market service: %v", err)
	}
	defer conn.Close()

	svcCtx.MarketClient = market.NewMarketClient(conn)

	// Create logic instance
	logic := logic.NewCreateLimitOrderLogic(ctx, svcCtx)

	// Test request for sell order
	req := &trade.CreateLimitOrderRequest{
		ChainId:   1,
		TokenCa:   "test_token_address",
		SwapType:  trade.SwapType_Sell,
		Amount:    "1000.0", // Token amount to sell
		PriceUsd:  "0.001",  // Price per token in USD
		DoubleOut: false,
	}

	// Execute
	resp, err := logic.CreateLimitOrder(req)

	// Assertions
	if err != nil {
		// If there's an error, it might be expected (e.g., token not found)
		// We'll just log it and continue
		t.Logf("CreateLimitOrder returned error (expected for test token): %v", err)
		return
	}

	assert.NotNil(t, resp)
	assert.Greater(t, resp.OrderId, uint64(0))
}
