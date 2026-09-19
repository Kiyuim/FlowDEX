package pumpfun

import (
	"fmt"

	"dex/pkg/pumpfun/pump/idl/generated/pump"

	"github.com/gagliardetto/solana-go"
)

type BondingCurvePublicKeys struct {
	BondingCurve           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
}

// GetBondingCurveAndAssociatedBondingCurve returns the bonding curve and associated bonding curve, in a structured format.
// This function uses the standard Token Program ID for backward compatibility.
// For Token-2022 tokens, use GetBondingCurveAndAssociatedBondingCurveWithTokenProgram instead.
func GetBondingCurveAndAssociatedBondingCurve(mint solana.PublicKey) (*BondingCurvePublicKeys, error) {
	return GetBondingCurveAndAssociatedBondingCurveWithTokenProgram(mint, solana.TokenProgramID)
}

// GetBondingCurveAndAssociatedBondingCurveWithTokenProgram returns the bonding curve and associated bonding curve
// with the specified token program ID. This is required for Token-2022 tokens.
func GetBondingCurveAndAssociatedBondingCurveWithTokenProgram(mint solana.PublicKey, tokenProgramID solana.PublicKey) (*BondingCurvePublicKeys, error) {
	// Derive bonding curve address.
	// define the seeds used to derive the PDA
	// getProgramDerivedAddress equivalent.
	seeds := [][]byte{
		[]byte("bonding-curve"),
		mint.Bytes(),
	}
	bondingCurve, _, err := solana.FindProgramAddress(seeds, pump.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("failed to derive bonding curve address: %w", err)
	}
	
	// Derive associated bonding curve address.
	// According to PumpFun IDL, associated_bonding_curve is calculated using:
	// seeds: [bonding_curve, token_program, mint]
	// program: Associated Token Account Program
	associatedBondingCurveSeeds := [][]byte{
		bondingCurve.Bytes(),
		tokenProgramID.Bytes(),
		mint.Bytes(),
	}
	associatedBondingCurve, _, err := solana.FindProgramAddress(
		associatedBondingCurveSeeds,
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive associated bonding curve address: %w", err)
	}
	
	return &BondingCurvePublicKeys{
		BondingCurve:           bondingCurve,
		AssociatedBondingCurve: associatedBondingCurve,
	}, nil
}
