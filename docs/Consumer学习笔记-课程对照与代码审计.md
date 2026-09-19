# Consumer 学习笔记 —— 课程内容 × 真实代码对照 + 代码审计

> 本笔记是与 AI 助手（Claude Code）的一次代码走读对话整理而成，围绕 `web3_small_group` 课程「获取最新的 slot」「从 Helius 解析交易数据」两节课，对照 `fun_dex_v2/consumer` 服务的真实实现逐条核对。所有代码位置均来自实际读取，未做任何修改。

---

## 一、核心链路总览（真实代码调用链）

```
Helius WebSocket (wss://devnet.helius-rpc.com)      [consumer/etc/consumer.yaml: Sol.WSUrl]
        │  MustConnect() + slotSubscribe write        [slot/ws.go:97,109]
        ▼
gorilla/websocket Conn.ReadMessage()                  [slot/ws.go:72]
        │  json.Unmarshal → SlotResp                   [slot/ws.go:83-88]
        ▼
realtimeCh <- slot   (buffered chan uint64, cap=50)    [slot/ws.go:94 / consumer.go:76]
        ▼
BlockService.GetBlockFromHttp()  <-s.slotChan           [block/block.go:86-103]
        ▼
BlockService.ProcessBlock(ctx, slot)                     [block/block.go:105]
        ▼
GetSolBlockInfoDelay → GetSolBlockInfo → c.GetBlockWithConfig
   (commitment=confirmed, TransactionDetails=Full)        [block/price.go:23-62]
        ▼
slice.ForEach(blockInfo.Transactions) → DecodeTx(tx)       [block/block.go:182 → 601]
        ▼
for i := range tx.Transaction.Message.Instructions          [block/block.go:622]
        ▼
DecodeInstruction: program := AccountKeys[instruction.ProgramIDIndex]  [block/block.go:682]
        │ program == ProgramStrPumpFun (pkg/constants/chain.go:50)
        ▼
DecodePumpInstruction(instruction)                            [block/pump.go:73]
   ├─ GetPumpInstruction: binary.LittleEndian.Uint64(data[:8]) → discriminator  [pump.go:66,70]
   ├─ accounts[3..6] → pair/to/tokenAccount/maker (AccountKeys映射)             [pump.go:100-103]
   └─ DecodePumpEvent(LogMessages) → borsh.Deserialize → PumpEvent{IsBuy,Mint,SolAmount,TokenAmount,...} [pump.go:31-60]
        ▼
trade := *types.TradeWithPair{ Type: Buy/Sell, PairAddr, Maker, TokenAmount, BaseTokenAmount, ... } [pump.go:127-211]
        ▼
ProcessBlock 聚合 trades → group.RunSafe(SaveTrades)              [block/block.go:277-289]
        ▼
SaveTrades → BatchSaveByTrade → BatchSaveTrade                    [block/db.go:64,124,188]
        ▼
solmodel.TradeModel.BatchInsertTrades → gorm CreateInBatches       [model/solmodel/trademodel.go:110-137]
        ▼
                MySQL（按 BlockTime 分表的 trade_YYYYMM 表）
```

---

## 二、逐节课要点对照

### 1. ServiceGroup / Service 接口

- `go-zero/core/service/servicegroup.go`：`Service = Starter(Start()) + Stopper(Stop())`，`sg.Add(s)` 只要求结构体同时实现这两个方法（Go 结构化类型，无需显式 `implements`）。
- `sg.Start()` 内部 `doStart()` 给每个 service 各起一个 goroutine 跑 `Start()`，然后 `Wait()` 阻塞；同时注册 `proc.AddShutdownListener`，收到退出信号后按**添加顺序的倒序**依次调用每个 service 的 `Stop()`（`Add()` 是 `push front`）。
- 本项目里 `slot.SlotServiceGroup`（`slot/group.go`）没有自己写 `Stop()`，是通过**内嵌 `*SlotService`** 把 `SlotService.Stop()`（`slot/slot.go:52`）提升上来满足接口——Go "组合优于继承" 的典型例子。

### 2. slot 与 block 的关系 / commitment 取舍

- 课程材料与代码完全一致：slot 是约 400ms 的出块时间窗口，一个 slot 最多一个 block，可能是空块（只有验证者投票交易）。
- getBlock 调用（`block/price.go:38-41`）用的是 `rpc.CommitmentConfirmed` + `TransactionDetails: Full`，符合课程里"实时交易平台通常用 confirmed，不用 finalized"的取舍原则。

### 3. WebSocket 推 slot、HTTP 拉 block 的"轻推重拉"设计

- 完全对应：`slot/ws.go` 只处理 `uint64` 的 slot 号；`block/price.go` 用 HTTP JSON-RPC `getBlock` 拿完整区块。

### 4. programIdIndex → accounts → data 的三段解析

- `DecodeInstruction`（`block/block.go:682`）：`program := tx.AccountKeys[instruction.ProgramIDIndex].String()`，与课程"用下标找 programId"完全一致。
- discriminator：`GetPumpInstruction`（`block/pump.go:66-71`）读 `data[:8]`，`binary.LittleEndian.Uint64`，与课程一致。
- accounts 映射：`block/pump.go:100-103`
  ```go
  pair         := accountKeys[instruction.Accounts[3]].String() // bonding curve
  to           := accountKeys[instruction.Accounts[4]].String() // associated bonding curve
  tokenAccount := accountKeys[instruction.Accounts[5]].String() // 用户 ATA
  maker        := accountKeys[instruction.Accounts[6]].String() // 用户钱包
  ```
  **与课程第 15.1 节举的例子（accounts[3]=Bonding Curve, [4]=Associated Bonding Curve, [5]=Buyer Token Account, [6]=Buyer Wallet）逐项对上**，说明代码严格按 pump.fun buy/sell 指令的固定账户顺序取值。

### 5. ⚠️ 重要偏差：instruction.data 里的 amount/maxSolCost ≠ 实际成交数量

课程第 12 节说"data 后续参数（amount、maxSolCost）需要按 IDL 顺序解析"，容易让人以为**实际成交的 SOL/Token 数量就是从 instruction.data 解出来的**。但实际代码里：

- `DecodePumpInstruction`（`block/pump.go:73-213`）在读完前 8 字节 discriminator 之后，**再也没有读取 `instruction.Data` 的任何字节**。
- 真正的成交数量来自**另一个独立来源**：交易日志里的自 CPI 事件。
  ```go
  dtx.PumpEvents, err = DecodePumpEvent(dtx.Tx.Meta.LogMessages)   // pump.go:110
  event := events[dtx.PumpEventIndex]
  trade.BaseTokenAmount = decimal.New(int64(event.SolAmount), ...)   // pump.go:160  真实花费的 SOL
  trade.TokenAmount     = decimal.New(int64(event.TokenAmount), ...) // pump.go:162  真实拿到的 token
  ```
  `DecodePumpEvent`（`pump.go:31-56`）扫描 `"Program data: vdt/007m"` 前缀的日志行，base64 解码后用 `near/borsh-go` 反序列化成 `PumpEvent` struct（`block/types.go:49-59`，含 `IsBuy/Mint/SolAmount/TokenAmount/VirtualSolReserves/VirtualTokenReserves`）。

**结论（也正好解释课程实测例子里的现象）**：
- `instruction.data` 里的 `amount` / `maxSolCost` 是用户下单时的**意图/滑点保护上限**（"我最多愿意花多少 SOL"）；
- 事件日志里的 `SolAmount` / `TokenAmount` 才是**链上实际执行结果**；
- 课程给的实测数据 `maxSolCost = 0.014 SOL` vs `actual cost = 0.006960838 SOL` 差距很大，正是因为这两个数字**来自完全不同的两处数据源**，不是滑点导致的微小误差——当前仓库代码没有解析前者（`maxSolCost`），只用了后者（真实成交事件）。这一点课程文字表述容易让人误解成"两者都从 data 里解析"，值得注意区分。

### 6. 多 Pod 重复消费 / 幂等落库

课程第 25 节提到"多个 producer 需要用 Redis SETNX 做 slot 认领，避免重复处理"。实际审计发现**当前仓库完全没有实现这个机制**：

