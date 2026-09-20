import React from 'react';

const faucets = [
  { name: 'Solana Official Faucet', url: 'https://faucet.solana.com/', desc: 'Official · 2× every 8h' },
  { name: 'solfaucet.com', url: 'https://solfaucet.com/', desc: 'Simple devnet faucet' },
  { name: 'QuickNode Faucet', url: 'https://faucet.quicknode.com/solana/devnet', desc: 'QuickNode devnet faucet' },
  { name: 'DevnetFaucet.org', url: 'https://www.devnetfaucet.org/', desc: 'Another devnet faucet' },
  { name: 'solfate.com Faucet', url: 'https://solfate.com/faucet', desc: 'Solfate devnet faucet' },
  { name: 'Ashwin Narayan Faucet', url: 'https://www.ashwinnarayan.com/dapps/solana-faucet/', desc: 'Community faucet' },
  { name: 'SPL Token Faucet', url: 'https://spl-token-faucet.com/', desc: 'SPL token faucet' },
  { name: 'DIA Faucet List', url: 'https://www.diadata.org/web3-builder-hub/faucets/solana-faucets/', desc: 'Faucet aggregator' },
];

export default function Faucet() {
  return (
    <div>
      <div className="mb-4">
        <h1 className="text-lg font-bold text-ink">🚰 Devnet Faucets</h1>
        <p className="text-xs text-muted">Get free SOL for testing. Opens in a new tab.</p>
      </div>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {faucets.map((f) => (
          <a
            key={f.url}
            href={f.url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-between rounded-xl border border-border bg-bg-card p-3 transition hover:border-accent/50 hover:bg-bg-hover"
          >
            <div className="min-w-0">
              <div className="truncate font-medium text-ink">{f.name}</div>
              <div className="truncate text-xs text-muted">{f.desc}</div>
            </div>
            <span className="ml-3 shrink-0 rounded-md border border-accent/40 bg-accent/10 px-2.5 py-1 text-xs font-semibold text-accent">
              Visit ↗
            </span>
          </a>
        ))}
      </div>
    </div>
  );
}
