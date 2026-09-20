import React from 'react';
import { shortAddr } from '../lib/trade';

function ago(ts) {
  if (!ts) return '';
  const s = Math.max(0, Math.floor(Date.now() / 1000) - Number(ts));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

const fmt = (v, d = 2) =>
  v == null ? '—' : Number(v).toLocaleString(undefined, { maximumFractionDigits: d });

export default function RecentTrades({ trades = [], status = 'loading' }) {
  return (
    <div className="rounded-xl border border-border bg-bg-card p-3">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-ink">Recent trades</h2>
        <span className="text-[11px] text-muted">on-chain · live</span>
      </div>

      {status !== 'ok' ? (
        <div className="py-8 text-center text-xs text-muted">
          {status === 'loading' ? 'Loading trades…' : status === 'empty' ? 'No trades yet' : 'Unavailable'}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-muted">
                <th className="pb-1.5 font-medium">Type</th>
                <th className="pb-1.5 text-right font-medium">SOL</th>
                <th className="pb-1.5 text-right font-medium">Tokens</th>
                <th className="pb-1.5 text-right font-medium">Price</th>
                <th className="pb-1.5 text-right font-medium">Maker</th>
                <th className="pb-1.5 text-right font-medium">Age</th>
              </tr>
            </thead>
            <tbody>
              {trades.slice(0, 30).map((t, i) => (
                <tr key={`${t.sig}-${i}`} className="border-t border-border/60">
                  <td className={`py-1.5 font-semibold ${t.isBuy ? 'text-up' : 'text-down'}`}>
                    {t.isBuy ? 'Buy' : 'Sell'}
                  </td>
                  <td className="py-1.5 text-right font-mono text-ink">{fmt(t.solAmount, 4)}</td>
                  <td className="py-1.5 text-right font-mono text-muted">{fmt(t.tokenAmount, 0)}</td>
                  <td className="py-1.5 text-right font-mono text-muted">
                    {t.priceUsd < 0.001 ? `$${t.priceUsd.toExponential(1)}` : `$${t.priceUsd.toFixed(6)}`}
                  </td>
                  <td className="py-1.5 text-right">
                    <a
                      className="text-muted hover:text-accent"
                      href={`https://solscan.io/tx/${t.sig}?cluster=devnet`}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {shortAddr(t.maker, 3)}
                    </a>
                  </td>
                  <td className="py-1.5 text-right text-muted">{ago(t.time)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
