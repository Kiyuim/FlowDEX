import React, { useState, useEffect, useRef, useCallback } from 'react';
import TokenCard from './TokenCard';
import ClmmPoolCard from './ClmmPoolCard';
import useTokenListWebSocket from '../hooks/useTokenListWebSocket';
import './TokenList.css';

const API_BASE_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

const TokenList = ({ onTokenSelect }) => {
  // Tab state
  const [activeTab, setActiveTab] = useState('pumpfun'); // 'pumpfun' or 'clmm'
  
  // PumpFun state
  const [newTokens, setNewTokens] = useState([]);
  const [completingTokens, setCompletingTokens] = useState([]);
  const [completedTokens, setCompletedTokens] = useState([]);
  
  // CLMM state
  const [clmmV1Pools, setClmmV1Pools] = useState([]);
  const [clmmV2Pools, setClmmV2Pools] = useState([]);
  const [solPriceUsd, setSolPriceUsd] = useState(0);
  
  // Common state
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [lastUpdate, setLastUpdate] = useState(null);
  const [realtimeCount, setRealtimeCount] = useState(0);
  const [newTokenNotifications, setNewTokenNotifications] = useState([]);
  const intervalRef = useRef(null);
  const setupCountRef = useRef(0);
  const componentIdRef = useRef(Math.random().toString(36).substr(2, 9));
  const hasInitializedRef = useRef(false);

  console.log(`🔄 TokenList render - Component ID: ${componentIdRef.current}`);
  console.log(`📊 TokenList current state:`, {
    activeTab,
    newTokens: newTokens.length,
    completingTokens: completingTokens.length, 
    completedTokens: completedTokens.length,
    clmmV1Pools: clmmV1Pools.length,
    clmmV2Pools: clmmV2Pools.length,
    loading,
    error: !!error,
    lastUpdate: lastUpdate?.toLocaleTimeString() || 'none',
    hasInitialized: hasInitializedRef.current,
    realtimeCount
  });

  // WebSocket handlers for PumpFun real-time updates
  const handleNewToken = useCallback((tokenData) => {
    if (activeTab !== 'pumpfun') return; // Only process for PumpFun tab
    
    console.log(`🆕 [${componentIdRef.current}] New token received via WebSocket:`, tokenData);
    
    const newToken = {
      id: tokenData.tokenAddress,
      tokenAddress: tokenData.tokenAddress,
      tokenName: tokenData.tokenName || tokenData.tokenSymbol,
      tokenIcon: tokenData.tokenIcon || '',
      launchTime: tokenData.launchTime,
      mktCap: tokenData.mktCap || 0,
      holdCount: tokenData.holdCount || 0,
      change24: 0,
      txs24h: 0,
      pairAddress: tokenData.pairAddress,
      _realtimeTimestamp: Date.now()
    };

    const pumpStatus = tokenData.pumpStatus || 1;
    
    if (pumpStatus === 1) {
      setNewTokens(prevTokens => {
        const exists = prevTokens.some(token => token.tokenAddress === newToken.tokenAddress);
        if (exists) return prevTokens;
        return [newToken, ...prevTokens];
      });
    } else if (pumpStatus === 2) {
      setCompletingTokens(prevTokens => {
        const exists = prevTokens.some(token => token.tokenAddress === newToken.tokenAddress);
        if (exists) return prevTokens;
        return [newToken, ...prevTokens];
      });
    } else if (pumpStatus === 4) {
      setCompletedTokens(prevTokens => {
        const exists = prevTokens.some(token => token.tokenAddress === newToken.tokenAddress);
        if (exists) return prevTokens;
        return [newToken, ...prevTokens];
      });
    }

    const notificationId = Date.now();
    setNewTokenNotifications(prev => [{
      id: notificationId,
      tokenName: newToken.tokenName,
      timestamp: Date.now()
    }, ...prev.slice(0, 4)]);

    setTimeout(() => {
      setNewTokenNotifications(prev => 
        prev.filter(notification => notification.id !== notificationId)
      );
    }, 5000);

    setRealtimeCount(prev => prev + 1);
  }, [activeTab]);

  const handleTokenUpdate = useCallback((tokenData) => {
    if (activeTab !== 'pumpfun') return; // Only process for PumpFun tab
    
    const { tokenAddress, pumpStatus, oldPumpStatus } = tokenData;
    
    if (oldPumpStatus !== pumpStatus) {
      // Remove from old list
      if (oldPumpStatus === 1) {
        setNewTokens(prev => prev.filter(token => token.tokenAddress !== tokenAddress));
      } else if (oldPumpStatus === 2) {
        setCompletingTokens(prev => prev.filter(token => token.tokenAddress !== tokenAddress));
      } else if (oldPumpStatus === 4) {
        setCompletedTokens(prev => prev.filter(token => token.tokenAddress !== tokenAddress));
      }
      
      // Add to new list
      const updatedToken = { ...tokenData, id: tokenAddress, _realtimeTimestamp: Date.now() };
      
      if (pumpStatus === 1) {
        setNewTokens(prev => [updatedToken, ...prev]);
      } else if (pumpStatus === 2) {
        setCompletingTokens(prev => [updatedToken, ...prev]);
      } else if (pumpStatus === 4) {
        setCompletedTokens(prev => [updatedToken, ...prev]);
      }
    }
  }, [activeTab]);

  // Initialize WebSocket connection
  const { connectionStatus } = useTokenListWebSocket(handleNewToken, handleTokenUpdate);

  // Fetch PumpFun tokens
  const fetchTokens = useCallback(async () => {
    setLoading(true);
    try {
      const [resNew, resCompleting, resCompleted] = await Promise.all([
        fetch(`${API_BASE_URL}/v1/market/index_pump?chain_id=100000&pump_status=1&page_no=1&page_size=50`),
        fetch(`${API_BASE_URL}/v1/market/index_pump?chain_id=100000&pump_status=2&page_no=1&page_size=50`),
        fetch(`${API_BASE_URL}/v1/market/index_pump?chain_id=100000&pump_status=4&page_no=1&page_size=50`)
      ]);

      const [dataNew, dataCompleting, dataCompleted] = await Promise.all([
        resNew.json(),
        resCompleting.json(),
        resCompleted.json()
      ]);

      const newTokens = (dataNew?.data?.list) || [];
      const completingTokens = (dataCompleting?.data?.list) || [];
      const completedTokens = (dataCompleted?.data?.list) || [];
      
      setNewTokens(newTokens);
      setCompletingTokens(completingTokens);
      setCompletedTokens(completedTokens);
      setError('');
      setLastUpdate(new Date());
    } catch (err) {
      setError(`Failed to fetch tokens: ${err.message}`);
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch CLMM pools
  const fetchPools = async () => {
    try {
      setLoading(true);
      setError('');

      const [resV1, resV2] = await Promise.all([
        fetch(`${API_BASE_URL}/v1/market/index_clmm?chain_id=100000&pool_version=1&page_no=1&page_size=20`),
        fetch(`${API_BASE_URL}/v1/market/index_clmm?chain_id=100000&pool_version=2&page_no=1&page_size=20`)
      ]);

      const [dataV1, dataV2] = await Promise.all([
        resV1.json(),
        resV2.json()
      ]);

      console.log('CLMM API responses:', { dataV1, dataV2 });

      if (resV1.ok && (dataV1.code === 0 || dataV1.code === 10000)) {
        const pools = dataV1.data?.list || [];
        console.log('CLMM V1 pools fetched:', pools);
        setClmmV1Pools(pools);
        if (dataV1.data?.solPriceUsd) setSolPriceUsd(dataV1.data.solPriceUsd);
      } else {
        console.error('CLMM V1 API error:', dataV1);
      }

      if (resV2.ok && (dataV2.code === 0 || dataV2.code === 10000)) {
        const pools = dataV2.data?.list || [];
        console.log('CLMM V2 pools fetched:', pools);
        setClmmV2Pools(pools);
        if (dataV2.data?.solPriceUsd) setSolPriceUsd(dataV2.data.solPriceUsd);
      } else {
        console.error('CLMM V2 API error:', dataV2);
      }

      setLastUpdate(new Date());
      setError('');
    } catch (error) {
      setError(`Failed to fetch pools: ${error.message}`);
    } finally {
      setLoading(false);
    }
  };

  // Handle tab change
  const handleTabChange = (tab) => {
    setActiveTab(tab);
    setError('');
    if (tab === 'pumpfun' && newTokens.length === 0) {
      fetchTokens();
    } else if (tab === 'clmm' && clmmV1Pools.length === 0 && clmmV2Pools.length === 0) {
      fetchPools();
    }
  };

  // Initial setup
  useEffect(() => {
    if (!hasInitializedRef.current) {
      hasInitializedRef.current = true;
      fetchTokens(); // Start with PumpFun data
    }
  }, [fetchTokens]);

  const formatLastUpdate = () => {
    return lastUpdate ? lastUpdate.toLocaleTimeString() : 'Never';
  };

  const handleManualRefresh = () => {
    if (activeTab === 'pumpfun') {
      fetchTokens();
    } else {
      fetchPools();
    }
  };

  // Each pool renders as one card (see ClmmPoolCard) — previously this split
  // every pool into two synthetic "token" cards (one per side) and
  // deduplicated by token address, which doesn't correspond to pool count at
  // all (every pool quoted in wSOL collapsed onto one shared wSOL card) and
  // produced a card grid with no sensible relationship to "V1: N | V2: M".
  const allPools = [
    ...clmmV1Pools.map(pool => ({ ...pool, poolVersion: pool.poolVersion || 1 })),
    ...clmmV2Pools.map(pool => ({ ...pool, poolVersion: pool.poolVersion || 2 })),
  ];
  const uniquePools = allPools.filter((pool, index, self) =>
    index === self.findIndex(p => p.poolState === pool.poolState)
  );

  // Render PumpFun content
  const renderPumpFunContent = () => {
    if (loading) {
      return (
        <div className="loading">
          <div className="loading-spinner"></div>
          <p>Loading pump tokens...</p>
        </div>
      );
    }

    if (error) {
      return (
        <div className="error">
          <p>{error}</p>
          <button onClick={fetchTokens} className="retry-button">Retry</button>
        </div>
      );
    }

    return (
      <div className="token-columns">
        <div className="token-column">
          <div className="column-header">
            <span className="column-icon">🆕</span>
            <span className="column-title">New Creations</span>
            <span className="column-count">{newTokens.length}</span>
          </div>
          <div className="column-content">
            {newTokens.map(token => (
              <TokenCard 
                key={token.id || token.tokenAddress} 
                token={token} 
                status="new" 
                onTokenSelect={onTokenSelect}
                isRealtime={!!token._realtimeTimestamp}
              />
            ))}
          </div>
        </div>

        <div className="token-column">
          <div className="column-header">
            <span className="column-icon">🔄</span>
            <span className="column-title">Completing</span>
            <span className="column-count">{completingTokens.length}</span>
          </div>
          <div className="column-content">
            {completingTokens.map(token => (
              <TokenCard 
                key={token.id || token.tokenAddress} 
                token={token} 
                status="completing" 
                onTokenSelect={onTokenSelect}
                isRealtime={!!token._realtimeTimestamp}
              />
            ))}
          </div>
        </div>

        <div className="token-column">
          <div className="column-header">
            <span className="column-icon">✅</span>
            <span className="column-title">Completed</span>
            <span className="column-count">{completedTokens.length}</span>
          </div>
          <div className="column-content">
            {completedTokens.map(token => (
              <TokenCard 
                key={token.id || token.tokenAddress} 
                token={token} 
                status="completed" 
                onTokenSelect={onTokenSelect}
                isRealtime={!!token._realtimeTimestamp}
              />
            ))}
          </div>
        </div>
      </div>
    );
  };

  // Render CLMM content
  const renderClmmContent = () => {
    if (loading) {
      return (
        <div className="loading-state">
          <div className="loading-spinner">🔄</div>
          <p>Loading CLMM tokens...</p>
        </div>
      );
    }

    if (error) {
      return (
        <div className="error">
          <p>{error}</p>
          <button onClick={fetchPools} className="retry-button">Retry</button>
        </div>
      );
    }

    if (uniquePools.length === 0) {
      return (
        <div className="empty-state">
          <div className="empty-icon">🏊‍♂️</div>
          <h3>No CLMM Pools Found</h3>
          <p>No concentrated liquidity pools are currently available on devnet.</p>
          <button className="retry-btn" onClick={fetchPools}>
            🔄 Try Again
          </button>
        </div>
      );
    }

    return (
      <div className="token-section">
        <h3 className="section-title">
          💧 CLMM Pools ({uniquePools.length})
        </h3>
        <div className="token-grid">
          {uniquePools.map((pool) => (
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
        </div>
      </div>
    );
  };

  return (
    <div className="token-list-container">
      {/* Tab Navigation */}
      <div className="tab-navigation">
        <div className="tab-buttons">
          <button 
            className={`tab-button ${activeTab === 'pumpfun' ? 'active' : ''}`}
            onClick={() => handleTabChange('pumpfun')}
          >
            🚀 PumpFun Tokens
          </button>
          <button 
            className={`tab-button ${activeTab === 'clmm' ? 'active' : ''}`}
            onClick={() => handleTabChange('clmm')}
          >
            🏊‍♂️ CLMM Tokens
          </button>
        </div>
      </div>

      {/* Status Bar */}
      <div className="update-indicator">
        <div style={{ display: 'flex', alignItems: 'center', gap: '15px', flexWrap: 'wrap' }}>
          <span>
            {activeTab === 'pumpfun' ? '🚀 PumpFun' : '🏊‍♂️ CLMM Tokens'} | Last updated: {formatLastUpdate()}
          </span>
          
          {activeTab === 'pumpfun' && (
            <div className="realtime-status" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span className={`connection-indicator ${connectionStatus}`}>
                {connectionStatus === 'connected' ? '🟢' : connectionStatus === 'disconnected' ? '🔴' : '🟡'} 
                {connectionStatus === 'connected' ? 'Live' : connectionStatus === 'disconnected' ? 'Offline' : 'Connecting...'}
              </span>
              {realtimeCount > 0 && (
                <span className="realtime-counter" style={{ 
                  background: '#00d4aa', 
                  color: '#000', 
                  padding: '2px 8px', 
                  borderRadius: '12px', 
                  fontSize: '12px',
                  fontWeight: 'bold'
                }}>
                  +{realtimeCount} live
                </span>
              )}
            </div>
          )}

          {activeTab === 'clmm' && (
            <span className="pool-counts">
              📊 V1: {clmmV1Pools.length} | V2: {clmmV2Pools.length}
            </span>
          )}
        </div>

        <div className="control-buttons">
          <button onClick={handleManualRefresh} className="retry-button">
            🔄 Manual Refresh
          </button>
          
        </div>

        {/* New Token Notifications */}
        {newTokenNotifications.length > 0 && activeTab === 'pumpfun' && (
          <div className="new-token-notifications" style={{ 
            marginTop: '10px',
            display: 'flex',
            flexWrap: 'wrap',
            gap: '8px'
          }}>
            {newTokenNotifications.map(notification => (
              <div 
                key={notification.id}
                className="notification-badge"
                style={{
                  background: 'linear-gradient(45deg, #00d4aa, #00ff88)',
                  color: '#000',
                  padding: '4px 12px',
                  borderRadius: '16px',
                  fontSize: '12px',
                  fontWeight: 'bold',
                  animation: 'slideInRight 0.3s ease-out',
                  boxShadow: '0 2px 8px rgba(0, 212, 170, 0.3)'
                }}
              >
                🆕 {notification.tokenName}
              </div>
            ))}
          </div>
        )}
      </div>
      
      {/* Content Area */}
      {activeTab === 'pumpfun' ? renderPumpFunContent() : renderClmmContent()}
      
    </div>
  );
};

export default TokenList; 