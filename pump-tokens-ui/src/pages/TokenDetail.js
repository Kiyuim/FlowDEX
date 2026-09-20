import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, useLocation, Link } from 'react-router-dom';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import TradingViewChart from '../components/TradingViewChart';
import RecentTrades from '../components/RecentTrades';
import TradePanel from '../components/TradePanel';
import OpenOrders from '../components/OpenOrders';
import useBondingCurveTrades from '../hooks/useBondingCurveTrades';
import useBondingCurveReserves from '../hooks/useBondingCurveReserves';
import useWatchlist from '../hooks/useWatchlist';
import { shortAddr } from '../lib/trade';
import { readCurveState } from '../lib/curve';

import { mergeToken, normalizeToken, tokenDisplayName, finiteNumber } from '../lib/tokenData';

function Stat({ label, value }) {
  return (
    <div className="rounded-lg border border-border bg-bg-card px-3 py-2">
      <div className="text-[11px] uppercase tracking-wide text-muted">{label}</div>
      <div className="mt-0.5 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

async function fetchTokenByMint(mint, walletAddress) {
  const indexed = fetch(`/v1/market/index_pump?chain_id=100000&token_address=${encodeURIComponent(mint)}&pump_status=1&page_size=100`)
    .then((r) => r.json()).then((d) => (d?.data?.list || []).map(normalizeToken).find((t) => t.tokenAddress === mint)).catch(() => null);
  const recorded = walletAddress ? fetch(`/v1/market/user_tokens?chain_id=100000&wallet_address=${walletAddress}`)
    .then((r) => r.json()).then((d) => (d?.data?.list || []).map(normalizeToken).find((t) => t.tokenAddress === mint)).catch(() => null) : Promise.resolve(null);
  const [live, own] = await Promise.all([indexed, recorded]);
  return live || own ? mergeToken(live, own) : null;
}

export default function TokenDetail() {
  const { mint } = useParams();
  const location = useLocation();
  const { publicKey } = useWallet();
  const [token, setToken] = useState(location.state?.token || null);
  const [loading, setLoading] = useState(!location.state?.token);
  // The candle chart (TradingViewChart) is the only price chart. `trades` (raw
  // on-chain trades) still feeds the stats and the RecentTrades list below.
  const { trades, status: tradesStatus, reload: reloadTrades } = useBondingCurveTrades(mint);
  const { reserves, status: reservesStatus, reload: reloadReserves } = useBondingCurveReserves(mint);
  const { isFav, toggle } = useWatchlist();
  const [ordersTick, setOrdersTick] = useState(0);
  const [chartRefresh, setChartRefresh] = useState(0);

  // Live bonding-curve state read straight from chain (source-aware: pump.fun /
  // PumpMeteora / V2). Gives a brand-new token a real STARTING price + reserves
  // before it has any trades to compute stats from.
  const { connection } = useConnection();
  const [curveState, setCurveState] = useState(null);
  const reloadCurveState = useCallback(() => {
    if (!mint) return;
    readCurveState(connection, mint).then((state) => setCurveState(state ? { ...state, mint } : null)).catch(() => {});
  }, [mint, connection]);
  useEffect(() => {
    if (!mint) return undefined;
    reloadCurveState();
    const id = setInterval(reloadCurveState, 15000);
    return () => clearInterval(id);
  }, [mint, reloadCurveState]);

  // Pool reserves/recent-trades/price all poll on their own interval —
  // without an explicit kick, the page just looks unchanged right after a
  // trade until the next scheduled poll happens to land.
  const handleTradeComplete = useCallback(() => {
    setChartRefresh((n) => n + 1);
    reloadCurveState();
    reloadReserves();
    reloadTrades();
  }, [reloadCurveState, reloadReserves, reloadTrades]);

  // Real stats computed from on-chain trades (backend metadata is often stale/0).
  const stats = useMemo(() => {
    if (!trades.length) return null;
    const now = Date.now() / 1000;
    const cutoff = now - 86400;
    const recent = trades.filter((t) => t.time >= cutoff);
    const price = trades[0].priceUsd;
    const vol24h = recent.reduce((s, t) => s + t.solAmount * 150, 0); // nominal SOL≈$150
    const buys24h = recent.filter((t) => t.isBuy).length;
    const traders = new Set(trades.map((t) => t.maker)).size;
    const older = trades.filter((t) => t.time < cutoff);
    const ref = older.length ? older[0].priceUsd : recent[recent.length - 1]?.priceUsd;
    const change = ref ? ((price - ref) / ref) * 100 : null;
    return { price, vol24h, txns24h: recent.length, buys24h, sells24h: recent.length - buys24h, traders, change };
  }, [trades]);

  const money = (v) => {
    if (v == null) return '—';
    if (v >= 1e6) return `$${(v / 1e6).toFixed(2)}M`;
    if (v >= 1e3) return `$${(v / 1e3).toFixed(1)}K`;
    return `$${v.toFixed(2)}`;
  };
  const priceStr = (p) => (p == null ? '—' : p < 0.001 ? `$${p.toExponential(2)}` : `$${p.toFixed(6)}`);
  // Prefer the same indexed market data used by Discovery; direct RPC trades
  // are only a recent sample, not the full 24-hour volume.
  const displayPrice = finiteNumber(token?.price) || stats?.price || (curveState?.mint === mint ? curveState.priceUsd : null) || null;
  const displayVolume = finiteNumber(token?.vol24h);
  const displayChange = finiteNumber(token?.change24);
  const currentCurve = curveState?.mint === mint ? curveState : null;
  const poolRes = reserves || currentCurve;

  useEffect(() => {
    let alive = true;
    // Always attempt the indexed lookup — it's the only source of a real
    // pairAddress (a token passed via router state, e.g. from Portfolio's
    // "Created by you" list, only carries name/symbol/icon, not a pair).
    // Don't block the UI with a spinner if we already have something to
    // show from nav state though.
    if (!token || token.tokenAddress !== mint) setLoading(true);
    const refresh = async () => {
      const t = await fetchTokenByMint(mint, publicKey?.toString());
      if (!alive) return;
      setToken((prev) => {
        const base = prev?.tokenAddress === mint ? prev : { tokenAddress: mint };
        return mergeToken(base, t);
      });
      setLoading(false);
    };
    // Router state belongs to this navigation; never show a previous mint's name.
    setToken((prev) => mergeToken(prev?.tokenAddress === mint ? prev : { tokenAddress: mint }, location.state?.token?.tokenAddress === mint ? location.state.token : null));
    refresh();
    const timer = window.setInterval(refresh, 15000);
    return () => { alive = false; window.clearInterval(timer); };
  }, [mint, publicKey, location.state, chartRefresh]); // eslint-disable-line

  const chartToken = useMemo(() => ({ ...token, pairAddress: (curveState?.mint === mint ? curveState.pairAddress : null) || token?.pairAddress }), [token, curveState, mint]);

  const fmt = (v, d = 2) =>
    v == null || v === '' ? '—' : Number(v).toLocaleString(undefined, { maximumFractionDigits: d });

  return (
    <div className="mx-auto max-w-7xl px-3 py-4 md:px-6">
      <Link to="/" className="mb-3 inline-flex items-center gap-1 text-sm text-muted hover:text-ink">
        ← Back
      </Link>

      {/* Header */}
      <div className="mb-4 flex items-center gap-3">
        {token?.tokenIcon ? (
          <img src={token.tokenIcon} alt="" className="h-11 w-11 rounded-full object-cover" />
        ) : (
          <div className="grid h-11 w-11 place-items-center rounded-full bg-bg-elev text-lg">🪙</div>
        )}
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h1 className="truncate text-xl font-bold text-ink">
              {tokenDisplayName(token)}
            </h1>
            {token?.tokenSymbol && (
              <span className="rounded bg-bg-elev px-1.5 py-0.5 text-xs text-muted">
                {token.tokenSymbol}
              </span>
            )}
          </div>
          <a
            className="text-xs text-muted hover:text-accent"
            href={`https://solscan.io/token/${mint}?cluster=devnet`}
            target="_blank"
            rel="noreferrer"
          >
            {shortAddr(mint, 6)} ↗
          </a>
        </div>
        <button
          onClick={() => toggle(mint)}
          aria-label={isFav(mint) ? 'Remove from watchlist' : 'Add to watchlist'}
          className={`ml-auto shrink-0 rounded-lg border px-2.5 py-1.5 text-lg leading-none ${
            isFav(mint) ? 'border-warn/40 bg-warn/10 text-warn' : 'border-border text-muted hover:text-ink'
          }`}
        >
          {isFav(mint) ? '★' : '☆'}
        </button>
      </div>

      {/* Main grid: chart + info (left), trade panel (right) */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_360px]">
        <div className="min-w-0 space-y-4">
          <div className="rounded-xl border border-border bg-bg-card p-2 shadow-card">
            <div className={token ? '' : 'h-[380px] md:h-[460px]'}>
              {token ? (
                // TradingViewChart fetches candles from our own backend
                // (/v1/market/get_candlestick), independent of on-chain
                // trades — an old seeded token can have real backend kline
                // history with zero on-chain trades (or vice versa for a
                // brand-new one). It shows its own empty state either way.
                <TradingViewChart token={chartToken} refreshKey={chartRefresh} visible />
              ) : (
                <div className="flex h-full flex-col items-center justify-center gap-2 px-4 text-center">
                  <div className="text-4xl">🌱</div>
                  <div className="text-sm font-semibold text-ink">No trades yet</div>
                  <div className="text-xs text-muted">
                    {curveState ? (
                      <>
                        Starting price <span className="text-ink">{priceStr(curveState.priceUsd)}</span> ·{' '}
                        {curveState.source} bonding curve · be the first to buy →
                      </>
                    ) : (
                      'Be the first to buy this token — the candle chart starts after the first trade.'
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <Stat
              label="Price"
              value={
                <span className="flex items-baseline gap-1.5">
                  {priceStr(displayPrice)}
                  {displayChange != null && (
                    <span className={`text-xs ${displayChange >= 0 ? 'text-up' : 'text-down'}`}>
                      {displayChange >= 0 ? '+' : ''}
                      {displayChange.toFixed(1)}%
                    </span>
                  )}
                </span>
              }
            />
            <Stat label="24h Volume" value={displayVolume != null ? money(displayVolume) : '—'} />
            <Stat label="Market Cap" value={money(finiteNumber(token?.mktCap))} />
            <Stat label="24h Trades" value={fmt(finiteNumber(token?.txs24h), 0)} />
            <Stat label="Traders" value={fmt(stats?.traders, 0)} />
          </div>

          {/* Pool reserves — the two sides of the bonding-curve pool, read on-chain */}
          <div className="rounded-xl border border-border bg-bg-card p-4">
            <div className="mb-2 flex items-center justify-between">
              <span className="text-xs uppercase tracking-wide text-muted">Pool reserves</span>
              {reservesStatus === 'loading' && <span className="text-[11px] text-muted">loading…</span>}
              {reservesStatus === 'error' && <span className="text-[11px] text-down">unavailable</span>}
              {poolRes?.complete && (
                <span className="rounded bg-up/10 px-1.5 py-0.5 text-[11px] text-up">migrated</span>
              )}
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="rounded-lg border border-border bg-bg-elev px-3 py-2">
                <div className="text-[11px] text-muted">{token?.tokenSymbol || 'TOKEN'} reserve</div>
                <div className="mt-0.5 text-sm font-semibold text-ink">
                  {poolRes ? fmt(poolRes.realToken, 0) : '—'}
                </div>
              </div>
              <div className="rounded-lg border border-border bg-bg-elev px-3 py-2">
                <div className="text-[11px] text-muted">SOL reserve</div>
                <div className="mt-0.5 text-sm font-semibold text-ink">
                  {poolRes ? `${fmt(poolRes.realSol, 4)} SOL` : '—'}
                </div>
              </div>
            </div>
            {poolRes && (
              <div className="mt-1.5 text-[11px] text-muted">
                virtual: {fmt(poolRes.virtualToken, 0)} {token?.tokenSymbol || 'TOKEN'} ·{' '}
                {fmt(poolRes.virtualSol, 4)} SOL
              </div>
            )}
          </div>

          {token?.domesticProgress != null && (
            <div className="rounded-xl border border-border bg-bg-card p-4">
              <div className="mb-1.5 flex justify-between text-xs text-muted">
                <span>Bonding curve progress</span>
                <span>{fmt(Math.min(100, Number(token.domesticProgress) * 100), 1)}%</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-bg-soft">
                <div
                  className="h-full rounded-full bg-accent"
                  style={{ width: `${Math.min(100, Number(token.domesticProgress) * 100)}%` }}
                />
              </div>
            </div>
          )}

          {token && <RecentTrades trades={trades} status={tradesStatus} />}
        </div>

        <div className="space-y-4 lg:sticky lg:top-4 lg:self-start">
          {loading ? (
            <div className="h-64 animate-pulse rounded-xl border border-border bg-bg-card" />
          ) : (
            <>
              <TradePanel
                token={token}
                currentPriceUsd={stats?.price ?? reserves?.priceUsd ?? 0}
                onLimitOrderPlaced={() => setOrdersTick((t) => t + 1)}
                onTradeComplete={handleTradeComplete}
              />
              <OpenOrders
                mint={mint}
                symbol={token?.tokenSymbol || 'TOKEN'}
                refreshTick={ordersTick}
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
