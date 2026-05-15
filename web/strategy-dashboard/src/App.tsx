import { useEffect, useState, useCallback } from 'react';
import { api } from './api/client';
import type {
  Instance,
  PnlData,
  LedgerData,
  IndicatorData,
  TimelineEvent,
  DailySummary,
} from './api/types';
import { IndicatorChart } from './components/IndicatorChart';
import { EventLog } from './components/EventLog';
import { StatsPanel } from './components/StatsPanel';
import { InstanceSelector } from './components/InstanceSelector';
import { AiChatPanel } from './components/AiChatPanel';
import { SystemGuidePage } from './components/SystemGuidePage';

/** Polling while «Мониторинг» is visible; slower when tab in background. */
const REFRESH_VISIBLE_MS = 5_000;
const REFRESH_HIDDEN_MS = 30_000;

type MainTab = 'monitor' | 'guide';

function tabFromHash(): MainTab {
  const h = window.location.hash.slice(1).toLowerCase();
  if (h === 'guide' || h === 'about' || h === 'справка') return 'guide';
  return 'monitor';
}

function applyTabToURL(t: MainTab) {
  const base = window.location.pathname + window.location.search;
  window.history.replaceState(null, '', t === 'guide' ? `${base}#guide` : base);
}

export default function App() {
  const [tab, setTab] = useState<MainTab>(() =>
    typeof window !== 'undefined' && tabFromHash() === 'guide' ? 'guide' : 'monitor'
  );
  const [guidePageURL, setGuidePageURL] = useState('');
  const [instances, setInstances] = useState<Instance[]>([]);
  const [selected, setSelected] = useState('');
  const [indicator, setIndicator] = useState<IndicatorData | null>(null);
  const [pnl, setPnl] = useState<PnlData | null>(null);
  const [ledger, setLedger] = useState<LedgerData | null>(null);
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [daily, setDaily] = useState<DailySummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  const loadInstances = useCallback(async () => {
    try {
      const list = await api.instances();
      const enabled = (list ?? []).filter((i) => i.enabled_in_config);
      setInstances(enabled);
      if (enabled.length && (!selected || !enabled.find((i) => i.id === selected))) {
        setSelected(enabled[0].id);
      }
      setError(null);
    } catch (e: any) {
      setError(`Failed to load instances: ${e.message}`);
    }
  }, [selected]);

  const loadData = useCallback(async () => {
    if (!selected) return;
    try {
      const [ind, p, l, ev, d] = await Promise.allSettled([
        api.indicator(selected),
        api.pnl(selected),
        api.ledger(selected),
        api.events(selected, 200),
        api.daily(),
      ]);
      if (ind.status === 'fulfilled') setIndicator(ind.value);
      if (p.status === 'fulfilled') setPnl(p.value);
      if (l.status === 'fulfilled') setLedger(l.value);
      if (ev.status === 'fulfilled') setEvents(ev.value ?? []);
      if (d.status === 'fulfilled') setDaily(d.value);
      setLastUpdate(new Date());
      setError(null);
    } catch (e: any) {
      setError(`Failed to load data: ${e.message}`);
    }
  }, [selected]);

  useEffect(() => {
    loadInstances();
  }, [loadInstances]);

  useEffect(() => {
    setGuidePageURL(`${window.location.origin}${window.location.pathname}#guide`);
  }, []);

  useEffect(() => {
    const onHash = () => setTab(tabFromHash());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  const selectTab = useCallback((t: MainTab) => {
    setTab(t);
    applyTabToURL(t);
  }, []);

  useEffect(() => {
    if (tab !== 'monitor') return;
    loadData();
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
      clearInterval(timer);
      document.removeEventListener('visibilitychange', onVis);
    };
  }, [loadData, tab]);

  return (
    <div className="min-h-screen p-4 max-w-7xl mx-auto">
      <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between mb-6">
        <div className="space-y-3 min-w-0">
          <h1 className="text-xl font-bold text-white">24alert Strategy Dashboard</h1>
          <div
            className="inline-flex rounded-lg border border-gray-800 bg-gray-900/80 p-0.5"
            role="tablist"
            aria-label="Разделы"
          >
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'monitor'}
              className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                tab === 'monitor'
                  ? 'bg-gray-800 text-white shadow-sm'
                  : 'text-gray-500 hover:text-gray-300'
              }`}
              onClick={() => selectTab('monitor')}
            >
              Мониторинг
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'guide'}
              className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                tab === 'guide'
                  ? 'bg-gray-800 text-white shadow-sm'
                  : 'text-gray-500 hover:text-gray-300'
              }`}
              onClick={() => selectTab('guide')}
            >
              О системе
            </button>
          </div>
          {guidePageURL && (
            <p className="text-xs text-gray-500">
              Справка по прямой ссылке:{' '}
              <a
                href="#guide"
                className="text-amber-400/90 hover:text-amber-300 underline underline-offset-2 font-mono break-all"
                onClick={(e) => {
                  e.preventDefault();
                  selectTab('guide');
                }}
              >
                {guidePageURL}
              </a>
            </p>
          )}
          {tab === 'monitor' &&
            (() => {
              const cur = instances.find((i) => i.id === selected);
              if (!cur) {
                return (
                  <p className="text-xs text-gray-500">
                    Автообновление ~{REFRESH_VISIBLE_MS / 1000} с (фон ~{REFRESH_HIDDEN_MS / 1000} с)
                    {lastUpdate && (
                      <> · последнее: {lastUpdate.toLocaleTimeString('ru-RU')}</>
                    )}
                  </p>
                );
              }
              return (
                <p className="text-xs text-gray-500">
                  {cur.tickers && (
                    <>
                      <span className="text-amber-400 font-semibold">{cur.tickers}</span>
                      {' · '}
                    </>
                  )}
                  <span className="text-gray-400">{cur.type}</span>
                  {' · '}
                  автообновление ~{REFRESH_VISIBLE_MS / 1000} с (фон ~{REFRESH_HIDDEN_MS / 1000} с)
                  {lastUpdate && <> · последнее: {lastUpdate.toLocaleTimeString('ru-RU')}</>}
                </p>
              );
            })()}
        </div>
        {tab === 'monitor' && instances.length > 0 && (
          <InstanceSelector
            instances={instances}
            selected={selected}
            onSelect={setSelected}
          />
        )}
      </header>

      {tab === 'monitor' && error && (
        <div className="bg-red-900/40 border border-red-700 rounded-lg p-3 mb-4 text-sm text-red-300">
          {error}
        </div>
      )}

      {tab === 'monitor' ? (
        <>
          <section className="mb-6">
            <IndicatorChart key={selected} data={indicator} />
          </section>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2">
              <EventLog events={events} />
            </div>
            <div>
              <StatsPanel pnl={pnl} ledger={ledger} daily={daily} />
            </div>
          </div>
        </>
      ) : (
        <section className="rounded-xl border border-gray-800 bg-gray-900/40 p-6 mb-6">
          <SystemGuidePage />
        </section>
      )}

      <AiChatPanel />
    </div>
  );
}
