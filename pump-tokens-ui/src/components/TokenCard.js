import React, { useState } from 'react';
import BuyModal from './BuyModal';
import './TokenCard.css';

const TokenCard = ({ token, status = 'new', onTokenSelect, isRealtime = false }) => {
  const [isBuyModalOpen, setIsBuyModalOpen] = useState(false);

  // Format number with K/M
  const formatNumber = (num) => {
    if (!num || isNaN(num)) return '0';
    if (num >= 1000000) {
      return (num / 1000000).toFixed(1) + 'M';
    } else if (num >= 1000) {
      return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
  };

  const formatPrice = (price) => {
    if (!price || isNaN(price)) return '0.00';
    if (price < 0.001) {
      return price.toExponential(2);
    }
    return price.toFixed(6);
  };

  // Format age from launchTime (seconds to hours ago)
  const formatAge = (launchTime) => {
    if (!launchTime) return '0h';
    const now = Math.floor(Date.now() / 1000);
    const diff = now - Number(launchTime);
    if (diff < 0) return '0h';
    const hours = Math.floor(diff / 3600);
    return `${hours}h`;
  };

  const getStatusIcon = () => {
    switch (status) {
      case 'completing':
        return '🔄';
      case 'completed':
        return '✅';
      default:
        return '🆕';
    }
  };

  const getStatusColor = () => {
    switch (status) {
      case 'completing':
        return '#ffa500';
      case 'completed':
        return '#00ff88';
      default:
        return '#666';
    }
  };

  const handleBuyClick = () => {
    setIsBuyModalOpen(true);
  };

  const handleCloseBuyModal = () => {
    setIsBuyModalOpen(false);
  };

  const handleChartClick = () => {
    console.log('🎯 TokenCard - Chart button clicked for token:', token.tokenName);
    if (onTokenSelect) {
      const tokenData = {
        ...token,
        tokenName: token.tokenName,
        tokenAddress: token.tokenAddress,
        pairAddress: token.pairAddress, // Make sure pairAddress is included
      };
      console.log('🎯 TokenCard - Calling onTokenSelect with:', tokenData);
      onTokenSelect(tokenData);
    } else {
      console.log('❌ TokenCard - onTokenSelect callback is not available');
    }
  };

  return (
    <>
      <div className={`token-card ${isRealtime ? 'realtime-token' : ''}`}>
        <div className="token-header">
          <div className="token-avatar">
            {token.tokenIcon ? (
              <img src={token.tokenIcon} alt={token.tokenName || '?'} />
            ) : (
              <div className="avatar-placeholder">
                {token.tokenName?.charAt(0) || '?'}
              </div>
            )}
          </div>
          <div className="token-info">
            <div className="token-name">
              {token.tokenName || '?'}
              <span className="token-symbol">{token.tokenAddress?.slice(0, 4) || ''}</span>
            </div>
            <div className="token-address">
              {token.tokenAddress ? `${token.tokenAddress.slice(0, 6)}...${token.tokenAddress.slice(-4)}` : '?'}
            </div>
            <div className="token-stats">
              <span className="stat">💎 {formatNumber(token.mktCap)}</span>
              <span className="stat">👥 {formatNumber(token.holdCount)}</span>
              <span className="stat">📈 {token.change24 ? `${token.change24 > 0 ? '+' : ''}${token.change24.toFixed(1)}%` : '0%'}</span>
            </div>
          </div>
        </div>

        <div className="token-metrics">
          <div className="metric">
            <span className="metric-label">PRICE</span>
            <span className="metric-value price">${formatPrice(token.price)}</span>
          </div>
          <div className="metric">
            <span className="metric-label">MC</span>
            <span className="metric-value">${formatNumber(token.mktCap)}</span>
          </div>
          <div className="metric">
            <span className="metric-label">AGE</span>
            <span className="metric-value">{formatAge(token.launchTime)}</span>
          </div>
          <div className="metric">
            <span className="metric-label">TXNS</span>
            <span className="metric-value">{formatNumber(token.txs24h)}</span>
          </div>
        </div>

        <div className="token-actions">
          <div className="status-indicator" style={{ color: getStatusColor() }}>
            {getStatusIcon()} {status === 'new' ? 'New Creation' : status === 'completing' ? 'Completing' : 'Completed'}
          </div>
          <div className="action-buttons">
            <button className="chart-button" onClick={handleChartClick} title="View Price Chart">
              <span className="chart-icon">📈</span>
              Chart
            </button>
            <button className="buy-button" onClick={handleBuyClick}>
              <span className="buy-icon">💰</span>
              Buy
            </button>
          </div>
        </div>

        <div className="token-progress">
          <div className="progress-bar">
            <div 
              className="progress-fill" 
              style={{ width: `${Math.min((token.mktCap || 0) / 1000000 * 100, 100)}%` }}
            ></div>
          </div>
          <div className="progress-text">
            Progress: {Math.min((token.mktCap || 0) / 1000000 * 100, 100).toFixed(1)}%
          </div>
        </div>
      </div>

      <BuyModal 
        isOpen={isBuyModalOpen}
        onClose={handleCloseBuyModal}
        token={token}
      />
    </>
  );
};

export default TokenCard; 