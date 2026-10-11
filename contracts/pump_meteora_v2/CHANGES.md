# 优化版改动清单（fix → finding）

对照原程序 `programs/pump-meteora/`，逐条列出优化版做了什么、对应哪个发现。
每条都标注了状态：✅ 已实现并编译通过；🧪 已实现但迁移路径（Meteora CPI）本环境无法运行时验证。

程序 ID（优化版）：`241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ`
编译产物：`target/deploy/pump_meteora.so`（522 KB，`cargo-build-sbf` 通过）

---

## 🔴 严重

| 发现 | 改动 | 文件 |
|---|---|---|
| **SEC-01** 初始化抢跑 | ✅ `configure` 首次初始化现在要求 `payer == program_data.upgrade_authority`（新增 `program` + `program_data` 账户），更新路径要求 `config.authority == payer`。抢跑窗口关闭。 | `instructions/admin/configure.rs` |
| **SEC-02** 末笔超额扣款 | ✅ 三重防护：(a) `apply_buy` 末笔分支 `sol_amount = recomputed.min(offered)`，永不超过用户出价；(b) `swap` 新增 `max_sol_cost` 参数，`require!(sol_amount <= max_sol_cost)`；(c) `require!(sol_amount <= buy_amount_applied)`。 | `state/bondingcurve.rs`、`lib.rs`、`instructions/curve/swap.rs` |

## 🟠 高

| 发现 | 改动 | 文件 |
|---|---|---|
| **SEC-03** 费率溢出 DoS | ✅ `Config::validate()` 校验每项费率 ≤ 100%、同路径三项之和 ≤ 100%；`configure` 写盘前调用它。所有费用减法改 `checked_sub().ok_or(ArithmeticError)?`。 | `state/config.rs`、`state/bondingcurve.rs`、`instructions/admin/configure.rs` |
| **SEC-04** 小数位错价 | ✅ `Config::validate()` 与 `create_bonding_curve` 均 `require!(decimals == 6)`；曲线数学的魔数换成具名常量 `TOKEN_SCALE`/`LAMPORT_SCALE`。 | `state/config.rs`、`state/bondingcurve.rs`、`constants.rs`、`instructions/curve/create_bonding_curve.rs` |
| **NEW-01** 迁移参数错位 | ✅ `initial_meteora_token_reserves`、`initial_meteora_sol_amount`、三个 migration 费率在建曲线时**快照进 `BondingCurve`**；`create_pool` 读快照而非现读 config。 | `state/bondingcurve.rs`、`instructions/curve/create_bonding_curve.rs`、`instructions/migration/create_pool.rs` |
| **SEC-05/08** 迁移账户约束松 | 🧪 `create_pool` 对 `token_a_mint`(==QUOTE_MINT)、`token_b_mint`(==curve mint) 加约束；`meteora_program`/`vault_program` 用 `address =` 常量校验。LP 托管的完整收口（把 `payer_pool_lp` 收成 global_vault ATA）**未做**——需按 Meteora 的 init 语义联调后再定，见下方"未完成/需联调"。 | `instructions/migration/create_pool.rs` |
| **SEC-06** 迁移无幂等 | ✅ `BondingCurve.is_migrated` 标记：`create_pool` 开头 `require!(!is_migrated)`，结尾置 `true` 并按实际转出量扣减 `real_sol_reserves`。 | `state/bondingcurve.rs`、`instructions/migration/create_pool.rs` |

## 🟡 中 / 🔵 低

