package solana

import (
	"context"
	"dex/pkg/sol"

	"github.com/blocto/solana-go-sdk/program/address_lookup_table"
	"github.com/blocto/solana-go-sdk/types"
	"github.com/shopspring/decimal"
)

func ConverString2Uint64(amount string, tokenDecimal uint8) (uint64, error) {
	amtDecimal, err := decimal.NewFromString(amount)
	if nil != err {
		return 0, err
	}
	amtDecimal = amtDecimal.Mul(decimal.NewFromInt(sol.Decimals2Value[tokenDecimal]))
	amountUint64 := uint64(amtDecimal.IntPart())
	return amountUint64, nil
}

func ConverFloat642Uint64(amount float64, tokenDecimal uint8) uint64 {
	amtDecimal := decimal.NewFromFloatWithExponent(amount, -9)
	amtDecimal = amtDecimal.Mul(decimal.NewFromInt(sol.Decimals2Value[tokenDecimal]))
	amountUint64 := uint64(amtDecimal.IntPart())
	return amountUint64
}

// CreateGlobalALT creates an Address Lookup Table with the five fixed addresses for maximum compatibility.
// Usage: Call this function with your payer keypair and recent slot.
func CreateGlobalALT(ctx context.Context, payer types.Account, recentSlot uint64) types.Instruction {
	// 1. Create the lookup table instruction
	createALTInstruction := address_lookup_table.CreateLookupTable(address_lookup_table.CreateLookupTableParams{
		Authority:  payer.PublicKey,
		Payer:      payer.PublicKey,
		RecentSlot: recentSlot,
	})

	// TODO: Derive and return the ALT address after creation if needed.
	// To add addresses, send an Extend instruction after creation.
	return createALTInstruction
}
