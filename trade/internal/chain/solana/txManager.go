package solana

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"dex/pkg/constants"

	// "dex/pkg/raydium/clmm/idl/generated/amm_v3"
	"dex/pkg/trade"
	"dex/pkg/xcode"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"dex/pkg/sol"

	"dex/trade/internal/clients"

	bin "github.com/gagliardetto/binary"

	aSDK "github.com/gagliardetto/solana-go"
	alt "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
	"gorm.io/gorm"
)

type GasType int32

const (
	GasType_GasTypeSpeedInvalid GasType = 0
	GasType_NormalSpeed         GasType = 1
	GasType_FastSpeed           GasType = 2
	GasType_SuperFastSpeed      GasType = 3
)

type TxManager struct {
	FeeReceiver aSDK.PublicKey

	Client *ag_rpc.Client

	// jito related
	JitoClient   *ag_rpc.Client
	JitoUUID     string
	jitoTipFloor *JitoTipFloor
	RWLock       sync.RWMutex

	SimulateOnly bool
	DB           *gorm.DB
	rentFee      uint64
}

func NewTxManager(db *gorm.DB, rpcEndpoint, jitoEndPoint, uuid string, simulateOnly bool) (*TxManager, error) {
	var jitoClient *ag_rpc.Client
	if len(jitoEndPoint) > 0 {
		if 0 == len(uuid) {
			return nil, errors.New("uuid not configured but jito is configured")
		}
		headers := make(map[string]string)
		headers["Content-Type"] = "application/json"
		headers["uuid"] = uuid
		jitoClient = ag_rpc.NewWithHeaders(jitoEndPoint, headers)
	}

	tm := &TxManager{
		DB:           db,
		FeeReceiver:  aSDK.MustPublicKeyFromBase58(sol.FeeReceiver),
		Client:       ag_rpc.New(rpcEndpoint),
		JitoClient:   jitoClient,
		JitoUUID:     uuid,
		SimulateOnly: simulateOnly,
	}

	if len(jitoEndPoint) > 0 {
		tm.jitoTipFloor = &JitoTipFloor{
			Time:                        time.Now(),
			LandedTips25ThPercentile:    0.000001,
			LandedTips50ThPercentile:    0.00001,
			LandedTips75ThPercentile:    0.00004,
			LandedTips95ThPercentile:    0.006,
			LandedTips99ThPercentile:    0.018,
			EmaLandedTips50ThPercentile: 0.000014,
		}
		threading.GoSafe(func() {
			tm.CheckJitoFloorFee()
		})
	}
	threading.GoSafe(func() {
		tm.CheckRentFee()
	})

	return tm, nil
}

func (tm *TxManager) CreateMarketOrder(ctx context.Context, createMarketTx *trade.CreateMarketTx) (string, error) {
	logc.Debugf(ctx, "CreateMarketOrder:%#v", createMarketTx)
	in, err := convertCreateMarketTx(createMarketTx)
	if err != nil {
		fmt.Println("convertCreateMarketTx error:", err)
		return "", err
	}
	fmt.Println("input is:", in)

	return tm.CreateMarketTx(ctx, in)
}

func convertCreateMarketTx(in *trade.CreateMarketTx) (*CreateMarketTx, error) {
	fmt.Printf("convertCreateMarketTx input: UserWalletAddress='%s', InTokenCa='%s', OutTokenCa='%s', InTokenProgram='%s', OutTokenProgram='%s'\n",
		in.UserWalletAddress, in.InTokenCa, in.OutTokenCa, in.InTokenProgram, in.OutTokenProgram)

	//TODO: how to pass userwalletaddress?
	userWalletAccount, err := aSDK.PublicKeyFromBase58(in.UserWalletAddress)
	if err != nil {
		fmt.Printf("Error parsing UserWalletAddress '%s': %v\n", in.UserWalletAddress, err)
		return nil, err
	}
	inMint, err := aSDK.PublicKeyFromBase58(in.InTokenCa)
	if err != nil {
		fmt.Printf("Error parsing InTokenCa '%s': %v\n", in.InTokenCa, err)
		return nil, err
	}
	outMint, err := aSDK.PublicKeyFromBase58(in.OutTokenCa)
	if err != nil {
		fmt.Printf("Error parsing OutTokenCa '%s': %v\n", in.OutTokenCa, err)
		return nil, err
	}
	if in.InTokenProgram == "" {
		in.InTokenProgram = aSDK.TokenProgramID.String()
	}
	inTokenProgram, err := aSDK.PublicKeyFromBase58(in.InTokenProgram)
	if err != nil {
		fmt.Printf("Error parsing InTokenProgram '%s': %v\n", in.InTokenProgram, err)
		return nil, err
	}
	if in.OutTokenProgram == "" {
		in.OutTokenProgram = aSDK.TokenProgramID.String()
	}
	outTokenProgram, err := aSDK.PublicKeyFromBase58(in.OutTokenProgram)
	if err != nil {
		fmt.Printf("Error parsing OutTokenProgram '%s': %v\n", in.OutTokenProgram, err)
		return nil, err
	}

	return &CreateMarketTx{
		UserId:            in.UserId,
		ChainId:           in.ChainId,
		UserWalletId:      in.UserWalletId,
		UserWalletAccount: userWalletAccount,
		AmountIn:          in.AmountIn,
		IsAntiMev:         in.IsAntiMev,
		Slippage:          in.Slippage,
		IsAutoSlippage:    in.IsAutoSlippage,
		GasType:           in.GasType,
		TradePoolName:     in.TradePoolName,
		InDecimal:         in.InDecimal,
		OutDecimal:        in.OutDecimal,
		InMint:            inMint,
		OutMint:           outMint,
		PairAddr:          in.PairAddr,
		Price:             in.Price,
		UsePriceLimit:     in.UsePriceLimit,
		InTokenProgram:    inTokenProgram,
		OutTokenProgram:   outTokenProgram,
	}, nil
}

// CreateMarketOrder creates a market order transaction on Solana
// It supports both Raydium V4 and PumpFun trading pools
// Parameters:
//   - ctx: context for the request
//   - in: input parameters for creating market order
//   - simulateOnly: if true, only simulate the transaction without sending
//
// Returns the transaction signature or error
func (tm *TxManager) CreateMarketTx(ctx context.Context, in *CreateMarketTx) (string, error) {
	var instructions []aSDK.Instruction
	var err error
	switch in.TradePoolName {
	case constants.RaydiumV4, constants.RaydiumConcentratedLiquidity, constants.RaydiumCPMM, constants.PumpSwap:
		// Create Raydium market order instructions
		fmt.Println("CreateMarketTx: RaydiumV4, RaydiumConcentratedLiquidity, RaydiumCPMM, PumpSwap")
		instructions, err = tm.CreateMarketOrderDex(ctx, in)
		if nil != err {
			return "", err
		}
	case constants.PumpFun:
		// Create PumpFun market order instructions
		instructions, err = tm.CreateMarketOrder4Pumpfun(ctx, in)
		if nil != err {
			return "", err
		}
	case constants.PumpMeteora:
		// Create PumpMeteora (bonding-curve) market order instructions
		instructions, err = tm.CreateMarketOrder4PumpMeteora(ctx, in)
		if nil != err {
			return "", err
		}
	case constants.PumpMeteoraV2:
		// Create optimized PumpMeteora market order instructions
		instructions, err = tm.CreateMarketOrder4PumpMeteoraV2(ctx, in)
		if nil != err {
			return "", err
		}
	default:
		return "", fmt.Errorf("TradePoolName:%s not support", in.TradePoolName)
	}

	// Prepare TEE signing parameters
	teeSignPara := &clients.SignTransactionReq{
		OmniAccount: strconv.FormatUint(in.UserId, 10),
		WalletIndex: in.UserWalletId,
		Address:     in.UserWalletAccount.String(),
	}

	// Sign and send transaction
	txHash, err := tm.SignByTeeAndSend(ctx, instructions, teeSignPara, in.IsAntiMev)
	if err != nil {
		logc.Error(ctx, err)
		return "", err

	}

	fmt.Println("txHash is:", txHash)

	return txHash, nil
}

// CreateMarketOrderDex creates instructions for a market order on Raydium DEX
func (tm *TxManager) CreateMarketOrder4PumpSwap(ctx context.Context, in *CreateMarketTx) ([]aSDK.Instruction, error) {
	initiator := in.UserWalletAccount
	outMint := in.OutMint
	//需要创建2个ata账户，尽管token 账户可能存在，但是为了避免网络请求，不做判断，直接认为需要这个费用
	lamportCost := tm.rentFee * 2

	amtDecimal, err := decimal.NewFromString(in.AmountIn)
	if nil != err {
		return nil, err
	}

	amtDecimal = amtDecimal.Mul(decimal.NewFromInt(sol.Decimals2Value[in.InDecimal]))
	amtUint64 := uint64(amtDecimal.IntPart())

	// #1 - Compute Budget: SetComputeUnitPrice
	var cu uint32
	switch in.TradePoolName {
	case constants.PumpSwap:
		cu = sol.PumpSwapCU
	default:
		return nil, fmt.Errorf("trade pool :%s not support", in.TradePoolName)
	}
	instructions, lamportCostFee, err := tm.CreateGasAndJitoByGasFee(ctx, in.IsAntiMev, initiator, cu, sol.GasMODE[sol.GasType(in.GasType)])
	if nil != err {
		return nil, err
	}
	lamportCost += lamportCostFee

	// #3 - Associated Token Account Program: CreateIdempotent
	instructionNew, err := sol.CreateAtaIdempotent(initiator, initiator, in.InMint, in.InTokenProgram)
	if nil != err {
		return nil, err
	}
	instructions = append(instructions, instructionNew)

	inAta, _, err := sol.FindAssociatedTokenAddress(initiator, in.InMint, in.InTokenProgram)
	if nil != err {
		return nil, err
	}

	outAta, _, err := sol.FindAssociatedTokenAddress(initiator, outMint, in.OutTokenProgram)
	if nil != err {
		return nil, err
	}

	solBalanceInfo, err := tm.Client.GetBalance(ctx, initiator, ag_rpc.CommitmentProcessed)
	if nil != err {
		return nil, err
	}
	solBalance := solBalanceInfo.Value

	// #4 - System Program: Transfer, if in mint is wrapper sol
	// #5 - Token Program: SyncNative
	serviceFee := uint64(0)
	isBuy := in.InMint == aSDK.WrappedSol
	if isBuy {
		if solBalance < amtUint64 {
			return nil, xcode.SolBalanceNotEnough
		}

		// 计算服务费，计算根据 原始的数量
		serviceFeeDecimal := amtDecimal.Mul(sol.ServericeFeePercent)
		serviceFee = uint64(serviceFeeDecimal.IntPart())
		lamportCost += serviceFee + amtUint64

		instructionNew, err = system.NewTransferInstruction(amtUint64, initiator, inAta).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)

		instructionNew, err = token.NewSyncNativeInstruction(inAta).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)
	}
	if lamportCost > solBalance {
		return nil, xcode.SolGasNotEnough
	}

	// #6 - Associated Token Account Program: CreateIdempotent
	instructionNew, err = sol.CreateAtaIdempotent(initiator, initiator, outMint, in.OutTokenProgram)
	if nil != err {
		return nil, err
	}
	instructions = append(instructions, instructionNew)

	var minAmountOut, amountOut uint64
	// #7 - dex instruction
	switch in.TradePoolName {
	case constants.PumpSwap:
		instructionNew, minAmountOut, amountOut, err = createPumpSwapInstructionV2(ctx, tm.DB, in, amtUint64, isBuy, inAta, outAta, tm.Client)
		if nil != err {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("trade pool :%s not support", in.TradePoolName)
	}
	instructions = append(instructions, instructionNew)

	// #8 - Token Program: CloseAccount
	if in.InMint == aSDK.WrappedSol {
		instructionNew, err = token.NewCloseAccountInstruction(inAta, initiator, initiator, nil).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)
	}

	if outMint == aSDK.WrappedSol && amountOut > 0 {
		instructionNew, err = token.NewCloseAccountInstruction(outAta, initiator, initiator, nil).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)

		serviceFee = uint64(decimal.NewFromUint64(amountOut).Mul(sol.ServericeFeePercent).IntPart())
	}

	logc.Debugf(ctx, "CreateMarketOrderDex, initiator=%s, serviceFee=%d, AmountIn=%d, minAmountOut=%d, inMint=%s, outMint=%s",
		initiator, serviceFee, amtUint64, minAmountOut, in.InMint.String(), outMint.String())

	return instructions, nil
}

