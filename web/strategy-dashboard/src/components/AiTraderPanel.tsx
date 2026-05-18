import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type { AiTraderDecisionEvent, AiTraderSession, Instance } from '../api/types';
import { useI18n } from '../i18n';
import { EM_DASH, formatDateTime, formatNumber } from '../format';
import { Badge, Button, Card, EmptyState, Stat } from './ui';

interface Props {
  instances: Instance[];
}

function analysisTone(source?: string): 'info' | 'warning' | 'neutral' {
  if (source === 'llm') return 'info';
  if (source === 'rules_fallback') return 'warning';
  return 'neutral';
}

function ThoughtEntry({ ev, lang, t }: { ev: AiTraderDecisionEvent; lang: 'ru' | 'en'; t: (k: string) => string }) {
  const analysisLabel = ev.analysis_source === 'llm'
    ? t('analysisLLM')
    : ev.analysis_source === 'rules_fallback'
      ? t('analysisFallback')
      : t('analysisRules');

  return (
    <article className="ai-trader-thought">
      <header className="ai-trader-thought-header">
        <time className="ai-trader-thought-time">{formatDateTime(ev.time, lang)}</time>
        <div className="ai-trader-thought-badges">
          <Badge tone={ev.action === 'block' ? 'danger' : ev.action.includes('plan') ? 'warning' : 'info'}>
            {ev.action}
          </Badge>
          <Badge tone={ev.market_bias === 'blocked' ? 'danger' : ev.market_bias === 'bullish' ? 'success' : ev.market_bias === 'bearish' ? 'warning' : 'neutral'}>
            {ev.market_bias ?? 'neutral'}
          </Badge>
          <Badge tone={analysisTone(ev.analysis_source)}>{analysisLabel}</Badge>
          <span className="ai-trader-thought-conf">{formatNumber(ev.confidence * 100, lang, 0)}%</span>
        </div>
      </header>
      <p className="ai-trader-thought-body">{ev.summary ?? ev.reason}</p>
      {ev.reason && ev.summary && ev.reason !== ev.summary && (
        <p className="ai-trader-thought-reason">{ev.reason}</p>
      )}
      {ev.next_watch && (
        <p className="ai-trader-thought-watch">
          <span className="font-semibold">{t('nextWatch')}:</span> {ev.next_watch}
        </p>
      )}
    </article>
  );
}

