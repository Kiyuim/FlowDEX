# Frontend-Backend Integration Errors

## 背景

在配置 PM2 管理后端服务（market, trade, consumer, gateway, websocket）和前端服务（pump-tokens-ui）的过程中，以及配置 Nginx 反向代理和 WebSocket 连接时，遇到了多个集成问题。这些问题涉及服务管理、网络配置、API 路径、WebSocket 连接和防火墙设置。

---

## 错误1：PM2 未安装

**背景**：尝试使用 PM2 管理后端服务时，发现 PM2 命令不存在。

**错误**：
```
Command 'pm2' not found, did you mean: ...
```

**原因**：系统未安装 PM2 进程管理器。

**方案**：
1. 使用 npm 全局安装 PM2：`sudo npm install -g pm2`
2. 验证安装：`pm2 --version`
3. PM2 会自动创建守护进程并初始化

---

## 错误2：WebSocket 服务端口被占用

**背景**：通过 PM2 启动 WebSocket 服务时，服务一直处于错误状态，无法正常启动。

**错误**：
```
Failed to start server: listen tcp 0.0.0.0:8086: bind: address already in use
```

**原因**：端口 8086 已被手动启动的进程占用（PID: 1739720），导致 PM2 无法绑定该端口。

**方案**：
1. 查找占用端口的进程：`ss -tlnp | grep 8086`
2. 停止占用端口的进程：`kill 1739720 1739448 1739447`
3. 通过 PM2 重新启动 WebSocket 服务：`pm2 start ecosystem.config.js --only websocket`
4. 验证服务状态：`pm2 status | grep websocket`

---

## 错误3：前端 WebSocket 连接 Mixed Content 错误

**背景**：通过 HTTPS（https://web3ite.cab）访问前端时，WebSocket 连接失败。

**错误**：
```
Mixed Content: The page at 'https://web3ite.cab/' was loaded over HTTPS, 
but attempted to connect to the insecure WebSocket endpoint 'ws://118.194.235.63:8086/ws/tokens?chain_id=100000'. 
This request has been blocked; this endpoint must be available over WSS.
```

**原因**：
- HTTPS 页面不能连接非加密的 WebSocket（ws://）
- 浏览器安全策略阻止混合内容（HTTPS 页面中的非加密资源）
- 需要使用加密的 WebSocket（wss://）或通过 Nginx 代理

**方案**：
1. 更新前端 WebSocket 连接逻辑，根据页面协议选择正确的 WebSocket 协议：
   - HTTPS 页面：使用 `wss://web3ite.cab/direct-api/ws/tokens`
   - localhost：使用 `ws://localhost:8086/ws/tokens`
2. 配置 Nginx 反向代理 WebSocket，将 `wss://web3ite.cab/direct-api/ws/*` 代理到 `ws://localhost:8086/ws/*`
3. 在 Nginx 配置中添加 WebSocket 支持的头信息（Upgrade, Connection）

---

## 错误4：Nginx WebSocket 代理配置缺失

**背景**：前端尝试通过 `wss://web3ite.cab/direct-api/ws/tokens` 连接 WebSocket，但连接失败。

**错误**：WebSocket 连接无法建立，返回 404 或连接被拒绝。

**原因**：Nginx 配置中缺少 `/direct-api/ws/` 路径的 WebSocket 代理配置。

**方案**：
1. 在 Nginx 配置中添加 WebSocket 代理块：
```nginx
location /direct-api/ws/ {
    proxy_pass http://localhost:8086/ws/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 86400;
    proxy_send_timeout 86400;
}
```
2. 重新加载 Nginx 配置：`sudo nginx -t && sudo systemctl reload nginx`

---

## 错误5：API 请求返回 HTML 而不是 JSON

**背景**：前端 API 请求（如获取 token 列表、创建交易订单）返回 HTML 页面而不是 JSON 数据。

**错误**：
```
Failed to fetch tokens: Failed to execute 'json' on 'Response': 
Unexpected token '<', "<!doctype "... is not valid JSON
```

**原因**：
1. Nginx API 代理配置中 `proxy_pass` 路径不正确
2. 前端组件使用了错误的 API 基础路径（`/api` 而不是 `/direct-api`）
3. 路径不匹配导致请求被转发到前端静态文件服务器，返回 HTML

**方案**：
1. 修复 Nginx API 代理配置：
   - 将 `proxy_pass http://localhost:8083;` 改为 `proxy_pass http://localhost:8083/;`（注意末尾斜杠）
   - 确保 `/direct-api/` 路径正确代理到 gateway 服务
2. 更新所有前端组件的 API 基础路径：
   - `TokenList.js`: `/api` → `/direct-api`
   - `BuyModal.js`: `/api` → `/direct-api`
   - `TokenSecurity.js`: `/v1/market/...` → `/direct-api/v1/market/...`
   - `TradingViewChart.js`: `/api` → `/direct-api`
   - `AddLiquidityHeader.js`: `/api` → `/direct-api`
   - `PoolCreation.js`: `/api/v1` → `/direct-api/v1`
   - `AddLiquidity.js`: `/api` → `/direct-api`
3. 重新构建前端：`CI=false npm run build`
4. 重启前端服务：`pm2 restart pump-tokens-ui`

---

## 错误6：防火墙端口 8086 未开放

