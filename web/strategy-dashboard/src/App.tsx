import { useEffect, useMemo, useState, useCallback } from 'react';
import { api } from './api/client';
import type {
  Instance,
  InstanceStatus,
  PnlData,
  LedgerData,
  PortfolioSnapshot,
  IndicatorData,
  TimelineEvent,
  DailySummary,
  OrderRecord,
  ExecutionRecord,
  StopOrder,
} from './api/types';
import { BotControlBar } from './components/BotControlBar';
import { AssistantPanel } from './components/AssistantPanel';
import { StrategyLabPanel } from './components/StrategyLabPanel';
import { IndicatorChart } from './components/IndicatorChart';
import { EventLog } from './components/EventLog';
import { StatsPanel } from './components/StatsPanel';
import { PositionOverview } from './components/PositionOverview';
import { InstanceSelector } from './components/InstanceSelector';
import { AiChatPanel } from './components/AiChatPanel';
import { AiTraderPanel } from './components/AiTraderPanel';
import { Badge, Card, EmptyState, LabelWithHelp, Stat, Table, Tabs } from './components/ui';
import { I18nProvider, useI18n } from './i18n';
import { ThemeProvider, useTheme } from './theme';
import { formatDateTime, formatMoney, formatNumber } from './format';

/** Polling while «Мониторинг» is visible; slower when tab in background. */
const REFRESH_VISIBLE_MS = 5_000;
const REFRESH_HIDDEN_MS = 30_000;

type MainTab = 'overview' | 'chart' | 'portfolio' | 'history' | 'assistant' | 'lab' | 'ai-trader' | 'guide';

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function tabFromHash(): MainTab {
  const h = window.location.hash.slice(1).toLowerCase();
  if (['chart', 'portfolio', 'history', 'assistant', 'lab', 'ai-trader', 'guide', 'overview'].includes(h)) return h as MainTab;
  if (h === 'about' || h === 'справка') return 'guide';
  return 'overview';
}

function applyTabToURL(t: MainTab) {
  const base = window.location.pathname + window.location.search;
  window.history.replaceState(null, '', t === 'overview' ? base : `${base}#${t}`);
}

export default function App() {
  return (
    <I18nProvider>
      <ThemeProvider>
        <DashboardApp />
      </ThemeProvider>
    </I18nProvider>
  );
}

