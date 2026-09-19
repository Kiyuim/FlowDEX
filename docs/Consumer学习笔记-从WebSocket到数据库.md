# Consumer 学习笔记：从 WebSocket 到数据库

> 简单版问答笔记，按课程大纲整理，每题尽量一两句话讲清楚，涉及代码的地方给出真实文件位置（`fun_dex_v2/consumer`），不做展开分析。

---

## 二、通过 Helius WebSocket 获取最新区块

**WebSocket 是什么？和 HTTP 的区别？**
HTTP 是"一问一答"：客户端发请求，服务端回一次就断开。WebSocket 是先握手建一条长连接，之后服务端可以随时主动往这条连接上推数据，不需要客户端每次都问。

**用什么方法获取 Slot？**
Helius 的 `slotSubscribe` 方法。项目里在 `slot/ws.go:109` 建好连接后发送：
```json
{"id":1,"jsonrpc":"2.0","method":"slotSubscribe"}
```
之后 Helius 就会不断往这条连接推最新 slot。

**什么是 Slot？**
一个约 400ms 的出块时间窗口，可以理解成"发车班次"：每 400ms 一班车，车上可能有乘客（交易），也可能空车。

**Slot 和区块的关系？Slot 里能生成几个区块？**
一个 slot 最多产生 **1 个**区块，也可以 **0 个**（没有区块）。所以 slot 编号总是 ≥ 区块高度。

**什么是空块？为什么会产生？**
空块是"这个 slot 没有普通用户交易，只有验证者的投票交易"（甚至可能完全没有区块）。原因：每个 slot 由一个 leader 节点负责打包，如果它网络延迟、掉线、处理慢，没能在 400ms 内把交易打包进去，这个 slot 就成了空块。

**为什么用 WebSocket，不用 HTTP 轮询？**
Solana 大约 400ms 一个 slot，如果用 HTTP 轮询，要么轮询间隔太密集浪费请求配额，要么太稀疏会错过 slot。WebSocket 是服务端一有新 slot 就主动推，不用猜"该不该问了"。

**为什么主动推送能提高实时性？**
轮询的延迟取决于"你多久问一次"，平均延迟约等于轮询间隔的一半。主动推送没有这个"等下一次轮询"的空档，事件一发生就立刻送达。

**结合业务场景怎么考虑？**
交易平台的核心体验是"数据要接近链上真实状态"，用户看的价格、K线、最新成交如果延迟太久会直接影响交易判断，所以整条链路从获取 slot 开始就要选实时性最好的方式。

---

## 三、生产者协程

**生产者协程怎么创建？**
`slot/group.go:22-25` 里 `SlotServiceGroup.Start()` 调用 `s.Ws.Start()` → `slot/ws.go:26-32` `SlotWsService.Start()` → `slot/ws.go:34-62` `SlotService.SlotWs()`。这整条调用最终跑在 `go-zero` 的 `ServiceGroup` 给每个 service 分配的独立 goroutine 里（框架层面，`sg.Start()` 内部 `routineGroup.Run(func(){ service.Start() })`）。

**Goroutine 和 Channel 在生产者里怎么协作？**
生产者 goroutine 里跑 `for { s.ReadSlotMessage() }`（`slot/ws.go:46-58`），每读到一条消息，就把 slot 号写进 channel：
```go
s.realtimeCh <- resp.Params.Result.Slot   // slot/ws.go:94
```
写 channel 这个动作本身是阻塞的：如果消费者没跟上，这一步会等着，天然形成"背压"（不会无限堆积在内存里）。

---

## 四、消费者协程

**消费者协程怎么创建？**
`consumer/consumer.go:82-86`：
```go
for i := 0; i < c.Consumer.Concurrency; i++ {
    sg.Add(block.NewBlockService(ctx, "block-real", realChan, i))
}
```
每次循环创建一个 `BlockService`，每个都会被 `ServiceGroup` 分配一个独立 goroutine 去跑它的 `Start()`。

**消费者怎么消费消息？**
`block/block.go:86-103` `GetBlockFromHttp`：
```go
for {
    select {
    case <-s.ctx.Done():
        return
    case slot, ok := <-s.slotChan:
        if !ok { return }
        threading.RunSafe(func() { s.ProcessBlock(ctx, int64(slot)) })
    }
}
```
从 channel 里读一个 slot，就去处理一个。

