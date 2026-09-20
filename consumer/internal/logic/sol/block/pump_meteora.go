package block

// Decoder for our own pump-meteora bonding-curve program (ProgramStrPumpMeteora,
// AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5) — a second "token source" indexed
// alongside pump.fun. Trades are tagged SwapName = constants.PumpMeteora so the UI
// source-selector can filter by it.
//
// Layouts below were validated against real devnet txs seeded by
// lessons/5.优化/experiments/devnet_seed.js (e.g. a Swap event is exactly 145 bytes:
// 8 disc + 3*32 pubkeys + amountIn + dir + minRecv + amountOut + 2*u64 reserves).

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"dex/consumer/internal/logic/pump"
	"dex/consumer/internal/svc"
	"dex/pkg/constants"
	"dex/pkg/types"
	"dex/pkg/util"

	"github.com/blocto/solana-go-sdk/common"
	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/near/borsh-go"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// instruction discriminators (LE uint64 of anchor sighash "global:<name>")
	MeteoraInstructionSwap   = 0xc88775e1919ec6f8
	MeteoraInstructionCreate = 0x2d085f45329e8b5e
	// event discriminators (LE uint64 of anchor "event:<Name>")
	MeteoraEventSwap   = 0xe2710826e8cdc640
	MeteoraEventLaunch = 0x5eef5c73822fc11b

	// config defaults for the deployed contract (whole-token units, 6 decimals)
	MeteoraInitRealTokenWhole = 793_100_000.0   // initial_real_token_reserves_config / 1e6
	MeteoraTotalSupplyWhole   = 1_000_000_000.0 // token_supply_config / 1e6
	MeteoraInitVirtualSol     = 30.0            // initial_virtual_sol_reserves_config / 1e9
)

// SwapEvent — the deployed contract emits REAL (post-trade) reserves into the
// "virtual" fields (a known quirk; see the audit BIZ-03). We name them accordingly.
type MeteoraSwapEvent struct {
	Disc                 uint64
	User                 common.PublicKey
	Mint                 common.PublicKey
	BondingCurve         common.PublicKey
	AmountIn             uint64
	Direction            uint8
	MinimumReceiveAmount uint64
	AmountOut            uint64
	RealSolReserves      uint64
	RealTokenReserves    uint64
}

type MeteoraLaunchEvent struct {
	Disc           uint64
	Creator        common.PublicKey
	Mint           common.PublicKey
	BondingCurve   common.PublicKey
	Metadata       common.PublicKey
	Decimals       uint8
	TokenSupply    uint64
	ReserveLamport uint64
	ReserveToken   uint64
}

