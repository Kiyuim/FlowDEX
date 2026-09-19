package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"dex/market/marketclient"
	"dex/model/trademodel"
	"dex/pkg/constants"
	trade2 "dex/pkg/trade"
	"dex/pkg/util"
	"dex/pkg/xcode"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	aSDK "github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/trace"
)

type CreateMarketOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMarketOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMarketOrderLogic {
	return &CreateMarketOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateMarketOrderLogic) CreateMarketOrder(in *trade.CreateMarketOrderRequest) (*trade.CreateMarketOrderResponse, error) {
	// 先校验参数
	amountDecimal, err := decimal.NewFromString(in.AmountIn)
	if err != nil {
		return nil, err
	}

	if !amountDecimal.IsPositive() {
		return nil, xcode.AmountErr
	}

	// save to db first
	var isAntiMev int64 = 0

	var isAutoSlippage int64 = 0

	//
	if in == nil {
		fmt.Println("in is nil")
	}

	//output in
	fmt.Println(" input in is:", in)

	fmt.Println("*********************About to call GetPairInfoByToken***************")
	fmt.Printf("Market client is: %+v\n", l.svcCtx.MarketClient)

	pairInfo, err := l.svcCtx.MarketClient.GetPairInfoByToken(l.ctx, &marketclient.GetPairInfoByTokenRequest{
		ChainId:      int64(in.ChainId),
		TokenAddress: in.TokenCa,
	})
	fmt.Println("*********************2222***************")
	if err != nil {
		fmt.Println("GetPairInfoByToken err is", err)

		return nil, fmt.Errorf("err is %s", err)
	}
	if pairInfo == nil {
		fmt.Println("2222GetPairInfoByToken err is", err)

		return nil, fmt.Errorf("pairInfo is nil for token: %s", in.TokenCa)
	}
	// check pair是否正常
	if pairInfo.Fdv == 0 {
		fmt.Println("3333 pairInfo.Fdv is 0")

		return nil, fmt.Errorf("%s pairInfo %s fdv is 0", pairInfo.Name, pairInfo.Address)
	}

	fmt.Println("*********************3333***************")

	capDecimal := decimal.NewFromFloat(pairInfo.Fdv)
	model := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	baseTokenPrice := decimal.NewFromFloat(pairInfo.BaseTokenPrice)
	tokenPriceUsdDecimal := decimal.NewFromFloat(pairInfo.TokenPrice)

	// Safety check to prevent division by zero
	if baseTokenPrice.IsZero() {
		l.Errorf("BaseTokenPrice is zero for token %s, pair %s - attempting to fetch current price", in.TokenCa, pairInfo.Address)

		// Try to get the current native token (SOL) price
		if in.ChainId == constants.SolChainIdInt || in.ChainId == 100000 {
			nativePrice, err := l.svcCtx.MarketClient.GetNativeTokenPrice(l.ctx, &marketclient.GetNativeTokenPriceRequest{
				ChainId: int64(in.ChainId),
			})
			if err == nil && nativePrice.BaseTokenPriceUsd > 0 {
				l.Infof("Retrieved SOL price: %f for token %s", nativePrice.BaseTokenPriceUsd, in.TokenCa)
				baseTokenPrice = decimal.NewFromFloat(nativePrice.BaseTokenPriceUsd)
			} else {
				l.Errorf("Failed to get native token price: %v", err)
				return nil, fmt.Errorf("invalid base token price (zero) for token %s and unable to fetch current price", in.TokenCa)
			}
		} else {
			return nil, fmt.Errorf("invalid base token price (zero) for token %s", in.TokenCa)
		}
	}

	tokenPriceDecimal := tokenPriceUsdDecimal.Div(baseTokenPrice)
	orderValueBase := amountDecimal
	if in.SwapType == trade.SwapType_Sell {
		orderValueBase = amountDecimal.Mul(tokenPriceDecimal)
	}
	fmt.Println("amountDecimal is:", amountDecimal)
	order := &trademodel.TradeOrder{
		TradeType:      int64(trade.TradeType_Market),
		ChainId:        int64(in.ChainId),
		TokenCa:        in.TokenCa,
		SwapType:       int64(in.SwapType),
		IsAutoSlippage: isAutoSlippage,
		Slippage:       5000, // 50% slippage
		IsAntiMev:      isAntiMev,
		GasType:        1,
		Status:         int64(trade.OrderStatus_Proc),
		OrderCap:       capDecimal,
		OrderAmount:    amountDecimal,
		OrderPriceBase: tokenPriceDecimal,
		OrderValueBase: orderValueBase,
		OrderBasePrice: baseTokenPrice,
		// 是否翻倍出本 1:是 0:否
		DoubleOut:     util.BoolToInt64(in.DoubleOut),
		DexName:       pairInfo.Name,
		PairCa:        pairInfo.Address,
		WalletAddress: in.UserWalletAddress,
	}
	if in.IsOneClick {
		order.TradeType = int64(trade.TradeType_OneClick)
	}
	err = model.InsertWithLog(l.ctx, order)
	if err != nil {
		l.Errorf("InsertWithLog err:%s", err.Error())
		return nil, xcode.ServerErr
	}

	// solana很快就直接同步了，别的比较慢走异步
	l.Infof("Route decision: ChainId=%d (SolChainIdInt=%d), SwapType=%d (SwapType_Buy=%d)",
		order.ChainId, constants.SolChainIdInt, order.SwapType, int64(trade.SwapType_Buy))

	if order.ChainId == constants.SolChainIdInt || order.SwapType == int64(trade.SwapType_Buy) {
		l.Infof("Taking synchronous path: ChainId=%d, SwapType=%d", order.ChainId, order.SwapType)
		txHash, err := l.CreateMarketTx(order, pairInfo)
		if err != nil {
			l.Errorf("CreateMarketTx error: %v", err)
			return nil, err
		}

		l.Infof("CreateMarketTx success, txHash length: %d", len(txHash))
		return &trade.CreateMarketOrderResponse{TxHash: txHash}, nil
	}

	l.Infof("Taking asynchronous path - returning empty txHash immediately")
	// threading.GoSafe(func() {
	// 	// 异步比较慢 需要拷贝一份ctx出来
	// 	asynCtx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(l.ctx))
	// 	newL := NewCreateMarketOrderLogic(asynCtx, l.svcCtx)
	// 	_, err = newL.CreateMarketTx(order, pairInfo)
	// 	if err != nil {
	// 		newL.Error(err)
	// 	}
	// })
	return &trade.CreateMarketOrderResponse{TxHash: ""}, nil
}

