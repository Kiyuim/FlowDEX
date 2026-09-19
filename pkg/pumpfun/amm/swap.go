package amm

import (
	"fmt"

	"dex/pkg/constants"
	"dex/pkg/pumpfun/amm/idl/generated/amm"

	ag_solanago "github.com/gagliardetto/solana-go"
)

type SwapDirection int

const (
	BuyDirection SwapDirection = iota
	SellDirection
)

type SwapParam struct {
	// Direction
	Direction SwapDirection
	// Parameters:
	TokenAmount1 uint64 // BaseAmountOut(Buy) Or BaseAmountIn(Sell)
	TokenAmount2 uint64 // MaxQuoteAmountIn(Buy) Or MinQuoteAmountOut(Sell)
	// Accounts:
	Pool                             ag_solanago.PublicKey
	User                             ag_solanago.PublicKey
	BaseMint                         ag_solanago.PublicKey
	QuoteMint                        ag_solanago.PublicKey
	UserBaseTokenAccount             ag_solanago.PublicKey
	UserQuoteTokenAccount            ag_solanago.PublicKey
	PoolBaseTokenAccount             ag_solanago.PublicKey
	PoolQuoteTokenAccount            ag_solanago.PublicKey
	ProtocolFeeRecipient             ag_solanago.PublicKey
	ProtocolFeeRecipientTokenAccount ag_solanago.PublicKey
	BaseTokenProgram                 ag_solanago.PublicKey
	QuoteTokenProgram                ag_solanago.PublicKey
	// Creator vault accounts for PumpFun
	CoinCreatorVaultAta       ag_solanago.PublicKey
	CoinCreatorVaultAuthority ag_solanago.PublicKey
}

// Fixed extra accounts required by updated PumpFun AMM buy instruction (#20-#23)
var (
	// #20 - Global Volume Accumulator (Writable)
	PumpAmmGlobalVolumeAccumulatorAddress = ag_solanago.MustPublicKeyFromBase58("C2aFPdENg4A2HQsmrd5rTw5TaYBX5Ku887cWjbFKtZpw")
	// #21 - User Volume Accumulator (Writable)
	PumpAmmUserVolumeAccumulatorAddress = ag_solanago.MustPublicKeyFromBase58("4FFtaeaneRAvMiTT2TZYCbKxM4F6aXxASEM4pv2VR3CX")
	// #22 - Fee Config (Read-only)
	PumpAmmFeeConfigAddress = ag_solanago.MustPublicKeyFromBase58("5PHirr8joyTMp9JMm6nW7hNDVyEYdkzDqazxPD7RaTjx")
	// #23 - Fee Program (Read-only)
	PumpAmmFeeProgramAddress = ag_solanago.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")
)

