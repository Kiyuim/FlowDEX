import React from 'react';
import { useNavigate } from 'react-router-dom';
import { shortAddr } from '../lib/trade';
import useWatchlist from '../hooks/useWatchlist';
import { normalizeToken, tokenDisplayName, finiteNumber } from '../lib/tokenData';

function compact(n, prefix = '') {
  const v = finiteNumber(n);
  if (v == null) return '—';
  const abs = Math.abs(v);
  if (abs >= 1e9) return `${prefix}${(v / 1e9).toFixed(2)}B`;
  if (abs >= 1e6) return `${prefix}${(v / 1e6).toFixed(2)}M`;
  if (abs >= 1e3) return `${prefix}${(v / 1e3).toFixed(1)}K`;
  return `${prefix}${v.toFixed(2)}`;
}

function age(ts) {
  if (!ts) return '';
  const s = Math.max(0, Math.floor(Date.now() / 1000) - Number(ts));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export default function DiscoveryCard({ token: rawToken, fresh }) {
  const token = normalizeToken(rawToken);
  const navigate = useNavigate();
  const { isFav, toggle } = useWatchlist();
  const mint = token.tokenAddress;
  const change = finiteNumber(token.change24);
  const progress = token.domesticProgress != null ? Math.min(100, Number(token.domesticProgress) * 100) : null;
  const fav = isFav(mint);

  return (
    <button
      onClick={() => navigate(`/token/${mint}`, { state: { token } })}
      className={`group w-full rounded-xl border bg-bg-card p-3 text-left transition hover:border-accent/50 hover:bg-bg-hover ${
        fresh ? 'border-accent/50 animate-fade-in' : 'border-border'
      }`}
    >
      <div className="flex items-center gap-3">
        {token.tokenIcon ? (
          <img src={token.tokenIcon} alt="" className="h-10 w-10 shrink-0 rounded-full object-cover" />
        ) : (
          <div className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-bg-elev">🪙</div>
        )}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate font-semibold text-ink">{tokenDisplayName(token)}</span>
            {token.tokenSymbol && <span className="shrink-0 text-xs text-muted">{token.tokenSymbol}</span>}
            {fresh && <span className="shrink-0 rounded bg-accent/20 px-1 text-[10px] font-bold text-accent">NEW</span>}
          </div>
          <div className="mt-0.5 flex items-center gap-2 text-xs text-muted">
            <span>{shortAddr(mint, 4)}</span>
            {token.launchTime ? <span>· {age(token.launchTime)}</span> : null}
          </div>
        </div>
        {change != null && (
          <span className={`shrink-0 text-sm font-semibold ${change >= 0 ? 'text-up' : 'text-down'}`}>
            {change >= 0 ? '+' : ''}
            {change.toFixed(1)}%
          </span>
        )}
        <span
          role="button"
          tabIndex={0}
          aria-label={fav ? 'Remove from watchlist' : 'Add to watchlist'}
          onClick={(e) => {
            e.stopPropagation();
            toggle(mint);
          }}
          className={`shrink-0 cursor-pointer text-base leading-none ${fav ? 'text-warn' : 'text-muted hover:text-ink'}`}
        >
          {fav ? '★' : '☆'}
        </span>
      </div>

      <div className="mt-2.5 grid grid-cols-3 gap-2 text-xs">
        <div>
          <div className="text-muted">MC</div>
          <div className="font-medium text-ink">{compact(token.mktCap, '$')}</div>
        </div>
        <div>
          <div className="text-muted">Vol</div>
          <div className="font-medium text-ink">{compact(token.vol24h, '$')}</div>
        </div>
        <div>
          <div className="text-muted">Holders</div>
          <div className="font-medium text-ink">{compact(token.holdCount)}</div>
        </div>
      </div>

      {progress != null && (
        <div className="mt-2.5">
          <div className="h-1.5 overflow-hidden rounded-full bg-bg-soft">
            <div className="h-full rounded-full bg-accent" style={{ width: `${progress}%` }} />
          </div>
        </div>
      )}
    </button>
  );
}
