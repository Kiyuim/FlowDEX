# Fun DEX — 单机自托管部署手册 (Self-Hosted Deployment Guide)

把整套 Fun DEX（5 个 Go 微服务 + WebSocket 服务 + React 前端 + MySQL / Redis / Kafka）
部署到**一台干净的 Linux 服务器**上，全部由 Docker Compose 编排。

> 本手册在 **47.242.157.80（阿里云 ECS 香港，Ubuntu 26.04 LTS，8 vCPU / 14 GB / 40 GB）**
> 上从零实测跑通。第 8 节《踩坑记录》列出了实测中遇到的**每一个**报错、根因和修复。

**分支：`devnet`**（Solana devnet）。

---

## 0. 成品拓扑

```
                                Browser
                                   |
                                   v
        +-------------------------------------------------------+
        |  frontend    nginx + CRA build output           :80   |  <-- only
        |     /               ->   static SPA                   |      public
        |     /v1/*           ->   gateway                      |      port
        |     /direct-api/*   ->   gateway                      |
        |     /ws/*           ->   websocket                    |
        +-----------------+--------------------+----------------+
                          |                    |
                          v                    v
        +----------------------+      +----------------------+
        |  gateway      :8083  |      |  websocket    :8086  |
        |  HTTP   ->   gRPC    |      |  Redis subscribe     |
        +----------+-----------+      +----------+-----------+
                   |                             |
         +---------+---------+                   |
         v                   v                   |
   +--------------+   +--------------+           |
   |  market      |   |  trade       |           |
   |  :8080       |   |  :8081       |           |
   +------+-------+   +------+-------+           |
          |                  |                   |
          +--------+---------+                   |
                   v                             v
          +----------------+            +----------------+
          |  mysql   :3306 |            |  redis   :6379 |
          +----------------+            +----------------+
                   ^                             ^
                   |                             |
          +--------+-----------------------------+--------+
          |  consumer     :8088                           |
          |  devnet blocks  ->  decode  ->  MySQL         |
          |                 ->  Kafka   ->  Redis publish |
          +---------------------+-------------------------+
                                | topic = web3fun
                                v
                     +----------------------+
                     |  kafka   (redpanda)  |
                     +----------+-----------+
                                | consume
                                v
                     +----------------------+
                     |  dataflow     :8084  |
                     |  OHLC -> MySQL/Redis |
                     +----------------------+
```

| 服务 | 端口 | 作用 |
|---|---|---|
| `frontend` | **80** | nginx 托管 React 构建产物，并反代 API 与 WS |
| `gateway` | 8083 | 唯一 HTTP 入口，按 protoset 把 REST 转成 gRPC |
| `market` | 8080 | 行情 / 代币列表 / K 线查询（gateway、trade、consumer 都拨它） |
| `trade` | 8081 | 构造**未签名**交易返回给前端钱包签名 |
| `consumer` | 8088 | 拉 devnet 区块 → 解析 pump / pump_meteora 指令 → 写库 + 投 Kafka + 发 Redis |
| `dataflow` | 8084 | 消费 Kafka → 聚合 OHLC K 线 → 写 MySQL / Redis（**无对外接口**，别给它开域名） |
| `websocket` | 8086 | 订阅 Redis，向前端推新代币 / K 线 |
| `mysql` / `redis` / `kafka` | 内网 | 数据与消息中间件 |

> **只有 80 端口必须对外开放。** 前端 nginx 已经把 `/v1`、`/direct-api`、`/ws`
> 全部代理到内部服务了。8083 / 8086 额外映射出来只是为了方便调试，生产可以去掉。

---

## 1. 前置条件

- 一台 Linux 服务器（实测 Ubuntu 26.04；Debian / CentOS 同理），**≥ 4 GB 内存、≥ 20 GB 磁盘**。
  构建阶段 6 个镜像并行编译，8 vCPU 上 load 会冲到 100+，小机器建议见 §8-⑧ 改成串行构建。
- 能访问外网。**国内 ECS 强烈建议选香港 / 新加坡节点**，否则 Docker Hub / Go proxy / npm 都要配镜像加速。
- root 或 sudo 权限。
- 云厂商**安全组放行 TCP 80**（想直连调试再放 8083 / 8086）。
  这一步在**控制台**做——服务器内部 `ufw` 是关的也没用，安全组才是第一道门。

