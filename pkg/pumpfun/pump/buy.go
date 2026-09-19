package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"dex/pkg/pumpfun/pump/idl/generated/pump"
	"dex/pkg/sol"
	"dex/pkg/trade"

	ag_binary "github.com/gagliardetto/binary"
	aSDK "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// Extra account program and seeds per latest PumpFun IDL
var (
	// Fee Program (constant)
	PumpFeeProgramAddress = aSDK.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")
	// Fee Config PDA seeds under fee_program: ["fee_config", <32-byte seed>]
	feeConfigSeedTag = []byte("fee_config")
	feeConfigSeedKey = []byte{1, 86, 224, 246, 147, 102, 90, 207, 68, 219, 21, 104, 191, 23, 91, 170, 81, 137, 203, 151, 245, 210, 255, 59, 101, 93, 43, 182, 253, 109, 24, 176}
)

// getMintAuthority fetches the mint authority from the mint account
func getMintAuthority(rpcClient *rpc.Client, mintAddress aSDK.PublicKey) (aSDK.PublicKey, error) {
	accountInfo, err := rpcClient.GetAccountInfoWithOpts(context.TODO(), mintAddress, &rpc.GetAccountInfoOpts{
		Encoding:   aSDK.EncodingBase64,
		Commitment: rpc.CommitmentProcessed,
	})
	if err != nil {
		return aSDK.PublicKey{}, fmt.Errorf("failed to get mint account info: %w", err)
	}

	if accountInfo.Value == nil {
		return aSDK.PublicKey{}, fmt.Errorf("mint account not found")
	}

	data := accountInfo.Value.Data.GetBinary()
	if len(data) < 64 {
		return aSDK.PublicKey{}, fmt.Errorf("invalid mint account data length")
	}

	// Parse mint authority from mint account data (offset 4-36 for mint authority)
	var mintAuthority aSDK.PublicKey
	copy(mintAuthority[:], data[4:36])

	return mintAuthority, nil
}

// CalculateBuyQuote calculates how many tokens can be purchased given a specific amount of SOL, bonding curve data, and percentage.
// solAmount is the amount of sol that you want to buy
// bondingCurve is the BondingCurveData, that includes the real, virtual token/sol reserves, in order to calculate the price.
// percentage is what you want to use to set the slippage. For 2% slippage, you want to set the percentage to 0.98.
func CalculateBuyQuote(solAmount uint64, bondingCurve *BondingCurveData, percentage float64) uint64 {
	// Convert solAmount to *big.Int
	solAmountBig := big.NewInt(int64(solAmount))

	// Clone bonding curve data to avoid mutations
	virtualSolReserves := new(big.Int).Set(bondingCurve.VirtualSolReserves)
	virtualTokenReserves := new(big.Int).Set(bondingCurve.VirtualTokenReserves)

	// Compute the new virtual reserves
	newVirtualSolReserves := new(big.Int).Add(virtualSolReserves, solAmountBig)
	invariant := new(big.Int).Mul(virtualSolReserves, virtualTokenReserves)
	newVirtualTokenReserves := new(big.Int).Div(invariant, newVirtualSolReserves)

	// Calculate the tokens to buy
	tokensToBuy := new(big.Int).Sub(virtualTokenReserves, newVirtualTokenReserves)

	// Apply the percentage reduction (e.g., 95% or 0.95)
	// Convert the percentage to a multiplier (0.95) and apply to tokensToBuy
	percentageMultiplier := big.NewFloat(percentage)
	tokensToBuyFloat := new(big.Float).SetInt(tokensToBuy)
	finalTokens := new(big.Float).Mul(tokensToBuyFloat, percentageMultiplier)

	// Convert the result back to *big.Int
	finalTokensBig, _ := finalTokens.Int(nil)

	return finalTokensBig.Uint64()
}

func BuildBuyInstruction(user aSDK.PublicKey, tokenMint aSDK.PublicKey,
	solAmountUint64 uint64, slippageBasisPoint uint32, rpcClient *rpc.Client,
	price float64, inDecimal, outDecimal uint8) (aSDK.Instruction, error) {
	return BuildBuyInstructionWithTokenProgram(user, tokenMint, solAmountUint64, slippageBasisPoint, rpcClient, price, inDecimal, outDecimal, token.ProgramID)
}

