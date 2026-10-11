use crate::errors::*;
use crate::{
    constants::{CONFIG, GLOBAL},
    state::config::*,
    utils::sol_transfer_from_user,
};
use anchor_lang::{prelude::*, system_program, Discriminator};

#[derive(Accounts)]
pub struct Configure<'info> {
    #[account(mut)]
    payer: Signer<'info>,

    /// CHECK: created / updated inside the instruction; PDA-checked by seeds.
    #[account(
        mut,
        seeds = [CONFIG.as_bytes()],
        bump,
    )]
    config: AccountInfo<'info>,

    /// CHECK: global vault pda which stores SOL
    #[account(
        mut,
        seeds = [GLOBAL.as_bytes()],
        bump,
    )]
    pub global_vault: AccountInfo<'info>,

    // SEC-01: bind first-time initialization to the program's upgrade authority.
    // `program` + `program_data` let us read who deployed the program; the handler
    // requires payer == upgrade_authority on the create path, closing the front-run window.
    #[account(constraint = program.programdata_address()? == Some(program_data.key()) @ ContractError::IncorrectAuthority)]
    pub program: Program<'info, crate::program::PumpMeteora>,
    pub program_data: Account<'info, ProgramData>,

    #[account(address = system_program::ID)]
    system_program: Program<'info, System>,
}

impl<'info> Configure<'info> {
    pub fn handler(&mut self, mut new_config: Config, config_bump: u8) -> Result<()> {
        // SEC-03 + SEC-13 + SEC-04: validate every parameter BEFORE persisting it.
        new_config.validate()?;
        new_config.initialized = true;

        let serialized_config =
            [&Config::DISCRIMINATOR[..], new_config.try_to_vec()?.as_slice()].concat();
        let serialized_config_len = serialized_config.len();
        let config_cost = Rent::get()?.minimum_balance(serialized_config_len);

        if self.config.owner != &crate::ID {
            // ---- FIRST INITIALIZATION ----
            // SEC-01: only the program's upgrade authority may initialize.
            let upgrade_authority = self
                .program_data
                .upgrade_authority_address
                .ok_or(ContractError::IncorrectAuthority)?;
            require_keys_eq!(
                upgrade_authority,
                self.payer.key(),
                ContractError::IncorrectAuthority
            );

            let cpi_context = CpiContext::new(
                self.system_program.to_account_info(),
                system_program::CreateAccount {
                    from: self.payer.to_account_info(),
                    to: self.config.to_account_info(),
                },
            );
            system_program::create_account(
                cpi_context.with_signer(&[&[CONFIG.as_bytes(), &[config_bump]]]),
                config_cost,
                serialized_config_len as u64,
                &crate::ID,
            )?;
        } else {
            // ---- UPDATE ----
            // only the current config authority may update.
            let data = self.config.try_borrow_data()?;
            if data.len() < 8 || &data[0..8] != Config::DISCRIMINATOR {
                return err!(ContractError::IncorrectConfigAccount);
            }
            let existing = Config::deserialize(&mut &data[8..])?;
            require_keys_eq!(
                existing.authority,
                self.payer.key(),
                ContractError::IncorrectAuthority
            );
        }

        // top up rent if the account is short (SEC-14: realloc is now unconditional below).
        let cur = self.config.lamports();
        if config_cost > cur {
            system_program::transfer(
                CpiContext::new(
                    self.system_program.to_account_info(),
                    system_program::Transfer {
                        from: self.payer.to_account_info(),
                        to: self.config.to_account_info(),
                    },
                ),
                config_cost - cur,
            )?;
        }
        // SEC-14: always size the account to the serialized length before writing,
        // regardless of the lamport delta (a no-op when the size is unchanged).
        self.config.realloc(serialized_config_len, false)?;
        self.config.try_borrow_mut_data()?[..serialized_config_len]
            .copy_from_slice(serialized_config.as_slice());

        // seed the global vault the first time so it is rent-exempt as a data-less system account.
        if self.global_vault.lamports() == 0 {
            sol_transfer_from_user(&self.payer, self.global_vault.clone(), &self.system_program, 1_000_000)?;
        }
        Ok(())
    }
}
