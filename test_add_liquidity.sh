#!/bin/bash

echo "Sending add_liquidity request to gateway with corrected tick values..."

curl -v -X POST http://localhost:8083/v1/trade/add_liquidity \
  -H "Content-Type: application/json" \
  -d '{
  "chain_id": 100000,
  "pool_id": "Bevpu2aknCe7ZotQDRy2LgbG1gtU8S1BFwcpLPziy8af",
  "tick_lower": -30720,
  "tick_upper": -15360,
  "base_token": 0,
  "base_amount": "95",
  "other_amount_max": "0.950000",
  "user_wallet_address": "3xbCoRgPcuUhUdsVJHrq79gmcGUT3VwqrHgMTkV296cP",
  "token_a_address": "8Egs1MYoEZoGmSyXE6wM4R9NcfXH1Nazpbo6DxuQ9ybF",
  "token_b_address": "So11111111111111111111111111111111111111112"
}' | tee response.json

echo -e "\nResponse content:"
cat response.json | jq 