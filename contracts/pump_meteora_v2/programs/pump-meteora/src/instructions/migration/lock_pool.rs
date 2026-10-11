use crate::constants::*;
use crate::errors::ContractError;
use crate::events::LpLockedEvent;
use crate::state::meteora::{get_function_hash, get_lock_lp_ix_data};
use crate::state::{bondingcurve::*, config::*};
use anchor_lang::prelude::*;
use anchor_lang::solana_program::{instruction::Instruction, program::invoke_signed};
use anchor_spl::token::Mint;
use anchor_spl::{
    associated_token::{self, AssociatedToken},
    token::{Token, TokenAccount},
};

#[derive(Accounts)]
pub struct LockPool<'info> {
    #[account(
        seeds = [CONFIG.as_bytes()],
        bump,
    )]
    global_config: Box<Account<'info, Config>>,

    #[account(
        mut,
        seeds = [BONDING_CURVE.as_bytes(), &token_mint.key().to_bytes()],
        bump
    )]
    bonding_curve: Account<'info, BondingCurve>,

    pub token_mint: Box<Account<'info, Mint>>,

    /// CHECK: global vault pda which stores SOL
    #[account(
        mut,
        seeds = [GLOBAL.as_bytes()],
        bump,
    )]
    pub global_vault: AccountInfo<'info>,

    #[account(mut)]
    /// CHECK: Meteora pool
    pub pool: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: pool LP mint
    pub lp_mint: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: token A vault LP account
    pub a_vault_lp: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: token B vault LP account
    pub b_vault_lp: UncheckedAccount<'info>,
    /// CHECK: token B mint
    pub token_b_mint: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Meteora vault A state
    pub a_vault: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Meteora vault B state
    pub b_vault: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Meteora vault A LP mint
    pub a_vault_lp_mint: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Meteora vault B LP mint
    pub b_vault_lp_mint: UncheckedAccount<'info>,

    #[account(mut)]
    pub payer_pool_lp: Box<Account<'info, TokenAccount>>,

    #[account(mut)]
    pub payer: Signer<'info>,

    #[account(mut, constraint = authority.key() == global_config.migration_authority @ContractError::InvalidMigrationAuthority)]
    pub authority: Signer<'info>,

    /// CHECK: must equal global_config.migration_wallet
    #[account(
        mut,
        constraint = global_config.migration_wallet == fee_receiver.key() @ContractError::IncorrectAuthority
    )]
    pub fee_receiver: AccountInfo<'info>,

    /// CHECK: must equal bonding_curve.creator
    #[account(
        mut,
        constraint = bonding_curve.creator == creator_receiver.key() @ContractError::IncorrectAuthority
    )]
    pub creator_receiver: AccountInfo<'info>,

    // SEC-07: typed program accounts instead of UncheckedAccount.
    pub token_program: Program<'info, Token>,
    pub associated_token_program: Program<'info, AssociatedToken>,
    pub system_program: Program<'info, System>,

    #[account(mut)]
    /// CHECK: lock escrow (creator)
    pub lock_escrow: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: lock escrow (platform)
    pub lock_escrow1: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: escrow vault
    pub escrow_vault: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: escrow vault
    pub escrow_vault1: UncheckedAccount<'info>,

    /// CHECK: SEC-07 — address-checked Meteora AMM program
    #[account(mut, address = METEORA_PROGRAM @ContractError::InvalidMeteoraProgram)]
    pub meteora_program: AccountInfo<'info>,

    /// CHECK: Meteora event authority
    pub event_authority: AccountInfo<'info>,
}

