package block

import (
	"context"
	"dex/consumer/internal/svc"
	"dex/model/solmodel"
	"dex/pkg/sol"
	"dex/pkg/types"
	"encoding/base64"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"dex/pkg/pumpfun/amm/idl/generated/amm"

	"dex/pkg/constants"

	"dex/pkg/util"

	"dex/consumer/internal/logic/pump"

	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/duke-git/lancet/v2/slice"
	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/shopspring/decimal"
)

var PUMPAMMMap = cmap.New[int]()

// InitPumpTokenAmount 1B = 10^9 = 1,000,000,000 : 1B * 80%
// const InitPumpTokenAmount = 1073000000
const InitPumpTokenAmount = 873000000
const InitSolTokenAmount = 0.015
const VirtualInitPumpTokenAmount = 1073000191

// TokenReservesDiff https://solscan.io/account/3wUTJYgQxiQ1qEDBirQDjqrVeVqBfiZ91V3SDscU28NW#anchorData virtualTokenReserves - realTokenReserves
const TokenReservesDiff = 279900000000000

// SolReservesDiff https://solscan.io/account/3wUTJYgQxiQ1qEDBirQDjqrVeVqBfiZ91V3SDscU28NW#anchorData virtualSolReserves - realSolReserves
const SolReservesDiff = 30000000000

const eventLogPrefix = "Program data: "

var eventTypes = map[[8]byte]reflect.Type{
	amm.BuyEventEventDataDiscriminator:        reflect.TypeOf(amm.BuyEventEventData{}),
	amm.CreatePoolEventEventDataDiscriminator: reflect.TypeOf(amm.CreatePoolEventEventData{}),
	amm.SellEventEventDataDiscriminator:       reflect.TypeOf(amm.SellEventEventData{}),
}

var eventNames = map[[8]byte]string{
	amm.BuyEventEventDataDiscriminator:        "BuyEvent",
	amm.CreatePoolEventEventDataDiscriminator: "CreatePoolEvent",
	amm.SellEventEventDataDiscriminator:       "SellEvent",
}

type PumpAmmDecoder struct {
	ctx                 context.Context
	svcCtx              *svc.ServiceContext
	dtx                 *DecodedTx
	compiledInstruction *solTypes.CompiledInstruction
}

func (decoder *PumpAmmDecoder) DecodePumpAmmInstruction() (*types.TradeWithPair, error) {
	accountMetas := slice.Map[int, *ag_solanago.AccountMeta](decoder.compiledInstruction.Accounts, func(_ int, index int) *ag_solanago.AccountMeta {
		return &ag_solanago.AccountMeta{
			PublicKey: ag_solanago.PublicKeyFromBytes(decoder.dtx.Tx.AccountKeys[index].Bytes()),
			// no need
			IsWritable: false,
			IsSigner:   false,
		}
	})
	decodeInstruction, err := decoder.decodeInstruction(accountMetas, decoder.compiledInstruction.Data)
	if err != nil {
		return nil, err
	}
	switch decodeInstruction.TypeID {
	case amm.Instruction_Buy:
		fmt.Println("*******instruction buy")
		buyImpl := decodeInstruction.Impl.(*amm.Buy)
		trade, _ := decoder.decodeBuyInstruction(buyImpl, decoder.dtx.Tx.Meta.LogMessages)
		if trade != nil {
			fmt.Println("decoder.decodeBuyInstruction successful pair name:", trade.PairInfo.Name)
		}
		return trade, nil
	case amm.Instruction_Sell:
		fmt.Println("*******instruction sell")
		sellImpl := decodeInstruction.Impl.(*amm.Sell)
		return decoder.decodeSellInstruction(sellImpl, decoder.dtx.Tx.Meta.LogMessages)
	case amm.Instruction_CreatePool:
		fmt.Println("*******instruction create pool")
		createPoolImpl := decodeInstruction.Impl.(*amm.CreatePool)
		return decoder.decodeCreatePoolInstruction(createPoolImpl, decoder.dtx.Tx.Meta.LogMessages)
	default:
		fmt.Println("*******instruction default amm")
		return nil, ErrNotSupportInstruction
	}
}

