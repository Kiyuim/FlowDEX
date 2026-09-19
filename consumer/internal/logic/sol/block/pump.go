package block

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"dex/consumer/internal/logic/pump"
	"dex/consumer/internal/svc"
	"dex/pkg/constants"
	"dex/pkg/types"
	"dex/pkg/util"

	"github.com/blocto/solana-go-sdk/client"
	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/near/borsh-go"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	PumpInstructionBuy    = 0xeaebda01123d0666
	PumpInstructionSync   = 0x1d9acb512ea545e4
	PumpInstructionSell   = 0xad837f01a485e633
	PumpInstructionCreate = 0x77071c0528c81e18
)

func DecodePumpEvent(logs []string) (events []PumpEvent, err error) {
	for i := range logs {
		var event PumpEvent
		if len(logs[i]) > 100 && strings.HasPrefix(logs[i], "Program data: vdt/007m") {
			prefix := strings.TrimPrefix(logs[i], "Program data: ")
			// 首先尝试标准base64解码
			data, err := base64.StdEncoding.DecodeString(prefix)
			if err != nil {
				// 如果标准解码失败，尝试Raw编码
				data, err = base64.RawStdEncoding.DecodeString(prefix)
				if err != nil {
					err = fmt.Errorf("base64 decode failed for both StdEncoding and RawStdEncoding: %w", err)
					return nil, err
				}
			}

			event, err = DeserializePumpEvent(data)
			if err != nil {
				err = fmt.Errorf("borsh.Deserialize err:%w", err)
				return nil, err
			}
			events = append(events, event)
		}
	}
	return
}
func DeserializePumpEvent(data []byte) (result PumpEvent, err error) {
	err = borsh.Deserialize(&result, data)
	return
}

func DeserializePumpEventBuy(data []byte) (result PumpEventBuy, err error) {
	err = borsh.Deserialize(&result, data)
	return
}
func GetPumpInstruction(data []byte) uint64 {
	if len(data) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(data[:8])
}

