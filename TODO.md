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
- **Portfolio page**: show a connected wallet's created tokens/pools.
  `MyTokens.js` and `MyPools.js` already exist as an unwired scaffold — they
  call `/v1/market/user_tokens?wallet_address=...`, which **does not exist yet**
  on the backend (not in `market.proto`, not routed in `gateway.yaml`). Needs
  a new market RPC method backed by the `user_created_assets` table, then wire
  these components into `App.js`'s tab list.

## Backend (see docs/项目已知问题与修复记录.md for full detail)

- Solana transaction deserialization: `blocto/solana-go-sdk` fails to parse a
  chunk of real devnet blocks even after the vendored `MaxSupportedTransactionVersion`
  patch (`failed to deserialize transaction, err: parse signature error`) —
  real on-chain data ingestion is still not fully working. Needs either a
  deeper fix to the vendored SDK's transaction parser or a switch to
  `gagliardetto/solana-go` for block fetching.
