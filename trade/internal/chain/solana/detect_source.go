package solana

import (
	"context"
	"fmt"

	"dex/pkg/constants"

	aSDK "github.com/gagliardetto/solana-go"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
)

// DetectTokenSource probes on-chain for which bonding-curve program a mint
// belongs to. Used as a fallback for a token the consumer hasn't indexed into
// the pair table yet (e.g. immediately after creation, or if its creation
// block was dropped) — the swap itself is always built from live on-chain
// state regardless of DB state, so this only needs to recover enough
// metadata (source name, curve address) for CreateMarketOrder to route and
// record the order correctly.
//
// Tries pump.fun, then both pump-meteora builds; returns the first program
// whose derived bonding-curve account actually exists on-chain.
func DetectTokenSource(ctx context.Context, client *ag_rpc.Client, mint string) (source string, pairAddress string, err error) {
	mintPk, err := aSDK.PublicKeyFromBase58(mint)
	if err != nil {
		return "", "", fmt.Errorf("invalid mint %q: %w", mint, err)
	}

	candidates := []struct {
		source  string
		program string
		seed    string
	}{
		{constants.PumpFun, constants.ProgramStrPumpFun, "bonding-curve"},
		{constants.PumpMeteora, constants.ProgramStrPumpMeteora, "bonding_curve"},
		{constants.PumpMeteoraV2, constants.ProgramStrPumpMeteoraOpt, "bonding_curve"},
	}
	for _, c := range candidates {
		programPk, err := aSDK.PublicKeyFromBase58(c.program)
		if err != nil {
			continue
		}
		curve, _, err := aSDK.FindProgramAddress([][]byte{[]byte(c.seed), mintPk.Bytes()}, programPk)
		if err != nil {
			continue
		}
		info, getErr := client.GetAccountInfo(ctx, curve)
		fmt.Println("PROBE_DEBUG_CANDIDATE", c.source, "program=", c.program, "curve=", curve.String(), "getErr=", getErr, "info nil?", info == nil)
		if getErr == nil && info != nil && info.Value != nil {
			return c.source, curve.String(), nil
		}
	}
	return "", "", fmt.Errorf("no bonding curve found on-chain for mint %s", mint)
}
