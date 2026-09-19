import React, { useState, useEffect } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import './TokenList.css';
import './MyPools.css';

const API_BASE_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

const MyPools = ({ onPoolSelect }) => {
  const { publicKey, connected } = useWallet();
  const { connection } = useConnection();
  
  // State management
  const [myPools, setMyPools] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  
  // Fetch user's created pools when wallet connects
  useEffect(() => {
    if (connected && publicKey) {
      fetchUserPools();
    } else {
      setMyPools([]);
    }
  }, [publicKey, connected]);
  
  const fetchUserPools = async () => {
    if (!publicKey) return;
    
    setLoading(true);
    try {
      // Use API_BASE_URL to work with proxy in both dev and production
      const response = await fetch(`${API_BASE_URL}/v1/market/user_pools?wallet_address=${publicKey.toString()}`);
      
      if (!response.ok) {
        throw new Error(`Failed to fetch pools: ${response.status}`);
      }
      
      const data = await response.json();
      if (data?.code === 10000 && data?.data?.list) {
        // Transform the response to match the expected format
        const transformedPools = data.data.list.map(pool => ({
          id: pool.id,
          poolState: pool.poolState,
          inputVaultMint: pool.inputVaultMint,
          outputVaultMint: pool.outputVaultMint,
          token0Symbol: pool.token0Symbol || 'Unknown',
          token1Symbol: pool.token1Symbol || 'Unknown',
          token0Decimals: pool.token0Decimals,
          token1Decimals: pool.token1Decimals,
          token0Liquidity: pool.token0Liquidity,
          token1Liquidity: pool.token1Liquidity,
          tradeFeeRate: pool.tradeFeeRate,
          initialPrice: pool.initialPrice,
          txHash: pool.txHash,
          poolVersion: pool.poolVersion,
          poolType: pool.poolType,
          ammConfig: pool.ammConfig,
          createdAt: pool.createdAt
        }));
        setMyPools(transformedPools);
      } else {
        setMyPools([]);
      }
    } catch (err) {
      console.error('Error fetching user pools:', err);
      setError(`Failed to fetch your pools: ${err.message}`);
    } finally {
      setLoading(false);
    }
  };
  
  // Manual refresh
  const handleRefresh = () => {
    fetchUserPools();
  };

  // Format token amounts with decimals
  const formatTokenAmount = (amount, decimals) => {
    if (!amount) return '0';
    
    const amountNum = parseFloat(amount);
    if (isNaN(amountNum)) return '0';
    
    return (amountNum / Math.pow(10, decimals || 9)).toFixed(4);
  };
  
  // Display pool fee as percentage
  const formatPoolFee = (feeRate) => {
    if (!feeRate && feeRate !== 0) return 'N/A';
    return `${(feeRate / 10000).toFixed(2)}%`;
  };

  // Format an address for display
  const formatAddress = (address) => {
    if (!address) return 'N/A';
    return `${address.slice(0, 4)}...${address.slice(-4)}`;
  };
  
  if (!connected) {
    return (
      <div className="token-list-container">
        <div className="token-list-header">
          <h2>My Liquidity Pools</h2>
        </div>
        <div className="wallet-not-connected">
          <h3>🔗 Connect Wallet</h3>
          <p>Please connect your wallet to view your liquidity pools</p>
        </div>
      </div>
    );
  }
  
  return (
    <div className="token-list-container">
      <div className="token-list-header">
        <h2>My Liquidity Pools</h2>
        <div className="token-list-controls">
          <button onClick={handleRefresh} disabled={loading}>
            {loading ? '⏳ Loading...' : '🔄 Refresh'}
          </button>
        </div>
      </div>
      
      {error && <div className="error-message">{error}</div>}
      
      {loading ? (
        <div className="loading-container">
          <div className="loader"></div>
          <p>Loading your pools...</p>
        </div>
      ) : myPools.length > 0 ? (
        <div className="pools-table-container">
          <table className="pools-table">
            <thead>
              <tr>
                <th>Pool ID</th>
                <th>Token A</th>
                <th>Token B</th>
                <th>Fee Rate</th>
                <th>Token A Liquidity</th>
                <th>Token B Liquidity</th>
                <th>Creation Time</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {myPools.map((pool) => (
                <tr key={pool.poolState || pool.id}>
                  <td>{formatAddress(pool.poolState)}</td>
                  <td>
                    <div className="token-cell">
                      {pool.token0Symbol || formatAddress(pool.inputVaultMint)}
                    </div>
                  </td>
                  <td>
                    <div className="token-cell">
                      {pool.token1Symbol || formatAddress(pool.outputVaultMint)}
                    </div>
                  </td>
                  <td>{formatPoolFee(pool.tradeFeeRate)}</td>
                  <td>{formatTokenAmount(pool.token0Liquidity, pool.token0Decimals)}</td>
                  <td>{formatTokenAmount(pool.token1Liquidity, pool.token1Decimals)}</td>
                  <td>{new Date(pool.createdAt).toLocaleString()}</td>
                  <td>
                    <div className="pool-actions">
                      <button 
                        className="action-btn view-btn"
                        onClick={() => onPoolSelect && onPoolSelect(pool)}
                      >
                        View
                      </button>
                      <button 
                        className="action-btn add-btn"
                        onClick={() => window.alert('Add liquidity functionality coming soon!')}
                      >
                        Add Liquidity
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="no-tokens-message">
          <h3>No pools found</h3>
          <p>You haven't created any liquidity pools yet. Go to the "Create Pool" tab to get started.</p>
        </div>
      )}
    </div>
  );
};

export default MyPools; 