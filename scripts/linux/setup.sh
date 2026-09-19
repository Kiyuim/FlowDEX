#!/bin/bash

# Fun DEX Scripts Setup Script
# This script sets up the environment and makes all scripts executable

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

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Function to make scripts executable
make_executable() {
    print_header "Making scripts executable..."
    
    local scripts=(
        "start-all-services.sh"
        "stop-all-services.sh"
        "restart-services.sh"
        "status-services.sh"
        "start-consumer.sh"
        "start-market.sh"
        "start-trade.sh"
        "start-gateway.sh"
        "start-websocket.sh"
        "manage-services.sh"
        "setup.sh"
    )
    
    for script in "${scripts[@]}"; do
        local script_path="$SCRIPT_DIR/$script"
        if [ -f "$script_path" ]; then
            chmod +x "$script_path"
            print_status "Made $script executable"
        else
            print_warning "$script not found"
        fi
    done
}

# Function to check prerequisites
check_prerequisites() {
    print_header "Checking prerequisites..."
    
    # Check if Go is installed
    if command -v go &> /dev/null; then
        local go_version=$(go version | awk '{print $3}')
        print_status "Go is installed: $go_version"
    else
        print_error "Go is not installed or not in PATH"
        print_error "Please install Go 1.24.2 or later"
        exit 1
    fi
    
    # Check if lsof is installed
    if command -v lsof &> /dev/null; then
        print_status "lsof is available"
    else
        print_warning "lsof is not installed. Some port checking features may not work."
        print_warning "Install with: sudo apt-get install lsof"
    fi
    
    # Check if netstat is installed
    if command -v netstat &> /dev/null; then
        print_status "netstat is available"
    else
        print_warning "netstat is not installed. Some network features may not work."
        print_warning "Install with: sudo apt-get install net-tools"
    fi
}

# Function to create necessary directories
create_directories() {
    print_header "Creating necessary directories..."
    
    # Create logs directory in project root
    local project_root=$(dirname "$(dirname "$SCRIPT_DIR")")
    local logs_dir="$project_root/logs"
    
    if [ ! -d "$logs_dir" ]; then
        mkdir -p "$logs_dir"
        print_status "Created logs directory: $logs_dir"
    else
        print_status "Logs directory already exists: $logs_dir"
    fi
}

# Function to check project structure
check_project_structure() {
    print_header "Checking project structure..."
    
    local project_root=$(dirname "$(dirname "$SCRIPT_DIR")")
    cd "$project_root"
    
    local required_dirs=("consumer" "market" "trade" "gateway" "websocket")
    local missing_dirs=()
    
    for dir in "${required_dirs[@]}"; do
        if [ -d "$dir" ]; then
            print_status "Found $dir directory"
        else
            print_warning "$dir directory not found"
            missing_dirs+=("$dir")
        fi
    done
    
    if [ ${#missing_dirs[@]} -gt 0 ]; then
        print_error "Missing required directories: ${missing_dirs[*]}"
        print_error "Please ensure you're running this from the correct project directory"
        exit 1
    fi
}

# Function to show usage instructions
show_usage() {
    print_header "Usage Instructions"
    echo -e ""
    print_status "The following scripts are now available:"
    echo -e ""
    echo -e "  ${GREEN}Main Scripts:${NC}"
    echo -e "    ./scripts/linux/start-all-services.sh    - Start all backend services"
    echo -e "    ./scripts/linux/stop-all-services.sh     - Stop all backend services"
    echo -e "    ./scripts/linux/restart-services.sh      - Restart all backend services"
    echo -e "    ./scripts/linux/status-services.sh       - Check services status"
    echo -e "    ./scripts/linux/manage-services.sh       - Interactive management menu"
    echo -e ""
    echo -e "  ${GREEN}Individual Service Scripts:${NC}"
    echo -e "    ./scripts/linux/start-consumer.sh        - Start Consumer service"
    echo -e "    ./scripts/linux/start-market.sh          - Start Market service"
    echo -e "    ./scripts/linux/start-trade.sh           - Start Trade service"
    echo -e "    ./scripts/linux/start-gateway.sh         - Start Gateway service"
    echo -e "    ./scripts/linux/start-websocket.sh       - Start WebSocket service"
    echo -e ""
    echo -e "  ${GREEN}Quick Start:${NC}"
    echo -e "    cd $(dirname "$(dirname "$SCRIPT_DIR")")"
    echo -e "    ./scripts/linux/start-all-services.sh"
    echo -e ""
    echo -e "  ${GREEN}Interactive Management:${NC}"
    echo -e "    ./scripts/linux/manage-services.sh"
    echo -e ""
    print_status "All scripts must be run from the project root directory"
}

# Main execution
main() {
    print_header "Fun DEX Scripts Setup"
    print_status "Setting up Fun DEX backend service scripts..."
    
    check_prerequisites
    check_project_structure
    create_directories
    make_executable
    
    print_header "Setup completed successfully!"
    show_usage
}

# Run main function
main 