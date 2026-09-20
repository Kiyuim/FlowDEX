import { useCallback, useEffect, useRef, useState } from 'react';
import { useConnection } from '@solana/wallet-adapter-react';
import { deriveBondingCurve, parsePumpEventLog } from '../lib/pump';
import { METEORA_PROGRAMS, deriveMeteoraBondingCurve, parseMeteoraSwapEventLog } from '../lib/meteora';

const CHUNK = 20; // public devnet RPC counts each batch item; keep calls small
const CHUNK_DELAY_MS = 400;
const RATE_LIMIT_COOLDOWN_MS = 60000;

// Reconstructs recent trades for a bonding-curve token directly from the chain:
// fetch the curve's recent signatures, parse the pump vdt/007m event from each.
// Returns trades newest-first. Shared by the chart and the trades feed so they
// don't double-fetch.
//
// The public devnet RPC rate-limits getParsedTransactions aggressively, so:
// already-parsed signatures are cached and skipped on later polls, new ones are
// fetched in small chunks, and a 429 puts the hook in a cooldown instead of
// hammering the endpoint every poll.
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
      const candidates = [
        { curve: deriveBondingCurve(mint), parse: parsePumpEventLog },
        { curve: deriveMeteoraBondingCurve(mint, METEORA_PROGRAMS.meteora), parse: parseMeteoraSwapEventLog },
        { curve: deriveMeteoraBondingCurve(mint, METEORA_PROGRAMS.meteorav2), parse: parseMeteoraSwapEventLog },
      ];
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

      const newSigs = sigs.filter((s) => !cache.bySig.has(s.signature));
      for (let i = 0; i < newSigs.length; i += CHUNK) {
        const chunk = newSigs.slice(i, i + CHUNK);
        const txs = await connection.getParsedTransactions(
          chunk.map((s) => s.signature),
          { maxSupportedTransactionVersion: 0 }
        );
        txs.forEach((tx, j) => {
          const sig = chunk[j].signature;
          const events = [];
          const logs = tx?.meta?.logMessages;
          if (logs) {
            for (const log of logs) {
              const ev = parseEvent(log);
              if (ev) events.push({ ...ev, time: ev.timestamp || tx.blockTime || 0, sig });
            }
          }
          cache.bySig.set(sig, events);
        });
        if (i + CHUNK < newSigs.length) {
          await new Promise((r) => setTimeout(r, CHUNK_DELAY_MS));
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

  useEffect(() => {
    load();
    const id = setInterval(load, pollMs);
    return () => clearInterval(id);
  }, [load, pollMs]);

  return { trades, status, reload: load };
}