**生产者怎么把消息发给消费者？**
就是同一个 channel：生产者 `s.realtimeCh <- slot`（写），消费者 `slot, ok := <-s.slotChan`（读）——这两个变量名不同，但指向 `consumer.go:76` 创建的同一个 `chan uint64`。

**生产者有几个？消费者创建了多少个？**
生产者 **1 个**（`consumer.go:104` 只创建一次 `SlotServiceGroup`）。消费者数量由配置 `Consumer.Concurrency` 决定（`consumer/etc/consumer.yaml` 当前配的是 `1`），有几个数字就创建几个 `BlockService`。

**创建多个消费者的目的？**
多个消费者可以同时从同一个 channel 抢 slot 来处理，slot 之间互相独立，谁先抢到谁处理，用来提高吞吐——生产 slot 的速度是固定的（Helius 推多快是多快），但处理一个 slot（拉区块+解析+落库）比较慢，多开几个消费者能让"处理"跟上"生产"的速度。

**多消费者情况下，怎么保障数据一致性？**
**这里是当前项目一个明确的短板**：`block` 表有 `slot` 唯一键（`model/migration/core_schema.sql:15`），两个消费者同时处理同一个 slot 时，block 那一行不会重复，但真正存交易的 `trade` 分表**没有唯一键**、`ON CONFLICT` 去重代码也被注释掉了（`model/solmodel/trademodel.go:132-137`），所以目前**没有机制**保证同一笔交易不会被插入两次。当前配置 `Concurrency:1` 只有一个消费者，暂时不会触发这个问题，但代码本身没有防护。

---

## 五、从区块结构里过滤 Pump.fun 的 Buy / Sell

**区块的数据结构是什么？**
```
Block
 └── Transactions[]
      ├── Transaction.Message.AccountKeys[]     // 这笔交易用到的所有账户
      └── Transaction.Message.Instructions[]    // 这笔交易要执行的指令
           ├── ProgramIDIndex  // 下标，指向 AccountKeys 里的某个账户
           ├── Accounts[]      // 下标数组，指向 AccountKeys 里用到的账户
           └── Data            // 指令参数的原始字节
```
对应代码：`block/block.go:182`（遍历 `blockInfo.Transactions`）→ `block/block.go:622`（遍历 `tx.Transaction.Message.Instructions`）。

**怎么解析出 Pump.fun 的交易？**
用 `ProgramIDIndex` 查出这条指令调用的是哪个程序，再跟 Pump.fun 的地址比较：
```go
program := tx.AccountKeys[instruction.ProgramIDIndex].String()   // block/block.go:682
if program == ProgramStrPumpFun { ... }                          // block/block.go:700
```
Pump.fun 的程序地址定义在 `pkg/constants/chain.go:50`：`"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"`。

**怎么解析出 Buy / Sell 这个具体指令？**
Anchor 程序的每个指令，`data` 的**前 8 个字节**是一个固定的"指令编号"（discriminator）。项目里读这 8 个字节转成 `uint64`（小端）再比较：
```go
func GetPumpInstruction(data []byte) uint64 {
    if len(data) < 8 { return 0 }
    return binary.LittleEndian.Uint64(data[:8])   // block/pump.go:66-71
}
// buy/sell 各自对应一个固定数值，block/pump.go:24-27
const (
    PumpInstructionBuy  = 0xeaebda01123d0666
    PumpInstructionSell = 0xad837f01a485e633
)
```

**实践：解析出 Buy/Sell（`fmt.Println`）**
项目代码里其实已经有大量调试用的 `fmt.Println`，可以直接看到解析结果，比如 `block/pump.go:91-92`（打印进入 pump 分支）和 `block/block.go:701-705`（打印识别到的 trade 结果）——运行 consumer 服务、连上真实 Helius WS 之后，控制台就能看到这些输出。

---

## 六、指令里的账户和参数怎么解析

**ATA 账户是什么？**
Associated Token Account，一个钱包持有某个 token 的"专属账户"。Solana 一个钱包地址本身不能直接存 SPL Token，必须为"钱包 + 某个 token"这个组合单独开一个账户，这个账户就是 ATA。

