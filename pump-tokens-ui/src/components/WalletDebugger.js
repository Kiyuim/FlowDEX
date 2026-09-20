import React, { useState, useEffect } from 'react';
import { useWallet, useConnection } from '@solana/wallet-adapter-react';
import { LAMPORTS_PER_SOL } from '@solana/web3.js';

const WalletDebugger = ({ isVisible }) => {
  const { wallet, publicKey, connected, connecting, disconnect, wallets } = useWallet();
  const { connection } = useConnection();
  const [balance, setBalance] = useState(null);
  const [connectionInfo, setConnectionInfo] = useState(null);
  const [debugInfo, setDebugInfo] = useState({});

  useEffect(() => {
    if (connected && publicKey) {
      // Get wallet balance
      connection.getBalance(publicKey).then(balance => {
        setBalance(balance / LAMPORTS_PER_SOL);
      }).catch(err => {
        console.error('Failed to get balance:', err);
        setBalance('Error');
      });

      // Get connection info
      connection.getVersion().then(version => {
        setConnectionInfo(version);
      }).catch(err => {
        console.error('Failed to get connection version:', err);
        setConnectionInfo('Error');
      });

      // Check wallet support for versioned transactions
      const walletObject = window.solana || window.phantom;
      const supportsVersioned = walletObject?.isVersionedTransactionSupported || false;

      // Gather debug info
      setDebugInfo({
        walletName: wallet?.adapter?.name || 'Unknown',
        publicKey: publicKey?.toString() || 'None',
        endpoint: connection.rpcEndpoint,
        connected: connected,
        connecting: connecting,
        readyState: wallet?.adapter?.readyState || 'Unknown',
        autoApprove: wallet?.adapter?.autoApprove || false,
        supportedTransactionVersions: wallet?.adapter?.supportedTransactionVersions || 'Unknown',
        canSignTransaction: !!wallet?.adapter?.signTransaction,
        canSendTransaction: !!wallet?.adapter?.sendTransaction,
        canSignMessage: !!wallet?.adapter?.signMessage,
        canSignAllTransactions: !!wallet?.adapter?.signAllTransactions,
        supportsVersionedTransactions: supportsVersioned,
        walletObject: {
          name: walletObject?.name || 'Unknown',
          version: walletObject?.version || 'Unknown',
          isPhantom: walletObject?.isPhantom || false,
          isSolflare: walletObject?.isSolflare || false,
        }
      });
    }
  }, [connected, publicKey, connection, wallet]);

  if (!isVisible) return null;

  return (
    <div style={{
      position: 'fixed',
      top: '10px',
      right: '10px',
      background: 'rgba(0, 0, 0, 0.9)',
      color: 'white',
      padding: '15px',
      borderRadius: '8px',
      fontSize: '12px',
      fontFamily: 'monospace',
      maxWidth: '450px',
      zIndex: 9999,
      border: '1px solid #333',
      maxHeight: '80vh',
      overflowY: 'auto'
    }}>
      <h3 style={{ margin: '0 0 10px 0', color: '#00ff00' }}>🐛 Wallet Debug Info</h3>
      
      <div style={{ marginBottom: '10px' }}>
        <strong>Connection Status:</strong> {connected ? '✅ Connected' : '❌ Disconnected'}
      </div>
      
      {connected && (
        <>
          <div style={{ marginBottom: '10px' }}>
            <strong>Wallet:</strong> {debugInfo.walletName}
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>Public Key:</strong> {debugInfo.publicKey?.slice(0, 20)}...
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>Balance:</strong> {balance} SOL
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>RPC Endpoint:</strong> {debugInfo.endpoint}
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>Ready State:</strong> {debugInfo.readyState}
          </div>
          
          <div style={{ marginBottom: '10px', padding: '10px', backgroundColor: 'rgba(255, 255, 0, 0.1)', borderRadius: '4px' }}>
            <strong>⚠️ Transaction Support:</strong>
            <ul style={{ margin: '5px 0', paddingLeft: '20px' }}>
              <li>Versioned Transactions: {debugInfo.supportsVersionedTransactions ? '✅ Supported' : '❌ Not Supported'}</li>
              <li>Legacy Transactions: ✅ Supported</li>
            </ul>
            {!debugInfo.supportsVersionedTransactions && (
              <div style={{ color: '#ff9900', marginTop: '5px' }}>
                <strong>⚠️ Warning:</strong> Your wallet may not support some transactions. Consider using Phantom or Solflare.
              </div>
            )}
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>Wallet Object Info:</strong>
            <ul style={{ margin: '5px 0', paddingLeft: '20px' }}>
              <li>Name: {debugInfo.walletObject.name}</li>
              <li>Version: {debugInfo.walletObject.version}</li>
              <li>Is Phantom: {debugInfo.walletObject.isPhantom ? '✅' : '❌'}</li>
              <li>Is Solflare: {debugInfo.walletObject.isSolflare ? '✅' : '❌'}</li>
            </ul>
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>Capabilities:</strong>
            <ul style={{ margin: '5px 0', paddingLeft: '20px' }}>
              <li>Sign Transaction: {debugInfo.canSignTransaction ? '✅' : '❌'}</li>
              <li>Send Transaction: {debugInfo.canSendTransaction ? '✅' : '❌'}</li>
              <li>Sign Message: {debugInfo.canSignMessage ? '✅' : '❌'}</li>
              <li>Sign All Transactions: {debugInfo.canSignAllTransactions ? '✅' : '❌'}</li>
            </ul>
          </div>
          
          <div style={{ marginBottom: '10px' }}>
            <strong>Supported TX Versions:</strong> {
              Array.isArray(debugInfo.supportedTransactionVersions) 
                ? debugInfo.supportedTransactionVersions.join(', ')
                : JSON.stringify(debugInfo.supportedTransactionVersions)
            }
          </div>
          
          {connectionInfo && (
            <div style={{ marginBottom: '10px' }}>
              <strong>Solana Version:</strong> {connectionInfo['solana-core'] || 'Unknown'}
            </div>
          )}
        </>
      )}
      
      <div style={{ marginTop: '10px' }}>
        <strong>Available Wallets:</strong>
        <ul style={{ margin: '5px 0', paddingLeft: '20px' }}>
          {wallets.map(wallet => (
            <li key={wallet.adapter.name}>
              {wallet.adapter.name} - {wallet.adapter.readyState}
            </li>
          ))}
        </ul>
      </div>
      
      <div style={{ marginTop: '10px', fontSize: '10px', color: '#888' }}>
        Press F12 to open console for more details
      </div>
    </div>
  );
};

export default WalletDebugger; 