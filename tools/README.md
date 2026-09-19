# Kafka Message Monitoring Tools

This directory contains tools to help monitor and debug Kafka message consumption.

## Tools Available

### 1. Enhanced Consumer Logging
The `TradeConsumer` in `dataflow/internal/mqs/consumers/trade_consumer.go` now includes enhanced logging with:
- 🔔 Message reception notifications with topic, partition, offset details
- ✅ Successful unmarshaling confirmations  
- ❌ Clear error messages for failures
- 📊 Processing time measurements
- 📈 Prometheus metrics via `dataflow_kafka_consumer_fetch` counter

### 2. Standalone Kafka Monitor (`kafka_monitor.go`)
A dedicated monitoring tool that can run independently to verify message flow.

## How to Check Message Reception

### Method 1: Check Application Logs
Monitor your dataflow service logs for these patterns:
```bash
# Look for message reception
grep "🔔 Kafka message received" /path/to/your/logs

# Check processing success
grep "✅ Successfully unmarshaled" /path/to/your/logs

# Monitor errors
grep "❌" /path/to/your/logs

# Check processing timing
grep "📊 tradeConsumer sendTime" /path/to/your/logs
```

### Method 2: Monitor Prometheus Metrics
Access your metrics endpoint (usually `/metrics`) and look for:
```
dataflow_kafka_consumer_fetch
```
This counter increments for each successfully processed message.

### Method 3: Use the Standalone Monitor
```bash
# Set environment variables for Kafka auth
export KAFKA_USERNAME="your_username"
export KAFKA_PASSWORD="your_password"

# Run the monitor
cd tools
go run kafka_monitor.go "broker:9093" "your-topic" "monitor-group"
```

The monitor will show:
- Message count and rate
- Topic, partition, offset details
- Message size and timestamp
- JSON validation
- Message preview

### Method 4: Database Verification
Check if processed messages result in database records:
```sql
-- Check recent kline data
SELECT COUNT(*), MAX(updated_at) 
FROM klines 
WHERE updated_at > NOW() - INTERVAL 5 MINUTE;

-- Check trade processing
SELECT chain_id, COUNT(*) as trade_count, MAX(create_time) as latest
FROM trades 
WHERE create_time > NOW() - INTERVAL 5 MINUTE
GROUP BY chain_id;
```

## Common Issues and Debugging

### No Messages Received
1. Check Kafka broker connectivity
2. Verify topic exists and has messages
3. Check consumer group configuration
4. Verify SASL credentials

### Messages Received but Not Processed
1. Check for JSON unmarshaling errors in logs
2. Verify message format matches expected structure
3. Check for nil pointer errors
4. Monitor worker pool capacity

### Performance Issues
1. Monitor processing time in logs
2. Check worker pool utilization
3. Monitor Redis and database performance
4. Check for blocking operations

## Configuration Files to Check
- `dataflow/etc/dataflow.yaml` - Consumer configuration
- Consumer group settings
- Kafka broker addresses
- SASL credentials 