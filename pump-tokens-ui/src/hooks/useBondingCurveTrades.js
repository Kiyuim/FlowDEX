import { useCallback, useEffect, useRef, useState } from 'react';
import { useConnection } from '@solana/wallet-adapter-react';
import { deriveBondingCurve, parsePumpEventLog } from '../lib/pump';
import { METEORA_PROGRAMS, deriveMeteoraBondingCurve, parseMeteoraSwapEventLog } from '../lib/meteora';

const REQ_DELAY_MS = 250;
const RATE_LIMIT_COOLDOWN_MS = 60000;
const MAX_NEW_PER_POLL = 15; // cap RPC calls per poll; the rest catch up next poll

function curveCandidates(mint) {
  return [
    { curve: deriveBondingCurve(mint), parse: parsePumpEventLog },
    { curve: deriveMeteoraBondingCurve(mint, METEORA_PROGRAMS.meteora), parse: parseMeteoraSwapEventLog },
    { curve: deriveMeteoraBondingCurve(mint, METEORA_PROGRAMS.meteorav2), parse: parseMeteoraSwapEventLog },
  ];
}

// Reconstructs recent trades for a bonding-curve token directly from the chain:
// fetch the curve's recent signatures, parse the pump vdt/007m event from each.
// Returns trades newest-first. Shared by the chart and the trades feed so they
// don't double-fetch.
//
// Fetches transactions ONE AT A TIME (getParsedTransaction, not the plural
// getParsedTransactions) — the plural form sends a single batched JSON-RPC
// request, which Helius's non-paid plans reject outright with a 403
// ("Batch requests are only available for paid plans"), not just rate-limit.
// Already-parsed signatures are cached and skipped on later polls, and a 429
// puts the hook in a cooldown instead of hammering the endpoint every poll.
export default function useBondingCurveTrades(mint, { limit = 80, pollMs = 20000 } = {}) {
  const { connection } = useConnection();
  const [trades, setTrades] = useState([]);
  const [status, setStatus] = useState('loading');
  const cacheRef = useRef({ mint: null, bySig: new Map(), backoffUntil: 0 });

  const load = useCallback(async () => {
    if (!mint) return;
    const cache = cacheRef.current;
    if (cache.mint !== mint) {
      cache.mint = mint;
      cache.bySig = new Map();
      cache.backoffUntil = 0;
    }
    if (Date.now() < cache.backoffUntil) return;
    try {
      // Source-aware: try pump.fun's curve first (the common case), then both
      // pump-meteora builds. Whichever program's curve account actually has
      // signatures is this token's real source.
      const candidates = curveCandidates(mint);
      let curve, parseEvent, sigs = [];
      for (const c of candidates) {
        const s = await connection.getSignaturesForAddress(c.curve, { limit });
        if (s.length) {
          curve = c.curve;
          parseEvent = c.parse;
          sigs = s;
          break;
        }
      }
      if (!sigs.length) {
        setStatus('empty');
        return;
      }
      if (cache.curveKey !== curve.toBase58()) {
        cache.curveKey = curve.toBase58();
        cache.bySig = new Map();
      }

      const newSigs = sigs.filter((s) => !cache.bySig.has(s.signature)).slice(0, MAX_NEW_PER_POLL);
      for (let i = 0; i < newSigs.length; i++) {
        const sig = newSigs[i].signature;
        try {
          const tx = await connection.getParsedTransaction(sig, { maxSupportedTransactionVersion: 0 });
          const events = [];
          const logs = tx?.meta?.logMessages;
          if (logs) {
            for (const log of logs) {
              const ev = parseEvent(log);
              if (ev) events.push({ ...ev, time: ev.timestamp || tx.blockTime || 0, sig });
            }
          }
          cache.bySig.set(sig, events);
        } catch (txErr) {
          const msg = String(txErr?.message || txErr).toLowerCase();
          if (msg.includes('too many requests') || msg.includes('429')) {
            cache.backoffUntil = Date.now() + RATE_LIMIT_COOLDOWN_MS;
            break; // stop this poll's fetching; resume next poll after cooldown
          }
          // leave this signature unfetched — retried on a later poll
        }
        if (i + 1 < newSigs.length) {
          await new Promise((r) => setTimeout(r, REQ_DELAY_MS));
        }
      }

      // Assemble from the current signature window; entries outside it stay
      // cached (cheap) but don't grow the result.
      const out = [];
      for (const s of sigs) {
        const evs = cache.bySig.get(s.signature);
        if (evs) out.push(...evs);
      }
      if (!out.length) {
        setStatus('empty');
        return;
      }
      out.sort((a, b) => b.time - a.time); // newest first
      setTrades(out);
      setStatus('ok');
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('useBondingCurveTrades', e);
      const msg = String(e?.message || e).toLowerCase();
      if (msg.includes('too many requests') || msg.includes('429')) {
        cache.backoffUntil = Date.now() + RATE_LIMIT_COOLDOWN_MS;
      }
      // partial progress is kept in the cache; next poll resumes where we left off
      setStatus((s) => (s === 'ok' ? 'ok' : 'error'));
    }
  }, [connection, mint, limit]);

  // Subscribe directly to the curve accounts. This is the low-latency path:
  // Solana emits the transaction logs immediately, while the historical
  // signature poll and Railway consumer can trail by minutes under load.
  useEffect(() => {
    if (!mint) return undefined;
    let cancelled = false;
    const subscriptions = [];
    const sockets = [];
    const heliusKey = process.env.REACT_APP_HELIUS_API_KEY;
    const wsEndpoint = process.env.REACT_APP_SOLANA_WS_URL ||
      (heliusKey ? `wss://devnet.helius-rpc.com/?api-key=${heliusKey}` : null);

    const handleLogs = (parse, logInfo) => {
      if (cancelled || logInfo?.err || !logInfo?.logs) return;
      const events = logInfo.logs
        .map((log) => parse(log))
        .filter(Boolean)
        .map((event) => ({ ...event, time: Math.floor(Date.now() / 1000), sig: logInfo.signature }));
      if (!events.length) return;
      const cache = cacheRef.current;
      if (cache.mint !== mint) return;
      cache.bySig.set(logInfo.signature, events);
      setTrades((previous) => {
        const seen = new Set();
        return [...events, ...previous]
          .filter((trade) => {
            const key = `${trade.sig}:${trade.time}:${trade.priceUsd}:${trade.tokenAmount}`;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
          })
          .sort((a, b) => b.time - a.time)
          .slice(0, limit);
      });
      setStatus('ok');
    };

    const subscribe = async () => {
      for (const { curve, parse } of curveCandidates(mint)) {
        if (wsEndpoint && typeof WebSocket !== 'undefined') {
          try {
            const ws = new WebSocket(wsEndpoint);
            sockets.push(ws);
            ws.onopen = () => ws.send(JSON.stringify({
              jsonrpc: '2.0', id: 1, method: 'logsSubscribe',
              params: [{ mentions: [curve.toBase58()] }, { commitment: 'confirmed' }],
            }));
            ws.onmessage = (event) => {
              try {
                const msg = JSON.parse(event.data);
                const value = msg?.params?.result?.value;
                if (value) handleLogs(parse, { logs: value.logs, err: value.err, signature: value.signature });
              } catch (_) { /* ignore malformed provider messages */ }
            };
            ws.onerror = () => ws.close();
            continue;
          } catch (e) {
            console.warn('Helius logs WebSocket unavailable', e);
          }
        }
        if (!connection?.onLogs) continue;
        try {
          const id = await connection.onLogs(curve, (logInfo) => {
            handleLogs(parse, logInfo);
          }, 'confirmed');
          if (!cancelled) subscriptions.push(id);
          else await connection.removeOnLogsListener(id);
        } catch (e) {
          // Some public RPC endpoints disable logsSubscribe. The existing
          // signature polling remains the fallback in that case.
          console.warn('curve logs subscription unavailable', e);
        }
      }
    };
    subscribe();
    return () => {
      cancelled = true;
      sockets.forEach((ws) => { ws.onclose = null; ws.close(); });
      for (const id of subscriptions) connection.removeOnLogsListener(id).catch(() => {});
    };
  }, [connection, mint, limit]);

  useEffect(() => {
    load();
    const id = setInterval(load, pollMs);
    return () => clearInterval(id);
  }, [load, pollMs]);

  return { trades, status, reload: load };
}
