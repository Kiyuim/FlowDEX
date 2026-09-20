import React, { useState, useEffect, useLayoutEffect, useRef, useMemo } from 'react';
import TokenList from './components/TokenList';
import TradingViewChart from './components/TradingViewChart';
import WalletDebugger from './components/WalletDebugger';
import TokenCreation from './components/TokenCreation';
import PoolHub from './components/PoolHub';
import Sources from './components/Sources';
import Portfolio from './components/Portfolio';
import Faucet from './components/Faucet'; // Added Faucet import
import TokenSecurity from './components/TokenSecurity'; // Added TokenSecurity import
import './App.css';
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
  
  // Use Helius devnet (same as backend configuration)
  // This matches the RPC endpoint used in trade service config
  const heliusApiKey = process.env.REACT_APP_HELIUS_API_KEY || '2d7580ca-93d2-4404-9316-656c2726a26a';
  const heliusEndpoint = `https://devnet.helius-rpc.com/?api-key=${heliusApiKey}`;
  
  // Fallback endpoints (in order of preference)
  const fallbackEndpoints = [
    'https://api.devnet.solana.com',  // Official Solana devnet
    'https://solana-devnet-rpc.allthatnode.com',  // AllThatNode public RPC
    'https://rpc.ankr.com/solana_devnet',  // Ankr public RPC
  ];
  
  // Use Helius as primary endpoint (more reliable)
  return heliusEndpoint;
};

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
  // select() updates `wallet` asynchronously (next render), so calling
  // connect() synchronously right after select() sees the OLD wallet (still
  // null) and throws WalletNotSelectedError — that's why the first click
  // always failed and the second (where `wallet` was already set) worked.
  // useLayoutEffect fires as soon as `wallet` actually updates, still inside
  // the same paint cycle as the click, so it doesn't break wallet popup
  // "user gesture" heuristics the way a setTimeout/useEffect delay would.
  const pendingConnectRef = useRef(false);

  const handleConnect = () => {
    setShowModal(true);
  };

  const doConnect = async () => {
    try {
      console.log('🔗 Connecting to wallet:', wallet?.adapter?.name);
      await connect();
      console.log('✅ Connect promise resolved');
    } catch (connectError) {
      console.error('❌ Error connecting wallet:', connectError);
      const errorMessage = connectError?.message?.toLowerCase() || '';
      const errorName = connectError?.name || '';

      if (errorMessage.includes('user rejected') ||
          errorMessage.includes('user cancelled') ||
          errorMessage.includes('user denied') ||
          errorName === 'WalletConnectionError' ||
          errorName === 'WalletNotConnectedError') {
        console.log('ℹ️ User rejected or cancelled the connection');
        alert('Connection was cancelled. Please try again if you want to connect.');
      } else {
        alert(`Failed to connect wallet: ${connectError.message || 'Unknown error'}. Please try again.`);
        setShowModal(true);
      }
    }
  };

  useLayoutEffect(() => {
    if (pendingConnectRef.current && wallet && !connected && !connecting) {
      pendingConnectRef.current = false;
      doConnect();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wallet, connected, connecting]);

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

      // Select the wallet, then let the useLayoutEffect above call connect()
      // once `wallet` has actually updated to this new selection. But if
      // this wallet was ALREADY selected (e.g. wallet-adapter restored it
      // from localStorage on mount), select() is a no-op and `wallet`'s
      // reference never changes — the effect would then never re-fire and
      // the pending connect would sit unconsumed forever (clicking Solflare
      // would "select" it again but nothing would visibly happen). Connect
      // immediately in that case instead of waiting for a change that isn't
      // coming.
      const alreadySelected = wallet?.adapter?.name === walletName;
      try {
        select(walletName);
        console.log('✅ Wallet selected:', walletName);
        if (alreadySelected) {
          doConnect();
        } else {
          pendingConnectRef.current = true;
        }
      } catch (selectError) {
        console.error('❌ Error selecting wallet:', selectError);
        alert(`Failed to select wallet: ${selectError.message}`);
        return;
      }

      setShowModal(false);
    } catch (error) {
      console.error('❌ Error selecting wallet:', error);
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
          padding: '8px 14px',
          fontSize: '13px',
          fontWeight: 600,
          backgroundColor: '#999',
          color: 'white',
          border: 'none',
          borderRadius: '8px',
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
        <span style={{ fontSize: '13px', fontWeight: 600, color: '#512DA8' }}>
          {wallet?.adapter?.name || 'Wallet'}: {publicKey.toString().slice(0, 4)}...{publicKey.toString().slice(-4)}
        </span>
        <button
          onClick={disconnect}
          style={{
            padding: '8px 14px',
            fontSize: '13px',
            fontWeight: 600,
            backgroundColor: '#ff4444',
            color: 'white',
            border: 'none',
            borderRadius: '8px',
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
          padding: '8px 14px',
          fontSize: '13px',
          fontWeight: 600,
          backgroundColor: '#512DA8',
          color: 'white',
          border: 'none',
          borderRadius: '8px',
          cursor: connecting ? 'not-allowed' : 'pointer',
          opacity: connecting ? 0.6 : 1
        }}
      >
        {connecting ? 'Connecting...' : 'Connect Wallet'}
      </button>

      {showModal && (
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
              maxHeight: '80vh',
              overflowY: 'auto'
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
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
              {wallets.map((wallet) => {
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
        </div>
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

  return (
    <ConnectionProvider endpoint={endpoint}>
      <WalletProvider wallets={wallets} autoConnect={true}>
        <WalletModalProvider>
          <WalletListDebugger />
          <div>
            <header className="app-header">
              <div className="app-header-left">
                <div className="app-brand">
                  <span>◆ FlowDEX</span>
                  <span className="devnet-badge">DEVNET</span>
                </div>
                <div className="tab-navigation">
                  <button
                    className={`tab-btn ${activeTab === 'tokens' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('tokens')}
                  >
                    🪙 Tokens
                  </button>
                  <button
                    className={`tab-btn ${activeTab === 'sources' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('sources')}
                  >
                    📁 Sources
                  </button>
                  <button
                    className={`tab-btn ${activeTab === 'portfolio' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('portfolio')}
                  >
                    👤 Portfolio
                  </button>
                  <button
                    className={`tab-btn ${activeTab === 'create-token' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('create-token')}
                  >
                    🪙 Create
                  </button>
                  <button
                    className={`tab-btn ${activeTab === 'pool' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('pool')}
                  >
                    🏊 Pools
                  </button>
                  <button
                    className={`tab-btn ${activeTab === 'faucet' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('faucet')}
                  >
                    🚰 Faucet
                  </button>
                  <button
                    className={`tab-btn ${activeTab === 'token-security' ? 'active' : ''}`}
                    onClick={() => handleTabSwitch('token-security')}
                  >
                    🛡️ Security
                  </button>
                </div>
              </div>
              <CustomWalletButton />
            </header>

            <main>
              {activeTab === 'tokens' && (
                <TokenList onTokenSelect={handleTokenSelect} />
              )}
              {activeTab === 'chart' && selectedToken && (
                <TradingViewChart token={selectedToken} />
              )}
              {activeTab === 'create-token' && (
                <TokenCreation />
              )}
              {activeTab === 'pool' && (
                <PoolHub />
              )}
              {activeTab === 'sources' && (
                <Sources onTokenSelect={handleTokenSelect} />
              )}
              {activeTab === 'portfolio' && (
                <Portfolio onTokenSelect={handleTokenSelect} />
              )}
              {activeTab === 'faucet' && (
                <Faucet />
              )}
              {activeTab === 'token-security' && (
                <TokenSecurity />
              )}
            </main>
            
            <WalletDebugger isVisible={showDebugger} />
          </div>
        </WalletModalProvider>
      </WalletProvider>
    </ConnectionProvider>
  );
}

export default App;