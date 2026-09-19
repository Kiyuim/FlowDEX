#!/bin/bash

echo "Testing direct connection to trade service"

# Start the debug proxy
echo "Starting debug proxy..."
node debug_proxy.js &
PROXY_PID=$!

# Wait for the proxy to start
sleep 2

# Test the AddLiquidityV1 endpoint through our debug proxy
echo -e "\nTesting AddLiquidityV1 endpoint:"
curl -v -X POST http://localhost:9000/trade.Trade/AddLiquidityV1 \
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

# Stop the debug proxy
echo -e "\nStopping debug proxy..."
kill $PROXY_PID

echo -e "\nTest completed" 