func decodeMeteoraEvent(logs []string, wantDisc uint64, out interface{}) (bool, error) {
	for _, l := range logs {
		if !strings.HasPrefix(l, "Program data: ") {
			continue
		}
		raw := strings.TrimPrefix(l, "Program data: ")
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			if data, err = base64.RawStdEncoding.DecodeString(raw); err != nil {
				continue
			}
		}
		if len(data) < 8 || binary.LittleEndian.Uint64(data[:8]) != wantDisc {
			continue
		}
		if err := borsh.Deserialize(out, data); err != nil {
			return false, fmt.Errorf("meteora event borsh: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func meteoraTokenDecimal(dtx *DecodedTx, accountKeys []common.PublicKey, instruction *solTypes.CompiledInstruction, acctIdx int, mint string) uint8 {
	if acctIdx < len(instruction.Accounts) {
		ta := accountKeys[instruction.Accounts[acctIdx]].String()
		if info := dtx.TokenAccountMap[ta]; info != nil && info.TokenDecimal > 0 {
			return info.TokenDecimal
		}
	}
	if d, ok := dtx.TokenDecimalMap[mint]; ok && d > 0 {
		return d
	}
	return 6 // pump-meteora tokens are 6-decimal (enforced by the contract)
}

// DecodePumpMeteoraInstruction routes a pump-meteora instruction to the swap/create decoder.
func DecodePumpMeteoraInstruction(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	fmt.Println("PROBE_METEORA_HIT tx=", dtx.TxHash, "disc=", GetPumpInstruction(instruction.Data))
	switch GetPumpInstruction(instruction.Data) {
	case MeteoraInstructionSwap:
		return decodeMeteoraSwap(dtx, instruction, logIndex)
	case MeteoraInstructionCreate:
		return decodeMeteoraCreate(dtx, instruction, logIndex)
	default:
		return nil, ErrUnknownProgram
	}
}

func decodeMeteoraSwap(dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	// swap account layout (13): [config, team, team2, creator, bonding_curve, global_vault,
	// token_mint, global_ata, user_ata, user, system, token, ata_program]
	if len(instruction.Accounts) < 13 {
		return nil, fmt.Errorf("meteora swap accounts len:%d hash:%v", len(instruction.Accounts), dtx.TxHash)
	}
	var ev MeteoraSwapEvent
	ok, derr := decodeMeteoraEvent(dtx.Tx.Meta.LogMessages, MeteoraEventSwap, &ev)
	if derr != nil {
		return nil, derr
	}
	if !ok {
		return nil, fmt.Errorf("meteora SwapEvent not found hash:%v", dtx.TxHash)
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

	currentToken := decimal.New(int64(ev.RealTokenReserves), -int32(decimals)).InexactFloat64()
	currentSol := decimal.New(int64(ev.RealSolReserves), -constants.SolDecimal).InexactFloat64()
	trade.CurrentBaseTokenInPoolAmount = currentSol
	trade.CurrentTokenInPoolAmount = currentToken
	// Deployed contract emits real reserves; approximate the virtual curve for charting.
	trade.PumpVirtualBaseTokenReserves = currentSol + MeteoraInitVirtualSol
	trade.PumpVirtualTokenReserves = currentToken

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
		Name:             constants.PumpMeteora,
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

	trade.SwapName = constants.PumpMeteora
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
	logx.Infof("METEORA OK swap type=%v token=%v price=%v hash=%v", trade.Type, mint, trade.TokenPriceUSD, dtx.TxHash)
	return trade, nil
}

func decodeMeteoraCreate(dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	// create_bonding_curve account layout (13): [config, global_vault, creator, token(mint),
	// bonding_curve, metadata, global_token_account, system, rent, token, ata, mpl, team]
	if len(instruction.Accounts) < 13 {
		return nil, fmt.Errorf("meteora create accounts len:%d hash:%v", len(instruction.Accounts), dtx.TxHash)
	}
	var ev MeteoraLaunchEvent
	ok, derr := decodeMeteoraEvent(dtx.Tx.Meta.LogMessages, MeteoraEventLaunch, &ev)
	if derr != nil {
		return nil, derr
	}
	if !ok {
		return nil, fmt.Errorf("meteora LaunchEvent not found hash:%v", dtx.TxHash)
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
		ChainId:          SolChainId,
		Addr:             pair,
		BaseTokenAddr:    util.GetBaseToken(SolChainIdInt).Address,
		BaseTokenDecimal: uint8(util.GetBaseToken(SolChainIdInt).Decimal),
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        mint,
		TokenDecimal:     decimals,
		BlockTime:        blockDb.BlockTime.Unix(),
		BlockNum:         blockDb.Slot,
		Name:             constants.PumpMeteora,
		TokenTotalSupply: MeteoraTotalSupplyWhole,
		InitTokenAmount:  MeteoraInitRealTokenWhole,
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

	trade.SwapName = constants.PumpMeteora
	trade.BaseTokenAccountAddress = ""
	trade.PumpLaunched = false
	trade.PumpPoint = 0
	trade.PumpPairAddr = pair
	trade.PumpStatus = pump.PumpStatusCreate

	logx.Infof("METEORA OK create token=%v curve=%v hash=%v", mint, pair, dtx.TxHash)
	return trade, nil
}
