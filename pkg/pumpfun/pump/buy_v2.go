package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	aSDK "github.com/gagliardetto/solana-go"
	sysprog "github.com/gagliardetto/solana-go/programs/system"
	tokprog "github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// Current devnet/mainnet pump.fun buy path. The legacy `buy` instruction is
// rejected for buyback-enabled tokens (BuybackFeeRecipientMissing 6062); the
// program now routes bonding-curve buys through `buy_v2`, which is WSOL-quoted
// and takes a wider account set (buyback recipient, sharing config, quote ATAs).
var (
	buyV2Discriminator      = []byte{184, 23, 238, 97, 103, 197, 211, 61}
	initUVADiscriminator    = []byte{94, 6, 202, 115, 255, 96, 232, 183}
	pumpProgramPk           = aSDK.MustPublicKeyFromBase58(PumpFunProgramID)
	pumpFeeProgramPk        = PumpFeeProgramAddress // pfeeUxB6...
	pumpFeeRecipientV2      = aSDK.MustPublicKeyFromBase58("6QgPshH1egekJ2TURfakiiApDdv98qfRuRe7RectX8xs")
	pumpBuybackFeeRecipient = aSDK.MustPublicKeyFromBase58("5YxQFdt3Tr9zJLvkFccqXVUwhdTWJQc1fFg2YPbxvxeD")
	// Mayhem-mode tokens require a fee recipient from the global config's
	// reserved_fee_recipient set (the normal fee_recipients are NotAuthorized for
	// them). This is the global.reserved_fee_recipient on devnet.
	pumpReservedFeeRecipient = aSDK.MustPublicKeyFromBase58("GesfTA3X2arioaHp8bbKdjG9vJtskViWACZoYvxp4twS")
)

// bondingCurveIsMayhem reports whether the bonding curve has is_mayhem_mode set.
// Layout after the 8-byte discriminator: 5×u64 (40) + complete(1) + creator(32),
// so is_mayhem_mode is at byte offset 81.
func bondingCurveIsMayhem(raw []byte) bool {
	const mayhemOffset = 8 + 40 + 1 + 32
	return len(raw) > mayhemOffset && raw[mayhemOffset] != 0
}

type rawInstruction struct {
	prog  aSDK.PublicKey
	accts []*aSDK.AccountMeta
	data  []byte
}

func (r *rawInstruction) ProgramID() aSDK.PublicKey     { return r.prog }
func (r *rawInstruction) Accounts() []*aSDK.AccountMeta { return r.accts }
func (r *rawInstruction) Data() ([]byte, error)         { return r.data, nil }

func pumpPDA(seeds [][]byte, prog aSDK.PublicKey) aSDK.PublicKey {
	p, _, _ := aSDK.FindProgramAddress(seeds, prog)
	return p
}

// globalInitialReserves returns the pump global config's initial virtual token
// and sol reserves (offsets 73 and 81 after the discriminator), used to price
// the first buy of a freshly-created curve. Falls back to the known devnet
// defaults if the account can't be read.
func globalInitialReserves(rc *rpc.Client) (vToken, vSol uint64) {
	vToken, vSol = 1073000000000000, 1000000000
	global := pumpPDA([][]byte{[]byte("global")}, pumpProgramPk)
	if info, e := rc.GetAccountInfo(context.Background(), global); e == nil && info != nil && info.Value != nil {
		d := info.Value.Data.GetBinary()
		if len(d) >= 89 {
			if t, s := binary.LittleEndian.Uint64(d[73:81]), binary.LittleEndian.Uint64(d[81:89]); t > 0 && s > 0 {
				vToken, vSol = t, s
			}
		}
	}
	return
}

func ataAddr(owner, mint aSDK.PublicKey) aSDK.PublicKey {
	a, _, _ := aSDK.FindAssociatedTokenAddress(owner, mint)
	return a
}

// ataAddrProg derives the associated token account for (owner, mint) under a
// specific token program (standard SPL Token or Token-2022).
func ataAddrProg(owner, mint, tokenProg aSDK.PublicKey) aSDK.PublicKey {
	a, _, _ := aSDK.FindProgramAddress(
		[][]byte{owner.Bytes(), tokenProg.Bytes(), mint.Bytes()},
		aSDK.SPLAssociatedTokenAccountProgramID,
	)
	return a
}

