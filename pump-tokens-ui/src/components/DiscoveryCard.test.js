import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import DiscoveryCard from './DiscoveryCard';

jest.mock('../hooks/useWatchlist', () => () => ({ isFav: () => false, toggle: jest.fn() }));

test('shows real name, protojson volume, market cap and a zero change', () => {
  render(<MemoryRouter><DiscoveryCard token={{ tokenAddress: 'mint123456', tokenName: 'My token', mktCap: 15000, vol24H: 2500, change24: 0 }} /></MemoryRouter>);
  expect(screen.getByText('My token')).toBeTruthy();
  expect(screen.getByText('$15.0K')).toBeTruthy();
  expect(screen.getByText('$2.5K')).toBeTruthy();
  expect(screen.getByText('+0.0%')).toBeTruthy();
});

test('zero volume is displayed as zero, not missing data', () => {
  render(<MemoryRouter><DiscoveryCard token={{ tokenAddress: 'mint123456', vol24H: 0 }} /></MemoryRouter>);
  expect(screen.getByText('$0.00')).toBeTruthy();
});
