# TODO

## Frontend

- **Token-2022 minting**: `TokenCreation.js` always creates a Classic SPL Token
  (`TOKEN_PROGRAM_ID` is hardcoded). Real Token-2022 support needs
  `createInitializeMint2Instruction` + `TOKEN_2022_PROGRAM_ID` + the
  Token-2022-specific ATA derivation. The old UI toggle for this was removed
  since it never actually switched programs.
- **Sources page**: browse/filter tokens by originating program (Pump.fun /
  PumpMeteora / etc). `token.program` already exists as a DB column — this is
  mostly a new frontend view + a market API filter, not a backend rebuild.
- **Portfolio page**: `GetUserTokens`/`GetUserPools`/`RecordUserAsset` RPCs now
  exist and are wired (`market/internal/logic/userassetslogic.go`,
  routed in `gateway.yaml`). `TokenCreation.js` calls `record_user_asset`
  after a successful mint. **Pool creation does NOT record itself yet** —
  `trade.CreatePoolResponse` only returns `tx_hash`, no pool state address,
  so there's nothing correct to record as `asset_address`. Needs either a
  `pool_address` field added to `CreatePoolResponse` (trade.proto) or the
  frontend deriving the CLMM pool-state PDA client-side from the known seeds.
  Until then Portfolio will show created tokens but never created pools.
  Also: this can't retroactively attribute anything created before this
  wiring shipped — `user_created_assets` was empty until now.

## Backend (see docs/项目已知问题与修复记录.md for full detail)

- Solana transaction deserialization: `blocto/solana-go-sdk` fails to parse a
  chunk of real devnet blocks even after the vendored `MaxSupportedTransactionVersion`
  patch (`failed to deserialize transaction, err: parse signature error`) —
  real on-chain data ingestion is still not fully working. Needs either a
  deeper fix to the vendored SDK's transaction parser or a switch to
  `gagliardetto/solana-go` for block fetching.
