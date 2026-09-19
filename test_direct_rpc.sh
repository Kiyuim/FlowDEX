#!/bin/bash

echo "Testing direct RPC calls to trade service on localhost:8081"

# Test the TestRpc method first (should work)
echo -e "\n1. Testing TestRpc method:"
grpcurl -plaintext -d '{"message": "Direct RPC test", "value": 42}' localhost:8081 trade.Trade/TestRpc

# Test the AddLiquidityV1 method
echo -e "\n2. Testing AddLiquidityV1 method:"
grpcurl -plaintext -d '{
  "chain_id": 100000,
  "pool_id": "Bevpu2aknCe7ZotQDRy2LgbG1gtU8S1BFwcpLPziy8af",
  "tick_lower": 100,
  "tick_upper": 200,
  "base_token": 0,
  "base_amount": "1.5",
  "other_amount_max": "2.5",
  "user_wallet_address": "8YUYRxRkjZPzTJyZ34JXd5Z1VC7CrYvxKj8JaKrQXMku",
  "token_a_address": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
  "token_b_address": "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
}' localhost:8081 trade.Trade/AddLiquidityV1

echo -e "\nTests completed" 