package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const PumpFunProgramID = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"

var (
	// Global account address for pump.fun
	GlobalPumpFunAddress = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	// Pump.fun mint authority
	PumpFunMintAuthority = solana.MustPublicKeyFromBase58("TSLvdd1pWpHVjahSpsvCXUbgwsL3JAcvokwaKt1eokM")
	// Pump.fun event authority
	PumpFunEventAuthority = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")
	// Pump.fun fee recipient
	PumpFunFeeRecipient = solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")

	// PUMP_PROGRAM_ID is the program ID for pump.fun
	PUMP_PROGRAM_ID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
)

// CreatorVaultResult holds the result of creator vault derivation
type CreatorVaultResult struct {
	Creator      solana.PublicKey
	CreatorVault solana.PublicKey
	Bump         uint8
}

// GetCreatorVault calculates the creator vault PDA based on the updated PumpFun IDL
func GetCreatorVault(creator solana.PublicKey) (solana.PublicKey, error) {
	// Fixed prefix seed: equivalent to []byte("creator-vault")
	seed1 := []byte{99, 114, 101, 97, 116, 111, 114, 45, 118, 97, 117, 108, 116}
	seed2 := creator.Bytes()

	// Calculate PDA
	pda, _, err := solana.FindProgramAddress([][]byte{seed1, seed2}, solana.MustPublicKeyFromBase58(PumpFunProgramID))
	if err != nil {
		return solana.PublicKey{}, err
	}

	return pda, nil
}

// GetCreatorVaultDynamic calculates the creator vault for a given creator using string seed
func GetCreatorVaultDynamic(creator solana.PublicKey) (solana.PublicKey, error) {
	// Fixed seed: "creator-vault"
	seed1 := []byte("creator-vault")
	seed2 := creator.Bytes()

	// Calculate PDA
	programID := solana.MustPublicKeyFromBase58(PumpFunProgramID)
	pda, _, err := solana.FindProgramAddress([][]byte{seed1, seed2}, programID)
	if err != nil {
		return solana.PublicKey{}, err
	}

	return pda, nil
}

// GetBondingCurvePDA derives the bonding curve PDA from a mint address
func GetBondingCurvePDA(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("bonding-curve"),
		mint.Bytes(),
	}

	pda, bump, err := solana.FindProgramAddress(seeds, PUMP_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, 0, fmt.Errorf("failed to find bonding curve PDA: %w", err)
	}

	return pda, bump, nil
}

// GetCreatorVaultPDA derives the creator vault PDA from a creator address
func GetCreatorVaultPDA(creator solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := [][]byte{
		[]byte("creator-vault"),
		creator.Bytes(),
	}

	pda, bump, err := solana.FindProgramAddress(seeds, PUMP_PROGRAM_ID)
	if err != nil {
		return solana.PublicKey{}, 0, fmt.Errorf("failed to find creator vault PDA: %w", err)
	}

	return pda, bump, nil
}