func BuildBuyInstructionWithTokenProgram(user aSDK.PublicKey, tokenMint aSDK.PublicKey,
	solAmountUint64 uint64, slippageBasisPoint uint32, rpcClient *rpc.Client,
	price float64, inDecimal, outDecimal uint8, tokenProgramID aSDK.PublicKey) (aSDK.Instruction, error) {

	/////////Going to build pumpfun buy instrustions /////
	// Use the correct token program ID to calculate associated_bonding_curve
	bondingCurveData, err := GetBondingCurveAndAssociatedBondingCurveWithTokenProgram(tokenMint, tokenProgramID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bonding curve data: %w", err)
	}
	fmt.Printf("DEBUG: Bonding curve: %s, Associated bonding curve: %s\n",
		bondingCurveData.BondingCurve.String(), bondingCurveData.AssociatedBondingCurve.String())
	var minAmountOut uint64
	// 如果价格不为空 那么按照价格走而不是恒乘积走
	if price != 0 {
		fmt.Println("price != 0")
		minAmountOut, _, err = trade.CalcMinAmountOutByPrice(slippageBasisPoint, solAmountUint64, true, price, inDecimal, outDecimal, trade.PumpFee)
		if err != nil {
			return nil, err
		}
	} else {
		fmt.Println("price == 0")
		bondingCurve, err := FetchBondingCurve(rpcClient, bondingCurveData.BondingCurve)
		if err != nil {
			return nil, fmt.Errorf("can't fetch bonding curve: %w", err)
		}

		slippage := big.NewFloat(float64(1))
		slippage = slippage.Quo(big.NewFloat(float64(slippageBasisPoint)), big.NewFloat(float64(1e4)))

		slippageF64, _ := slippage.Float64()
		percentage := float64(1.0 - slippageF64)
		minAmountOut = CalculateBuyQuote(solAmountUint64, bondingCurve, percentage)
	}

	// Use the correct method to find ATA based on token program ID
	// Import sol package to use FindAssociatedTokenAddress which handles both Token and Token-2022
	ata, _, err := sol.FindAssociatedTokenAddress(user, tokenMint, tokenProgramID)
	if err != nil {
		return nil, fmt.Errorf("failed to find ATA: %w", err)
	}
	if tokenProgramID.Equals(aSDK.Token2022ProgramID) {
		fmt.Printf("DEBUG: Using Token-2022 ATA: %s\n", ata.String())
	} else {
		fmt.Printf("DEBUG: Using standard Token ATA: %s\n", ata.String())
	}

	// Validate inputs to avoid on-chain BuyZeroAmount error
	if solAmountUint64 == 0 {
		return nil, fmt.Errorf("buy failed: solAmount is 0")
	}

	// Get creator and vault dynamically using the new method with fallback
	creator, creatorVault, err := GetCreatorAndVaultFromTransactions(tokenMint.String(), rpcClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get creator and vault: %w", err)
	}

	// If computed token amount is zero, fail fast with a clear error
	if minAmountOut == 0 {
		return nil, fmt.Errorf("buy failed: calculated token amount is 0; increase SOL amount or adjust slippage/decimals")
	}

	// Debug log the creator and vault
	fmt.Printf("DEBUG: Token mint: %s\n", tokenMint.String())
	fmt.Printf("DEBUG: Creator: %s\n", creator.String())
	fmt.Printf("DEBUG: Creator vault: %s\n", creatorVault.String())

	// Resolve fee recipient from on-chain Global to satisfy authorization
	feeRecipient, err := getGlobalFeeRecipient(rpcClient)
	if err != nil {
		// Fallback to constant if decode fails
		fmt.Printf("WARN: using default PumpFunFeeRecipient due to error: %v\n", err)
		feeRecipient = PumpFunFeeRecipient
	}

	// Create custom buy instruction with creator vault
	buyInstr := buildBuyInstructionWithCreatorVault(
		minAmountOut,
		solAmountUint64,
		GlobalPumpFunAddress,
		feeRecipient,
		tokenMint,
		bondingCurveData.BondingCurve,
		bondingCurveData.AssociatedBondingCurve,
		ata,
		user,
		system.ProgramID,
		tokenProgramID, // Use the correct token program ID
		PumpFunEventAuthority,
		creatorVault, // Add creator vault
		pump.ProgramID,
	)

	return buyInstr, nil
}

// getGlobalFeeRecipient fetches the configured fee recipient from the Global account
func getGlobalFeeRecipient(rpcClient *rpc.Client) (aSDK.PublicKey, error) {
	// Derive Global PDA using seed "global"
	globalPDA, _, err := aSDK.FindProgramAddress([][]byte{[]byte("global")}, pump.ProgramID)
	if err != nil {
		return aSDK.PublicKey{}, fmt.Errorf("derive global PDA: %w", err)
	}
	acct, err := rpcClient.GetAccountInfoWithOpts(context.TODO(), globalPDA, &rpc.GetAccountInfoOpts{
		Encoding:   aSDK.EncodingBase64,
		Commitment: rpc.CommitmentProcessed,
	})
	if err != nil {
		return aSDK.PublicKey{}, fmt.Errorf("fetch global account: %w", err)
	}
	if acct.Value == nil || acct.Value.Data == nil {
		return aSDK.PublicKey{}, fmt.Errorf("global account not found")
	}
	data := acct.Value.Data.GetBinary()
	dec := ag_binary.NewBinDecoder(data)
	var global pump.Global
	if err := global.UnmarshalWithDecoder(dec); err != nil {
		return aSDK.PublicKey{}, fmt.Errorf("decode global: %w", err)
	}
	return global.FeeRecipient, nil
}

