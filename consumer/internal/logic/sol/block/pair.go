package block

import (
	"context"
	"dex/market/market"
	"dex/model/solmodel"
	"dex/pkg/constants"
	"dex/pkg/types"
	"dex/trade/trade"
	"dex/trade/tradeclient"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	pkgconstants "dex/pkg/constants"
	"dex/pkg/util"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

func (s *BlockService) SavePair(ctx context.Context, trade *types.TradeWithPair, tokenDb *solmodel.Token) (pairAtDB *solmodel.Pair, err error) {
	fmt.Println("准备插入信息中11111111111111")

	chainId := SolChainIdInt
	var tokenTotalSupply float64
	var tokenSymbol = trade.PairInfo.TokenSymbol
	if tokenDb != nil {
		tokenTotalSupply = tokenDb.TotalSupply
		tokenSymbol = tokenDb.Symbol
	}
	fmt.Println("准备插入信息中22222222222222")

	pairAtDB, err = s.sc.PairModel.FindOneByChainIdAddress(ctx, int64(chainId), trade.PairAddr)
	if err != nil {
		fmt.Println("777777777777777777err is:", err)
	}
	fmt.Println("准备插入信息中88888888888888")
	liq := trade.CurrentBaseTokenInPoolAmount*trade.BaseTokenPriceUSD + trade.CurrentTokenInPoolAmount*trade.TokenPriceUSD
	if liq == 0 {
		fmt.Println("准备插入信息中9999999999999")
		fmt.Println("trade.CLMMOpenPositionInfo.Liquidity is:")
		if trade.CLMMOpenPositionInfo != nil && trade.CLMMOpenPositionInfo.Liquidity != nil {
			fmt.Println("准备插入信息中qqqqqqqqqqqqqqq")

			u := trade.CLMMOpenPositionInfo.Liquidity
			bi := new(big.Int).Lsh(new(big.Int).SetUint64(u.Hi), 64)
			bi = bi.Add(bi, new(big.Int).SetUint64(u.Lo))
			f, _ := new(big.Float).SetInt(bi).Float64()
			liq = f

			fmt.Println("liq is:", liq)
		}
	}
	fmt.Println("准备插入信息中33333333333333")

	if trade.SwapName == pkgconstants.PumpFun {
		liq = trade.CurrentBaseTokenInPoolAmount * trade.BaseTokenPriceUSD * 2
	}

	baseTokenPrice := trade.BaseTokenPriceUSD
	tokenPrice := trade.TokenPriceUSD
	if baseTokenPrice == 0 {
		baseTokenPrice = 161.876662583626140000
	}
	if tokenPrice == 0 {
		tokenPrice = 0.000004522833952587
	}

	fmt.Println("准备插入信息中44444444444444")

	switch {
	case errors.Is(err, solmodel.ErrNotFound) || (err != nil && strings.Contains(err.Error(), "record not found")):
		fmt.Println("准备插入信息中55555555555555")
		var baseTokenIsNativeToken, baseTokenIsToken0 int64
		if trade.PairInfo.BaseTokenIsNativeToken {
			baseTokenIsNativeToken = 1
		}

		if trade.PairInfo.BaseTokenIsToken0 {
			baseTokenIsToken0 = 1
		}

		fmt.Println("trade.BaseTokenPriceUSD is:", baseTokenPrice)
		fmt.Println("trade.TokenPriceUSD is:", tokenPrice)

		pairAtDB = &solmodel.Pair{
			ChainId:                      int64(chainId),
			Address:                      trade.PairAddr,
			Name:                         trade.SwapName,
			FactoryAddress:               "",
			BaseTokenAddress:             trade.PairInfo.BaseTokenAddr,
			TokenAddress:                 trade.PairInfo.TokenAddr,
			BaseTokenSymbol:              util.GetBaseToken(SolChainIdInt).Symbol,
			TokenSymbol:                  tokenSymbol,
			BaseTokenDecimal:             int64(trade.PairInfo.BaseTokenDecimal),
			TokenDecimal:                 int64(trade.PairInfo.TokenDecimal),
			BaseTokenIsNativeToken:       baseTokenIsNativeToken,
			BaseTokenIsToken0:            baseTokenIsToken0,
			CurrentBaseTokenAmount:       trade.CurrentBaseTokenInPoolAmount,
			CurrentTokenAmount:           trade.CurrentTokenInPoolAmount,
			Fdv:                          tokenTotalSupply * trade.TokenPriceUSD,
			MktCap:                       tokenTotalSupply * trade.TokenPriceUSD,
			Liquidity:                    liq,
			BlockNum:                     trade.PairInfo.BlockNum,
			BlockTime:                    time.Unix(trade.BlockTime, 0),
			Slot:                         trade.Slot,
			PumpPoint:                    trade.PumpPoint,
			PumpLaunched:                 util.BoolToInt64(trade.PumpLaunched),
			PumpMarketCap:                trade.PumpMarketCap,
			PumpOwner:                    trade.PumpOwner,
			PumpSwapPairAddr:             trade.PumpSwapPairAddr,
			PumpVirtualBaseTokenReserves: trade.PumpVirtualBaseTokenReserves,
			PumpVirtualTokenReserves:     trade.PumpVirtualTokenReserves,
			PumpStatus:                   int64(trade.PumpStatus),
			PumpPairAddr:                 trade.PumpPairAddr,
			LatestTradeTime:              time.Unix(trade.BlockTime, 0),
			BaseTokenPrice:               baseTokenPrice,
			TokenPrice:                   tokenPrice,
		}

		trade.Mcap = pairAtDB.MktCap
		trade.Fdv = pairAtDB.Fdv

		if trade.PairInfo.InitBaseTokenAmount > 0 && trade.PairInfo.InitTokenAmount > 0 {
			pairAtDB.InitBaseTokenAmount = trade.PairInfo.InitBaseTokenAmount
			pairAtDB.InitTokenAmount = trade.PairInfo.InitTokenAmount
		}

		//you should push here
		// if pairAtDB.Name =PumpFun and PumpPoint==0 ,这就是新token

		// Push new pump.fun token creation to WebSocket
		if pairAtDB.Name == pkgconstants.PumpFun || pairAtDB.Name == "PumpFun" && pairAtDB.PumpPoint == 0 {
			go func() {
				fmt.Printf("🆕 [NEW PUMP TOKEN] Broadcasting: %s (%s)\n", pairAtDB.TokenSymbol, pairAtDB.TokenAddress)

				pushReq := &market.PushTokenInfoRequest{
					ChainId:      pairAtDB.ChainId,
					TokenAddress: pairAtDB.TokenAddress,
					PairAddress:  pairAtDB.Address,
					TokenPrice:   pairAtDB.TokenPrice,
					MktCap:       pairAtDB.MktCap,
					TokenName:    "", // Will be populated by market service from token database
					TokenSymbol:  pairAtDB.TokenSymbol,
					TokenIcon:    "", // Will be populated by market service from token database
					LaunchTime:   pairAtDB.BlockTime.Unix(),
					HoldCount:    0,   // Will be calculated by market service
					Change_24:    0.0, // Will be calculated by market service
					Txs_24H:      0,   // Will be calculated by market service
					PumpStatus:   int32(pairAtDB.PumpStatus),
				}

				_, err := s.sc.MarketService.PushTokenInfo(ctx, pushReq)
				if err != nil {
					fmt.Printf("❌ [NEW PUMP TOKEN] Failed to push: %v\n", err)
				} else {
					fmt.Printf("✅ [NEW PUMP TOKEN] Successfully pushed: %s\n", pairAtDB.TokenSymbol)
				}
			}()
		}

		err = s.sc.PairModel.Insert(ctx, pairAtDB)
		if err != nil {
			if strings.Contains(err.Error(), "Duplicate entry") {
				// db already exists
				pairAtDB, err = s.sc.PairModel.FindOneByChainIdAddress(ctx, int64(chainId), trade.PairAddr)
				if err != nil {
					return nil, err
				}
				return pairAtDB, nil
			}
			err = fmt.Errorf("PairModel.Insert err:%w", err)
			return
		}

	case err == nil:
		fmt.Println("准备插入信息中66666666666666")
		// 已存在，更新关键信息
		// Name/TokenSymbol are only ever set at insert time otherwise — if the
		// original insert happened before a decoder/source-tagging fix landed
		// (or before token metadata was indexed), the row stays wrong forever.
		// Self-heal both from the current trade on every update instead.
		if trade.SwapName != "" {
			pairAtDB.Name = trade.SwapName
		}
		if tokenSymbol != "" {
			pairAtDB.TokenSymbol = tokenSymbol
		}
		pairAtDB.CurrentBaseTokenAmount = trade.CurrentBaseTokenInPoolAmount
		pairAtDB.CurrentTokenAmount = trade.CurrentTokenInPoolAmount
		pairAtDB.Fdv = liq
		fmt.Println("trade.CurrentBaseTokenInPoolAmount is:", trade.CurrentBaseTokenInPoolAmount)
		fmt.Println("trade.BaseTokenPriceUSD is:", trade.BaseTokenPriceUSD)
		fmt.Println("trade.CurrentTokenInPoolAmount is:", trade.CurrentTokenInPoolAmount)
		fmt.Println("trade.TokenPriceUSD is:", trade.TokenPriceUSD)
		fmt.Println("pairAtDB.Fdv is:", pairAtDB.Fdv)
		if trade.SwapName == pkgconstants.PumpFun {
			pairAtDB.Liquidity = trade.CurrentBaseTokenInPoolAmount * trade.BaseTokenPriceUSD * 2
		}
		pairAtDB.BaseTokenPrice = baseTokenPrice
		pairAtDB.TokenPrice = tokenPrice
		pairAtDB.Slot = trade.Slot
		pairAtDB.BlockTime = time.Unix(trade.BlockTime, 0)
		pairAtDB.Liquidity = liq

		// 其它可选字段也可同步更新
		// 保存到数据库
		err = s.sc.PairModel.Update(ctx, pairAtDB)
		if err != nil {
			err = fmt.Errorf("PairModel.Update err:%w", err)
			return
		}

		// 同步 trade 的市值等
		trade.Mcap = pairAtDB.MktCap
		trade.Fdv = pairAtDB.Fdv
		return
	default:
		err = fmt.Errorf("PairModel.FindOneByChainIdAddress err:%w", err)
		return
	}
	// logx.Infof("SavePair:%v db token price: %v, trade token price: %v", trade.PairAddr, pairAtDB.TokenPrice, trade.TokenPriceUSD)

	// 默认值
	trade.Mcap = pairAtDB.MktCap
	trade.Fdv = pairAtDB.Fdv

	if trade.Slot > pairAtDB.Slot {
		// s.Infof("SavePair will UpdatePairDBPoint slot: %v, db slot: %v, hash: %v, pair address: %v", trade.Slot, pairAtDB.Slot, trade.TxHash, trade.PairAddr)

		if pairAtDB.InitBaseTokenAmount == 0 || pairAtDB.InitTokenAmount == 0 {
			if trade.PairInfo.InitBaseTokenAmount > 0 && trade.PairInfo.InitTokenAmount > 0 {
				pairAtDB.InitBaseTokenAmount = trade.PairInfo.InitBaseTokenAmount
				pairAtDB.InitTokenAmount = trade.PairInfo.InitTokenAmount
			}
		}

		// s.initAmount(pairAtDB)

		pairAtDB.TokenSymbol = tokenSymbol
		pairAtDB.Slot = trade.Slot
		pairAtDB.Liquidity = liq
		err = UpdatePairDBPoint(trade, pairAtDB, tokenTotalSupply)
		if err != nil {
			err = fmt.Errorf("UpdatePairDBPoint err:%w", err)
			return
		}
		pairAtDB.BaseTokenPrice = baseTokenPrice
		pairAtDB.TokenPrice = tokenPrice

		trade.Mcap = pairAtDB.MktCap
		trade.Fdv = pairAtDB.Fdv

		err = s.sc.PairModel.Update(ctx, pairAtDB)
		if err != nil {
			err = fmt.Errorf("PairModel.Update err:%w", err)
			return
		}
	}

	return
}

// UpdatePairDBPoint updates trading information with delayed price updates.
func UpdatePairDBPoint(trade *types.TradeWithPair, pairDB *solmodel.Pair, tokenTotalSupply float64) error {
	currentTokenInPoolAmount := trade.CurrentTokenInPoolAmount
	currentBaseTokenInPoolAmount := trade.CurrentBaseTokenInPoolAmount
	baseTokenPriceUSD := trade.BaseTokenPriceUSD
	tokenPriceUSD := trade.TokenPriceUSD
	tradeTime := trade.BlockTime

	if pairDB.InitTokenAmount == 0 || pairDB.InitBaseTokenAmount == 0 {
		if trade.PairInfo.InitTokenAmount > 0 && trade.PairInfo.InitBaseTokenAmount > 0 {
			pairDB.InitTokenAmount = trade.PairInfo.InitTokenAmount
			pairDB.InitBaseTokenAmount = trade.PairInfo.InitBaseTokenAmount
			logx.Infof("UpdatePairDBPoint:update init token amount,swapName: %v, %v,%v", trade.SwapName, pairDB.InitTokenAmount, pairDB.InitBaseTokenAmount)
		}
	}

	pairDB.PumpPoint = trade.PumpPoint
	pairDB.PumpStatus = int64(trade.PumpStatus)
	pairDB.PumpVirtualBaseTokenReserves = trade.PumpVirtualBaseTokenReserves
	pairDB.PumpVirtualTokenReserves = trade.PumpVirtualTokenReserves
	// logx.Infof("UpdatePairDBPoint:update token address: %v pump ponit: %v", trade.PairInfo.TokenAddr, pairDB.PumpPoint)

	// Reset token price if base token liquidity is critically low, unless from specific swap types.
	// if trade.SwapName != util.SwapNamePump && currentBaseTokenInPoolAmount > 0 && currentBaseTokenInPoolAmount < 0.01 {
	// 	tokenPriceUSD = 0
	// }

	// Return early if the trade is older than the last update.
	// if tradeTime < pairDB.LatestTradeTime.Unix() {
	// 	return nil
	// }

	// Update token and base token prices only if valid.
	if tokenPriceUSD > 0 {
		pairDB.TokenPrice = tokenPriceUSD
		// logx.Infof("UpdatePairDBPoint %v db price:%v, trade price %v,", pairDB.Address, pairDB.TokenPrice, trade.TokenPriceUSD)
		// if trade.TokenPriceUSD != pairDB.TokenPrice {
		// 	logx.Infof("Diff UpdatePairDBPoint %v db price:%v, trade price %v,", pairDB.Address, pairDB.TokenPrice, trade.TokenPriceUSD)
		// }
	}
	pairDB.BaseTokenPrice = baseTokenPriceUSD

	// Update FDV (fully diluted valuation) based on token supply.
	if tokenTotalSupply > 0 {
		pairDB.Fdv = decimal.NewFromFloat(tokenPriceUSD).Mul(decimal.NewFromFloat(tokenTotalSupply)).InexactFloat64()
		pairDB.MktCap = decimal.NewFromFloat(tokenPriceUSD).Mul(decimal.NewFromFloat(tokenTotalSupply)).InexactFloat64()
	}

	// Update current liquidity only if both amounts are positive.
	if currentBaseTokenInPoolAmount > 0 && currentTokenInPoolAmount > 0 {
		pairDB.CurrentBaseTokenAmount = currentBaseTokenInPoolAmount
		pairDB.CurrentTokenAmount = currentTokenInPoolAmount
	}

	// Update the latest trade time.
	pairDB.LatestTradeTime = time.Unix(tradeTime, 0)

	// Calculate market cap based on the current liquidity and prices.
	if pairDB.Name == pkgconstants.PumpFun {
		pairDB.Liquidity = decimal.NewFromFloat(baseTokenPriceUSD).Mul(decimal.NewFromFloat(pairDB.CurrentBaseTokenAmount)).Mul(decimal.NewFromFloat(2)).InexactFloat64()
	} else {
		pairDB.Liquidity = decimal.NewFromFloat(tokenPriceUSD).Mul(decimal.NewFromFloat(pairDB.CurrentTokenAmount)).
			Add(decimal.NewFromFloat(baseTokenPriceUSD).Mul(decimal.NewFromFloat(pairDB.CurrentBaseTokenAmount))).InexactFloat64()
	}

	// pairDB.MktCap = tokenPriceUSD*pairDB.CurrentTokenAmount + baseTokenPriceUSD*pairDB.CurrentBaseTokenAmount

	// TODO: Update pair cache.
	// pair.PairCache.Update(pairDB)
	return nil
}

// sendTokenPrice2Trade sends token price information to Trade service
func (s *BlockService) sendTokenPrice2Trade(ctx context.Context, solPrice float64, tradeWithPair *types.TradeWithPair) {
	if tradeWithPair.Type != types.TradeTypeBuy && tradeWithPair.Type != types.TradeTypeSell {
		s.Infof("sendTokenPrice2Trade tradeWithPair type in not buy or sell,tx hash: %v", tradeWithPair.TxHash)
		return
	}

	var swapType trade.SwapType
	if tradeWithPair.Type == types.TradeTypeBuy {
		swapType = trade.SwapType_Buy
	} else {
		swapType = trade.SwapType_Sell
	}

	token2BasePrice := decimal.NewFromFloat(tradeWithPair.TokenPriceUSD).Div(decimal.NewFromFloat(solPrice)).String()

	_, err := s.sc.TradeService.ProcTokenPrice(ctx, &tradeclient.ProcTokenPriceRequest{
		TokenCa:  tradeWithPair.PairInfo.TokenAddr,
		Price:    token2BasePrice,
		SwapType: swapType,
		ChainId:  int64(constants.SolChainIdInt),
	})
	if err != nil {
		logx.Errorf("sendTokenPrice2Trade failed: pair=%v, err=%v", tradeWithPair.PairAddr, err)
		return
	}
}
