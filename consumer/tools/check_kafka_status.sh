#!/bin/bash

# Kafka Broker Status Check Script
# Usage: ./check_kafka_status.sh

BROKER="115.159.107.189:9093"
USERNAME="alikafka_post-cn-zp54bjj8x004"
PASSWORD="htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1"
TOPIC="web3fun"

echo "=========================================="
echo "Kafka Broker Status Check"
echo "=========================================="
echo "Broker: $BROKER"
echo "Username: $USERNAME"
echo "Topic: $TOPIC"
echo ""

# Method 1: Using Go tool (if available)
if command -v go &> /dev/null; then
    echo "📊 Method 1: Using Go Kafka Check Tool"
    echo "----------------------------------------"
    cd "$(dirname "$0")" || exit
    if [ -f "kafka_check.go" ]; then
        go run kafka_check.go "$BROKER" "$USERNAME" "$PASSWORD"
        echo ""
    fi
fi

# Method 2: Using kafka-broker-api-versions (if Kafka tools installed)
if command -v kafka-broker-api-versions &> /dev/null; then
    echo "📊 Method 2: Using Kafka CLI Tools"
    echo "----------------------------------------"
    
    # Create temporary config file
    CONFIG_FILE=$(mktemp)
    cat > "$CONFIG_FILE" << EOF
security.protocol=SASL_SSL
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="$USERNAME" password="$PASSWORD";
EOF
    
    echo "1. Broker API Versions:"
    kafka-broker-api-versions --bootstrap-server "$BROKER" --command-config "$CONFIG_FILE" 2>&1 | head -20
    echo ""
    
    echo "2. List Topics:"
    kafka-topics --bootstrap-server "$BROKER" --list --command-config "$CONFIG_FILE" 2>&1
    echo ""
    
    echo "3. Describe Topic ($TOPIC):"
    kafka-topics --bootstrap-server "$BROKER" --describe --topic "$TOPIC" --command-config "$CONFIG_FILE" 2>&1
    echo ""
    
    echo "4. Consumer Groups:"
    kafka-consumer-groups --bootstrap-server "$BROKER" --list --command-config "$CONFIG_FILE" 2>&1
    echo ""
    
    rm -f "$CONFIG_FILE"
fi

# Method 3: Using kcat/kafkacat (if installed)
if command -v kcat &> /dev/null || command -v kafkacat &> /dev/null; then
    echo "📊 Method 3: Using kcat/kafkacat"
    echo "----------------------------------------"
    KCAT_CMD=$(command -v kcat || command -v kafkacat)
    
    echo "1. Broker Metadata:"
    $KCAT_CMD -b "$BROKER" \
        -X security.protocol=SASL_SSL \
        -X sasl.mechanism=PLAIN \
        -X sasl.username="$USERNAME" \
        -X sasl.password="$PASSWORD" \
        -L 2>&1
    echo ""
    
    echo "2. List Topics:"
    $KCAT_CMD -b "$BROKER" \
        -X security.protocol=SASL_SSL \
        -X sasl.mechanism=PLAIN \
        -X sasl.username="$USERNAME" \
        -X sasl.password="$PASSWORD" \
        -L -J | grep -o '"name":"[^"]*"' | cut -d'"' -f4
    echo ""
fi

# Method 4: Simple network check
echo "📊 Method 4: Network Connectivity"
echo "----------------------------------------"
if timeout 3 bash -c "cat < /dev/null > /dev/tcp/${BROKER%:*}/${BROKER#*:}" 2>/dev/null; then
    echo "✅ Port ${BROKER#*:} is open and accessible"
else
    echo "❌ Cannot connect to port ${BROKER#*:}"
fi
echo ""

echo "=========================================="
echo "Status Check Complete"
echo "=========================================="
