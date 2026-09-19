package block

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"dex/pkg/util"

	"dex/pkg/types"

	"dex/pkg/sol"

	"dex/pkg/raydium/cpmm/idl/generated/raydium_cp_swap"

	"dex/consumer/internal/svc"
	"dex/model/solmodel"
	"dex/pkg/constants"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/duke-git/lancet/v2/slice"
	ag_solanago "github.com/gagliardetto/solana-go"
	"github.com/near/borsh-go"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"golang.org/x/net/context"
)

var CPMMap = cmap.New[int]()

type CPMMDecoder struct {
	ctx                 context.Context
	svcCtx              *svc.ServiceContext
	dtx                 *DecodedTx
	compiledInstruction *solTypes.CompiledInstruction
	innerInstruction    *client.InnerInstruction
}

func (decoder *CPMMDecoder) DecodeCPMMLog() (*types.TradeWithPair, error) {

	for index, str := range decoder.dtx.Tx.Meta.LogMessages {
		if strings.HasPrefix(str, "Program CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C") {
			logIndex := index + 1
			dataIndex := index + 2
			if dataIndex >= len(decoder.dtx.Tx.Meta.LogMessages) {
				return nil, errors.New("index out of range")
			}

			data := strings.TrimPrefix(decoder.dtx.Tx.Meta.LogMessages[dataIndex], "Program data: ")

			if strings.Contains(decoder.dtx.Tx.Meta.LogMessages[logIndex], "Instruction: SwapBaseInput") {
				return decoder.DecodeSwapBaseInputLog(data)
			}

			if strings.Contains(decoder.dtx.Tx.Meta.LogMessages[logIndex], "Instruction: Withdraw") {
				return decoder.DecodeSwapBaseInputLog(data)
			}

			if strings.Contains(decoder.dtx.Tx.Meta.LogMessages[logIndex], "Instruction: SwapBaseInput") {
				return decoder.DecodeSwapBaseInputLog(data)
			}

			if strings.Contains(decoder.dtx.Tx.Meta.LogMessages[logIndex], "Instruction: SwapBaseInput") {
				return decoder.DecodeSwapBaseInputLog(data)
			}

		}
	}
	return nil, nil
}

func (decoder *CPMMDecoder) decodeCPMMLog(name string) []string {

	res := make([]string, 0)

	for index, str := range decoder.dtx.Tx.Meta.LogMessages {
		if strings.HasPrefix(str, "Program CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C invoke") {
			logIndex := index + 1
			dataIndex := index + 2
			if dataIndex >= len(decoder.dtx.Tx.Meta.LogMessages) {
				return res
			}

			if strings.Contains(decoder.dtx.Tx.Meta.LogMessages[logIndex], name) {
				data := strings.TrimPrefix(decoder.dtx.Tx.Meta.LogMessages[dataIndex], "Program data: ")
				res = append(res, data)
			}
		}
	}
	return res
}

func (decoder *CPMMDecoder) DecodeCPMMInstruction() (*types.TradeWithPair, error) {
	accountMetas := slice.Map[int, *ag_solanago.AccountMeta](decoder.compiledInstruction.Accounts, func(_ int, index int) *ag_solanago.AccountMeta {
		return &ag_solanago.AccountMeta{
			PublicKey: ag_solanago.PublicKeyFromBytes(decoder.dtx.Tx.AccountKeys[index].Bytes()),
			// no need
			IsWritable: false,
			IsSigner:   false,
		}
	})
	decodeInstruction, err := raydium_cp_swap.DecodeInstruction(accountMetas, decoder.compiledInstruction.Data)
	if err != nil {
		return nil, err
	}

	switch decodeInstruction.TypeID {
	case raydium_cp_swap.Instruction_SwapBaseInput:
		swapBaseInput := decodeInstruction.Impl.(*raydium_cp_swap.SwapBaseInput)
		return decoder.DecodeSwapBaseInput(swapBaseInput)
	case raydium_cp_swap.Instruction_SwapBaseOutput:
		swapBaseOutput := decodeInstruction.Impl.(*raydium_cp_swap.SwapBaseOutput)
		return decoder.DecodeSwapBaseOutput(swapBaseOutput)
	case raydium_cp_swap.Instruction_Withdraw:
		withdraw := decodeInstruction.Impl.(*raydium_cp_swap.Withdraw)
		return decoder.DecodeWithdraw(withdraw)
	case raydium_cp_swap.Instruction_Deposit:
		deposit := decodeInstruction.Impl.(*raydium_cp_swap.Deposit)
		return decoder.DecodeDeposit(deposit)
	default:
		return nil, ErrNotSupportInstruction
	}
}

