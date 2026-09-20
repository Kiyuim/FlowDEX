import React, { useEffect, useRef, useState } from 'react';
import { createChart } from 'lightweight-charts';
import './TradingViewChart.css';
import { websocketURL } from '../lib/websocket';
import { tokenDisplayName } from '../lib/tokenData';
import { hitsCandle } from '../lib/candleHit';

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

// lightweight-charts uses Unix seconds for intraday data and a BusinessDay
// object for daily/weekly data. Keep both paths in the viewer's local zone so
// changing the interval cannot produce an offset or an invalid x-axis.
function chartTimeToDate(time) {
  if (typeof time === 'number') return new Date(time * 1000);
  if (time && typeof time === 'object' && time.year && time.month && time.day) {
    return new Date(time.year, time.month - 1, time.day);
  }
  return new Date(NaN);
}

// Fills any missing candle gaps between consecutive candles and up to the current bucket.
// Flat continuation candles have open = high = low = close = previous close, and volume = 0.
function fillCandleGaps(candles, intervalSeconds, maxBars = 300) {
  if (!candles || candles.length === 0) return [];
  const sorted = [...candles].sort((a, b) => a.time - b.time);
  const now = Math.floor(Date.now() / 1000);
  const currentBucket = Math.floor(now / intervalSeconds) * intervalSeconds;

  const filled = [];
  let prev = null;

  for (const c of sorted) {
    if (prev) {
      let gapTime = prev.time + intervalSeconds;
      const gapCount = Math.floor((c.time - gapTime) / intervalSeconds);
      if (gapCount > 0 && gapCount <= maxBars) {
        while (gapTime < c.time) {
          filled.push({
            time: gapTime,
            open: prev.close,
            high: prev.close,
            low: prev.close,
            close: prev.close,
            volume: 0,
          });
          gapTime += intervalSeconds;
        }
      }
    }
    filled.push(c);
    prev = c;
  }

  // Extend flat candles up to the current time bucket so K-lines never have discontinuous gaps
  if (prev && prev.time < currentBucket) {
    let gapTime = prev.time + intervalSeconds;
    const gapCount = Math.floor((currentBucket - gapTime) / intervalSeconds);
    if (gapCount >= 0 && gapCount <= maxBars) {
      while (gapTime <= currentBucket) {
        filled.push({
          time: gapTime,
          open: prev.close,
          high: prev.close,
          low: prev.close,
          close: prev.close,
          volume: 0,
        });
        gapTime += intervalSeconds;
      }
    }
  }

  return filled.slice(-maxBars);
}

// Sequentially updates candlestickSeries so no intermediate or gap-filled candles are skipped,
// while preserving lightweight-charts zoom/pan state and animating wicks smoothly.
function applyCandleUpdates(candlestickSeries, oldData, newData) {
  if (!candlestickSeries || !newData || newData.length === 0) return;
  if (!oldData || oldData.length === 0) {
    candlestickSeries.setData(newData);
    return;
  }
  const oldLastTime = oldData[oldData.length - 1].time;
  const newFirstTime = newData[0].time;

  // If time jumped backwards or interval changed, reload with setData
  if (newFirstTime > oldLastTime || newData.length < oldData.length / 2) {
    candlestickSeries.setData(newData);
    return;
  }

  const pendingBars = newData.filter((b) => b.time >= oldLastTime);
  if (pendingBars.length === 0) {
    candlestickSeries.setData(newData);
    return;
  }

  try {
    for (const bar of pendingBars) {
      candlestickSeries.update(bar);
    }
  } catch (err) {
    console.warn('applyCandleUpdates fallback to setData:', err);
    candlestickSeries.setData(newData);
  }
}

function mergeLiveTradeCandles(base, trades, interval) {
  const intervalSeconds = {
    '1m': 60, '5m': 300, '15m': 900, '1h': 3600, '4h': 14400, '1d': 86400,
  }[interval] || 3600;
  const merged = (base || []).map((c) => ({ ...c }));

  // Sort trades chronologically (oldest to newest) to build accurate OHLC bars
  const sortedTrades = (trades || [])
    .filter((t) => {
      const time = Number(t.time);
      const price = Number(t.priceUsd);
      return Number.isFinite(time) && time > 0 && Number.isFinite(price) && price > 0;
    })
    .sort((a, b) => Number(a.time) - Number(b.time));

  const grouped = new Map();
  for (const trade of sortedTrades) {
    const time = Number(trade.time);
    const price = Number(trade.priceUsd);
    const bucket = Math.floor(time / intervalSeconds) * intervalSeconds;
    const vol = Number(trade.solAmount || 0) * (trade.solPriceUsd || 150);
    const row = grouped.get(bucket);
    if (!row) {
      grouped.set(bucket, {
        time: bucket,
        open: price,
        high: price,
        low: price,
        close: price,
        volume: vol,
      });
    } else {
      row.high = Math.max(row.high, price);
      row.low = Math.min(row.low, price);
      row.close = price;
      row.volume = (row.volume || 0) + vol;
    }
  }

  for (const [bucket, incoming] of grouped) {
    const idx = merged.findIndex((c) => c.time === bucket);
    if (idx < 0) {
      merged.push(incoming);
    } else {
      const current = merged[idx];
      merged[idx] = {
        ...current,
        open: current.open > 0 ? current.open : incoming.open,
        high: Math.max(current.high || 0, incoming.high),
        low: current.low > 0 ? Math.min(current.low, incoming.low) : incoming.low,
        close: incoming.close,
        volume: (current.volume || 0) + (incoming.volume || 0),
      };
    }
  }
  const sortedMerged = merged.sort((a, b) => a.time - b.time);
  return fillCandleGaps(sortedMerged, intervalSeconds);
}

