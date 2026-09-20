// Tiny localStorage-backed watchlist store with a subscribe/notify so all
// components (cards, detail page, discovery filter) stay in sync.
const KEY = 'fundex_watchlist';

let favs = new Set();
try {
  favs = new Set(JSON.parse(localStorage.getItem(KEY) || '[]'));
} catch {
  favs = new Set();
}

const listeners = new Set();

export function getFavs() {
  return favs;
}

export function toggleFav(mint) {
  if (!mint) return;
  const next = new Set(favs);
  if (next.has(mint)) next.delete(mint);
  else next.add(mint);
  favs = next; // new reference so snapshots change
  try {
    localStorage.setItem(KEY, JSON.stringify([...favs]));
  } catch {}
  listeners.forEach((l) => l());
}

export function subscribe(l) {
  listeners.add(l);
  return () => listeners.delete(l);
}
