import React from 'react';
import './TokenCard.css';
import './ClmmPoolCard.css';

// Renders one CLMM pool as a single card (pair, fee tier, real 24h stats).
// Previously each pool was split into two synthetic "token" cards (one per
// side) and deduplicated by address — since most pools share wSOL as the
// input side, that produced a card count with no relation to the actual
// pool count and no way to see a pool's own price/fee tier/volume as one
// thing. This renders pools as pools instead.
const ClmmPoolCard = ({ pool, solPriceUsd, onClick }) => {
  const formatPrice = (price) => {
    if (!price || isNaN(price)) return '0.00';
    if (price < 0.001) return price.toExponential(2);
    return price.toFixed(6);
  };

  const formatNumber = (num) => {
    if (!num || isNaN(num)) return '0';
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toFixed(0);
  };

  const formatAge = (launchTime) => {
    if (!launchTime) return '0h';
    const diff = Math.floor(Date.now() / 1000) - Number(launchTime);
    if (diff < 0) return '0h';
    return `${Math.floor(diff / 3600)}h`;
  };

  const inputSymbol = pool.inputTokenSymbol && pool.inputTokenSymbol !== 'Unknown' ? pool.inputTokenSymbol : '?';
  const outputSymbol = pool.outputTokenSymbol && pool.outputTokenSymbol !== 'Unknown' ? pool.outputTokenSymbol : '?';
  const feePct = pool.tradeFeeRate ? (pool.tradeFeeRate / 10000).toFixed(2) : '0.00';

  return (
    <div className="token-card clmm-pool-card" onClick={onClick}>
      <div className="token-header">
        <div className="clmm-pair-avatars">
          <div className="token-avatar clmm-avatar-front">
            {pool.outputTokenIcon ? (
              <img src={pool.outputTokenIcon} alt={outputSymbol} />
            ) : (
              <div className="avatar-placeholder">{outputSymbol.slice(0, 2)}</div>
            )}
          </div>
          <div className="token-avatar clmm-avatar-back">
            {pool.inputTokenIcon ? (
              <img src={pool.inputTokenIcon} alt={inputSymbol} />
            ) : (
              <div className="avatar-placeholder">{inputSymbol.slice(0, 2)}</div>
            )}
          </div>
        </div>
        <div className="token-info">
          <div className="token-name-row">
            <span className="token-name">{outputSymbol}/{inputSymbol}</span>
            <span className="badge clmm-version-badge">CLMM {pool.poolVersion === 2 ? 'V2' : 'V1'}</span>
            <span className="badge clmm-fee-badge">{feePct}%</span>
          </div>
          <div className="token-stats-row">
            <span className="stat">⏱ {formatAge(pool.launchTime)}</span>
            {solPriceUsd > 0 && (
              <span className="stat">◎ SOL ≈ ${solPriceUsd.toFixed(2)}</span>
            )}
          </div>
        </div>
      </div>

      <div className="token-metrics">
        <div className="metric">
          <span className="metric-label">PRICE</span>
          <span className="metric-value price">${formatPrice(pool.price)}</span>
        </div>
        <div className="metric">
          <span className="metric-label">VOL 24H</span>
          <span className="metric-value">${formatNumber(pool.vol24h)}</span>
        </div>
        <div className="metric">
          <span className="metric-label">TXNS 24H</span>
          <span className="metric-value">{formatNumber(pool.txs24h)}</span>
        </div>
        <div className="metric">
          <span className="metric-label">TVL</span>
          <span className="metric-value">
            {pool.liquidityUsd > 0 ? `$${formatNumber(pool.liquidityUsd)}` : 'N/A'}
          </span>
        </div>
      </div>
    </div>
  );
};

export default ClmmPoolCard;
