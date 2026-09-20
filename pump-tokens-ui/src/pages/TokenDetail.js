import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useParams, useLocation, Link } from 'react-router-dom';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import TradingViewChart from '../components/TradingViewChart';
import RecentTrades from '../components/RecentTrades';
import TradePanel from '../components/TradePanel';
import OpenOrders from '../components/OpenOrders';
import useBondingCurveTrades from '../hooks/useBondingCurveTrades';
import useIndexedTrades from '../hooks/useIndexedTrades';
import useBondingCurveReserves from '../hooks/useBondingCurveReserves';
import useWatchlist from '../hooks/useWatchlist';
import { shortAddr } from '../lib/trade';
import { readCurveState } from '../lib/curve';
import { deriveBondingCurve } from '../lib/pump';
import { getSolUsd } from '../lib/solPrice';

import { mergeToken, normalizeToken, tokenDisplayName, finiteNumber } from '../lib/tokenData';

function Stat({ label, value }) {
  return (
    <div className="rounded-lg border border-border bg-bg-card px-3 py-2">
      <div className="text-[11px] uppercase tracking-wide text-muted">{label}</div>
      <div className="mt-0.5 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function getFallbackPair(mint) {
  if (!mint) return '';
  try {
    return deriveBondingCurve(mint).toBase58();
  } catch {
    return mint;
  }
}

async function fetchTokenByMint(mint, walletAddress) {
  try {
    const r = await fetch(
      `/v1/market/index_pump?chain_id=100000&token_address=${encodeURIComponent(mint)}&page_size=10`
    );
    const d = await r.json();
    const list = d?.data?.list || [];
    const found = list.map(normalizeToken).find((t) => t.tokenAddress === mint);
    if (found) return found;
  } catch {}

  if (walletAddress) {
    try {
      const r = await fetch(
        `/v1/market/user_tokens?chain_id=100000&wallet_address=${encodeURIComponent(walletAddress)}`
      );
      const d = await r.json();
      const list = d?.data?.list || [];
      const found = list.map(normalizeToken).find((t) => t.tokenAddress === mint);
      if (found) return found;
    } catch {}
  }
  return null;
}

export default function TokenDetail() {
  const { mint } = useParams();
  const location = useLocation();
  const { publicKey } = useWallet();
  const defaultPair = useMemo(() => getFallbackPair(mint), [mint]);

  const [token, setToken] = useState(() => {
    const navToken = location.state?.token;
    if (navToken && navToken.tokenAddress === mint) {
      return {
        pairAddress: navToken.pairAddress || defaultPair,
        ...navToken,
        tokenAddress: mint,
      };
    }
    return {
      tokenAddress: mint,
      pairAddress: defaultPair,
      tokenSymbol: 'TOKEN',
    };
  });
  // The candle chart (TradingViewChart) is the only price chart. `trades` (raw
  // on-chain trades) still feeds the stats and the RecentTrades list below.
  // On-chain trades are the fallback for a pair the indexer hasn't recorded
  // yet; once the backend has it, the index is the single source for the
  // list, the stats and the chart (same numbers as the token list card).
  const { trades: chainTrades, status: chainTradesStatus, reload: reloadTrades } = useBondingCurveTrades(mint);
  const { reserves, status: reservesStatus, reload: reloadReserves } = useBondingCurveReserves(mint);
  const { isFav, toggle } = useWatchlist();
  const [ordersTick, setOrdersTick] = useState(0);
  const [chartRefresh, setChartRefresh] = useState(0);
  const [candleStats, setCandleStats] = useState(null);

  const handleCandleStats = useCallback((s) => {
    if (s) setCandleStats((prev) => ({ ...prev, ...s }));
  }, []);

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
    const id = setInterval(reloadCurveState, 5000);
    return () => clearInterval(id);
  }, [mint, reloadCurveState]);

  // Pool reserves/recent-trades/price all poll on their own interval —
  // without an explicit kick, the page just looks unchanged right after a
  // trade until the next scheduled poll happens to land.
  const indexedPair = (curveState?.mint === mint ? curveState.pairAddress : null)
    || (token?.pairAddress && token.pairAddress !== mint ? token.pairAddress : null);
  const { trades: indexedTrades, stats: indexedStats, status: indexedStatus, reload: reloadIndexed } = useIndexedTrades(indexedPair);
  const useIndex = indexedStatus === 'ok';
  const trades = useIndex ? indexedTrades : chainTrades;
  const tradesStatus = useIndex ? 'ok' : (indexedStatus === 'loading' && chainTradesStatus !== 'ok' ? 'loading' : chainTradesStatus);

  // Event-driven refresh: a WebSocket candle push means a trade was just
  // indexed for this pair, so pull price/stats/reserves/progress right then
  // instead of waiting for the next timer tick.
  const lastLiveRef = useRef(0);
  useEffect(() => {
    if (!candleStats) return;
    const now = Date.now();
    if (now - lastLiveRef.current < 1500) return;
    lastLiveRef.current = now;
    reloadIndexed();
    reloadCurveState();
    reloadReserves();
  }, [candleStats, reloadIndexed, reloadCurveState, reloadReserves]);

  const handleTradeComplete = useCallback(() => {
    setChartRefresh((n) => n + 1);
    reloadCurveState();
    reloadReserves();
    reloadIndexed();
    reloadTrades();
  }, [reloadCurveState, reloadReserves, reloadTrades, reloadIndexed]);

  // Real stats computed from on-chain trades (backend metadata is often stale/0).
  const stats = useMemo(() => {
    if (useIndex && indexedStats) return indexedStats;
    if (!trades.length) return null;
    const now = Date.now() / 1000;
    const cutoff = now - 86400;
    const recent = trades.filter((t) => t.time >= cutoff);
    const price = trades[0].priceUsd;
    const vol24h = recent.reduce((s, t) => s + (t.solAmount || 0) * getSolUsd(), 0);
    const buys24h = recent.filter((t) => t.isBuy).length;
    const traders = new Set(trades.map((t) => t.maker)).size;
    const older = trades.filter((t) => t.time < cutoff);
    const initialPrice = curveState?.priceUsd || (30 / 1073000000) * getSolUsd();
    let ref = older.length ? older[0].priceUsd : (recent.length > 1 ? recent[recent.length - 1]?.priceUsd : initialPrice);
    const change = ref && ref > 0 ? ((price - ref) / ref) * 100 : null;
    return { price, vol24h, txns24h: recent.length, buys24h, sells24h: recent.length - buys24h, traders, change };
  }, [trades, curveState, useIndex, indexedStats]);

  const money = (v) => {
    if (v == null) return '—';
    if (v >= 1e6) return `$${(v / 1e6).toFixed(2)}M`;
    if (v >= 1e3) return `$${(v / 1e3).toFixed(1)}K`;
    return `$${v.toFixed(2)}`;
  };
  const priceStr = (p) => (p == null ? '—' : p < 0.001 ? `$${p.toExponential(2)}` : `$${p.toFixed(6)}`);
  // Real-time price, volume, change, and market cap updated from on-chain trades, candle stream, or curve
  const displayPrice = stats?.price ?? candleStats?.price ?? (curveState?.mint === mint ? curveState.priceUsd : null) ?? finiteNumber(token?.price) ?? null;
  const displayVolume = stats?.vol24h ?? candleStats?.volume ?? finiteNumber(token?.vol24h);
  // Once a live quote exists, never pair it with the old indexed percentage.
  // That combination was the visible "$3.16e-6 / +12%" mismatch. If the
  // direct trade history is temporarily unavailable, show an empty change
  // until a live reference is available instead of presenting stale data.
  const displayChange = stats?.change ?? candleStats?.change
    ?? ((stats?.price == null && candleStats?.price == null && curveState?.priceUsd == null)
      ? finiteNumber(token?.change24)
      : null);
  const currentCurve = curveState?.mint === mint ? curveState : null;
  const poolRes = reserves || currentCurve;
  const supply = finiteNumber(currentCurve?.tokenTotalSupply)
    || finiteNumber(poolRes?.tokenTotalSupply)
    || finiteNumber(token?.totalSupply)
    || 1e9;
  const displayMktCap = displayPrice != null ? displayPrice * supply : finiteNumber(token?.mktCap);
  const curveProgress = currentCurve
    ? (currentCurve.complete
      ? 1
      : currentCurve.source?.startsWith('PumpMeteora')
        ? 1 - (Number(currentCurve.realToken || 0) / 793100000)
        : 1 - (Number(currentCurve.realToken || 0) / 873000000))
    : null;
  const displayProgress = curveProgress != null && Number.isFinite(curveProgress)
    ? Math.max(0, Math.min(1, curveProgress))
    : finiteNumber(token?.domesticProgress);

  useEffect(() => {
    let alive = true;
    const refresh = async () => {
      const t = await fetchTokenByMint(mint, publicKey?.toString());
      if (!alive || !t) return;
      setToken((prev) => {
        const base = prev?.tokenAddress === mint ? prev : { tokenAddress: mint, pairAddress: defaultPair };
        return mergeToken(base, t);
      });
    };
    if (location.state?.token?.tokenAddress === mint) {
      setToken((prev) => mergeToken(prev, location.state.token));
    }
    refresh();
    const timer = window.setInterval(refresh, 15000);
    return () => { alive = false; window.clearInterval(timer); };
  }, [mint, publicKey, location.state, defaultPair]);

  const chartToken = useMemo(() => ({
    tokenAddress: mint,
    pairAddress: (curveState?.mint === mint ? curveState.pairAddress : null) || token?.pairAddress || defaultPair,
    ...token,
  }), [token, curveState, mint, defaultPair]);

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
            <TradingViewChart
              token={chartToken}
              refreshKey={chartRefresh}
              onCandleStats={handleCandleStats}
              visible
            />
          </div>

          <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
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
            <Stat label="Market Cap" value={displayMktCap != null ? money(displayMktCap) : '—'} />
            <Stat
              label="24h Trades"
              value={
                stats ? (
                  <span>
                    {fmt(stats.txns24h, 0)}{' '}
                    <span className="text-[11px] text-up">{stats.buys24h}B</span>
                    <span className="text-[11px] text-muted">/</span>
                    <span className="text-[11px] text-down">{stats.sells24h}S</span>
                  </span>
                ) : (
                  fmt(finiteNumber(token?.txs24h), 0)
                )
              }
            />
            <Stat label="Traders" value={fmt(stats?.traders, 0)} />
          </div>

          {/* Pool reserves — the two sides of the bonding-curve pool, read on-chain */}
          <div className="rounded-xl border border-border bg-bg-card p-4">
            <div className="mb-2 flex items-center justify-between">
              <span className="text-xs uppercase tracking-wide text-muted">Pool reserves</span>
              {reservesStatus === 'loading' && <span className="text-[11px] text-muted">loading…</span>}
              {reservesStatus === 'error' && !poolRes && <span className="text-[11px] text-down">unavailable</span>}
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

          {displayProgress != null && (
            <div className="rounded-xl border border-border bg-bg-card p-4">
              <div className="mb-1.5 flex justify-between text-xs text-muted">
                <span>Bonding curve progress</span>
                <span>{fmt(Math.min(100, Number(displayProgress) * 100), 1)}%</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-bg-soft">
                <div
                  className="h-full rounded-full bg-accent"
                  style={{ width: `${Math.min(100, Number(displayProgress) * 100)}%` }}
                />
              </div>
            </div>
          )}

          {token && <RecentTrades trades={trades} status={tradesStatus} />}
        </div>

        <div className="space-y-4 lg:sticky lg:top-4 lg:self-start">
          <TradePanel
            token={token}
            // Use the same resolved price shown in the header.
            currentPriceUsd={displayPrice ?? reserves?.priceUsd ?? 0}
            onLimitOrderPlaced={() => setOrdersTick((t) => t + 1)}
            onTradeComplete={handleTradeComplete}
          />
          <OpenOrders
            mint={mint}
            symbol={token?.tokenSymbol || 'TOKEN'}
            refreshTick={ordersTick}
          />
        </div>
      </div>
    </div>
  );
}