func (decoder *CPMMDecoder) DecodeSwapBaseInput(swapBaseInput *raydium_cp_swap.SwapBaseInput) (*types.TradeWithPair, error) {

	tokenSwap := &Swap{}

	{
		cpmmLogs := decoder.decodeCPMMLog(raydium_cp_swap.InstructionIDToName(raydium_cp_swap.Instruction_SwapBaseInput))
		if len(cpmmLogs) == 0 {
			return nil, fmt.Errorf("decodeSwapBaseInputdecodeCPMMLog failed, tx hash: %v", decoder.dtx.TxHash)
		}

		// SwapEvent 定义了 SwapEvent 事件的结构
		type SwapEvent struct {
			PoolID            common.PublicKey `json:"pool_id"`             // 使用 string 来表示 Pubkey（Solana 的公钥）
			InputVaultBefore  uint64           `json:"input_vault_before"`  // 使用 uint64 表示 u64 类型
			OutputVaultBefore uint64           `json:"output_vault_before"` // 使用 uint64 表示 u64 类型
			InputAmount       uint64           `json:"input_amount"`        // 使用 uint64 表示 u64 类型
			OutputAmount      uint64           `json:"output_amount"`       // 使用 uint64 表示 u64 类型
			InputTransferFee  uint64           `json:"input_transfer_fee"`  // 使用 uint64 表示 u64 类型
			OutputTransferFee uint64           `json:"output_transfer_fee"` // 使用 uint64 表示 u64 类型
			BaseInput         bool             `json:"base_input"`          // 使用 bool 表示 bool 类型
		}

		swapEvent := &SwapEvent{}

		key := decoder.dtx.TxHash + "_" + raydium_cp_swap.InstructionIDToName(raydium_cp_swap.Instruction_SwapBaseInput) + swapBaseInput.GetPoolStateAccount().PublicKey.String()
		value, ok := CPMMap.Get(key)

		count := 0

		if ok && value > 0 && value < len(cpmmLogs) {
			count = value
		}

		for _, cpmmLog := range cpmmLogs {
			result, _ := base64.StdEncoding.DecodeString(cpmmLog)
			if err := borsh.Deserialize(swapEvent, result[8:]); err != nil {
				return nil, err
			}

			if swapBaseInput.GetPoolStateAccount().PublicKey.String() == swapEvent.PoolID.String() {
				if count > 0 {
					count--
					continue
				}

				value, ok := CPMMap.Get(key)
				if !ok {
					CPMMap.Set(key, 1)
				} else {
					CPMMap.Set(key, value+1)
				}
				break
			}
		}

		if swapBaseInput.GetPoolStateAccount().PublicKey.String() != swapEvent.PoolID.String() {
			return nil, fmt.Errorf("swap base input does not match pool state,tx hash: %v", decoder.dtx.TxHash)
		}

		fromTokenAccount := swapBaseInput.GetInputTokenAccountAccount()
		fromTokenAccountInfo := decoder.dtx.TokenAccountMap[fromTokenAccount.PublicKey.String()]
		if fromTokenAccountInfo == nil {
			err := errors.New("decodeSwapBaseInput fromTokenAccountInfo not found")
			return nil, err
		}
		toTokenAccount := swapBaseInput.GetOutputTokenAccountAccount()
		toTokenAccountInfo := decoder.dtx.TokenAccountMap[toTokenAccount.PublicKey.String()]
		if toTokenAccountInfo == nil {
			err := errors.New("decodeSwapBaseInput toTokenAccountInfo not found")
			return nil, err
		}

		if fromTokenAccountInfo.TokenAddress == TokenStrWrapSol {
			tokenSwap.BaseTokenInfo = fromTokenAccountInfo
			tokenSwap.TokenInfo = toTokenAccountInfo
			tokenSwap.Type = types.TradeTypeBuy

			tokenSwap.BaseTokenAmountInt = int64(swapEvent.InputAmount)
			tokenSwap.BaseTokenAmount = decimal.NewFromUint64(swapEvent.InputAmount).Div(decimal.NewFromFloat(math.Pow10(int(fromTokenAccountInfo.TokenDecimal)))).InexactFloat64()
			tokenSwap.TokenAmountInt = int64(swapEvent.OutputAmount)
			tokenSwap.TokenAmount = decimal.NewFromUint64(swapEvent.OutputAmount).Div(decimal.NewFromFloat(math.Pow10(int(toTokenAccountInfo.TokenDecimal)))).InexactFloat64()

			tokenSwap.To = toTokenAccountInfo.Owner
		} else if toTokenAccountInfo.TokenAddress == TokenStrWrapSol {
			tokenSwap.BaseTokenInfo = toTokenAccountInfo
			tokenSwap.TokenInfo = fromTokenAccountInfo
			tokenSwap.Type = types.TradeTypeSell

			tokenSwap.BaseTokenAmountInt = int64(swapEvent.OutputAmount)
			tokenSwap.BaseTokenAmount = decimal.NewFromUint64(swapEvent.OutputAmount).Div(decimal.NewFromFloat(math.Pow10(int(toTokenAccountInfo.TokenDecimal)))).InexactFloat64()
			tokenSwap.TokenAmountInt = int64(swapEvent.InputAmount)
			tokenSwap.TokenAmount = decimal.NewFromUint64(swapEvent.InputAmount).Div(decimal.NewFromFloat(math.Pow10(int(fromTokenAccountInfo.TokenDecimal)))).InexactFloat64()

			tokenSwap.To = fromTokenAccountInfo.Owner
		} else {
			return nil, ErrNotSupportWarp
		}
	}

	if tokenSwap.BaseTokenInfo == nil || tokenSwap.TokenInfo == nil {
		return nil, fmt.Errorf("decodeSwapBaseInput BaseTokenInfo or TokenInfo is nil, tx hash: %v", decoder.dtx.TxHash)
	}

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = swapBaseInput.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             swapBaseInput.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    tokenSwap.BaseTokenInfo.TokenAddress,
		BaseTokenDecimal: tokenSwap.BaseTokenInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenSwap.TokenInfo.TokenAddress,
		TokenDecimal:     tokenSwap.TokenInfo.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}

	trade.Maker = swapBaseInput.GetInputTokenAccountAccount().PublicKey.String()
	trade.Type = tokenSwap.Type
	trade.BaseTokenAmount = tokenSwap.BaseTokenAmount
	trade.TokenAmount = tokenSwap.TokenAmount
	trade.BaseTokenPriceUSD = decoder.dtx.SolPrice
	trade.TotalUSD = decimal.NewFromFloat(tokenSwap.BaseTokenAmount).Mul(decimal.NewFromFloat(decoder.dtx.SolPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err := fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return nil, err
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(tokenSwap.TokenAmount)).InexactFloat64() // price

	if !swapBaseInput.GetInputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swapBaseInput.GetInputVaultAccount().PublicKey.String()]
		if poolTokenAccount.TokenAddress == tokenSwap.BaseTokenInfo.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		} else if poolTokenAccount.TokenAddress == tokenSwap.TokenInfo.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		}
	}

	if !swapBaseInput.GetOutputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swapBaseInput.GetOutputVaultAccount().PublicKey.String()]
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
	trade.SwapName = constants.RaydiumCPMM
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	trade.BaseTokenAccountAddress = tokenSwap.BaseTokenInfo.TokenAccountAddress
	trade.TokenAccountAddress = tokenSwap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = tokenSwap.BaseTokenAmountInt
	trade.TokenAmountInt = tokenSwap.TokenAmountInt

	trade.CpmmPoolInfo = &solmodel.CpmmPoolInfo{
		AmmConfig: swapBaseInput.GetAmmConfigAccount().PublicKey.String(),
		// Authority:          swapBaseInput.GetAuthorityAccount().PublicKey.String(),
		PoolState:          swapBaseInput.GetPoolStateAccount().PublicKey.String(),
		InputVault:         swapBaseInput.GetInputVaultAccount().PublicKey.String(),
		OutputVault:        swapBaseInput.GetOutputVaultAccount().PublicKey.String(),
		InputTokenProgram:  swapBaseInput.GetInputTokenProgramAccount().PublicKey.String(),
		OutputTokenProgram: swapBaseInput.GetOutputTokenProgramAccount().PublicKey.String(),
		InputTokenMint:     swapBaseInput.GetInputTokenMintAccount().PublicKey.String(),
		OutputTokenMint:    swapBaseInput.GetOutputTokenMintAccount().PublicKey.String(),
		ObservationState:   swapBaseInput.GetObservationStateAccount().PublicKey.String(),
		TxHash:             decoder.dtx.TxHash,
	}

	if trade.CpmmPoolInfo.OutputTokenMint == constants.TokenStrWrapSol {
		trade.CpmmPoolInfo.InputVault, trade.CpmmPoolInfo.OutputVault = trade.CpmmPoolInfo.OutputVault, trade.CpmmPoolInfo.InputVault
		trade.CpmmPoolInfo.InputTokenProgram, trade.CpmmPoolInfo.OutputTokenProgram = trade.CpmmPoolInfo.OutputTokenProgram, trade.CpmmPoolInfo.InputTokenProgram
		trade.CpmmPoolInfo.InputTokenMint, trade.CpmmPoolInfo.OutputTokenMint = trade.CpmmPoolInfo.OutputTokenMint, trade.CpmmPoolInfo.InputTokenMint
	}

	return trade, nil
}

