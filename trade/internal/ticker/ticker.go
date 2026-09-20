// Package ticker confirms on-chain fills for trade orders and drives the
// double-out (翻倍出本) flow.
//
// Every tick it scans orders stuck at status=OnChain and looks their tx hash up
// in the consumer-built trade_YYYY_MM_DD shard tables — our own indexed view of
// on-chain facts (cheaper than RPC, and it carries the fill price/amount, which
// getSignatureStatuses doesn't). A hit backfills the Final* fields, moves the
// order OnChain→Suc, and — when the order is a double-out buy — auto-creates a
// limit sell at 2x the fill price for half the fill amount, entirely reusing
// the limit-order machinery (MySQL + Redis trigger list + matcher).
//
// The shard tables are not complete: the consumer's throttled block fetch drops
// slots under load (consumer slot/ws.go), so a fill can be final on-chain yet
// never appear there. Orders older than the normal indexing lag therefore fall
// back to getTransaction (budgeted per tick to spare the devnet RPC), which
// either confirms them from token-balance deltas or fails them terminally.
package ticker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dex/model/solmodel"
	"dex/model/trademodel"
	"dex/pkg/constants"
	"dex/pkg/xredis"
	"dex/trade/internal/logic"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	bin "github.com/gagliardetto/binary"
	aSDK "github.com/gagliardetto/solana-go"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

const (
	tickInterval = 2 * time.Second
	// One instance scans at a time; two instances confirming the same buy
	// would create two double-out sells.
	lockKey    = "dex:ticker:check_onchain_tx:lock"
	lockExpire = 30 // seconds
	scanBatch  = 20
	// The devnet consumer trails the chain tip (throttled block fetch), so
	// don't time orders out aggressively — just stop rescanning stale ones.
	scanWindow = 24 * time.Hour

	// Past this age, go ask the chain directly instead of waiting for the
	// consumer to index the block. Client-signed orders arrive here already
	// confirmed (ConfirmMarketOrder is called after confirmTransaction), so a
	// long wait is pure dead time before the double-out/trailing sell can be
	// created; finality lands ~13s after confirmation, and a not-yet-final tx
	// just retries next tick. Was 2 minutes — users saw the follow-up sell
	// appear well over a minute after the buy.
	rpcFallbackAfter = 2 * time.Second
	// getTransaction calls per tick; keeps worst-case RPC load ~1.5 req/s
	// (devnet public RPC 429s well below its nominal limits).
	rpcChecksPerTick = 3
	// A tx's blockhash dies ~1.5 min after submission, so a tx still absent
	// from the chain this long after creation can never land — fail the order.
	rpcAbandonAfter = 15 * time.Minute
)

var two = decimal.NewFromInt(2)

func init() {
	// Column type is decimal(32,18); shopspring's default Div precision (16)
	// would silently grind the last two digits off 18-decimal prices.
	if decimal.DivisionPrecision < 18 {
		decimal.DivisionPrecision = 18
	}
}

type TradeTicker struct {
	svcCtx *svc.ServiceContext
	done   chan struct{}
}

func NewTradeTicker(svcCtx *svc.ServiceContext) *TradeTicker {
	return &TradeTicker{svcCtx: svcCtx, done: make(chan struct{})}
}

func (t *TradeTicker) Stop() { close(t.done) }

func (t *TradeTicker) Start() {
	tk := time.NewTicker(tickInterval)
	defer tk.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-tk.C:
			t.tick()
		}
	}
}

func (t *TradeTicker) tick() {
	ctx := context.Background()
	lock, ok, err := xredis.Lock(ctx, t.svcCtx.Redis, lockKey, lockExpire)
	if err != nil {
		logx.Errorf("ticker: lock err: %v", err)
		return
	}
	if !ok {
		return // another instance holds this tick
	}
	defer xredis.ReleaseLock(lock)

	t.checkOnChainTx(ctx)
}

