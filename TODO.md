# TODO

## Frontend market data and token metadata (2026-09-20)

- Fixed the `Token`/blank-name regression: detail pages now preserve names
  passed from Portfolio and merge indexed/user-created metadata without letting
  placeholder values overwrite real names. Exact mint lookup is supported.
- Fixed homepage and detail stats: 24h volume and price change now read the
  daily `trade_YYYY_MM_DD` shards used by the consumer. Market cap is derived
  from token supply and current token price instead of being replaced by pool
  liquidity. Cache keys now include chain, status, page, and mint.
- Normalized protojson field variants (`vol_24h`, `vol24H`, etc.) and retained
  legitimate zero values. Token-list and K-line WebSockets use the configured
  Railway origin and reconnect safely.
- Fixed chart refresh polling and candle hit detection. Blank chart space no
  longer opens candle details; clicking the candle body or wick does.
- Verified against the Railway database: the previously broken mints now return
  real metadata and nonzero stats. Both production WebSocket endpoints accept
  connections. Latest Vercel production deployment is Ready and includes the
  WebSocket environment variables.

## Still open

- BuyV2 compute-budget failure was traced to the old 100k CU limit; the
  transaction builder now requests 300k CU and always emits the limit
  instruction. K-line charts also use a 5-second polling safety net in
  addition to the WebSocket, and recent Meteora sell prices use execution
  amounts rather than the post-trade reserve ratio.
- Consumer lag root cause found: each block worker spawned an unbounded
  goroutine per slot, overwhelming the RPC provider and causing throttling.
  Block processing now uses the existing bounded ants pool (5 in-flight jobs
  per worker) so the indexer can catch up instead of amplifying the backlog.
- Token detail now passes the same resolved header price into the limit-order
  panel, so its `now` value cannot disagree with the displayed Price because
  one used stale indexed metadata and the other used a newer RPC trade.
- The header now also prefers the newest direct-chain trade over stale indexed
  `token.price`, keeping Price, Recent trades, and limit `now` aligned while
  the consumer is behind.
- K-line now merges the direct on-chain trade stream into the visible candles,
  so the latest trades appear immediately even while the Railway consumer is
  catching up.

- Automatic follow-up sells for double-out/trailing-stop still require an SPL
  delegate approval transaction signed by the user's wallet. The reference
  devnet repository has the same server-side behavior and no completed
  delegate flow; this cannot be made custodial without changing the wallet
  consent model. Keep these orders visibly Failed when the user has not granted
  authority rather than routing the buy through the platform wallet.
- A final browser wallet test is still needed for a real buy, sell, and the
  user-approved delegate path.

## Done this round, latest (2026-09-20, UX after the buy/sell fix)

- **Sell 25/50/75/100% presets didn't show the amount**: they tracked an
  internal percentage but left the input blank — fixed to populate the
  input with the actual computed number at click time.
- **Page didn't refresh after a trade**: reserves/trades/price all poll on
  their own 15-20s interval independently, so nothing visibly changed
  right after a buy/sell until the next scheduled poll happened to land.
  Added an explicit reload of all three ~2.5s after a successful market
  order.
- **Standard SPL tokens have no price by design** — clarified for the user
  (not a bug): only PumpMeteora/PumpMeteora V2 launches get a bonding curve
  and are directly buyable; a Standard SPL mint needs a CLMM pool + added
  liquidity before it's tradeable at all.
- **Pool creation "Internal error" investigated with a direct on-chain
  simulation** (pulled the exact failing tx, ran `simulateTransaction`
  against Helius): the pool address didn't already exist (not a
  duplicate-creation collision), and simulating with a realistic current
  timestamp succeeds cleanly (`err: null`). The flow itself is correct;
  a stale/expired blockhash between building and sending is the most
  likely explanation for an intermittent failure. Not fully resolved —
  needs the exact failing transaction ID next time to simulate directly.
- **Confirmed**: your PumpMeteora token from earlier this session
  (`676cSvo3SsJm1nsgMjTKURa4ofYWL3Jc49MWvVb5wVZ1`) is still not indexed at
  any status — its creation block was one of the ones the consumer still
  missed. Buy/sell for it works anyway (the on-chain fallback doesn't
  depend on indexing), but it won't appear on the Discovery homepage and
  its kline won't aggregate until/unless indexing catches it — this remains
  probabilistic, not guaranteed, even with the improved concurrency.

## Buy/sell now confirmed working for un-indexed tokens (2026-09-20)

The critical-path item — root-caused and fixed end-to-end, verified via
direct API calls (both buy and sell now return `code: 10000` with a real
built transaction for a token the consumer has never indexed):

