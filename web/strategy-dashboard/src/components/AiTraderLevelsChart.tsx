import { useEffect, useMemo, useRef, useState } from 'react';
import {
  CandlestickSeries,
  ColorType,
  CrosshairMode,
  LineStyle,
  createChart,
  createSeriesMarkers,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type Time,
  type UTCTimestamp,
} from 'lightweight-charts';
import type { AiTraderCandleBar, AiTraderLevel, AiTraderSession } from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime, formatNumber } from '../format';
import { useTheme } from '../theme';
import { Card } from './ui';
import {
  collectSessionChartLevels,
  enrichLevels,
  shortLevelSource,
  type ChartLevelLine,
} from './aiTraderLevelUtils';

interface Props {
  session: AiTraderSession;
}

type ChartTimeframe = '1m' | '5m';

function toUnix(t: string): UTCTimestamp {
  return Math.floor(new Date(t).getTime() / 1000) as UTCTimestamp;
}

function levelLineStyle(lv: ChartLevelLine): {
  color: string;
  lineWidth: 1 | 2 | 3 | 4;
  lineStyle: LineStyle;
  title: string;
} {
  const tag = shortLevelSource(lv.level.source);
  const kind = lv.level.kind === 'support' ? 'S' : lv.level.kind === 'resistance' ? 'R' : '·';
  const title = `${kind} ${tag}`;
  if (lv.tier === 'preferred') {
    return { color: '#38bdf8', lineWidth: 3, lineStyle: LineStyle.Solid, title: `★ ${title}` };
  }
  if (lv.tier === 'strategy') {
    return { color: '#a78bfa', lineWidth: 3, lineStyle: LineStyle.Solid, title: `◎ ${title}` };
  }
  const isDaily = lv.level.source.startsWith('daily');
  const isSupport = lv.level.kind === 'support';
  const color = isSupport ? '#22c55e' : lv.level.kind === 'resistance' ? '#ef4444' : '#94a3b8';
  return {
    color,
    lineWidth: isDaily ? 2 : 1,
    lineStyle: isDaily ? LineStyle.Solid : LineStyle.Dashed,
    title,
  };
}

