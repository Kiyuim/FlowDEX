import React, { useCallback, useEffect, useRef, useState } from 'react';
import DiscoveryCard from '../components/DiscoveryCard';
import useTokenListWebSocket from '../hooks/useTokenListWebSocket';
import useWatchlist from '../hooks/useWatchlist';

const COLUMNS = [
  { key: 'new', status: 1, title: 'New', hint: 'Just launched' },
  { key: 'completing', status: 2, title: 'Completing', hint: 'Filling the curve' },
  { key: 'completed', status: 4, title: 'Graduated', hint: 'Moved to AMM' },
];

const mintOf = (t) => t?.tokenAddress || t?.token_ca || t?.address;

export default function Discovery() {
  const [lists, setLists] = useState({ new: [], completing: [], completed: [] });
  const [loading, setLoading] = useState(true);
  const [fresh, setFresh] = useState({}); // mint -> true (recently added)
  const [mobileTab, setMobileTab] = useState('new');
  const [query, setQuery] = useState('');
  const [onlyFavs, setOnlyFavs] = useState(false);
  const { favs } = useWatchlist();
  const freshTimers = useRef({});

  const fetchAll = useCallback(async () => {
    try {
      const results = await Promise.all(
        COLUMNS.map((c) =>
          fetch(`/v1/market/index_pump?chain_id=100000&pump_status=${c.status}&page_no=1&page_size=50`)
            .then((r) => r.json())
            .then((d) => d?.data?.list || [])
            .catch(() => [])
        )
      );
      // Dedup across columns so a token shows in exactly one. The backend's
      // completing (status 2) and graduated (status 4) queries overlap — a token
      // the backend marks pump_status=2 is *completing*, so let Completing claim
      // it first (otherwise the Completing column renders empty).
      const seen = new Set();
      const dedup = (arr) => {
        const out = [];
        for (const t of arr) {
          const m = mintOf(t);
          if (!m || seen.has(m)) continue;
          seen.add(m);
          out.push(t);
        }
        return out;
      };
      const completing = dedup(results[1]); // status 2 — claim first
      const completed = dedup(results[2]); // status 4 — minus the completing ones
      const nw = dedup(results[0]); // status 1
      setLists({ new: nw, completing, completed });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
    const id = setInterval(fetchAll, 15000);
    return () => clearInterval(id);
  }, [fetchAll]);

  const markFresh = useCallback((mint) => {
    setFresh((f) => ({ ...f, [mint]: true }));
    clearTimeout(freshTimers.current[mint]);
    freshTimers.current[mint] = setTimeout(
      () => setFresh((f) => { const n = { ...f }; delete n[mint]; return n; }),
      12000
    );
  }, []);

  const onNewToken = useCallback(
    (t) => {
      const m = mintOf(t);
      if (!m) return;
      setLists((ls) => {
        if ([...ls.new, ...ls.completing, ...ls.completed].some((x) => mintOf(x) === m)) return ls;
        return { ...ls, new: [t, ...ls.new].slice(0, 60) };
      });
      markFresh(m);
    },
    [markFresh]
  );

  const onTokenUpdate = useCallback(
    (t) => {
      const m = mintOf(t);
      if (!m) return;
      const bucket = t.pumpStatus === 2 ? 'completing' : t.pumpStatus === 4 ? 'completed' : 'new';
      setLists((ls) => {
        const strip = (arr) => arr.filter((x) => mintOf(x) !== m);
        const next = { new: strip(ls.new), completing: strip(ls.completing), completed: strip(ls.completed) };
        next[bucket] = [t, ...next[bucket]].slice(0, 60);
        return next;
      });
      markFresh(m);
    },
    [markFresh]
  );

  const { connectionStatus } = useTokenListWebSocket(onNewToken, onTokenUpdate);
  const live = connectionStatus === 'connected';

  const match = (t) => {
    if (onlyFavs && !favs.has(mintOf(t))) return false;
    const q = query.trim().toLowerCase();
    if (!q) return true;
    return (
      (t.tokenName || '').toLowerCase().includes(q) ||
      (t.tokenSymbol || '').toLowerCase().includes(q) ||
      (mintOf(t) || '').toLowerCase().includes(q)
    );
  };

  return (
    <div className="mx-auto max-w-7xl px-3 py-4 md:px-6">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-bold text-ink">Discover</h1>
          <p className="text-xs text-muted">Live pump.fun tokens on devnet</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setOnlyFavs((v) => !v)}
            className={`flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs ${
              onlyFavs ? 'border-warn/40 bg-warn/10 text-warn' : 'border-border bg-bg-card text-muted hover:text-ink'
            }`}
          >
            {onlyFavs ? '★' : '☆'} Watchlist
            {favs.size > 0 && <span className="opacity-70">({favs.size})</span>}
          </button>
          <span
            className={`flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs ${
              live ? 'border-up/40 bg-up/10 text-up' : 'border-border bg-bg-card text-muted'
            }`}
          >
            <span className={`h-1.5 w-1.5 rounded-full ${live ? 'bg-up animate-pulse-soft' : 'bg-muted'}`} />
            {live ? 'Live' : 'Polling'}
          </span>
        </div>
      </div>

      {/* Search */}
      <div className="mb-3">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search name, symbol, or address…"
          className="w-full rounded-lg border border-border bg-bg-soft px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
      </div>

      {/* Mobile tabs */}
      <div className="mb-3 flex gap-1 rounded-lg bg-bg-soft p-1 md:hidden">
        {COLUMNS.map((c) => (
          <button
            key={c.key}
            onClick={() => setMobileTab(c.key)}
            className={`flex-1 rounded-md py-1.5 text-sm font-medium ${
              mobileTab === c.key ? 'bg-bg-elev text-ink' : 'text-muted'
            }`}
          >
            {c.title}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {COLUMNS.map((c) => {
          const shown = lists[c.key].filter(match);
          return (
            <div key={c.key} className={`${mobileTab === c.key ? 'block' : 'hidden'} md:block`}>
              <div className="mb-2 flex items-baseline justify-between">
                <h2 className="text-sm font-semibold text-ink">{c.title}</h2>
                <span className="text-xs text-muted">{shown.length}</span>
              </div>
              <div className="space-y-2">
                {loading && shown.length === 0
                  ? Array.from({ length: 4 }).map((_, i) => (
                      <div key={i} className="h-24 animate-pulse rounded-xl border border-border bg-bg-card" />
                    ))
                  : shown.length === 0
                  ? <div className="rounded-xl border border-dashed border-border p-6 text-center text-xs text-muted">{query ? 'No matches' : 'No tokens'}</div>
                  : shown.map((t) => (
                      <DiscoveryCard key={mintOf(t)} token={t} fresh={!!fresh[mintOf(t)]} />
                    ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