- `consumer/internal/svc/servicecontext.go` 里的 `ServiceContext` **没有 Redis 字段**，全仓库 `consumer` 目录下 grep `redis`/`SETNX` 只在注释和字符串里出现，没有真实连接和加锁代码。
- `consumer.yaml` 里虽然配置了 `Redis:` 段，但 consumer 服务代码根本没有读取它做认领。
- 当前"防重复"完全依赖**只跑一个副本**（隐式约定，没有代码保证）。
- 叠加下面第三节提到的"幂等写入被注释掉"，如果真的水平扩容多个 consumer 实例，会产生：同一 slot 被多个实例各自 `getBlock` 一次（浪费 RPC 配额）+ 同一笔交易被多次 `INSERT`（因为 `ON CONFLICT` upsert 逻辑被注释，见下）。这是两个独立问题叠加后的真实生产风险。

---

## 三、代码审计发现的生产环境缺口（未修改代码，只记录位置）

| 机制 | 现状 | 位置 | 风险 |
|---|---|---|---|
| WS 断线重连 | 部分实现，判断条件过窄 | `slot/ws.go:64-95` 只在错误信息含 `"close"`/`"broken pipe"` 时重连，其余错误直接 return，外层循环会紧接着再次调用可能形成无 sleep 忙循环 | 未覆盖的错误类型下 CPU 占满且不会真正重连 |
| RPC 重试 | 存在，固定间隔无退避 | `block/price.go:29-61` 固定 sleep 1s，最多 10 次 | 限流高峰期容易 10 次都失败 |
| 失败 slot 补偿 | **代码写了但是坏的（nil channel）** | `slot/slot.go:35-46` `errorCh` 字段从未 `make`；`slot/not_complete.go:65` 向其发送会永久阻塞该 goroutine；`consumer.go:93-96` 消费端整段被注释掉 | 失败区块补录形同虚设 |
| slot 连续性 / gap 检测 | 不存在 | `slot/ws.go:89-94` 原样转发 Helius 推来的 slot，不检查连号 | 断线重连期间中间 slot 会静默丢失，没有回补 |
| worker pool | **创建了但没用** | `block/block.go:41,69,78` `ants.NewPool(5)` 赋值给 `workerPool` 字段后全仓库无 `Submit` 调用 | 死代码，真实并发度只来自 `Consumer.Concurrency` 个独立 `BlockService` 实例 |
| backpressure | 存在，隐式 | `consumer.go:76-77` buffered channel cap=50，阻塞发送 | 简单可用，没有更细的丢弃/限流策略 |
| 幂等写入 | **被注释掉了** | `model/solmodel/trademodel.go:132-135` `ON CONFLICT` upsert 整段注释，实际执行普通 `CreateInBatches`（第137行） | 重复处理同一笔交易会插入重复行 |
| 多 Pod 去重 / slot 认领 | 不存在 | 见上节"六" | 水平扩容会重复拉块、重复落库 |
| graceful shutdown | 部分实现 | `consumer.go:35` `proc.SetTimeToForceQuit(30s)`；`slot/ws.go:27-30` shutdown listener；`block/block.go:52-61` `Stop()` | 框架层面有，但没有显式等待"正在进行的 DB 批量写/Kafka 发送"完成 |
| panic recovery | 较全 | `slot/ws.go:65-71`；`block/block.go:98` `threading.RunSafe`；`block/db.go:290-301` 等多处 `defer recover()` | 是链路里做得最好的一项 |
| RPC failover | 只有轮询，无健康检查/熔断 | `svc/servicecontext.go:131-138` `GetSolClient()` 取模轮询；当前 `consumer.yaml` 实际只配了 1 个 `NodeUrl`，轮询无意义 | 节点故障时会持续轮到坏节点 |

---

## 四、思考题回答存档

**1. 服务启动时如何处理历史数据缺失？**
`setStartSlot()`（`slot/ws.go:135-164`）查 DB 里 `FindLastSuccessBlock`，取 `db_slot - 100`（代码自己注释写的"TODO: 要理解这里为什么设置100 冗余"，作者也没写清楚原因）作为补录起点；`setEndSlot()`（`ws.go:166-204`）用 WS 收到的第一条推送作为补录终点；`consumeHistoricalSlots()`（`slot/slot.go:65-88`）把区间内每个 slot 限速（5ms/个）送入 `historicalCh`。

**2. 存量和增量的切换时机如何确定？**
由 `historicalDone`（`chan struct{}`）信号触发：`consumeHistoricalSlots()` 把 `[startSlot, endSlot]` 发送完毕后关闭 channel 并发出信号，等待该信号的 goroutine 才开始无限循环读实时推送（`slot/ws.go:46-58`）。有一个隐藏风险：`setEndSlot()` 只读了一条消息就返回，之后到 `historicalDone` 触发之前，WS 连接持续收到的推送消息没人在读，堆积在 TCP 缓冲区里，补录区间大时有缓冲区被写满、连接被断的风险。

**3. 自动重连的策略和频率如何设计？**
现状：`MustConnect()`（`slot/ws.go:97-120`）固定 1s 间隔重试拨号，重连触发条件只匹配两种错误子串。更合理的设计：指数退避+上限+抖动；用语义化的错误判断代替字符串匹配；重连后做 gap-fill（记录断线前最后收到的 slot，重连后补拉中间缺失的区块）；加心跳/ping-pong 主动探测半开连接。

---

## 五、复述检验（合上文档后应该能讲出来）

一个 Pump.fun buy 事件是怎么从链上解析出来的：WebSocket 收到 slot → `getBlock(slot, commitment=confirmed)` 拿完整区块 → 遍历 `block.Transactions` → 遍历每笔交易 `message.Instructions` → 用 `accountKeys[instruction.ProgramIDIndex]` 判断是不是 pump.fun 程序 → 读 `data[:8]` 小端 uint64 判断 buy/sell discriminator → 用 `accounts[3..6]` 下标从 `accountKeys` 里还原出 bonding curve / ATA / 用户钱包 → 再从交易日志的 "Program data" 事件（borsh 反序列化）里拿到真实成交的 SOL/Token 数量和买卖方向 → 拼成 `types.TradeWithPair` → 批量落库（`gorm CreateInBatches`，目前非幂等）。

---

*本笔记生成时间：与 AI 助手对话整理，对应仓库 `fun_dex_v2` 当前状态。若代码有更新（尤其是 `errorCh`/幂等写入/worker pool 这几处已知缺口被修复），请重新核对后更新本文档。*

---

# 六、consumer 深度审计（数据库持久化 / 失败恢复 / 并发高可用 / 亮点真伪）

> 审计原则：只依据当前仓库真实代码作答；找不到证据一律写"当前仓库未找到实现/无法确认"，不按通用架构猜测"应该有"。每条结论给出文件路径、函数名、关键代码、调用方、被调用方。

## 6.1 第一部分：数据库持久化

### 6.1.1 block 表 schema

真实建表 DDL（**实际会被执行**，不是注释）：`model/migration/core_schema.sql:2-19`（`migration.EnsureCoreSchema(db)` 在 `consumer/internal/svc/servicecontext.go:96` 启动时调用一次，逐条 `CREATE TABLE IF NOT EXISTS` 执行）。`model/sql/sol.sql` 里同一段 DDL 是**注释掉的参考文档**，不会被执行——真正生效的版本在 `core_schema.sql`。

```sql
CREATE TABLE IF NOT EXISTS `block` (
    `id`           bigint          NOT NULL AUTO_INCREMENT,
    `slot`         bigint          NOT NULL DEFAULT '0' COMMENT 'slot',
    `block_height` bigint          NOT NULL DEFAULT '0' COMMENT 'block_height',
    `block_time`   timestamp       NOT NULL COMMENT 'block_time',
    `status`       tinyint         NOT NULL DEFAULT '0' COMMENT '1 processed, 2 failed',
    `sol_price`    decimal(64,18)  NOT NULL DEFAULT '0.000000000000000000',
    `created_at`   timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`   timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at`   timestamp       NULL DEFAULT NULL,
    `err_message`  varchar(1000)   NOT NULL DEFAULT '',
    PRIMARY KEY (`id`),
    UNIQUE KEY `slot_index` (`slot`),
    KEY `block_time_index` (`block_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='block';