// CreateMarketOrderDex creates instructions for a market order on Raydium DEX
func (tm *TxManager) CreateMarketOrderDex(ctx context.Context, in *CreateMarketTx) ([]aSDK.Instruction, error) {
	initiator := in.UserWalletAccount
	outMint := in.OutMint
	//需要创建2个ata账户，尽管token 账户可能存在，但是为了避免网络请求，不做判断，直接认为需要这个费用
	lamportCost := tm.rentFee * 2

	amtDecimal, err := decimal.NewFromString(in.AmountIn)
	if nil != err {
		return nil, err
	}

	amtDecimal = amtDecimal.Mul(decimal.NewFromInt(sol.Decimals2Value[in.InDecimal]))
	amtUint64 := uint64(amtDecimal.IntPart())

	// #1 - Compute Budget: SetComputeUnitPrice
	var cu uint32
	switch in.TradePoolName {
	case constants.RaydiumCPMM:
		cu = sol.RaydiumCpmmSwapCu
	case constants.RaydiumConcentratedLiquidity:
		cu = sol.RaydiumClmmSwapCu
	case constants.RaydiumV4:
		cu = sol.RaydiumV4SwapCU
	case constants.PumpSwap:
		cu = sol.PumpSwapCU
	default:
		return nil, fmt.Errorf("trade pool :%s not support", in.TradePoolName)
	}
	instructions, lamportCostFee, err := tm.CreateGasAndJitoByGasFee(ctx, in.IsAntiMev, initiator, cu, sol.GasMODE[sol.GasType(in.GasType)])
	if nil != err {
		return nil, err
	}
	lamportCost += lamportCostFee

	// #3 - Associated Token Account Program: CreateIdempotent
	instructionNew, err := sol.CreateAtaIdempotent(initiator, initiator, in.InMint, in.InTokenProgram)
	if nil != err {
		return nil, err
	}
	instructions = append(instructions, instructionNew)

	inAta, _, err := sol.FindAssociatedTokenAddress(initiator, in.InMint, in.InTokenProgram)
	if nil != err {
		return nil, err
	}

	outAta, _, err := sol.FindAssociatedTokenAddress(initiator, outMint, in.OutTokenProgram)
	if nil != err {
		return nil, err
	}

	solBalanceInfo, err := tm.Client.GetBalance(ctx, initiator, ag_rpc.CommitmentProcessed)
	if nil != err {
		return nil, err
	}
	solBalance := solBalanceInfo.Value

	// #4 - System Program: Transfer, if in mint is wrapper sol
	// #5 - Token Program: SyncNative
	serviceFee := uint64(0)
	isBuy := in.InMint == aSDK.WrappedSol
	if isBuy {
		if solBalance < amtUint64 {
			return nil, xcode.SolBalanceNotEnough
		}

		// 计算服务费，计算根据 原始的数量
		serviceFeeDecimal := amtDecimal.Mul(sol.ServericeFeePercent)
		serviceFee = uint64(serviceFeeDecimal.IntPart())
		lamportCost += serviceFee + amtUint64

		instructionNew, err = system.NewTransferInstruction(amtUint64, initiator, inAta).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)

		instructionNew, err = token.NewSyncNativeInstruction(inAta).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)
	}
	if lamportCost > solBalance {
		return nil, xcode.SolGasNotEnough
	}

	// #6 - Associated Token Account Program: CreateIdempotent
	instructionNew, err = sol.CreateAtaIdempotent(initiator, initiator, outMint, in.OutTokenProgram)
	if nil != err {
		return nil, err
	}
	instructions = append(instructions, instructionNew)

	var minAmountOut, amountOut uint64
	// #7 - dex instruction
	switch in.TradePoolName {
	case constants.RaydiumCPMM:
		instructionNew, minAmountOut, amountOut, err = createRaydiumCpmmInstruction(ctx, tm.DB, in, amtUint64, isBuy, inAta, outAta, tm.Client)
		if nil != err {
			return nil, err
		}
	case constants.RaydiumConcentratedLiquidity:
		instructionNew, minAmountOut, amountOut, err = createRaydiumClmmInstruction(ctx, tm.DB, in, amtUint64, isBuy, inAta, outAta, tm.Client)
		if nil != err {
			return nil, err
		}
	case constants.PumpSwap:
		instructionNew, minAmountOut, amountOut, err = createPumpSwapInstructionV2(ctx, tm.DB, in, amtUint64, isBuy, inAta, outAta, tm.Client)
		if nil != err {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("trade pool :%s not support", in.TradePoolName)
	}
	instructions = append(instructions, instructionNew)

	// #8 - Token Program: CloseAccount
	if in.InMint == aSDK.WrappedSol {
		instructionNew, err = token.NewCloseAccountInstruction(inAta, initiator, initiator, nil).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)
	}

	if outMint == aSDK.WrappedSol && amountOut > 0 {
		instructionNew, err = token.NewCloseAccountInstruction(outAta, initiator, initiator, nil).ValidateAndBuild()
		if nil != err {
			return nil, err
		}
		instructions = append(instructions, instructionNew)

		serviceFee = uint64(decimal.NewFromUint64(amountOut).Mul(sol.ServericeFeePercent).IntPart())
	}

	logc.Debugf(ctx, "CreateMarketOrderDex, initiator=%s, serviceFee=%d, AmountIn=%d, minAmountOut=%d, inMint=%s, outMint=%s",
		initiator, serviceFee, amtUint64, minAmountOut, in.InMint.String(), outMint.String())

	return instructions, nil
}

func (tm *TxManager) simulate(ctx context.Context, tx *aSDK.Transaction) error {
	simOut, err := tm.Client.SimulateTransactionWithOpts(ctx, tx, &ag_rpc.SimulateTransactionOpts{
		Commitment: ag_rpc.CommitmentProcessed,
	})
	if err != nil {
		logc.Error(ctx, err)
		return err
	}
	if nil != simOut && nil != simOut.Value && simOut.Value.Err != nil {
		logs := strings.Join(simOut.Value.Logs, " ")
		logc.Infof(ctx, "simOut failed , logs %s , err:%v", logs, simOut.Value.Err)
		return errors.New(logs)
	}
	return nil
}

func GetMulTokenBalance(ctx context.Context, cli *ag_rpc.Client, accounts ...aSDK.PublicKey) ([]uint64, error) {
	res, err := cli.GetMultipleAccountsWithOpts(ctx, accounts, &ag_rpc.GetMultipleAccountsOpts{
		Commitment: ag_rpc.CommitmentProcessed,
	})
	if err != nil {
		return nil, err
	}

	var amounts []uint64
	for i := range res.Value {
		if res.Value[i] == nil || res.Value[i].Data == nil {
			continue
		}
		var coinBalance token.Account
		if err = bin.NewBinDecoder(res.Value[i].Data.GetBinary()).Decode(&coinBalance); nil != err {
			return nil, err
		}

		amounts = append(amounts, coinBalance.Amount)
	}

	return amounts, nil
}

func (tm *TxManager) SignByTeeAndSend(ctx context.Context, insts []aSDK.Instruction, signTransactionReq *clients.SignTransactionReq, isAntiMev bool) (string, error) {
	tx, err := tm.Sign(ctx, insts, signTransactionReq)
	if err != nil {
		return "", err
	}
	if tm.SimulateOnly {
		err = tm.simulate(ctx, tx)
		if err != nil {
			return "", err
		}

		return tx.Signatures[0].String(), err
	}

	// Only route via Jito when a Jito client is actually configured; otherwise
	// (e.g. devnet with Jito disabled) fall back to a normal RPC send instead
	// of dereferencing a nil JitoClient.
	if isAntiMev && tm.JitoClient != nil {
		sig, err := tm.SendViaJitoRetry(ctx, tx)
		if nil != err {
			txData, _ := json.Marshal(tx)
			logc.Infof(ctx, "SendTransaction tx:%s failed:%s", string(txData), err.Error())
			return "", err
		}
		return sig, nil
	}
	if isAntiMev && tm.JitoClient == nil {
		logc.Infof(ctx, "anti-mev requested but Jito is disabled; sending via normal RPC")
	}

	sig, err := tm.Client.SendTransactionWithOpts(ctx, tx, ag_rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: ag_rpc.CommitmentProcessed,
	})
	if nil != err {
		txData, _ := json.Marshal(tx)
		logc.Infof(ctx, "SendTransaction tx:%s failed:%s", string(txData), err.Error())
		return "", err
	}

	return sig.String(), nil
}

func (tm *TxManager) Sign(ctx context.Context, insts []aSDK.Instruction, signTransactionReq *clients.SignTransactionReq) (*aSDK.Transaction, error) {
	// Get latest blockhash with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := tm.Client.GetLatestBlockhash(timeoutCtx, ag_rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	feePayer, err := aSDK.PublicKeyFromBase58(signTransactionReq.Address)
	if err != nil {
		return nil, err
	}

	tx, err := aSDK.NewTransaction(insts, resp.Value.Blockhash, aSDK.TransactionPayer(feePayer))
	if err != nil {
		return nil, err
	}

	//TODO: 私钥使用环境变量
	privateKeyBase64 := os.Getenv("PRIVATE_KEY")
	if privateKeyBase64 == "" {
		return nil, fmt.Errorf("SOLANA_PRIVATE_KEY environment variable not set")
	}

	// Decode base64 private key
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %v", err)
	}

	// Create ed25519 private key
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: expected %d, got %d", ed25519.PrivateKeySize, len(privateKeyBytes))
	}

	privateKey := ed25519.PrivateKey(privateKeyBytes)

	// Sign the transaction message
	messageContent, err := tx.Message.MarshalBinary()
	if err != nil {
		logc.Error(ctx, err)
		return nil, xcode.InternalError
	}

	signature := ed25519.Sign(privateKey, messageContent)

	// Set signatures for all required signers
	signerKeys := tx.Message.AccountKeys[0:tx.Message.Header.NumRequiredSignatures]
	tx.Signatures = make([]aSDK.Signature, len(signerKeys))

	// Copy signature to all required signers (assuming single signer for now)
	for i := range signerKeys {
		copy(tx.Signatures[i][:], signature)
	}

	return tx, nil
}

// BuildSignAndSend builds the swap, signs it with the server-held key
// (PRIVATE_KEY env, base64 ed25519) and submits it. Used for triggered limit
// orders where no user wallet is present to sign. The fee payer must be the
// server wallet (createMarketTx.UserWalletAddress) or the signature won't match.
func (tm *TxManager) BuildSignAndSend(ctx context.Context, createMarketTx *trade.CreateMarketTx) (string, error) {
	in, err := convertCreateMarketTx(createMarketTx)
	if err != nil {
		return "", err
	}
	return tm.CreateMarketTx(ctx, in)
}

// ServerWalletAddress derives the public address of the server-held signing key
// (PRIVATE_KEY env). Returns an error when the key is absent or malformed, so
// callers can fail an order with a clear reason instead of an invalid tx.
func ServerWalletAddress() (string, error) {
	privateKeyBase64 := os.Getenv("PRIVATE_KEY")
	if privateKeyBase64 == "" {
		return "", fmt.Errorf("PRIVATE_KEY environment variable not set")
	}
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode PRIVATE_KEY: %v", err)
	}
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid PRIVATE_KEY length: expected %d, got %d", ed25519.PrivateKeySize, len(privateKeyBytes))
	}
	pub := ed25519.PrivateKey(privateKeyBytes).Public().(ed25519.PublicKey)
	return aSDK.PublicKeyFromBytes(pub).String(), nil
}

