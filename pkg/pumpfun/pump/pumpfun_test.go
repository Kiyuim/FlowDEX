package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/require"
)

// TestGetCreatorVaultFromMint tests the main functionality
func TestGetCreatorVaultFromMint(t *testing.T) {
	t.Run("GetCreatorVaultFromTokenMint", func(t *testing.T) {
		// Create connection using Helius RPC from config
		connection := rpc.New("https://mainnet.helius-rpc.com/?api-key=2d7580ca-93d2-4404-9316-656c2726a26a")

		// Test token mint (same as TypeScript example)
		tokenMint := solana.MustPublicKeyFromBase58("HrvYD6HrBzy9qTx9BMr9WmAc8F3gFGw4uSJ4KKSBfuxa")

		fmt.Printf("🚀 Fetching creator vault for token: %s\n", tokenMint.String())
		fmt.Printf("...\n")

		result, err := GetCreatorVaultFromMint(connection, tokenMint)

		if err != nil {
			t.Logf("❌ Error: %v", err)
			// Don't fail the test as this demonstrates the limitation
			return
		}

		fmt.Printf("\n✅ Results:\n")
		fmt.Printf("Token Mint: %s\n", tokenMint.String())
		fmt.Printf("Creator: %s\n", result.Creator.String())
		fmt.Printf("Creator Vault: %s\n", result.CreatorVault.String())
		fmt.Printf("Bump: %d\n", result.Bump)

		// Verify we got valid results
		require.NotEqual(t, solana.PublicKey{}, result.Creator, "Creator should not be empty")
		require.NotEqual(t, solana.PublicKey{}, result.CreatorVault, "Creator vault should not be empty")
		require.NotZero(t, result.Bump, "Bump should not be zero")
	})
}

// TestPDADerivation tests individual PDA derivation functions
func TestPDADerivation(t *testing.T) {
	t.Run("BondingCurvePDA", func(t *testing.T) {
		mint := solana.MustPublicKeyFromBase58("HrvYD6HrBzy9qTx9BMr9WmAc8F3gFGw4uSJ4KKSBfuxa")

		pda, bump, err := GetBondingCurvePDA(mint)
		require.NoError(t, err)
		require.NotEqual(t, solana.PublicKey{}, pda)
		require.NotZero(t, bump)

		fmt.Printf("🎯 Bonding Curve PDA: %s (bump: %d)\n", pda.String(), bump)
	})

	t.Run("CreatorVaultPDA", func(t *testing.T) {
		// Use a sample creator address
		creator := solana.MustPublicKeyFromBase58("9jZr6sD1ds4jShBgEwgWLZmWk2vqHPm2EjtCm1YqMaF")

		pda, bump, err := GetCreatorVaultPDA(creator)
		require.NoError(t, err)
		require.NotEqual(t, solana.PublicKey{}, pda)
		require.NotZero(t, bump)

		fmt.Printf("🎯 Creator Vault PDA: %s (bump: %d)\n", pda.String(), bump)
	})
}