export function AiTraderLevelsChart({ session }: Props) {
  const { t, lang } = useI18n();
  const { chart: chartTheme } = useTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const markersRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const priceLinesRef = useRef<IPriceLine[]>([]);
  const fitOnceRef = useRef(true);
  const chartSnapRef = useRef<{ tf: ChartTimeframe; len: number; lastTime: number } | null>(null);
  const [hover, setHover] = useState('');
  const [timeframe, setTimeframe] = useState<ChartTimeframe>('1m');

  const bars1m = session.market_context?.chart_bars ?? [];
  const bars5m = session.market_context?.chart_bars_5m ?? [];
  const liveMid = session.features?.mid ?? 0;
  const rawLevels = useMemo(() => {
    const merged: AiTraderLevel[] = [];
    const seen = new Set<string>();
    const add = (list?: AiTraderLevel[]) => {
      for (const lv of list ?? []) {
        const k = `${lv.kind}-${lv.price}-${lv.source}`;
        if (seen.has(k)) continue;
        seen.add(k);
        merged.push(lv);
      }
    };
    add(session.level_playbook?.levels);
    add(session.market_context?.levels);
    add(session.session_strategy?.key_levels);
    add(session.active_policy?.preferred_levels);
    return merged;
  }, [
    session.level_playbook?.levels,
    session.market_context?.levels,
    session.session_strategy?.key_levels,
    session.active_policy?.preferred_levels,
  ]);

  const refPrice =
    session.phase_progress?.buffer_stats?.last_price ??
    session.market_context?.tape_stats?.last_price ??
    session.features?.mid ??
    0;

  const chartLevels = useMemo(
    () =>
      collectSessionChartLevels(rawLevels, refPrice, {
        preferred: session.active_policy?.preferred_levels,
        strategy: session.session_strategy?.key_levels,
      }),
    [rawLevels, refPrice, session.active_policy?.preferred_levels, session.session_strategy?.key_levels],
  );

  const enrichedLadder = useMemo(() => enrichLevels(rawLevels, refPrice), [rawLevels, refPrice]);

  const displayBars = useMemo(() => {
    const src = timeframe === '5m' ? bars5m : bars1m;
    if (src.length === 0) return [];
    const out = src.map((b) => ({ ...b }));
    const mid = liveMid > 0 ? liveMid : refPrice;
    if (mid > 0) {
      const i = out.length - 1;
      out[i] = {
        ...out[i],
        close: mid,
        high: Math.max(out[i].high, mid),
        low: Math.min(out[i].low, mid),
      };
    }
    return out;
  }, [timeframe, bars1m, bars5m, liveMid, refPrice]);

  const refreshLabel = session.last_playbook_refresh_at
    ? new Date(session.last_playbook_refresh_at).toLocaleTimeString(lang === 'ru' ? 'ru-RU' : 'en-US')
    : null;

  useEffect(() => {
    if (!containerRef.current) return;
    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: chartTheme.background },
        textColor: chartTheme.text,
      },
      localization: {
        locale: lang === 'ru' ? 'ru-RU' : 'en-US',
        timeFormatter: (time: number | string) =>
          typeof time === 'number' ? formatDateTime(time, lang) : String(time),
      },
      grid: {
        vertLines: { color: chartTheme.grid },
        horzLines: { color: chartTheme.grid },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: chartTheme.border, autoScale: true },
      timeScale: {
        borderColor: chartTheme.border,
        timeVisible: true,
        secondsVisible: false,
        rightOffset: 12,
        barSpacing: 8,
        tickMarkFormatter: (time: number | string) =>
          typeof time === 'number' ? formatDateTime(time, lang) : String(time),
      },
      width: containerRef.current.clientWidth,
      height: 440,
    });
    chartRef.current = chart;
    const series = chart.addSeries(CandlestickSeries, {
      upColor: chartTheme.up,
      downColor: chartTheme.down,
      borderUpColor: chartTheme.up,
      borderDownColor: chartTheme.down,
      wickUpColor: chartTheme.up,
      wickDownColor: chartTheme.down,
    });
    seriesRef.current = series;

    chart.subscribeCrosshairMove((param) => {
      const point = param.seriesData.get(series);
      if (!point || !('open' in point)) {
        setHover('');
        return;
      }
      const bar = point as { open: number; high: number; low: number; close: number };
      setHover(
        `O ${bar.open.toFixed(4)} · H ${bar.high.toFixed(4)} · L ${bar.low.toFixed(4)} · C ${bar.close.toFixed(4)}`,
      );
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
      chartRef.current = null;
      seriesRef.current = null;
      markersRef.current = null;
      priceLinesRef.current = [];
    };
  }, [chartTheme, lang]);

  useEffect(() => {
    const series = seriesRef.current;
    if (!series || displayBars.length === 0) return;

    for (const pl of priceLinesRef.current) {
      series.removePriceLine(pl);
    }
    priceLinesRef.current = [];

    const candles = displayBars.map((b: AiTraderCandleBar) => ({
      time: toUnix(b.time),
      open: b.open,
      high: b.high,
      low: b.low,
      close: b.close,
    }));

    const lastTime = candles.length ? (candles[candles.length - 1].time as number) : 0;
    const prev = chartSnapRef.current;
    if (
      prev &&
      prev.tf === timeframe &&
      prev.len === candles.length &&
      prev.lastTime === lastTime
    ) {
      series.update(candles[candles.length - 1]);
    } else if (prev && prev.tf === timeframe && prev.len + 1 === candles.length) {
      series.update(candles[candles.length - 1]);
    } else {
      series.setData(candles);
      if (!prev || prev.tf !== timeframe) {
        fitOnceRef.current = true;
      }
    }
    chartSnapRef.current = { tf: timeframe, len: candles.length, lastTime };

    const firstBarT = candles.length ? (candles[0].time as number) : 0;
    const lastBarT = candles.length ? (candles[candles.length - 1].time as number) : 0;
    const clampBarTime = (t: number) => {
      if (!candles.length) return t;
      if (t < firstBarT) return firstBarT;
      if (t > lastBarT) return lastBarT;
      return t;
    };

    for (const cl of chartLevels) {
      const st = levelLineStyle(cl);
      const pl = series.createPriceLine({
        price: cl.level.price,
        color: st.color,
        lineWidth: st.lineWidth,
        lineStyle: st.lineStyle,
        axisLabelVisible: true,
        title: `${st.title} ${cl.level.price.toFixed(2)}`,
      });
      priceLinesRef.current.push(pl);
    }

    if (refPrice > 0) {
      const pl = series.createPriceLine({
        price: refPrice,
        color: chartTheme.fast,
        lineWidth: 2,
        lineStyle: LineStyle.Solid,
        axisLabelVisible: true,
        title: t('currentPrice'),
      });
      priceLinesRef.current.push(pl);
    }

    const ls = session.live_state ?? session.paper_state;
    if (ls?.avg_price && ls.avg_price > 0 && (ls.position_lots ?? 0) !== 0) {
      const pl = series.createPriceLine({
        price: ls.avg_price,
        color: chartTheme.brokerAvg,
        lineWidth: 2,
        lineStyle: LineStyle.Dotted,
        axisLabelVisible: true,
        title: `Avg ${ls.position_lots > 0 ? 'LONG' : 'SHORT'}`,
      });
      priceLinesRef.current.push(pl);
    }

    const sl = session.live_state?.stop_loss ?? session.paper_state?.stop_loss;
    const tp = session.live_state?.take_profit ?? session.paper_state?.take_profit;
    if (sl && sl > 0) {
      priceLinesRef.current.push(
        series.createPriceLine({
          price: sl,
          color: chartTheme.down,
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          axisLabelVisible: true,
          title: 'SL',
        }),
      );
    }
    if (tp && tp > 0) {
      priceLinesRef.current.push(
        series.createPriceLine({
          price: tp,
          color: chartTheme.up,
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          axisLabelVisible: true,
          title: 'TP',
        }),
      );
    }

    const sig = session.last_trade_signal;
    if (sig?.level_price && sig.level_price > 0) {
      priceLinesRef.current.push(
        series.createPriceLine({
          price: sig.level_price,
          color: chartTheme.trailing,
          lineWidth: 2,
          lineStyle: LineStyle.Dotted,
          axisLabelVisible: true,
          title: `LLM ${sig.side}`,
        }),
      );
    }

    for (const o of session.live_state?.working_orders ?? session.paper_state?.working_orders ?? []) {
      if (!o.price || o.price <= 0 || o.status === 'cancelled') continue;
      priceLinesRef.current.push(
        series.createPriceLine({
          price: o.price,
          color: o.side === 'buy' ? chartTheme.up : chartTheme.down,
          lineWidth: 1,
          lineStyle: LineStyle.SparseDotted,
          axisLabelVisible: true,
          title: `${o.side} lim`,
        }),
      );
    }

    const markers: {
      time: UTCTimestamp;
      position: 'aboveBar' | 'belowBar' | 'inBar';
      color: string;
      shape: 'circle' | 'arrowUp' | 'arrowDown';
      text?: string;
    }[] = [];

    for (const p of session.market_context?.recent_prints ?? []) {
      if (!p.time || !p.price) continue;
      markers.push({
        time: clampBarTime(toUnix(p.time) as number) as UTCTimestamp,
        position: p.direction === 'buy' ? 'belowBar' : 'aboveBar',
        color: p.direction === 'buy' ? chartTheme.up : chartTheme.down,
        shape: 'circle',
        text: `${p.direction === 'buy' ? 'B' : 'S'} ${p.quantity}`,
      });
    }

    for (const e of session.execution_log ?? []) {
      if (!e.time || !e.price) continue;
      markers.push({
        time: clampBarTime(toUnix(e.time) as number) as UTCTimestamp,
        position: 'inBar',
        color: e.kind === 'entry' ? chartTheme.fill : chartTheme.text,
        shape: e.side === 'buy' ? 'arrowUp' : 'arrowDown',
        text: `${e.kind} ${e.side}`,
      });
    }

    markers.sort((a, b) => Number(a.time) - Number(b.time));
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
      if (fitOnceRef.current) {
        ts.fitContent();
        fitOnceRef.current = false;
      } else {
        ts.scrollToRealTime();
      }
    });
  }, [displayBars, timeframe, chartLevels, refPrice, session, chartTheme, t]);

  if ((bars1m.length === 0 && bars5m.length === 0) || refPrice <= 0) {
    return (
      <Card title={t('levelsChartTitle')} subtitle={t('levelsChartWaiting')}>
        <p className="text-sm text-[var(--muted)]">{t('levelsChartWaiting')}</p>
      </Card>
    );
  }

  const nearestSupport = enrichedLadder.filter((l) => l.price <= refPrice).sort((a, b) => b.price - a.price)[0];
  const nearestResistance = enrichedLadder.filter((l) => l.price > refPrice).sort((a, b) => a.price - b.price)[0];

  return (
    <Card
      title={t('levelsChartTitle')}
      subtitle={
        refreshLabel
          ? `${t('levelsChartTvNote')} · ${t('playbookRefreshedAt')} ${refreshLabel}`
          : t('levelsChartTvNote')
      }
    >
      <div className="flex flex-wrap items-center gap-2 mb-2 px-1">
        <span className="text-xs text-[var(--muted)]">{t('chartTimeframe')}:</span>
        <div className="ai-trader-tf-toggle" role="group" aria-label={t('chartTimeframe')}>
          <button
            type="button"
            className={timeframe === '1m' ? 'ai-trader-tf-btn active' : 'ai-trader-tf-btn'}
            onClick={() => {
              setTimeframe('1m');
              fitOnceRef.current = true;
              chartSnapRef.current = null;
            }}
          >
            1m
            {bars1m.length > 0 && (
              <span className="text-[var(--muted)] ml-1">({bars1m.length})</span>
            )}
          </button>
          <button
            type="button"
            className={timeframe === '5m' ? 'ai-trader-tf-btn active' : 'ai-trader-tf-btn'}
            onClick={() => {
              setTimeframe('5m');
              fitOnceRef.current = true;
              chartSnapRef.current = null;
            }}
          >
            5m
            {bars5m.length > 0 && (
              <span className="text-[var(--muted)] ml-1">({bars5m.length})</span>
            )}
          </button>
        </div>
        <span className="text-xs text-[var(--muted)]">{t('chartHistoryWeek')}</span>
      </div>
      <p className="text-xs text-[var(--muted)] mb-2 px-1">{t('levelsChartLegend')}</p>
      <div className="relative">
        {hover && (
          <div className="absolute left-3 top-3 z-10 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-xs text-[var(--text)] shadow">
            {hover}
          </div>
        )}
        <div
          ref={containerRef}
          className="ai-trader-tv-chart rounded-lg overflow-hidden border border-[var(--border)]"
          role="img"
          aria-label={t('levelsChartTitle')}
        />
      </div>

      <div className="ai-trader-nearest-levels mt-3">
        {nearestSupport && (
          <p className="text-sm">
            <span className="text-[var(--success)] font-semibold">{t('nearestSupport')}:</span>{' '}
            {formatNumber(nearestSupport.price, lang, 4)}{' '}
            <span className="text-[var(--muted)]">
              ({formatNumber(Math.abs(nearestSupport.dist_bps), lang, 1)} bps {t('levelBelow')})
            </span>
          </p>
        )}
        {nearestResistance && (
          <p className="text-sm">
            <span className="text-[var(--danger)] font-semibold">{t('nearestResistance')}:</span>{' '}
            {formatNumber(nearestResistance.price, lang, 4)}{' '}
            <span className="text-[var(--muted)]">
              ({formatNumber(Math.abs(nearestResistance.dist_bps), lang, 1)} bps {t('levelAbove')})
            </span>
          </p>
        )}
      </div>

      <div className="ai-trader-levels-ladder mt-4">
        <h4 className="text-xs font-semibold text-[var(--muted)] mb-2">{t('levelsLadderTitle')}</h4>
        <p className="text-xs text-[var(--muted)] mb-2">{t('levelsLadderHelp')}</p>
        <table className="ai-trader-levels-table">
          <thead>
            <tr>
              <th>{t('levelKind')}</th>
              <th>{t('levelPrice')}</th>
              <th>{t('levelSource')}</th>
              <th>{t('levelDistance')}</th>
            </tr>
          </thead>
          <tbody>
            {enrichedLadder.slice(0, 12).map((lv) => (
              <tr key={`${lv.kind}-${lv.price}-${lv.source}`}>
                <td>
                  <span
                    className={
                      lv.kind === 'support'
                        ? 'text-[var(--success)]'
                        : lv.kind === 'resistance'
                          ? 'text-[var(--danger)]'
                          : 'text-[var(--muted)]'
                    }
                  >
                    {lv.kind}
                  </span>
                </td>
                <td className="font-mono">{formatNumber(lv.price, lang, 4)}</td>
                <td className="text-[var(--muted)] text-xs">{shortLevelSource(lv.source)}</td>
                <td className="font-mono text-xs">
                  {lv.dist_bps >= 0 ? '−' : '+'}
                  {formatNumber(Math.abs(lv.dist_bps), lang, 1)} bps
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
