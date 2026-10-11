use crate::constants::{
    BONDING_CURVE, CONFIG, GLOBAL, METEORA_PROGRAM, METEORA_VAULT_PROGRAM, MIGRATION_FLAT_FEE,
    MIGRATION_RENT_RESERVE, QUOTE_MINT, TOKEN_VAULT_SEED,
};
use crate::events::MigrationEvent;
use crate::state::{bondingcurve::*, meteora::get_pool_create_ix_data};
use crate::utils::{bps_mul, sol_transfer_with_signer};
use crate::{errors::ContractError, state::config::*};
use anchor_lang::prelude::*;
use anchor_lang::solana_program::program::invoke_signed;
use anchor_lang::solana_program::{instruction::Instruction, system_instruction};
use anchor_spl::token::{self, Mint, Token, TokenAccount, Transfer};

#[derive(Accounts)]
pub struct InitializePoolWithConfig<'info> {
    #[account(
        seeds = [CONFIG.as_bytes()],
        bump,
    )]
    global_config: Box<Account<'info, Config>>,

    /// CHECK: must equal global_config.team_wallet
    #[account(
        mut,
        constraint = global_config.team_wallet == team_wallet.key() @ContractError::IncorrectTeamWallet
    )]
    pub team_wallet: AccountInfo<'info>,

    /// CHECK: must equal global_config.team_wallet_secondary
    #[account(
        mut,
        constraint = global_config.team_wallet_secondary == team_wallet_secondary.key() @ContractError::IncorrectTeamWallet
    )]
    pub team_wallet_secondary: AccountInfo<'info>,

    /// CHECK: must equal bonding_curve.creator
    #[account(mut,
        constraint = bonding_curve.creator == creator.key() @ContractError::IncorrectAuthority)]
    pub creator: AccountInfo<'info>,

    pub token_mint: Box<Account<'info, Mint>>,

    #[account(
        mut,
        seeds = [BONDING_CURVE.as_bytes(), &token_mint.key().to_bytes()],
        bump
    )]
    bonding_curve: Account<'info, BondingCurve>,

    #[account(mut)]
    /// CHECK: Pool account (Meteora PDA)
    pub pool: UncheckedAccount<'info>,
    /// CHECK: Meteora fee config
    pub config: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Pool LP mint (Meteora)
    pub lp_mint: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: token A vault LP account (Meteora)
    pub a_vault_lp: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: token B vault LP account (Meteora)
    pub b_vault_lp: UncheckedAccount<'info>,

    /// CHECK: token A mint; must be the quote mint (WSOL)
    #[account(constraint = token_a_mint.key() == QUOTE_MINT @ContractError::NotSOL)]
    pub token_a_mint: UncheckedAccount<'info>,

    #[account(mut, constraint = token_b_mint.key() == bonding_curve.token_mint @ContractError::NotBondingCurveMint)]
    pub token_b_mint: Box<Account<'info, Mint>>,

    #[account(mut)]
    /// CHECK: Meteora vault A state
    pub a_vault: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Meteora vault B state
    pub b_vault: UncheckedAccount<'info>,

    #[account(
        mut,
        seeds = [TOKEN_VAULT_SEED.as_bytes(), a_vault.key.as_ref()],
        bump,
        seeds::program = vault_program.key()
    )]
    pub a_token_vault: Box<Account<'info, TokenAccount>>,

    #[account(
        mut,
        seeds = [TOKEN_VAULT_SEED.as_bytes(), b_vault.key.as_ref()],
        bump,
        seeds::program = vault_program.key()
    )]
    pub b_token_vault: Box<Account<'info, TokenAccount>>,

    #[account(mut)]
    /// CHECK: Meteora vault A LP mint
    pub a_vault_lp_mint: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: Meteora vault B LP mint
    pub b_vault_lp_mint: UncheckedAccount<'info>,

    /// CHECK: global vault pda which stores SOL
    #[account(
        mut,
        seeds = [GLOBAL.as_bytes()],
        bump,
    )]
    pub global_vault: AccountInfo<'info>,

    /// CHECK: ata of global vault (source of the migrated tokens)
    #[account(
        mut,
        seeds = [
            global_vault.key().as_ref(),
            anchor_spl::token::spl_token::ID.as_ref(),
            token_mint.key().as_ref(),
        ],
        bump,
        seeds::program = anchor_spl::associated_token::ID
    )]
    global_token_account: AccountInfo<'info>,

    #[account(mut)]
    /// CHECK: liquidity bootstrap account (token A / WSOL)
    pub payer_token_a: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: liquidity bootstrap account (token B)
    pub payer_token_b: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: LP receiving account
    pub payer_pool_lp: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: protocol fee token A account
    pub protocol_token_a_fee: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK: protocol fee token B account
    pub protocol_token_b_fee: UncheckedAccount<'info>,

    // NEW-03: the payer (which controls the transient liquidity/LP accounts) must be the
    // migration authority itself, not an arbitrary third party.
    #[account(mut, constraint = payer.key() == authority.key() @ContractError::InvalidMigrationAuthority)]
    pub payer: Signer<'info>,

    #[account(mut, constraint = authority.key() == global_config.migration_authority @ContractError::InvalidMigrationAuthority)]
    pub authority: Signer<'info>,

    #[account(mut)]
    /// CHECK: LP mint metadata PDA (Metaplex checks it)
    pub mint_metadata: UncheckedAccount<'info>,
    /// CHECK: rent sysvar
    pub rent: UncheckedAccount<'info>,
    /// CHECK: metadata program
    pub metadata_program: UncheckedAccount<'info>,
    /// CHECK: SEC-07 — address-checked Meteora vault program
    #[account(address = METEORA_VAULT_PROGRAM @ContractError::InvalidMeteoraVaultProgram)]
    pub vault_program: UncheckedAccount<'info>,
    pub token_program: Program<'info, Token>,
    /// CHECK: associated token program
    pub associated_token_program: UncheckedAccount<'info>,
    system_program: Program<'info, System>,
    /// CHECK: Meteora event authority
    pub event_authority: UncheckedAccount<'info>,

    /// CHECK: SEC-08 — address-checked Meteora AMM program
    #[account(mut, address = METEORA_PROGRAM @ContractError::InvalidMeteoraProgram)]
    pub meteora_program: AccountInfo<'info>,
}