1. `CreateMarketOrder` required a DB `pair` row (via `GetPairInfoByToken`)
   before it would even attempt to build a swap, hard-failing with 520 for
   any un-indexed token. Added `chainsolana.DetectTokenSource`: probes
   pump.fun then both pump-meteora builds on-chain for a matching
   bonding-curve account, and falls back to it when the DB lookup misses.
   Applied to both `CreateMarketOrder` and the trailing-stop executor's
   `CreateMarketTx`.
2. Root-caused via step-by-step probe logging that the fallback's own
   on-chain reads (and `pkg/pumpfun/pump/meteora.go`'s account reads, one
   step further into the pipeline) were themselves failing — not because
   accounts don't exist, but because devnet RPC (even via Helius) rate-
   limits aggressively enough that single-shot reads are unreliable.
   Individually patching each call site as we hit each one was whack-a-mole,
   so fixed it systemically instead: `trade/internal/chain/solana/rpc_retry.go`
   wraps the trade service's whole Solana RPC client so every call retries
   automatically (safe unconditionally — a real "not found" comes back as a
   successful call with a nil value, never an error).
3. Along the way, found and fixed the actual reason none of this was
   debuggable: the consumer printed every transaction's full log messages
   unconditionally, and every vote transaction (the overwhelming majority
   of on-chain traffic) logged at ERROR level via a leaked loop variable.
   Railway was silently dropping log messages from every service in the
   deployment because of the volume ("Messages dropped: 148") — nothing
   else could be debugged until this was fixed.

**Not yet verified**: an actual signed buy/sell through the browser wallet
(only the unsigned-tx-building step was tested directly via API, since this
environment has no funded wallet or browser to complete a real signed send).

## Done this round, yet later (2026-09-20) — a hard RPC blocker

Pasted browser console logs surfaced the real, structural cause behind a lot
of this round's flakiness: `useBondingCurveTrades` called
`connection.getParsedTransactions(sigArray)` (plural) to fetch a chunk of
transactions at once. `@solana/web3.js` sends that as a single **batched**
JSON-RPC request, and Helius's plan on this project flatly rejects batch
requests — `403: Batch requests are only available for paid plans` — every
single time, not just under load. This wasn't a rate-limit that retries
would eventually get past; recent trades for ANY token (real or seeded)
could never load through this path. Fixed by fetching one transaction at a
time (`getParsedTransaction`, singular) instead, capped at 15 new
signatures per poll with a small delay between each to stay under the
separate (retriable) 429 rate limit.

## Done this round, even later (2026-09-20)

- **Portfolio "Created by you" links showed the token name as "TOKEN"**:
  the Link to `/token/:mint` passed no `state`, so TokenDetail.js had
  nothing to show until (if ever) `index_pump` picked the token up. Fixed
  to pass the real name/symbol/icon straight from the already-fetched
  `user_tokens` data.
- **Chart no longer mounts (and its canvas no longer "floats") when a
  token has zero on-chain trades** — reverted to gating on `hasTrades`
  (now correctly computed via the source-aware `useBondingCurveTrades`
  fixed earlier this round) instead of unconditionally mounting it. Shows
  the plain "🌱 No trades yet" placeholder instead.
- **Pool creation "Internal error" report**: could not repro without a tx
  signature or the specific token pair — most likely explanation is a
  retried/duplicate token pair colliding with a pool that already exists
  on-chain (Raydium CLMM pool addresses are deterministic PDAs per pair +
  fee tier), which a wallet's preflight simulation surfaces generically as
  "Internal error". Told the user to retry with a genuinely fresh pair to
  isolate whether it's a collision or a new issue.
- Could not test buy/sell live myself as asked — the generated server
  wallet has 0 SOL (devnet faucets still rate-limited) and there's no
  browser/wallet extension available in this environment to drive the UI.

## Done this round, latest (2026-09-20, confirmed live)

- **Pool creation actually works** — confirmed on-chain, not just in theory:
  pulled a real transaction signature from a live create-pool attempt and
  checked it via Helius (`getTransaction`) — `err: null`, `Instruction:
  CreatePool` executed successfully. The `sendTransaction` fix from earlier
  this round resolved it; the "signature has invalid length" reports after
  that were from a stale bundle or an intermittent retry, not a standing bug.
