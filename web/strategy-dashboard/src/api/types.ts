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

export interface AiTraderLimits {
  max_position_lots?: number;
  max_order_size?: number;
  max_active_orders?: number;
  max_trades_per_minute?: number;
  max_cancel_replace_per_minute?: number;
  max_session_loss_rub?: number;
  max_daily_loss_rub?: number;
  max_spread_bps?: number;
  stale_data_ms?: number;
  session_timeout_minutes?: number;
  observation_interval_ms?: number;
}

export interface AiTraderBookLevel {
  price: number;
  quantity: number;
}

export interface AiTraderWall {
  side: string;
  price: number;
  quantity: number;
  rank: number;
}

export interface AiTraderFeatures {
  uid: string;
  ticker?: string;
  observed_at: string;
  exchange_time?: string;
  depth: number;
  best_bid: number;
  best_ask: number;
  mid: number;
  spread_abs: number;
  spread_bps: number;
  top_bid_volume: number;
  top_ask_volume: number;
  imbalance: number;
  depth_skew: number;
  largest_bid_wall: AiTraderWall;
  largest_ask_wall: AiTraderWall;
  data_freshness_ms: number;
  stale: boolean;
  orderbook_top: {
    bids: AiTraderBookLevel[];
    asks: AiTraderBookLevel[];
  };
}

export interface AiTraderDecisionEvent {
  time: string;
  session_id: string;
  mode: string;
  action: string;
  intent: string;
  reason: string;
  summary?: string;
  market_bias?: string;
  next_watch?: string;
  operator_note?: string;
  confidence: number;
  risk_result: string;
  analysis_source?: 'llm' | 'rules' | 'rules_fallback' | 'session';
  llm_model?: string;
  features?: AiTraderFeatures;
}

