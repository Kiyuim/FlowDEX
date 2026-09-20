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
	chainsolana "dex/trade/internal/chain/solana"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	aSDK "github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/trace"
)

// NominalSolPriceUsd is a fallback SOL/USD price used only for order-record
// metadata when no live price feed is available (e.g. devnet). It never affects
// the on-chain swap, which is built from live pool state.
const NominalSolPriceUsd = 150

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

	// Attached trailing stop: buy-only, N in (0,100) (same zombie-order guard
	// as CreateTrailingStop), and not combinable with double-out — both attach
	// a sell leg to the same fill and would race over the same tokens.
	if in.TrailingPercent != 0 {
		if in.TrailingPercent < 0 || in.TrailingPercent >= 100 {
			return nil, xcode.AmountErr
		}
		if in.SwapType != trade.SwapType_Buy {
			return nil, fmt.Errorf("trailing_percent is only valid on buy orders")
		}
		if in.DoubleOut {
			return nil, fmt.Errorf("double_out and trailing_percent cannot be combined")
		}
	}

	// save to db first
	var isAntiMev int64 = 0

	// Auto slippage (自动滑点): on a slippage failure the executor escalates the
	// slippage tier and rebuilds the swap instead of failing the order. The flag
	// is stored on the order so server-signed legs created later (double-out
	// sell, attached trailing stop, triggered limit) inherit it too.
	isAutoSlippage := util.BoolToInt64(in.IsAutoSlippage)

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
	// A zero Fdv only means the market-cap metadata hasn't synced yet (e.g. a
	// freshly-created or recently-reprocessed pump token). It does NOT mean the
	// token is untradeable — the swap is built from live on-chain pool state, not
	// from Fdv, which is only used for the OrderCap metadata below. So warn and
	// continue instead of hard-failing every buy with a generic 515.
	if pairInfo.Fdv == 0 {
		l.Errorf("pairInfo.Fdv is 0 for token %s (pair %s) — proceeding anyway; OrderCap metadata will be 0", in.TokenCa, pairInfo.Address)
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
				// The SOL/USD price is only used for order-record metadata (USD
				// valuation), not for the on-chain swap, which is built from live
				// pool state. On devnet there is no SOL price feed, so fall back to
				// a nominal value and proceed instead of failing every buy.
				l.Errorf("Failed to get native token price (%v); using nominal fallback for order metadata", err)
				baseTokenPrice = decimal.NewFromInt(NominalSolPriceUsd)
			}
		} else {
			baseTokenPrice = decimal.NewFromInt(NominalSolPriceUsd)
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
		DoubleOut: util.BoolToInt64(in.DoubleOut),
		// >0 = auto-create a trailing stop for the fill once the buy confirms
		TrailingPercent: int64(in.TrailingPercent),
		DexName:         pairInfo.Name,
		PairCa:          pairInfo.Address,
		WalletAddress:   in.UserWalletAddress,
	}
	if in.IsOneClick {
		order.TradeType = int64(trade.TradeType_OneClick)
	}

	// Double-out and trailing-stop-attached buys run custodially: the server
	// wallet pays, signs and holds the tokens, so the auto-created sell leg can
	// later be executed server-side too (the user isn't around to sign either
	// leg after checkout).
	if (in.DoubleOut || in.TrailingPercent > 0) && in.SwapType == trade.SwapType_Buy {
		serverWallet, err := chainsolana.ServerWalletAddress()
		if err != nil {
			return nil, fmt.Errorf("double-out requires the server wallet: %v", err)
		}
		order.WalletAddress = serverWallet
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

	// Solana pump/AMM pairs are always quoted in WSOL. If the pair record has no
	// base-token address (e.g. it was first created by the indexed `create`
	// before a trade filled the field in), default it so the swap can be built.
	if inTokenAddr == "" && order.ChainId == constants.SolChainIdInt {
		inTokenAddr = constants.TokenStrWrapSol
	}
	// The traded token is the one the request named; fall back to it if the pair
	// record's token address hasn't been populated yet.
	if outTokenAddr == "" {
		outTokenAddr = order.TokenCa
	}
	// TradePoolName routes the swap to the right DEX builder. If the pair metadata
	// is incomplete (empty name), default a Solana pump token to the bonding-curve
	// (PumpFun) route — BuildBuyV2Instructions derives all accounts from the mint.
	tradePoolName := pairInfo.Name
	if tradePoolName == "" && order.ChainId == constants.SolChainIdInt {
		tradePoolName = constants.PumpFun
	}

	inDecimal, outDecimal := uint8(pairInfo.BaseTokenDecimal), uint8(pairInfo.TokenDecimal)
	// Devnet pair records can have incomplete metadata (0 decimals). For Solana the
	// base/quote side is always WSOL (9 decimals); default the traded token to 6
	// (the pump.fun standard) so sell amounts aren't scaled by the wrong power.
	if order.ChainId == constants.SolChainIdInt {
		if inDecimal == 0 {
			inDecimal = 9
		}
		if outDecimal == 0 {
			outDecimal = 6
		}
	}
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
		TradePoolName: tradePoolName,
		// 输入代币精度
		InDecimal: inDecimal,
		// 输出代币精度
		OutDecimal: outDecimal,
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
		// 限价单/移动止损触发时用户不在场，由服务端持有的密钥代签并直接上链；
		// 翻倍出本的买单同样托管执行（后续自动卖单也要由服务端卖出）
		ServerSign: order.TradeType == int64(trade.TradeType_Limit) ||
			order.TradeType == int64(trade.TradeType_TokenCapLimit) ||
			order.TradeType == int64(trade.TradeType_TrailingStop) ||
			(order.DoubleOut == 1 && order.SwapType == int64(trade.SwapType_Buy)) ||
			(order.TrailingPercent > 0 && order.SwapType == int64(trade.SwapType_Buy)),
	}

	// to make and send tx
	var txHash string
	tryTimes := 0
	//  判断如果是自动滑点情况下，滑点过大的错误，那么增大滑点并重试
	// 滑点失败的根源是报价过期（链上价格已比 minOut 差），所以每次重试都走完整的
	// createAndSendTx：重新读池子 → 重新报价 → 新 blockhash 重建，而不是原样重发。
	for tryTimes == 0 || (param.IsAutoSlippage && errors.Is(err, xcode.SlippageLimit) && tryTimes < 3) {
		tryTimes++
		switch tryTimes {
		case 1:
		case 2:
			// 档位只升不降：用户滑点本来就 ≥ 档位时保持原值，仍然重试 —— 重建
			// 拿到新池子状态，光重新报价就可能救活这一单。
			param.Slippage = max(param.Slippage, 4500)
			l.Infof("AutoSlippageRetry try=%d slippage=%d", tryTimes, param.Slippage)
		case 3:
			param.Slippage = max(param.Slippage, 7000)
			l.Infof("AutoSlippageRetry try=%d slippage=%d", tryTimes, param.Slippage)
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

		// Triggered limit orders have no user present to sign — build, sign with
		// the server-held key and submit, returning a real tx hash.
		if param.ServerSign {
			var txHash string
			var err error
			for attempt := 1; attempt <= 3; attempt++ {
				txHash, err = l.svcCtx.SolTxMananger.BuildSignAndSend(l.ctx, param)
				if err == nil {
					break
				}
				l.Errorf("SolTxMananger.BuildSignAndSend attempt %d/3 err:%v", attempt, err)
				// A slippage failure won't pass at the same slippage — hand it to
				// the auto-slippage engine (outer loop) immediately so it escalates
				// the tier instead of burning same-tier resends here.
				if param.IsAutoSlippage && errors.Is(convertSwapErr(param.TradePoolName, err), xcode.SlippageLimit) {
					break
				}
			}
			return txHash, err
		}

		// The build issues several RPC calls to the (slow, rate-limited) public
		// devnet RPC; a single transient failure would surface to the user as a
		// generic 515. Retry the whole build a few times to absorb those.
		var unsignedTxBase64 string
		var err error
		for attempt := 1; attempt <= 3; attempt++ {
			unsignedTxBase64, err = l.svcCtx.SolTxMananger.BuildUnsignedTransaction(l.ctx, param)
			if err == nil {
				break
			}
			l.Errorf("SolTxMananger.BuildUnsignedTransaction attempt %d/3 err:%v", attempt, err)
		}
		if err != nil {
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
