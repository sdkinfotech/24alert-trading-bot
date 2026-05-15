export interface Instance {
  id: string;
  type: string;
  account_id: string;
  enabled_in_config: boolean;
  running: boolean;
}

export interface PnlData {
  instance_id: string;
  realized_rub: number;
  unrealized_rub: number;
  total_rub: number;
}

export interface LedgerData {
  instance_id: string;
  quantities: Record<string, number>;
  avg_prices: Record<string, number>;
  realized_rub: number;
}

export interface CandlePoint {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  fast_sma: number;
  slow_sma: number;
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
  fast_period: number;
  slow_period: number;
  position: number;
  candles: CandlePoint[];
  signals: SignalPoint[];
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
