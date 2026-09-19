# Fun DEX Ubuntu 服务管理脚本

这个目录包含了在 Ubuntu 系统上管理 Fun DEX 微服务的脚本工具。

## 📋 脚本列表

- `manage.sh` - 统一管理入口（推荐使用，包含所有功能）
- `start-services.sh` - 启动所有微服务
- `stop-services.sh` - 停止所有微服务

## 🚀 快速使用

### 方式一：使用统一管理脚本（推荐）

```bash
# 进入项目根目录
cd /usr/code/alan_fun_dex_v2

# 查看帮助
./scripts/ubuntu/manage.sh help

# 启动 Docker 中间件
./scripts/ubuntu/manage.sh docker-up

# 启动所有服务
./scripts/ubuntu/manage.sh start

# 查看服务状态
./scripts/ubuntu/manage.sh status

# 停止所有服务
./scripts/ubuntu/manage.sh stop

# 重启服务
./scripts/ubuntu/manage.sh restart
```

### 方式二：直接调用具体脚本

```bash
# 添加执行权限
chmod +x scripts/ubuntu/*.sh

# 启动所有服务
./scripts/ubuntu/start-services.sh

# 查看服务状态
./scripts/ubuntu/manage.sh status

# 停止所有服务
./scripts/ubuntu/stop-services.sh -f
```

## 📊 服务端口

| 服务 | 端口 | 描述 |
|------|------|------|
| Consumer | 8082 | 消费者服务 |
| Market | 8080 | 市场数据服务 |
| Trade | 8081 | 交易服务 |
| Gateway | 8083 | API 网关 |
| WebSocket | 8086 | 实时通信服务 |
| Frontend | 3001 | 前端界面 |

## 🔗 访问地址

- **前端界面**: http://localhost:3001
- **API 网关**: http://localhost:8083
- **WebSocket**: ws://localhost:8086/ws/tokens

## 📝 日志查看

启动后，所有服务的日志都会保存在项目根目录的 `logs/` 文件夹中：

```bash
# 查看所有日志
make logs

# 查看特定服务日志
tail -f logs/consumer.log
tail -f logs/market.log
tail -f logs/gateway.log
```

## 🐳 Docker 中间件

在启动微服务之前，确保 Docker 中间件服务已启动：

```bash
# 启动 Docker 服务
make docker-up

# 检查 Docker 状态
docker ps
```

## 🛠️ 故障排除

### 端口占用问题
```bash
# 查看端口占用
ss -tlnp | grep -E "(8080|8081|8082|8083|8086|3001)"

# 强制停止所有服务
make force-stop
```

### 重新部署
```bash
# 完全重新部署
make stop
make clean
make deploy
```

### 查看数据库状态
```bash
# 检查数据库数据
make db-status
```

## 📈 开发模式

如果你是开发者，可以使用开发模式：

```bash
# 开发模式启动（不构建前端）
make dev

# 单独重新构建前端
make build

# 测试 API 接口
make test-api
```

## ⚠️ 注意事项

1. **首次运行**：建议使用 `make deploy` 进行完整部署
2. **端口冲突**：如果遇到端口冲突，使用 `make force-stop` 强制停止
3. **日志监控**：可以使用 `make logs-<service>` 实时查看特定服务日志
4. **数据库**：确保 Docker 中的 MySQL 和 Redis 服务正常运行

## 🔧 自定义配置

如果需要修改配置：

1. **服务端口**: 编辑各服务的配置文件（如 `consumer/etc/consumer-local.yaml`）
2. **数据库连接**: 编辑 `docker/docker-compose.yml`
3. **前端代理**: 编辑 `pump-tokens-ui/package.json` 中的 proxy 设置
