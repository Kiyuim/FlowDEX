import React, { useState, useEffect } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import TokenCard from './TokenCard';
import './TokenList.css';

const API_BASE_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

const MyTokens = ({ onTokenSelect }) => {
  const { publicKey, connected } = useWallet();
  const { connection } = useConnection();
  
  // State management
  const [myTokens, setMyTokens] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  
  // Fetch user's created tokens when wallet connects
  useEffect(() => {
    if (connected && publicKey) {
      fetchUserTokens();
    } else {
      setMyTokens([]);
    }
  }, [publicKey, connected]);
  
  const fetchUserTokens = async () => {
    if (!publicKey) return;
    
    setLoading(true);
    try {
      // Use API_BASE_URL to work with proxy in both dev and production
      const response = await fetch(`${API_BASE_URL}/v1/market/user_tokens?wallet_address=${publicKey.toString()}`);
      
      if (!response.ok) {
        throw new Error(`Failed to fetch tokens: ${response.status}`);
      }
      
      const data = await response.json();
      if (data?.code === 0 && data?.data?.list) {
        // Transform the response to match the expected format
        const transformedTokens = data.data.list.map(token => ({
          id: token.id,
          tokenAddress: token.tokenAddress,
          tokenName: token.tokenName || token.tokenSymbol,
          tokenSymbol: token.tokenSymbol,
          tokenIcon: token.tokenIcon || '',
          decimals: token.tokenDecimals,
          totalSupply: token.tokenSupply,
          description: token.description,
          txHash: token.txHash,
          createdAt: token.createdAt
        }));
        setMyTokens(transformedTokens);
      } else {
        setMyTokens([]);
      }
    } catch (err) {
      console.error('Error fetching user tokens:', err);
      setError(`Failed to fetch your tokens: ${err.message}`);
    } finally {
      setLoading(false);
    }
  };
  
  // Manual refresh
  const handleRefresh = () => {
    fetchUserTokens();
  };
  
  if (!connected) {
    return (
      <div className="token-list-container">
        <div className="token-list-header">
          <h2>My Created Tokens</h2>
        </div>
        <div className="wallet-not-connected">
          <h3>🔗 Connect Wallet</h3>
          <p>Please connect your wallet to view your tokens</p>
        </div>
      </div>
    );
  }
  
  return (
    <div className="token-list-container">
      <div className="token-list-header">
        <h2>My Created Tokens</h2>
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
          <p>Loading your tokens...</p>
        </div>
      ) : myTokens.length > 0 ? (
        <div className="token-grid">
          {myTokens.map((token) => (
            <TokenCard
              key={token.tokenAddress}
              token={token}
              status="completed"
              onTokenSelect={onTokenSelect}
            />
          ))}
        </div>
      ) : (
        <div className="no-tokens-message">
          <h3>No tokens found</h3>
          <p>You haven't created any tokens yet. Go to the "Create Token" tab to get started.</p>
        </div>
      )}
    </div>
  );
};

export default MyTokens; 