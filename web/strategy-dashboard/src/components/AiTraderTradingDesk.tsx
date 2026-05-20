import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import type {
  AiTraderBrokerSnapshot,
  AiTraderDecisionEvent,
  AiTraderSession,
  LiveFill,
  LiveOrder,
} from '../api/types';
import { useI18n } from '../i18n';
import { EM_DASH, formatDateTime, formatNumber } from '../format';
import { Badge, Button, Card, EmptyState, Stat, Table, Tabs } from './ui';

type DeskTab = 'position' | 'orders' | 'fills' | 'journal';

interface Props {
  session: AiTraderSession;
  onFlatten?: () => void;
  flattenLoading?: boolean;
}

function posLabel(lots: number): string {
  if (lots > 0) return 'LONG';
  if (lots < 0) return 'SHORT';
  return 'FLAT';
}

function posTone(lots: number): 'success' | 'danger' | 'neutral' {
  if (lots > 0) return 'success';
  if (lots < 0) return 'danger';
  return 'neutral';
}

function dirBadge(side: string): 'success' | 'danger' | 'neutral' {
  if (side === 'buy') return 'success';
  if (side === 'sell') return 'danger';
  return 'neutral';
}

function isTradeEvent(e: AiTraderDecisionEvent): boolean {
  const a = (e.action || '').toLowerCase();
  const r = (e.risk_result || '').toLowerCase();
  return (
    a.includes('order') ||
    a.includes('trade') ||
    a.includes('fill') ||
    a.includes('stop') ||
    a.includes('flatten') ||
    a.includes('start_trading') ||
    a === 'trade_entry' ||
    a === 'trade_exit' ||
    r.includes('live') ||
    r.includes('paper') ||
    r.includes('submitted')
  );
}