**ATA 账户是怎么生成的？**
它是一个 **PDA（程序派生地址）**，由 `[钱包地址, Token Program ID, Token Mint 地址]` 这三个种子，通过 Associated Token Program 的规则算出来的固定地址——同一个钱包 + 同一个 mint，算出来的 ATA 永远是同一个地址，不需要去链上查。

**ATA、Token Mint、钱包账户的关系？**
钱包（一个身份）+ Token Mint（一种代币）→ 唯一确定一个 ATA（存这个钱包持有的这种代币）。一个钱包如果持有 10 种代币，就对应 10 个不同的 ATA。

> 补充一点：这个 consumer 服务**不需要自己算 ATA**——因为链上交易本身已经把用到的 ATA 地址明明白白放在 `AccountKeys` 里了，consumer 只是"读出来"，不是"算出来"（全仓库搜索 `FindAssociatedTokenAddress` 在 `consumer` 目录下没有结果）。真正需要"算出 ATA"的场景是**构造交易**（比如 `trade` 服务要帮用户下单买入时，需要提前算出/创建 ATA），跟这里的"读区块"场景不一样。

**实践：解析出池子账户 / 用户钱包 / Token Mint / Token 对应的 ATA**
都是通过 `instruction.Accounts` 这个下标数组，去 `accountKeys` 里取值，`block/pump.go:100-103`：
```go
pair         := accountKeys[instruction.Accounts[3]].String() // 池子账户（bonding curve）
to           := accountKeys[instruction.Accounts[4]].String() // 池子的 ATA
tokenAccount := accountKeys[instruction.Accounts[5]].String() // 用户的 ATA
maker        := accountKeys[instruction.Accounts[6]].String() // 用户钱包
```
Token Mint 地址不是从 accounts 下标拿的，是从交易日志里解析出来的（下面会讲）。

**实践：解析出买入数量**
这里容易搞混的一点：买入/卖出的**真实成交数量**不是从 `instruction.Data` 里解析出来的，而是从这笔交易的**日志**里解析出来的。Pump.fun 程序执行完会打一条日志（自我调用产生的"事件"），格式是 `"Program data: ..."`，项目用 base64 解码 + `borsh` 反序列化拿到：
```go
event := events[dtx.PumpEventIndex]        // block/pump.go:122
tokenAddress := event.Mint.String()         // Token Mint，block/pump.go:124
trade.BaseTokenAmount = ...event.SolAmount  // 花了多少 SOL，block/pump.go:160
trade.TokenAmount     = ...event.TokenAmount // 买到多少 Token，block/pump.go:162
```
（`instruction.Data` 里前 8 字节是指令编号，后面理论上有用户下单时的"意图参数"如最大花费，但当前代码没有解析这部分，只用日志里的真实成交结果。）

---

## 七、把解析结果存到数据库

**Block 表字段含义（数据源是什么）**

| 字段 | 含义 | 数据源 |
|---|---|---|
| slot | 这个 block 对应的 slot 号 | channel 传进来的 slot |
| block_height | 链上真实区块高度 | `getBlock` RPC 返回 |
| block_time | 链上真实出块时间 | `getBlock` RPC 返回 |
| status | 1=处理成功，2=失败，3=跳过（空块/查不到） | consumer 自己维护（细节见第八节） |
| sol_price | 这个 block 里反推出的 SOL/USD 价格 | consumer 计算 |
| err_message | 错误信息 | 表里有这一列，但当前代码**没有地方给它赋值** |

**SQL → 模板代码是怎么生成的？**
用的是 `goctl model`（go-zero 官方 model 生成工具），但这个项目**改了它用的模板**——`template/model/` 目录下的模板文件把官方默认的 sqlx 换成了第三方库 `github.com/klen-ygs/gorm-zero`（基于 GORM）。所以生成出来的 `model/solmodel/blockmodel_gen.go` 文件头会写 `// Code generated by goctl. DO NOT EDIT!`，但内部用的是 GORM 的写法。简化流程：
```
model/sql/sol.sql（写 CREATE TABLE）
   → goctl model mysql ddl ...（读 SQL，套用 template/model/ 下的模板）
   → 生成 model/solmodel/blockmodel_gen.go（不能手改）
   → 项目在旁边再写一个 model/solmodel/blockmodel.go（手写，专门加自定义方法）
```

