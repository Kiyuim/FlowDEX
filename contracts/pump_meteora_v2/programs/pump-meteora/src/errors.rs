use anchor_lang::prelude::*;

pub use ContractError::*;

#[error_code]
pub enum ContractError {
    #[msg("ValueTooSmall")]
    ValueTooSmall,
    #[msg("ValueTooLarge")]
    ValueTooLarge,
    #[msg("ValueInvalid")]
    ValueInvalid,
    #[msg("IncorrectConfigAccount")]
    IncorrectConfigAccount,
    #[msg("IncorrectAuthority")]
    IncorrectAuthority,
    #[msg("Overflow or underflow occured")]
    OverflowOrUnderflowOccurred,
    #[msg("Amount is invalid")]
    InvalidAmount,
    #[msg("Incorrect team wallet address")]
    IncorrectTeamWallet,
    #[msg("Curve is not completed")]
    CurveNotCompleted,
    #[msg("Can not swap after the curve is completed")]
    CurveAlreadyCompleted,
    #[msg("Return amount is too small compared to the minimum received amount")]
    ReturnAmountTooSmall,
    #[msg("AMM is already exist")]
    AmmAlreadyExists,
    #[msg("Global Not Initialized")]
    NotInitialized,
    #[msg("Not enough tokens to complete the sell order.")]
    InsufficientTokens,
    #[msg("Sell Failed")]
    SellFailed,
    #[msg("Buy Failed")]
    BuyFailed,
    #[msg("This token is not a bonding curve token")]
    NotBondingCurveMint,
    #[msg("Not quote mint")]
    NotSOL,
    #[msg("Invalid Migration Authority")]
    InvalidMigrationAuthority,
    #[msg("Bonding curve is not completed")]
    NotCompleted,
    #[msg("Invalid Meteora Program")]
    InvalidMeteoraProgram,
    #[msg("Invalid Meteora Vault Program")]
    InvalidMeteoraVaultProgram,
    #[msg("Arithmetic Error")]
    ArithmeticError,
    #[msg("Slippage Exceeded")]
    SlippageExceeded,
    #[msg("Max SOL cost exceeded")]
    MaxSolCostExceeded,
    #[msg("Invalid swap direction")]
    InvalidDirection,
    #[msg("Account is not owned by the program vault")]
    InvalidVaultOwner,
}
