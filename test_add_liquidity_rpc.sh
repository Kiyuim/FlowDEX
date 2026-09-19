#!/bin/bash

# 测试AddLiquidityV1 RPC方法
echo "测试 AddLiquidityV1 RPC方法..."

# 使用grpcurl工具直接调用RPC服务
# 如果没有安装grpcurl，请先安装：
# go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# RPC请求数据
REQUEST='{
  "chain_id": 100000,
  "pool_id": "Bevpu2aknCe7ZotQDRy2LgbG1gtU8S1BFwcpLPziy8af",
  "tick_lower": 100,
  "tick_upper": 200,
  "base_token": 0,
  "base_amount": "1.5",
  "other_amount_max": "10",
  "user_wallet_address": "YOUR_WALLET_ADDRESS",
  "token_a_address": "TOKEN_A_ADDRESS",
  "token_b_address": "TOKEN_B_ADDRESS"
}'

# 直接调用RPC服务
echo "请求数据: $REQUEST"
echo "调用RPC服务..."

# 使用grpcurl调用
# 注意：这里使用的是-plaintext参数，如果服务使用TLS则需要移除此参数或提供证书
grpcurl -plaintext -d "$REQUEST" localhost:8081 trade.Trade/AddLiquidityV1

# 如果没有grpcurl，也可以尝试使用curl访问gateway
echo -e "\n使用curl通过gateway测试..."
curl -v -X POST http://localhost:8083/v1/trade/add_liquidity_v1 \
  -H "Content-Type: application/json" \
  -d "$REQUEST"

echo -e "\n测试完成" 