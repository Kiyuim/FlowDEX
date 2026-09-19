#!/bin/bash

# Fun DEX Backend Services Stop Script
# This script stops all running backend services

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

# Function to stop a service by PID file
stop_service_by_pid() {
    local service_name=$1
    local pid_file="logs/${service_name,,}.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 $pid 2>/dev/null; then
            print_status "Stopping $service_name (PID: $pid)..."
            kill -TERM $pid
            
            # Wait for graceful shutdown
            local count=0
            while kill -0 $pid 2>/dev/null && [ $count -lt 10 ]; do
                sleep 1
                count=$((count + 1))
            done
            
            # Force kill if still running
            if kill -0 $pid 2>/dev/null; then
                print_warning "Force killing $service_name (PID: $pid)..."
                kill -KILL $pid
            fi
            
            print_status "$service_name stopped"
        else
            print_warning "$service_name PID file exists but process is not running"
        fi
        
        # Remove PID file
        rm -f "$pid_file"
    else
        print_warning "No PID file found for $service_name"
    fi
}

# Function to stop services by port
stop_service_by_port() {
    local port=$1
    local service_name=$2
    
    local pid=$(lsof -ti :$port 2>/dev/null || true)
    if [ -n "$pid" ]; then
        print_status "Stopping $service_name on port $port (PID: $pid)..."
        kill -TERM $pid 2>/dev/null || true
        
        # Wait for graceful shutdown
        local count=0
        while kill -0 $pid 2>/dev/null && [ $count -lt 10 ]; do
            sleep 1
            count=$((count + 1))
        done
        
        # Force kill if still running
        if kill -0 $pid 2>/dev/null; then
            print_warning "Force killing $service_name on port $port (PID: $pid)..."
            kill -KILL $pid 2>/dev/null || true
        fi
        
        print_status "$service_name stopped"
    else
        print_warning "$service_name is not running on port $port"
    fi
}

# Main execution
main() {
    print_header "Fun DEX Backend Services Stop"
    print_status "Stopping all backend services..."
    
    # Stop services by PID files first
    stop_service_by_pid "Consumer"
    stop_service_by_pid "Market"
    stop_service_by_pid "Trade"
    stop_service_by_pid "Gateway"
    stop_service_by_pid "WebSocket"
    
    # Stop services by port as backup
    stop_service_by_port "8080" "Consumer"
    stop_service_by_port "8081" "Market"
    stop_service_by_port "8082" "Trade"
    stop_service_by_port "8083" "Gateway"
    stop_service_by_port "8084" "WebSocket"
    
    print_header "All services stopped successfully!"
    print_status "Service logs are still available in the logs/ directory"
}

# Run main function
main 