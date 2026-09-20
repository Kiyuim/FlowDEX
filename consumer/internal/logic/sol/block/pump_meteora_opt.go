package block

// Decoder for the OPTIMIZED (security-hardened) pump-meteora program
// (ProgramStrPumpMeteoraOpt, 241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ) — a third
// token source, tagged SwapName = constants.PumpMeteoraV2.
//
// Same instruction discriminators + account layouts as the original pump-meteora, but
// the emitted events carry EXTRA fields (the hardened build emits real reserves as their
// own fields + a timestamp, fixing the audit's BIG-03), so the borsh structs differ.

import (
	"context"
	"fmt"

	"dex/consumer/internal/logic/pump"
	"dex/consumer/internal/svc"
	"dex/pkg/constants"
	"dex/pkg/types"
	"dex/pkg/util"

	"github.com/blocto/solana-go-sdk/common"
	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

// SwapEvent (optimized): adds real_sol_reserves, real_token_reserves and a timestamp.
type MeteoraOptSwapEvent struct {
	Disc                 uint64
	User                 common.PublicKey
	Mint                 common.PublicKey
	BondingCurve         common.PublicKey
	AmountIn             uint64
	Direction            uint8
	MinimumReceiveAmount uint64
	AmountOut            uint64
	VirtualSolReserves   uint64
	VirtualTokenReserves uint64
	RealSolReserves      uint64
	RealTokenReserves    uint64
	Timestamp            int64
}

// LaunchEvent (optimized): adds a timestamp.
type MeteoraOptLaunchEvent struct {
	Disc           uint64
	Creator        common.PublicKey
	Mint           common.PublicKey
	BondingCurve   common.PublicKey
	Metadata       common.PublicKey
	Decimals       uint8
	TokenSupply    uint64
	ReserveLamport uint64
	ReserveToken   uint64
	Timestamp      int64
}

// DecodePumpMeteoraOptInstruction routes an optimized pump-meteora instruction.
func DecodePumpMeteoraOptInstruction(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	switch GetPumpInstruction(instruction.Data) {
	case MeteoraInstructionSwap:
		return decodeMeteoraOptSwap(dtx, instruction, logIndex)
	case MeteoraInstructionCreate:
		return decodeMeteoraOptCreate(dtx, instruction, logIndex)
	default:
		return nil, ErrUnknownProgram
	}
}

func decodeMeteoraOptSwap(dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	if len(instruction.Accounts) < 13 {
		return nil, fmt.Errorf("meteora-opt swap accounts len:%d hash:%v", len(instruction.Accounts), dtx.TxHash)
	}
	var ev MeteoraOptSwapEvent
	ok, derr := decodeMeteoraEvent(dtx.Tx.Meta.LogMessages, MeteoraEventSwap, &ev)
	if derr != nil {
		return nil, derr
	}
	if !ok {
		return nil, fmt.Errorf("meteora-opt SwapEvent not found hash:%v", dtx.TxHash)
	}

	accountKeys := dtx.Tx.AccountKeys
	pair := ev.BondingCurve.String()
	mint := ev.Mint.String()
	maker := ev.User.String()
	decimals := meteoraTokenDecimal(dtx, accountKeys, instruction, 8, mint)
	tokenAccount := ""
	if 8 < len(instruction.Accounts) {
		tokenAccount = accountKeys[instruction.Accounts[8]].String()
	}

	isBuy := ev.Direction == 0
	var solAmt, tokAmt uint64
	if isBuy {
		solAmt, tokAmt = ev.AmountIn, ev.AmountOut
	} else {
		solAmt, tokAmt = ev.AmountOut, ev.AmountIn
	}

	blockDb := dtx.BlockDb
	solPrice := dtx.SolPrice

	trade = &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = dtx.TxHash
	trade.PairAddr = pair
	trade.Maker = maker
	trade.To = maker

	// optimized build emits real AND virtual reserves as distinct fields — use them directly.
	currentToken := decimal.New(int64(ev.RealTokenReserves), -int32(decimals)).InexactFloat64()
	currentSol := decimal.New(int64(ev.RealSolReserves), -constants.SolDecimal).InexactFloat64()
	trade.CurrentBaseTokenInPoolAmount = currentSol
	trade.CurrentTokenInPoolAmount = currentToken
	trade.PumpVirtualBaseTokenReserves = decimal.New(int64(ev.VirtualSolReserves), -constants.SolDecimal).InexactFloat64()
	trade.PumpVirtualTokenReserves = decimal.New(int64(ev.VirtualTokenReserves), -int32(decimals)).InexactFloat64()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             pair,
		BaseTokenAddr:    util.GetBaseToken(SolChainIdInt).Address,
		BaseTokenDecimal: uint8(util.GetBaseToken(SolChainIdInt).Decimal),
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        mint,
		TokenDecimal:     decimals,
		BlockTime:        blockDb.BlockTime.Unix(),
		BlockNum:         blockDb.Slot,
		Name:             constants.PumpMeteoraV2,
		TokenTotalSupply: MeteoraTotalSupplyWhole,
	}

	if isBuy {
		trade.Type = types.TradeTypeBuy
	} else {
		trade.Type = types.TradeTypeSell
	}
	trade.BaseTokenAmount = decimal.New(int64(solAmt), -constants.SolDecimal).InexactFloat64()
	trade.TokenAmount = decimal.New(int64(tokAmt), -int32(decimals)).InexactFloat64()
	trade.BaseTokenPriceUSD = solPrice
	trade.TotalUSD = decimal.NewFromFloat(trade.BaseTokenAmount).Mul(decimal.NewFromFloat(solPrice)).InexactFloat64()
	if trade.TokenAmount == 0 {
		return nil, ErrTokenAmountIsZero
	}
	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(trade.TokenAmount)).InexactFloat64()
	trade.BlockNum = blockDb.Slot
	trade.BlockTime = blockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)
	trade.TransactionIndex = dtx.TxIndex
	trade.LogIndex = int(logIndex)

	trade.SwapName = constants.PumpMeteoraV2
	trade.BaseTokenAccountAddress = ""
	trade.TokenAccountAddress = tokenAccount
	trade.BaseTokenAmountInt = int64(solAmt)
	trade.TokenAmountInt = int64(tokAmt)
	trade.PumpLaunched = false

	pumpPoint := 1 - (currentToken / MeteoraInitRealTokenWhole)
	if pumpPoint < 0 {
		pumpPoint = 0
	}
	if pumpPoint > 1 || currentToken <= 0 {
		pumpPoint = 1
	}
	trade.PairInfo.InitTokenAmount = MeteoraInitRealTokenWhole
	trade.PairInfo.InitBaseTokenAmount = MeteoraInitVirtualSol
	trade.PumpPoint = pumpPoint

	trade.PumpMarketCap = decimal.NewFromFloat(trade.TokenPriceUSD).Mul(decimal.NewFromFloat(trade.PairInfo.TokenTotalSupply)).InexactFloat64()
	trade.Fdv = trade.PumpMarketCap
	trade.PumpPairAddr = pair
	trade.PumpStatus = pump.PumpStatusTrading
	if trade.PumpPoint >= 0.999 {
		trade.PumpStatus = pump.PumpStatusMigrating
		trade.PumpPoint = 1
	}
	logx.Infof("METEORA-OPT OK swap type=%v token=%v price=%v hash=%v", trade.Type, mint, trade.TokenPriceUSD, dtx.TxHash)
	return trade, nil
}

