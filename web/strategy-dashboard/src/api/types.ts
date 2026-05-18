export interface Instance {
  id: string;
  type: string;
  /** MOEX tickers from instrument cache (comma-separated if several UIDs). */
  tickers?: string;
  account_id: string;
  instruments?: string[];
  params?: Record<string, string>;
  enabled_in_config: boolean;
  running: boolean;
}

export interface PnlData {
  instance_id: string;
  realized_rub: number;
  unrealized_rub: number;
  total_rub: number;
  source?: string;
}

export interface LedgerData {
  instance_id: string;
  quantities: Record<string, number>;
  avg_prices: Record<string, number>;
  realized_rub: number;
}

export interface PortfolioPosition {
  instrument_uid: string;
  ticker?: string;
  instrument_type?: string;
  figi?: string;
  quantity: number;
  average_price: number;
  current_price: number;
  expected_yield: number;
  currency?: string;
  blocked: boolean;
  in_instance: boolean;
}

export interface PortfolioSnapshot {
  instance_id: string;
  account_id: string;
  total_amount_shares: number;
  total_amount_bonds: number;
  total_amount_etf: number;
  total_amount_currencies: number;
  total_amount_futures: number;
  expected_yield: number;
  positions: PortfolioPosition[];
  instance_position_count: number;
  last_broker_sync: string;
  portfolio_error?: string;
}

export interface CandlePoint {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  fast_sma?: number;
  slow_sma?: number;
  range_high?: number;
  range_low?: number;
  /** Level Bounce: primary support / resistance at this bar */
  support?: number;
  resistance?: number;
}

export interface SignalPoint {
  time: string;
  direction: string;
  reason: string;
  ref_price: number;
}

export interface IndicatorData {
  instance_id?: string;
  instrument_uid?: string;
  /** Config params.interval (empty means runner default: 5m). */
  chart_interval_param?: string;
  chart_instrument_uid?: string;
  /** Proto enum string; must match CandleHub subscription. */
  chart_subscription_interval?: string;
  chart_rest_interval?: string;
  strategy_type?: string;
  fast_period?: number;
  slow_period?: number;
  trailing_stop_pct?: number;
  trailing_best_price?: number;
  trailing_stop_price?: number;
  trailing_stop_active?: boolean;
  position: number;
  candles: CandlePoint[];
  signals: SignalPoint[];
  range_high?: number;
  range_low?: number;
  range_formed?: boolean;
  current_day?: string;
  /** Level Bounce: all support / resistance levels (from daily stack) */
  support?: number[];
  resistance?: number[];
  support_sources?: LevelSource[];
  resistance_sources?: LevelSource[];
  level_method?: string;
  level_days?: number;
  atr?: number;
}

export interface LevelSource {
  price: number;
  date: string;
  kind: 'low' | 'high';
  rank: number;
}

export interface SignalRecord {
  InstanceID: string;
  InstrumentUID: string;
  Direction: string;
  Quantity: number;
  OrderType: string;
  RefPrice: number;
  Reason: string;
  CreatedAt: string;
}

export interface OrderRecord {
  InstanceID: string;
  OrderID: string;
  InstrumentUID: string;
  Direction: string;
  Quantity: number;
  OrderType: string;
  RefPrice: number;
  CreatedAt: string;
}

export interface ExecutionRecord {
  InstanceID: string;
  OrderID: string;
  InstrumentUID: string;
  Status: string;
  FilledQty: number;
  AvgPrice: number;
  Message: string;
  CreatedAt: string;
}

export interface StopOrder {
  stop_order_id: string;
  instrument_uid: string;
  direction: string;
  stop_order_type: string;
  lots: number;
  stop_price: number;
  price: number;
  status: string;
  created_at?: string;
  expiration_at?: string;
}

export interface TimelineEvent {
  type: string;
  time: string;
  instance_id: string;
  instrument_uid?: string;
  direction?: string;
  quantity?: number;
  order_type?: string;
  ref_price?: number;
  reason?: string;
  order_id?: string;
  status?: string;
  filled_qty?: number;
  avg_price?: number;
  message?: string;
}

export interface DailySummary {
  DayUTC: string;
  SignalsCount: number;
  OrdersCount: number;
  ExecutionsCount: number;
}

export interface AiChatResponse {
  reply?: string;
  model?: string;
  error?: string;
}

export interface AiChatStatus {
  available: boolean;
  model: string;
  scanner_cron: boolean;
  cursor_key_set: boolean;
}
