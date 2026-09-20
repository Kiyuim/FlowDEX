import { mergeToken, normalizeToken, tokenDisplayName } from './tokenData';

test('normalizes protojson volume and transaction count without losing zero', () => {
  expect(normalizeToken({ vol24H: 23.5, txs24H: 3, change24: 0 })).toMatchObject({ vol24h: 23.5, txs24h: 3, change24: 0 });
  expect(normalizeToken({ vol_24h: 0 })).toMatchObject({ vol24h: 0 });
});

test('index placeholders cannot overwrite the Portfolio name', () => {
  const token = mergeToken({ tokenAddress: 'mint', tokenName: 'Real name', tokenSymbol: 'REAL', tokenIcon: 'icon' },
    { tokenAddress: 'mint', tokenName: 'Token', tokenSymbol: '', tokenIcon: '', pairAddress: 'curve', vol24H: 0 });
  expect(token).toMatchObject({ tokenName: 'Real name', tokenSymbol: 'REAL', tokenIcon: 'icon', pairAddress: 'curve', vol24h: 0 });
});

test('partial live updates preserve metadata and cannot carry it to a different mint', () => {
  const old = { tokenAddress: 'a', tokenName: 'Alpha', vol24h: 100 };
  expect(mergeToken(old, { token_address: 'a', mkt_cap: 42 })).toMatchObject({ tokenName: 'Alpha', vol24h: 100, mktCap: 42 });
  expect(mergeToken(old, { tokenAddress: 'b' }).tokenName).toBeUndefined();
  expect(mergeToken(null, null)).toEqual({});
  expect(tokenDisplayName({ tokenAddress: '123456789abcdef', tokenName: 'TOKEN' })).toBe('12345…cdef');
});
