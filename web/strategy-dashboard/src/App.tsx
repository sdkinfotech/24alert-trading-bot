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

const REFRESH_MS = 30_000;

export default function App() {
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
      setInstances(list ?? []);
      if (list?.length && (!selected || !list.find((i) => i.id === selected))) {
        setSelected(list[0].id);
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
    loadData();
    const timer = setInterval(loadData, REFRESH_MS);
    return () => clearInterval(timer);
  }, [loadData]);

  return (
    <div className="min-h-screen p-4 max-w-7xl mx-auto">
      <header className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-bold text-white">24alert Strategy Dashboard</h1>
          <p className="text-xs text-gray-500 mt-0.5">
            Auto-refresh every {REFRESH_MS / 1000}s
            {lastUpdate && (
              <> &middot; last: {lastUpdate.toLocaleTimeString('ru-RU')}</>
            )}
          </p>
        </div>
        {instances.length > 0 && (
          <InstanceSelector
            instances={instances}
            selected={selected}
            onSelect={setSelected}
          />
        )}
      </header>

      {error && (
        <div className="bg-red-900/40 border border-red-700 rounded-lg p-3 mb-4 text-sm text-red-300">
          {error}
        </div>
      )}

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
    </div>
  );
}
