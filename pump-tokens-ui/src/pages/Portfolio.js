import React, { useCallback, useEffect, useState } from 'react';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import { Link } from 'react-router-dom';
import { PublicKey } from '@solana/web3.js';
import { shortAddr } from '../lib/trade';

const TOKEN_PROGRAM_ID = new PublicKey('TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA');
const TOKEN_2022_PROGRAM_ID = new PublicKey('TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb');

async function tokenMetaMap(walletAddress) {
  const map = {};
  await Promise.all(
    [1, 2, 4].map((st) =>
      fetch(`/v1/market/index_pump?chain_id=100000&pump_status=${st}&page_no=1&page_size=100`)
        .then((r) => r.json())
        .then((d) => (d?.data?.list || []).forEach((t) => (map[t.tokenAddress] = t)))
        .catch(() => {})
    )
  );
  // Tokens recorded at creation time (record_user_asset) — the source of
  // truth for a token's real name/symbol regardless of whether the consumer
  // has indexed it yet, and the only source at all for a plain SPL mint
  // (index_pump only lists bonding-curve/pump tokens, which have a pair
  // record; a plain mint never gets one, so it would otherwise always fall
  // back to a truncated address instead of its real name).
  if (walletAddress) {
    await fetch(`/v1/market/user_tokens?chain_id=100000&wallet_address=${walletAddress}`)
      .then((r) => r.json())
      .then((d) =>
        (d?.data?.list || []).forEach((t) => {
          map[t.tokenAddress] = {
            ...map[t.tokenAddress],
            tokenName: t.tokenName,
            tokenSymbol: t.tokenSymbol,
            tokenIcon: map[t.tokenAddress]?.tokenIcon || t.tokenIcon,
          };
        })
      )
      .catch(() => {});
  }
  return map;
}