const TradingViewChart = ({ token, liveTrades = [], visible = true, mockMode = false, refreshKey = 0, onCandleStats }) => {
  const chartContainerRef = useRef();
  const chartRef = useRef();
  const candlestickSeriesRef = useRef();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [interval, setChartInterval] = useState('1m');
  const fetchGeneration = useRef(0);
  const pendingFetch = useRef(null);
  const [wsConnection, setWsConnection] = useState(null);
  const mockIntervalRef = useRef(null);
  const candleDataRef = useRef([]); // full series, kept in time order, for prev-close lookup
  const [clickInfo, setClickInfo] = useState(null); // { x, y, candle, prevClose }
  const liveTradesRef = useRef(liveTrades);

  const notifyCandleStats = (candles) => {
    if (!candles || candles.length === 0 || !onCandleStats) return;
    const last = candles[candles.length - 1];
    const first = candles[0];
    const volume = candles.reduce((sum, c) => sum + (Number(c.volume) || 0), 0);
    const change = first.open > 0 ? ((last.close - first.open) / first.open) * 100 : 0;
    onCandleStats({
      price: last.close,
      change,
      volume,
      high: Math.max(...candles.map((c) => c.high || c.close)),
      low: Math.min(...candles.map((c) => c.low || c.close)),
    });
  };

  // The direct on-chain trade hook can be newer than the Railway indexer. Fold
  // those trades into the current candle so the chart does not wait for the
  // consumer to catch up before showing the last few minutes.
  useEffect(() => {
    // Save the data even if the chart has not finished mounting yet. The
    // chain hook can resolve before createChart; returning before this line
    // left the fetch path with an empty ref forever, so Recent trades updated
    // while the chart remained stuck at the last indexed candle.
    liveTradesRef.current = liveTrades || [];
    if (!candlestickSeriesRef.current || !liveTrades?.length) return;
    const merged = mergeLiveTradeCandles(candleDataRef.current, liveTrades, interval);
    if (!merged.length) return;
    applyCandleUpdates(candlestickSeriesRef.current, candleDataRef.current, merged);
    candleDataRef.current = merged;
    notifyCandleStats(merged);
  }, [liveTrades, interval]);

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
        // lightweight-charts renders unix timestamps in UTC by default, so a
        // trade made at 18:53 local (UTC+8) showed up under ~10:53 on the
        // axis. Format both the axis ticks and the crosshair in the
        // viewer's local zone instead of shifting the data (which would
        // break the click-info card's own time and the WS upserts).
        localization: {
          timeFormatter: (t) => chartTimeToDate(t).toLocaleString(),
        },
        timeScale: {
          borderColor: '#485c7b',
          timeVisible: true,
          secondsVisible: false,
          tickMarkFormatter: (t, tickType) => {
            const d = chartTimeToDate(t);
            // tickType: 0 year, 1 month, 2 day-of-month, 3 time, 4 time+seconds
            if (tickType === 0) return String(d.getFullYear());
            if (tickType === 1) return d.toLocaleDateString(undefined, { month: 'short' });
            if (tickType === 2) return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
            return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hour12: false });
          },
        },
        width: chartContainerRef.current.clientWidth,
        height: chartContainerRef.current.clientHeight || 400,
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
        if (!hitsCandle({
          x: param.point.x, y: param.point.y,
          centerX: chart.timeScale().timeToCoordinate(param.time),
          openY: candlestickSeriesRef.current.priceToCoordinate(candle.open),
          closeY: candlestickSeriesRef.current.priceToCoordinate(candle.close),
          highY: candlestickSeriesRef.current.priceToCoordinate(candle.high),
          lowY: candlestickSeriesRef.current.priceToCoordinate(candle.low),
          barSpacing: chart.timeScale().options().barSpacing,
        })) {
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
            height: chartContainerRef.current.clientHeight || 400,
          });
        }
      };

      window.addEventListener('resize', handleResize);
      const resizeObserver = typeof ResizeObserver !== 'undefined'
        ? new ResizeObserver(handleResize)
        : null;
      resizeObserver?.observe(chartContainerRef.current);

      return () => {
        window.removeEventListener('resize', handleResize);
        resizeObserver?.disconnect();
        chart.remove();
        chartRef.current = null;
        candlestickSeriesRef.current = null;
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
                setError('');
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
                setError('');
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
        const wsUrl = websocketURL('/ws/kline', { pair_address: token.pairAddress, chain_id: 100000, interval }, true);
        const ws = new WebSocket(wsUrl);
        activeWs = ws;

        ws.onopen = () => {
          console.log('WebSocket connected for kline updates');
          setWsConnection(ws);
          // Pull the current candle immediately after reconnecting. This
          // closes the gap while Redis/WebSocket delivery is recovering.
          fetchKlineData(interval, true);
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
                const intervalSeconds = {
                  '1m': 60, '5m': 300, '15m': 900, '1h': 3600, '4h': 14400, '1d': 86400,
                }[interval] || 3600;
                const nextCandles = upsertCandle(candleDataRef.current, chartData);
                const filled = fillCandleGaps(nextCandles, intervalSeconds);
                applyCandleUpdates(candlestickSeriesRef.current, candleDataRef.current, filled);
                candleDataRef.current = filled;
                notifyCandleStats(filled);
                setError('');
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
  const fetchKlineData = async (selectedInterval = interval, background = false) => {
    // Never skip a poll because an older request is still "pending": one
    // hung response would then suppress every later poll and freeze the
    // chart at its last load. Abort the stale request and go again; the
    // generation check below discards whatever the old one returns.
    const generation = ++fetchGeneration.current;
    pendingFetch.current?.abort();
    const controller = new AbortController();
    pendingFetch.current = controller;
    const timeout = window.setTimeout(() => controller.abort(), 8000);
    if (!token?.pairAddress || !candlestickSeriesRef.current) {
      console.warn('Cannot fetch kline data:', { 
        hasPairAddress: !!token?.pairAddress,
        hasTokenAddress: !!token?.tokenAddress,
        hasCandlestickSeries: !!candlestickSeriesRef.current,
        token 
      });
      pendingFetch.current = null;
      window.clearTimeout(timeout);
      return;
    }

    console.log('Fetching kline data for pair:', token.pairAddress);
    if (!background) setIsLoading(true);
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
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
        },
      });

      const data = await response.json();
      if (generation !== fetchGeneration.current || !candlestickSeriesRef.current) return;

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

      const mergedChartData = mergeLiveTradeCandles(chartData, liveTradesRef.current, selectedInterval);

      if (mergedChartData.length > 0) {
        if (!background || !candleDataRef.current.length) {
          candlestickSeriesRef.current.setData(mergedChartData);
          candleDataRef.current = mergedChartData;
        } else {
          applyCandleUpdates(candlestickSeriesRef.current, candleDataRef.current, mergedChartData);
          candleDataRef.current = mergedChartData;
        }
        setError('');
        notifyCandleStats(mergedChartData);
        
        // Add real-time connection status indicator
        const lastDataPoint = mergedChartData[mergedChartData.length - 1];
        console.log('Chart updated with', mergedChartData.length, 'data points');
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
      if (error.name === 'AbortError' || generation !== fetchGeneration.current) return;
      console.error('Error fetching kline data:', error);
      if (background) return;
      if (candlestickSeriesRef.current) {
        candlestickSeriesRef.current.setData([]);
        candleDataRef.current = [];
      }
      setError('Failed to load chart data: ' + error.message);
    } finally {
      window.clearTimeout(timeout);
      if (generation === fetchGeneration.current) { pendingFetch.current = null; setIsLoading(false); }
    }
  };

  // Load data when token or interval changes
  useEffect(() => {
    setClickInfo(null);
    if (token?.pairAddress && visible) {
      fetchKlineData(interval);
      // WebSocket is the low-latency path; keep a 15s polling safety net so
      // a dropped Redis subscription never leaves the chart visibly stale.
      const timer = window.setInterval(() => fetchKlineData(interval, true), 5000);
      return () => { window.clearInterval(timer); ++fetchGeneration.current; pendingFetch.current?.abort(); pendingFetch.current = null; };
    }
  }, [token?.pairAddress, interval, visible, refreshKey]);

  // Interval change handler
  const handleIntervalChange = (newInterval) => {
    setChartInterval(newInterval);
  };

  // Expected state for a token with no candles yet (real or otherwise) — shown
  // inside the chart canvas itself, not as a page-level error banner.
  const isNoDataError = error === 'No chart data available for this token';

  if (!visible) return null;

  return (
    <div className="tradingview-chart">
      <div className="chart-header">
        <div className="chart-title">
          <h3>{tokenDisplayName(token)} Price Chart</h3>
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
                setError('');
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
