import React, { useState } from 'react';

const API_URL =
  process.env.NODE_ENV === 'development'
    ? '/v1/market/token_security_check'
    : '/direct-api/v1/market/token_security_check';

function Row({ label, children }) {
  return (
    <div className="flex items-start justify-between gap-4 border-t border-border/60 py-2 text-sm">
      <span className="text-muted">{label}</span>
      <span className="break-all text-right font-mono text-ink">{children}</span>
    </div>
  );
}

export default function TokenSecurity() {
  const [address, setAddress] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setResult(null);
    if (!address.trim()) {
      setError('Please enter a token mint address.');
      return;
    }
    setLoading(true);
    try {
      const resp = await fetch(`${API_URL}?mint_address=${address.trim()}`);
      const data = await resp.json();
      if (data.code !== 10000 || !data.data) {
        setError(data.message || 'Token not found or error occurred.');
      } else {
        setResult(data.data);
      }
    } catch (err) {
      setError('Network error or invalid response.');
    } finally {
      setLoading(false);
    }
  };

  const badge = (ok) => (
    <span className={ok ? 'text-up' : 'text-down'}>{ok ? 'Safe' : 'Risk'}</span>
  );

  return (
    <div>
      <div className="mb-4">
        <h1 className="text-lg font-bold text-ink">🛡️ Token Security Check</h1>
        <p className="text-xs text-muted">Check a Solana SPL token mint's authorities and status.</p>
      </div>

      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          className="flex-1 rounded-lg border border-border bg-bg-soft px-3 py-2.5 text-sm text-ink outline-none focus:border-accent"
          type="text"
          placeholder="Enter token mint address…"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          disabled={loading}
        />
        <button
          type="submit"
          disabled={loading}
          className="rounded-lg bg-accent px-4 py-2.5 text-sm font-semibold text-bg transition hover:brightness-110 disabled:opacity-50"
        >
          {loading ? 'Checking…' : 'Check'}
        </button>
      </form>

      {error && (
        <div className="mt-3 rounded-lg border border-down/40 bg-down/10 px-3 py-2 text-sm text-down">{error}</div>
      )}

      {result && (
        <div className="mt-4 rounded-xl border border-border bg-bg-card p-4">
          <Row label="Mint Address">{result.mintAddress}</Row>
          <Row label="Decimals">{result.decimals}</Row>
          <Row label="Initialized">{result.isInitialized ? 'Yes' : 'No'}</Row>
          <Row label="Mint Authority">
            {result.mintAuthority || <span className="text-up">None ✅</span>}
          </Row>
          <Row label="Freeze Authority">
            {result.freezeAuthority || <span className="text-up">None ✅</span>}
          </Row>
          <Row label="Mint Authority">{badge(result.mintAuthoritySafe)}</Row>
          <Row label="Freeze Authority">{badge(result.freezeAuthoritySafe)}</Row>
          <Row label="Summary">{result.securitySummary}</Row>
        </div>
      )}
    </div>
  );
}
