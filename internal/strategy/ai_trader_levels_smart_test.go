package strategy

import "testing"

func TestSelectTradeableLevelsExcludesWalls(t *testing.T) {
	mid := 106.0
	levels := []AITraderLevel{
		{Price: 105.72, Kind: "support", Source: "bid_wall", Rank: 6},
		{Price: 104.14, Kind: "support", Source: "daily_low 2026-05-20", Rank: 2},
		{Price: 106.52, Kind: "support", Source: "hourly_low 2026-05-20 14:00", Rank: 2},
		{Price: 112.0, Kind: "resistance", Source: "daily_high 2026-05-18", Rank: 1},
	}
	out := selectTradeableLevels(levels, mid, "BMM6")
	if len(out) == 0 {
		t.Fatal("expected structural levels near mid")
	}
	for _, lv := range out {
		if isBookWallSource(lv.Source) {
			t.Fatalf("wall should be excluded: %s", lv.Source)
		}
	}
}

func TestEntrySignalAllowedAtStructuralLevel(t *testing.T) {
	mid := 106.0
	tradeable := []AITraderLevel{
		{Price: 105.90, Kind: "support", Source: "hourly_low 2026-05-20 14:00"},
	}
	sig := &AITraderTradeSignal{Side: "buy", LevelPrice: 105.92, Confidence: 0.6}
	f := &AITraderFeatures{Mid: mid}
	if !entrySignalAllowed(sig, tradeable, f, nil) {
		t.Fatal("expected buy near hourly support allowed")
	}
	sigWall := &AITraderTradeSignal{Side: "sell", LevelPrice: 105.76, Confidence: 0.6}
	if entrySignalAllowed(sigWall, tradeable, f, nil) {
		t.Fatal("sell at ask wall without resistance level should be blocked")
	}
}

func TestNormalizeQuotationPrice(t *testing.T) {
	mid := 106.0
	got := normalizeQuotationPrice(7564.19, mid)
	if got < 105 || got > 107 {
		t.Fatalf("want ~106 pts, got %.4f", got)
	}
	if normalizeQuotationPrice(106.2, mid) != 106.2 {
		t.Fatal("points should pass through")
	}
}