// GetCreatorVaultFromMint gets the creator and creator vault from a token mint address
func GetCreatorVaultFromMint(connection *rpc.Client, mint solana.PublicKey) (*CreatorVaultResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Get bonding curve PDA
	bondingCurvePDA, _, err := GetBondingCurvePDA(mint)
	if err != nil {
		return nil, fmt.Errorf("failed to derive bonding curve PDA: %w", err)
	}

	fmt.Printf("🔍 Bonding curve PDA: %s\n", bondingCurvePDA.String())

	// Step 2: Fetch bonding curve account data
	accountInfo, err := connection.GetAccountInfo(ctx, bondingCurvePDA)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bonding curve account: %w", err)
	}

	if accountInfo == nil || accountInfo.Value == nil || accountInfo.Value.Data == nil {
		return nil, fmt.Errorf("bonding curve account not found")
	}

	data := accountInfo.Value.Data.GetBinary()

	fmt.Printf("\n🔍 Bonding Curve Account Debug Info:\n")
	fmt.Printf("Account owner: %s\n", accountInfo.Value.Owner.String())
	fmt.Printf("Account data length: %d bytes\n", len(data))
	if len(data) >= 64 {
		fmt.Printf("Account data (first 64 bytes): %x\n", data[:64])
	} else {
		fmt.Printf("Account data (all %d bytes): %x\n", len(data), data)
	}

	// Step 3: Parse the creator from the account data
	// Anchor accounts have an 8-byte discriminator at the beginning
	// From IDL: BondingCurve discriminator is [23, 183, 248, 55, 96, 216, 172, 96]
	if len(data) < 8 {
		return nil, fmt.Errorf("account data too short for discriminator")
	}

	discriminator := data[:8]
	expectedDiscriminator := []byte{23, 183, 248, 55, 96, 216, 172, 96}

	fmt.Printf("Account discriminator: %v\n", discriminator)
	fmt.Printf("Expected discriminator: %v\n", expectedDiscriminator)

	// BondingCurve struct layout (after 8-byte discriminator):
	// - virtual_token_reserves: u64 (8 bytes)
	// - virtual_sol_reserves: u64 (8 bytes)
	// - real_token_reserves: u64 (8 bytes)
	// - real_sol_reserves: u64 (8 bytes)
	// - token_total_supply: u64 (8 bytes)
	// - complete: bool (1 byte)
	// - creator: pubkey (32 bytes)

	creatorOffset := 8 + 8 + 8 + 8 + 8 + 8 + 1 // 8 (discriminator) + 41 (struct fields) = 49 bytes offset

	if len(data) < creatorOffset+32 {
		return nil, fmt.Errorf("account data too short for creator field: need %d bytes, got %d", creatorOffset+32, len(data))
	}

	fmt.Printf("Creator offset: %d\n", creatorOffset)
	creatorBytes := data[creatorOffset : creatorOffset+32]
	fmt.Printf("Creator bytes (hex): %x\n", creatorBytes)

	creator := solana.PublicKeyFromBytes(creatorBytes)
	fmt.Printf("Parsed creator: %s\n", creator.String())

	// Step 4: Get creator vault PDA
	creatorVault, bump, err := GetCreatorVaultPDA(creator)
	if err != nil {
		return nil, fmt.Errorf("failed to derive creator vault PDA: %w", err)
	}

	fmt.Printf("\n🎯 Creator Vault Results:\n")
	fmt.Printf("Calculated creator vault: %s\n", creatorVault.String())
	expectedCreatorVault := "5MrCuuYpPkBk2skPScZf8z3xTvA55cS33cJApuKtRw6p"
	fmt.Printf("Expected creator vault: %s\n", expectedCreatorVault)
	fmt.Printf("Match: %t\n", creatorVault.String() == expectedCreatorVault)

	return &CreatorVaultResult{
		Creator:      creator,
		CreatorVault: creatorVault,
		Bump:         bump,
	}, nil
}

// extractCreatorFromBondingCurveData attempts to extract creator from bonding curve account data
func extractCreatorFromBondingCurveData(data []byte) (solana.PublicKey, error) {
	// ❌ IMPORTANT: Original PumpFun bonding curves do NOT store creator information!
	//
	// Official PumpFun BondingCurve structure (from IDL):
	// [8 bytes]  discriminator: [23, 183, 248, 55, 96, 216, 172, 96]
	// [8 bytes]  VirtualTokenReserves (u64)
	// [8 bytes]  VirtualSolReserves (u64)
	// [8 bytes]  RealTokenReserves (u64)
	// [8 bytes]  RealSolReserves (u64)
	// [8 bytes]  TokenTotalSupply (u64)
	// [1 byte]   Complete (bool)
	//
	// Total: 8 + 8*5 + 1 = 49 bytes
	//
	// ⚠️ NO CREATOR FIELD EXISTS IN BONDING CURVE ACCOUNT DATA!

	if len(data) < 49 {
		return solana.PublicKey{}, fmt.Errorf("insufficient data length for bonding curve: need at least 49 bytes, got %d", len(data))
	}

	// Check for official PumpFun bonding curve discriminator
	expectedDiscriminator := []byte{23, 183, 248, 55, 96, 216, 172, 96}
	actualDiscriminator := data[:8]

	// Verify discriminator
	discriminatorMatch := true
	for i, expected := range expectedDiscriminator {
		if actualDiscriminator[i] != expected {
			discriminatorMatch = false
			break
		}
	}

	if discriminatorMatch {
		fmt.Printf("✅ Official PumpFun BondingCurve discriminator confirmed\n")

		// Parse the bonding curve data to show what's actually there
		offset := 8 // Skip discriminator

		virtualTokenReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		fmt.Printf("📊 Virtual Token Reserves: %d\n", virtualTokenReserves)

		virtualSolReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		fmt.Printf("📊 Virtual SOL Reserves: %d\n", virtualSolReserves)

		realTokenReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		fmt.Printf("📊 Real Token Reserves: %d\n", realTokenReserves)

		realSolReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		fmt.Printf("📊 Real SOL Reserves: %d\n", realSolReserves)

		tokenTotalSupply := binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		fmt.Printf("📊 Token Total Supply: %d\n", tokenTotalSupply)

		complete := data[offset] != 0
		fmt.Printf("📊 Complete: %t\n", complete)

		return solana.PublicKey{}, fmt.Errorf("❌ CREATOR NOT FOUND: Original PumpFun bonding curves do not store creator information in account data. Creator must be found through:\n" +
			"1. CreateEvent transaction logs\n" +
			"2. Transaction history analysis\n" +
			"3. External indexing services")
	}

	fmt.Printf("📊 Discriminator mismatch:\n")
	fmt.Printf("   Expected PumpFun: %v\n", expectedDiscriminator)
	fmt.Printf("   Actual:           %v\n", actualDiscriminator)

	// This might be a different type of account, try generic extraction
	return extractCreatorGeneric(data)
}