// BuildUnsignedTransaction builds an unsigned transaction for third-party wallet signing
func (tm *TxManager) BuildUnsignedTransaction(ctx context.Context, createMarketTx *trade.CreateMarketTx) (string, error) {
	logx.WithContext(ctx).Infof("Building unsigned transaction for third-party wallet signing")

	in, err := convertCreateMarketTx(createMarketTx)
	if err != nil {
		return "", err
	}

	// Get instructions without signing
	var instructions []aSDK.Instruction
	logx.WithContext(ctx).Infof("BuildUnsignedTransaction: TradePoolName=%s, PairAddr=%s, InMint=%s, OutMint=%s",
		in.TradePoolName, in.PairAddr, in.InMint.String(), in.OutMint.String())
	switch in.TradePoolName {
	case constants.PumpFun:
		logx.WithContext(ctx).Infof("Using PumpFun pool for transaction")
		instructions, err = tm.CreateMarketOrder4Pumpfun(ctx, in)
		if err != nil {
			return "", err
		}
	case constants.PumpMeteora:
		logx.WithContext(ctx).Infof("Using PumpMeteora pool for transaction")
		instructions, err = tm.CreateMarketOrder4PumpMeteora(ctx, in)
		if err != nil {
			return "", err
		}
	case constants.PumpMeteoraV2:
		logx.WithContext(ctx).Infof("Using PumpMeteoraV2 (optimized) pool for transaction")
		instructions, err = tm.CreateMarketOrder4PumpMeteoraV2(ctx, in)
		if err != nil {
			return "", err
		}
	case constants.RaydiumV4, constants.RaydiumConcentratedLiquidity, constants.RaydiumCPMM:
		logx.WithContext(ctx).Infof("Using Raydium DEX pool (type: %s) for transaction", in.TradePoolName)
		instructions, err = tm.CreateMarketOrderDex(ctx, in)
		if err != nil {
			return "", err
		}
	case constants.PumpSwap:
		logx.WithContext(ctx).Infof("✅ Using PumpSwap pool for transaction - PairAddr: %s, TokenMint: %s",
			in.PairAddr, in.OutMint.String())
		instructions, err = tm.CreateMarketOrder4PumpSwap(ctx, in)
		if err != nil {
			logx.WithContext(ctx).Errorf("CreateMarketOrder4PumpSwap failed: %v", err)
			return "", err
		}
		logx.WithContext(ctx).Infof("✅ PumpSwap transaction instructions created successfully, count: %d", len(instructions))
	default:
		logx.WithContext(ctx).Errorf("Unsupported TradePoolName: %s", in.TradePoolName)
		return "", fmt.Errorf("TradePoolName:%s not support", in.TradePoolName)
	}

	// Get latest blockhash with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := tm.Client.GetLatestBlockhash(timeoutCtx, ag_rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	// Create unsigned transaction
	feePayer, err := aSDK.PublicKeyFromBase58(createMarketTx.UserWalletAddress)
	if err != nil {
		return "", err
	}

	tx, err := aSDK.NewTransaction(instructions, resp.Value.Blockhash, aSDK.TransactionPayer(feePayer))
	if err != nil {
		return "", err
	}

	// Initialize empty signatures for the transaction
	numSigners := int(tx.Message.Header.NumRequiredSignatures)
	tx.Signatures = make([]aSDK.Signature, numSigners)

	// Serialize the complete transaction (with empty signatures)
	txData, err := tx.MarshalBinary()
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to serialize transaction: %v", err)
		return "", err
	}

	fmt.Println("unsigned transaction is:*********************", base64.StdEncoding.EncodeToString(txData))

	// Return the serialized transaction as base64
	return base64.StdEncoding.EncodeToString(txData), nil
}

// BuildUnsignedPoolTransaction builds an unsigned pool creation transaction for third-party wallet signing.
// Returns (unsignedTxBase64, poolStateAddress, error).
func (tm *TxManager) BuildUnsignedPoolTransaction(ctx context.Context, createPoolTx *trade.CreatePoolTx) (string, string, error) {
	logx.WithContext(ctx).Infof("🏊 Building unsigned pool creation transaction for third-party wallet signing")

	// Get instructions for pool creation
	instructions, poolAddress, err := tm.CreatePoolInstructions(ctx, createPoolTx)
	if err != nil {
		return "", "", err
	}

	// Get latest blockhash with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := tm.Client.GetLatestBlockhash(timeoutCtx, ag_rpc.CommitmentFinalized)
	if err != nil {
		return "", "", fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	// Create unsigned transaction
	feePayer, err := aSDK.PublicKeyFromBase58(createPoolTx.UserWalletAddress)
	if err != nil {
		return "", "", err
	}

	tx, err := aSDK.NewTransaction(instructions, resp.Value.Blockhash, aSDK.TransactionPayer(feePayer))
	if err != nil {
		return "", "", err
	}

	// Initialize empty signatures for the transaction
	numSigners := int(tx.Message.Header.NumRequiredSignatures)
	tx.Signatures = make([]aSDK.Signature, numSigners)

	// Serialize the complete transaction (with empty signatures)
	txData, err := tx.MarshalBinary()
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to serialize pool creation transaction: %v", err)
		return "", "", err
	}

	logx.WithContext(ctx).Infof("✅ Pool creation transaction serialized successfully, size: %d bytes", len(txData))
	// Return the serialized transaction as base64
	return base64.StdEncoding.EncodeToString(txData), poolAddress, nil
}

// BuildUnsignedAddLiquidityTransaction builds an unsigned transaction for adding liquidity to a pool
// It returns a base64-encoded serialized transaction that can be signed by a wallet
func (tm *TxManager) BuildUnsignedAddLiquidityTransaction(ctx context.Context, addLiquidityTx *trade.AddLiquidityTx) (string, string, error) {
	logx.Infof("🚀 BuildUnsignedAddLiquidityTransaction: Starting transaction build process")
	logx.Infof("📋 Request details - PoolId=%s, BaseToken=%d, BaseAmount=%s, OtherAmountMax=%s, TokenA=%s, TokenB=%s",
		addLiquidityTx.PoolId, addLiquidityTx.BaseToken, addLiquidityTx.BaseAmount, addLiquidityTx.OtherAmountMax, addLiquidityTx.TokenAAddress, addLiquidityTx.TokenBAddress)

	// Create instructions for adding liquidity
	logx.Infof("🔍 Step 1: Creating instructions for adding liquidity")
	instructions, positionMintKeypair, err := tm.CreateAddLiquidityInstructions(ctx, addLiquidityTx)
	if err != nil {
		logx.Errorf("❌ CreateAddLiquidityInstructions error: %v", err)
		return "", "", fmt.Errorf("failed to create add liquidity instructions: %v", err)
	}
	logx.Infof("✅ Successfully created %d instructions", len(instructions))

	// Get latest blockhash
	logx.Infof("🔍 Step 2: Getting latest blockhash")
	latestBlockHash, err := tm.Client.GetLatestBlockhash(ctx, ag_rpc.CommitmentFinalized)
	if err != nil {
		logx.Errorf("❌ GetLatestBlockhash error: %v", err)
		return "", "", fmt.Errorf("failed to get latest blockhash: %v", err)
	}
	logx.Infof("✅ Latest blockhash: %s", latestBlockHash.Value.Blockhash.String())

	// Create transaction with instructions
	logx.Infof("🔍 Step 3: Creating transaction with instructions")

	// === 加载 ALT，解决交易过大问题 ===
	altPDA := aSDK.MustPublicKeyFromBase58("Dqgo35VeFKqzqJteZpDPtTLCFEknicbhfEXWeMYoXB7m")
	_ = tm.PrintALTTable(ctx, altPDA.String())

	// 1. 拉取 ALT 账户内容
	accountInfo, err := tm.Client.GetAccountInfo(ctx, altPDA)
	if err != nil {
		logx.Errorf("❌ Get ALT account info error: %v", err)
		return "", "", fmt.Errorf("failed to get ALT account info: %v", err)
	}
	altState, err := alt.DecodeAddressLookupTableState(accountInfo.GetBinary())
	if err != nil {
		logx.Errorf("❌ Decode ALT state error: %v", err)
		return "", "", fmt.Errorf("failed to get ALT account info: %v", err)
	}

	// 2. 构造 addressTables 参数
	addressTables := map[aSDK.PublicKey]aSDK.PublicKeySlice{
		altPDA: altState.Addresses,
	}

	// 3. 构建 v0 交易
	tx, err := aSDK.NewTransaction(
		instructions,
		latestBlockHash.Value.Blockhash,
		aSDK.TransactionPayer(aSDK.MustPublicKeyFromBase58(addLiquidityTx.UserWalletAddress)),
		aSDK.TransactionAddressTables(addressTables),
	)
	if err != nil {
		logx.Errorf("❌ Create transaction error: %v", err)
		return "", "", fmt.Errorf("failed to create transaction: %v", err)
	}
	logx.Infof("✅ Transaction created successfully (with ALT)")

	// === 后端部分签名 positionMint ===
	logx.Infof("🔍 Step 4: Backend partial signing positionMint")

	// 找到 positionMint 在 AccountKeys 中的位置
	positionMintIndex := -1
	for i, key := range tx.Message.AccountKeys {
		if key.String() == positionMintKeypair.PublicKey().String() {
			positionMintIndex = i
			break
		}
	}

	if positionMintIndex == -1 {
		logx.Errorf("❌ PositionMint not found in transaction account keys")
		return "", "", fmt.Errorf("positionMint not found in transaction")
	}

	logx.Infof("✅ PositionMint found at index %d", positionMintIndex)

	// 使用 positionMintKeypair 签名交易消息
	messageContent, err := tx.Message.MarshalBinary()
	if err != nil {
		logx.Errorf("❌ Failed to marshal transaction message: %v", err)
		return "", "", fmt.Errorf("failed to marshal transaction message: %v", err)
	}

	positionMintSignature := ed25519.Sign(ed25519.PrivateKey(positionMintKeypair.PrivateKey[:]), messageContent)

	// Initialize empty signatures for the transaction
	numSigners := int(tx.Message.Header.NumRequiredSignatures)
	tx.Signatures = make([]aSDK.Signature, numSigners)
	for i := range tx.Signatures {
		tx.Signatures[i] = aSDK.Signature{} // 64字节全0
	}

	// 设置 positionMint 的签名
	if positionMintIndex < len(tx.Signatures) {
		copy(tx.Signatures[positionMintIndex][:], positionMintSignature)
		logx.Infof("✅ PositionMint signature set at index %d", positionMintIndex)
	}

	// === 日志：打印交易结构 ===
	logx.Infof("📝 Transaction debug info:")
	logx.Infof("   - NumRequiredSignatures: %d", tx.Message.Header.NumRequiredSignatures)
	logx.Infof("   - Signatures length: %d", len(tx.Signatures))
	for i, sig := range tx.Signatures {
		logx.Infof("   - Signature[%d]: %x", i, sig)
	}
	logx.Infof("   - FeePayer: %s", tx.Message.AccountKeys[0].String())
	logx.Infof("   - AccountKeys count: %d", len(tx.Message.AccountKeys))
	//打印出所有账户
	for _, key := range tx.Message.AccountKeys {
		logx.Infof("📊 AccountKey: %s", key.String())
	}

	//打印出message.addrestableLookups账户
	logx.Infof("📊 AddressTableLookups count: %d", len(tx.Message.AddressTableLookups))
	for _, lookup := range tx.Message.AddressTableLookups {
		logx.Infof("📊 AddressTableLookup: %s", lookup.AccountKey.String())
	}
	logx.Infof("   - RecentBlockhash: %s", latestBlockHash.Value.Blockhash.String())
	logx.Infof("   - Instructions count: %d", len(instructions))
	// txSize := len(tx.Message.AccountKeys)*32 + len(tx.Signatures)*64 + 1024
	// logx.Infof("📊 Transaction size estimate: %d bytes (max: 1232)", txSize)
	// if txSize > 1232 {
	// 	logx.Error("❌ Transaction too large")
	// 	return "", errors.New("transaction too large")
	// }

	for i, meta := range tx.Message.AccountKeys {
		isSigner := i < int(tx.Message.Header.NumRequiredSignatures)
		logx.Infof("AccountKey[%d]: %s, isSigner: %v", i, meta.String(), isSigner)
	}

	// Serialize transaction to base64
	logx.Infof("🔍 Step 4: Serializing transaction to base64")
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		logx.Errorf("❌ Marshal transaction error: %v", err)
		return "", "", fmt.Errorf("failed to serialize transaction: %v", err)
	}
	logx.Infof("✅ Transaction serialized, length: %d bytes", len(txBytes))

	// Return base64 encoded transaction
	encodedTx := base64.StdEncoding.EncodeToString(txBytes)
	positionMintPrivateKeyBase64 := base64.StdEncoding.EncodeToString(positionMintKeypair.PrivateKey)

	logx.Infof("✅ Unsigned add liquidity transaction created, length: %d bytes", len(encodedTx))
	return encodedTx, positionMintPrivateKeyBase64, nil
}