**怎么用生成的增删改查代码写数据？**
生成代码给了一个接口 `BlockModel`，直接 `New` 出来调用方法就行，不用自己拼 SQL：
```go
sc.BlockModel.FindOneBySlot(ctx, slot)   // 查
sc.BlockModel.Insert(ctx, block)         // 写（新记录是 INSERT，已存在的记录是 UPDATE，靠 GORM 的 Save 自动判断）
```

**实践：Block 数据怎么写进表里？**
完整链路：
```
BlockService.ProcessBlock(slot)              block/block.go:105
  → sc.BlockModel.FindOneBySlot(ctx, slot)    block/block.go:120   （先查有没有这条）
  → 拉区块、填字段、设置 status               block/block.go:138-178
  → sc.BlockModel.Insert(ctx, block)          block/block.go:362   （最后统一写库）
       → db.Save(&data)                       model/solmodel/blockmodel_gen.go:66-70
       → MySQL
```

---

## 八、失败区块的兜底恢复

**为什么会有失败的区块？**
最常见的原因是 `getBlock` 这个 RPC 调用失败——节点限流、超时、网络问题，或者这个 slot 本身没有产生区块。代码里：`block/price.go:29-61`，重试最多 10 次、每次间隔 1 秒，还是不行就放弃。

**失败的区块是怎么记录下来的？**
理论设计是：写一个失败状态到 `block` 表，再把这个 slot 塞进一个专门的 channel（`errorCh`）等着被重新处理。**但这里是当前代码明确的一个问题**：
- `getBlock` 失败（非"被跳过"）时，`block/block.go:148` 那一行代码根本**没有**把 `status` 设成"失败"，写进去的是 Go 的**零值 0**，不是设计文档说的 `BlockFailed(2)`。
- 负责扫描失败记录、往 `errorCh` 发送的代码在 `slot/not_complete.go:65`，但 `errorCh` 这个 channel 在创建 `SlotService` 时（`slot/slot.go:35-46`）**从来没有 `make` 过**，是个 nil channel——往 nil channel 发送会**永久卡住**，不会报错也不会崩溃，就是卡在那不动。

**怎么恢复失败的区块？**
项目里确实写了一个 `ResumeBlockService`（`block/resume_block.go`），本意是重新拉一次区块、重新解析、再存一遍。但现状是：
1. 这个 service 从来没被启动过——`consumer.go:98-101` 创建它的代码整段被注释掉了。
2. 就算手动启动，它的函数体里也**没有调用任何数据库写入**，往 Kafka 发送那一行也被注释掉了（`resume_block.go:150`）——算完数据直接扔掉，不落地。

**结论**：失败区块恢复这套设计目前是"看得见、跑不通"——两处关键代码（`errorCh` 没 `make`、消费者被注释）都没接上。

**实践：设计一套简单可行的测试思路——偶数正常 / 奇数失败**

不用改动已有代码，可以在两个天然的位置注入一个"人为让它失败"的开关，专门用来跑这套验证：

1. **`GetSolBlockInfo`（`block/price.go:29`）**：这是唯一一个"getBlock 失败"真实会发生的入口，函数签名里已经带了 `slot uint64`，可以在最前面加一句"如果 `slot%2==1` 就直接返回一个模拟的错误"，这样不需要真的等链上出问题，就能稳定复现"奇数失败、偶数正常"。
2. 跑起来之后要盯的几个地方：
   - **DB**：`block` 表里对应 slot 的 `status` 到底写成了什么（验证是不是真的写成了"失败"，还是又是零值）。
   - **日志**：`processBlock:%v getSolBlockInfo error` 这类 `logx` 输出，确认失败分支真的走到了。
   - **Channel**：`errorCh` 有没有真的收到数据——目前的答案是"收不到，卡住"，这正是要验证并且要修的地方。
   - **恢复后的表**：如果把恢复链路接通了，还要检查同一个 slot 的交易在 `trade` 分表里是不是被插入了两遍（因为分表没有唯一键，重复处理大概率会产生重复数据）。

---

## 九、SOL Price 解析

