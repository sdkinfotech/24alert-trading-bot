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