export function AiTraderPanel({ instances }: Props) {
  const { t, lang } = useI18n();
  const [selectedKey, setSelectedKey] = useState('');
  const [session, setSession] = useState<AiTraderSession | null>(null);
  const [instruction, setInstruction] = useState(() => t('aiTraderDefaultInstruction'));
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const instruments = useMemo(() => instances.flatMap((inst) =>
    (inst.instruments ?? []).map((uid, index) => ({
      key: `${inst.account_id}:${uid}`,
      accountID: inst.account_id,
      uid,
      ticker: (inst.tickers ?? '').split(',')[index]?.trim() || inst.tickers || uid.slice(0, 8),
      sourceInstanceID: inst.id,
      sourceType: inst.type,
    })),
  ), [instances]);
  const selected = instruments.find((item) => item.key === selectedKey) ?? instruments[0];
  const sessionLookup = session?.id ?? selected?.key ?? '';

  const load = useCallback(async () => {
    if (!sessionLookup) return;
    try {
      const data = await api.aiTraderSession(sessionLookup);
      setSession(data);
      setError(null);
    } catch {
      setSession(null);
    }
  }, [sessionLookup]);

  useEffect(() => {
    const initial = window.setTimeout(() => void load(), 0);
    const timer = window.setInterval(() => void load(), 1500);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(timer);
    };
  }, [load]);

  async function start(mode: 'observe' | 'paper') {
    if (!selected) return;
    setLoading(true);
    try {
      const data = await api.startAiTraderSession({
        account_id: selected.accountID,
        instrument_uid: selected.uid,
        ticker: selected.ticker,
        mode,
        instruction,
        depth: 50,
        limits: {
          max_position_lots: 1,
          max_order_size: 1,
          max_active_orders: 2,
          max_spread_bps: 15,
          stale_data_ms: 1500,
          session_timeout_minutes: 30,
          observation_interval_ms: 2000,
        },
      });
      setSession(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  async function stop() {
    if (!session) return;
    setLoading(true);
    try {
      const data = await api.stopAiTraderSession(session.id);
      setSession(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  const f = session?.features;
  const d = session?.last_decision;

  const thoughtStream = useMemo(() => {
    const events = session?.events ?? [];
    if (events.length > 0) return [...events];
    if (d) return [d];
    return [];
  }, [session?.events, d]);

  return (
    <div className="space-y-6">
      <Card
        title={t('aiTrader')}
        subtitle={t('aiTraderHelp')}
        actions={session ? <Badge tone={session.status === 'running' ? 'success' : 'neutral'}>{session.status}</Badge> : null}
      >
        <div className="grid gap-4 lg:grid-cols-[1fr_auto]">
          <div>
            <label className="text-xs font-semibold text-[var(--muted)]">{t('aiTraderInstrument')}</label>
            <select
              value={selected?.key ?? ''}
              onChange={(e) => {
                setSelectedKey(e.target.value);
                setSession(null);
              }}
              className="mt-1 mb-3 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
            >
              {instruments.map((item) => (
                <option key={item.key} value={item.key}>
                  {item.ticker} · account {item.accountID}
                </option>
              ))}
            </select>
            <label className="text-xs font-semibold text-[var(--muted)]">{t('instruction')}</label>
            <textarea
              value={instruction}
              onChange={(e) => setInstruction(e.target.value)}
              rows={4}
              className="mt-1 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
            />
            <p className="mt-2 text-xs text-[var(--muted)]">{t('aiTraderSeparate')}</p>
            {error && <p className="mt-2 text-sm text-[var(--danger)]">{error}</p>}
          </div>
          <div className="flex flex-col gap-2 lg:w-44">
            <Button disabled={loading || !selected} onClick={() => void start('observe')} variant="primary">{t('startObserve')}</Button>
            <Button disabled={loading || !selected} onClick={() => void start('paper')} variant="secondary">{t('startPaper')}</Button>
            <Button disabled={loading || !session} onClick={() => void stop()} variant="ghost">{t('stopSession')}</Button>
          </div>
        </div>
      </Card>

      {!session && <EmptyState>{t('noAiTraderSession')}</EmptyState>}

      {session && (
        <>
          <div className="ai-trader-session-stats">
            <Stat label={t('aiTraderInstrument')} value={session.ticker ?? EM_DASH} sub={session.instrument_uid.slice(0, 12)} />
            <Stat label="Mode" value={session.mode} tone={session.mode === 'paper' ? 'warning' : 'info'} />
            <Stat label={t('spread')} value={f ? `${formatNumber(f.spread_bps, lang, 2)} bps` : EM_DASH} tone={f && f.spread_bps > 15 ? 'danger' : 'success'} />
            <Stat label={t('freshness')} value={f ? `${f.data_freshness_ms} ms` : EM_DASH} tone={f?.stale ? 'danger' : 'success'} />
            {d && (
              <Stat
                label={t('marketBias')}
                value={d.market_bias ?? 'neutral'}
                tone={d.market_bias === 'bullish' ? 'success' : d.market_bias === 'bearish' ? 'warning' : 'neutral'}
              />
            )}
          </div>

          <Card title={t('aiTraderThoughtStream')} subtitle={t('aiTraderThoughtStreamHelp')}>
            {thoughtStream.length > 0 ? (
              <div className="ai-trader-thought-stream">
                {thoughtStream.map((ev, i) => (
                  <ThoughtEntry key={`${ev.time}-${ev.action}-${i}`} ev={ev} lang={lang} t={t} />
                ))}
              </div>
            ) : (
              <EmptyState>{t('noEvents')}</EmptyState>
            )}
          </Card>
        </>
      )}
    </div>
  );
}