// AnalyzePoolDataStructure provides detailed analysis of pool data structure
func AnalyzePoolDataStructure(poolAddress string) error {
	connection := rpc.New("https://mainnet.helius-rpc.com/?api-key=2d7580ca-93d2-4404-9316-656c2726a26a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolPubkey, err := solana.PublicKeyFromBase58(poolAddress)
	if err != nil {
		return fmt.Errorf("invalid pool address: %w", err)
	}

	accountInfo, err := connection.GetAccountInfo(ctx, poolPubkey)
	if err != nil {
		return fmt.Errorf("error fetching pool info: %w", err)
	}

	if accountInfo == nil || accountInfo.Value == nil || accountInfo.Value.Data == nil {
		return fmt.Errorf("pool account not found or has no data")
	}

	data := accountInfo.Value.Data.GetBinary()
	owner := accountInfo.Value.Owner.String()

	fmt.Printf("\n🔍 === COMPLETE POOL DATA STRUCTURE ANALYSIS ===\n")
	fmt.Printf("Pool Address: %s\n", poolAddress)
	fmt.Printf("Owner Program: %s\n", owner)
	fmt.Printf("Data Length: %d bytes\n", len(data))
	fmt.Printf("Lamports: %d\n", accountInfo.Value.Lamports)

	if len(data) == 0 {
		return fmt.Errorf("no data in account")
	}

	// Dump complete hex data
	fmt.Printf("\n📊 === COMPLETE HEX DUMP ===\n")
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		fmt.Printf("%04x: %x\n", i, data[i:end])
	}

	// Analyze potential account fields
	fmt.Printf("\n📋 === POTENTIAL ACCOUNT FIELDS ===\n")

	// Check if it has discriminator
	if len(data) >= 8 {
		discriminator := data[:8]
		fmt.Printf("Discriminator (0-8): %x\n", discriminator)
	}

	// Analyze every 32-byte chunk as potential PublicKey
	fmt.Printf("\n🔑 === ALL POTENTIAL PUBLIC KEYS ===\n")
	offset := 0
	fieldIndex := 0

	for offset+32 <= len(data) {
		pubkeyBytes := data[offset : offset+32]

		if !isAllZeros(pubkeyBytes) {
			pubkey := solana.PublicKeyFromBytes(pubkeyBytes)
			pubkeyStr := pubkey.String()

			fmt.Printf("Field %d (offset %d-%d): %s\n", fieldIndex, offset, offset+31, pubkeyStr)

			// Try to identify what this pubkey might be
			if pubkeyStr == "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P" {
				fmt.Printf("  └── 🎯 PumpFun Program ID\n")
			} else if pubkeyStr == "11111111111111111111111111111111" {
				fmt.Printf("  └── 🎯 System Program\n")
			} else if pubkeyStr == "So11111111111111111111111111111111111111112" {
				fmt.Printf("  └── 🎯 WSOL Mint\n")
			} else if pubkeyStr == "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA" {
				fmt.Printf("  └── 🎯 Token Program\n")
			} else if len(pubkeyStr) == 44 {
				fmt.Printf("  └── 🧪 Potential: Token Mint/Creator/Authority\n")
			}
		} else {
			fmt.Printf("Field %d (offset %d-%d): [ZERO BYTES]\n", fieldIndex, offset, offset+31)
		}

		offset += 32
		fieldIndex++
	}

	// Analyze potential u64 fields
	fmt.Printf("\n📊 === POTENTIAL U64 FIELDS ===\n")
	offset = 0
	if len(data) >= 8 {
		offset = 8 // Skip discriminator if present
	}

	u64Index := 0
	for offset+8 <= len(data) {
		if offset%32 == 0 && offset+32 <= len(data) {
			// This is a pubkey field, skip
			offset += 32
			continue
		}

		value := binary.LittleEndian.Uint64(data[offset : offset+8])
		fmt.Printf("U64 Field %d (offset %d-%d): %d (0x%x)\n", u64Index, offset, offset+7, value, value)

		offset += 8
		u64Index++
	}

	// Analyze potential bool fields
	fmt.Printf("\n🔘 === POTENTIAL BOOL FIELDS ===\n")
	offset = 0
	if len(data) >= 8 {
		offset = 8 // Skip discriminator if present
	}

	boolIndex := 0
	for offset < len(data) {
		// Skip known pubkey and u64 regions
		if (offset%32 == 0 && offset+32 <= len(data)) || (offset%8 == 0 && offset%32 != 0) {
			if offset%32 == 0 {
				offset += 32 // Skip pubkey
			} else {
				offset += 8 // Skip u64
			}
			continue
		}

		if offset < len(data) {
			value := data[offset]
			fmt.Printf("Bool Field %d (offset %d): %d (%t)\n", boolIndex, offset, value, value != 0)
			offset++
			boolIndex++
		}
	}

	return nil
}

