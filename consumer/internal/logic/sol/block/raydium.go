package block

import (
	"context"
	"dex/consumer/internal/svc"
	"dex/pkg/constants"
	"dex/pkg/types"
	"dex/pkg/util"
	"errors"
	"fmt"
	"math"

	"dex/model/solmodel"

	"dex/consumer/pkg/raydium"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/token"
	solTypes "github.com/blocto/solana-go-sdk/types"
	aSDK "github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

var RaydiumSwapAccountV1IndexMap = make(map[string]int)
var RaydiumSwapAccountV2IndexMap = make(map[string]int)

func GetRaydiumSwapAccountIndex(name string, l int) int {
	if l == 17 {
		return RaydiumSwapAccountV1IndexMap[name]
	} else if l == 18 {
		return RaydiumSwapAccountV2IndexMap[name]
	}
	return 1
}

func DecodeRaydiumInstruction(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, innerInstructions *client.InnerInstruction) (trade *types.TradeWithPair, err error) {
	if len(instruction.Data) == 0 {
		return
	}
	switch instruction.Data[0] {
	case 9, 11:
		return DecodeRaydiumSwap(ctx, sc, dtx, instruction, innerInstructions)
	case 1:
		// https://solscan.io/tx/2YY7BM9NMH8fbSgNsy6YhWg6hEvnbpau4kNdmTXYcqs7UHo6NxY8MuZZH7qJke71vuNvmQmUUDA8LdkdgqYyrFP9
		return DecodeRaydiumAdd(ctx, sc, dtx, instruction, innerInstructions)
	}
	return
}

func DecodeRaydiumSwap(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, innerInstructions *client.InnerInstruction) (trade *types.TradeWithPair, err error) {
	tx := dtx.Tx
	tokenAccountMap := dtx.TokenAccountMap
	accountKeys := tx.AccountKeys
	txHash := dtx.TxHash
	solPrice := dtx.SolPrice
	blockDb := dtx.BlockDb

	if len(instruction.Accounts) != 18 && len(instruction.Accounts) != 17 {
		err = errors.New("RaydiumV4 swap instruction account fail len")
		return
	}

	swap, err := GetTokenSwap(ctx, sc, accountKeys, tokenAccountMap, instruction, innerInstructions)
	if err != nil {
		if !errors.Is(err, ErrNotSupportWarp) {
			err = fmt.Errorf("GetTokenSwap err:%w", err)
		}
		return
	}

	maker := tx.AccountKeys[instruction.Accounts[len(instruction.Accounts)-1]].String()
	pair := tx.AccountKeys[instruction.Accounts[1]].String()
	accountLen := len(instruction.Accounts)

	trade = &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = txHash
	trade.PairAddr = pair
	trade.RaydiumPool = &solmodel.RaydiumPool{
		ChainId:               SolChainIdInt,
		AmmId:                 tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("AmmId", accountLen)]].String(),
		AmmAuthority:          tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("AmmAuthority", accountLen)]].String(),
		AmmOpenOrders:         tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("AmmOpenOrders", accountLen)]].String(),
		PoolCoinTokenAccount:  tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("PoolCoinTokenAccount", accountLen)]].String(),
		PoolPcTokenAccount:    tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("PoolPcTokenAccount", accountLen)]].String(),
		SerumProgramId:        tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumProgramId", accountLen)]].String(),
		SerumMarket:           tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumMarket", accountLen)]].String(),
		SerumBids:             tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumBids", accountLen)]].String(),
		SerumAsks:             tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumAsks", accountLen)]].String(),
		SerumEventQueue:       tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumEventQueue", accountLen)]].String(),
		SerumCoinVaultAccount: tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumCoinVaultAccount", accountLen)]].String(),
		SerumPcVaultAccount:   tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumPcVaultAccount", accountLen)]].String(),
		SerumVaultSigner:      tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("SerumVaultSigner", accountLen)]].String(),
		TxHash:                txHash,
	}
	if accountLen == 18 {
		trade.RaydiumPool.AmmTargetOrders = tx.AccountKeys[instruction.Accounts[GetRaydiumSwapAccountIndex("AmmTargetOrders", accountLen)]].String()
	}
	if trade.RaydiumPool.PoolPcTokenAccount != "" {
		poolTokenAccount := tokenAccountMap[trade.RaydiumPool.PoolPcTokenAccount]
		if poolTokenAccount != nil {
			if poolTokenAccount.TokenAddress == swap.BaseTokenInfo.TokenAddress {
				trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			} else if poolTokenAccount.TokenAddress == swap.TokenInfo.TokenAddress {
				trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			}
		}
	}
	if trade.RaydiumPool.PoolCoinTokenAccount != "" {
		poolTokenAccount := tokenAccountMap[trade.RaydiumPool.PoolCoinTokenAccount]
		if poolTokenAccount != nil {
			if poolTokenAccount.TokenAddress == swap.BaseTokenInfo.TokenAddress {
				trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			} else if poolTokenAccount.TokenAddress == swap.TokenInfo.TokenAddress {
				trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			}
		}
	}

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             pair,
		BaseTokenAddr:    swap.BaseTokenInfo.TokenAddress,
		BaseTokenDecimal: swap.BaseTokenInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        swap.TokenInfo.TokenAddress,
		TokenDecimal:     swap.TokenInfo.TokenDecimal,
		BlockTime:        blockDb.BlockTime.Unix(),
		BlockNum:         blockDb.Slot,
	}

	trade.Maker = maker
	trade.Type = swap.Type
	trade.BaseTokenAmount = swap.BaseTokenAmount
	trade.TokenAmount = swap.TokenAmount
	trade.BaseTokenPriceUSD = solPrice
	trade.TotalUSD = decimal.NewFromFloat(swap.BaseTokenAmount).Mul(decimal.NewFromFloat(solPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err = fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(swap.TokenAmount)).InexactFloat64() // price

	trade.To = swap.To
	trade.BlockNum = blockDb.Slot
	trade.BlockTime = blockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)
	trade.TransactionIndex = dtx.TxIndex
	trade.LogIndex = int(innerInstructions.Index)

	trade.SwapName = constants.RaydiumV4
	trade.PairInfo.Name = trade.SwapName
	trade.BaseTokenAccountAddress = swap.BaseTokenInfo.TokenAccountAddress
	trade.TokenAccountAddress = swap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = swap.BaseTokenAmountInt
	trade.TokenAmountInt = swap.TokenAmountInt
	return
}