pub fn initialize_pool_with_config(ctx: Context<InitializePoolWithConfig>) -> Result<()> {
    // SEC-06: idempotency — a curve can be migrated exactly once.
    require!(
        !ctx.accounts.bonding_curve.is_migrated,
        ContractError::AmmAlreadyExists
    );
    require!(
        ctx.accounts.bonding_curve.is_completed,
        ContractError::NotCompleted
    );

    // NEW-01: read migration amounts + fees from the PER-CURVE SNAPSHOT, not the mutable config.
    let bc_snapshot_token = ctx.accounts.bonding_curve.meteora_token_reserves;
    let bc_snapshot_sol = ctx.accounts.bonding_curve.meteora_sol_amount;
    let real_sol = ctx.accounts.bonding_curve.real_sol_reserves;

    let platform_fee_lamports = bps_mul(
        ctx.accounts.bonding_curve.migration_platform_fee,
        real_sol,
        10_000,
    )
    .ok_or(ContractError::ArithmeticError)?;
    let platform_fee_secondary_lamports = bps_mul(
        ctx.accounts.bonding_curve.migration_platform_fee_secondary,
        real_sol,
        10_000,
    )
    .ok_or(ContractError::ArithmeticError)?;
    let creator_fee_lamports = bps_mul(
        ctx.accounts.bonding_curve.migration_creator_fee,
        real_sol,
        10_000,
    )
    .ok_or(ContractError::ArithmeticError)?;

    // SEC-03: checked arithmetic with named constants (was bare `2_000_000_00` / `70_000_000`).
    let check_sol_amount = real_sol
        .checked_sub(MIGRATION_RENT_RESERVE)
        .and_then(|v| v.checked_sub(MIGRATION_FLAT_FEE))
        .and_then(|v| v.checked_sub(platform_fee_lamports))
        .and_then(|v| v.checked_sub(platform_fee_secondary_lamports))
        .and_then(|v| v.checked_sub(creator_fee_lamports))
        .ok_or(ContractError::ArithmeticError)?;
    require!(
        check_sol_amount >= bc_snapshot_sol,
        ContractError::ArithmeticError
    );

    let token_a_amount = bc_snapshot_sol;
    let token_b_amount = bc_snapshot_token;

    let signer_seeds: &[&[&[u8]]] = &[&[GLOBAL.as_bytes(), &[ctx.bumps.global_vault]]];

    // move migrated tokens out of the global ATA
    token::transfer(
        CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            Transfer {
                from: ctx.accounts.global_token_account.to_account_info(),
                to: ctx.accounts.payer_token_b.to_account_info(),
                authority: ctx.accounts.global_vault.to_account_info(),
            },
            signer_seeds,
        ),
        token_b_amount,
    )?;

    // wrap SOL into the token-A (WSOL) account
    let sol_ix = system_instruction::transfer(
        ctx.accounts.global_vault.key,
        ctx.accounts.payer_token_a.key,
        token_a_amount,
    );
    invoke_signed(
        &sol_ix,
        &[
            ctx.accounts.global_vault.to_account_info(),
            ctx.accounts.payer_token_a.to_account_info(),
            ctx.accounts.system_program.to_account_info(),
        ],
        signer_seeds,
    )?;
    token::sync_native(CpiContext::new_with_signer(
        ctx.accounts.token_program.to_account_info(),
        token::SyncNative {
            account: ctx.accounts.payer_token_a.to_account_info(),
        },
        signer_seeds,
    ))?;

    // --- Meteora: create the permissionless constant-product pool via CPI ---
    let mut accounts = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new_readonly(ctx.accounts.config.key(), false),
        AccountMeta::new(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new_readonly(ctx.accounts.token_a_mint.key(), false),
        AccountMeta::new_readonly(ctx.accounts.token_b_mint.key(), false),
        AccountMeta::new(ctx.accounts.a_vault.key(), false),
        AccountMeta::new(ctx.accounts.b_vault.key(), false),
        AccountMeta::new(ctx.accounts.a_token_vault.key(), false),
        AccountMeta::new(ctx.accounts.b_token_vault.key(), false),
        AccountMeta::new(ctx.accounts.a_vault_lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.b_vault_lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.a_vault_lp.key(), false),
        AccountMeta::new(ctx.accounts.b_vault_lp.key(), false),
        AccountMeta::new(ctx.accounts.payer_token_a.key(), false),
        AccountMeta::new(ctx.accounts.payer_token_b.key(), false),
        AccountMeta::new(ctx.accounts.payer_pool_lp.key(), false),
        AccountMeta::new(ctx.accounts.protocol_token_a_fee.key(), false),
        AccountMeta::new(ctx.accounts.protocol_token_b_fee.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true),
        AccountMeta::new_readonly(ctx.accounts.rent.key(), false),
        AccountMeta::new(ctx.accounts.mint_metadata.key(), false),
        AccountMeta::new_readonly(ctx.accounts.metadata_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.vault_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.token_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.associated_token_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.system_program.key(), false),
    ];
    accounts.extend(ctx.remaining_accounts.iter().map(|acc| AccountMeta {
        pubkey: *acc.key,
        is_signer: false,
        is_writable: true,
    }));

    let data = get_pool_create_ix_data(token_a_amount, token_b_amount);
    let instruction = Instruction {
        program_id: ctx.accounts.meteora_program.key(),
        accounts,
        data,
    };
    invoke_signed(
        &instruction,
        &[
            ctx.accounts.pool.to_account_info(),
            ctx.accounts.config.to_account_info(),
            ctx.accounts.lp_mint.to_account_info(),
            ctx.accounts.token_a_mint.to_account_info(),
            ctx.accounts.token_b_mint.to_account_info(),
            ctx.accounts.a_vault.to_account_info(),
            ctx.accounts.b_vault.to_account_info(),
            ctx.accounts.a_token_vault.to_account_info(),
            ctx.accounts.b_token_vault.to_account_info(),
            ctx.accounts.a_vault_lp_mint.to_account_info(),
            ctx.accounts.b_vault_lp_mint.to_account_info(),
            ctx.accounts.a_vault_lp.to_account_info(),
            ctx.accounts.b_vault_lp.to_account_info(),
            ctx.accounts.payer_token_a.to_account_info(),
            ctx.accounts.payer_token_b.to_account_info(),
            ctx.accounts.payer_pool_lp.to_account_info(),
            ctx.accounts.protocol_token_a_fee.to_account_info(),
            ctx.accounts.protocol_token_b_fee.to_account_info(),
            ctx.accounts.payer.to_account_info(),
            ctx.accounts.rent.to_account_info(),
            ctx.accounts.mint_metadata.to_account_info(),
            ctx.accounts.metadata_program.to_account_info(),
            ctx.accounts.vault_program.to_account_info(),
            ctx.accounts.token_program.to_account_info(),
            ctx.accounts.associated_token_program.to_account_info(),
            ctx.accounts.system_program.to_account_info(),
            ctx.accounts.event_authority.to_account_info(),
        ],
        signer_seeds,
    )?;

    // flat migration fee
    let flat_ix = system_instruction::transfer(
        ctx.accounts.global_vault.key,
        ctx.accounts.team_wallet.key,
        MIGRATION_FLAT_FEE,
    );
    invoke_signed(
        &flat_ix,
        &[
            ctx.accounts.global_vault.to_account_info(),
            ctx.accounts.team_wallet.to_account_info(),
            ctx.accounts.system_program.to_account_info(),
        ],
        signer_seeds,
    )?;

    if platform_fee_lamports > 0 {
        sol_transfer_with_signer(
            ctx.accounts.global_vault.to_account_info(),
            ctx.accounts.team_wallet.to_account_info(),
            &ctx.accounts.system_program,
            signer_seeds,
            platform_fee_lamports,
        )?;
    }
    if platform_fee_secondary_lamports > 0 {
        sol_transfer_with_signer(
            ctx.accounts.global_vault.to_account_info(),
            ctx.accounts.team_wallet_secondary.to_account_info(),
            &ctx.accounts.system_program,
            signer_seeds,
            platform_fee_secondary_lamports,
        )?;
    }
    if creator_fee_lamports > 0 {
        sol_transfer_with_signer(
            ctx.accounts.global_vault.to_account_info(),
            ctx.accounts.creator.to_account_info(),
            &ctx.accounts.system_program,
            signer_seeds,
            creator_fee_lamports,
        )?;
    }

    // SEC-06: mark migrated and reconcile the ledger with what physically left the vault.
    let vault_outflow = token_a_amount
        .checked_add(MIGRATION_FLAT_FEE)
        .and_then(|v| v.checked_add(platform_fee_lamports))
        .and_then(|v| v.checked_add(platform_fee_secondary_lamports))
        .and_then(|v| v.checked_add(creator_fee_lamports))
        .ok_or(ContractError::ArithmeticError)?;
    let bc = &mut ctx.accounts.bonding_curve;
    bc.real_sol_reserves = bc
        .real_sol_reserves
        .checked_sub(vault_outflow)
        .ok_or(ContractError::ArithmeticError)?;
    bc.real_token_reserves = 0;
    bc.is_migrated = true;

    emit!(MigrationEvent {
        mint: ctx.accounts.token_mint.key(),
        bonding_curve: ctx.accounts.bonding_curve.key(),
        pool: ctx.accounts.pool.key(),
        lp_mint: ctx.accounts.lp_mint.key(),
        sol_amount: token_a_amount,
        token_amount: token_b_amount,
        timestamp: Clock::get()?.unix_timestamp,
    });

    Ok(())
}
