import React, { useState } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import './PoolCreation.css';
import { Transaction, VersionedTransaction, Message } from '@solana/web3.js';
import { Buffer } from 'buffer';
import * as aSDK from '@solana/spl-token-registry';
import { recordUserAsset } from '../lib/trade';

const PoolCreation = () => {
  const { publicKey, connected, sendTransaction } = useWallet();
  const { connection } = useConnection();
  
  // Form state
  const [formData, setFormData] = useState({
    tokenMint0: '',
    // Default quote token to WSOL — the only quote mint that works on devnet
    // (mainnet USDC/USDT mints don't exist there).
    tokenMint1: 'So11111111111111111111111111111111111111112',
    initialPrice: '',
    feeTier: '30', // Default to 0.3% (30 basis points)
    openTime: Math.floor(Date.now() / 1000), // Current timestamp
  });
  
  // UI state
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [txSignature, setTxSignature] = useState('');
  const [debugInfo, setDebugInfo] = useState(null);

  // API URL - explicitly using port 8083
  const API_URL = process.env.NODE_ENV === 'development' 
    ? '/v1' // Use proxy in development
    : '/direct-api/v1'; // Use Nginx proxy in production (via /direct-api)

  // Fee tier options based on common CLMM standards (in basis points)
  const feeTiers = [
    { value: '1', label: '0.01% - Stablecoins', tickSpacing: 1 },
    { value: '5', label: '0.05% - Low volatility', tickSpacing: 10 },
    { value: '30', label: '0.3% - Standard', tickSpacing: 60 },
    { value: '100', label: '1% - High volatility', tickSpacing: 200 }
  ];

  const handleInputChange = (e) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value
    }));

    // If openTime is updated through the date picker, convert it to Unix timestamp
    if (name === 'openTime') {
      try {
        const timestamp = Math.floor(new Date(value).getTime() / 1000);
        setFormData(prev => ({ ...prev, openTime: timestamp }));
      } catch (err) {
        console.error("Date parsing error:", err);
      }
    }
  };

  const validateForm = () => {
    if (!formData.tokenMint0.trim()) return 'Token 0 address is required';
    if (!formData.tokenMint1.trim()) return 'Token 1 address is required';
    if (formData.tokenMint0 === formData.tokenMint1) return 'Token addresses must be different';
    if (!formData.initialPrice || parseFloat(formData.initialPrice) <= 0) return 'Initial price must be greater than 0';
    return null;
  };

  const logTransactionDetails = (transaction, title = "Transaction Details") => {
    console.group(title);
    try {
      // Try to extract and log key properties
      if (transaction instanceof Transaction) {
        console.log("📄 Transaction Type: Legacy Transaction");
        console.log("🔑 Fee Payer:", transaction.feePayer?.toString() || "Not set");
        console.log("📚 Instructions Count:", transaction.instructions?.length || 0);
        console.log("🧮 Signatures Count:", transaction.signatures?.length || 0);
        console.log("📝 Recent Blockhash:", transaction.recentBlockhash || "Not set");
        
        // Log instructions
        if (transaction.instructions?.length) {
          console.group("🧩 Instructions:");
          transaction.instructions.forEach((ix, i) => {
            console.log(`Instruction #${i + 1}:`);
            console.log(`  Program ID: ${ix.programId?.toString() || "Unknown"}`);
            console.log(`  Data (hex): ${Buffer.from(ix.data || []).toString('hex')}`);
            console.log(`  Accounts: ${ix.keys?.length || 0}`);
          });
          console.groupEnd();
        }
      } else if (transaction instanceof VersionedTransaction) {
        console.log("📄 Transaction Type: Versioned Transaction");
        console.log("🔢 Transaction Version:", transaction.version);
        
        if (transaction.message) {
          console.log("🔑 Addresses:", transaction.message.staticAccountKeys?.length || 0);
          console.log("📚 Instructions Count:", transaction.message.compiledInstructions?.length || 0);
          console.log("🧮 Signatures Count:", transaction.signatures?.length || 0);
        }
      } else {
        console.log("Unknown transaction type:", typeof transaction);
      }
      
      // Try to serialize to check if it's valid
      try {
        const serialized = transaction.serialize ? 
          transaction.serialize() : 
          (transaction instanceof VersionedTransaction ? 
            transaction.serialize() : 
            "Serialization method not available");
            
        console.log("✅ Transaction can be serialized:", serialized.length, "bytes");
      } catch (e) {
        console.error("❌ Serialization error:", e);
      }
    } catch (e) {
      console.error("Error logging transaction:", e);
    }
    console.groupEnd();
    
    // Store debugging info for UI display
    try {
      let debugData = {
        transactionType: transaction instanceof VersionedTransaction ? "Versioned" : "Legacy",
        instructionsCount: transaction.instructions?.length || transaction.message?.compiledInstructions?.length || "Unknown",
        signaturesNeeded: transaction.signatures?.length || "Unknown",
        signaturesPresent: transaction.signatures?.filter(s => s !== null)?.length || 0,
        feePayer: transaction.feePayer?.toString() || "Unknown"
      };
      setDebugInfo(debugData);
    } catch (e) {
      console.error("Error storing debug info:", e);
    }
  };

  const createPool = async () => {
    if (!connected || !publicKey) {
      setError('Please connect your wallet first');
      return;
    }

    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    setIsLoading(true);
    setError('');
    setSuccess('');
    setTxSignature('');
    setDebugInfo(null);

    try {
      console.group('🏊 Pool Creation Process');
      console.log('Starting pool creation...');
      console.log('Form data:', formData);

      // Set current time as the open time
      const currentTime = Math.floor(Date.now() / 1000);

      // Prepare the API request
      const requestData = {
        chain_id: 100000, // Solana devnet ID
        token_mint_0: formData.tokenMint0,
        token_mint_1: formData.tokenMint1,
        initial_price: formData.initialPrice.toString(),
        fee_tier: parseInt(formData.feeTier),
        open_time: currentTime, // Use current time
        user_wallet_address: publicKey.toString()
      };

      console.log('🔗 Calling pool creation API with:', requestData);
      console.log('📅 Using current time for openTime:', new Date(currentTime * 1000).toISOString());

      // Call the backend API using the explicit URL with port 8083
      console.time('API Call Duration');
      const response = await fetch(`${API_URL}/trade/create_pool`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestData)
      });
      console.timeEnd('API Call Duration');

      console.log('📡 API Response status:', response.status);

      if (!response.ok) {
        const errorText = await response.text();
        console.error('❌ API Error:', errorText);
        throw new Error(`API error: ${response.status} ${errorText}`);
      }

      const result = await response.json();
      console.log('✅ Pool creation API result:', result);

      // Check for transaction hash in the API response structure
      const txHash = result.data?.txHash;
      const poolAddress = result.data?.poolAddress;
      if (txHash) {
        console.log('🎯 Received transaction from API, now signing and sending...');
        console.log('Transaction hash length:', txHash.length);
        
        try {
          // Decode the transaction from base64
          console.time('Transaction Processing');
          const transactionBuffer = Buffer.from(txHash, 'base64');
          
          console.log('📄 Transaction buffer length:', transactionBuffer.length);
          console.log('Transaction buffer (hex):', transactionBuffer.toString('hex').substring(0, 100) + '...');
          
          // Try different transaction deserialization methods
          let transaction;
          let deserializationMethod = 'unknown';

          try {
            // First try legacy Transaction format
            console.log('🔍 Attempting to deserialize as legacy transaction...');
            transaction = Transaction.from(transactionBuffer);
            console.log('✅ Successfully deserialized as legacy transaction');
            deserializationMethod = 'legacy';
          } catch (legacyError) {
            console.warn('⚠️ Legacy transaction deserialization failed:', legacyError);
            console.error('Legacy error details:', {
              name: legacyError.name,
              message: legacyError.message,
              stack: legacyError.stack
            });
            
            try {
              // Then try VersionedTransaction format
              console.log('🔍 Attempting to deserialize as versioned transaction...');
              transaction = VersionedTransaction.deserialize(transactionBuffer);
              console.log('✅ Successfully deserialized as versioned transaction');
              deserializationMethod = 'versioned';
            } catch (versionedError) {
              // Last resort: Try to parse as raw Message
              console.warn('⚠️ Versioned transaction deserialization failed:', versionedError);
              console.error('Versioned error details:', {
                name: versionedError.name,
                message: versionedError.message,
                stack: versionedError.stack
              });
              
              console.log('🔄 Attempting to parse as Message and reconstruct transaction...');
              
              try {
                // Try to parse as raw Message format
                const message = Message.from(transactionBuffer);
                transaction = new Transaction();
                transaction.feePayer = publicKey;
                
                // Populate from the message
                transaction.recentBlockhash = message.recentBlockhash;
                
                message.instructions.forEach(ix => {
                  transaction.add({
                    programId: ix.programId,
                    keys: ix.keys,
                    data: ix.data
                  });
                });
                
                console.log('✅ Successfully reconstructed transaction from Message');
                deserializationMethod = 'message';
              } catch (messageError) {
                console.error('❌ All transaction deserialization methods failed');
                console.error('Message parsing error:', {
                  name: messageError.name, 
                  message: messageError.message,
                  stack: messageError.stack
                });
                
                // Last desperate attempt - try as partial transaction
                try {
                  console.log('🔄 Last attempt: Creating minimal transaction');
                  // Create a minimal transaction
                  transaction = new Transaction();
                  transaction.feePayer = publicKey;
                  // Get a recent blockhash
                  const { blockhash } = await connection.getLatestBlockhash();
                  transaction.recentBlockhash = blockhash;
                  deserializationMethod = 'minimal';
                } catch (minimalError) {
                  throw new Error(`Unable to create any valid transaction: ${minimalError.message}`);
                }
              }
            }
          }

          console.log('✅ Transaction created using method:', deserializationMethod);
          
          // Log transaction details
          logTransactionDetails(transaction, "🔍 Pre-Signing Transaction Details");
          
          // Add extra options for transaction sending
          const sendOptions = {
            skipPreflight: true,
            maxRetries: 5
          };
          
          console.log('🔏 Sending transaction to wallet for signing...');
          console.time('Transaction Signing');
          
          try {
            // Use wallet-adapter's own sendTransaction rather than manually
            // signing + re-serializing + sendRawTransaction: that round-trip
            // through app code (sign, then serialize the signed result
            // ourselves) is what produced "signature has invalid length" for
            // some wallets. sendTransaction lets the wallet extension use its
            // own atomic signAndSendTransaction where available.
            console.log('Sending transaction via wallet-adapter...');
            const signature = await sendTransaction(transaction, connection, sendOptions);
            console.log('🚀 Transaction sent:', signature);
            setTxSignature(signature);
          } catch (signError) {
            console.error('❌ Transaction signing/sending error:', signError);
            console.error('Error details:', {
              name: signError.name,
              message: signError.message,
              code: signError.code,
              logs: signError.logs,
              stack: signError.stack
            });
            
            // Try to extract error code if available
            const errorCode = signError.code || 
                           (signError.message && signError.message.includes('code') 
                              ? signError.message.match(/code\s*:\s*(\d+)/i)?.[1] 
                              : null);
            
            if (errorCode) {
              throw new Error(`Wallet error (code ${errorCode}): ${signError.message}`);
            } else {
              throw signError;
            }
          }
          console.timeEnd('Transaction Signing');
          
          console.log('Waiting for confirmation...');
          console.time('Transaction Confirmation');
          // Wait for confirmation (optional)
          const confirmation = await connection.confirmTransaction(txSignature);
          console.timeEnd('Transaction Confirmation');
          console.log('✅ Transaction confirmed:', confirmation);
          
          setSuccess(`🎉 Pool created successfully! Transaction: ${txSignature}`);
          if (poolAddress) {
            recordUserAsset({
              wallet_address: publicKey.toString(),
              asset_type: 'pool',
              asset_name: `${formData.tokenMint0.slice(0, 4)}/${formData.tokenMint1.slice(0, 4)} CLMM Pool`,
              asset_address: poolAddress,
              token0_address: formData.tokenMint0,
              token1_address: formData.tokenMint1,
              pool_type: 'RaydiumCLMM',
              fee_tier: Number(formData.feeTier),
            }).catch((e) => console.warn('record_user_asset failed (non-fatal):', e));
          }
        } catch (signError) {
          console.error('❌ Failed to sign/send transaction:', signError);
          setError(`Failed to sign/send transaction: ${signError.message}`);
        } finally {
          console.timeEnd('Transaction Processing');
        }
      } else {
        console.error('❌ Missing transaction hash in API response:', result);
        throw new Error('No transaction hash received from API');
      }
      
    } catch (err) {
      console.error('❌ Pool creation error:', err);
      setError(`Failed to create pool: ${err.message}`);
    } finally {
      setIsLoading(false);
      console.groupEnd();
    }
  };

  if (!connected) {
    return (
      <div className="pool-creation-container">
        <div className="wallet-not-connected">
          <h2>🔗 Connect Wallet</h2>
          <p>Please connect your wallet to create pools</p>
        </div>
      </div>
    );
  }

  return (
    <div className="pool-creation-container">
      <div className="pool-creation-card">
        <div className="card-header">
          <h2>🏊 Create CLMM Pool</h2>
          <p>Create a concentrated liquidity pool for your token pair</p>
        </div>

        <div className="form-section">
          <div className="token-pair-section">
            <h3>💱 Token Pair</h3>
            <div className="input-grid">
              <div className="input-group">
                <label>Token 0 Address *</label>
                <input
                  type="text"
                  name="tokenMint0"
                  value={formData.tokenMint0}
                  onChange={handleInputChange}
                  placeholder="e.g. So11111111111111111111111111111111111111112"
                  disabled={isLoading}
                />
                <small>Base token (usually the project token)</small>
              </div>

              <div className="input-group">
                <label>Token 1 Address *</label>
                <input
                  type="text"
                  name="tokenMint1"
                  value={formData.tokenMint1}
                  onChange={handleInputChange}
                  placeholder="e.g. EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
                  disabled={isLoading}
                />
                <small>Quote token (usually SOL or USDC)</small>
              </div>
            </div>
          </div>

          <div className="price-section">
            <h3>💰 Initial Price</h3>
            <div className="input-group">
              <label>Price (Token1 per Token0) *</label>
              <input
                type="number"
                name="initialPrice"
                value={formData.initialPrice}
                onChange={handleInputChange}
                placeholder="0.001"
                step="any"
                min="0"
                disabled={isLoading}
              />
              <small>Initial exchange rate for the pool</small>
            </div>
          </div>

          <div className="fee-section">
            <h3>⚡ Fee Tier</h3>
            <div className="fee-grid">
              {feeTiers.map(tier => (
                <label key={tier.value} className="fee-option">
                  <input
                    type="radio"
                    name="feeTier"
                    value={tier.value}
                    checked={formData.feeTier === tier.value}
                    onChange={handleInputChange}
                    disabled={isLoading}
                  />
                  <div className="fee-content">
                    <span className="fee-rate">{tier.label}</span>
                    <small>Tick spacing: {tier.tickSpacing}</small>
                  </div>
                </label>
              ))}
            </div>
          </div>

          <div className="timing-section">
            <h3>⏰ Launch Settings</h3>
            <div className="input-group">
              <label>Open Time</label>
              <div className="current-time-notice">
                Using current time for pool opening
              </div>
              <small>Pool will open immediately for trading</small>
            </div>
          </div>

          <div className="info-section">
            <h3>ℹ️ Pool Information</h3>
            <div className="info-grid">
              <div className="info-item">
                <span>Type:</span>
                <span>Raydium CLMM</span>
              </div>
              <div className="info-item">
                <span>Program:</span>
                <span>675kPX...1Mp8</span>
              </div>
              <div className="info-item">
                <span>Network:</span>
                <span>Devnet</span>
              </div>
              </div>
            </div>
          </div>

        {error && <div className="error-message">❌ {error}</div>}
        {success && <div className="success-message">{success}</div>}
        {txSignature && (
          <div className="tx-signature">
            <p>Transaction ID:</p>
            <a 
              href={`https://explorer.solana.com/tx/${txSignature}?cluster=devnet`} 
              target="_blank" 
              rel="noopener noreferrer"
            >
              {txSignature.slice(0, 8)}...{txSignature.slice(-8)}
            </a>
            </div>
          )}

            <button
          className="create-pool-button" 
              onClick={createPool}
          disabled={isLoading}
            >
          {isLoading ? 'Creating Pool...' : 'Create Pool'}
            </button>
      </div>
    </div>
  );
};

export default PoolCreation; 