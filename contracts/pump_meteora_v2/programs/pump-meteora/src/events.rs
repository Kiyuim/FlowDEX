use anchor_lang::prelude::*;

#[event]
pub struct LaunchEvent {
    pub creator: Pubkey,
    pub mint: Pubkey,
    pub bonding_curve: Pubkey,
    pub metadata: Pubkey,
    pub decimals: u8,
    pub token_supply: u64,
    pub reserve_lamport: u64,
    pub reserve_token: u64,
    pub timestamp: i64,
}

// BIZ-03 fix: emit BOTH virtual and real reserves under correctly-named fields.
#[event]
pub struct SwapEvent {
    pub user: Pubkey,
    pub mint: Pubkey,
    pub bonding_curve: Pubkey,
    pub amount_in: u64,
    pub direction: u8,
    pub minimum_receive_amount: u64,
    pub amount_out: u64,
    pub virtual_sol_reserves: u64,
    pub virtual_token_reserves: u64,
    pub real_sol_reserves: u64,
    pub real_token_reserves: u64,
    pub timestamp: i64,
}

#[event]
pub struct CompleteEvent {
    pub user: Pubkey,
    pub mint: Pubkey,
    pub bonding_curve: Pubkey,
    pub timestamp: i64,
}

// BIZ-02 fix: migration is now observable off-chain.
#[event]
pub struct MigrationEvent {
    pub mint: Pubkey,
    pub bonding_curve: Pubkey,
    pub pool: Pubkey,
    pub lp_mint: Pubkey,
    pub sol_amount: u64,
    pub token_amount: u64,
    pub timestamp: i64,
}

#[event]
pub struct LpLockedEvent {
    pub mint: Pubkey,
    pub pool: Pubkey,
    pub lp_amount_locked: u64,
    pub timestamp: i64,
}
