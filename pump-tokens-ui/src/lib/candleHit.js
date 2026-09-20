export function hitsCandle({ x, y, centerX, openY, closeY, highY, lowY, barSpacing }) {
  if (![x, y, centerX, openY, closeY, highY, lowY, barSpacing].every(Number.isFinite)) return false;
  const distance = Math.abs(x - centerX);
  const body = distance <= Math.max(1, barSpacing * 0.35)
    && y >= Math.min(openY, closeY) - 1 && y <= Math.max(openY, closeY) + 1;
  const wick = distance <= 2 && y >= Math.min(highY, lowY) - 2 && y <= Math.max(highY, lowY) + 2;
  return body || wick;
}
