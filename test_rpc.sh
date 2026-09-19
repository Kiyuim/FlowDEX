#!/bin/bash

echo "Sending add_liquidity request to gateway..."

curl -v -X POST http://localhost:8083/v1/trade/add_liquidity \
  -H "Content-Type: application/json" \
  -d '{
    "chain_id": 100000,
    "pool_id": "8JUjWjVdqtW5LJYnUkDgMjwx6uKn3STGxhX5G8TJYPVb",
    "tick_lower": 0,
    "tick_upper": 0,
    "base_token": 0,
    "base_amount": "1000000",
    "other_amount_max": "1000000",
    "user_wallet_address": "DfLZV18rD7wCQwjYvhTFwuvLh49WSbfAdyNiwdMN7JDS",
    "token_a_address": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
    "token_b_address": "DEeEbtiiQA4zBLjonmCPMpzADXSmQRbyW2XJNk5DF1mD"
  }'

echo -e "\n\nNow trying with the /api/ prefix..."

curl -v -X POST http://localhost:8083/api/v1/trade/add_liquidity \
  -H "Content-Type: application/json" \
  -d '{
    "chain_id": 100000,
    "pool_id": "8JUjWjVdqtW5LJYnUkDgMjwx6uKn3STGxhX5G8TJYPVb",
    "tick_lower": 0,
    "tick_upper": 0,
    "base_token": 0,
    "base_amount": "1000000",
    "other_amount_max": "1000000",
    "user_wallet_address": "DfLZV18rD7wCQwjYvhTFwuvLh49WSbfAdyNiwdMN7JDS",
    "token_a_address": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
    "token_b_address": "DEeEbtiiQA4zBLjonmCPMpzADXSmQRbyW2XJNk5DF1mD"
  }' 