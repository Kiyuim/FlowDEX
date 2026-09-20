import React, { useEffect, useState, useCallback } from 'react';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import toast from 'react-hot-toast';
import {
  submitMarketOrder,
  submitLimitOrder,
  submitTrailingStop,
  getTokenBalance,
  SWAP_BUY,
  SWAP_SELL,
} from '../lib/trade';

const BUY_PRESETS = [0.01, 0.05, 0.1, 0.5, 1];
const SELL_PRESETS = [25, 50, 75, 100];
const LIMIT_PCT_PRESETS = [-10, -5, 5, 10];
const TRAIL_PRESETS = [10, 20, 30, 50];

export default function TradePanel({ token, currentPriceUsd, onLimitOrderPlaced, onTradeComplete }) {
  const { connection } = useConnection();
  const { publicKey, connected, signTransaction, sendTransaction } = useWallet();
  const [side, setSide] = useState('buy');
  const [mode, setMode] = useState('market');
  const [amount, setAmount] = useState('');
  const [limitPrice, setLimitPrice] = useState('');
  const [trailPct, setTrailPct] = useState('20');
  const [doubleOut, setDoubleOut] = useState(false);
  const [attachTrail, setAttachTrail] = useState(false);
  const [autoSlippage, setAutoSlippage] = useState(true);
  const [sellPct, setSellPct] = useState(0);
  const [balance, setBalance] = useState(0);
  const [solBalance, setSolBalance] = useState(0);
  const [busy, setBusy] = useState(false);

  const mint = token?.tokenAddress;
  const symbol = token?.tokenSymbol || token?.tokenName || 'TOKEN';

  const refreshBalances = useCallback(async () => {
    if (!connected || !publicKey) return;
    if (mint) setBalance(await getTokenBalance(connection, publicKey.toString(), mint));
    try {
      setSolBalance((await connection.getBalance(publicKey)) / 1e9);
    } catch {}
  }, [connection, publicKey, connected, mint]);

  useEffect(() => {
    refreshBalances();
  }, [refreshBalances]);

  const submitLimit = async () => {
    const price = parseFloat(limitPrice);
    if (!price || price <= 0) return toast.error('Enter a limit price (USD)');
    let amountIn;
    if (side === 'buy') {
      amountIn = parseFloat(amount);
      if (!amountIn || amountIn <= 0) return toast.error('Enter a SOL amount');
    } else {
      amountIn = sellPct > 0 ? (balance * sellPct) / 100 : parseFloat(amount);
      if (!amountIn || amountIn <= 0) return toast.error('Enter an amount to sell');
    }
    setBusy(true);
    const t = toast.loading('Placing limit order…');
    try {
      const orderId = await submitLimitOrder({
        tokenCa: mint,
        swapType: side === 'buy' ? SWAP_BUY : SWAP_SELL,
        amount: amountIn,
        priceUsd: limitPrice,
        doubleOut: side === 'buy' && doubleOut,
        autoSlippage,
      });
      toast.success(
        `Limit ${side} #${orderId} placed — triggers when price ${side === 'buy' ? '≤' : '≥'} $${price}`,
        { id: t, duration: 8000 }
      );
      setAmount('');
      setLimitPrice('');
      setSellPct(0);
      onLimitOrderPlaced?.();
    } catch (e) {
      toast.error(`Limit order failed: ${e.message || e}`, { id: t });
    } finally {
      setBusy(false);
    }
  };

  const submitTrailing = async () => {
    const pct = parseInt(trailPct, 10);
    if (!pct || pct <= 0 || pct >= 100) return toast.error('Trailing % must be between 1 and 99');
    const amountIn = sellPct > 0 ? (balance * sellPct) / 100 : parseFloat(amount);
    if (!amountIn || amountIn <= 0) return toast.error('Enter an amount to sell');
    setBusy(true);
    const t = toast.loading('Placing trailing stop…');
    try {
      const orderId = await submitTrailingStop({
        tokenCa: mint,
        amount: amountIn,
        trailingPercent: pct,
        autoSlippage,
      });
      toast.success(
        `Trailing stop #${orderId} placed — sells when price drops ${pct}% from its post-order high`,
        { id: t, duration: 8000 }
      );
      setAmount('');
      setSellPct(0);
      onLimitOrderPlaced?.();
    } catch (e) {
      toast.error(`Trailing stop failed: ${e.message || e}`, { id: t });
    } finally {
      setBusy(false);
    }
  };

  const submit = async () => {
    if (!connected) return toast.error('Connect your wallet first');
    if (mode === 'limit') return submitLimit();
    if (mode === 'trailing') return submitTrailing();
    let amountIn;
    if (side === 'buy') {
      amountIn = parseFloat(amount);
      if (!amountIn || amountIn <= 0) return toast.error('Enter a SOL amount');
    } else {
      amountIn = sellPct > 0 ? (balance * sellPct) / 100 : parseFloat(amount);
      if (!amountIn || amountIn <= 0) return toast.error('Enter an amount to sell');
    }
    const trailAttachPct = side === 'buy' && attachTrail ? parseInt(trailPct, 10) : 0;
    if (side === 'buy' && attachTrail && (!trailAttachPct || trailAttachPct <= 0 || trailAttachPct >= 100)) {
      return toast.error('Trailing % must be between 1 and 99');
    }
    setBusy(true);
    const t = toast.loading(`Submitting ${side} order…`);
    try {
      const sig = await submitMarketOrder({
        connection,
        publicKey,
        signTransaction,
        sendTransaction,
        tokenCa: mint,
        swapType: side === 'buy' ? SWAP_BUY : SWAP_SELL,
        amountIn,
        doubleOut: side === 'buy' && doubleOut,
        trailingPercent: trailAttachPct,
        autoSlippage,
      });
      toast.success(
        <span>
          {side === 'buy' ? 'Bought' : 'Sold'} {symbol} ·{' '}
          <a
            className="underline text-accent"
            href={`https://solscan.io/tx/${sig}?cluster=devnet`}
            target="_blank"
            rel="noreferrer"
          >
            view
          </a>
        </span>,
        { id: t, duration: 8000 }
      );
      setAmount('');
      setSellPct(0);
      if (side === 'buy' && (doubleOut || attachTrail)) {
        setDoubleOut(false);
        setAttachTrail(false);
        onLimitOrderPlaced?.(); // the order (and later its auto sell) shows in the orders panel
      }
      // The tx is confirmed at this point, so balances can be re-read now;
      // read again shortly after in case the RPC node lags the confirmation.
      refreshBalances();
      setTimeout(refreshBalances, 1500);
      setTimeout(refreshBalances, 4000);
      onTradeComplete?.();
      setTimeout(() => onTradeComplete?.(), 3000);
    } catch (e) {
      toast.error(`${side === 'buy' ? 'Buy' : 'Sell'} failed: ${e.message || e}`, { id: t });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-bg-card p-4 shadow-card">
      {/* Buy / Sell toggle */}
      <div className="grid grid-cols-2 gap-1 rounded-lg bg-bg-soft p-1 mb-4">
        {['buy', 'sell'].map((s) => (
          <button
            key={s}
            onClick={() => {
              setSide(s);
              setAmount('');
              setSellPct(0);
              // trailing stop protects a position — sell-only mode
              if (s === 'buy' && mode === 'trailing') setMode('market');
            }}
            className={`py-2 rounded-md text-sm font-semibold capitalize transition ${
              side === s
                ? s === 'buy'
                  ? 'bg-up/15 text-up'
                  : 'bg-down/15 text-down'
                : 'text-muted hover:text-ink'
            }`}
          >
            {s}
          </button>
        ))}
      </div>

      {/* Market / Limit / Trailing (sell-only) mode */}
      <div className="mb-4 flex gap-1 rounded-lg bg-bg-soft p-1">
        {(side === 'sell' ? ['market', 'limit', 'trailing'] : ['market', 'limit']).map((m) => (
          <button
            key={m}
            onClick={() => setMode(m)}
            className={`flex-1 rounded-md py-1.5 text-xs font-semibold capitalize transition ${
              mode === m ? 'bg-bg-card text-ink shadow' : 'text-muted hover:text-ink'
            }`}
          >
            {m === 'trailing' ? 'Trail stop' : m}
          </button>
        ))}
      </div>

      {/* Limit price */}
      {mode === 'limit' && (
        <div className="mb-3">
          <div className="mb-2 flex items-center justify-between text-xs text-muted">
            <span>Limit price (USD)</span>
            {currentPriceUsd > 0 && (
              <button
                onClick={() => setLimitPrice(currentPriceUsd.toPrecision(6))}
                className="hover:text-accent"
                title="Use current price"
              >
                now: ${currentPriceUsd < 0.001 ? currentPriceUsd.toExponential(2) : currentPriceUsd.toFixed(6)}
              </button>
            )}
          </div>
          <input
            inputMode="decimal"
            value={limitPrice}
            onChange={(e) => setLimitPrice(e.target.value.replace(/[^0-9.eE-]/g, ''))}
            placeholder="0.0"
            className="w-full rounded-lg border border-border bg-bg-soft px-3 py-2.5 text-lg font-mono text-ink outline-none focus:border-accent"
          />
          {currentPriceUsd > 0 && (
            <div className="mt-2 flex flex-wrap gap-2">
              {LIMIT_PCT_PRESETS.map((p) => (
                <button
                  key={p}
                  onClick={() => setLimitPrice((currentPriceUsd * (1 + p / 100)).toPrecision(6))}
                  className="rounded-md border border-border bg-bg-soft px-2.5 py-1 text-xs text-muted hover:border-accent hover:text-ink"
                >
                  {p > 0 ? `+${p}` : p}%
                </button>
              ))}
            </div>
          )}
          <div className="mt-2 text-[11px] leading-snug text-muted">
            {side === 'buy'
              ? 'Executes when the price drops to your limit or below.'
              : 'Executes when the price rises to your limit or above.'}
          </div>
        </div>
      )}

      {/* Trailing stop percent (移动止盈止损): sell when price falls N% from
          the highest price seen after placement. The trigger only ratchets up,
          so one line acts as both take-profit and stop-loss. */}
      {mode === 'trailing' && (
        <div className="mb-3">
          <div className="mb-2 flex items-center justify-between text-xs text-muted">
            <span>Trailing drawdown (%)</span>
            {currentPriceUsd > 0 && trailPct > 0 && trailPct < 100 && (
              <span title="Initial trigger — rises if the price makes new highs">
                triggers now at ${(currentPriceUsd * (1 - trailPct / 100)).toPrecision(4)}
              </span>
            )}
          </div>
          <input
            inputMode="numeric"
            value={trailPct}
            onChange={(e) => setTrailPct(e.target.value.replace(/[^0-9]/g, ''))}
            placeholder="20"
            className="w-full rounded-lg border border-border bg-bg-soft px-3 py-2.5 text-lg font-mono text-ink outline-none focus:border-accent"
          />
          <div className="mt-2 flex flex-wrap gap-2">
            {TRAIL_PRESETS.map((p) => (
              <button
                key={p}
                onClick={() => setTrailPct(String(p))}
                className={`rounded-md border px-2.5 py-1 text-xs ${
                  String(p) === trailPct
                    ? 'border-accent text-accent'
                    : 'border-border bg-bg-soft text-muted hover:border-accent hover:text-ink'
                }`}
              >
                -{p}%
              </button>
            ))}
          </div>
          <div className="mt-2 text-[11px] leading-snug text-muted">
            Sells at market when the price drops {trailPct || 'N'}% from its highest point after
            you place the order. New highs raise the trigger; it never moves down.
          </div>
        </div>
      )}

      {/* Amount */}
      <div className="mb-2 flex items-center justify-between text-xs text-muted">
        <span>{side === 'buy' ? 'Amount (SOL)' : `Amount (${symbol})`}</span>
        <span>
          {side === 'buy'
            ? `Balance: ${solBalance.toFixed(3)} SOL`
            : `Holdings: ${balance.toLocaleString(undefined, { maximumFractionDigits: 4 })}`}
        </span>
      </div>
      <input
        inputMode="decimal"
        value={amount}
        onChange={(e) => {
          setAmount(e.target.value.replace(/[^0-9.]/g, ''));
          setSellPct(0);
        }}
        placeholder="0.0"
        className="w-full rounded-lg border border-border bg-bg-soft px-3 py-2.5 text-lg font-mono text-ink outline-none focus:border-accent"
      />

      {/* Presets */}
      <div className="mt-2 flex flex-wrap gap-2">
        {side === 'buy'
          ? BUY_PRESETS.map((v) => (
              <button
                key={v}
                onClick={() => {
                  setAmount(String(v));
                  setSellPct(0);
                }}
                className="rounded-md border border-border bg-bg-soft px-2.5 py-1 text-xs text-muted hover:border-accent hover:text-ink"
              >
                {v} SOL
              </button>
            ))
          : SELL_PRESETS.map((p) => (
              <button
                key={p}
                onClick={() => {
                  setSellPct(p);
                  setAmount(String((balance * p) / 100));
                }}
                className={`rounded-md border px-2.5 py-1 text-xs ${
                  sellPct === p
                    ? 'border-down text-down'
                    : 'border-border bg-bg-soft text-muted hover:border-accent hover:text-ink'
                }`}
              >
                {p}%
              </button>
            ))}
      </div>

      {/* Double out (翻倍出本): when the buy fills, the backend auto-places a
          limit sell at 2x the fill price for half the fill amount — the sell
          returns the principal and the rest rides for free. */}
      {side === 'buy' && (
        <label className="mt-4 flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-bg-soft px-3 py-2.5">
          <input
            type="checkbox"
            checked={doubleOut}
            onChange={(e) => {
              setDoubleOut(e.target.checked);
              if (e.target.checked) setAttachTrail(false); // one auto sell leg per fill
            }}
            className="mt-0.5 accent-accent"
          />
          <span className="text-xs leading-snug">
            <span className="font-semibold text-ink">⚡ Double out</span>{' '}
            <span className="text-muted">
              — when this buy fills, auto-place a sell at 2× price for half the tokens,
              recovering your cost.
              {mode === 'market' && ' Executed by the platform devnet wallet (no signing).'}
            </span>
          </span>
        </label>
      )}

      {/* Attach a trailing stop to the buy: when the fill confirms, the backend
          auto-places a trailing stop for the whole fill, anchored at the actual
          fill price — protection starts the moment the position exists. */}
      {side === 'buy' && mode === 'market' && (
        <label className="mt-2 flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-bg-soft px-3 py-2.5">
          <input
            type="checkbox"
            checked={attachTrail}
            onChange={(e) => {
              setAttachTrail(e.target.checked);
              if (e.target.checked) setDoubleOut(false); // one auto sell leg per fill
            }}
            className="mt-0.5 accent-accent"
          />
          <span className="min-w-0 flex-1 text-xs leading-snug">
            <span className="font-semibold text-ink">🛡️ Trailing stop</span>{' '}
            <span className="text-muted">
              — when this buy fills, auto-place a trailing stop for the tokens: sells if the
              price drops the chosen % from its post-fill high. Executed by the platform
              devnet wallet (no signing).
            </span>
            {attachTrail && (
              <span className="mt-2 flex flex-wrap items-center gap-2">
                {TRAIL_PRESETS.map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={(e) => {
                      e.preventDefault();
                      setTrailPct(String(p));
                    }}
                    className={`rounded-md border px-2.5 py-1 text-xs ${
                      String(p) === trailPct
                        ? 'border-accent text-accent'
                        : 'border-border bg-bg-card text-muted hover:border-accent hover:text-ink'
                    }`}
                  >
                    -{p}%
                  </button>
                ))}
                <input
                  inputMode="numeric"
                  value={trailPct}
                  onChange={(e) => setTrailPct(e.target.value.replace(/[^0-9]/g, ''))}
                  onClick={(e) => e.preventDefault()}
                  className="w-14 rounded-md border border-border bg-bg-card px-2 py-1 text-xs font-mono text-ink outline-none focus:border-accent"
                />
                <span className="text-muted">%</span>
              </span>
            )}
          </span>
        </label>
      )}

      {/* Base slippage is fixed server-side (50% market / 10% limit) on this
          devnet build. Auto slippage (自动滑点): if a server-signed send fails
          on slippage, the backend escalates the tier (base → 45% → 70%) and
          rebuilds the swap with a fresh quote instead of failing the order. */}
      <div className="mt-4 flex items-center justify-between text-xs">
        <span className="text-muted">Max slippage</span>
        <span className="flex items-center gap-2">
          <span className="text-ink">
            {mode === 'market' ? '50%' : '10%'}
            {autoSlippage && <span className="text-muted"> → up to 70%</span>}
          </span>
          <button
            onClick={() => setAutoSlippage((v) => !v)}
            title="Auto slippage: on a slippage failure, retry with a bigger slippage and a fresh quote (up to 70%) instead of failing the order"
            className={`rounded-md border px-2 py-0.5 text-[11px] font-semibold transition ${
              autoSlippage
                ? 'border-accent text-accent'
                : 'border-border bg-bg-soft text-muted hover:border-accent hover:text-ink'
            }`}
          >
            Auto {autoSlippage ? 'on' : 'off'}
          </button>
        </span>
      </div>

      <button
        disabled={busy || !connected}
        onClick={submit}
        className={`mt-4 w-full rounded-lg py-3 text-sm font-bold transition disabled:opacity-50 ${
          side === 'buy' ? 'bg-up text-bg hover:brightness-110' : 'bg-down text-white hover:brightness-110'
        }`}
      >
        {busy
          ? 'Submitting…'
          : !connected
          ? 'Connect Wallet'
          : mode === 'trailing'
          ? 'Place trailing stop'
          : mode === 'limit'
          ? side === 'buy'
            ? `Place limit buy`
            : `Place limit sell`
          : side === 'buy'
          ? `Buy ${symbol}`
          : `Sell ${symbol}`}
      </button>
    </div>
  );
}
