import type {
  Instance,
  InstanceStatus,
  FlattenResult,
  ReloadConfigResult,
  AssistantAnalysis,
  AssistantStartResponse,
  AssistantChartPayload,
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

async function post<T>(path: string, body: unknown, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  });
  return parseResponse<T>(res);
}

function postWithTimeout<T>(path: string, body: unknown, timeoutMs: number): Promise<T> {
  const ctrl = new AbortController();
  const timer = window.setTimeout(() => ctrl.abort(), timeoutMs);
  return post<T>(path, body, ctrl.signal).finally(() => window.clearTimeout(timer));
}

async function postNoContent(path: string): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  });
  if (!res.ok) {
    const raw = await res.text();
    try {
      const parsed = JSON.parse(raw) as { error?: string; message?: string };
      throw new Error(parsed.error || parsed.message || `${res.status} ${res.statusText}`);
    } catch (e) {
      if (e instanceof Error && e.message !== 'Unexpected token') throw e;
      throw new Error(raw || `${res.status} ${res.statusText}`);
    }
  }
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
  startInstance: (id: string) => postNoContent(`/instances/${encodeURIComponent(id)}/start`),
  stopInstance: (id: string) => postNoContent(`/instances/${encodeURIComponent(id)}/stop`),
  flattenInstance: (id: string) =>
    post<FlattenResult>(`/instances/${encodeURIComponent(id)}/flatten`, {}),
  instanceStatus: (id: string) =>
    get<InstanceStatus>(`/instances/${encodeURIComponent(id)}/status`),
  reloadConfig: () => post<ReloadConfigResult>('/config/reload', {}),
  startAssistantAnalysis: (ticker: string) =>
    post<AssistantStartResponse>('/assistant/analyses', { ticker }),
  getAssistantAnalysis: (id: string) =>
    get<AssistantAnalysis>(`/assistant/analyses/${encodeURIComponent(id)}`),
  getAssistantChart: (id: string, tf: string) =>
    get<AssistantChartPayload>(
      `/assistant/analyses/${encodeURIComponent(id)}/chart?tf=${encodeURIComponent(tf)}`,
    ),
  deleteAssistantAnalysis: async (id: string) => {
    const res = await fetch(`${BASE}/assistant/analyses/${encodeURIComponent(id)}`, { method: 'DELETE' });
    if (!res.ok) {
      const raw = await res.text();
      throw new Error(raw || `${res.status} ${res.statusText}`);
    }
  },
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
  aiTraderAnalystReport: (sessionID: string) =>
    get<import('./types').AiTraderSessionReport>(
      `/ai-trader/analyst/sessions/${encodeURIComponent(sessionID)}/report`,
    ),
  aiTraderRunPostmarket: (sessionID: string) =>
    post<import('./types').AiTraderSessionReport>(
      `/ai-trader/analyst/sessions/${encodeURIComponent(sessionID)}/postmarket`,
      {},
    ),
  aiTraderInstrumentJournal: (ticker: string) =>
    get<import('./types').AiTraderInstrumentJournal>(
      `/ai-trader/analyst/instruments/${encodeURIComponent(ticker)}/journal`,
    ),
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
  strategyLabCatalog: () => get<import('./types').StrategyLabCatalog>('/strategy-lab/catalog'),
  strategyLabAnalyze: (body: { ticker: string; uid?: string; days?: number; lang?: string }) =>
    postWithTimeout<import('./types').StrategyLabAnalyzeResponse>(
      '/strategy-lab/analyze',
      body,
      240_000,
    ),
  strategyLabCompare: (body: { ticker: string; uid?: string; days?: number }) =>
    postWithTimeout<import('./types').StrategyLabMatrixResponse>('/strategy-lab/compare', body, 240_000),
  strategyLabOptimize: (body: { uid: string; ticker: string; strategy: string; days?: number }) =>
    postWithTimeout<
      import('./types').StrategyLabOptimizeResponse | import('./types').StrategyLabMatrixResponse
    >('/strategy-lab/optimize', body, 180_000),
  strategyLabApply: (body: {
    phase?: 'stage' | 'enable_live';
    confirm_live?: boolean;
    analysis_verdict?: string;
    instance_id?: string;
    type: string;
    account_id?: string;
    instrument_uid: string;
    ticker: string;
    params: Record<string, string>;
    enabled?: boolean;
    start?: boolean;
  }) => post<import('./types').StrategyLabApplyResult>('/strategy-lab/apply', body),
  strategyLabInterpret: (body: {
    optimization?: import('./types').StrategyLabOptimization;
    ticker: string;
    days?: number;
    lang?: string;
    strategy?: string;
    rows: import('./types').StrategyLabRunRow[];
    selected?: import('./types').StrategyLabRunRow | null;
    production?: import('./types').StrategyLabRunRow | null;
  }) =>
    postWithTimeout<import('./types').StrategyLabInterpretResponse>(
      '/strategy-lab/interpret',
      {
        ...body,
        selected: body.selected ?? undefined,
        production: body.production ?? undefined,
      },
      100_000,
    ),
};
