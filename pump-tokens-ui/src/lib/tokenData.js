const aliases = {
  tokenAddress: ['token_address', 'token_ca', 'address'],
  pairAddress: ['pair_address', 'pairAddr'],
  tokenName: ['token_name'], tokenSymbol: ['token_symbol'], tokenIcon: ['token_icon'],
  mktCap: ['mkt_cap', 'fdv'], vol24h: ['vol24H', 'vol_24h', 'vol_24H'],
  txs24h: ['txs24H', 'txs_24h', 'txs_24H'], change24: ['change_24'],
  holdCount: ['hold_count'], launchTime: ['launch_time'], domesticProgress: ['domestic_progress'],
  pumpStatus: ['pump_status'],
};

export function normalizeToken(raw = {}) {
  raw = raw || {};
  const token = { ...raw };
  for (const [key, names] of Object.entries(aliases)) {
    const value = [key, ...names].map((name) => raw[name]).find((v) => v != null && v !== '');
    if (value !== undefined) token[key] = value;
  }
  return token;
}

export function hasTokenName(value) {
  return typeof value === 'string' && value.trim() !== '' && !/^(token|unknown)$/i.test(value.trim());
}

// A partial websocket update or a delayed index lookup must never erase known
// metadata. A numeric zero is real data and is deliberately kept.
export function mergeToken(previous, incoming) {
  const old = normalizeToken(previous);
  const next = normalizeToken(incoming);
  if (old.tokenAddress && next.tokenAddress && old.tokenAddress !== next.tokenAddress) return next;
  const result = { ...old };
  for (const [key, value] of Object.entries(next)) {
    if (value == null || value === '') continue;
    if ((key === 'tokenName' || key === 'tokenSymbol') && !hasTokenName(value) && hasTokenName(old[key])) continue;
    result[key] = value;
  }
  return result;
}

export function tokenDisplayName(token) {
  if (hasTokenName(token?.tokenName)) return token.tokenName;
  if (hasTokenName(token?.tokenSymbol)) return token.tokenSymbol;
  const mint = token?.tokenAddress || '';
  return mint ? `${mint.slice(0, 5)}…${mint.slice(-4)}` : 'Loading…';
}

export function finiteNumber(value) {
  if (value == null || value === '') return null;
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
}
