package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Import the generated protobuf code
	pb "dex/trade/trade"
)

func main() {
	// Set up a connection to the server
	conn, err := grpc.Dial("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := pb.NewTradeClient(conn)

	// Set up a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create the request payload
	req := &pb.AddLiquidityRequest{
		ChainId:           100000,
		PoolId:            "Bevpu2aknCe7ZotQDRy2LgbG1gtU8S1BFwcpLPziy8af",
		TickLower:         100,
		TickUpper:         200,
		BaseToken:         0,
		BaseAmount:        "1.5",
		OtherAmountMax:    "2.5",
		UserWalletAddress: "8YUYRxRkjZPzTJyZ34JXd5Z1VC7CrYvxKj8JaKrQXMku",
		TokenAAddress:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		TokenBAddress:     "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
	}

	// Print the request as JSON
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	fmt.Printf("Request:\n%s\n\n", reqJSON)

	// Call the service
	resp, err := client.AddLiquidityV1(ctx, req)
	if err != nil {
		log.Fatalf("Failed to call AddLiquidityV1: %v", err)
	}

	// Print the response
	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("Response:\n%s\n", respJSON)
}