// checkOnChainTx confirms submitted orders against the indexed trade tables.
func (t *TradeTicker) checkOnChainTx(ctx context.Context) {
	var orders []*trademodel.TradeOrder
	err := t.svcCtx.DB.WithContext(ctx).
		Where("status = ? AND tx_hash <> '' AND created_at > ?",
			int64(trade.OrderStatus_OnChain), time.Now().Add(-scanWindow)).
		Order("id ASC").Limit(scanBatch).
		Find(&orders).Error
	if err != nil {
		logx.Errorf("ticker: scan onchain orders err: %v", err)
		return
	}

	rpcBudget := rpcChecksPerTick
	for _, order := range orders {
		if err := t.confirmOrder(ctx, order, &rpcBudget); err != nil {
			logx.Errorf("ticker: confirm order %d err: %v", order.Id, err)
		}
	}
}

func (t *TradeTicker) confirmOrder(ctx context.Context, order *trademodel.TradeOrder, rpcBudget *int) error {
	tradeRow, err := solmodel.NewTradeModel(t.svcCtx.DB).
		FindOneByTxHashWithTime(ctx, order.TxHash, order.CreatedAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Not indexed. Young orders just wait for the consumer; older ones are
		// likely in a dropped block and would otherwise stay OnChain forever.
		if time.Since(order.CreatedAt) > rpcFallbackAfter && *rpcBudget > 0 {
			*rpcBudget--
			return t.confirmOrderViaRPC(ctx, order)
		}
		return nil
	}
	if err != nil {
		return err
	}

	// Backfill the Final* family from the on-chain fact. Order* is what the
	// user asked for; Final* is what actually happened — slippage lives in the
	// gap, and double-out must be computed from Final (see lesson 21_3).
	order.FinalAmount = decimal.NewFromFloat(tradeRow.TokenAmount) // buy: tokens received
	order.FinalValueBase = decimal.NewFromFloat(tradeRow.BaseTokenAmount)
	if order.SwapType == int64(trade.SwapType_Sell) {
		order.FinalAmount = decimal.NewFromFloat(tradeRow.BaseTokenAmount) // sell: SOL received
	}
	order.FinalBasePrice = decimal.NewFromFloat(tradeRow.BaseTokenPriceUsd)
	if tradeRow.BaseTokenPriceUsd > 0 {
		order.FinalPriceBase = decimal.NewFromFloat(tradeRow.TokenPriceUsd).
			Div(decimal.NewFromFloat(tradeRow.BaseTokenPriceUsd))
	}

	return t.finalizeOrder(ctx, order)
}

// finalizeOrder moves a backfilled order OnChain→Suc and kicks off double-out.
// CAS: state moves one way, so the next tick can't re-confirm this order (and
// re-create the double-out sell). rowsAffected==0 means someone else won the
// transition — stand down.
func (t *TradeTicker) finalizeOrder(ctx context.Context, order *trademodel.TradeOrder) error {
	model := trademodel.NewTradeOrderModel(t.svcCtx.DB)
	rowsAffected, err := model.UpdateOrderStatus(ctx, order,
		int64(trade.OrderStatus_OnChain), int64(trade.OrderStatus_Suc))
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return nil
	}
	order.Status = int64(trade.OrderStatus_Suc)

	if err := model.UpdateOrderBySelect(ctx, order,
		"final_amount", "final_value_base", "final_price_base", "final_base_price"); err != nil {
		logx.Errorf("ticker: backfill final fields order %d err: %v", order.Id, err)
	}
	logx.Infof("ticker: order %d confirmed on-chain (tx %s), final amount %s", order.Id, order.TxHash, order.FinalAmount.String())

	if err := t.processDoubleOut(ctx, order); err != nil {
		return err
	}
	return t.processTrailingAttach(ctx, order)
}