| 发现 | 改动 | 文件 |
|---|---|---|
| **SEC-07** 迁移程序账户未校验 | ✅ `lock_pool` 的 token/ATA/system program 改为类型化 `Program<...>`；meteora/vault program 用 `address =` 校验。 | `instructions/migration/lock_pool.rs`、`create_pool.rs` |
| **SEC-09** `bps_mul` 内部 unwrap | ✅ 改为纯 `?` 链，失败返回 `None`；调用点 `.ok_or(ArithmeticError)?`。 | `utils.rs` |
| **SEC-11** 方向未校验 | ✅ `SwapDirection` 枚举 + `TryFrom<u8>`，非 0/1 直接 `InvalidDirection`。 | `state/bondingcurve.rs` |
| **SEC-12** lock 用陈旧余额 | ✅ 第一次锁 `total/2`，`reload()` 后第二次锁**剩余全额**，不留 LP 尘埃。 | `instructions/migration/lock_pool.rs` |
| **SEC-13** 配置不变量未校验 | ✅ `Config::validate()`：`supply == real + meteora`、`virtual_token >= real_token`、`last_virtual_sol >= initial_virtual_sol`、reserves > 0。 | `state/config.rs` |
| **SEC-14** realloc 只在补款分支 | ✅ `realloc` 移出 `if`，无条件执行。 | `instructions/admin/configure.rs` |
| **NEW-03** payer 无约束 | ✅ `create_pool` 加 `require!(payer == authority)`，迁移的临时账户由迁移权限本人掌控。 | `instructions/migration/create_pool.rs` |
| **BIZ-02** 迁移零事件 | ✅ 新增 `MigrationEvent` + `LpLockedEvent`；所有事件加 `timestamp`。 | `events.rs`、`create_pool.rs`、`lock_pool.rs` |
| **BIZ-03** SwapEvent 字段名错 | ✅ `SwapEvent` 同时携带 `virtual_*` 与 `real_*` 四个字段，各自名副其实。 | `events.rs`、`state/bondingcurve.rs` |
| **BIZ-04** 迁移成本硬编码 | ✅ `2_000_000_00`/`70_000_000` 换成具名常量 `MIGRATION_RENT_RESERVE`/`MIGRATION_FLAT_FEE`。 | `constants.rs`、`create_pool.rs` |
| **QLT-01** 调试日志刷 CU | ✅ 删掉 `create_bonding_curve` 里 8 行 `msg!("{:?}")` 和热路径的 `buy_amount_applied` 日志。 | `create_bonding_curve.rs`、`bondingcurve.rs` |
| **QLT-02** 死代码 | ✅ 删除 `convert_to_float`/`convert_from_float`、遗留浮点常量、`TOKEN_LAUNCH`、`Initialize` 空结构体、`configure` 里没用的 `global_wsol_account`/`native_mint`；`AmountConfig`/未用错误码保留（教学用途）。 | 多处 |
| **QLT-03** size_of → InitSpace | ✅ `BondingCurve` 用 `#[derive(InitSpace)]`，space = `8 + INIT_SPACE`。 | `state/bondingcurve.rs`、`create_bonding_curve.rs` |
| **QLT-06** 常量 from_str | ✅ Meteora/vault/quote 地址用 `pubkey!` 常量，消掉运行时 `Pubkey::from_str().unwrap()`。 | `constants.rs` |

---

## ABI 变化（客户端需同步）

优化版**不是** drop-in 替换，以下指令签名/账户变了：

1. **`configure`** 新增两个账户：`program`、`program_data`（用于 SEC-01 的升级权限校验），并删除了 `global_wsol_account`/`native_mint`/`token_program`/`associated_token_program`（原本为一个未使用的 WSOL ATA 服务）。
2. **`swap`** 新增参数 `max_sol_cost: u64`（买入时的 SOL 上限；卖出忽略）。
3. **`BondingCurve` / `Config`** 布局变化（新增快照字段 / 事件字段）——链下索引器与反序列化需更新。

---

## 未完成 / 需联调（诚实标注）

- **SEC-05 的 LP 托管完整收口**：把 `payer_token_a/b/pool_lp` 收成 `global_vault` 的 ATA 是最彻底的修法，但这依赖 Meteora `initialize_permissionless_constant_product_pool_with_config` 对这些账户 owner/authority 的确切要求。本环境无法起带 Meteora（AMM + vault）的本地链做运行时验证，**贸然改 owner 模型可能破坏 CPI**。因此优化版只做了不破坏语义的收紧（mint 约束、program 地址校验、payer==authority），LP 的完整 PDA 托管留待 devnet 上带真实 Meteora 账户联调后再落地。
- **迁移路径（create_pool / lock_pool）整体**：编译通过，但 Meteora CPI 未做运行时验证（同上原因）。买卖/建曲线/configure 路径的修复是可以本地验证的（见 README）。

## 验证状态

- ✅ **编译**：`cargo-build-sbf` 通过，产物 `target/deploy/pump_meteora.so`（522 KB）。
- ⏳ **运行时再攻击验证**：计划用本地验证机部署优化版、重跑 6 个 attack PoC 证明已被防御，但本次会话中本地验证机的后台进程能力发生退化（进程一起后台即被杀），未能完成。复现命令见 [README.md](./README.md)，可自行运行。