// NewSwapInstruction
func NewSwapInstruction(para *SwapParam) (ag_solanago.Instruction, error) {
	switch para.Direction {
	case BuyDirection:
		if err := ValidateProtocolFeeRecipient(para.ProtocolFeeRecipient); err != nil {
			return nil, err
		}
		buy := amm.NewBuyInstruction(
			para.TokenAmount1, // BaseAmountOut
			para.TokenAmount2, // MaxQuoteAmountIn
			para.Pool,
			para.User,
			PumpAmmGlobalConfigAddress,
			para.BaseMint,
			para.QuoteMint,
			para.UserBaseTokenAccount,
			para.UserQuoteTokenAccount,
			para.PoolBaseTokenAccount,
			para.PoolQuoteTokenAccount,
			para.ProtocolFeeRecipient,
			para.ProtocolFeeRecipientTokenAccount,
			para.BaseTokenProgram,
			para.QuoteTokenProgram,
			ag_solanago.SystemProgramID,
			ag_solanago.SPLAssociatedTokenAccountProgramID,
			PumpAmmEventAuthorityAddress,
			amm.ProgramID,
		)

		// Manually add the creator vault accounts (accounts #18 and #19)
		if !para.CoinCreatorVaultAta.IsZero() && !para.CoinCreatorVaultAuthority.IsZero() {
			// Add CoinCreatorVaultAta as account #18 (writable)
			buy.AccountMetaSlice = append(buy.AccountMetaSlice, &ag_solanago.AccountMeta{
				PublicKey:  para.CoinCreatorVaultAta,
				IsWritable: true,
				IsSigner:   false,
			})
			// Add CoinCreatorVaultAuthority as account #19
			buy.AccountMetaSlice = append(buy.AccountMetaSlice, &ag_solanago.AccountMeta{
				PublicKey:  para.CoinCreatorVaultAuthority,
				IsWritable: false,
				IsSigner:   false,
			})
		}

		// Append required extra accounts (#20-#23) for updated IDL
		buy.AccountMetaSlice = append(buy.AccountMetaSlice, &ag_solanago.AccountMeta{
			PublicKey:  PumpAmmGlobalVolumeAccumulatorAddress,
			IsWritable: true,
			IsSigner:   false,
		})
		buy.AccountMetaSlice = append(buy.AccountMetaSlice, &ag_solanago.AccountMeta{
			PublicKey:  PumpAmmUserVolumeAccumulatorAddress,
			IsWritable: true,
			IsSigner:   false,
		})
		buy.AccountMetaSlice = append(buy.AccountMetaSlice, ag_solanago.Meta(PumpAmmFeeConfigAddress))
		buy.AccountMetaSlice = append(buy.AccountMetaSlice, ag_solanago.Meta(PumpAmmFeeProgramAddress))

		return buy.ValidateAndBuild()
	case SellDirection:
		if err := ValidateProtocolFeeRecipient(para.ProtocolFeeRecipient); err != nil {
			return nil, err
		}
		sell := amm.NewSellInstruction(
			para.TokenAmount1, // BaseAmountIn
			para.TokenAmount2, // MinQuoteAmountOut
			para.Pool,
			para.User,
			PumpAmmGlobalConfigAddress,
			para.BaseMint,
			para.QuoteMint,
			para.UserBaseTokenAccount,
			para.UserQuoteTokenAccount,
			para.PoolBaseTokenAccount,
			para.PoolQuoteTokenAccount,
			para.ProtocolFeeRecipient,
			para.ProtocolFeeRecipientTokenAccount,
			para.BaseTokenProgram,
			para.QuoteTokenProgram,
			ag_solanago.SystemProgramID,
			ag_solanago.SPLAssociatedTokenAccountProgramID,
			PumpAmmEventAuthorityAddress,
			amm.ProgramID,
		)

		// Manually add the creator vault accounts (accounts #18 and #19)
		if !para.CoinCreatorVaultAta.IsZero() && !para.CoinCreatorVaultAuthority.IsZero() {
			// Add CoinCreatorVaultAta as account #18 (writable)
			sell.AccountMetaSlice = append(sell.AccountMetaSlice, &ag_solanago.AccountMeta{
				PublicKey:  para.CoinCreatorVaultAta,
				IsWritable: true,
				IsSigner:   false,
			})
			// Add CoinCreatorVaultAuthority as account #19
			sell.AccountMetaSlice = append(sell.AccountMetaSlice, &ag_solanago.AccountMeta{
				PublicKey:  para.CoinCreatorVaultAuthority,
				IsWritable: false,
				IsSigner:   false,
			})
		}

		return sell.ValidateAndBuild()
	default:
		return nil, fmt.Errorf("unknown swap direction: %d", para.Direction)
	}
}

func ValidateProtocolFeeRecipient(recipient ag_solanago.PublicKey) error {
	if !recipient.Equals(ProtocolFeeRecipients[0]) &&
		!recipient.Equals(ProtocolFeeRecipients[1]) &&
		!recipient.Equals(ProtocolFeeRecipients[2]) &&
		!recipient.Equals(ProtocolFeeRecipients[3]) &&
		!recipient.Equals(ProtocolFeeRecipients[4]) &&
		!recipient.Equals(ProtocolFeeRecipients[5]) &&
		!recipient.Equals(ProtocolFeeRecipients[6]) &&
		!recipient.Equals(ProtocolFeeRecipients[7]) {
		return fmt.Errorf("invalid protocol fee recipient: %s", recipient.String())
	}
	return nil
}

