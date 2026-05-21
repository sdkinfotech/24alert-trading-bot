package strategy

import (
	"strings"
	"testing"
)

func TestFilterLevelsForChart1dOnlyDaily(t *testing.T) {
	ref := 320.0
	levels := []AssistantLevel{
		{ID: "L1", Price: 329, Source: "daily_high 2025-07-17", Kind: "resistance", Strength: 5},
		{ID: "L2", Price: 325, Source: "mirror S42/R64", Kind: "mirror", Strength: 5},
		{ID: "L3", Price: 278, Source: "daily_low 2024-03-01", Kind: "support", Strength: 5},
	}
	got := filterLevelsForChart(levels, "1d", ref)
	if len(got) != 2 {
		t.Fatalf("1d chart want 2 daily levels, got %d: %+v", len(got), got)
	}
	for _, l := range got {
		if !strings.HasPrefix(l.Source, "daily") {
			t.Fatalf("unexpected on 1d chart: %+v", l)
		}
	}
}
