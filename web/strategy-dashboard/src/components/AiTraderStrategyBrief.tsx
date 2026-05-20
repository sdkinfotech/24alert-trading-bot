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
  const st = session.session_strategy;
  const pb = session.level_playbook;
  const pp = session.phase_progress;

  if (!st && !pb) return null;

  const pipeline = [
    pp
      ? pp.collect_seconds >= pp.min_collect_sec
        ? t('pipelineCollectDone')
        : `${t('pipelineCollectPending')}: ${pp.collect_seconds}/${pp.min_collect_sec}s`
      : null,
    pp?.reports_ready?.includes('15m') ? t('pipeline15mOk') : t('pipeline15mPending'),
    pp?.strategy_ready ? t('pipelineStrategyOk') : t('pipelineStrategyPending'),
    pp?.trading_ready ? t('pipelinePlaybookOk') : t('pipelinePlaybookPending'),
  ].filter(Boolean) as string[];

  return (
    <Card
      title={st ? t('sessionStrategyTitle') : t('strategyBriefTitle')}
      subtitle={st?.hypothesis ?? pb?.summary}
      actions={
        st?.status === 'active' ? (
          <Badge tone="success">{t('sessionStrategyActive')}</Badge>
        ) : (
          <Badge tone="warning">{t('sessionStrategyForming')}</Badge>
        )
      }
    >
      <ul className="ai-trader-pipeline mb-4">
        {pipeline.map((line) => (
          <li key={line}>{line}</li>
        ))}
      </ul>

      {st && (
        <>
          {st.participants && (
            <p className="text-sm mb-2 text-[var(--muted)]">
              <strong>{t('sessionParticipants')}:</strong> {st.participants}
            </p>
          )}
          {st.tactics && (
            <p className="text-sm mb-2">
              {t('sessionTactics')}: <Badge tone="info">{st.tactics}</Badge>
              {' '}
              long={st.allow_long ? '✓' : '✗'} short={st.allow_short ? '✓' : '✗'}
            </p>
          )}
          {st.rules && st.rules.length > 0 && (
            <section className="mb-3">
              <h4 className="text-xs font-semibold text-[var(--muted)] mb-1">{t('sessionRules')}</h4>
              <ul className="text-sm space-y-1 list-disc pl-4">
                {st.rules.map((r, i) => (
                  <li key={i}>{r}</li>
                ))}
              </ul>
            </section>
          )}
          {st.key_levels && st.key_levels.length > 0 && (
            <ul className="text-sm font-mono space-y-0.5 mb-3">
              {st.key_levels.map((lv) => (
                <li key={`${lv.kind}-${lv.price}`}>
                  {lv.kind} {formatNumber(lv.price, lang, 4)}{' '}
                  <span className="text-[var(--muted)]">({lv.source})</span>
                </li>
              ))}
            </ul>
          )}
        </>
      )}

      {session.micro_signals && session.micro_signals.length > 0 && (
        <section className="mb-3">
          <h4 className="text-xs font-semibold text-[var(--muted)] mb-1">{t('microSignals')}</h4>
          <ul className="text-xs font-mono space-y-0.5">
            {session.micro_signals.slice(-5).map((ms, i) => (
              <li key={i}>
                {ms.kind} {ms.side} @ {formatNumber(ms.price ?? 0, lang, 4)} — {ms.detail}
              </li>
            ))}
          </ul>
        </section>
      )}

      {pb?.entry_rules && pb.entry_rules.length > 0 && (
        <section className="mb-3">
          <h4 className="text-xs font-semibold text-[var(--muted)] mb-1">{t('entryRules')}</h4>
          <ul className="text-sm space-y-1 list-disc pl-4 text-[var(--muted)]">
            {pb.entry_rules.map((r, i) => (
              <li key={i}>{r}</li>
            ))}
          </ul>
        </section>
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
