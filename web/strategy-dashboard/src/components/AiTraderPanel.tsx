import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import type { AiTraderSession, Instance } from '../api/types';
import { useI18n } from '../i18n';
import { formatDateTime, formatMoney, formatNumber } from '../format';
import { Badge, Button, Card, EmptyState, Stat, Table } from './ui';

interface Props {
  instance: Instance | null;
}

const defaultInstruction = 'наблюдай стакан, ищи плотности/перекосы, real orders запрещены';

export function AiTraderPanel({ instance }: Props) {
  const { t, lang } = useI18n();
  const [session, setSession] = useState<AiTraderSession | null>(null);
  const [instruction, setInstruction] = useState(defaultInstruction);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const instanceID = instance?.id;

  const load = useCallback(async () => {
    if (!instanceID) return;
    try {
      const data = await api.aiTraderSession(instanceID);
      setSession(data);
      setError(null);
    } catch {
      setSession(null);
    }
  }, [instanceID]);

  useEffect(() => {
    const initial = window.setTimeout(() => void load(), 0);
    const timer = window.setInterval(() => void load(), 3000);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(timer);
    };
  }, [load]);

  async function start(mode: 'observe' | 'paper') {
    if (!instanceID) return;
    setLoading(true);
    try {
      const data = await api.startAiTraderSession({
        instance_id: instanceID,
        mode,
        instruction,
        depth: 20,
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
    if (!instanceID) return;
    setLoading(true);
    try {
      const data = await api.stopAiTraderSession(instanceID);
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

  return (
    <div className="space-y-6">
      <Card
        title={t('aiTrader')}
        subtitle={t('aiTraderHelp')}
        actions={session ? <Badge tone={session.status === 'running' ? 'success' : 'neutral'}>{session.status}</Badge> : null}
      >
        <div className="grid gap-4 lg:grid-cols-[1fr_auto]">
          <div>
            <label className="text-xs font-semibold text-[var(--muted)]">{t('instruction')}</label>
            <textarea
              value={instruction}
              onChange={(e) => setInstruction(e.target.value)}
              rows={3}
              className="mt-1 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3 text-sm text-[var(--text)] outline-none focus:ring-2 focus:ring-[var(--accent)]"
            />
            <p className="mt-2 text-xs text-[var(--muted)]">
              {instance?.tickers ?? instance?.id ?? '—'} · {instance?.instruments?.[0] ?? 'no uid'} · live orders disabled
            </p>
            {error && <p className="mt-2 text-sm text-[var(--danger)]">{error}</p>}
          </div>
          <div className="flex flex-col gap-2 lg:w-44">
            <Button disabled={loading || !instance} onClick={() => void start('observe')} variant="primary">{t('startObserve')}</Button>
            <Button disabled={loading || !instance} onClick={() => void start('paper')} variant="secondary">{t('startPaper')}</Button>
            <Button disabled={loading || !session} onClick={() => void stop()} variant="ghost">{t('stopSession')}</Button>
          </div>
        </div>
      </Card>

      {!session && <EmptyState>{t('noAiTraderSession')}</EmptyState>}

      {session && (
        <>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Stat label="Mode" value={session.mode} tone={session.mode === 'paper' ? 'warning' : 'info'} sub={session.id} />
            <Stat label={t('spread')} value={f ? `${formatNumber(f.spread_bps, lang, 2)} bps` : '—'} tone={f && f.spread_bps > 15 ? 'danger' : 'success'} sub={f ? `${formatNumber(f.best_bid, lang, 4)} / ${formatNumber(f.best_ask, lang, 4)}` : undefined} />
            <Stat label={t('imbalance')} value={f ? formatNumber(f.imbalance, lang, 3) : '—'} tone={f && Math.abs(f.imbalance) > 0.35 ? 'warning' : 'neutral'} />
            <Stat label={t('freshness')} value={f ? `${f.data_freshness_ms} ms` : '—'} tone={f?.stale ? 'danger' : 'success'} sub={f?.observed_at ? formatDateTime(f.observed_at, lang) : undefined} />
          </div>

          <Card title={t('lastDecision')}>
            {d ? (
              <div className="space-y-2 text-sm">
                <div className="flex flex-wrap gap-2">
                  <Badge tone={d.action === 'block' ? 'danger' : d.action.includes('plan') ? 'warning' : 'info'}>{d.action}</Badge>
                  <Badge tone="neutral">{d.intent}</Badge>
                  <Badge tone="neutral">{d.risk_result}</Badge>
                  <span className="text-[var(--muted)]">{formatDateTime(d.time, lang)}</span>
                </div>
                <p>{d.reason}</p>
                <p className="text-xs text-[var(--muted)]">confidence {formatNumber(d.confidence * 100, lang, 1)}%</p>
              </div>
            ) : <EmptyState>{t('noEvents')}</EmptyState>}
          </Card>

          <Card title={t('orderbookFeatures')}>
            {f ? (
              <div className="grid gap-4 xl:grid-cols-2">
                <div className="space-y-3">
                  <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
                    <div className="text-xs text-[var(--muted)]">{t('bidWall')}</div>
                    <div className="font-mono">{formatNumber(f.largest_bid_wall.price, lang, 4)} x {f.largest_bid_wall.quantity}</div>
                  </div>
                  <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
                    <div className="text-xs text-[var(--muted)]">{t('askWall')}</div>
                    <div className="font-mono">{formatNumber(f.largest_ask_wall.price, lang, 4)} x {f.largest_ask_wall.quantity}</div>
                  </div>
                  <div className="text-xs text-[var(--muted)]">mid {formatNumber(f.mid, lang, 4)} · spread {formatMoney(f.spread_abs, lang)}</div>
                </div>
                <Table>
                  <thead>
                    <tr><th>Bid price</th><th>Bid qty</th><th>Ask price</th><th>Ask qty</th></tr>
                  </thead>
                  <tbody>
                    {Array.from({ length: Math.max(f.orderbook_top.bids.length, f.orderbook_top.asks.length) }).map((_, i) => {
                      const bid = f.orderbook_top.bids[i];
                      const ask = f.orderbook_top.asks[i];
                      return (
                        <tr key={i}>
                          <td className="font-mono text-[var(--success)]">{bid ? formatNumber(bid.price, lang, 4) : '—'}</td>
                          <td>{bid?.quantity ?? '—'}</td>
                          <td className="font-mono text-[var(--danger)]">{ask ? formatNumber(ask.price, lang, 4) : '—'}</td>
                          <td>{ask?.quantity ?? '—'}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </Table>
              </div>
            ) : <EmptyState>{t('noEvents')}</EmptyState>}
          </Card>

          <Card title="Decision journal">
            {session.events?.length ? (
              <div className="max-h-[420px] space-y-2 overflow-y-auto pr-1">
                {session.events.map((ev) => (
                  <div key={`${ev.time}-${ev.action}`} className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3 text-sm">
                    <div className="flex flex-wrap gap-2">
                      <Badge tone={ev.action === 'block' ? 'danger' : 'info'}>{ev.action}</Badge>
                      <span className="text-[var(--muted)]">{formatDateTime(ev.time, lang)}</span>
                    </div>
                    <div className="mt-1">{ev.reason}</div>
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
