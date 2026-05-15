import type {
  Instance,
  PnlData,
  LedgerData,
  IndicatorData,
  TimelineEvent,
  DailySummary,
} from './types';

const BASE = import.meta.env.VITE_API_BASE ?? '';

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
}

export const api = {
  instances: () => get<Instance[]>('/instances'),
  pnl: (id: string) => get<PnlData>(`/instances/${id}/pnl`),
  ledger: (id: string) => get<LedgerData>(`/instances/${id}/ledger`),
  indicator: (id: string) => get<IndicatorData>(`/instances/${id}/indicator`),
  events: (id: string, limit = 200) =>
    get<TimelineEvent[]>(`/instances/${id}/events?limit=${limit}`),
  daily: (date?: string) =>
    get<DailySummary>(`/report/daily${date ? `?date=${date}` : ''}`),
};
