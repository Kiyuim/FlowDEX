#!/bin/bash

echo "测试TestRpc接口..."
echo "通过网关HTTP调用TestRpc方法"

# 构造请求数据
REQUEST='{
  "message": "测试消息",
  "value": 42
}'

# 使用curl发送请求
echo -e "\n通过curl调用测试接口:"
curl -v -X POST http://localhost:8083/v1/trade/test_rpc \
  -H "Content-Type: application/json" \
  -d "$REQUEST"

echo -e "\n测试完成" 