import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { createPortal } from 'react-dom';
import Header from './components/Header';
import TokenList from './components/TokenList';
import TradingViewChart from './components/TradingViewChart';
import WalletDebugger from './components/WalletDebugger';
import TokenCreation from './components/TokenCreation';
import PoolCreation from './components/PoolCreation';
import AddLiquidity from './components/AddLiquidity';
import Faucet from './components/Faucet'; // Added Faucet import
import TokenSecurity from './components/TokenSecurity'; // Added TokenSecurity import
import './App.css';
import { BrowserRouter, Routes, Route, Link } from 'react-router-dom';
import { Toaster } from 'react-hot-toast';
import Discovery from './pages/Discovery';
import TokenDetail from './pages/TokenDetail';
import Pools from './pages/Pools';
import Portfolio from './pages/Portfolio';
import TokenSource from './pages/TokenSource';
import { ConnectionProvider, WalletProvider, useWallet } from '@solana/wallet-adapter-react';
import { WalletModalProvider } from '@solana/wallet-adapter-react-ui';
import { 
  PhantomWalletAdapter,
  SolflareWalletAdapter,
  CoinbaseWalletAdapter,
  LedgerWalletAdapter,
  TorusWalletAdapter,
  Coin98WalletAdapter,
  TrustWalletAdapter,
  TokenPocketWalletAdapter,
  MathWalletAdapter,
  SafePalWalletAdapter,
  SolongWalletAdapter,
  CloverWalletAdapter,
  BitKeepWalletAdapter,
  NightlyWalletAdapter,
  NufiWalletAdapter,
  SkyWalletAdapter,
  SpotWalletAdapter,
} from '@solana/wallet-adapter-wallets';
import { clusterApiUrl } from '@solana/web3.js';
import '@solana/wallet-adapter-react-ui/styles.css';
import { setSolUsd } from './lib/solPrice';

// Initialize wallets with error handling
// Using both standard adapters (auto-detected) and legacy adapters for maximum compatibility
// Note: This is moved inside App component to use useMemo for proper React optimization

// Use Solana RPC endpoint with fallback options
// Try multiple endpoints for better reliability
const getRpcEndpoint = () => {
  // Try environment variable first
  if (process.env.REACT_APP_SOLANA_RPC_URL) {
    return process.env.REACT_APP_SOLANA_RPC_URL;
  }
  
  // Optional Helius key (only used if explicitly provided and valid).
  const heliusApiKey = process.env.REACT_APP_HELIUS_API_KEY;
  if (heliusApiKey) {
    return `https://devnet.helius-rpc.com/?api-key=${heliusApiKey}`;
  }

  // Default to the public Solana devnet RPC. It needs no API key and is what the
  // wallet uses to fetch a blockhash and submit the buy. (The previously
  // hard-coded Helius key returned 401 Invalid API key, which surfaced in the
  // wallet as a generic "Internal error" on send.)
  return 'https://api.devnet.solana.com';
};

// Second provider (e.g. Alchemy) to fall over to when the primary RPC returns
// 429 "max usage reached" — Helius devnet free-tier credits are shared across
// every key on the account and exhaust fast under real traffic. Optional: if
// unset, requests just fail on 429 same as before.
const FALLBACK_RPC_URL = process.env.REACT_APP_SOLANA_RPC_FALLBACK_URL || '';

// Wraps fetch so every RPC call made through the Connection (getBalance,
// getAccountInfo, sendTransaction, ...) transparently retries against
// FALLBACK_RPC_URL on a 429, instead of failing the whole page/action.
function createFailoverFetch(fallbackUrl) {
  return async (input, init) => {
    const res = await fetch(input, init);
    if (res.status === 429 && fallbackUrl) {
      console.warn('⚠️ Primary RPC rate-limited (429) — retrying via fallback endpoint');
      return fetch(fallbackUrl, init);
    }
    return res;
  };
}

