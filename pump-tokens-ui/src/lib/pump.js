import { PublicKey } from '@solana/web3.js';
import { Buffer } from 'buffer';

export const PUMP_PROGRAM = new PublicKey('6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P');
export const SOL_USD = 150; // nominal SOL price (matches the backend metadata)
export const TOKEN_DECIMALS = 6; // pump.fun standard

export function deriveBondingCurve(mint) {
  const [pda] = PublicKey.findProgramAddressSync(
    [Buffer.from('bonding-curve'), new PublicKey(mint).toBuffer()],
    PUMP_PROGRAM
  );
  return pda;
}

function b64ToBytes(b64) {
  const bin = atob(b64);
  const arr = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i);
  return arr;
}

function u64(view, off) {
  const lo = view.getUint32(off, true);
  const hi = view.getUint32(off + 4, true);
  return hi * 4294967296 + lo; // safe for values < 2^53
}

// BondingCurve account layout (after the 8-byte anchor discriminator):
// virtual_token_reserves u64 [8:16] | virtual_sol_reserves u64 [16:24] |
// real_token_reserves u64 [24:32] | real_sol_reserves u64 [32:40] |
// token_total_supply u64 [40:48] | complete bool [48] | creator 32 [49:81]
// Returns human-scaled reserves (token by 10^6, SOL by 10^9) or null if not a curve.
export function parseBondingCurve(data) {
  if (!data || data.length < 49) return null;
  const v = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const vToken = u64(v, 8);
  const vSol = u64(v, 16);
  const rToken = u64(v, 24);
  const rSol = u64(v, 32);
  const complete = data[48] === 1;
  return {
    virtualToken: vToken / 10 ** TOKEN_DECIMALS,
    virtualSol: vSol / 1e9,
    realToken: rToken / 10 ** TOKEN_DECIMALS,
    realSol: rSol / 1e9,
    complete,
    priceUsd: vToken ? (vSol / 1e9 / (vToken / 10 ** TOKEN_DECIMALS)) * SOL_USD : 0,
  };
}

// pump TradeEvent (log prefix "Program data: vdt/007m…") borsh layout:
// Sign u64 | Mint 32 | SolAmount u64 | TokenAmount u64 | IsBuy bool | User 32 |
// Timestamp u64 | VirtualSolReserves u64 | VirtualTokenReserves u64
export function parsePumpEventLog(log) {
  if (!log || !log.startsWith('Program data: vdt/007m')) return null;
  const b = b64ToBytes(log.slice(14));
  if (b.length < 113) return null;
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength);
  const solAmount = u64(v, 40);
  const tokenAmount = u64(v, 48);
  const isBuy = b[56] === 1;
  const timestamp = u64(v, 89);
  const vSol = u64(v, 97);
  const vToken = u64(v, 105);
  const maker = new PublicKey(b.slice(57, 89)).toBase58();
  const priceSol = vToken ? vSol / 1e9 / (vToken / 10 ** TOKEN_DECIMALS) : 0;
  return {
    isBuy,
    timestamp,
    maker,
    solAmount: solAmount / 1e9,
    tokenAmount: tokenAmount / 10 ** TOKEN_DECIMALS,
    priceUsd: priceSol * SOL_USD,
  };
}
