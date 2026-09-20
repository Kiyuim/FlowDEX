import { useEffect, useRef, useState } from 'react';
import { normalizeToken } from '../lib/tokenData';
import { websocketURL } from '../lib/websocket';

export default function useTokenListWebSocket(onNewToken, onTokenUpdate) {
  const [connectionStatus, setConnectionStatus] = useState('disconnected');
  const [enabled, setEnabled] = useState(true);
  const [generation, setGeneration] = useState(0);
  const callbacks = useRef({ onNewToken, onTokenUpdate });
  callbacks.current = { onNewToken, onTokenUpdate };
  useEffect(() => {
    if (!enabled) { setConnectionStatus('disconnected'); return undefined; }
    let stopped = false, socket, timer, attempts = 0;
    const connect = () => {
      if (stopped) return;
      setConnectionStatus('connecting');
      socket = new WebSocket(websocketURL('/ws/tokens', { chain_id: 100000 }));
      socket.onopen = () => {
        attempts = 0;
        setConnectionStatus('connected');
        socket.send(JSON.stringify({ type: 'subscribe', data: { chain_id: 100000, categories: ['new_creation', 'completing', 'completed'] } }));
      };
      socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (!msg.data) return;
          const token = normalizeToken(msg.data);
          if (msg.type === 'new_token') callbacks.current.onNewToken?.(token);
          if (msg.type === 'token_status_update') callbacks.current.onTokenUpdate?.(token);
        } catch (err) { console.error('Invalid token update', err); }
      };
      socket.onerror = () => setConnectionStatus('error');
      socket.onclose = () => {
        if (stopped) return;
        setConnectionStatus('disconnected');
        timer = window.setTimeout(connect, Math.min(1000 * 2 ** attempts++, 30000));
      };
    };
    connect();
    return () => {
      stopped = true;
      window.clearTimeout(timer);
      if (socket) { socket.onclose = null; socket.close(); }
    };
  }, [enabled, generation]);
  return { connectionStatus, disconnect: () => setEnabled(false), reconnect: () => { setEnabled(true); setGeneration((n) => n + 1); } };
}