// createIdempotentATA builds a create-idempotent ATA instruction (data=[1]) for
// (owner, mint) under the given token program, paid by payer.
func createIdempotentATA(payer, owner, mint, tokenProg aSDK.PublicKey) aSDK.Instruction {
	return &rawInstruction{
		prog: aSDK.SPLAssociatedTokenAccountProgramID,
		accts: []*aSDK.AccountMeta{
			aSDK.Meta(payer).WRITE().SIGNER(),
			aSDK.Meta(ataAddrProg(owner, mint, tokenProg)).WRITE(),
			aSDK.Meta(owner),
			aSDK.Meta(mint),
			aSDK.Meta(aSDK.SystemProgramID),
			aSDK.Meta(tokenProg),
		},
		data: []byte{1},
	}
}

// getAccountWithRetry fetches an account's raw data, retrying transient RPC
// failures. The public devnet RPC is slow (~1s/call) and rate-limits, so a
// single miss shouldn't fail the whole buy build.
func getAccountWithRetry(rc *rpc.Client, addr aSDK.PublicKey, attempts int) ([]byte, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		info, err := rc.GetAccountInfo(context.Background(), addr)
		if err == nil && info != nil && info.Value != nil && info.Value.Data != nil {
			return info.Value.Data.GetBinary(), nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("account not found: %s", addr)
		}
	}
	return nil, lastErr
}

