# TODO

## Done this round (2026-09-20)

- **Candlestick click-to-inspect**: clicking a candle in `TradingViewChart`
  shows a floating card (O/H/L/C/Vol/% change, timestamp) at the click
  point, via `lightweight-charts`' `subscribeClick` — GMGN-style chart
  interaction.
- **Header spacing**: `.tab-btn` padding (12px/20px) made the nav row much
  taller than the brand text, inflating the gap to the header's bottom
  border. Tightened button and header padding.
- **Wallet connect silently doing nothing**: the `useLayoutEffect` fix for
  the first-click race only re-fires when `wallet`'s reference actually
  changes. If wallet-adapter had already restored the same wallet from
  localStorage, `select()` is a no-op, `wallet` never changes, and the
  pending connect sat unconsumed forever — clicking did nothing. Now
  connects immediately when the clicked wallet was already selected.
- **Real token/pool price**: `PumpTokenItem.Price` / `ClmmPoolItem.Price` now
  carry the last real trade price (from `trade`), replacing `TokenCard.js`'s
  hardcoded `$0.00`.
- **Real SOL/USD price**: `pkg/solprice` (CoinGecko, cached) backs
  `SolPriceUsd` on both list responses. Independent of the project's own
  on-chain price pipeline, which never populated `block.sol_price` (still
  zero rows — see the SDK bug below).