```

对应 Go struct（goctl 生成）：`model/solmodel/blockmodel_gen.go:35-46`，字段一一对应，`gorm:"column:xxx"` tag，`DeletedAt` 额外带 `;index`。

⚠️ `status` 字段 DDL 注释只写了 "1 processed, 2 failed"，但代码里还用了第三个值：`pkg/constants/block.go:7` `BlockSkipped = 3`（用于 `getBlock` 返回 "was skipped" 错误时，见 `block/block.go:141-146`）——DDL 注释文档滞后于代码，不算错误但值得注意。

字段来源见 6.5 节表2。

### 6.1.2 goctl 生成方式

- README 只记录了 **RPC 服务**的生成命令（`README.md:154-155`）：
  ```
  goctl rpc protoc market.proto --go_out=./ --go-grpc_out=./ --zrpc_out=./
  ```
  这条命令生成的是 gRPC 代码，**不是** model 代码。
- Model 文件头部有明确标记：`model/solmodel/blockmodel_gen.go:1` / `trademodel_gen.go:1` 均为 `// Code generated by goctl. DO NOT EDIT!`，并 import 了 `github.com/klen-ygs/gorm-zero/gormc`（第三方 goctl model 插件，用 GORM 而非 go-zero 官方 sqlx model）。
- **本仓库 README/Makefile/脚本里没有记录生成 model 用的具体 `goctl model` 命令行**——无 Makefile（`find -iname Makefile*` 结果为空），README 全文只出现一次 goctl，且是 RPC 用法。**结论：goctl model 的具体生成命令当前仓库未找到实现/无法确认**，只能从生成文件头注释反推使用了 goctl model + gorm-zero 插件。

### 6.1.3 BlockModel / TradeModel：生成接口 + 自定义扩展

**BlockModel**（`model/solmodel/blockmodel.go:17-36` + `blockmodel_gen.go:20-28`）：
```go
// 生成部分 blockmodel_gen.go:20-28
type blockModel interface {
    Insert(ctx context.Context, data *Block) error
    FindOne(ctx context.Context, id int64) (*Block, error)
    FindOneBySlot(ctx context.Context, slot int64) (*Block, error)
    Update(ctx context.Context, data *Block) error
    Delete(ctx context.Context, id int64) error
}
// 自定义扩展 blockmodel.go:20-31
type customBlockLogicModel interface {
    WithSession(tx *gorm.DB) BlockModel
    FindOneByNearSlot(ctx context.Context, slot int64) (*Block, error)
    FindLastSuccessBlock(ctx context.Context) (*Block, error)
    FindFirstFailBlock(ctx context.Context) (*Block, error)
    FindProcessingSlots(ctx context.Context, slot int64, limit int) ([]*Block, error)
}
```
`BlockModel = blockModel + customBlockLogicModel`（`blockmodel.go:20-23`）。自定义方法实现见 `blockmodel.go:59-100`，均为普通 `gorm.Model(&Block{}).Where(...)` 查询，**没有新增自定义的写方法**——block 表的写入只用生成的 `Insert`/`Update`（本质都是 `Save`，见下）。

**TradeModel**（`model/solmodel/trademodel.go:23-40` + `trademodel_gen.go:20-28`）：
```go
// 生成部分 trademodel_gen.go:20-28
type tradeModel interface {
    Insert(ctx context.Context, data *Trade) error
    FindOne(ctx context.Context, id int64) (*Trade, error)
    FindOneByHashId(ctx context.Context, hashId string) (*Trade, error)
    Update(ctx context.Context, data *Trade) error
    Delete(ctx context.Context, id int64) error
}
// 自定义扩展 trademodel.go:31-35
type customTradeLogicModel interface {
    WithSession(tx *gorm.DB) TradeModel
    BatchInsertTrades(ctx context.Context, trades []*Trade) error
    GetNativeTokenPrice(ctx context.Context, chainId int64, searchTime time.Time) (float64, error)
}
```
`TradeModel = tradeModel + customTradeLogicModel`（`trademodel.go:26-29`）。**自定义新增的写方法只有 `BatchInsertTrades`**（`trademodel.go:110-149`）。

⚠️ **重要发现**：`Trade.TableName()`（`trademodel_gen.go:65-67`）固定返回 `"trade"`（不分表的原始表名）。goctl 生成的 `Insert`/`FindOne`/`FindOneByHashId`/`Update`/`Delete` 全部通过 `m.conn.Model(&Trade{})` 操作这张**未分表的 `trade` 表**。但真实写入路径（见 6.1.5）只调用自定义的 `BatchInsertTrades`，它内部用 `getShardTableNameByTime` 把表名换成按天分表的 `trade_<year>_<month>_<day>`（例如 `trade_2026_9_9`），完全绕开了未分表的 `trade` 表。全仓库 grep `TradeModel.Insert(` / `TradeModel.FindOneByHashId(` / `TradeModel.FindOne(` / `TradeModel.Update(` / `TradeModel.Delete(` **零命中**——**结论：这五个 goctl 生成的方法在生产链路里是死代码，它们指向的 `trade` 表不会被真实流量写入。**

### 6.1.4 Block 数据写库：完整调用链（从 ProcessBlock 开始）

```
BlockService.ProcessBlock(ctx, slot)                                    block/block.go:105
  ├─ block, err := s.sc.BlockModel.FindOneBySlot(ctx, slot)             block/block.go:120
  │    未找到 → block = &solmodel.Block{Slot: slot}（Id=0）              block/block.go:122-125
  │    找到   → block = 已存在的行（Id>0）                                block/block.go:126
  ├─ ...填充 block.BlockTime / BlockHeight / Status / SolPrice...        block/block.go:156-178
  └─ err = s.sc.BlockModel.Insert(ctx, block)                            block/block.go:362
        └─ defaultBlockModel.Insert(ctx, data)                          model/solmodel/blockmodel_gen.go:66-70
              └─ db.WithContext(ctx).Save(&data)                        blockmodel_gen.go:68
                    GORM Save 语义：Id==0 → INSERT；Id!=0 → UPDATE 全部列
```
调用方：`BlockService.GetBlockFromHttp`（`block.go:98` `threading.RunSafe(func(){ s.ProcessBlock(...) })`）。被调用方：`gorm.DB.Save` → MySQL。

### 6.1.5 Trade 数据写库：完整调用链（从 DecodePumpInstruction/TradeWithPair 开始）

```
DecodePumpInstruction(...) 返回 *types.TradeWithPair                    block/pump.go:73-213
  └─ DecodeInstruction 收集进 trades []*types.TradeWithPair              block/block.go:601-663 (DecodeTx)
       └─ ProcessBlock 汇总所有 tx 的 trades，按 PairAddr 分组 tradeMap  block/block.go:182-222
            └─ group.RunSafe(func(){ s.SaveTrades(ctx, chainId, tradeMap) })   block/block.go:277-279
                 └─ SaveTrades(ctx, chainId, tradeMap)                         block/db.go:64-122
                      └─ 按 pairAddr 并发 group.RunSafe(BatchSaveByTrade)      db.go:102-117
                           └─ BatchSaveByTrade → BatchSaveTrade                db.go:124-132, 188-233
                                └─ tradeDbs := []*solmodel.Trade{...}          db.go:210-212
                                └─ s.sc.TradeModel.BatchInsertTrades(ctx, tradeDbs)  db.go:225
                                     └─ defaultTradeModel.BatchInsertTrades         model/solmodel/trademodel.go:110-149
                                          ├─ table := getShardTableNameByTime("trade", trade.BlockTime)  trademodel.go:118
                                          ├─ createShardTableIfNotExists(table)                          trademodel.go:127
                                          │     └─ conn.Table(table).Migrator().CreateTable(&Trade{})    trademodel.go:100
                                          └─ conn.Table(table).CreateInBatches(group, 1024)               trademodel.go:137
```
调用方：`ProcessBlock` 里的并发 goroutine（`block.go:277-279`）。被调用方：`gorm.DB.CreateInBatches` → MySQL。

### 6.1.6 单条还是 Batch？batch size 多少？

**Trade 是 Batch**：`model/solmodel/trademodel.go:137` `m.conn.WithContext(ctx).Table(table).CreateInBatches(group, 1024).Error`——**batch size = 1024**。

**Block 是单条**：`blockmodel_gen.go:68` 每次 `ProcessBlock` 结束只 `Save` 一个 `*Block`，没有批量写入口。

### 6.1.7 是否使用数据库事务？

