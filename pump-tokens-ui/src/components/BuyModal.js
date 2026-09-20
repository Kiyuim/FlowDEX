import React, { useState } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import { Transaction, VersionedTransaction, TransactionMessage, MessageV0 } from '@solana/web3.js';
import { Buffer } from 'buffer';
import './BuyModal.css';

// API URL configuration - using the same pattern as other components
const API_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

const BuyModal = ({ isOpen, onClose, token }) => {
  const { publicKey, connected, signTransaction, sendTransaction } = useWallet();
  const { connection } = useConnection();
  const [amountIn, setAmountIn] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [txSignature, setTxSignature] = useState('');

  // Helper function to convert VersionedTransaction to legacy Transaction
  const convertToLegacyTransaction = (versionedTx) => {
    try {
      console.log('🔄 Converting VersionedTransaction to legacy Transaction...');
      
      // Extract the message from versioned transaction
      const message = versionedTx.message;
      console.log('Message type:', message.constructor.name);
      
      // Check if this looks like a MessageV0 by examining its properties
      // (Using property detection instead of instanceof since class names are minified)
      const hasV0Properties = message.header && 
                             message.staticAccountKeys && 
                             message.recentBlockhash && 
                             message.instructions &&
                             Array.isArray(message.staticAccountKeys) &&
                             Array.isArray(message.instructions);
      
      if (hasV0Properties) {
        console.log('Detected MessageV0-like structure, proceeding with conversion...');
        
        // Extract data from message
        const {
          header,
          staticAccountKeys,
          recentBlockhash,
          instructions,
          addressTableLookups
        } = message;
        
        console.log('Message data:', {
          staticAccountKeys: staticAccountKeys?.length || 0,
          instructions: instructions?.length || 0,
          addressTableLookups: addressTableLookups?.length || 0,
          recentBlockhash: recentBlockhash?.toString().slice(0, 10) + '...',
          header: header ? Object.keys(header) : 'none'
        });
        
        // For legacy transactions, we can only use static account keys
        // Address table lookups are not supported in legacy format
        if (addressTableLookups && addressTableLookups.length > 0) {
          throw new Error('Cannot convert transaction with address table lookups to legacy format');
        }
        
        // Create legacy transaction using proper constructor
        const legacyTransaction = new Transaction();
        legacyTransaction.recentBlockhash = recentBlockhash;
        legacyTransaction.feePayer = staticAccountKeys[0]; // First account is usually the fee payer
        
        // Convert and add instructions using proper Transaction methods
        for (let index = 0; index < instructions.length; index++) {
          const instruction = instructions[index];
          
          try {
            console.log(`Converting instruction ${index}:`);
            console.log(`  programIdIndex: ${instruction.programIdIndex}`);
            console.log(`  accountsLength: ${instruction.accounts?.length || 0}`);
            console.log(`  dataLength: ${instruction.data?.length || 0}`);
            console.log(`  dataType: ${typeof instruction.data}`);
            console.log(`  dataConstructor: ${instruction.data?.constructor?.name || 'unknown'}`);
            console.log(`  dataIsUint8Array: ${instruction.data instanceof Uint8Array}`);
            console.log(`  dataIsBuffer: ${Buffer.isBuffer(instruction.data)}`);
            console.log(`  dataIsArray: ${Array.isArray(instruction.data)}`);
            console.log(`  dataFirstFewBytes: ${instruction.data?.slice ? Array.from(instruction.data.slice(0, 10)) : 'no slice method'}`);
            
            if (instruction.data) {
              console.log(`  Raw data object:`, instruction.data);
            }
            
            // Ensure instruction data is a Buffer
            let instructionData = instruction.data;
            
            // Handle null/undefined data
            if (!instructionData) {
              console.log(`Instruction ${index} has no data, creating empty buffer`);
              instructionData = Buffer.alloc(0);
            } else if (!Buffer.isBuffer(instructionData)) {
              if (Array.isArray(instructionData)) {
                instructionData = Buffer.from(instructionData);
              } else if (typeof instructionData === 'string') {
                instructionData = Buffer.from(instructionData, 'base64');
              } else if (instructionData instanceof Uint8Array) {
                instructionData = Buffer.from(instructionData);
              } else if (instructionData.constructor && (instructionData.constructor.name === 'Uint8Array' || instructionData.constructor.name === 'Er' || instructionData.constructor.name.includes('Array'))) {
                // Handle minified class names
                instructionData = Buffer.from(instructionData);
              } else if (typeof instructionData === 'object' && instructionData !== null) {
                // Handle object-like structures (like serialized Uint8Array)
                if (instructionData.type === 'Buffer' && Array.isArray(instructionData.data)) {
                  instructionData = Buffer.from(instructionData.data);
                } else if (instructionData.length !== undefined && typeof instructionData.length === 'number') {
                  // Object with length property, try to convert to array
                  instructionData = Buffer.from(Object.values(instructionData));
                } else {
                  console.warn(`Unknown data type for instruction ${index}, attempting conversion:`, instructionData);
                  instructionData = Buffer.from(Object.values(instructionData));
                }
              } else {
                console.warn(`Unknown data type for instruction ${index}, attempting conversion:`, instructionData);
                instructionData = Buffer.from(Object.values(instructionData));
              }
            }
            
            console.log(`Instruction ${index} data converted:`, {
              originalType: typeof instruction.data,
              originalConstructor: instruction.data?.constructor?.name || 'unknown',
              newType: Buffer.isBuffer(instructionData) ? 'Buffer' : typeof instructionData,
              newConstructor: instructionData?.constructor?.name || 'unknown',
              length: instructionData?.length || 0,
              isBuffer: Buffer.isBuffer(instructionData),
              firstFewBytes: instructionData?.slice ? Array.from(instructionData.slice(0, 10)) : 'no slice method'
            });
            
            // Create the instruction object
            const convertedInstruction = {
              programId: staticAccountKeys[instruction.programIdIndex],
              keys: instruction.accounts.map((accountIndex, keyIndex) => {
                const accountIndexValue = typeof accountIndex === 'object' ? accountIndex.accountIndex : accountIndex;
                const pubkey = staticAccountKeys[accountIndexValue];
                
                if (!pubkey) {
                  throw new Error(`Invalid account index ${accountIndexValue} for instruction ${index}, key ${keyIndex}`);
                }
                
                return {
                  pubkey: pubkey,
                  isSigner: accountIndexValue < header.numRequiredSignatures,
                  isWritable: accountIndexValue < header.numRequiredSignatures - header.numReadonlySignedAccounts || 
                             (accountIndexValue >= header.numRequiredSignatures && 
                              accountIndexValue < staticAccountKeys.length - header.numReadonlyUnsignedAccounts)
                };
              }),
              data: Buffer.isBuffer(instructionData) ? instructionData : Buffer.alloc(0) // Ensure we always have a Buffer
            };
            
            // Add instruction to transaction using proper method
            legacyTransaction.add(convertedInstruction);
            
          } catch (instError) {
            console.error(`Failed to convert instruction ${index}:`, instError);
            throw new Error(`Instruction conversion failed: ${instError.message}`);
          }
        }
        
        console.log('✅ Successfully converted to legacy Transaction');
        console.log('Legacy transaction instructions:', legacyTransaction.instructions.length);
        console.log('Legacy transaction structure:', {
          recentBlockhash: !!legacyTransaction.recentBlockhash,
          feePayer: !!legacyTransaction.feePayer,
          instructions: legacyTransaction.instructions.length,
          signatures: legacyTransaction.signatures.length,
          allInstructionDataAreBuffers: legacyTransaction.instructions.every(inst => Buffer.isBuffer(inst.data))
        });
        
        // Validate the transaction structure
        legacyTransaction.instructions.forEach((inst, idx) => {
          if (!Buffer.isBuffer(inst.data)) {
            console.error(`Instruction ${idx} data is not a Buffer:`, typeof inst.data);
          }
          if (!inst.programId) {
            console.error(`Instruction ${idx} missing programId`);
          }
          if (!Array.isArray(inst.keys)) {
            console.error(`Instruction ${idx} keys is not an array`);
          }
        });
        
        return legacyTransaction;
        
      } else {
        console.error('Message structure:', Object.keys(message));
        console.error('Message properties:', {
          hasHeader: !!message.header,
          hasStaticAccountKeys: !!message.staticAccountKeys,
          hasRecentBlockhash: !!message.recentBlockhash,
          hasInstructions: !!message.instructions,
          hasAddressTableLookups: !!message.addressTableLookups
        });
        throw new Error(`Unsupported message structure. Expected MessageV0-like properties but got: ${Object.keys(message).join(', ')}`);
      }
      
    } catch (conversionError) {
      console.error('❌ Failed to convert to legacy transaction:', conversionError);
      console.error('Conversion error details:', {
        message: conversionError.message,
        stack: conversionError.stack
      });
      throw conversionError;
    }
  };

  const handleBuy = async () => {
    if (!connected || !publicKey) {
      setError('Please connect your wallet first');
      return;
    }

    if (!amountIn || isNaN(parseFloat(amountIn)) || parseFloat(amountIn) <= 0) {
      setError('Please enter a valid amount');
      return;
    }

    if (!signTransaction || !sendTransaction) {
      setError('Wallet does not support transaction signing');
      return;
    }

    setIsLoading(true);
    setError('');
    setSuccess('');
    setTxSignature('');

    // Check wallet support for versioned transactions upfront
    const wallet = window.solana || window.phantom;
    const walletSupportsVersioned = wallet?.isVersionedTransactionSupported || false;
    console.log('Wallet supports versioned transactions:', walletSupportsVersioned);

    try {
      // Step 1: Create unsigned transaction
      setSuccess('Creating transaction...');
      
      // Prepare API payload - using standard format for now
      const apiPayload = {
        chain_id: 100000, // Solana
        token_ca: token.tokenAddress,
        swap_type: 1, // Buy
        amount_in: amountIn,
        user_wallet_address: publicKey.toString(),
      };

      console.log('API request payload:', apiPayload);

      const response = await fetch(`${API_URL}/v1/trade/create_market_order`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(apiPayload),
      });

      const data = await response.json();
      console.log('📨 Full API response:', data);
      
      // Log API response details
      console.log('🔍 API Response Analysis:');
      console.log('  📊 Response Status:', response.status);
      console.log('  📝 Response Keys:', Object.keys(data));
      if (data.data) {
        console.log('  📦 Data Keys:', Object.keys(data.data));
      }

      if (!response.ok) {
        console.error('API response not OK:', data);
        setError(data.message || `API Error ${response.status}: ${response.statusText}`);
        return;
      }
      
      // Handle different possible response structures
      const txHash = data.data?.txHash || data.txHash || data.tx_hash;
      
      // Log transaction hash details
      console.log('🔗 Transaction Hash Analysis:');
      console.log('  📏 Hash Length:', txHash?.length || 0);
      console.log('  🔤 Hash Type:', typeof txHash);
      console.log('  📄 Hash Preview:', txHash?.slice(0, 50) + '...' || 'None');
      
      // Try to detect if this is base64 encoded transaction vs actual hash
      const looksLikeBase64Transaction = txHash && txHash.length > 100 && !txHash.includes('-');
      console.log('  🎯 Appears to be:', looksLikeBase64Transaction ? 'Base64 Transaction' : 'Transaction Hash');
      if (!txHash) {
        console.error('Transaction data not found in response:', data);
        console.error('Expected: data.data.txHash, data.txHash, or data.tx_hash');
        console.error('Available response keys:', Object.keys(data));
        if (data.data) {
          console.error('Available data keys:', Object.keys(data.data));
        }
        
        // If no transaction data, check if backend returned an error message
        if (data.code && data.message) {
          console.error('Backend returned:', data.code, data.message);
          setError(`Backend Error ${data.code}: ${data.message}`);
        } else {
        setError('No transaction data received from server');
        }
        return;
      }

      console.log('Unsigned transaction (base64):', txHash);
      console.log('Transaction length:', txHash.length);

      // Step 2: Decode base64 transaction
      setSuccess('Preparing transaction for signing...');
      let transaction;
      let transactionType;
      
      try {
        console.log('Raw transaction data from API:', txHash);
        console.log('Transaction data length:', txHash.length);
        
        // Use Buffer to properly decode base64 for Solana transactions
        const transactionBuffer = Buffer.from(txHash, 'base64');
        console.log('Successfully decoded base64 to buffer, length:', transactionBuffer.length);
        console.log('First 20 bytes:', Array.from(transactionBuffer.slice(0, 20)));
        
        // Try to determine transaction type and create appropriate object
        let isVersionedTransaction = false;
        let originalVersionedTransaction = null;
        
        try {
          // First try as legacy transaction (since backend creates legacy transactions)
        transaction = Transaction.from(transactionBuffer);
          transactionType = 'Transaction (legacy)';
          isVersionedTransaction = false;
          console.log('✅ Successfully created legacy Transaction object:', transaction);
          
        } catch (legacyError) {
          console.log('Failed to deserialize as legacy transaction, trying versioned format:', legacyError);
          
          try {
            // Fall back to versioned transaction
            originalVersionedTransaction = VersionedTransaction.deserialize(transactionBuffer);
            isVersionedTransaction = true;
            console.log('Successfully created VersionedTransaction object:', originalVersionedTransaction);
            console.log('Transaction version:', originalVersionedTransaction.version);
            console.log('Transaction message type:', originalVersionedTransaction.message.constructor.name);
            
            // Check if wallet supports versioned transactions
            if (!walletSupportsVersioned) {
              console.warn('⚠️ Wallet does not support versioned transactions, attempting conversion...');
              
              try {
                // Convert to legacy transaction
                transaction = convertToLegacyTransaction(originalVersionedTransaction);
                transactionType = 'Transaction (converted from VersionedTransaction)';
                console.log('✅ Successfully converted to legacy Transaction format');
              } catch (conversionError) {
                console.error('❌ Failed to convert to legacy format:', conversionError);
                setError(`Cannot convert transaction for your wallet: ${conversionError.message}. Please try a different wallet like Phantom or Solflare.`);
                return;
              }
            } else {
              // Wallet supports versioned transactions, use as-is
              transaction = originalVersionedTransaction;
              transactionType = 'VersionedTransaction';
            }
            
          } catch (versionedError) {
            console.error('Failed to deserialize as both legacy and versioned transaction:', { legacyError, versionedError });
            setError(`Cannot deserialize transaction: ${legacyError.message}`);
            return;
          }
        }
        
        console.log('Final transaction type:', transactionType);
        console.log('Transaction signatures length:', transaction.signatures?.length || 0);
        
        // For legacy transactions, log instructions count
        if (transaction.instructions) {
        console.log('Transaction instructions length:', transaction.instructions.length);
        }
        
        // Additional debugging for versioned transactions
        if (isVersionedTransaction && originalVersionedTransaction) {
          console.log('Original versioned transaction details:');
          console.log('- Version:', originalVersionedTransaction.version);
          console.log('- Message addressTableLookups:', originalVersionedTransaction.message.addressTableLookups?.length || 0);
          console.log('- Message instructions:', originalVersionedTransaction.message.instructions?.length || 0);
          console.log('- Message accountKeys:', originalVersionedTransaction.message.staticAccountKeys?.length || 0);
        }
        
      } catch (decodeError) {
        console.error('Failed to decode transaction:', decodeError);
        console.error('Error details:', {
          message: decodeError.message,
          stack: decodeError.stack,
          txHashLength: txHash?.length,
          txHashSample: txHash?.substring(0, 100)
        });
        setError(`Failed to decode transaction: ${decodeError.message}`);
        return;
      }

      // Step 3: Send to wallet for signing and submission
      setSuccess('Please approve the transaction in your wallet...');
      
      try {
        console.log('Sending transaction to wallet for signing...');
        console.log('Transaction type:', transactionType);
        console.log('Connection endpoint:', connection.rpcEndpoint);
        
        // Additional debugging: Check wallet capabilities
        console.log('Wallet capabilities:', {
          signTransaction: typeof signTransaction,
          sendTransaction: typeof sendTransaction,
          publicKey: publicKey?.toString(),
          connected: connected,
          walletName: wallet?.name || 'Unknown',
          walletVersion: wallet?.version || 'Unknown'
        });

        // Check if wallet supports the transaction type
        const isVersionedTx = transaction.constructor.name === 'VersionedTransaction';
        const isLegacyTx = transaction.constructor.name === 'Transaction';
        console.log('Transaction compatibility check:', {
          transactionType: transaction.constructor.name,
          isVersionedTx,
          isLegacyTx,
          walletSupportsVersioned: walletSupportsVersioned,
          phantomAvailable: !!(window.solana || window.phantom?.solana),
          phantomSignAndSend: !!(window.solana?.signAndSendTransaction || window.phantom?.solana?.signAndSendTransaction)
        });

        // Detailed transaction inspection before sending
        console.log('🔍 Detailed transaction inspection:');
        console.log('Transaction constructor:', transaction.constructor.name);
        console.log('Transaction properties:', Object.keys(transaction));
        console.log('Transaction structure:', {
          recentBlockhash: transaction.recentBlockhash?.toString().slice(0, 10) + '...',
          feePayer: transaction.feePayer?.toString().slice(0, 10) + '...',
          instructions: transaction.instructions?.length || 0,
          signatures: transaction.signatures?.length || 0
        });

        // Validate each instruction in detail
        if (transaction.instructions) {
          transaction.instructions.forEach((instruction, idx) => {
            console.log(`Instruction ${idx}:`, {
              programId: instruction.programId?.toString().slice(0, 10) + '...',
              keysCount: instruction.keys?.length || 0,
              dataLength: instruction.data?.length || 0,
              dataIsBuffer: Buffer.isBuffer(instruction.data),
              keys: instruction.keys?.map((key, keyIdx) => ({
                pubkey: key.pubkey?.toString().slice(0, 10) + '...',
                isSigner: key.isSigner,
                isWritable: key.isWritable
              }))
            });
            
            // Additional validation
            if (!instruction.programId) {
              console.error(`❌ Instruction ${idx} has no programId`);
            }
            if (!instruction.keys || !Array.isArray(instruction.keys)) {
              console.error(`❌ Instruction ${idx} has invalid keys`);
            }
            if (!Buffer.isBuffer(instruction.data)) {
              console.error(`❌ Instruction ${idx} data is not a Buffer`);
            }
          });
        }

        // Check for common transaction issues
        console.log('🔍 Transaction validation checks:', {
          hasRecentBlockhash: !!transaction.recentBlockhash,
          hasFeePayer: !!transaction.feePayer,
          feePayerMatchesWallet: transaction.feePayer?.toString() === publicKey?.toString(),
          instructionCount: transaction.instructions?.length || 0,
          allInstructionsValid: transaction.instructions?.every(inst => 
            inst.programId && Array.isArray(inst.keys) && Buffer.isBuffer(inst.data)
          ),
          totalAccountKeys: transaction.instructions?.reduce((total, inst) => total + inst.keys.length, 0)
        });

        // Try to serialize the transaction message to validate its structure
        try {
          console.log('🔄 Testing transaction message serialization...');
          const serialized = transaction.serializeMessage();
          console.log('✅ Transaction message serialization successful, length:', serialized.length);
        } catch (serializeError) {
          console.error('❌ Transaction message serialization failed:', serializeError);
          setError(`Invalid transaction structure: ${serializeError.message}`);
          return;
        }

        // Try to simulate the transaction to get more detailed error information
        try {
          console.log('🔄 Simulating transaction to check for potential issues...');
          
          // Log detailed transaction information before simulation
          console.log('📋 Transaction Details Before Simulation:');
          console.log('  🆔 Recent Blockhash:', transaction.recentBlockhash?.slice(0, 10) + '...');
          console.log('  💳 Fee Payer:', transaction.feePayer?.toString());
          console.log('  📝 Instructions Count:', transaction.instructions.length);
          
          // Log each instruction with detailed account information
          transaction.instructions.forEach((instruction, index) => {
            console.log(`  🔧 Instruction [${index}]:`);
            console.log(`    🏛️  Program ID: ${instruction.programId.toString()}`);
            console.log(`    📊 Data Length: ${instruction.data?.length || 0} bytes`);
            console.log(`    🗝️  Accounts (${instruction.keys.length}):`);
            
            instruction.keys.forEach((key, keyIndex) => {
              const pubkeyStr = key.pubkey.toString();
              // Try to identify tick arrays by looking for common patterns
              const mightBeTickArray = pubkeyStr.includes('tick') || 
                                     (pubkeyStr.length === 44 && !key.isSigner); // Most tick arrays are non-signer accounts
              console.log(`      [${keyIndex}] ${pubkeyStr} ${key.isSigner ? '🖊️' : ''}${key.isWritable ? '✏️' : '👁️'} ${mightBeTickArray ? '🎱?' : ''}`);
            });
          });
          
          const simulation = await connection.simulateTransaction(transaction);
          console.log('🔍 Transaction simulation result:', simulation);
          
          if (simulation.value.err) {
            console.error('❌ Transaction simulation failed:', simulation.value.err);
            console.error('Simulation logs:', simulation.value.logs);
            
            // Get more details about the error
            if (simulation.value.err === 'ProgramAccountNotFound') {
              console.error('🔍 Inspecting transaction accounts for missing program accounts:');
              transaction.instructions.forEach((inst, idx) => {
                console.error(`Instruction ${idx} Program ID:`, inst.programId?.toString());
                inst.keys?.forEach((key, keyIdx) => {
                  console.error(`  Account ${keyIdx}: ${key.pubkey?.toString()}, signer: ${key.isSigner}, writable: ${key.isWritable}`);
                });
              });
            }
            
            setError(`Transaction simulation failed: ${JSON.stringify(simulation.value.err)}`);
            return;
          } else {
            console.log('✅ Transaction simulation successful');
            console.log('Simulation logs:', simulation.value.logs);
          }
        } catch (simulationError) {
          console.error('❌ Failed to simulate transaction:', simulationError);
          console.error('Simulation error details:', {
            message: simulationError.message,
            code: simulationError.code,
            data: simulationError.data,
            stack: simulationError.stack
          });
          // Don't return here, still try to send the transaction
        }

        // Try to send transaction
        console.log('📤 Sending transaction to wallet...');
        
        // Check if we can use Phantom's native signAndSendTransaction method
        const phantomProvider = window.solana || window.phantom?.solana;
        let signature;
        
        if (phantomProvider && phantomProvider.signAndSendTransaction) {
          console.log('🔄 Using Phantom\'s native signAndSendTransaction method...');
          try {
            const result = await phantomProvider.signAndSendTransaction(transaction);
            signature = result.signature || result;
            console.log('✅ Transaction submitted with Phantom native method, signature:', signature);
          } catch (phantomError) {
            console.error('❌ Phantom native method failed:', phantomError);
            console.error('Phantom error details:', {
              message: phantomError.message,
              code: phantomError.code,
              data: phantomError.data,
              stack: phantomError.stack
            });
            
            // Fall back to wallet adapter method
            console.log('🔄 Falling back to wallet adapter sendTransaction...');
            signature = await sendTransaction(transaction, connection);
            console.log('✅ Transaction submitted with wallet adapter, signature:', signature);
          }
        } else {
          console.log('🔄 Using wallet adapter sendTransaction method...');
          signature = await sendTransaction(transaction, connection);
          console.log('✅ Transaction submitted with wallet adapter, signature:', signature);
        }
        
        setTxSignature(signature);
        setSuccess(`Transaction submitted successfully! Signature: ${signature}`);
        
        // Step 4: Confirm transaction
        setSuccess('Confirming transaction...');
        const confirmation = await connection.confirmTransaction(signature, 'confirmed');
        
        if (confirmation.value.err) {
          setError(`Transaction failed: ${confirmation.value.err.toString()}`);
        } else {
          setSuccess(`✅ Transaction confirmed! View on Solscan: https://solscan.io/tx/${signature}?cluster=devnet`);
        }
        
      } catch (walletError) {
        console.error('Wallet error:', walletError);
        console.error('Wallet error details:', {
          message: walletError.message,
          code: walletError.code,
          stack: walletError.stack,
          name: walletError.name,
          cause: walletError.cause
        });

        // Try to get more specific error information
        if (walletError.cause) {
          console.error('Wallet error cause:', walletError.cause);
        }
        
        // Check if there's any additional error info in the transaction
        console.log('Transaction state when error occurred:', {
          hasRecentBlockhash: !!transaction.recentBlockhash,
          hasFeePayer: !!transaction.feePayer,
          instructionCount: transaction.instructions?.length || 0,
          signatureCount: transaction.signatures?.length || 0,
          transactionType: transaction.constructor.name
        });
        
        // More specific error handling
        if (walletError.message?.includes('User rejected')) {
          setError('Transaction was rejected by user');
        } else if (walletError.message?.includes('blockhash')) {
          setError('Transaction expired. Please try again.');
        } else if (walletError.message?.includes('insufficient')) {
          setError('Insufficient funds for transaction');
        } else if (walletError.message?.includes('network')) {
          setError('Network error. Please check your connection.');
        } else if (walletError.message === 'Unexpected error') {
          setError('Transaction format issue. This converted transaction may not be compatible with your wallet. Please try using Phantom or Solflare wallet instead.');
        } else {
          setError('Failed to sign/send transaction: ' + (walletError.message || 'Unknown error'));
        }
      }

    } catch (err) {
      console.error('General error:', err);
      setError('Network error: ' + err.message);
    } finally {
      setIsLoading(false);
    }
  };

  const resetForm = () => {
    setAmountIn('');
    setError('');
    setSuccess('');
    setTxSignature('');
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={handleClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Buy {token?.tokenName || 'Token'}</h2>
          <button className="close-button" onClick={handleClose}>×</button>
        </div>

        <div className="modal-body">
          <div className="token-info-section">
            <div className="token-display">
              {token?.tokenIcon ? (
                <img src={token.tokenIcon} alt={token.tokenName} className="token-icon" />
              ) : (
                <div className="token-placeholder">{token?.tokenName?.charAt(0) || '?'}</div>
              )}
              <div>
                <div className="token-name">{token?.tokenName || 'Unknown Token'}</div>
                <div className="token-address">{token?.tokenAddress}</div>
              </div>
            </div>
          </div>

          <div className="input-section">
            <label htmlFor="amount">Amount (SOL)</label>
            <input
              id="amount"
              type="number"
              placeholder="0.0001"
              step="0.0001"
              min="0"
              value={amountIn}
              onChange={(e) => setAmountIn(e.target.value)}
              disabled={isLoading}
            />
            <div className="quick-amounts">
            <button onClick={() => setAmountIn('0.0001')} disabled={isLoading}>0.0001 SOL</button>
              <button onClick={() => setAmountIn('0.001')} disabled={isLoading}>0.001 SOL</button>
              <button onClick={() => setAmountIn('0.01')} disabled={isLoading}>0.01 SOL</button>
              <button onClick={() => setAmountIn('0.1')} disabled={isLoading}>0.1 SOL</button>
              <button onClick={() => setAmountIn('1')} disabled={isLoading}>1 SOL</button>
            </div>
          </div>

          {error && <div className="error-message">{error}</div>}
          {success && <div className="success-message">{success}</div>}
          {txSignature && (
            <div className="tx-signature">
              <strong>Transaction Signature:</strong>
              <div className="signature-text">{txSignature}</div>
              <a 
                href={`https://solscan.io/tx/${txSignature}?cluster=devnet`}
                target="_blank"
                rel="noopener noreferrer"
                className="solscan-link"
              >
                View on Solscan →
              </a>
            </div>
          )}

          <div className="wallet-info">
            {connected ? (
              <div className="wallet-connected">
                ✅ Wallet connected: {publicKey?.toString().slice(0, 8)}...
              </div>
            ) : (
              <div className="wallet-not-connected">
                ❌ Please connect your wallet to continue
              </div>
            )}
          </div>
        </div>

        <div className="modal-footer">
          <button 
            className="cancel-button" 
            onClick={handleClose}
            disabled={isLoading}
          >
            Cancel
          </button>
          <button 
            className="buy-button" 
            onClick={handleBuy}
            disabled={!connected || isLoading || !amountIn}
          >
            {isLoading ? 'Creating Transaction...' : 'Buy Token'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default BuyModal; 