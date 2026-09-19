# TODO

## Done this round (2026-09-20)

- **Real token/pool price**: `PumpTokenItem.Price` / `ClmmPoolItem.Price` now
  carry the last real trade price (from `trade`), replacing `TokenCard.js`'s
  hardcoded `$0.00`.
- **Real SOL/USD price**: `pkg/solprice` (CoinGecko, cached) backs
  `SolPriceUsd` on both list responses. Independent of the project's own
  on-chain price pipeline, which never populated `block.sol_price` (still
  zero rows — see the SDK bug below).
- **Sources page**: new tab, groups tokens by `PumpTokenItem.Program`.
- **Portfolio page**: `GetUserTokens`/`GetUserPools`/`RecordUserAsset` RPCs
  built and wired (`market/internal/logic/userassetslogic.go`, routed in
  `gateway.yaml`). `TokenCreation.js` calls `record_user_asset` after a
  successful mint, verified live end-to-end.
- **Pool + Add Liquidity merged** into one "Pool" tab (`PoolHub.js`).
- **CLMM pool cards**: replaced "split each pool into two fake token cards,
  dedupe by address" (card count never matched pool count) with one real
  `ClmmPoolCard` per pool.
- **CLMM wSOL showing "Unknown"/blank icon**: fixed with a well-known-mint
  fallback in `getclmmpoollistlogic.go`.
- **CLMM pool_state filter**: `GetClmmPoolListRequest.pool_state` now actually
  filters (previously ignored server-side; the manual pool-address entry in
  Add Liquidity would silently get back an unrelated pool and use its data).
- **Grid overflow**: `.token-grid` column minimum (300px) was smaller than
  `.token-card`'s own `min-width` (420px), forcing cards to overflow their
  grid cells. Fixed.
- **Wallet connect race**: calling `connect()` synchronously right after
  `select()` read the stale pre-update `wallet` and threw
  `WalletNotSelectedError` on every first click (second click worked because
  `wallet` had caught up by then). Fixed via `useLayoutEffect` keyed off
  `wallet` actually changing.
- **Kline history**: seed data extended with sparser trades 1-18 days back
  (dense recent-hours trades untouched, so already-verified 24h stats didn't
  shift) so 4h/12h/1d candles have more than ~1 data point. Backfill to
  production is applying now (see below).

## In progress

- **Kline backfill to production**: ~16k INSERT statements applying via
  chunked `mysql <` over a Railway TCP proxy (slow — each 1000-line chunk
  takes a few minutes; DDL and trade rows are already fully applied, only
  the kline table rows are still trickling in). Self-resolving, no action
  needed; charts gain history progressively as it completes.

## Still open

- **Pool creation doesn't record to Portfolio**: `trade.CreatePoolResponse`
  only returns `tx_hash`, no pool state address, so there's nothing correct
  to record as `asset_address` yet. Needs either a `pool_address` field added
  to `CreatePoolResponse` (trade.proto) or the frontend deriving the CLMM
  pool-state PDA client-side from the known seeds.
- **Holder count (人数) still 0**: not a code bug — `FetchHolderCounts` is
  wired correctly, but `sol_token_account` has zero seed rows. Needs seed
  data for that table.
- **Wallet-select modal off-center / "on the right side"**: reviewed the
  code — `position:fixed; inset:0; flex-center` is textbook-correct. Couldn't
  reproduce a bug in our own modal; likely the wallet browser extension's own
  OS-level popup, which we don't control.
- **Token-2022 minting**: `TokenCreation.js` always creates a Classic SPL
  Token (`TOKEN_PROGRAM_ID` hardcoded). Needs
  `createInitializeMint2Instruction` + `TOKEN_2022_PROGRAM_ID` + the
  Token-2022-specific ATA derivation.
- **Solana transaction deserialization**: `blocto/solana-go-sdk` fails to
  parse a chunk of real devnet blocks even after the vendored
  `MaxSupportedTransactionVersion` patch (`failed to deserialize
  transaction, err: parse signature error`) — real on-chain data ingestion
  still isn't fully working. This is also why `block.sol_price` has never
  been populated by the real pipeline. Needs either a deeper fix to the
  vendored SDK's transaction parser or a switch to `gagliardetto/solana-go`
  for block fetching.
