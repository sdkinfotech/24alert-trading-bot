import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type { AiTraderSession, Instance } from '../api/types';
import { useI18n } from '../i18n';
import { EM_DASH, formatDateTime, formatNumber } from '../format';
import { Badge, Button, Card, EmptyState, Stat } from './ui';
import { ScalperDOM } from './ScalperDOM';

interface Props {
  instances: Instance[];
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
  const mc = session?.market_context;
  const d = session?.last_decision;
  const analysisLabel = d?.analysis_source === 'llm'
    ? t('analysisLLM')
    : d?.analysis_source === 'rules_fallback'
      ? t('analysisFallback')
      : t('analysisRules');

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
              rows={3}
              className="mt-1 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
            />
            <p className="mt-2 text-xs text-[var(--muted)]">
              {t('aiTraderSeparate')}
            </p>
            <p className="mt-1 text-xs text-[var(--muted)]">
              {selected ? `${selected.ticker} · ${selected.uid} · source list: ${selected.sourceInstanceID} (${selected.sourceType}) · live orders disabled` : 'no instrument'}
            </p>
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
          <div className="ai-trader-workbench">
            <Card className="scalper-card" title={t('scalperDom')} subtitle={t('scalperDomHelp')}>
              <ScalperDOM mc={mc} features={f} ticker={session.ticker} lang={lang} />
            </Card>

            <aside className="ai-trader-side">
              <div className="ai-trader-stat-grid">
                <Stat label="Mode" value={session.mode} tone={session.mode === 'paper' ? 'warning' : 'info'} sub={session.id} />
                <Stat label={t('spread')} value={f ? `${formatNumber(f.spread_bps, lang, 2)} bps` : EM_DASH} tone={f && f.spread_bps > 15 ? 'danger' : 'success'} sub={f ? `${formatNumber(f.best_bid, lang, 4)} / ${formatNumber(f.best_ask, lang, 4)}` : undefined} />
                <Stat label={t('imbalance')} value={f ? formatNumber(f.imbalance, lang, 3) : EM_DASH} tone={f && Math.abs(f.imbalance) > 0.35 ? 'warning' : 'neutral'} />
                <Stat label={t('freshness')} value={f ? `${f.data_freshness_ms} ms` : EM_DASH} tone={f?.stale ? 'danger' : 'success'} sub={f?.observed_at ? formatDateTime(f.observed_at, lang) : undefined} />
              </div>

              <Card title={t('aiTraderConclusion')}>
                {d ? (
                  <div className="space-y-3 text-sm">
                    <div className="flex flex-wrap gap-2">
                      <Badge tone={d.analysis_source === 'llm' ? 'info' : d.analysis_source === 'rules_fallback' ? 'warning' : 'neutral'}>
                        {t('analysisSource')}: {analysisLabel}
                      </Badge>
                      <Badge tone={d.market_bias === 'blocked' ? 'danger' : d.market_bias === 'bullish' ? 'success' : d.market_bias === 'bearish' ? 'warning' : 'neutral'}>
                        {t('marketBias')}: {d.market_bias ?? 'neutral'}
                      </Badge>
                      <Badge tone={d.action === 'block' ? 'danger' : d.action.includes('plan') ? 'warning' : 'info'}>{d.action}</Badge>
                    </div>
                    <p className="text-base font-medium text-[var(--text)]">{d.summary ?? d.reason}</p>
                    {d.next_watch && (
                      <p><span className="font-semibold">{t('nextWatch')}:</span> {d.next_watch}</p>
                    )}
                    <p className="text-xs text-[var(--muted)]">confidence {formatNumber(d.confidence * 100, lang, 1)}% · {formatDateTime(d.time, lang)}</p>
                  </div>
                ) : <EmptyState>{t('noEvents')}</EmptyState>}
              </Card>

              <Card title={t('marketContext')}>
                {mc ? (
                  <div className="space-y-3 text-sm">
                    <div>
                      <div className="text-xs font-semibold text-[var(--muted)]">{t('tape')} ({mc.tape_stats.window_sec}s)</div>
                      <div>trades {mc.tape_stats.trade_count} · buy {mc.tape_stats.buy_volume} · sell {mc.tape_stats.sell_volume}</div>
                      <div>last {formatNumber(mc.tape_stats.last_price, lang, 4)} · vwap {formatNumber(mc.tape_stats.vwap, lang, 4)} · delta {formatNumber(mc.tape_stats.delta_pct * 100, lang, 1)}%</div>
                      {mc.tape_stats.aggressor && <Badge tone="neutral">{mc.tape_stats.aggressor}</Badge>}
                    </div>

                    <div>
                      <div className="text-xs font-semibold text-[var(--muted)]">{t('keyLevels')}</div>
                      <div className="ai-trader-compact-list space-y-1 font-mono text-xs">
                        {(mc.levels ?? []).map((lv) => (
                          <div key={`${lv.kind}-${lv.rank}`}>{lv.kind} #{lv.rank}: {formatNumber(lv.price, lang, 4)} ({lv.source})</div>
                        ))}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs font-semibold text-[var(--muted)]">{t('bookEvolution')}</div>
                      <div className="ai-trader-compact-list space-y-1">
                        {(mc.book_timeline ?? []).slice(-5).reverse().map((b) => (
                          <div key={b.time} className="text-xs text-[var(--muted)]">{formatDateTime(b.time, lang)} mid {formatNumber(b.mid, lang, 4)} imb {formatNumber(b.imbalance, lang, 2)} · {b.bid_wall} / {b.ask_wall}</div>
                        ))}
                      </div>
                    </div>
                  </div>
                ) : <EmptyState>{t('noEvents')}</EmptyState>}
              </Card>
            </aside>
          </div>

          <Card title="Decision journal">
            {session.events?.length ? (
              <div className="max-h-[360px] space-y-2 overflow-y-auto pr-1">
                {session.events.map((ev) => (
                  <div key={`${ev.time}-${ev.action}`} className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3 text-sm">
                    <div className="flex flex-wrap gap-2">
                      <Badge tone={ev.action === 'block' ? 'danger' : 'info'}>{ev.action}</Badge>
                      <span className="text-[var(--muted)]">{formatDateTime(ev.time, lang)}</span>
                    </div>
                    <div className="mt-1">{ev.summary ?? ev.reason}</div>
                    {ev.next_watch && <div className="mt-1 text-xs text-[var(--muted)]">{ev.next_watch}</div>}
                  </div>
                ))}
              </div>
            ) : <EmptyState>{t('noEvents')}</EmptyState>}
          </Card>
        </>
      )}
    </div>
  );
}
