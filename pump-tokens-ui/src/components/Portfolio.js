import React, { useState } from 'react';
import MyTokens from './MyTokens';
import MyPools from './MyPools';

const Portfolio = ({ onTokenSelect }) => {
  const [subTab, setSubTab] = useState('tokens');

  return (
    <div>
      <div className="tab-navigation" style={{ marginBottom: 16 }}>
        <button
          className={`tab-btn ${subTab === 'tokens' ? 'active' : ''}`}
          onClick={() => setSubTab('tokens')}
        >
          🪙 My Tokens
        </button>
        <button
          className={`tab-btn ${subTab === 'pools' ? 'active' : ''}`}
          onClick={() => setSubTab('pools')}
        >
          🏊 My Pools
        </button>
      </div>
      {subTab === 'tokens' ? (
        <MyTokens onTokenSelect={onTokenSelect} />
      ) : (
        <MyPools />
      )}
    </div>
  );
};

export default Portfolio;