func (l *CreateMarketOrderLogic) CreateMarketTx(order *trademodel.TradeOrder, pairInfo *marketclient.GetPairInfoByTokenResponse) (string, error) {
	var err error
	defer func() {
		// 如果订单状态是触发中 并且有错误，那么将订单状态改为失败
		if order.Status == int64(trade.OrderStatus_Proc) && err != nil {
			err2 := l.updateDbByTxResult(order, nil, "", err)
			if err2 != nil {
				l.Error(err2)
			}
		}
	}()

	if pairInfo == nil {
		// Get trading pair information
		pairInfo, err = l.svcCtx.MarketClient.GetPairInfoByToken(l.ctx, &marketclient.GetPairInfoByTokenRequest{
			ChainId:      order.ChainId,
			TokenAddress: order.TokenCa,
		})
		if err != nil {
			l.Errorf("CreateMarketTxOkx GetPairInfoByToken failed token:%s, err:%v", order.TokenCa, err)
			return "", err
		}
	}
	var txhash string
	txhash, err = l.createMarketTxWithPairInfo(order, pairInfo)
	l.Infof("createMarketTxWithPairInfo returned: txhash length=%d, err=%v", len(txhash), err)
	if err != nil {
		return "", err
	}

	l.Infof("CreateMarketTx returning: txhash length=%d", len(txhash))
	return txhash, nil
}