全仓库检索 `model/solmodel` 和 `consumer/internal` 目录下的 `.Transaction(func` / `.Begin(`：**零命中**。**结论：Block 写入和 Trade 批量写入都没有用显式数据库事务包裹**，Block 的单条 `Save` 和 Trade 的 `CreateInBatches` 各自是独立的数据库调用，两者之间、以及同一次 `CreateInBatches` 内部跨 batch 之间都不是原子的。（旁证：`model/trademodel/tradeorderlogmodel.go:74-94` 在 `trade` 服务的订单日志表上确实用了 `ON DUPLICATE KEY ... = VALUES(...)`，说明团队在别处会用 upsert 手法，只是没有用在 consumer 的 Block/Trade 写入上。）

### 6.1.8 Block 和 Trade 各有哪些 UNIQUE KEY？

| 表 | UNIQUE KEY | 位置 | 是否对真实写入路径生效 |
|---|---|---|---|
| `block`（不分表，单表） | `slot_index (slot)` | `core_schema.sql:15` | ✅ 生效——block 表由 `EnsureCoreSchema` 一次性以原始 SQL 建好，这个唯一键是物理存在的 |
| `trade`（未分表的旧表） | `hash_id_index (hash_id)` | `core_schema.sql:93` | ⚠️ 生效但**无意义**——这张表没有真实写入路径（见 6.1.3） |
| `trade_<year>_<month>_<day>`（运行时按天分表，真实写入的表） | **无** | 由 `Migrator().CreateTable(&Trade{})`（`trademodel.go:100`）基于 `Trade` struct 的 gorm tag 建表；`trademodel_gen.go:36-56` 里 `Trade` struct 除 `DeletedAt` 的 `;index` 外**没有任何 `uniqueIndex`/`index` tag** | ❌ **不生效**——分表由 GORM AutoMigrate 风格建表，无法继承 `core_schema.sql` 里手写的 `hash_id_index`，所以真正接收数据的分表在 `hash_id` 上没有唯一约束 |

### 6.1.9 是否存在 upsert / ON DUPLICATE KEY / ON CONFLICT / INSERT IGNORE？

- `model/solmodel/trademodel.go:132-135`：`ON CONFLICT` upsert 代码整段被注释掉：
  ```go
  // err := m.conn.WithContext(ctx).Table(table).Clauses(clause.OnConflict{
  //     Columns:   []clause.Column{{Name: "hash_id_index"}},
  //     UpdateAll: true,
  // }).CreateInBatches(group, 1024).Error
  ```
  实际执行的是第 137 行没有 `Clauses` 的普通 `CreateInBatches`。
- `model/solmodel` 和 `consumer/internal` 目录下检索 `INSERT IGNORE`/`ON DUPLICATE`：**零命中**。
- **结论：Block 和 Trade 的写入路径当前都没有任何形式的 upsert/去重子句在生效。**（`INSERT IGNORE`/`ON DUPLICATE KEY` 在别的服务的其他表里存在，见 6.1.7 旁证，但不在这条链路上。）

### 6.1.10 同一个 Slot / Transaction / Instruction 被处理两次，数据库会发生什么？

**Block（有唯一键 + find-then-save，基本幂等，但有竞态窗口）**：
- 顺序重放（先前一次已提交完成）：`FindOneBySlot` 会命中已存在的行（Id>0），`Insert()`→`Save()` 走 UPDATE 分支，只是把同一行的 `status/sol_price/block_time` 等字段重写一遍，**不会产生重复行**。
- 并发重放（两个 goroutine/两个实例同时处理同一 slot，`FindOneBySlot` 都返回 not-found）：两边都构造 `Block{Slot: slot, Id: 0}` 并 `Save()`（→INSERT），后提交的一方会因为 `slot_index` UNIQUE KEY 冲突而在 `blockmodel_gen.go:68` 返回 MySQL 报错；这个错误在 `block/block.go:362-366` 只是 `s.Errorf(...)` 记日志后 `return`，**不重试、不 panic**。

**Trade（无有效唯一键，一定会重复插入）**：
- 因为真实写入的分表 `trade_YYYY_MM_DD` 上没有 `hash_id` 唯一约束（6.1.8），且 upsert 子句被注释（6.1.9），**同一笔 transaction/instruction 被重新处理一次，`BatchInsertTrades` 会原样插入第二条 `hash_id` 相同的新行**，不会报错、不会被拒绝、也不会更新旧行——是纯粹的重复数据。
- 时序上要注意：`SaveTrades`（写 trade）发生在 `ProcessBlock` 里 Block 行最终 `Insert`（`block.go:362`）**之前**（`block.go:277-289` 在前，`block.go:362` 在后），所以即使后面 Block 行因为唯一键冲突写失败，前面的 Trade 早已经批量插入成功——**Block 和 Trade 两张表的写入结果可能不一致**（trade 已重复写入，block 状态更新失败）。

## 6.2 第二部分：失败 Block / Slot

### 6.2.1 相关代码检索

`grep -rniE "failed.?block|failed.?slot|not.?complete|resume|recover|errorCh|failedCh" consumer` 命中：
- `slot/not_complete.go`（整个文件）
- `slot/slot.go:27` `errorCh chan uint64`
- `block/resume_block.go`（整个文件）
- `block/block.go` 里 `constants.BlockFailed`/`BlockSkipped` 的使用（141-150行）

没有 `failedCh` 这个名字，只有 `errorCh`。

### 6.2.2 "Block 处理失败"的代码定义

`block.Status` 只有三种取值（`pkg/constants/block.go:4-7`）：
- `BlockProcessed = 1`：`getBlock` 成功、SOL 价格计算完成（`block/block.go:167`）
- `BlockFailed = 2`：仅在 `resume_block.go:76,99` 里被赋值（**注意：这是死代码路径，见 6.2.9**）；`block/block.go` 的主流程 `ProcessBlock` 里**从未把 `block.Status` 设成 `BlockFailed`**——`getBlock` 失败时（`block.go:140-150`）直接 `Insert` 一个 `Status` 仍是零值（0，既不是 1 也不是 2）的 `block` 记录，然后 `return`。
- `BlockSkipped = 3`：`getBlock` 报 "was skipped" 错误时（`block.go:141-146`）

**结论：`ProcessBlock` 真实主链路里"失败"对应的不是 `BlockFailed(2)`，而是 `status=0`（零值，未赋值）。** `FindFirstFailBlock`（`blockmodel.go:75-82`）和 `FindProcessingSlots`（`blockmodel.go:84-100`）查询条件写的是 `status = constants.BlockFailed`（即 2）——**它们永远查不到 `ProcessBlock` 主流程产生的失败记录（status=0），因为主流程根本不写 2 这个值。** 这是一个真实存在的语义不一致：失败恢复的查询条件（找 status=2）和失败产生的实际写入值（status=0）对不上。

### 6.2.3 getBlock / Parse / DB 写失败分别怎么处理

**getBlock 失败**（`block/block.go:138-151`）：
```go
blockInfo, err := GetSolBlockInfoDelay(s.sc.GetSolClient(), ctx, uint64(slot))
if err != nil || blockInfo == nil {
    if err != nil && strings.Contains(err.Error(), "was skipped") {
        block.Status = constants.BlockSkipped
        _ = s.sc.BlockModel.Insert(ctx, block)
        return
    }
    _ = s.sc.BlockModel.Insert(ctx, block)   // status 仍是零值
    s.Errorf("processBlock:%v getSolBlockInfo error: %v", slot, err)
    return
}
```
写入一条 `status` 为 0 或 3 的 block 行，然后函数直接 `return`——**没有重试、没有推入任何失败队列**。

**Parse（DecodeTx/DecodeInstruction）失败**（`block/block.go:191-195, 630-635`）：
```go
trade, err := DecodeTx(ctx, s.sc, decodeTx)
if err != nil {
    s.Errorf("processBlock:%v decodeTx err:%v, tx:%v", slot, err, decodeTx.TxHash)
    return   // 这条 tx 直接跳过，continue 到下一条 tx
}
```
单笔交易解析失败只跳过这一笔交易（在 `slice.ForEach` 回调里 `return` 相当于 `continue`），**不影响同一 block 里其它交易，也不会让整个 block 被标记失败**——ProcessBlock 后面仍然会正常执行到 `block.Status = constants.BlockProcessed`（`block.go:167`，这一行在 tx 遍历**之前**就已经设置了，所以哪怕全部 tx 解析失败，block 状态照样是 `BlockProcessed`）。

