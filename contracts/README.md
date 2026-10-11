# Contracts

Anchor sources for the launch programs the backend indexes and builds
transactions for (`pkg/pumpfun/pump/meteora.go`).

| Dir | Program | Backend constant | devnet ID |
|---|---|---|---|
| `pump_meteora_v2/` | pump-meteora, hardened ("optimized") build | `PumpMeteoraOptProgramID` | `241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ` |
| `pump_meteora/` | pump-meteora, original | `PumpMeteoraProgramID` | `AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5` |

Both are already deployed on devnet; upgrade authority is the admin wallet
`8vNXfHkG6xkfVSBMjdPaXXbGEBVFHEmv5Mnjy3adSHVV`.

`pump_meteora/` was copied from the `pumpfun` repo, whose `declare_id!` is
`5hZwFQyG9fmQUuRKSSt2q5AdFGD4VjZRgzimhrLHKTcG` — not the deployed `AEBUS7…`.
Change `declare_id!` and `Anchor.toml` to the deployed ID before upgrading it.

Program/admin keypairs live in each workspace's `keys/` and are gitignored;
copy them in locally to build-and-deploy:

```bash
cd contracts/pump_meteora_v2
cargo-build-sbf
solana program deploy target/deploy/pump_meteora.so \
  --program-id keys/optimized-program-keypair.json \
  --upgrade-authority <admin keypair> -u devnet
```
