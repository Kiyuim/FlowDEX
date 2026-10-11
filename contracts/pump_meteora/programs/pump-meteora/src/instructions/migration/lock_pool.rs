use crate::constants::*;
use crate::errors::ContractError;
use crate::state::{bondingcurve::*,config::*};
use crate::state::meteora::{get_function_hash, get_lock_lp_ix_data};
use anchor_lang::prelude::*;
use anchor_lang::solana_program::{instruction::Instruction, program::invoke_signed};
use anchor_spl::{associated_token, token::TokenAccount};
use anchor_spl::token:: Mint;
use std::str::FromStr;

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


    //TODO: meteora 池？
    #[account(mut)]
    /// CHECK: Pool account (PDA address)
    pub pool: UncheckedAccount<'info>,


    //TODO: LP mint （类似于token mint）?
    #[account(mut)]
    /// CHECK: Pool LP Token Mint (AMM 池级别的 LP Token)
    /// Meteora AMM 池的 LP Token Mint 账户，用于定义和 mint/burn Pool LP Token
    /// 作用：
    /// - 定义 Pool LP Token 的类型（SPL Token Mint）
    /// - 当用户向池子提供流动性时，会 mint Pool LP Token 给用户
    /// - 当用户从池子移除流动性时，会 burn 用户的 Pool LP Token
    /// - 代表用户在 AMM 池中的流动性份额
    /// 创建逻辑：PDA [SEEDS.LP_MINT, pool.key()] 由 Meteora AMM 程序派生
    /// 
    /// 与 a_vault_lp_mint / b_vault_lp_mint 的区别：
    /// - lp_mint: 池子级别的 LP Token，用户持有，代表用户在 AMM 池中的流动性份额
    /// - a_vault_lp_mint / b_vault_lp_mint: Vault 级别的 LP Token，池子持有，代表池子在 Vault 中的代币份额
    pub lp_mint: UncheckedAccount<'info>,

    //TODO: ? 
    #[account(mut)]
    /// CHECK: Token A Vault LP Token Account
    /// Vault LP Token 账户，用于存储 Token A (SOL/WSOL) 的 Vault LP Token
    /// 当池子向 a_vault 存入/提取代币时，Vault 系统会 mint/burn Vault LP Token 作为凭证
    /// 创建逻辑：PDA [a_vault.key(), pool.key()] 由 Meteora AMM 程序派生
    pub a_vault_lp: UncheckedAccount<'info>,

    //TODO: ? 
    #[account(mut)]
    /// CHECK: Token B Vault LP Token Account
    /// Vault LP Token 账户，用于存储 Token B (代币) 的 Vault LP Token
    /// 当池子向 b_vault 存入/提取代币时，Vault 系统会 mint/burn Vault LP Token 作为凭证
    /// 创建逻辑：PDA [b_vault.key(), pool.key()] 由 Meteora AMM 程序派生
    pub b_vault_lp: UncheckedAccount<'info>,


    /// CHECK: Token B mint
    pub token_b_mint: UncheckedAccount<'info>,

    //TODO: ? 
    #[account(mut)]
    /// CHECK: Vault state account for Token A (SOL/WSOL)
    /// Vault 状态账户，由 Meteora Vault 程序管理（程序地址：24Uqj9JCLxUeoC3hGfh5W3s9FM9uCHDS2SG3LYwBpyTi）
    /// 作用：
    /// - 存储 vault 的配置和状态信息
    /// - 管理 Token A 的存入和提取逻辑
    /// - 提供代币管理的抽象层，支持收益聚合等功能
    /// - 通过 Vault 程序间接控制 a_token_vault（实际代币存储账户）
    /// 关系：a_vault (状态账户) → Vault 程序 → a_token_vault (实际代币存储)
    pub a_vault: UncheckedAccount<'info>,

    //TODO: ? 
    #[account(mut)]
    /// CHECK: Vault state account for Token B (代币)
    /// Vault 状态账户，由 Meteora Vault 程序管理（程序地址：24Uqj9JCLxUeoC3hGfh5W3s9FM9uCHDS2SG3LYwBpyTi）
    /// 作用：
    /// - 存储 vault 的配置和状态信息
    /// - 管理 Token B 的存入和提取逻辑
    /// - 提供代币管理的抽象层，支持收益聚合等功能
    /// - 通过 Vault 程序间接控制 b_token_vault（实际代币存储账户）
    /// 关系：b_vault (状态账户) → Vault 程序 → b_token_vault (实际代币存储)
    pub b_vault: UncheckedAccount<'info>,

    //TODO: ? 
    #[account(mut)]
    /// CHECK: Vault LP Token Mint for Token A (SOL/WSOL)
    /// Vault LP Token 的 Mint 账户，用于定义和 mint/burn Token A 的 Vault LP Token
    /// 作用：
    /// - 定义 Vault LP Token 的类型（类似 SPL Token Mint）
    /// - 当池子向 a_vault 存入代币时，Vault 系统通过此 mint 账户 mint Vault LP Token 到 a_vault_lp
    /// - 当池子从 a_vault 提取代币时，Vault 系统通过此 mint 账户 burn a_vault_lp 中的 Vault LP Token
    /// - a_vault_lp 账户的 mint 字段指向此账户
    /// 创建逻辑：由 Vault 程序在创建 a_vault 时生成，存储在 a_vault.lpMint 字段中
    pub a_vault_lp_mint: UncheckedAccount<'info>,

    //TODO: ? 
    #[account(mut)]
    /// CHECK: Vault LP Token Mint for Token B (代币)
    /// Vault LP Token 的 Mint 账户，用于定义和 mint/burn Token B 的 Vault LP Token
    /// 作用：
    /// - 定义 Vault LP Token 的类型（类似 SPL Token Mint）
    /// - 当池子向 b_vault 存入代币时，Vault 系统通过此 mint 账户 mint Vault LP Token 到 b_vault_lp
    /// - 当池子从 b_vault 提取代币时，Vault 系统通过此 mint 账户 burn b_vault_lp 中的 Vault LP Token
    /// - b_vault_lp 账户的 mint 字段指向此账户
    /// 创建逻辑：由 Vault 程序在创建 b_vault 时生成，存储在 b_vault.lpMint 字段中
    pub b_vault_lp_mint: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Accounts to bootstrap the pool with initial liquidity
    pub payer_pool_lp: Box<Account<'info, TokenAccount>>,

    #[account(mut)]
    pub payer: Signer<'info>,

    #[account(mut)]
    pub authority: Signer<'info>,

    /// CHECK: should be same with the address in the global_config
    #[account(
        mut,
        constraint = global_config.migration_wallet == fee_receiver.key() @ContractError::IncorrectAuthority
    )]
    pub fee_receiver: AccountInfo<'info>,

    /// CHECK: should be same with the address in the bonding_curve
    #[account(
        mut,
        constraint = bonding_curve.creator == creator_receiver.key() @ContractError::IncorrectAuthority
    )]
    pub creator_receiver: AccountInfo<'info>,

    /// CHECK: Token program account
    pub token_program: UncheckedAccount<'info>,
    /// CHECK: Associated token program account
    pub associated_token_program: UncheckedAccount<'info>,
    /// CHECK: System program account
    pub system_program: UncheckedAccount<'info>,
    #[account(mut)]
    /// CHECK lock escrow
    pub lock_escrow: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK lock escrow
    pub lock_escrow1: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Escrow vault
    pub escrow_vault: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Escrow vault
    pub escrow_vault1: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK:
    pub meteora_program: AccountInfo<'info>,

    /// CHECK: Meteora Event Autority
    pub event_authority: AccountInfo<'info>,
}

