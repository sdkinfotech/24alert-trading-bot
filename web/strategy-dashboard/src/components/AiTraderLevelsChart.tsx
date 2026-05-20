import { useMemo } from 'react';
import type { AiTraderCandleBar, AiTraderSession } from '../api/types';
import { useI18n } from '../i18n';
import { formatNumber } from '../format';
import { Badge, Card } from './ui';
import {
  barPriceRange,
  enrichLevels,
  pickChartLevels,
  shortLevelSource,
  type EnrichedLevel,
} from './aiTraderLevelUtils';

interface Props {
  session: AiTraderSession;
}

const CHART_W = 720;
const CHART_H = 280;
const PAD_LEFT = 52;
const PAD_RIGHT = 88;

function levelStroke(lv: EnrichedLevel): string {
  if (lv.kind === 'support') return '#22c55e';
  if (lv.kind === 'resistance') return '#ef4444';
  return '#94a3b8';
}

function levelDash(source: string): string {
  if (source.startsWith('hourly')) return '5 4';
  if (source.startsWith('daily')) return '10 4';
  return '4 2';
}

export function AiTraderLevelsChart({ session }: Props) {
  const { t, lang } = useI18n();
  const bars = session.market_context?.chart_bars ?? [];
  const rawLevels = session.market_context?.levels ?? session.level_playbook?.levels ?? [];
  const refPrice =
    session.phase_progress?.buffer_stats?.last_price ??
    session.market_context?.tape_stats?.last_price ??
    session.features?.mid ??
    0;

  const enriched = useMemo(() => enrichLevels(rawLevels, refPrice), [rawLevels, refPrice]);
  const preferredPrices = useMemo(() => {
    const set = new Set<number>();
    for (const lv of session.active_policy?.preferred_levels ?? []) {
      if (lv.price > 0) set.add(lv.price);
    }
    return set;
  }, [session.active_policy?.preferred_levels]);
  const refreshLabel = session.last_playbook_refresh_at
    ? new Date(session.last_playbook_refresh_at).toLocaleTimeString('ru-RU')
    : null;

  const layout = useMemo(() => {
    const recent = bars.slice(-45);
    const br = barPriceRange(recent);
    if (!br && enriched.length === 0) return null;

    let minP = br?.min ?? refPrice * 0.998;
    let maxP = br?.max ?? refPrice * 1.002;
    if (refPrice > 0) {
      minP = Math.min(minP, refPrice);
      maxP = Math.max(maxP, refPrice);
    }
    const span = maxP - minP || refPrice * 0.002 || 0.01;
    const pad = Math.max(span * 0.12, refPrice * 0.0008);
    minP -= pad;
    maxP += pad;
    const range = maxP - minP;
    const plotH = CHART_H - 24;
    const plotW = CHART_W - PAD_LEFT - PAD_RIGHT;
    const y = (p: number) => 12 + plotH - ((p - minP) / range) * plotH;

    const visible = pickChartLevels(enriched, refPrice, minP, maxP);
    const tickCount = 5;
    const ticks: number[] = [];
    for (let i = 0; i < tickCount; i++) {
      ticks.push(minP + (range * i) / (tickCount - 1));
    }

    const nearestSupport = enriched.filter((l) => l.price <= refPrice).sort((a, b) => b.price - a.price)[0];
    const nearestResistance = enriched.filter((l) => l.price > refPrice).sort((a, b) => a.price - b.price)[0];

    return {
      minP,
      maxP,
      y,
      plotW,
      plotH,
      recent,
      visible,
      ticks,
      nearestSupport,
      nearestResistance,
    };
  }, [bars, enriched, refPrice]);

  if (!layout || refPrice <= 0) {
    return (
      <Card title={t('levelsChartTitle')} subtitle={t('levelsChartWaiting')}>
        <p className="text-sm text-[var(--muted)]">{t('levelsChartWaiting')}</p>
      </Card>
    );
  }

  const barW = Math.max(3, layout.plotW / Math.max(layout.recent.length, 1) - 1.5);

  return (
    <Card
      title={t('levelsChartTitle')}
      subtitle={
        refreshLabel
          ? `${t('levelsChartZoomNote')} · ${t('playbookRefreshedAt')} ${refreshLabel}`
          : t('levelsChartZoomNote')
      }
    >
      <div className="ai-trader-levels-wrap">
        <svg
          className="ai-trader-levels-chart"
          viewBox={`0 0 ${CHART_W} ${CHART_H}`}
          preserveAspectRatio="xMidYMid meet"
          role="img"
          aria-label={t('levelsChartTitle')}
        >
          {layout.ticks.map((tick) => (
            <g key={tick}>
              <line
                x1={PAD_LEFT}
                y1={layout.y(tick)}
                x2={PAD_LEFT + layout.plotW}
                y2={layout.y(tick)}
                stroke="var(--border)"
                strokeWidth={0.5}
                opacity={0.5}
              />
              <text
                x={PAD_LEFT - 6}
                y={layout.y(tick) + 4}
                textAnchor="end"
                className="ai-trader-chart-axis"
              >
                {formatNumber(tick, lang, 2)}
              </text>
            </g>
          ))}

          {layout.visible.map((lv) => {
            const yy = layout.y(lv.price);
            const preferred = preferredPrices.has(lv.price);
            return (
              <g key={`${lv.kind}-${lv.price}-${lv.source}`}>
                <line
                  x1={PAD_LEFT}
                  y1={yy}
                  x2={PAD_LEFT + layout.plotW}
                  y2={yy}
                  stroke={preferred ? 'var(--accent)' : levelStroke(lv)}
                  strokeWidth={preferred ? 2.5 : lv.source.startsWith('daily') ? 1.5 : 1}
                  strokeDasharray={preferred ? undefined : levelDash(lv.source)}
                  opacity={preferred ? 1 : 0.9}
                />
                <text
                  x={PAD_LEFT + layout.plotW + 4}
                  y={yy + 4}
                  className="ai-trader-chart-level-label"
                  fill={levelStroke(lv)}
                >
                  {shortLevelSource(lv.source)} {formatNumber(lv.price, lang, 2)}
                </text>
              </g>
            );
          })}

          {layout.recent.map((b: AiTraderCandleBar, i: number) => {
            const x = PAD_LEFT + i * (barW + 1.5);
            const openY = layout.y(b.open);
            const closeY = layout.y(b.close);
            const highY = layout.y(b.high);
            const lowY = layout.y(b.low);
            const up = b.close >= b.open;
            const color = up ? '#22c55e' : '#ef4444';
            const bodyH = Math.max(2, Math.abs(closeY - openY));
            return (
              <g key={b.time}>
                <line x1={x + barW / 2} y1={highY} x2={x + barW / 2} y2={lowY} stroke={color} strokeWidth={1.2} />
                <rect
                  x={x}
                  y={Math.min(openY, closeY)}
                  width={barW}
                  height={bodyH}
                  fill={color}
                  opacity={0.9}
                />
              </g>
            );
          })}

          <line
            x1={PAD_LEFT}
            y1={layout.y(refPrice)}
            x2={PAD_LEFT + layout.plotW}
            y2={layout.y(refPrice)}
            stroke="var(--accent)"
            strokeWidth={2.5}
          />
          <text
            x={PAD_LEFT + layout.plotW + 4}
            y={layout.y(refPrice) + 4}
            className="ai-trader-chart-price-label"
          >
            {t('currentPrice')} {formatNumber(refPrice, lang, 2)}
          </text>
        </svg>

        <div className="ai-trader-nearest-levels">
          {layout.nearestSupport && (
            <p className="text-sm">
              <span className="text-[var(--success)] font-semibold">{t('nearestSupport')}:</span>{' '}
              {formatNumber(layout.nearestSupport.price, lang, 4)}{' '}
              <span className="text-[var(--muted)]">
                ({formatNumber(Math.abs(layout.nearestSupport.dist_bps), lang, 1)} bps {t('levelBelow')})
              </span>
            </p>
          )}
          {layout.nearestResistance && (
            <p className="text-sm">
              <span className="text-[var(--danger)] font-semibold">{t('nearestResistance')}:</span>{' '}
              {formatNumber(layout.nearestResistance.price, lang, 4)}{' '}
              <span className="text-[var(--muted)]">
                ({formatNumber(Math.abs(layout.nearestResistance.dist_bps), lang, 1)} bps {t('levelAbove')})
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
              {enriched.slice(0, 12).map((lv) => (
                <tr key={`${lv.kind}-${lv.price}-${lv.source}`}>
                  <td>
                    <Badge tone={lv.kind === 'support' ? 'success' : lv.kind === 'resistance' ? 'danger' : 'neutral'}>
                      {lv.kind}
                    </Badge>
                  </td>
                  <td className="font-mono">{formatNumber(lv.price, lang, 4)}</td>
                  <td className="text-[var(--muted)] text-xs">{shortLevelSource(lv.source)}</td>
                  <td className="font-mono">
                    {lv.dist_bps >= 0 ? '−' : '+'}
                    {formatNumber(Math.abs(lv.dist_bps), lang, 1)} bps
                    <span className="text-[var(--muted)] ml-1">
                      ({lv.dist_bps > 2 ? t('levelBelow') : lv.dist_bps < -2 ? t('levelAbove') : '≈'})
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </Card>
  );
}
