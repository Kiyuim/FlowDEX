import { useEffect, useRef, useState, useCallback } from 'react';

const useTokenListWebSocket = (onNewToken, onTokenUpdate) => {
  const [connectionStatus, setConnectionStatus] = useState('disconnected');
  const [lastMessage, setLastMessage] = useState(null);
  const [clientCount, setClientCount] = useState(0);
  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const reconnectCount = useRef(0);
  const maxReconnectAttempts = 10;

  const connect = useCallback(() => {
    // Don't connect if already connecting or connected
    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) {
      console.log('🔌 [TOKEN WS] Already connected or connecting, skipping...');
      return;
    }

    try {
      console.log(`🔌 [TOKEN WS] Connecting to token WebSocket (attempt ${reconnectCount.current + 1})...`);
      
      // Clear any existing connection
      if (wsRef.current) {
        wsRef.current.close();
      }
      
      const WS_BASE = process.env.REACT_APP_WS_URL
        || (window.location.hostname === 'localhost' ? 'ws://localhost:8086' : `wss://${window.location.hostname}`);
      const wsUrl = `${WS_BASE.replace(/\/$/, '')}/ws/tokens?chain_id=100000`;
      console.log(`🔗 [TOKEN WS] WebSocket URL: ${wsUrl}`);
      
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      // Connection opened
      ws.onopen = () => {
        console.log('✅ [TOKEN WS] WebSocket connection established');
        setConnectionStatus('connected');
        reconnectCount.current = 0;
        
        // Send subscription message immediately
        const subscribeMsg = {
          type: 'subscribe',
          data: {
            chain_id: 100000,
            categories: ['new_creation', 'completing', 'completed']
          }
        };
        
        try {
          ws.send(JSON.stringify(subscribeMsg));
          console.log('📨 [TOKEN WS] Subscription message sent:', subscribeMsg);
        } catch (error) {
          console.error('❌ [TOKEN WS] Failed to send subscription:', error);
        }
      };

      // Message received
      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          console.log('📨 [TOKEN WS] Message received:', data);
          setLastMessage(data);

          switch (data.type) {
            case 'new_token':
              if (data.data && onNewToken) {
                console.log('🆕 [TOKEN WS] Processing new token:', data.data.token_name || data.data.token_symbol);
                onNewToken({
                  tokenAddress: data.data.token_address,
                  tokenName: data.data.token_name,
                  tokenSymbol: data.data.token_symbol,
                  tokenIcon: data.data.token_icon,
                  launchTime: data.data.launch_time,
                  mktCap: data.data.mkt_cap,
                  holdCount: data.data.hold_count,
                  pairAddress: data.data.pair_address,
                  pumpStatus: data.data.pump_status,
                  twitterUsername: data.data.twitter_username,
                  telegram: data.data.telegram
                });
              }
              break;
              
            case 'token_status_update':
              if (data.data && onTokenUpdate) {
                console.log('🔄 [TOKEN WS] Processing token update:', data.data.token_name);
                onTokenUpdate(data.data);
              }
              break;
              
            case 'connection_established':
              console.log('🎯 [TOKEN WS] Connection confirmed:', data.data);
              setClientCount(data.data.client_count || 1);
              break;
              
            default:
              console.log('🔍 [TOKEN WS] Unknown message type:', data.type, data);
          }
        } catch (error) {
          console.error('❌ [TOKEN WS] Error parsing message:', error, 'Raw data:', event.data);
        }
      };

      // Connection error
      ws.onerror = (error) => {
        console.error('❌ [TOKEN WS] WebSocket error:', error);
        setConnectionStatus('error');
      };

      // Connection closed
      ws.onclose = (event) => {
        console.log(`🔌 [TOKEN WS] Connection closed:`, {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean
        });
        
        setConnectionStatus('disconnected');
        wsRef.current = null;

        // Auto-reconnect if not a clean close and under attempt limit
        if (!event.wasClean && reconnectCount.current < maxReconnectAttempts) {
          const delay = Math.min(1000 * Math.pow(2, reconnectCount.current), 30000);
          console.log(`🔄 [TOKEN WS] Reconnecting in ${delay}ms (attempt ${reconnectCount.current + 1}/${maxReconnectAttempts})`);
          
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectCount.current++;
            connect();
          }, delay);
        } else if (reconnectCount.current >= maxReconnectAttempts) {
          console.error(`❌ [TOKEN WS] Max reconnection attempts (${maxReconnectAttempts}) reached`);
          setConnectionStatus('failed');
        }
      };

    } catch (error) {
      console.error('❌ [TOKEN WS] Error creating WebSocket:', error);
      setConnectionStatus('error');
    }
  }, [onNewToken, onTokenUpdate]);

  const disconnect = useCallback(() => {
    console.log('🛑 [TOKEN WS] Manual disconnect requested');
    
    // Clear reconnection timeout
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    // Close WebSocket connection
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.close(1000, 'Manual disconnect');
    }
    
    wsRef.current = null;
    setConnectionStatus('disconnected');
  }, []);

  const forceReconnect = useCallback(() => {
    console.log('🔄 [TOKEN WS] Force reconnect requested');
    reconnectCount.current = 0;
    disconnect();
    setTimeout(connect, 1000);
  }, [connect, disconnect]);

  // Auto-connect on mount
  useEffect(() => {
    connect();
    
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    connectionStatus,
    lastMessage,
    clientCount,
    disconnect,
    reconnect: forceReconnect,
    isConnected: connectionStatus === 'connected',
    isConnecting: connectionStatus === 'connecting',
    isError: connectionStatus === 'error' || connectionStatus === 'failed'
  };
};

export default useTokenListWebSocket; 