- **Sources page**: groups tokens by `PumpTokenItem.Program`, plus Raydium
  CLMM pools as their own source group (pools aren't tokens with a program
  value, so they can't share that grouping).
- **Portfolio page**: `GetUserTokens`/`GetUserPools`/`RecordUserAsset` RPCs
  built and wired. `TokenCreation.js` calls `record_user_asset` after a
  successful mint, verified live end-to-end.
- **Pool + Add Liquidity merged** into one "Pools" tab (`PoolHub.js`).
- **CLMM pool cards**: replaced "split each pool into two fake token cards,
  dedupe by address" (card count never matched pool count) with one real
  `ClmmPoolCard` per pool.
- **CLMM wSOL showing "Unknown"/blank icon**: fixed with a well-known-mint
  fallback in `getclmmpoollistlogic.go`.
- **CLMM pool_state filter**: `GetClmmPoolListRequest.pool_state` now actually
  filters (previously ignored server-side; Add Liquidity's manual
  pool-address entry would silently get back an unrelated pool).
- **Grid overflow**: `.token-grid` column minimum (300px) was smaller than
  `.token-card`'s own `min-width` (420px), forcing overflow. Fixed.
- **Wallet connect race**: `WalletNotSelectedError` on first click, every
  time — fixed via `useLayoutEffect` keyed off `wallet` actually changing.
- **Kline chart stuck at ~1 day of history** (the big one): `GetKlineLogic`
  hardcoded the query window to "last 24h, limit 100" and silently ignored
  whatever `from_timestamp`/`to_timestamp`/`limit` the caller actually sent.
  `TradingViewChart.js` independently had the same bug (always requested
  `now - 24h`). Both fixed — the frontend now scales its lookback window
  with the selected interval size. Verified live: a 1d-interval request now
  returns 18 real daily candles instead of 1; CLMM pool charts (keyed by
  `pool_state`) work the same way.
- **Kline history extended**: seed data now has trades spanning 1-18 days
  back (dense recent-hours trades untouched, so 24h stats didn't shift).
- **Holder count (人数) fixed**: seeded the correct week-sharded
  `sol_token_account_20260914` table (matching
  `CountByTokenAddressWithTime`'s table-name derivation) with realistic
  holder rows per token. The counting logic itself was already correct —
  there was just no data.
- **Header redesign**: brand+nav grouped on the left (aligned with the page
  content's left edge, not the raw browser edge), wallet button on the
  right. Nav reordered: Tokens, Sources, Portfolio, Create, Pools, Faucet,
  Security.
- **Data-integrity incident (self-caused, self-fixed)**: widening a
  `random.randint()` range in the seed-data generator desynced Python's
  global RNG state from the original production-seeding run, causing 9 of
  10 pairs' generated addresses to diverge into fabricated strings for the
  newly-generated historical trades/klines. Found via a live API check
  showing only 1 CHAD candle despite 19 rows existing in the DB for a
  *different, garbled* address. Remapped every wrong→correct address across
  `trade` and all 7 kline tables (matched by token symbol, which was
  unaffected), verified zero orphaned addresses remain.
- Removed Claude co-authorship from git history (all 19 commits rewritten,
  force-pushed) per explicit request; future commits won't add it either.

## Done this round, continued

- **Pool creation now records to Portfolio**: `CreatePoolInstructions` /
  `BuildUnsignedPoolTransaction` now also return the derived pool state PDA;
  `CreatePoolResponse.pool_address` (new proto field) carries it back;
  `PoolCreation.js` calls `record_user_asset` with it after a successful
  creation. Portfolio now shows both created tokens and created pools.
- **Header nav wrapping below the brand**: `.tab-navigation` was
  `flex-wrap: wrap`, so once brand + all 7 nav buttons didn't fit on one
  row, the nav dropped to a second line instead of staying level with
  FlowDEX. Nav now scrolls horizontally within its own row instead of
  wrapping.

## Done this round, continued

- **Token-2022 minting implemented**: the toggle now actually works —
  `createInitializeMintInstruction`, `createAssociatedTokenAccountInstruction`,
  and `createMintToInstruction` all receive the selected `programId`
  (previously hardcoded to `TOKEN_PROGRAM_ID` regardless of any toggle, the
  same bug pattern as the earlier fake toggle). Mint sizing uses
  `getMintLen([])` instead of a hardcoded 82-byte constant.

- **Real on-chain block ingestion fixed** (the deepest bug in the project):
  `getSolBlockInfo error: ... parse signature error` was failing on
  essentially every devnet block, and `trade size: 0` for every single one
  — real ingestion had likely never worked. Root-caused with a standalone
  repro tool: the failing bytes are Vote-program transactions using a
  newer, more compact wire format; confirmed independently against
  `gagliardetto/solana-go` (already a project dependency), which fails on
  the *identical* bytes with `numSignatures 129 is too large for remaining
  bytes 288` — so this was never a blocto-specific parser bug, and it's
  data the DEX has no reason to read anyway. The actual bug was
  `convertBlock` aborting the **entire block** the instant any single
  transaction failed to parse, silently discarding every real Pump.fun/
  Raydium swap in that block along with the irrelevant vote tx. Patched to
  skip and log unparseable transactions instead. Verified live: blocks
  that previously failed `GetBlockWithConfig` outright now return 24-48
  real transactions each; `getSolBlockInfo error` no longer appears at all.
  `trade_size` is still 0 in the blocks checked so far — plausibly just low
  swap volume on devnet right now, not a parsing issue — worth monitoring
  over a longer window to see a real trade actually land.
- **Wallet-select-button visual consistency**: normalized the Connect/
  Connecting/Disconnect button's padding, font-size, and border-radius
  across all three states to match `.tab-btn`, for consistent header
  alignment.
- **Token-2022 launch-target UI**: replaced the toggle-switch with a 3-tab
  selector (Standard SPL / Token-2022 / PumpMeteora) per requested reference
  design.

## Still open

- **Create Token: PumpMeteora launch target**: UI tab now exists
  (disabled, "not implemented yet") but isn't wired to anything on-chain.
  Blocked on the actual PumpMeteora program interface — it isn't a
  documented public program (websearch only turns up separate Pump.fun and
  Meteora Dynamic Bonding Curve programs, neither matching the "241xjm…"
  prefix mentioned), and guessing at instruction/account layouts for an
  unknown program risks building transactions that either fail outright or
  behave unexpectedly on-chain. Needs the program ID + IDL from wherever
  this reference originally came from.
- **Wallet-select modal off-center / "on the right side"**: reviewed the
  code — `position:fixed; inset:0; flex-center` is textbook-correct. Couldn't
  reproduce a bug in our own modal; likely the wallet browser extension's own
  OS-level popup, which we don't control.
