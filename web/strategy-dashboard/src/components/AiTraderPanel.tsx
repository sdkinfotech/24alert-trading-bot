import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type { AiTraderDecisionEvent, AiTraderSession, Instance } from '../api/types';
import { useI18n } from '../i18n';
import { EM_DASH, formatDateTime, formatNumber } from '../format';
import { InstrumentSearchPicker, type SelectedInstrument } from './InstrumentSearchPicker';
import { AiTraderAnalysisPanel } from './AiTraderAnalysisPanel';
import { Badge, Button, Card, EmptyState, Stat } from './ui';

const BRENT_MINI: SelectedInstrument = {
  uid: 'dc1ffa30-70a4-4a7b-807a-4f31c2951f7e',
  ticker: 'BMM6',
  name: 'Brent mini',
  kind: 'future',
};

interface Props {
  instances: Instance[];
}

function analysisTone(source?: string): 'info' | 'warning' | 'neutral' {
  if (source === 'llm') return 'info';
  if (source === 'rules_fallback') return 'warning';
  if (source === 'session') return 'neutral';
  return 'neutral';
}

function phaseLabel(phase: string | undefined, t: (k: string) => string): string {
  switch (phase) {
    case 'collecting':
      return t('phaseCollecting');
    case 'analyzing':
      return t('phaseAnalyzing');
    case 'ready':
      return t('phaseReady');
    case 'trading':
      return t('phaseTrading');
    default:
      return phase ?? EM_DASH;
  }
}

