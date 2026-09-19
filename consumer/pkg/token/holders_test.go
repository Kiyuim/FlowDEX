package token

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestGetTokenHolders(t *testing.T) {

	sol_url := "https://rpc.ankr.com/solana/42d3f3c9c3ecb7cf326088a9d43c07c154901e5c7de5667206ab2b5bbfd6c3a2"

	holders, err := GetTokenHolders(context.Background(),
		// "https://mainnet.helius-rpc.com?api-key=276652a7-1dbb-46c7-8734-791d5b5ce280",
		sol_url,
		// solana.MustPublicKeyFromBase58("SENDdRQtYMWaQrBroBrJ2Q53fgVuq95CV9UPGEvpCxa"))
		// solana.MustPublicKeyFromBase58("FUAfBo2jgks6gB4Z4LfZkqSZgzNucisEHqnNebaRxM1P"))
		solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"))
	t.Log(len(holders))
	t.Error(err)
}

func TestGetToken2022Holders(t *testing.T) {

	sol_url := "https://rpc.ankr.com/solana/42d3f3c9c3ecb7cf326088a9d43c07c154901e5c7de5667206ab2b5bbfd6c3a2"
	// // sol_url := "https://mainnet.helius-rpc.com?api-key=276652a7-1dbb-46c7-8734-791d5b5ce280"
	//
	// c := ag_rpc.New(sol_url)
	//
	// accountsByOwner, err := c.GetProgramAccountsWithOpts(context.Background(), solana.Token2022ProgramID,
	// 	&ag_rpc.GetProgramAccountsOpts{
	// 		Commitment: ag_rpc.CommitmentConfirmed,
	// 		Encoding:   solana.EncodingBase64Zstd,
	// 	},
	// )
	// if err != nil {
	// 	t.Fatal(err)
	// }
	//
	// // _ = accountsByOwner
	//
	// spew.Dump(accountsByOwner)
	// return

	holders, err := GetToken2022Holders(context.Background(),
		// "https://mainnet.helius-rpc.com?api-key=276652a7-1dbb-46c7-8734-791d5b5ce280",
		sol_url,
		// solana.MustPublicKeyFromBase58("SENDdRQtYMWaQrBroBrJ2Q53fgVuq95CV9UPGEvpCxa"))
		// solana.MustPublicKeyFromBase58("FUAfBo2jgks6gB4Z4LfZkqSZgzNucisEHqnNebaRxM1P"))
		// solana.MustPublicKeyFromBase58("HeLp6NuQkmYB4pYWo2zYs22mESHXPQYzXbB8n4V98jwC"))
		solana.MustPublicKeyFromBase58("A87WTvNy6oF21adEbgmyPioNagLvUXXJaqtpAJKXX1F7"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(len(holders))
}