**DB 写失败**：
- Block：`block.go:362-366`，`err = s.sc.BlockModel.Insert(...)`；失败只 `s.Errorf(...)` 记日志，`return`——不重试。
- Trade：`db.go:110-115`（`BatchSaveByTrade` 内）和 `db.go:225-231`（`BatchInsertTrades` 内）都是失败即 `s.Errorf(...)`/返回 error 记日志，**没有重试逻辑**，上层 `group.RunSafe` 也不会因为这个 error 重新调度。

### 6.2.4 失败 Slot 记录在哪

只有一处尝试记录："内存 channel" `errorCh`（`slot/slot.go:27`），**没有写 DB、没有写 Redis、没有其它内存结构**。但见 6.2.5，这个 channel 本身是坏的。

### 6.2.5 errorCh：make 位置、发送方、消费方

- **声明**：`slot/slot.go:27` `errorCh chan uint64 // 失败重试 Slot 队列`
- **make 位置**：`NewSlotService`（`slot/slot.go:35-46`）只 `make` 了 `historicalDone`；**`errorCh` 从未被 `make`，是 nil channel**。
- **发送方**：`slot/not_complete.go:65` `s.errorCh <- uint64(slot.Slot)`——对 nil channel 发送，**这个 goroutine 会永久阻塞**（Go 语义：向 nil channel 发送永远阻塞，不会 panic，也不会超时）。
- **消费方**：`consumer/consumer.go:93-96` 原本要消费 `errChan`（即 `errorCh`）的 10 个 `block.NewBlockService(ctx, "block-error", errChan, i)` **整段被注释掉**，从未 `sg.Add`。**结论：`errorCh` 既没有真正的消费者，发送方还会永久阻塞——这条失败恢复路径是完全断裂的死代码。**

### 6.2.6 失败 Slot 是否持久化？重启后能否恢复？

不持久化。`errorCh` 是纯内存 `chan uint64`（就算它能正常工作，进程重启这个 channel 里的数据也会全部丢失）；DB 里也没有真正记录"失败"（见 6.2.2，主流程根本不写 `BlockFailed=2`）。**结论：当前仓库没有可用的失败 slot 持久化机制，服务重启后无法恢复处理失败的 slot。**

### 6.2.7 Recovery Service/Goroutine 创建与启动链路

```
slot.NewSlotServiceGroup(ctx, realChan, historyChan)     consumer.go:104
  └─ NotCompleted: NewSlotNotCompleteService(slotService)   slot/group.go:18
       (SlotServiceGroup).Start()                            slot/group.go:22-25
         └─ go s.NotCompleted.Start()                        slot/group.go:23
              └─ SlotNotCompleteService.Start()               slot/not_complete.go:20-22
                   └─ s.SlotNotCompleted()                    slot/not_complete.go:24-70
                        └─ 每 5s Tick 一次 FindProcessingSlots(status=BlockFailed)  not_complete.go:49
                        └─ 每找到一条，1s 一次尝试 s.errorCh <- slot                not_complete.go:65（会永久阻塞，见6.2.5）
```
这条 goroutine 会真的启动、真的每 5 秒查一次 DB，但因为 6.2.2 提到的 status 语义不一致（查的是 `BlockFailed=2`，主流程从不写这个值）大概率永远查不到东西；就算查到了，`s.errorCh <- slot` 也会卡死这个 goroutine。

### 6.2.8 Failed Slot → Retry → ProcessBlock → DB 完整调用链

**当前仓库找不到一条真正走通的链路**。理论设计上应该是：
```
SlotNotCompleteService.SlotNotCompleted (查 DB status=BlockFailed)
    → errorCh (nil channel，发送即阻塞)
    → [应该有的] BlockService("block-error", errorCh) 消费者 —— consumer.go:93-96 已注释，不存在
    → ProcessBlock → DB
```
**结论：这条链路在"发送到 errorCh"这一步就断了，"当前仓库未找到实现"。**

### 6.2.9 max retry / interval / backoff / 最终失败后果 / recovery 成功标记

- **getBlock 层面的重试**（唯一真正生效的重试，见 `block/price.go:29-61`）：最多 10 次，固定 1 秒间隔，无指数退避；触发条件是错误信息包含 `"Block not available for slot"` 或 `"limit"`。
- **Slot 级别的 retry（重新调度整个失败 slot）**：不存在——`errorCh` 断链（见上）。
- **最终失败后果**：`getBlock` 重试 10 次仍失败 → `GetSolBlockInfo` 返回 err → `ProcessBlock` 写一条 `status` 为 0（零值）的 Block 行然后 `return`（`block.go:148-150`）——**这条 slot 的数据永久丢失，没有任何后续动作**。
- **recovery 成功后标记 completed/success 的位置**：`当前仓库未找到实现`。`ResumeBlockService.ResumeBlock`（`block/resume_block.go:55-153`）是仓库里唯一带"恢复"字样的处理函数，但通读全函数：
  - 从未调用 `s.sc.BlockModel.Insert`/`Update`（不写 block 表）。
  - 唯一的 Kafka 发送调用被注释掉：`resume_block.go:150` `// s.SendTx(ctx, slot, trades)`。
  - 而且这个 service 本身从未被实例化——`consumer.go:98-101` `NewResumeBlockService(...)` 整段注释。
  - **结论：即使这条"恢复"路径被人工触发，它现在的代码也不会持久化任何东西，纯粹计算完就丢弃。**

### 6.2.10 Recovery 是否可能造成重复 Trade？有没有幂等保护？

`resume_block.go` 内部对同一个 block 内的重复交易做了简单去重：
```go
trades = slice.UniqueByComparator[*types.TradeWithPair](trades, func(item, other *types.TradeWithPair) bool {
    return item.TxHash == other.TxHash
})   // resume_block.go:135-137
```
这只是**同一次内存计算里按 TxHash 去重**，跟数据库层面的幂等完全无关（而且这段代码从不会真正执行到写库那一步，因为压根不写库）。数据库层面的幂等保护见 6.1.8/6.1.9：**没有**。如果这条 recovery 路径将来被修好并接上 `BatchInsertTrades`，只要它和主流程 `ProcessBlock` 处理了同一个 slot，两边都会各自插入一遍 trade（同样因为分表缺唯一键），**会产生重复交易**。

## 6.3 第三部分：测试

### 6.3.1 Block Recovery 相关测试

全仓库 `*_test.go`（排除 vendored SDK 目录）搜索，`consumer` 目录下只有三个测试文件：
- `consumer/internal/logic/mq/consumer_test.go`
- `consumer/internal/logic/mq/producer_test.go`
- `consumer/pkg/token/{token_hold_test.go, holders_test.go, mint_test.go}`

读了前两个文件的内容：**它们不是这个项目 Kafka 配置的连通性测试，而是来自别处教程/文档的 Sarama 使用示例**——`consumer_test.go:16-17` 连的是硬编码地址 `115.159.107.189:9093`、消费组 `my-consumer-group2`、topic `web3fun`；`producer_test.go:17,28,68-69` 连的是 Azure Event Hubs 示例地址 `test-litentry-event.servicebus.windows.net:9093` 和阿里云 Kafka 的硬编码用户名/密码（`alikafka_post-cn-zp54bjj8x004` / 明文密码），跟 `consumer.yaml` 里真实的 `KqSolTrades`（topic `web3fun-kafka`）配置对不上。⚠️ 顺带一提：`producer_test.go:69` 里那个明文密码字符串已经提交进仓库了，如果是真实凭证，建议单独处理（不在本次问的问题范围内，仅作为审计中顺带看到的情况记录）。

**结论：全仓库找不到任何一个测试是针对 block recovery（`SlotNotCompleteService`/`errorCh`/`ResumeBlockService`）写的。当前仓库未找到实现。**

### 6.3.2 是否已存在故障注入逻辑

检索 `inject`/`fault`/`chaos`/`mock.*fail`/`ForceFail` 等模式，在 `consumer` 目录下**零命中**。**结论：当前仓库未找到故障注入实现。**（仅说明缺失，未做任何修改。）

### 6.3.3 最适合注入"偶数成功、奇数失败"测试的位置（只分析，不修改）

按当前函数结构，有两个天然的注入点：

