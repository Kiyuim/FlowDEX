// Source-aware bonding-curve reader. Reads the token's curve account straight from
// chain for whichever program owns it (pump.fun / PumpMeteora / PumpMeteora V2) and
// returns a live price + reserves. This gives a freshly-created token (no trades yet)
// a real STARTING price, since useBondingCurveTrades has nothing to compute from.
import { PublicKey } from '@solana/web3.js';
import { deriveBondingCurve, parseBondingCurve } from './pump';

const SOL_USD = 150; // nominal SOL price (matches the backend metadata)

// both meteora builds share the same leading BondingCurve layout + "bonding_curve" seed
const METEORA_PROGRAMS = [
  { source: 'PumpMeteora', id: new PublicKey('AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5') },
  { source: 'PumpMeteoraV2', id: new PublicKey('241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ') },
];
const enc = (s) => new TextEncoder().encode(s);
const u64 = (dv, off) => Number(dv.getBigUint64(off, true));

// meteora BondingCurve after the 8-byte disc: token_mint(32) creator(32)
// token_total_supply(8) virtual_sol(8)@80 virtual_token(8)@88 last_virtual_sol(8)
// real_sol(8)@104 real_token(8)@112 is_completed(1)@120
function parseMeteoraCurve(data) {
  if (!data || data.length < 121) return null;
  const dv = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const virtualSol = u64(dv, 80) / 1e9;
  const virtualToken = u64(dv, 88) / 1e6;
  return {
    virtualSol,
    virtualToken,
    realSol: u64(dv, 104) / 1e9,
    realToken: u64(dv, 112) / 1e6,
    complete: data[120] === 1,
    priceUsd: virtualToken ? (virtualSol / virtualToken) * SOL_USD : 0,
  };
}

// Returns { source, priceUsd, virtualSol, virtualToken, realSol, realToken, complete } | null
export async function readCurveState(connection, mint) {
  if (!mint) return null;
  for (const { source, id } of METEORA_PROGRAMS) {
    try {
      const [curve] = PublicKey.findProgramAddressSync([enc('bonding_curve'), new PublicKey(mint).toBuffer()], id);
      const acc = await connection.getAccountInfo(curve);
      if (acc?.data) {
        const p = parseMeteoraCurve(acc.data);
        if (p) return { source, ...p };
      }
    } catch (_) { /* try next */ }
  }
  try {
    const acc = await connection.getAccountInfo(deriveBondingCurve(mint));
    if (acc?.data) {
      const p = parseBondingCurve(acc.data);
      if (p) return { source: 'PumpFun', ...p };
    }
  } catch (_) { /* ignore */ }
  return null;
}
