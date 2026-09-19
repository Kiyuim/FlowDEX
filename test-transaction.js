const { Connection, Transaction, VersionedTransaction, PublicKey } = require('@solana/web3.js');
const { Buffer } = require('buffer');

// Test transaction deserialization
async function testTransactionDeserialization() {
  console.log('🔍 Testing transaction deserialization...');
  
  const testEndpoint = 'https://devnet.helius-rpc.com/?api-key=2d7580ca-93d2-4404-9316-656c2726a26a';
  const connection = new Connection(testEndpoint, 'confirmed');
  
  // Test if we can connect to the network
  try {
    const version = await connection.getVersion();
    console.log('✅ Connected to Solana network:', version);
  } catch (err) {
    console.error('❌ Failed to connect to network:', err);
    return;
  }
  
  // Test API call to backend
  try {
    const testWallet = 'HN7cABqLq46Es1jh92dQQisAq662SmxELLLsHHe4YWrH'; // Test wallet
    const testTokenAddress = 'So11111111111111111111111111111111111111112'; // Wrapped SOL
    
    console.log('\n🔍 Testing backend API call...');
    console.log('Test wallet:', testWallet);
    console.log('Test token:', testTokenAddress);
    
    // Use global fetch or try to import node-fetch
    let fetchFn;
    try {
      fetchFn = global.fetch || require('node-fetch');
    } catch (e) {
      console.error('❌ fetch not available. Please install node-fetch: npm install node-fetch');
      console.error('Or run this script in a Node.js environment that supports fetch (Node 18+)');
      return;
    }
    
    const response = await fetchFn('http://118.194.235.63:8083/v1/trade/create_market_order', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        chain_id: 100000,
        token_ca: testTokenAddress,
        swap_type: 1,
        amount_in: '0.001',
        user_wallet_address: testWallet,
      }),
    });
    
    if (!response.ok) {
      console.error('❌ API call failed:', response.status, response.statusText);
      const errorText = await response.text();
      console.error('Error response:', errorText);
      return;
    }
    
    const data = await response.json();
    console.log('✅ API response received:', {
      status: response.status,
      dataKeys: Object.keys(data),
      hasData: !!data.data,
      hasTxHash: !!(data.data?.txHash || data.txHash || data.tx_hash)
    });
    
    // Get transaction hash
    const txHash = data.data?.txHash || data.txHash || data.tx_hash;
    if (!txHash) {
      console.error('❌ No transaction hash found in response');
      console.log('Full response:', JSON.stringify(data, null, 2));
      return;
    }
    
    console.log('\n🔍 Testing transaction deserialization...');
    console.log('Transaction hash length:', txHash.length);
    console.log('Transaction hash (first 100 chars):', txHash.substring(0, 100));
    
    // Try to deserialize the transaction
    try {
      const transactionBuffer = Buffer.from(txHash, 'base64');
      console.log('✅ Base64 decoded successfully, buffer length:', transactionBuffer.length);
      
      // Try versioned transaction first
      let transaction;
      let transactionType;
      
      try {
        transaction = VersionedTransaction.deserialize(transactionBuffer);
        transactionType = 'VersionedTransaction';
        console.log('✅ Successfully deserialized as VersionedTransaction');
      } catch (versionedError) {
        console.log('⚠️  Failed to deserialize as VersionedTransaction:', versionedError.message);
        
        try {
          transaction = Transaction.from(transactionBuffer);
          transactionType = 'Transaction';
          console.log('✅ Successfully deserialized as legacy Transaction');
        } catch (legacyError) {
          console.error('❌ Failed to deserialize as both transaction types:');
          console.error('Versioned error:', versionedError.message);
          console.error('Legacy error:', legacyError.message);
          return;
        }
      }
      
      console.log('\n✅ Transaction successfully deserialized!');
      console.log('Transaction type:', transactionType);
      console.log('Signatures length:', transaction.signatures?.length || 0);
      
      if (transaction.instructions) {
        console.log('Instructions count:', transaction.instructions.length);
      }
      
      if (transaction.message) {
        console.log('Message type:', transaction.message.constructor.name);
        if (transaction.message.accountKeys) {
          console.log('Account keys count:', transaction.message.accountKeys.length);
        }
      }
      
      // Test if we can use this transaction with a wallet
      console.log('\n🔍 Testing wallet compatibility...');
      console.log('Transaction is ready for wallet signing');
      
    } catch (decodeError) {
      console.error('❌ Failed to decode base64 transaction:', decodeError);
      console.error('Error details:', {
        message: decodeError.message,
        stack: decodeError.stack
      });
    }
    
  } catch (error) {
    console.error('❌ Test failed:', error);
  }
}

// Run the test
testTransactionDeserialization().catch(console.error); 