pub fn lock_pool(ctx: Context<LockPool>) -> Result<()> {
    let signer_seeds: &[&[&[u8]]] = &[&[GLOBAL.as_bytes(), &[ctx.bumps.global_vault]]];
    let meteora_program_id = METEORA_PROGRAM;

    // ---- create the two lock escrows (creator + platform) ----
    let escrow_accounts = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new(ctx.accounts.lock_escrow.key(), false),
        AccountMeta::new_readonly(ctx.accounts.creator_receiver.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true),
        AccountMeta::new_readonly(ctx.accounts.system_program.key(), false),
    ];
    let escrow_accounts1 = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new(ctx.accounts.lock_escrow1.key(), false),
        AccountMeta::new_readonly(ctx.accounts.fee_receiver.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true),
        AccountMeta::new_readonly(ctx.accounts.system_program.key(), false),
    ];
    invoke_signed(
        &Instruction {
            program_id: meteora_program_id,
            accounts: escrow_accounts,
            data: get_function_hash("global", "create_lock_escrow").into(),
        },
        &[
            ctx.accounts.pool.to_account_info(),
            ctx.accounts.lock_escrow.to_account_info(),
            ctx.accounts.creator_receiver.to_account_info(),
            ctx.accounts.lp_mint.to_account_info(),
            ctx.accounts.payer.to_account_info(),
            ctx.accounts.system_program.to_account_info(),
        ],
        signer_seeds,
    )?;
    invoke_signed(
        &Instruction {
            program_id: meteora_program_id,
            accounts: escrow_accounts1,
            data: get_function_hash("global", "create_lock_escrow").into(),
        },
        &[
            ctx.accounts.pool.to_account_info(),
            ctx.accounts.lock_escrow1.to_account_info(),
            ctx.accounts.fee_receiver.to_account_info(),
            ctx.accounts.lp_mint.to_account_info(),
            ctx.accounts.payer.to_account_info(),
            ctx.accounts.system_program.to_account_info(),
        ],
        signer_seeds,
    )?;

    associated_token::create_idempotent(CpiContext::new(
        ctx.accounts.associated_token_program.to_account_info(),
        associated_token::Create {
            payer: ctx.accounts.payer.to_account_info(),
            associated_token: ctx.accounts.escrow_vault.to_account_info(),
            authority: ctx.accounts.lock_escrow.to_account_info(),
            mint: ctx.accounts.lp_mint.to_account_info(),
            token_program: ctx.accounts.token_program.to_account_info(),
            system_program: ctx.accounts.system_program.to_account_info(),
        },
    ))?;
    associated_token::create_idempotent(CpiContext::new(
        ctx.accounts.associated_token_program.to_account_info(),
        associated_token::Create {
            payer: ctx.accounts.payer.to_account_info(),
            associated_token: ctx.accounts.escrow_vault1.to_account_info(),
            authority: ctx.accounts.lock_escrow1.to_account_info(),
            mint: ctx.accounts.lp_mint.to_account_info(),
            token_program: ctx.accounts.token_program.to_account_info(),
            system_program: ctx.accounts.system_program.to_account_info(),
        },
    ))?;

    // ---- lock the LP in two halves ----
    // SEC-12: use the live balance for each lock (reload between them) so no LP dust is
    // stranded on odd balances, instead of reusing a stale `amount / 2` twice.
    let total_lp = ctx.accounts.payer_pool_lp.amount;
    let first_half = total_lp / 2;

    lock_once(&ctx, signer_seeds, &ctx.accounts.lock_escrow, &ctx.accounts.escrow_vault, first_half)?;

    ctx.accounts.payer_pool_lp.reload()?;
    let remaining = ctx.accounts.payer_pool_lp.amount;
    lock_once(&ctx, signer_seeds, &ctx.accounts.lock_escrow1, &ctx.accounts.escrow_vault1, remaining)?;

    emit!(LpLockedEvent {
        mint: ctx.accounts.token_mint.key(),
        pool: ctx.accounts.pool.key(),
        lp_amount_locked: total_lp,
        timestamp: Clock::get()?.unix_timestamp,
    });
    Ok(())
}

fn lock_once<'info>(
    ctx: &Context<LockPool<'info>>,
    signer_seeds: &[&[&[u8]]],
    lock_escrow: &UncheckedAccount<'info>,
    escrow_vault: &UncheckedAccount<'info>,
    amount: u64,
) -> Result<()> {
    let source_tokens = &ctx.accounts.payer_pool_lp;
    let lock_accounts = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(lock_escrow.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true),
        AccountMeta::new(source_tokens.key(), false),
        AccountMeta::new(escrow_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.token_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault_lp.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault_lp.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault_lp_mint.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault_lp_mint.key(), false),
    ];
    invoke_signed(
        &Instruction {
            program_id: METEORA_PROGRAM,
            accounts: lock_accounts,
            data: get_lock_lp_ix_data(amount),
        },
        &[
            ctx.accounts.pool.to_account_info(),
            ctx.accounts.lp_mint.to_account_info(),
            lock_escrow.to_account_info(),
            ctx.accounts.payer.to_account_info(),
            source_tokens.to_account_info(),
            escrow_vault.to_account_info(),
            ctx.accounts.token_program.to_account_info(),
            ctx.accounts.a_vault.to_account_info(),
            ctx.accounts.b_vault.to_account_info(),
            ctx.accounts.a_vault_lp.to_account_info(),
            ctx.accounts.b_vault_lp.to_account_info(),
            ctx.accounts.a_vault_lp_mint.to_account_info(),
            ctx.accounts.b_vault_lp_mint.to_account_info(),
        ],
        signer_seeds,
    )?;
    Ok(())
}