// buildBuyInstructionWithCreatorVault creates a buy instruction with creator vault account
func buildBuyInstructionWithCreatorVault(
	amount uint64,
	maxSolCost uint64,
	global aSDK.PublicKey,
	feeRecipient aSDK.PublicKey,
	mint aSDK.PublicKey,
	bondingCurve aSDK.PublicKey,
	associatedBondingCurve aSDK.PublicKey,
	associatedUser aSDK.PublicKey,
	user aSDK.PublicKey,
	systemProgram aSDK.PublicKey,
	tokenProgram aSDK.PublicKey,
	eventAuthority aSDK.PublicKey,
	creatorVault aSDK.PublicKey,
	program aSDK.PublicKey,
) aSDK.Instruction {
	// Create accounts slice matching the correct PumpFun order (12 accounts total)
	accounts := []*aSDK.AccountMeta{
		aSDK.Meta(global).WRITE(),                 // #0 - Global (WRITABLE)
		aSDK.Meta(feeRecipient).WRITE(),           // #1 - Fee Recipient (WRITABLE)
		aSDK.Meta(mint).WRITE(),                   // #2 - Mint (WRITABLE)
		aSDK.Meta(bondingCurve).WRITE(),           // #3 - Bonding Curve (WRITABLE)
		aSDK.Meta(associatedBondingCurve).WRITE(), // #4 - Associated Bonding Curve (WRITABLE)
		aSDK.Meta(associatedUser).WRITE(),         // #5 - Associated User (WRITABLE)
		aSDK.Meta(user).WRITE().SIGNER(),          // #6 - User (WRITABLE, SIGNER)
		aSDK.Meta(systemProgram),                  // #7 - System Program
		aSDK.Meta(tokenProgram),                   // #8 - Token Program (read-only, not writable)
		aSDK.Meta(creatorVault).WRITE(),           // #9 - Creator Vault (WRITABLE)
		aSDK.Meta(eventAuthority).WRITE(),         // #10 - Event Authority (WRITABLE)
		aSDK.Meta(program),                        // #11 - Program
	}

	// Derive and append updated PumpFun extra accounts (#13-#16)
	{
		// #12 - Global Volume Accumulator PDA: ["global_volume_accumulator"] under pump program
		globalVolSeed := []byte("global_volume_accumulator")
		if pda, _, err := aSDK.FindProgramAddress([][]byte{globalVolSeed}, program); err == nil {
			accounts = append(accounts, aSDK.Meta(pda).WRITE())
		} else {
			accounts = append(accounts, aSDK.Meta(global).WRITE())
		}
		// #13 - User Volume Accumulator PDA: ["user_volume_accumulator", user]
		userVolSeed := []byte("user_volume_accumulator")
		if pda, _, err := aSDK.FindProgramAddress([][]byte{userVolSeed, user.Bytes()}, program); err == nil {
			accounts = append(accounts, aSDK.Meta(pda).WRITE())
		} else {
			accounts = append(accounts, aSDK.Meta(user).WRITE())
		}
		// #14 - Fee Config PDA under fee_program: ["fee_config", feeConfigSeedKey]
		if pda, _, err := aSDK.FindProgramAddress([][]byte{feeConfigSeedTag, feeConfigSeedKey}, PumpFeeProgramAddress); err == nil {
			accounts = append(accounts, aSDK.Meta(pda))
		} else {
			accounts = append(accounts, aSDK.Meta(PumpFeeProgramAddress))
		}
		// #15 - Fee Program
		accounts = append(accounts, aSDK.Meta(PumpFeeProgramAddress))
	}

	// Create instruction data
	data := make([]byte, 16) // 8 bytes discriminator + 8 bytes amount

	// Add buy instruction discriminator [102, 6, 61, 18, 1, 218, 235, 234]
	copy(data[0:8], []byte{102, 6, 61, 18, 1, 218, 235, 234})

	// Add amount parameter (u64, little endian)
	binary.LittleEndian.PutUint64(data[8:16], amount)

	// Add maxSolCost parameter (u64, little endian)
	data = append(data, make([]byte, 8)...)
	binary.LittleEndian.PutUint64(data[16:24], maxSolCost)
	// Add track_volume (bool)
	data = append(data, byte(1))

	return aSDK.NewInstruction(
		program,
		accounts,
		data,
	)
}
