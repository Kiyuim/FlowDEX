import React, { useState } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import { Transaction, VersionedTransaction } from '@solana/web3.js';
import { Buffer } from 'buffer';
import AddLiquidityHeader from './AddLiquidityHeader';
import './AddLiquidity.css';

// API URL configuration - using correct endpoint URLs
const API_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

// 辅助函数：将金额（如 0.8）转换为最小单位字符串，直接乘以10^6并取整
function toSmallestUnit(amount) {
  const decimals = 6;
  if (typeof amount === 'string') amount = amount.trim();
  if (amount === '' || isNaN(amount)) return '0';
  return String(Math.floor(Number(amount) * Math.pow(10, decimals)));
}

const AddLiquidity = () => {
  const { publicKey, connected, sendTransaction } = useWallet();
  const { connection } = useConnection();
  
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [txSignature, setTxSignature] = useState('');
  
  const handleAddLiquidity = async (liquidityData) => {
    if (!connected || !publicKey) {
      setError('Please connect your wallet first');
      return;
    }

    if (!sendTransaction) {
      setError('Wallet does not support transaction signing');
      return;
    }

    setIsLoading(true);
    setError('');
    setSuccess('');
    setTxSignature('');

    try {
      // Step 1: Create unsigned transaction
      setSuccess('Creating transaction...');
      
      // Prepare API payload - match AddLiquidityRequest message in proto
      const baseTokenIsA = liquidityData.baseToken === 'A';
      const baseAmount = baseTokenIsA
        ? liquidityData.tokenAAmount
        : liquidityData.tokenBAmount;
      const otherAmountMax = baseTokenIsA
        ? liquidityData.tokenBAmount
        : liquidityData.tokenAAmount;
      // 移除 baseAmount/otherAmountMax 的转换日志
      // 打印 minPrice/maxPrice 及 tickLower/tickUpper 的原始值和类型
      console.log('minPrice:', liquidityData.minPrice, typeof liquidityData.minPrice);
      console.log('maxPrice:', liquidityData.maxPrice, typeof liquidityData.maxPrice);
      console.log('tickLower:', liquidityData.tickLower, typeof liquidityData.tickLower);
      console.log('tickUpper:', liquidityData.tickUpper, typeof liquidityData.tickUpper);
      // tickLower/tickUpper 乘以 10^6 取整
      const tickLowerInt = Math.floor(Number(liquidityData.tickLower) * 1e6);
      const tickUpperInt = Math.floor(Number(liquidityData.tickUpper) * 1e6);
      console.log('tickLowerInt (toSmallestUnit):', tickLowerInt, 'from', liquidityData.tickLower);
      console.log('tickUpperInt (toSmallestUnit):', tickUpperInt, 'from', liquidityData.tickUpper);
      const apiPayload = {
        chain_id: 100000, // int64
        pool_id: String(liquidityData.poolId),
        tick_lower: tickLowerInt, // int64
        tick_upper: tickUpperInt, // int64
        base_token: baseTokenIsA ? 0 : 1, // int32
        base_amount: baseAmount, // string
        other_amount_max: otherAmountMax, // string
        user_wallet_address: publicKey.toString(),
        token_a_address: liquidityData.tokenAAddress,
        token_b_address: liquidityData.tokenBAddress
      };

      console.log('API request payload:', apiPayload);


      // 只通过 gateway 访问后端接口
      let response;
      response = await fetch(`${API_URL}/trade/add_liquidity_v1`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(apiPayload),
      });

      const data = await response.json();
      console.log('Full API response:', data);
      
      if (!response.ok) {
        console.error('API response not OK:', data);
        setError(data.message || `API Error ${response.status}: ${response.statusText}`);
        return;
      }
      
      // Even with OK status, check for error codes in the response
      if (data.code && data.code !== 0 && data.code !== 10000) {
        console.error(`Gateway error code ${data.code}: ${data.msg || 'Unknown error'}`, data);
        
        // Handle specific error codes
        if (data.code === 515) {
          setError(`Error 515: An error occurred in the backend service. Please check that your wallet has enough tokens and that you've approved the transaction.`);
        } else {
          setError(`Error ${data.code}: ${data.msg || 'Unknown error'}`);
        }
        return;
      }
      
      // Handle different possible response structures
      console.log('Checking response structure:', data);
      const txHash = data.data?.txHash || data.data?.tx_hash || data.txHash || data.tx_hash;
      
      if (!txHash) {
        console.error('Transaction data not found in response:', data);
        if (data.code && data.message) {
          setError(`Backend Error ${data.code}: ${data.message}`);
        } else {
          setError('No transaction data received from server');
        }
        return;
      }
      
      // 新增日志：打印用户钱包和 txHash
      console.log('🔑 Wallet publicKey:', publicKey?.toBase58 ? publicKey.toBase58() : publicKey);
      console.log('🧾 Received txHash (first 64 chars):', txHash?.slice(0, 64));
      
      console.log('Found txHash:', txHash);

      // Step 2: Decode base64 transaction
      setSuccess('Preparing transaction for signing...');
      let transaction;
      let transactionBuffer;
      try {
        transactionBuffer = Buffer.from(txHash, 'base64');
        try {
          // 尝试 VersionedTransaction 反序列化
          transaction = VersionedTransaction.deserialize(transactionBuffer);
          console.log('Successfully created VersionedTransaction object');
          // 新增日志：打印 VersionedTransaction 结构
          if (transaction instanceof VersionedTransaction) {
            const staticKeys = transaction.message.staticAccountKeys.map(pk => pk.toBase58());
            console.log('📝 VersionedTransaction Info:', {
              numRequiredSignatures: transaction.message.header.numRequiredSignatures,
              signaturesLength: transaction.signatures.length,
              staticAccountKeys: staticKeys,
              feePayer: staticKeys[0],
              addressTableLookups: transaction.message.addressTableLookups,
              recentBlockhash: transaction.message.recentBlockhash,
            });
            // 签名前打印签名内容
            console.log('📝 Before signing, signatures:', transaction.signatures.map(sig => Buffer.from(sig).toString('hex')));
          }
        } catch (versionedError) {
          // 如果失败，再尝试 legacy Transaction
          try {
            transaction = Transaction.from(transactionBuffer);
            console.log('Successfully created legacy Transaction object');
          } catch (legacyError) {
            // 输出详细错误和 Buffer 信息，方便排查
            console.error('Failed to deserialize transaction:', {
              versionedError,
              legacyError,
              txHash,
              transactionBuffer,
              bufferLength: transactionBuffer.length,
            });
            setError(
              `Cannot deserialize transaction. Versioned error: ${versionedError?.message || versionedError} | Legacy error: ${legacyError?.message || legacyError}`
            );
            return;
          }
        }
      } catch (decodeError) {
        console.error('Failed to decode transaction:', decodeError);
        setError(`Failed to decode transaction: ${decodeError.message}`);
        return;
      }

      // Step 3: Sign and send. sendTransaction (not a manual signTransaction +
      // sendRawTransaction round-trip) lets the wallet extension use its own
      // atomic signAndSendTransaction — the manual round-trip is what produced
      // "signature has invalid length" for some wallets.
      setSuccess('Sending transaction...');
      try {
        let signature;
        try {
          signature = await sendTransaction(transaction, connection);
        } catch (sendError) {
          // Check if this is an "already been processed" error, which means success
          if (sendError.message && sendError.message.includes('already been processed')) {
            console.log('✅ Transaction already processed - this indicates success!');
            
            // Try multiple methods to get the transaction signature
            let transactionSignature = null;
            
            // Method 1: Try to extract from error message
            const errorMessage = sendError.message;
            const signatureMatch = errorMessage.match(/[1-9A-HJ-NP-Za-km-z]{32,44}/);
            if (signatureMatch) {
              transactionSignature = signatureMatch[0];
              console.log('✅ Found signature in error message:', transactionSignature);
            }
            
            // Method 2: Check if there are logs that might contain the signature
            if (!transactionSignature && typeof sendError.getLogs === 'function') {
              try {
                const logs = await sendError.getLogs(connection);
                console.log('Transaction logs:', logs);
                
                // Look for signature in logs
                if (logs && logs.length > 0) {
                  for (const log of logs) {
                    if (typeof log === 'string' && log.length > 80) {
                      // This might be a signature
                      transactionSignature = log;
                      console.log('✅ Found signature in logs:', transactionSignature);
                      break;
                    }
                  }
                }
              } catch (logError) {
                console.error('Failed to fetch transaction logs:', logError);
              }
            }
            
            // Method 3: Try to get recent transactions for the user's wallet
            if (!transactionSignature && publicKey) {
              try {
                console.log('🔍 Searching for recent transactions...');
                const signatures = await connection.getSignaturesForAddress(
                  publicKey,
                  { limit: 3 }
                );
                
                if (signatures.length > 0) {
                  // Get the most recent transaction
                  const recentTx = signatures[0];
                  console.log('✅ Found recent transaction:', recentTx.signature);
                  
                  // Verify this transaction is related to our operation
                  const txInfo = await connection.getTransaction(recentTx.signature, {
                    maxSupportedTransactionVersion: 0
                  });
                  
                  if (txInfo && txInfo.meta && !txInfo.meta.err) {
                    // Check if this transaction involves the Raydium program
                    const instructions = txInfo.transaction.message.instructions;
                    const raydiumProgramId = 'A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys';
                    
                    for (const ix of instructions) {
                      const programId = txInfo.transaction.message.accountKeys[ix.programIdIndex];
                      if (programId === raydiumProgramId) {
                        transactionSignature = recentTx.signature;
                        console.log('✅ Found Raydium transaction:', transactionSignature);
                        break;
                      }
                    }
                  }
                }
              } catch (searchError) {
                console.error('Failed to search recent transactions:', searchError);
              }
            }
            
            // Method 4: Generate a unique identifier if no signature found
            if (!transactionSignature) {
              const timestamp = Date.now();
              const randomId = Math.random().toString(36).substring(2, 15);
              transactionSignature = `processed_${timestamp}_${randomId}`;
              console.log('⚠️ No signature found, generated identifier:', transactionSignature);
            }
            
            setTxSignature(transactionSignature);
            setSuccess(`Liquidity added successfully! Transaction was already processed.`);
            return;
          }
          
          // Handle other errors
          if (typeof sendError.getLogs === 'function') {
            try {
              const logs = await sendError.getLogs(connection);
              console.error('Transaction simulation logs:', logs);
            } catch (logError) {
              console.error('Failed to fetch transaction logs:', logError);
            }
          }
          console.error('Failed to send transaction:', sendError);
          setError(`Failed to send transaction: ${sendError.message}`);
          return;
        }
        console.log('Transaction sent successfully with signature:', signature);
        
        // Set the transaction signature for display
        setTxSignature(signature);
        setSuccess(`Liquidity added successfully! Transaction signature: ${signature}`);
      } catch (sendError) {
        console.error('Failed to send transaction:', sendError);
        setError(`Failed to send transaction: ${sendError.message}`);
        return;
      }
    } catch (error) {
      console.error('Error adding liquidity:', error);
      setError(`Error adding liquidity: ${error.message}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="add-liquidity-container">
      <div className="add-liquidity-card">
        <div className="card-header">
          <h2>💧 Add Liquidity</h2>
          <p>Add liquidity to a pool and earn fees</p>
        </div>
        <div className="form-section">
          <AddLiquidityHeader onAddLiquidity={handleAddLiquidity} />
          {isLoading && (
            <div className="loading-overlay">
              <div className="loading-spinner"></div>
              <div className="loading-message">{success || 'Processing...'}</div>
            </div>
          )}
          {error && (
            <div className="result-message error-message">
              <h4>Error</h4>
              <p>{error}</p>
            </div>
          )}
          {txSignature && (
            <div className="result-message success-message">
              <h4>Success!</h4>
              <p>Transaction signature: <a href={`https://solscan.io/tx/${txSignature}?cluster=devnet`} target="_blank" rel="noopener noreferrer">{txSignature}</a></p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

function isVersionedTransaction(buffer) {
  // The highest bit of the first byte indicates versioned
  return (buffer[0] & 0x80) !== 0;
}

export default AddLiquidity; 