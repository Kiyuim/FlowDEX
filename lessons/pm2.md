# PM2 Configuration for Backend Services

## Overview

Create a PM2 ecosystem configuration file (`ecosystem.config.js`) to manage all backend services (market, trade, consumer, websocket, gateway) and frontend (pump-tokens-ui) as background processes.

## Current State

- Existing `ecosystem.config.js` contains `pump-tokens-ui` configuration that needs to be updated/verified
- Backend services are Go applications that need to be run from their respective directories
- Each service uses relative config file paths (e.g., `etc/market.yaml`)
- Backend services use `go run` command to start
- Frontend is a React app served via `npx serve -s build -l 3001` (from start-pump-ui.sh)

## Services to Configure

### 1. Market Service

- **File**: `market/market.go`
- **Config**: `market/etc/market.yaml`
- **Port**: 8080 (from config file)
- **Working Directory**: `market/`
- **Command**: `go run market.go`

### 2. Trade Service

- **File**: `trade/trade.go`
- **Config**: `trade/etc/trade.yaml`
- **Port**: 8081 (from start-all-services.sh)
- **Working Directory**: `trade/`
- **Command**: `go run trade.go`

### 3. Consumer Service

- **File**: `consumer/consumer.go`
- **Config**: `consumer/etc/consumer.yaml`
- **Port**: 8082 (from start-all-services.sh)
- **Working Directory**: `consumer/`
- **Command**: `go run consumer.go`

### 4. Gateway Service

- **File**: `gateway/gateway.go`
- **Config**: `gateway/etc/gateway.yaml`
- **Port**: 8083 (from start-all-services.sh)
- **Working Directory**: `gateway/`
- **Command**: `go run gateway.go`

### 5. WebSocket Service

- **File**: `websocket/token_websocket_server.go`
- **Config**: `websocket/etc/websocket.yaml`
- **Port**: 8086 (from config file)
- **Working Directory**: `websocket/`
- **Command**: `go run token_websocket_server.go`

## Implementation Plan

### Step 1: Update ecosystem.config.js

Update the existing `ecosystem.config.js` file to include all 5 backend services and update/verify the `pump-tokens-ui` frontend configuration.

**Configuration Details**:

- Each service needs its own directory as `cwd` (current working directory)
- Use `go run` command with the appropriate `.go` file
- Set up log files in a `logs/` directory
- Configure environment variables if needed (e.g., PATH for Go)
- Set `autorestart: true` for production reliability
- Configure memory limits if needed

### Step 2: Create Logs Directory

Ensure `logs/` directory exists for log files.

### Step 3: PM2 Commands

After configuration, services can be managed with:

- `pm2 start ecosystem.config.js` - Start all services
- `pm2 stop ecosystem.config.js` - Stop all services
- `pm2 restart ecosystem.config.js` - Restart all services
- `pm2 delete ecosystem.config.js` - Remove all services
- `pm2 logs` - View logs
- `pm2 status` - Check service status

## File Structure

```
fun_dex_v2/
├── ecosystem.config.js (updated)
├── logs/ (created if not exists)
│   ├── market.log
│   ├── trade.log
│   ├── consumer.log
│   ├── gateway.log
│   ├── websocket.log
│   └── pump-tokens-ui.log
├── market/
├── trade/
├── consumer/
├── gateway/
├── websocket/
└── pump-tokens-ui/
    └── build/ (must exist for serve command)
```

## Configuration Template

Each service will have:

- `name`: Service identifier
- `cwd`: Service directory path
- `script`: `go run <service_file.go>`
- `instances`: 1 (single instance per service)
- `autorestart`: true
- `watch`: false (disable file watching for production)
- `max_memory_restart`: Appropriate limit (e.g., '500M' for Go services)
- `env`: Environment variables (PATH for Go)
- `error_file`, `out_file`: Log file paths
- `log_date_format`: Timestamp format

## Notes

- All services must be run from project root directory (`/home/ubuntu/dex_full/fun_dex_v2`)
- Go binary must be in PATH (typically `/usr/local/go/bin`)
- Backend services will start in dependency order if needed (consumer → market → trade → gateway → websocket)
- Frontend requires `build/` directory to exist (run `npm run build` in `pump-tokens-ui/` first)
- PM2 will handle process management, auto-restart, and log rotation
- Frontend can use either the shell script (`start-pump-ui.sh`) or direct `npx serve` command


### 常用命令
cd /home/ubuntu/dex_full/fun_dex_v2

# 启动所有服务
cd /home/ubuntu/dex_full/fun_dex_v2
pm2 start ecosystem.config.js


# 查看服务状态
pm2 status

# 查看日志
pm2 logs

# 查看特定服务的日志
pm2 logs market
pm2 logs pump-tokens-ui

# 停止所有服务
pm2 stop ecosystem.config.js

# 重启所有服务
pm2 restart ecosystem.config.js

# 删除所有服务
pm2 delete ecosystem.config.js

# 保存当前进程列表（开机自启）
pm2 save
pm2 startup  # 生成启动脚本