export default function Portfolio() {
  const { connection } = useConnection();
  const { publicKey, connected } = useWallet();
  const [sol, setSol] = useState(0);
  const [holdings, setHoldings] = useState([]);
  const [createdTokens, setCreatedTokens] = useState([]);
  const [createdPools, setCreatedPools] = useState([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!connected || !publicKey) return;
    setLoading(true);
    try {
      const wallet = publicKey.toString();
      const [bal, respStd, resp22, meta, userTokens, userPools] = await Promise.all([
        connection.getBalance(publicKey),
        connection.getParsedTokenAccountsByOwner(publicKey, { programId: TOKEN_PROGRAM_ID }),
        connection.getParsedTokenAccountsByOwner(publicKey, { programId: TOKEN_2022_PROGRAM_ID }).catch(() => ({ value: [] })),
        tokenMetaMap(wallet),
        fetch(`/v1/market/user_tokens?chain_id=100000&wallet_address=${wallet}`)
          .then((r) => r.json())
          .then((d) => d?.data?.list || [])
          .catch(() => []),
        fetch(`/v1/market/user_pools?chain_id=100000&wallet_address=${wallet}`)
          .then((r) => r.json())
          .then((d) => d?.data?.list || [])
          .catch(() => []),
      ]);
      setSol(bal / 1e9);
      const list = [...respStd.value, ...resp22.value]
        .map(({ account }) => {
          const info = account.data.parsed.info;
          return { mint: info.mint, amount: info.tokenAmount.uiAmount || 0 };
        })
        .filter((h) => h.amount > 0)
        .map((h) => ({ ...h, meta: meta[h.mint] }))
        .sort((a, b) => (b.meta ? 1 : 0) - (a.meta ? 1 : 0) || b.amount - a.amount);
      setHoldings(list);
      // Tokens/pools you created — separate from wallet balance, since a
      // bonding-curve creator mints the supply into the program's own vault,
      // not their own wallet, so a created token never shows up as a
      // "holding" no matter how much later trading happens.
      setCreatedTokens(userTokens);
      setCreatedPools(userPools);
    } finally {
      setLoading(false);
    }
  }, [connection, publicKey, connected]);

  useEffect(() => {
    load();
  }, [load]);

  if (!connected) {
    return (
      <div className="mx-auto max-w-3xl px-3 py-16 text-center md:px-6">
        <div className="text-4xl">👛</div>
        <h1 className="mt-3 text-lg font-bold text-ink">Your Portfolio</h1>
        <p className="mt-1 text-sm text-muted">Connect your wallet to see your holdings.</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl px-3 py-4 md:px-6">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-bold text-ink">Portfolio</h1>
          <p className="text-xs text-muted">{shortAddr(publicKey.toString(), 6)}</p>
        </div>
        <button
          onClick={load}
          className="rounded-md border border-border bg-bg-card px-3 py-1.5 text-sm text-muted hover:text-ink"
        >
          {loading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      <div className="mb-4 rounded-xl border border-border bg-bg-card p-4">
        <div className="text-xs uppercase tracking-wide text-muted">SOL Balance</div>
        <div className="mt-1 text-2xl font-bold text-ink">{sol.toFixed(4)} SOL</div>
      </div>

      {(createdTokens.length > 0 || createdPools.length > 0) && (
        <div className="mb-4">
          <h2 className="mb-2 text-sm font-semibold text-ink">Created by you</h2>
          <div className="space-y-2">
            {createdTokens.map((t) => (
              <Link
                key={`t-${t.tokenAddress}`}
                to={`/token/${t.tokenAddress}`}
                className="flex items-center gap-3 rounded-xl border border-border bg-bg-card p-3 transition hover:border-accent/50 hover:bg-bg-hover"
              >
                {t.tokenIcon ? (
                  <img src={t.tokenIcon} alt="" className="h-9 w-9 rounded-full object-cover" />
                ) : (
                  <div className="grid h-9 w-9 place-items-center rounded-full bg-bg-elev">🪙</div>
                )}
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium text-ink">
                    {t.tokenName || t.tokenSymbol || shortAddr(t.tokenAddress, 5)}
                  </div>
                  <div className="text-xs text-muted">{shortAddr(t.tokenAddress, 4)}</div>
                </div>
                <div className="text-xs text-accent">Trade →</div>
              </Link>
            ))}
            {createdPools.map((p) => (
              <div
                key={`p-${p.poolState}`}
                className="flex items-center gap-3 rounded-xl border border-border bg-bg-card p-3"
              >
                <div className="grid h-9 w-9 place-items-center rounded-full bg-bg-elev">🏊</div>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium text-ink">
                    {p.token0Symbol || 'Token'}/{p.token1Symbol || 'Token'} Pool
                  </div>
                  <div className="text-xs text-muted">{shortAddr(p.poolState, 4)}</div>
                </div>
                <div className="text-xs text-muted">{p.poolType || 'CLMM'}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      <h2 className="mb-2 text-sm font-semibold text-ink">Token Holdings</h2>
      {loading && holdings.length === 0 ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-16 animate-pulse rounded-xl border border-border bg-bg-card" />
          ))}
        </div>
      ) : holdings.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border p-8 text-center text-sm text-muted">
          No token holdings yet.{' '}
          <Link to="/" className="text-accent underline">
            Discover tokens
          </Link>
        </div>
      ) : (
        <div className="space-y-2">
          {holdings.map((h) => (
            <Link
              key={h.mint}
              to={`/token/${h.mint}`}
              state={{ token: h.meta }}
              className="flex items-center gap-3 rounded-xl border border-border bg-bg-card p-3 transition hover:border-accent/50 hover:bg-bg-hover"
            >
              {h.meta?.tokenIcon ? (
                <img src={h.meta.tokenIcon} alt="" className="h-9 w-9 rounded-full object-cover" />
              ) : (
                <div className="grid h-9 w-9 place-items-center rounded-full bg-bg-elev">🪙</div>
              )}
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium text-ink">
                  {h.meta?.tokenName || h.meta?.tokenSymbol || shortAddr(h.mint, 5)}
                </div>
                <div className="text-xs text-muted">{shortAddr(h.mint, 4)}</div>
              </div>
              <div className="text-right">
                <div className="font-semibold text-ink">
                  {h.amount.toLocaleString(undefined, { maximumFractionDigits: 4 })}
                </div>
                <div className="text-xs text-accent">Trade →</div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