// CreateAddLiquidityInstructions creates the instructions to add liquidity to a Raydium CLMM pool
func (tm *TxManager) CreateAddLiquidityInstructions(ctx context.Context, addLiquidityTx *trade.AddLiquidityTx) ([]aSDK.Instruction, *aSDK.Wallet, error) {
	logx.Infof("🚀 CreateAddLiquidityInstructions: Starting instruction creation for pool %s", addLiquidityTx.PoolId)

	// 1. Get wallet public key
	logx.Infof("🔍 Step 1: Getting wallet public key for %s", addLiquidityTx.UserWalletAddress)
	userWallet, err := aSDK.PublicKeyFromBase58(addLiquidityTx.UserWalletAddress)
	if err != nil {
		logx.Errorf("❌ Invalid user wallet address: %v", err)
		return nil, nil, fmt.Errorf("invalid user wallet address: %v", err)
	}
	logx.Infof("✅ User wallet public key: %s", userWallet.String())

	// 2. Get pool state key
	logx.Infof("🔍 Step 2: Getting pool state key for %s", addLiquidityTx.PoolId)
	poolState, err := aSDK.PublicKeyFromBase58(addLiquidityTx.PoolId)
	if err != nil {
		logx.Errorf("❌ Invalid pool ID: %v", err)
		return nil, nil, fmt.Errorf("invalid pool ID: %v", err)
	}
	logx.Infof("✅ Pool state key: %s", poolState.String())

	// 3. Get token mint addresses
	logx.Infof("🔍 Step 3: Getting token mint addresses")
	tokenAMint, err := aSDK.PublicKeyFromBase58(addLiquidityTx.TokenAAddress)
	if err != nil {
		logx.Errorf("❌ Invalid token A address: %v", err)
		return nil, nil, fmt.Errorf("invalid token A address: %v", err)
	}
	logx.Infof("✅ Token A mint: %s", tokenAMint.String())

	tokenBMint, err := aSDK.PublicKeyFromBase58(addLiquidityTx.TokenBAddress)
	if err != nil {
		logx.Errorf("❌ Invalid token B address: %v", err)
		return nil, nil, fmt.Errorf("invalid token B address: %v", err)
	}
	logx.Infof("✅ Token B mint: %s", tokenBMint.String())

	// 4. Convert amounts to decimals
	logx.Infof("🔍 Step 4: Converting amounts to decimals")
	baseAmount, err := decimal.NewFromString(addLiquidityTx.BaseAmount)
	if err != nil {
		logx.Errorf("❌ Invalid base amount: %v", err)
		return nil, nil, fmt.Errorf("invalid base amount: %v", err)
	}
	logx.Infof("✅ Base amount: %s", baseAmount.String())

	otherAmountMax, err := decimal.NewFromString(addLiquidityTx.OtherAmountMax)
	if err != nil {
		logx.Errorf("❌ Invalid other amount max: %v", err)
		return nil, nil, fmt.Errorf("invalid other amount max: %v", err)
	}
	logx.Infof("✅ Other amount max: %s", otherAmountMax.String())

	// 5. Get the pool information from the on-chain state
	logx.Infof("🔍 Step 5: Getting pool information")
	poolInfo, err := tm.getPoolInfo(ctx, poolState)
	if err != nil {
		logx.Errorf("❌ Failed to get pool info: %v", err)
		return nil, nil, fmt.Errorf("failed to get pool info: %v", err)
	}
	logx.Infof("✅ Pool program ID: %s", poolInfo.ProgramID)

	// 6. Get the token vaults, observation state, and tick arrays
	logx.Infof("🔍 Step 6: Getting pool accounts (token vaults, observation state)")
	// tokenVaultA, tokenVaultB, observationState, err := tm.getPoolAccounts(ctx, poolState, tokenAMint, tokenBMint, aSDK.MustPublicKeyFromBase58(poolInfo.ProgramID))
	// if err != nil {
	// 	logx.Errorf("❌ Failed to get pool accounts: %v", err)
	// 	return nil, fmt.Errorf("failed to get pool accounts: %v", err)
	// }

	tokenMint0, tokenMint1, tokenVault0, tokenVault1, observationState, tickSpacing, err := tm.GetPoolVaultsByPoolStateAddr(ctx, poolState)
	if err != nil {
		logx.Errorf("❌ Failed to get pool vaults: %v", err)
		return nil, nil, fmt.Errorf("failed to get pool vaults: %v", err)
	}
	logx.Infof("✅ Pool token mint 0: %s", tokenMint0.String())
	logx.Infof("✅ Pool token mint 1: %s", tokenMint1.String())
	logx.Infof("✅ Token vault 0: %s", tokenVault0.String())
	logx.Infof("✅ Token vault 1: %s", tokenVault1.String())
	logx.Infof("✅ Observation state: %s", observationState.String())

	// // 7. Find or create associated token accounts for user
	// logx.Infof("🔍 Step 7: Finding or creating associated token accounts for user")
	// userTokenAccountA, err := tm.findOrCreateATAInstruction(ctx, tokenAMint, userWallet)
	// if err != nil {
	// 	logx.Errorf("❌ Failed to find/create token account A: %v", err)
	// 	return nil, nil, fmt.Errorf("failed to find/create token account A: %v", err)
	// }
	// logx.Infof("✅ User token account A: %s", userTokenAccountA.String())

	// userTokenAccountB, err := tm.findOrCreateATAInstruction(ctx, tokenBMint, userWallet)
	// if err != nil {
	// 	logx.Errorf("❌ Failed to find/create token account B: %v", err)
	// 	return nil, nil, fmt.Errorf("failed to find/create token account B: %v", err)
	// }
	// logx.Infof("✅ User token account B: %s", userTokenAccountB.String())

	// Determine token order based on pool's actual token mints
	// In Raydium CLMM, tokens are sorted by address (token0 < token1)
	var tokenVaultA, tokenVaultB aSDK.PublicKey
	var userTokenA, userTokenB aSDK.PublicKey
	var tokenAMintSorted, tokenBMintSorted aSDK.PublicKey

	// Use pool's token mints to determine the correct order
	if bytes.Compare(tokenMint0[:], tokenMint1[:]) < 0 {
		// tokenMint0 < tokenMint1, so tokenMint0 is token0
		tokenVaultA = tokenVault0
		tokenVaultB = tokenVault1
		tokenAMintSorted = tokenMint0
		tokenBMintSorted = tokenMint1

		// Find user token accounts based on the sorted order
		if bytes.Compare(tokenAMint[:], tokenBMint[:]) < 0 {
			// Frontend tokenA is token0, tokenB is token1
			userTokenA, err = tm.findOrCreateATAInstruction(ctx, tokenAMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account A: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account A: %v", err)
			}
			userTokenB, err = tm.findOrCreateATAInstruction(ctx, tokenBMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account B: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account B: %v", err)
			}
		} else {
			// Frontend tokenB is token0, tokenA is token1
			userTokenA, err = tm.findOrCreateATAInstruction(ctx, tokenBMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account A: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account A: %v", err)
			}
			userTokenB, err = tm.findOrCreateATAInstruction(ctx, tokenAMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account B: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account B: %v", err)
			}
		}
	} else {
		// tokenMint1 < tokenMint0, so tokenMint1 is token0
		tokenVaultA = tokenVault1
		tokenVaultB = tokenVault0
		tokenAMintSorted = tokenMint1
		tokenBMintSorted = tokenMint0

		// Find user token accounts based on the sorted order
		if bytes.Compare(tokenAMint[:], tokenBMint[:]) < 0 {
			// Frontend tokenA is token0, tokenB is token1
			userTokenA, err = tm.findOrCreateATAInstruction(ctx, tokenAMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account A: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account A: %v", err)
			}
			userTokenB, err = tm.findOrCreateATAInstruction(ctx, tokenBMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account B: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account B: %v", err)
			}
		} else {
			// Frontend tokenB is token0, tokenA is token1
			userTokenA, err = tm.findOrCreateATAInstruction(ctx, tokenBMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account A: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account A: %v", err)
			}
			userTokenB, err = tm.findOrCreateATAInstruction(ctx, tokenAMint, userWallet)
			if err != nil {
				logx.Errorf("❌ Failed to find/create token account B: %v", err)
				return nil, nil, fmt.Errorf("failed to find/create token account B: %v", err)
			}
		}
	}

	logx.Infof("✅ Token vault A (sorted): %s", tokenVaultA.String())
	logx.Infof("✅ Token vault B (sorted): %s", tokenVaultB.String())
	logx.Infof("✅ User token A (sorted): %s (mint: %s)", userTokenA.String(), tokenAMintSorted.String())
	logx.Infof("✅ User token B (sorted): %s (mint: %s)", userTokenB.String(), tokenBMintSorted.String())
	logx.Infof("✅ Token mint A (sorted): %s", tokenAMintSorted.String())
	logx.Infof("✅ Token mint B (sorted): %s", tokenBMintSorted.String())

	if tokenAMintSorted == aSDK.WrappedSol {
		logx.Infof("[WSOL] userTokenA is WSOL ATA: %s", userTokenA.String())
	}
	if tokenBMintSorted == aSDK.WrappedSol {
		logx.Infof("[WSOL] userTokenB is WSOL ATA: %s", userTokenB.String())
	}

	// --- 计算amount0Max/amount1Max/baseTokenIndex逻辑提前 ---
	// 计算 amounts for the OpenPosition instruction
	var amount0Max, amount1Max uint64
	var baseTokenIndex uint8
	var frontendToken0 aSDK.PublicKey
	var frontendAmount0, frontendAmount1 decimal.Decimal
	if bytes.Compare(tokenAMint[:], tokenBMint[:]) < 0 {
		frontendToken0 = tokenAMint
		frontendAmount0 = baseAmount
		frontendAmount1 = otherAmountMax
	} else {
		frontendToken0 = tokenBMint
		frontendAmount0 = otherAmountMax
		frontendAmount1 = baseAmount
	}
	if bytes.Compare(tokenMint0[:], tokenMint1[:]) < 0 {
		if bytes.Compare(frontendToken0[:], tokenMint0[:]) == 0 {
			amount0Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 0
		} else {
			amount0Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 1
		}
	} else {
		if bytes.Compare(frontendToken0[:], tokenMint1[:]) == 0 {
			amount0Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 0
		} else {
			amount0Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 1
		}
	}

	// 8. Check and handle WSOL wrapping if needed
	logx.Infof("🔍 Step 8: Checking for WSOL wrapping requirements")

	// Get user's SOL balance
	solBalanceInfo, err := tm.Client.GetBalance(ctx, userWallet, ag_rpc.CommitmentProcessed)
	if err != nil {
		logx.Errorf("❌ Failed to get SOL balance: %v", err)
		return nil, nil, fmt.Errorf("failed to get SOL balance: %v", err)
	}
	solBalance := solBalanceInfo.Value
	logx.Infof("✅ User SOL balance: %d lamports", solBalance)

	// Check if either token is WSOL and needs wrapping
	var wsolInstructions []aSDK.Instruction
	var wsolAmount0, wsolAmount1 uint64

	// Check token A for WSOL
	fmt.Println("tokenAMintSorted", tokenAMintSorted)
	if tokenAMintSorted == aSDK.WrappedSol {
		logx.Infof("🔍 Token A is WSOL, checking if wrapping is needed")
		wsolAmount0 = amount0Max
		if wsolAmount0 > 0 {
			logx.Infof("📦 WSOL amount 0 needed: %d lamports", wsolAmount0)
			if solBalance < wsolAmount0 {
				logx.Errorf("❌ Insufficient SOL balance for WSOL wrapping")
				return nil, nil, fmt.Errorf("insufficient SOL balance for WSOL wrapping: need %d, have %d", wsolAmount0, solBalance)
			}

			// --- Ensure ATA exists, create if not ---
			accountInfo, err := tm.Client.GetAccountInfo(ctx, userTokenA)
			if err != nil || accountInfo.Value == nil {
				logx.Infof("🛠️ userTokenA does not exist, adding CreateAssociatedTokenAccount instruction")
				createATAInst := associatedtokenaccount.NewCreateInstruction(
					userWallet,      // payer
					userWallet,      // wallet (owner)
					aSDK.WrappedSol, // mint
				)
				inst, err := createATAInst.ValidateAndBuild()
				if err != nil {
					logx.Errorf("❌ Failed to create CreateAssociatedTokenAccount instruction: %v", err)
					return nil, nil, fmt.Errorf("failed to create CreateAssociatedTokenAccount instruction: %v", err)
				}
				wsolInstructions = append(wsolInstructions, inst)
				logx.Infof("✅ Added CreateAssociatedTokenAccount instruction for WSOL A")
			}
			// Add System Transfer instruction
			fmt.Println("userTokenA is:", userTokenA)
			transferInst, err := system.NewTransferInstruction(wsolAmount0, userWallet, userTokenA).ValidateAndBuild()
			if err != nil {
				logx.Errorf("❌ Failed to create System Transfer instruction: %v", err)
				return nil, nil, fmt.Errorf("failed to create System Transfer instruction: %v", err)
			}
			wsolInstructions = append(wsolInstructions, transferInst)
			logx.Infof("✅ Added System Transfer instruction for WSOL A")

			// --- Add logs before SyncNative ---
			accountInfo, err = tm.Client.GetAccountInfo(ctx, userTokenA)
			if err != nil {
				logx.Errorf("❌ Failed to fetch account info for userTokenA: %v", err)
			} else if accountInfo.Value != nil && accountInfo.Value.Data != nil {
				data := accountInfo.Value.Data.GetBinary()
				if len(data) >= 64 {
					mint := aSDK.PublicKeyFromBytes(data[0:32])
					owner := aSDK.PublicKeyFromBytes(data[32:64])
					logx.Infof("📝 userTokenA account info: mint=%s, owner=%s, program=%s", mint.String(), owner.String(), accountInfo.Value.Owner.String())
				}
			}
			logx.Infof("📝 About to call SyncNative on account: %s, using program: %s", userTokenA.String(), aSDK.TokenProgramID.String())

			// Add SyncNative instruction
			syncNativeInst, err := token.NewSyncNativeInstruction(userTokenA).ValidateAndBuild()
			if err != nil {
				logx.Errorf("❌ Failed to create SyncNative instruction: %v", err)
				return nil, nil, fmt.Errorf("failed to create SyncNative instruction: %v", err)
			}
			wsolInstructions = append(wsolInstructions, syncNativeInst)
			logx.Infof("✅ Added SyncNative instruction for WSOL A")
		}
	}

	// Check token B for WSOL
	fmt.Println("tokenBMintSorted", tokenBMintSorted)
	if tokenBMintSorted == aSDK.WrappedSol {
		logx.Infof("🔍 Token B is WSOL, checking if wrapping is needed")
		wsolAmount1 = amount1Max
		if wsolAmount1 > 0 {
			logx.Infof("📦 WSOL amount 1 needed: %d lamports", wsolAmount1)
			totalWsolNeeded := wsolAmount0 + wsolAmount1
			if solBalance < totalWsolNeeded {
				logx.Errorf("❌ Insufficient SOL balance for WSOL wrapping")
				return nil, nil, fmt.Errorf("insufficient SOL balance for WSOL wrapping: need %d, have %d", totalWsolNeeded, solBalance)
			}

			// --- Ensure ATA exists, create if not ---
			accountInfo, err := tm.Client.GetAccountInfo(ctx, userTokenB)
			if err != nil || accountInfo.Value == nil {
				logx.Infof("🛠️ userTokenB does not exist, adding CreateAssociatedTokenAccount instruction")
				createATAInst := associatedtokenaccount.NewCreateInstruction(
					userWallet,      // payer
					userWallet,      // wallet (owner)
					aSDK.WrappedSol, // mint
				)
				inst, err := createATAInst.ValidateAndBuild()
				if err != nil {
					logx.Errorf("❌ Failed to create CreateAssociatedTokenAccount instruction: %v", err)
					return nil, nil, fmt.Errorf("failed to create CreateAssociatedTokenAccount instruction: %v", err)
				}
				wsolInstructions = append(wsolInstructions, inst)
				logx.Infof("✅ Added CreateAssociatedTokenAccount instruction for WSOL B")
			}
			// Add System Transfer instruction
			transferInst, err := system.NewTransferInstruction(wsolAmount1, userWallet, userTokenB).ValidateAndBuild()
			if err != nil {
				logx.Errorf("❌ Failed to create System Transfer instruction: %v", err)
				return nil, nil, fmt.Errorf("failed to create System Transfer instruction: %v", err)
			}
			wsolInstructions = append(wsolInstructions, transferInst)
			logx.Infof("✅ Added System Transfer instruction for WSOL B")

			// --- Add logs before SyncNative ---
			accountInfo, err = tm.Client.GetAccountInfo(ctx, userTokenB)
			if err != nil {
				logx.Errorf("❌ Failed to fetch account info for userTokenA: %v", err)
			} else if accountInfo.Value != nil && accountInfo.Value.Data != nil {
				data := accountInfo.Value.Data.GetBinary()
				if len(data) >= 64 {
					mint := aSDK.PublicKeyFromBytes(data[0:32])
					owner := aSDK.PublicKeyFromBytes(data[32:64])
					logx.Infof("📝 userTokenA account info: mint=%s, owner=%s, program=%s", mint.String(), owner.String(), accountInfo.Value.Owner.String())
				}
			}
			logx.Infof("📝 About to call SyncNative on account: %s, using program: %s", userTokenA.String(), aSDK.TokenProgramID.String())

			// Add SyncNative instruction
			syncNativeInst, err := token.NewSyncNativeInstruction(userTokenB).ValidateAndBuild()
			if err != nil {
				logx.Errorf("❌ Failed to create SyncNative instruction: %v", err)
				return nil, nil, fmt.Errorf("failed to create SyncNative instruction: %v", err)
			}
			wsolInstructions = append(wsolInstructions, syncNativeInst)
			logx.Infof("✅ Added SyncNative instruction for WSOL B")
		}
	}

	if len(wsolInstructions) > 0 {
		logx.Infof("✅ Added %d WSOL wrapping instructions", len(wsolInstructions))
	} else {
		logx.Infof("✅ No WSOL wrapping required")
	}

	// 8. Find position mint and associated accounts
	logx.Infof("🔍 Step 8: Creating position mint and associated accounts")
	positionMintKeypair := aSDK.NewWallet()
	positionMint := positionMintKeypair.PublicKey()
	logx.Infof("✅ Position mint: %s", positionMint.String())

	positionPDA, _, err := aSDK.FindProgramAddress(
		[][]byte{
			[]byte("position"),
			positionMint[:],
		},
		aSDK.MustPublicKeyFromBase58(poolInfo.ProgramID),
	)
	if err != nil {
		logx.Errorf("❌ Failed to derive position PDA: %v", err)
		return nil, nil, fmt.Errorf("failed to derive position PDA: %v", err)
	}
	logx.Infof("✅ Position PDA: %s", positionPDA.String())

	// 9. Find tick arrays for lower and upper tick
	logx.Infof("🔍 Step 9: Finding tick arrays for lower tick %d and upper tick %d",
		addLiquidityTx.TickLower, addLiquidityTx.TickUpper)
	tickArrayLower, tickArrayUpper, tickArrayLowerStartIndex, tickArrayUpperStartIndex, tickLower, tickUpper, err := tm.findTickArrays(
		ctx,
		poolState,
		tickSpacing,
		int32(addLiquidityTx.TickLower),
		int32(addLiquidityTx.TickUpper),
		aSDK.MustPublicKeyFromBase58(poolInfo.ProgramID),
	)
	if err != nil {
		logx.Errorf("❌ Failed to find tick arrays: %v", err)
		return nil, nil, fmt.Errorf("failed to find tick arrays: %v", err)
	}
	logx.Infof("✅ Tick array lower: %s", tickArrayLower.String())
	logx.Infof("✅ Tick array upper: %s", tickArrayUpper.String())

	protocol_position_pda, _, err := DeriveProtocolPositionPDA(poolState, int32(tickLower), int32(tickUpper), aSDK.MustPublicKeyFromBase58(poolInfo.ProgramID))
	if err != nil {
		logx.Errorf("❌ Failed to derive protocol position PDA: %v", err)
		return nil, nil, fmt.Errorf("failed to derive protocol position PDA: %v", err)
	}
	logx.Infof("✅ Protocol position PDA: %s", protocol_position_pda.String())

	// 10. Calculate the amounts for the OpenPosition instruction
	logx.Infof("🔍 Step 10: Calculating amounts for the OpenPosition instruction")

	// Calculate amounts based on pool's token order

	// Determine which frontend token corresponds to which pool token
	// var frontendToken0 aSDK.PublicKey
	// var frontendAmount0, frontendAmount1 decimal.Decimal
	//
	if bytes.Compare(tokenAMint[:], tokenBMint[:]) < 0 {
		// Frontend tokenA < tokenB
		frontendToken0 = tokenAMint
		frontendAmount0 = baseAmount
		frontendAmount1 = otherAmountMax
	} else {
		// Frontend tokenB < tokenA
		frontendToken0 = tokenBMint
		frontendAmount0 = otherAmountMax
		frontendAmount1 = baseAmount
	}

	// Map frontend tokens to pool tokens
	if bytes.Compare(tokenMint0[:], tokenMint1[:]) < 0 {
		// Pool token0 < token1
		if bytes.Compare(frontendToken0[:], tokenMint0[:]) == 0 {
			// Frontend token0 matches pool token0
			amount0Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 0
		} else {
			// Frontend token0 matches pool token1
			amount0Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 1
		}
	} else {
		// Pool token1 < token0
		if bytes.Compare(frontendToken0[:], tokenMint1[:]) == 0 {
			// Frontend token0 matches pool token1
			amount0Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 0
		} else {
			// Frontend token0 matches pool token0
			amount0Max = uint64(frontendAmount0.Mul(decimal.New(1, 9)).IntPart())
			amount1Max = uint64(frontendAmount1.Mul(decimal.New(1, 9)).IntPart())
			baseTokenIndex = 1
		}
	}

	logx.Infof("✅ Frontend token order: tokenA=%s, tokenB=%s", tokenAMint.String(), tokenBMint.String())
	logx.Infof("✅ Pool token order: token0=%s, token1=%s", tokenMint0.String(), tokenMint1.String())
	logx.Infof("✅ Frontend amounts: baseAmount=%s, otherAmountMax=%s", baseAmount.String(), otherAmountMax.String())
	logx.Infof("✅ Adjusted base token index: %d", baseTokenIndex)
	logx.Infof("✅ Amount 0 max (token0): %d", amount0Max)
	logx.Infof("✅ Amount 1 max (token1): %d", amount1Max)

	// 11. Build OpenPositionV2 instruction
	logx.Infof("🔍 Step 11: Building OpenPositionV2 instruction")
	openPositionInst := tm.buildOpenPositionInstruction(
		userWallet,
		positionMint,
		poolState,
		tokenAMintSorted,
		tokenBMintSorted,
		userTokenA,
		userTokenB,
		tokenVaultA,
		tokenVaultB,
		int32(tickLower),
		int32(tickUpper),
		tickArrayLower,
		tickArrayUpper,
		positionPDA,
		observationState,
		baseTokenIndex,
		amount0Max,
		amount1Max,
		tickArrayLowerStartIndex,
		tickArrayUpperStartIndex,
		protocol_position_pda,
	)
	logx.Infof("✅ OpenPositionV2 instruction built")

	// Compute the minimum rent exemption for a token account
	rentExemption := uint64(2039280) // Approximate minimum for a token account
	logx.Infof("📊 Rent exemption for token account: %d lamports", rentExemption)

	// 12. Combine all instructions
	logx.Infof("🔍 Step 12: Combining all instructions")

	// Create account instruction parameters
	logx.Infof("📋 Creating account instruction parameters:")
	logx.Infof("   - Rent exemption: %d lamports", rentExemption)
	logx.Infof("   - Space: %d bytes", 82)
	logx.Infof("   - From: %s", userWallet.String())
	logx.Infof("   - To: %s", positionMint.String())
	logx.Infof("   - Owner: %s", aSDK.TokenProgramID.String())

	// Create the instructions
	// instructions := []aSDK.Instruction{
	// Add instruction to create position mint account if needed
	// system.NewCreateAccountInstruction(
	// 	rentExemption,       // lamports
	// 	uint64(82),          // space - minimum size for token mint account
	// 	aSDK.TokenProgramID, //
	// 	userWallet,          // from
	// 	positionMint,        // to
	// ).Build(),

	// // Add instruction to initialize the mint account
	// token.NewInitializeMintInstruction(
	// 	0, // decimals
	// 	userWallet, // mint_authority
	// 	userWallet, // freeze_authority
	// 	positionMint, // mint account
	// 	aSDK.SysVarRentPubkey, // rent sysvar
	// ).Build(),

	// 	// Add the OpenPositionV2 instruction
	// openPositionInst,
	// }

	instructions := []aSDK.Instruction{}

	// Add WSOL wrapping instructions first (if needed)
	if len(wsolInstructions) > 0 {
		logx.Infof("📦 Adding %d WSOL wrapping instructions", len(wsolInstructions))
		instructions = append(instructions, wsolInstructions...)
	}

	// Add the OpenPositionV2 instruction
	instructions = append(instructions, openPositionInst)

	logx.Infof("✅ Successfully combined %d instructions", len(instructions))
	return instructions, positionMintKeypair, nil
}