func (decoder *CPMMDecoder) DecodeSwapBaseOutput(swapBaseOutput *raydium_cp_swap.SwapBaseOutput) (*types.TradeWithPair, error) {

	tokenSwap := &Swap{}

	{
		cpmmLogs := decoder.decodeCPMMLog(raydium_cp_swap.InstructionIDToName(raydium_cp_swap.Instruction_SwapBaseOutput))
		if len(cpmmLogs) == 0 {
			return nil, fmt.Errorf("decodeSwapBaseOutput failed, tx hash: %v", decoder.dtx.TxHash)
		}

		type SwapEvent struct {
			PoolID            common.PublicKey `json:"pool_id"`             // 使用 string 来表示 Pubkey（Solana 的公钥）
			InputVaultBefore  uint64           `json:"input_vault_before"`  // 使用 uint64 表示 u64 类型
			OutputVaultBefore uint64           `json:"output_vault_before"` // 使用 uint64 表示 u64 类型
			InputAmount       uint64           `json:"input_amount"`        // 使用 uint64 表示 u64 类型
			OutputAmount      uint64           `json:"output_amount"`       // 使用 uint64 表示 u64 类型
			InputTransferFee  uint64           `json:"input_transfer_fee"`  // 使用 uint64 表示 u64 类型
			OutputTransferFee uint64           `json:"output_transfer_fee"` // 使用 uint64 表示 u64 类型
			BaseInput         bool             `json:"base_input"`          // 使用 bool 表示 bool 类型
		}

		swapEvent := &SwapEvent{}

		for _, cpmmLog := range cpmmLogs {
			result, _ := base64.StdEncoding.DecodeString(cpmmLog)
			if err := borsh.Deserialize(swapEvent, result[8:]); err != nil {
				return nil, err
			}

			if swapBaseOutput.GetPoolStateAccount().PublicKey.String() == swapEvent.PoolID.String() {
				break
			}
		}

		if swapBaseOutput.GetPoolStateAccount().PublicKey.String() != swapEvent.PoolID.String() {
			return nil, fmt.Errorf("swap base output does not match pool state,tx hash: %v", decoder.dtx.TxHash)
		}

		fromTokenAccount := swapBaseOutput.GetInputTokenAccountAccount()
		fromTokenAccountInfo := decoder.dtx.TokenAccountMap[fromTokenAccount.PublicKey.String()]
		if fromTokenAccountInfo == nil {
			err := errors.New("decodeSwapBaseInput fromTokenAccountInfo not found")
			return nil, err
		}
		toTokenAccount := swapBaseOutput.GetOutputTokenAccountAccount()
		toTokenAccountInfo := decoder.dtx.TokenAccountMap[toTokenAccount.PublicKey.String()]
		if toTokenAccountInfo == nil {
			err := errors.New("decodeSwapBaseInput toTokenAccountInfo not found")
			return nil, err
		}

		if fromTokenAccountInfo.TokenAddress == TokenStrWrapSol {
			tokenSwap.BaseTokenInfo = fromTokenAccountInfo
			tokenSwap.TokenInfo = toTokenAccountInfo
			tokenSwap.Type = types.TradeTypeBuy

			tokenSwap.BaseTokenAmountInt = int64(swapEvent.InputAmount)
			tokenSwap.BaseTokenAmount = decimal.NewFromUint64(swapEvent.InputAmount).Div(decimal.NewFromFloat(math.Pow10(int(fromTokenAccountInfo.TokenDecimal)))).InexactFloat64()
			tokenSwap.TokenAmountInt = int64(swapEvent.OutputAmount)
			tokenSwap.TokenAmount = decimal.NewFromUint64(swapEvent.OutputAmount).Div(decimal.NewFromFloat(math.Pow10(int(toTokenAccountInfo.TokenDecimal)))).InexactFloat64()

			tokenSwap.To = toTokenAccountInfo.Owner
		} else if toTokenAccountInfo.TokenAddress == TokenStrWrapSol {
			tokenSwap.BaseTokenInfo = toTokenAccountInfo
			tokenSwap.TokenInfo = fromTokenAccountInfo
			tokenSwap.Type = types.TradeTypeSell

			tokenSwap.BaseTokenAmountInt = int64(swapEvent.OutputAmount)
			tokenSwap.BaseTokenAmount = decimal.NewFromUint64(swapEvent.OutputAmount).Div(decimal.NewFromFloat(math.Pow10(int(toTokenAccountInfo.TokenDecimal)))).InexactFloat64()
			tokenSwap.TokenAmountInt = int64(swapEvent.InputAmount)
			tokenSwap.TokenAmount = decimal.NewFromUint64(swapEvent.InputAmount).Div(decimal.NewFromFloat(math.Pow10(int(fromTokenAccountInfo.TokenDecimal)))).InexactFloat64()

			tokenSwap.To = fromTokenAccountInfo.Owner
		} else {
			return nil, ErrNotSupportWarp
		}
	}

	if tokenSwap.BaseTokenInfo == nil || tokenSwap.TokenInfo == nil {
		return nil, fmt.Errorf("decodeSwapBaseOutput BaseTokenInfo or TokenInfo is nil, tx hash: %v", decoder.dtx.TxHash)
	}

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = swapBaseOutput.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             swapBaseOutput.GetPoolStateAccount().PublicKey.String(),
		BaseTokenAddr:    tokenSwap.BaseTokenInfo.TokenAddress,
		BaseTokenDecimal: tokenSwap.BaseTokenInfo.TokenDecimal,
		BaseTokenSymbol:  util.GetBaseToken(SolChainIdInt).Symbol,
		TokenAddr:        tokenSwap.TokenInfo.TokenAddress,
		TokenDecimal:     tokenSwap.TokenInfo.TokenDecimal,
		BlockTime:        decoder.dtx.BlockDb.BlockTime.Unix(),
		BlockNum:         decoder.dtx.BlockDb.Slot,
	}

	trade.Maker = swapBaseOutput.GetInputTokenAccountAccount().PublicKey.String()
	trade.Type = tokenSwap.Type
	trade.BaseTokenAmount = tokenSwap.BaseTokenAmount
	trade.TokenAmount = tokenSwap.TokenAmount
	trade.BaseTokenPriceUSD = decoder.dtx.SolPrice
	trade.TotalUSD = decimal.NewFromFloat(tokenSwap.BaseTokenAmount).Mul(decimal.NewFromFloat(decoder.dtx.SolPrice)).InexactFloat64() // total usd
	if trade.TokenAmount == 0 {
		err := fmt.Errorf("trade.TokenAmount is zero, tx:%v", trade.TxHash)
		return nil, err
	}

	trade.TokenPriceUSD = decimal.NewFromFloat(trade.TotalUSD).Div(decimal.NewFromFloat(tokenSwap.TokenAmount)).InexactFloat64() // price

	if !swapBaseOutput.GetInputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swapBaseOutput.GetInputVaultAccount().PublicKey.String()]
		if poolTokenAccount.TokenAddress == tokenSwap.BaseTokenInfo.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		} else if poolTokenAccount.TokenAddress == tokenSwap.TokenInfo.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(poolTokenAccount.PostValue, -int32(poolTokenAccount.TokenDecimal)).InexactFloat64()
		}
	}

	if !swapBaseOutput.GetOutputVaultAccount().PublicKey.IsZero() {
		poolTokenAccount := decoder.dtx.TokenAccountMap[swapBaseOutput.GetOutputVaultAccount().PublicKey.String()]
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
	trade.SwapName = constants.RaydiumCPMM
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	trade.BaseTokenAccountAddress = tokenSwap.BaseTokenInfo.TokenAccountAddress
	trade.TokenAccountAddress = tokenSwap.TokenInfo.TokenAccountAddress
	trade.BaseTokenAmountInt = tokenSwap.BaseTokenAmountInt
	trade.TokenAmountInt = tokenSwap.TokenAmountInt

	trade.CpmmPoolInfo = &solmodel.CpmmPoolInfo{
		AmmConfig: swapBaseOutput.GetAmmConfigAccount().PublicKey.String(),
		PoolState: swapBaseOutput.GetPoolStateAccount().PublicKey.String(),
		// Authority:          swapBaseOutput.GetAuthorityAccount().PublicKey.String(),
		InputVault:         swapBaseOutput.GetInputVaultAccount().PublicKey.String(),
		OutputVault:        swapBaseOutput.GetOutputVaultAccount().PublicKey.String(),
		InputTokenProgram:  swapBaseOutput.GetInputTokenProgramAccount().PublicKey.String(),
		OutputTokenProgram: swapBaseOutput.GetOutputTokenProgramAccount().PublicKey.String(),
		InputTokenMint:     swapBaseOutput.GetInputTokenMintAccount().PublicKey.String(),
		OutputTokenMint:    swapBaseOutput.GetOutputTokenMintAccount().PublicKey.String(),
		ObservationState:   swapBaseOutput.GetObservationStateAccount().PublicKey.String(),
		TxHash:             decoder.dtx.TxHash,
	}

	if trade.CpmmPoolInfo.OutputTokenMint == constants.TokenStrWrapSol {
		trade.CpmmPoolInfo.InputVault, trade.CpmmPoolInfo.OutputVault = trade.CpmmPoolInfo.OutputVault, trade.CpmmPoolInfo.InputVault
		trade.CpmmPoolInfo.InputTokenProgram, trade.CpmmPoolInfo.OutputTokenProgram = trade.CpmmPoolInfo.OutputTokenProgram, trade.CpmmPoolInfo.InputTokenProgram
		trade.CpmmPoolInfo.InputTokenMint, trade.CpmmPoolInfo.OutputTokenMint = trade.CpmmPoolInfo.OutputTokenMint, trade.CpmmPoolInfo.InputTokenMint
	}
	return trade, nil
}

