#!/bin/bash

# Simple MySQL import script
DB_CONN="mysql -hlocalhost -P3306 -uroot -pweb3ite.fun fun_dexs"

echo "Starting database import..."
echo "Found $(find docker/fun_dexs -name "*.sql" | wc -l) SQL files"

# Drop all tables first
echo "Dropping all existing tables..."
$DB_CONN -e "SET FOREIGN_KEY_CHECKS = 0; 
DROP TABLE IF EXISTS block;
DROP TABLE IF EXISTS clmm_pool_info_v1;
DROP TABLE IF EXISTS clmm_pool_info_v2;
DROP TABLE IF EXISTS cpmm_pool_info;
DROP TABLE IF EXISTS pair;
DROP TABLE IF EXISTS pump_amm_info;
DROP TABLE IF EXISTS raydium_pool;
DROP TABLE IF EXISTS sol_account;
DROP TABLE IF EXISTS sol_token_account;
DROP TABLE IF EXISTS token;
DROP TABLE IF EXISTS trade;
DROP TABLE IF EXISTS trade_kline_12h_06;
DROP TABLE IF EXISTS trade_kline_15m_06;
DROP TABLE IF EXISTS trade_kline_1d_06;
DROP TABLE IF EXISTS trade_kline_1h_06;
DROP TABLE IF EXISTS trade_kline_1m_06;
DROP TABLE IF EXISTS trade_kline_4h_06;
DROP TABLE IF EXISTS trade_kline_5m_06;
DROP TABLE IF EXISTS trade_order;
DROP TABLE IF EXISTS trade_order_log;
SET FOREIGN_KEY_CHECKS = 1;" 2>/dev/null

# Import all files
count=0
for file in docker/fun_dexs/*.sql; do
    filename=$(basename "$file")
    echo "[$((++count))] Importing: $filename"
    if $DB_CONN < "$file" 2>/dev/null; then
        echo "✅ Success: $filename"
    else
        echo "❌ Failed: $filename"
        $DB_CONN < "$file"
    fi
done

echo "Import complete!"
echo "Tables in database:"
$DB_CONN -e "SHOW TABLES;" 