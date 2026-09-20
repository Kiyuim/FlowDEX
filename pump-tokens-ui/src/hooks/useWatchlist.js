import { useEffect, useState } from 'react';
import { getFavs, subscribe, toggleFav } from '../lib/watchlist';

export default function useWatchlist() {
  const [favs, setFavs] = useState(getFavs());
  useEffect(() => subscribe(() => setFavs(getFavs())), []);
  return { favs, isFav: (m) => favs.has(m), toggle: toggleFav };
}
