#!/bin/bash

# Fun DEX 微服务启动脚本
# 按照正确的依赖顺序启动所有服务

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="/usr/code/alan_fun_dex_v2"

# 日志目录
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"

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

# 函数：检查端口是否被占用
check_port() {
    local port=$1
    local service=$2
    if ss -tlnp | grep -q ":$port "; then
        print_warning "端口 $port 已被占用 ($service)"
        return 1
    fi
    return 0
}

# 函数：启动服务
start_service() {
    local service_name=$1
    local service_dir=$2
    local service_file=$3
    local port=$4
    local description=$5

    print_info "启动 $description ($service_name)..."
    
    # 检查端口
    if ! check_port "$port" "$service_name"; then
        print_error "$service_name 端口冲突，跳过启动"
        return 1
    fi

    # 进入服务目录
    cd "$PROJECT_ROOT/$service_dir" || {
        print_error "无法进入目录: $PROJECT_ROOT/$service_dir"
        return 1
    }

    # 启动服务（后台运行，输出重定向到日志）
    nohup go run "$service_file" > "$LOG_DIR/$service_name.log" 2>&1 &
    local pid=$!
    
    # 等待服务启动
    sleep 3
    
    # 检查服务是否成功启动
    if kill -0 "$pid" 2>/dev/null && ss -tlnp | grep -q ":$port "; then
        print_success "$description 启动成功 (PID: $pid, Port: $port)"
        echo "$pid" > "$LOG_DIR/$service_name.pid"
        return 0
    else
        print_error "$description 启动失败"
        return 1
    fi
}

# 函数：启动前端服务
start_frontend() {
    print_info "构建并启动前端服务..."
    
    cd "$PROJECT_ROOT/pump-tokens-ui" || {
        print_error "无法进入前端目录"
        return 1
    }

    # 检查是否已构建
    if [ ! -d "build" ]; then
        print_info "构建前端应用..."
        npm run build || {
            print_error "前端构建失败"
            return 1
        }
    fi

    # 检查端口
    if ! check_port "3001" "frontend"; then
        print_error "前端端口冲突，跳过启动"
        return 1
    fi

    # 启动前端服务
    nohup npx serve@13 -s build -l 3001 > "$LOG_DIR/frontend.log" 2>&1 &
    local pid=$!
    
    sleep 3
    
    if kill -0 "$pid" 2>/dev/null && ss -tlnp | grep -q ":3001 "; then
        print_success "前端服务启动成功 (PID: $pid, Port: 3001)"
        echo "$pid" > "$LOG_DIR/frontend.pid"
        return 0
    else
        print_error "前端服务启动失败"
        return 1
    fi
}

# 主启动流程
main() {
    print_info "================================="
    print_info "Fun DEX 微服务启动脚本"
    print_info "================================="

    # 检查 Docker 服务
    print_info "检查 Docker 中间件服务..."
    if ! docker ps | grep -q "mysql-db\|redis-cache\|kafka-broker"; then
        print_warning "Docker 中间件服务未完全启动，请先运行: cd docker && docker compose up -d"
    fi

    # 按依赖顺序启动服务
    print_info "开始启动微服务..."

    # 1. Consumer 服务 (数据源)
    start_service "consumer" "consumer" "consumer.go" "8082" "消费者服务"
    sleep 2

    # 2. Market 服务 (Market API)
    start_service "market" "market" "market.go" "8080" "市场数据服务"
    sleep 2

    # 3. Trade 服务 (交易 API)
    start_service "trade" "trade" "trade.go" "8081" "交易服务"
    sleep 2

    # 4. Gateway 服务 (API 网关)
    start_service "gateway" "gateway" "gateway.go" "8083" "网关服务"
    sleep 2

    # 5. WebSocket 服务 (实时通信)
    start_service "websocket" "websocket" "token_websocket_server.go" "8086" "WebSocket 服务"
    sleep 2

    # 6. 前端服务
    start_frontend

    print_info "================================="
    print_success "所有服务启动完成！"
    print_info "================================="
    
    # 显示服务状态
    echo -e "\n${BLUE}服务访问地址：${NC}"
    echo "• 前端界面: http://localhost:3001"
    echo "• API 网关: http://localhost:8083"
    echo "• WebSocket: ws://localhost:8086/ws/tokens"
    echo ""
    echo -e "${BLUE}服务端口：${NC}"
    echo "• Consumer: 8082"
    echo "• Market: 8080" 
    echo "• Trade: 8081"
    echo "• Gateway: 8083"
    echo "• WebSocket: 8086"
    echo "• Frontend: 3001"
    echo ""
    echo -e "${BLUE}日志位置：${NC} $LOG_DIR/"
    echo -e "${BLUE}停止服务：${NC} ./stop-services.sh"
}

# 检查参数
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help    显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0            启动所有服务"
    echo ""
    exit 0
fi

# 执行主函数
main "$@"
