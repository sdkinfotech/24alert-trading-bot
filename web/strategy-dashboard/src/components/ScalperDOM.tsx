import { useMemo } from 'react';
import type { AiTraderFeatures, AiTraderMarketContext, AiTraderPrint } from '../api/types';
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
}

function isBuy(dir: string): boolean {
  return dir.toLowerCase().includes('buy');
}

function fmtVol(n: number, lang: Lang): string {
  if (n >= 1000) return `${formatNumber(n / 1000, lang, 1)}k`;
  return String(n);
}

export function ScalperDOM({ mc, features, ticker, lang }: Props) {
  const dom = mc?.dom_book;
  const footprint = mc?.footprint ?? [];
  const prints = mc?.recent_prints ?? [];
  const tick = dom?.tick_size ?? 0.01;

  const layout = useMemo(() => {
    const bidMap = new Map<number, number>();
    const askMap = new Map<number, number>();
    for (const b of dom?.bids ?? []) bidMap.set(b.price, b.quantity);
    for (const a of dom?.asks ?? []) askMap.set(a.price, a.quantity);

    const priceSet = new Set<number>();
    for (const p of bidMap.keys()) priceSet.add(p);
    for (const p of askMap.keys()) priceSet.add(p);
    for (const col of footprint) {
      for (const c of col.cells) priceSet.add(c.price);
    }
    for (const p of prints) priceSet.add(Math.round(p.price / tick) * tick);

    let prices = [...priceSet].sort((a, b) => b - a);
    if (prices.length === 0 && features) {
      const mid = features.mid || (features.best_bid + features.best_ask) / 2;
      if (mid > 0) {
        prices = Array.from({ length: 25 }, (_, i) => mid + (12 - i) * tick);
      }
    }
    if (prices.length > 48) {
      const bestBid = dom?.best_bid ?? features?.best_bid ?? 0;
      const bestAsk = dom?.best_ask ?? features?.best_ask ?? 0;
      const mid = (bestBid + bestAsk) / 2;
      prices = prices.filter((p) => Math.abs(p-mid) <= tick * 24);
    }

    const rows: PriceRow[] = prices.map((price) => ({
      price,
      bidQty: bidMap.get(price) ?? 0,
      askQty: askMap.get(price) ?? 0,
      isBestBid: price === dom?.best_bid,
      isBestAsk: price === dom?.best_ask,
    }));

    const maxBookQty = Math.max(
      1,
      ...rows.map((r) => Math.max(r.bidQty, r.askQty)),
    );

    const cellLookup = new Map<string, { buy: number; sell: number; total: number }>();
    for (const col of footprint) {
      for (const c of col.cells) {
        cellLookup.set(`${col.time}:${c.price}`, { buy: c.buy_vol, sell: c.sell_vol, total: c.total });
      }
    }

    const colMaxVol = footprint.map((col) =>
      Math.max(1, ...col.cells.map((c) => c.total)),
    );

    const printsByPrice = new Map<number, AiTraderPrint[]>();
    for (const p of prints) {
      const px = Math.round(p.price / tick) * tick;
      const list = printsByPrice.get(px) ?? [];
      list.push(p);
      printsByPrice.set(px, list);
    }

    return { rows, footprint, maxBookQty, cellLookup, colMaxVol, printsByPrice };
  }, [dom, footprint, prints, features, tick]);

  const { rows, maxBookQty, cellLookup, colMaxVol, printsByPrice } = layout;
  const cols = layout.footprint;
  const lastPrice = mc?.tape_stats?.last_price ?? features?.mid ?? dom?.best_bid;

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
          <span className="text-[var(--muted)]"> · {lastPrice ? formatNumber(lastPrice, lang, 2) : '—'}</span>
        </div>
        <div className="scalper-dom-legend">
          <span className="scalper-legend-bid">bid</span>
          <span className="scalper-legend-ask">ask</span>
          <span className="scalper-legend-buy">Δ+</span>
          <span className="scalper-legend-sell">Δ−</span>
        </div>
      </div>

      <div className="scalper-dom-grid">
        <div className="scalper-footprint">
          <div className="scalper-col-header scalper-footprint-header">
            {cols.map((col) => (
              <div key={col.time} className="scalper-col-label">{col.label}</div>
            ))}
            {cols.length === 0 && <div className="scalper-col-label">clusters</div>}
          </div>
          <div className="scalper-body">
            {rows.map((row) => (
              <div key={row.price} className={`scalper-row ${row.isBestBid || row.isBestAsk ? 'scalper-row-mid' : ''}`}>
                {cols.map((col, ci) => {
                  const cell = cellLookup.get(`${col.time}:${row.price}`);
                  if (!cell) {
                    return <div key={col.time} className="scalper-cluster-cell" />;
                  }
                  const intensity = cell.total / (colMaxVol[ci] ?? 1);
                  const buyDom = cell.buy >= cell.sell;
                  return (
                    <div
                      key={col.time}
                      className={`scalper-cluster-cell ${buyDom ? 'scalper-cluster-buy' : 'scalper-cluster-sell'}`}
                      style={{ opacity: 0.35 + intensity * 0.65 }}
                      title={`buy ${cell.buy} sell ${cell.sell}`}
                    >
                      <span>{cell.total > 0 ? fmtVol(cell.total, lang) : ''}</span>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
          <div className="scalper-footprint-footer">
            {cols.map((col) => (
              <div key={`f-${col.time}`} className="scalper-footer-cell">
                <span>{fmtVol(col.total_vol, lang)}</span>
                <span className={col.delta >= 0 ? 'scalper-delta-pos' : 'scalper-delta-neg'}>
                  {col.delta >= 0 ? '+' : ''}{fmtVol(col.delta, lang)}
                </span>
              </div>
            ))}
          </div>
        </div>

        <div className="scalper-tape">
          <div className="scalper-col-header scalper-tape-header">лента</div>
          <div className="scalper-body scalper-tape-body">
            {rows.map((row) => {
              const pts = printsByPrice.get(row.price) ?? [];
              return (
                <div key={row.price} className="scalper-row scalper-tape-row">
                  {pts.slice(-6).map((p, i) => (
                    <div
                      key={`${p.time}-${i}`}
                      className={`scalper-tape-dot ${isBuy(p.direction) ? 'scalper-tape-buy' : 'scalper-tape-sell'}`}
                      title={`${p.direction} ${p.price} x${p.quantity}`}
                    />
                  ))}
                </div>
              );
            })}
          </div>
        </div>

        <div className="scalper-book">
          <div className="scalper-col-header scalper-book-header">
            <span>ask</span>
            <span>price</span>
            <span>bid</span>
          </div>
          <div className="scalper-body">
            {rows.map((row) => (
              <div
                key={row.price}
                className={`scalper-row scalper-book-row ${row.isBestBid ? 'scalper-best-bid' : ''} ${row.isBestAsk ? 'scalper-best-ask' : ''}`}
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
                <div className="scalper-book-price">{formatNumber(row.price, lang, 2)}</div>
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
      </div>
    </div>
  );
}
