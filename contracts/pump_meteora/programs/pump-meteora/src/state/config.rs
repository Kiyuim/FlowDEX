use crate::errors::*;
use anchor_lang::{prelude::*, AnchorDeserialize, AnchorSerialize};
use core::fmt::Debug;

#[account]
#[derive(Debug)]
pub struct Config {
    pub authority: Pubkey,
    //  use this for meteora migration
    pub migration_authority: Pubkey,

    pub team_wallet: Pubkey,
    pub team_wallet_secondary: Pubkey,
    pub migration_wallet: Pubkey,

    pub platform_buy_fee: u64, //  platform fee percentage
    pub platform_sell_fee: u64,
    pub platform_migration_fee: u64,
    pub platform_buy_fee_secondary: u64,
    pub platform_sell_fee_secondary: u64,
    pub platform_migration_fee_secondary: u64,
    pub creator_buy_fee: u64,
    pub creator_sell_fee: u64,
    pub creator_migration_fee: u64,

    pub token_supply_config: u64,
    pub token_decimals_config: u8,

    pub initial_virtual_token_reserves_config: u64,
    pub initial_virtual_sol_reserves_config: u64,
    pub initial_real_token_reserves_config: u64,
    pub initial_meteora_token_reserves: u64,
    pub initial_meteora_sol_amount: u64,
    pub last_virtual_sol_reserves: u64,

    pub initialized: bool,
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, PartialEq, Eq, Debug)]
pub enum AmountConfig<T: PartialEq + PartialOrd + Debug> {
    Range { min: Option<T>, max: Option<T> },
    Enum(Vec<T>),
}

impl<T: PartialEq + PartialOrd + Debug> AmountConfig<T> {
    pub fn validate(&self, value: &T) -> Result<()> {
        match self {
            Self::Range { min, max } => {
                if let Some(min) = min {
                    if value < min {
                        // msg!("value {value:?} too small, expected at least {min:?}");
                        return Err(ValueTooSmall.into());
                    }
                }
                if let Some(max) = max {
                    if value > max {
                        // msg!("value {value:?} too large, expected at most {max:?}");
                        return Err(ValueTooLarge.into());
                    }
                }

                Ok(())
            }
            Self::Enum(options) => {
                if options.contains(value) {
                    Ok(())
                } else {
                    // msg!("invalid value {value:?}, expected one of: {options:?}");
                    Err(ValueInvalid.into())
                }
            }
        }
    }
}