// extractCreatorGeneric tries to extract creator from unknown pool types
func extractCreatorGeneric(data []byte) (solana.PublicKey, error) {
	fmt.Printf("🔍 Attempting generic creator extraction...\n")

	// Try different common offsets where creator might be located
	commonOffsets := []int{
		11, // PumpFun AMM (discriminator + bump + index)
		8,  // After discriminator only
		40, // After discriminator + first pubkey
		72, // After discriminator + two pubkeys
		0,  // No discriminator
		32, // After first pubkey
	}

	for _, offset := range commonOffsets {
		if len(data) >= offset+32 {
			creatorBytes := data[offset : offset+32]

			if !isAllZeros(creatorBytes) {
				creator := solana.PublicKeyFromBytes(creatorBytes)
				creatorStr := creator.String()

				// Basic validation
				if len(creatorStr) == 44 && creatorStr != "11111111111111111111111111111111" {
					fmt.Printf("🧪 Potential creator at offset %d: %s\n", offset, creatorStr)
					return creator, nil
				}
			}
		}
	}

	return solana.PublicKey{}, fmt.Errorf("could not extract creator from unknown pool type")
}

// isAllZeros checks if a byte slice contains all zeros
func isAllZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// GetCreatorFromToken dynamically gets the creator for any PumpFun token
func GetCreatorFromToken(tokenMint string, rpcClient *rpc.Client) (solana.PublicKey, error) {
	// Parse token mint
	mint, err := solana.PublicKeyFromBase58(tokenMint)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("invalid token mint: %w", err)
	}

	// Use the improved method to get creator from mint
	result, err := GetCreatorVaultFromMint(rpcClient, mint)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to get creator from mint: %w", err)
	}

	return result.Creator, nil
}

// Complete workflow: Get creator and creator vault for any token
func GetCreatorAndVault(tokenMint string, rpcClient *rpc.Client) (creator solana.PublicKey, creatorVault solana.PublicKey, err error) {
	// Parse token mint
	mint, err := solana.PublicKeyFromBase58(tokenMint)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("invalid token mint: %w", err)
	}

	// Use the improved method to get creator and vault
	result, err := GetCreatorVaultFromMint(rpcClient, mint)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to get creator and vault: %w", err)
	}

	return result.Creator, result.CreatorVault, nil
}

// getBondingCurveAddressDynamic derives the bonding curve address for a token
func getBondingCurveAddressDynamic(mint solana.PublicKey) (solana.PublicKey, error) {
	// Fixed seed: "bonding-curve"
	seed1 := []byte("bonding-curve")
	seed2 := mint.Bytes()

	// Calculate PDA
	programID := solana.MustPublicKeyFromBase58(PumpFunProgramID)
	pda, _, err := solana.FindProgramAddress([][]byte{seed1, seed2}, programID)
	if err != nil {
		return solana.PublicKey{}, err
	}

	return pda, nil
}