func decodeMeteoraOptCreate(dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	if len(instruction.Accounts) < 13 {
		return nil, fmt.Errorf("meteora-opt create accounts len:%d hash:%v", len(instruction.Accounts), dtx.TxHash)
	}
	var ev MeteoraOptLaunchEvent
	ok, derr := decodeMeteoraEvent(dtx.Tx.Meta.LogMessages, MeteoraEventLaunch, &ev)
	if derr != nil {
		return nil, derr
	}
	if !ok {
		return nil, fmt.Errorf("meteora-opt LaunchEvent not found hash:%v", dtx.TxHash)
	}

	pair := ev.BondingCurve.String()
	mint := ev.Mint.String()
	maker := ev.Creator.String()
	decimals := ev.Decimals
	if decimals == 0 {
		decimals = 6
	}

	blockDb := dtx.BlockDb
	solPrice := dtx.SolPrice

	trade = &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = dtx.TxHash
	trade.PairAddr = pair
	trade.Maker = maker

	initToken := decimal.New(int64(ev.ReserveToken), -int32(decimals)).InexactFloat64()
	initSol := decimal.New(int64(ev.ReserveLamport), -constants.SolDecimal).InexactFloat64()
	trade.PairInfo = types.Pair{
		ChainId:             SolChainId,
		Addr:                pair,
		BaseTokenAddr:       util.GetBaseToken(SolChainIdInt).Address,
		BaseTokenDecimal:    uint8(util.GetBaseToken(SolChainIdInt).Decimal),
		BaseTokenSymbol:     util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:           mint,
		TokenDecimal:        decimals,
		BlockTime:           blockDb.BlockTime.Unix(),
		BlockNum:            blockDb.Slot,
		Name:                constants.PumpMeteoraV2,
		TokenTotalSupply:    MeteoraTotalSupplyWhole,
		InitTokenAmount:     MeteoraInitRealTokenWhole,
		InitBaseTokenAmount: initSol,
	}

	trade.CurrentTokenInPoolAmount = initToken
	trade.CurrentBaseTokenInPoolAmount = initSol
	trade.PumpVirtualBaseTokenReserves = initSol + MeteoraInitVirtualSol
	trade.PumpVirtualTokenReserves = initToken

	trade.Type = types.TradePumpCreate
	trade.BaseTokenPriceUSD = solPrice
	trade.TotalUSD = 0
	trade.TokenPriceUSD = decimal.NewFromFloat(solPrice).Mul(decimal.NewFromFloat(0.00000000775)).InexactFloat64()
	trade.To = ""
	trade.BlockNum = blockDb.Slot
	trade.BlockTime = blockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)
	trade.TransactionIndex = dtx.TxIndex
	trade.LogIndex = int(logIndex)

	trade.SwapName = constants.PumpMeteoraV2
	trade.BaseTokenAccountAddress = ""
	trade.PumpLaunched = false
	trade.PumpPoint = 0
	trade.PumpPairAddr = pair
	trade.PumpStatus = pump.PumpStatusCreate

	logx.Infof("METEORA-OPT OK create token=%v curve=%v hash=%v", mint, pair, dtx.TxHash)
	return trade, nil
}
