package clmm

import (
	"dex/pkg/raydium/clmm/idl/generated/amm_v3"

	"github.com/blocto/solana-go-sdk/common"

	ag_solanago "github.com/gagliardetto/solana-go"
)

var (
	ProgramRaydiumConcentratedLiquidity = common.PublicKeyFromString("CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK")
	ProgramClMMDevNet                   = common.PublicKeyFromString("A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys")
)

// SetDevnetProgramID sets the CLMM program ID to use the devnet deployed program
func SetDevnetProgramID() {
	devnetProgramID := ag_solanago.MustPublicKeyFromBase58("A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys")
	amm_v3.SetProgramID(devnetProgramID)
}
