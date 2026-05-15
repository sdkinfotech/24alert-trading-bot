import { useEffect, useRef } from 'react';
import {
  createChart,
  createSeriesMarkers,
  CandlestickSeries,
  LineSeries,
  LineStyle,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  ColorType,
  CrosshairMode,
} from 'lightweight-charts';
import type { IndicatorData } from '../api/types';

interface Props {
  data: IndicatorData | null;
}

function isORB(data: IndicatorData): boolean {
  if (data.strategy_type === 'level_bounce') return false;
  return (
    data.strategy_type === 'orb_breakout' ||
    (data.range_high != null && data.range_high > 0)
  );
}

function isLevelBounce(data: IndicatorData): boolean {
  return (
    data.strategy_type === 'level_bounce' ||
    ((data.support?.length ?? 0) > 0 && (data.resistance?.length ?? 0) > 0)
  );
}

function hasSMA(data: IndicatorData): boolean {
  return data.candles.some(
    (c) => (c.fast_sma ?? 0) > 0 || (c.slow_sma ?? 0) > 0,
  );
}

/** Support / resistance prices for Level Bounce (from snapshot or candle trail). */
function levelBounceLevels(data: IndicatorData): { support: number[]; resistance: number[] } {
  let sup = [...(data.support ?? [])].filter((p) => p > 0);
  let res = [...(data.resistance ?? [])].filter((p) => p > 0);
  if (sup.length === 0 || res.length === 0) {
    const fromCandles = data.candles.filter(
      (c) => (c.support ?? 0) > 0 || (c.resistance ?? 0) > 0,
    );
    if (fromCandles.length > 0) {
      const last = fromCandles[fromCandles.length - 1];
      if ((last.support ?? 0) > 0) sup = [last.support!];
      if ((last.resistance ?? 0) > 0) res = [last.resistance!];
    }
  }
  return { support: sup, resistance: res };
}