func (l *CreateMarketOrderLogic) updateDbByTxResult(order *trademodel.TradeOrder, param *trade2.CreateMarketTx, txHash string, errReason error) error {
	model := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	// 防止ctx取消导致更新数据库失败，复制一份ctx出来进行更新数据库
	dbCtx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(l.ctx))
	orderData, _ := json.Marshal(order)
	l.Debugf("updateDbByTxResult order: %s", string(orderData))
	// 失败的情况
	if errReason != nil {
		selectStr := []string{"status", "fail_reason"}
		if param != nil && order.Slippage != int64(param.Slippage) {
			order.Slippage = int64(param.Slippage)
			selectStr = append(selectStr, "slippage")
		}
		order.Status = int64(trade.OrderStatus_Fail)
		order.FailReason = errReason.Error()
		if err := model.UpdateOrderBySelect(dbCtx, order, selectStr...); err != nil {
			l.Errorf("updateDbByTxResult:UpdateOrderBySelect err:&s", err.Error())
			return xcode.ServerErr
		}
		return nil
	}
	// 成功的情况
	selectStr := []string{"status"}

	// This function now only handles regular transaction hashes (not unsigned transactions)
	// Unsigned transactions are handled separately and bypass database storage
	order.Status = int64(trade.OrderStatus_OnChain)
	order.TxHash = txHash
	selectStr = append(selectStr, "tx_hash")
	l.Infof("Transaction hash stored: %s", txHash)

	if order.DexName != param.TradePoolName {
		order.DexName = param.TradePoolName
		selectStr = append(selectStr, "dex_name")
	}
	if order.PairCa != param.PairAddr {
		order.PairCa = param.PairAddr
		selectStr = append(selectStr, "pair_ca")
	}
	if order.Slippage != int64(param.Slippage) {
		order.Slippage = int64(param.Slippage)
		selectStr = append(selectStr, "slippage")
	}
	err := model.UpdateOrderBySelect(dbCtx, order, selectStr...)

	if err != nil {
		l.Errorf("CreateMarketOrder:UpdateOrderBySelect err:%s", err.Error())
		return xcode.ServerErr
	}
	orderData, _ = json.Marshal(order)
	l.Debugf("CreateMarketOrder suc order:%s", string(orderData))
	return nil
}

func (l *CreateMarketOrderLogic) createMarketTxWithPairInfo(order *trademodel.TradeOrder, pairInfo *marketclient.GetPairInfoByTokenResponse) (string, error) {
	tokenInfo, err := l.svcCtx.MarketClient.GetTokenInfo(l.ctx, &marketclient.GetTokenInfoRequest{
		ChainId:      order.ChainId,
		TokenAddress: order.TokenCa,
	})
	if err != nil {
		l.Errorf("CreateMarketOrder GetTokenInfo failed token:%s, err:%v", order.TokenCa, err)
		return "", err
	}
	usePriceLimit := false
	// 目前根据订单类型和池子种类来判断，限价单开启和clmm池子都开启
	if order.TradeType == int64(trade.TradeType_Limit) || order.TradeType == int64(trade.TradeType_TokenCapLimit) ||
		order.DexName == constants.RaydiumConcentratedLiquidity {
		usePriceLimit = true
	}

	inTokenAddr := pairInfo.BaseTokenAddress
	outTokenAddr := pairInfo.TokenAddress

	inDecimal, outDecimal := uint8(pairInfo.BaseTokenDecimal), uint8(pairInfo.TokenDecimal)
	fmt.Println("inDecimal is:", inDecimal)
	fmt.Println("outDecimal is:", outDecimal)
	var inTokenProgram, outTokenProgram string
	if order.ChainId == constants.SolChainIdInt {
		inTokenProgram, outTokenProgram = aSDK.TokenProgramID.String(), aSDK.TokenProgramID.String()
		if tokenInfo.Program != "" {
			outTokenProgramAccount, err := aSDK.PublicKeyFromBase58(tokenInfo.Program)
			if nil != err {
				return "", err
			}
			outTokenProgram = outTokenProgramAccount.String()
		}
	}

	// 如果是卖单,需要交换输入输出代币地址和精度
	// 卖单时输入代币为交易代币,输出代币为基础代币
	if order.SwapType == int64(trade.SwapType_Sell) {
		inTokenAddr, outTokenAddr = outTokenAddr, inTokenAddr
		inDecimal, outDecimal = outDecimal, inDecimal
		inTokenProgram, outTokenProgram = outTokenProgram, inTokenProgram
	}

	// to make and send tx
	// 构建市价交易参数
	param := &trade2.CreateMarketTx{
		// 用户ID
		UserId: uint64(order.Uid),
		// 链ID
		ChainId: uint64(order.ChainId),
		// 钱包组ID
		UserWalletId: uint32(order.WalletIndex),
		// 用户钱包地址
		UserWalletAddress: order.WalletAddress,
		// 输入代币数量
		AmountIn: order.OrderAmount.String(),
		// 是否开启反抢跑
		IsAntiMev: order.IsAntiMev != 0,
		// 是否自动滑点
		IsAutoSlippage: order.IsAutoSlippage != 0,
		// 滑点设置
		Slippage: uint32(order.Slippage),
		// Gas类型
		GasType: int32(order.GasType),
		// 交易池名称
		TradePoolName: pairInfo.Name,
		// 输入代币精度
		InDecimal: 9,
		// 输出代币精度
		OutDecimal: 9,
		// 输入代币地址(默认为基础代币地址)
		InTokenCa: inTokenAddr,
		// 输出代币地址(默认为交易代币地址)
		OutTokenCa: outTokenAddr,
		// 交易对地址
		PairAddr: pairInfo.Address,
		// 代币价格(基础币本位)
		Price: order.OrderPriceBase.String(),
		// 是否开启价格限制
		UsePriceLimit: usePriceLimit,
		// 输入代币的合约类型 token/token2022
		InTokenProgram: inTokenProgram,
		// 输出代币的合约类型 token/token2022
		OutTokenProgram: outTokenProgram,
	}

	// to make and send tx
	var txHash string
	tryTimes := 0
	//  判断如果是自动滑点情况下，滑点过大的错误，那么增大滑点并重试
	for tryTimes == 0 || (param.IsAutoSlippage && errors.Is(err, xcode.SlippageLimit) && tryTimes < 3) {
		tryTimes++
		switch tryTimes {
		case 1:
		case 2:
			param.Slippage = 4500
			l.Info("AutoSlippageRetry")
		case 3:
			param.Slippage = 7000
			l.Info("AutoSlippageRetry")
		}
		txHash, err = l.createAndSendTx(param)
		if err != nil {
			err = convertSwapErr(param.TradePoolName, err)
		}
	}
	// For unsigned transactions (length > 100), don't update database - just return the transaction
	if len(txHash) > 100 {
		l.Infof("Returning unsigned transaction directly to client, length: %d", len(txHash))
		return txHash, err
	}

	// For regular tx hashes, update database as normal
	err2 := l.updateDbByTxResult(order, param, txHash, err)
	if err2 != nil {
		l.Error(err2)
		return "", err
	}

	l.Infof("createMarketTxWithPairInfo returning: txHash length=%d, err=%v", len(txHash), err)
	return txHash, err
}

