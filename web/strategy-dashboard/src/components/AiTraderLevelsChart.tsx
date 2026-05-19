import { useMemo } from 'react';
import type { AiTraderCandleBar, AiTraderSession } from '../api/types';
import { useI18n } from '../i18n';
import { formatNumber } from '../format';
import { Card } from './ui';

interface Props {
  session: AiTraderSession;
}

function levelColor(kind: string): string {
  if (kind === 'support') return 'var(--success, #22c55e)';
  if (kind === 'resistance') return 'var(--danger, #ef4444)';
  return 'var(--muted)';
}

export function AiTraderLevelsChart({ session }: Props) {
  const { t, lang } = useI18n();
  const bars = session.market_context?.chart_bars ?? [];
  const levels = session.market_context?.levels ?? session.level_playbook?.levels ?? [];
  const mid = session.features?.mid ?? session.phase_progress?.buffer_stats?.mid;
  const last =
    session.phase_progress?.buffer_stats?.last_price ??
    session.market_context?.tape_stats?.last_price ??
    mid;

  const layout = useMemo(() => {
    if (bars.length === 0 && levels.length === 0) return null;
    const prices: number[] = [];
    for (const b of bars) {
      prices.push(b.low, b.high, b.close);
    }
    for (const l of levels) {
      if (l.price > 0) prices.push(l.price);
    }
    if (last && last > 0) prices.push(last);
    if (mid && mid > 0) prices.push(mid);
    if (prices.length === 0) return null;
    let minP = Math.min(...prices);
    let maxP = Math.max(...prices);
    const pad = (maxP - minP) * 0.08 || maxP * 0.001 || 0.01;
    minP -= pad;
    maxP += pad;
    const range = maxP - minP || 1;
    const w = 640;
    const h = 200;
    const y = (p: number) => h - ((p - minP) / range) * h;
    return { minP, maxP, w, h, y, bars: bars.slice(-30), levels };
  }, [bars, levels, last, mid]);

  if (!layout) {
    return (
      <Card title={t('levelsChartTitle')} subtitle={t('levelsChartEmpty')}>
        <p className="text-sm text-[var(--muted)]">{t('levelsChartWaiting')}</p>
      </Card>
    );
  }

  const barW = Math.max(2, layout.w / Math.max(layout.bars.length, 1) - 2);

  return (
    <Card title={t('levelsChartTitle')} subtitle={t('levelsChartHelp')}>
      <svg
        className="ai-trader-levels-chart"
        viewBox={`0 0 ${layout.w} ${layout.h}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={t('levelsChartTitle')}
      >
        {layout.levels.map((lv) => (
          <g key={`${lv.kind}-${lv.price}-${lv.source}`}>
            <line
              x1={0}
              y1={layout.y(lv.price)}
              x2={layout.w}
              y2={layout.y(lv.price)}
              stroke={levelColor(lv.kind)}
              strokeWidth={1}
              strokeDasharray={lv.source.startsWith('hourly') ? '4 3' : '6 2'}
              opacity={0.75}
            />
          </g>
        ))}
        {layout.bars.map((b: AiTraderCandleBar, i: number) => {
          const x = i * (barW + 2) + 4;
          const openY = layout.y(b.open);
          const closeY = layout.y(b.close);
          const highY = layout.y(b.high);
          const lowY = layout.y(b.low);
          const up = b.close >= b.open;
          const color = up ? 'var(--success, #22c55e)' : 'var(--danger, #ef4444)';
          return (
            <g key={b.time}>
              <line x1={x + barW / 2} y1={highY} x2={x + barW / 2} y2={lowY} stroke={color} strokeWidth={1} />
              <rect
                x={x}
                y={Math.min(openY, closeY)}
                width={barW}
                height={Math.max(2, Math.abs(closeY - openY))}
                fill={color}
                opacity={0.85}
              />
            </g>
          );
        })}
        {last != null && last > 0 && (
          <line
            x1={0}
            y1={layout.y(last)}
            x2={layout.w}
            y2={layout.y(last)}
            stroke="var(--accent)"
            strokeWidth={2}
          />
        )}
      </svg>
      <div className="mt-2 flex flex-wrap gap-3 text-xs font-mono text-[var(--muted)]">
        {last != null && last > 0 && (
          <span>
            {t('currentPrice')}: <strong className="text-[var(--accent)]">{formatNumber(last, lang, 4)}</strong>
          </span>
        )}
        {mid != null && mid > 0 && (
          <span>mid: {formatNumber(mid, lang, 4)}</span>
        )}
        <span>{t('levelsLegendDaily')}</span>
        <span className="opacity-70">{t('levelsLegendHourly')}</span>
      </div>
    </Card>
  );
}