**为什么公式里要乘 1000？**
是小数位数换算，不是随手写的系数。项目里 wrapped SOL 是 9 位小数（lamports），USDC/USDT 是 6 位小数（`pkg/constants/chain.go:40` `SolDecimal = 9`）；而代码里 `transfer.Amount` 拿到的是链上原始整数（最小单位，没换算过），直接相除会差 `10^(9-6) = 1000` 倍。公式（`block/price.go:159,170,185`）：
```go
solPrice = float64(transferUSD.Amount) / float64(transferSOL.Amount) * 1000
```
本质是：`(USD原始值/10^6) ÷ (SOL原始值/10^9) = USD原始值/SOL原始值 × 1000`，乘 1000 就是把两种代币不同的小数位数拉平，换算成"每 1 个 SOL 值多少美元"这个正确尺度。

**为什么选 Orca、Raydium 等特定 DEX 做价格源？**
代码里是一个白名单（`block/price.go:16-21`）：
```go
var StableCoinSwapDexes = []common.PublicKey{ProgramOrca, ProgramRaydiumConcentratedLiquidity, ProgramMeteoraDLMM, ProgramPhoneNix}
```
Orca（Whirlpool）、Raydium CLMM、Meteora DLMM、Phoenix 都是 Solana 上 SOL/USDC、SOL/USDT 交易量最大的几个 DEX：
- **实时性**：主流交易平台随时有真实成交，不用等半天才有一笔样本。
- **准确性**：池子深度深，一笔正常大小的交易不容易把价格砸偏（参考课程材料里滑点那节：池子越深，同样的交易量对价格影响越小）。

**内部指令和外部指令的结构关系？**
- 外部指令（`tx.Transaction.Message.Instructions`）：用户自己签名直接调用的顶层指令。
- 内部指令（`tx.Meta.InnerInstructions`）：某个外部指令执行过程中，通过 CPI（跨程序调用）触发的一串"影子指令"。Solana 把这些内部指令按"是哪个外部指令触发的"分组，每组有一个 `Index` 字段，指向触发它的那个外部指令在 `Instructions` 数组里的位置（`block/block.go:369-376` `GetInnerInstructionMap` 就是按这个 `Index` 建的 map）。

**为什么代码对外部指令和内部指令分别算一次 SOL 价格？**
因为"DEX 程序在哪一层被调用"有两种情况，取转账指令的方式不一样：
- **DEX 是外部指令**（用户直接调 swap）：真正的两笔代币转账会被记录成这个外部指令对应的**内部指令组**，要按 `Index` 去 `innerInstructionMap` 里查（`price.go:80-87`）。
- **DEX 是内部指令**（比如聚合器/路由合约在中间又调用了 DEX 程序，这属于 CPI 里的 CPI）：swap 指令本身混在某个内部指令组的一串"平铺"列表里，紧跟着它的两笔转账就是同一个列表里往后数的第 1、2 条，要按位置取，不是按 `Index` 查（`price.go:88-96` + `GetInnerInstructionByInner`：往后取 2 条）。

⚠️ 顺带说一个读代码时发现的问题（不是课程问的，但正好能加深理解"这两种取法为什么不能混用"）：外部指令这条分支（`price.go:82`）实际写的是 `innerInstructionMap[instruction.ProgramIDIndex]`——但 `innerInstructionMap` 是按"外部指令在数组里的位置"建索引的，`ProgramIDIndex` 是"程序地址在 `accountKeys` 里的下标"，这是两个完全不同含义的数字。用错误的 key 去查，基本查不中（除非两个数字碰巧相等），这条分支大概率一直在算 0。这个例子正好说明：内部指令的分组用的是"位置"这个概念，一旦拿错索引类型，逻辑就静默失效（不报错，只是查不到东西）。

---

## 十、Token Price 解析

**Token price 的一般解析原理？**
跟 SOL price 是同一个思路：不是查订单簿，而是拿"这一笔真实成交"两边的金额做比值。`block/pump.go:170`：
```go
trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(trade.TokenAmount)).InexactFloat64()
```
花了多少美元 ÷ 买到多少 token，就是这一笔成交反推出的单价。

**Event 事件在合约中有什么作用？**
Anchor 程序可以在执行过程中"自己调用自己"打一条结构化日志（术语叫 self-CPI event），把这次执行的关键结果（比如真实成交量、储备量变化）编码后写进交易日志里。这样外部的索引器（比如这个 `consumer`）不需要重新模拟交易、也不用去猜账户状态变了多少，直接从日志里解码出程序"亲口告诉你"的结果，比自己算更准确、也更省事。

