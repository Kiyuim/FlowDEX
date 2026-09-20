export function websocketURL(path, params = {}, kline = false) {
  const local = ['localhost', '127.0.0.1', '0.0.0.0'].includes(window.location.hostname);
  const configured = kline
    ? process.env.REACT_APP_KLINE_WS_URL || process.env.REACT_APP_WS_URL
    : process.env.REACT_APP_WS_URL || process.env.REACT_APP_KLINE_WS_URL;
  const origin = configured || (local ? 'ws://localhost:8086' : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`);
  const url = new URL(path, origin.replace(/^http/, 'ws'));
  url.search = new URLSearchParams(params).toString();
  return url.toString();
}
