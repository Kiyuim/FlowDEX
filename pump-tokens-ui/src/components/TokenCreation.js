import React, { useState } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import {
  PublicKey,
  Transaction,
  TransactionInstruction,
  SystemProgram,
  Keypair,
  ComputeBudgetProgram,
  LAMPORTS_PER_SOL
} from '@solana/web3.js';
import {
  createInitializeMintInstruction,
  createAssociatedTokenAccountInstruction,
  createMintToInstruction,
  getAssociatedTokenAddress,
  getMintLen,
  TOKEN_PROGRAM_ID,
  TOKEN_2022_PROGRAM_ID,
  ASSOCIATED_TOKEN_PROGRAM_ID,
} from '@solana/spl-token';
import './TokenCreation.css';

const API_BASE_URL = process.env.NODE_ENV === 'development'
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

// PumpMeteora: a Pump.fun-style bonding-curve fork deployed on devnet for
// this course. Not a documented public program — this interface (PDAs,
// discriminator, account order) was reverse-engineered from the reference
// site's own production JS bundle (pump-tokens-ui.vercel.app), not guessed.
const PUMPMETEORA_PROGRAMS = {
  meteora: new PublicKey('AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5'),
  meteorav2: new PublicKey('241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ'),
};
const PUMPMETEORA_LABELS = {
  meteora: 'PumpMeteora',
  meteorav2: 'PumpMeteora V2',
};
const METAPLEX_METADATA_PROGRAM_ID = new PublicKey('metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s');
// Anchor discriminator: sha256("global:create")[:8], shared by both program versions.
const PUMPMETEORA_CREATE_DISCRIMINATOR = Uint8Array.from([94, 139, 158, 50, 69, 95, 8, 45]);

function encodeBorshString(str) {
  const bytes = new TextEncoder().encode(str);
  const buf = new Uint8Array(4 + bytes.length);
  new DataView(buf.buffer).setUint32(0, bytes.length, true);
  buf.set(bytes, 4);
  return buf;
}

function findPda(seeds, programId) {
  return PublicKey.findProgramAddressSync(seeds, programId)[0];
}

