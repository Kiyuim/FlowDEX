#!/bin/bash

# Fun DEX 微服务管理脚本
# 统一管理入口，调用同目录下的具体脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 颜色定义
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 确保在项目根目录
cd "$PROJECT_ROOT" || {
    echo "错误：无法进入项目目录 $PROJECT_ROOT"
    exit 1
}

# 添加执行权限
chmod +x "$SCRIPT_DIR"/*.sh 2>/dev/null

# 显示帮助信息
show_help() {
    echo -e "${BLUE}Fun DEX 微服务管理工具${NC}"
    echo ""
    echo "用法: $0 <command>"
    echo ""
    echo -e "${GREEN}基础命令:${NC}"
    echo "  start     启动所有微服务"
    echo "  stop      停止所有微服务"
    echo "  restart   重启所有微服务"
    echo "  status    查看服务状态"
    echo "  logs      查看服务日志"
    echo ""
    echo -e "${GREEN}Docker 命令:${NC}"
    echo "  docker-up   启动 Docker 中间件"
    echo "  docker-down 停止 Docker 中间件"
    echo ""
    echo -e "${GREEN}开发命令:${NC}"
    echo "  clean     清理日志文件"
    echo "  build     构建前端应用"
    echo ""
    echo -e "${GREEN}示例:${NC}"
    echo "  $0 docker-up && $0 start  # 完整启动"
    echo "  $0 status                 # 查看状态"
    echo "  $0 restart                # 重启服务"
    echo ""
    echo -e "${YELLOW}详细文档: scripts/ubuntu/README.md${NC}"
}

# 主逻辑
case "$1" in
    "start")
        echo "🚀 启动 Fun DEX 微服务..."
        "$SCRIPT_DIR"/start-services.sh
        ;;
    "stop")
        echo "🛑 停止 Fun DEX 微服务..."
        "$SCRIPT_DIR"/stop-services.sh
        ;;
    "force-stop")
        echo "⚠️  强制停止 Fun DEX 微服务..."
        "$SCRIPT_DIR"/stop-services.sh -f
        ;;
    "restart")
        echo "🔄 重启 Fun DEX 微服务..."
        "$SCRIPT_DIR"/stop-services.sh
        sleep 2
        "$SCRIPT_DIR"/start-services.sh
        ;;
    "status")
        echo "============================================="
        echo "Fun DEX 微服务状态检查"
        echo "============================================="
        echo ""
        
        # 检查微服务状态
        echo "检查微服务状态："
        check_service() {
            local service_name=$1
            local port=$2
            local description=$3
            
            if ss -tlnp | grep -q ":$port "; then
                local pid=$(ss -tlnp | grep ":$port " | awk '{print $6}' | sed 's/.*pid=\([0-9]*\).*/\1/' | head -1)
                echo -e "\033[0;32m[✓]\033[0m $description 运行中 (端口: $port, PID: $pid)"
            else
                echo -e "\033[0;31m[✗]\033[0m $description 未运行 (端口: $port)"
            fi
        }
        
        check_service "consumer" "8082" "消费者服务"
        check_service "market" "8080" "市场数据服务"
        check_service "trade" "8081" "交易服务"
        check_service "gateway" "8083" "网关服务"
        check_service "websocket" "8086" "WebSocket 服务"
        check_service "frontend" "3001" "前端服务"
        echo ""
        
        # 检查数据库
        echo "检查数据库连接："
        if docker exec mysql-db mysql -u root -p'web3ite.fun' -e "SELECT 1;" >/dev/null 2>&1; then
            echo -e "\033[0;32m[✓]\033[0m MySQL 数据库连接正常"
            local pump_count=$(docker exec mysql-db mysql -u root -p'web3ite.fun' -e "USE fun_dexs; SELECT COUNT(*) FROM pair WHERE name='PumpFun';" 2>/dev/null | tail -1)
            if [ "$pump_count" -gt 0 ]; then
                echo -e "\033[0;32m[✓]\033[0m PumpFun 数据已采集 ($pump_count 条记录)"
            else
                echo -e "\033[0;31m[✗]\033[0m PumpFun 数据未采集"
            fi
        else
            echo -e "\033[0;31m[✗]\033[0m MySQL 数据库连接失败"
        fi
        
        if docker exec redis-cache redis-cli ping >/dev/null 2>&1; then
            echo -e "\033[0;32m[✓]\033[0m Redis 缓存连接正常"
        else
            echo -e "\033[0;31m[✗]\033[0m Redis 缓存连接失败"
        fi
        echo ""
        
        echo "============================================="
        echo "服务访问地址："
        echo "• 前端界面: http://localhost:3001"
        echo "• API 网关: http://localhost:8083"
        echo "• WebSocket: ws://localhost:8086/ws/tokens"
        echo "============================================="
        ;;
    "logs")
        if [ -d "logs" ]; then
            echo "📋 最近的服务日志："
            for log in logs/*.log; do
                if [ -f "$log" ]; then
                    echo ""
                    echo "=== $(basename "$log") ==="
                    tail -10 "$log"
                fi
            done
        else
            echo "❌ 日志目录不存在，请先启动服务"
        fi
        ;;
    "docker-up")
        echo "🐳 启动 Docker 中间件..."
        cd docker && docker compose up -d
        echo "✅ Docker 服务启动完成"
        ;;
    "docker-down")
        echo "🐳 停止 Docker 中间件..."
        cd docker && docker compose down
        echo "✅ Docker 服务停止完成"
        ;;
    "clean")
        echo "🧹 清理日志文件..."
        rm -rf logs/*.log logs/*.pid
        echo "✅ 清理完成"
        ;;
    "build")
        echo "🔨 构建前端应用..."
        cd pump-tokens-ui && npm run build
        echo "✅ 前端构建完成"
        ;;
    "help"|"-h"|"--help"|"")
        show_help
        ;;
    *)
        echo "❌ 错误：未知命令 '$1'"
        echo ""
        show_help
        exit 1
        ;;
esac
