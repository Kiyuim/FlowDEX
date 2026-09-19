#!/bin/bash

# Consumer Service Restart Script
# This script will restart the consumer service to reconnect to Kafka

echo "=========================================="
echo "Consumer Service Restart"
echo "=========================================="

# Find consumer processes
CONSUMER_PIDS=$(ps aux | grep -E "consumer\.go|consumer$" | grep -v grep | awk '{print $2}')

if [ -z "$CONSUMER_PIDS" ]; then
    echo "⚠️  No consumer process found"
    echo "Starting consumer service..."
    cd /home/ubuntu/dex_full/fun_dex_v2/consumer
    nohup go run consumer.go -f etc/consumer.yaml > /tmp/consumer.log 2>&1 &
    echo "✅ Consumer service started"
    echo "📋 Logs: tail -f /tmp/consumer.log"
else
    echo "📋 Found consumer processes: $CONSUMER_PIDS"
    echo "🛑 Stopping consumer service..."
    
    # Kill all consumer processes
    for pid in $CONSUMER_PIDS; do
        echo "   Killing PID: $pid"
        kill -TERM $pid 2>/dev/null || kill -9 $pid 2>/dev/null
    done
    
    # Wait for processes to stop
    sleep 2
    
    # Verify processes are stopped
    REMAINING=$(ps aux | grep -E "consumer\.go|consumer$" | grep -v grep | awk '{print $2}')
    if [ -n "$REMAINING" ]; then
        echo "⚠️  Force killing remaining processes..."
        for pid in $REMAINING; do
            kill -9 $pid 2>/dev/null
        done
    fi
    
    echo "✅ Consumer service stopped"
    echo ""
    echo "🚀 Starting consumer service..."
    
    cd /home/ubuntu/dex_full/fun_dex_v2/consumer
    nohup go run consumer.go -f etc/consumer.yaml > /tmp/consumer.log 2>&1 &
    
    sleep 2
    
    # Check if started successfully
    NEW_PID=$(ps aux | grep -E "consumer\.go" | grep -v grep | awk '{print $2}' | head -1)
    if [ -n "$NEW_PID" ]; then
        echo "✅ Consumer service started successfully (PID: $NEW_PID)"
        echo ""
        echo "📋 Checking Kafka initialization..."
        sleep 3
        echo ""
        echo "📊 Recent logs:"
        tail -20 /tmp/consumer.log | grep -i "kafka\|Initializing\|producer\|error" || echo "   (No Kafka-related logs yet)"
        echo ""
        echo "📋 Full logs: tail -f /tmp/consumer.log"
    else
        echo "❌ Failed to start consumer service"
        echo "📋 Check logs: cat /tmp/consumer.log"
    fi
fi

echo ""
echo "=========================================="
echo "Next Steps:"
echo "1. Monitor logs: tail -f /tmp/consumer.log"
echo "2. Look for: '✅ Kafka producer initialized successfully'"
echo "3. Verify no errors: '_kafkaClient is Nil'"
echo "=========================================="
