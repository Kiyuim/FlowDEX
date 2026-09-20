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

## Still open

- **Create Token: PumpMeteora / PumpMeteora V2 launch targets**: requested,
  not started. Needs understanding the PumpMeteora on-chain program
  interface (bonding curve init instruction, fixed decimals/supply per the
  program) — this is new on-chain integration work, not a UI-only change.
- **Wallet-select modal off-center / "on the right side"**: reviewed the
  code — `position:fixed; inset:0; flex-center` is textbook-correct. Couldn't
  reproduce a bug in our own modal; likely the wallet browser extension's own
  OS-level popup, which we don't control.
- **Solana transaction deserialization**: `blocto/solana-go-sdk` fails to
  parse a chunk of real devnet blocks even after the vendored
  `MaxSupportedTransactionVersion` patch (`failed to deserialize
  transaction, err: parse signature error`) — real on-chain data ingestion
  still isn't fully working. This is also why `block.sol_price` has never
  been populated by the real pipeline. Needs either a deeper fix to the
  vendored SDK's transaction parser or a switch to `gagliardetto/solana-go`
  for block fetching.