func (decoder *CPMMDecoder) DecodeWithdraw(withdraw *raydium_cp_swap.Withdraw) (*types.TradeWithPair, error) {

	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = withdraw.GetPoolStateAccount().PublicKey.String()

	account0 := withdraw.GetVault0MintAccount()
	account0Info := decoder.dtx.TokenAccountMap[account0.PublicKey.String()]
	if account0Info == nil {
		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, account0.PublicKey.String())

		if mintInfo != nil {
			account0Info = &TokenAccount{
				TokenAddress: account0.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}
		}

		if account0Info == nil {
			err := fmt.Errorf("decodeWithdraw account0Info not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
	}
	account1 := withdraw.GetVault1MintAccount()
	account1Info := decoder.dtx.TokenAccountMap[account1.PublicKey.String()]
	if account1Info == nil {

		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, account1.PublicKey.String())

		if mintInfo != nil {
			account1Info = &TokenAccount{
				TokenAddress: account1.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}

		}

		if account1Info == nil {
			err := fmt.Errorf("decodeWithdraw account1Info not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
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
		if account1Info.TokenAddress == baseAccount.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		} else if account1Info.TokenAddress == tokenAccount.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		}
	}

	trade.Maker = withdraw.GetOwnerAccount().PublicKey.String()
	trade.Type = types.TradeRaydiumCPMMDecreaseLiquidity
	trade.To = withdraw.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             withdraw.GetPoolStateAccount().PublicKey.String(),
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
	trade.SwapName = constants.RaydiumCPMM
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	return trade, nil
}