**背景**：从外部访问 WebSocket 服务时，连接失败。

**错误**：WebSocket 连接超时或被拒绝。

**原因**：UFW 防火墙未允许端口 8086 的入站连接。

**方案**：
1. 检查防火墙状态：`sudo ufw status`
2. 添加端口 8086 的防火墙规则：`sudo ufw allow 8086/tcp`
3. 验证规则已添加：`sudo ufw status | grep 8086`
4. 注意：如果仍然无法连接，可能需要检查云服务商的安全组规则

---

## 错误7：WebSocket 连接使用错误的服务器 IP

**背景**：前端尝试连接 WebSocket 时，使用了内网 IP 而不是公网 IP。

**错误**：WebSocket 连接失败，无法从外部访问。

**原因**：
- 代码中硬编码了内网 IP（10.35.8.99）
- 应该使用公网 IP（118.194.235.63）以便外部访问

**方案**：
1. 获取服务器公网 IP：`hostname -I` 或询问服务器管理员
2. 更新前端 WebSocket 连接逻辑中的 `SERVER_IP` 常量
3. 重新构建前端并重启服务

---

## 错误8：交易 API 返回错误码 515（FDV 为 0）

**背景**：尝试创建市价交易订单时，后端返回错误码 515。

**错误**：
```
API Response: {code: 515, message: 'An error occurred, please try again', data: {}}
Backend log: "3333 pairInfo.Fdv is 0"
```

**原因**：
- 错误码 515 是 `xcode.InternalError`，表示内部错误
- 实际原因是 `pairInfo.Fdv == 0`（Fully Diluted Valuation 为 0）
- 这是数据问题，不是代码问题：
  - Token 在数据库中的 FDV 值为 0
  - 可能是新 token，数据尚未同步
  - Consumer 服务可能未更新该 token 的市值数据

**方案**：
1. **临时方案**：等待 Consumer 服务同步数据，或手动更新数据库中的 token FDV 值
2. **根本方案**：
   - 检查 Consumer 服务是否正常运行：`pm2 status | grep consumer`
   - 检查 Consumer 服务日志，确认是否在同步链上数据
   - 检查数据库中该 token 的 pair 信息是否完整
   - 如果是新 token，等待 Consumer 服务完成数据同步后再尝试交易
3. **代码改进建议**：
   - 在返回错误时提供更详细的错误信息，而不是通用的内部错误
   - 区分数据问题和系统错误，返回不同的错误码

---

## 错误9：前端 API 路径配置不一致

**背景**：多个前端组件使用了不同的 API 基础路径，导致部分 API 请求失败。

**错误**：部分组件能正常工作，部分组件返回 HTML 错误。

**原因**：
- 不同组件使用了不同的 API 路径配置：
  - 有些使用 `/api`
  - 有些使用 `/v1/market/...`
  - 有些使用环境变量判断
- 生产环境应该统一使用 `/direct-api` 路径

**方案**：
1. 统一所有组件的 API 基础路径配置：
   - 开发环境：使用空字符串或相对路径（通过 craco proxy）
   - 生产环境：统一使用 `/direct-api`
2. 创建统一的 API 配置常量文件，避免硬编码
3. 批量更新所有组件：
   ```javascript
   const API_URL = process.env.NODE_ENV === 'development' 
     ? '' 
     : '/direct-api';
   ```
4. 重新构建并部署前端

---

## 总结

### 主要问题类别

1. **服务管理问题**：PM2 未安装、端口占用
2. **网络配置问题**：防火墙规则、Nginx 代理配置
3. **协议兼容性问题**：HTTPS 与 WebSocket 协议不匹配
4. **路径配置问题**：API 路径不一致、代理路径错误
5. **数据问题**：数据库中的 token 信息不完整

### 最佳实践

1. **统一配置管理**：使用环境变量或配置文件统一管理 API 路径
2. **错误处理**：提供详细的错误信息，区分数据错误和系统错误
3. **服务监控**：使用 PM2 监控服务状态，及时发现问题
4. **日志记录**：在关键位置添加日志，便于问题排查
5. **配置验证**：在部署前验证 Nginx 配置、防火墙规则等

### 配置检查清单

- [ ] PM2 已安装并运行
- [ ] 所有服务端口未被占用
- [ ] 防火墙规则已配置（端口 8086, 8083, 3001, 443）
- [ ] Nginx 配置正确（WebSocket 代理、API 代理）
- [ ] 前端 API 路径统一为 `/direct-api`
- [ ] WebSocket 连接根据协议使用正确的 URL（ws:// 或 wss://）
- [ ] Consumer 服务正常运行，数据同步正常
- [ ] 数据库中的 token 和 pair 信息完整

---

## 相关文件

- PM2 配置：`/home/ubuntu/dex_full/fun_dex_v2/ecosystem.config.js`
- Nginx 配置：`/etc/nginx/sites-available/web3ite.cab`
- WebSocket Hook：`/home/ubuntu/dex_full/fun_dex_v2/pump-tokens-ui/src/hooks/useTokenListWebSocket.js`
- 交易逻辑：`/home/ubuntu/dex_full/fun_dex_v2/trade/internal/logic/createmarketorderlogic.go`
- Gateway 错误处理：`/home/ubuntu/dex_full/fun_dex_v2/gateway/middleware/response.go`
