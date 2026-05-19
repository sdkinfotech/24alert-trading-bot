import type { AiTraderSession } from '../api/types';
import { useI18n } from '../i18n';
import { formatNumber } from '../format';
import { Badge, Card } from './ui';

interface Props {
  session: AiTraderSession;
  onScrollToAdvisor?: () => void;
}

export function AiTraderStrategyBrief({ session, onScrollToAdvisor }: Props) {
  const { t, lang } = useI18n();
  const pb = session.level_playbook;
  const pp = session.phase_progress;
  const show =
    session.phase === 'ready' ||
    session.phase === 'trading' ||
    (session.phase === 'analyzing' && pb && pb.levels && pb.levels.length >= 2);

  if (!show || !pb) return null;

  const collectLine =
    pp && pp.collect_seconds >= pp.min_collect_sec
      ? t('pipelineCollectDone')
      : `${t('pipelineCollectPending')}: ${pp?.collect_seconds ?? 0}/${pp?.min_collect_sec ?? 60}s`;
  const pipeline = [
    collectLine,
    pp?.reports_ready?.includes('15m') ? t('pipeline15mOk') : t('pipeline15mPending'),
    pp?.trading_ready ? t('pipelinePlaybookOk') : t('pipelinePlaybookPending'),
  ];

  return (
    <Card
      title={t('strategyBriefTitle')}
      subtitle={pb.summary}
      actions={
        <Badge tone={session.phase === 'ready' ? 'success' : 'info'}>
          {t('strategyKindLevelIntraday')}
        </Badge>
      }
    >
      <ul className="ai-trader-pipeline mb-4">
        {pipeline.map((line) => (
          <li key={line}>{line}</li>
        ))}
      </ul>

      {pb.market_bias && (
        <p className="text-sm mb-2">
          {t('marketBias')}: <Badge tone={pb.market_bias === 'bullish' ? 'success' : pb.market_bias === 'bearish' ? 'warning' : 'neutral'}>{pb.market_bias}</Badge>
        </p>
      )}

      {pb.entry_rules && pb.entry_rules.length > 0 && (
        <section className="mb-3">
          <h4 className="text-xs font-semibold text-[var(--muted)] mb-1">{t('entryRules')}</h4>
          <ul className="text-sm space-y-1 list-disc pl-4">
            {pb.entry_rules.map((r, i) => (
              <li key={i}>{r}</li>
            ))}
          </ul>
        </section>
      )}

      {pb.risk_notes && pb.risk_notes.length > 0 && (
        <section className="mb-3">
          <h4 className="text-xs font-semibold text-[var(--muted)] mb-1">{t('riskNotes')}</h4>
          <ul className="text-sm space-y-1 list-disc pl-4 text-[var(--muted)]">
            {pb.risk_notes.map((r, i) => (
              <li key={i}>{r}</li>
            ))}
          </ul>
        </section>
      )}

      <p className="text-xs font-mono text-[var(--muted)] mb-3">
        SL: {pb.sl_mult_atr ?? 0.5}×ATR · TP: {pb.tp_mult_atr ?? 1.5}×ATR · {t('paperLimitsHint')}
      </p>

      {pb.levels && pb.levels.length > 0 && (
        <ul className="text-sm font-mono space-y-0.5 mb-3">
          {pb.levels.slice(0, 10).map((lv) => (
            <li key={`${lv.kind}-${lv.price}`}>
              {lv.kind} {formatNumber(lv.price, lang, 4)} <span className="text-[var(--muted)]">({lv.source})</span>
            </li>
          ))}
        </ul>
      )}

      {onScrollToAdvisor && (
        <button
          type="button"
          className="text-sm text-[var(--accent)] hover:underline"
          onClick={onScrollToAdvisor}
        >
          {t('openAdvisorReports')}
        </button>
      )}
    </Card>
  );
}