function ThoughtEntry({ ev, lang, t }: { ev: AiTraderDecisionEvent; lang: 'ru' | 'en'; t: (k: string) => string }) {
  const analysisLabel = ev.analysis_source === 'llm'
    ? t('analysisLLM')
    : ev.analysis_source === 'rules_fallback'
      ? t('analysisFallback')
      : ev.analysis_source === 'session'
        ? t('analysisSession')
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
          {ev.llm_model && (
            <Badge tone="neutral">
              {ev.llm_model.length > 28 ? `${ev.llm_model.slice(0, 26)}…` : ev.llm_model}
            </Badge>
          )}
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

function PhaseStepper({ session, t }: { session: AiTraderSession; t: (k: string) => string }) {
  const p = session.phase_progress;
  const steps = ['collecting', 'analyzing', 'ready', 'trading'] as const;
  const idx = Math.max(0, steps.indexOf((session.phase ?? 'collecting') as (typeof steps)[number]));

  return (
    <div className="ai-trader-phase-stepper">
      <div className="flex flex-wrap gap-2">
        {steps.map((step, i) => (
          <span
            key={step}
            className={`rounded-full border px-3 py-1 text-xs font-medium ${
              i <= idx
                ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]'
                : 'border-[var(--border)] text-[var(--muted)]'
            } ${session.phase === step ? 'ring-2 ring-[var(--accent)]/40' : ''}`}
          >
            {phaseLabel(step, t)}
          </span>
        ))}
      </div>
      {p && (
        <p className="mt-2 text-xs text-[var(--muted)]">
          {t('phaseProgress')}: {p.collect_seconds}/{p.min_collect_sec}s
          {p.ready_reason ? ` — ${p.ready_reason}` : ''}
        </p>
      )}
    </div>
  );
}

export function AiTraderPanel({ instances }: Props) {
  const { t, lang } = useI18n();
  const [accountID, setAccountID] = useState('');
  const [instrument, setInstrument] = useState<SelectedInstrument | null>(BRENT_MINI);
  const [session, setSession] = useState<AiTraderSession | null>(null);
  const [instruction, setInstruction] = useState(() => t('aiTraderDefaultInstruction'));
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const accounts = useMemo(() => {
    const ids = new Set<string>();
    for (const inst of instances) {
      if (inst.account_id) ids.add(inst.account_id);
    }
    return [...ids].sort();
  }, [instances]);

  const strategyPicks = useMemo(() => {
    const seen = new Set<string>();
    const out: SelectedInstrument[] = [];
    for (const inst of instances) {
      (inst.instruments ?? []).forEach((uid, index) => {
        if (!uid || seen.has(uid)) return;
        seen.add(uid);
        const ticker = (inst.tickers ?? '').split(',')[index]?.trim() || inst.tickers || uid.slice(0, 8);
        out.push({ uid, ticker, name: inst.id, kind: 'strategy' });
      });
    }
    return out;
  }, [instances]);

  useEffect(() => {
    if (!accountID && accounts.length > 0) setAccountID(accounts[0]);
  }, [accounts, accountID]);

  const sessionLookup = session?.id ?? (accountID && instrument?.uid ? `${accountID}:${instrument.uid}` : '');

  const load = useCallback(async () => {
    if (!sessionLookup) return;
    try {
      const data = await api.aiTraderSession(sessionLookup);
      setSession(data);
      if (data.status === 'running') {
        setAccountID(data.account_id);
        setInstrument({
          uid: data.instrument_uid,
          ticker: data.ticker ?? data.instrument_uid.slice(0, 8),
          name: '',
          kind: '',
        });
      }
      setError(null);
    } catch {
      if (!session?.id) setSession(null);
    }
  }, [sessionLookup, session?.id]);

  useEffect(() => {
    const initial = window.setTimeout(() => void load(), 0);
    const timer = window.setInterval(() => void load(), 1500);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(timer);
    };
  }, [load]);

  async function startMonitoring() {
    if (!accountID || !instrument?.uid) {
      setError(t('aiTraderPickInstrument'));
      return;
    }
    setLoading(true);
    try {
      const data = await api.startAiTraderSession({
        account_id: accountID,
        instrument_uid: instrument.uid,
        ticker: instrument.ticker,
        strategy_kind: 'level_intraday',
        instruction,
        depth: 50,
        limits: {
          max_position_lots: 1,
          max_order_size: 1,
          max_active_orders: 2,
          max_spread_bps: 50,
          stale_data_ms: 1500,
          session_timeout_minutes: 120,
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

  async function startTrading() {
    if (!session?.id) return;
    setLoading(true);
    try {
      const data = await api.startAiTraderTrading(session.id);
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
  const sessionRunning = session?.status === 'running';
  const canStartTrading = Boolean(
    sessionRunning && session.phase === 'ready' && session.phase_progress?.trading_ready,
  );

  const thoughtStream = useMemo(() => {
    const events = session?.events ?? [];
    if (events.length > 0) return [...events];
    if (d) return [d];
    return [];
  }, [session?.events, d]);

  const reportChips = ['5m', '15m', '1h'];

  return (
    <div className="space-y-6">
      <Card
        title={t('aiTrader')}
        subtitle={t('aiTraderHelp')}
        actions={
          session ? (
            <Badge tone={session.phase === 'ready' || session.phase === 'trading' ? 'success' : 'info'}>
              {phaseLabel(session.phase, t)}
            </Badge>
          ) : null
        }
      >
        <div className="grid gap-4 lg:grid-cols-[1fr_auto]">
          <div>
            <label className="text-xs font-semibold text-[var(--muted)]">{t('aiTraderAccount')}</label>
            <select
              value={accountID}
              disabled={sessionRunning}
              onChange={(e) => {
                setAccountID(e.target.value);
                setSession(null);
              }}
              className="mt-1 mb-3 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
            >
              {accounts.map((id) => (
                <option key={id} value={id}>{id}</option>
              ))}
            </select>

            <label className="text-xs font-semibold text-[var(--muted)]">{t('aiTraderInstrument')}</label>
            <div className="mt-1 mb-2">
              <InstrumentSearchPicker
                value={instrument}
                disabled={sessionRunning}
                onChange={(item) => {
                  setInstrument(item);
                  setSession(null);
                }}
              />
            </div>

            {!sessionRunning && (
              <div className="mb-3 flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => setInstrument(BRENT_MINI)}
                  className={`rounded-full border px-2.5 py-1 text-xs font-mono transition ${
                    instrument?.uid === BRENT_MINI.uid
                      ? 'border-[var(--accent)] bg-[var(--accent)]/15 text-[var(--accent)]'
                      : 'border-[var(--border)] text-[var(--muted)] hover:border-[var(--accent)]'
                  }`}
                >
                  {t('brentMiniPreset')}
                </button>
                {strategyPicks.map((pick) => (
                  <button
                    key={pick.uid}
                    type="button"
                    onClick={() => setInstrument(pick)}
                    className={`rounded-full border px-2.5 py-1 text-xs font-mono transition ${
                      instrument?.uid === pick.uid
                        ? 'border-[var(--accent)] bg-[var(--accent)]/15 text-[var(--accent)]'
                        : 'border-[var(--border)] text-[var(--muted)] hover:border-[var(--accent)]'
                    }`}
                  >
                    {pick.ticker}
                  </button>
                ))}
              </div>
            )}

            <label className="text-xs font-semibold text-[var(--muted)]">{t('instruction')}</label>
            <textarea
              value={instruction}
              onChange={(e) => setInstruction(e.target.value)}
              rows={4}
              disabled={sessionRunning}
              className="mt-1 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)] disabled:opacity-60"
            />
            <p className="mt-2 text-xs text-[var(--muted)]">{t('aiTraderSeparate')}</p>
            {error && <p className="mt-2 text-sm text-[var(--danger)]">{error}</p>}
          </div>
          <div className="flex flex-col gap-2 lg:w-48">
            <Button
              disabled={loading || !accountID || !instrument || sessionRunning}
              onClick={() => void startMonitoring()}
              variant="primary"
            >
              {t('startMonitoring')}
            </Button>
            <Button
              disabled={loading || !canStartTrading}
              onClick={() => void startTrading()}
              variant="primary"
              className={canStartTrading ? 'ai-trader-start-trading-pulse' : undefined}
            >
              {t('startTrading')}
            </Button>
            <Button disabled={loading || !session} onClick={() => void stop()} variant="ghost">
              {t('stopSession')}
            </Button>
          </div>
        </div>
      </Card>

      {!session && <EmptyState>{t('noAiTraderSession')}</EmptyState>}

      {session && (
        <>
          <PhaseStepper session={session} t={t} />

          <div className="flex flex-wrap gap-2">
            {reportChips.map((tf) => {
              const ready = session.phase_progress?.reports_ready?.includes(tf);
              return (
                <Badge key={tf} tone={ready ? 'success' : 'neutral'}>
                  {tf} {ready ? '✓' : '…'}
                </Badge>
              );
            })}
          </div>

          <div className="ai-trader-session-stats">
            <Stat label={t('aiTraderInstrument')} value={session.ticker ?? EM_DASH} sub={session.instrument_uid.slice(0, 12)} />
            <Stat label={t('phaseProgress')} value={phaseLabel(session.phase, t)} tone={session.phase === 'ready' ? 'success' : 'info'} />
            <Stat label={t('spread')} value={f ? `${formatNumber(f.spread_bps, lang, 2)} bps` : EM_DASH} tone={f && f.spread_bps > 50 ? 'danger' : 'success'} />
            <Stat label={t('freshness')} value={f ? `${f.data_freshness_ms} ms` : EM_DASH} tone={f?.stale ? 'danger' : 'success'} />
            {d && (
              <Stat
                label={t('marketBias')}
                value={d.market_bias ?? 'neutral'}
                tone={d.market_bias === 'bullish' ? 'success' : d.market_bias === 'bearish' ? 'warning' : 'neutral'}
              />
            )}
          </div>

          {session.level_playbook?.levels && session.level_playbook.levels.length > 0 && (
            <Card title={t('playbookLevels')} subtitle={session.level_playbook.summary}>
              <ul className="text-sm space-y-1 font-mono">
                {session.level_playbook.levels.map((lv) => (
                  <li key={`${lv.kind}-${lv.price}-${lv.source}`}>
                    {lv.kind} {formatNumber(lv.price, lang, 4)}{' '}
                    <span className="text-[var(--muted)]">({lv.source})</span>
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {session.paper_state && session.phase === 'trading' && (
            <Card title={t('paperPosition')} subtitle={t('paperOrders')}>
              <p className="text-sm">
                pos={session.paper_state.position_lots} @ {formatNumber(session.paper_state.avg_price, lang, 4)}
                {' '}| realized {formatNumber(session.paper_state.realized_rub, lang, 2)} RUB
              </p>
              {session.paper_state.working_orders && session.paper_state.working_orders.length > 0 && (
                <ul className="mt-2 text-xs font-mono space-y-1">
                  {session.paper_state.working_orders.map((o) => (
                    <li key={o.id}>
                      {o.status} {o.side} {o.quantity}@{formatNumber(o.price, lang, 4)} ({o.level_ref})
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          )}

          <Card title={t('aiTraderThoughtStream')} subtitle={t('aiTraderThoughtStreamHelp')}>
            {session.last_error && <p className="mb-3 text-sm text-[var(--danger)]">{session.last_error}</p>}
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

          <AiTraderAnalysisPanel sessionId={session.id} />
        </>
      )}
    </div>
  );
}
