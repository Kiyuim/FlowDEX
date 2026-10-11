use crate::constants::{LAMPORT_SCALE, TOKEN_SCALE};
use crate::errors::*;
use crate::events::CompleteEvent;
use crate::events::SwapEvent;
use crate::state::config::*;
use crate::utils::*;
use anchor_lang::{prelude::*, AnchorDeserialize, AnchorSerialize};
use anchor_spl::token::Mint;
use anchor_spl::token::Token;

#[account]
#[derive(InitSpace)]
pub struct BondingCurve {
    pub token_mint: Pubkey,
    pub creator: Pubkey,

    pub token_total_supply: u64,

    pub virtual_sol_reserves: u64,
    pub virtual_token_reserves: u64,
    pub last_virtual_sol_reserves: u64,

    pub real_sol_reserves: u64,
    pub real_token_reserves: u64,

    pub is_completed: bool,

    // NEW-01 + SEC-06: migration parameters are SNAPSHOTTED here at creation, so a later
    // configure() change can never brick or re-price an already-created curve's migration.
    pub is_migrated: bool,
    pub meteora_token_reserves: u64,
    pub meteora_sol_amount: u64,
    pub migration_platform_fee: u64,
    pub migration_platform_fee_secondary: u64,
    pub migration_creator_fee: u64,

    // padding for forward-compatible upgrades without a breaking realloc
    pub _reserved: [u8; 64],
}

// SEC-11: swap direction is a validated enum, not a bare u8 with an `else` catch-all.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum SwapDirection {
    Buy,
    Sell,
}

impl TryFrom<u8> for SwapDirection {
    type Error = anchor_lang::error::Error;
    fn try_from(v: u8) -> Result<Self> {
        match v {
            0 => Ok(SwapDirection::Buy),
            1 => Ok(SwapDirection::Sell),
            _ => Err(ContractError::InvalidDirection.into()),
        }
    }
}

#[derive(Debug, Clone)]
pub struct SellResult {
    pub token_amount: u64,
    pub sol_amount: u64,
}

#[derive(Debug, Clone)]
pub struct BuyResult {
    pub token_amount: u64,
    pub sol_amount: u64,
}

pub trait BondingCurveAccount<'info> {
    #[allow(clippy::too_many_arguments)]
    fn swap(
        &mut self,
        global_config: &Account<'info, Config>,
        token_mint: &Account<'info, Mint>,
        global_ata: &mut AccountInfo<'info>,
        user_ata: &mut AccountInfo<'info>,
        source: &mut AccountInfo<'info>,
        team_wallet: &mut AccountInfo<'info>,
        team_wallet_secondary: &mut AccountInfo<'info>,
        creator: &mut AccountInfo<'info>,

        amount: u64,
        direction: u8,
        minimum_receive_amount: u64,
        max_sol_cost: u64,

        user: &Signer<'info>,
        signer: &[&[&[u8]]],

        token_program: &Program<'info, Token>,
        system_program: &Program<'info, System>,
    ) -> Result<()>;

    fn apply_sell(&mut self, token_amount: u64) -> Option<SellResult>;
    fn apply_buy(&mut self, sol_amount: u64) -> Option<BuyResult>;
    fn get_sol_for_sell_tokens(&self, token_amount: u64) -> Option<u64>;
    fn get_tokens_for_buy_sol(&self, sol_amount: u64) -> Option<u64>;
}

