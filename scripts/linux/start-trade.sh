#!/bin/bash

# Fun DEX Trade Service Start Script

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

# Service configuration
SERVICE_NAME="Trade"
SERVICE_DIR="trade"
SERVICE_FILE="trade.go"
SERVICE_PORT="8082"

# Function to check if service is running
check_service() {
    if lsof -i :$SERVICE_PORT >/dev/null 2>&1; then
        print_warning "$SERVICE_NAME is already running on port $SERVICE_PORT"
        return 0
    else
        return 1
    fi
}

# Main execution
main() {
    print_header "Starting $SERVICE_NAME Service"
    
    # Check if service is already running
    if check_service; then
        exit 0
    fi
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check if directory exists
    if [ ! -d "$SERVICE_DIR" ]; then
        print_error "Directory $SERVICE_DIR does not exist!"
        exit 1
    fi
    
    # Check if service file exists
    if [ ! -f "$SERVICE_DIR/$SERVICE_FILE" ]; then
        print_error "Service file $SERVICE_DIR/$SERVICE_FILE does not exist!"
        exit 1
    fi
    
    # Create logs directory
    mkdir -p logs
    
    # Start service
    print_status "Starting $SERVICE_NAME service..."
    cd "$SERVICE_DIR"
    nohup go run "$SERVICE_FILE" > "../logs/${SERVICE_NAME,,}.log" 2>&1 &
    local pid=$!
    
    # Save PID
    echo $pid > "../logs/${SERVICE_NAME,,}.pid"
    
    print_status "$SERVICE_NAME started with PID $pid"
    
    # Wait a moment for service to start
    sleep 2
    
    # Check if service is running
    if kill -0 $pid 2>/dev/null; then
        print_status "$SERVICE_NAME is running successfully on port $SERVICE_PORT"
    else
        print_error "$SERVICE_NAME failed to start"
        exit 1
    fi
    
    cd - > /dev/null
    print_header "$SERVICE_NAME service started successfully!"
}

# Run main function
main 