先做连通性体检：

```bash
for u in https://registry-1.docker.io/v2/ https://proxy.golang.org/ \
         https://registry.npmjs.org/ https://api.devnet.solana.com; do
  printf "%-42s " "$u"
  curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" --max-time 12 "$u"
done
```

四行都要有响应（Docker registry 返回 `401` 属正常，说明 TLS 握手通了）。实测输出：

```
https://registry-1.docker.io/v2/           401 0.686488s
https://proxy.golang.org/                  200 0.040976s
https://registry.npmjs.org/                200 0.025411s
https://api.devnet.solana.com              200 0.129159s
```

---

## 2. 安装 Docker

```bash
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y ca-certificates curl gnupg gettext-base

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

CODENAME=$(. /etc/os-release && echo "$VERSION_CODENAME")
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
https://download.docker.com/linux/ubuntu $CODENAME stable" > /etc/apt/sources.list.d/docker.list

apt-get update -qq
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

docker --version        # 实测：Docker version 29.7.0
docker compose version  # 实测：Docker Compose version v5.3.1
```

`gettext-base` 提供 `envsubst`，第 4 节渲染配置要用。

> Ubuntu 26.04 的 codename 是 `resolute`，Docker 官方源已经有对应目录，实测直接装成功。
> 如果你的发行版还没被 Docker 收录（`apt-get update` 报 404），把 `$CODENAME` 换成上一个
> LTS 代号（如 `noble`）即可。

---

## 3. 上传代码

在**本地开发机**上（排除 `.git` / `node_modules` / 前端构建产物后约 107 MB）：

```bash
cd /path/to/fun_dex_v2
git checkout devnet

ssh root@<HOST> 'mkdir -p /opt/fun_dex_v2'
tar czf - --exclude=.git --exclude=node_modules \
          --exclude=pump-tokens-ui/build --exclude='*.log' . \
  | ssh root@<HOST> 'tar xzf - -C /opt/fun_dex_v2'
```

或者直接在服务器上 `git clone <repo> /opt/fun_dex_v2 && cd /opt/fun_dex_v2 && git checkout devnet`。

---

## 4. 配置

所有部署产物集中在 `deploy/` 目录，**不需要改动仓库里任何原有的 `*/etc/*.yaml`**：

```
deploy/
├── docker-compose.yml      # 全栈编排（10 个服务）
├── .env.example            # 唯一的配置源，复制成 .env 后修改
├── render.sh               # templates/*.yaml --envsubst--> config/*.yaml
├── templates/*.yaml        # 6 个服务的配置模板（指向 compose 内部 DNS）
├── config/*.yaml           # 渲染产物（已 gitignore），bind-mount 进容器
├── Dockerfile.frontend     # CRA 构建 + nginx
└── nginx/default.conf      # 静态托管 + /v1 /direct-api /ws 反代
```

### 为什么要 `render.sh` 这一步？

1. 仓库里的 `*/etc/*.yaml` 全部指向 Railway 内网域名（`mysql.railway.internal`、
   `market.railway.internal` …）。自托管必须换成 compose 的服务名
   （`mysql` / `redis` / `kafka` / `market` / `trade`）。
2. go-zero 的 `conf.MustLoad` **默认不做 `${VAR}` 展开**（各服务都没传 `conf.UseEnv()`），
   所以不能直接在 YAML 里写环境变量占位符。
3. 于是用 `envsubst` 在**部署时**渲染一次 —— 这样 **`.env` 是唯一的真相来源**，
   改个 MySQL 密码不用在 5 个文件里同步改。

渲染出的 `config/*.yaml` 通过 bind-mount **覆盖**镜像里的 `/app/etc/*.yaml`，
好处是**镜像和 Railway 上跑的完全一致，改配置不需要重新 build，重启即可**。

### 操作

```bash
cd /opt/fun_dex_v2/deploy
cp .env.example .env
vi .env            # 至少改 PUBLIC_HOST / REACT_APP_WS_URL / REACT_APP_KLINE_WS_URL
./render.sh        # 渲染出 config/*.yaml；有未替换的 ${} 会直接报错退出
```

`.env` 中必须确认的项：

