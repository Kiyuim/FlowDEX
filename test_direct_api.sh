#!/bin/bash

echo "Testing API endpoints directly"

# Test the TestRpc endpoint (should work)
echo -e "\n1. Testing TestRpc endpoint:"
curl -v -X POST http://localhost:8083/v1/trade/test_rpc \
  -H "Content-Type: application/json" \
  -d '{"message": "API test", "value": 42}'

# Test the AddLiquidityV1 endpoint with exact payload from frontend
echo -e "\n2. Testing AddLiquidityV1 endpoint:"
curl -v -X POST http://localhost:8083/v1/trade/add_liquidity_v1 \
  -H "Content-Type: application/json" \
  -d '{
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
  }'

echo -e "\nTests completed" 