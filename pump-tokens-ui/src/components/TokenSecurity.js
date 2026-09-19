import React, { useState } from 'react';
import './TokenSecurity.css';

const API_URL = process.env.NODE_ENV === 'development' 
  ? '/v1/market/token_security_check' // Use proxy in development
  : '/direct-api/v1/market/token_security_check'; // Use Nginx proxy in production

const TokenSecurity = () => {
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

  return (
    <div className="token-security-container">
      <div className="token-security-card">
        <div className="card-header">
          <h2>🛡️ Token Security Check</h2>
          <p>Enter a Solana SPL Token Mint address to check its security status.</p>
        </div>
        <form className="search-form" onSubmit={handleSubmit}>
          <input
            className="search-input"
            type="text"
            placeholder="Enter token mint address..."
            value={address}
            onChange={e => setAddress(e.target.value)}
            disabled={loading}
          />
          <button className="search-btn" type="submit" disabled={loading}>
            {loading ? 'Checking...' : 'Check'}
          </button>
        </form>
        {error && <div className="error-message">{error}</div>}
        {result && (
          <div className="result-section">
            <div className="result-row"><span className="label">Mint Address:</span> <span className="value">{result.mintAddress}</span></div>
            <div className="result-row"><span className="label">Decimals:</span> <span className="value">{result.decimals}</span></div>
            <div className="result-row"><span className="label">Initialized:</span> <span className="value">{result.isInitialized ? 'Yes' : 'No'}</span></div>
            <div className="result-row"><span className="label">Mint Authority:</span> <span className="value">{result.mintAuthority || <span className="safe">None ✅</span>}</span></div>
            <div className="result-row"><span className="label">Freeze Authority:</span> <span className="value">{result.freezeAuthority || <span className="safe">None ✅</span>}</span></div>
            <div className="result-row"><span className="label">Mint Authority Safe:</span> <span className={`value ${result.mintAuthoritySafe ? 'safe' : 'risk'}`}>{result.mintAuthoritySafe ? 'Safe' : 'Risk'}</span></div>
            <div className="result-row"><span className="label">Freeze Authority Safe:</span> <span className={`value ${result.freezeAuthoritySafe ? 'safe' : 'risk'}`}>{result.freezeAuthoritySafe ? 'Safe' : 'Risk'}</span></div>
            <div className="result-row"><span className="label">Security Summary:</span> <span className="value summary">{result.securitySummary}</span></div>
          </div>
        )}
      </div>
    </div>
  );
};

export default TokenSecurity; 