| 变量 | 说明 |
|---|---|
| `PUBLIC_HOST` | 浏览器访问用的 IP 或域名 |
| `REACT_APP_WS_URL`<br>`REACT_APP_KLINE_WS_URL` | **必须**写成 `ws://<PUBLIC_HOST>`。留空的话前端代码会 fallback 到 `wss://<hostname>`，在没上 TLS 的机器上必然连不上（见 §8-③） |
| `MYSQL_ROOT_PASSWORD` | 自行修改 |
| `HELIUS_API_KEY` | **默认 RPC 服务商是 Helius devnet**，只填 key 即可；`SOL_NODE_URL` / `SOL_WS_URL` / 前端 `REACT_APP_HELIUS_API_KEY` 都由它派生。留空时 `render.sh` 直接报错 |
| `SOL_NODE_URL` / `SOL_WS_URL` | 只有想换别的服务商时才改，显式值优先于 `HELIUS_API_KEY` |
| `CONSUMER_DISABLE_SLOT_SCANNER` | 默认 `true`：关掉全链逐块扫描（几分钟就能耗光 Helius 免费额度），只用 program watcher 索引我们自己的发射合约 |
| `CONSUMER_CONCURRENCY` | 区块拉取并发。devnet 约 2.5 块/秒，1 个 worker 约 0.5 块/秒 → **6** 才追得上；用公共 RPC 被限流就降到 2 |
| `HTTP_PORT` | 前端端口，默认 80 |

> ⚠️ `REACT_APP_*` 是 **CRA 构建期**注入并写死进 bundle 的。改了这些值必须
> `docker compose build frontend && docker compose up -d frontend`，**只重启容器无效**。

---

## 5. 启动

### 5.1 先起中间件

```bash
cd /opt/fun_dex_v2/deploy
docker compose up -d mysql redis kafka
docker compose ps          # 三个都要显示 (healthy) 再往下走
```

### 5.2 构建 + 启动全部

首次构建约 **10–20 分钟**（6 个 Go 镜像 + 1 次 npm 构建）：

```bash
docker compose build
docker compose up -d
docker compose ps
```

内存 < 8 GB 的机器改成串行构建，避免把机器压垮：

```bash
for s in market trade consumer dataflow gateway websocket frontend; do
  docker compose build "$s" || break
done
```

`docker-compose.yml` 已用 `depends_on` + `healthcheck` 保证启动顺序：
`mysql/redis/kafka` healthy → `market` → `trade` / `consumer` / `dataflow` → `gateway` → `frontend`。

**数据库表不用手工导 SQL** —— `consumer` 启动时会跑
`model/migration.EnsureCoreSchema`（`CREATE TABLE IF NOT EXISTS`，幂等），
自动建出全部 14 张表。

实测启动后：

```
SERVICE     STATUS                   PORTS
consumer    Up                       8088/tcp
dataflow    Up                       8084/tcp
frontend    Up                       0.0.0.0:80->80/tcp
gateway     Up                       0.0.0.0:8083->8083/tcp
kafka       Up (healthy)             9092/tcp
market      Up                       8080/tcp
mysql       Up (healthy)             3306/tcp
redis       Up (healthy)             6379/tcp
trade       Up                       8081/tcp
websocket   Up                       0.0.0.0:8086->8086/tcp
```

## 5.3 HTTPS（Let's Encrypt，可直接用 IP）

```bash
cd /opt/fun_dex_v2/deploy
# .env 里设好 TLS_HOST（IP 或域名）、TLS_PROFILE、ACME_EMAIL
./tls.sh        # 首次签发；之后重复执行只在快到期时续期
(crontab -l 2>/dev/null; echo "17 3 * * * /opt/fun_dex_v2/deploy/tls.sh >> /var/log/fundex-tls.log 2>&1") | crontab -
```

- 云厂商安全组要放行 **TCP 443**。:80 只负责 ACME 验证和 301 跳转到 HTTPS。
- **IP 证书**：`TLS_PROFILE=shortlived`，有效期约 6 天，cron 每天检查，剩 2 天时续期。
  lego 必须带 `--disable-cn`（LE 拒绝 Common Name 里放 IP，报 `badCSR`）。
- **域名证书**：`TLS_HOST=<域名>`、`TLS_PROFILE=`（默认 90 天）。
- 有证书之后 `REACT_APP_WS_URL` / `REACT_APP_KLINE_WS_URL` 用 `wss://<TLS_HOST>`，
  这样自托管页面和 Vercel 的 https 页面都能连上实时推送。

