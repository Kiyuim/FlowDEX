use anchor_lang::solana_program::{pubkey, pubkey::Pubkey};

pub const CONFIG: &str = "config";
pub const GLOBAL: &str = "global";
pub const BONDING_CURVE: &str = "bonding_curve";
pub const METADATA: &str = "metadata";
pub const TOKEN_VAULT_SEED: &str = "token_vault";

// ---- token math is defined for 6-decimal tokens ONLY (SEC-04) ----
// These are enforced by Config::validate() + a require! in create_bonding_curve.
pub const TOKEN_DECIMALS: u8 = 6;
pub const TOKEN_SCALE: u128 = 1_000_000; // 10^TOKEN_DECIMALS
pub const LAMPORT_SCALE: u128 = 1_000_000_000; // 10^9
pub const MAX_BPS: u64 = 10_000; // 100%

// ---- migration costs (previously bare magic numbers in create_pool.rs) ----
pub const MIGRATION_RENT_RESERVE: u64 = 200_000_000; // 0.2 SOL kept back for Meteora account rents
pub const MIGRATION_FLAT_FEE: u64 = 70_000_000; // 0.07 SOL flat platform migration fee

// ---- external programs (typed constants replace Pubkey::from_str(...).unwrap()) ----
pub const METEORA_PROGRAM: Pubkey = pubkey!("Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB");
pub const METEORA_VAULT_PROGRAM: Pubkey = pubkey!("24Uqj9JCLxUeoC3hGfh5W3s9FM9uCHDS2SG3LYwBpyTi");
pub const QUOTE_MINT: Pubkey = pubkey!("So11111111111111111111111111111111111111112");
