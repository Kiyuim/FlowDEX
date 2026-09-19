package block

import (
	"fmt"
	"math"

	"dex/consumer/internal/svc"
	"dex/pkg/constants"
	"dex/pkg/raydium/clmm/idl/generated/amm_v3"
	"dex/pkg/types"
	"dex/pkg/util"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/token"
	"github.com/blocto/solana-go-sdk/rpc"
	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/duke-git/lancet/v2/slice"
	bin "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/net/context"
)

type ConcentratedLiquidityDecoder struct {
	ctx                 context.Context
	svcCtx              *svc.ServiceContext
	dtx                 *DecodedTx
	compiledInstruction *solTypes.CompiledInstruction
	innerInstruction    *client.InnerInstruction
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquidityInstruction() (*types.TradeWithPair, error) {
	// todo: error 2025-01-26T12:25:53.583+08:00	 error 	error find clmm tx: 54WyiccBSauoTUzCw4Q636FUa8tZNMDHChtfwTtoqoEoKru2kjZVGdhniwbEej6tLHE8WQfA7cDQPxuxckUJuSrV, err : toTransfer not found 	caller=block/block.go:560
	// todo: error 2025-01-26T12:25:53.227+08:00	 error 	error find clmm tx: 3sR3RJYMRQ96BmFRrTBHz29uBmpsvy2kLp2b45CnErM9QCTZK6AgA6nF2UfHanhGGM3xYFzVNE5JxymWkqrF5RcP, err : not support swap	caller=block/block.go:560

	accountMetas := slice.Map[int, *ag_solanago.AccountMeta](decoder.compiledInstruction.Accounts, func(_ int, index int) *ag_solanago.AccountMeta {
		return &ag_solanago.AccountMeta{
			PublicKey: ag_solanago.PublicKeyFromBytes(decoder.dtx.Tx.AccountKeys[index].Bytes()),
			// no need
			IsWritable: false,
			IsSigner:   false,
		}
	})

	decodeInstruction, err := amm_v3.DecodeInstruction(accountMetas, decoder.compiledInstruction.Data)
	if err != nil {
		return nil, err
	}

	fmt.Println("decodeInstruction is:", decodeInstruction.TypeID)
	switch decodeInstruction.TypeID {
	case amm_v3.Instruction_Swap:
		//TODO: v1 version
		fmt.Println("decodeInstruction is Instruction_Swap:", decodeInstruction)
		swap := decodeInstruction.Impl.(*amm_v3.Swap)
		return decoder.DecodeRaydiumConcentratedLiquiditySwap(swap)
	case amm_v3.Instruction_SwapV2:
		fmt.Println("decodeInstruction is Instruction_SwapV2:", decodeInstruction)

		swapV2 := decodeInstruction.Impl.(*amm_v3.SwapV2)
		return decoder.DecodeRaydiumConcentratedLiquiditySwapV2(swapV2)
	case amm_v3.Instruction_CreateAmmConfig:
		fmt.Println("decodeInstruction is Instruction_CreateAmmConfig:", decodeInstruction)

		return nil, ErrNotSupportInstruction
	case amm_v3.Instruction_CreatePool:
		fmt.Println("decodeInstruction is Instruction_CreatePool:", decodeInstruction)

		createPool := decodeInstruction.Impl.(*amm_v3.CreatePool)
		return decoder.DecodeRaydiumConcentratedLiquidityCreatePool(createPool)
	case amm_v3.Instruction_IncreaseLiquidityV2:
		fmt.Println("decodeInstruction is Instruction_IncreaseLiquidityV2:", decodeInstruction)

		increaseLiquidityV2 := decodeInstruction.Impl.(*amm_v3.IncreaseLiquidityV2)
		return decoder.DecodeRaydiumConcentratedLiquidityIncreaseLiquidityV2(increaseLiquidityV2)
	case amm_v3.Instruction_DecreaseLiquidityV2:
		fmt.Println("decodeInstruction is Instruction_DecreaseLiquidityV2:", decodeInstruction)

		decreaseLiquidityV2 := decodeInstruction.Impl.(*amm_v3.DecreaseLiquidityV2)
		return decoder.DecodeRaydiumConcentratedLiquidityDecreaseLiquidityV2(decreaseLiquidityV2)
	case amm_v3.Instruction_SwapRouterBaseIn:
		fmt.Println("decodeInstruction is Instruction_SwapRouterBaseIn:", decodeInstruction)

		return nil, ErrNotSupportInstruction
	case amm_v3.Instruction_OpenPosition:
		fmt.Println("decodeInstruction is Instruction_OpenPosition:", decodeInstruction)
		openPosition := decodeInstruction.Impl.(*amm_v3.OpenPosition)
		return decoder.DecodeRaydiumConcentratedLiquidityOpenPosition(openPosition)
		// return nil, ErrNotSupportInstruction
	default:
		fmt.Println("decodeInstruction is default:", decodeInstruction)
		return nil, ErrNotSupportInstruction
	}
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquidityOpenPosition(openPosition *amm_v3.OpenPosition) (*types.TradeWithPair, error) {
	fmt.Println("openPosition decoing is working")
	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = openPosition.GetPoolStateAccount().PublicKey.String()
	trade.Type = "open_position"
	trade.Maker = openPosition.GetPayerAccount().PublicKey.String()
	trade.To = openPosition.GetPoolStateAccount().PublicKey.String()
	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.RaydiumConcentratedLiquidity

	// 组装OpenPosition参数和账户
	trade.CLMMOpenPositionInfo = &types.CLMMOpenPositionInfo{
		TickLowerIndex:           openPosition.TickLowerIndex,
		TickUpperIndex:           openPosition.TickUpperIndex,
		TickArrayLowerStartIndex: openPosition.TickArrayLowerStartIndex,
		TickArrayUpperStartIndex: openPosition.TickArrayUpperStartIndex,
		Liquidity:                openPosition.Liquidity,
		Amount0Max:               openPosition.Amount0Max,
		Amount1Max:               openPosition.Amount1Max,
		Payer:                    openPosition.GetPayerAccount().PublicKey.String(),
		PositionNftOwner:         openPosition.GetPositionNftOwnerAccount().PublicKey.String(),
		PositionNftMint:          openPosition.GetPositionNftMintAccount().PublicKey.String(),
		PositionNftAccount:       openPosition.GetPositionNftAccountAccount().PublicKey.String(),
		MetadataAccount:          openPosition.GetMetadataAccountAccount().PublicKey.String(),
		PoolState:                openPosition.GetPoolStateAccount().PublicKey.String(),
		ProtocolPosition:         openPosition.GetProtocolPositionAccount().PublicKey.String(),
		TickArrayLower:           openPosition.GetTickArrayLowerAccount().PublicKey.String(),
		TickArrayUpper:           openPosition.GetTickArrayUpperAccount().PublicKey.String(),
		PersonalPosition:         openPosition.GetPersonalPositionAccount().PublicKey.String(),
		TokenAccount0:            openPosition.GetTokenAccount0Account().PublicKey.String(),
		TokenAccount1:            openPosition.GetTokenAccount1Account().PublicKey.String(),
		TokenVault0:              openPosition.GetTokenVault0Account().PublicKey.String(),
		TokenVault1:              openPosition.GetTokenVault1Account().PublicKey.String(),
		Rent:                     openPosition.GetRentAccount().PublicKey.String(),
		SystemProgram:            openPosition.GetSystemProgramAccount().PublicKey.String(),
		TokenProgram:             openPosition.GetTokenProgramAccount().PublicKey.String(),
		AssociatedTokenProgram:   openPosition.GetAssociatedTokenProgramAccount().PublicKey.String(),
		MetadataProgram:          openPosition.GetMetadataProgramAccount().PublicKey.String(),
	}

	// === 自动填充 PairInfo ===
	// symbol 通过全局 TokenMap 查找
	tokenAccount0 := openPosition.GetTokenAccount0Account().PublicKey.String()
	tokenAccount1 := openPosition.GetTokenAccount1Account().PublicKey.String()
	token0Info := decoder.dtx.TokenAccountMap[tokenAccount0]
	token1Info := decoder.dtx.TokenAccountMap[tokenAccount1]
	var (
		baseTokenAddr, tokenAddr, baseTokenSymbol, tokenSymbol string
		baseTokenDecimal, tokenDecimal                         int64
	)
	if token0Info != nil && token0Info.TokenAddress == TokenStrWrapSol {
		baseTokenAddr = token0Info.TokenAddress
		baseTokenSymbol = util.GetBaseToken(SolChainIdInt).Symbol
		baseTokenDecimal = int64(token0Info.TokenDecimal)
		tokenAddr = token1Info.TokenAddress
		// tokenSymbol = token1Info.TokenSymbol // 错误
		tokenSymbol = "" // 或通过全局TokenMap查找
		tokenDecimal = int64(token1Info.TokenDecimal)
	} else if token1Info != nil && token1Info.TokenAddress == TokenStrWrapSol {
		baseTokenAddr = token1Info.TokenAddress
		baseTokenSymbol = util.GetBaseToken(SolChainIdInt).Symbol
		baseTokenDecimal = int64(token1Info.TokenDecimal)
		tokenAddr = token0Info.TokenAddress
		// tokenSymbol = token0Info.TokenSymbol // 错误
		tokenSymbol = ""
		tokenDecimal = int64(token0Info.TokenDecimal)
	} else if token0Info != nil && token1Info != nil {
		baseTokenAddr = token0Info.TokenAddress
		// baseTokenSymbol = token0Info.TokenSymbol // 错误
		baseTokenSymbol = ""
		baseTokenDecimal = int64(token0Info.TokenDecimal)
		tokenAddr = token1Info.TokenAddress
		// tokenSymbol = token1Info.TokenSymbol // 错误
		tokenSymbol = ""
		tokenDecimal = int64(token1Info.TokenDecimal)
	} else {
		baseTokenAddr = tokenAccount0
		tokenAddr = tokenAccount1
		baseTokenSymbol = ""
		tokenSymbol = ""
		baseTokenDecimal = 6
		tokenDecimal = 6
	}
	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             openPosition.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    baseTokenAddr,
		BaseTokenSymbol:  baseTokenSymbol,
		BaseTokenDecimal: uint8(baseTokenDecimal),
		TokenAddr:        tokenAddr,
		TokenSymbol:      tokenSymbol,
		TokenDecimal:     uint8(tokenDecimal),
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
		Name:             trade.SwapName,
	}

	trade.ClmmPoolInfoV1 = &types.CLMMPoolInfo{
		PoolState: solTypes.AccountMeta{PubKey: common.PublicKeyFromString(openPosition.GetPoolStateAccount().PublicKey.String())},
		TickArray: common.PublicKeyFromString(openPosition.GetTickArrayUpperAccount().PublicKey.String()),
		RemainingAccounts: []solTypes.AccountMeta{
			{
				PubKey:     common.PublicKeyFromString(openPosition.GetTickArrayLowerAccount().PublicKey.String()),
				IsSigner:   false,
				IsWritable: false,
			},
		},
	}

	fmt.Println("tokenAddress is:", trade.PairInfo.TokenAddr)
	fmt.Println("baseTokenAddress is:", trade.PairInfo.BaseTokenAddr)

	fmt.Println("tickArrayUpper is:", openPosition.GetTickArrayUpperAccount().PublicKey.String())
	fmt.Println("tickArrayLower is:", openPosition.GetTickArrayLowerAccount().PublicKey.String())

	fmt.Println("liquidity is:", trade.CLMMOpenPositionInfo.Liquidity)

	return trade, nil
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquiditySwap(swap *amm_v3.Swap) (*types.TradeWithPair, error) {
	tokenSwap, err := decoder.decodeRaydiumConcentratedLiquidityTokenSwap(swap)
	if err != nil {
		return nil, err
	}

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = swap.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             swap.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    tokenSwap.BaseTokenInfo.TokenAddress,
		BaseTokenDecimal: tokenSwap.BaseTokenInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenSwap.TokenInfo.TokenAddress,
		TokenDecimal:     tokenSwap.TokenInfo.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}

	trade.Maker = swap.GetInputTokenAccountAccount().PublicKey.String()
	trade.Type = tokenSwap.Type
	trade.BaseTokenAmount = tokenSwap.BaseTokenAmount
	trade.TokenAmount = tokenSwap.TokenAmount
	trade.BaseTokenPriceUSD = decoder.dtx.SolPrice
	trade.TotalUSD = decimal.NewFromFloat(tokenSwap.BaseTokenAmount).Mul(decimal.NewFromFloat(decoder.dtx.SolPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err = fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return nil, err
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(tokenSwap.TokenAmount)).InexactFloat64() // price

	if !swap.GetInputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swap.GetInputVaultAccount().PublicKey.String()]
		if poolTokenAccount.TokenAddress == tokenSwap.BaseTokenInfo.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		} else if poolTokenAccount.TokenAddress == tokenSwap.TokenInfo.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		}
	}

	if !swap.GetOutputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swap.GetOutputVaultAccount().PublicKey.String()]
		if poolTokenAccount.TokenAddress == tokenSwap.BaseTokenInfo.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		} else if poolTokenAccount.TokenAddress == tokenSwap.TokenInfo.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		}
	}

	trade.To = tokenSwap.To
	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.RaydiumConcentratedLiquidity
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	trade.BaseTokenAccountAddress = tokenSwap.BaseTokenInfo.TokenAccountAddress
	trade.TokenAccountAddress = tokenSwap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = tokenSwap.BaseTokenAmountInt
	trade.TokenAmountInt = tokenSwap.TokenAmountInt

	accountMetas := slice.Map[*ag_solanago.AccountMeta, solTypes.AccountMeta](swap.GetAccounts(), func(_ int, meta *ag_solanago.AccountMeta) solTypes.AccountMeta {
		return solTypes.AccountMeta{
			PubKey:     common.PublicKeyFromBytes(meta.PublicKey.Bytes()),
			IsSigner:   meta.IsSigner,
			IsWritable: meta.IsWritable,
		}
	})

	if len(accountMetas) > 9 {
		accountMetas = accountMetas[10:]
		clmmInfo := &types.CLMMPoolInfo{
			AmmConfig:         common.PublicKeyFromBytes(swap.GetAmmConfigAccount().PublicKey.Bytes()),
			PoolState:         solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swap.GetPoolStateAccount().PublicKey.Bytes())},
			InputVault:        solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swap.GetInputVaultAccount().PublicKey.Bytes())},
			OutputVault:       solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swap.GetOutputVaultAccount().PublicKey.Bytes())},
			ObservationState:  solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swap.GetObservationStateAccount().PublicKey.Bytes())},
			TokenProgram:      common.TokenProgramID,
			TokenProgram2022:  common.Token2022ProgramID,
			MemoProgram:       common.MemoProgramID,
			TickArray:         common.PublicKeyFromBytes(swap.GetTickArrayAccount().PublicKey.Bytes()),
			InputVaultMint:    common.PublicKeyFromString(tokenSwap.BaseTokenInfo.TokenAddress),
			OutputVaultMint:   common.PublicKeyFromString(tokenSwap.TokenInfo.TokenAddress),
			RemainingAccounts: accountMetas,
			TxHash:            decoder.dtx.TxHash,
		}

		fmt.Println("clmmInfo is:", clmmInfo)

		// 如果是卖单 数据库默认解析是买单
		if tokenSwap.Type == types.TradeTypeSell {
			clmmInfo.InputVault, clmmInfo.OutputVault = clmmInfo.OutputVault, clmmInfo.InputVault
		}

		// parse fee
		solClient := decoder.svcCtx.GetSolClient()
		accountInfo, err := solClient.GetAccountInfoWithConfig(decoder.ctx, clmmInfo.AmmConfig.String(), client.GetAccountInfoConfig{
			Commitment: rpc.CommitmentConfirmed,
		})
		if err != nil {
			return nil, err
		}
		ammConfig := amm_v3.AmmConfig{}
		if err := ammConfig.UnmarshalWithDecoder(bin.NewBorshDecoder(accountInfo.Data)); err != nil {
			return nil, err
		}

		clmmInfo.TradeFeeRate = ammConfig.TradeFeeRate

		trade.ClmmPoolInfoV1 = clmmInfo
		logx.Infof("decoder clmmInfo v1 tx hash: %v, clmm id: %v", decoder.dtx.TxHash, clmmInfo.PoolState.PubKey.String())
	}

	return trade, nil
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquiditySwapV2(swapV2 *amm_v3.SwapV2) (*types.TradeWithPair, error) {
	tokenSwap, err := decoder.decodeRaydiumConcentratedLiquidityTokenSwapV2(swapV2)
	if err != nil {
		return nil, err
	}

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = swapV2.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             swapV2.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    tokenSwap.BaseTokenInfo.TokenAddress,
		BaseTokenDecimal: tokenSwap.BaseTokenInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenSwap.TokenInfo.TokenAddress,
		TokenDecimal:     tokenSwap.TokenInfo.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}

	trade.Maker = swapV2.GetInputTokenAccountAccount().PublicKey.String()
	trade.Type = tokenSwap.Type
	trade.BaseTokenAmount = tokenSwap.BaseTokenAmount
	trade.TokenAmount = tokenSwap.TokenAmount
	trade.BaseTokenPriceUSD = decoder.dtx.SolPrice
	trade.TotalUSD = decimal.NewFromFloat(tokenSwap.BaseTokenAmount).Mul(decimal.NewFromFloat(decoder.dtx.SolPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err = fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return nil, err
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(tokenSwap.TokenAmount)).InexactFloat64() // price

	if !swapV2.GetInputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swapV2.GetInputVaultAccount().PublicKey.String()]
		if poolTokenAccount.TokenAddress == tokenSwap.BaseTokenInfo.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		} else if poolTokenAccount.TokenAddress == tokenSwap.TokenInfo.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		}
	}

	if !swapV2.GetOutputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swapV2.GetOutputVaultAccount().PublicKey.String()]
		if poolTokenAccount.TokenAddress == tokenSwap.BaseTokenInfo.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		} else if poolTokenAccount.TokenAddress == tokenSwap.TokenInfo.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		}
	}

	trade.To = tokenSwap.To
	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.RaydiumConcentratedLiquidity
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	trade.BaseTokenAccountAddress = tokenSwap.BaseTokenInfo.TokenAccountAddress
	trade.TokenAccountAddress = tokenSwap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = tokenSwap.BaseTokenAmountInt
	trade.TokenAmountInt = tokenSwap.TokenAmountInt

	accountMetas := slice.Map[*ag_solanago.AccountMeta, solTypes.AccountMeta](swapV2.GetAccounts(), func(_ int, meta *ag_solanago.AccountMeta) solTypes.AccountMeta {
		return solTypes.AccountMeta{
			PubKey:     common.PublicKeyFromBytes(meta.PublicKey.Bytes()),
			IsSigner:   meta.IsSigner,
			IsWritable: meta.IsWritable,
		}
	})

	if len(accountMetas) > 12 {
		accountMetas = accountMetas[13:]
		clmmInfo := &types.CLMMPoolInfo{
			AmmConfig:         common.PublicKeyFromBytes(swapV2.GetAmmConfigAccount().PublicKey.Bytes()),
			PoolState:         solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swapV2.GetPoolStateAccount().PublicKey.Bytes())},
			InputVault:        solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swapV2.GetInputVaultAccount().PublicKey.Bytes())},
			OutputVault:       solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swapV2.GetOutputVaultAccount().PublicKey.Bytes())},
			ObservationState:  solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(swapV2.GetObservationStateAccount().PublicKey.Bytes())},
			TokenProgram:      common.TokenProgramID,
			TokenProgram2022:  common.Token2022ProgramID,
			MemoProgram:       common.MemoProgramID,
			InputVaultMint:    common.PublicKeyFromBytes(swapV2.GetInputVaultMintAccount().PublicKey.Bytes()),
			OutputVaultMint:   common.PublicKeyFromBytes(swapV2.GetOutputVaultMintAccount().PublicKey.Bytes()),
			RemainingAccounts: accountMetas,
			TxHash:            decoder.dtx.TxHash,
		}
		// 如果是卖单 数据库默认解析是买单
		if tokenSwap.Type == types.TradeTypeSell {
			clmmInfo.InputVault, clmmInfo.OutputVault = clmmInfo.OutputVault, clmmInfo.InputVault
			clmmInfo.InputVaultMint, clmmInfo.OutputVaultMint = clmmInfo.OutputVaultMint, clmmInfo.InputVaultMint
		}

		// parse fee
		solClient := decoder.svcCtx.GetSolClient()
		accountInfo, err := solClient.GetAccountInfoWithConfig(decoder.ctx, clmmInfo.AmmConfig.String(), client.GetAccountInfoConfig{
			Commitment: rpc.CommitmentConfirmed,
		})
		if err != nil {
			return nil, err
		}
		ammConfig := amm_v3.AmmConfig{}
		if err := ammConfig.UnmarshalWithDecoder(bin.NewBorshDecoder(accountInfo.Data)); err != nil {
			return nil, err
		}

		clmmInfo.TradeFeeRate = ammConfig.TradeFeeRate
		trade.ClmmPoolInfoV2 = clmmInfo
		logx.Infof("decoder clmmInfo v2 tx hash: %v, clmm id: %v", decoder.dtx.TxHash, clmmInfo.PoolState.PubKey.String())
	}

	return trade, nil
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquidityIncreaseLiquidityV2(increaseLiquidityV2 *amm_v3.IncreaseLiquidityV2) (*types.TradeWithPair, error) {

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = increaseLiquidityV2.GetPoolStateAccount().PublicKey.String()

	account0 := increaseLiquidityV2.GetTokenAccount0Account()
	account0Info := decoder.dtx.TokenAccountMap[account0.PublicKey.String()]
	if account0Info == nil {
		err := errors.New("account0Info not found")
		return nil, err
	}
	account1 := increaseLiquidityV2.GetTokenAccount1Account()
	account1Info := decoder.dtx.TokenAccountMap[account1.PublicKey.String()]
	if account1Info == nil {
		err := errors.New("account1Info not found")
		return nil, err
	}

	baseAccount := account0Info
	tokenAccount := account1Info
	if account1.PublicKey.String() == TokenStrWrapSol {
		baseAccount, tokenAccount = account1Info, account0Info
	}

	if !account0.PublicKey.IsZero() {
		if account0Info.TokenAddress == baseAccount.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(account0Info.PostValue, -int32(account0Info.TokenDecimal)).InexactFloat64()
		} else if account0Info.TokenAddress == tokenAccount.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(account0Info.PostValue, -int32(account0Info.TokenDecimal)).InexactFloat64()
		}
	}

	if !account1.PublicKey.IsZero() {
		if account1Info.TokenAddress == account1Info.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		} else if account1Info.TokenAddress == account1Info.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		}
	}

	trade.Maker = increaseLiquidityV2.GetNftOwnerAccount().PublicKey.String()
	trade.Type = types.TradeRaydiumConcentratedLiquidityDecreaseLiquidity
	trade.To = increaseLiquidityV2.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             increaseLiquidityV2.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    baseAccount.TokenAddress,
		BaseTokenDecimal: baseAccount.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenAccount.TokenAddress,
		TokenDecimal:     tokenAccount.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}

	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.RaydiumConcentratedLiquidity
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	return trade, nil
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquidityDecreaseLiquidityV2(decreaseLiquidityV2 *amm_v3.DecreaseLiquidityV2) (*types.TradeWithPair, error) {
	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = decreaseLiquidityV2.GetPoolStateAccount().PublicKey.String()

	account0 := decreaseLiquidityV2.GetTokenVault0Account()
	account0Info := decoder.dtx.TokenAccountMap[account0.PublicKey.String()]
	if account0Info == nil {
		err := errors.New("account0Info not found")
		return nil, err
	}
	account1 := decreaseLiquidityV2.GetTokenVault1Account()
	account1Info := decoder.dtx.TokenAccountMap[account1.PublicKey.String()]
	if account1Info == nil {
		err := errors.New("account1Info not found")
		return nil, err
	}

	baseAccount := account0Info
	tokenAccount := account1Info
	if account1.PublicKey.String() == TokenStrWrapSol {
		baseAccount, tokenAccount = account1Info, account0Info
	}

	if !account0.PublicKey.IsZero() {
		if account0Info.TokenAddress == baseAccount.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(account0Info.PostValue, -int32(account0Info.TokenDecimal)).InexactFloat64()
		} else if account0Info.TokenAddress == tokenAccount.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(account0Info.PostValue, -int32(account0Info.TokenDecimal)).InexactFloat64()
		}
	}

	if !account1.PublicKey.IsZero() {
		if account1Info.TokenAddress == account1Info.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		} else if account1Info.TokenAddress == account1Info.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		}
	}

	trade.Maker = decreaseLiquidityV2.GetNftOwnerAccount().PublicKey.String()
	trade.Type = types.TradeRaydiumConcentratedLiquidityIncreaseLiquidity
	trade.To = decreaseLiquidityV2.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             decreaseLiquidityV2.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    baseAccount.TokenAddress,
		BaseTokenDecimal: baseAccount.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenAccount.TokenAddress,
		TokenDecimal:     tokenAccount.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}

	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.RaydiumConcentratedLiquidity
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	return trade, nil
}