---

## 6. 验证（实测结果）

### 6.1 API

```bash
curl -s "http://<HOST>/v1/market/index_pump?chain_id=100000&pump_status=1&page_no=1&page_size=5"
```

应返回：

```json
{"code":10000,"message":"OK","data":{"list":[],"total":0}}
```

> 全新数据库 + consumer 只从**当前链头**开始索引（不回补历史），所以刚起来时 `list` 是空的，
> 这是**正常**的 —— 能返回这个 JSON 就说明 HTTP → gRPC → MySQL 整条链路已经通了。
>
> 本次实测运行 7 分钟后：`block` 表 433 行（区块在持续索引），但 `pair` / `trade` / `token`
> 全是 0 —— 因为这段时间 devnet 上**恰好没有 pump 代币的创建或成交**。devnet 的 pump
> 活跃度本来就低。想立刻看到数据，用 `experiments/devnet_seed.js` 之类的脚本自己造一个代币，
> 或者从前端 Create 页面发一个（见 §6.6 之后的说明）。

三条路径都要是 200：

```bash
curl -o /dev/null -w "%{http_code}\n" "http://<HOST>/"                                    # SPA          → 200
curl -o /dev/null -w "%{http_code}\n" "http://<HOST>/source"                              # SPA fallback → 200
curl -o /dev/null -w "%{http_code}\n" "http://<HOST>/direct-api/v1/market/index_pump?chain_id=100000&pump_status=1&page_no=1&page_size=1"   # → 200
```

### 6.2 数据库自动建表

```bash
docker compose exec -T mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "show tables from fun_dexs;"
```

实测 14 张表：`block`、`clmm_pool_info_v1`、`clmm_pool_info_v2`、`cpmm_pool_info`、`pair`、
`pump_amm_info`、`raydium_pool`、`sol_account`、`sol_token_account`、`token`、`trade`、
`trade_order`、`trade_order_log`、`user_created_assets`。

### 6.3 索引器在跑

```bash
docker compose logs --tail=50 consumer | grep -E "processBlock|kafka"
```

实测：

```
processBlock:480199439 saveTrades tx_size: 43, dur: 1.065s, trade_size: 0
[kafka] send event log to kafka success: web3fun:0:53, 480199439, len(data): 2
```

```bash
docker compose exec -T mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" fun_dexs \
  -e "select count(*) from block;"      # 数字应该持续增长
```

**最关键的健康指标：consumer 有没有跟上链头。** 对比两个 slot：

```bash
# consumer 处理到哪个 slot
docker compose logs --tail=200 consumer | grep -oE "processBlock:[0-9]+" | tail -1
# 链头在哪
curl -s https://api.devnet.solana.com -X POST -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"getSlot"}'
```

实测（运行 7 分钟后）：

```
processBlock:480200471
{"jsonrpc":"2.0","result":480200506,"id":1}
```

差 35 个 slot ≈ 14 秒，属于**贴着链头**跑，正常。
差距如果持续拉大到几千，就要调大 `CONSUMER_CONCURRENCY` 或换 Helius RPC。

> 日志里刷屏的 `decodeTx err:unknown program` 是**正常噪音**——devnet 上绝大多数交易
> （Vote、ComputeBudget、其它项目的程序）本来就不是 pump 指令，解码器跳过它们。**不是错误。**

### 6.4 Kafka 全链路

```bash
docker compose exec -T kafka rpk topic list          # 应看到 web3fun（自动创建）
docker compose logs --tail=20 dataflow
```

实测 dataflow 正在消费：

```
🔔 Kafka message received - Topic: web3fun, Partition: 0, Offset: 75, Key: 480199499
✅ Successfully unmarshaled 0 SOL trade messages
```

`consumer → kafka → dataflow` 三段都通了。

### 6.5 WebSocket

**不要用 `curl` 测 WebSocket**，它做不了完整握手，会误报 400（见 §8-⑤）。用真客户端：

```bash
python3 - <<'EOF'
import socket, base64, os
host, port, path = "<HOST>", 80, "/ws/tokens?chain_id=100000"
key = base64.b64encode(os.urandom(16)).decode()
s = socket.create_connection((host, port), timeout=15)
s.sendall((f"GET {path} HTTP/1.1\r\nHost: {host}\r\nUpgrade: websocket\r\n"
           f"Connection: Upgrade\r\nSec-WebSocket-Key: {key}\r\n"
           f"Sec-WebSocket-Version: 13\r\n\r\n").encode())
print(s.recv(4096).decode(errors="replace")[:500])
EOF
```

