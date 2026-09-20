import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { PublicKey } from '@solana/web3.js';
import DiscoveryCard from '../components/DiscoveryCard';

// Token Source page: browse tokens by their originating on-chain program ("source").
// Three sources are supported:
//   - Pump.fun        : 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P
//   - PumpMeteora     : AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5 (our own bonding-curve contract)
//   - PumpMeteora V2  : 241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ (optimized/hardened build)
// The market feed tags each token's source (backend `program` field, set by the consumer's
// indexer). Prefer that; fall back to deriving the bonding-curve PDA client-side and matching
// the token's pairAddress, for the rare case the backend hasn't tagged a token yet.

const PUMPFUN_PROGRAM = new PublicKey('6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P');
const METEORA_PROGRAM = new PublicKey('AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5');
const METEORA_V2_PROGRAM = new PublicKey('241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ');

const _pdaCache = new Map();
function bondingCurvePda(mint, program, seed) {
  const key = `${mint}|${program.toBase58()}|${seed}`;
  if (_pdaCache.has(key)) return _pdaCache.get(key);
  let out = null;
  try {
    out = PublicKey.findProgramAddressSync(
      [new TextEncoder().encode(seed), new PublicKey(mint).toBuffer()],
      program
    )[0].toBase58();
  } catch (_) {
    out = null;
  }
  _pdaCache.set(key, out);
  return out;
}

const mintOf = (t) => t?.tokenAddress || t?.token_ca || t?.address;
const pairOf = (t) => t?.pairAddress || t?.pair_address || t?.pairAddr;

// Returns 'pumpmeteorav2' | 'pumpmeteora' | 'pumpfun' | 'unknown'
function detectSource(t) {
  // Prefer the backend's own tag (pair.Name, set by the consumer's per-program
  // decoder when the trade/create was indexed) — real and correct for any
  // token that actually went through on-chain indexing.
  const backendProgram = (t?.program || '').toLowerCase();
  if (backendProgram === 'pumpmeteorav2') return 'pumpmeteorav2';
  if (backendProgram === 'pumpmeteora') return 'pumpmeteora';
  if (backendProgram === 'pumpfun') return 'pumpfun';

  // Fallback for tokens the backend hasn't tagged: derive the bonding-curve
  // PDA client-side and match against the token's pairAddress.
  const mint = mintOf(t);
  const pair = pairOf(t);
  if (!mint || !pair) return 'unknown';
  if (bondingCurvePda(mint, METEORA_V2_PROGRAM, 'bonding_curve') === pair) return 'pumpmeteorav2';
  if (bondingCurvePda(mint, METEORA_PROGRAM, 'bonding_curve') === pair) return 'pumpmeteora';
  if (bondingCurvePda(mint, PUMPFUN_PROGRAM, 'bonding-curve') === pair) return 'pumpfun';
  return 'unknown';
}

const SOURCES = [
  { key: 'all', label: 'All Sources', match: () => true },
  {
    key: 'pumpfun',
    label: 'Pump.fun',
    program: '6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P',
    match: (t) => detectSource(t) === 'pumpfun',
  },
  {
    key: 'pumpmeteora',
    label: 'PumpMeteora',
    program: 'AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5',
    match: (t) => detectSource(t) === 'pumpmeteora',
  },
  {
    key: 'pumpmeteorav2',
    label: 'PumpMeteora V2',
    program: '241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ',
    match: (t) => detectSource(t) === 'pumpmeteorav2',
  },
];

const STATUSES = [1, 2, 4]; // new, completing, graduated

export default function TokenSource() {
  const [tokens, setTokens] = useState([]);
  const [loading, setLoading] = useState(true);
  const [source, setSource] = useState('all');
  const [query, setQuery] = useState('');

  const fetchAll = useCallback(async () => {
    try {
      const results = await Promise.all(
        STATUSES.map((st) =>
          fetch(`/v1/market/index_pump?chain_id=100000&pump_status=${st}&page_no=1&page_size=50`)
            .then((r) => r.json())
            .then((d) => d?.data?.list || [])
            .catch(() => [])
        )
      );
      const seen = new Set();
      const merged = [];
      for (const arr of results) {
        for (const t of arr) {
          const m = mintOf(t);
          if (!m || seen.has(m)) continue;
          seen.add(m);
          merged.push(t);
        }
      }
      setTokens(merged);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
    const id = setInterval(fetchAll, 15000);
    return () => clearInterval(id);
  }, [fetchAll]);

  const active = SOURCES.find((s) => s.key === source) || SOURCES[0];

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return tokens.filter((t) => {
      if (!active.match(t)) return false;
      if (!q) return true;
      return (
        (t.tokenName || '').toLowerCase().includes(q) ||
        (t.tokenSymbol || '').toLowerCase().includes(q) ||
        (mintOf(t) || '').toLowerCase().includes(q)
      );
    });
  }, [tokens, active, query]);

  return (
    <div className="mx-auto max-w-7xl px-3 py-4 md:px-6">
      <div className="mb-4">
        <h1 className="text-lg font-bold text-ink">Token Source</h1>
        <p className="text-xs text-muted">
          Browse and trade tokens by their originating program.
        </p>
      </div>

      {/* Source selector (segmented) */}
      <div className="mb-3 inline-flex rounded-lg bg-bg-soft p-1">
        {SOURCES.map((s) => (
          <button
            key={s.key}
            onClick={() => setSource(s.key)}
            className={`rounded-md px-3 py-1.5 text-sm font-medium transition ${
              source === s.key ? 'bg-bg-elev text-ink shadow-sm' : 'text-muted hover:text-ink'
            }`}
          >
            {s.label}
          </button>
        ))}
      </div>

      {active.program && (
        <div className="mb-3 rounded-lg border border-border bg-bg-card px-3 py-2 text-xs text-muted">
          Program:{' '}
          <span className="font-mono text-ink">{active.program}</span>
        </div>
      )}

      {/* Search */}
      <div className="mb-3">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search name, symbol, or address…"
          className="w-full rounded-lg border border-border bg-bg-soft px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
      </div>

      <div className="mb-2 flex items-baseline justify-between">
        <h2 className="text-sm font-semibold text-ink">{active.label}</h2>
        <span className="text-xs text-muted">{shown.length}</span>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {loading && shown.length === 0 ? (
          Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-24 animate-pulse rounded-xl border border-border bg-bg-card" />
          ))
        ) : shown.length === 0 ? (
          <div className="col-span-full rounded-xl border border-dashed border-border p-8 text-center text-xs text-muted">
            {source.startsWith('pumpmeteora')
              ? 'No tokens for this source indexed yet. They appear once the consumer indexes this program on devnet.'
              : query
              ? 'No matches'
              : 'No tokens'}
          </div>
        ) : (
          shown.map((t) => <DiscoveryCard key={mintOf(t)} token={t} />)
        )}
      </div>
    </div>
  );
}