// 派生 protocolPositionPDA
func DeriveProtocolPositionPDA(poolState aSDK.PublicKey, tickLower, tickUpper int32, programID aSDK.PublicKey) (aSDK.PublicKey, uint8, error) {
	seed := [][]byte{
		[]byte("position"),
		poolState[:],
		int32ToBytesBigEndian(tickLower),
		int32ToBytesBigEndian(tickUpper),
	}
	pda, bump, err := aSDK.FindProgramAddress(seed, programID)
	return pda, bump, err
}

// 辅助函数：int32 转大端字节序
func int32ToBytesBigEndian(i int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(i))
	return b
}

// Helper methods required for the CreateAddLiquidityInstructions function

// getPoolInfo fetches pool information from the chain
func (tm *TxManager) getPoolInfo(ctx context.Context, poolState aSDK.PublicKey) (*struct {
	ProgramID string
}, error) {
	accountInfo, err := tm.Client.GetAccountInfo(ctx, poolState)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account info: %w", err)
	}
	programID := accountInfo.Value.Owner.String()
	return &struct {
		ProgramID string
	}{
		ProgramID: programID,
	}, nil
}

// getPoolAccounts retrieves the token vaults and observation state for a pool
// func (tm *TxManager) getPoolAccounts(ctx context.Context, poolState aSDK.PublicKey, tokenMintA aSDK.PublicKey, tokenMintB aSDK.PublicKey, programID aSDK.PublicKey) (tokenVaultA aSDK.PublicKey, tokenVaultB aSDK.PublicKey, observationState aSDK.PublicKey, err error) {
// 		// 1. 先生成 poolState
// 		// amm_config := aSDK.MustPublicKeyFromBase58("FiyUUSnhBgLhBgVGWNBVozhzSbAbFCU1Q8iWMHH3xUhA")
// 		amm_config, err := GetAmmConfigPDA(7, programID)
// 		if err != nil {
// 			return
// 		}
// 		// 保证 tokenMintA < tokenMintB
// 		if bytes.Compare(tokenMintA[:], tokenMintB[:]) > 0 {
// 			tokenMintA, tokenMintB = tokenMintB, tokenMintA
// 		}
// 		logx.Infof("✅ Amm config: %s", amm_config.String())
// 		poolState, _, err = aSDK.FindProgramAddress(
// 			[][]byte{
// 				[]byte("pool"),
// 				amm_config[:],
// 				tokenMintA[:],
// 				tokenMintB[:],
// 			},
// 			programID,
// 		)
// 		if err != nil {
// 			return
// 		}
// 		logx.Infof("✅ Pool state11: %s", poolState.String())