实测（经 nginx :80 代理）：

```
HTTP/1.1 101 Switching Protocols
Server: nginx/1.27.5
Connection: upgrade
Upgrade: websocket
Sec-WebSocket-Accept: eY3UDMAN7fiIpok4mo5m0gJm0BA=

{"type":"connection_established","data":{"categories":["new_creation","completing","completed"],"chain_id":100000,"status":"connected"}}
```

`/ws/kline?pair_address=...&chain_id=100000&interval=1m` 同样返回 101。

### 6.6 交易链路

```bash
curl -s -X POST http://<HOST>/v1/trade/create_market_order \
  -H "Content-Type: application/json" \
  -d '{"chain_id":100000,"token_ca":"<某个 devnet pump 代币>","swap_type":1,
       "amount_in":"0.001","user_wallet_address":"<你的钱包>"}'
```

- 返回 `{"code":515,...}` → **路由是通的**（gateway 成功转发到 trade 服务），
  515 只是该代币没有 bonding curve / 参数不合法。
- 返回 `502` / 连接被拒 → gateway 到 trade 的 gRPC 有问题，查 §8-⑦。

> ⚠️ 请求体里**不能带 `slippage` 字段**。`trade.proto` 的 `CreateMarketOrderRequest`
> 没有这个字段，go-zero gateway 会拒绝未知 JSON 字段并返回笼统的 515。

### 6.7 前端 bundle 里的地址烧对了没

```bash
docker compose exec -T frontend grep -o "ws://<HOST>" /usr/share/nginx/html/static/js/*.js | head -3
```

有输出才说明 `REACT_APP_WS_URL` 被正确注入了。没有的话回 §4 改 `.env` 后**重新 build frontend**。

---

## 7. 日常运维

```bash
cd /opt/fun_dex_v2/deploy

docker compose ps                              # 状态
docker compose logs -f --tail=100 gateway      # 跟日志
docker compose restart consumer

# 改了 .env 或 templates/ —— 配置是 bind-mount，不用重新 build
./render.sh && docker compose up -d

# 改了 Go 代码
docker compose build market && docker compose up -d market

# 改了前端代码或任何 REACT_APP_*
docker compose build frontend && docker compose up -d frontend

# 停止（保留数据）
docker compose down

# 彻底清空（含 MySQL/Redis/Kafka 数据卷）
docker compose down -v
```

数据持久化在 3 个 named volume：`fundex_mysql-data`、`fundex_redis-data`、`fundex_kafka-data`。

---

## 8. 踩坑记录（实测遇到的每一个问题）

### ① Redpanda 容器起来就退出，exit 0，**一行日志都没有**

**现象**

```
fundex-kafka-1   Restarting (0) 2 seconds ago
$ docker compose logs kafka
（空）
```

**根因**：写法是

```yaml
entrypoint: ["/bin/bash", "-lc"]
command: >
  exec rpk redpanda start --smp 1 ...
```

Compose 对**字符串形式**的 `command`（包括 YAML 的 `>` 折叠块）会做 **shell 分词**，
`docker compose config` 里能看到它被拆成了数组：

```yaml
command:
  - exec
  - rpk
  - redpanda
  ...
```

于是实际执行的是 `bash -lc "exec"`，`$0="rpk"`，脚本体只有 `exec` 一个词 → 什么都没跑 → 正常退出 0。

**修复**：`command` 必须是**单元素列表**，整条命令是那一个元素：

```yaml
command:
  - >
    exec rpk redpanda start --overprovisioned --smp 1 ...
```

**排查技巧**：容器 exit 0 且无日志时，第一件事就是 `docker compose config <service>`
看看 Compose 最终解析出来的 `entrypoint` / `command` 到底长什么样。

---

### ② Redpanda 必须 `--smp 1`

seastar 是按**宿主机**的 CPU 数来自我调优的，在容器里会看到全部核心并按核数申请 AIO slot，
超出 `/proc/sys/fs/aio-max-nr` 就起不来：

```
Could not initialize seastar: Could not setup Async I/O: Not enough request
capacity in /proc/sys/fs/aio-max-nr.
```

