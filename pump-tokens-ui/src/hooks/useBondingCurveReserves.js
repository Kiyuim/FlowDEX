import { useCallback, useEffect, useState } from 'react';
import { useConnection } from '@solana/wallet-adapter-react';
import { deriveBondingCurve, parseBondingCurve } from '../lib/pump';
import { readCurveState } from '../lib/curve';

// Reads the two pool reserves (token + SOL) straight from the token's bonding
// curve account and polls so the trade page stays roughly live. Source-aware:
// tries pump.fun's own (fast-path) parser first, then falls back to the
// source-aware pump-meteora reader. Returns
// { reserves: {realToken, realSol, virtualToken, virtualSol, complete, priceUsd} | null, status }.
export default function useBondingCurveReserves(mint, { pollMs = 15000 } = {}) {
  const { connection } = useConnection();
  const [reserves, setReserves] = useState(null);
  const [status, setStatus] = useState('loading');

  const load = useCallback(async () => {
    if (!mint) return;
    try {
      const curve = deriveBondingCurve(mint);
      const acc = await connection.getAccountInfo(curve);
      const parsed = acc?.data ? parseBondingCurve(acc.data) : null;
      if (parsed) {
        setReserves(parsed);
        setStatus('ok');
        return;
      }
      const curveState = await readCurveState(connection, mint);
      if (curveState) {
        setReserves(curveState);
        setStatus('ok');
        return;
      }
      setStatus((s) => (s === 'ok' ? 'ok' : 'empty'));
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('useBondingCurveReserves', e);
      setStatus((s) => (s === 'ok' ? 'ok' : 'error'));
    }
  }, [connection, mint]);

  useEffect(() => {
    load();
    const id = setInterval(load, pollMs);
    return () => clearInterval(id);
  }, [load, pollMs]);

  return { reserves, status, reload: load };
}