func DecodeRaydiumAdd(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, innerInstructions *client.InnerInstruction) (trade *types.TradeWithPair, err error) {
	tx := dtx.Tx
	tokenAccountMap := dtx.TokenAccountMap
	txHash := dtx.TxHash
	solPrice := dtx.SolPrice
	blockDb := dtx.BlockDb
	if len(instruction.Accounts) != 21 {
		err = errors.New("RaydiumV4 Add instruction account fail len")
		return
	}
	maker := tx.AccountKeys[instruction.Accounts[17]].String()
	pair := tx.AccountKeys[instruction.Accounts[4]].String()

	trade = &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = txHash
	trade.PairAddr = pair
	trade.RaydiumPool = &solmodel.RaydiumPool{
		ChainId:              SolChainIdInt,
		AmmId:                pair,
		AmmAuthority:         tx.AccountKeys[instruction.Accounts[5]].String(),
		AmmOpenOrders:        tx.AccountKeys[instruction.Accounts[6]].String(),
		PoolCoinTokenAccount: tx.AccountKeys[instruction.Accounts[10]].String(),
		PoolPcTokenAccount:   tx.AccountKeys[instruction.Accounts[11]].String(),
		SerumProgramId:       tx.AccountKeys[instruction.Accounts[15]].String(),
		SerumMarket:          tx.AccountKeys[instruction.Accounts[16]].String(),
		TxHash:               txHash,
		BaseMint:             tx.AccountKeys[instruction.Accounts[8]].String(),
		QuoteMint:            tx.AccountKeys[instruction.Accounts[9]].String(),
	}
	trade.RaydiumPool.AmmTargetOrders = tx.AccountKeys[instruction.Accounts[13]].String()

	tokenAddress := tx.AccountKeys[instruction.Accounts[9]].String()
	baseTokenAddress := tx.AccountKeys[instruction.Accounts[8]].String()
	if tokenAddress == constants.TokenStrWrapSol {
		tokenAddress, baseTokenAddress = baseTokenAddress, tokenAddress
	} else if baseTokenAddress != constants.TokenStrWrapSol {
		err = fmt.Errorf("not sol swap")
		return nil, err
	}
	baseTokenDecimal := uint8(9)
	tokenDecimal := uint8(0)

	if trade.RaydiumPool.PoolPcTokenAccount != "" {
		poolTokenAccount := tokenAccountMap[trade.RaydiumPool.PoolPcTokenAccount]
		if poolTokenAccount != nil {
			if poolTokenAccount.TokenAddress == tokenAddress {
				tokenDecimal = poolTokenAccount.TokenDecimal
				trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			} else if poolTokenAccount.TokenAddress == baseTokenAddress {
				baseTokenDecimal = poolTokenAccount.TokenDecimal
				trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			}
		}
	}
	if trade.RaydiumPool.PoolCoinTokenAccount != "" {
		poolTokenAccount := tokenAccountMap[trade.RaydiumPool.PoolCoinTokenAccount]
		if poolTokenAccount != nil {
			if poolTokenAccount.TokenAddress == tokenAddress {
				tokenDecimal = poolTokenAccount.TokenDecimal
				trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			} else if poolTokenAccount.TokenAddress == baseTokenAddress {
				baseTokenDecimal = poolTokenAccount.TokenDecimal
				trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
			}
		}
	}

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             pair,
		BaseTokenAddr:    baseTokenAddress,
		BaseTokenDecimal: uint8(baseTokenDecimal),
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenAddress,
		TokenDecimal:     uint8(tokenDecimal),
		BlockTime:        blockDb.BlockTime.Unix(),
		BlockNum:         blockDb.Slot,
	}

	ii, err := raydium.DecodeRaydiumCreate(instruction)
	if err != nil || ii == nil {
		logx.Errorf("DecodeRaydiumCreate error: %v, tx hash: %v", err, dtx.TxHash)
	}
	if err == nil && ii != nil {
		initTokenAmountInt := ii.InitPcAmount
		initTokenAmount := decimal.NewFromUint64(ii.InitPcAmount).Div(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(tokenDecimal)))).InexactFloat64()
		initBaseTokenAmountInt := ii.InitPcAmount
		initBaseTokenAmount := decimal.NewFromUint64(ii.InitCoinAmount).Div(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(baseTokenDecimal)))).InexactFloat64()
		// 池子里的交易对可能反过来填，这里也需要反过来，如果 QuoteMint 是空 表明数据服务那还未更新，由于大部分的都是正常的，所以空的话这里就不反过来
		if trade.RaydiumPool.QuoteMint != "" && trade.RaydiumPool.QuoteMint == aSDK.WrappedSol.String() {
			initTokenAmount, initBaseTokenAmount = initBaseTokenAmount, initTokenAmount
			initTokenAmountInt, initBaseTokenAmountInt = initBaseTokenAmountInt, initTokenAmountInt
		}

		trade.PairInfo.InitTokenAmount = initTokenAmount
		trade.PairInfo.InitBaseTokenAmount = initBaseTokenAmount

		trade.TokenAmountInt = int64(initTokenAmountInt)
		trade.BaseTokenAmountInt = int64(initBaseTokenAmountInt)
	}

	trade.Maker = maker                                        // address
	trade.Type = types.TradeTypeAddPosition                    // tag: sell/buy/add_position/remove_position
	trade.BaseTokenAmount = trade.PairInfo.InitBaseTokenAmount // Amount of base token changed
	trade.TokenAmount = trade.PairInfo.InitTokenAmount         // Amount of non-base token changed
	trade.BaseTokenPriceUSD = solPrice
	trade.TotalUSD = decimal.NewFromFloat(trade.BaseTokenAmount).Mul(decimal.NewFromFloat(solPrice)).InexactFloat64() // total usd
	if trade.TokenAmount > 0 {
		trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(trade.TokenAmount)).InexactFloat64() // price
	}

	trade.To = ""
	trade.BlockNum = blockDb.Slot              // block num
	trade.BlockTime = blockDb.BlockTime.Unix() // block time
	trade.HashId = fmt.Sprintf("%v#%d", blockDb.Slot, dtx.TxIndex)
	trade.TransactionIndex = dtx.TxIndex
	trade.LogIndex = int(innerInstructions.Index)
	trade.SwapName = constants.RaydiumV4
	trade.PairInfo.Name = trade.SwapName
	trade.BaseTokenAccountAddress = ""
	trade.TokenAccountAddress = ""

	return
}

