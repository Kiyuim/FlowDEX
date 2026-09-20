import React, { useState, useCallback, useEffect } from 'react';
import useTokenListWebSocket from '../hooks/useTokenListWebSocket_improved';

const TokenListWithWebSocket = () => {
  const [tokens, setTokens] = useState([]);
  const [connectionMetrics, setConnectionMetrics] = useState({
    totalReceived: 0,
    newTokens: 0,
    updates: 0
  });

  // Callback for handling new tokens
  const handleNewToken = useCallback((tokenData) => {
    console.log('🆕 [COMPONENT] New token received:', tokenData);
    
    setTokens(prevTokens => {
      // Check if token already exists
      const existingIndex = prevTokens.findIndex(t => t.tokenAddress === tokenData.tokenAddress);
      
      if (existingIndex >= 0) {
        // Update existing token
        const updatedTokens = [...prevTokens];
        updatedTokens[existingIndex] = { ...updatedTokens[existingIndex], ...tokenData };
        return updatedTokens;
      } else {
        // Add new token at the beginning
        return [tokenData, ...prevTokens.slice(0, 49)]; // Keep only 50 latest tokens
      }
    });

    setConnectionMetrics(prev => ({
      ...prev,
      totalReceived: prev.totalReceived + 1,
      newTokens: prev.newTokens + 1
    }));
  }, []);

  // Callback for handling token updates
  const handleTokenUpdate = useCallback((tokenData) => {
    console.log('🔄 [COMPONENT] Token update received:', tokenData);
    
    setTokens(prevTokens => {
      const existingIndex = prevTokens.findIndex(t => t.tokenAddress === tokenData.token_address);
      
      if (existingIndex >= 0) {
        const updatedTokens = [...prevTokens];
        updatedTokens[existingIndex] = { 
          ...updatedTokens[existingIndex], 
          ...tokenData,
          tokenAddress: tokenData.token_address, // Normalize field names
          tokenName: tokenData.token_name,
          tokenSymbol: tokenData.token_symbol
        };
        return updatedTokens;
      }
      
      return prevTokens;
    });

    setConnectionMetrics(prev => ({
      ...prev,
      totalReceived: prev.totalReceived + 1,
      updates: prev.updates + 1
    }));
  }, []);

  // Use the WebSocket hook
  const {
    connectionStatus,
    lastMessage,
    clientCount,
    disconnect,
    reconnect,
    isConnected,
    isConnecting,
    isError
  } = useTokenListWebSocket(handleNewToken, handleTokenUpdate);

  // Format market cap for display
  const formatMarketCap = (mktCap) => {
    if (!mktCap) return 'Unknown';
    if (mktCap >= 1000000) return `$${(mktCap / 1000000).toFixed(2)}M`;
    if (mktCap >= 1000) return `$${(mktCap / 1000).toFixed(1)}K`;
    return `$${mktCap.toFixed(2)}`;
  };

  // Format time ago
  const formatTimeAgo = (timestamp) => {
    if (!timestamp) return 'Unknown';
    const now = Math.floor(Date.now() / 1000);
    const diff = now - timestamp;
    
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  };

  return (
    <div className="token-list-websocket">
      {/* Connection Status Header */}
      <div className={`connection-status ${connectionStatus}`}>
        <div className="status-info">
          <span className="status-indicator">
            {isConnected && '🟢'}
            {isConnecting && '🟡'} 
            {isError && '🔴'}
            {connectionStatus === 'disconnected' && '⚫'}
          </span>
          <span className="status-text">
            {isConnected && 'Connected'}
            {isConnecting && 'Connecting...'}
            {isError && 'Connection Error'}
            {connectionStatus === 'disconnected' && 'Disconnected'}
          </span>
          <span className="client-count">({clientCount} clients)</span>
        </div>
        
        <div className="connection-controls">
          <button 
            onClick={reconnect} 
            disabled={isConnecting}
            className="reconnect-btn"
          >
            {isConnecting ? 'Connecting...' : 'Reconnect'}
          </button>
          <button 
            onClick={disconnect}
            disabled={!isConnected}
            className="disconnect-btn"
          >
            Disconnect
          </button>
        </div>
      </div>

      {/* Metrics Dashboard */}
      <div className="metrics-dashboard">
        <div className="metric">
          <span className="metric-value">{tokens.length}</span>
          <span className="metric-label">Tokens Listed</span>
        </div>
        <div className="metric">
          <span className="metric-value">{connectionMetrics.totalReceived}</span>
          <span className="metric-label">Messages Received</span>
        </div>
        <div className="metric">
          <span className="metric-value">{connectionMetrics.newTokens}</span>
          <span className="metric-label">New Tokens</span>
        </div>
        <div className="metric">
          <span className="metric-value">{connectionMetrics.updates}</span>
          <span className="metric-label">Updates</span>
        </div>
      </div>

      {/* Token List */}
      <div className="token-list">
        <h2>🔥 Live Pump.fun Tokens</h2>
        
        {tokens.length === 0 ? (
          <div className="empty-state">
            {isConnected ? (
              <p>🔍 Waiting for new tokens...</p>
            ) : (
              <p>❌ Not connected to WebSocket</p>
            )}
          </div>
        ) : (
          <div className="token-grid">
            {tokens.map((token, index) => (
              <div key={token.tokenAddress || index} className="token-card">
                <div className="token-header">
                  <div className="token-icon">
                    {token.tokenIcon ? (
                      <img src={token.tokenIcon} alt={token.tokenSymbol} />
                    ) : (
                      <div className="placeholder-icon">💰</div>
                    )}
                  </div>
                  <div className="token-info">
                    <h3 className="token-name">{token.tokenName || 'Unnamed Token'}</h3>
                    <p className="token-symbol">${token.tokenSymbol || 'N/A'}</p>
                  </div>
                  <div className="token-status">
                    <span className={`status-badge status-${token.pumpStatus || 0}`}>
                      {token.pumpStatus === 1 ? 'Pumping' : 'New'}
                    </span>
                  </div>
                </div>
                
                <div className="token-metrics">
                  <div className="metric-item">
                    <span className="metric-label">Market Cap</span>
                    <span className="metric-value">{formatMarketCap(token.mktCap)}</span>
                  </div>
                  <div className="metric-item">
                    <span className="metric-label">Holders</span>
                    <span className="metric-value">{token.holdCount || 0}</span>
                  </div>
                  <div className="metric-item">
                    <span className="metric-label">Launched</span>
                    <span className="metric-value">{formatTimeAgo(token.launchTime)}</span>
                  </div>
                </div>

                {/* Social Links */}
                {(token.twitterUsername || token.telegram) && (
                  <div className="social-links">
                    {token.twitterUsername && (
                      <a 
                        href={`https://twitter.com/${token.twitterUsername}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="social-link twitter"
                      >
                        🐦 Twitter
                      </a>
                    )}
                    {token.telegram && (
                      <a 
                        href={token.telegram}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="social-link telegram"
                      >
                        ✈️ Telegram
                      </a>
                    )}
                  </div>
                )}

                <div className="token-actions">
                  <button 
                    className="view-btn"
                    onClick={() => window.open(`https://pump.fun/${token.tokenAddress}`, '_blank')}
                  >
                    View on Pump.fun
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Debug Information */}
      {/* Debug panel removed */}
    </div>
  );
};

export default TokenListWithWebSocket; 