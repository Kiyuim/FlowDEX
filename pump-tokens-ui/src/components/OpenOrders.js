import React, { useCallback, useEffect, useState } from 'react';
import toast from 'react-hot-toast';
import { fetchCurrentOrders, cancelOrder, ORDER_STATUS } from '../lib/trade';

const TONE_CLASSES = {
  warn: 'bg-warn/10 text-warn',
  accent: 'bg-accent/10 text-accent',
  up: 'bg-up/10 text-up',
  down: 'bg-down/10 text-down',
  muted: 'bg-bg-elev text-muted',
};

const priceStr = (p) =>
  p == null || Number.isNaN(p) ? '—' : p < 0.001 ? `$${p.toExponential(2)}` : `$${p.toFixed(6)}`;

/**
 * Open limit orders for one token. Polls the backend so a Waiting order that
 * gets triggered by an on-chain trade flips to "Triggered" without a reload.
 */
export default function OpenOrders({ mint, symbol, refreshTick = 0 }) {
  const [orders, setOrders] = useState([]);
  const [status, setStatus] = useState('loading');
  const [cancelling, setCancelling] = useState(null);

  const load = useCallback(async () => {
    if (!mint) return;
    try {
      setOrders(await fetchCurrentOrders(mint));
      setStatus('ok');
    } catch {
      setStatus('error');
    }
  }, [mint]);

  useEffect(() => {
    load();
    const t = setInterval(load, 6000);
    return () => clearInterval(t);
  }, [load, refreshTick]);

  const onCancel = async (id) => {
    setCancelling(id);
    try {
      await cancelOrder(id);
      toast.success(`Order #${id} cancelled`);
      await load();
    } catch (e) {
      toast.error(`Cancel failed: ${e.message || e}`);
    } finally {
      setCancelling(null);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-bg-card p-4 shadow-card">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-xs uppercase tracking-wide text-muted">Open orders</span>
        {status === 'loading' && <span className="text-[11px] text-muted">loading…</span>}
        {status === 'error' && <span className="text-[11px] text-down">unavailable</span>}
      </div>

      {status === 'ok' && orders.length === 0 && (
        <div className="py-3 text-center text-xs text-muted">No open orders for this token</div>
      )}

      <div className="space-y-2">
        {orders.map((o) => {
          const st = ORDER_STATUS[o.status] || { label: `#${o.status}`, tone: 'muted' };
          const isBuy = o.swapType === 1;
          const isTrailing = o.tradeType === 5;
          // Waiting orders are always cancellable. A market order still at
          // "Triggered" after 2 minutes never got its on-chain result reported
          // back (or predates that mechanism) — nothing will move it on its
          // own, so let the user clear it. Mirrors the backend's stuckOrderAge.
          const stuck = o.status === 2 && Date.now() / 1000 - o.createTime > 120;
          const cancellable = o.status === 1 || stuck;
          return (
            <div
              key={o.id}
              className="flex items-center gap-2 rounded-lg border border-border bg-bg-soft px-3 py-2"
            >
              <span className={`text-xs font-bold ${isBuy ? 'text-up' : 'text-down'}`}>
                {isBuy ? 'BUY' : 'SELL'}
              </span>
              {isTrailing && (
                <span
                  className="rounded bg-warn/10 px-1 py-0.5 text-[10px] font-bold text-warn"
                  title={`Trailing stop: sells at market when the price drops ${o.trailingPercent}% from its highest point since placement (trigger shown is the placement anchor; it ratchets up with new highs)`}
                >
                  TRAIL −{o.trailingPercent}%
                </span>
              )}
              {o.doubleOut === 1 && (
                <span
                  className="rounded bg-accent/10 px-1 py-0.5 text-[10px] font-bold text-accent"
                  title={
                    isBuy
                      ? 'Double out: a 2× sell will be auto-placed when this buy fills'
                      : 'Auto-created by double out (2× price, half the fill)'
                  }
                >
                  2×
                </span>
              )}
              <div className="min-w-0 flex-1">
                <div className="truncate text-xs text-ink">
                  {Number(o.amount).toLocaleString(undefined, { maximumFractionDigits: 4 })}{' '}
                  {isBuy ? 'SOL' : symbol}{' '}
                  {isTrailing
                    ? `· anchor ${priceStr(o.priceUsd)}, trigger ≤ ${priceStr(
                        o.priceUsd * (1 - o.trailingPercent / 100)
                      )}`
                    : `@ ${priceStr(o.priceUsd)}`}
                </div>
                <div className="text-[11px] text-muted">
                  #{o.id} · {new Date(o.createTime * 1000).toLocaleString()}
                </div>
              </div>
              <span className={`rounded px-1.5 py-0.5 text-[11px] ${TONE_CLASSES[st.tone]}`}>
                {st.label}
              </span>
              {cancellable && (
                <button
                  onClick={() => onCancel(o.id)}
                  disabled={cancelling === o.id}
                  className="rounded-md border border-border px-2 py-1 text-[11px] text-muted hover:border-down hover:text-down disabled:opacity-50"
                >
                  {cancelling === o.id ? '…' : 'Cancel'}
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