// Mirrors the reference site's create-instruction builder exactly (PDA
// seeds, account order, and instruction data layout) — the "config" PDA
// must already be initialized on-chain for this program (the fee recipient
// lives in its data at bytes [72,104)); if it isn't, this throws rather
// than building a transaction that would just fail on-chain.
async function buildPumpMeteoraCreateInstruction(connection, programId, payer, mint, { name, symbol, uri }) {
  const configPda = findPda([new TextEncoder().encode('config')], programId);
  const globalPda = findPda([new TextEncoder().encode('global')], programId);
  const bondingCurvePda = findPda([new TextEncoder().encode('bonding_curve'), mint.toBuffer()], programId);
  const metadataPda = findPda(
    [new TextEncoder().encode('metadata'), METAPLEX_METADATA_PROGRAM_ID.toBuffer(), mint.toBuffer()],
    METAPLEX_METADATA_PROGRAM_ID
  );
  const bondingCurveAta = findPda(
    [bondingCurvePda.toBuffer(), TOKEN_PROGRAM_ID.toBuffer(), mint.toBuffer()],
    ASSOCIATED_TOKEN_PROGRAM_ID
  );

  const configInfo = await connection.getAccountInfo(configPda);
  if (!configInfo || !configInfo.data || configInfo.data.length < 104) {
    throw new Error('This program has no initialized config on-chain — cannot create a token here.');
  }
  const feeRecipient = new PublicKey(configInfo.data.slice(72, 104));

  const nameBytes = encodeBorshString(name);
  const symbolBytes = encodeBorshString(symbol);
  const uriBytes = encodeBorshString(uri);
  const data = new Uint8Array(8 + nameBytes.length + symbolBytes.length + uriBytes.length);
  data.set(PUMPMETEORA_CREATE_DISCRIMINATOR, 0);
  let offset = 8;
  data.set(nameBytes, offset); offset += nameBytes.length;
  data.set(symbolBytes, offset); offset += symbolBytes.length;
  data.set(uriBytes, offset);

  const keys = [
    { pubkey: configPda, isSigner: false, isWritable: true },
    { pubkey: globalPda, isSigner: false, isWritable: true },
    { pubkey: payer, isSigner: true, isWritable: true },
    { pubkey: mint, isSigner: true, isWritable: true },
    { pubkey: bondingCurvePda, isSigner: false, isWritable: true },
    { pubkey: metadataPda, isSigner: false, isWritable: true },
    { pubkey: bondingCurveAta, isSigner: false, isWritable: true },
    { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
    { pubkey: new PublicKey('SysvarRent111111111111111111111111111111111'), isSigner: false, isWritable: false },
    { pubkey: TOKEN_PROGRAM_ID, isSigner: false, isWritable: false },
    { pubkey: ASSOCIATED_TOKEN_PROGRAM_ID, isSigner: false, isWritable: false },
    { pubkey: METAPLEX_METADATA_PROGRAM_ID, isSigner: false, isWritable: false },
    { pubkey: feeRecipient, isSigner: false, isWritable: true },
  ];

  return new TransactionInstruction({ programId, keys, data: Buffer.from(data) });
}

const TokenCreation = () => {
  const { publicKey, connected, signTransaction, sendTransaction } = useWallet();
  const { connection } = useConnection();
  
  // Form state
  const [formData, setFormData] = useState({
    name: '',
    symbol: '',
    decimals: 9,
    supply: 1000000,
    description: '',
    image: '',
    freezeAuthority: true,
    updateAuthority: true
  });
  // 'standard' | 'token2022' | 'meteora' | 'meteorav2'
  const [launchTarget, setLaunchTarget] = useState('standard');
  const useToken2022 = launchTarget === 'token2022';
  const isPumpMeteora = launchTarget === 'meteora' || launchTarget === 'meteorav2';
  
  // UI state
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [txSignature, setTxSignature] = useState('');
  const [tokenMint, setTokenMint] = useState('');

  const handleInputChange = (e) => {
    const { name, value, type, checked } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value
    }));
  };

  const validateForm = () => {
    if (!formData.name.trim()) return 'Token name is required';
    if (!formData.symbol.trim()) return 'Token symbol is required';
    if (formData.symbol.length > 10) return 'Symbol must be 10 characters or less';
    if (formData.decimals < 0 || formData.decimals > 9) return 'Decimals must be between 0-9';
    if (formData.supply <= 0) return 'Supply must be greater than 0';
    
    // Check if the final token amount would exceed JavaScript's safe integer limit
    const finalAmount = formData.supply * Math.pow(10, formData.decimals);
    if (finalAmount > Number.MAX_SAFE_INTEGER) {
      return 'Token supply is too large. Please reduce supply or decimals.';
    }
    
    return null;
  };

  const createToken = async () => {
    if (isLoading) return; // 防止并发
    setIsLoading(true);
    setError('');
    setSuccess('');
    setTxSignature('');
    setTokenMint('');
    console.log('createToken called at', new Date().toISOString());
    try {
      console.log('🚀 Starting token creation...');
      console.log('Form data:', formData);
      console.log('Connected wallet:', publicKey.toString());

      if (isPumpMeteora) {
        await createPumpMeteoraToken();
        return;
      }

      // Generate new mint keypair
      const mintKeypair = Keypair.generate();
      console.log('✅ Generated mint keypair:', mintKeypair.publicKey.toString());

      const tokenProgram = useToken2022 ? TOKEN_2022_PROGRAM_ID : TOKEN_PROGRAM_ID;
      console.log('✅ Using token program:', tokenProgram.toString());

      // Calculate space needed for mint account (no extensions requested,
      // so this is 82 bytes for both Token and Token-2022 — getMintLen
      // computes it correctly either way rather than hardcoding it).
      const mintSpace = getMintLen([]);
      console.log('✅ Mint space calculated:', mintSpace);

      // Calculate rent
      const rentExemptBalance = await connection.getMinimumBalanceForRentExemption(mintSpace);
      console.log('✅ Rent exempt balance:', rentExemptBalance);

      // Get associated token account
      const associatedTokenAccount = await getAssociatedTokenAddress(
        mintKeypair.publicKey,
        publicKey,
        false,
        tokenProgram
      );
      console.log('✅ Associated token account:', associatedTokenAccount.toString());

      const transaction = new Transaction();
      console.log('✅ Transaction created');

      // Create mint account
      try {
        const createAccountIx = SystemProgram.createAccount({
          fromPubkey: publicKey,
          newAccountPubkey: mintKeypair.publicKey,
          space: mintSpace,
          lamports: rentExemptBalance,
          programId: tokenProgram,
        });
        transaction.add(createAccountIx);
        console.log('✅ Added create account instruction');
      } catch (err) {
        console.error('❌ Error creating account instruction:', err);
        throw new Error(`Failed to create account instruction: ${err.message}`);
      }

      // Initialize mint
      try {
        const freezeAuthority = formData.freezeAuthority ? publicKey : null;
        const initMintIx = createInitializeMintInstruction(
          mintKeypair.publicKey,    // mint
          formData.decimals,        // decimals
          publicKey,               // mintAuthority
          freezeAuthority,         // freezeAuthority (can be null)
          tokenProgram
        );
        transaction.add(initMintIx);
        console.log('✅ Added initialize mint instruction with decimals:', formData.decimals);
      } catch (err) {
        console.error('❌ Error creating initialize mint instruction:', err);
        throw new Error(`Failed to create initialize mint instruction: ${err.message}`);
      }

      // Create associated token account
      try {
        const createATAIx = createAssociatedTokenAccountInstruction(
          publicKey,           // payer
          associatedTokenAccount, // associatedToken
          publicKey,           // owner
          mintKeypair.publicKey,  // mint
          tokenProgram
        );
        transaction.add(createATAIx);
        console.log('✅ Added create ATA instruction');
      } catch (err) {
        console.error('❌ Error creating ATA instruction:', err);
        throw new Error(`Failed to create ATA instruction: ${err.message}`);
      }

      // Mint initial supply
      try {
        const mintAmount = formData.supply * Math.pow(10, formData.decimals);
        console.log('💰 Mint amount calculated:', mintAmount);
        
        const mintToIx = createMintToInstruction(
          mintKeypair.publicKey,    // mint
          associatedTokenAccount,   // destination
          publicKey,               // authority (mint authority)
          mintAmount,              // amount
          [],
          tokenProgram
        );
        transaction.add(mintToIx);
        console.log('✅ Added mint to instruction');
      } catch (err) {
        console.error('❌ Error creating mint to instruction:', err);
        throw new Error(`Failed to create mint to instruction: ${err.message}`);
      }

      // Get latest blockhash
      try {
        const { blockhash } = await connection.getLatestBlockhash();
        transaction.recentBlockhash = blockhash;
        transaction.feePayer = publicKey;
        console.log('✅ Set blockhash and fee payer');
      } catch (err) {
        console.error('❌ Error getting blockhash:', err);
        throw new Error(`Failed to get latest blockhash: ${err.message}`);
      }

      // Sign and send transaction
      try {
        console.log('🖊️ Requesting wallet signature...');
        
        // Sign with mint keypair first
        transaction.partialSign(mintKeypair);
        console.log('✅ Partially signed with mint keypair');
        
        // Get wallet signature
        const signedTransaction = await signTransaction(transaction);
        console.log('✅ Transaction signed by wallet');
        console.log('Transaction signatures:', signedTransaction.signatures.map(s => s.publicKey.toString()));
        console.log('Transaction blockhash:', transaction.recentBlockhash);
        
        // Send with sendRawTransaction
        console.log('📡 Sending transaction...');
        const txid = await connection.sendRawTransaction(signedTransaction.serialize());
        await connection.confirmTransaction(txid, 'confirmed');
        console.log('✅ Transaction confirmed!');

        setSuccess('Token created successfully!');
        setTxSignature(txid);
        setTokenMint(mintKeypair.publicKey.toString());

        // Best-effort attribution for the Portfolio tab — token creation is
        // entirely client-side, so the backend has no way to know this
        // happened unless we tell it. Never block the create flow on this.
        fetch(`${API_BASE_URL}/v1/market/record_user_asset`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            chain_id: 100000,
            wallet_address: publicKey.toString(),
            asset_type: 'token',
            asset_name: formData.name,
            asset_symbol: formData.symbol,
            asset_address: mintKeypair.publicKey.toString(),
            decimals: formData.decimals,
            total_supply: String(formData.supply),
          }),
        }).catch(err => console.warn('⚠️ Failed to record token in Portfolio:', err));
      } catch (err) {
        console.error('❌ Error signing/sending transaction:', err);
        throw new Error(`Failed to sign or send transaction: ${err.message}`);
      }

    } catch (err) {
      console.error('Token creation error:', err);
      setError(friendlyTokenCreationError(err));
    } finally {
      setIsLoading(false);
    }
  };

  // Launches a bonding curve on the PumpMeteora program instead of minting
  // a plain SPL token. Decimals/supply are fixed by the program itself (the
  // form fields for those don't apply here) — only name/symbol/image feed
  // into the on-chain metadata. Called from within createToken()'s try
  // block, so its errors propagate to that function's existing catch/finally.
  const createPumpMeteoraToken = async () => {
    const programId = PUMPMETEORA_PROGRAMS[launchTarget];
    const label = PUMPMETEORA_LABELS[launchTarget];
    const mintKeypair = Keypair.generate();
    console.log(`✅ Generated mint keypair for ${label}:`, mintKeypair.publicKey.toString());

    const uri = (formData.image || '').trim() || 'https://ipfs.io/ipfs/bafkreih-placeholder.json';
    const createIx = await buildPumpMeteoraCreateInstruction(connection, programId, publicKey, mintKeypair.publicKey, {
      name: formData.name.trim(),
      symbol: formData.symbol.trim(),
      uri,
    });

    const transaction = new Transaction();
    transaction.add(ComputeBudgetProgram.setComputeUnitLimit({ units: 400000 }));
    transaction.add(createIx);

    const { blockhash } = await connection.getLatestBlockhash();
    transaction.recentBlockhash = blockhash;
    transaction.feePayer = publicKey;
    transaction.partialSign(mintKeypair);

    const signedTransaction = await signTransaction(transaction);
    const txid = await connection.sendRawTransaction(signedTransaction.serialize());
    await connection.confirmTransaction(txid, 'confirmed');

    setSuccess(`${label} token created — it's now a live bonding curve. Trade it from the Sources page.`);
    setTxSignature(txid);
    setTokenMint(mintKeypair.publicKey.toString());

    fetch(`${API_BASE_URL}/v1/market/record_user_asset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        chain_id: 100000,
        wallet_address: publicKey.toString(),
        asset_type: 'token',
        asset_name: formData.name,
        asset_symbol: formData.symbol,
        asset_address: mintKeypair.publicKey.toString(),
      }),
    }).catch(err => console.warn('⚠️ Failed to record token in Portfolio:', err));
  };

  // Solana surfaces insufficient-balance failures as a raw simulation log
  // ("Attempt to debit an account but found no record of a prior credit" for
  // a wallet with 0 SOL, "insufficient lamports" once it has some but not
  // enough) — translate the common case into something a wallet's owner can
  // actually act on instead of a cryptic RPC error string.
  const friendlyTokenCreationError = (err) => {
    const msg = err?.message || '';
    if (
      msg.includes('no record of a prior credit') ||
      msg.includes('insufficient lamports') ||
      msg.includes('insufficient funds')
    ) {
      return 'Insufficient SOL balance. This wallet has no (or not enough) devnet SOL to pay for the transaction — use the Faucet tab to airdrop some, then try again.';
    }
    return `Failed to create token: ${msg}`;
  };

  if (!connected) {
    return (
      <div className="token-creation-container">
        <div className="wallet-not-connected">
          <h2>🔗 Connect Wallet</h2>
          <p>Please connect your wallet to create tokens</p>
        </div>
      </div>
    );
  }

  return (
    <div className="token-creation-container">
      <div className="token-creation-card">
        <div className="card-header">
          <h2>🪙 Create New Token</h2>
          <p>Deploy your own SPL token with advanced features</p>
        </div>

        <div className="form-section">
          <div className="program-selector">
            <div className="tab-navigation">
              <button
                type="button"
                className={`tab-btn ${launchTarget === 'standard' ? 'active' : ''}`}
                onClick={() => setLaunchTarget('standard')}
                disabled={isLoading}
              >
                🔒 Standard SPL
              </button>
              <button
                type="button"
                className={`tab-btn ${launchTarget === 'token2022' ? 'active' : ''}`}
                onClick={() => setLaunchTarget('token2022')}
                disabled={isLoading}
              >
                🆕 Token-2022
              </button>
              <button
                type="button"
                className={`tab-btn ${launchTarget === 'meteora' ? 'active' : ''}`}
                onClick={() => setLaunchTarget('meteora')}
                disabled={isLoading}
              >
                🌊 PumpMeteora
              </button>
              <button
                type="button"
                className={`tab-btn ${launchTarget === 'meteorav2' ? 'active' : ''}`}
                onClick={() => setLaunchTarget('meteorav2')}
                disabled={isLoading}
              >
                🛡️ PumpMeteora V2
              </button>
            </div>
            {launchTarget === 'standard' && (
              <p className="info-text">Classic SPL Token program — works everywhere, no extensions.</p>
            )}
            {launchTarget === 'token2022' && (
              <p className="info-text">Token-2022 (Token Extensions) program — same fields below, minted on the newer token program.</p>
            )}
            {isPumpMeteora && (
              <p className="info-text">
                Launches an on-chain bonding curve on {PUMPMETEORA_LABELS[launchTarget]} (
                {PUMPMETEORA_PROGRAMS[launchTarget].toString().slice(0, 6)}…). Decimals & supply are
                fixed by the program; name, symbol and image URL are used. Buy/sell it from the
                Sources page.
              </p>
            )}
          </div>

          <div className="input-grid">
            <div className="input-group">
              <label>Token Name *</label>
              <input
                type="text"
                name="name"
                value={formData.name}
                onChange={handleInputChange}
                placeholder="My Awesome Token"
                maxLength="32"
                disabled={isLoading}
              />
            </div>

            <div className="input-group">
              <label>Symbol *</label>
              <input
                type="text"
                name="symbol"
                value={formData.symbol}
                onChange={handleInputChange}
                placeholder="MAT"
                maxLength="10"
                disabled={isLoading}
              />
            </div>

            {!isPumpMeteora && (
              <div className="input-group">
                <label>Decimals</label>
                <input
                  type="number"
                  name="decimals"
                  value={formData.decimals}
                  onChange={handleInputChange}
                  min="0"
                  max="9"
                  disabled={isLoading}
                />
              </div>
            )}

            {!isPumpMeteora && (
              <div className="input-group">
                <label>Initial Supply</label>
                <input
                  type="number"
                  name="supply"
                  value={formData.supply}
                  onChange={handleInputChange}
                  min="1"
                  disabled={isLoading}
                />
              </div>
            )}
          </div>

          {isPumpMeteora && (
            <div className="input-group">
              <label>Image URL</label>
              <input
                type="url"
                name="image"
                value={formData.image}
                onChange={handleInputChange}
                placeholder="https://example.com/image.png"
                disabled={isLoading}
              />
            </div>
          )}

          {!isPumpMeteora && (
            <div className="authorities-section">
              <h3>🔐 Token Authorities</h3>
              <div className="authorities-grid">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    name="freezeAuthority"
                    checked={formData.freezeAuthority}
                    onChange={handleInputChange}
                    disabled={isLoading}
                  />
                  <span>🧊 Freeze Authority</span>
                  <small>Ability to freeze token accounts</small>
                </label>
              </div>
            </div>
          )}

          {error && (
            <div className="error-message">
              ❌ {error}
            </div>
          )}

          {success && (
            <div className="success-message">
              ✅ {success}
              {txSignature && (
                <div className="tx-details">
                  <div><strong>Transaction:</strong> 
                    <a 
                      href={`https://explorer.solana.com/tx/${txSignature}?cluster=devnet`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      {txSignature.slice(0, 8)}...{txSignature.slice(-8)}
                    </a>
                  </div>
                  {tokenMint && (
                    <div><strong>Token Mint:</strong> 
                      <a 
                        href={`https://explorer.solana.com/address/${tokenMint}?cluster=devnet`}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {tokenMint.slice(0, 8)}...{tokenMint.slice(-8)}
                      </a>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          <div className="form-actions">
            <button
              className="create-token-btn"
              onClick={createToken}
              disabled={isLoading || !connected}
            >
              {isLoading ? (
                <>
                  <span className="spinner"></span>
                  Creating Token...
                </>
              ) : (
                <>
                  🚀 Create Token
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default TokenCreation; 