固定加 `--overprovisioned --smp 1 --memory 1G --reserve-memory 0M`。这是 Redpanda 官方的容器配方，不是 workaround。

---

### ③ 前端 WebSocket 连不上（控制台一直重连）

`useTokenListWebSocket.js` 的 fallback 逻辑是：`REACT_APP_WS_URL` 没设 且 hostname 不是
localhost → 用 `wss://<hostname>/ws/tokens`。纯 HTTP 部署上 `wss://` 必然失败。

**修复**：`.env` 里显式设 `REACT_APP_WS_URL=ws://<HOST>`、`REACT_APP_KLINE_WS_URL=ws://<HOST>`，
并让 nginx 把 `/ws/` 反代到 `websocket:8086`。上了 TLS 之后再改回 `wss://`。

---

### ④ 改了 `REACT_APP_*` 却不生效

CRA 是**构建期**把 `process.env.REACT_APP_*` 内联进 bundle 的，不是运行时读取。
`docker compose restart frontend` 没有任何作用，必须
`docker compose build frontend && docker compose up -d frontend`。

验证方法见 §6.7（直接 grep 构建产物）。

---

### ⑤ `curl` 测 WebSocket 返回 400 —— **假警报**

```
$ curl -H "Connection: Upgrade" -H "Upgrade: websocket" ... http://<HOST>:8086/ws/tokens
400
```

curl 并不实现 WebSocket 握手（没有正确的 `Sec-WebSocket-*` 协商），服务端拒绝是对的。
换成真客户端（§6.5 的 Python 脚本）后直连 8086 和经 nginx :80 **都返回 101**。

**别因为 curl 的 400 去改 nginx 配置**，白折腾。

---

### ⑥ 前端同时用了 `/v1/*` 和 `/direct-api/*` 两套前缀

源码里两种写法都有：

- `pages/TokenSource.js`、`pages/TokenDetail.js`、`lib/trade.js` → `fetch('/v1/...')`
- `components/TokenList.js`、`BuyModal.js`、`TradingViewChart.js` … → `fetch('/direct-api/v1/...')`

nginx **两个都要代理**，否则一半页面能用、一半 404：

```nginx
location /v1/         { proxy_pass http://gateway:8083;  }   # 不带尾斜杠 → 路径原样透传
location /direct-api/ { proxy_pass http://gateway:8083/; }   # 带尾斜杠   → 剥掉前缀
```

尾斜杠的有无决定了 nginx 是否剥掉 location 前缀，写反了就会变成 `/v1/v1/...`。

---

### ⑦ gateway 崩溃 → 整站 502

go-zero 的 gateway 在启动时如果拨不通上游 gRPC（即使配了 `nonblock: true` + ProtoSets）
会直接 **fatal 退出**。trade 抖一下就会让 gateway 崩，然后所有接口 502。

仓库的 `Dockerfile.gateway` 已经内置了自愈循环：

```dockerfile
CMD ["sh","-c","while true; do /app/app -f etc/gateway.yaml; sleep 3; done"]
```

**另一个常见根因是端口对不上**：`trade` 实际监听的端口和 `gateway.yaml` 里的
`Target:` 不一致。排查方法 —— 在各服务日志里找 `Starting rpc server at ...`，
跟 gateway 配置逐个对：`market` 必须 **8080**，`trade` 必须 **8081**。

> `market/etc/market.yaml` 历史上曾是 8089，而所有人都按 8080 拨它。本手册的
> `deploy/templates/market.yaml` 已经固定为 8080。

---

### ⑧ 并行构建把机器压垮

`docker compose build` 会**同时**构建 7 个镜像。实测 8 vCPU 机器上：

```
load average: 104.30, 43.95, 16.63
```

跑得完，但内存小的机器可能 OOM 掉编译进程（表现为 `signal: killed`）。
小机器请按 §5.2 改成串行 `for s in ...; do docker compose build "$s"; done`。

---

### ⑨ go-zero 不展开 YAML 里的 `${VAR}`

各服务都是 `conf.MustLoad(*configFile, &c)`，**没有**传 `conf.UseEnv()`，
所以 YAML 里写 `${MYSQL_PASSWORD}` 会被当成字面量字符串，连不上数据库。

只有少数字段带了 `env=` 结构体 tag 才能被环境变量覆盖，例如：

