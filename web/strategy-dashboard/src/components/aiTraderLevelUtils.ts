import type { AiTraderLevel } from '../api/types';

export interface EnrichedLevel extends AiTraderLevel {
  dist_abs: number;
  dist_bps: number;
  side: 'above' | 'below' | 'at';
}

export function enrichLevels(levels: AiTraderLevel[], refPrice: number): EnrichedLevel[] {
  if (!refPrice || refPrice <= 0) return [];
  return levels
    .filter((l) => l.price > 0)
    .map((l) => {
      const distAbs = refPrice - l.price;
      const distBps = (distAbs / refPrice) * 10000;
      let side: EnrichedLevel['side'] = 'at';
      if (distBps > 2) side = 'below';
      else if (distBps < -2) side = 'above';
      return { ...l, dist_abs: distAbs, dist_bps: distBps, side };
    })
    .sort((a, b) => Math.abs(a.dist_bps) - Math.abs(b.dist_bps));
}

export function pickChartLevels(levels: EnrichedLevel[], refPrice: number, yMin: number, yMax: number): EnrichedLevel[] {
  const inRange = levels.filter((l) => l.price >= yMin && l.price <= yMax);
  if (inRange.length >= 2 && inRange.length <= 8) return inRange;

  const supports = levels.filter((l) => l.price <= refPrice).sort((a, b) => b.price - a.price);
  const resistances = levels.filter((l) => l.price > refPrice).sort((a, b) => a.price - b.price);
  const picked: EnrichedLevel[] = [];
  for (const l of supports.slice(0, 3)) picked.push(l);
  for (const l of resistances.slice(0, 3)) picked.push(l);
  return picked.length > 0 ? picked : levels.slice(0, 6);
}

export function shortLevelSource(source: string): string {
  if (source.startsWith('daily_high')) return 'D-H';
  if (source.startsWith('daily_low')) return 'D-L';
  if (source.startsWith('hourly_high')) return 'H-H';
  if (source.startsWith('hourly_low')) return 'H-L';
  if (source === 'bid_wall') return 'стена bid';
  if (source === 'ask_wall') return 'стена ask';
  if (source.startsWith('footprint')) return 'POC';
  return source.slice(0, 12);
}

export function barPriceRange(bars: { low: number; high: number }[]): { min: number; max: number } | null {
  if (bars.length === 0) return null;
  let min = bars[0].low;
  let max = bars[0].high;
  for (const b of bars) {
    if (b.low < min) min = b.low;
    if (b.high > max) max = b.high;
  }
  return { min, max };
}
