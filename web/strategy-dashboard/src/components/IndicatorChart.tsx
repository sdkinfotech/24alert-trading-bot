import { useEffect, useRef, useState } from 'react';
import {
  createChart,
  createSeriesMarkers,
  CandlestickSeries,
  LineSeries,
  LineStyle,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type Time,
  ColorType,
  CrosshairMode,
  type UTCTimestamp,
} from 'lightweight-charts';
import type { IndicatorData, PortfolioSnapshot, TimelineEvent } from '../api/types';
import type { ChartTheme } from '../theme';
import type { Lang } from '../i18n';
import { formatDateTime } from '../format';

interface Props {
  data: IndicatorData | null;
  events?: TimelineEvent[];
  portfolio?: PortfolioSnapshot | null;
  chartTheme?: ChartTheme;
  lang?: Lang;
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
  return (data.candles ?? []).some(
    (c) => (c.fast_sma ?? 0) > 0 || (c.slow_sma ?? 0) > 0,
  );
}

/** Support / resistance prices for Level Bounce (from snapshot or candle trail). */
function levelBounceLevels(data: IndicatorData): { support: number[]; resistance: number[] } {
  let sup = [...(data.support ?? [])].filter((p) => p > 0);
  let res = [...(data.resistance ?? [])].filter((p) => p > 0);
  if (sup.length === 0 || res.length === 0) {
    const fromCandles = (data.candles ?? []).filter(
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

function formatLevelSources(
  label: string,
  sources: NonNullable<IndicatorData['support_sources']>,
): string {
  if (!sources.length) return '';
  return `${label}: ${sources
    .map((s) => `${s.rank}:${s.price.toFixed(2)} ${s.kind} ${s.date || '?'}`)
    .join(' · ')}`;
}

const fallbackTheme: ChartTheme = {
  background: '#0f172a',
  text: '#94a3b8',
  grid: '#243044',
  border: '#334155',
  up: '#22c55e',
  down: '#f87171',
  fast: '#60a5fa',
  slow: '#fbbf24',
  support: '#4ade80',
  resistance: '#f87171',
  brokerAvg: '#c4b5fd',
  trailing: '#fb923c',
  fill: '#c084fc',
};

export function IndicatorChart({ data, events = [], portfolio = null, chartTheme = fallbackTheme, lang = 'ru' }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const isFirstDataPaintRef = useRef(true);
  const candleRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const line1Ref = useRef<ISeriesApi<'Line'> | null>(null);
  const line2Ref = useRef<ISeriesApi<'Line'> | null>(null);
  const markersRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const priceLinesRef = useRef<IPriceLine[]>([]);
  const [hover, setHover] = useState<string>('');

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: chartTheme.background },
        textColor: chartTheme.text,
      },
      localization: {
        locale: lang === 'ru' ? 'ru-RU' : 'en-US',
        timeFormatter: (t: number | string) => typeof t === 'number' ? formatDateTime(t, lang) : String(t),
      },
      grid: {
        vertLines: { color: chartTheme.grid },
        horzLines: { color: chartTheme.grid },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: chartTheme.border },
      timeScale: {
        borderColor: chartTheme.border,
        timeVisible: true,
        secondsVisible: false,
        rightOffset: 8,
        tickMarkFormatter: (t: number | string) => typeof t === 'number' ? formatDateTime(t, lang) : String(t),
      },
      width: containerRef.current.clientWidth,
      height: 420,
    });
    chartRef.current = chart;

    candleRef.current = chart.addSeries(CandlestickSeries, {
      upColor: chartTheme.up,
      downColor: chartTheme.down,
      borderUpColor: chartTheme.up,
      borderDownColor: chartTheme.down,
      wickUpColor: chartTheme.up,
      wickDownColor: chartTheme.down,
    });

    line1Ref.current = chart.addSeries(LineSeries, { color: chartTheme.fast, lineWidth: 2 });
    line2Ref.current = chart.addSeries(LineSeries, { color: chartTheme.slow, lineWidth: 2 });
    chart.subscribeCrosshairMove((param) => {
      const point = param.seriesData.get(candleRef.current!);
      if (!point || !('open' in point)) {
        setHover('');
        return;
      }
      const bar = point as { open: number; high: number; low: number; close: number };
      setHover(`O ${bar.open.toFixed(4)} · H ${bar.high.toFixed(4)} · L ${bar.low.toFixed(4)} · C ${bar.close.toFixed(4)}`);
    });

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
  }, [chartTheme, lang]);

  useEffect(() => {
    if (!data || !candleRef.current || !line1Ref.current || !line2Ref.current) return;

    const series = candleRef.current;

    for (const pl of priceLinesRef.current) {
      series.removePriceLine(pl);
    }
    priceLinesRef.current = [];

    const toUnix = (t: string): UTCTimestamp => Math.floor(new Date(t).getTime() / 1000) as UTCTimestamp;

    const candles = (data.candles ?? []).map((c) => ({
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
      const supColors = [chartTheme.support, chartTheme.up, '#86efac'];
      const resColors = [chartTheme.resistance, chartTheme.down, '#fca5a5'];

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
        color: chartTheme.support,
        lineWidth: 2,
        lineStyle: LineStyle.Dashed,
      });

      const rangeLowData = data.candles
        .filter((c) => (c.range_low ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.range_low! }));
      line2Ref.current.setData(rangeLowData);
      line2Ref.current.applyOptions({
        color: chartTheme.resistance,
        lineWidth: 2,
        lineStyle: LineStyle.Dashed,
      });
    } else if (smaMode) {
      const fastData = data.candles
        .filter((c) => (c.fast_sma ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.fast_sma! }));
      line1Ref.current.setData(fastData);
      line1Ref.current.applyOptions({
        color: chartTheme.fast,
        lineWidth: 2,
        lineStyle: LineStyle.Solid,
      });

      const slowData = data.candles
        .filter((c) => (c.slow_sma ?? 0) > 0)
        .map((c) => ({ time: toUnix(c.time), value: c.slow_sma! }));
      line2Ref.current.setData(slowData);
      line2Ref.current.applyOptions({
        color: chartTheme.slow,
        lineWidth: 2,
        lineStyle: LineStyle.Solid,
      });
    } else {
      line1Ref.current.setData([]);
      line2Ref.current.setData([]);
    }

    for (const p of portfolio?.positions ?? []) {
      if (!p.in_instance || p.quantity === 0 || p.average_price <= 0) continue;
      const pl = series.createPriceLine({
        price: p.average_price,
        color: chartTheme.brokerAvg,
        lineWidth: 2,
        lineStyle: LineStyle.Dotted,
        axisLabelVisible: true,
        title: `Broker avg ${p.quantity > 0 ? 'LONG' : 'SHORT'} ${p.ticker || ''}`.trim(),
      });
      priceLinesRef.current.push(pl);
    }
    if (data.trailing_stop_active && (data.trailing_stop_price ?? 0) > 0) {
      const pl = series.createPriceLine({
        price: data.trailing_stop_price!,
        color: chartTheme.trailing,
        lineWidth: 2,
        lineStyle: LineStyle.Dashed,
        axisLabelVisible: true,
        title: `Trailing ${(data.trailing_stop_pct! * 100).toFixed(2)}%`,
      });
      priceLinesRef.current.push(pl);
    }

    const signals = data.signals ?? [];
    const signalMarkers = signals.map((s) => {
        const rawT = toUnix(s.time) as number;
        const t = clampBarTime(rawT);
        const eod = (s.reason ?? '').toLowerCase().includes('eod');
        const clipped = rawT !== t;
        let text = s.direction.toUpperCase();
        if (eod) text = clipped ? `${text} EOD*` : `${text} EOD`;
        return {
          time: t as UTCTimestamp,
          position: s.direction === 'buy' ? ('belowBar' as const) : ('aboveBar' as const),
          color: s.direction === 'buy' ? chartTheme.up : chartTheme.down,
          shape: s.direction === 'buy' ? ('arrowUp' as const) : ('arrowDown' as const),
          text,
        };
      });
    const executionMarkers = events
      .filter((e) => e.type === 'execution' && e.avg_price && e.avg_price > 0)
      .map((e) => {
        const rawT = toUnix(e.time) as number;
        const t = clampBarTime(rawT);
        const status = (e.status ?? '').toLowerCase();
        const filled = status.includes('filled');
        return {
          time: t as UTCTimestamp,
          position: 'inBar' as const,
          color: filled ? chartTheme.fill : chartTheme.text,
          shape: 'circle' as const,
          text: `FILL ${e.filled_qty ?? 0} @ ${e.avg_price?.toFixed(2)}`,
        };
      });
    const markers = [...signalMarkers, ...executionMarkers].sort((a, b) => Number(a.time) - Number(b.time));
    if (markers.length > 0) {
      if (markersRef.current) {
        markersRef.current.setMarkers(markers);
      } else {
        markersRef.current = createSeriesMarkers(series, markers);
      }
    } else if (markersRef.current) {
      markersRef.current.setMarkers([]);
    }

    requestAnimationFrame(() => {
      const chart = chartRef.current;
      if (!chart) return;
      const ts = chart.timeScale();
      if (isFirstDataPaintRef.current) {
        ts.fitContent();
        isFirstDataPaintRef.current = false;
      }
      ts.scrollToRealTime();
    });
  }, [data, events, portfolio, chartTheme]);

  const buyCount = data?.signals?.filter((s) => s.direction === 'buy').length ?? 0;
  const sellCount = data?.signals?.filter((s) => s.direction === 'sell').length ?? 0;
  const total = buyCount + sellCount;

  const posLabel =
    data?.position === 1 ? 'LONG' : data?.position === -1 ? 'SHORT' : 'FLAT';
  const posColor =
    data?.position === 1
      ? 'text-green-400'
      : data?.position === -1
        ? 'text-red-400'
        : 'text-gray-400';

  const brokerPositions = portfolio?.positions?.filter((p) => p.in_instance && p.quantity !== 0) ?? [];
  const brokerLabel = brokerPositions.length
    ? brokerPositions.map((p) => `${p.quantity > 0 ? 'LONG' : 'SHORT'} ${p.quantity} ${p.ticker || p.instrument_uid.slice(0, 8)}`).join(', ')
    : 'FLAT';
  const brokerColor = brokerPositions.length ? 'text-blue-300' : 'text-gray-400';

  const orbMode = data ? isORB(data) : false;
  const lbMode = data ? isLevelBounce(data) : false;
  const smaMode = data ? hasSMA(data) : false;

  const intervalHint =
    data?.chart_interval_param ||
    (data?.chart_subscription_interval
      ? data.chart_subscription_interval.replace(/^SUBSCRIPTION_INTERVAL_/, '')
      : '');

  let headerLabel = '';
  if (lbMode && data) {
    const { support, resistance } = levelBounceLevels(data);
    const atr = data.atr ?? 0;
    headerLabel = `Level Bounce · levels ${data.level_days ?? '?'}d · ATR ${atr > 0 ? atr.toFixed(2) : '—'} · ${support.length}S / ${resistance.length}R`;
  } else if (orbMode && data) {
    headerLabel = `ORB (Range ${data.range_formed ? 'formed' : 'forming'}: ${data.range_high?.toFixed(2) ?? '?'} / ${data.range_low?.toFixed(2) ?? '?'})`;
  } else if (smaMode && data) {
    headerLabel = `SMA(${data.fast_period ?? '?'}/${data.slow_period ?? '?'})`;
    if (data.trailing_stop_pct && data.trailing_stop_pct > 0) {
      headerLabel += ` · trailing ${(data.trailing_stop_pct * 100).toFixed(2)}%`;
    }
  } else if (data) {
    headerLabel = data.strategy_type ?? 'Strategy';
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2 px-1">
        <div className="text-sm text-[var(--muted)]">
          {headerLabel}
          {intervalHint && (
            <>
              {' '}
              &middot; <span className="text-[var(--muted)]">свеча: {intervalHint}</span>
            </>
          )}
          {' '}
          &middot;{' '}
          Signals: <span className="font-medium text-[var(--text)]">{total}</span>{' '}
          <span className="text-[var(--success)]">({buyCount} buy</span> /{' '}
          <span className="text-[var(--danger)]">{sellCount} sell)</span>
        </div>
        <div className="text-right text-sm">
          <div className={`font-semibold ${brokerColor}`}>Broker: {brokerLabel}</div>
          <div className={`text-xs ${posColor}`}>Strategy state: {posLabel}</div>
        </div>
      </div>
      {lbMode && (
        <div className="text-xs text-gray-500 mb-1 px-1 space-y-1">
          <p>
            Уровни: {data?.level_method ?? 'top daily highs/lows'}. Зелёные пунктиры — поддержка (S1–S3),
            красные — сопротивление (R1–R3).
          </p>
          {data?.support_sources?.length ? (
            <p className="text-emerald-400/80">{formatLevelSources('Support sources', data.support_sources)}</p>
          ) : null}
          {data?.resistance_sources?.length ? (
            <p className="text-red-400/80">
              {formatLevelSources('Resistance sources', data.resistance_sources)}
            </p>
          ) : null}
          <p>
            Позиция справа — внутреннее состояние стратегии; в Stats «Positions» — факт по счёту у брокера
            (может отличаться до исполнения ордеров). Маркеры строятся по времени свечи; «EOD*» — сигнал
            после последней свечи на графике, стрелка показана на последнем баре.
          </p>
        </div>
      )}
      {smaMode && data?.trailing_stop_pct && data.trailing_stop_pct > 0 ? (
        <div className="text-xs text-gray-500 mb-1 px-1">
          <span className="text-orange-300">Trailing stop:</span>{' '}
          {data.trailing_stop_active && data.trailing_stop_price
            ? `active at ${data.trailing_stop_price.toFixed(4)} (best ${data.trailing_best_price?.toFixed(4) ?? '—'})`
            : 'configured, waiting for an open position'}
        </div>
      ) : null}
      <div className="relative">
        {hover && (
          <div className="absolute left-3 top-3 z-10 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-xs text-[var(--text)] shadow">
            {hover}
          </div>
        )}
        <div ref={containerRef} className="rounded-lg overflow-hidden border border-[var(--border)]" />
      </div>
    </div>
  );
}
