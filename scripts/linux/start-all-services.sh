#!/bin/bash

# Fun DEX Backend Services Startup Script
# This script starts all backend services in the correct order

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}[HEADER]${NC} $1"
}

# Function to check if a service is running
check_service() {
    local port=$1
    local service_name=$2
    
    if lsof -i :$port >/dev/null 2>&1; then
        print_warning "$service_name is already running on port $port"
        return 0
    else
        return 1
    fi
}

# Function to start a service
start_service() {
    local service_dir=$1
    local service_file=$2
    local service_name=$3
    local port=$4
    
    print_header "Starting $service_name..."
    
    # Check if service is already running
    if check_service $port "$service_name"; then
        return 0
    fi
    
    # Check if directory exists
    if [ ! -d "$service_dir" ]; then
        print_error "Directory $service_dir does not exist!"
        return 1
    fi
    
    # Check if service file exists
    if [ ! -f "$service_dir/$service_file" ]; then
        print_error "Service file $service_dir/$service_file does not exist!"
        return 1
    fi
    
    # Start service in background
    cd "$service_dir"
    nohup go run "$service_file" > "../logs/${service_name,,}.log" 2>&1 &
    local pid=$!
    
    # Save PID
    echo $pid > "../logs/${service_name,,}.pid"
    
    print_status "$service_name started with PID $pid"
    
    # Wait a moment for service to start
    sleep 3
    
    # Check if service is running
    if kill -0 $pid 2>/dev/null; then
        print_status "$service_name is running successfully on port $port"
    else
        print_error "$service_name failed to start"
        return 1
    fi
    
    cd - > /dev/null
}

# Main execution
main() {
    print_header "Fun DEX Backend Services Startup"
    print_status "Starting all backend services..."
    
    # Create logs directory
    mkdir -p logs
    
    # Start services in dependency order
    # Consumer service first (processes blockchain data)
    start_service "consumer" "consumer.go" "Consumer" "8082"
    
    # Market service (provides market data)
    start_service "market" "market.go" "Market" "8080"
    
    # Trade service (handles trading operations)
    start_service "trade" "trade.go" "Trade" "8081"
    
    # Gateway service (API gateway - depends on market and trade)
    start_service "gateway" "gateway.go" "Gateway" "8083"
    
    # WebSocket service (real-time communication)
    start_service "websocket" "token_websocket_server.go" "WebSocket" "8086"
    
    print_header "All services started successfully!"
    print_status "Service logs are available in the logs/ directory"
    print_status "Use 'scripts/linux/stop-all-services.sh' to stop all services"
    print_status "Use 'scripts/linux/status-services.sh' to check service status"
    
    # Show service summary
    echo -e ""
    print_header "Service Summary:"
    echo -e "  ✓ Consumer Service  - Port 8082 - Blockchain data processing"
    echo -e "  ✓ Market Service    - Port 8080 - Market data and token info"
    echo -e "  ✓ Trade Service     - Port 8081 - Trading operations"
    echo -e "  ✓ Gateway Service   - Port 8083 - API gateway and routing"
    echo -e "  ✓ WebSocket Service - Port 8086 - Real-time communication"
    echo -e ""
    print_status "All services are now running!"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

# Check if running from project root
if [ ! -d "consumer" ] || [ ! -d "market" ] || [ ! -d "trade" ] || [ ! -d "gateway" ] || [ ! -d "websocket" ]; then
    print_error "Please run this script from the project root directory"
    exit 1
fi

# Run main function
main 