func (decoder *ConcentratedLiquidityDecoder) DecodeRaydiumConcentratedLiquidityCreatePool(createPool *amm_v3.CreatePool) (*types.TradeWithPair, error) {
	fmt.Println("DecodeRaydiumConcentratedLiquidityCreatePool")
	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = createPool.GetPoolStateAccount().PublicKey.String()

	// Extract token mints from the CreatePool instruction
	tokenMint0 := createPool.GetTokenMint0Account().PublicKey.String()
	tokenMint1 := createPool.GetTokenMint1Account().PublicKey.String()

	// Determine which one is the base token (usually SOL or a major token)
	baseTokenAddr := tokenMint0
	tokenAddr := tokenMint1

	// Handle SOL token as base token
	if tokenMint1 == TokenStrWrapSol {
		baseTokenAddr, tokenAddr = tokenMint1, tokenMint0
	}

	trade.PairInfo = types.Pair{
		ChainId:       SolChainId,
		Addr:          createPool.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr: baseTokenAddr,
		TokenAddr:     tokenAddr,
		BlockTime:     decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:      decoder.dtx.BlockDb.Slot,
	}

	// Set the creator (poolCreator) as the maker
	trade.Maker = createPool.GetPoolCreatorAccount().PublicKey.String()

	// Use a specific trade type for CreatePool
	trade.Type = "create" // Using a string literal for clarity
	trade.To = createPool.GetPoolStateAccount().PublicKey.String()

	trade.Slot = decoder.dtx.BlockDb.Slot
	trade.BlockTime = decoder.dtx.BlockDb.BlockTime.Unix()
	trade.HashId = fmt.Sprintf("%v#%d", decoder.dtx.BlockDb.Slot, decoder.dtx.TxIndex)
	trade.TransactionIndex = decoder.dtx.TxIndex
	trade.SwapName = constants.RaydiumConcentratedLiquidity
	trade.PairInfo.Name = trade.SwapName

	// Store initial liquidity data if available
	if createPool.SqrtPriceX64 != nil {
		// Convert sqrtPriceX64 to a real price if needed
		logx.Infof("Created pool with sqrtPriceX64: %v", createPool.SqrtPriceX64)
	}

	// Create and populate ClmmPoolInfoV1
	accountMetas := slice.Map[*ag_solanago.AccountMeta, solTypes.AccountMeta](createPool.GetAccounts(), func(_ int, meta *ag_solanago.AccountMeta) solTypes.AccountMeta {
		return solTypes.AccountMeta{
			PubKey:     common.PublicKeyFromBytes(meta.PublicKey.Bytes()),
			IsSigner:   meta.IsSigner,
			IsWritable: meta.IsWritable,
		}
	})

	// Initialize ClmmPoolInfoV1 with data from createPool
	clmmInfo := &types.CLMMPoolInfo{
		AmmConfig:         common.PublicKeyFromBytes(createPool.GetAmmConfigAccount().PublicKey.Bytes()),
		PoolState:         solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(createPool.GetPoolStateAccount().PublicKey.Bytes())},
		InputVault:        solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(createPool.GetTokenVault0Account().PublicKey.Bytes())},
		OutputVault:       solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(createPool.GetTokenVault1Account().PublicKey.Bytes())},
		ObservationState:  solTypes.AccountMeta{PubKey: common.PublicKeyFromBytes(createPool.GetObservationStateAccount().PublicKey.Bytes())},
		TokenProgram:      common.TokenProgramID,
		TokenProgram2022:  common.Token2022ProgramID,
		MemoProgram:       common.MemoProgramID,
		InputVaultMint:    common.PublicKeyFromString(baseTokenAddr),
		OutputVaultMint:   common.PublicKeyFromString(tokenAddr),
		RemainingAccounts: accountMetas,
		TxHash:            decoder.dtx.TxHash,
		TickArray:         common.PublicKeyFromString(createPool.GetObservationStateAccount().PublicKey.String()),
	}

	// Process AMM config to get the trade fee rate
	solClient := decoder.svcCtx.GetSolClient()
	if solClient != nil {
		accountInfo, err := solClient.GetAccountInfoWithConfig(decoder.ctx, clmmInfo.AmmConfig.String(), client.GetAccountInfoConfig{
			Commitment: rpc.CommitmentConfirmed,
		})
		if err == nil {
			ammConfig := amm_v3.AmmConfig{}
			if err := ammConfig.UnmarshalWithDecoder(bin.NewBorshDecoder(accountInfo.Data)); err == nil {
				clmmInfo.TradeFeeRate = ammConfig.TradeFeeRate
				logx.Infof("Created pool with fee rate: %d", ammConfig.TradeFeeRate)
			}
		}
	}

	// Set the ClmmPoolInfoV1 field in the trade object
	trade.ClmmPoolInfoV1 = clmmInfo

	logx.Infof("CLMM Pool Creation: Successfully created pool with ID: %s, Base Token: %s, Quote Token: %s, TxHash: %s",
		trade.PairAddr, baseTokenAddr, tokenAddr, trade.TxHash)
	fmt.Println("trade is success:", trade)

	return trade, nil
}

