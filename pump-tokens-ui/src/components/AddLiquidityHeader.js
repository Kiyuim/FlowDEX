import React, { useState, useEffect } from 'react';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import './AddLiquidityHeader.css';
import { PublicKey } from '@solana/web3.js';

// API URL configuration
const API_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

const AddLiquidityHeader = ({ onAddLiquidity }) => {
  const { connection } = useConnection();
  const { publicKey, connected } = useWallet();
  
  // State for form values
  const [pools, setPools] = useState([]);
  const [selectedPool, setSelectedPool] = useState(null);
  const [manualPoolAddress, setManualPoolAddress] = useState('');
  const [useManualInput, setUseManualInput] = useState(false);
  const [priceRange, setPriceRange] = useState([0, 0]);
  const [inputToken, setInputToken] = useState('A'); // 'A' or 'B'
  const [tokenAAmount, setTokenAAmount] = useState('');
  const [tokenBAmount, setTokenBAmount] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  
  // Manual pool details input
  const [showManualPoolDetails, setShowManualPoolDetails] = useState(false);
  const [manualTokenAAddress, setManualTokenAAddress] = useState('');
  const [manualTokenBAddress, setManualTokenBAddress] = useState('');
  const [manualTokenASymbol, setManualTokenASymbol] = useState('Token A');
  const [manualTokenBSymbol, setManualTokenBSymbol] = useState('Token B');
  const [manualCurrentPrice, setManualCurrentPrice] = useState('1');
    // 新增自动获取token mint相关状态
  const [autoTokenA, setAutoTokenA] = useState('');
  const [autoTokenB, setAutoTokenB] = useState('');
  const [autoFetchLoading, setAutoFetchLoading] = useState(false);
    
  // Pool information
  const [poolInfo, setPoolInfo] = useState(null);
  
  // Fetch pools on component mount
  useEffect(() => {
    fetchPools();
  }, []);
  
  // Update pool info when selected pool changes
  useEffect(() => {
    if (selectedPool) {
      fetchPoolDetails(selectedPool);
    }
  }, [selectedPool]);

  // Update pool info when manual pool address changes and is valid
  useEffect(() => {
    if (useManualInput && manualPoolAddress && manualPoolAddress.length >= 32) {
      fetchPoolDetailsByAddress(manualPoolAddress);
    }
  }, [useManualInput, manualPoolAddress]);
  
  // Fetch available pools
  const fetchPools = async () => {
    try {
      setIsLoading(true);
      // Replace with actual API call
      const response = await fetch(`${API_URL}/v1/market/index_clmm?chain_id=100000&pool_version=1&page_no=1&page_size=50`);
      const data = await response.json();
      
      if (data && data.data && data.data.list) {
        setPools(data.data.list);
        // If no pools available, default to manual input
        if (data.data.list.length === 0) {
          setUseManualInput(true);
        }
      }
    } catch (error) {
      console.error('Error fetching pools:', error);
      setError('Failed to fetch pools');
      // Default to manual input on error
      setUseManualInput(true);
    } finally {
      setIsLoading(false);
    }
  };
  
  // Fetch pool details by ID (from dropdown)
  const fetchPoolDetails = async (poolId) => {
    try {
      setIsLoading(true);
      // Find the pool in the already fetched pools
      const pool = pools.find(p => p.poolState === poolId);

      if (pool) {
        setPoolInfo({
          id: pool.poolState,
          tokenA: {
            symbol: pool.inputTokenSymbol || 'Token A',
            address: pool.inputVaultMint
          },
          tokenB: {
            symbol: pool.outputTokenSymbol || 'Token B',
            address: pool.outputVaultMint
          },
          price: pool.price || 1,
          minPrice: pool.price * 0.5, // Example - use actual min from API
          maxPrice: pool.price * 2,   // Example - use actual max from API
        });
        
        // Set initial price range based on current price
        setPriceRange([pool.price * 0.8, pool.price * 1.2]);
      }
    } catch (error) {
      console.error('Error fetching pool details:', error);
      setError('Failed to fetch pool details');
    } finally {
      setIsLoading(false);
    }
  };

  // Fetch pool details by address (manual input)
  const fetchPoolDetailsByAddress = async (address) => {
    try {
      setIsLoading(true);
      setError('');
      
      // Try to find the pool in already fetched pools first
      const existingPool = pools.find(p => p.poolState === address);
      
      if (existingPool) {
        setSelectedPool(address);
        return; // The useEffect will handle setting the pool info
      }
      
      // If not found, fetch directly from API - use the index_clmm endpoint with pool_state filter
      try {
        const response = await fetch(`${API_URL}/v1/market/index_clmm?chain_id=100000&pool_state=${address}&page_no=1&page_size=1`);
        
        if (!response.ok) {
          // API failed, show manual input form
          setShowManualPoolDetails(true);
          setError(`Pool details not found (Status: ${response.status}). Please enter pool details manually.`);
          return;
        }
        
        const data = await response.json();
        
        if (data && data.data && data.data.list && data.data.list.length > 0) {
          const pool = data.data.list[0]; // Get first item from list
          setPoolInfo({
            id: address,
            tokenA: {
              symbol: pool.inputTokenSymbol || 'Token A',
              address: pool.inputVaultMint
            },
            tokenB: {
              symbol: pool.outputTokenSymbol || 'Token B',
              address: pool.outputVaultMint
            },
            price: pool.price || 1,
            minPrice: pool.price * 0.5,
            maxPrice: pool.price * 2,
          });
          
          // Set initial price range based on current price
          setPriceRange([pool.price * 0.8, pool.price * 1.2]);
          setShowManualPoolDetails(false);
        } else {
          // API returned no data
          setShowManualPoolDetails(true);
          setError('Pool details not found. Please enter pool details manually.');
        }
      } catch (apiError) {
        console.error('API error:', apiError);
        setShowManualPoolDetails(true);
        setError('Failed to fetch pool details. Please enter pool details manually.');
      }
    } catch (error) {
      console.error('Error fetching pool details by address:', error);
      setError('Failed to fetch pool details. Please enter pool details manually.');
      setShowManualPoolDetails(true);
    } finally {
      setIsLoading(false);
    }
  };
  
  // Create pool info from manual inputs
  const createManualPoolInfo = () => {
    if (!manualPoolAddress || !manualTokenAAddress || !manualTokenBAddress) {
      setError('Please fill in all required pool details');
      return;
    }
    
    try {
      const price = parseFloat(manualCurrentPrice);
      if (isNaN(price) || price <= 0) {
        setError('Please enter a valid price');
        return;
      }
      
      const poolInfo = {
        id: manualPoolAddress,
        tokenA: {
          symbol: manualTokenASymbol || 'Token A',
          address: manualTokenAAddress
        },
        tokenB: {
          symbol: manualTokenBSymbol || 'Token B',
          address: manualTokenBAddress
        },
        price: price,
        minPrice: price * 0.5,
        maxPrice: price * 2,
      };
      
      setPoolInfo(poolInfo);
      setPriceRange([price * 0.8, price * 1.2]);
      setError('');
    } catch (error) {
      console.error('Error creating pool info:', error);
      setError('Failed to create pool info. Please check your inputs.');
    }
  };
  
  // Calculate other token amount based on price and input
  const calculateOtherAmount = (amount, isTokenA) => {
    if (!poolInfo || !amount) return '';
    
    // Simple calculation example - replace with actual formula based on CLMM math
    const price = poolInfo.price || 1;
    
    if (isTokenA) {
      return (parseFloat(amount) * price).toFixed(6);
    } else {
      return (parseFloat(amount) / price).toFixed(6);
    }
  };
  
  // Handle token amount changes
  const handleTokenAChange = (e) => {
    const value = e.target.value;
    setTokenAAmount(value);
    if (inputToken === 'A') {
      setTokenBAmount(calculateOtherAmount(value, true));
    }
  };
  
  const handleTokenBChange = (e) => {
    const value = e.target.value;
    setTokenBAmount(value);
    if (inputToken === 'B') {
      setTokenAAmount(calculateOtherAmount(value, false));
    }
  };
  
  // Switch input token
  const switchInputToken = () => {
    setInputToken(inputToken === 'A' ? 'B' : 'A');
    // Recalculate amounts
    if (inputToken === 'A' && tokenAAmount) {
      setTokenBAmount(calculateOtherAmount(tokenAAmount, true));
    } else if (inputToken === 'B' && tokenBAmount) {
      setTokenAAmount(calculateOtherAmount(tokenBAmount, false));
    }
  };

  // Toggle between dropdown and manual input
  const toggleInputMethod = () => {
    setUseManualInput(!useManualInput);
    // Clear selected pool when switching to manual input
    if (!useManualInput) {
      setSelectedPool(null);
    } else {
      setManualPoolAddress('');
    }
    // Clear pool info when switching input methods
    setPoolInfo(null);
    setShowManualPoolDetails(false);
  };
  
  // Handle add liquidity
  const handleAddLiquidity = () => {
    if (!connected) {
      setError('Please connect your wallet first');
      return;
    }
    
    const currentPoolId = useManualInput ? manualPoolAddress : selectedPool;
    
    if (!currentPoolId || !tokenAAmount || !tokenBAmount) {
      setError('Please fill in all required fields');
      return;
    }
    
    onAddLiquidity({
      poolId: currentPoolId,
      tickLower: priceRange[0],
      tickUpper: priceRange[1],
      baseToken: inputToken,
      tokenAAmount,
      tokenBAmount,
      walletAddress: publicKey.toString(),
      // Include token addresses from pool info
      tokenAAddress: poolInfo?.tokenA?.address || manualTokenAAddress,
      tokenBAddress: poolInfo?.tokenB?.address || manualTokenBAddress
    });
  };
  
  return (
    <div className="clmm-add-liquidity-header">
      <h3>Add Liquidity to CLMM Pool</h3>
      <div className="divider"></div>
      
      {error && <div className="error-message">{error}</div>}
      
      <div className="form-section">
        <div className="input-method-toggle">
          <button 
            className={`toggle-btn ${!useManualInput ? 'active' : ''}`}
            onClick={() => setUseManualInput(false)}
            disabled={pools.length === 0}
          >
            Select Pool
          </button>
          <button 
            className={`toggle-btn ${useManualInput ? 'active' : ''}`}
            onClick={() => setUseManualInput(true)}
          >
            Enter Pool Address
          </button>
        </div>
        
        {!useManualInput ? (
          <>
            <label>Select Pool</label>
            <select 
              className="select-input"
              onChange={(e) => setSelectedPool(e.target.value)}
              value={selectedPool || ''}
              disabled={pools.length === 0}
            >
              <option value="">Select a pool</option>
              {pools.map(pool => (
                <option key={pool.poolState} value={pool.poolState}>
                  {pool.inputTokenSymbol}/{pool.outputTokenSymbol}
                </option>
              ))}
            </select>
            {pools.length === 0 && (
              <div className="no-pools-message">
                No pools available. Please use manual input.
              </div>
            )}
          </>
        ) : (
          <>
            <label>Pool Address</label>
            <input 
              type="text"
              className="text-input"
              value={manualPoolAddress}
              onChange={(e) => setManualPoolAddress(e.target.value)}
              placeholder="Enter pool address (e.g., Bevpu2aknCe7ZotQDRy2LgbG1gtU8S1BFwcpLPziy8af)"
            />
          {/* Auto-fetch Token Info按钮和展示 */}
          <button
              type="button"
              style={{
                marginTop: 8,
                marginBottom: 8,
                padding: '8px 16px',
                backgroundColor: autoFetchLoading ? '#bfbfbf' : '#1890ff',
                color: 'white',
                border: 'none',
                borderRadius: 4,
                fontWeight: 500,
                fontSize: 14,
                cursor: autoFetchLoading ? 'not-allowed' : 'pointer',
                boxShadow: autoFetchLoading ? 'none' : '0 2px 8px rgba(24,144,255,0.08)',
                transition: 'background 0.2s',
                position: 'relative',
              }}
              disabled={!manualPoolAddress || autoFetchLoading}
              onMouseOver={e => { if (!autoFetchLoading) e.currentTarget.style.backgroundColor = '#40a9ff'; }}
              onMouseOut={e => { if (!autoFetchLoading) e.currentTarget.style.backgroundColor = '#1890ff'; }}
              onClick={async () => {
                setAutoFetchLoading(true);
                setError('');
                try {
                  const poolPubkey = new PublicKey(manualPoolAddress.trim());
                  const accountInfo = await connection.getAccountInfo(poolPubkey);
                  if (!accountInfo || !accountInfo.data) {
                    setError('Pool account not found or data is empty');
                    setAutoFetchLoading(false);
                    return;
                  }
                  const data = accountInfo.data;
                  const tokenMint0Bytes = data.slice(73, 105);
                  const tokenMint1Bytes = data.slice(105, 137);
                  const tokenMint0Pubkey = new PublicKey(tokenMint0Bytes).toBase58();
                  const tokenMint1Pubkey = new PublicKey(tokenMint1Bytes).toBase58();
                  setAutoTokenA(tokenMint0Pubkey);
                  setAutoTokenB(tokenMint1Pubkey);
                  setManualTokenAAddress(tokenMint0Pubkey);
                  setManualTokenBAddress(tokenMint1Pubkey);
                  setError('Auto-fetch successful, token addresses filled in');
                } catch (e) {
                  setError('Failed to fetch pool token info: ' + e.message);
                }
                setAutoFetchLoading(false);
              }}
            >
                 {autoFetchLoading ? (
                <span style={{ display: 'inline-flex', alignItems: 'center' }}>
                  <span className="loading-spinner" style={{ width: 18, height: 18, borderWidth: 3, marginRight: 8, borderTopColor: '#fff' }} />
                  Fetching...
                </span>
              ) : 'Auto-fetch Token Info'}
            </button>
            <div style={{ fontSize: 12, marginBottom: 8 }}>
              {autoTokenA && <>
                Token Mint 0: <span style={{ color: '#333' }}>{autoTokenA}</span><br />
              </>}
              {autoTokenB && <>
                Token Mint 1: <span style={{ color: '#333' }}>{autoTokenB}</span>
              </>}
              {error && error.startsWith('auto-success') && (
                <div style={{ color: '#2e7d32', marginTop: 4 }}>Auto-fetch successful, token addresses filled in</div>
              )}
              {error && error.startsWith('auto-fail:') && (
                <div style={{ color: '#d32f2f', marginTop: 4 }}>Failed to fetch pool token info: {error.replace('auto-fail:', '')}</div>
              )}
            </div>
            {/* Manual pool details input (existing) */}


            
            {showManualPoolDetails && (
              <div className="manual-pool-details">
                <h4>Enter Pool Details</h4>
                
                <div className="form-group">
                  <label>Token A Address</label>
                  <input 
                    type="text"
                    className="text-input"
                    value={manualTokenAAddress}
                    onChange={(e) => setManualTokenAAddress(e.target.value)}
                    placeholder="Enter token A mint address"
                  />
                </div>
                
                <div className="form-group">
                  <label>Token A Symbol</label>
                  <input 
                    type="text"
                    className="text-input"
                    value={manualTokenASymbol}
                    onChange={(e) => setManualTokenASymbol(e.target.value)}
                    placeholder="E.g., SOL"
                  />
                </div>
                
                <div className="form-group">
                  <label>Token B Address</label>
                  <input 
                    type="text"
                    className="text-input"
                    value={manualTokenBAddress}
                    onChange={(e) => setManualTokenBAddress(e.target.value)}
                    placeholder="Enter token B mint address"
                  />
                </div>
                
                <div className="form-group">
                  <label>Token B Symbol</label>
                  <input 
                    type="text"
                    className="text-input"
                    value={manualTokenBSymbol}
                    onChange={(e) => setManualTokenBSymbol(e.target.value)}
                    placeholder="E.g., USDC"
                  />
                </div>
                
                <div className="form-group">
                  <label>Current Price (Token B per Token A)</label>
                  <input 
                    type="number"
                    className="text-input"
                    value={manualCurrentPrice}
                    onChange={(e) => setManualCurrentPrice(e.target.value)}
                    placeholder="E.g., 1.5"
                    step="0.000001"
                    min="0.000001"
                  />
                </div>
                
                <button 
                  className="confirm-details-button"
                  onClick={createManualPoolInfo}
                >
                  Confirm Pool Details
                </button>
              </div>
            )}
          </>
        )}
      </div>
      
      {poolInfo && (
        <>
          <div className="form-section">
            <label>Set Price Range</label>
            <div className="price-range-container">
              <div className="price-inputs">
                <div className="price-input">
                  <label>Min Price</label>
                  <input 
                    type="number"
                    value={priceRange[0]}
                    onChange={(e) => setPriceRange([parseFloat(e.target.value) || 0, priceRange[1]])}
                  />
                </div>
                <div className="price-input">
                  <label>Max Price</label>
                  <input 
                    type="number"
                    value={priceRange[1]}
                    onChange={(e) => setPriceRange([priceRange[0], parseFloat(e.target.value) || 0])}
                  />
                </div>
              </div>
              <div className="slider-container">
                <input
                  type="range"
                  min={poolInfo.minPrice}
                  max={poolInfo.price}
                  value={priceRange[0]}
                  onChange={(e) => setPriceRange([parseFloat(e.target.value), priceRange[1]])}
                  className="range-slider"
                />
                <input
                  type="range"
                  min={poolInfo.price}
                  max={poolInfo.maxPrice}
                  value={priceRange[1]}
                  onChange={(e) => setPriceRange([priceRange[0], parseFloat(e.target.value)])}
                  className="range-slider"
                />
              </div>
              <div className="current-price">
                Current Price: {poolInfo.price} {poolInfo.tokenB.symbol}/{poolInfo.tokenA.symbol}
              </div>
            </div>
          </div>
          
          <div className="form-section">
            <label>Input Token Amounts</label>
            <div className="token-inputs-container">
              <div className="token-input">
                <label>{poolInfo.tokenA.symbol}</label>
                <input 
                  type="number"
                  value={tokenAAmount}
                  onChange={handleTokenAChange}
                  disabled={inputToken !== 'A'}
                  placeholder={`Enter ${poolInfo.tokenA.symbol} amount`}
                />
              </div>
              
              <button
                className="swap-button"
                onClick={switchInputToken}
                aria-label="Switch input token"
                title="Switch input token"
              >
                ↕️
              </button>
              
              <div className="token-input">
                <label>{poolInfo.tokenB.symbol}</label>
                <input 
                  type="number"
                  value={tokenBAmount}
                  onChange={handleTokenBChange}
                  disabled={inputToken !== 'B'}
                  placeholder={`Enter ${poolInfo.tokenB.symbol} amount`}
                />
              </div>
            </div>
          </div>
          
          <button 
            className="add-liquidity-button"
            onClick={handleAddLiquidity}
            disabled={!(useManualInput ? manualPoolAddress : selectedPool) || !tokenAAmount || !tokenBAmount || priceRange[0] >= priceRange[1] || isLoading}
          >
            {isLoading ? 'Processing...' : 'Add Liquidity'}
          </button>
        </>
      )}
    </div>
  );
};

export default AddLiquidityHeader; 