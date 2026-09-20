import { useCallback, useEffect, useRef, useState } from 'react';

const POLL_MS = 3000;

/**
 * A pair's recent trades and 24h stats from the backend index — the same
 * source the token list is built from, so the token page and its list card
 * agree, and it arrives in one request instead of the client paging
 * transactions one at a time through a rate-limited RPC.
 *
 * Rows are shaped like the on-chain hook's rows so RecentTrades can render
 * either. status: 'loading' | 'ok' | 'empty' | 'error'.
 */
export default function useIndexedTrades(pairAddress, limit = 50) {
  const [trades, setTrades] = useState([]);
  const [stats, setStats] = useState(null);
  const [status, setStatus] = useState('loading');
  const gen = useRef(0);

  const load = useCallback(async () => {
    if (!pairAddress) {
      setTrades([]); setStats(null); setStatus('empty');
      return;
    }
    const my = ++gen.current;
    try {
      const r = await fetch(`/v1/market/recent_trades?chain_id=100000&pair_address=${pairAddress}&limit=${limit}`);
      const d = await r.json();
      if (my !== gen.current) return;
      if (d.code && d.code !== 10000 && d.code !== 0) throw new Error(d.message || `Backend error ${d.code}`);
      const list = (d.data?.list || []).map((t) => ({
        sig: t.txHash,
        isBuy: t.tradeType === 'buy',
        solAmount: Number(t.baseTokenAmount) || 0,
        tokenAmount: Number(t.tokenAmount) || 0,
        priceUsd: Number(t.tokenPriceUsd) || 0,
        maker: t.maker,
        time: Number(t.blockTime) || 0,
      }));
      const s = d.data?.stats;
      setTrades(list);
      setStats(s && Number(s.txs24h) > 0 ? {
        price: Number(s.lastPriceUsd) || null,
        change: Number(s.change24h),
        vol24h: Number(s.vol24hUsd) || 0,
        txns24h: Number(s.txs24h) || 0,
        buys24h: Number(s.buys24h) || 0,
        sells24h: Number(s.sells24h) || 0,
        traders: Number(s.traders24h) || 0,
      } : null);
      setStatus(list.length ? 'ok' : 'empty');
    } catch (e) {
      if (my !== gen.current) return;
      setStatus('error');
    }
  }, [pairAddress, limit]);

  useEffect(() => {
    setStatus('loading');
    load();
    const t = setInterval(load, POLL_MS);
    return () => clearInterval(t);
  }, [load]);

  return { trades, stats, status, reload: load };
}
