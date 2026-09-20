package logic

import (
	"context"
	"fmt"

	"dex/model/trademodel"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfirmMarketOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmMarketOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmMarketOrderLogic {
	return &ConfirmMarketOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConfirmMarketOrder records the final signature for a buy the client signed
// and sent itself (CreateMarketOrder returned an unsigned tx + order_id for
// these). Storing tx_hash + moving the order to OnChain is exactly what
// updateDbByTxResult does for the server-signed path — from here the existing
// ticker (checkOnChainTx) picks the order up on its own, backfills the Final*
// fields from the indexed trade, moves it to Suc, and creates any attached
// double-out/trailing-stop follow-up leg. Without this call the order sits at
// Proc forever: invisible to the ticker, so the follow-up leg never gets
// created and the order never leaves Open Orders.
func (l *ConfirmMarketOrderLogic) ConfirmMarketOrder(in *trade.ConfirmMarketOrderRequest) (*trade.ConfirmMarketOrderResponse, error) {
	if in.OrderId <= 0 || (in.TxHash == "" && in.Error == "") {
		return nil, fmt.Errorf("order_id and one of tx_hash/error are required")
	}

	model := trademodel.NewTradeOrderModel(l.svcCtx.DB)
	order, err := model.FindOne(l.ctx, in.OrderId)
	if err != nil {
		return nil, fmt.Errorf("order %d not found: %w", in.OrderId, err)
	}

	if order.Status != int64(trade.OrderStatus_Proc) {
		// Already confirmed/failed/finalized — nothing to do, not an error
		// (the client may retry this call after a network hiccup).
		return &trade.ConfirmMarketOrderResponse{}, nil
	}

	if in.Error != "" {
		// User rejected the signature, or send/confirm failed client-side —
		// without this the order would sit at Proc ("Triggered" in the UI)
		// forever, since nothing else ever learns it didn't happen.
		order.Status = int64(trade.OrderStatus_Fail)
		order.FailReason = in.Error
		if err := model.UpdateOrderBySelect(l.ctx, order, "status", "fail_reason"); err != nil {
			return nil, fmt.Errorf("ConfirmMarketOrder fail-update err: %w", err)
		}
		l.Infof("ConfirmMarketOrder: order %d failed client-side: %s", order.Id, in.Error)
		return &trade.ConfirmMarketOrderResponse{}, nil
	}

	order.Status = int64(trade.OrderStatus_OnChain)
	order.TxHash = in.TxHash
	if err := model.UpdateOrderBySelect(l.ctx, order, "status", "tx_hash"); err != nil {
		return nil, fmt.Errorf("ConfirmMarketOrder update err: %w", err)
	}

	l.Infof("ConfirmMarketOrder: order %d confirmed with tx %s, handed off to ticker", order.Id, in.TxHash)
	return &trade.ConfirmMarketOrderResponse{}, nil
}
