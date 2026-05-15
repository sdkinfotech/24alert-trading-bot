import { useEffect, useRef } from 'react';
import {
  createChart,
  createSeriesMarkers,
  CandlestickSeries,
  LineSeries,
  type IChartApi,
  type ISeriesApi,
  ColorType,
  CrosshairMode,
} from 'lightweight-charts';
import type { IndicatorData } from '../api/types';

interface Props {
  data: IndicatorData | null;
}

function isORB(data: IndicatorData): boolean {
  return data.strategy_type === 'orb_breakout' ||
    (data.range_high != null && data.range_high > 0);
}

function hasSMA(data: IndicatorData): boolean {
  return data.candles.some((c) => c.fast_sma > 0 || c.slow_sma > 0);
}

export function IndicatorChart({ data }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const line1Ref = useRef<ISeriesApi<'Line'> | null>(null);
  const line2Ref = useRef<ISeriesApi<'Line'> | null>(null);
  const markersRef = useRef<ReturnType<typeof createSeriesMarkers> | null>(null);

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

    const toUnix = (t: string) => Math.floor(new Date(t).getTime() / 1000) as any;

    const candles = data.candles.map((c) => ({
      time: toUnix(c.time),
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
    }));
    candleRef.current.setData(candles);

    const orbMode = isORB(data);
    const smaMode = hasSMA(data);

    if (orbMode) {
      const rangeHighData = data.candles
        .filter((c) => c.range_high > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.range_high }));
      line1Ref.current.setData(rangeHighData);
      line1Ref.current.applyOptions({
        color: '#22c55e',
        lineWidth: 2,
        lineStyle: 2, // dashed
        title: 'Range High',
      } as any);

      const rangeLowData = data.candles
        .filter((c) => c.range_low > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.range_low }));
      line2Ref.current.setData(rangeLowData);
      line2Ref.current.applyOptions({
        color: '#ef4444',
        lineWidth: 2,
        lineStyle: 2,
        title: 'Range Low',
      } as any);
    } else if (smaMode) {
      const fastData = data.candles
        .filter((c) => c.fast_sma > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.fast_sma }));
      line1Ref.current.setData(fastData);
      line1Ref.current.applyOptions({
        color: '#3b82f6',
        lineWidth: 2,
        lineStyle: 0,
        title: 'Fast SMA',
      } as any);

      const slowData = data.candles
        .filter((c) => c.slow_sma > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.slow_sma }));
      line2Ref.current.setData(slowData);
      line2Ref.current.applyOptions({
        color: '#f59e0b',
        lineWidth: 2,
        lineStyle: 0,
        title: 'Slow SMA',
      } as any);
    } else {
      line1Ref.current.setData([]);
      line2Ref.current.setData([]);
    }

    if (data.signals.length > 0 && candleRef.current) {
      const markers = data.signals.map((s) => ({
        time: toUnix(s.time),
        position: s.direction === 'buy' ? ('belowBar' as const) : ('aboveBar' as const),
        color: s.direction === 'buy' ? '#22c55e' : '#ef4444',
        shape: s.direction === 'buy' ? ('arrowUp' as const) : ('arrowDown' as const),
        text: s.direction.toUpperCase(),
      }));
      if (markersRef.current) {
        markersRef.current.setMarkers(markers);
      } else {
        markersRef.current = createSeriesMarkers(candleRef.current, markers);
      }
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

  const headerLabel = orbMode
    ? `ORB (Range ${data?.range_formed ? 'formed' : 'forming'}: ${data?.range_high?.toFixed(2) ?? '?'} / ${data?.range_low?.toFixed(2) ?? '?'})`
    : `SMA(${data?.fast_period ?? '?'}/${data?.slow_period ?? '?'})`;

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
      <div ref={containerRef} className="rounded-lg overflow-hidden border border-gray-800" />
    </div>
  );
}