**Pump.fun 的 buy/sell 指令里有没有 event？**
有。识别方式是找日志里以 `"Program data: vdt/007m"` 开头的行（`block/pump.go:34`），取出 base64 内容解码，再用 borsh 反序列化。

**这个 event 具体是什么类型？怎么查？**
类型叫 **`TradeEvent`**。这个仓库自己就带着 pump.fun 的官方 IDL 文件：`pkg/pumpfun/pump/idl.json`，里面 `events`/`types` 字段能直接查到完整定义，不用去外部猜。

**怎么查看某个 event 具体包含哪些数据？**
直接看 `idl.json` 或者配套的 `pkg/pumpfun/pump/idl.md`。`TradeEvent` 真实字段（从 `idl.json` 里解析出来的）一共 21 个，包括：`mint`、`sol_amount`、`token_amount`、`is_buy`、`user`、`timestamp`、`virtual_sol_reserves`、`virtual_token_reserves`、`real_sol_reserves`、`real_token_reserves`、`fee_recipient`、`fee`、`creator`、`creator_fee` 等等。

⚠️ 对比一下就会发现：项目里 `block/types.go:49-59` 的 `PumpEvent` struct 只解码了 `TradeEvent` **最前面 9 个字段**（第一个字段 `Sign uint64` 其实读的是 8 字节的指令 discriminator，不是 `TradeEvent` 真正的业务字段；从 `Mint` 开始往后 8 个字段才对应 `TradeEvent` 的 `mint`~`virtual_token_reserves`），后面的 `real_sol_reserves`/`real_token_reserves` 等字段完全没有解码。这直接导致了一个连锁后果：因为拿不到真实储备量，`pump.go:134-135` 只能用硬编码常量去"估算"：
```go
realTokenReserves := event.VirtualTokenReserves - TokenReservesDiff
realSolReserves := event.VirtualSolReserves - SolReservesDiff
```
这两个常量的来历写在 `block/pump_amm.go:40-44` 的注释里——是作者手动在 Solscan 上查了**某一个具体账户**的快照，算出 `virtual - real` 的差值，然后写死成全局常量，假设所有代币都适用。而实际上 `TradeEvent` 里本来就有现成的 `real_sol_reserves`/`real_token_reserves` 真实值，只是当前代码没有去解码它，所以才需要这样一个"估算"的绕路。

### 补充问题

**pump.fun 的代码是怎么"下载"下来的？**
不是下载的，是**生成**的。`pkg/pumpfun/pump/idl.md` 里记录了完整步骤：
```bash
git clone git@github.com:dreamerinsgp/solana-anchor-go-new.git
cd solana-anchor-go-new
go build
./solana-anchor-go -src=.../pkg/pumpfun/pump/idl.json -pkg=pump -dst=.../pkg/pumpfun/pump/idl/generated/pump
```
用一个第三方工具（`solana-anchor-go`，跟本项目同一个作者维护的分支）读取 pump.fun 程序的 Anchor IDL 文件（`idl.json`，记录了这个程序所有指令/账户/事件的结构），生成对应的 Go 代码。这跟第七节讲的"SQL → goctl → model 代码"是同一种模式，只是这里是"IDL → anchor-go → 合约交互代码"。

**`DecodePumpEvent(dtx.Tx.Meta.LogMessages)` 这段代码效率怎么样？**
函数本身（`block/pump.go:31-56`）对日志数组是一次线性遍历（O(n)），只有匹配到目标前缀的行才会做 base64 解码 + borsh 反序列化这种有开销的操作，单看这个函数设计没问题。但**调用方式**有浪费：`DecodePumpEvent` 是在 `DecodePumpInstruction`（`pump.go:110`）内部调用的，而 `DecodePumpInstruction` 是"每识别到一条 pump 指令就调用一次"（`block/block.go:702`，在 `DecodeTx` 的指令循环里）。如果同一笔交易里有多条 pump 指令（比如一笔交易里连续买了两次），**同一份 `LogMessages` 会被完整重新扫描、重新解码好几遍**——更合理的做法是在整笔交易层面只解析一次日志、把结果缓存下来，供这笔交易里的所有 pump 指令共用，而不是每条指令都重新扫一遍。

---

## 十一、把数据写入 pair 表