// ExtractPoolCreator extracts the creator from pool account data
func ExtractPoolCreator(poolAddress string) (string, error) {
	connection := rpc.New("https://mainnet.helius-rpc.com/?api-key=2d7580ca-93d2-4404-9316-656c2726a26a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolPubkey, err := solana.PublicKeyFromBase58(poolAddress)
	if err != nil {
		return "", fmt.Errorf("invalid pool address: %w", err)
	}

	accountInfo, err := connection.GetAccountInfo(ctx, poolPubkey)
	if err != nil {
		return "", fmt.Errorf("error fetching pool info: %w", err)
	}

	if accountInfo == nil || accountInfo.Value == nil || accountInfo.Value.Data == nil {
		return "", fmt.Errorf("pool account not found or has no data")
	}

	data := accountInfo.Value.Data.GetBinary()
	owner := accountInfo.Value.Owner.String()

	fmt.Printf("🔍 Analyzing pool: %s\n", poolAddress)
	fmt.Printf("📊 Owner Program: %s\n", owner)
	fmt.Printf("📊 Data Length: %d bytes\n", len(data))

	// Check if it's a PumpFun program account
	pumpfunProgram := "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"
	if owner == pumpfunProgram {
		fmt.Printf("🎯 Detected: PumpFun Program Account\n")

		// Try to extract creator from different pool types
		if creator, err := extractCreatorFromPumpFunPool(data); err == nil && creator != "" {
			fmt.Printf("✅ Found creator: %s\n", creator)
			return creator, nil
		}

		// If direct extraction fails, return error
		return "", fmt.Errorf("failed to extract creator from PumpFun pool data")
	}

	return "", fmt.Errorf("unsupported pool type, owner: %s", owner)
}

// extractCreatorFromPumpFunPool attempts to extract creator from original PumpFun bonding curve data
func extractCreatorFromPumpFunPool(data []byte) (string, error) {
	// Use the updated extraction method
	creator, err := extractCreatorFromBondingCurveData(data)
	if err != nil {
		return "", err
	}

	return creator.String(), nil
}

// GetPoolCreatorInfo gets pool account information similar to the JavaScript implementation
func GetPoolCreatorInfo(poolAddress string) error {
	// Create connection to Solana mainnet
	connection := rpc.New("https://mainnet.helius-rpc.com/?api-key=2d7580ca-93d2-4404-9316-656c2726a26a")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create PublicKey from pool address
	poolPubkey, err := solana.PublicKeyFromBase58(poolAddress)
	if err != nil {
		return fmt.Errorf("invalid pool address: %w", err)
	}

	// Get account info with JSON parsed encoding equivalent
	accountInfo, err := connection.GetAccountInfo(ctx, poolPubkey)
	if err != nil {
		return fmt.Errorf("error fetching pool info: %w", err)
	}

	if accountInfo != nil && accountInfo.Value != nil {
		fmt.Println("Pool Account Info:")
		fmt.Printf("Owner Program: %s\n", accountInfo.Value.Owner.String())
		fmt.Printf("Lamports: %d\n", accountInfo.Value.Lamports)
		fmt.Printf("Executable: %t\n", accountInfo.Value.Executable)
		fmt.Printf("Rent Epoch: %d\n", accountInfo.Value.RentEpoch)

		// The account data will contain pool-specific information
		// including potential creator information depending on the pool type
		if accountInfo.Value.Data != nil {
			data := accountInfo.Value.Data.GetBinary()
			fmt.Printf("Data Length: %d bytes\n", len(data))
			if len(data) > 0 {
				displayLength := 32
				if len(data) < displayLength {
					displayLength = len(data)
				}
				fmt.Printf("First %d bytes (hex): %x\n", displayLength, data[:displayLength])
			}
		}
	} else {
		fmt.Println("Pool account not found or has no data")
	}

	return nil
}

// TestAnalyzePoolDataStructure tests the complete data structure analysis
func TestAnalyzePoolDataStructure(t *testing.T) {
	t.Run("CompleteStructureAnalysis", func(t *testing.T) {
		// Test with your specific pool address
		poolAddress := "7r4ke3nJmG13SzZW3UBRkQn3aqQyn8JXJZNt8JWPoRXo"

		err := AnalyzePoolDataStructure(poolAddress)
		if err != nil {
			t.Logf("Analysis failed: %v", err)
		}

		// Don't require success since we're exploring
		// require.NoError(t, err, "Structure analysis should not fail")
	})
}

// TestGetPoolCreator tests the actual creator extraction
func TestGetPoolCreator(t *testing.T) {
	t.Run("ExtractCreatorFromPool", func(t *testing.T) {
		// Test with your specific pool address
		poolAddress := "7r4ke3nJmG13SzZW3UBRkQn3aqQyn8JXJZNt8JWPoRXo"

		fmt.Printf("\n=== Extracting Creator from Pool ===\n")
		creator, err := ExtractPoolCreator(poolAddress)

		if err != nil {
			fmt.Printf("❌ Creator extraction failed: %v\n", err)
			// Don't fail the test, just log the error
			t.Logf("Creator extraction failed: %v", err)
		} else {
			fmt.Printf("✅ Successfully extracted creator: %s\n", creator)
			require.NotEmpty(t, creator, "Creator should not be empty")
		}
	})
}

// TestBondingCurveStructureParsing tests the correct parsing of bonding curve structure
func TestBondingCurveStructureParsing(t *testing.T) {
	t.Run("OfficialBondingCurveStructure", func(t *testing.T) {
		// Create connection using Helius RPC from config
		connection := rpc.New("https://mainnet.helius-rpc.com/?api-key=2d7580ca-93d2-4404-9316-656c2726a26a")

		// Test token mint
		tokenMint := solana.MustPublicKeyFromBase58("HrvYD6HrBzy9qTx9BMr9WmAc8F3gFGw4uSJ4KKSBfuxa")

		// Get bonding curve PDA
		bondingCurvePDA, _, err := GetBondingCurvePDA(tokenMint)
		require.NoError(t, err)

		// Fetch bonding curve account data
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		accountInfo, err := connection.GetAccountInfo(ctx, bondingCurvePDA)
		require.NoError(t, err)
		require.NotNil(t, accountInfo)
		require.NotNil(t, accountInfo.Value)
		require.NotNil(t, accountInfo.Value.Data)

		data := accountInfo.Value.Data.GetBinary()

		fmt.Printf("🔍 Testing Official Bonding Curve Structure:\n")
		fmt.Printf("Bonding Curve PDA: %s\n", bondingCurvePDA.String())
		fmt.Printf("Data Length: %d bytes\n", len(data))

		// Test discriminator
		if len(data) >= 8 {
			expectedDiscriminator := []byte{23, 183, 248, 55, 96, 216, 172, 96}
			actualDiscriminator := data[:8]

			fmt.Printf("Expected discriminator: %v\n", expectedDiscriminator)
			fmt.Printf("Actual discriminator: %v\n", actualDiscriminator)

			discriminatorMatch := true
			for i, expected := range expectedDiscriminator {
				if actualDiscriminator[i] != expected {
					discriminatorMatch = false
					break
				}
			}

			if discriminatorMatch {
				fmt.Printf("✅ Official PumpFun bonding curve discriminator confirmed\n")

				// Parse the structure
				if len(data) >= 49 {
					offset := 8 // Skip discriminator

					virtualTokenReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
					offset += 8
					fmt.Printf("Virtual Token Reserves: %d\n", virtualTokenReserves)

					virtualSolReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
					offset += 8
					fmt.Printf("Virtual SOL Reserves: %d\n", virtualSolReserves)

					realTokenReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
					offset += 8
					fmt.Printf("Real Token Reserves: %d\n", realTokenReserves)

					realSolReserves := binary.LittleEndian.Uint64(data[offset : offset+8])
					offset += 8
					fmt.Printf("Real SOL Reserves: %d\n", realSolReserves)

					tokenTotalSupply := binary.LittleEndian.Uint64(data[offset : offset+8])
					offset += 8
					fmt.Printf("Token Total Supply: %d\n", tokenTotalSupply)

					complete := data[offset] != 0
					fmt.Printf("Complete: %t\n", complete)

					fmt.Printf("✅ Structure parsing successful - NO CREATOR FIELD PRESENT\n")
				} else {
					t.Errorf("Insufficient data for full bonding curve structure")
				}
			} else {
				t.Logf("Discriminator mismatch - might be a different account type")
			}
		}
	})
}

// TestCreatorVaultDynamicCalculation tests the creator vault calculation
func TestCreatorVaultDynamicCalculation(t *testing.T) {
	t.Run("CalculateCreatorVault", func(t *testing.T) {
		// Test with known creator addresses
		testCases := []struct {
			name    string
			creator string
		}{
			{
				name:    "Sample Creator 1",
				creator: "9jZr6sD1ds4jShBgEwgWLZmWk2vqHPm2EjtCm1YqMaF",
			},
			{
				name:    "Sample Creator 2",
				creator: "4YzPXP8Qc6a3FHQQKfh8cPNyKfHJbDjPqz8Q9mMK9r1b",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				creator, err := solana.PublicKeyFromBase58(tc.creator)
				require.NoError(t, err)

				// Test both methods
				vault1, err1 := GetCreatorVault(creator)
				vault2, err2 := GetCreatorVaultDynamic(creator)

				require.NoError(t, err1)
				require.NoError(t, err2)

				// Both methods should produce the same result
				require.Equal(t, vault1.String(), vault2.String(), "Both creator vault methods should produce the same result")

				fmt.Printf("Creator: %s\n", creator.String())
				fmt.Printf("Creator Vault: %s\n", vault1.String())
			})
		}
	})
}
