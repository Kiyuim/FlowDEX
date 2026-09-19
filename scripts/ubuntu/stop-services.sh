#!/bin/bash

# Fun DEX 微服务停止脚本

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="/usr/code/alan_fun_dex_v2"
LOG_DIR="$PROJECT_ROOT/logs"

# 函数：打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 函数：停止服务
stop_service() {
    local service_name=$1
    local description=$2
    
    print_info "停止 $description ($service_name)..."
    
    # 尝试从 PID 文件停止
    local pid_file="$LOG_DIR/$service_name.pid"
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            sleep 2
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid"
                print_warning "$description 强制停止 (PID: $pid)"
            else
                print_success "$description 已停止 (PID: $pid)"
            fi
        else
            print_warning "$description PID 文件存在但进程不存在"
        fi
        rm -f "$pid_file"
    else
        # 尝试通过进程名停止
        pkill -f "$service_name"
        print_success "$description 已停止"
    fi
}

# 函数：停止所有相关进程
force_stop_all() {
    print_warning "强制停止所有相关进程..."
    
    # 获取所有相关进程的PID
    local pids=()
    
    # 查找Go服务进程
    for service in consumer market trade gateway token_websocket; do
        local pid=$(pgrep -f "$service" | head -1)
        if [ -n "$pid" ]; then
            pids+=($pid)
            print_info "找到 $service 进程: PID $pid"
        fi
    done
    
    # 查找前端服务进程
    local frontend_pid=$(pgrep -f "serve.*3001" | head -1)
    if [ -n "$frontend_pid" ]; then
        pids+=($frontend_pid)
        print_info "找到前端服务进程: PID $frontend_pid"
    fi
    
    # 查找基于端口的进程
    for port in 8080 8081 8082 8083 8086 3001; do
        local port_pid=$(ss -tlnp | grep ":$port " | awk '{print $6}' | sed 's/.*pid=\([0-9]*\).*/\1/' | head -1)
        if [ -n "$port_pid" ] && [[ "$port_pid" =~ ^[0-9]+$ ]]; then
            pids+=($port_pid)
            print_info "找到端口 $port 进程: PID $port_pid"
        fi
    done
    
    # 去重PID列表
    local unique_pids=($(printf "%s\n" "${pids[@]}" | sort -u))
    
    if [ ${#unique_pids[@]} -eq 0 ]; then
        print_success "没有找到需要停止的进程"
        return
    fi
    
    # 首先尝试优雅停止
    print_info "尝试优雅停止进程..."
    for pid in "${unique_pids[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            print_info "发送TERM信号到进程 $pid"
        fi
    done
    
    # 等待进程停止
    sleep 3
    
    # 强制停止仍在运行的进程
    print_info "强制停止剩余进程..."
    for pid in "${unique_pids[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid"
            print_warning "强制停止进程 $pid"
        fi
    done
    
    # 清理所有 PID 文件
    rm -f "$LOG_DIR"/*.pid
    
    print_success "所有进程已强制停止"
}

# 主停止流程
main() {
    print_info "================================="
    print_info "Fun DEX 微服务停止脚本"
    print_info "================================="

    if [ "$1" = "-f" ] || [ "$1" = "--force" ]; then
        force_stop_all
        return
    fi

    # 按相反顺序停止服务（与启动顺序相反）
    stop_service "frontend" "前端服务"
    stop_service "websocket" "WebSocket 服务"
    stop_service "gateway" "网关服务" 
    stop_service "trade" "交易服务"
    stop_service "market" "市场数据服务"
    stop_service "consumer" "消费者服务"

    print_info "================================="
    print_success "所有服务已停止！"
    print_info "================================="
    
    # 检查是否还有相关进程
    local remaining_processes=0
    for service in consumer market trade gateway token_websocket; do
        if pgrep -f "$service" > /dev/null; then
            remaining_processes=1
            break
        fi
    done
    
    if pgrep -f "serve.*3001" > /dev/null; then
        remaining_processes=1
    fi
    
    if [ $remaining_processes -eq 1 ]; then
        print_warning "仍有部分进程在运行，使用 $0 -f 强制停止"
    fi
}

# 帮助信息
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help    显示此帮助信息"
    echo "  -f, --force   强制停止所有相关进程"
    echo ""
    echo "示例:"
    echo "  $0            正常停止所有服务"
    echo "  $0 -f         强制停止所有服务"
    echo ""
    exit 0
fi

# 执行主函数
main "$@"