| 字段 | 环境变量 | 备注 |
|---|---|---|
| consumer / trade `Sol.NodeUrl` | `SOL_NODE_URL` | 经 `ApplyEnvOverrides` 折叠（go-zero 不能 env-bind `[]string`） |
| consumer / trade `Sol.NodeUrl`（+ consumer `WSUrl`） | `HELIUS_API_KEY` | 未设 `SOL_NODE_URL` 时默认拼成 Helius devnet 地址 |
| consumer / trade MySQL | `MYSQL_USER` / `MYSQL_PASSWORD` / … | market、dataflow 用的是 gorm-zero 的结构体，**没有** env tag |

所以本方案统一用 `render.sh` + `envsubst` 在部署时渲染，`.env` 作为唯一真相来源。

---

### ⑩ `trade.yaml` 的 `NodeUrl` 不能为空

`Sol.NodeUrl []string` 是必填项，注释掉或留空会让 go-zero 在加载配置时直接崩：

```
Sol.nodeurl EOF
```

模板里保留了一个非空默认值 `https://api.devnet.solana.com`，真正的 RPC 再由
`SOL_NODE_URL` 环境变量覆盖。

---

### ⑪ `websocket/` 是**独立的 Go module**

它有自己的 `go.mod`（module 名就叫 `websocket`），不能和根 module `dex` 一起构建。
`Dockerfile.websocket` 单独 `COPY websocket/go.mod websocket/go.sum` 再在该目录里 build。
删 `websocket/go.sum` 或想把它并进根 module 都会构建失败。

---

### ⑫ consumer 日志刷屏 `unknown program` —— **不是错误**

devnet 上绝大部分交易不是 pump 指令（Vote、ComputeBudget、其它项目），
解码器遇到不认识的程序就报这行然后跳过。属正常噪音。

真正要关注的是：
- `processBlock:<slot> ... trade_size: N` —— N > 0 说明解出了交易
- `[kafka] send event log to kafka success` —— Kafka 投递成功
- slot 号要**持续跟着链头涨**。落后了就调大 `CONSUMER_CONCURRENCY` 或换 Helius RPC。

---

### ⑬ 安全组 ≠ 服务器防火墙

实测服务器上 `ufw` 是 inactive、`iptables -S INPUT` 是 `-P INPUT ACCEPT`，**全放开**，
但外网仍可能连不上 —— 因为阿里云**安全组**才是第一道门，必须在控制台放行 TCP 80。

区分方法：

```bash
# 在服务器上自己 curl 自己
curl -o /dev/null -w "%{http_code}\n" http://127.0.0.1/     # 200 → 服务没问题
# 从外部机器 curl
curl -o /dev/null -w "%{http_code}\n" http://<HOST>/        # 超时 → 安全组没放行
```

---

### ⑭ 已知缺口（**不是部署问题**，是代码本身的遗留）

| 现象 | 原因 |
|---|---|
| `/v1/market/user_tokens`、`/v1/market/user_pools` 返回 404 | 前端 `MyTokens.js` / `MyPools.js` 会调，但 `gateway.yaml` 的 `Mappings` 里**没有**这两条路由 |
| 新代币刚创建时 K 线是空的 | 该代币还没有成交记录；`market` 的 `GetKline` 会退化成对 `trade` 表做 SQL 聚合，没有 trade 就没有 candle。前端会显示 "No trades yet · Starting price $X" 占位 |
| 交易请求带 `slippage` 字段必定 515 | `trade.proto` 的 `CreateMarketOrderRequest` 没有该字段，gateway 拒绝未知 JSON 字段。后端目前把滑点硬编码为 50%，UI 上的滑点控件是**装饰性**的 |

---

## 9. 附：本次部署的实测环境

| 项 | 值 |
|---|---|
| 服务器 | 47.242.157.80（阿里云 ECS 香港） |
| 系统 | Ubuntu 26.04 LTS（codename `resolute`），内核 7.0.0-28 |
| 规格 | 8 vCPU / 14 GB RAM / 40 GB 磁盘 |
| Docker | 29.7.0 |
| Docker Compose | v5.3.1 |
| 代码分支 | `devnet` |
| Solana 网络 | devnet（公共 RPC `https://api.devnet.solana.com`） |
| 首次构建耗时 | 约 15 分钟（7 个镜像并行） |
| 访问地址 | http://47.242.157.80/ |
