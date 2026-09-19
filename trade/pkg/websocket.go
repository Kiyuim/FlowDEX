package pkg

// "dex/websocket/websocket"

// func WsPushOrder(ctx context.Context, order *trademodel.TradeOrder, marketClient market.MarketClient, websocketClient websocket.WebsocketClient) error {
// 	token, err := marketClient.GetTokenInfo(ctx, &market.GetTokenInfoRequest{
// 		ChainId:      order.ChainId,
// 		TokenAddress: order.TokenCa,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	_, err = websocketClient.PushTradeOrder(ctx, &websocket.TradeOrderRequest{
// 		Uid:            order.Uid,
// 		TradeType:      order.TradeType,
// 		ChainId:        order.ChainId,
// 		TokenCa:        order.TokenCa,
// 		SwapType:       order.SwapType,
// 		WalletIndex:    order.WalletIndex,
// 		WalletAddress:  order.WalletAddress,
// 		Slippage:       order.Slippage,
// 		IsAntiMev:      order.IsAntiMev,
// 		GasType:        order.GasType,
// 		Status:         order.Status,
// 		OrderCap:       order.OrderCap.String(),
// 		OrderAmount:    order.OrderAmount.String(),
// 		OrderPriceBase: order.OrderPriceBase.String(),
// 		OrderValueBase: order.OrderValueBase.String(),
// 		OrderBasePrice: order.OrderBasePrice.String(),
// 		FinalCap:       order.FinalCap.String(),
// 		FinalAmount:    order.FinalAmount.String(),
// 		FinalPriceBase: order.FinalPriceBase.String(),
// 		FinalValueBase: order.FinalValueBase.String(),
// 		FinalBasePrice: order.FinalBasePrice.String(),
// 		TxHash:         order.TxHash,
// 		DexName:        order.DexName,
// 		PairCa:         order.PairCa,
// 		TokenSymbol:    token.Symbol,
// 		IsAutoSlippage: int32(order.IsAutoSlippage),
// 		FailReason:     order.FailReason,
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
