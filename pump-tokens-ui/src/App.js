import React, { useState, useEffect, useMemo } from 'react';
import TokenList from './components/TokenList';
import TradingViewChart from './components/TradingViewChart';
import WalletDebugger from './components/WalletDebugger';
import TokenCreation from './components/TokenCreation';
import PoolCreation from './components/PoolCreation';
import AddLiquidity from './components/AddLiquidity';
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

  const handleConnect = () => {
    setShowModal(true);
  };

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
      
      // First select the wallet - this must happen synchronously
      try {
        select(walletName);
        console.log('✅ Wallet selected:', walletName);
      } catch (selectError) {
        console.error('❌ Error selecting wallet:', selectError);
        alert(`Failed to select wallet: ${selectError.message}`);
        return;
      }
      
      // Close modal immediately
      setShowModal(false);
      
      // IMPORTANT: connect() must be called directly in the user event handler
      // Don't delay it, as browsers may block popups/authorization prompts
      // The wallet extension should show an authorization popup now
      try {
        console.log('🔗 Connecting to wallet:', walletName);
        console.log('🔍 Current wallet state before connect:', {
          wallet: wallet?.adapter?.name,
          connected,
          connecting,
          publicKey: publicKey?.toString()
        });
        
        // Call connect directly - this should trigger wallet authorization popup
        // The user should see a popup from the wallet extension asking for permission
        const connectPromise = connect();
        console.log('⏳ Waiting for wallet authorization...');
        console.log('👤 Please check your wallet extension for an authorization popup');
        
        await connectPromise;
        
        console.log('✅ Connect promise resolved');
        
        // Check state after a short delay to allow React to update
        setTimeout(() => {
          console.log('🔍 State after connect:', {
            wallet: wallet?.adapter?.name,
            connected,
            connecting,
            publicKey: publicKey?.toString()
          });
          
          if (!connected || !publicKey) {
            console.warn('⚠️ Connection completed but state not updated. This might be a timing issue.');
          }
        }, 500);
        
      } catch (connectError) {
        console.error('❌ Error connecting wallet:', connectError);
        console.error('❌ Error details:', {
          message: connectError?.message,
          name: connectError?.name,
          stack: connectError?.stack,
          error: connectError
        });
        
        // Check if it's a user rejection
        const errorMessage = connectError?.message?.toLowerCase() || '';
        const errorName = connectError?.name || '';
        
        if (errorMessage.includes('user rejected') || 
            errorMessage.includes('user cancelled') ||
            errorMessage.includes('user denied') ||
            errorName === 'WalletConnectionError' ||
            errorName === 'WalletNotConnectedError') {
          console.log('ℹ️ User rejected or cancelled the connection');
          alert('Connection was cancelled. Please try again if you want to connect.');
          // Don't reopen modal on user rejection
        } else {
          // Show error to user
          alert(`Failed to connect wallet: ${connectError.message || 'Unknown error'}. Please try again.`);
          // Reopen modal if connection fails for other reasons
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
      <WalletProvider wallets={wallets} autoConnect={false}>
        <WalletModalProvider>
          <WalletListDebugger />
          <div>
            <header className="app-header">
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
                  className={`tab-btn ${activeTab === 'create-token' ? 'active' : ''}`}
                  onClick={() => handleTabSwitch('create-token')}
                >
                  🪙 Create Token
                </button>
                <button
                  className={`tab-btn ${activeTab === 'create-pool' ? 'active' : ''}`}
                  onClick={() => handleTabSwitch('create-pool')}
                >
                  🏊 Create Pool
                </button>
                <button
                  className={`tab-btn ${activeTab === 'add-liquidity' ? 'active' : ''}`}
                  onClick={() => handleTabSwitch('add-liquidity')}
                >
                  💧 Add Liquidity
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
                  🛡️ Token Security
                </button>
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
              {activeTab === 'create-pool' && (
                <PoolCreation />
              )}
              {activeTab === 'add-liquidity' && (
                <AddLiquidity />
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