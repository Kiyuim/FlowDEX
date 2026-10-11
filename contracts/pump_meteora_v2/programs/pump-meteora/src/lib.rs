use anchor_lang::prelude::*;
pub mod constants;
pub mod errors;
pub mod events;
pub mod instructions;
pub mod state;
pub mod utils;

use instructions::{configure::*, create_bonding_curve::*, create_pool::*, lock_pool::*, swap::*};
use state::config::*;

declare_id!("241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ");

#[program]
pub mod pump_meteora {
    use super::*;

    pub fn configure(ctx: Context<Configure>, new_config: Config) -> Result<()> {
        ctx.accounts.handler(new_config, ctx.bumps.config)
    }

    pub fn create_bonding_curve(
        ctx: Context<CreateBondingCurve>,
        //  metadata
        name: String,
        symbol: String,
        uri: String,
    ) -> Result<()> {
        ctx.accounts
            .handler(name, symbol, uri, ctx.bumps.global_vault)
    }

    // SEC-02: `max_sol_cost` bounds the SOL a buyer can be charged (ignored for sells).
    pub fn swap(
        ctx: Context<Swap>,
        amount: u64,
        direction: u8,
        minimum_receive_amount: u64,
        max_sol_cost: u64,
    ) -> Result<()> {
        ctx.accounts.handler(
            amount,
            direction,
            minimum_receive_amount,
            max_sol_cost,
            ctx.bumps.global_vault,
        )
    }

    pub fn create_pool(ctx: Context<InitializePoolWithConfig>) -> Result<()> {
        instructions::initialize_pool_with_config(ctx)
    }

    pub fn lock_pool(ctx: Context<LockPool>) -> Result<()> {
        instructions::lock_pool(ctx)
    }
}
