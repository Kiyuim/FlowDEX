# pump-meteora — 安全加固版（optimized）

对 [../1.安全审计.md](../1.安全审计.md) / [../5.二次复核.md](../5.二次复核.md) 里发现的问题做了修复的程序版本。
逐条改动见 [CHANGES.md](./CHANGES.md)。

- 程序 ID：`241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ`
- 编译状态：✅ `cargo-build-sbf` 通过（`target/deploy/pump_meteora.so`，522 KB）
- 部署状态：✅ **已部署 devnet**（2026-07-23，deploy sig `26faJ8rjwMGfg…`，升级权限 = admin `8vNXfHkG…`）。**尚未 `configure` 初始化**（优化版首次初始化要求 `payer == program_data.upgrade_authority`，即用 admin 钱包调用）。
- ⚠️ **不是** 原程序的 drop-in 替换：`configure` 和 `swap` 的 ABI 有变（见 CHANGES.md「ABI 变化」）。

## 目录

```
optimized_contract/
  Cargo.toml / Anchor.toml / Cargo.lock   # 从原仓库复制 lock 以固定依赖版本
  keys/optimized-program-keypair.json      # 优化版程序 keypair
  programs/pump-meteora/src/**             # 加固后的源码
  CHANGES.md                               # 修复清单（fix → finding）
```

## 编译

```bash
cd lessons/5.优化/optimized_contract
export PATH="$HOME/.local/share/solana/install/active_release/bin:$PATH"
cargo-build-sbf --offline      # 依赖已由 Cargo.lock 固定；首次可去掉 --offline 拉取
# 产物：target/deploy/pump_meteora.so
```

> 依赖版本由从原仓库复制来的 `Cargo.lock` 固定。若不带 lock，cargo 会拉到需要 `edition2024`
> 的过新传递依赖（`constant_time_eq 0.4.2`），在当前 SBF 工具链（cargo 1.79）下编译失败。

## 部署到 devnet

```bash
SOL=$HOME/.local/share/solana/install/active_release/bin/solana
# 用一个你控制的钱包做部署者（它将成为升级权限 = 唯一能初始化 config 的人，SEC-01）
$SOL program deploy target/deploy/pump_meteora.so \
  --program-id keys/optimized-program-keypair.json \
  --keypair <你的部署钱包.json> --upgrade-authority <你的部署钱包.json> \
  --use-rpc -u https://devnet.helius-rpc.com/?api-key=<KEY>
# 沙箱里默认 TPU 路径会 panic，务必加 --use-rpc；公共 devnet RPC 会限流，用私有端点
```

## 运行时验证（自行复现）

优化版的**非迁移路径**（configure / create / swap）可以在本地验证机上验证修复生效。
思路：部署优化版后，把 [../experiments](../experiments) 里的攻击脚本按新 ABI 改一改，断言它们**现在失败**：

| 原攻击 | 优化版预期 |
|---|---|
| attack_01 抢跑 | 非升级权限调 `configure` → `IncorrectAuthority` 被拒 |
| attack_02 超额 | 出价 1.2 SOL 完成曲线 → 实扣 ≤ 1.2 SOL（`apply_buy` 钳制）；`max_sol_cost` 过低 → `MaxSolCostExceeded` |
| attack_03 费率 DoS | 费率 110% 的 `configure` → `ValueTooLarge` 在**写入时**被拒（而非 buy 时 panic）|
| attack_04 小数位 | `decimals=9` 的 `configure` → `ValueInvalid` 被拒 |
| attack_11 方向 | `direction=2` → `InvalidDirection` 被拒 |

复现步骤：

```bash
# 1) 起本地验证机（克隆 Metaplex 元数据程序），部署优化版
bash ../experiments/reset.sh            # 需改成部署本目录的 .so 与 241xj 程序 id
# 2) 按上表用新 ABI 调 configure/swap，断言被拒
```

> 注：本次交付时，本地验证机在当前会话环境里出现后台进程被杀的退化，未能由我跑完这一步；
> 但程序**编译通过**，且每条修复在 [CHANGES.md](./CHANGES.md) 中都对应到具体代码行。迁移路径
> （Meteora CPI）需在 devnet 上带真实 Meteora 账户联调，见 CHANGES.md「未完成 / 需联调」。

## 相关

- [../README.md](../README.md) · [../1.安全审计.md](../1.安全审计.md) · [../4.修复补丁参考.md](../4.修复补丁参考.md) · [../experiments](../experiments)
