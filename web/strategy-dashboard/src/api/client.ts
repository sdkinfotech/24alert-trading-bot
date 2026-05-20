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
  AiTraderBrokerSnapshot,
  AiTraderSession,
  AiTraderPersistedSummary,
  AdvisorAnalysisReport,
  AdvisorStrategyResponse,
  AdvisorTimeframe,
  CatalogInstrument,
  AiChatResponse,
  AiChatStatus,
} from './types';

const BASE = import.meta.env.VITE_API_BASE ?? '';

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  return parseResponse<T>(res);
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return parseResponse<T>(res);
}

async function parseResponse<T>(res: Response): Promise<T> {
  const contentType = res.headers.get('content-type') ?? '';
  const raw = await res.text();
  const isJSON = contentType.includes('application/json') || raw.trim().startsWith('{') || raw.trim().startsWith('[');
  if (!res.ok) {
    if (isJSON && raw) {
      try {
        const parsed = JSON.parse(raw) as { error?: string; message?: string };
        throw new Error(parsed.error || parsed.message || `${res.status} ${res.statusText}`);
      } catch (e) {
        if (e instanceof Error && !e.message.includes('Unexpected token')) throw e;
      }
    }
    const text = raw.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim();
    throw new Error(text || `${res.status} ${res.statusText}`);
  }
  if (!isJSON) {
    const text = raw.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim();
    throw new Error(text || 'Backend returned non-JSON response');
  }
  return JSON.parse(raw) as T;
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
  instrumentCatalog: (q = '', kind: 'all' | 'share' | 'future' = 'all', limit = 50) => {
    const params = new URLSearchParams();
    if (q.trim()) params.set('q', q.trim());
    if (kind !== 'all') params.set('kind', kind);
    params.set('limit', String(limit));
    return get<CatalogInstrument[]>(`/instruments/catalog?${params.toString()}`);
  },
  aiTraderSessions: () => get<AiTraderSession[]>('/ai-trader/sessions'),
  aiTraderPersistedSessions: () => get<AiTraderPersistedSummary[]>('/ai-trader/sessions/persisted'),
  resumeAiTraderSession: (sessionID: string, body: { reconnect_only?: boolean; resume_trading?: boolean }) =>
    post<AiTraderSession>(`/ai-trader/sessions/${encodeURIComponent(sessionID)}/resume`, body),
  aiTraderSession: (instanceID: string) =>
    get<AiTraderSession>(`/ai-trader/sessions/${encodeURIComponent(instanceID)}`),
  aiTraderConfig: () => get<import('./types').AiTraderPublicConfig>('/ai-trader/config'),
  setAiTraderKillSwitch: (active: boolean) =>
    post<{ kill_switch: boolean }>('/ai-trader/kill-switch', { active }),
  startAiTraderSession: (body: {
    instance_id?: string;
    account_id: string;
    instrument_uid: string;
    ticker?: string;
    strategy_kind?: string;
    mode?: string;
    confirm_live?: boolean;
    instruction: string;
    depth?: number;
    limits?: AiTraderLimits;
  }) => post<AiTraderSession>('/ai-trader/sessions', body),
  startAiTraderTrading: (sessionID: string) =>
    post<AiTraderSession>(`/ai-trader/sessions/${encodeURIComponent(sessionID)}/start-trading`, {}),
  stopAiTraderSession: (instanceID: string) =>
    post<AiTraderSession>(`/ai-trader/sessions/${encodeURIComponent(instanceID)}/stop`, {}),
  aiTraderBrokerPosition: (sessionID: string) =>
    get<AiTraderBrokerSnapshot>(`/ai-trader/sessions/${encodeURIComponent(sessionID)}/broker-position`),
  flattenAiTraderSession: (sessionID: string) =>
    post<AiTraderSession>(`/ai-trader/sessions/${encodeURIComponent(sessionID)}/flatten`, {}),
  advisorAnalyses: (sessionId: string, tf: AdvisorTimeframe, limit = 20) =>
    get<AdvisorAnalysisReport[]>(
      `/advisor/sessions/${encodeURIComponent(sessionId)}/analyses?tf=${encodeURIComponent(tf)}&limit=${limit}`,
    ),
  advisorReport: (sessionId: string, reportId: string) =>
    get<AdvisorAnalysisReport>(
      `/advisor/sessions/${encodeURIComponent(sessionId)}/analyses/${encodeURIComponent(reportId)}`,
    ),
  advisorStrategy: (sessionId: string) =>
    get<AdvisorStrategyResponse>(`/advisor/sessions/${encodeURIComponent(sessionId)}/strategy`),
  advisorFinalize: (sessionId: string) =>
    post<{ status: string }>(`/advisor/sessions/${encodeURIComponent(sessionId)}/finalize`, {}),
};