// getCreatorFromBondingCurve extracts creator from bonding curve account data (DEPRECATED)
// NOTE: This function is deprecated because original PumpFun bonding curves do not store creator information.
// Use GetCreatorVaultFromMint or transaction-based methods instead.
func getCreatorFromBondingCurve(bondingCurve solana.PublicKey, rpcClient *rpc.Client) (solana.PublicKey, error) {
	fmt.Printf("⚠️ WARNING: getCreatorFromBondingCurve is deprecated. Original PumpFun bonding curves do not store creator information.\n")

	// Fetch the account data
	accountInfo, err := rpcClient.GetAccountInfoWithOpts(context.TODO(), bondingCurve, &rpc.GetAccountInfoOpts{
		Encoding:   solana.EncodingBase64,
		Commitment: rpc.CommitmentProcessed,
	})
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to fetch account: %w", err)
	}

	if accountInfo.Value == nil || accountInfo.Value.Data == nil {
		return solana.PublicKey{}, fmt.Errorf("account not found or has no data")
	}

	data := accountInfo.Value.Data.GetBinary()

	// Try to extract using the improved method
	creator, err := extractCreatorFromBondingCurveData(data)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to extract creator: %w", err)
	}

	return creator, nil
}

// GetCreatorFromTransactions is a placeholder for transaction-based creator discovery
// For now, this returns an error to force fallback to bonding curve method
func GetCreatorFromTransactions(tokenMint string, rpcClient *rpc.Client) (solana.PublicKey, error) {
	fmt.Printf("🔍 Transaction analysis method not yet fully implemented\n")
	return solana.PublicKey{}, fmt.Errorf("transaction analysis method not implemented")
}

// GetCreatorAndVaultFromTransactions uses the correct GetCreatorVaultFromMint method
// with fallback for completed tokens where bonding curve account may be closed
func GetCreatorAndVaultFromTransactions(tokenMint string, rpcClient *rpc.Client) (creator solana.PublicKey, creatorVault solana.PublicKey, err error) {
	// Parse token mint
	mint, err := solana.PublicKeyFromBase58(tokenMint)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("invalid token mint: %w", err)
	}

	// Use the correct method to get creator vault from mint
	result, err := GetCreatorVaultFromMint(rpcClient, mint)
	if err != nil {
		// If bonding curve account not found, the token may be completed
		// Try to derive creator vault using a default/fallback approach
		if err.Error() == "bonding curve account not found" || 
		   err.Error() == "failed to fetch bonding curve account: not found" {
			fmt.Printf("⚠️ Bonding curve account not found for token %s, token may be completed\n", tokenMint)
			fmt.Printf("⚠️ Attempting fallback: using mint authority as creator (this may not work for completed tokens)\n")
			
			// Fallback: Try to get mint authority and use it as creator
			// Note: This is a workaround and may not work for all tokens
			mintAccountInfo, mintErr := rpcClient.GetAccountInfo(context.Background(), mint)
			if mintErr == nil && mintAccountInfo != nil && mintAccountInfo.Value != nil {
				data := mintAccountInfo.Value.Data.GetBinary()
				if len(data) >= 36 {
					// Mint authority is at offset 4-36
					var mintAuthority solana.PublicKey
					copy(mintAuthority[:], data[4:36])
					
					// If mint authority is not System Program, use it as creator
					if !mintAuthority.Equals(solana.SystemProgramID) {
						creatorVault, _, vaultErr := GetCreatorVaultPDA(mintAuthority)
						if vaultErr == nil {
							fmt.Printf("✅ Using mint authority %s as creator (fallback)\n", mintAuthority.String())
							return mintAuthority, creatorVault, nil
						}
					}
				}
			}
			
			// If fallback also fails, return the original error
			return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("bonding curve account not found and fallback failed: token %s may be completed or migrated. Original error: %w", tokenMint, err)
		}
		
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to get creator vault from mint: %w", err)
	}

	fmt.Printf("✅ Creator and vault found via GetCreatorVaultFromMint\n")
	return result.Creator, result.CreatorVault, nil
}