func (l *CreateMarketOrderLogic) createAndSendTx(param *trade2.CreateMarketTx) (string, error) {
	// Debug: Print the param values
	fmt.Printf("createAndSendTx param: UserWalletAddress='%s', InTokenCa='%s', OutTokenCa='%s', InTokenProgram='%s', OutTokenProgram='%s'\n",
		param.UserWalletAddress, param.InTokenCa, param.OutTokenCa, param.InTokenProgram, param.OutTokenProgram)

	switch param.ChainId {
	case constants.SolChainIdInt:
		if l.svcCtx.SolTxMananger == nil {
			return "", fmt.Errorf("SolTxMananger is nil - check if Sol configuration is enabled")
		}

		// Build unsigned transaction for third-party wallet signing
		unsignedTxBase64, err := l.svcCtx.SolTxMananger.BuildUnsignedTransaction(l.ctx, param)
		if err != nil {
			l.Errorf("SolTxMananger.BuildUnsignedTransaction err:%v", err)
			return "", err
		}

		l.Infof("BuildUnsignedTransaction success, length=%d", len(unsignedTxBase64))
		// Return the unsigned transaction as base64 for the client to sign
		return unsignedTxBase64, nil
	default:
		return "", xcode.RequestErr
	}
}

func convertSwapErr(poolName string, err error) error {
	result := err.Error()
	if strings.Contains(result, "liquidity") {
		return xcode.PoolLiquidityNotEnough
	}
	if strings.Contains(result, "insufficient") {
		return xcode.BalanceNotEnough
	}
	if strings.Contains(result, "frozen") {
		return xcode.TokenAccountFrozen
	}
	if strings.Contains(result, "slippage") || strings.Contains(result, "TooLittleOutputReceived") {
		return xcode.SlippageLimit
	}
	switch poolName {
	case constants.RaydiumV4:
	case constants.RaydiumCPMM:
	case constants.RaydiumConcentratedLiquidity:
		if strings.Contains(result, "InsufficientLiquidityForDirection") {
			return xcode.PoolLiquidityNotEnough
		}
	case constants.PumpFun:
		if strings.Contains(result, "TooLittleSolReceived") || strings.Contains(result, "attempt to subtract with overflow") {
			return xcode.PumpPoolZeroErr
		}
	}
	return err
}
