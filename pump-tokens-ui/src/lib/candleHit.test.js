import { hitsCandle } from './candleHit';
const candle = { centerX: 100, openY: 40, closeY: 60, highY: 20, lowY: 80, barSpacing: 20 };
test('clicks on the body and wick show candle info', () => {
  expect(hitsCandle({ ...candle, x: 105, y: 50 })).toBe(true);
  expect(hitsCandle({ ...candle, x: 100, y: 25 })).toBe(true);
});
test('blank space beside a wick or above a candle does not show info', () => {
  expect(hitsCandle({ ...candle, x: 106, y: 25 })).toBe(false);
  expect(hitsCandle({ ...candle, x: 100, y: 10 })).toBe(false);
  expect(hitsCandle({ ...candle, x: 100, y: 50, centerX: null })).toBe(false);
});
