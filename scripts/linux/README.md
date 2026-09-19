# Fun DEX Linux Scripts

This directory contains automated scripts for managing Fun DEX backend services on Linux systems.

## Quick Start

1. **Setup** (run once):
   ```bash
   chmod +x scripts/linux/setup.sh
   ./scripts/linux/setup.sh
   ```

2. **Start all services**:
   ```bash
   ./scripts/linux/start-all-services.sh
   ```

3. **Check status**:
   ```bash
   ./scripts/linux/status-services.sh
   ```

4. **Interactive management**:
   ```bash
   ./scripts/linux/manage-services.sh
   ```

## Available Scripts

### Main Service Management

| Script | Description |
|--------|-------------|
| `start-all-services.sh` | Start all backend services in the correct order |
| `stop-all-services.sh` | Stop all running backend services |
| `restart-services.sh` | Restart all backend services |
| `status-services.sh` | Check the status of all services |
| `manage-services.sh` | Interactive menu for service management |

### Individual Service Scripts

| Script | Service | Port |
|--------|---------|------|
| `start-consumer.sh` | Consumer Service | 8080 |
| `start-market.sh` | Market Service | 8081 |
| `start-trade.sh` | Trade Service | 8082 |
| `start-gateway.sh` | Gateway Service | 8083 |
| `start-websocket.sh` | WebSocket Service | 8084 |

### Utility Scripts

| Script | Description |
|--------|-------------|
| `setup.sh` | Initial setup and make scripts executable |

## Service Architecture

The scripts manage the following services in order:

1. **Consumer Service** (Port 8080)
   - Processes blockchain data
   - Feeds data to other services

2. **Market Service** (Port 8081)
   - Provides market data
   - Token information and prices

3. **Trade Service** (Port 8082)
   - Handles trading operations
   - Order processing

4. **Gateway Service** (Port 8083)
   - API gateway
   - Request routing

5. **WebSocket Service** (Port 8084)
   - Real-time communication
   - Live data streaming

## Features

### Service Management
- **Automatic startup order**: Services start in the correct dependency order
- **Process monitoring**: Check if services are running and healthy
- **Graceful shutdown**: Proper termination with fallback to force kill
- **Log management**: Centralized logging with rotation
- **PID tracking**: Process ID tracking for reliable service management

### Error Handling
- **Dependency checks**: Verify Go installation and project structure
- **Port conflict detection**: Check for port conflicts before starting
- **Service validation**: Verify services start successfully
- **Comprehensive logging**: Detailed logs for troubleshooting

### System Integration
- **Background execution**: Services run as background processes
- **Signal handling**: Proper signal handling for clean shutdown
- **Resource monitoring**: CPU, memory, and disk usage tracking
- **Network monitoring**: Connection monitoring and port status

## Prerequisites

- **Go 1.24.2+**: Required for running the services
- **lsof**: For port checking (install with `sudo apt-get install lsof`)
- **netstat**: For network monitoring (install with `sudo apt-get install net-tools`)
- **Linux system**: Optimized for Ubuntu/Debian systems

## Directory Structure

```
scripts/linux/
├── README.md                 # This file
├── setup.sh                 # Initial setup script
├── start-all-services.sh    # Start all services
├── stop-all-services.sh     # Stop all services
├── restart-services.sh      # Restart all services
├── status-services.sh       # Check service status
├── manage-services.sh       # Interactive management
├── start-consumer.sh        # Individual service scripts
├── start-market.sh
├── start-trade.sh
├── start-gateway.sh
└── start-websocket.sh
```

## Log Files

Logs are stored in the `logs/` directory at the project root:

```
logs/
├── consumer.log      # Consumer service logs
├── consumer.pid      # Consumer service PID
├── market.log        # Market service logs
├── market.pid        # Market service PID
├── trade.log         # Trade service logs
├── trade.pid         # Trade service PID
├── gateway.log       # Gateway service logs
├── gateway.pid       # Gateway service PID
├── websocket.log     # WebSocket service logs
└── websocket.pid     # WebSocket service PID
```

## Usage Examples

### Starting Services

```bash
# Start all services
./scripts/linux/start-all-services.sh

# Start individual service
./scripts/linux/start-consumer.sh

# Start with interactive menu
./scripts/linux/manage-services.sh
```

### Monitoring Services

```bash
# Check all services
./scripts/linux/status-services.sh

# View specific service logs
tail -f logs/consumer.log

# Monitor system resources
./scripts/linux/manage-services.sh
# Then select option 8 (System Information)
```

### Stopping Services

```bash
# Stop all services
./scripts/linux/stop-all-services.sh

# Stop specific service by port
sudo kill $(lsof -ti :8080)  # Consumer service
```

### Troubleshooting

```bash
# Check service status
./scripts/linux/status-services.sh

# View recent logs
./scripts/linux/manage-services.sh
# Then select option 6 (View Service Logs)

# Clean old logs
./scripts/linux/manage-services.sh
# Then select option 7 (Clean Service Logs)
```

## Interactive Management Menu

The `manage-services.sh` script provides an interactive menu with the following options:

1. **Start All Services** - Start all backend services
2. **Stop All Services** - Stop all running services
3. **Restart All Services** - Restart all services
4. **Check Services Status** - View detailed status information
5. **Start Individual Service** - Start a specific service
6. **View Service Logs** - View recent log entries
7. **Clean Service Logs** - Remove old log files
8. **System Information** - View system resource usage
9. **Exit** - Exit the management interface

## Best Practices

1. **Always run from project root**: Scripts must be executed from the project root directory
2. **Use setup script first**: Run `setup.sh` before using other scripts
3. **Check status regularly**: Monitor service health with `status-services.sh`
4. **Review logs**: Check log files for errors and warnings
5. **Graceful shutdown**: Use `stop-all-services.sh` instead of manual kills
6. **Sequential startup**: Services have dependencies, use `start-all-services.sh`

## Troubleshooting

### Common Issues

1. **Port already in use**:
   ```bash
   # Check what's using the port
   lsof -i :8080
   # Kill the process
   sudo kill $(lsof -ti :8080)
   ```

2. **Go not found**:
   ```bash
   # Check Go installation
   which go
   go version
   # Add Go to PATH if needed
   export PATH=$PATH:/usr/local/go/bin
   ```

3. **Permission denied**:
   ```bash
   # Make scripts executable
   chmod +x scripts/linux/*.sh
   ```

4. **Service won't start**:
   ```bash
   # Check logs
   tail -f logs/servicename.log
   # Check dependencies
   ./scripts/linux/status-services.sh
   ```

### Getting Help

- Check service logs in `logs/` directory
- Run `status-services.sh` for detailed status
- Use the interactive menu for guided troubleshooting
- Ensure all prerequisites are installed

## Security Considerations

- Scripts run services with current user permissions
- Log files may contain sensitive information
- Services bind to localhost by default
- Consider firewall rules for production deployment

## Performance Tips

- Monitor system resources with the management menu
- Clean log files regularly to save disk space
- Use individual service scripts for development
- Consider process limits for production deployment 