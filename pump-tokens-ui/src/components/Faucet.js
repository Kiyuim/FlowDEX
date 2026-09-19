import React from 'react';
import './Faucet.css';

const faucets = [
  {
    name: 'Solana Official Faucet',
    url: 'https://faucet.solana.com/',
    desc: 'Official, 2x every 8h'
  },
  {
    name: 'solfaucet.com',
    url: 'https://solfaucet.com/',
    desc: 'Simple devnet faucet'
  },
  {
    name: 'QuickNode Faucet',
    url: 'https://faucet.quicknode.com/solana/devnet',
    desc: 'QuickNode devnet faucet'
  },
  {
    name: 'DevnetFaucet.org',
    url: 'https://www.devnetfaucet.org/',
    desc: 'Another devnet faucet'
  },
  {
    name: 'solfate.com Faucet',
    url: 'https://solfate.com/faucet',
    desc: 'Solfate devnet faucet'
  },
  {
    name: 'Ashwin Narayan Faucet',
    url: 'https://www.ashwinnarayan.com/dapps/solana-faucet/',
    desc: 'Ashwin Narayan devnet faucet'
  },
  {
    name: 'SPL Token Faucet',
    url: 'https://spl-token-faucet.com/',
    desc: 'SPL Token faucet'
  },
  {
    name: 'Diadata Faucet List',
    url: 'https://www.diadata.org/web3-builder-hub/faucets/solana-faucets/',
    desc: 'Diadata faucet aggregator'
  }
];

const Faucet = () => {
  return (
    <div className="faucet-container">
      <div className="faucet-card">
        <div className="card-header">
          <h2>🚰 Solana Faucets</h2>
          <p>Get free SOL for devnet testing. Click a faucet below to open in a new tab.</p>
        </div>
        <div className="faucet-list">
          {faucets.map(faucet => (
            <div className="faucet-item" key={faucet.url}>
              <div className="faucet-info">
                <div className="faucet-name">{faucet.name}</div>
                <div className="faucet-desc">{faucet.desc}</div>
              </div>
              <a
                className="faucet-link"
                href={faucet.url}
                target="_blank"
                rel="noopener noreferrer"
              >
                Visit Faucet
              </a>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default Faucet; 