// 		accountInfo, err := tm.Client.GetAccountInfo(ctx, poolState)

// 		logx.Infof("✅ Account info1111: %s", accountInfo.Value.Owner.String())
// 	tokenVaultA, _, err = aSDK.FindProgramAddress(
// 		[][]byte{
// 			[]byte("token_vault"),
// 			poolState[:],
// 			tokenMintA[:],
// 		},
// 		programID,
// 	)
// 	if err != nil {
// 		return
// 	}

// 	tokenVaultB, _, err = aSDK.FindProgramAddress(
// 		[][]byte{
// 			[]byte("token_vault"),
// 			poolState[:],
// 			tokenMintB[:],
// 		},
// 		programID,
// 	)
// 	if err != nil {
// 		return
// 	}

// 	observationState, _, err = aSDK.FindProgramAddress(
// 		[][]byte{
// 			[]byte("observation"),
// 			poolState[:],
// 		},
// 		programID,
// 	)
// 	return
// }

// GetPoolVaultsByPoolStateAddr 获取 poolState 账户的 token vault 地址
func (tm *TxManager) GetPoolVaultsByPoolStateAddr(ctx context.Context, poolStateAddr aSDK.PublicKey) (aSDK.PublicKey, aSDK.PublicKey, aSDK.PublicKey, aSDK.PublicKey, aSDK.PublicKey, int, error) {
	accountInfo, err := tm.Client.GetAccountInfo(ctx, poolStateAddr)
	if err != nil {
		return aSDK.PublicKey{}, aSDK.PublicKey{}, aSDK.PublicKey{}, aSDK.PublicKey{}, aSDK.PublicKey{}, 0, fmt.Errorf("failed to get poolState account info: %w", err)
	}
	data := accountInfo.Value.Data.GetBinary()
	tokenMint0 := aSDK.PublicKeyFromBytes(data[73:105])
	tokenMint1 := aSDK.PublicKeyFromBytes(data[105:137])
	tokenVault0 := aSDK.PublicKeyFromBytes(data[137:169])
	tokenVault1 := aSDK.PublicKeyFromBytes(data[169:201])
	observationKey := aSDK.PublicKeyFromBytes(data[201:233])
	tickSpacingBytes := data[235:237]                                // []byte
	tickSpacing := int(binary.LittleEndian.Uint16(tickSpacingBytes)) // int
	fmt.Println("tickSpacing:", tickSpacing)

	return tokenMint0, tokenMint1, tokenVault0, tokenVault1, observationKey, tickSpacing, nil
}

// findOrCreateATAInstruction finds or creates an associated token account
func (tm *TxManager) findOrCreateATAInstruction(ctx context.Context, mint, owner aSDK.PublicKey) (aSDK.PublicKey, error) {
	// In a real implementation, you would:
	// 1. Check if the ATA exists
	// 2. If not, create an instruction to create it
	// 3. Return the ATA public key

	// Derive the associated token address
	ata, _, err := aSDK.FindProgramAddress(
		[][]byte{
			owner[:],
			aSDK.TokenProgramID[:],
			mint[:],
		},
		aSDK.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return aSDK.PublicKey{}, err
	}

	return ata, nil
}

// int32ToBytes returns little-endian bytes for int32
func int32ToBytes(n int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	return b
}

// findTickArrays finds the tick arrays for the lower and upper ticks
func (tm *TxManager) findTickArrays(ctx context.Context, poolState aSDK.PublicKey, tickSpacing int, tickLowerPrice, tickUpperPrice int32, programID aSDK.PublicKey) (tickArrayLower, tickArrayUpper aSDK.PublicKey, tickArrayLowerStartIndex, tickArrayUpperStartIndex int, tickLower, tickUpper int, err error) {

	//除以10的六次方
	fmt.Println("1tickLowerPrice:", tickLowerPrice)
	fmt.Println("2tickUpperPrice:", tickUpperPrice)

	tickLowerDec := decimal.NewFromInt32(tickLowerPrice).Div(decimal.NewFromInt(1000000))
	tickUpperDec := decimal.NewFromInt32(tickUpperPrice).Div(decimal.NewFromInt(1000000))

	fmt.Println("3tickLowerPrice:", tickLowerDec)
	fmt.Println("3tickUpperPrice:", tickUpperDec)

	// Calculate tick array start indices
	tickArraySize := int(60) // Standard size for Raydium CLMM
	// 如果你需要 tickLower/tickUpper 作为整数参与 tick 计算，先将 decimal 转 float64，再转 int
	tickLower = tickWithSpacing(priceToTick(tickLowerDec.InexactFloat64()), int(tickSpacing))
	tickUpper = tickWithSpacing(priceToTick(tickUpperDec.InexactFloat64()), int(tickSpacing))
	tickArrayLowerStartIndex = getTickArrayStartIndexByTick(tickLower, tickSpacing, tickArraySize)
	tickArrayUpperStartIndex = getTickArrayStartIndexByTick(tickUpper, tickSpacing, tickArraySize)

	// 	tickSpacing: 60
	// tickLowerPrice: 0
	// tickUpperPrice: 1
	// tickLower: 0
	// tickUpper: 0
	// tickArrayLowerStartIndex: 0
	// tickArrayUpperStartIndex: 0
	fmt.Println("tickLower:", tickLower)
	fmt.Println("tickUpper:", tickUpper)
	fmt.Println("tickArrayLowerStartIndex:", tickArrayLowerStartIndex)
	fmt.Println("tickArrayUpperStartIndex:", tickArrayUpperStartIndex)

	tickArrayLower, bumpLower, err := aSDK.FindProgramAddress(
		[][]byte{
			[]byte("tick_array"),
			poolState[:],
			int32ToBytes(int32(tickArrayLowerStartIndex)),
		},
		programID,
	)
	if err != nil {
		logx.Errorf("Failed to derive tickArrayLower: %v", err)
	}
	logx.Infof("tickArrayLower: %s, bump: %d, startIndex: %d", tickArrayLower.String(), bumpLower, tickArrayLowerStartIndex)

	tickArrayUpper, bumpUpper, err := aSDK.FindProgramAddress(
		[][]byte{
			[]byte("tick_array"),
			poolState[:],
			int32ToBytes(int32(tickArrayUpperStartIndex)),
		},
		programID,
	)
	if err != nil {
		logx.Errorf("Failed to derive tickArrayUpper: %v", err)
	}
	logx.Infof("tickArrayUpper: %s, bump: %d, startIndex: %d", tickArrayUpper.String(), bumpUpper, tickArrayUpperStartIndex)

	return
}

// priceToTick 计算 tick 索引
func priceToTick(price float64) int {
	const Q_RATIO = 1.0001
	return int(math.Log(price) / math.Log(Q_RATIO))
}

// tickWithSpacing 对齐到 tickSpacing 的整数倍
func tickWithSpacing(tick, tickSpacing int) int {
	compressed := tick / tickSpacing
	if tick < 0 && tick%tickSpacing != 0 {
		compressed -= 1 // round towards negative infinity
	}
	return compressed * tickSpacing
}

func getTickArrayStartIndexByTick(tickIndex, tickSpacing, tickArraySize int) int {
	ticksInArray := tickSpacing * tickArraySize
	start := tickIndex / ticksInArray
	if tickIndex < 0 && tickIndex%ticksInArray != 0 {
		start -= 1
	}
	return start * ticksInArray
}

