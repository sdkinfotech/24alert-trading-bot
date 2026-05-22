package strategy

// Strategy is implemented by built-in Go strategies and by the gRPC adapter.
type Strategy interface {
	Info() StrategyInfo
	Configure(params map[string]string) error
	OnCandle(candle Candle) []Signal
	OnOrderbook(ob Orderbook) []Signal
	OnExecution(event ExecutionEvent)
	Stop()
}

// SignalDispatchFailureHandler is optional: called when the runner could not submit
// an order for a signal (risk rejection, risk error, PostOrder error). Strategies
// that optimistically updated internal state when emitting the signal should revert it here.
type SignalDispatchFailureHandler interface {
	OnSignalDispatchFailed(sig Signal, stage string)
}

// PostWarmupCleanup is optional: called after historical candle warmup so strategies
// can clear simulated position / pending flags while keeping indicator buffers.
type PostWarmupCleanup interface {
	ResetTradingStateAfterWarmup()
}

// BrokerPositionSyncer is optional: implemented by strategies that can align
// their confirmed trading state with broker portfolio before live candles start.
type BrokerPositionSyncer interface {
	SyncBrokerPosition(instrumentUID string, quantity float64, averagePrice float64, currentPrice float64)
}

// LiveCandleHandler is optional: it receives in-progress stream candles.
// Strategies should use it only for protective logic that is safe before bar close
// (for example trailing stops), not for bar-close indicators like SMA crosses.
type LiveCandleHandler interface {
	OnLiveCandle(candle Candle) []Signal
}

// ProtectiveStopProvider is optional: supplies broker stop-loss price after entry.
// When set, runner prefers this over entry±pct when it is wider (e.g. swing high for shorts).
type ProtectiveStopProvider interface {
	ProtectiveStopPrice(instrumentUID string, quantity, avgPrice float64) (stopPrice float64, ok bool)
}
