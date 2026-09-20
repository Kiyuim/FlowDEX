package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	aSDK "github.com/gagliardetto/solana-go"
	tokprog "github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// sell_v2 discriminator (LE uint64 0xb240e9e73c82f65d), the WSOL-quoted sell that
// mirrors buy_v2. Required for Token-2022 and buyback-enabled mints, which the
// legacy `sell` rejects (token_program InvalidProgramId / buyback missing).
var sellV2Discriminator = []byte{93, 246, 130, 60, 231, 233, 64, 178}

// BuildSellV2Instructions assembles a pump.fun bonding-curve sell via sell_v2.
// It mirrors BuildBuyV2Instructions in reverse: the user sends base tokens and
// receives WSOL, which is unwrapped back to native SOL. Returns the instruction
// list and the estimated gross SOL output (lamports) for order metadata.
//
//   - baseTokenProgram: the token program that owns `mint` (SPL Token or Token-2022)
//   - tokenAmount: base units of the token to sell
//   - slippageBps: slippage tolerance in basis points (e.g. 1000 = 10%)
func BuildSellV2Instructions(rc *rpc.Client, user, mint, baseTokenProgram aSDK.PublicKey, tokenAmount uint64, slippageBps uint32) ([]aSDK.Instruction, uint64, error) {
	if tokenAmount == 0 {
		return nil, 0, fmt.Errorf("sell_v2: tokenAmount is 0")
	}
	wsol := aSDK.SolMint
	tp := aSDK.TokenProgramID // quote (WSOL) is always standard SPL Token
	if baseTokenProgram.IsZero() {
		baseTokenProgram = tp
	}

	bondingCurve := pumpPDA([][]byte{[]byte("bonding-curve"), mint.Bytes()}, pumpProgramPk)

	// Read the bonding curve once (with retry) — reserves for the quote, creator
	// for the creator vault, and the mayhem flag.
	rawCurve, err := getAccountWithRetry(rc, bondingCurve, 3)
	if err != nil {
		return nil, 0, fmt.Errorf("sell_v2: fetch bonding curve: %w", err)
	}
	if len(rawCurve) < 81 {
		return nil, 0, fmt.Errorf("sell_v2: bonding curve data too short (%d bytes)", len(rawCurve))
	}

	curve := &BondingCurveData{
		RealTokenReserves:    big.NewInt(0),
		VirtualTokenReserves: new(big.Int).SetUint64(binary.LittleEndian.Uint64(rawCurve[8:16])),
		VirtualSolReserves:   new(big.Int).SetUint64(binary.LittleEndian.Uint64(rawCurve[16:24])),
	}
	if curve.VirtualSolReserves.Sign() == 0 || curve.VirtualTokenReserves.Sign() == 0 {
		vToken, vSol := globalInitialReserves(rc)
		curve.VirtualTokenReserves = new(big.Int).SetUint64(vToken)
		curve.VirtualSolReserves = new(big.Int).SetUint64(vSol)
	}
	pct := 1.0 - float64(slippageBps)/1e4
	if pct < 0 {
		pct = 0
	}
	minSolOutput, solOutput := calculateSellQuote(tokenAmount, curve, pct)

	creator := aSDK.PublicKeyFromBytes(rawCurve[49:81])
	creatorVault := pumpPDA([][]byte{[]byte("creator-vault"), creator.Bytes()}, pumpProgramPk)

	feeRecipient := pumpFeeRecipientV2
	if bondingCurveIsMayhem(rawCurve) {
		feeRecipient = pumpReservedFeeRecipient
	}
	ataQuoteFeeRecipient := ataAddr(feeRecipient, wsol)

	// NOTE: sell_v2 has 26 accounts (verified against an on-chain devnet sell_v2)
	// — it omits the global_volume_accumulator that buy_v2 carries.
	uva := pumpPDA([][]byte{[]byte("user_volume_accumulator"), user.Bytes()}, pumpProgramPk)
	sharingConfig := pumpPDA([][]byte{[]byte("sharing-config"), mint.Bytes()}, pumpFeeProgramPk)
	feeConfig := pumpPDA([][]byte{[]byte("fee_config"), pumpProgramPk.Bytes()}, pumpFeeProgramPk)
	global := pumpPDA([][]byte{[]byte("global")}, pumpProgramPk)
	eventAuthority := pumpPDA([][]byte{[]byte("__event_authority")}, pumpProgramPk)
	quoteUser := ataAddr(user, wsol)

	var ixs []aSDK.Instruction

	// Probe which setup accounts already exist to keep the tx within size limits.
	ataBase := ataAddrProg(user, mint, baseTokenProgram)
	ataQuoteBC := ataAddr(bondingCurve, wsol)
	ataQuoteCV := ataAddr(creatorVault, wsol)
	ataQuoteUVA := ataAddr(uva, wsol)
	probe := []aSDK.PublicKey{uva, ataBase, quoteUser, ataQuoteBC, ataQuoteCV, ataQuoteUVA, ataQuoteFeeRecipient}
	exists := make(map[string]bool)
	if res, e := rc.GetMultipleAccounts(context.Background(), probe...); e == nil && res != nil {
		for i, acc := range res.Value {
			if i < len(probe) && acc != nil {
				exists[probe[i].String()] = true
			}
		}
	}

	if !exists[uva.String()] {
		ixs = append(ixs, &rawInstruction{
			prog: pumpProgramPk,
			accts: []*aSDK.AccountMeta{
				aSDK.Meta(user).WRITE().SIGNER(),
				aSDK.Meta(user),
				aSDK.Meta(uva).WRITE(),
				aSDK.Meta(aSDK.SystemProgramID),
				aSDK.Meta(eventAuthority),
				aSDK.Meta(pumpProgramPk),
			},
			data: initUVADiscriminator,
		})
	}
	// Required ATAs (idempotent). The base ATA already exists (the user holds the
	// tokens) but an idempotent create is a no-op. The user's WSOL ATA must exist
	// to receive the proceeds.
	if !exists[ataBase.String()] {
		ixs = append(ixs, createIdempotentATA(user, user, mint, baseTokenProgram))
	}
	if !exists[quoteUser.String()] {
		ixs = append(ixs, createIdempotentATA(user, user, wsol, tp))
	}
	if !exists[ataQuoteBC.String()] {
		ixs = append(ixs, createIdempotentATA(user, bondingCurve, wsol, tp))
	}
	if !exists[ataQuoteCV.String()] {
		ixs = append(ixs, createIdempotentATA(user, creatorVault, wsol, tp))
	}
	if !exists[ataQuoteUVA.String()] {
		ixs = append(ixs, createIdempotentATA(user, uva, wsol, tp))
	}
	if !exists[ataQuoteFeeRecipient.String()] {
		ixs = append(ixs, createIdempotentATA(user, feeRecipient, wsol, tp))
	}

	// sell_v2 (27 accounts — same layout as buy_v2).
	accts := []*aSDK.AccountMeta{
		aSDK.Meta(global),
		aSDK.Meta(mint),
		aSDK.Meta(wsol),
		aSDK.Meta(baseTokenProgram),
		aSDK.Meta(tp),
		aSDK.Meta(aSDK.SPLAssociatedTokenAccountProgramID),
		aSDK.Meta(feeRecipient).WRITE(),
		aSDK.Meta(ataQuoteFeeRecipient).WRITE(),
		aSDK.Meta(pumpBuybackFeeRecipient).WRITE(),
		aSDK.Meta(ataAddr(pumpBuybackFeeRecipient, wsol)).WRITE(),
		aSDK.Meta(bondingCurve).WRITE(),
		aSDK.Meta(ataAddrProg(bondingCurve, mint, baseTokenProgram)).WRITE(),
		aSDK.Meta(ataAddr(bondingCurve, wsol)).WRITE(),
		aSDK.Meta(user).WRITE().SIGNER(),
		aSDK.Meta(ataAddrProg(user, mint, baseTokenProgram)).WRITE(),
		aSDK.Meta(quoteUser).WRITE(),
		aSDK.Meta(creatorVault).WRITE(),
		aSDK.Meta(ataAddr(creatorVault, wsol)).WRITE(),
		aSDK.Meta(sharingConfig),
		aSDK.Meta(uva).WRITE(),
		aSDK.Meta(ataAddr(uva, wsol)).WRITE(),
		aSDK.Meta(feeConfig),
		aSDK.Meta(pumpFeeProgramPk),
		aSDK.Meta(aSDK.SystemProgramID),
		aSDK.Meta(eventAuthority),
		aSDK.Meta(pumpProgramPk),
	}
	data := make([]byte, 24)
	copy(data[0:8], sellV2Discriminator)
	binary.LittleEndian.PutUint64(data[8:16], tokenAmount)   // amount (base tokens in)
	binary.LittleEndian.PutUint64(data[16:24], minSolOutput) // min_sol_output (lamports)
	ixs = append(ixs, &rawInstruction{prog: pumpProgramPk, accts: accts, data: data})

	// Unwrap the received WSOL back to native SOL.
	ixs = append(ixs, tokprog.NewCloseAccountInstruction(quoteUser, user, user, nil).Build())

	return ixs, solOutput, nil
}