export function AiTraderTradingDesk({ session, onFlatten, flattenLoading }: Props) {
  const { t, lang } = useI18n();
  const [tab, setTab] = useState<DeskTab>('position');
  const [broker, setBroker] = useState<AiTraderBrokerSnapshot | null>(null);
  const [journalFilter, setJournalFilter] = useState<'all' | 'trades' | 'decisions'>('trades');

  const live = session.live_state;
  const paper = session.paper_state;
  const isLive = session.execution_mode === 'armed_live';
  const state = isLive ? live : paper;
  const mid = session.features?.mid;

  const loadBroker = useCallback(async () => {
    if (!session.id || session.status !== 'running') return;
    try {
      const b = await api.aiTraderBrokerPosition(session.id);
      setBroker(b);
    } catch {
      setBroker(null);
    }
  }, [session.id, session.status]);

  useEffect(() => {
    void loadBroker();
    const id = window.setInterval(() => void loadBroker(), 5000);
    return () => window.clearInterval(id);
  }, [loadBroker]);

  const orders: LiveOrder[] = useMemo(() => {
    const raw = (state?.working_orders ?? []) as LiveOrder[];
    return [...raw].sort((a, b) => (b.placed_at || '').localeCompare(a.placed_at || ''));
  }, [state?.working_orders]);

  const fills: LiveFill[] = useMemo(() => {
    const raw = (live?.fills ?? paper?.fills ?? []) as LiveFill[];
    return [...raw].sort((a, b) => (b.time || '').localeCompare(a.time || ''));
  }, [live?.fills, paper?.fills]);

  const events = useMemo(() => {
    const all = session.events ?? [];
    const filtered =
      journalFilter === 'trades'
        ? all.filter(isTradeEvent)
        : journalFilter === 'decisions'
          ? all.filter((e) => !isTradeEvent(e))
          : all;
    return filtered;
  }, [session.events, journalFilter]);

  const runnerLots = state?.position_lots ?? 0;
  const brokerQty = broker?.quantity ?? 0;
  const misaligned = Math.abs(runnerLots - brokerQty) > 1e-6 && !broker?.portfolio_error;

  const unrealized = useMemo(() => {
    if (!state || state.position_lots === 0 || !mid || state.avg_price <= 0) return null;
    const sign = state.position_lots > 0 ? 1 : -1;
    return (mid - state.avg_price) * sign * Math.abs(state.position_lots);
  }, [state, mid]);

  const tabs: Array<{ id: DeskTab; label: string }> = [
    { id: 'position', label: t('aiDeskPosition') },
    { id: 'orders', label: `${t('aiDeskOrders')} (${orders.length})` },
    { id: 'fills', label: `${t('aiDeskFills')} (${fills.length})` },
    { id: 'journal', label: `${t('aiDeskJournal')} (${events.length})` },
  ];

  if (!state && session.phase !== 'trading') {
    return null;
  }

  return (
    <Card
      title={t('aiTradingDesk')}
      subtitle={t('aiTradingDeskHelp')}
      actions={
        <Badge tone={isLive ? 'danger' : 'info'}>
          {isLive ? t('armedLiveActive') : t('paperMode')}
        </Badge>
      }
    >
      <div className="ai-trader-desk-stats">
        <Stat
          label={t('aiDeskRunnerPos')}
          value={posLabel(runnerLots)}
          tone={posTone(runnerLots)}
          sub={
            runnerLots !== 0
              ? `${runnerLots} @ ${formatNumber(state?.avg_price ?? 0, lang, 4)}`
              : t('flat')
          }
        />
        <Stat
          label={t('aiDeskBrokerPos')}
          value={broker?.portfolio_error ? t('unavailable') : posLabel(Math.round(brokerQty))}
          tone={broker?.portfolio_error ? 'warning' : posTone(Math.round(brokerQty))}
          sub={
            broker && !broker.portfolio_error
              ? `${formatNumber(broker.quantity, lang, 4)} @ ${formatNumber(broker.average_price, lang, 4)}`
              : broker?.portfolio_error
          }
        />
        <Stat
          label={t('realized')}
          value={`${formatNumber(state?.realized_rub ?? 0, lang, 2)} ₽`}
          tone={(state?.realized_rub ?? 0) < 0 ? 'danger' : 'success'}
        />
        <Stat
          label={t('aiDeskUnrealized')}
          value={unrealized != null ? `${formatNumber(unrealized, lang, 2)} ₽` : EM_DASH}
          tone={unrealized != null && unrealized < 0 ? 'danger' : 'success'}
          sub={mid ? `mid ${formatNumber(mid, lang, 4)}` : undefined}
        />
      </div>

      {session.active_policy && (
        <div className="mb-3 rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3 text-sm">
          <h4 className="text-xs font-semibold text-[var(--muted)] mb-2">{t('aiDeskSessionPlan')}</h4>
          <p className="text-[var(--text)]">
            {t('aiDeskPlanBias')}: <Badge tone="info">{session.active_policy.market_bias || 'neutral'}</Badge>
            {' · '}
            {t('aiDeskPlanEntry')}: ≥{formatNumber(session.active_policy.entry_min_confidence, lang, 2)}
            {' · '}
            {t('aiDeskPlanConfluence')}: ≥{formatNumber(session.active_policy.confluence_min_score, lang, 1)}
          </p>
          <p className="text-xs text-[var(--muted)] mt-1">
            SL {formatNumber(session.active_policy.sl_mult_atr, lang, 2)}×ATR · TP{' '}
            {formatNumber(session.active_policy.tp_mult_atr, lang, 2)}×ATR ·{' '}
            {session.active_policy.allow_new_entry ? t('aiDeskPlanEntriesOn') : t('aiDeskPlanEntriesOff')}
            {session.active_policy.source ? ` · ${session.active_policy.source}` : ''}
            {session.active_policy.updated_at
              ? ` · ${formatDateTime(session.active_policy.updated_at, lang)}`
              : ''}
          </p>
          {session.active_policy.summary && (
            <p className="text-xs mt-1 text-[var(--muted)]">{session.active_policy.summary}</p>
          )}
        </div>
      )}

      {isLive && live && (live.stop_loss || live.take_profit) && (
        <p className="text-xs text-[var(--muted)] mb-3">
          SL {live.stop_loss ? formatNumber(live.stop_loss, lang, 4) : '—'} · TP{' '}
          {live.take_profit ? formatNumber(live.take_profit, lang, 4) : '—'}
        </p>
      )}

      {misaligned && (
        <div className="mb-3 rounded-lg border border-[var(--warning)] bg-[var(--warning-soft)] p-3 text-sm text-[var(--warning)]">
          {t('aiDeskMismatch')}
        </div>
      )}

      {live?.halted && (
        <div className="mb-3 rounded-lg border border-[var(--danger)] bg-[var(--danger-soft)] p-3 text-sm text-[var(--danger)]">
          {t('aiDeskHalted')}: {live.halt_reason || 'kill_switch'}
        </div>
      )}

      <div className="ai-trader-desk-toolbar">
        <Tabs tabs={tabs} active={tab} onChange={setTab} />
        <div className="flex flex-wrap gap-2 ml-auto">
          {isLive && onFlatten && (runnerLots !== 0 || brokerQty !== 0) && (
            <Button variant="ghost" disabled={flattenLoading} onClick={onFlatten}>
              {t('aiDeskFlatten')}
            </Button>
          )}
          <Button variant="ghost" onClick={() => void loadBroker()}>
            {t('refresh')}
          </Button>
        </div>
      </div>

      {tab === 'position' && (
        <div className="grid gap-4 md:grid-cols-2 mt-4">
          <div className="rounded-lg border border-[var(--border)] p-3">
            <h4 className="text-xs font-semibold text-[var(--muted)] mb-2">{t('aiDeskRunnerPos')}</h4>
            <p className="text-lg font-semibold">{posLabel(runnerLots)}</p>
            <p className="text-sm text-[var(--muted)] mt-1">
              {t('quantity')}: {runnerLots} · {t('avgPrice')}: {formatNumber(state?.avg_price ?? 0, lang, 4)}
            </p>
            <p className="text-sm mt-1">
              {t('realized')}: {formatNumber(state?.realized_rub ?? 0, lang, 2)} ₽
            </p>
          </div>
          <div className="rounded-lg border border-[var(--border)] p-3">
            <h4 className="text-xs font-semibold text-[var(--muted)] mb-2">{t('aiDeskBrokerPos')}</h4>
            {broker?.portfolio_error ? (
              <p className="text-sm text-[var(--danger)]">{broker.portfolio_error}</p>
            ) : (
              <>
                <p className="text-lg font-semibold">{posLabel(Math.round(brokerQty))}</p>
                <p className="text-sm text-[var(--muted)] mt-1">
                  {formatNumber(broker?.quantity ?? 0, lang, 4)} @ {formatNumber(broker?.average_price ?? 0, lang, 4)}
                </p>
                <p className={`text-sm mt-1 ${(broker?.expected_yield ?? 0) < 0 ? 'text-red-400' : 'text-green-400'}`}>
                  {t('expectedYield')}: {formatNumber(broker?.expected_yield ?? 0, lang, 2)} ₽
                </p>
                <p className="text-[10px] text-[var(--muted)] mt-2">
                  sync {broker?.last_broker_sync ? formatDateTime(broker.last_broker_sync, lang) : '—'}
                </p>
              </>
            )}
          </div>
          {session.last_trade_signal && (
            <div className="md:col-span-2 rounded-lg border border-[var(--border)] p-3 text-sm">
              <span className="text-[var(--muted)]">{t('lastSignal')}: </span>
              <Badge tone={dirBadge(session.last_trade_signal.side)}>
                {session.last_trade_signal.side}
              </Badge>{' '}
              @ {formatNumber(session.last_trade_signal.level_price, lang, 4)} conf=
              {formatNumber(session.last_trade_signal.confidence, lang, 2)}
              {session.last_trade_signal.reason ? ` — ${session.last_trade_signal.reason}` : ''}
            </div>
          )}
        </div>
      )}

      {tab === 'orders' && (
        <div className="mt-4">
          {orders.length === 0 ? (
            <EmptyState>{t('aiDeskNoOrders')}</EmptyState>
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>{t('time')}</th>
                  <th>{t('side')}</th>
                  <th>{t('price')}</th>
                  <th>{t('quantity')}</th>
                  <th>{t('status')}</th>
                  <th>{t('level')}</th>
                  <th>broker id</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.id}>
                    <td className="font-mono text-xs">{formatDateTime(o.placed_at, lang)}</td>
                    <td><Badge tone={dirBadge(o.side)}>{o.side}</Badge></td>
                    <td className="font-mono">{formatNumber(o.price, lang, 4)}</td>
                    <td className="font-mono">{o.quantity}</td>
                    <td>{o.status}</td>
                    <td className="text-xs">{o.level_ref || '—'}</td>
                    <td className="font-mono text-[10px]">{o.broker_order_id?.slice(0, 12) || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </div>
      )}

      {tab === 'fills' && (
        <div className="mt-4">
          {fills.length === 0 ? (
            <EmptyState>{t('aiDeskNoFills')}</EmptyState>
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>{t('time')}</th>
                  <th>{t('side')}</th>
                  <th>{t('price')}</th>
                  <th>{t('quantity')}</th>
                  <th>{t('note')}</th>
                </tr>
              </thead>
              <tbody>
                {fills.map((f, i) => (
                  <tr key={`${f.time}-${i}`}>
                    <td className="font-mono text-xs">{formatDateTime(f.time, lang)}</td>
                    <td><Badge tone={dirBadge(f.side)}>{f.side}</Badge></td>
                    <td className="font-mono">{formatNumber(f.price, lang, 4)}</td>
                    <td className="font-mono">{f.quantity}</td>
                    <td className="text-xs">{f.note || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </div>
      )}

      {tab === 'journal' && (
        <div className="mt-4">
          <div className="flex flex-wrap gap-1 mb-3">
            {(['trades', 'decisions', 'all'] as const).map((f) => (
              <button
                key={f}
                type="button"
                className={`ui-button px-2 py-1 text-xs ${journalFilter === f ? 'ui-button-primary' : 'ui-button-ghost'}`}
                onClick={() => setJournalFilter(f)}
              >
                {f === 'trades' ? t('aiDeskFilterTrades') : f === 'decisions' ? t('aiDeskFilterDecisions') : t('all')}
              </button>
            ))}
          </div>
          <div className="max-h-[420px] overflow-y-auto space-y-1 pr-1">
            {events.length === 0 && <EmptyState>{t('noEvents')}</EmptyState>}
            {events.map((e, i) => (
              <div
                key={`${e.time}-${e.action}-${i}`}
                className="border-l-2 border-[var(--accent)] pl-3 py-1.5 rounded-r bg-[var(--surface-muted)]"
              >
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <span className="font-mono text-[var(--muted)]">{formatDateTime(e.time, lang)}</span>
                  <Badge tone="info">{e.action}</Badge>
                  {e.risk_result && <Badge tone="neutral">{e.risk_result}</Badge>}
                </div>
                <p className="text-sm mt-0.5">{e.summary || e.reason}</p>
                {e.operator_note && (
                  <p className="text-xs text-[var(--muted)] mt-0.5">{e.operator_note}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}
