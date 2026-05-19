import { useMemo } from 'react';
import type { AiTraderCollectEvent, AiTraderDecisionEvent, AiTraderSession } from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime } from '../format';
import { Badge, Card, EmptyState } from './ui';

type StreamItem =
  | { kind: 'feed'; time: string; data: AiTraderCollectEvent }
  | { kind: 'event'; time: string; data: AiTraderDecisionEvent };

interface Props {
  session: AiTraderSession;
}

function ThoughtEntry({ ev, lang, t }: { ev: AiTraderDecisionEvent; lang: 'ru' | 'en'; t: (k: string) => string }) {
  const isPipeline = ev.intent === 'pipeline' || ev.analysis_source === 'session';
  const analysisLabel = ev.analysis_source === 'llm'
    ? t('analysisLLM')
    : ev.analysis_source === 'rules_fallback'
      ? t('analysisFallback')
      : isPipeline
        ? t('analysisSession')
        : t('analysisRules');

  return (
    <article className={`ai-trader-thought ${isPipeline ? 'ai-trader-thought-pipeline' : ''}`}>
      <header className="ai-trader-thought-header">
        <time className="ai-trader-thought-time">{formatDateTime(ev.time, lang)}</time>
        <div className="ai-trader-thought-badges">
          <Badge tone={ev.action === 'block' ? 'danger' : ev.action.includes('plan') || ev.action.includes('ready') ? 'warning' : 'info'}>
            {ev.action}
          </Badge>
          {ev.market_bias && (
            <Badge tone={ev.market_bias === 'bullish' ? 'success' : ev.market_bias === 'bearish' ? 'warning' : 'neutral'}>
              {ev.market_bias}
            </Badge>
          )}
          <Badge tone={ev.analysis_source === 'llm' ? 'info' : 'neutral'}>{analysisLabel}</Badge>
        </div>
      </header>
      <p className="ai-trader-thought-body">{ev.summary ?? ev.reason}</p>
      {ev.reason && ev.summary && ev.reason !== ev.summary && (
        <p className="ai-trader-thought-reason">{ev.reason}</p>
      )}
    </article>
  );
}

function FeedLine({ ev, lang }: { ev: AiTraderCollectEvent; lang: 'ru' | 'en' }) {
  return (
    <article className="ai-trader-feed-inline">
      <time className="ai-trader-feed-time">{formatDateTime(ev.time, lang)}</time>
      <Badge tone="neutral">{ev.kind}</Badge>
      <p className="ai-trader-feed-msg">{ev.message}</p>
      {ev.detail && <p className="ai-trader-feed-detail">{ev.detail}</p>}
    </article>
  );
}

export function AiTraderActivityStream({ session }: Props) {
  const { t, lang } = useI18n();

  const items = useMemo(() => {
    const out: StreamItem[] = [];
    for (const ev of session.collect_feed ?? []) {
      out.push({ kind: 'feed', time: ev.time, data: ev });
    }
    for (const ev of session.events ?? []) {
      out.push({ kind: 'event', time: ev.time, data: ev });
    }
    out.sort((a, b) => (a.time < b.time ? 1 : a.time > b.time ? -1 : 0));
    return out.slice(0, 120);
  }, [session.collect_feed, session.events]);

  return (
    <Card title={t('aiTraderThoughtStream')} subtitle={t('aiTraderActivityHelp')}>
      {session.last_error && <p className="mb-3 text-sm text-[var(--danger)]">{session.last_error}</p>}
      {items.length > 0 ? (
        <div className="ai-trader-thought-stream">
          {items.map((item, i) =>
            item.kind === 'feed' ? (
              <FeedLine key={`f-${item.time}-${i}`} ev={item.data} lang={lang} />
            ) : (
              <ThoughtEntry key={`e-${item.time}-${item.data.action}-${i}`} ev={item.data} lang={lang} t={t} />
            ),
          )}
        </div>
      ) : (
        <EmptyState>{t('noEvents')}</EmptyState>
      )}
    </Card>
  );
}