func (decoder *ConcentratedLiquidityDecoder) decodeRaydiumConcentratedLiquidityTokenSwapV2(swapV2 *amm_v3.SwapV2) (swap *Swap, err error) {
	var fromTransfer *token.TransferParam
	var toTransfer *token.TransferParam

	var (
		accountKeys = decoder.dtx.Tx.AccountKeys
	)

	swap = &Swap{}

	fromTokenAccount := swapV2.GetInputTokenAccountAccount()
	fromTokenAccountInfo := decoder.dtx.TokenAccountMap[fromTokenAccount.PublicKey.String()]
	if fromTokenAccountInfo == nil {
		err = fmt.Errorf("fromTokenAccountInfo not found,tx hash: %v", decoder.dtx.TxHash)
		return
	}
	toTokenAccount := swapV2.GetOutputTokenAccountAccount()
	toTokenAccountInfo := decoder.dtx.TokenAccountMap[toTokenAccount.PublicKey.String()]
	if toTokenAccountInfo == nil {
		err = fmt.Errorf("toTokenAccountInfo not found,tx hash: %v", decoder.dtx.TxHash)
		return
	}

	if decoder.innerInstruction == nil {
		err = fmt.Errorf("innerInstruction not found,tx hash: %v", decoder.dtx.TxHash)
		return
	}

	for _, innerInstruction := range decoder.innerInstruction.Instructions {
		transfer, err := DecodeTokenTransfer(accountKeys, &innerInstruction)
		if err != nil {
			// err = fmt.Errorf("DecodeTokenTransfer err:%w", err)
			continue
		}
		if transfer.From.String() == fromTokenAccount.PublicKey.String() {
			fromTransfer = transfer
		} else if transfer.To.String() == toTokenAccount.PublicKey.String() {
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
	if !IsSwapTransfer(fromTransfer, toTransfer, decoder.dtx.TokenAccountMap) {
		err = errors.New("not swap transfer")
		return
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

func (decoder *ConcentratedLiquidityDecoder) decodeRaydiumConcentratedLiquidityTokenSwap(ammSwap *amm_v3.Swap) (swap *Swap, err error) {
	var fromTransfer *token.TransferParam
	var toTransfer *token.TransferParam

	var (
		accountKeys = decoder.dtx.Tx.AccountKeys
	)

	swap = &Swap{}

	fromTokenAccount := ammSwap.GetInputTokenAccountAccount()
	fromTokenAccountInfo := decoder.dtx.TokenAccountMap[fromTokenAccount.PublicKey.String()]
	if fromTokenAccountInfo == nil {
		err = fmt.Errorf("fromTokenAccountInfo not found,tx hash: %v", decoder.dtx.TxHash)
		return
	}
	toTokenAccount := ammSwap.GetOutputTokenAccountAccount()
	toTokenAccountInfo := decoder.dtx.TokenAccountMap[toTokenAccount.PublicKey.String()]
	if toTokenAccountInfo == nil {
		err = fmt.Errorf("toTokenAccountInfo not found,tx hash: %v", decoder.dtx.TxHash)
		return
	}

	if decoder.innerInstruction == nil {
		err = fmt.Errorf("innerInstruction not found,tx hash: %v", decoder.dtx.TxHash)
		return
	}

	for _, innerInstruction := range decoder.innerInstruction.Instructions {
		transfer, err := DecodeTokenTransfer(accountKeys, &innerInstruction)
		if err != nil {
			// err = fmt.Errorf("DecodeTokenTransfer err:%w", err)
			continue
		}
		if transfer.From.String() == fromTokenAccount.PublicKey.String() {
			fromTransfer = transfer
		} else if transfer.To.String() == toTokenAccount.PublicKey.String() {
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
	if !IsSwapTransfer(fromTransfer, toTransfer, decoder.dtx.TokenAccountMap) {
		err = errors.New("not swap transfer")
		return
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
