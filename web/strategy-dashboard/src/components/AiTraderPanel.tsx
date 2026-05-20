import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../api/client';
import type { AiTraderPersistedSummary, AiTraderPublicConfig, AiTraderSession, Instance } from '../api/types';
import { useI18n } from '../i18n';
import { EM_DASH, formatNumber } from '../format';
import { InstrumentSearchPicker, type SelectedInstrument } from './InstrumentSearchPicker';
import { AiTraderAnalysisPanel } from './AiTraderAnalysisPanel';
import { AiTraderActivityStream } from './AiTraderActivityStream';
import { AiTraderCollectPanel } from './AiTraderCollectPanel';
import { AiTraderLevelsChart } from './AiTraderLevelsChart';
import { AiTraderStrategyBrief } from './AiTraderStrategyBrief';
import { AiTraderAnalystPanel } from './AiTraderAnalystPanel';
import { AiTraderExecutionLog } from './AiTraderExecutionLog';
import { AiTraderTradingDesk } from './AiTraderTradingDesk';
import { clearAiTraderLast, readAiTraderLast, writeAiTraderLast } from './aiTraderStorage';
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
  const [resumeCandidate, setResumeCandidate] = useState<AiTraderSession | null>(null);
  const [persistedCandidate, setPersistedCandidate] = useState<AiTraderPersistedSummary | null>(null);
  const [armedLive, setArmedLive] = useState(false);
  const [traderConfig, setTraderConfig] = useState<AiTraderPublicConfig | null>(null);
  const [killSwitch, setKillSwitch] = useState(false);
  const advisorRef = useRef<HTMLDivElement>(null);
  const resumedRef = useRef(false);

  const accounts = useMemo(() => {
    const ids = new Set<string>();
    if (traderConfig?.default_account_id) ids.add(traderConfig.default_account_id);
    for (const inst of instances) {
      if (inst.account_id) ids.add(inst.account_id);
    }
    return [...ids].sort();
  }, [instances, traderConfig?.default_account_id]);

  useEffect(() => {
    if (!accountID && accounts.length > 0) {
      const saved = readAiTraderLast();
      if (saved?.account_id && accounts.includes(saved.account_id)) {
        setAccountID(saved.account_id);
      } else {
        setAccountID(accounts[0]);
      }
    }
  }, [accounts, accountID]);

  useEffect(() => {
    void api
      .aiTraderConfig()
      .then((c) => {
        setTraderConfig(c);
        setKillSwitch(c.kill_switch);
        if (c.default_account_id) setAccountID(c.default_account_id);
      })
      .catch(() => setTraderConfig(null));
  }, []);

  useEffect(() => {
    const saved = readAiTraderLast();
    if (saved?.instrument_uid && !session) {
      setInstrument({
        uid: saved.instrument_uid,
        ticker: saved.ticker ?? saved.instrument_uid.slice(0, 8),
        name: '',
        kind: 'future',
      });
    }
  }, [session]);

  const applySession = useCallback((data: AiTraderSession) => {
    setSession(data);
    if (data.instruction) setInstruction(data.instruction);
    if (data.status === 'running') {
      setAccountID(data.account_id);
      setInstrument({
        uid: data.instrument_uid,
        ticker: data.ticker ?? data.instrument_uid.slice(0, 8),
        name: '',
        kind: '',
      });
      writeAiTraderLast({
        account_id: data.account_id,
        instrument_uid: data.instrument_uid,
        ticker: data.ticker,
        session_id: data.id,
      });
    }
  }, []);

  const sessionLookup = session?.id ?? (accountID && instrument?.uid ? `${accountID}:${instrument.uid}` : '');

  const load = useCallback(async () => {
    if (!sessionLookup) return;
    try {
      const data = await api.aiTraderSession(sessionLookup);
      applySession(data);
      setResumeCandidate(null);
      setError(null);
    } catch {
      if (!session?.id) setSession(null);
    }
  }, [sessionLookup, session?.id, applySession]);

  useEffect(() => {
    if (resumedRef.current) return;
    resumedRef.current = true;
    void (async () => {
      try {
        const all = await api.aiTraderSessions();
        const running = all.filter((s) => s.status === 'running');
        const saved = readAiTraderLast();
        let pick: AiTraderSession | undefined;
        if (saved?.session_id) {
          pick = running.find((s) => s.id === saved.session_id);
        }
        if (!pick && saved) {
          pick = running.find(
            (s) => s.account_id === saved.account_id && s.instrument_uid === saved.instrument_uid,
          );
        }
        if (!pick && running.length === 1) pick = running[0];
        if (pick) {
          applySession(pick);
        } else if (running.length > 0 && !session) {
          setResumeCandidate(running[0]);
        } else if (!pick && running.length === 0) {
          const persisted = await api.aiTraderPersistedSessions();
          if (persisted.length > 0) {
            const match =
              (saved?.session_id && persisted.find((p) => p.id === saved.session_id)) ||
              (saved?.account_id &&
                persisted.find(
                  (p) => p.account_id === saved.account_id && p.instrument_uid === saved.instrument_uid,
                )) ||
              persisted[0];
            setPersistedCandidate(match);
          }
        }
      } catch {
        /* ignore */
      }
    })();
  }, [applySession, session]);

  async function resumePersisted(reconnectOnly: boolean) {
    if (!persistedCandidate) return;
    setLoading(true);
    try {
      const data = await api.resumeAiTraderSession(persistedCandidate.id, {
        reconnect_only: reconnectOnly,
        resume_trading: !reconnectOnly,
      });
      applySession(data);
      setPersistedCandidate(null);
      setResumeCandidate(null);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

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
    if (armedLive && !window.confirm(t('armedLiveConfirm'))) {
      return;
    }
    setLoading(true);
    try {
      const data = await api.startAiTraderSession({
        account_id: accountID,
        instrument_uid: instrument.uid,
        ticker: instrument.ticker,
        strategy_kind: 'level_intraday',
        mode: armedLive ? 'armed_live' : undefined,
        confirm_live: armedLive,
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
      applySession(data);
      setResumeCandidate(null);
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
      applySession(data);
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
      clearAiTraderLast();
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  const f = session?.features;
  const sessionRunning = session?.status === 'running';
  const canStartTrading = Boolean(
    sessionRunning &&
      session.phase === 'ready' &&
      session.phase_progress?.trading_ready &&
      session.phase_progress?.strategy_ready,
  );

  const reportChips = ['5m', '15m', '1h'];

  return (
    <div className="space-y-6">
      {resumeCandidate && !sessionRunning && (
        <div className="ai-trader-resume-banner">
          <span className="text-sm">
            {t('resumeSessionBanner')}: <strong>{resumeCandidate.ticker}</strong> ({resumeCandidate.account_id})
          </span>
          <Button variant="primary" onClick={() => applySession(resumeCandidate)}>
            {t('resumeSessionAction')}
          </Button>
        </div>
      )}

      {persistedCandidate && !sessionRunning && !resumeCandidate && (
        <div className="ai-trader-resume-banner border-amber-500/40 bg-amber-500/10">
          <span className="text-sm">
            {t('persistedSessionBanner')}: <strong>{persistedCandidate.ticker}</strong> — {phaseLabel(persistedCandidate.phase, t)}
          </span>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" disabled={loading} onClick={() => void resumePersisted(false)}>
              {t('persistedResumeTrading')}
            </Button>
            <Button variant="secondary" disabled={loading} onClick={() => void resumePersisted(true)}>
              {t('persistedReconnectOnly')}
            </Button>
          </div>
        </div>
      )}

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
                if (!sessionRunning) setSession(null);
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
                  if (!sessionRunning) setSession(null);
                }}
              />
            </div>

            {!sessionRunning && (
              <div className="mb-3 flex flex-wrap gap-2 items-center">
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
                <span className="text-xs text-[var(--muted)]">{t('classicStrategiesOff')}</span>
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
            {traderConfig?.armed_live_enabled && !sessionRunning && (
              <label className="mt-3 flex items-center gap-2 text-sm text-[var(--danger)]">
                <input
                  type="checkbox"
                  checked={armedLive}
                  onChange={(e) => setArmedLive(e.target.checked)}
                />
                {t('armedLiveMode')}
              </label>
            )}
            {sessionRunning && (
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Badge tone={session.execution_mode === 'armed_live' ? 'danger' : 'neutral'}>
                  {session.execution_mode === 'armed_live' ? t('armedLiveActive') : t('paperMode')}
                </Badge>
                <Button
                  variant="ghost"
                  disabled={loading}
                  onClick={() => {
                    void api.setAiTraderKillSwitch(!killSwitch).then((r) => {
                      setKillSwitch(r.kill_switch);
                    });
                  }}
                >
                  {killSwitch ? t('killSwitchOff') : t('killSwitchOn')}
                </Button>
              </div>
            )}
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
          <AiTraderStrategyBrief
            session={session}
            onScrollToAdvisor={() => advisorRef.current?.scrollIntoView({ behavior: 'smooth' })}
          />

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
          </div>

          <AiTraderLevelsChart session={session} />
          <AiTraderCollectPanel session={session} />

          {(session.phase === 'trading' || session.live_state || session.paper_state) && (
            <>
              <AiTraderExecutionLog session={session} />
              <AiTraderTradingDesk
                session={session}
                flattenLoading={loading}
                onFlatten={
                  session.execution_mode === 'armed_live' && session.status === 'running'
                    ? async () => {
                        if (!session.id) return;
                        setLoading(true);
                        try {
                          const data = await api.flattenAiTraderSession(session.id);
                          applySession(data);
                          setError(null);
                        } catch (e) {
                          setError(e instanceof Error ? e.message : String(e));
                        } finally {
                          setLoading(false);
                        }
                      }
                    : undefined
                }
              />
              <AiTraderAnalystPanel sessionId={session.id} ticker={session.ticker} />
            </>
          )}

          <AiTraderActivityStream session={session} />

          <div ref={advisorRef}>
            <AiTraderAnalysisPanel sessionId={session.id} />
          </div>
        </>
      )}
    </div>
  );
}
