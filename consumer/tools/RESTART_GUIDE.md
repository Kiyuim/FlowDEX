# Kafka 修复后重启指南

## ✅ 问题已修复

Kafka broker 已经修复并运行：
- ✅ Broker 运行在 9092 (PLAINTEXT) 和 9093 (SASL_PLAINTEXT)
- ✅ JAAS 配置已添加
- ✅ Topic `web3fun` 存在
- ✅ 权限问题已修复

## 🔄 重启 Consumer 服务

### 方法 1: 如果使用 PM2

```bash
# 查看 consumer 服务状态
pm2 list | grep consumer

# 重启 consumer 服务
pm2 restart consumer

# 或重启所有服务
pm2 restart all

# 查看日志
pm2 logs consumer
```

### 方法 2: 如果使用脚本管理

```bash
cd /home/ubuntu/dex_full/fun_dex_v2

# 停止 consumer 服务
./scripts/linux/stop-all-services.sh
# 或只停止 consumer
kill $(lsof -ti :8088)  # Consumer 监听在 8088

# 启动 consumer 服务
./scripts/linux/start-consumer.sh
# 或启动所有服务
./scripts/linux/start-all-services.sh
```

### 方法 3: 如果直接运行二进制

```bash
# 1. 找到运行中的 consumer 进程
ps aux | grep consumer.go
# 或
lsof -ti :8088

# 2. 停止进程
kill <PID>
# 或强制停止
kill -9 <PID>

# 3. 重新启动
cd /home/ubuntu/dex_full/fun_dex_v2/consumer
go run consumer.go -f etc/consumer.yaml
# 或如果已编译
./consumer -f etc/consumer.yaml
```

### 方法 4: 如果使用 systemd

```bash
# 重启服务
sudo systemctl restart consumer.service

# 查看状态
sudo systemctl status consumer.service

# 查看日志
sudo journalctl -u consumer.service -f
```

## ✅ 验证 Kafka 连接

重启后，检查日志中是否有以下信息：

### 1. 检查初始化日志

```bash
# 查找 Kafka 初始化日志
grep -i "Initializing Kafka\|Kafka producer\|_kafkaClient" /path/to/consumer/logs

# 应该看到：
# ✅ "Initializing Kafka producer..."
# ✅ "Initializing Kafka producer with brokers: [115.159.107.189:9093], username: alikafka_post-cn-zp54bjj8x004"
# ✅ "✅ Kafka producer initialized successfully"
# ✅ "producer  connect success"
```

### 2. 检查错误日志

```bash
# 不应该再看到这些错误：
grep -i "_kafkaClient is Nil\|Cannot send message to Kafka" /path/to/consumer/logs

# 如果看到，说明初始化仍然失败，需要检查：
# - Kafka broker 是否真的在运行
# - 网络连接是否正常
# - 用户名密码是否正确
```

### 3. 测试消息发送

等待一段时间后，检查是否有消息成功发送到 Kafka：

```bash
# 查看是否有成功发送的日志
grep -i "send event log to kafka success" /path/to/consumer/logs

# 或使用 Kafka 工具查看消息
kcat -b 115.159.107.189:9093 \
  -X security.protocol=SASL_SSL \
  -X sasl.mechanism=PLAIN \
  -X sasl.username=alikafka_post-cn-zp54bjj8x004 \
  -X sasl.password=htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1 \
  -C -t web3fun -c 5
```

## 🔍 故障排查

### 如果重启后仍然失败

1. **检查 Kafka broker 状态**
   ```bash
   # 检查端口是否监听
   netstat -tlnp | grep 9093
   # 或
   ss -tlnp | grep 9093
   ```

2. **测试 Kafka 连接**
   ```bash
   cd /home/ubuntu/dex_full/fun_dex_v2/consumer/tools
   go run kafka_check.go 115.159.107.189:9093 alikafka_post-cn-zp54bjj8x004 htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1
   ```

3. **检查配置文件**
   ```bash
   cat /home/ubuntu/dex_full/fun_dex_v2/consumer/etc/consumer.yaml | grep -A 10 KqSolTrades
   ```

4. **查看详细错误日志**
   ```bash
   # 查看 consumer 启动日志
   tail -f /path/to/consumer/logs/consumer.log
   # 或
   pm2 logs consumer
   ```

## 📝 预期结果

重启成功后，你应该看到：

1. ✅ **启动日志**：
   ```
   Initializing Kafka producer...
   Initializing Kafka producer with brokers: [115.159.107.189:9093], username: alikafka_post-cn-zp54bjj8x004
   ✅ Kafka producer initialized successfully
   producer  connect success
   ```

2. ✅ **消息发送成功日志**：
   ```
   [kafka] send event log to kafka success: web3fun:0:12345, slot, len(data): 1024
   ```

3. ❌ **不再出现错误**：
   - 不应该再看到 `_kafkaClient is Nil`
   - 不应该再看到 `Cannot send message to Kafka`

## 🎯 快速重启命令

```bash
# 一键重启（如果使用 PM2）
pm2 restart consumer && pm2 logs consumer --lines 50

# 或（如果直接运行）
pkill -f consumer.go && cd /home/ubuntu/dex_full/fun_dex_v2/consumer && go run consumer.go -f etc/consumer.yaml &
```
