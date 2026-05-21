package strategy

import (
	"strings"
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func TestFilterLevelsForChart1dOnlyDaily(t *testing.T) {
	ref := 320.0
	levels := []AssistantLevel{
		{ID: "L1", Price: 329, Source: "daily_high 2025-07-17", Kind: "resistance", Strength: 5},
		{ID: "L2", Price: 325, Source: "mirror S42/R64", Kind: "mirror", Strength: 5},
		{ID: "L3", Price: 318, Source: "daily90_low 2026-04-01", Kind: "support", Strength: 5},
		{ID: "L4", Price: 322, Source: "volume_poc_1d", Kind: "poc", Strength: 4},
		{ID: "L5", Price: 321, Source: "daily_swing_support rank1", Kind: "support", Strength: 4},
	}
	got := filterLevelsForChart(levels, "1d", ref)
	if len(got) < 3 {
		t.Fatalf("1d chart want >=3 daily levels, got %d: %+v", len(got), got)
	}
	for _, l := range got {
		if !isDailyStructuralSource(l.Source) && !strings.HasPrefix(l.Source, "daily_swing") {
			t.Fatalf("unexpected on 1d chart: %+v", l)
		}
	}
}

func TestCountTouchesDeduped(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	candles := []marketdata.Candle{
		{Time: now, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 10},
		{Time: now.Add(time.Hour), Open: 100.5, High: 101, Low: 99.5, Close: 100.2, Volume: 10},
		{Time: now.Add(2 * time.Hour), Open: 100.2, High: 105, Low: 100, Close: 104, Volume: 10},
		{Time: now.Add(3 * time.Hour), Open: 104, High: 104.5, Low: 103, Close: 103.5, Volume: 10},
	}
	touches, _ := countTouchesDeduped(candles, 100, 1.0)
	if touches != 1 {
		t.Fatalf("deduped touches want 1, got %d", touches)
	}
}

func TestRejectLevelsNearZones(t *testing.T) {
	zones := []AssistantLevel{{Price: 100}}
	candidates := []AssistantLevel{{Price: 100.2}, {Price: 110}}
	out := rejectLevelsNearZones(candidates, zones, 0.5)
	if len(out) != 1 || out[0].Price != 110 {
		t.Fatalf("reject near zones: %+v", out)
	}
}

func TestBuildAssistantChartsUnifiedRef(t *testing.T) {
	cs := assistantCandleSet{
		Daily1y:   []marketdata.Candle{{Close: 300}},
		FiveMin7d: []marketdata.Candle{{Close: 322.5}},
	}
	ref := refPrice(cs)
	if ref != 322.5 {
		t.Fatalf("refPrice want 322.5, got %v", ref)
	}
	charts := buildAssistantCharts(cs, nil)
	for tf, ch := range charts {
		_ = tf
		_ = ch
	}
	_ = filterLevelsForChart([]AssistantLevel{{Price: 322, Source: "volume_poc_5m"}}, "5m", ref)
}
