# TODO

## Backend deep-merge with devnet-branch trading features (2026-09-20)

Per explicit direction ("深度整合...主要以功能多的为主也就是devnet"), merged the
reference's trading logic onto this backend instead of overwriting it —
keeping this session's own fixes (block-ingestion abort fix, kline window,
pumpstats/holder-count, CLMM list fixes, solprice, full Portfolio RPC set,
`pool_address` on `CreatePoolResponse`) while adopting what devnet has that
this backend lacked. Deployed live (Railway `FlowDEX` service bundles all Go
services in one container via `start-backend.sh`).

- **Fixed the real "can't buy tokens" bug**: `CreateMarketOrder4Pumpfun` was
  still building legacy `buy`/`sell` instructions. The currently-deployed
  pump program rejects legacy `buy` outright (`BuybackFeeRecipientMissing`)
  and silently drops legacy `sell` fills. Switched to `buy_v2`/`sell_v2`
  (`pkg/pumpfun/pump`). This affects every pump.fun-sourced token, not just
  the ones added this round — probably the actual reason trading looked
  broken across the board.
- **PumpMeteora trading + indexing**: build/dispatch in
  `trade/internal/chain/solana/pump_meteora.go` +
  `pkg/pumpfun/pump/meteora.go`; on-chain create/swap decoding in
  `consumer/internal/logic/sol/block/pump_meteora*.go`, wired into both the
  outer and inner instruction dispatchers for both PumpMeteora program
  builds. New tokens created via the "PumpMeteora"/"PumpMeteora V2" launch
  targets will now actually get indexed and become tradeable/chartable.
- **Re-enabled the CLMM (Raydium concentrated liquidity) decoder** in the
  consumer — it was fully implemented but commented out, so CLMM pool trades
  were never being indexed at all.
- **Enabled `SendTokenPrice2TradeRPC`** — this was commented out
  (`// 推送...用于限价单交易匹配`), meaning the price feed limit orders and
  trailing stops need to ever trigger was never running. Limit orders and
  trailing stops were wired up but silently inert; should now actually fire.
- **Trailing stop implemented**: `createtrailingstoplogic.go` was a stub
  (`// todo: add your logic here`) — now a real implementation with a
  nominal-SOL-price fallback for devnet's zero `base_token_price`.
- **Auto-slippage + double-out + trailing-stop-attached buys**: added to
  `trade.proto` (`is_auto_slippage`, `trailing_percent`, `double_out`) and
  `createmarketorderlogic.go`/`createlimitorderlogic.go`.
- **New ticker** (`trade/internal/ticker`) confirms on-chain fills against
  the consumer's indexed trade tables (falling back to direct RPC lookup for
  fills the consumer missed) and auto-creates the double-out sell / attached
  trailing stop once a buy confirms.
- **Action needed — server wallet for custodial orders**: limit orders,
  trailing stops, and double-out sells are executed by a server-held key
  (`PRIVATE_KEY` env var, base64 ed25519), since the user isn't present to
  sign a triggered order. Generated a fresh **devnet-only** keypair
  (`8TxSw6R2k9rhLQwPD1rBqgQca2iWXyJL7gpMSE3b751u`) — set it on Railway
  (`FlowDEX` service → Variables → `PRIVATE_KEY` = the base64 value handed to
  you in-conversation; this is a secret-store write, so it needed your own
  action rather than mine). It also needs a small amount of devnet SOL to
  pay fees — both public devnet faucets (Solana's and Helius's) were
  rate-limited when I tried; fund it once they reset, or send it some
  devnet SOL directly from your own wallet.
- Removed the dead Token-2022 toggle from `TokenCreation.js` — the checkbox
  did nothing (`createToken` always used `TOKEN_PROGRAM_ID` regardless).

## Frontend/backend data-shape fixes (2026-09-20)

- **Add Liquidity pool selector was silently broken**: `AddLiquidityHeader.js`
  read snake_case fields (`pool_state`, `token0_symbol`, `token0_mint`) but
  the live API returns camelCase (`poolState`, `inputTokenSymbol`,
  `inputVaultMint`) — every field came back `undefined`, so selecting a pool
  never populated `poolInfo` and the whole form below (price range, amounts,
  submit button) never rendered. Fixed the field mapping.
