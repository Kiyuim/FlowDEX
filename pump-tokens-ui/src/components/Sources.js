import React, { useState, useEffect, useCallback } from 'react';
import TokenCard from './TokenCard';
import ClmmPoolCard from './ClmmPoolCard';
import './TokenList.css';

const API_BASE_URL = process.env.NODE_ENV === 'development'
  ? ''
  : '/direct-api';

const CLMM_SOURCE = 'Raydium CLMM';

// Browse tokens/pools grouped by originating source (Pump.fun, Raydium CLMM,
// ...). token.program already exists on the backend (sol_token.program
// column) for pump tokens; CLMM pools aren't tokens with their own program
// value, so they're grouped under a fixed "Raydium CLMM" source instead —
// they're a genuinely different trading venue, not a program variant.
const Sources = ({ onTokenSelect }) => {
  const [tokens, setTokens] = useState([]);
  const [pools, setPools] = useState([]);
  const [solPriceUsd, setSolPriceUsd] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [activeSource, setActiveSource] = useState('all');

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const statuses = [1, 2, 4];
      const [tokenResponses, poolResponses] = await Promise.all([
        Promise.all(statuses.map(s => fetch(`${API_BASE_URL}/v1/market/index_pump?chain_id=100000&pump_status=${s}&page_no=1&page_size=50`))),
        Promise.all([1, 2].map(v => fetch(`${API_BASE_URL}/v1/market/index_clmm?chain_id=100000&pool_version=${v}&page_no=1&page_size=50`))),
      ]);
      const tokenBodies = await Promise.all(tokenResponses.map(r => r.json()));
      const poolBodies = await Promise.all(poolResponses.map(r => r.json()));
      setTokens(tokenBodies.flatMap(b => b?.data?.list || []));
      const mergedPools = poolBodies.flatMap(b => b?.data?.list || []);
      setPools(mergedPools);
      const price = poolBodies.find(b => b?.data?.solPriceUsd)?.data?.solPriceUsd;
      if (price) setSolPriceUsd(price);
    } catch (err) {
      setError(`Failed to fetch tokens: ${err.message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchAll(); }, [fetchAll]);

  const programOf = (token) => token.program && token.program.trim() !== '' ? token.program : 'Unknown';

  const counts = tokens.reduce((acc, t) => {
    const p = programOf(t);
    acc[p] = (acc[p] || 0) + 1;
    return acc;
  }, {});
  if (pools.length > 0) counts[CLMM_SOURCE] = pools.length;

  const sources = Object.keys(counts).sort((a, b) => counts[b] - counts[a]);
  const total = tokens.length + pools.length;

  const visibleTokens = activeSource === 'all' || activeSource !== CLMM_SOURCE
    ? tokens.filter(t => activeSource === 'all' || programOf(t) === activeSource)
    : [];
  const visiblePools = activeSource === 'all' || activeSource === CLMM_SOURCE ? pools : [];

  if (loading) {
    return (
      <div className="loading">
        <div className="loading-spinner"></div>
        <p>Loading sources...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="error">
        <p>{error}</p>
        <button onClick={fetchAll} className="retry-button">Retry</button>
      </div>
    );
  }

  return (
    <div className="token-list-container">
      <div className="token-section">
        <h3 className="section-title">📁 Sources</h3>
        <div className="tab-navigation" style={{ marginBottom: 16, flexWrap: 'wrap', justifyContent: 'center' }}>
          <button
            className={`tab-btn ${activeSource === 'all' ? 'active' : ''}`}
            onClick={() => setActiveSource('all')}
          >
            All ({total})
          </button>
          {sources.map(p => (
            <button
              key={p}
              className={`tab-btn ${activeSource === p ? 'active' : ''}`}
              onClick={() => setActiveSource(p)}
            >
              {p} ({counts[p]})
            </button>
          ))}
        </div>

        {visibleTokens.length === 0 && visiblePools.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">📁</div>
            <h3>No tokens from this source</h3>
          </div>
        ) : (
          <div className="token-grid">
            {visiblePools.map((pool) => (
              <ClmmPoolCard
                key={pool.poolState}
                pool={pool}
                solPriceUsd={solPriceUsd}
                onClick={() => onTokenSelect?.({
                  tokenAddress: pool.outputVaultMint,
                  pairAddress: pool.poolState,
                  tokenName: pool.outputTokenSymbol,
                  tokenSymbol: pool.outputTokenSymbol,
                  tokenIcon: pool.outputTokenIcon,
                  tokenType: 'CLMM',
                })}
              />
            ))}
            {visibleTokens.map((token, index) => (
              <TokenCard
                key={`${token.tokenAddress}-${index}`}
                token={token}
                onClick={() => onTokenSelect?.(token)}
                customBadge={programOf(token)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default Sources;
