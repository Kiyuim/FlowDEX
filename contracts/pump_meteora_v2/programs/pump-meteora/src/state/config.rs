use crate::constants::*;
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

    pub platform_buy_fee: u64, //  bps
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

impl Config {
    // SEC-03 + SEC-13 + SEC-04: validate every admin-supplied parameter at the boundary,
    // so bad config is rejected at write-time (one failed tx) instead of panicking at
    // use-time (whole platform bricked / migrations stuck).
    pub fn validate(&self) -> Result<()> {
        // --- fees: each <= 100%, and each same-path sum <= 100% ---
        let fees = [
            self.platform_buy_fee,
            self.platform_sell_fee,
            self.platform_migration_fee,
            self.platform_buy_fee_secondary,
            self.platform_sell_fee_secondary,
            self.platform_migration_fee_secondary,
            self.creator_buy_fee,
            self.creator_sell_fee,
            self.creator_migration_fee,
        ];
        for f in fees {
            require!(f <= MAX_BPS, ContractError::ValueTooLarge);
        }
        let buy_sum = self
            .platform_buy_fee
            .checked_add(self.platform_buy_fee_secondary)
            .and_then(|v| v.checked_add(self.creator_buy_fee))
            .ok_or(ContractError::ArithmeticError)?;
        let sell_sum = self
            .platform_sell_fee
            .checked_add(self.platform_sell_fee_secondary)
            .and_then(|v| v.checked_add(self.creator_sell_fee))
            .ok_or(ContractError::ArithmeticError)?;
        let mig_sum = self
            .platform_migration_fee
            .checked_add(self.platform_migration_fee_secondary)
            .and_then(|v| v.checked_add(self.creator_migration_fee))
            .ok_or(ContractError::ArithmeticError)?;
        require!(buy_sum <= MAX_BPS, ContractError::ValueTooLarge);
        require!(sell_sum <= MAX_BPS, ContractError::ValueTooLarge);
        require!(mig_sum <= MAX_BPS, ContractError::ValueTooLarge);

        // --- decimals locked to 6 (curve math hardcodes 6-decimal scaling) ---
        require!(
            self.token_decimals_config == TOKEN_DECIMALS,
            ContractError::ValueInvalid
        );

        // --- supply invariant: everything minted must be allocated (curve + meteora) ---
        // Without this, a completed curve's ATA can hold fewer tokens than migration needs.
        let allocated = self
            .initial_real_token_reserves_config
            .checked_add(self.initial_meteora_token_reserves)
            .ok_or(ContractError::ArithmeticError)?;
        require!(
            self.token_supply_config == allocated,
            ContractError::ValueInvalid
        );

        // --- curve ordering invariants ---
        require!(
            self.initial_virtual_token_reserves_config >= self.initial_real_token_reserves_config,
            ContractError::ValueInvalid
        );
        require!(
            self.last_virtual_sol_reserves >= self.initial_virtual_sol_reserves_config,
            ContractError::ValueInvalid
        );
        require!(
            self.initial_virtual_sol_reserves_config > 0
                && self.initial_virtual_token_reserves_config > 0,
            ContractError::ValueInvalid
        );

        Ok(())
    }
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
                        return Err(ValueTooSmall.into());
                    }
                }
                if let Some(max) = max {
                    if value > max {
                        return Err(ValueTooLarge.into());
                    }
                }
                Ok(())
            }
            Self::Enum(options) => {
                if options.contains(value) {
                    Ok(())
                } else {
                    Err(ValueInvalid.into())
                }
            }
        }
    }
}
