package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"

	"math/big"
	"strconv"

	"dex/pkg/pumpfun/pump/idl/generated/pump"
	"dex/pkg/trade"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// BuildSellInstruction is a function that returns the pump.fun instructions to sell the token
func BuildSellInstruction(ata, user, mint solana.PublicKey, sellTokenAmount uint64, slippageBasisPoint uint32,
	all bool, rpcClient *rpc.Client, price float64, inDecimal, outDecimal uint8) (*pump.Instruction, uint64, error) {
	if all {
		tokenAccounts, err := rpcClient.GetTokenAccountBalance(context.TODO(), ata, rpc.CommitmentConfirmed)
		if err != nil {
			return nil, 0, fmt.Errorf("can't get amount of token in balance: %w", err)
		}
		amount, err := strconv.Atoi(tokenAccounts.Value.Amount)
		if err != nil {
			return nil, 0, fmt.Errorf("can't convert token amount to integer: %w", err)
		}
		sellTokenAmount = uint64(amount)
	}
	bondingCurveData, err := GetBondingCurveAndAssociatedBondingCurve(mint)
	if err != nil {
		return nil, 0, fmt.Errorf("can't get bonding curve data: %w", err)
	}

	var minSolOutputUint64, solOutput uint64
	// 如果价格不为空 那么按照价格走而不是恒乘积走
	if price != 0 {
		minSolOutputUint64, solOutput, err = trade.CalcMinAmountOutByPrice(slippageBasisPoint, sellTokenAmount, false, price, inDecimal, outDecimal, trade.PumpFee)
		if err != nil {
			return nil, 0, err
		}
	} else {
		bondingCurve, err := FetchBondingCurve(rpcClient, bondingCurveData.BondingCurve)
		if err != nil {
			return nil, 0, fmt.Errorf("can't fetch bonding curve: %w", err)
		}

		//percentage := float64(1.0 - (slippageBasisPoint / 10e3))

		slippage := big.NewFloat(float64(1))
		slippage = slippage.Quo(big.NewFloat(float64(slippageBasisPoint)), big.NewFloat(float64(1e4)))

		slippageF64, _ := slippage.Float64()
		percentage := float64(1.0 - slippageF64)

		minSolOutputUint64, solOutput = calculateSellQuote(sellTokenAmount, bondingCurve, percentage)
	}

	// Get creator and vault dynamically using the new method
	creator, creatorVault, err := GetCreatorAndVault(mint.String(), rpcClient)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get creator and vault: %w", err)
	}

	// Debug log the creator and vault
	fmt.Printf("DEBUG SELL: Token mint: %s\n", mint.String())
	fmt.Printf("DEBUG SELL: Creator: %s\n", creator.String())
	fmt.Printf("DEBUG SELL: Creator vault: %s\n", creatorVault.String())

	// Create custom sell instruction with creator vault
	sellInstr := buildSellInstructionWithCreatorVault(
		sellTokenAmount,
		minSolOutputUint64,
		GlobalPumpFunAddress,
		PumpFunFeeRecipient,
		mint,
		bondingCurveData.BondingCurve,
		bondingCurveData.AssociatedBondingCurve,
		ata,
		user,
		system.ProgramID,
		token.ProgramID,
		PumpFunEventAuthority,
		creatorVault, // Add creator vault
		pump.ProgramID,
	)

	// Wrap the custom instruction in pump.Instruction format
	pumpInstr := &pump.Instruction{
		BaseVariant: ag_binary.BaseVariant{
			Impl:   sellInstr,
			TypeID: pump.Instruction_Sell,
		},
	}

	return pumpInstr, solOutput, nil
}

// getMintAuthorityFromSell fetches the mint authority from the mint account (separate function to avoid duplicate)
func getMintAuthorityFromSell(rpcClient *rpc.Client, mintAddress solana.PublicKey) (solana.PublicKey, error) {
	accountInfo, err := rpcClient.GetAccountInfoWithOpts(context.TODO(), mintAddress, &rpc.GetAccountInfoOpts{
		Encoding:   solana.EncodingBase64,
		Commitment: rpc.CommitmentProcessed,
	})
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to get mint account info: %w", err)
	}

	if accountInfo.Value == nil {
		return solana.PublicKey{}, fmt.Errorf("mint account not found")
	}

	data := accountInfo.Value.Data.GetBinary()
	if len(data) < 64 {
		return solana.PublicKey{}, fmt.Errorf("invalid mint account data length")
	}

	// Parse mint authority from mint account data (offset 4-36 for mint authority)
	var mintAuthority solana.PublicKey
	copy(mintAuthority[:], data[4:36])

	return mintAuthority, nil
}

