import type {
  Instance,
  PnlData,
  LedgerData,
  PortfolioSnapshot,
  IndicatorData,
  TimelineEvent,
  DailySummary,
  OrderRecord,
  SignalRecord,
  ExecutionRecord,
  StopOrder,
  AiTraderLimits,
  AiTraderSession,
  AiChatResponse,
  AiChatStatus,
} from './types';

const BASE = import.meta.env.VITE_API_BASE ?? '';

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok && res.status !== 502) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
}

export const api = {
  instances: () => get<Instance[]>('/instances'),
  pnl: (id: string) => get<PnlData>(`/instances/${id}/pnl`),
  ledger: (id: string) => get<LedgerData>(`/instances/${id}/ledger`),
  portfolio: (id: string) => get<PortfolioSnapshot>(`/instances/${id}/portfolio`),
  indicator: (id: string) => get<IndicatorData>(`/instances/${id}/indicator`),
  events: (id: string, limit = 200) =>
    get<TimelineEvent[]>(`/instances/${id}/events?limit=${limit}`),
  signals: (id: string, limit = 200) =>
    get<SignalRecord[]>(`/instances/${id}/signals?limit=${limit}`),
  orders: (id: string, limit = 200) =>
    get<OrderRecord[]>(`/instances/${id}/orders?limit=${limit}`),
  executions: (id: string, limit = 200) =>
    get<ExecutionRecord[]>(`/instances/${id}/executions?limit=${limit}`),
  stopOrders: (id: string) => get<StopOrder[]>(`/instances/${id}/stop-orders`),
  daily: (date?: string) =>
    get<DailySummary>(`/report/daily${date ? `?date=${date}` : ''}`),
  aiChat: (message: string) =>
    post<AiChatResponse>('/ai-chat', { message }),
  aiChatReset: () => post<{ status: string }>('/ai-chat/reset', {}),
  aiChatStatus: () => get<AiChatStatus>('/ai-chat/status'),
  aiTraderSessions: () => get<AiTraderSession[]>('/ai-trader/sessions'),
  aiTraderSession: (instanceID: string) =>
    get<AiTraderSession>(`/ai-trader/sessions/${instanceID}`),
  startAiTraderSession: (body: {
    instance_id: string;
    mode: 'observe' | 'paper';
    instruction: string;
    depth?: number;
    limits?: AiTraderLimits;
  }) => post<AiTraderSession>('/ai-trader/sessions', body),
  stopAiTraderSession: (instanceID: string) =>
    post<AiTraderSession>(`/ai-trader/sessions/${instanceID}/stop`, {}),
};