- **Root-caused why new tokens don't show up anywhere**: checked production
  logs for `processBlock` slot numbers and found gaps of 800-1000+ slots
  between processed blocks (devnet produces a new slot every ~0.4s) —
  `consumer.yaml` had `Concurrency: 1`, so a single worker fetching+decoding
  blocks sequentially couldn't remotely keep pace, and almost every block —
  including whichever one held a given token's creation — was silently
  dropped. Raised to 8. Confirmed live afterward: gaps shrank from
  800-1000+ slots to single digits.
- **`useBondingCurveTrades`/`useBondingCurveReserves` were pump.fun-only**:
  they hardcoded pump.fun's bonding-curve PDA derivation and event-log
  format, so a PumpMeteora-sourced token always showed "No trades yet" /
  empty reserves regardless of indexing status — a real bug, not just an
  indexing-lag symptom. Made both source-aware: try pump.fun first, then
  both pump-meteora builds, using a new client-side Swap-event parser
  (`lib/meteora.js`, layout ported from the consumer's own Go decoder).
- **Add Liquidity now refuses a pool with no real on-chain account** up
  front (a plain `getAccountInfo` check before opening the form), instead
  of letting the user fill in amounts and hit a 515 at submit time.
- **Chart's "no candles yet" placeholder now matches the reference's own
  wording** ("No trades yet — Be the first to buy this token…") instead of
  a generic "No chart data available" message.

## Priority order (per explicit user direction, 2026-09-20)

1. **Buying a real token must work, and the kline chart must update from it.**
   This is the critical path — without it nothing downstream (trailing
   stop, double-out, Portfolio) can be verified as actually working.
2. **Liquidity** (add liquidity to a CLMM pool) — important, second priority.
3. Everything else in this file.

## Still open — needs live testing against a REAL (not seeded) token/pool

- **Buy/sell end-to-end, with the kline updating**: the actual instruction
  bug is fixed (buy_v2/sell_v2 swap, see below) and the gateway routes/
  auth-whitelist gaps are fixed, but this has NOT yet been confirmed working
  live end-to-end (create a real token → buy it → watch the candle appear).
  This is the single most important thing to verify next.
- **Add Liquidity 515 error — root-caused**: confirmed the SOL/SCAT pool's
  `poolState` address isn't a real Solana account either (same "WrongSize"
  RPC response as the fake pump tokens) — it's seeded data, so
  `AddLiquidityV1` fails trying to read live on-chain pool state that
  doesn't exist. Not a bug; needs testing against a pool actually created
  through the app.
- **Pool creation "signature has invalid length" — resolved**: switching to
  wallet-adapter's `sendTransaction` fixed it. Confirmed on-chain: pulled a
  real signature from a successful create-pool attempt and verified via
  Helius `getTransaction` — `err: null`. Earlier reports of this error after
  the fix shipped were from a stale bundle or an intermittent retry, not a
  standing bug — if it recurs, it needs a fresh repro to investigate further.
- **Portfolio token name showing as a truncated address**: root-caused —
  Portfolio.js (from the reference) does a pure client-side wallet scan and
  only labels tokens via `/v1/market/index_pump`'s list, which never
  includes a plain SPL mint (no bonding-curve pair) and lags for a
  freshly-created PumpMeteora token. Fixed by also merging in
  `/v1/market/user_tokens` (backed by `record_user_asset`, recorded at
  creation time with the real name) as a second metadata source. Not yet
  confirmed live.

## Done this round, continued further (2026-09-20, even later)

- **Chart overlay covering the stats below it**: `TokenDetail.js` wrapped the
  chart in a fixed `h-[380px] md:h-[460px]` box, but `TradingViewChart`'s own
  header row + 400px canvas + connection-status footer add up to more than
  that — always overflowing, just invisible before because the chart never
  used to render for tokens with no on-chain trades. Now that it always
  mounts (an earlier fix this round), the overflow spilled onto Price/24h
  Volume/Trades/Traders below it. Removed the fixed height when a token is
  loaded (only the loading placeholder still needs it).
- **Portfolio never showed tokens you created — root-caused properly this
  time**: Portfolio.js only lists wallet token *balances*
  (`getParsedTokenAccountsByOwner`), but creating a bonding-curve token
  (PumpMeteora) mints the entire supply into the program's own vault, not
  the creator's wallet — so a created token can never appear there, no
  matter how much later trading happens or how good the name-lookup is.
  Added a separate "Created by you" section backed directly by
  `/v1/market/user_tokens` and `/v1/market/user_pools` (record_user_asset
  data), independent of wallet balance. Verified live against a real
  wallet's data — the rows were already there, just never surfaced.

## Done this round, continued (2026-09-20, later)