pub fn lock_pool(ctx: Context<LockPool>) -> Result<()> {
    require!(
        ctx.accounts.authority.key() == ctx.accounts.global_config.migration_authority.key(),
        ContractError::InvalidMigrationAuthority
    );

    require!(
        ctx.accounts.meteora_program.key() == Pubkey::from_str(METEORA_PROGRAM_KEY).unwrap(),
        ContractError::InvalidMeteoraProgram
    );

    let signer_seeds: &[&[&[u8]]] = &[&[
        GLOBAL.as_bytes(),
        &[ctx.bumps.global_vault],
    ]];

    let meteora_program_id: Pubkey = Pubkey::from_str(METEORA_PROGRAM_KEY).unwrap();
    let source_tokens = ctx.accounts.payer_pool_lp.clone();
    let lp_mint_amount = ctx.accounts.payer_pool_lp.amount / 2;

    // Create Lock Escrow
    let escrow_accounts = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new(ctx.accounts.lock_escrow.key(), false),
        AccountMeta::new_readonly(ctx.accounts.creator_receiver.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true), // Bonding Curve Sol Escrow is the payer/signer
        AccountMeta::new_readonly(ctx.accounts.system_program.key(), false),
    ];

    let escrow_accounts1 = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new(ctx.accounts.lock_escrow1.key(), false),
        AccountMeta::new_readonly(ctx.accounts.fee_receiver.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true), // Bonding Curve Sol Escrow is the payer/signer
        AccountMeta::new_readonly(ctx.accounts.system_program.key(), false),
    ];

    let escrow_instruction = Instruction {
        program_id: meteora_program_id,
        accounts: escrow_accounts,
        data: get_function_hash("global", "create_lock_escrow").into(),
    };

    let escrow_instruction1 = Instruction {
        program_id: meteora_program_id,
        accounts: escrow_accounts1,
        data: get_function_hash("global", "create_lock_escrow").into(),
    };

    invoke_signed(
        &escrow_instruction,
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
        &escrow_instruction1,
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

    // Lock Pool
    let lock_accounts = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.lock_escrow.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true), // Bonding Curve Sol Escrow is the payer/signer
        AccountMeta::new(source_tokens.key(), false),
        AccountMeta::new(ctx.accounts.escrow_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.token_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault_lp.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault_lp.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault_lp_mint.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault_lp_mint.key(), false),
    ];

    let lock_accounts1 = vec![
        AccountMeta::new(ctx.accounts.pool.key(), false),
        AccountMeta::new_readonly(ctx.accounts.lp_mint.key(), false),
        AccountMeta::new(ctx.accounts.lock_escrow1.key(), false),
        AccountMeta::new(ctx.accounts.payer.key(), true), // Bonding Curve Sol Escrow is the payer/signer
        AccountMeta::new(source_tokens.key(), false),
        AccountMeta::new(ctx.accounts.escrow_vault1.key(), false),
        AccountMeta::new_readonly(ctx.accounts.token_program.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault_lp.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault_lp.key(), false),
        AccountMeta::new_readonly(ctx.accounts.a_vault_lp_mint.key(), false),
        AccountMeta::new_readonly(ctx.accounts.b_vault_lp_mint.key(), false),
    ];

    let lock_instruction = Instruction {
        program_id: meteora_program_id,
        accounts: lock_accounts,
        data: get_lock_lp_ix_data(lp_mint_amount),
    };

    let lock_instruction1 = Instruction {
        program_id: meteora_program_id,
        accounts: lock_accounts1,
        data: get_lock_lp_ix_data(lp_mint_amount),
    };

    invoke_signed(
        &lock_instruction,
        &[
            ctx.accounts.pool.to_account_info(),
            ctx.accounts.lp_mint.to_account_info(),
            ctx.accounts.lock_escrow.to_account_info(),
            ctx.accounts.payer.to_account_info(), // Bonding Curve Sol Escrow is the payer/signer
            source_tokens.to_account_info(),
            ctx.accounts.escrow_vault.to_account_info(),
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

    invoke_signed(
        &lock_instruction1,
        &[
            ctx.accounts.pool.to_account_info(),
            ctx.accounts.lp_mint.to_account_info(),
            ctx.accounts.lock_escrow1.to_account_info(),
            ctx.accounts.payer.to_account_info(), // Bonding Curve Sol Escrow is the payer/signer
            source_tokens.to_account_info(),
            ctx.accounts.escrow_vault1.to_account_info(),
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