const connectionConfig = FALLBACK_RPC_URL
  ? { commitment: 'confirmed', fetch: createFailoverFetch(FALLBACK_RPC_URL) }
  : { commitment: 'confirmed' };

const endpoint = getRpcEndpoint();
console.log('🔗 Using Solana RPC endpoint:', endpoint.replace(/api-key=[^&]+/, 'api-key=***'));

// Component to debug wallet registration
const WalletListDebugger = () => {
  const { wallets } = useWallet();
  
  useEffect(() => {
    console.log('🔍 Registered wallets in WalletProvider:', wallets.length);
    wallets.forEach((wallet, index) => {
      console.log(`  ${index + 1}. ${wallet.adapter.name} - ReadyState: ${wallet.adapter.readyState}, Installed: ${wallet.readyState === 'Installed'}`);
    });
  }, [wallets]);
  
  return null;
};

// Custom wallet button that ensures all wallets are shown
const CustomWalletButton = () => {
  const { wallets, select, connect, connecting, connected, disconnect, publicKey, wallet } = useWallet();
  const [showModal, setShowModal] = useState(false);
  const [drag, setDrag] = useState({ x: 0, y: 0 });

  // Drag the modal by its header (pointer events → works for mouse + touch).
  const startDrag = useCallback(
    (e) => {
      if (e.target.closest('button')) return; // don't drag when clicking the close button
      e.preventDefault();
      const startX = e.clientX;
      const startY = e.clientY;
      const orig = { ...drag };
      const move = (ev) =>
        setDrag({ x: orig.x + (ev.clientX - startX), y: orig.y + (ev.clientY - startY) });
      const up = () => {
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', up);
      };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', up);
    },
    [drag]
  );

  const handleConnect = () => {
    setDrag({ x: 0, y: 0 });
    setShowModal(true);
  };

  // Wallet-standard bridges can register wallets that aren't actually usable:
  // Solflare's extension advertises a "MetaMask" wallet (its MetaMask-Snap
  // bridge) with readyState "Installed" even when the MetaMask extension
  // itself is absent — connecting then throws "MetaMask extension not found".
  // Only list MetaMask when the real extension is present.
  const visibleWallets = wallets.filter((w) => {
    if (w.adapter.name === 'MetaMask') {
      return typeof window !== 'undefined' && window.ethereum?.isMetaMask === true;
    }
    return true;
  });

  const handleSelectWallet = async (walletName, event) => {
    // Prevent event propagation
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    
    try {
      console.log('🔌 Selecting wallet:', walletName);
      
      // Find the wallet adapter
      const selectedWallet = wallets.find(w => w.adapter.name === walletName);
      if (!selectedWallet) {
        console.error('❌ Wallet not found:', walletName);
        alert(`Wallet ${walletName} not found. Please refresh the page.`);
        return;
      }
      
      console.log('📋 Wallet adapter found:', {
        name: selectedWallet.adapter.name,
        readyState: selectedWallet.readyState,
        adapter: selectedWallet.adapter
      });
      
      // Check if wallet adapter is actually available
      if (selectedWallet.readyState === 'NotDetected') {
        console.warn('⚠️ Wallet not detected, opening install page');
        window.open(getInstallUrl(walletName), '_blank');
        return;
      }
      
      // Two constraints pull in opposite directions here:
      //  - the hook's connect() reads the selected wallet from React state,
      //    which select() only schedules — same-tick connect() throws
      //    WalletNotSelectedError;
      //  - deferring connect() to an effect breaks the click's user-gesture
      //    chain, and Solflare then auto-rejects ("Connection rejected")
      //    because its popup gets blocked outside a gesture.
      // Calling the adapter's connect() directly satisfies both: it doesn't
      // depend on React state, and it runs inside the click. select() still
      // updates the provider, which syncs connected/publicKey from the
      // adapter's connect event.
      try {
        select(walletName);
        console.log('✅ Wallet selected:', walletName);
      } catch (selectError) {
        console.error('❌ Error selecting wallet:', selectError);
        alert(`Failed to select wallet: ${selectError.message}`);
        return;
      }

      setShowModal(false);

      try {
        console.log('🔗 Connecting to wallet (adapter):', walletName);
        if (!selectedWallet.adapter.connected) {
          await selectedWallet.adapter.connect();
        }
        console.log('✅ Wallet connected:', selectedWallet.adapter.publicKey?.toString());
      } catch (connectError) {
        console.error('❌ Error connecting wallet:', connectError);
        const errorMessage = connectError?.message?.toLowerCase() || '';
        const errorName = connectError?.name || '';
        if (
          errorMessage.includes('user rejected') ||
          errorMessage.includes('user cancelled') ||
          errorMessage.includes('user denied') ||
          errorMessage.includes('connection rejected') ||
          errorName === 'WalletNotConnectedError'
        ) {
          console.log('ℹ️ User rejected or cancelled the connection');
          alert('Connection was cancelled. Please try again if you want to connect.');
        } else {
          alert(`Failed to connect wallet: ${connectError?.message || 'Unknown error'}. Please try again.`);
          setShowModal(true);
        }
      }
    } catch (error) {
      console.error('❌ Error selecting wallet:', error);
      console.error('❌ Error details:', {
        message: error?.message,
        name: error?.name,
        stack: error?.stack
      });
      alert(`Error: ${error.message || 'Unknown error'}`);
      setShowModal(true); // Reopen modal on error
    }
  };

  // Debug: Log state changes
  useEffect(() => {
    console.log('🔍 Wallet state changed:', {
      connected,
      connecting,
      publicKey: publicKey?.toString(),
      wallet: wallet?.adapter?.name,
      showModal
    });
    
    // Log when connection succeeds
    if (connected && publicKey) {
      console.log('✅✅✅ Wallet is CONNECTED! PublicKey:', publicKey.toString());
    }
  }, [connected, connecting, publicKey, wallet, showModal]);

  // Auto-connect when wallet is selected but not connected
  // NOTE: Disabled auto-connect to avoid issues with wallet authorization prompts
  // Auto-connect can cause problems because it's not triggered by a user click event
  // useEffect(() => {
  //   if (wallet && !connected && !connecting && !showModal) {
  //     console.log('🔄 Auto-connecting to selected wallet:', wallet.adapter.name);
  //     connect().catch(err => {
  //       console.error('❌ Auto-connect failed:', err);
  //     });
  //   }
  // }, [wallet, connected, connecting, showModal, connect]);


  const getInstallUrl = (walletName) => {
    const urls = {
      'Phantom': 'https://phantom.app/',
      'Solflare': 'https://solflare.com/',
      'Coinbase': 'https://www.coinbase.com/wallet',
      'Ledger': 'https://www.ledger.com/',
      'Torus': 'https://tor.us/',
      'Coin98': 'https://coin98.com/',
      'Trust Wallet': 'https://trustwallet.com/',
      'TokenPocket': 'https://tokenpocket.pro/',
      'MathWallet': 'https://mathwallet.org/',
      'SafePal': 'https://www.safepal.com/',
      'Solong': 'https://solongwallet.com/',
      'Clover': 'https://clover.finance/',
      'BitKeep': 'https://bitkeep.com/',
      'Nightly': 'https://nightly.app/',
      'Nufi': 'https://nufi.com/',
      'Sky': 'https://skypool.com/',
      'Spot': 'https://spot-wallet.com/',
    };
    return urls[walletName] || '#';
  };

  // Show connecting state
  if (connecting) {
    return (
      <button
        disabled
        style={{
          padding: '8px 16px',
          backgroundColor: '#999',
          color: 'white',
          border: 'none',
          borderRadius: '4px',
          cursor: 'not-allowed'
        }}
      >
        Connecting...
      </button>
    );
  }

  // Show connected state
  if (connected && publicKey) {
    console.log('✅ Rendering connected state:', {
      connected,
      publicKey: publicKey.toString(),
      wallet: wallet?.adapter?.name
    });
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <span style={{ fontSize: '14px', fontWeight: 'bold', color: '#512DA8' }}>
          {wallet?.adapter?.name || 'Wallet'}: {publicKey.toString().slice(0, 4)}...{publicKey.toString().slice(-4)}
        </span>
        <button 
          onClick={disconnect}
          style={{
            padding: '8px 16px',
            backgroundColor: '#ff4444',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: 'pointer'
          }}
        >
          Disconnect
        </button>
      </div>
    );
  }

  return (
    <>
      <button
        onClick={handleConnect}
        disabled={connecting}
        style={{
          padding: '8px 16px',
          backgroundColor: '#512DA8',
          color: 'white',
          border: 'none',
          borderRadius: '4px',
          cursor: connecting ? 'not-allowed' : 'pointer',
          opacity: connecting ? 0.6 : 1
        }}
      >
        {connecting ? 'Connecting...' : 'Connect Wallet'}
      </button>

      {showModal && createPortal(
        <div
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.7)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 10000
          }}
          onClick={() => setShowModal(false)}
        >
          <div
            style={{
              backgroundColor: 'white',
              borderRadius: '12px',
              padding: '24px',
              maxWidth: '500px',
              width: '90%',
              maxHeight: '85vh',
              overflowY: 'auto',
              transform: `translate(${drag.x}px, ${drag.y}px)`,
              boxShadow: '0 24px 60px -20px rgba(0,0,0,0.7)'
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div
              onPointerDown={startDrag}
              style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', cursor: 'move', touchAction: 'none', userSelect: 'none' }}
            >
              <h2 style={{ margin: 0 }}>Select Wallet</h2>
              <button
                onClick={() => setShowModal(false)}
                style={{
                  background: 'none',
                  border: 'none',
                  fontSize: '24px',
                  cursor: 'pointer',
                  color: '#666'
                }}
              >
                ×
              </button>
            </div>
            
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: '12px' }}>
              {visibleWallets.map((wallet) => {
                const isInstalled = wallet.readyState === 'Installed';
                const isLoadable = wallet.readyState === 'Loadable';
                const canConnect = isInstalled || isLoadable;

                return (
                  <div
                    key={wallet.adapter.name}
                    onClick={async (e) => {
                      e.stopPropagation();
                      e.preventDefault();
                      if (canConnect) {
                        console.log('🖱️ Clicked wallet:', wallet.adapter.name, 'ReadyState:', wallet.readyState);
                        await handleSelectWallet(wallet.adapter.name, e);
                      } else {
                        window.open(getInstallUrl(wallet.adapter.name), '_blank');
                      }
                    }}
                    style={{
                      padding: '16px',
                      border: '2px solid',
                      borderColor: canConnect ? '#512DA8' : '#ddd',
                      borderRadius: '8px',
                      cursor: 'pointer',
                      textAlign: 'center',
                      backgroundColor: canConnect ? '#f5f5f5' : '#fff',
                      transition: 'all 0.2s',
                      opacity: canConnect ? 1 : 0.7
                    }}
                    onMouseEnter={(e) => {
                      if (canConnect) {
                        e.currentTarget.style.backgroundColor = '#e8e8e8';
                      }
                    }}
                    onMouseLeave={(e) => {
                      if (canConnect) {
                        e.currentTarget.style.backgroundColor = '#f5f5f5';
                      }
                    }}
                  >
                    {(() => {
                      const icon = wallet.adapter.icon;
                      const iconUrl = typeof icon === 'string' ? icon : (icon?.src || icon);
                      
                      return iconUrl ? (
                        <img
                          src={iconUrl}
                          alt={wallet.adapter.name}
                          style={{ width: '48px', height: '48px', marginBottom: '8px', objectFit: 'contain' }}
                          onError={(e) => {
                            e.target.style.display = 'none';
                            e.target.nextSibling.style.display = 'flex';
                          }}
                        />
                      ) : null;
                    })()}
                    <div 
                      style={{ 
                        width: '48px', 
                        height: '48px', 
                        marginBottom: '8px', 
                        backgroundColor: '#f0f0f0', 
                        borderRadius: '8px', 
                        display: wallet.adapter.icon ? 'none' : 'flex', 
                        alignItems: 'center', 
                        justifyContent: 'center', 
                        fontSize: '24px',
                        margin: '0 auto 8px auto'
                      }}
                    >
                      💼
                    </div>
                    <div style={{ fontSize: '14px', fontWeight: 'bold', marginBottom: '4px' }}>
                      {wallet.adapter.name}
                    </div>
                    {canConnect ? (
                      <div style={{ fontSize: '12px', color: '#512DA8' }}>Click to connect</div>
                    ) : (
                      <div style={{ fontSize: '12px', color: '#666' }}>Click to install</div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        </div>,
        document.body
      )}
    </>
  );
};

function App() {
  const [selectedToken, setSelectedToken] = useState(null);
  const [activeTab, setActiveTab] = useState('tokens');
  const [showDebugger, setShowDebugger] = useState(false);

  // Initialize wallets with useMemo for proper React optimization
  // This ensures wallets are only created once and all wallets are available
  const wallets = useMemo(() => {
    const walletAdapters = [
      // Legacy adapters for wallets that may not fully support Wallet Standard yet
      new PhantomWalletAdapter(),
      new SolflareWalletAdapter(),
      new CoinbaseWalletAdapter(),
      new LedgerWalletAdapter(),
      new TorusWalletAdapter(),
      new Coin98WalletAdapter(),
      new TrustWalletAdapter(),
      new TokenPocketWalletAdapter(),
      new MathWalletAdapter(),
      new SafePalWalletAdapter(),
      new SolongWalletAdapter(),
      new CloverWalletAdapter(),
      new BitKeepWalletAdapter(),
      new NightlyWalletAdapter(),
      new NufiWalletAdapter(),
      new SkyWalletAdapter(),
      new SpotWalletAdapter(),
    ];

    const initializedWallets = walletAdapters.filter(wallet => {
      try {
        // Filter out wallets that fail to initialize
        if (wallet === null || wallet === undefined) {
          return false;
        }
        // Log wallet initialization for debugging
        console.log(`✅ Wallet adapter initialized: ${wallet.name || 'Unknown'}`);
        return true;
      } catch (error) {
        console.warn('Failed to initialize wallet adapter:', error);
        return false;
      }
    });

    // Log all initialized wallets
    console.log(`📋 Total wallets initialized: ${initializedWallets.length}`);
    initializedWallets.forEach((wallet, index) => {
      console.log(`  ${index + 1}. ${wallet.name || 'Unknown'} (${wallet.constructor.name})`);
    });

    return initializedWallets;
  }, []); // Empty dependency array means this only runs once

  console.log(`[${new Date().toLocaleTimeString()}] 🔄 App render - activeTab:`, activeTab, 'selectedToken:', selectedToken?.tokenName || 'none');

  const handleTokenSelect = (token) => {
    console.log(`[${new Date().toLocaleTimeString()}] 🎯 Token selected:`, token?.tokenName, 'switching to chart tab');
    setSelectedToken(token);
    setActiveTab('chart');
  };

  const handleTabSwitch = (tab) => {
    console.log(`[${new Date().toLocaleTimeString()}] 🔀 Tab switching from`, activeTab, 'to', tab);
    setActiveTab(tab);
  };

  // Keep client-side price math on the same SOL/USD the backend uses
  // (index_pump carries its live CoinGecko value); refresh every minute.
  useEffect(() => {
    let alive = true;
    const pull = async () => {
      try {
        const r = await fetch('/v1/market/index_pump?chain_id=100000&pump_status=1&page_no=1&page_size=1');
        const d = await r.json();
        if (alive) setSolUsd(d?.data?.solPriceUsd);
      } catch {}
    };
    pull();
    const t = setInterval(pull, 60000);
    return () => { alive = false; clearInterval(t); };
  }, []);

  // Track tab changes
  useEffect(() => {
    console.log(`[${new Date().toLocaleTimeString()}] 📋 Active tab changed to:`, activeTab);
  }, [activeTab]);

  // Track selected token changes
  useEffect(() => {
    console.log(`[${new Date().toLocaleTimeString()}] 🎯 Selected token changed to:`, selectedToken?.tokenName || 'none');
  }, [selectedToken]);

  // Keyboard shortcut to toggle debugger
  useEffect(() => {
    const handleKeyDown = (event) => {
      if (event.ctrlKey && event.shiftKey && event.key === 'D') {
        setShowDebugger(!showDebugger);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [showDebugger]);

  const NAV = [
    ['/', 'Tokens'],
    ['/source', 'Sources'],
    ['/portfolio', 'Portfolio'],
    ['/create', 'Create'],
    ['/pools', 'Pools'],
    ['/faucet', 'Faucet'],
    ['/security', 'Security'],
  ];

  return (
    <ConnectionProvider endpoint={endpoint} config={connectionConfig}>
      <WalletProvider wallets={wallets} autoConnect={true}>
        <WalletModalProvider>
          <BrowserRouter>
            <Toaster
              position="bottom-right"
              toastOptions={{
                style: { background: '#141722', color: '#e7ebf3', border: '1px solid #232838' },
              }}
            />
            <WalletListDebugger />
            <div className="min-h-screen bg-bg text-ink">
              <header className="sticky top-0 z-30 border-b border-border bg-bg-soft/80 backdrop-blur">
                <div className="mx-auto flex max-w-7xl items-center gap-3 px-3 py-2.5 md:px-6">
                  <Link to="/" className="flex items-center gap-2 font-bold text-ink">
                    <span className="text-accent text-lg">◆</span>
                    <span className="hidden sm:inline">FlowDEX</span>
                  </Link>
                  <span className="rounded-full border border-warn/40 bg-warn/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-warn">
                    Devnet
                  </span>
                  <nav className="ml-2 hidden items-center gap-1 md:flex">
                    {NAV.map(([to, label]) => (
                      <Link
                        key={to}
                        to={to}
                        className="rounded-md px-3 py-1.5 text-sm text-muted hover:bg-bg-hover hover:text-ink"
                      >
                        {label}
                      </Link>
                    ))}
                  </nav>
                  <div className="ml-auto">
                    <CustomWalletButton />
                  </div>
                </div>
                <nav className="flex gap-1 overflow-x-auto border-t border-border px-3 py-1.5 md:hidden">
                  {NAV.map(([to, label]) => (
                    <Link
                      key={to}
                      to={to}
                      className="whitespace-nowrap rounded-md px-3 py-1 text-sm text-muted hover:text-ink"
                    >
                      {label}
                    </Link>
                  ))}
                </nav>
              </header>

              <main>
                <Routes>
                  <Route path="/" element={<Discovery />} />
                  <Route path="/source" element={<TokenSource />} />
                  <Route path="/token/:mint" element={<TokenDetail />} />
                  <Route path="/portfolio" element={<Portfolio />} />
                  <Route
                    path="/create"
                    element={
                      <div className="mx-auto max-w-4xl px-3 py-4 md:px-6">
                        <TokenCreation />
                      </div>
                    }
                  />
                  <Route path="/pools" element={<Pools />} />
                  <Route
                    path="/faucet"
                    element={
                      <div className="mx-auto max-w-3xl px-3 py-4 md:px-6">
                        <Faucet />
                      </div>
                    }
                  />
                  <Route
                    path="/security"
                    element={
                      <div className="mx-auto max-w-3xl px-3 py-4 md:px-6">
                        <TokenSecurity />
                      </div>
                    }
                  />
                  <Route path="*" element={<Discovery />} />
                </Routes>
              </main>

              <WalletDebugger isVisible={showDebugger} />
            </div>
          </BrowserRouter>
        </WalletModalProvider>
      </WalletProvider>
    </ConnectionProvider>
  );
}

export default App;