1. **`GetSolBlockInfo`（`block/price.go:29`）**：这是所有失败路径的唯一真实入口（`getBlock` 失败 → `ProcessBlock` 走失败分支）。因为它的签名是 `func(c *client.Client, ctx context.Context, slot uint64) (*client.Block, error)`，且调用方 `GetSolBlockInfoDelay`（`price.go:23`）单纯透传参数，**理论上**可以在这一层按 `slot%2` 决定是否提前返回一个模拟错误，从而驱动 `ProcessBlock` 走到"getBlock 失败"分支（6.2.3），不需要动其它任何函数——但这需要修改函数体本身，我不做，只指出位置。
2. **`BlockService.ProcessBlock`（`block/block.go:105`）入口处**：因为它直接接收 `slot int64` 参数，是 `select { case slot, ok := <-s.slotChan: ...}`（`block.go:92-100`）之后的第一站，理论上可以在函数最开头按 `slot%2` 分支决定是否跳过真实逻辑、直接构造一个失败态的 `Block{Status: constants.BlockFailed}` 并 `Insert`，用来单独测试"失败态 Block 行是否真的能被 `FindProcessingSlots`/`FindFirstFailBlock` 查询到"，绕开对 `getBlock` 真实网络调用的依赖。

两者都只是"位置分析"，代码未做任何改动。

### 6.3.4 验证 recovery 时应该观察什么

- **DB 表**：`block` 表的 `status`/`err_message`/`updated_at` 列（观察是否真的从失败态被更新为 `BlockProcessed`）；分表 `trade_<year>_<month>_<day>` 里对应 slot 的 `hash_id` 是否出现重复行（因为没有唯一键，见 6.1.8，这是判断"是否重复写"的直接方式：`SELECT hash_id, COUNT(*) FROM trade_xxx GROUP BY hash_id HAVING COUNT(*)>1`）。
- **日志**：`processBlock:%v getSolBlockInfo error`、`resumeBlock:%v ...`、`SlotNotCompleted: push slot: %v to err chain`（`not_complete.go:63`）这几条 `logx` 输出，用来确认失败检测/推送确实发生过。
- **Channel**：`errorCh`（`slot.go:27`）目前是 nil，观测不到任何东西流过——这本身就是需要验证/修复的对象。
- **状态**：`s.sc.BlockModel.FindFirstFailBlock`/`FindProcessingSlots` 的查询结果，配合 6.2.2 提到的 status 语义不一致问题一起看。

## 6.4 第四部分：Consumer 高并发和高可用

| 问题 | 真实答案 | 证据 |
|---|---|---|
| Slot Producer 有几个 | **1 个**：`slot.NewSlotServiceGroup` 只创建一次 | `consumer.go:104` |
| Block Consumer 有几个 | **`c.Consumer.Concurrency` 个**（`consumer.yaml` 当前配的是 `Concurrency: 1`），全部竞争同一个 `realChan` | `consumer.go:82-86`（`for i := 0; i < c.Consumer.Concurrency; i++ { sg.Add(block.NewBlockService(ctx,"block-real",realChan,i)) }`）、`consumer/etc/consumer.yaml`（`Consumer: Concurrency: 1`） |
| 真实并发模型 | **多个 `BlockService` 竞争同一个 buffered channel**，不是 ants worker pool | 见下一行 |
| ants.NewPool/Submit 是否真用了 | `ants.NewPool(5)` 在 `block/block.go:69,78` 被创建并赋给 `workerPool` 字段，全仓库 grep `workerPool\.\|ants\.` **只有创建、没有一次 `Submit` 调用**——**死代码** | `block/block.go:41,69,78` |
| channel 类型/容量/生产方/消费方 | `realChan`/`historyChan` 都是 `chan uint64`，容量 50；生产方是 `SlotWsService`（`realtimeCh<-slot` / `historicalCh<-slot`）；消费方是 N 个 `BlockService.GetBlockFromHttp`（只有 `realChan` 真正被消费，`historyChan` 的消费者在 `consumer.go:89-91` 被注释） | `consumer.go:76-77`；`slot/ws.go:94`、`slot/slot.go:77`；`block/block.go:86-103` |
| channel 满时会发生什么 | 生产方（`SlotWsService`/`SlotNotCompleteService`）向 channel 发送时**阻塞**，直到消费方腾出空间——WS 读取协程本身会被这个阻塞拖慢 | Go channel 语义 + `slot/ws.go:94`（无 `select+default`，是纯阻塞发送） |
| WS reconnect | 见第一节 4.1（未变）：只匹配 `"close"`/`"broken pipe"` 两种错误子串触发重连，固定 1s 重试 | `slot/ws.go:64-95, 97-120` |
| RPC retry | `getBlock` 固定 1s、最多 10 次，见 6.2.9 | `block/price.go:29-61` |
| gap detection | 全仓库 `slot` 目录检索不到任何"连号校验"逻辑，`ReadSlotMessage`（`ws.go:64-95`）原样转发收到的 slot | `slot/ws.go:89-94`；**当前仓库未找到实现** |
| historical backfill | **存在**：`setStartSlot`/`setEndSlot`/`consumeHistoricalSlots`（服务启动时补 `[db_last_slot-100, ws_first_slot]` 区间），但只在**启动时**跑一次，跟"运行中途断线后的 gap backfill"是两回事，后者当前仓库未找到实现 | `slot/ws.go:135-204`、`slot/slot.go:65-88` |
| 多 Pod 是否有 Redis SETNX / DB claim / leader election / MQ consumer group | **均未找到**。`consumer/internal/svc/servicecontext.go` 的 `ServiceContext` 结构体没有任何 Redis 字段；`mq/producer.go` 只建了 `sarama.SyncProducer`（生产者），consumer 服务代码里**没有**创建过 `sarama.NewConsumerGroup`（唯一出现 `NewConsumerGroup` 的地方是 `mq/consumer_test.go:24`，一个跟本项目 Kafka 配置不匹配的示例测试，见 6.3.1）；没有 etcd/leader-election 相关 import。**结论：多实例并发防重复，当前仓库未找到任何协调机制**，全靠约定"只跑一个副本" | `consumer/internal/svc/servicecontext.go:26-45`；`consumer/internal/logic/mq/producer.go`；`consumer/consumer.go` 全文 |
| ServiceContext/config/yaml 里 Redis 是否真的被 consumer 代码使用 | **配置存在，代码不用**。`consumer.yaml` 里有 `Redis:` 段（Host/Pass/Type/Key/PingTimeout/Tls），但 `config.Config`（`consumer/internal/config/config.go:17-30`）**没有 `Redis` 字段**去接收它，`ServiceContext` 也没有 Redis client——`consumer.go:37` 那行唯一提到 `c.Redis` 的代码还是被注释掉的日志行 | `consumer/etc/consumer.yaml`（Redis 段）；`consumer/internal/config/config.go:17-30`（无 Redis 字段）；`consumer/consumer.go:37`（注释） |
| RPC pool 是负载均衡还是 failover | **纯轮询负载均衡，不是 failover**：`ServiceContext.GetSolClient()`（`servicecontext.go:131-138`）对 `solClients` 取模轮询，不检查上一次调用是否报错、不跳过坏节点；没有 health check / circuit breaker 代码 | `consumer/internal/svc/servicecontext.go:131-138`；`consumer.yaml` 当前 `Sol.NodeUrl` 实际只配了 1 个地址，轮询在当前配置下没有实际意义 |

## 6.5 第五部分：脑图"项目亮点"真实性审计

⚠️ 这些能力大多不在 `consumer` 服务里，而是分布在 `market`/`trade` 服务——按实际代码位置如实标注，不因为不在 consumer 就归为"未实现"。

