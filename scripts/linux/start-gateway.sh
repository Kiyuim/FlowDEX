#!/bin/bash

# Fun DEX Gateway Service Start Script

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
SERVICE_NAME="Gateway"
SERVICE_DIR="gateway"
SERVICE_FILE="gateway.go"
SERVICE_PORT="8083"

# Directories
PID_DIR="./pids"
LOG_DIR="./logs"

# Function to check if service is running
check_service() {
    if lsof -i :$SERVICE_PORT >/dev/null 2>&1; then
        print_warning "$SERVICE_NAME is already running on port $SERVICE_PORT"
        return 0
    else
        return 1
    fi
}

# Function to start service
start_service() {
    print_status "Starting $SERVICE_NAME service on port $SERVICE_PORT..."
    
    # Create directories if they don't exist
    mkdir -p "$PID_DIR" "$LOG_DIR"
    
    # Change to service directory
    cd "$SERVICE_DIR"
    
    # Start the service in background
    nohup go run "$SERVICE_FILE" > "../${LOG_DIR}/gateway.log" 2>&1 &
    
    # Get the PID
    local pid=$!
    
    # Save PID to file
    echo "$pid" > "../${PID_DIR}/gateway.pid"
    
    # Wait a moment for service to start
    sleep 2
    
    # Check if service started successfully
    if kill -0 "$pid" 2>/dev/null && lsof -i :$SERVICE_PORT >/dev/null 2>&1; then
        print_status "$SERVICE_NAME started successfully with PID $pid"
        return 0
    else
        print_error "$SERVICE_NAME failed to start"
        return 1
    fi
}

# Main execution
main() {
    print_header "Starting $SERVICE_NAME Service"
    
    # Ensure Go is in PATH
    export PATH="/usr/local/go/bin:$PATH"
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check if service directory exists
    if [ ! -d "$SERVICE_DIR" ]; then
        print_error "Service directory not found: $SERVICE_DIR"
        exit 1
    fi
    
    # Check if service file exists
    if [ ! -f "$SERVICE_DIR/$SERVICE_FILE" ]; then
        print_error "Service file not found: $SERVICE_DIR/$SERVICE_FILE"
        exit 1
    fi
    
    # Check if service is already running
    if check_service; then
        print_status "$SERVICE_NAME is already running. Use stop script to stop it first."
        exit 0
    fi
    
    # Start the service
    if start_service; then
        print_status "✅ $SERVICE_NAME service started successfully!"
        print_status "You can check the logs at: ${LOG_DIR}/gateway.log"
        print_status "You can check the status with: ./scripts/linux/status-services.sh"
    else
        print_error "❌ Failed to start $SERVICE_NAME service"
        exit 1
    fi
}

# Run main function
main "$@" 