// confirmOrderViaRPC settles an order the consumer never indexed by asking the
// chain itself. Terminal either way when the chain has an answer: a found tx
// confirms (with Final* rebuilt from balance deltas) or fails on its meta.Err;
// a tx still absent long after its blockhash expired can never land, so the
// order fails too. Only transient RPC errors leave it for the next tick.
func (t *TradeTicker) confirmOrderViaRPC(ctx context.Context, order *trademodel.TradeOrder) error {
	tm := t.svcCtx.SolTxMananger
	if tm == nil || tm.Client == nil {
		return nil
	}
	sig, err := aSDK.SignatureFromBase58(order.TxHash)
	if err != nil {
		return fmt.Errorf("bad tx hash %q: %w", order.TxHash, err)
	}
	maxVer := uint64(0)
	res, err := tm.Client.GetTransaction(ctx, sig, &ag_rpc.GetTransactionOpts{
		Encoding:                       aSDK.EncodingBase64,
		// The client reported this tx after confirmTransaction('confirmed'),
		// so it is visible at confirmed right away; finalized would add ~13s.
		Commitment:                     ag_rpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &maxVer,
	})
	if errors.Is(err, ag_rpc.ErrNotFound) {
		if time.Since(order.CreatedAt) > rpcAbandonAfter {
			return t.failOrder(ctx, order, "transaction never landed on-chain (blockhash expired)")
		}
		return nil
	}
	if err != nil {
		return err // transient RPC error — next tick
	}
	if res.Meta == nil {
		return nil
	}
	if res.Meta.Err != nil {
		return t.failOrder(ctx, order, fmt.Sprintf("transaction failed on-chain: %v", res.Meta.Err))
	}

	if err := t.backfillFromTx(order, res); err != nil {
		return fmt.Errorf("rpc backfill: %w", err)
	}
	logx.Infof("ticker: order %d confirmed via RPC fallback (tx %s, consumer missed the block)", order.Id, order.TxHash)
	return t.finalizeOrder(ctx, order)
}

// backfillFromTx rebuilds the Final* family from the transaction's balance
// deltas — the same facts the consumer would have decoded, read the hard way.
func (t *TradeTicker) backfillFromTx(order *trademodel.TradeOrder, res *ag_rpc.GetTransactionResult) error {
	parsedTx, err := aSDK.TransactionFromDecoder(bin.NewBinDecoder(res.Transaction.GetBinary()))
	if err != nil {
		return fmt.Errorf("decode tx: %w", err)
	}
	wallet, err := aSDK.PublicKeyFromBase58(order.WalletAddress)
	if err != nil {
		return fmt.Errorf("bad wallet %q: %w", order.WalletAddress, err)
	}
	mint, err := aSDK.PublicKeyFromBase58(order.TokenCa)
	if err != nil {
		return fmt.Errorf("bad token ca %q: %w", order.TokenCa, err)
	}

	walletIdx := -1
	for i, key := range parsedTx.Message.AccountKeys {
		if key.Equals(wallet) {
			walletIdx = i
			break
		}
	}
	if walletIdx < 0 || walletIdx >= len(res.Meta.PreBalances) || walletIdx >= len(res.Meta.PostBalances) {
		return fmt.Errorf("wallet %s not in tx accounts", order.WalletAddress)
	}

	tokenDelta := tokenBalanceDelta(res.Meta, wallet, mint).Abs()

	// The wallet's lamport delta, net of the tx fee (it is the fee payer on
	// every tx we build) and of rent deposited into accounts this tx created
	// (e.g. the buyer's new ATA). What remains is the SOL side of the swap.
	lamports := decimal.NewFromInt(int64(res.Meta.PostBalances[walletIdx]) - int64(res.Meta.PreBalances[walletIdx]))
	if walletIdx == 0 {
		lamports = lamports.Add(decimal.NewFromInt(int64(res.Meta.Fee)))
	}
	for _, tb := range res.Meta.PostTokenBalances {
		i := int(tb.AccountIndex)
		if i < len(res.Meta.PreBalances) && i < len(res.Meta.PostBalances) && res.Meta.PreBalances[i] == 0 {
			lamports = lamports.Add(decimal.NewFromInt(int64(res.Meta.PostBalances[i])))
		}
	}
	swapSol := lamports.Abs().Div(decimal.NewFromInt(int64(aSDK.LAMPORTS_PER_SOL)))

	if !tokenDelta.IsPositive() || !swapSol.IsPositive() {
		return fmt.Errorf("no usable balance change (tokens %s, sol %s)", tokenDelta.String(), swapSol.String())
	}

	order.FinalAmount = tokenDelta // buy: tokens received
	if order.SwapType == int64(trade.SwapType_Sell) {
		order.FinalAmount = swapSol // sell: SOL received
	}
	order.FinalValueBase = swapSol
	order.FinalBasePrice = decimal.NewFromInt(constants.NominalSolPriceUsd)
	order.FinalPriceBase = swapSol.Div(tokenDelta)
	return nil
}

