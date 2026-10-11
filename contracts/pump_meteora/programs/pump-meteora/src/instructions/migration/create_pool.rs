use crate::constants::{METEORA_PROGRAM_KEY,CONFIG,  BONDING_CURVE, QUOTE_MINT, GLOBAL, TOKEN_VAULT_SEED};
use crate::state::{bondingcurve::*, meteora::get_pool_create_ix_data};
use crate::utils::{bps_mul, sol_transfer_with_signer};
use crate::{errors::ContractError, state::config::*};
use anchor_lang::prelude::*;
use anchor_lang::solana_program::program::invoke_signed;
use anchor_lang::solana_program::{instruction::Instruction, system_instruction};
use anchor_spl::token::{self, Mint, Token, TokenAccount, Transfer};
use std::str::FromStr;

#[derive(Accounts)]
pub struct InitializePoolWithConfig<'info> {
    #[account(
        seeds = [CONFIG.as_bytes()],
        bump,
    )]
    global_config: Box<Account<'info, Config>>,

    /// CHECK: should be same with the address in the global_config
    #[account(
        mut,
        constraint = global_config.team_wallet == team_wallet.key() @ContractError::IncorrectAuthority
    )]
    pub team_wallet: AccountInfo<'info>,

     /// CHECK: should be same with the address in the global_config
    #[account(
        mut,
        constraint = global_config.team_wallet_secondary == team_wallet_secondary.key() @ContractError::IncorrectAuthority
    )]
    pub team_wallet_secondary: AccountInfo<'info>,

    /// CHECK: should be same with the address in the bonding_curve
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
    /// CHECK: Pool account (PDA address)
    pub pool: UncheckedAccount<'info>,

    /// CHECK: Config for fee
    pub config: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Pool LP Token Mint (AMM 池级别的 LP Token Mint)
    /// Meteora AMM 池的 LP Token Mint 账户，用于定义和 mint/burn Pool LP Token
    /// 作用：
    /// - **定义 LP Token 类型**：定义 Pool LP Token 的类型（SPL Token Mint）
    /// - **Mint LP Token**：当用户向池子提供流动性时，会通过此 mint 账户 mint Pool LP Token 给用户
    /// - **Burn LP Token**：当用户从池子移除流动性时，会通过此 mint 账户 burn 用户的 Pool LP Token
    /// - **代表流动性份额**：Pool LP Token 代表用户在 AMM 池中的流动性份额
    /// 创建逻辑：PDA [SEEDS.LP_MINT, pool.key()] 由 Meteora AMM 程序派生
    /// 
    /// 在池子创建中的作用：
    /// - 池子创建并添加初始流动性后，会通过此 mint 账户 mint LP 代币
    /// - 初始 LP 代币会被 mint 到 payer_pool_lp 账户中，作为提供流动性的凭证
    /// - LP 代币数量 = √(token_a_amount * token_b_amount)
    /// 
    /// 与 a_vault_lp_mint / b_vault_lp_mint 的区别：
    /// - lp_mint: 池子级别的 LP Token，用户持有，代表用户在 AMM 池中的流动性份额
    /// - a_vault_lp_mint / b_vault_lp_mint: Vault 级别的 LP Token，池子持有，代表池子在 Vault 中的代币份额
    /// 
    /// 为什么 lp_mint 不区分 a/b，而 a_vault_lp_mint / b_vault_lp_mint 需要区分？
    /// 1. **AMM 池是配对池**：一个 AMM 池包含 Token A 和 Token B 两种代币，用户提供流动性时必须同时提供两种代币（按比例）
    ///    因此只需要一个 LP Token 来代表用户在**整个配对池**中的流动性份额
    ///    例如：用户提供 10 SOL + 1000 Token，获得 LP Token，代表在 (SOL, Token) 配对池中的份额
    /// 2. **Vault 是单一代币存储**：每个 Vault 只存储一种代币类型
    ///    - a_vault 只存储 Token A（如 SOL/WSOL）
    ///    - b_vault 只存储 Token B（代币）
    ///    因此需要分别为每种代币创建不同的 Vault LP Token 来跟踪池子在各自 Vault 中的份额
    ///    例如：池子向 a_vault 存入 10 SOL，获得 a_vault_lp_mint 的 Vault LP Token
    ///          池子向 b_vault 存入 1000 Token，获得 b_vault_lp_mint 的 Vault LP Token
    /// 
    /// 数据结构（SPL Token Mint，82 字节）：
    /// - mint_authority: Option<Pubkey> (36 字节) - Mint 权限
    /// - supply: u64 (8 字节) - 代币总供应量
    /// - decimals: u8 (1 字节) - 小数位数
    /// - is_initialized: bool (1 字节) - 是否已初始化
    /// - freeze_authority: Option<Pubkey> (36 字节) - 冻结权限
    pub lp_mint: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Vault LP Token Account for Token A (SOL/WSOL)
    /// Vault LP Token 账户，用于存储 Token A 的 Vault LP Token（凭证代币）
    /// 作用：
    /// - **存储 Vault LP Token**：存储由 a_vault_lp_mint mint 的 Vault LP Token
    /// - **代表池子份额**：Vault LP Token 数量代表池子在 a_vault 中的代币份额
    /// - **凭证机制**：当池子向 a_vault 存入代币时，Vault 系统会 mint Vault LP Token 到此账户
    /// - **提取凭证**：当池子从 a_vault 提取代币时，需要 burn 此账户中的 Vault LP Token
    /// 创建逻辑：PDA [a_vault.key(), pool.key()] 由 Meteora AMM 程序派生
    /// 
    /// 与 a_token_vault 的区别：
    /// - a_token_vault: 存储实际的 Token A（如 SOL/WSOL），是池子流动性的实际存储
    /// - a_vault_lp: 存储 Vault LP Token（凭证代币），代表池子在 a_vault 中的份额
    /// 工作流程：池子存入 Token A → Token A 存入 a_token_vault → Vault mint Vault LP Token → 存入 a_vault_lp
    pub a_vault_lp: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Vault LP Token Account for Token B (代币)
    /// Vault LP Token 账户，用于存储 Token B 的 Vault LP Token（凭证代币）
    /// 作用：
    /// - **存储 Vault LP Token**：存储由 b_vault_lp_mint mint 的 Vault LP Token
    /// - **代表池子份额**：Vault LP Token 数量代表池子在 b_vault 中的代币份额
    /// - **凭证机制**：当池子向 b_vault 存入代币时，Vault 系统会 mint Vault LP Token 到此账户
    /// - **提取凭证**：当池子从 b_vault 提取代币时，需要 burn 此账户中的 Vault LP Token
    /// 创建逻辑：PDA [b_vault.key(), pool.key()] 由 Meteora AMM 程序派生
    /// 
    /// 与 b_token_vault 的区别：
    /// - b_token_vault: 存储实际的 Token B（代币），是池子流动性的实际存储
    /// - b_vault_lp: 存储 Vault LP Token（凭证代币），代表池子在 b_vault 中的份额
    /// 工作流程：池子存入 Token B → Token B 存入 b_token_vault → Vault mint Vault LP Token → 存入 b_vault_lp
    pub b_vault_lp: UncheckedAccount<'info>,

    /// CHECK: Token A mint
    pub token_a_mint: UncheckedAccount<'info>,

    #[account(mut)]
    pub token_b_mint: Box<Account<'info, Mint>>,

    #[account(mut)]
    /// CHECK: Vault accounts for token A
    pub a_vault: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Vault accounts for token B
    pub b_vault: UncheckedAccount<'info>,

    #[account(
        mut,
        seeds = [TOKEN_VAULT_SEED.as_bytes(), a_vault.key.as_ref()],
        bump,
        seeds::program = vault_program.key()
    )]
    /// CHECK: Actual token storage account for Token A (SOL/WSOL)
    /// 实际代币存储账户，真正存储 Token A 的 SPL Token 账户
    /// 作用：
    /// - **实际存储代币**：池子的 Token A 流动性代币存储在这里
    /// - **交易交互**：交易时，Token A 从这里转入转出
    /// - **Authority**：由 Vault 程序通过 PDA 控制（authority = Vault Program PDA）
    /// 派生方式：PDA ["token_vault", a_vault.key()] 由 Vault 程序派生
    /// 
    /// 与 a_vault 的关系：
    /// - a_vault: Vault 状态账户，存储配置和状态信息（不直接存储代币）
    /// - a_token_vault: 实际代币存储账户，真正存储 Token A（由 Vault 程序通过 PDA 控制）
    /// 类比：a_vault 类似 Wallet（状态账户），a_token_vault 类似 ATA（代币账户）
    pub a_token_vault: Box<Account<'info, TokenAccount>>,

    #[account(
        mut,
        seeds = [TOKEN_VAULT_SEED.as_bytes(), b_vault.key.as_ref()],
        bump,
        seeds::program = vault_program.key()
    )]
    /// CHECK: Actual token storage account for Token B (代币)
    /// 实际代币存储账户，真正存储 Token B 的 SPL Token 账户
    /// 作用：
    /// - **实际存储代币**：池子的 Token B 流动性代币存储在这里
    /// - **交易交互**：交易时，Token B 从这里转入转出
    /// - **Authority**：由 Vault 程序通过 PDA 控制（authority = Vault Program PDA）
    /// 派生方式：PDA ["token_vault", b_vault.key()] 由 Vault 程序派生
    /// 
    /// 与 b_vault 的关系：
    /// - b_vault: Vault 状态账户，存储配置和状态信息（不直接存储代币）
    /// - b_token_vault: 实际代币存储账户，真正存储 Token B（由 Vault 程序通过 PDA 控制）
    /// 类比：b_vault 类似 Wallet（状态账户），b_token_vault 类似 ATA（代币账户）
    pub b_token_vault: Box<Account<'info, TokenAccount>>,

    #[account(mut)]
    /// CHECK: Vault LP Token Mint for Token A (SOL/WSOL)
    /// Vault LP Token 的 Mint 账户，用于定义和 mint/burn Token A 的 Vault LP Token
    /// 作用：
    /// - **定义 Vault LP Token 类型**：定义 Token A 的 Vault LP Token 类型（SPL Token Mint）
    /// - **Mint Vault LP Token**：当池子向 a_vault 存入 Token A 时，Vault 系统通过此 mint 账户 mint Vault LP Token 到 a_vault_lp
    /// - **Burn Vault LP Token**：当池子从 a_vault 提取 Token A 时，Vault 系统通过此 mint 账户 burn a_vault_lp 中的 Vault LP Token
    /// - **代表池子份额**：Vault LP Token 代表池子在 a_vault 中的代币份额
    /// 创建逻辑：由 Vault 程序在创建 a_vault 时生成，存储在 a_vault.lpMint 字段中
    /// 
    /// 与 lp_mint 的区别：
    /// - lp_mint: 池子级别的 LP Token Mint，用户持有 Pool LP Token，代表用户在 AMM 池中的流动性份额
    /// - a_vault_lp_mint: Vault 级别的 LP Token Mint，池子持有 Vault LP Token，代表池子在 a_vault 中的代币份额
    /// 
    /// 为什么需要区分 a_vault_lp_mint 和 b_vault_lp_mint？
    /// - 因为每个 Vault 只存储一种代币类型，a_vault 存储 Token A，b_vault 存储 Token B
    /// - 池子需要分别跟踪在 a_vault 和 b_vault 中的份额，所以需要两个独立的 Vault LP Token Mint
    /// - 这与 lp_mint 不同：lp_mint 代表用户在配对池中的份额，配对池包含两种代币，所以只需要一个 LP Token
    /// 
    /// 层级关系：
    /// 用户 → lp_mint (Pool LP Token，代表在配对池中的份额) → 池子 → a_vault_lp_mint (Vault LP Token，代表在 Token A Vault 中的份额) → a_vault
    pub a_vault_lp_mint: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Vault LP Token Mint for Token B (代币)
    /// Vault LP Token 的 Mint 账户，用于定义和 mint/burn Token B 的 Vault LP Token
    /// 作用：
    /// - **定义 Vault LP Token 类型**：定义 Token B 的 Vault LP Token 类型（SPL Token Mint）
    /// - **Mint Vault LP Token**：当池子向 b_vault 存入 Token B 时，Vault 系统通过此 mint 账户 mint Vault LP Token 到 b_vault_lp
    /// - **Burn Vault LP Token**：当池子从 b_vault 提取 Token B 时，Vault 系统通过此 mint 账户 burn b_vault_lp 中的 Vault LP Token
    /// - **代表池子份额**：Vault LP Token 代表池子在 b_vault 中的代币份额
    /// 创建逻辑：由 Vault 程序在创建 b_vault 时生成，存储在 b_vault.lpMint 字段中
    /// 
    /// 与 lp_mint 的区别：
    /// - lp_mint: 池子级别的 LP Token Mint，用户持有 Pool LP Token，代表用户在 AMM 池中的流动性份额
    /// - b_vault_lp_mint: Vault 级别的 LP Token Mint，池子持有 Vault LP Token，代表池子在 b_vault 中的代币份额
    /// 
    /// 为什么需要区分 a_vault_lp_mint 和 b_vault_lp_mint？
    /// - 因为每个 Vault 只存储一种代币类型，a_vault 存储 Token A，b_vault 存储 Token B
    /// - 池子需要分别跟踪在 a_vault 和 b_vault 中的份额，所以需要两个独立的 Vault LP Token Mint
    /// - 这与 lp_mint 不同：lp_mint 代表用户在配对池中的份额，配对池包含两种代币，所以只需要一个 LP Token
    /// 
    /// 层级关系：
    /// 用户 → lp_mint (Pool LP Token，代表在配对池中的份额) → 池子 → b_vault_lp_mint (Vault LP Token，代表在 Token B Vault 中的份额) → b_vault
    pub b_vault_lp_mint: UncheckedAccount<'info>,

    /// CHECK: global vault pda which stores SOL
    #[account(
        mut,
        seeds = [GLOBAL.as_bytes()],
        bump,
    )]
    pub global_vault: AccountInfo<'info>,

    /// CHECK: ata of global vault
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
    /// CHECK: Accounts to bootstrap the pool with initial liquidity
    pub payer_token_a: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Accounts to bootstrap the pool with initial liquidity
    pub payer_token_b: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Accounts to bootstrap the pool with initial liquidity
    pub payer_pool_lp: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Protocol fee token a accounts
    pub protocol_token_a_fee: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Protocol fee token b accounts
    pub protocol_token_b_fee: UncheckedAccount<'info>,

    #[account(mut)]
    pub payer: Signer<'info>,

    #[account(mut)]
    pub authority: Signer<'info>,

    #[account(mut)]
    /// CHECK: LP mint metadata PDA. Metaplex do the checking.
    pub mint_metadata: UncheckedAccount<'info>,
    /// CHECK: Additional program accounts
    pub rent: UncheckedAccount<'info>,
    /// CHECK: Metadata program account
    pub metadata_program: UncheckedAccount<'info>,
    /// CHECK: Vault program account
    pub vault_program: UncheckedAccount<'info>,
    /// CHECK: Token program account
    pub token_program: Program<'info, Token>,
    /// CHECK: Associated token program account
    pub associated_token_program: UncheckedAccount<'info>,
    /// CHECK: System program account
    system_program: Program<'info, System>,
    ///CHECK: Event Authority account
    pub event_authority: UncheckedAccount<'info>,

    #[account(mut)]
    /// CHECK: Meteora Program
    pub meteora_program: AccountInfo<'info>,
}

