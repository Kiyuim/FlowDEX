import React, { useState, useEffect } from 'react';

const MockTokenWebSocket = ({ onNewToken, onTokenUpdate, enabled = false }) => {
  const [intervalId, setIntervalId] = useState(null);
  const [tokenCount, setTokenCount] = useState(0);

  // Mock token names for realistic testing
  const mockTokenNames = [
    'DogeCoin Classic', 'SafeMoon V3', 'ElonMars Token', 'PepeCoin Ultra',
    'ShibaInu Gold', 'Floki Mania', 'BabyDoge Elite', 'MoonRocket Pro',
    'DiamondHands Token', 'ToTheMoon Coin', 'HODL Forever', 'Lambo Dreams',
    'RocketShip Token', 'GemHunter Coin', 'Alpha Wolf Token', 'Sigma Grind'
  ];

  const generateMockToken = () => {
    const randomName = mockTokenNames[Math.floor(Math.random() * mockTokenNames.length)];
    const tokenAddress = 'MOCK' + Math.random().toString(36).substr(2, 40).toUpperCase();
    const pairAddress = 'PAIR' + Math.random().toString(36).substr(2, 40).toUpperCase();
    
    return {
      tokenAddress,
      pairAddress,
      tokenName: randomName,
      tokenSymbol: randomName.split(' ').map(w => w[0]).join('').toUpperCase(),
      tokenIcon: null,
      launchTime: Math.floor(Date.now() / 1000),
      mktCap: Math.random() * 1000000,
      holdCount: Math.floor(Math.random() * 10000),
      pumpStatus: Math.random() > 0.7 ? 2 : 1, // 70% new creation, 30% completing
      change24: (Math.random() - 0.5) * 200, // -100% to +100%
      txs24h: Math.floor(Math.random() * 1000),
    };
  };

  const generateMockUpdate = () => {
    return {
      tokenAddress: 'MOCK' + Math.random().toString(36).substr(2, 40).toUpperCase(),
      pumpStatus: Math.floor(Math.random() * 3) + 1, // 1, 2, or 4
      oldPumpStatus: Math.floor(Math.random() * 3) + 1,
      mktCap: Math.random() * 2000000,
      holdCount: Math.floor(Math.random() * 15000),
    };
  };

  useEffect(() => {
    if (enabled && !intervalId) {
      console.log('🧪 [MOCK WS] Starting mock WebSocket for tokens...');
      
      // Send a new token every 3-8 seconds
      const id = setInterval(() => {
        if (Math.random() > 0.3) { // 70% chance of new token
          const mockToken = generateMockToken();
          console.log('🆕 [MOCK WS] Generating new mock token:', mockToken.tokenName);
          onNewToken && onNewToken(mockToken);
          setTokenCount(prev => prev + 1);
        } else { // 30% chance of update
          const mockUpdate = generateMockUpdate();
          console.log('🔄 [MOCK WS] Generating mock token update');
          onTokenUpdate && onTokenUpdate(mockUpdate);
        }
      }, Math.random() * 5000 + 3000); // 3-8 seconds

      setIntervalId(id);
    } else if (!enabled && intervalId) {
      console.log('🛑 [MOCK WS] Stopping mock WebSocket for tokens...');
      clearInterval(intervalId);
      setIntervalId(null);
    }

    return () => {
      if (intervalId) {
        clearInterval(intervalId);
      }
    };
  }, [enabled, intervalId, onNewToken, onTokenUpdate]);

  if (!enabled) return null;

  return (
    <div style={{
      position: 'fixed',
      top: '10px',
      left: '10px',
      background: 'linear-gradient(45deg, #ff6b35, #f7931e)',
      color: 'white',
      padding: '8px 12px',
      borderRadius: '20px',
      fontSize: '12px',
      fontWeight: 'bold',
      zIndex: 9999,
      boxShadow: '0 4px 12px rgba(255, 107, 53, 0.4)',
      animation: 'mockPulse 2s ease-in-out infinite'
    }}>
      🧪 MOCK MODE: {tokenCount} tokens generated
      <style>{`
        @keyframes mockPulse {
          0%, 100% { transform: scale(1); }
          50% { transform: scale(1.05); }
        }
      `}</style>
    </div>
  );
};

export default MockTokenWebSocket; 