// buildOpenPositionInstruction creates the OpenPosition instruction
func (tm *TxManager) buildOpenPositionInstruction(
	userWallet aSDK.PublicKey,
	positionMint aSDK.PublicKey,
	poolState aSDK.PublicKey,
	tokenAMint aSDK.PublicKey,
	tokenBMint aSDK.PublicKey,
	userTokenA aSDK.PublicKey,
	userTokenB aSDK.PublicKey,
	tokenVaultA aSDK.PublicKey,
	tokenVaultB aSDK.PublicKey,
	tickLower int32,
	tickUpper int32,
	tickArrayLower aSDK.PublicKey,
	tickArrayUpper aSDK.PublicKey,
	positionPDA aSDK.PublicKey,
	observationState aSDK.PublicKey,
	baseTokenIndex uint8,
	amount0Max uint64,
	amount1Max uint64,
	tickArrayLowerStartIndex int,
	tickArrayUpperStartIndex int,
	protocol_position_pda aSDK.PublicKey,
) aSDK.Instruction {
	logx.Infof("🔧 buildOpenPositionInstruction: Building OpenPosition instruction (smaller transaction)")
	logx.Infof("📋 Instruction parameters:")
	logx.Infof("   - User wallet: %s", userWallet.String())
	logx.Infof("   - Position mint: %s", positionMint.String())
	logx.Infof("   - Pool state: %s", poolState.String())
	logx.Infof("   - Token A mint: %s", tokenAMint.String())
	logx.Infof("   - Token B mint: %s", tokenBMint.String())
	logx.Infof("   - User token A: %s", userTokenA.String())
	logx.Infof("   - User token B: %s", userTokenB.String())
	logx.Infof("   - Token vault A: %s", tokenVaultA.String())
	logx.Infof("   - Token vault B: %s", tokenVaultB.String())
	logx.Infof("   - Tick lower: %d", tickLower)
	logx.Infof("   - Tick upper: %d", tickUpper)
	logx.Infof("   - Tick array lower: %s", tickArrayLower.String())
	logx.Infof("   - Tick array upper: %s", tickArrayUpper.String())
	logx.Infof("   - Position PDA: %s", positionPDA.String())
	logx.Infof("   - Observation state: %s", observationState.String())
	logx.Infof("   - Base token index: %d", baseTokenIndex)
	logx.Infof("   - Amount 0 max: %d", amount0Max)
	logx.Infof("   - Amount 1 max: %d", amount1Max)

	// === Derive Metaplex MetadataAccount PDA for the position mint ===
	metadataProgramID := aSDK.MustPublicKeyFromBase58("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s")
	metadataAccount, _, err := aSDK.FindProgramAddress(
		[][]byte{
			[]byte("metadata"),
			metadataProgramID[:],
			positionMint[:],
		},
		metadataProgramID,
	)
	if err != nil {
		logx.Errorf("❌ Failed to derive MetadataAccount PDA: %v", err)
	} else {
		logx.Infof("✅ MetadataAccount PDA: %s", metadataAccount.String())
	}

	// Note: amount0Max and amount1Max are now passed as parameters from the caller
	// and are already calculated based on the sorted token order

	// Create the position token account (ATA for position NFT)
	positionATA, _, err := aSDK.FindProgramAddress(
		[][]byte{
			userWallet[:],
			aSDK.TokenProgramID[:],
			positionMint[:],
		},
		aSDK.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		logx.Errorf("❌ Failed to derive position ATA: %v", err)
		// Fall back to using userWallet as positionATA in case of error
		positionATA = userWallet
	}

	// Use binary.Uint128 with correct values for liquidity
	// For now, setting a placeholder liquidity amount
	liquidity := bin.Uint128{
		Lo: amount0Max, // Using amount0Max as Lo for simplicity
		Hi: 0,          // Using 0 as Hi for simplicity
	}

	fmt.Println("tickLower:", tickLower)
	fmt.Println("tickUpper:", tickUpper)
	fmt.Println("tickArrayLowerStartIndex:", tickArrayLowerStartIndex)
	fmt.Println("tickArrayUpperStartIndex:", tickArrayUpperStartIndex)
	fmt.Println("amount0Max:", amount0Max)
	fmt.Println("amount1Max:", amount1Max)

	// openPositionInst := amm_v3.NewOpenPositionInstruction(
	// 	int32(tickLower),
	// 	int32(tickUpper),
	// 	int32(tickArrayLowerStartIndex),
	// 	int32(tickArrayUpperStartIndex),
	// 	liquidity,
	// 	amount0Max,
	// 	amount1Max,
	// 	//accounts:
	// 	userWallet,
	// 	userWallet,
	// 	positionMint,
	// 	positionATA,
	// 	metadataAccount,
	// 	poolState,
	// 	protocol_position_pda,
	// 	tickArrayLower,
	// 	tickArrayUpper,
	// 	positionPDA,
	// 	userTokenA,
	// 	userTokenB,
	// 	tokenVaultA,
	// 	tokenVaultB,
	// 	aSDK.SysVarRentPubkey,
	// 	aSDK.SystemProgramID,
	// 	aSDK.TokenProgramID,
	// 	aSDK.SPLAssociatedTokenAccountProgramID,
	// 	metadataProgramID,
	// )

	// Manually build the OpenPosition instruction with correct account order
	// This ensures only the first 3 accounts are marked as signers
	accountMetas := aSDK.AccountMetaSlice{
		// 1. payer (用户钱包) - 需要签名
		aSDK.Meta(userWallet).WRITE().SIGNER(),
		// 2. position_nft_owner (用户钱包) - 需要签名
		aSDK.Meta(userWallet).SIGNER(),
		// 3. position_nft_mint (positionMint) - 需要签名
		aSDK.Meta(positionMint).WRITE().SIGNER(),
		// 4. position_nft_account (positionATA) - 不需要签名
		aSDK.Meta(positionATA).WRITE(),
		// 5. metadata_account - 不需要签名
		aSDK.Meta(metadataAccount).WRITE(),
		// 6. pool_state - 不需要签名
		aSDK.Meta(poolState).WRITE(),
		// 7. protocol_position - 不需要签名
		aSDK.Meta(protocol_position_pda).WRITE(),
		// 8. tick_array_lower - 不需要签名
		aSDK.Meta(tickArrayLower).WRITE(),
		// 9. tick_array_upper - 不需要签名
		aSDK.Meta(tickArrayUpper).WRITE(),
		// 10. personal_position - 不需要签名
		aSDK.Meta(positionPDA).WRITE(),
		// 11. token_account_0 (userTokenA) - 不需要签名
		aSDK.Meta(userTokenA).WRITE(),
		// 12. token_account_1 (userTokenB) - 不需要签名
		aSDK.Meta(userTokenB).WRITE(),
		// 13. token_vault_0 (tokenVaultA) - 不需要签名
		aSDK.Meta(tokenVaultA).WRITE(),
		// 14. token_vault_1 (tokenVaultB) - 不需要签名
		aSDK.Meta(tokenVaultB).WRITE(),
		// 15. rent - 不需要签名
		aSDK.Meta(aSDK.SysVarRentPubkey),
		// 16. system_program - 不需要签名
		aSDK.Meta(aSDK.SystemProgramID),
		// 17. token_program - 不需要签名
		aSDK.Meta(aSDK.TokenProgramID),
		// 18. associated_token_program - 不需要签名
		aSDK.Meta(aSDK.SPLAssociatedTokenAccountProgramID),
		// 19. metadata_program - 不需要签名
		aSDK.Meta(metadataProgramID),
	}

	// Create instruction data for OpenPosition
	// The discriminator for OpenPosition is {77, 184, 74, 214, 112, 86, 241, 199}
	instructionData := []byte{135, 128, 47, 77, 15, 152, 240, 49}
	// ag_binary.TypeID([8]byte{})
	// Add instruction parameters
	// We need to encode: tick_lower_index, tick_upper_index, tick_array_lower_start_index, tick_array_upper_start_index, liquidity, amount_0_max, amount_1_max
	// Each parameter should be encoded as little-endian

	// Add tick_lower_index (i32, 4 bytes)
	tickLowerBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tickLowerBytes, uint32(tickLower))
	instructionData = append(instructionData, tickLowerBytes...)

	// Add tick_upper_index (i32, 4 bytes)
	tickUpperBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tickUpperBytes, uint32(tickUpper))
	instructionData = append(instructionData, tickUpperBytes...)

	// Add tick_array_lower_start_index (i32, 4 bytes)
	tickArrayLowerStartIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tickArrayLowerStartIndexBytes, uint32(tickArrayLowerStartIndex))
	instructionData = append(instructionData, tickArrayLowerStartIndexBytes...)

	// Add tick_array_upper_start_index (i32, 4 bytes)
	tickArrayUpperStartIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(tickArrayUpperStartIndexBytes, uint32(tickArrayUpperStartIndex))
	instructionData = append(instructionData, tickArrayUpperStartIndexBytes...)

	// Add liquidity (u128, 16 bytes)
	liquidityBytes := make([]byte, 16)
	binary.LittleEndian.PutUint64(liquidityBytes[:8], liquidity.Lo)
	binary.LittleEndian.PutUint64(liquidityBytes[8:], liquidity.Hi)
	instructionData = append(instructionData, liquidityBytes...)

	// Add amount_0_max (u64, 8 bytes)
	amount0MaxBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amount0MaxBytes, amount0Max)
	instructionData = append(instructionData, amount0MaxBytes...)

	// Add amount_1_max (u64, 8 bytes)
	amount1MaxBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amount1MaxBytes, amount1Max)
	instructionData = append(instructionData, amount1MaxBytes...)

	// Add withMetadata (bool, 1 byte) - true for create metadata
	withMetadata := byte(1) // true
	instructionData = append(instructionData, withMetadata)

	// Add optionBaseFlag (u8, 1 byte) - 0 for no base flag
	optionBaseFlag := byte(0)
	instructionData = append(instructionData, optionBaseFlag)

	// Add baseFlag (bool, 1 byte) - false for no base flag
	baseFlag := byte(0) // false
	instructionData = append(instructionData, baseFlag)

	// Create the instruction
	openPositionInst := aSDK.NewInstruction(
		aSDK.MustPublicKeyFromBase58("A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys"), // Raydium CLMM program ID
		accountMetas,
		instructionData,
	)

	// If the builder supports setting MetadataAccount, add it here:
	// .SetMetadataAccount(metadataAccount)
	// If not, log for further integration.

	// Build the instruction
	// instruction, err := openPositionInst.ValidateAndBuild()
	// if err != nil || instruction == nil {
	// 	logx.Errorf("❌ Failed to build OpenPosition instruction: %v", err)
	// 	// Fall back to a simple transfer if the instruction build fails
	// 	return system.NewTransferInstruction(
	// 		1000, // Minimal amount
	// 		userWallet,
	// 		userWallet,
	// 	).Build()
	// }

	logx.Infof("✅ Successfully built OpenPosition instruction")
	return openPositionInst
}

