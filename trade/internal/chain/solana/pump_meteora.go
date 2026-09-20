package solana

import (
	"context"
	"errors"

	pumpfun "dex/pkg/pumpfun/pump"
	"dex/pkg/sol"
	"dex/pkg/xcode"

	ag_solanago "github.com/gagliardetto/solana-go"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

// meteoraOrderSetup does the shared work for both pump-meteora builds: resolve direction,
// amount, compute-budget instructions and the SOL balance check. Returns the base
// instructions, the direction (0=buy/1=sell), the token mint and the amount in base units.
func (tm *TxManager) meteoraOrderSetup(ctx context.Context, in *CreateMarketTx) ([]ag_solanago.Instruction, uint8, ag_solanago.PublicKey, uint64, error) {
	initiator := in.UserWalletAccount
	if in.InMint != ag_solanago.WrappedSol && in.OutMint != ag_solanago.WrappedSol {
		return nil, 0, ag_solanago.PublicKey{}, 0, errors.New("ErrWrongMint")
	}

	var direction uint8 = 0 // buy: SOL -> token
	tokenMint := in.OutMint
	if in.OutMint == ag_solanago.WrappedSol {
		direction = 1 // sell: token -> SOL
		tokenMint = in.InMint
	}

	amtDecimal, err := decimal.NewFromString(in.AmountIn)
	if err != nil {
		return nil, 0, ag_solanago.PublicKey{}, 0, err
	}
	amtDecimal = amtDecimal.Mul(decimal.NewFromInt(sol.Decimals2Value[in.InDecimal]))
	amountUint64 := uint64(amtDecimal.IntPart())
	if amountUint64 == 0 {
		return nil, 0, ag_solanago.PublicKey{}, 0, errors.New("amount is zero")
	}

	instructions, lamportCostFee, err := tm.CreateGasAndJitoByGasFee(ctx, in.IsAntiMev, initiator, sol.PumpFunSwapCU, sol.GasMODE[1])
	if err != nil {
		return nil, 0, ag_solanago.PublicKey{}, 0, err
	}
	lamportCost := tm.rentFee + lamportCostFee

	solBalanceInfo, err := tm.Client.GetBalance(ctx, initiator, ag_rpc.CommitmentFinalized)
	if err != nil {
		return nil, 0, ag_solanago.PublicKey{}, 0, err
	}
	solBalance := solBalanceInfo.Value
	if direction == 0 {
		if solBalance < amountUint64 {
			return nil, 0, ag_solanago.PublicKey{}, 0, xcode.SolBalanceNotEnough
		}
		lamportCost += amountUint64
	}
	if lamportCost > solBalance {
		return nil, 0, ag_solanago.PublicKey{}, 0, xcode.SolGasNotEnough
	}
	return instructions, direction, tokenMint, amountUint64, nil
}

// CreateMarketOrder4PumpMeteora builds a buy/sell against the original pump-meteora program.
func (tm *TxManager) CreateMarketOrder4PumpMeteora(ctx context.Context, in *CreateMarketTx) ([]ag_solanago.Instruction, error) {
	instructions, direction, tokenMint, amountUint64, err := tm.meteoraOrderSetup(ctx, in)
	if err != nil {
		return nil, err
	}
	logx.WithContext(ctx).Infof("CreateMarketOrder4PumpMeteora dir=%d mint=%s amount=%d", direction, tokenMint.String(), amountUint64)
	// minReceive=0: bonding-curve price impact is deterministic per-tx (TODO: quote+slippage).
	swapIxs, err := pumpfun.BuildMeteoraSwapInstructions(tm.Client, in.UserWalletAccount, tokenMint, amountUint64, direction, 0)
	if err != nil {
		return nil, err
	}
	return append(instructions, swapIxs...), nil
}

// CreateMarketOrder4PumpMeteoraV2 builds a buy/sell against the optimized program, which
// takes an extra max_sol_cost. For a buy we cap it at the offered SOL (`amountUint64`) —
// the program's principal is always <= amount minus fees, so this never rejects a normal
// buy; for a sell max_sol_cost is ignored by the program.
func (tm *TxManager) CreateMarketOrder4PumpMeteoraV2(ctx context.Context, in *CreateMarketTx) ([]ag_solanago.Instruction, error) {
	instructions, direction, tokenMint, amountUint64, err := tm.meteoraOrderSetup(ctx, in)
	if err != nil {
		return nil, err
	}
	logx.WithContext(ctx).Infof("CreateMarketOrder4PumpMeteoraV2 dir=%d mint=%s amount=%d", direction, tokenMint.String(), amountUint64)
	swapIxs, err := pumpfun.BuildMeteoraSwapInstructionsV2(tm.Client, in.UserWalletAccount, tokenMint, amountUint64, direction, 0, amountUint64)
	if err != nil {
		return nil, err
	}
	return append(instructions, swapIxs...), nil
}
