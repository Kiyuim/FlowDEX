#!/bin/bash

# Fun DEX Services Status Script
# This script checks the status of all Fun DEX backend services

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
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

print_service() {
    echo -e "${CYAN}[SERVICE]${NC} $1"
}

print_running() {
    echo -e "${GREEN}[RUNNING]${NC} $1"
}

print_stopped() {
    echo -e "${RED}[STOPPED]${NC} $1"
}

# Service configuration
declare -A SERVICES=(
    ["consumer"]="8080"
    ["market"]="8081"
    ["trade"]="8082"
    ["gateway"]="8083"
    ["websocket"]="8084"
)

# PID file directory
PID_DIR="./pids"

# Function to check if a port is in use
check_port() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        lsof -i :$port >/dev/null 2>&1
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tlnp | grep ":$port " >/dev/null 2>&1
    else
        return 1
    fi
}

# Function to get PID from port
get_pid_from_port() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        lsof -t -i :$port 2>/dev/null | head -1
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tlnp | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | head -1
    else
        echo ""
    fi
}

# Function to get process info
get_process_info() {
    local pid=$1
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        ps -p "$pid" -o pid,ppid,cmd --no-headers 2>/dev/null
    else
        echo ""
    fi
}

# Function to get memory usage
get_memory_usage() {
    local pid=$1
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        ps -p "$pid" -o rss --no-headers 2>/dev/null | awk '{print $1/1024 " MB"}'
    else
        echo "N/A"
    fi
}

# Function to get CPU usage
get_cpu_usage() {
    local pid=$1
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        ps -p "$pid" -o pcpu --no-headers 2>/dev/null | awk '{print $1"%"}'
    else
        echo "N/A"
    fi
}

# Function to check service status
check_service_status() {
    local service=$1
    local port=$2
    local pid_file="$PID_DIR/${service}.pid"
    
    print_service "Checking $service service (Port: $port)"
    
    # Check if port is in use
    if check_port $port; then
        local pid=$(get_pid_from_port $port)
        local process_info=$(get_process_info $pid)
        local memory=$(get_memory_usage $pid)
        local cpu=$(get_cpu_usage $pid)
        
        if [ -n "$process_info" ]; then
            print_running "$service is running"
            echo "  🔹 PID: $pid"
            echo "  🔹 Port: $port"
            echo "  🔹 Memory: $memory"
            echo "  🔹 CPU: $cpu"
            echo "  🔹 Process: $process_info"
            
            # Check if PID file exists and matches
            if [ -f "$pid_file" ]; then
                local stored_pid=$(cat "$pid_file" 2>/dev/null)
                if [ "$stored_pid" = "$pid" ]; then
                    echo "  ✅ PID file matches running process"
                else
                    print_warning "PID file mismatch (stored: $stored_pid, actual: $pid)"
                fi
            else
                print_warning "PID file not found: $pid_file"
            fi
        else
            print_error "$service port is occupied but process not found"
        fi
    else
        print_stopped "$service is not running"
        echo "  🔹 Port: $port (not in use)"
        
        # Check if PID file exists
        if [ -f "$pid_file" ]; then
            local stored_pid=$(cat "$pid_file" 2>/dev/null)
            if [ -n "$stored_pid" ] && kill -0 "$stored_pid" 2>/dev/null; then
                print_warning "PID file exists but process may be running on different port"
                echo "  🔹 Stored PID: $stored_pid"
            else
                print_warning "Stale PID file exists: $pid_file"
            fi
        fi
    fi
    
    # Show recent logs if available
    local log_file="logs/${service}.log"
    if [ -f "$log_file" ]; then
        echo "  📋 Recent logs (last 3 lines):"
        tail -n 3 "$log_file" 2>/dev/null | sed 's/^/    /'
    else
        echo "  📋 No log file found: $log_file"
    fi
    
    echo ""
}

# Function to check MySQL connection
check_mysql_connection() {
    print_header "MySQL Database Status"
    
    local mysql_cmd="mysql -hlocalhost -P3306 -uroot -pweb3ite.fun fun_dexs"
    
    if systemctl is-active --quiet mysql 2>/dev/null; then
        print_running "MySQL service is running"
        
        # Test connection
        if $mysql_cmd -e "SELECT 1;" >/dev/null 2>&1; then
            print_running "Database connection successful"
            
            # Show table count
            local table_count=$($mysql_cmd -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='fun_dexs';" 2>/dev/null | tail -1)
            echo "  🔹 Database: fun_dexs"
            echo "  🔹 Tables: $table_count"
            echo "  🔹 Host: localhost:3306"
            echo "  🔹 User: root"
        else
            print_error "Database connection failed"
        fi
    else
        print_stopped "MySQL service is not running"
    fi
    
    echo ""
}

# Function to show system resources
show_system_resources() {
    print_header "System Resources"
    
    # Memory usage
    local memory_info=$(free -h | grep "Mem:")
    echo "  💾 Memory: $(echo $memory_info | awk '{print $3"/"$2" ("$3/$2*100"%)"}')"
    
    # CPU load
    local load_avg=$(uptime | awk -F'load average:' '{print $2}')
    echo "  🖥️  Load Average:$load_avg"
    
    # Disk usage
    local disk_usage=$(df -h / | tail -1)
    echo "  💽 Disk Usage: $(echo $disk_usage | awk '{print $3"/"$2" ("$5")"}')"
    
    echo ""
}

# Function to show service summary
show_service_summary() {
    print_header "Service Summary"
    
    local running_count=0
    local total_count=${#SERVICES[@]}
    
    for service in "${!SERVICES[@]}"; do
        local port=${SERVICES[$service]}
        if check_port $port; then
            ((running_count++))
            echo "  ✅ $service (Port: $port)"
        else
            echo "  ❌ $service (Port: $port)"
        fi
    done
    
    echo ""
    echo "  📊 Services Running: $running_count/$total_count"
    
    if [ $running_count -eq $total_count ]; then
        print_running "All services are running! 🎉"
    elif [ $running_count -gt 0 ]; then
        print_warning "Some services are down"
    else
        print_error "No services are running"
    fi
    
    echo ""
}

# Main execution
main() {
    print_header "Fun DEX Services Status Check"
    echo "Timestamp: $(date)"
    echo ""
    
    # Check MySQL first
    check_mysql_connection
    
    # Check all services
    for service in "${!SERVICES[@]}"; do
        check_service_status "$service" "${SERVICES[$service]}"
    done
    
    # Show system resources
    show_system_resources
    
    # Show summary
    show_service_summary
    
    print_header "Status check completed!"
}

# Create PID directory if it doesn't exist
mkdir -p "$PID_DIR"

# Run with options
case "${1:-}" in
    --summary|-s)
        show_service_summary
        ;;
    --mysql|-m)
        check_mysql_connection
        ;;
    --resources|-r)
        show_system_resources
        ;;
    --help|-h)
        print_header "Fun DEX Service Status Script"
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --summary, -s      Show service summary only"
        echo "  --mysql, -m        Check MySQL status only"
        echo "  --resources, -r    Show system resources only"
        echo "  --help, -h         Show this help message"
        echo ""
        echo "No options: Show full status report"
        ;;
    *)
        main
        ;;
esac 