| 检索词 | 结论 | 代码证据 |
|---|---|---|
| honeypot / freeze authority（安全检测） | **部分实现**——只有"自建"的 mint/freeze authority 检测，不是接第三方审计服务 | `market/internal/logic/tokensecuritychecklogic.go:24-126`：直接 RPC `GetAccountInfo` 拉 mint 账户原始字节，手动按 SPL Token mint 布局解出 `mintAuthority`/`freezeAuthority`，返回 `FreezeAuthoritySafe`/`MintAuthoritySafe` + 文字 summary。是真实可运行的自研实现。 |
| QuickIntel / GoPlus（第三方审计服务集成） | **当前仓库未找到实现** | `market/market.pb.go` 里有 `AuditInfo.QuickIntel`/`GoPlus`、`Pass.QuickIntel`/`GoPlus` 等 proto 字段（8265-8377行），但全仓库 grep `quick-intel`/`goplus.io`/`gopluslabs` 等真实域名/HTTP 调用**零命中**——这些 proto 字段目前只是"占位的字符串透传字段"，没有代码去请求 QuickIntel/GoPlus 的接口并填充它们。`market/internal/ticker/pumpTicker.go:41,46` 的注释写着"QuickIntel runs in its own goroutine"，但那两个 goroutine（`StartTicker`/`StartNewCreationTicker`）实际跑的是别的逻辑，注释与实现不符。 |
| sell tax / blacklist（token 表字段） | **当前仓库未找到实现（字段存在但无写入）** | `model/migration/core_schema.sql:36-47`（`token` 表）确实建了 `is_honey_scam`/`sell_tax`/`buy_tax`/`is_have_black_list`/`is_can_pause_trade`/`is_can_change_tax` 等列，`market/market.pb.go` 也有对应 proto 字段；但全仓库 grep 这些字段名的**赋值**（`IsHoneyScam =`、`SellTax =`、`IsHaveBlackList =`）**零命中**（排除 `_gen.go`/`.pb.go` 自身的声明）。这些列在当前代码里永远是建表时的默认值（0/false），没有任何采集/审计流程往里写真实数据。 |
| Jito / Helius Sender / priority fee / compute budget | **已实现**，但在 `trade` 服务，不在 `consumer` | `trade/internal/chain/solana/jito.go`：真实的 Jito bundle/tx 提交（`SendViaJito`/`SendViaJitoRetry`/`SendViaJitoBundles`，`jito.go:100-142`）、tip floor 轮询（`updateJitoFloorFee`/`queryJitoRpc`，`jito.go:48-96`）；`trade/internal/chain/solana/gas.go:13-54` `CreateGasAndJitoByGasFee` 组装 `ComputeBudget111111111111111111111111111111` 的 `SetComputeUnitPrice`/`SetComputeUnitLimit` 指令；`trade/internal/svc/servicecontext.go:99` 把 `c.SolConfig.Jito` 真正接进 `TxManager`。**"Helius Sender"字面量**：全仓库 grep 无命中，当前仓库未找到实现（用的是标准 RPC `SendTransactionWithOpts` + Jito 双路径，`txManager.go:558-577`）。 |
| slippage / auto slippage / transaction retry | **已实现**，在 `trade` 服务 | `trade/internal/logic/createmarketorderlogic.go:358-367`：`for tryTimes == 0 || (param.IsAutoSlippage && errors.Is(err, xcode.SlippageLimit) && tryTimes < 3)` 循环，命中滑点错误后把 `param.Slippage` 逐步调到 4500→7000（基点）重试，最多 3 次，日志 `l.Info("AutoSlippageRetry")`；`trade/internal/chain/solana/pump.go`/`raydium.go` 里 `CalcMinAmountOutByPrice`/`CalcPSMinQuoteAmountOut` 等函数把 `Slippage` 真正用于计算 `minAmountOut`。 |

**特别说明**：以上五类"亮点"里，**没有一个属于 `consumer` 服务**——`consumer` 只负责第一部分链路提到的"解析链上数据写库"，token 安全检测在 `market`，MEV/Jito/滑点在 `trade`。如果脑图把它们标成"consumer 的功能"，属于服务归属错位；如果脑图标的是"整个项目的功能"，除 QuickIntel/GoPlus/sell-tax/blacklist 这几项外，其余基本属实。

## 6.6 四张总表

### 表1：功能 | 状态 | 文件/函数 | 真实行为 | 问题

| 功能 | 状态 | 文件/函数 | 真实行为 | 问题 |
|---|---|---|---|---|
| Block 写库 | 已实现 | `block/block.go:362` → `blockmodel_gen.go:66-70` | `FindOneBySlot` 后 `Save`，Id 有值则 UPDATE、无值则 INSERT | 无事务；并发重放同一 slot 会在唯一键上报错但只记日志 |
| Trade 写库 | 已实现 | `block/db.go:225` → `trademodel.go:110-149` | 按天分表，`CreateInBatches(1024)` | 分表无唯一键，重放会插入重复行 |
| goctl model 生成 | 部分可确认 | `blockmodel_gen.go`/`trademodel_gen.go` 文件头 | 用了 goctl + gorm-zero 插件 | 具体生成命令 README/Makefile 未记录，无法确认 |
| Trade 未分表 CRUD（Insert/FindOne/FindOneByHashId/Update/Delete） | 死代码 | `trademodel_gen.go:76-112` | 指向从不写入的 `trade` 表 | 全仓库零调用 |
| ants worker pool | 死代码 | `block/block.go:41,69,78` | 创建了 `ants.Pool` 但从未 `Submit` | 误导性代码，实际并发靠多个 BlockService 实例 |
| Slot 生产/消费 | 已实现 | `slot/ws.go`、`block/block.go:86-103` | 1 个 WS 生产者 + N 个 channel 消费者 | N 默认=1（`Concurrency:1`） |
| WS 断线重连 | 部分实现 | `slot/ws.go:64-120` | 只匹配两种错误子串重连，固定 1s | 未覆盖错误类型下可能忙循环 |
| getBlock 重试 | 部分实现 | `block/price.go:29-61` | 固定 1s、最多 10 次 | 无退避 |
| gap detection | 未找到实现 | — | — | 断线期间漏 slot 无法感知 |
| 启动时历史回补 | 已实现 | `slot/ws.go:135-204` | 补 `[last_success-100, ws_first_slot]` | 只在启动时跑一次，非持续 gap 修复 |
| 失败 slot 重试（errorCh 链路） | 损坏/死代码 | `slot/not_complete.go:65`、`slot/slot.go:27`、`consumer.go:93-96` | `errorCh` 从未 `make`，发送永久阻塞；消费者代码被注释 | 完全断链 |
| Block 失败恢复（ResumeBlockService） | 死代码 | `block/resume_block.go`、`consumer.go:98-101` | 从未实例化；即便手动调用也不写库、Kafka 发送被注释 | 计算完全部丢弃 |
| 幂等写入（ON CONFLICT） | 已注释/未生效 | `trademodel.go:132-135` | 被注释，实际走普通 insert | 重复写入无保护 |
| 多 Pod 协调（Redis/MQ consumer group/leader election） | 未找到实现 | — | consumer 只用 Kafka 单向生产，无消费组、无 Redis 客户端 | 水平扩容会重复处理 |
| RPC 节点池 | 部分实现（负载均衡非failover） | `svc/servicecontext.go:131-138` | 取模轮询 | 无健康检查/熔断；当前只配 1 个节点 |
| token 安全检测（自建 freeze/mint authority） | 已实现（market 服务） | `market/internal/logic/tokensecuritychecklogic.go` | 手动解析 mint 账户字节 | 不在 consumer |
| QuickIntel/GoPlus 集成 | 未找到实现 | `market/market.pb.go`（仅 proto 字段） | 无真实 HTTP 调用 | 只是占位字段 |
| sell tax/blacklist 采集 | 未找到实现 | `core_schema.sql:36-47`（仅建表） | 字段永远是默认值 | 无采集流程 |
| Jito/priority fee/compute budget | 已实现（trade 服务） | `trade/internal/chain/solana/jito.go`、`gas.go` | 真实 bundle 提交+tip轮询+CU指令 | 不在 consumer |
| slippage/auto slippage/tx retry | 已实现（trade 服务） | `trade/internal/logic/createmarketorderlogic.go:358-367` | 命中滑点错误后自动放宽滑点重试≤3次 | 不在 consumer |

### 表2：数据库字段 | 数据来源 | 写入位置 | 是否必需

**block 表**