func DecodePumpInstruction(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	pumpInstruction := GetPumpInstruction(instruction.Data)
	switch pumpInstruction {
	// https://solscan.io/tx/583fVDdeDZL4hTJtdtgRZyeWUojBcgU5bRsoGrp2uvGrHxHu7vgcYrz2Lq6CDzZrKnkGJfibx816YbMGZDt9jasR
	// https://solscan.io/tx/23SQDChrDcbGiX5Fc4rxsCqRkz18hqfJy8tAnAYW9xXNa3QpCFNoVzSJ9LH1n5D1nuPTws16FF2FUdJXGpyppbb2
	case PumpInstructionBuy, PumpInstructionSell:
	default:
		logx.Errorf("decodePumpInstruction pump instruction err:%x, hash: %v", pumpInstruction, dtx.TxHash)
		return
	}

	tx := dtx.Tx
	tokenAccountMap := dtx.TokenAccountMap
	accountKeys := tx.AccountKeys
	txHash := dtx.TxHash
	solPrice := dtx.SolPrice
	blockDb := dtx.BlockDb

	fmt.Println("111111111111******111111111111111")

	// jupiter maybe 13 https://solscan.io/tx/2bPUgLQCRFufi2fsYTZ7PwPvUN3h3f3d8QGhsK9TDa6YNVTgt6UHWpUD4cQPJZtsr4NCywQxhUjyMM8SNtZFDntZ
	if len(instruction.Accounts) < 12 {
		err = fmt.Errorf("pump swap instruction account fail len:%d, hash: %v", len(instruction.Accounts), dtx.TxHash)
		return
	}

	// https://solscan.io/tx/583fVDdeDZL4hTJtdtgRZyeWUojBcgU5bRsoGrp2uvGrHxHu7vgcYrz2Lq6CDzZrKnkGJfibx816YbMGZDt9jasR
	pair := accountKeys[instruction.Accounts[3]].String()
	to := accountKeys[instruction.Accounts[4]].String()
	maker := accountKeys[instruction.Accounts[6]].String()
	tokenAccount := accountKeys[instruction.Accounts[5]].String()

	tokenAccountInfo := tokenAccountMap[tokenAccount]
	if tokenAccountInfo == nil {
		err = fmt.Errorf("tokenAccountInfo not found")
		return
	}
	dtx.PumpEvents, err = DecodePumpEvent(dtx.Tx.Meta.LogMessages)
	if err != nil {
		logx.Errorf("pump swap instruction decode err:%v, hash: %v", err, dtx.TxHash)
	}
	events := dtx.PumpEvents
	// https://solscan.io/tx/FG8X6y5hWwqFLp7PWzPdA3gT8G5EktXJizkt3AWKZfnWB1SC2T7BiMpQEkkyt9aer2XRRJQgKjQgDRTZZ1msNa3
	// https://solscan.io/tx/588MwDkmm5Wv7RCBdJ7T3wnhUx19aetEiPNB1WzuaNGyKE8peEHJWtUNJYGPd8G4kB25buioBgUtMKB4yszZwssY
	if dtx.PumpEventIndex >= len(events) {
		err = fmt.Errorf("pump event not found, i:%v, len:%v, hash:%v", dtx.PumpEventIndex, len(events), txHash)
		logx.Error(err)
		return
	}
	event := events[dtx.PumpEventIndex]
	dtx.PumpEventIndex++
	tokenAddress := event.Mint.String()
	tokenDecimal := tokenAccountInfo.TokenDecimal

	trade = &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = txHash
	trade.PairAddr = pair
	trade.Maker = maker
	trade.To = to

	realTokenReserves := event.VirtualTokenReserves - TokenReservesDiff
	realSolReserves := event.VirtualSolReserves - SolReservesDiff

	trade.CurrentBaseTokenInPoolAmount = decimal.New(int64(realSolReserves), -constants.SolDecimal).InexactFloat64()
	trade.CurrentTokenInPoolAmount = decimal.New(int64(realTokenReserves), -int32(tokenDecimal)).InexactFloat64()
	trade.PumpVirtualBaseTokenReserves = decimal.New(int64(event.VirtualSolReserves), -constants.SolDecimal).InexactFloat64()
	trade.PumpVirtualTokenReserves = decimal.New(int64(event.VirtualTokenReserves), -int32(tokenDecimal)).InexactFloat64()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             pair,
		BaseTokenAddr:    util.GetBaseToken(SolChainIdInt).Address,
		BaseTokenDecimal: uint8(util.GetBaseToken(SolChainIdInt).Decimal),
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenAddress,
		TokenDecimal:     tokenAccountInfo.TokenDecimal,
		BlockTime:        blockDb.BlockTime.Unix(),
		BlockNum:         blockDb.Slot,
	}

	if event.IsBuy {
		trade.Type = types.TradeTypeBuy
	} else {
		trade.Type = types.TradeTypeSell
	}

	trade.BaseTokenAmount = decimal.New(int64(event.SolAmount), -constants.SolDecimal).InexactFloat64()

	trade.TokenAmount = decimal.New(int64(event.TokenAmount), -int32(tokenDecimal)).InexactFloat64()
	trade.BaseTokenPriceUSD = solPrice
	trade.TotalUSD = decimal.NewFromFloat(trade.BaseTokenAmount).Mul(decimal.NewFromFloat(solPrice)).InexactFloat64()
	if trade.TokenAmount == 0 {
		// https://solscan.io/tx/pxZ9itnoK5bLvjhzsUFLaKE1bbVreX7gMNhmnEeagUKeTdA29YE1f9YeyNdNsD5RikjF4o9PtRYTjvkZLXLETS4
		err = ErrTokenAmountIsZero
		return
	}
	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(trade.TokenAmount)).InexactFloat64()
	trade.BlockNum = blockDb.Slot
	trade.BlockTime = blockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)
	trade.TransactionIndex = dtx.TxIndex
	trade.LogIndex = int(logIndex)

	trade.SwapName = constants.PumpFun
	trade.PairInfo.Name = trade.SwapName
	trade.BaseTokenAccountAddress = ""
	trade.TokenAccountAddress = tokenAccount
	trade.BaseTokenAmountInt = int64(event.SolAmount)
	trade.TokenAmountInt = int64(event.TokenAmount)
	trade.PumpLaunched = false

	pumpPoint := decimal.NewFromInt(1).Sub(decimal.NewFromFloat(trade.CurrentTokenInPoolAmount).Div(decimal.NewFromInt(int64(InitPumpTokenAmount)))).InexactFloat64()
	pumpPoint = min(max(pumpPoint, 0), 1)

	if trade.CurrentTokenInPoolAmount <= 0 {
		pumpPoint = 1
	}

	trade.PairInfo.InitTokenAmount = VirtualInitPumpTokenAmount
	trade.PairInfo.InitBaseTokenAmount = InitSolTokenAmount

	trade.PumpPoint = pumpPoint

	fmt.Println("11111111111111I found pair and it pumpPoint now is:", pair, trade.PumpPoint)
	if pair == "65XEXmw6yrRBobFQy6WdECbpSMX2dybuDe1C3ziMzK9q" {
		fmt.Println("I found pair and it pumpPoint now is:", trade.PumpPoint)
	}

	trade.PumpMarketCap = decimal.NewFromFloat(trade.TokenPriceUSD).Mul(decimal.NewFromFloat(trade.PairInfo.TokenTotalSupply)).InexactFloat64()
	trade.Fdv = trade.PumpMarketCap
	trade.PumpPairAddr = pair

	fmt.Println("trade.Fdv is:", trade.Fdv)
	trade.PumpStatus = pump.PumpStatusTrading
	if trade.PumpPoint >= 0.999 {
		trade.PumpStatus = pump.PumpStatusMigrating
		trade.PumpPoint = 1
	}
	return
}