- **Token Source filters always showed 0** for every specific source
  (Pump.fun/PumpMeteora/PumpMeteora V2), even though "All Sources" showed
  tokens: it classified tokens by deriving a bonding-curve PDA client-side
  and matching `pairAddress` — which can never match this project's
  fabricated seed-data addresses (confirmed one isn't even a valid-format
  Solana address). The backend already tags each token's real trading source
  via `pair.Name` (set by the consumer's per-program decoder — "PumpFun"/
  "PumpMeteora"/"PumpMeteoraV2"), but `market`'s `GetPumpTokenList` was
  exposing `token.Program` under this field instead, which is actually the
  *SPL token program* (Token/Token-2022) the mint uses — a different axis
  entirely, that only coincidentally looked right on old seed data. Fixed
  the backend to expose `pair.Name`, and the frontend to prefer it (PDA
  derivation kept only as a fallback).
- **Candle chart wrongly gated on on-chain trades**: `TokenDetail.js` only
  mounted `TradingViewChart` when the *on-chain* trades hook had data — but
  that chart component fetches candles from **our own backend**
  (`/v1/market/get_candlestick`), independent of on-chain state. The gate
  was hiding a chart that renders fine on backend data alone. Now always
  mounts when a token is present.
- **Kline chart stuck at ~1 day of history — again**: `TradingViewChart.js`
  had regressed back to a hardcoded 24h lookback window (this exact bug was
  fixed earlier this session; the fix was lost when the frontend got
  wholesale-replaced with the reference). Re-fixed to scale the window with
  the selected interval.
- **Candlestick click-to-inspect — restored**: the frontend replacement also
  silently dropped this (GMGN-style O/H/L/C/%-change popup on click),
  re-added to the reference's own `TradingViewChart.js`.

### Still not addressed this round
- **Old seeded demo tokens (CHAD, PEPESOL, FROG, etc.) have no real on-chain
  presence** — their addresses are fabricated, not real Solana accounts, so
  they can never chart real on-chain trades or actually execute a buy/sell
  under the new architecture (which reads bonding-curve reserves/trades
  live from chain for the trade panel, though the candle chart itself is
  backend-fed and works fine for them). You asked to delete these and
  create real replacements with 10+ days of candle history — not done yet,
  planned next: create real tokens on-chain (PumpMeteora, now that trading
  actually works), then backfill historical trade/kline rows under their
  *real* addresses the same way the original seed data was generated.
- Redundant-file cleanup (`以及查看是否有冗余文件可以删除`) — not yet done;
  revisit once the data-recreation plan above lands, since the frontend
  swap may have reintroduced files worth re-checking (`MockTokenWebSocket.js`,
  `MockTradingViewTest.js`, `Header.js`) for actual usage before deleting.

## Frontend replaced with the reference implementation (2026-09-20)

Swapped `pump-tokens-ui`'s entire `src/` for the more complete reference
build (pump-tokens-ui.vercel.app) instead of continuing to rebuild its
features piecemeal. Traced the live site to the `devnet` branch of
`github.com/dreamerinsgp/fun_dex_v2` by matching commit SHAs against that
site's own Vercel deployment history (the repo's default `main` branch is
an older, less-featured snapshot — the advanced code is only on `devnet`).

Brings in: full trading terminal (`pages/TokenDetail.js` — pool reserves,
recent trades, buy/sell, limit orders, trailing stop, "double out"),
`pages/Portfolio.js`, `pages/Pools.js`, `pages/TokenSource.js`, a real
PumpMeteora integration (`lib/meteora.js`), React Router navigation, and a
Tailwind-based design system. Kept this project's own `vercel.json`
(already pointed at this project's Railway backend) and `craco.config.js`
(vm-browserify polyfill); merged in the new deps (tailwindcss/postcss/
autoprefixer, react-hot-toast, react-router-dom). Rebranded "FunDex" to
"FlowDEX"; translated the one file with user-facing Chinese strings
(AddLiquidityHeader's auto-fetch status messages) to English.

**Action needed**: this reference code reads a couple of env vars this
project's Vercel project may not have set yet — check/add:
- `REACT_APP_WS_URL` — the Railway websocket service's public URL (used
  for the live token-list feed; without it, falls back to a same-origin
  `wss://` that Vercel won't proxy, so live updates silently won't connect
  — not a crash, just a missing feature).
- `REACT_APP_HELIUS_API_KEY` / `REACT_APP_KLINE_WS_URL` — likely already
  set from earlier work; confirm they still apply.
- `REACT_APP_SOLANA_RPC_URL` — optional, only needed to override the
  Helius-key-derived endpoint.

Build verified clean locally and confirmed live (new bundle hash, contains
`PumpMeteora`/`TokenDetail`/`TradePanel`) — not yet manually exercised
end-to-end in a browser against this project's backend, so there may be
API-shape mismatches to fix as they turn up (this backend evolved a lot
this session — price fields, Portfolio RPCs, CLMM fixes — the reference's
frontend expectations haven't been cross-checked against all of it).

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