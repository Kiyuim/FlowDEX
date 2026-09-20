// SOL/USD used by all client-side price math (curve reserves, event logs).
// Seeded with the same nominal fallback the backend uses on devnet, then
// overwritten with the backend's live CoinGecko value (index_pump returns
// solPriceUsd) so client-computed prices match server-computed ones —
// a hardcoded 150 vs a live ~108 made the header price and the limit
// panel's "now" price disagree by ~40%.
let solUsd = 150;
export const getSolUsd = () => solUsd;
export function setSolUsd(v) {
  const n = Number(v);
  if (Number.isFinite(n) && n > 0) solUsd = n;
}
