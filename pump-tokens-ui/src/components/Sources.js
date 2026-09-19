import React, { useState, useEffect, useCallback } from 'react';
import TokenCard from './TokenCard';
import './TokenList.css';

const API_BASE_URL = process.env.NODE_ENV === 'development'
  ? ''
  : '/direct-api';

// Browse tokens grouped by originating program (Pump.fun, PumpMeteora, ...).
// token.program already exists on the backend (sol_token.program column) —
// this is just a view over the existing pump token list, grouped client-side
// rather than a new filtered backend query.
const Sources = ({ onTokenSelect }) => {
  const [tokens, setTokens] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [activeProgram, setActiveProgram] = useState('all');

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const statuses = [1, 2, 4];
      const responses = await Promise.all(
        statuses.map(s => fetch(`${API_BASE_URL}/v1/market/index_pump?chain_id=100000&pump_status=${s}&page_no=1&page_size=50`))
      );
      const bodies = await Promise.all(responses.map(r => r.json()));
      const merged = bodies.flatMap(b => b?.data?.list || []);
      setTokens(merged);
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

  const programs = Object.keys(counts).sort((a, b) => counts[b] - counts[a]);
  const visibleTokens = activeProgram === 'all' ? tokens : tokens.filter(t => programOf(t) === activeProgram);

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
        <div className="tab-navigation" style={{ marginBottom: 16, flexWrap: 'wrap' }}>
          <button
            className={`tab-btn ${activeProgram === 'all' ? 'active' : ''}`}
            onClick={() => setActiveProgram('all')}
          >
            All ({tokens.length})
          </button>
          {programs.map(p => (
            <button
              key={p}
              className={`tab-btn ${activeProgram === p ? 'active' : ''}`}
              onClick={() => setActiveProgram(p)}
            >
              {p} ({counts[p]})
            </button>
          ))}
        </div>

        {visibleTokens.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">📁</div>
            <h3>No tokens from this source</h3>
          </div>
        ) : (
          <div className="token-grid">
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
