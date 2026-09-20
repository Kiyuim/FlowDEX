import React, { useState } from 'react';
import PoolCreation from '../components/PoolCreation';
import AddLiquidity from '../components/AddLiquidity';

export default function Pools() {
  const [tab, setTab] = useState('create');
  return (
    <div className="mx-auto max-w-4xl px-3 py-4 md:px-6">
      <div className="mb-4 inline-flex gap-1 rounded-lg bg-bg-soft p-1">
        {[
          ['create', 'Create Pool'],
          ['add', 'Add Liquidity'],
        ].map(([k, label]) => (
          <button
            key={k}
            onClick={() => setTab(k)}
            className={`rounded-md px-3 py-1.5 text-sm font-medium ${
              tab === k ? 'bg-bg-elev text-ink' : 'text-muted hover:text-ink'
            }`}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === 'create' ? <PoolCreation /> : <AddLiquidity />}
    </div>
  );
}