**MySQL 唯一键的属性特征？**
- 保证这一列（或几列组合）在整张表里任意两行都不会重复。
- 允许 NULL 出现多次（MySQL 里 NULL 之间不算相等，所以多行 NULL 不会互相冲突）。
- 可以是单列，也可以是"多列组合唯一"（比如 `pair` 表的 `chain_id_address_index (chain_id, address)`，是这两列**放在一起**不能重复，不是分别不能重复）。
- 由数据库存储引擎在写入那一刻就地校验，是原子操作——比应用层自己先 `SELECT` 判断"存不存在"再 `INSERT` 更可靠（后者中间有一个短暂的竞态窗口，两个并发请求可能同时判断"不存在"）。

**唯一键在业务场景里怎么用？**
项目里已经有几个真实例子：
- `block.slot_index UNIQUE(slot)`（`model/migration/core_schema.sql:15`）：防止同一个 slot 被写成两行 block 记录。
- `pair.chain_id_address_index UNIQUE(chain_id, address)`（`core_schema.sql`，见 `pair` 表定义）：防止同一条链上同一个交易对地址被建两条 `pair` 记录。**实际用法**在 `block/pair.go:165-177`：
  ```go
  err = s.sc.PairModel.Insert(ctx, pairAtDB)
  if err != nil {
      if strings.Contains(err.Error(), "Duplicate entry") {
          // 已经存在，转去查那一条已有记录，直接复用，而不是让整个流程报错
          pairAtDB, err = s.sc.PairModel.FindOneByChainIdAddress(ctx, int64(chainId), trade.PairAddr)
          ...
          return pairAtDB, nil
      }
      ...
  }
  ```
  这是"利用唯一键 + 捕获冲突错误"实现幂等写入的真实写法：先按正常流程插入，如果撞了唯一键就转去查已有的那条，而不是让整条链路失败。
- `token` 表也有类似的 `chain_id_address_index`/`chain_id_address_symbol_index`，防止同一个 token 被重复插入。
- 对比一下 `trade.hash_id_index`（见第七节 `Consumer学习笔记-课程对照与代码审计.md`）：DDL 里确实定义了这个唯一键，但**真正写入的是运行时按天建的分表**，分表建表时用的是 Go struct 的 tag（没有声明唯一约束），所以这个唯一键在真实写入路径上**没有生效**——提醒一下："表定义里写了唯一键"不代表"真正接收数据的那张物理表也有这个唯一键"，一定要看运行时实际操作的是哪张表。

### 实践：建一张带唯一键的表，模拟重复插入

不改项目代码，用一段独立的教学 SQL 演示现象（可以在自己本地或测试库上跑一遍）：
```sql
CREATE TABLE demo_unique (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    biz_key VARCHAR(64) NOT NULL,
    UNIQUE KEY uk_biz_key (biz_key)
);

INSERT INTO demo_unique (biz_key) VALUES ('abc');   -- 成功
INSERT INTO demo_unique (biz_key) VALUES ('abc');   -- 报错：
-- ERROR 1062 (23000): Duplicate entry 'abc' for key 'demo_unique.uk_biz_key'
```

项目里实际用到的两种"优雅应对重复"的写法，正好可以对照着理解：

1. **应用层捕获错误再处理**（`pair.go:165-177` 真实在用）：正常 `INSERT`，如果报错信息包含 `"Duplicate entry"`，就转去 `SELECT` 已有的那一行，流程不中断。缺点是"先插后处理"，如果并发极高，理论上还是有一个极短的中间态。
2. **数据库层面直接 upsert**：一条 SQL 搞定，比如 `INSERT ... ON DUPLICATE KEY UPDATE ...`（项目里 `model/trademodel/tradeorderlogmodel.go:74-94` 真实用了这种写法，用在 `trade` 服务的订单日志表上）或者 `INSERT IGNORE`——不用应用层判断错误类型，并发安全性更好，因为判断和写入是数据库同一次操作里原子完成的。

两种写法在这个项目里其实**都能找到真实例子**，只是用在了不同的表上——可以对照着看它们各自的取舍。

---

*本笔记只做简单问答整理，涉及"为什么会重复""为什么恢复链路跑不通"这类问题的完整代码级证据链，见同目录下的 `Consumer学习笔记-课程对照与代码审计.md` 第六节。*