// tokenBalanceDelta sums the owner's post-minus-pre balance across all token
// accounts of the given mint touched by the tx.
func tokenBalanceDelta(meta *ag_rpc.TransactionMeta, owner, mint aSDK.PublicKey) decimal.Decimal {
	sum := func(list []ag_rpc.TokenBalance) decimal.Decimal {
		total := decimal.Zero
		for _, tb := range list {
			if tb.Owner == nil || !tb.Owner.Equals(owner) || !tb.Mint.Equals(mint) || tb.UiTokenAmount == nil {
				continue
			}
			if v, err := decimal.NewFromString(tb.UiTokenAmount.UiAmountString); err == nil {
				total = total.Add(v)
			}
		}
		return total
	}
	return sum(meta.PostTokenBalances).Sub(sum(meta.PreTokenBalances))
}

// failOrder CASes OnChain→Fail so provably dead orders stop being rescanned
// (and stop burning the RPC budget) forever.
func (t *TradeTicker) failOrder(ctx context.Context, order *trademodel.TradeOrder, reason string) error {
	model := trademodel.NewTradeOrderModel(t.svcCtx.DB)
	rowsAffected, err := model.UpdateOrderStatus(ctx, order,
		int64(trade.OrderStatus_OnChain), int64(trade.OrderStatus_Fail))
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return nil
	}
	order.Status = int64(trade.OrderStatus_Fail)
	order.FailReason = reason
	if err := model.UpdateOrderBySelect(ctx, order, "fail_reason"); err != nil {
		logx.Errorf("ticker: save fail reason order %d err: %v", order.Id, err)
	}
	logx.Infof("ticker: order %d failed: %s", order.Id, reason)
	return nil
}

// processTrailingAttach turns a confirmed BUY that carries trailing_percent
// into an automatic trailing stop sell for the whole fill, anchored at the
// actual fill price — protection starts the moment the position exists, with
// a better anchor than a hand-placed stop minutes later would get.
func (t *TradeTicker) processTrailingAttach(ctx context.Context, order *trademodel.TradeOrder) error {
	// The Buy check stops any recursion: the auto-created stop is a sell (and
	// carries the same trailing_percent as lineage for the UI).
	if order.Status != int64(trade.OrderStatus_Suc) ||
		order.TrailingPercent <= 0 ||
		order.SwapType != int64(trade.SwapType_Buy) {
		return nil
	}

	anchor := order.FinalPriceBase
	amount := order.FinalAmount
	if !anchor.IsPositive() || !amount.IsPositive() {
		logx.Errorf("ticker: trailing-attach order %d has unusable fill (price %s, amount %s), skip",
			order.Id, anchor.String(), amount.String())
		return nil
	}

	stopOrder := &trademodel.TradeOrder{
		ChainId:        order.ChainId,
		TradeType:      int64(trade.TradeType_TrailingStop),
		GasType:        order.GasType,
		IsAutoSlippage: order.IsAutoSlippage,
		Slippage:       1000, // 10%, same as hand-placed trailing stops
		IsAntiMev:      order.IsAntiMev,
		TokenCa:        order.TokenCa,
		SwapType:       int64(trade.SwapType_Sell),
		WalletAddress:  order.WalletAddress, // server wallet (custodial buy)
		OrderCap:       order.OrderCap,
		OrderAmount:    amount,
		OrderPriceBase: anchor,
		OrderValueBase: anchor.Mul(amount),
		OrderBasePrice: order.FinalBasePrice,
		DrawdownPrice:  logic.CalculateDrawDownPrice(anchor, int(order.TrailingPercent)),
		// lineage: same N as the buy that spawned it
		TrailingPercent: order.TrailingPercent,
		Status:          int64(trade.OrderStatus_Waiting),
	}

	// Same entrance as a hand-placed trailing stop: MySQL insert + trigger
	// note into the per-token Redis list. From here on, the ratchet owns it.
	if err := logic.NewCreateTrailingStopLogic(ctx, t.svcCtx).CreateTrailingStopOrder(stopOrder); err != nil {
		return err
	}
	logx.Infof("ticker: trailing stop %d auto-created from buy %d: sell %s, anchor %s, trigger %s (token/SOL, -%d%%)",
		stopOrder.Id, order.Id, amount.String(), anchor.String(), stopOrder.DrawdownPrice.String(), order.TrailingPercent)
	return nil
}

