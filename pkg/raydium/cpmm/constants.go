package cpmm

import (
	"dex/pkg/raydium/cpmm/idl/generated/raydium_cp_swap"

	"github.com/blocto/solana-go-sdk/common"

	ag_solanago "github.com/gagliardetto/solana-go"
)

var (
	ProgramRaydiumCPMMProgram = common.PublicKeyFromString("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")
	ProgramCPMMDevNet         = common.PublicKeyFromString("CPMDWBwJDtYax9qW7AyRuVC19Cc4L4Vcy4n2BHAbHkCW")
)

// SetDevnetProgramID sets the CPMM program ID to use the devnet deployed program
func SetDevnetProgramID() {
	devnetProgramID := ag_solanago.MustPublicKeyFromBase58("CPMDWBwJDtYax9qW7AyRuVC19Cc4L4Vcy4n2BHAbHkCW")
	raydium_cp_swap.SetProgramID(devnetProgramID)
}