// buildSellInstructionWithCreatorVault creates a sell instruction with creator vault account
func buildSellInstructionWithCreatorVault(
	amount uint64,
	minSolCost uint64,
	global solana.PublicKey,
	feeRecipient solana.PublicKey,
	mint solana.PublicKey,
	bondingCurve solana.PublicKey,
	associatedBondingCurve solana.PublicKey,
	associatedUser solana.PublicKey,
	user solana.PublicKey,
	systemProgram solana.PublicKey,
	tokenProgram solana.PublicKey,
	eventAuthority solana.PublicKey,
	creatorVault solana.PublicKey,
	program solana.PublicKey,
) solana.Instruction {
	// Create accounts slice matching the correct PumpFun order (12 accounts total)
	accounts := []*solana.AccountMeta{
		solana.Meta(global).WRITE(),                 // #0 - Global (WRITABLE)
		solana.Meta(feeRecipient).WRITE(),           // #1 - Fee Recipient (WRITABLE)
		solana.Meta(mint).WRITE(),                   // #2 - Mint (WRITABLE)
		solana.Meta(bondingCurve).WRITE(),           // #3 - Bonding Curve (WRITABLE)
		solana.Meta(associatedBondingCurve).WRITE(), // #4 - Associated Bonding Curve (WRITABLE)
		solana.Meta(associatedUser).WRITE(),         // #5 - Associated User (WRITABLE)
		solana.Meta(user).WRITE().SIGNER(),          // #6 - User (WRITABLE, SIGNER)
		solana.Meta(systemProgram),                  // #7 - System Program
		solana.Meta(tokenProgram).WRITE(),           // #8 - Token Program (WRITABLE)
		solana.Meta(creatorVault).WRITE(),           // #9 - Creator Vault (WRITABLE)
		solana.Meta(eventAuthority).WRITE(),         // #10 - Event Authority (WRITABLE)
		solana.Meta(program),                        // #11 - Program
	}

	// Create instruction data
	data := make([]byte, 24) // 8 bytes discriminator + 8 bytes for each u64 parameter

	// Add sell instruction discriminator [51, 230, 133, 164, 1, 127, 131, 173]
	copy(data[0:8], []byte{51, 230, 133, 164, 1, 127, 131, 173})

	// Add amount parameter (u64, little endian)
	binary.LittleEndian.PutUint64(data[8:16], amount)

	// Add minSolCost parameter (u64, little endian)
	binary.LittleEndian.PutUint64(data[16:24], minSolCost)

	return solana.NewInstruction(
		program,
		accounts,
		data,
	)
}

// calculateSellQuote calculates how many SOL should be received for selling a specific amount of tokens, given a specific amount of token, bonding curve data, and percentage.
// tokenAmount is the amount of token you want to sell
// bondingCurve is the bonding curve data, that will help to calculate the number of sol to get
// percentage is the slippage, 0.98 means 2% slippage
func calculateSellQuote(tokenAmount uint64, bondingCurve *BondingCurveData, percentage float64) (uint64, uint64) {
	amount := big.NewInt(int64(tokenAmount))

	// Clone bonding curve data to avoid mutations
	virtualSolReserves := new(big.Int).Set(bondingCurve.VirtualSolReserves)
	virtualTokenReserves := new(big.Int).Set(bondingCurve.VirtualTokenReserves)

	// Compute the new virtual reserves
	x := new(big.Int).Mul(virtualSolReserves, virtualTokenReserves)
	y := new(big.Int).Add(virtualTokenReserves, amount)
	a := new(big.Int).Div(x, y)
	out := new(big.Int).Sub(virtualSolReserves, a)
	percentageMultiplier := big.NewFloat(percentage)

	outFloat := new(big.Float).SetInt(out)
	number := new(big.Float).Mul(outFloat, percentageMultiplier)
	final, _ := number.Int(nil)
	return final.Uint64(), out.Uint64()
}