export interface AiTraderCandleBar {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export interface AiTraderLevel {
  price: number;
  kind: string;
  source: string;
  rank: number;
}

export interface AiTraderPrint {
  time: string;
  direction: string;
  price: number;
  quantity: number;
}

export interface AiTraderTapeStats {
  window_sec: number;
  trade_count: number;
  buy_volume: number;
  sell_volume: number;
  last_price: number;
  vwap: number;
  delta_pct: number;
  aggressor?: string;
}

export interface AiTraderBookDigest {
  time: string;
  mid: number;
  spread_bps: number;
  imbalance: number;
  bid_wall: string;
  ask_wall: string;
}

export interface AiTraderFootprintCell {
  price: number;
  buy_vol: number;
  sell_vol: number;
  total: number;
}

export interface AiTraderFootprintColumn {
  time: string;
  label: string;
  cells: AiTraderFootprintCell[];
  total_vol: number;
  delta: number;
}

export interface AiTraderDOMBook {
  observed_at: string;
  tick_size: number;
  best_bid: number;
  best_ask: number;
  bids: AiTraderBookLevel[];
  asks: AiTraderBookLevel[];
}

export interface AiTraderMarketContext {
  chart_bars?: AiTraderCandleBar[];
  footprint?: AiTraderFootprintColumn[];
  dom_book?: AiTraderDOMBook;
  levels?: AiTraderLevel[];
  recent_prints?: AiTraderPrint[];
  tape_stats: AiTraderTapeStats;
  book_timeline?: AiTraderBookDigest[];
  scene_notes?: string[];
  updated_at?: string;
}

export interface CatalogInstrument {
  uid: string;
  ticker: string;
  name: string;
  class_code: string;
  instrument_type: string;
  exchange: string;
  kind: 'share' | 'future' | string;
}

export interface AiTraderBufferStats {
  book_samples: number;
  print_samples: number;
  chart_bars: number;
  level_count: number;
  daily_levels: number;
  hourly_levels: number;
  mid?: number;
  last_price?: number;
}

export interface AiTraderCollectEvent {
  time: string;
  kind: string;
  message: string;
  detail?: string;
}

export interface AiTraderPhaseProgress {
  collect_seconds: number;
  min_collect_sec: number;
  reports_ready?: string[];
  trading_ready: boolean;
  ready_reason?: string;
  buffer_stats?: AiTraderBufferStats;
}

export interface AiTraderLevelPlaybook {
  summary?: string;
  market_bias?: string;
  levels?: AiTraderLevel[];
  entry_rules?: string[];
  risk_notes?: string[];
  sl_mult_atr?: number;
  tp_mult_atr?: number;
  ready_to_trade?: boolean;
}

export interface PaperOrder {
  id: string;
  side: string;
  price: number;
  quantity: number;
  level_ref?: string;
  status: string;
  placed_at: string;
}

export interface PaperFill {
  time: string;
  side: string;
  price: number;
  quantity: number;
  note?: string;
}

export interface LiveOrder {
  id: string;
  broker_order_id?: string;
  side: string;
  price: number;
  quantity: number;
  level_ref?: string;
  status: string;
  placed_at: string;
}

export interface LiveFill {
  time: string;
  side: string;
  price: number;
  quantity: number;
  broker_order_id?: string;
  note?: string;
}

export interface LiveTradingState {
  position_lots: number;
  avg_price: number;
  realized_rub: number;
  stop_loss?: number;
  take_profit?: number;
  working_orders?: LiveOrder[];
  fills?: LiveFill[];
  halted?: boolean;
  halt_reason?: string;
  updated_at: string;
}

export interface AiTraderTradeSignal {
  side: string;
  level_price: number;
  confidence: number;
  reason?: string;
  risk_override?: string;
  received_at?: string;
}

export interface AiTraderPublicConfig {
  armed_live_enabled: boolean;
  stream_book: boolean;
  kill_switch: boolean;
  min_report_tf: string;
}

export interface PaperTradingState {
  position_lots: number;
  avg_price: number;
  realized_rub: number;
  working_orders?: PaperOrder[];
  fills?: PaperFill[];
  updated_at: string;
}

export type AiTraderPhase = 'collecting' | 'analyzing' | 'ready' | 'trading' | 'stopped';

export interface AiTraderSession {
  id: string;
  instance_id: string;
  account_id: string;
  instrument_uid: string;
  ticker?: string;
  strategy_kind?: string;
  mode?: string;
  instruction: string;
  limits: AiTraderLimits;
  status: string;
  phase?: AiTraderPhase;
  phase_progress?: AiTraderPhaseProgress;
  level_playbook?: AiTraderLevelPlaybook;
  paper_state?: PaperTradingState;
  execution_mode?: 'paper' | 'armed_live';
  live_state?: LiveTradingState;
  last_trade_signal?: AiTraderTradeSignal;
  session_regime?: string;
  started_at: string;
  updated_at: string;
  stopped_at?: string;
  last_error?: string;
  features?: AiTraderFeatures;
  market_context?: AiTraderMarketContext;
  last_decision?: AiTraderDecisionEvent;
  events?: AiTraderDecisionEvent[];
  collect_feed?: AiTraderCollectEvent[];
}

export type AdvisorTimeframe = '5m' | '15m' | '30m' | '1h' | '4h' | '1d' | 'strategy';

export interface AdvisorParticipantNote {
  role: string;
  notes: string;
}

export interface AdvisorLimitNote {
  side: string;
  price: number;
  quantity: number;
  event: string;
}

export interface AdvisorDensityNote {
  price: number;
  side: string;
  assessment: string;
  reason: string;
}

export interface AdvisorStructured {
  market_regime?: string;
  key_levels?: string[];
  participants?: AdvisorParticipantNote[];
  volume_notes?: string[];
  large_limits?: AdvisorLimitNote[];
  repositioning?: string[];
  mm_clouds?: string[];
  densities?: AdvisorDensityNote[];
  iceberg_hints?: string[];
  conclusions?: string[];
  next_watch?: string[];
  trading_ideas?: string[];
  confidence?: number;
}

export interface AdvisorAnalysisReport {
  id: string;
  session_id: string;
  timeframe: AdvisorTimeframe;
  period_start: string;
  period_end: string;
  status: 'ok' | 'failed' | 'pending' | string;
  summary_md?: string;
  structured?: AdvisorStructured;
  source_report_ids?: string[];
  model?: string;
  prompt_version?: string;
  error_message?: string;
  created_at: string;
}

export interface AdvisorStrategyDraft {
  id: string;
  session_id: string;
  kind: string;
  title: string;
  body: string;
  ticker?: string;
  instrument_uid?: string;
  created_at: string;
}

export interface AdvisorSynthesis {
  session_id: string;
  summary_md: string;
  structured?: AdvisorStructured;
  drafts?: AdvisorStrategyDraft[];
  model?: string;
  created_at: string;
}

export interface AdvisorStrategyResponse {
  synthesis: AdvisorSynthesis | null;
  reports: AdvisorAnalysisReport[];
}