func (decoder *CPMMDecoder) DecodeDeposit(deposit *raydium_cp_swap.Deposit) (*types.TradeWithPair, error) {
	trade := &types.TradeWithPair{}
	trade.ChainId = SolChainId
	trade.TxHash = decoder.dtx.TxHash
	trade.PairAddr = deposit.GetPoolStateAccount().PublicKey.String()

	account0 := deposit.GetVault0MintAccount()
	account0Info := decoder.dtx.TokenAccountMap[account0.PublicKey.String()]
	if account0Info == nil {
		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, account0.PublicKey.String())

		if mintInfo != nil {
			account0Info = &TokenAccount{
				TokenAddress: account0.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}
		}

		if account0Info == nil {
			err := fmt.Errorf("decodeDeposit account0Info not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
	}
	account1 := deposit.GetVault1MintAccount()
	account1Info := decoder.dtx.TokenAccountMap[account1.PublicKey.String()]
	if account1Info == nil {

		c := decoder.svcCtx.GetSolClient()
		mintInfo, _ := sol.GetTokenMintInfo(c, decoder.ctx, account1.PublicKey.String())

		if mintInfo != nil {
			account1Info = &TokenAccount{
				TokenAddress: account1.PublicKey.String(),
				TokenDecimal: mintInfo.Decimals,
			}

		}

		if account1Info == nil {
			err := fmt.Errorf("decodeDeposit account1Info not found, tx hash: %v", decoder.dtx.TxHash)
			return nil, err
		}
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
		if account1Info.TokenAddress == baseAccount.TokenAddress {
			trade.CurrentBaseTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		} else if account1Info.TokenAddress == tokenAccount.TokenAddress {
			trade.CurrentTokenInPoolAmount = decimal.New(account1Info.PostValue, -int32(account1Info.TokenDecimal)).InexactFloat64()
		}
	}

	trade.Maker = deposit.GetOwnerAccount().PublicKey.String()
	trade.Type = types.TradeRaydiumCPMMIncreaseLiquidity
	trade.To = deposit.GetPoolStateAccount().PublicKey.String()

	trade.PairInfo = types.Pair{
		ChainId:          SolChainId,
		Addr:             deposit.GetPoolStateAccount().PublicKey.String(),
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
	trade.SwapName = constants.RaydiumCPMM
	trade.PairInfo.Name = trade.SwapName
	trade.PairInfo.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
	trade.PairInfo.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
	return trade, nil
}

func (decoder *CPMMDecoder) DecodeSwapBaseInputLog(input string) (*types.TradeWithPair, error) {
	result, _ := base64.StdEncoding.DecodeString(input)

	// SwapEvent 定义了 SwapEvent 事件的结构
	type SwapEvent struct {
		PoolID            common.PublicKey `json:"pool_id"`             // 使用 string 来表示 Pubkey（Solana 的公钥）
		InputVaultBefore  uint64           `json:"input_vault_before"`  // 使用 uint64 表示 u64 类型
		OutputVaultBefore uint64           `json:"output_vault_before"` // 使用 uint64 表示 u64 类型
		InputAmount       uint64           `json:"input_amount"`        // 使用 uint64 表示 u64 类型
		OutputAmount      uint64           `json:"output_amount"`       // 使用 uint64 表示 u64 类型
		InputTransferFee  uint64           `json:"input_transfer_fee"`  // 使用 uint64 表示 u64 类型
		OutputTransferFee uint64           `json:"output_transfer_fee"` // 使用 uint64 表示 u64 类型
		BaseInput         bool             `json:"base_input"`          // 使用 bool 表示 bool 类型
	}

	swapEvent := &SwapEvent{}
	if err := borsh.Deserialize(swapEvent, result[8:]); err != nil {
		return nil, err
	}

	return nil, nil
}

func (decoder *CPMMDecoder) DecodeSwapBaseOutputLog(output *raydium_cp_swap.SwapBaseOutput) (*types.TradeWithPair, error) {
	return nil, nil
}

func (decoder *CPMMDecoder) DecodeWithdrawLog(withdraw *raydium_cp_swap.Withdraw) (*types.TradeWithPair, error) {
	return nil, nil
}

func (decoder *CPMMDecoder) DecodeDepositLog(deposit *raydium_cp_swap.Deposit) (*types.TradeWithPair, error) {
	return nil, nil
}
