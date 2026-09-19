import { useEffect, useRef, useState } from 'react';

const useTokenListWebSocket = (onNewToken, onTokenUpdate) => {
  const [connectionStatus, setConnectionStatus] = useState('disconnected');
  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const reconnectCount = useRef(0);

  const connect = () => {
    try {
      console.log('🔌 [TOKEN WS] Connecting to token list WebSocket...');
      
      // Connect to token list WebSocket
      // Use WebSocket service directly on port 8086
      // For localhost access, use localhost:8086
      // For remote access (web3ite.cab), use server IP:8086
      const isLocalhost = window.location.hostname === 'localhost' ||
                         window.location.hostname === '127.0.0.1' ||
                         window.location.hostname === '0.0.0.0';

      // Preferred: full WS base URL from env (e.g. wss://websocket-xxx.up.railway.app).
      // Set REACT_APP_WS_URL in Vercel to the Railway websocket service domain.
      const WS_BASE = process.env.REACT_APP_WS_URL;

      let wsUrl;
      if (WS_BASE) {
        wsUrl = `${WS_BASE.replace(/\/$/, '')}/ws/tokens?chain_id=100000`;
      } else if (isLocalhost) {
        wsUrl = `ws://localhost:8086/ws/tokens?chain_id=100000`;
      } else {
        // Fallback: same-origin wss through a reverse proxy
        wsUrl = `wss://${window.location.hostname}/ws/tokens?chain_id=100000`;
      }
      
      console.log('🔗 [TOKEN WS] WebSocket URL:', wsUrl, '(hostname:', window.location.hostname + ', protocol:', window.location.protocol + ')');
      const ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        console.log('✅ [TOKEN WS] Token list WebSocket connected');
        setConnectionStatus('connected');
        reconnectCount.current = 0;
        
        // Send subscription message for pump tokens
        const subscribeMsg = {
          type: 'subscribe',
          data: {
            chain_id: 100000,
            categories: ['new_creation', 'completing', 'completed']
          }
        };
        ws.send(JSON.stringify(subscribeMsg));
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          console.log('📨 [TOKEN WS] Received message:', data);

          switch (data.type) {
            case 'new_token':
              if (data.data && onNewToken) {
                console.log('🆕 [TOKEN WS] New token created:', data.data.token_name || data.data.tokenName);
                // Convert snake_case to camelCase for frontend
                const tokenData = {
                  tokenAddress: data.data.token_address,
                  pairAddress: data.data.pair_address,
                  tokenName: data.data.token_name,
                  tokenSymbol: data.data.token_symbol,
                  tokenIcon: data.data.token_icon,
                  launchTime: data.data.launch_time,
                  mktCap: data.data.mkt_cap,
                  holdCount: data.data.hold_count,
                  change24: data.data.change_24,
                  txs24h: data.data.txs_24h,
                  pumpStatus: data.data.pump_status,
                  chainId: data.data.chain_id
                };
                onNewToken(tokenData);
              }
              break;
              
            case 'token_status_update':
              if (data.data && onTokenUpdate) {
                console.log('🔄 [TOKEN WS] Token status updated:', data.data.token_name || data.data.tokenName);
                // Convert snake_case to camelCase for frontend
                const updateData = {
                  tokenAddress: data.data.token_address,
                  pairAddress: data.data.pair_address,
                  tokenName: data.data.token_name,
                  tokenSymbol: data.data.token_symbol,
                  tokenIcon: data.data.token_icon,
                  launchTime: data.data.launch_time,
                  mktCap: data.data.mkt_cap,
                  holdCount: data.data.hold_count,
                  change24: data.data.change_24,
                  txs24h: data.data.txs_24h,
                  pumpStatus: data.data.pump_status,
                  chainId: data.data.chain_id
                };
                onTokenUpdate(updateData);
              }
              break;
              
            case 'connection_established':
              console.log('🎯 [TOKEN WS] Connection established:', data.data);
              break;
              
            default:
              console.log('🔍 [TOKEN WS] Unknown message type:', data.type);
          }
        } catch (error) {
          console.error('❌ [TOKEN WS] Error parsing message:', error, event.data);
        }
      };

      ws.onerror = (error) => {
        console.error('❌ [TOKEN WS] WebSocket error:', error);
        setConnectionStatus('error');
      };

      ws.onclose = (event) => {
        console.log('🔌 [TOKEN WS] WebSocket closed:', event.code, event.reason);
        setConnectionStatus('disconnected');
        wsRef.current = null;

        // Reconnect with exponential backoff
        if (!event.wasClean) {
          const delay = Math.min(1000 * Math.pow(2, reconnectCount.current), 30000);
          console.log(`🔄 [TOKEN WS] Reconnecting in ${delay}ms (attempt ${reconnectCount.current + 1})`);
          
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectCount.current++;
            connect();
          }, delay);
        }
      };

      wsRef.current = ws;
    } catch (error) {
      console.error('❌ [TOKEN WS] Error creating WebSocket connection:', error);
      setConnectionStatus('error');
    }
  };

  const disconnect = () => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      console.log('🛑 [TOKEN WS] Manually closing WebSocket connection');
      wsRef.current.close(1000, 'Manual disconnect');
    }
  };

  useEffect(() => {
    connect();

    return () => {
      disconnect();
    };
  }, []);

  return {
    connectionStatus,
    disconnect,
    reconnect: connect
  };
};

export default useTokenListWebSocket; 