impl<'info> BondingCurveAccount<'info> for Account<'info, BondingCurve> {
    fn swap(
        &mut self,
        global_config: &Account<'info, Config>,
        token_mint: &Account<'info, Mint>,
        global_ata: &mut AccountInfo<'info>,
        user_ata: &mut AccountInfo<'info>,
        source: &mut AccountInfo<'info>,
        team_wallet: &mut AccountInfo<'info>,
        team_wallet_secondary: &mut AccountInfo<'info>,
        creator: &mut AccountInfo<'info>,

        amount: u64,
        direction: u8,
        minimum_receive_amount: u64,
        max_sol_cost: u64,

        user: &Signer<'info>,
        signer: &[&[&[u8]]],

        token_program: &Program<'info, Token>,
        system_program: &Program<'info, System>,
    ) -> Result<()> {
        require!(amount > 0, ContractError::InvalidAmount);

        // SEC-11: reject any out-of-range direction instead of silently treating it as a buy.
        let dir = SwapDirection::try_from(direction)?;

        let sol_amount: u64;
        let token_amount: u64;

        if dir == SwapDirection::Sell {
            let sell_result = self.apply_sell(amount).ok_or(ContractError::SellFailed)?;
            sol_amount = sell_result.sol_amount;
            token_amount = sell_result.token_amount;

            let platform_fee_lamports =
                bps_mul(global_config.platform_sell_fee, sell_result.sol_amount, 10_000)
                    .ok_or(ContractError::ArithmeticError)?;
            let platform_fee_secondary_lamports = bps_mul(
                global_config.platform_sell_fee_secondary,
                sell_result.sol_amount,
                10_000,
            )
            .ok_or(ContractError::ArithmeticError)?;
            let creator_fee_lamports = bps_mul(
                global_config.creator_sell_fee,
                sell_result.sol_amount,
                10_000,
            )
            .ok_or(ContractError::ArithmeticError)?;

            // SEC-03: checked subtraction — impossible-but-safe, and returns an error code
            // instead of panicking (fees are already validated <= 100% in configure).
            let sell_amount_minus_fee = sell_result
                .sol_amount
                .checked_sub(platform_fee_lamports)
                .and_then(|v| v.checked_sub(platform_fee_secondary_lamports))
                .and_then(|v| v.checked_sub(creator_fee_lamports))
                .ok_or(ContractError::ArithmeticError)?;

            require!(
                sell_amount_minus_fee >= minimum_receive_amount,
                ContractError::SlippageExceeded
            );

            token_transfer_user(
                user_ata.clone(),
                user,
                global_ata.clone(),
                token_program,
                sell_result.token_amount,
            )?;

            sol_transfer_with_signer(
                source.clone(),
                user.to_account_info(),
                system_program,
                signer,
                sell_amount_minus_fee,
            )?;

            if platform_fee_lamports > 0 {
                sol_transfer_with_signer(
                    source.clone(),
                    team_wallet.clone(),
                    system_program,
                    signer,
                    platform_fee_lamports,
                )?;
            }
            if platform_fee_secondary_lamports > 0 {
                sol_transfer_with_signer(
                    source.clone(),
                    team_wallet_secondary.clone(),
                    system_program,
                    signer,
                    platform_fee_secondary_lamports,
                )?;
            }
            if creator_fee_lamports > 0 {
                sol_transfer_with_signer(
                    source.clone(),
                    creator.clone(),
                    system_program,
                    signer,
                    creator_fee_lamports,
                )?;
            }
        } else {
            // ---- buy ----
            let platform_fee_lamports = bps_mul(global_config.platform_buy_fee, amount, 10_000)
                .ok_or(ContractError::ArithmeticError)?;
            let platform_fee_secondary_lamports =
                bps_mul(global_config.platform_buy_fee_secondary, amount, 10_000)
                    .ok_or(ContractError::ArithmeticError)?;
            let creator_fee_lamports = bps_mul(global_config.creator_buy_fee, amount, 10_000)
                .ok_or(ContractError::ArithmeticError)?;

            // SEC-03: checked subtraction (cannot underflow now that fees are validated).
            let buy_amount_applied = amount
                .checked_sub(platform_fee_lamports)
                .and_then(|v| v.checked_sub(platform_fee_secondary_lamports))
                .and_then(|v| v.checked_sub(creator_fee_lamports))
                .ok_or(ContractError::ArithmeticError)?;

            let buy_result = self
                .apply_buy(buy_amount_applied)
                .ok_or(ContractError::BuyFailed)?;

            sol_amount = buy_result.sol_amount;
            token_amount = buy_result.token_amount;

            // SEC-02: bound the SOL side. On the completing buy, apply_buy already clamps the
            // debit to <= what the buyer offered; this max_sol_cost check gives the user an
            // explicit, wallet-visible cap on top of that.
            require!(
                buy_result.sol_amount <= max_sol_cost,
                ContractError::MaxSolCostExceeded
            );
            require!(
                buy_result.sol_amount <= buy_amount_applied,
                ContractError::SlippageExceeded
            );
            require!(
                buy_result.token_amount >= minimum_receive_amount,
                ContractError::SlippageExceeded
            );

            if self.is_completed {
                emit!(CompleteEvent {
                    user: user.key(),
                    mint: token_mint.key(),
                    bonding_curve: self.key(),
                    timestamp: Clock::get()?.unix_timestamp,
                });
            }

            token_transfer_with_signer(
                global_ata.clone(),
                source.clone(),
                user_ata.clone(),
                token_program,
                signer,
                buy_result.token_amount,
            )?;

            sol_transfer_from_user(user, source.clone(), system_program, buy_result.sol_amount)?;

            if platform_fee_lamports > 0 {
                sol_transfer_from_user(user, team_wallet.clone(), system_program, platform_fee_lamports)?;
            }
            if platform_fee_secondary_lamports > 0 {
                sol_transfer_from_user(
                    user,
                    team_wallet_secondary.clone(),
                    system_program,
                    platform_fee_secondary_lamports,
                )?;
            }
            if creator_fee_lamports > 0 {
                sol_transfer_from_user(user, creator.clone(), system_program, creator_fee_lamports)?;
            }
        }

        // BIZ-03: emit both virtual and real reserves under correctly-named fields.
        emit!(SwapEvent {
            user: user.key(),
            mint: token_mint.key(),
            bonding_curve: self.key(),
            amount_in: if dir == SwapDirection::Sell { token_amount } else { sol_amount },
            direction,
            minimum_receive_amount,
            amount_out: if dir == SwapDirection::Sell { sol_amount } else { token_amount },
            virtual_sol_reserves: self.virtual_sol_reserves,
            virtual_token_reserves: self.virtual_token_reserves,
            real_sol_reserves: self.real_sol_reserves,
            real_token_reserves: self.real_token_reserves,
            timestamp: Clock::get()?.unix_timestamp,
        });

        Ok(())
    }

    fn get_sol_for_sell_tokens(&self, token_amount: u64) -> Option<u64> {
        if token_amount == 0 {
            return None;
        }
        let current_sol = self.virtual_sol_reserves as u128;
        let current_tokens = (self.virtual_token_reserves as u128)
            .checked_mul(LAMPORT_SCALE)?
            .checked_div(TOKEN_SCALE)?;

        let new_tokens = current_tokens.checked_add(
            (token_amount as u128)
                .checked_mul(LAMPORT_SCALE)?
                .checked_div(TOKEN_SCALE)?,
        )?;

        let numerator = current_sol.checked_mul(current_tokens)?;
        // ceil -> new_sol rounded up -> sol_out rounded down -> favors the protocol
        let new_sol = numerator
            .checked_add(new_tokens.checked_sub(1)?)?
            .checked_div(new_tokens)?;

        let sol_out = current_sol.checked_sub(new_sol)?;
        <u128 as TryInto<u64>>::try_into(sol_out).ok()
    }

    fn get_tokens_for_buy_sol(&self, sol_amount: u64) -> Option<u64> {
        if sol_amount == 0 {
            return None;
        }
        let current_sol = self.virtual_sol_reserves as u128;
        let current_tokens = (self.virtual_token_reserves as u128)
            .checked_mul(LAMPORT_SCALE)?
            .checked_div(TOKEN_SCALE)?;

        let new_sol = current_sol.checked_add(sol_amount as u128)?;
        let new_tokens = current_sol.checked_mul(current_tokens)?.checked_div(new_sol)?;
        let tokens_out = current_tokens.checked_sub(new_tokens)?;

        let tokens_out = tokens_out.checked_mul(TOKEN_SCALE)?.checked_div(LAMPORT_SCALE)?;
        <u128 as TryInto<u64>>::try_into(tokens_out).ok()
    }

    fn apply_buy(&mut self, mut sol_amount: u64) -> Option<BuyResult> {
        let mut token_amount = self.get_tokens_for_buy_sol(sol_amount)?;

        if token_amount >= self.real_token_reserves {
            // Last buy — completes the curve.
            token_amount = self.real_token_reserves;

            let current_virtual_token_reserves = self.virtual_token_reserves;
            let current_virtual_sol_reserves = self.virtual_sol_reserves;

            self.virtual_token_reserves = (current_virtual_token_reserves as u128)
                .checked_sub(token_amount as u128)?
                .try_into()
                .ok()?;
            self.virtual_sol_reserves = self.last_virtual_sol_reserves;

            let recomputed_sol_amount = self.get_sol_for_sell_tokens(token_amount)?;

            self.virtual_token_reserves = current_virtual_token_reserves;
            self.virtual_sol_reserves = current_virtual_sol_reserves;

            // SEC-02: NEVER charge more than the buyer offered. If the graduation top-up
            // implied by last_virtual_sol_reserves would exceed the offer, cap it at the offer.
            sol_amount = recomputed_sol_amount.min(sol_amount);

            self.is_completed = true;
        }

        let new_virtual_token_reserves =
            (self.virtual_token_reserves as u128).checked_sub(token_amount as u128)?;
        let new_real_token_reserves =
            (self.real_token_reserves as u128).checked_sub(token_amount as u128)?;
        let new_virtual_sol_reserves =
            (self.virtual_sol_reserves as u128).checked_add(sol_amount as u128)?;
        let new_real_sol_reserves =
            (self.real_sol_reserves as u128).checked_add(sol_amount as u128)?;

        self.virtual_token_reserves = new_virtual_token_reserves.try_into().ok()?;
        self.real_token_reserves = new_real_token_reserves.try_into().ok()?;
        self.virtual_sol_reserves = new_virtual_sol_reserves.try_into().ok()?;
        self.real_sol_reserves = new_real_sol_reserves.try_into().ok()?;

        Some(BuyResult {
            token_amount,
            sol_amount,
        })
    }

    fn apply_sell(&mut self, token_amount: u64) -> Option<SellResult> {
        let sol_amount = self.get_sol_for_sell_tokens(token_amount)?;

        let new_virtual_token_reserves =
            (self.virtual_token_reserves as u128).checked_add(token_amount as u128)?;
        let new_real_token_reserves =
            (self.real_token_reserves as u128).checked_add(token_amount as u128)?;
        let new_virtual_sol_reserves =
            (self.virtual_sol_reserves as u128).checked_sub(sol_amount as u128)?;
        let new_real_sol_reserves = self.real_sol_reserves.checked_sub(sol_amount)?;

        self.virtual_token_reserves = new_virtual_token_reserves.try_into().ok()?;
        self.real_token_reserves = new_real_token_reserves.try_into().ok()?;
        self.virtual_sol_reserves = new_virtual_sol_reserves.try_into().ok()?;
        self.real_sol_reserves = new_real_sol_reserves;

        Some(SellResult {
            token_amount,
            sol_amount,
        })
    }
}
