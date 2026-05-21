import { useEffect, useMemo, useRef } from 'react';
import {
  CandlestickSeries,
  ColorType,
  CrosshairMode,
  LineStyle,
  createChart,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type Time,
  type UTCTimestamp,
} from 'lightweight-charts';
import type { AssistantChartPayload, AssistantLevel } from '../api/types';
import { useTheme } from '../theme';

type ChartTF = '1d' | '1h' | '5m';

function toUnix(t: string): UTCTimestamp {
  return Math.floor(new Date(t).getTime() / 1000) as UTCTimestamp;
}

function levelStyle(lv: AssistantLevel, highlight?: string): {
  color: string;
  lineWidth: 1 | 2 | 3;
  lineStyle: LineStyle;
  title: string;
} {
  const hl = highlight === lv.id;
  if (lv.kind === 'mirror') {
    return {
      color: hl ? '#f472b6' : '#db2777',
      lineWidth: hl ? 3 : 2,
      lineStyle: LineStyle.Solid,
      title: `⇄ ${lv.id}`,
    };
  }
  if (lv.kind === 'poc') {
    return { color: '#a78bfa', lineWidth: 2, lineStyle: LineStyle.Dotted, title: `POC ${lv.id}` };
  }
  const isSupport = lv.kind === 'support';
  return {
    color: isSupport ? '#22c55e' : '#ef4444',
    lineWidth: lv.strength >= 4 ? 2 : 1,
    lineStyle: lv.source.startsWith('daily') ? LineStyle.Solid : LineStyle.Dashed,
    title: `${isSupport ? 'S' : 'R'} ${lv.id}`,
  };
}

interface Props {
  chart?: AssistantChartPayload;
  timeframe: ChartTF;
  highlightLevelId?: string;
}

export function AssistantChart({ chart, timeframe, highlightLevelId }: Props) {
  const { chart: chartTheme } = useTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const linesRef = useRef<IPriceLine[]>([]);

  const bars = chart?.candles ?? [];
  const levels = chart?.levels ?? [];

  const candleData = useMemo(
    () =>
      bars.map((b) => ({
        time: toUnix(b.time) as Time,
        open: b.open,
        high: b.high,
        low: b.low,
        close: b.close,
      })),
    [bars],
  );

  useEffect(() => {
    if (!containerRef.current) return;
    const el = containerRef.current;
    const lc = createChart(el, {
      layout: {
        background: { type: ColorType.Solid, color: chartTheme.background },
        textColor: chartTheme.text,
      },
      grid: {
        vertLines: { color: chartTheme.grid },
        horzLines: { color: chartTheme.grid },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: chartTheme.border },
      timeScale: { borderColor: chartTheme.border },
      width: el.clientWidth,
      height: 420,
    });
    const series = lc.addSeries(CandlestickSeries, {
      upColor: chartTheme.up,
      downColor: chartTheme.down,
      borderVisible: false,
      wickUpColor: chartTheme.up,
      wickDownColor: chartTheme.down,
    });
    chartRef.current = lc;
    seriesRef.current = series;

    const ro = new ResizeObserver(() => {
      lc.applyOptions({ width: el.clientWidth });
    });
    ro.observe(el);
    return () => {
      ro.disconnect();
      lc.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, [chartTheme]);

  useEffect(() => {
    const series = seriesRef.current;
    const lc = chartRef.current;
    if (!series || !lc) return;
    series.setData(candleData);
    for (const pl of linesRef.current) {
      series.removePriceLine(pl);
    }
    linesRef.current = [];
    for (const lv of levels) {
      const st = levelStyle(lv, highlightLevelId);
      linesRef.current.push(
        series.createPriceLine({
          price: lv.price,
          color: st.color,
          lineWidth: st.lineWidth,
          lineStyle: st.lineStyle,
          title: st.title,
        }),
      );
    }
    lc.timeScale().fitContent();
  }, [candleData, levels, highlightLevelId, timeframe]);

  if (!bars.length) {
    return (
      <div className="flex h-[420px] items-center justify-center text-sm text-[var(--muted)]">
        {timeframe} — no candles
      </div>
    );
  }

  return <div ref={containerRef} className="w-full min-h-[420px]" />;
}
