package strategy

import "testing"

type warmupFailureHandlerStrategy struct {
	failures int
}

func (s *warmupFailureHandlerStrategy) Info() StrategyInfo                    { return StrategyInfo{} }
func (s *warmupFailureHandlerStrategy) Configure(map[string]string) error     { return nil }
func (s *warmupFailureHandlerStrategy) OnCandle(Candle) []Signal              { return nil }
func (s *warmupFailureHandlerStrategy) OnOrderbook(Orderbook) []Signal        { return nil }
func (s *warmupFailureHandlerStrategy) OnExecution(ExecutionEvent)            {}
func (s *warmupFailureHandlerStrategy) Stop()                                 {}
func (s *warmupFailureHandlerStrategy) OnSignalDispatchFailed(Signal, string) { s.failures++ }

func TestDiscardWarmupSignalsClearsStrategyPendingState(t *testing.T) {
	st := &warmupFailureHandlerStrategy{}

	discardWarmupSignals(st, []Signal{{Direction: "buy"}, {Direction: "sell"}})

	if st.failures != 2 {
		t.Fatalf("failures = %d, want 2", st.failures)
	}
}
