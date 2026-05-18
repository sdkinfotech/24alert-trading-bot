import { useMemo } from 'react';
import type { AiTraderFeatures, AiTraderFootprintColumn, AiTraderMarketContext } from '../api/types';
import type { Lang } from '../i18n';
import { formatNumber } from '../format';

interface Props {
  mc?: AiTraderMarketContext;
  features?: AiTraderFeatures;
  ticker?: string;
  lang: Lang;
}

interface PriceRow {
  price: number;
  bidQty: number;
  askQty: number;
  isBestBid: boolean;
  isBestAsk: boolean;
  isLast: boolean;
}

const LADDER_TICKS_EACH_SIDE = 56;
const MAX_LADDER_ROWS = 160;
const ROW_HEIGHT = 22;
const TAPE_VIEWBOX_WIDTH = 260;

function isBuy(dir: string): boolean {
  return dir.toLowerCase().includes('buy');
}

function decimalsFromTick(tick: number): number {
  if (!Number.isFinite(tick) || tick <= 0) return 2;
  const s = tick.toString();
  if (!s.includes('.')) return 0;
  return Math.min(6, s.split('.')[1]?.length ?? 2);
}

function normalizePrice(price: number, tick: number): number {
  const decimals = decimalsFromTick(tick);
  return Number((Math.round(price / tick) * tick).toFixed(decimals));
}

function priceKey(price: number, tick: number): string {
  return normalizePrice(price, tick).toFixed(decimalsFromTick(tick));
}

function priceLabel(price: number, lang: Lang, tick: number): string {
  return formatNumber(price, lang, decimalsFromTick(tick));
}

function clamp(n: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, n));
}

function fmtVol(n: number, lang: Lang): string {
  const sign = n < 0 ? '-' : '';
  const abs = Math.abs(n);
  if (abs >= 1000) return `${sign}${formatNumber(abs / 1000, lang, 1)}k`;
  return `${sign}${abs}`;
}

function minuteCellKey(col: AiTraderFootprintColumn, price: number, tick: number): string {
  return `${col.time}:${priceKey(price, tick)}`;
}

function bubbleRadius(qty: number): number {
  return clamp(Math.sqrt(Math.max(1, qty)) * 2.15, 7, 22);
}

function buildScalperPriceRows(
  dom: AiTraderMarketContext['dom_book'] | undefined,
  features: AiTraderFeatures | undefined,
  mc: AiTraderMarketContext | undefined,
): PriceRow[] {
  const tick = dom?.tick_size && dom.tick_size > 0 ? dom.tick_size : 0.01;
  const bestBid = dom?.best_bid ?? features?.best_bid ?? 0;
  const bestAsk = dom?.best_ask ?? features?.best_ask ?? 0;
  const last = mc?.tape_stats?.last_price ?? features?.mid ?? bestBid;
  const mid = bestBid > 0 && bestAsk > 0 ? (bestBid + bestAsk) / 2 : last;

  if (!mid || !Number.isFinite(mid)) return [];

  const bidMap = new Map<string, number>();
  const askMap = new Map<string, number>();
  for (const b of dom?.bids ?? []) bidMap.set(priceKey(b.price, tick), b.quantity);
  for (const a of dom?.asks ?? []) askMap.set(priceKey(a.price, tick), a.quantity);

  let minPrice = normalizePrice(mid - LADDER_TICKS_EACH_SIDE * tick, tick);
  let maxPrice = normalizePrice(mid + LADDER_TICKS_EACH_SIDE * tick, tick);
  const bookPrices = [...(dom?.bids ?? []), ...(dom?.asks ?? [])].map((x) => normalizePrice(x.price, tick));
  if (bookPrices.length) {
    minPrice = Math.min(minPrice, ...bookPrices);
    maxPrice = Math.max(maxPrice, ...bookPrices);
  }

  const rows: PriceRow[] = [];
  const steps = Math.min(MAX_LADDER_ROWS - 1, Math.max(1, Math.round((maxPrice - minPrice) / tick)));
  for (let i = 0; i <= steps; i += 1) {
    const price = normalizePrice(maxPrice - i * tick, tick);
    const key = priceKey(price, tick);
    rows.push({
      price,
      bidQty: bidMap.get(key) ?? 0,
      askQty: askMap.get(key) ?? 0,
      isBestBid: bestBid > 0 && key === priceKey(bestBid, tick),
      isBestAsk: bestAsk > 0 && key === priceKey(bestAsk, tick),
      isLast: last > 0 && key === priceKey(last, tick),
    });
  }
  return rows;
}