| 字段 | 数据来源 | 写入位置 | 是否必需 |
|---|---|---|---|
| slot | channel 传入的 slot 号（Helius WS 或存量补录） | `block/block.go:111` `s.slot = uint64(slot)`；`block.go:123` `Block{Slot: slot}` | 必需（唯一键） |
| block_height | `getBlock` RPC 响应 | `block.go:164-166` `block.BlockHeight = *blockInfo.BlockHeight` | 依赖 RPC 是否返回 |
| block_time | `getBlock` RPC 响应 | `block.go:156-157` `block.BlockTime = *blockInfo.BlockTime` | 依赖 RPC 是否返回 |
| status | Indexer 自己维护（枚举值），但主流程实际写入的是**零值**而非 `BlockFailed` | `block.go:142`(Skipped)/`167`(Processed)；失败分支未显式赋值（见6.2.2） | 必需，但当前语义不一致 |
| sol_price | 程序计算（`GetBlockSolPrice` 用块内稳定币↔SOL 转账反推价格） | `block.go:171-178` | 必需但可能为 0（无稳定币成交时退化取历史值） |
| err_message | **当前仓库未找到写入**——DDL 有这一列，`ProcessBlock`/`resume_block.go` 都没有对 `block.ErrMessage` 赋值 | — | 列存在但未使用 |
| created_at/updated_at/deleted_at | GORM 自动维护（`autoCreateTime`/`autoUpdateTime` 语义 + 软删除列） | 无需手写 | 框架维护 |

**trade 表（分表 `trade_<year>_<month>_<day>`）**

| 字段 | 数据来源 | 写入位置 | 是否必需 |
|---|---|---|---|
| hash_id | 程序计算 `fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)` | `block/pump.go:173` | 必需（业务去重键，但物理无唯一约束，见6.1.8） |
| tx_hash | 链上真实签名，`base58.Encode(tx.Transaction.Signatures[0])` | `block/block.go:606` | 必需 |
| maker/to/pair_addr | instruction accounts 下标映射（`accountKeys[instruction.Accounts[i]]`） | `block/pump.go:100-103,131-132` | 必需 |
| trade_type(buy/sell) | 事件日志 `event.IsBuy` | `block/pump.go:154-158` | 必需 |
| base_token_amount/token_amount | 事件日志 `event.SolAmount`/`event.TokenAmount`（borsh 反序列化，非 instruction.data） | `block/pump.go:160-162` | 必需 |
| base_token_price_usd/total_usd/token_price_usd | 程序计算（用 `solPrice` 和上面的成交量算） | `block/pump.go:163-170` | 必需 |
| block_num/block_time/block_time_stamp | 来自当前处理的 `blockDb`（即 block 表那条记录） | `block/pump.go:171-172` | 必需 |
| swap_name | 配置/常量（`constants.PumpFun` 等硬编码字符串） | `block/pump.go:177` | 必需 |

### 表3：失败场景 | 当前处理 | 是否会丢数据 | 是否会重复 | 恢复入口

| 失败场景 | 当前处理 | 是否会丢数据 | 是否会重复 | 恢复入口 |
|---|---|---|---|---|
| getBlock 连续失败（重试10次仍失败） | 写一条 status=0 的 block 行，`return` | **会丢**——这个 slot 的交易永久不会被解析 | 不会重复（这个 slot 本来就没产生 trade） | 当前仓库未找到实现（`errorCh` 断链，`ResumeBlockService` 未接线也不写库） |
| 单笔交易 DecodeTx/DecodeInstruction 解析报错 | 跳过这一笔，其余交易继续 | 会丢这一笔交易 | 不适用 | 当前仓库未找到实现 |
| Block 行写入失败（如唯一键冲突） | 记日志 `return`，不重试 | 该次状态更新丢失（但因为 find-then-save，行本身可能已存在） | 不会重复行（唯一键生效） | 当前仓库未找到实现 |
| Trade 批量写入失败/被重复处理 | 记日志返回 error，不重试；若是"同 slot 被处理两次"则会插入 | 单次失败会丢这批 trade | **会重复**（分表无唯一键 + upsert 已注释） | 当前仓库未找到实现 |
| WS 连接断开（错误信息含 close/broken pipe） | 自动重连（固定1s），重连后重新 `slotSubscribe` | 断线期间的 slot 会丢（无 gap detection/backfill） | 不适用 | `MustConnect`（`slot/ws.go:97`），仅覆盖两种错误 |
| WS 连接断开（其它错误类型） | 不触发重连，外层循环可能忙转 | 会丢 | 不适用 | 当前仓库未找到实现（错误类型未覆盖） |
| 进程崩溃重启 | `setStartSlot`/`consumeHistoricalSlots` 从"上次成功 slot - 100"开始补 | 缺口在 100 slot 冗余范围内可补，超出范围会丢 | **会重复**——补录区间内的 slot 会被 `ProcessBlock` 重新跑一遍，Trade 无唯一键保护 | `slot/ws.go:135-164`（`setStartSlot`） |

### 表4：脑图项目亮点 | 已实现/部分/未找到 | 代码证据

| 脑图项目亮点 | 状态 | 代码证据 |
|---|---|---|
| Helius WS 获取最新 slot | 已实现 | `slot/ws.go:97-120`（consumer 服务） |
| getBlock 解析交易/账户/指令 | 已实现 | `block/block.go:601-663`、`block/pump.go`（consumer 服务） |
| Pump.fun buy/sell 事件解析 | 已实现 | `block/pump.go:73-213`（consumer 服务） |
| 失败 slot 自动恢复 | 未找到实现（代码存在但断链/死代码） | `slot/not_complete.go`、`block/resume_block.go`，见 6.2 全节 |
| 幂等落库/防重复 | 未找到实现 | `trademodel.go:132-137`，见 6.1.9 |
| 多实例水平扩容防重复 | 未找到实现 | 见 6.4 表格"多 Pod" 一行 |
| Token 安全检测（freeze/mint authority） | 已实现（market 服务，自建非第三方） | `market/internal/logic/tokensecuritychecklogic.go` |
| QuickIntel/GoPlus 第三方审计接入 | 未找到实现 | `market/market.pb.go` 仅占位字段 |
| Sell tax/黑名单检测 | 未找到实现 | `core_schema.sql` 仅建表，无采集代码 |
| Jito MEV 保护/优先费/Compute Budget | 已实现（trade 服务） | `trade/internal/chain/solana/jito.go`、`gas.go` |
| 自动滑点重试 | 已实现（trade 服务） | `trade/internal/logic/createmarketorderlogic.go:358-367` |

## 6.7 当前 consumer 真实调用链（含失败/恢复分支）

```
slotSubscribe (slot/ws.go:97,109)
    ↓ ReadMessage + json.Unmarshal (slot/ws.go:72-88)
    ↓
slot → realtimeCh (chan uint64, cap 50)  (slot/ws.go:94)
    ↓
BlockService.GetBlockFromHttp 消费 (block/block.go:86-103)
    ↓
ProcessBlock(ctx, slot) (block/block.go:105)
    ↓
GetSolBlockInfo/getBlock (commitment=confirmed) (block/price.go:29-61)
    │
    ├─ 失败(10次重试后) → block.Status=0/Skipped(3) → BlockModel.Insert → DB
    │        ↓
    │   [理论恢复入口] errorCh (slot/slot.go:27, 从未 make，死链)
    │        ↓
    │   [理论消费者] consumer.go:93-96（已注释，不存在）
    │        ↓
    │   ✗ 当前仓库未找到能真正跑通的恢复路径
    │
    └─ 成功 → DecodeTx (block/block.go:601)
                ↓ for instruction in Message.Instructions (block.go:622)
                ↓
              DecodeInstruction: accountKeys[ProgramIDIndex]==PumpFun (block.go:682)
                ↓
              DecodePumpInstruction: discriminator(data[:8]) + accounts映射 + 事件日志(borsh) (pump.go:73-213)
                ↓
              *types.TradeWithPair
                ↓
              ProcessBlock 汇总 trades → group.RunSafe(SaveTrades) (block.go:277-289)
                ↓
              SaveTrades → BatchSaveByTrade → BatchSaveTrade (db.go:64-233)
                ↓
              TradeModel.BatchInsertTrades → 按天分表 → CreateInBatches(1024) (trademodel.go:110-149)
                ↓
              MySQL trade_<year>_<month>_<day>（无唯一键，重放会重复）
                ↓
              (并行) BlockModel.Insert(block) → Save (block.go:362 → blockmodel_gen.go:66-70)
                ↓
              MySQL block（有 slot 唯一键，重放会 UPDATE 或报唯一键冲突后被吞掉）
```

---

*本节（第六节）为针对"数据库持久化 / 失败恢复 / 高并发高可用 / 项目亮点真实性"的专项代码审计，方法论：只信代码，不信注释/配置/文档里"应该有"的描述，找不到证据一律标注"当前仓库未找到实现"。未修改仓库任何代码。*