// processDoubleOut turns a confirmed double-out BUY into an automatic limit
// sell: price ×2, amount ÷2 — (amount/2)×(price×2) equals the original cost,
// so the fill returns the principal and the remaining half rides for free.
func (t *TradeTicker) processDoubleOut(ctx context.Context, order *trademodel.TradeOrder) error {
	// Triple guard. The Buy check is what stops the loop: the auto-created
	// sell also carries double_out=1 (lineage, shown in the UI), and without
	// this check its own fill would spawn yet another sell, forever.
	if order.Status != int64(trade.OrderStatus_Suc) ||
		order.DoubleOut != 1 ||
		order.SwapType != int64(trade.SwapType_Buy) {
		return nil
	}

	// finalizeOrder receives a row that was backfilled in memory. Reload it
	// after the CAS/update so the child order is built from the values actually
	// persisted by the consumer or RPC fallback, even when the ticker raced a
	// client confirmation or another worker.
	if persisted, err := trademodel.NewTradeOrderModel(t.svcCtx.DB).FindOne(ctx, order.Id); err == nil {
		order = persisted
	}

	sellPrice := order.FinalPriceBase.Mul(two)
	amount := order.FinalAmount.Div(two)
	if !sellPrice.IsPositive() || !amount.IsPositive() {
		logx.Errorf("ticker: double-out order %d has unusable fill (price %s, amount %s), skip",
			order.Id, sellPrice.String(), amount.String())
		return nil
	}

	sellOrder := &trademodel.TradeOrder{
		ChainId:        order.ChainId,
		TradeType:      int64(trade.TradeType_Limit),
		GasType:        order.GasType,
		IsAutoSlippage: order.IsAutoSlippage,
		Slippage:       order.Slippage,
		IsAntiMev:      order.IsAntiMev,
		TokenCa:        order.TokenCa,
		SwapType:       int64(trade.SwapType_Sell),
		DoubleOut:      1, // lineage: this sell was auto-created by double-out
		WalletAddress:  order.WalletAddress,
		OrderCap:       order.OrderCap.Mul(two),
		OrderAmount:    amount,
		OrderPriceBase: sellPrice,
		OrderValueBase: sellPrice.Mul(amount),
		OrderBasePrice: order.FinalBasePrice,
		Status:         int64(trade.OrderStatus_Waiting),
	}

	// Same entrance as a hand-placed limit order: MySQL insert + RPush into
	// the per-token Redis trigger list. From here on, 7.12's matcher owns it.
	if err := logic.NewCreateLimitOrderLogic(ctx, t.svcCtx).CreateTradeOrder(sellOrder); err != nil {
		return err
	}
	logx.Infof("ticker: double-out sell %d auto-created from buy %d: sell %s @ %s (token/SOL)",
		sellOrder.Id, order.Id, amount.String(), sellPrice.String())
	return nil
}