export function ScalperDOM({ mc, features, ticker, lang }: Props) {
  const dom = mc?.dom_book;
  const tick = dom?.tick_size && dom.tick_size > 0 ? dom.tick_size : 0.01;

  const layout = useMemo(() => {
    const footprint = mc?.footprint ?? [];
    const prints = mc?.recent_prints ?? [];
    const rows = buildScalperPriceRows(dom, features, mc);
    const maxBookQty = Math.max(1, ...rows.map((r) => Math.max(r.bidQty, r.askQty)));

    const cellLookup = new Map<string, { buy: number; sell: number; total: number }>();
    for (const col of footprint) {
      for (const c of col.cells) {
        cellLookup.set(minuteCellKey(col, c.price, tick), { buy: c.buy_vol, sell: c.sell_vol, total: c.total });
      }
    }
    const maxFootprintVol = Math.max(1, ...footprint.flatMap((col) => col.cells.map((c) => c.total)));

    const rowIndex = new Map<string, number>();
    rows.forEach((row, i) => rowIndex.set(priceKey(row.price, tick), i));

    const windowMs = Math.max(30, mc?.tape_stats?.window_sec ?? 60) * 1000;
    const newestPrintTime = Math.max(
      0,
      ...prints.map((p) => Date.parse(p.time)).filter((ts) => Number.isFinite(ts)),
    );
    const fallbackTime = Date.parse(dom?.observed_at ?? mc?.updated_at ?? '');
    const now = newestPrintTime > 0 ? newestPrintTime : (Number.isFinite(fallbackTime) ? fallbackTime : 0);
    const minTime = now - windowMs;
    const tapePoints = prints
      .map((p, i) => {
        const ts = Date.parse(p.time);
        const index = rowIndex.get(priceKey(p.price, tick));
        if (!Number.isFinite(ts) || index == null) return null;
        const x = clamp(((ts - minTime) / windowMs) * TAPE_VIEWBOX_WIDTH, 10, TAPE_VIEWBOX_WIDTH - 10);
        return {
          key: `${p.time}-${p.price}-${i}`,
          x,
          y: index * ROW_HEIGHT + ROW_HEIGHT / 2,
          radius: bubbleRadius(p.quantity),
          buy: isBuy(p.direction),
          quantity: p.quantity,
          price: p.price,
          direction: p.direction,
        };
      })
      .filter((p): p is NonNullable<typeof p> => p != null);

    return { rows, maxBookQty, cellLookup, maxFootprintVol, tapePoints };
  }, [dom, features, mc, tick]);

  const { rows, maxBookQty, cellLookup, maxFootprintVol, tapePoints } = layout;
  const cols = mc?.footprint ?? [];
  const lastPrice = mc?.tape_stats?.last_price ?? features?.mid ?? dom?.best_bid;
  const canvasHeight = Math.max(ROW_HEIGHT * rows.length, 560);

  if (!rows.length) {
    return (
      <div className="scalper-dom scalper-dom-empty">
        <p>Запустите AI Trader session — кластеры, лента и стакан появятся при поступлении данных.</p>
      </div>
    );
  }

  return (
    <div className="scalper-dom">
      <div className="scalper-dom-header">
        <div className="scalper-dom-title">
          <span className="font-semibold">{ticker ?? '—'}</span>
          <span className="text-[var(--muted)]"> · {lastPrice ? priceLabel(lastPrice, lang, tick) : '—'}</span>
        </div>
        <div className="scalper-dom-legend">
          <span className="scalper-legend-buy">buy prints</span>
          <span className="scalper-legend-sell">sell prints</span>
          <span className="scalper-legend-bid">bid limits</span>
          <span className="scalper-legend-ask">ask limits</span>
        </div>
      </div>

      <div className="scalper-dom-grid">
        <div
          className="scalper-col-header scalper-footprint-header"
          style={{ gridTemplateColumns: `repeat(${Math.max(1, cols.length)}, minmax(64px, 1fr))` }}
        >
          {cols.map((col) => (
            <div key={col.time} className="scalper-col-label">{col.label}</div>
          ))}
          {cols.length === 0 && <div className="scalper-col-label">clusters</div>}
        </div>
        <div className="scalper-col-header scalper-tape-header">лента принтов</div>
        <div className="scalper-col-header scalper-book-header">
          <span>ask</span>
          <span>price</span>
          <span>bid</span>
        </div>

        <div className="scalper-scroll">
          <div className="scalper-footprint">
            <div className="scalper-body">
              {rows.map((row) => (
                <div
                  key={row.price}
                  className={`scalper-row ${row.isBestBid || row.isBestAsk ? 'scalper-row-mid' : ''} ${row.isLast ? 'scalper-row-last' : ''}`}
                  style={{ gridTemplateColumns: `repeat(${Math.max(1, cols.length)}, minmax(64px, 1fr))` }}
                >
                  {cols.map((col) => {
                    const cell = cellLookup.get(minuteCellKey(col, row.price, tick));
                    if (!cell) {
                      return <div key={col.time} className="scalper-cluster-cell" />;
                    }
                    const intensity = clamp(cell.total / maxFootprintVol, 0.12, 1);
                    return (
                      <div
                        key={col.time}
                        className="scalper-cluster-cell scalper-cluster-split"
                        style={{ opacity: 0.52 + intensity * 0.48 }}
                        title={`buy ${cell.buy} sell ${cell.sell} total ${cell.total}`}
                      >
                        <span className="scalper-cluster-half scalper-cluster-sell">{cell.sell > 0 ? fmtVol(cell.sell, lang) : ''}</span>
                        <span className="scalper-cluster-half scalper-cluster-buy">{cell.buy > 0 ? fmtVol(cell.buy, lang) : ''}</span>
                      </div>
                    );
                  })}
                </div>
              ))}
            </div>
          </div>

          <div className="scalper-tape">
            <svg
              className="scalper-tape-svg"
              viewBox={`0 0 ${TAPE_VIEWBOX_WIDTH} ${canvasHeight}`}
              preserveAspectRatio="none"
              style={{ height: canvasHeight }}
            >
              {rows.map((row, i) => (
                <line
                  key={row.price}
                  x1="0"
                  x2={TAPE_VIEWBOX_WIDTH}
                  y1={i * ROW_HEIGHT + ROW_HEIGHT}
                  y2={i * ROW_HEIGHT + ROW_HEIGHT}
                  className={row.isLast ? 'scalper-tape-last-line' : 'scalper-tape-grid-line'}
                />
              ))}
              {tapePoints.map((p) => (
                <g key={p.key}>
                  <circle
                    cx={p.x}
                    cy={p.y}
                    r={p.radius}
                    className={p.buy ? 'scalper-tape-bubble-buy' : 'scalper-tape-bubble-sell'}
                  />
                  <text x={p.x} y={p.y + 3} textAnchor="middle" className="scalper-tape-bubble-text">
                    {fmtVol(p.quantity, lang)}
                  </text>
                  <title>{`${p.direction} ${priceLabel(p.price, lang, tick)} x${p.quantity}`}</title>
                </g>
              ))}
            </svg>
          </div>

          <div className="scalper-book">
            {rows.map((row) => (
              <div
                key={row.price}
                className={`scalper-row scalper-book-row ${row.isBestBid ? 'scalper-best-bid' : ''} ${row.isBestAsk ? 'scalper-best-ask' : ''} ${row.isLast ? 'scalper-row-last' : ''}`}
              >
                <div className="scalper-book-side scalper-book-ask">
                  {row.askQty > 0 && (
                    <div
                      className="scalper-book-bar scalper-book-bar-ask"
                      style={{ width: `${(row.askQty / maxBookQty) * 100}%` }}
                    />
                  )}
                  <span className="scalper-book-qty">{row.askQty > 0 ? row.askQty : ''}</span>
                </div>
                <div className="scalper-book-price">{priceLabel(row.price, lang, tick)}</div>
                <div className="scalper-book-side scalper-book-bid">
                  {row.bidQty > 0 && (
                    <div
                      className="scalper-book-bar scalper-book-bar-bid"
                      style={{ width: `${(row.bidQty / maxBookQty) * 100}%` }}
                    />
                  )}
                  <span className="scalper-book-qty">{row.bidQty > 0 ? row.bidQty : ''}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div
          className="scalper-footprint-footer"
          style={{ gridTemplateColumns: `repeat(${Math.max(1, cols.length)}, minmax(64px, 1fr))` }}
        >
          {cols.map((col) => (
            <div key={`f-${col.time}`} className="scalper-footer-cell">
              <span>{fmtVol(col.total_vol, lang)}</span>
              <span className={col.delta >= 0 ? 'scalper-delta-pos' : 'scalper-delta-neg'}>
                {col.delta >= 0 ? '+' : ''}{fmtVol(col.delta, lang)}
              </span>
            </div>
          ))}
        </div>
        <div className="scalper-tape-footer">
          prints {mc?.tape_stats.trade_count ?? 0} · Δ {formatNumber((mc?.tape_stats.delta_pct ?? 0) * 100, lang, 1)}%
        </div>
        <div className="scalper-book-footer">
          depth {dom?.bids.length ?? 0}/{dom?.asks.length ?? 0}
        </div>
      </div>
    </div>
  );
}