export function IndicatorChart({ data }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const line1Ref = useRef<ISeriesApi<'Line'> | null>(null);
  const line2Ref = useRef<ISeriesApi<'Line'> | null>(null);
  const markersRef = useRef<ReturnType<typeof createSeriesMarkers> | null>(null);
  const priceLinesRef = useRef<IPriceLine[]>([]);

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: '#0a0a0f' },
        textColor: '#9ca3af',
      },
      grid: {
        vertLines: { color: '#1f2937' },
        horzLines: { color: '#1f2937' },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: '#374151' },
      timeScale: { borderColor: '#374151', timeVisible: true },
      width: containerRef.current.clientWidth,
      height: 420,
    });
    chartRef.current = chart;

    candleRef.current = chart.addSeries(CandlestickSeries, {
      upColor: '#22c55e',
      downColor: '#ef4444',
      borderUpColor: '#22c55e',
      borderDownColor: '#ef4444',
      wickUpColor: '#22c55e',
      wickDownColor: '#ef4444',
    });

    line1Ref.current = chart.addSeries(LineSeries, { color: '#3b82f6', lineWidth: 2 });
    line2Ref.current = chart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 });

    const ro = new ResizeObserver((entries) => {
      for (const e of entries) {
        chart.applyOptions({ width: e.contentRect.width });
      }
    });
    ro.observe(containerRef.current);
    return () => {
      ro.disconnect();
      chart.remove();
    };
  }, []);

  useEffect(() => {
    if (!data || !candleRef.current || !line1Ref.current || !line2Ref.current) return;

    const series = candleRef.current;

    for (const pl of priceLinesRef.current) {
      series.removePriceLine(pl);
    }
    priceLinesRef.current = [];

    const toUnix = (t: string) => Math.floor(new Date(t).getTime() / 1000) as any;

    const candles = data.candles.map((c) => ({
      time: toUnix(c.time),
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
    }));
    series.setData(candles);

    const firstBarT = candles.length ? (candles[0].time as number) : 0;
    const lastBarT = candles.length ? (candles[candles.length - 1].time as number) : 0;
    /** lightweight-charts only draws markers at existing bar times; clamp into range. */
    const clampBarTime = (t: number) => {
      if (!candles.length) return t;
      if (t < firstBarT) return firstBarT;
      if (t > lastBarT) return lastBarT;
      return t;
    };

    const orbMode = isORB(data);
    const smaMode = hasSMA(data);
    const lbMode = isLevelBounce(data);

    if (lbMode) {
      line1Ref.current.setData([]);
      line2Ref.current.setData([]);

      const { support, resistance } = levelBounceLevels(data);
      const supColors = ['#22c55e', '#4ade80', '#86efac'];
      const resColors = ['#ef4444', '#f87171', '#fca5a5'];

      let si = 0;
      for (const p of support) {
        const pl = series.createPriceLine({
          price: p,
          color: supColors[si % supColors.length],
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          axisLabelVisible: true,
          title: `S${si + 1}`,
        });
        priceLinesRef.current.push(pl);
        si++;
      }
      let ri = 0;
      for (const p of resistance) {
        const pl = series.createPriceLine({
          price: p,
          color: resColors[ri % resColors.length],
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          axisLabelVisible: true,
          title: `R${ri + 1}`,
        });
        priceLinesRef.current.push(pl);
        ri++;
      }
    } else if (orbMode) {
      const rangeHighData = data.candles
        .filter((c) => (c.range_high ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.range_high! }));
      line1Ref.current.setData(rangeHighData);
      line1Ref.current.applyOptions({
        color: '#22c55e',
        lineWidth: 2,
        lineStyle: LineStyle.Dashed,
        title: 'Range High',
      } as any);

      const rangeLowData = data.candles
        .filter((c) => (c.range_low ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.range_low! }));
      line2Ref.current.setData(rangeLowData);
      line2Ref.current.applyOptions({
        color: '#ef4444',
        lineWidth: 2,
        lineStyle: LineStyle.Dashed,
        title: 'Range Low',
      } as any);
    } else if (smaMode) {
      const fastData = data.candles
        .filter((c) => (c.fast_sma ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.fast_sma! }));
      line1Ref.current.setData(fastData);
      line1Ref.current.applyOptions({
        color: '#3b82f6',
        lineWidth: 2,
        lineStyle: LineStyle.Solid,
        title: 'Fast SMA',
      } as any);

      const slowData = data.candles
        .filter((c) => (c.slow_sma ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.slow_sma! }));
      line2Ref.current.setData(slowData);
      line2Ref.current.applyOptions({
        color: '#f59e0b',
        lineWidth: 2,
        lineStyle: LineStyle.Solid,
        title: 'Slow SMA',
      } as any);
    } else {
      line1Ref.current.setData([]);
      line2Ref.current.setData([]);
    }

    if (data.signals.length > 0) {
      const markers = data.signals.map((s) => {
        const rawT = toUnix(s.time) as number;
        const t = clampBarTime(rawT);
        const eod = (s.reason ?? '').toLowerCase().includes('eod');
        const clipped = rawT !== t;
        let text = s.direction.toUpperCase();
        if (eod) text = clipped ? `${text} EOD*` : `${text} EOD`;
        return {
          time: t as any,
          position: s.direction === 'buy' ? ('belowBar' as const) : ('aboveBar' as const),
          color: s.direction === 'buy' ? '#22c55e' : '#ef4444',
          shape: s.direction === 'buy' ? ('arrowUp' as const) : ('arrowDown' as const),
          text,
        };
      });
      if (markersRef.current) {
        markersRef.current.setMarkers(markers);
      } else {
        markersRef.current = createSeriesMarkers(series, markers);
      }
    } else if (markersRef.current) {
      markersRef.current.setMarkers([]);
    }

    chartRef.current?.timeScale().fitContent();
  }, [data]);

  const buyCount = data?.signals.filter((s) => s.direction === 'buy').length ?? 0;
  const sellCount = data?.signals.filter((s) => s.direction === 'sell').length ?? 0;
  const total = buyCount + sellCount;

  const posLabel =
    data?.position === 1 ? 'LONG' : data?.position === -1 ? 'SHORT' : 'FLAT';
  const posColor =
    data?.position === 1
      ? 'text-green-400'
      : data?.position === -1
        ? 'text-red-400'
        : 'text-gray-400';

  const orbMode = data ? isORB(data) : false;
  const lbMode = data ? isLevelBounce(data) : false;
  const smaMode = data ? hasSMA(data) : false;

  let headerLabel = '';
  if (lbMode && data) {
    const { support, resistance } = levelBounceLevels(data);
    const atr = data.atr ?? 0;
    headerLabel = `Level Bounce · ATR ${atr > 0 ? atr.toFixed(2) : '—'} · ${support.length}S / ${resistance.length}R`;
  } else if (orbMode && data) {
    headerLabel = `ORB (Range ${data.range_formed ? 'formed' : 'forming'}: ${data.range_high?.toFixed(2) ?? '?'} / ${data.range_low?.toFixed(2) ?? '?'})`;
  } else if (smaMode && data) {
    headerLabel = `SMA(${data.fast_period ?? '?'}/${data.slow_period ?? '?'})`;
  } else if (data) {
    headerLabel = data.strategy_type ?? 'Strategy';
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2 px-1">
        <div className="text-sm text-gray-400">
          {headerLabel} &middot;{' '}
          Signals: <span className="text-white font-medium">{total}</span>{' '}
          <span className="text-green-400">({buyCount} buy</span> /{' '}
          <span className="text-red-400">{sellCount} sell)</span>
        </div>
        <div className={`text-sm font-semibold ${posColor}`}>
          Position: {posLabel}
        </div>
      </div>
      {lbMode && (
        <p className="text-xs text-gray-500 mb-1 px-1">
          Зелёные пунктиры — поддержка (S1–S3), красные — сопротивление (R1–R3). Позиция справа — внутреннее
          состояние стратегии; в Stats «Positions» — факт по счёту у брокера (может отличаться до исполнения
          ордеров). Маркеры строятся по времени свечи; «EOD*» — сигнал после последней свечи на графике,
          стрелка показана на последнем баре.
        </p>
      )}
      <div ref={containerRef} className="rounded-lg overflow-hidden border border-gray-800" />
    </div>
  );
}