function DashboardApp() {
  const { t, lang, setLang } = useI18n();
  const { mode, setMode, chart } = useTheme();
  const [tab, setTab] = useState<MainTab>(() =>
    typeof window !== 'undefined' ? tabFromHash() : 'overview'
  );
  const [instances, setInstances] = useState<Instance[]>([]);
  const [selected, setSelected] = useState('');
  const [indicator, setIndicator] = useState<IndicatorData | null>(null);
  const [pnl, setPnl] = useState<PnlData | null>(null);
  const [ledger, setLedger] = useState<LedgerData | null>(null);
  const [portfolio, setPortfolio] = useState<PortfolioSnapshot | null>(null);
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [daily, setDaily] = useState<DailySummary | null>(null);
  const [orders, setOrders] = useState<OrderRecord[]>([]);
  const [executions, setExecutions] = useState<ExecutionRecord[]>([]);
  const [stopOrders, setStopOrders] = useState<StopOrder[]>([]);
  const [instanceStatus, setInstanceStatus] = useState<InstanceStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [aiTraderArchived, setAiTraderArchived] = useState(true);

  const loadInstances = useCallback(async () => {
    try {
      const list = await api.instances();
      const rows = list ?? [];
      setInstances(rows);
      if (rows.length && (!selected || !rows.find((i) => i.id === selected))) {
        setSelected(rows[0].id);
      }
      setError(null);
    } catch (e: unknown) {
      setError(`Failed to load instances: ${errorMessage(e)}`);
    }
  }, [selected]);

  const loadData = useCallback(async () => {
    if (!selected) return;
    try {
      const [ind, p, l, pf, ev, d, ord, exe, stops, st] = await Promise.allSettled([
        api.indicator(selected),
        api.pnl(selected),
        api.ledger(selected),
        api.portfolio(selected),
        api.events(selected, 1000),
        api.daily(),
        api.orders(selected, 1000),
        api.executions(selected, 1000),
        api.stopOrders(selected),
        api.instanceStatus(selected),
      ]);
      if (ind.status === 'fulfilled') setIndicator(ind.value);
      if (p.status === 'fulfilled') setPnl(p.value);
      if (l.status === 'fulfilled') setLedger(l.value);
      if (pf.status === 'fulfilled') setPortfolio(pf.value);
      if (ev.status === 'fulfilled') setEvents(ev.value ?? []);
      if (d.status === 'fulfilled') setDaily(d.value);
      if (ord.status === 'fulfilled') setOrders(ord.value ?? []);
      if (exe.status === 'fulfilled') setExecutions(exe.value ?? []);
      if (stops.status === 'fulfilled') setStopOrders(stops.value ?? []);
      if (st.status === 'fulfilled') setInstanceStatus(st.value);
      else setInstanceStatus(null);
      setLastUpdate(new Date());
      setError(null);
    } catch (e: unknown) {
      setError(`Failed to load data: ${errorMessage(e)}`);
    }
  }, [selected]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadInstances();
      void api.aiTraderConfig().then((c) => setAiTraderArchived(Boolean(c.archived))).catch(() => setAiTraderArchived(true));
    }, 0);
    return () => window.clearTimeout(timer);
  }, [loadInstances]);

  useEffect(() => {
    const onHash = () => setTab(tabFromHash());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  const selectTab = useCallback((t: MainTab) => {
    setTab(t);
    applyTabToURL(t);
  }, []);

  const refreshAll = useCallback(() => {
    void loadInstances();
    void loadData();
  }, [loadInstances, loadData]);

  useEffect(() => {
    if (aiTraderArchived && tab === 'ai-trader') {
      selectTab('overview');
    }
  }, [aiTraderArchived, tab, selectTab]);

  useEffect(() => {
    const initial = window.setTimeout(() => {
      void loadData();
    }, 0);
    let timer: ReturnType<typeof setInterval>;
    const arm = () => {
      clearInterval(timer);
      const ms =
        typeof document !== 'undefined' && document.visibilityState === 'hidden'
          ? REFRESH_HIDDEN_MS
          : REFRESH_VISIBLE_MS;
      timer = setInterval(loadData, ms);
    };
    arm();
    const onVis = () => {
      if (document.visibilityState === 'visible') loadData();
      arm();
    };
    document.addEventListener('visibilitychange', onVis);
    return () => {
      window.clearTimeout(initial);
      clearInterval(timer);
      document.removeEventListener('visibilitychange', onVis);
    };
  }, [loadData]);

  const current = instances.find((i) => i.id === selected) ?? null;
  const brokerPositions = portfolio?.positions.filter((p) => p.in_instance) ?? [];
  const openBrokerPnl = brokerPositions.reduce((sum, p) => sum + p.expected_yield, 0);
  const mismatchCount = brokerPositions.filter((p) => Math.abs(p.quantity - (ledger?.quantities?.[p.instrument_uid] ?? 0)) > 1e-6).length;

  const tabs = useMemo((): { id: MainTab; label: string }[] => {
    const base: { id: MainTab; label: string }[] = [
      { id: 'overview', label: t('overview') },
      { id: 'chart', label: t('chart') },
      { id: 'portfolio', label: t('portfolio') },
      { id: 'history', label: t('history') },
      { id: 'assistant', label: t('assistant') },
      { id: 'lab', label: t('lab') },
      { id: 'guide', label: t('guide') },
    ];
    if (!aiTraderArchived) {
      return [
        ...base.slice(0, 4),
        { id: 'ai-trader', label: t('aiTrader') },
        ...base.slice(4),
      ];
    }
    return base;
  }, [t, aiTraderArchived]);

  return (
    <div className="min-h-screen p-4 max-w-7xl mx-auto">
      <header className="mb-6 grid gap-4 lg:grid-cols-[1fr_auto] lg:items-start">
        <div className="space-y-3 min-w-0">
          <h1 className="text-2xl font-bold text-[var(--text)]">{t('appTitle')}</h1>
          <Tabs tabs={tabs} active={tab} onChange={selectTab} />
          <p className="text-xs text-[var(--muted)]">
            {tab === 'ai-trader' ? (
              <span className="font-semibold text-[var(--warning)]">{t('aiTraderSeparate')}</span>
            ) : tab === 'lab' ? (
              <span className="font-semibold text-[var(--warning)]">{t('labSubtitle')}</span>
            ) : (
              <>
                {current?.tickers ? <span className="font-semibold text-[var(--warning)]">{current.tickers} · </span> : null}
                {current?.type ?? 'strategy'} · {t('autoRefresh')} ~{REFRESH_VISIBLE_MS / 1000}s
              </>
            )}
            {lastUpdate ? ` · ${t('lastUpdate')}: ${formatDateTime(lastUpdate.toISOString(), lang)}` : ''}
          </p>
        </div>
        <div className="space-y-3">
          <div className="flex flex-wrap justify-end gap-2">
            <button className="ui-button ui-button-secondary" onClick={() => setLang(lang === 'ru' ? 'en' : 'ru')}>
              {t('language')}: {lang.toUpperCase()}
            </button>
            <button className="ui-button ui-button-secondary" onClick={() => setMode(mode === 'dark' ? 'light' : 'dark')}>
              {t('theme')}: {mode === 'dark' ? t('dark') : t('light')}
            </button>
          </div>
          {tab !== 'ai-trader' && tab !== 'lab' && tab !== 'assistant' && instances.length > 0 && (
            <InstanceSelector instances={instances} selected={selected} onSelect={setSelected} />
          )}
        </div>
      </header>

      {error && (
        <div className="mb-4 rounded-lg border border-[var(--danger)] bg-[var(--danger-soft)] p-3 text-sm text-[var(--danger)]">
          {error}
        </div>
      )}

      {tab === 'overview' && (
        <>
          <BotControlBar
            instance={current}
            status={instanceStatus}
            portfolio={portfolio}
            recentEvents={events}
            onAction={refreshAll}
            onGoHistory={() => selectTab('history')}
          />
          <div className="mb-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Stat label={t('totalPnl')} value={formatMoney(pnl?.total_rub, lang)} tone={(pnl?.total_rub ?? 0) < 0 ? 'danger' : 'success'} sub={<LabelWithHelp label={pnl?.source ?? t('source')} help={t('expectedYieldHelp')} />} />
            <Stat label={t('expectedYield')} help={t('expectedYieldHelp')} value={formatMoney(portfolio?.expected_yield, lang)} tone={(portfolio?.expected_yield ?? 0) < 0 ? 'danger' : 'success'} />
            <Stat label={t('positions')} value={brokerPositions.length ? `${brokerPositions.length} ${t('open')}` : t('flat')} tone={brokerPositions.length ? 'info' : 'neutral'} sub={formatMoney(openBrokerPnl, lang)} />
            <Stat label={t('protectiveStop')} help={t('protectiveStopHelp')} value={stopOrders.length ? stopOrders.length : t('noStopOrders')} tone={brokerPositions.length && stopOrders.length === 0 ? 'danger' : 'success'} />
          </div>
          <div className="mb-6 grid gap-6 xl:grid-cols-[2fr_1fr]">
            <PositionOverview instance={current} indicator={indicator} ledger={ledger} portfolio={portfolio} stopOrders={stopOrders} />
            <StatsPanel pnl={pnl} ledger={ledger} portfolio={portfolio} daily={daily} orders={orders} executions={executions} stopOrders={stopOrders} />
          </div>
          <Card title={t('riskWarnings')} subtitle={t('brokerTruthHelp')}>
            {mismatchCount > 0 ? (
              <div className="rounded-lg border border-[var(--danger)] bg-[var(--danger-soft)] p-3 text-sm text-[var(--danger)]">
                {mismatchCount} ledger mismatch. Broker truth, runner ledger and strategy state must be aligned for live trading.
              </div>
            ) : (
              <div className="flex flex-wrap gap-2">
                <Badge tone="success">ledger aligned</Badge>
                <Badge tone={brokerPositions.length && stopOrders.length === 0 ? 'danger' : 'success'}>{t('protectiveStop')}: {stopOrders.length}</Badge>
                <Badge tone={current?.running ? 'success' : 'warning'}>{current?.running ? t('running') : t('stopped')}</Badge>
              </div>
            )}
          </Card>
        </>
      )}

      {tab === 'chart' && (
        <Card title={t('chart')} subtitle={`${current?.tickers ?? selected} · ${current?.type ?? ''}`}>
          <IndicatorChart key={`${selected}-${mode}-${lang}`} data={indicator} events={events} portfolio={portfolio} chartTheme={chart} lang={lang} />
        </Card>
      )}

      {tab === 'portfolio' && (
        <PortfolioPage portfolio={portfolio} ledger={ledger} stopOrders={stopOrders} />
      )}

      {tab === 'history' && (
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-[2fr_1fr]">
          <Card title={t('events')}>
            <EventLog events={events} />
          </Card>
          <HistoryTables orders={orders} executions={executions} stopOrders={stopOrders} />
        </div>
      )}

      {tab === 'assistant' && <AssistantPanel />}

      {tab === 'lab' && (
        <StrategyLabPanel onDeployed={() => void loadInstances()} />
      )}

      {tab === 'ai-trader' && <AiTraderPanel instances={instances} />}

      {tab === 'guide' && <GuidePage />}

      <AiChatPanel />
    </div>
  );
}

function PortfolioPage({ portfolio, ledger, stopOrders }: { portfolio: PortfolioSnapshot | null; ledger: LedgerData | null; stopOrders: StopOrder[] }) {
  const { t, lang } = useI18n();
  return (
    <div className="space-y-6">
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <Stat label={t('futures')} value={formatMoney(portfolio?.total_amount_futures, lang)} />
        <Stat label={t('cash')} value={formatMoney(portfolio?.total_amount_currencies, lang)} />
        <Stat label={t('shares')} value={formatMoney(portfolio?.total_amount_shares, lang)} />
        <Stat label={t('bonds')} value={formatMoney(portfolio?.total_amount_bonds, lang)} />
        <Stat label={t('etf')} value={formatMoney(portfolio?.total_amount_etf, lang)} />
      </div>
      <Card title={t('brokerTruth')} subtitle={t('brokerTruthHelp')}>
        {portfolio?.positions?.length ? (
          <Table>
            <thead><tr><th>{t('instrument')}</th><th>{t('quantity')}</th><th>{t('average')}</th><th>{t('current')}</th><th>{t('expectedYield')}</th><th>{t('status')}</th></tr></thead>
            <tbody>
              {portfolio.positions.map((p) => {
                const lq = ledger?.quantities?.[p.instrument_uid] ?? 0;
                const mismatch = Math.abs(lq - p.quantity) > 1e-6;
                return (
                  <tr key={p.instrument_uid}>
                    <td><div className="font-mono">{p.ticker || p.instrument_uid.slice(0, 8)}</div><div className="text-xs text-[var(--muted)]">{p.instrument_uid}</div></td>
                    <td>{formatNumber(p.quantity, lang, 4)}</td>
                    <td>{formatMoney(p.average_price, lang, p.currency || 'RUB')}</td>
                    <td>{formatMoney(p.current_price, lang, p.currency || 'RUB')}</td>
                    <td className={p.expected_yield < 0 ? 'text-[var(--danger)]' : 'text-[var(--success)]'}>{formatMoney(p.expected_yield, lang)}</td>
                    <td><Badge tone={mismatch ? 'danger' : p.in_instance ? 'success' : 'neutral'}>{mismatch ? 'ledger mismatch' : p.in_instance ? 'in strategy' : 'account'}</Badge></td>
                  </tr>
                );
              })}
            </tbody>
          </Table>
        ) : <EmptyState>{t('noPositions')}</EmptyState>}
      </Card>
      <Card title={t('protectiveStop')} subtitle={t('protectiveStopHelp')}>
        <StopOrdersTable stopOrders={stopOrders} />
      </Card>
    </div>
  );
}

function HistoryTables({ orders, executions, stopOrders }: { orders: OrderRecord[]; executions: ExecutionRecord[]; stopOrders: StopOrder[] }) {
  const { t, lang } = useI18n();
  return (
    <div className="space-y-6">
      <Card title={t('orders')}>
        {orders.length ? (
          <Table>
            <thead><tr><th>ID</th><th>{t('instrument')}</th><th>{t('quantity')}</th><th>{t('status')}</th><th>{t('current')}</th></tr></thead>
            <tbody>{orders.slice(0, 30).map((o) => (
              <tr key={`${o.OrderID}-${o.CreatedAt}`}><td className="font-mono">{o.OrderID.slice(0, 12)}…</td><td>{o.InstrumentUID.slice(0, 8)}</td><td>{o.Direction} {o.Quantity}</td><td>{o.OrderType}</td><td>{formatMoney(o.RefPrice, lang)}</td></tr>
            ))}</tbody>
          </Table>
        ) : <EmptyState>{t('noEvents')}</EmptyState>}
      </Card>
      <Card title={t('executions')}>
        {executions.length ? (
          <Table>
            <thead><tr><th>ID</th><th>{t('quantity')}</th><th>{t('average')}</th><th>{t('status')}</th></tr></thead>
            <tbody>{executions.slice(0, 30).map((e) => (
              <tr key={`${e.OrderID}-${e.CreatedAt}`}><td className="font-mono">{e.OrderID.slice(0, 12)}…</td><td>{e.FilledQty}</td><td>{formatMoney(e.AvgPrice, lang)}</td><td><Badge tone={e.Status?.toLowerCase() === 'filled' ? 'success' : 'neutral'}>{e.Status}</Badge></td></tr>
            ))}</tbody>
          </Table>
        ) : <EmptyState>{t('noEvents')}</EmptyState>}
      </Card>
      <Card title={t('stopOrders')}><StopOrdersTable stopOrders={stopOrders} /></Card>
    </div>
  );
}

function StopOrdersTable({ stopOrders }: { stopOrders: StopOrder[] }) {
  const { t, lang } = useI18n();
  if (!stopOrders.length) return <EmptyState>{t('noStopOrders')}</EmptyState>;
  return (
    <Table>
      <thead><tr><th>ID</th><th>{t('instrument')}</th><th>{t('quantity')}</th><th>Stop</th><th>{t('status')}</th></tr></thead>
      <tbody>{stopOrders.map((s) => (
        <tr key={s.stop_order_id}><td className="font-mono">{s.stop_order_id.slice(0, 12)}…</td><td>{s.instrument_uid.slice(0, 8)}</td><td>{s.direction} {s.lots}</td><td>{formatMoney(s.stop_price || s.price, lang)}</td><td><Badge tone="info">{s.status}</Badge></td></tr>
      ))}</tbody>
    </Table>
  );
}

function GuidePage() {
  const { t } = useI18n();
  const items = [
    ['brokerTruth', 'brokerTruthHelp'],
    ['runnerLedger', 'runnerLedgerHelp'],
    ['strategyState', 'strategyStateHelp'],
    ['expectedYield', 'expectedYieldHelp'],
    ['trailingStop', 'trailingStopHelp'],
    ['protectiveStop', 'protectiveStopHelp'],
    ['signalCancelled', 'signalCancelledHelp'],
    ['watchdogFlatten', 'watchdogFlattenHelp'],
  ] as const;
  return (
    <Card title={t('guide')} subtitle={t('guideText')}>
      <div className="grid gap-3 md:grid-cols-2">
        {items.map(([label, help]) => (
          <div key={label} className="rounded-lg border border-[var(--border)] bg-[var(--surface-muted)] p-3">
            <div className="font-semibold">{t(label)}</div>
            <div className="mt-1 text-sm text-[var(--muted)]">{t(help)}</div>
          </div>
        ))}
      </div>
    </Card>
  );
}