func BuildBuyInstruction(
	baseAmountOut uint64,
	maxQuoteAmountIn uint64,
	user ag_solanago.PublicKey,
	pool ag_solanago.PublicKey,
	baseMint ag_solanago.PublicKey,
	quoteMint ag_solanago.PublicKey,
	userBaseTokenAccount ag_solanago.PublicKey,
	userQuoteTokenAccount ag_solanago.PublicKey,
	poolBaseTokenAccount ag_solanago.PublicKey,
	poolQuoteTokenAccount ag_solanago.PublicKey,
	protocolFeeRecipient ag_solanago.PublicKey,
	protocolFeeRecipientTokenAccount ag_solanago.PublicKey,
) (ag_solanago.Instruction, error) {
	swapParam := &SwapParam{
		Direction:                        BuyDirection,
		TokenAmount1:                     baseAmountOut,
		TokenAmount2:                     maxQuoteAmountIn,
		Pool:                             pool,
		User:                             user,
		BaseMint:                         baseMint,
		QuoteMint:                        quoteMint,
		UserBaseTokenAccount:             userBaseTokenAccount,
		UserQuoteTokenAccount:            userQuoteTokenAccount,
		PoolBaseTokenAccount:             poolBaseTokenAccount,
		PoolQuoteTokenAccount:            poolQuoteTokenAccount,
		ProtocolFeeRecipient:             protocolFeeRecipient,
		ProtocolFeeRecipientTokenAccount: protocolFeeRecipientTokenAccount,
		BaseTokenProgram:                 ag_solanago.MustPublicKeyFromBase58(constants.ProgramStrToken),
		QuoteTokenProgram:                ag_solanago.MustPublicKeyFromBase58(constants.ProgramStrToken),
	}

	return NewSwapInstruction(swapParam)
}

func BuildSellInstruction(
	baseAmountIn uint64,
	minQuoteAmountOut uint64,
	user ag_solanago.PublicKey,
	pool ag_solanago.PublicKey,
	baseMint ag_solanago.PublicKey,
	quoteMint ag_solanago.PublicKey,
	userBaseTokenAccount ag_solanago.PublicKey,
	userQuoteTokenAccount ag_solanago.PublicKey,
	poolBaseTokenAccount ag_solanago.PublicKey,
	poolQuoteTokenAccount ag_solanago.PublicKey,
	protocolFeeRecipient ag_solanago.PublicKey,
	protocolFeeRecipientTokenAccount ag_solanago.PublicKey,
) (ag_solanago.Instruction, error) {
	swapParam := &SwapParam{
		Direction:                        SellDirection,
		TokenAmount1:                     baseAmountIn,
		TokenAmount2:                     minQuoteAmountOut,
		Pool:                             pool,
		User:                             user,
		BaseMint:                         baseMint,
		QuoteMint:                        quoteMint,
		UserBaseTokenAccount:             userBaseTokenAccount,
		UserQuoteTokenAccount:            userQuoteTokenAccount,
		PoolBaseTokenAccount:             poolBaseTokenAccount,
		PoolQuoteTokenAccount:            poolQuoteTokenAccount,
		ProtocolFeeRecipient:             protocolFeeRecipient,
		ProtocolFeeRecipientTokenAccount: protocolFeeRecipientTokenAccount,
		BaseTokenProgram:                 ag_solanago.MustPublicKeyFromBase58(constants.ProgramStrToken),
		QuoteTokenProgram:                ag_solanago.MustPublicKeyFromBase58(constants.ProgramStrToken),
	}
	return NewSwapInstruction(swapParam)
}
