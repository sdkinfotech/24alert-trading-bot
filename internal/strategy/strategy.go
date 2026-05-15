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
