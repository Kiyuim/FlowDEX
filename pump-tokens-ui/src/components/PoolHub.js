import React, { useState } from 'react';
import PoolCreation from './PoolCreation';
import AddLiquidity from './AddLiquidity';

// "Create Pool" and "Add Liquidity" were two separate top-level tabs even
// though they're really two steps of the same workflow (make a pool, then
// fund it) — merged into one "Pool" tab with an internal toggle instead.
const PoolHub = () => {
  const [subTab, setSubTab] = useState('create');

  return (
    <div>
      <div className="tab-navigation" style={{ marginBottom: 16, justifyContent: 'center' }}>
        <button
          className={`tab-btn ${subTab === 'create' ? 'active' : ''}`}
          onClick={() => setSubTab('create')}
        >
          🏊 Create Pool
        </button>
        <button
          className={`tab-btn ${subTab === 'liquidity' ? 'active' : ''}`}
          onClick={() => setSubTab('liquidity')}
        >
          💧 Add Liquidity
        </button>
      </div>
      {subTab === 'create' ? <PoolCreation /> : <AddLiquidity />}
    </div>
  );
};

export default PoolHub;
