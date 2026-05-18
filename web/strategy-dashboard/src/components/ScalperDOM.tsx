import { useEffect, useMemo, useRef, type CSSProperties } from 'react';
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

interface PrintAgg {
  buyVol: number;
  sellVol: number;
  count: number;
}

const LADDER_TICKS_EACH_SIDE = 56;
const MAX_LADDER_ROWS = 160;
const ROW_HEIGHT = 17;
const FOOTPRINT_COL_MIN = 48;
const TAPE_COL_W = 32;
const DOM_SIDE_MIN = 54;
const DOM_SIDE_MAX = 80;
const PRICE_COL_W = 52;

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

function buildColumnTemplate(fpCount: number): string {
  const n = Math.max(1, fpCount);
  return [
    `repeat(${n}, minmax(${FOOTPRINT_COL_MIN}px, 1fr))`,
    `${TAPE_COL_W}px`,
    `minmax(${DOM_SIDE_MIN}px, ${DOM_SIDE_MAX}px)`,
    `${PRICE_COL_W}px`,
    `minmax(${DOM_SIDE_MIN}px, ${DOM_SIDE_MAX}px)`,
  ].join(' ');
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
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const cols = mc?.footprint ?? [];

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

    const printsByPrice = new Map<string, PrintAgg>();
    for (const p of prints) {
      const key = priceKey(p.price, tick);
      const agg = printsByPrice.get(key) ?? { buyVol: 0, sellVol: 0, count: 0 };
      if (isBuy(p.direction)) agg.buyVol += p.quantity;
      else agg.sellVol += p.quantity;
      agg.count += 1;
      printsByPrice.set(key, agg);
    }
    const maxPrintVol = Math.max(
      1,
      ...[...printsByPrice.values()].map((a) => a.buyVol + a.sellVol),
    );

    const bestBid = dom?.best_bid ?? features?.best_bid ?? 0;
    const bestAsk = dom?.best_ask ?? features?.best_ask ?? 0;
    const spreadMid = bestBid > 0 && bestAsk > 0 ? (bestBid + bestAsk) / 2 : (mc?.tape_stats?.last_price ?? features?.mid ?? 0);
    const centerIndex = rows.reduce((bestIndex, row, i) => {
      if (spreadMid <= 0) return bestIndex;
      const bestDist = Math.abs(rows[bestIndex]?.price - spreadMid);
      const dist = Math.abs(row.price - spreadMid);
      return dist < bestDist ? i : bestIndex;
    }, 0);

    const totalAskQty = rows.reduce((s, r) => s + r.askQty, 0);
    const totalBidQty = rows.reduce((s, r) => s + r.bidQty, 0);
    const columnTemplate = buildColumnTemplate(footprint.length);

    return {
      rows,
      maxBookQty,
      cellLookup,
      maxFootprintVol,
      printsByPrice,
      maxPrintVol,
      centerIndex,
      totalAskQty,
      totalBidQty,
      columnTemplate,
    };
  }, [dom, features, mc, tick]);

  const {
    rows,
    maxBookQty,
    cellLookup,
    maxFootprintVol,
    printsByPrice,
    maxPrintVol,
    centerIndex,
    totalAskQty,
    totalBidQty,
    columnTemplate,
  } = layout;

  const lastPrice = mc?.tape_stats?.last_price ?? features?.mid ?? dom?.best_bid;
  const spreadCenterKey = `${dom?.best_bid ?? features?.best_bid ?? 0}:${dom?.best_ask ?? features?.best_ask ?? 0}:${rows.length}`;
  const gridStyle = { gridTemplateColumns: columnTemplate } as CSSProperties;

  useEffect(() => {
    const el = scrollRef.current;
    if (!el || rows.length === 0) return;
    const target = centerIndex * ROW_HEIGHT - el.clientHeight / 2 + ROW_HEIGHT / 2;
    el.scrollTop = Math.max(0, target);
  }, [centerIndex, rows.length, spreadCenterKey]);

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
        <div className="scalper-ladder-header" style={gridStyle}>
          {cols.map((col) => (
            <div key={col.time} className="scalper-hcell scalper-hcell-fp">{col.label}</div>
          ))}
          {cols.length === 0 && <div className="scalper-hcell scalper-hcell-fp">clusters</div>}
          <div className="scalper-hcell scalper-hcell-tape">tape</div>
          <div className="scalper-hcell scalper-hcell-dom">ask</div>
          <div className="scalper-hcell scalper-hcell-price">price</div>
          <div className="scalper-hcell scalper-hcell-dom">bid</div>
        </div>

        <div ref={scrollRef} className="scalper-scroll">
          {rows.map((row) => {
            const rowClass = [
              'scalper-ladder-row',
              row.isBestBid || row.isBestAsk ? 'scalper-row-mid' : '',
              row.isLast ? 'scalper-row-last' : '',
            ]
              .filter(Boolean)
              .join(' ');

            const pKey = priceKey(row.price, tick);
            const printAgg = printsByPrice.get(pKey);
            const printTotal = printAgg ? printAgg.buyVol + printAgg.sellVol : 0;
            const dotSize = printTotal > 0 ? clamp((printTotal / maxPrintVol) * 18, 4, 18) : 0;
            const dotBuy = printAgg ? printAgg.buyVol >= printAgg.sellVol : false;

            const askFill = row.askQty > 0 ? `${(row.askQty / maxBookQty) * 100}%` : '0%';
            const bidFill = row.bidQty > 0 ? `${(row.bidQty / maxBookQty) * 100}%` : '0%';

            return (
              <div key={row.price} className={rowClass} style={gridStyle}>
                {cols.length > 0
                  ? cols.map((col) => {
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
                          <span className="scalper-cluster-half scalper-cluster-sell">
                            {cell.sell > 0 ? fmtVol(cell.sell, lang) : ''}
                          </span>
                          <span className="scalper-cluster-half scalper-cluster-buy">
                            {cell.buy > 0 ? fmtVol(cell.buy, lang) : ''}
                          </span>
                        </div>
                      );
                    })
                  : (
                    <div className="scalper-cluster-cell scalper-cluster-empty" />
                  )}

                <div className="scalper-tape-cell">
                  {printAgg && dotSize > 0 && (
                    <div
                      className={`scalper-tape-dot ${dotBuy ? 'scalper-tape-dot-buy' : 'scalper-tape-dot-sell'}`}
                      style={{ width: dotSize, height: dotSize }}
                      title={`buy ${printAgg.buyVol} sell ${printAgg.sellVol} (${printAgg.count} prints)`}
                    >
                      {dotSize >= 12 && <span>{fmtVol(printTotal, lang)}</span>}
                    </div>
                  )}
                </div>

                <div
                  className={`dom-ask ${row.isBestAsk ? 'scalper-best-ask' : ''}`}
                  style={{ '--fill': askFill } as CSSProperties}
                >
                  {row.askQty > 0 ? row.askQty : ''}
                </div>
                <div className={`dom-price ${row.isBestBid || row.isBestAsk ? 'dom-price-mid' : ''}`}>
                  {priceLabel(row.price, lang, tick)}
                </div>
                <div
                  className={`dom-bid ${row.isBestBid ? 'scalper-best-bid' : ''}`}
                  style={{ '--fill': bidFill } as CSSProperties}
                >
                  {row.bidQty > 0 ? row.bidQty : ''}
                </div>
              </div>
            );
          })}
        </div>

        <div className="scalper-ladder-footer" style={gridStyle}>
          {cols.map((col) => (
            <div key={`f-${col.time}`} className="scalper-footer-cell">
              <span>{fmtVol(col.total_vol, lang)}</span>
              <span className={col.delta >= 0 ? 'scalper-delta-pos' : 'scalper-delta-neg'}>
                {col.delta >= 0 ? '+' : ''}
                {fmtVol(col.delta, lang)}
              </span>
            </div>
          ))}
          {cols.length === 0 && <div className="scalper-footer-cell">—</div>}
          <div className="scalper-footer-cell scalper-footer-tape">
            {mc?.tape_stats.trade_count ?? 0} · Δ {formatNumber((mc?.tape_stats.delta_pct ?? 0) * 100, lang, 1)}%
          </div>
          <div className="scalper-footer-cell scalper-footer-dom">{fmtVol(totalAskQty, lang)}</div>
          <div className="scalper-footer-cell scalper-footer-price" />
          <div className="scalper-footer-cell scalper-footer-dom">{fmtVol(totalBidQty, lang)}</div>
        </div>
      </div>
    </div>
  );
}