pub fn initialize_pool_with_config(ctx: Context<InitializePoolWithConfig>) -> Result<()> {
    let quote_mint: Pubkey = Pubkey::from_str(QUOTE_MINT).unwrap();

    require!(
        ctx.accounts.authority.key() == ctx.accounts.global_config.migration_authority.key(),
        ContractError::InvalidMigrationAuthority
    );

    require!(
        ctx.accounts.bonding_curve.token_mint.key() == ctx.accounts.token_b_mint.key(),
        ContractError::NotBondingCurveMint
    );

    require!(
        quote_mint.key() == ctx.accounts.token_a_mint.key(),
        ContractError::NotSOL
    );

    require!(
        ctx.accounts.bonding_curve.is_completed,
        ContractError::NotCompleted
    );

    require!(
        ctx.accounts.meteora_program.key() == Pubkey::from_str(METEORA_PROGRAM_KEY).unwrap(),
        ContractError::InvalidMeteoraProgram
    );

    let platform_fee_lamports: u64;
    let platform_fee_secondary_lamports: u64;
    let creator_fee_lamports: u64;

    platform_fee_lamports = bps_mul(
        ctx.accounts.global_config.platform_migration_fee,
        ctx.accounts.bonding_curve.real_sol_reserves,
        10_000,
    )
    .unwrap();

    platform_fee_secondary_lamports = bps_mul(
        ctx.accounts.global_config.platform_migration_fee_secondary,
        ctx.accounts.bonding_curve.real_sol_reserves,
        10_000,
    )
    .unwrap();
  

    creator_fee_lamports = bps_mul(
        ctx.accounts.global_config.creator_migration_fee,
        ctx.accounts.bonding_curve.real_sol_reserves,
        10_000,
    )
    .unwrap();

    let check_sol_amount = ctx
        .accounts
        .bonding_curve
        .real_sol_reserves
        .checked_sub(2_000_000_00)
        .ok_or(ContractError::ArithmeticError)?
        .checked_sub(70_000_000)
        .ok_or(ContractError::ArithmeticError)?
        .checked_sub(platform_fee_lamports)
        .ok_or(ContractError::ArithmeticError)?
        .checked_sub(platform_fee_secondary_lamports)
        .ok_or(ContractError::ArithmeticError)?
        .checked_sub(creator_fee_lamports)
        .ok_or(ContractError::ArithmeticError)?;

    require!(
        check_sol_amount >= ctx.accounts.global_config.initial_meteora_sol_amount , ContractError::ArithmeticError
    );

    let token_a_amount = ctx.accounts.global_config.initial_meteora_sol_amount;
    let token_b_amount = ctx.accounts.global_config.initial_meteora_token_reserves;

    let signer_seeds: &[&[&[u8]]] = &[&[
        GLOBAL.as_bytes(),
        &[ctx.bumps.global_vault],
    ]];

    // Transfer Mint B to payer token b - Bonding Curve is Signer
    let cpi_accounts = Transfer {
        from: ctx.accounts.global_token_account.to_account_info(),
        to: ctx.accounts.payer_token_b.to_account_info(),
        authority: ctx.accounts.global_vault.to_account_info(),
    };

    token::transfer(
        CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            cpi_accounts,
            signer_seeds,
        ),
        token_b_amount,
    )?;

    // Transfer and wrap sol to payer token a - Sol Escrow is Signer
    let sol_ix = system_instruction::transfer(
        &ctx.accounts.global_vault.to_account_info().key,
        &ctx.accounts.payer_token_a.to_account_info().key,
        token_a_amount,
    );

    invoke_signed(
        &sol_ix,
        &[
            ctx.accounts
                .global_vault
                .to_account_info()
                .clone(),
            ctx.accounts.payer_token_a.to_account_info().clone(),
            ctx.accounts.system_program.to_account_info(),
        ],
        signer_seeds,
    )?;

    // Sync Native mint ATA
    let cpi_accounts = token::SyncNative {
        account: ctx.accounts.payer_token_a.to_account_info(),
    };

    let cpi_program = ctx.accounts.token_program.to_account_info();
    let cpi_ctx = CpiContext::new_with_signer(cpi_program, cpi_accounts, signer_seeds);
    token::sync_native(cpi_ctx)?;

    // Create pool
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

    //生成调用 Meteora 程序的指令数据
    //这是一个无权限的恒定乘积池（类似 Uniswap V2），通过 CPI 调用 Meteora 程序完成创建。
    //程序地址：Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB（Meteora AMM 程序）
    let data = get_pool_create_ix_data(token_a_amount, token_b_amount);

    // 创建CPI指令
    // 指定 Meteora 程序地址作为 program_id
    // 传入所需账户和初始流动性参数
    let instruction = Instruction {
        program_id: ctx.accounts.meteora_program.key(),
        accounts,
        data,
    };

    // 执行CPI
    //使用 invoke_signed() 执行跨程序调用
    // 使用 global_vault PDA 作为签名者
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
        signer_seeds, // Signer is the SOL Escrow
    )?;

    // Fee transfer
    let sol_ix = system_instruction::transfer(
        &ctx.accounts.global_vault.to_account_info().key,
        &ctx.accounts.team_wallet.to_account_info().key,
        70_000_000,
    );

    invoke_signed(
        &sol_ix,
        &[
            ctx.accounts
                .global_vault
                .to_account_info()
                .clone(),
            ctx.accounts.team_wallet.clone(),
            ctx.accounts.system_program.to_account_info(),
        ],
        signer_seeds,
    )?;

    if platform_fee_lamports > 0 {
        //  transfer fee to team wallet
        sol_transfer_with_signer(
            ctx.accounts
                .global_vault
                .to_account_info()
                .clone(),
                ctx.accounts.team_wallet.to_account_info().clone(),
            &ctx.accounts.system_program,
            signer_seeds,
            platform_fee_lamports,
        )?;
    }

    if platform_fee_secondary_lamports > 0 {
        //  transfer fee to team wallet secondary
        sol_transfer_with_signer(
            ctx.accounts
                .global_vault
                .to_account_info()
                .clone(),
                ctx.accounts.team_wallet_secondary.to_account_info().clone(),
            &ctx.accounts.system_program,
            signer_seeds,
            platform_fee_secondary_lamports,
        )?;
    }

    if creator_fee_lamports > 0 {
        //  transfer fee to creator
        sol_transfer_with_signer(
            ctx.accounts
                .global_vault
                .to_account_info()
                .clone(),
                ctx.accounts.creator.to_account_info().clone(),
            &ctx.accounts.system_program,
            signer_seeds,
            creator_fee_lamports,
        )?;
    }

    Ok(())
}

