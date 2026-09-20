package pumpfun

// Builder for our own pump-meteora bonding-curve program (the second token source).
// Unlike pump.fun's WSOL-quoted buy_v2/sell_v2, pump-meteora's `swap` takes native SOL
// directly and the program creates the buyer's token ATA itself, so a single instruction
// is enough (plus the caller's compute-budget ixs).
//
// swap ix (13 accounts, validated against devnet):
//   [config, team_wallet, team_wallet_secondary, creator, bonding_curve, global_vault,
//    token_mint, global_ata, user_ata, user, system, token, associated_token]
// data = disc(global:swap) + amount(u64) + direction(u8: 0=buy,1=sell) + minimum_receive(u64)

import (
	"context"
	"encoding/binary"
	"fmt"

	aSDK "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const PumpMeteoraProgramID = "AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5"

var (
	meteoraProgramPk = aSDK.MustPublicKeyFromBase58(PumpMeteoraProgramID)
	meteoraSwapDisc  = []byte{248, 198, 158, 145, 225, 117, 135, 200} // sha256("global:swap")[:8]
)

func meteoraATA(owner, mint aSDK.PublicKey) aSDK.PublicKey {
	a, _, _ := aSDK.FindProgramAddress(
		[][]byte{owner.Bytes(), aSDK.TokenProgramID.Bytes(), mint.Bytes()},
		aSDK.SPLAssociatedTokenAccountProgramID,
	)
	return a
}

// BuildMeteoraSwapInstructions assembles a pump-meteora bonding-curve swap.
// amount is lamports for a buy (direction 0) or token base units for a sell (direction 1).
func BuildMeteoraSwapInstructions(rc *rpc.Client, user, mint aSDK.PublicKey, amount uint64, direction uint8, minReceive uint64) ([]aSDK.Instruction, error) {
	config := pumpPDA([][]byte{[]byte("config")}, meteoraProgramPk)
	globalVault := pumpPDA([][]byte{[]byte("global")}, meteoraProgramPk)
	bondingCurve := pumpPDA([][]byte{[]byte("bonding_curve"), mint.Bytes()}, meteoraProgramPk)
	globalAta := meteoraATA(globalVault, mint)
	userAta := meteoraATA(user, mint)

	// team wallets live in the (singleton) config account
	cfgInfo, err := rc.GetAccountInfo(context.Background(), config)
	if err != nil || cfgInfo == nil || cfgInfo.Value == nil || cfgInfo.Value.Data == nil {
		return nil, fmt.Errorf("meteora config not found: %v", err)
	}
	cfg := cfgInfo.Value.Data.GetBinary()
	if len(cfg) < 8+128 {
		return nil, fmt.Errorf("meteora config too short: %d", len(cfg))
	}
	teamWallet := aSDK.PublicKeyFromBytes(cfg[8+64 : 8+96])
	teamWalletSecondary := aSDK.PublicKeyFromBytes(cfg[8+96 : 8+128])

	// creator lives in the bonding-curve account
	bcInfo, err := rc.GetAccountInfo(context.Background(), bondingCurve)
	if err != nil || bcInfo == nil || bcInfo.Value == nil || bcInfo.Value.Data == nil {
		return nil, fmt.Errorf("meteora bonding curve not found for mint %s: %v", mint.String(), err)
	}
	bc := bcInfo.Value.Data.GetBinary()
	if len(bc) < 8+64 {
		return nil, fmt.Errorf("meteora bonding curve too short: %d", len(bc))
	}
	creator := aSDK.PublicKeyFromBytes(bc[8+32 : 8+64])

	data := make([]byte, 8+8+1+8)
	copy(data[0:8], meteoraSwapDisc)
	binary.LittleEndian.PutUint64(data[8:16], amount)
	data[16] = direction
	binary.LittleEndian.PutUint64(data[17:25], minReceive)

	accts := []*aSDK.AccountMeta{
		aSDK.NewAccountMeta(config, false, false),
		aSDK.NewAccountMeta(teamWallet, true, false),
		aSDK.NewAccountMeta(teamWalletSecondary, true, false),
		aSDK.NewAccountMeta(creator, true, false),
		aSDK.NewAccountMeta(bondingCurve, true, false),
		aSDK.NewAccountMeta(globalVault, true, false),
		aSDK.NewAccountMeta(mint, false, false),
		aSDK.NewAccountMeta(globalAta, true, false),
		aSDK.NewAccountMeta(userAta, true, false),
		aSDK.NewAccountMeta(user, true, true),
		aSDK.NewAccountMeta(aSDK.SystemProgramID, false, false),
		aSDK.NewAccountMeta(aSDK.TokenProgramID, false, false),
		aSDK.NewAccountMeta(aSDK.SPLAssociatedTokenAccountProgramID, false, false),
	}

	return []aSDK.Instruction{&rawInstruction{prog: meteoraProgramPk, accts: accts, data: data}}, nil
}

// --- optimized/hardened pump-meteora build (PumpMeteoraV2) ---
// Same 13-account swap, but its `swap` takes an extra max_sol_cost arg (33-byte data),
// and every PDA is derived under the optimized program id.

const PumpMeteoraOptProgramID = "241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ"

var meteoraOptProgramPk = aSDK.MustPublicKeyFromBase58(PumpMeteoraOptProgramID)

func meteoraOptATA(owner, mint aSDK.PublicKey) aSDK.PublicKey {
	a, _, _ := aSDK.FindProgramAddress(
		[][]byte{owner.Bytes(), aSDK.TokenProgramID.Bytes(), mint.Bytes()},
		aSDK.SPLAssociatedTokenAccountProgramID,
	)
	return a
}

// BuildMeteoraSwapInstructionsV2 assembles a swap against the optimized program.
// maxSolCost bounds the SOL a buy may cost (ignored by the program for sells).
func BuildMeteoraSwapInstructionsV2(rc *rpc.Client, user, mint aSDK.PublicKey, amount uint64, direction uint8, minReceive, maxSolCost uint64) ([]aSDK.Instruction, error) {
	config := pumpPDA([][]byte{[]byte("config")}, meteoraOptProgramPk)
	globalVault := pumpPDA([][]byte{[]byte("global")}, meteoraOptProgramPk)
	bondingCurve := pumpPDA([][]byte{[]byte("bonding_curve"), mint.Bytes()}, meteoraOptProgramPk)
	globalAta := meteoraOptATA(globalVault, mint)
	userAta := meteoraOptATA(user, mint)

	cfgInfo, err := rc.GetAccountInfo(context.Background(), config)
	if err != nil || cfgInfo == nil || cfgInfo.Value == nil || cfgInfo.Value.Data == nil {
		return nil, fmt.Errorf("meteora-opt config not found: %v", err)
	}
	cfg := cfgInfo.Value.Data.GetBinary()
	if len(cfg) < 8+128 {
		return nil, fmt.Errorf("meteora-opt config too short: %d", len(cfg))
	}
	teamWallet := aSDK.PublicKeyFromBytes(cfg[8+64 : 8+96])
	teamWalletSecondary := aSDK.PublicKeyFromBytes(cfg[8+96 : 8+128])

	bcInfo, err := rc.GetAccountInfo(context.Background(), bondingCurve)
	if err != nil || bcInfo == nil || bcInfo.Value == nil || bcInfo.Value.Data == nil {
		return nil, fmt.Errorf("meteora-opt bonding curve not found for mint %s: %v", mint.String(), err)
	}
	bc := bcInfo.Value.Data.GetBinary()
	if len(bc) < 8+64 {
		return nil, fmt.Errorf("meteora-opt bonding curve too short: %d", len(bc))
	}
	creator := aSDK.PublicKeyFromBytes(bc[8+32 : 8+64])

	// data = disc + amount(u64) + direction(u8) + minimum_receive(u64) + max_sol_cost(u64)
	data := make([]byte, 8+8+1+8+8)
	copy(data[0:8], meteoraSwapDisc)
	binary.LittleEndian.PutUint64(data[8:16], amount)
	data[16] = direction
	binary.LittleEndian.PutUint64(data[17:25], minReceive)
	binary.LittleEndian.PutUint64(data[25:33], maxSolCost)

	accts := []*aSDK.AccountMeta{
		aSDK.NewAccountMeta(config, false, false),
		aSDK.NewAccountMeta(teamWallet, true, false),
		aSDK.NewAccountMeta(teamWalletSecondary, true, false),
		aSDK.NewAccountMeta(creator, true, false),
		aSDK.NewAccountMeta(bondingCurve, true, false),
		aSDK.NewAccountMeta(globalVault, true, false),
		aSDK.NewAccountMeta(mint, false, false),
		aSDK.NewAccountMeta(globalAta, true, false),
		aSDK.NewAccountMeta(userAta, true, false),
		aSDK.NewAccountMeta(user, true, true),
		aSDK.NewAccountMeta(aSDK.SystemProgramID, false, false),
		aSDK.NewAccountMeta(aSDK.TokenProgramID, false, false),
		aSDK.NewAccountMeta(aSDK.SPLAssociatedTokenAccountProgramID, false, false),
	}

	return []aSDK.Instruction{&rawInstruction{prog: meteoraOptProgramPk, accts: accts, data: data}}, nil
}