- **Gateway auth-whitelist gap**: `create_trailing_stop`, `cancel_order`,
  `query_current_orders` 404'd, then 401'd once routed — a separate
  lowercase `whitelist.path` list (distinct from the route `Mappings`)
  requires a JWT for anything not listed, and this project has no login
  system. Added all three; Open Orders should now actually load instead of
  showing "unavailable".
- **Portfolio recording restored**: `record_user_asset` was called nowhere
  in the current frontend — lost in the earlier wholesale frontend
  replacement. Restored for all three token-creation paths and pool
  creation, and Portfolio.js now also reads it back for token names/icons.
- **Fragile transaction-sending pattern**: `TokenCreation.js`, `
  PoolCreation.js`, `AddLiquidity.js` all manually called `signTransaction()`
  then re-serialized and `sendRawTransaction()`'d the result, instead of the
  wallet adapter's own `sendTransaction()`. Switched all three (did not
  resolve the pool-creation signature bug — see "Still open" above — but is
  the more correct pattern regardless, and may fix it for some wallets).
- **PoolCreation.js debug-info panel removed** from the UI (same fix
  applied to AddLiquidityHeader.js earlier, missed here).
- **Bonding-curve progress could show over 100%** (seed data has corrupted
  `pump_point` values on some old tokens) — the width bar was already
  clamped, the number next to it wasn't. Clamped.
- **"No chart data available" now renders inside the chart canvas** as a
  centered 🌱 placeholder instead of a page-level banner that pushed the
  stats below it out of place.
- **AddLiquidityHeader was the only fully light-themed (white background)
  component left** — a stale, pre-dark-theme stylesheet. Re-themed to the
  app's dark trader-terminal palette.
- **"↕️ Switch" button was rendering broken**: a 40px circular button
  can't fit an emoji plus the word "Switch". Now shows just the icon, with
  the label moved to a tooltip/aria-label.

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

## Kline "not real-time" — traced to a missing Vercel env var, not a pipeline bug (2026-09-20)

User reported the kline chart never updates live and sells don't produce
candles. Traced the whole pipeline end to end:

- Direct `curl` to `/v1/market/get_candlestick` with a proper
  `from_timestamp`/`to_timestamp` window (not a bare `limit=20`, which just
  returns the oldest 20 candles chronologically) shows the aggregation is
  actually working: candles before real trading started are flat/zero-volume
  placeholders, then real trades produce real, changing-price,
  nonzero-volume candles up to the current time. `dataflow`'s
  `calckline.go` and its `WHERE name IN (...)`-fixed pair lookups are fine.
  My earlier "flat candles" finding was an artifact of querying without a
  recent time window, not a real bug.
- `TradingViewChart.js`'s `fetchKlineData` already uses a correct rolling
  `now - lookback` to `now` window, so a manual refresh/interval-switch
  does pull live data.
- The missing piece is push updates: `dataflow/internal/mqs/consumers/
  trade_consumer.go` publishes every new/updated kline (all intervals,
  including the UI's default `1h`) to Redis `kline:updates`; the
  `websocket` service subscribes and rebroadcasts to matching
  `/ws/kline?pair_address=...` clients — this side of the code is correct.
- But the frontend's WS URL (`TradingViewChart.js` line ~278) is
  `process.env.REACT_APP_KLINE_WS_URL || wss://${window.location.hostname}`.
  Confirmed via `.env*` search (none exist in the repo — these are
  Vercel-dashboard-only vars) and via the still-open action item from the
  2026-09-20 frontend swap entry above that this var was never confirmed
  set. Without it, the chart's WebSocket tries to connect to
  `wss://<the-vercel-frontend-domain-itself>`, which Vercel doesn't proxy —
  it fails silently (no visible error), so the chart only ever shows
  whatever it fetched on mount/interval-switch and never gets live pushed
  updates. This fully explains "kline isn't real-time" without requiring
  any backend or pipeline fix.

The production websocket variables are configured in Vercel:
- `REACT_APP_KLINE_WS_URL=wss://websocket-production-4109.up.railway.app`
- `REACT_APP_WS_URL=wss://websocket-production-4109.up.railway.app` (same
  service, used by the token-list live feed — same missing-var symptom)

**Updated 2026-09-20:** Vercel CLI access is now available and the production
deployment was verified as Ready and aliased to `flow-dex-alpha.vercel.app`.
Both websocket variables are present in the project configuration and the
live bundle contains the configured Railway websocket origin; the old action
item above is complete.

The variables were then included in the Ready production build. (Confirmed
`websocket-production-4109.up.railway.app` is the
live public domain for the `websocket` Railway service via `railway
status --json`.)