func (decoder *PumpAmmDecoder) decodeInstruction(accounts []*ag_solanago.AccountMeta, data []byte) (*amm.Instruction, error) {
	inst := new(amm.Instruction)
	if err := ag_binary.NewBorshDecoder(data).Decode(inst); err != nil {
		return nil, fmt.Errorf("unable to decode instruction: %w", err)
	}
	if v, ok := inst.Impl.(ag_solanago.AccountsSettable); ok {
		err := v.SetAccounts(accounts)
		if err != nil {
			return nil, fmt.Errorf("unable to set accounts for instruction: %w", err)
		}
	}
	return inst, nil
}

func (decoder *PumpAmmDecoder) decodeBuyInstruction(
	buyImpl *amm.Buy,
	logMessages []string,
) (*types.TradeWithPair, error) {
	key := decoder.dtx.TxHash + "_" + amm.InstructionIDToName(amm.Instruction_Buy) + buyImpl.GetPoolAccount().PublicKey.String()
	value, ok := PUMPAMMMap.Get(key)
	count := 0
	if ok && value > 0 && value < len(logMessages) {
		count = value
	}
	events, err := decoder.parsePumpAmmEvents(logMessages, amm.BuyEventEventDataDiscriminator)
	if err != nil {
		return nil, err
	}
	var buyEvent *amm.BuyEventEventData
	for _, event := range events {
		buyEvent = event.Data.(*amm.BuyEventEventData)
		if buyEvent.Pool.String() == buyImpl.GetPoolAccount().PublicKey.String() {
			if count > 0 {
				count--
				continue
			}
			value, ok := PUMPAMMMap.Get(key)
			if !ok {
				PUMPAMMMap.Set(key, 1)
			} else {
				PUMPAMMMap.Set(key, value+1)
			}
			break
		}
	}

	if buyImpl.GetQuoteMintAccount().PublicKey.String() != constants.TokenStrWrapSol {
		return nil, fmt.Errorf("not support token,tx hash: %v", decoder.dtx.TxHash)
	}

	baseTokenAccount := buyImpl.GetBaseMintAccount()
	baseTokenAccountInfo := decoder.dtx.TokenAccountMap[baseTokenAccount.PublicKey.String()]
	if baseTokenAccountInfo == nil {
		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, baseTokenAccount.PublicKey.String())

		if mintInfo != nil {
			baseTokenAccountInfo = &TokenAccount{
				TokenAddress: baseTokenAccount.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}
		}

		if baseTokenAccountInfo == nil {
			err := fmt.Errorf("baseTokenAccountInfo not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
	}
	quoteTokenAccount := buyImpl.GetQuoteMintAccount()
	quoteTokenAccountInfo := decoder.dtx.TokenAccountMap[quoteTokenAccount.PublicKey.String()]
	if quoteTokenAccountInfo == nil {
		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, quoteTokenAccount.PublicKey.String())

		if mintInfo != nil {
			quoteTokenAccountInfo = &TokenAccount{
				TokenAddress: quoteTokenAccount.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}
		}

		if quoteTokenAccountInfo == nil {
			err := fmt.Errorf("quoteTokenAccountInfo not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
	}

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = buyImpl.GetPoolAccount().PublicKey.String()
	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             buyImpl.GetPoolAccount().PublicKey.String(),
		BaseTokenAddr:    quoteTokenAccountInfo.TokenAddress,
		BaseTokenDecimal: quoteTokenAccountInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        baseTokenAccountInfo.TokenAddress,
		TokenDecimal:     baseTokenAccountInfo.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}
	trade.Maker = buyImpl.GetUserAccount().PublicKey.String()
	trade.Type = types.TradeTypeBuy
	trade.BaseTokenAmount = decimal.NewFromUint64(buyEvent.QuoteAmountInWithLpFee).Div(decimal.NewFromFloat(math.Pow10(int(quoteTokenAccountInfo.TokenDecimal)))).InexactFloat64()
	trade.TokenAmount = decimal.NewFromUint64(buyEvent.BaseAmountOut).Div(decimal.NewFromFloat(math.Pow10(int(baseTokenAccountInfo.TokenDecimal)))).InexactFloat64()
	trade.To = buyEvent.UserQuoteTokenAccount.String()
	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.PumpSwap
	trade.TokenAmount1 = buyEvent.BaseAmountOut
	trade.TokenAmount2 = buyEvent.MaxQuoteAmountIn
	trade.PoolBaseTokenReserves = buyEvent.PoolBaseTokenReserves
	trade.PoolQuoteTokenReserves = buyEvent.PoolQuoteTokenReserves

	trade.CurrentBaseTokenInPoolAmount = float64(buyEvent.PoolQuoteTokenReserves)
	trade.CurrentTokenInPoolAmount = float64(buyEvent.PoolBaseTokenReserves)
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	// trade.BaseTokenAccountAddress = tokenSwap.BaseTokenInfo.TokenAccountAddress
	// trade.TokenAccountAddress = tokenSwap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = int64(buyEvent.QuoteAmountInWithLpFee)
	trade.TokenAmountInt = int64(buyEvent.BaseAmountOut)

	trade.BaseTokenPriceUSD = decoder.dtx.SolPrice
	trade.TotalUSD = decimal.NewFromFloat(trade.BaseTokenAmount).Mul(decimal.NewFromFloat(decoder.dtx.SolPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err = fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return nil, err
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(trade.TokenAmount)).InexactFloat64() // price

	now := time.Now()
	trade.PumpAmmInfo = &solmodel.PumpAmmInfo{
		PoolAccount:                      buyImpl.GetPoolAccount().PublicKey.String(),
		GlobalConfigAccount:              buyImpl.GetGlobalConfigAccount().PublicKey.String(),
		BaseMintAccount:                  buyImpl.GetBaseMintAccount().PublicKey.String(),
		QuoteMintAccount:                 buyImpl.GetQuoteMintAccount().PublicKey.String(),
		PoolBaseTokenAccount:             buyImpl.GetPoolBaseTokenAccountAccount().PublicKey.String(),
		PoolQuoteTokenAccount:            buyImpl.GetPoolQuoteTokenAccountAccount().PublicKey.String(),
		ProtocolFeeRecipientAccount:      buyImpl.GetProtocolFeeRecipientAccount().PublicKey.String(),
		ProtocolFeeRecipientTokenAccount: buyImpl.GetProtocolFeeRecipientTokenAccountAccount().PublicKey.String(),
		BaseTokenProgram:                 buyImpl.GetBaseTokenProgramAccount().PublicKey.String(),
		QuoteTokenProgram:                buyImpl.GetQuoteTokenProgramAccount().PublicKey.String(),
		EventAuthorityAccount:            buyImpl.GetEventAuthorityAccount().PublicKey.String(),
		CreatedAt:                        now,
		UpdatedAt:                        now,
	}

	pumpPoint := decimal.NewFromInt(1).Sub(decimal.NewFromFloat(trade.CurrentTokenInPoolAmount).Div(decimal.NewFromInt(int64(InitPumpTokenAmount)))).InexactFloat64()
	pumpPoint = min(max(pumpPoint, 0), 1)

	if trade.CurrentTokenInPoolAmount <= 0 {
		pumpPoint = 1
	}

	trade.PairInfo.InitTokenAmount = VirtualInitPumpTokenAmount
	trade.PairInfo.InitBaseTokenAmount = InitSolTokenAmount

	trade.PumpPoint = pumpPoint

	trade.PumpMarketCap = decimal.NewFromFloat(trade.TokenPriceUSD).Mul(decimal.NewFromFloat(trade.PairInfo.TokenTotalSupply)).InexactFloat64()
	trade.PumpPairAddr = trade.PairAddr
	trade.PumpStatus = pump.PumpStatusTrading
	if trade.PumpPoint >= 0.999 {
		trade.PumpStatus = pump.PumpStatusMigrating
		trade.PumpPoint = 1
	}

	return trade, nil
}

func (decoder *PumpAmmDecoder) decodeSellInstruction(
	sellImpl *amm.Sell,
	logMessages []string,
) (*types.TradeWithPair, error) {
	key := decoder.dtx.TxHash + "_" + amm.InstructionIDToName(amm.Instruction_Sell) + sellImpl.GetPoolAccount().PublicKey.String()
	value, ok := PUMPAMMMap.Get(key)
	count := 0
	if ok && value > 0 && value < len(logMessages) {
		count = value
	}
	events, err := decoder.parsePumpAmmEvents(logMessages, amm.SellEventEventDataDiscriminator)
	if err != nil {
		return nil, err
	}
	var sellEvent *amm.SellEventEventData
	for _, event := range events {
		sellEvent = event.Data.(*amm.SellEventEventData)
		if sellEvent.Pool.String() == sellImpl.GetPoolAccount().PublicKey.String() {
			if count > 0 {
				count--
				continue
			}
			value, ok := PUMPAMMMap.Get(key)
			if !ok {
				PUMPAMMMap.Set(key, 1)
			} else {
				PUMPAMMMap.Set(key, value+1)
			}
			break
		}
	}

	if sellImpl.GetQuoteMintAccount().PublicKey.String() != constants.TokenStrWrapSol {
		return nil, fmt.Errorf("not support token,tx hash: %v", decoder.dtx.TxHash)
	}

	baseTokenAccount := sellImpl.GetBaseMintAccount()
	baseTokenAccountInfo := decoder.dtx.TokenAccountMap[baseTokenAccount.PublicKey.String()]
	if baseTokenAccountInfo == nil {
		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, baseTokenAccount.PublicKey.String())

		if mintInfo != nil {
			baseTokenAccountInfo = &TokenAccount{
				TokenAddress: baseTokenAccount.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}
		}

		if baseTokenAccountInfo == nil {
			err := fmt.Errorf("baseTokenAccountInfo not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
	}
	quoteTokenAccount := sellImpl.GetQuoteMintAccount()
	quoteTokenAccountInfo := decoder.dtx.TokenAccountMap[quoteTokenAccount.PublicKey.String()]
	if quoteTokenAccountInfo == nil {
		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, quoteTokenAccount.PublicKey.String())

		if mintInfo != nil {
			quoteTokenAccountInfo = &TokenAccount{
				TokenAddress: quoteTokenAccount.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}
		}

		if quoteTokenAccountInfo == nil {
			err := fmt.Errorf("quoteTokenAccountInfo not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
	}
	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = sellImpl.GetPoolAccount().PublicKey.String()
	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             sellImpl.GetPoolAccount().PublicKey.String(),
		BaseTokenAddr:    quoteTokenAccountInfo.TokenAddress,
		BaseTokenDecimal: quoteTokenAccountInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        baseTokenAccountInfo.TokenAddress,
		TokenDecimal:     baseTokenAccountInfo.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}
	trade.Maker = sellImpl.GetUserAccount().PublicKey.String()
	trade.Type = types.TradeTypeSell
	trade.BaseTokenAmount = decimal.NewFromUint64(sellEvent.QuoteAmountOutWithoutLpFee).Div(decimal.NewFromFloat(math.Pow10(int(quoteTokenAccountInfo.TokenDecimal)))).InexactFloat64()
	trade.TokenAmount = decimal.NewFromUint64(sellEvent.BaseAmountIn).Div(decimal.NewFromFloat(math.Pow10(int(baseTokenAccountInfo.TokenDecimal)))).InexactFloat64()
	trade.To = sellEvent.UserBaseTokenAccount.String()
	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.PumpSwap
	trade.TokenAmount1 = sellEvent.BaseAmountIn
	trade.TokenAmount2 = sellEvent.MinQuoteAmountOut
	trade.PoolBaseTokenReserves = sellEvent.PoolBaseTokenReserves
	trade.PoolQuoteTokenReserves = sellEvent.PoolQuoteTokenReserves

	trade.CurrentBaseTokenInPoolAmount = float64(sellEvent.PoolQuoteTokenReserves)
	trade.CurrentTokenInPoolAmount = float64(sellEvent.PoolBaseTokenReserves)
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	// trade.BaseTokenAccountAddress = tokenSwap.BaseTokenInfo.TokenAccountAddress
	// trade.TokenAccountAddress = tokenSwap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = int64(sellEvent.QuoteAmountOutWithoutLpFee)
	trade.TokenAmountInt = int64(sellEvent.BaseAmountIn)

	trade.BaseTokenPriceUSD = decoder.dtx.SolPrice
	trade.TotalUSD = decimal.NewFromFloat(trade.BaseTokenAmount).Mul(decimal.NewFromFloat(decoder.dtx.SolPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err = fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return nil, err
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(trade.TokenAmount)).InexactFloat64() // pricex

	now := time.Now()

	trade.PumpAmmInfo = &solmodel.PumpAmmInfo{
		PoolAccount:                      sellImpl.GetPoolAccount().PublicKey.String(),
		GlobalConfigAccount:              sellImpl.GetGlobalConfigAccount().PublicKey.String(),
		BaseMintAccount:                  sellImpl.GetBaseMintAccount().PublicKey.String(),
		QuoteMintAccount:                 sellImpl.GetQuoteMintAccount().PublicKey.String(),
		PoolBaseTokenAccount:             sellImpl.GetPoolBaseTokenAccountAccount().PublicKey.String(),
		PoolQuoteTokenAccount:            sellImpl.GetPoolQuoteTokenAccountAccount().PublicKey.String(),
		ProtocolFeeRecipientAccount:      sellImpl.GetProtocolFeeRecipientAccount().PublicKey.String(),
		ProtocolFeeRecipientTokenAccount: sellImpl.GetProtocolFeeRecipientTokenAccountAccount().PublicKey.String(),
		BaseTokenProgram:                 sellImpl.GetBaseTokenProgramAccount().PublicKey.String(),
		QuoteTokenProgram:                sellImpl.GetQuoteTokenProgramAccount().PublicKey.String(),
		EventAuthorityAccount:            sellImpl.GetEventAuthorityAccount().PublicKey.String(),
		CreatedAt:                        now,
		UpdatedAt:                        now,
	}

	pumpPoint := decimal.NewFromInt(1).Sub(decimal.NewFromFloat(trade.CurrentTokenInPoolAmount).Div(decimal.NewFromInt(int64(InitPumpTokenAmount)))).InexactFloat64()
	pumpPoint = min(max(pumpPoint, 0), 1)

	trade.PairInfo.InitTokenAmount = VirtualInitPumpTokenAmount
	trade.PairInfo.InitBaseTokenAmount = InitSolTokenAmount

	trade.PumpPoint = pumpPoint

	trade.PumpMarketCap = decimal.NewFromFloat(trade.TokenPriceUSD).Mul(decimal.NewFromFloat(trade.PairInfo.TokenTotalSupply)).InexactFloat64()
	trade.PumpPairAddr = trade.PairAddr
	trade.PumpStatus = pump.PumpStatusTrading
	if trade.PumpPoint >= 0.999 {
		trade.PumpStatus = pump.PumpStatusMigrating
		trade.PumpPoint = 1
	}

	return trade, nil
}

// https://solscan.io/tx/TarLwEKyKMsiozUB4Te4MST9fEhLPx6HPifSd54vPJYuNfJ2Cgs7Qw7DFeChEreYvREeEsgEnfFmVna22YnR1Ff
func (decoder *PumpAmmDecoder) decodeCreatePoolInstruction(
	createPoolImpl *amm.CreatePool,
	logMessages []string,
) (*types.TradeWithPair, error) {
	key := decoder.dtx.TxHash + "_" + amm.InstructionIDToName(amm.Instruction_CreatePool) + createPoolImpl.GetPoolAccount().PublicKey.String()
	value, ok := PUMPAMMMap.Get(key)
	count := 0
	if ok && value > 0 && value < len(logMessages) {
		count = value
	}
	events, err := decoder.parsePumpAmmEvents(logMessages, amm.CreatePoolEventEventDataDiscriminator)
	if err != nil {
		return nil, err
	}
	var createPoolEvent *amm.CreatePoolEventEventData
	for _, event := range events {
		createPoolEvent = event.Data.(*amm.CreatePoolEventEventData)
		if createPoolEvent.Pool.String() == createPoolImpl.GetPoolAccount().PublicKey.String() {
			if count > 0 {
				count--
				continue
			}
			value, ok := PUMPAMMMap.Get(key)
			if !ok {
				PUMPAMMMap.Set(key, 1)
			} else {
				PUMPAMMMap.Set(key, value+1)
			}
			break
		}
	}
	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = createPoolImpl.GetPoolAccount().PublicKey.String()
	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             createPoolImpl.GetPoolAccount().PublicKey.String(),
		BaseTokenAddr:    createPoolEvent.BaseMint.String(),
		BaseTokenDecimal: createPoolEvent.BaseMintDecimals,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        createPoolEvent.QuoteMint.String(),
		TokenDecimal:     createPoolEvent.QuoteMintDecimals,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}
	trade.Maker = createPoolImpl.GetCreatorAccount().PublicKey.String()
	trade.Type = types.TradePumpAmmCreatePool
	trade.LpMintAddress = createPoolEvent.LpMint.String()
	trade.PoolBaseTokenReserves = createPoolEvent.PoolBaseAmount
	trade.PoolQuoteTokenReserves = createPoolEvent.PoolQuoteAmount
	return trade, nil
}

func (decoder *PumpAmmDecoder) parsePumpAmmEvents(logMessages []string, filterType [8]byte) (evts []*amm.Event, err error) {
	eventBinaries := make([][]byte, 0)
	for _, log := range logMessages {
		if strings.HasPrefix(log, eventLogPrefix) {
			eventBase64 := log[len(eventLogPrefix):]

			var eventBinary []byte
			if eventBinary, err = base64.StdEncoding.DecodeString(eventBase64); err != nil {
				err = fmt.Errorf("failed to decode logMessage event: %s", eventBase64)
				return
			}
			eventBinaries = append(eventBinaries, eventBinary)
		}
	}
	ag_decoder := ag_binary.NewDecoderWithEncoding(nil, ag_binary.EncodingBorsh)
	for _, eventBinary := range eventBinaries {
		eventDiscriminator := ag_binary.TypeID(eventBinary[:8])
		if eventType, ok := eventTypes[eventDiscriminator]; ok {
			if filterType != [8]byte{} && eventDiscriminator != filterType {
				continue
			}
			eventData := reflect.New(eventType).Interface().(amm.EventData)
			ag_decoder.Reset(eventBinary)
			if err = eventData.UnmarshalWithDecoder(ag_decoder); err != nil {
				err = fmt.Errorf("failed to unmarshal event %s: %w", eventType.String(), err)
				return
			}
			evts = append(evts, &amm.Event{
				Name: eventNames[eventDiscriminator],
				Data: eventData,
			})
		}
	}
	return
}
