package strategy

import "testing"

func TestAppendCollectFeed_CapAndThrottle(t *testing.T) {
	s := &AITraderSession{}
	for i := 0; i < aiTraderMaxCollectFeed+5; i++ {
		s.appendCollectFeed("print", "msg", "")
	}
	if len(s.CollectFeed) != aiTraderMaxCollectFeed {
		t.Fatalf("expected cap %d, got %d", aiTraderMaxCollectFeed, len(s.CollectFeed))
	}
}

func TestComputeHourlyLevels(t *testing.T) {
	// minimal smoke — empty input
	if lv := computeHourlyLevels(nil, 48); lv != nil {
		t.Fatalf("expected nil for empty candles")
	}
}
