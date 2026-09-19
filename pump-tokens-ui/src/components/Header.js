import React from 'react';
import { WalletMultiButton } from '@solana/wallet-adapter-react-ui';

const Header = () => {
  return (
    <header className="header">
      <div className="logo">
        🌊 FlowDEX
      </div>
      
      <nav className="nav-tabs">
        <button className="nav-tab active">Trenches</button>
        <button className="nav-tab">New pair</button>
        <button className="nav-tab">Trending</button>
        <button className="nav-tab">CopyTrade</button>
        <button className="nav-tab">Monitor</button>
        <button className="nav-tab">Track</button>
        <button className="nav-tab">Holding</button>
      </nav>

      <div className="user-controls">
        <div className="search-bar">
          <input 
            type="text" 
            className="search-input" 
            placeholder="Search token/contract/wallet"
          />
        </div>
        <WalletMultiButton />
        <button className="btn btn-secondary">SOL</button>
        <button className="btn btn-primary">Deposit</button>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <span>💰 0</span>
          <span>47.8A...ONLP</span>
        </div>
      </div>
    </header>
  );
};

export default Header; 