// BuildBuyV2Instructions assembles the full instruction list for a pump.fun
// bonding-curve buy via `buy_v2`: (optional) user-volume-accumulator init, the
// required ATAs, WSOL wrapping of the spend, the buy, and unwrapping the
// remainder. All instructions are signed by `user` (suitable for an unsigned tx
// returned to the wallet to sign).
//
//   - baseTokenProgram: the token program that owns `mint` (SPL Token or Token-2022)
//   - solAmount:   lamports the user intends to spend (also the max_sol_cost cap)
//   - slippageBps: slippage tolerance in basis points (e.g. 1000 = 10%)
func BuildBuyV2Instructions(rc *rpc.Client, user, mint, baseTokenProgram aSDK.PublicKey, solAmount uint64, slippageBps uint32) ([]aSDK.Instruction, error) {
	if solAmount == 0 {
		return nil, fmt.Errorf("buy_v2: solAmount is 0")
	}
	wsol := aSDK.SolMint
	tp := aSDK.TokenProgramID // quote (WSOL) is always standard SPL Token
	if baseTokenProgram.IsZero() {
		baseTokenProgram = tp
	}

	bondingCurve := pumpPDA([][]byte{[]byte("bonding-curve"), mint.Bytes()}, pumpProgramPk)

	// Read the bonding curve ONCE (with retry) and derive everything from it:
	// reserves (for the quote), the stored creator (for the creator vault) and the
	// mayhem flag. Doing three separate reads on the slow, rate-limited devnet RPC
	// was causing intermittent build failures (515) on the buy.
	rawCurve, err := getAccountWithRetry(rc, bondingCurve, 3)
	if err != nil {
		return nil, fmt.Errorf("buy_v2: fetch bonding curve: %w", err)
	}
	if len(rawCurve) < 81 {
		return nil, fmt.Errorf("buy_v2: bonding curve data too short (%d bytes)", len(rawCurve))
	}

	// Quote: reserves at [8:16] (virtual_token) and [16:24] (virtual_quote/sol).
	curve := &BondingCurveData{
		RealTokenReserves:    big.NewInt(0),
		VirtualTokenReserves: new(big.Int).SetUint64(binary.LittleEndian.Uint64(rawCurve[8:16])),
		VirtualSolReserves:   new(big.Int).SetUint64(binary.LittleEndian.Uint64(rawCurve[16:24])),
	}
	// A freshly-created curve stores zero reserves until its first trade; the
	// first buy is priced against the global config's initial virtual reserves.
	if curve.VirtualSolReserves.Sign() == 0 || curve.VirtualTokenReserves.Sign() == 0 {
		vToken, vSol := globalInitialReserves(rc)
		curve.VirtualTokenReserves = new(big.Int).SetUint64(vToken)
		curve.VirtualSolReserves = new(big.Int).SetUint64(vSol)
	}

	pct := 1.0 - float64(slippageBps)/1e4
	if pct < 0 {
		pct = 0
	}
	minTokensOut := CalculateBuyQuote(solAmount, curve, pct)
	if minTokensOut == 0 {
		return nil, fmt.Errorf("buy_v2: computed 0 tokens out for %d lamports", solAmount)
	}

	// Creator vault: keyed by the curve's stored creator (offset 49 = 8 disc +
	// 5×u64 + complete bool).
	creator := aSDK.PublicKeyFromBytes(rawCurve[49:81])
	creatorVault := pumpPDA([][]byte{[]byte("creator-vault"), creator.Bytes()}, pumpProgramPk)

	// Mayhem-mode tokens (is_mayhem_mode at offset 81) require a reserved fee
	// recipient; the normal set is NotAuthorized for them.
	feeRecipient := pumpFeeRecipientV2
	if bondingCurveIsMayhem(rawCurve) {
		feeRecipient = pumpReservedFeeRecipient
	}
	ataQuoteFeeRecipient := ataAddr(feeRecipient, wsol)

	gva := pumpPDA([][]byte{[]byte("global_volume_accumulator")}, pumpProgramPk)
	uva := pumpPDA([][]byte{[]byte("user_volume_accumulator"), user.Bytes()}, pumpProgramPk)
	sharingConfig := pumpPDA([][]byte{[]byte("sharing-config"), mint.Bytes()}, pumpFeeProgramPk)
	feeConfig := pumpPDA([][]byte{[]byte("fee_config"), pumpProgramPk.Bytes()}, pumpFeeProgramPk)
	global := pumpPDA([][]byte{[]byte("global")}, pumpProgramPk)
	eventAuthority := pumpPDA([][]byte{[]byte("__event_authority")}, pumpProgramPk)
	quoteUser := ataAddr(user, wsol)

	var ixs []aSDK.Instruction

	// buy_v2 references many accounts, so the whole tx is close to the 1232-byte
	// legacy limit. Only emit the setup instructions we actually need by probing
	// which accounts already exist on-chain (one batched RPC call). For a token
	// that has been traded before, the curve/creator/accumulator quote ATAs
	// already exist and can be skipped, keeping the tx within the size limit.
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

	// Init the user volume accumulator only if it doesn't exist yet.
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

	// Required ATAs (idempotent) — only the ones that don't already exist. The
	// user's quote (WSOL) ATA is closed after each buy to reclaim rent, so it is
	// typically recreated here.
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
	// The fee recipient's WSOL ATA must exist for buy_v2; reserved (mayhem) fee
	// recipients may not have one yet, so create it if missing.
	if !exists[ataQuoteFeeRecipient.String()] {
		ixs = append(ixs, createIdempotentATA(user, feeRecipient, wsol, tp))
	}

	// Wrap the spend: transfer lamports into the user's WSOL ATA and sync.
	ixs = append(ixs,
		sysprog.NewTransferInstruction(solAmount, user, quoteUser).Build(),
		tokprog.NewSyncNativeInstruction(quoteUser).Build(),
	)

	// buy_v2 (27 accounts).
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
		aSDK.Meta(gva),
		aSDK.Meta(uva).WRITE(),
		aSDK.Meta(ataAddr(uva, wsol)).WRITE(),
		aSDK.Meta(feeConfig),
		aSDK.Meta(pumpFeeProgramPk),
		aSDK.Meta(aSDK.SystemProgramID),
		aSDK.Meta(eventAuthority),
		aSDK.Meta(pumpProgramPk),
	}
	data := make([]byte, 24)
	copy(data[0:8], buyV2Discriminator)
	binary.LittleEndian.PutUint64(data[8:16], minTokensOut) // amount (tokens out)
	binary.LittleEndian.PutUint64(data[16:24], solAmount)   // max_sol_cost
	ixs = append(ixs, &rawInstruction{prog: pumpProgramPk, accts: accts, data: data})

	// Unwrap any leftover WSOL back to native SOL.
	ixs = append(ixs, tokprog.NewCloseAccountInstruction(quoteUser, user, user, nil).Build())

	return ixs, nil
}