func GetInnerInstruction(dtx *DecodedTx, i, j int, innerLen int) (innerInstructions *client.InnerInstruction, err error) {
	instructions := dtx.InnerInstructionMap[i]
	if instructions == nil {
		err = errors.New("innerInstructions not found")
		return
	}
	if len(instructions.Instructions) < j+innerLen {
		err = errors.New("no enough instruction")
		return
	}
	innerInstructions = &client.InnerInstruction{
		Index: uint64(j),
	}
	for k := 0; k < innerLen; k++ {
		innerInstructions.Instructions = append(innerInstructions.Instructions, instructions.Instructions[j+k])
	}

	return
}

func DecodePumpCreate(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, logIndex int) (trade *types.TradeWithPair, err error) {
	fmt.Println("*****************111111111111***********s")
	tx := dtx.Tx
	tokenAccountMap := dtx.TokenAccountMap
	accountKeys := tx.AccountKeys
	txHash := dtx.TxHash
	solPrice := dtx.SolPrice
	blockDb := dtx.BlockDb

	if len(instruction.Accounts) != 14 {
		err = fmt.Errorf("pump create instruction account fail len:%d, hash: %#v", len(instruction.Accounts), dtx.TxHash)
		return
	}
	tokenAddress := accountKeys[instruction.Accounts[0]].String()
	// mintAuthority := accountKeys[instruction.Accounts[1]]
	pair := accountKeys[instruction.Accounts[2]].String()
	pairTokenAccount := accountKeys[instruction.Accounts[3]].String()
	// global
	// MplTokenMetadata
	// Metadata
	maker := accountKeys[instruction.Accounts[7]].String()
	// System Program:
	// Token Program:
	// Associated Token Program:
	// Rent
	// Event Authority
	// Program

	pairTokenAccountInfo := tokenAccountMap[pairTokenAccount]
	if pairTokenAccountInfo == nil {
		err = fmt.Errorf("coverTokenAccount not found")
		return
	}

	tokenDecimal := pairTokenAccountInfo.TokenDecimal
	trade = &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = txHash
	trade.PairAddr = pair
	trade.Maker = maker

	trade.PairInfo.InitTokenAmount = VirtualInitPumpTokenAmount
	trade.PairInfo.InitBaseTokenAmount = InitSolTokenAmount

	trade.CurrentTokenInPoolAmount = VirtualInitPumpTokenAmount
	trade.CurrentBaseTokenInPoolAmount = InitSolTokenAmount

	trade.PumpVirtualBaseTokenReserves = 30
	trade.PumpVirtualTokenReserves = VirtualInitPumpTokenAmount

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             pair,
		BaseTokenAddr:    util.GetBaseToken(SolChainIdInt).Address,
		BaseTokenDecimal: uint8(util.GetBaseToken(SolChainIdInt).Decimal),
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenAddress,
		TokenDecimal:     tokenDecimal,
		BlockTime:        blockDb.BlockTime.Unix(),
		BlockNum:         blockDb.Slot,
	}

	trade.Maker = maker // address
	trade.Type = types.TradePumpCreate
	// trade.BaseTokenAmount = decimal.New(int64(event.SolAmount), -constants.SolDecimal).InexactFloat64()
	// trade.TokenAmount = decimal.New(int64(event.TokenAmount), -int32(tokenDecimal)).InexactFloat64()
	trade.BaseTokenPriceUSD = solPrice
	trade.TotalUSD = decimal.NewFromFloat(trade.BaseTokenAmount).Mul(decimal.NewFromFloat(solPrice)).InexactFloat64() // total usd

	trade.TokenPriceUSD = decimal.NewFromFloat(solPrice).Mul(decimal.NewFromFloat(0.00000000775)).InexactFloat64() // price
	trade.To = ""
	trade.BlockNum = blockDb.Slot              // block num
	trade.BlockTime = blockDb.BlockTime.Unix() // block time
	trade.HashId = fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)
	trade.TransactionIndex = dtx.TxIndex
	trade.LogIndex = int(logIndex)

	trade.SwapName = constants.PumpFun
	trade.PairInfo.Name = trade.SwapName
	trade.BaseTokenAccountAddress = ""
	// trade.TokenAccountAddress = tokenAccount
	// trade.BaseTokenAmountInt = int64(event.SolAmount)
	// trade.TokenAmountInt = int64(event.TokenAmount)
	trade.PumpLaunched = false
	trade.PumpPoint = 0
	trade.PumpPairAddr = pair
	trade.PumpStatus = pump.PumpStatusCreate
	if trade.PumpPoint >= 0.999 {
		trade.PumpStatus = pump.PumpStatusMigrating
	}

	// Note: WebSocket push moved to pair.go for complete token data

	return
}
