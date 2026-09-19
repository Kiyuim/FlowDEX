import React, { useState } from 'react';
import TradingViewChart from './TradingViewChart';

const MockTradingViewTest = () => {
  const [mockMode, setMockMode] = useState(false);
  
  // Mock token data for testing
  const mockToken = {
    tokenName: 'Test Token (MOCK)',
    pairAddress: 'MOCK_PAIR_ADDRESS_123',
    tokenAddress: 'MOCK_TOKEN_ADDRESS_456'
  };

  return (
    <div style={{ padding: '20px', background: '#0a0a0a', minHeight: '100vh' }}>
      <div style={{ marginBottom: '20px', padding: '15px', background: '#1a1a1a', borderRadius: '8px' }}>
        <h2 style={{ color: '#ffffff', marginBottom: '15px' }}>Mock WebSocket Data Test</h2>
        
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', marginBottom: '15px' }}>
          <button
            onClick={() => setMockMode(!mockMode)}
            style={{
              padding: '10px 20px',
              background: mockMode ? '#00d4aa' : '#333',
              color: mockMode ? '#000' : '#fff',
              border: 'none',
              borderRadius: '5px',
              cursor: 'pointer',
              fontWeight: 'bold'
            }}
          >
            {mockMode ? '🧪 Mock Mode ON' : '🔌 Real Mode'}
          </button>
          
          <span style={{ color: '#888' }}>
            {mockMode ? 'Using simulated WebSocket data' : 'Using real WebSocket connection'}
          </span>
        </div>
        
        <div style={{ color: '#ccc', fontSize: '14px' }}>
          <p><strong>Mock Mode Features:</strong></p>
          <ul>
            <li>🎯 Generates realistic price movements</li>
            <li>⏰ Correct timestamp progression based on selected interval</li>
            <li>🔄 Auto-updates every 5 seconds</li>
            <li>📡 Manual data injection for testing</li>
            <li>📊 Supports all intervals (1m, 5m, 15m, 1h, 4h, 1d)</li>
          </ul>
        </div>
      </div>

      <TradingViewChart 
        token={mockToken}
        visible={true}
        mockMode={mockMode}
      />
      
      <div style={{ marginTop: '20px', padding: '15px', background: '#1a1a1a', borderRadius: '8px' }}>
        <h3 style={{ color: '#ffffff', marginBottom: '10px' }}>Console Output</h3>
        <p style={{ color: '#888', fontSize: '14px' }}>
          Open your browser's developer console (F12) to see detailed logs of mock data generation and chart updates.
        </p>
        
        <div style={{ marginTop: '15px' }}>
          <h4 style={{ color: '#ffffff', marginBottom: '10px' }}>What to look for:</h4>
          <ul style={{ color: '#ccc', fontSize: '14px' }}>
            <li><code style={{ background: '#333', padding: '2px 4px', borderRadius: '3px' }}>🧪 Starting mock WebSocket data...</code></li>
            <li><code style={{ background: '#333', padding: '2px 4px', borderRadius: '3px' }}>📡 Mock WebSocket data: {'{...}'}</code></li>
            <li><code style={{ background: '#333', padding: '2px 4px', borderRadius: '3px' }}>📊 Updating chart with mock data: {'{...}'}</code></li>
            <li><code style={{ background: '#333', padding: '2px 4px', borderRadius: '3px' }}>✅ Processing kline data for matching interval: 1h</code></li>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default MockTradingViewTest; 