func GetTokenSwap(ctx context.Context, sc *svc.ServiceContext, accountKeys []common.PublicKey, tokenAccountMap map[string]*TokenAccount, instruction *solTypes.CompiledInstruction, innerInstructions *client.InnerInstruction) (swap *Swap, err error) {
	var fromTransfer *token.TransferParam
	var toTransfer *token.TransferParam
	swap = &Swap{}
	fromTokenAccount := accountKeys[instruction.Accounts[len(instruction.Accounts)-3]]
	fromTokenAccountStr := fromTokenAccount.String()
	fromTokenAccountInfo := tokenAccountMap[fromTokenAccountStr]
	if fromTokenAccountInfo == nil {
		err = errors.New("fromTokenAccountInfo not found")
		return
	}
	toTokenAccount := accountKeys[instruction.Accounts[len(instruction.Accounts)-2]]
	toTokenAccountStr := toTokenAccount.String()
	toTokenAccountInfo := tokenAccountMap[toTokenAccountStr]
	if toTokenAccountInfo == nil {
		err = errors.New("toTokenAccountInfo not found ")
		return
	}
	for j := range innerInstructions.Instructions {
		transfer, err := DecodeTokenTransfer(accountKeys, &innerInstructions.Instructions[j])
		if err != nil {
			// err = fmt.Errorf("DecodeTokenTransfer err:%w", err)
			continue
		}
		if transfer.From.String() == fromTokenAccountStr {
			fromTransfer = transfer
		} else if transfer.To.String() == toTokenAccountStr {
			toTransfer = transfer
		}
	}
	if fromTransfer == nil {
		err = errors.New("fromTransfer not found ")
		return
	}
	if toTransfer == nil {
		err = errors.New("toTransfer not found ")
		return
	}
	if !IsSwapTransfer(fromTransfer, toTransfer, tokenAccountMap) {
		err = errors.New("not swap transfer")
		return
	}
	if fromTokenAccountInfo.TokenDecimal == 0 {
		fromTokenAccountInfo.TokenDecimal, err = GetTokenDecimal(ctx, sc, fromTokenAccountInfo.TokenAddress)
		if err != nil {
			err = fmt.Errorf("GetFormTokenDecimal err:%w, token(%v)", err, fromTokenAccountInfo.TokenAddress)
			return
		}
	}
	if toTokenAccountInfo.TokenDecimal == 0 {
		toTokenAccountInfo.TokenDecimal, err = GetTokenDecimal(ctx, sc, toTokenAccountInfo.TokenAddress)
		if err != nil {
			err = fmt.Errorf("GetToTokenDecimal err:%w, token(%v)", err, toTokenAccountInfo.TokenAddress)
			return
		}
	}
	if fromTokenAccountInfo.TokenAddress == TokenStrWrapSol {
		swap.BaseTokenInfo = fromTokenAccountInfo
		swap.TokenInfo = toTokenAccountInfo
		swap.Type = types.TradeTypeBuy
		swap.BaseTokenAmountInt = int64(fromTransfer.Amount)
		swap.BaseTokenAmount = float64(fromTransfer.Amount) / math.Pow10(int(fromTokenAccountInfo.TokenDecimal))
		swap.TokenAmountInt = int64(toTransfer.Amount)
		swap.TokenAmount = float64(toTransfer.Amount) / math.Pow10(int(toTokenAccountInfo.TokenDecimal))
		swap.To = toTokenAccountInfo.Owner
	} else if toTokenAccountInfo.TokenAddress == TokenStrWrapSol {
		swap.BaseTokenInfo = toTokenAccountInfo
		swap.TokenInfo = fromTokenAccountInfo
		swap.Type = types.TradeTypeSell
		swap.BaseTokenAmountInt = int64(toTransfer.Amount)
		swap.BaseTokenAmount = float64(toTransfer.Amount) / math.Pow10(int(toTokenAccountInfo.TokenDecimal))
		swap.TokenAmountInt = int64(fromTransfer.Amount)
		swap.TokenAmount = float64(fromTransfer.Amount) / math.Pow10(int(fromTokenAccountInfo.TokenDecimal))
		swap.To = fromTokenAccountInfo.Owner
	} else {
		err = ErrNotSupportWarp
		return
	}
	return
}
