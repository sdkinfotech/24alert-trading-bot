import type { AiTraderExecutionLogEntry, AiTraderSession } from '../api/types';
import { useI18n, type Lang } from '../i18n';
import { EM_DASH, formatDateTime, formatNumber } from '../format';
import { Badge, Card, EmptyState } from './ui';

interface Props {
  session: AiTraderSession;
}

function kindTone(kind: string): 'success' | 'danger' | 'info' {
  if (kind === 'entry') return 'success';
  if (kind === 'exit') return 'danger';
  return 'info';
}

function triggerLabel(trigger: string, t: (k: string) => string): string {
  const key = `execTrigger_${trigger}`;
  const v = t(key);
  return v !== key ? v : trigger;
}

function EntryRow({ e, lang, t }: { e: AiTraderExecutionLogEntry; lang: Lang; t: (k: string) => string }) {
  return (
    <li className="ai-exec-log-item">
      <div className="ai-exec-log-head">
        <Badge tone={kindTone(e.kind)}>{e.kind === 'entry' ? t('execEntry') : t('execExit')}</Badge>
        <span className="font-mono text-sm">
          {e.side.toUpperCase()} {formatNumber(e.quantity, lang, 0)} @ {formatNumber(e.price, lang, 4)}
        </span>
        <span className="text-xs text-[var(--muted)]">{formatDateTime(e.time, lang)}</span>
        <Badge tone="info">{triggerLabel(e.trigger, t)}</Badge>
      </div>
      {(e.llm_summary || e.llm_reason || e.trade_signal_reason) && (
        <div className="ai-exec-log-llm">
          <p className="text-xs font-semibold text-[var(--muted)] mb-1">{t('execLlmBasis')}</p>
          {e.llm_bias && (
            <span className="mr-2">
              {t('aiDeskPlanBias')}: <Badge tone="info">{e.llm_bias}</Badge>
            </span>
          )}
          {e.llm_confidence != null && e.llm_confidence > 0 && (
            <span className="text-xs text-[var(--muted)] mr-2">
              conf {formatNumber(e.llm_confidence, lang, 2)}
            </span>
          )}
          {e.llm_intent && (
            <p className="text-xs text-[var(--muted)]">
              {t('execIntent')}: {e.llm_intent}
            </p>
          )}
          {e.llm_summary && <p className="text-sm text-[var(--text)] mt-1">{e.llm_summary}</p>}
          {!e.llm_summary && e.llm_reason && <p className="text-sm text-[var(--text)] mt-1">{e.llm_reason}</p>}
          {e.trade_signal_reason && (
            <p className="text-xs text-[var(--muted)] mt-1">
              {t('lastSignal')}: {e.trade_signal_reason}
            </p>
          )}
        </div>
      )}
      <div className="ai-exec-log-meta text-xs text-[var(--muted)] mt-1">
        {e.position_after !== 0 && (
          <span>
            {t('execPosAfter')}: {e.position_after > 0 ? 'LONG' : 'SHORT'} {Math.abs(e.position_after)}
          </span>
        )}
        {(e.stop_loss || e.take_profit) && (
          <span className="ml-3">
            SL {e.stop_loss ? formatNumber(e.stop_loss, lang, 4) : EM_DASH} · TP{' '}
            {e.take_profit ? formatNumber(e.take_profit, lang, 4) : EM_DASH}
          </span>
        )}
        {e.realized_rub != null && e.kind === 'exit' && (
          <span className="ml-3">
            {t('realized')}: {formatNumber(e.realized_rub, lang, 2)} ₽
          </span>
        )}
      </div>
    </li>
  );
}

export function AiTraderExecutionLog({ session }: Props) {
  const { t, lang } = useI18n();
  const log = session.execution_log ?? [];

  return (
    <Card title={t('execLogTitle')} subtitle={t('execLogHelp')}>
      {log.length === 0 ? (
        <EmptyState>{t('execLogEmpty')}</EmptyState>
      ) : (
        <ul className="ai-exec-log-list space-y-3 max-h-[420px] overflow-y-auto pr-1">
          {log.map((e, i) => (
            <EntryRow key={`${e.time}-${i}`} e={e} lang={lang} t={t} />
          ))}
        </ul>
      )}
    </Card>
  );
}