func (tm *TxManager) CreatePoolInstructions(ctx context.Context, createPoolTx *trade.CreatePoolTx) ([]aSDK.Instruction, string, error) {
	logx.WithContext(ctx).Infof("🔧 Creating Raydium CLMM pool instructions for tokens: %s, %s", createPoolTx.TokenMint0, createPoolTx.TokenMint1)

	// 1. Convert string addresses to PublicKeys
	tokenMint0, err := aSDK.PublicKeyFromBase58(createPoolTx.TokenMint0)
	if err != nil {
		return nil, "", fmt.Errorf("invalid token_mint_0: %v", err)
	}

	tokenMint1, err := aSDK.PublicKeyFromBase58(createPoolTx.TokenMint1)
	if err != nil {
		return nil, "", fmt.Errorf("invalid token_mint_1: %v", err)
	}

	poolCreator, err := aSDK.PublicKeyFromBase58(createPoolTx.UserWalletAddress)
	if err != nil {
		return nil, "", fmt.Errorf("invalid user wallet address: %v", err)
	}

	// 2. Add compute budget instructions (always required for complex transactions)
	// Set compute unit price (priority fee)
	computeUnitPriceInstruction := aSDK.NewInstruction(
		aSDK.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111"),
		aSDK.AccountMetaSlice{},
		[]byte{3, 159, 217, 5, 0, 0, 0, 0, 0}, // 375000 microlamports
	)

	// Set compute unit limit
	computeUnitLimitInstruction := aSDK.NewInstruction(
		aSDK.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111"),
		aSDK.AccountMetaSlice{},
		[]byte{2, 0, 16, 3, 0, 0, 0, 0}, // 200000 compute units
	)

	// 3. Set up Raydium CLMM program ID (this is the official Raydium CLMM program on devnet/mainnet)
	raydiumProgramID := aSDK.MustPublicKeyFromBase58("A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys")

	// 4. Sort tokens for deterministic ordering (Raydium requirement)
	// Important: Raydium expects tokens to be sorted by their address
	sortedToken0, sortedToken1 := tokenMint0, tokenMint1
	if bytes.Compare(tokenMint0[:], tokenMint1[:]) > 0 {
		sortedToken0, sortedToken1 = tokenMint1, tokenMint0
		logx.WithContext(ctx).Infof("⚠️ Tokens were reordered for Raydium requirements")
	}

	logx.WithContext(ctx).Infof("Sorted tokens: token0=%s, token1=%s", sortedToken0, sortedToken1)

	// 5. Calculate initial price in sqrt-price-x64 format
	// Price is provided as decimal, e.g. 0.001
	initialPrice, err := decimal.NewFromString(createPoolTx.InitialPrice)
	if err != nil {
		return nil, "", fmt.Errorf("invalid initial price: %v", err)
	}

	// Convert price to sqrtPriceX64
	// Using decimal for precision: sqrt(price) * 2^64
	sqrtPrice := decimal.NewFromFloat(math.Sqrt(initialPrice.InexactFloat64()))
	sqrtPriceX64 := sqrtPrice.Mul(decimal.NewFromInt(1).Shift(64))

	logx.WithContext(ctx).Infof("Price calculation: initialPrice=%s, sqrtPrice=%s, sqrtPriceX64=%s",
		initialPrice, sqrtPrice, sqrtPriceX64)

	// 7. Get AMM Config based on fee tier
	// These are the official Raydium AMM config addresses on devnet
	ammConfig := aSDK.MustPublicKeyFromBase58("3UfAopmTFEMwiHaWvyQgNbBnhX2279RYMriFCXGBMLFz") // Default 0.05% fee

	// Map the fee tier to the appropriate AMM config
	switch createPoolTx.FeeTier {
	case 1:
		// 0.01% fee tier (1 BP), 1 tick spacing
		ammConfig = aSDK.MustPublicKeyFromBase58("37x3waVE77oJ6iF7zVBkdRRkwfq8xfGrSFY3sziWLyuE")
	case 30:
		// 0.3% fee tier (30 BP), 60 tick spacing
		ammConfig = aSDK.MustPublicKeyFromBase58("FiyUUSnhBgLhBgVGWNBVozhzSbAbFCU1Q8iWMHH3xUhA")
	case 100:
		// 1% fee tier (100 BP), 200 tick spacing
		ammConfig = aSDK.MustPublicKeyFromBase58("C4E93u37E9RUdczGpfMngFEgUuPNrxiWRcLE7oRgSDao")
	}
	logx.WithContext(ctx).Infof("AMM Config: %s for fee tier %d", ammConfig, createPoolTx.FeeTier)

	// 6. Derive PDAs needed for pool accounts
	// Use our helper function to get the correct PDAs
	poolStatePDA, tokenVault0PDA, tokenVault1PDA, observationStatePDA, tickArrayBitmapPDA, err := findRaydiumPoolPDAs(
		ctx,
		raydiumProgramID,
		sortedToken0,
		sortedToken1,
		ammConfig,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to derive PDAs: %v", err)
	}

	logx.WithContext(ctx).Infof("Pool State PDA: %s", poolStatePDA)
	logx.WithContext(ctx).Infof("Token Vault 0 PDA: %s", tokenVault0PDA)
	logx.WithContext(ctx).Infof("Token Vault 1 PDA: %s", tokenVault1PDA)
	logx.WithContext(ctx).Infof("Observation State PDA: %s", observationStatePDA)
	logx.WithContext(ctx).Infof("Tick Array Bitmap PDA: %s", tickArrayBitmapPDA)

	// 8. Create the instruction data for Raydium create pool
	// Format should be: [8-byte discriminator][16-bytes sqrtPriceX64][8-bytes openTime]

	// First, create the instruction with the correct discriminator for createPool
	// The discriminator for Raydium CLMM createPool is {233, 146, 209, 142, 207, 104, 64, 188}
	instructionData := []byte{233, 146, 209, 142, 207, 104, 64, 188}

	// Next, encode sqrtPriceX64 as a 16-byte (128-bit) big-endian unsigned integer
	// We need to carefully handle endianness here
	sqrtPriceX64Int := sqrtPriceX64.BigInt()
	sqrtPriceX64Bytes := make([]byte, 16)

	// Ensure we have proper big-endian representation
	sqrtBytes := sqrtPriceX64Int.Bytes()

	// Handle the case where sqrtBytes might be larger than 16 bytes
	if len(sqrtBytes) > 16 {
		// Take only the last 16 bytes (least significant) if larger
		copy(sqrtPriceX64Bytes, sqrtBytes[len(sqrtBytes)-16:])
	} else {
		// Pad with leading zeros if smaller
		copy(sqrtPriceX64Bytes[16-len(sqrtBytes):], sqrtBytes)
	}

	logx.WithContext(ctx).Infof("SqrtPriceX64 bytes (hex): %x", sqrtPriceX64Bytes)
	instructionData = append(instructionData, sqrtPriceX64Bytes...)

	// Finally, encode openTime as a 8-byte (64-bit) little-endian unsigned integer
	// Raydium uses little-endian encoding for the timestamp
	openTimeBytes := make([]byte, 8)
	openTime := uint64(createPoolTx.OpenTime)
	binary.LittleEndian.PutUint64(openTimeBytes, openTime)

	logx.WithContext(ctx).Infof("OpenTime bytes (hex): %x", openTimeBytes)
	instructionData = append(instructionData, openTimeBytes...)

	logx.WithContext(ctx).Infof("Complete instruction data (hex): %x", instructionData)

	// 9. Create the account meta slice with ALL accounts needed for the createPool instruction
	// Account order is critical for Raydium instructions
	poolAccounts := aSDK.AccountMetaSlice{
		aSDK.Meta(poolCreator).WRITE().SIGNER(), // [0] poolCreator (owner account that pays for and signs the tx)
		aSDK.Meta(ammConfig),                    // [1] ammConfig (read-only)
		aSDK.Meta(poolStatePDA).WRITE(),         // [2] poolState (write)
		aSDK.Meta(sortedToken0),                 // [3] tokenMint0 (read-only)
		aSDK.Meta(sortedToken1),                 // [4] tokenMint1 (read-only)
		aSDK.Meta(tokenVault0PDA).WRITE(),       // [5] tokenVault0 (write)
		aSDK.Meta(tokenVault1PDA).WRITE(),       // [6] tokenVault1 (write)
		aSDK.Meta(observationStatePDA).WRITE(),  // [7] observationState (write)
		aSDK.Meta(tickArrayBitmapPDA).WRITE(),   // [8] tickArrayBitmap (write)
		aSDK.Meta(aSDK.TokenProgramID),          // [9] tokenProgram0 (read-only)
		aSDK.Meta(aSDK.TokenProgramID),          // [10] tokenProgram1 (read-only)
		aSDK.Meta(aSDK.SystemProgramID),         // [11] systemProgram (read-only)
		aSDK.Meta(aSDK.SysVarRentPubkey),        // [12] rent (read-only)
	}

	// 10. Create the pool creation instruction with the program, accounts, and data
	poolInstruction := aSDK.NewInstruction(
		raydiumProgramID,
		poolAccounts,
		instructionData,
	)

	// Log details about the pool creation
	logx.WithContext(ctx).Infof("✅ Created Raydium CLMM pool creation instruction with parameters:")
	logx.WithContext(ctx).Infof("  - Pool Creator: %s", poolCreator.String())
	logx.WithContext(ctx).Infof("  - Token 0: %s", sortedToken0.String())
	logx.WithContext(ctx).Infof("  - Token 1: %s", sortedToken1.String())
	logx.WithContext(ctx).Infof("  - Initial Price: %s", createPoolTx.InitialPrice)
	logx.WithContext(ctx).Infof("  - Fee Tier: %d basis points", createPoolTx.FeeTier)
	logx.WithContext(ctx).Infof("  - Open Time: %d (%s)", createPoolTx.OpenTime,
		time.Unix(createPoolTx.OpenTime, 0).Format(time.RFC3339))

	// Return all instructions plus the derived pool state address, so the
	// caller can hand it back to the frontend for Portfolio attribution —
	// CreatePoolResponse used to only carry the unsigned tx, with no way to
	// know which pool a successful creation actually produced.
	return []aSDK.Instruction{
		computeUnitPriceInstruction,
		computeUnitLimitInstruction,
		poolInstruction,
	}, poolStatePDA.String(), nil
}

// findRaydiumPoolPDAs finds the correct PDAs for Raydium CLMM pools
// This function implements the PDA derivation logic from the Raydium SDK
func findRaydiumPoolPDAs(ctx context.Context, programID aSDK.PublicKey, token0, token1, ammConfig aSDK.PublicKey) (
	poolState aSDK.PublicKey,
	tokenVault0 aSDK.PublicKey,
	tokenVault1 aSDK.PublicKey,
	observationState aSDK.PublicKey,
	tickArrayBitmap aSDK.PublicKey,
	err error) {

	// Define seed constants
	poolSeed := []byte("pool")
	poolVaultSeed := []byte("pool_vault") // Corrected from "token_vault" to "pool_vault"
	observationSeed := []byte("observation")
	bitmapSeed := []byte("pool_tick_array_bitmap_extension")

	// First, derive the pool state PDA using ammConfig and token mints
	// This matches the getPdaPoolId function in JS
	poolIdSeeds := [][]byte{
		poolSeed,
		ammConfig[:],
		token0[:],
		token1[:],
	}

	poolState, _, err = aSDK.FindProgramAddress(poolIdSeeds, programID)
	if err != nil {
		return
	}

	// Log derivation data for debugging
	logx.WithContext(ctx).Infof("Deriving PDAs for: token0=%s, token1=%s, ammConfig=%s, programID=%s",
		token0.String(), token1.String(), ammConfig.String(), programID.String())
	logx.WithContext(ctx).Infof("Derived pool state PDA: %s", poolState.String())

	// Derive token vault PDAs using poolVaultSeed
	// This matches the getPdaPoolVaultId function in JS
	tokenVault0Seeds := [][]byte{
		poolVaultSeed,
		poolState[:],
		token0[:],
	}
	tokenVault0, _, err = aSDK.FindProgramAddress(tokenVault0Seeds, programID)
	if err != nil {
		return
	}

	tokenVault1Seeds := [][]byte{
		poolVaultSeed,
		poolState[:],
		token1[:],
	}
	tokenVault1, _, err = aSDK.FindProgramAddress(tokenVault1Seeds, programID)
	if err != nil {
		return
	}

	logx.WithContext(ctx).Infof("Derived token vault 0 PDA: %s", tokenVault0.String())
	logx.WithContext(ctx).Infof("Derived token vault 1 PDA: %s", tokenVault1.String())

	// Derive the observation state PDA using the pool state
	// This matches the getPdaObservationAccount function in JS
	observationStateSeeds := [][]byte{
		observationSeed,
		poolState[:],
	}
	observationState, _, err = aSDK.FindProgramAddress(observationStateSeeds, programID)
	if err != nil {
		return
	}

	// Derive the tick array bitmap PDA using the pool state
	// This matches the getPdaExBitmapAccount function in JS
	tickArrayBitmapSeeds := [][]byte{
		bitmapSeed,
		poolState[:],
	}
	tickArrayBitmap, _, err = aSDK.FindProgramAddress(tickArrayBitmapSeeds, programID)

	return
}

// PrintALTTable loads and logs the ALT table content for debugging
func (tm *TxManager) PrintALTTable(ctx context.Context, altBase58 string) error {
	client := tm.Client // ag_rpc.Client must implement GetAccountInfo
	altAddr := aSDK.MustPublicKeyFromBase58(altBase58)
	altState, err := alt.GetAddressLookupTable(ctx, client, altAddr)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to load ALT: %v", err)
		return err
	}
	logx.WithContext(ctx).Infof("ALT Authority: %v", altState.Authority)
	logx.WithContext(ctx).Infof("ALT DeactivationSlot: %d", altState.DeactivationSlot)
	logx.WithContext(ctx).Infof("ALT LastExtendedSlot: %d", altState.LastExtendedSlot)
	logx.WithContext(ctx).Infof("ALT Addresses:")
	for i, addr := range altState.Addresses {
		logx.WithContext(ctx).Infof("  [%d] %s", i, addr.String())
	}
	return nil
}

func GetAmmConfigPDA(index uint16, programID aSDK.PublicKey) (aSDK.PublicKey, error) {
	indexBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(indexBytes, index) // <--- 用 BigEndian
	seeds := [][]byte{
		[]byte("amm_config"),
		indexBytes,
	}

	pda, _, err := aSDK.FindProgramAddress(seeds, programID)
	return pda, err
}
