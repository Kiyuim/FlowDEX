import React, { useEffect, useRef, useState } from 'react';
import { createChart } from 'lightweight-charts';
import './TradingViewChart.css';

// API URL configuration - using the same pattern as other components
const API_URL = process.env.NODE_ENV === 'development' 
  ? '' // Use proxy in development
  : '/direct-api'; // Use Nginx proxy in production (via /direct-api)

// Keeps a time-ordered candle array in sync with a single upsert, so the
// click-info popup can look up the previous candle's close for % change.
function upsertCandle(series, candle) {
  const idx = series.findIndex((c) => c.time === candle.time);
  if (idx >= 0) {
    const next = series.slice();
    next[idx] = candle;
    return next;
  }
  return [...series, candle].sort((a, b) => a.time - b.time);
}

const TradingViewChart = ({ token, visible = true, mockMode = false }) => {
  const chartContainerRef = useRef();
  const chartRef = useRef();
  const candlestickSeriesRef = useRef();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [interval, setInterval] = useState('1h');
  const [wsConnection, setWsConnection] = useState(null);
  const mockIntervalRef = useRef(null);
  const candleDataRef = useRef([]); // full series, kept in time order, for prev-close lookup
  const [clickInfo, setClickInfo] = useState(null); // { x, y, candle, prevClose }

  // Initialize chart
  useEffect(() => {
    if (!chartContainerRef.current || !visible) {
      console.log('Chart initialization skipped:', { 
        hasContainer: !!chartContainerRef.current, 
        visible 
      });
      return;
    }

    console.log('Creating TradingView chart...');
    
    try {
      const chart = createChart(chartContainerRef.current, {
        layout: {
          background: { color: '#1a1a1a' },
          textColor: '#d1d4dc',
        },
        grid: {
          vertLines: { color: '#2a2a2a' },
          horzLines: { color: '#2a2a2a' },
        },
        crosshair: {
          mode: 1, // CrosshairMode.Normal
        },
        rightPriceScale: {
          borderColor: '#485c7b',
        },
        timeScale: {
          borderColor: '#485c7b',
          timeVisible: true,
          secondsVisible: false,
        },
        width: chartContainerRef.current.clientWidth,
        height: 400,
      });

      console.log('Chart created successfully:', chart);

      const candlestickSeries = chart.addCandlestickSeries({
        upColor: '#00d4aa',
        downColor: '#ff6838',
        borderVisible: false,
        wickUpColor: '#00d4aa',
        wickDownColor: '#ff6838',
        // Default precision is 2 decimal places (minMove 0.01) — fine for a
        // $1 token, but a bonding-curve token can be priced at $0.000004.
        // Without this every candle rounds to $0.00 (O=H=L=C=0), rendering
        // as an invisible/degenerate line even when the data is real.
        priceFormat: { type: 'price', precision: 9, minMove: 0.000000001 },
      });

      console.log('Candlestick series created successfully:', candlestickSeries);

      chartRef.current = chart;
      candlestickSeriesRef.current = candlestickSeries;

      // GMGN-style click-to-inspect: clicking a candle shows a floating card
      // with its O/H/L/C, volume and % change at the click point.
      chart.subscribeClick((param) => {
        if (!param.point || !param.time || !candlestickSeriesRef.current) {
          setClickInfo(null);
          return;
        }
        const candle = param.seriesData?.get(candlestickSeriesRef.current);
        if (!candle) {
          setClickInfo(null);
          return;
        }
        // lightweight-charts' click event is time-column-based — it fires for
        // any click within a candle's time slot, including the blank space
        // above/below the actual wick. Only show the info card when the click
        // lands within the rendered high-low pixel range (with a small
        // tolerance so hitting a thin wick line isn't overly finicky).
        const highY = candlestickSeriesRef.current.priceToCoordinate(candle.high);
        const lowY = candlestickSeriesRef.current.priceToCoordinate(candle.low);
        if (highY == null || lowY == null) {
          setClickInfo(null);
          return;
        }
        const tolerance = 6;
        if (param.point.y < highY - tolerance || param.point.y > lowY + tolerance) {
          setClickInfo(null);
          return;
        }
        const series = candleDataRef.current;
        const idx = series.findIndex((c) => c.time === param.time);
        const prevClose = idx > 0 ? series[idx - 1].close : null;
        setClickInfo({
          x: param.point.x,
          y: param.point.y,
          time: param.time,
          candle,
          prevClose,
        });
      });

      // Handle resize
      const handleResize = () => {
        if (chartContainerRef.current) {
          chart.applyOptions({
            width: chartContainerRef.current.clientWidth,
          });
        }
      };

      window.addEventListener('resize', handleResize);

      return () => {
        window.removeEventListener('resize', handleResize);
        chart.remove();
      };
    } catch (error) {
      console.error('Error creating TradingView chart:', error);
      setError('Failed to initialize chart: ' + error.message);
    }
  }, [visible]);

  // Mock data generator
  const generateMockKlineData = (basePrice = 0.000005, volatility = 0.1) => {
    const now = Math.floor(Date.now() / 1000);
    
    // Calculate interval seconds
    const intervalMap = {
      '1m': 60,
      '5m': 300,
      '15m': 900,
      '1h': 3600,
      '4h': 14400,
      '1d': 86400
    };
    
    const intervalSeconds = intervalMap[interval] || 3600;
    const candleTime = Math.floor(now / intervalSeconds) * intervalSeconds;
    
    // Generate realistic price movement
    const change = (Math.random() - 0.5) * volatility;
    const open = basePrice * (1 + change);
    const close = open * (1 + (Math.random() - 0.5) * volatility * 0.5);
    const high = Math.max(open, close) * (1 + Math.random() * volatility * 0.3);
    const low = Math.min(open, close) * (1 - Math.random() * volatility * 0.3);
    
    return {
      type: 'kline_update',
      data: {
        pair_address: token?.pairAddress || 'MOCK_PAIR',
        chain_id: 100000,
        interval: interval,
        candle_time: candleTime,
        open: open,
        high: high,
        low: low,
        close: close,
        volume: Math.random() * 10000,
        timestamp: Math.floor(Date.now() / 1000)
      }
    };
  };

  // Mock WebSocket connection
  const startMockWebSocket = () => {
    if (mockIntervalRef.current) {
      clearInterval(mockIntervalRef.current);
    }

    console.log('🧪 Starting mock WebSocket data...');
    let lastPrice = 0.000005; // Starting price
    
    // Send initial data
    setTimeout(() => {
      if (candlestickSeriesRef.current && visible) {
        const mockData = generateMockKlineData(lastPrice);
        console.log('📡 Mock WebSocket data:', mockData);
        
        // Process the mock data through the same pipeline
        const klineData = mockData.data;
        
        const chartData = {
          time: klineData.candle_time,
          open: parseFloat(klineData.open),
          high: parseFloat(klineData.high),
          low: parseFloat(klineData.low),
          close: parseFloat(klineData.close),
        };
        
        console.log('📊 Updating chart with mock data:', chartData);
        
        try {
          candlestickSeriesRef.current.update(chartData);
          candleDataRef.current = upsertCandle(candleDataRef.current, chartData);
          lastPrice = chartData.close; // Update base price for next iteration
        } catch (error) {
          console.warn('Mock data update error:', error);
        }
      }
    }, 1000);
    
    // Continue sending data every 5 seconds
    mockIntervalRef.current = setInterval(() => {
      if (candlestickSeriesRef.current && visible) {
        const mockData = generateMockKlineData(lastPrice);
        console.log('📡 Mock WebSocket data:', mockData);
        
        const klineData = mockData.data;
        const chartData = {
          time: klineData.candle_time,
          open: parseFloat(klineData.open),
          high: parseFloat(klineData.high),
          low: parseFloat(klineData.low),
          close: parseFloat(klineData.close),
        };
        
        console.log('📊 Updating chart with mock data:', chartData);
        
        try {
          candlestickSeriesRef.current.update(chartData);
          candleDataRef.current = upsertCandle(candleDataRef.current, chartData);
          lastPrice = chartData.close;
        } catch (error) {
          console.warn('Mock data update error:', error);
        }
      }
    }, 5000); // Send new data every 5 seconds
  };

  // Stop mock WebSocket
  const stopMockWebSocket = () => {
    if (mockIntervalRef.current) {
      clearInterval(mockIntervalRef.current);
      mockIntervalRef.current = null;
      console.log('🛑 Mock WebSocket stopped');
    }
  };

  // WebSocket connection for real-time updates
  useEffect(() => {
    if (!visible) return;
    
    // Use mock data if mockMode is enabled
    if (mockMode) {
      console.log('🧪 Mock mode enabled, starting mock WebSocket...');
      startMockWebSocket();
      return () => {
        stopMockWebSocket();
      };
    }
    
    if (!token?.pairAddress) return;

    // Guards so the reconnect loop can't outlive this effect.
    let cancelled = false;
    let reconnectTimer = null;
    let activeWs = null;

    const connectWebSocket = () => {
      try {
        // Kline WebSocket, served by the websocket service on /ws/kline (it
        // subscribes to the Redis kline:updates channel that dataflow publishes).
        // Set REACT_APP_KLINE_WS_URL to the websocket service origin, e.g.
        // wss://websocket-production-3acc.up.railway.app
        const KLINE_BASE = process.env.REACT_APP_KLINE_WS_URL
          || (window.location.hostname === 'localhost' ? 'ws://localhost:8085' : `wss://${window.location.hostname}`);
        const wsUrl = `${KLINE_BASE.replace(/\/$/, '')}/ws/kline?pair_address=${token.pairAddress}&chain_id=100000&interval=${interval}`;
        const ws = new WebSocket(wsUrl);
        activeWs = ws;

        ws.onopen = () => {
          console.log('WebSocket connected for kline updates');
          setWsConnection(ws);
        };

        ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            console.log('Received real-time kline update:', data);
            
            // Check if component is still mounted and chart is valid
            if (data.type === 'kline_update' && data.data && candlestickSeriesRef.current && visible) {
              const klineData = data.data;
              
              // Map frontend interval to backend interval format
              const intervalMap = {
                '1m': '1m',
                '5m': '5m', 
                '15m': '15m',
                '1h': '1h',
                '4h': '4h',
                '1d': '1d'
              };
              
              const expectedInterval = intervalMap[interval] || '1h';
              
              // Only process data matching current chart interval
              if (klineData.interval !== expectedInterval) {
                console.log(`Filtering out kline data: received interval "${klineData.interval}", expected "${expectedInterval}"`);
                return;
              }
              
              console.log(`✅ Processing kline data for matching interval: ${klineData.interval}`);
              
              // Parse timestamp properly
              let timestamp = klineData.candle_time || klineData.candleTime || klineData.CandleTime;
              
              // Convert to number if it's a string
              if (typeof timestamp === 'string') {
                timestamp = parseInt(timestamp);
              }
              
              // Convert milliseconds to seconds if needed
              if (timestamp > 1000000000000) {
                timestamp = Math.floor(timestamp / 1000);
              }
              
              // Validate timestamp
              if (!timestamp || timestamp <= 0) {
                console.warn('Invalid timestamp in kline data:', timestamp);
                return;
              }
              
              // Transform the real-time data
              const chartData = {
                time: timestamp,
                open: parseFloat(klineData.open || 0),
                high: parseFloat(klineData.high || 0),
                low: parseFloat(klineData.low || 0),
                close: parseFloat(klineData.close || 0),
              };
              
              // Validate price data
              if (chartData.open <= 0 || chartData.high <= 0 || chartData.low <= 0 || chartData.close <= 0) {
                console.warn('Invalid price data in kline:', chartData);
                return;
              }
              
              console.log('Updating chart with:', chartData);

              // Update the chart with new data, but handle timestamp ordering
              try {
                candlestickSeriesRef.current.update(chartData);
                candleDataRef.current = upsertCandle(candleDataRef.current, chartData);
                console.log('Chart updated successfully with timestamp:', chartData.time);
              } catch (updateError) {
                console.error('Error updating chart:', updateError);
                
                // If we get a timestamp ordering error, skip this update
                if (updateError.message && updateError.message.includes('oldest data')) {
                  console.warn('Timestamp ordering issue - skipping update for timestamp:', chartData.time);
                  console.warn('This usually means the real-time data is older than chart data');
                  // Don't break the flow, just skip this update
                  return;
                }
                
                // If chart is disposed, don't continue processing
                if (updateError.message && updateError.message.includes('disposed')) {
                  console.log('Chart is disposed, closing WebSocket');
                  ws.close();
                  return;
                }
                
                // For other errors, log but continue
                console.warn('Continuing despite chart update error:', updateError.message);
              }
            }
          } catch (error) {
            console.error('Error processing WebSocket message:', error);
            console.error('Raw message data:', event.data);
          }
        };

        ws.onerror = (error) => {
          console.error('WebSocket error:', error);
        };

        ws.onclose = () => {
          console.log('WebSocket connection closed');
          setWsConnection(null);
          // Reconnect after a delay, but only if this effect is still active.
          // The handle is tracked so cleanup can cancel it — otherwise a pending
          // reconnect fires after unmount and creates an orphan socket.
          if (!cancelled) {
            reconnectTimer = setTimeout(connectWebSocket, 5000);
          }
        };

        return ws;
      } catch (error) {
        console.error('Error creating WebSocket connection:', error);
        return null;
      }
    };

    connectWebSocket();

    // Cleanup: stop reconnecting and close whatever socket is current. Closing
    // only on readyState === OPEN would leak sockets still in CONNECTING.
    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (activeWs) {
        activeWs.onclose = null; // don't let our own teardown schedule a reconnect
        activeWs.close();
      }
    };
  }, [token?.pairAddress, visible, interval, mockMode]);

  // NOTE: there used to be a second `useEffect(..., [wsConnection])` here whose
  // cleanup closed the socket. Because onopen calls setWsConnection(ws), that
  // cleanup ran on every successful connect and immediately closed the socket it
  // had just opened — onclose then scheduled a 5s reconnect, producing an endless
  // connect/close cycle in the console. The effect above already owns the
  // socket's whole lifecycle, so no separate unmount cleanup is needed.

  // Fetch kline data
  const fetchKlineData = async (selectedInterval = interval) => {
    if (!token?.pairAddress || !candlestickSeriesRef.current) {
      console.warn('Cannot fetch kline data:', { 
        hasPairAddress: !!token?.pairAddress,
        hasTokenAddress: !!token?.tokenAddress,
        hasCandlestickSeries: !!candlestickSeriesRef.current,
        token 
      });
      return;
    }

    console.log('Fetching kline data for pair:', token.pairAddress);
    setIsLoading(true);
    setError('');

    try {
      const now = Math.floor(Date.now() / 1000);
      // Scale the lookback window with the candle size so e.g. 1d candles show
      // real history instead of always fetching just the last 24h (which for
      // a 1d interval is at most a single candle).
      const intervalSeconds = {
        '1m': 60, '5m': 300, '15m': 900, '1h': 3600, '4h': 14400, '1d': 86400,
      }[selectedInterval] || 3600;
      const limit = 200;
      const lookback = intervalSeconds * limit;
      const fromTimestamp = now - lookback;

      const params = new URLSearchParams({
        chain_id: 100000,
        pair_address: token.pairAddress,
        interval: selectedInterval,
        from_timestamp: fromTimestamp,
        to_timestamp: now,
        limit,
      });

      const response = await fetch(`${API_URL}/v1/market/get_candlestick?${params}`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.message || 'Failed to fetch kline data');
      }

      console.log('Kline API response:', data);

      // Transform the data for TradingView
      const klineData = data.data?.list || data.list || [];
      const chartData = klineData
        .map(kline => {
          const open = parseFloat(kline.open || kline.Open || 0);
          const high = parseFloat(kline.high || kline.High || 0);
          const low = parseFloat(kline.low || kline.Low || 0);
          const close = parseFloat(kline.close || kline.Close || 0);
          const time = parseInt(kline.candle_time || kline.candleTime || kline.CandleTime || 0);

          if (!time || !open || !high || !low || !close) {
            console.warn('Skipping invalid kline data:', kline);
            return null;
          }

          return {
            time: time,
            open: open,
            high: high,
            low: low,
            close: close,
          };
        })
        .filter(item => item !== null)
        .sort((a, b) => a.time - b.time);

      console.log('Transformed chart data:', chartData);

      if (chartData.length > 0) {
        candlestickSeriesRef.current.setData(chartData);
        candleDataRef.current = chartData.slice();
        
        // Add real-time connection status indicator
        const lastDataPoint = chartData[chartData.length - 1];
        console.log('Chart updated with', chartData.length, 'data points');
        console.log('Latest data point:', lastDataPoint);
        console.log('WebSocket status:', wsConnection?.readyState === WebSocket.OPEN ? 'Connected' : 'Disconnected');
      } else {
        console.warn('No valid chart data to display');
        // Clear any candles left over from a previously-viewed token —
        // without this, switching to a token with no data yet still showed
        // the old candles underneath the "no data" placeholder.
        candlestickSeriesRef.current.setData([]);
        candleDataRef.current = [];
        setError('No chart data available for this token');
      }
    } catch (error) {
      console.error('Error fetching kline data:', error);
      if (candlestickSeriesRef.current) {
        candlestickSeriesRef.current.setData([]);
        candleDataRef.current = [];
      }
      setError('Failed to load chart data: ' + error.message);
    } finally {
      setIsLoading(false);
    }
  };

  // Load data when token or interval changes
  useEffect(() => {
    setClickInfo(null);
    if (token?.pairAddress && visible) {
      fetchKlineData(interval);
    }
  }, [token?.pairAddress, interval, visible]);

  // Interval change handler
  const handleIntervalChange = (newInterval) => {
    setInterval(newInterval);
    fetchKlineData(newInterval);
  };

  // Expected state for a token with no candles yet (real or otherwise) — shown
  // inside the chart canvas itself, not as a page-level error banner.
  const isNoDataError = error === 'No chart data available for this token';

  if (!visible) return null;

  return (
    <div className="tradingview-chart">
      <div className="chart-header">
        <div className="chart-title">
          <h3>{token?.tokenName || 'Token'} Price Chart</h3>
          <span className="token-address">Pair: {token?.pairAddress}</span>
        </div>
        
        <div className="chart-controls">
          <div className="interval-selector">
            {['1m', '5m', '15m', '1h', '4h', '1d'].map((int) => (
              <button
                key={int}
                className={`interval-btn ${interval === int ? 'active' : ''}`}
                onClick={() => handleIntervalChange(int)}
                disabled={isLoading}
              >
                {int}
              </button>
            ))}
          </div>
          
          <button 
            className="refresh-btn"
            onClick={() => fetchKlineData(interval)}
            disabled={isLoading}
          >
            {isLoading ? '⟳' : '↻'}
          </button>
          
          {mockMode && (
            <div className="mock-controls">
              <button 
                className="mock-btn"
                onClick={() => {
                  const mockData = generateMockKlineData();
                  console.log('📡 Manual mock data:', mockData);
                  const klineData = mockData.data;
                  const chartData = {
                    time: klineData.candle_time,
                    open: parseFloat(klineData.open),
                    high: parseFloat(klineData.high),
                    low: parseFloat(klineData.low),
                    close: parseFloat(klineData.close),
                  };
                  if (candlestickSeriesRef.current) {
                    candlestickSeriesRef.current.update(chartData);
                    candleDataRef.current = upsertCandle(candleDataRef.current, chartData);
                  }
                }}
              >
                📡 Send Mock Data
              </button>
              <button 
                className="mock-btn"
                onClick={startMockWebSocket}
              >
                ▶️ Start Auto Mock
              </button>
              <button 
                className="mock-btn"
                onClick={stopMockWebSocket}
              >
                ⏹️ Stop Auto Mock
              </button>
            </div>
          )}
        </div>
      </div>

      {error && !isNoDataError && (
        <div className="chart-error">
          <span>⚠️ {error}</span>
          <button onClick={() => fetchKlineData(interval)}>Retry</button>
        </div>
      )}

      {isLoading && (
        <div className="chart-loading">
          <div className="loading-spinner"></div>
          <span>Loading price data...</span>
        </div>
      )}

      <div style={{ position: 'relative' }}>
        <div
          ref={chartContainerRef}
          className="chart-container"
          style={{
            position: 'relative',
            width: '100%',
            height: '400px',
            opacity: isLoading ? 0.6 : 1,
          }}
        />

        {isNoDataError && !isLoading && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 6,
              pointerEvents: 'none',
              color: '#9aa',
              fontSize: 13,
              textAlign: 'center',
              padding: '0 16px',
            }}
          >
            <div style={{ fontSize: 32 }}>🌱</div>
            <div style={{ fontWeight: 600, color: '#d1d4dc' }}>No trades yet</div>
            <div>Be the first to buy this token — the candle chart starts after the first trade.</div>
          </div>
        )}

        {clickInfo && (() => {
          const { candle, prevClose } = clickInfo;
          const changePct = prevClose ? ((candle.close - prevClose) / prevClose) * 100 : null;
          const up = candle.close >= candle.open;
          // Keep the card inside the chart bounds near the click point.
          const left = Math.min(Math.max(clickInfo.x - 90, 4), (chartContainerRef.current?.clientWidth || 400) - 184);
          const top = Math.min(Math.max(clickInfo.y + 12, 4), 400 - 140);
          return (
            <div
              className="candle-info-popup"
              style={{
                position: 'absolute',
                left,
                top,
                width: 180,
                zIndex: 5,
                background: 'rgba(20,20,20,0.95)',
                border: '1px solid #3a3a3a',
                borderRadius: 8,
                padding: '8px 10px',
                fontSize: 12,
                color: '#d1d4dc',
                pointerEvents: 'none',
                boxShadow: '0 4px 12px rgba(0,0,0,0.4)',
              }}
            >
              <div style={{ marginBottom: 4, color: '#9aa', fontSize: 11 }}>
                {new Date(clickInfo.time * 1000).toLocaleString()}
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span>O</span><span>{candle.open}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span>H</span><span>{candle.high}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span>L</span><span>{candle.low}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span>C</span><span>{candle.close}</span>
              </div>
              {changePct != null && (
                <div style={{ marginTop: 4, fontWeight: 600, color: up ? '#00d4aa' : '#ff6838' }}>
                  {changePct >= 0 ? '+' : ''}
                  {changePct.toFixed(2)}%
                </div>
              )}
            </div>
          );
        })()}
      </div>

      {/* Real-time connection status */}
      <div className="connection-status">
        <span className={`status-indicator ${wsConnection?.readyState === WebSocket.OPEN ? 'connected' : 'disconnected'}`}>
          {wsConnection?.readyState === WebSocket.OPEN ? '🟢 Live' : '🔴 Offline'}
        </span>
      </div>
    </div>
  );
};

export default TradingViewChart; 