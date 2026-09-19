# SASL Handshake 故障排查指南

## 🔴 错误症状

```
[sarama] Error while performing SASL handshake 115.159.107.189:9093
[sarama] Closed connection to broker 115.159.107.189:9093
[sarama] client/metadata got error from broker -1 while fetching metadata: EOF
```

## 🔍 问题诊断

SASL handshake 失败通常由以下原因引起：

### 1. Broker SASL 配置问题

**检查 broker 的 server.properties：**

```properties
# 检查 listeners 配置
listeners=SASL_PLAINTEXT://0.0.0.0:9093

# 或
listeners=SASL_SSL://0.0.0.0:9093

# 检查 security.inter.broker.protocol
security.inter.broker.protocol=SASL_PLAINTEXT

# 检查 SASL 机制
sasl.enabled.mechanisms=PLAIN
sasl.mechanism.inter.broker.protocol=PLAIN
```

### 2. JAAS 配置文件问题

**检查 broker 的 JAAS 配置** (`config/kafka_server_jaas.conf`):

```properties
KafkaServer {
    org.apache.kafka.common.security.plain.PlainLoginModule required
    username="admin"
    password="admin-secret"
    user_admin="admin-secret"
    user_alikafka_post-cn-zp54bjj8x004="htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1";
};
```

**重要检查点：**
- ✅ JAAS 文件路径正确
- ✅ `java.security.auth.login.config` 系统属性已设置
- ✅ 用户名和密码匹配
- ✅ `user_<username>` 条目存在

### 3. 客户端配置不匹配

**当前代码会自动尝试两种配置：**
- SASL_PLAINTEXT (无 TLS)
- SASL_SSL (有 TLS)

如果两种都失败，检查：

```bash
# 1. 验证 broker 监听器类型
netstat -tlnp | grep 9093
# 应该看到 LISTEN 状态

# 2. 测试连接（不使用 SASL）
telnet 115.159.107.189 9093
# 如果连接成功，说明端口开放，问题在 SASL

# 3. 检查 broker 日志
tail -f /path/to/kafka/logs/server.log | grep -i "sasl\|auth\|handshake"
```

## 🛠️ 修复步骤

### 步骤 1: 验证 Broker JAAS 配置

```bash
# 检查 JAAS 文件是否存在
cat config/kafka_server_jaas.conf

# 检查启动脚本是否设置了系统属性
grep -i "java.security.auth.login.config" bin/kafka-server-start.sh

# 应该看到类似：
# export KAFKA_OPTS="-Djava.security.auth.login.config=$base_dir/../config/kafka_server_jaas.conf"
```

### 步骤 2: 验证用户名密码

```bash
# 在 JAAS 文件中确认用户名和密码
grep "user_alikafka_post-cn-zp54bjj8x004" config/kafka_server_jaas.conf

# 应该看到：
# user_alikafka_post-cn-zp54bjj8x004="htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1"
```

### 步骤 3: 重启 Broker（如果修改了配置）

```bash
# 停止 broker
pkill -f kafka-server-start

# 等待几秒
sleep 3

# 重新启动
bin/kafka-server-start.sh config/server.properties
```

### 步骤 4: 测试连接

```bash
# 使用 Go 工具测试
cd /home/ubuntu/dex_full/fun_dex_v2/consumer/tools
go run kafka_check.go 115.159.107.189:9093 alikafka_post-cn-zp54bjj8x004 htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1
```

### 步骤 5: 检查 Broker 日志

```bash
# 查看 broker 启动日志
tail -50 /path/to/kafka/logs/server.log

# 查找 SASL 相关错误
grep -i "sasl\|auth\|handshake\|jaas" /path/to/kafka/logs/server.log
```

## 🔧 常见问题修复

### 问题 1: JAAS 文件未加载

**症状：**
```
Could not find a 'KafkaServer' entry in the JAAS configuration
```

**修复：**
```bash
# 在启动脚本中添加
export KAFKA_OPTS="-Djava.security.auth.login.config=$base_dir/../config/kafka_server_jaas.conf"

# 或在 server.properties 中设置
# 但推荐在启动脚本中设置
```

### 问题 2: 用户名密码不匹配

**症状：**
```
SASL authentication failed
```

**修复：**
1. 检查客户端配置的用户名密码
2. 检查 JAAS 文件中的 `user_<username>` 条目
3. 确保密码完全匹配（包括引号）

### 问题 3: SASL Mechanism 不匹配

**症状：**
```
Error while performing SASL handshake
```

**修复：**
- 客户端使用：`sarama.SASLTypePlaintext`
- Broker 应该配置：`sasl.enabled.mechanisms=PLAIN`

### 问题 4: TLS/SASL 组合错误

**症状：**
```
EOF during handshake
```

**修复：**
- 如果 broker 使用 `SASL_PLAINTEXT`，客户端禁用 TLS
- 如果 broker 使用 `SASL_SSL`，客户端启用 TLS
- 当前代码会自动尝试两种配置

## 📋 检查清单

- [ ] Broker 正在运行
- [ ] JAAS 配置文件存在且格式正确
- [ ] `java.security.auth.login.config` 系统属性已设置
- [ ] JAAS 文件中的用户名密码与客户端匹配
- [ ] Broker 的 `listeners` 配置正确（SASL_PLAINTEXT 或 SASL_SSL）
- [ ] Broker 的 `sasl.enabled.mechanisms` 包含 `PLAIN`
- [ ] 端口 9093 正在监听
- [ ] 防火墙允许连接

## 🧪 测试命令

```bash
# 1. 测试端口连接
nc -zv 115.159.107.189 9093

# 2. 使用 Kafka CLI 测试（如果已安装）
kafka-console-producer --bootstrap-server 115.159.107.189:9093 \
  --topic web3fun \
  --producer-property security.protocol=SASL_PLAINTEXT \
  --producer-property sasl.mechanism=PLAIN \
  --producer-property sasl.jaas.config="org.apache.kafka.common.security.plain.PlainLoginModule required username=\"alikafka_post-cn-zp54bjj8x004\" password=\"htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1\";"

# 3. 使用 Go 工具测试
cd /home/ubuntu/dex_full/fun_dex_v2/consumer/tools
go run kafka_check.go 115.159.107.189:9093 alikafka_post-cn-zp54bjj8x004 htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1
```

## 📝 下一步

1. **检查 broker 日志**：查看详细的 SASL 错误信息
2. **验证 JAAS 配置**：确保用户名密码正确
3. **重启 broker**：应用配置更改
4. **重启 consumer**：使用新的客户端配置
