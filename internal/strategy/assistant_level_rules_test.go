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
		{ID: "L3", Price: 318, Source: "daily90_low 2026-04-01", Kind: "support", Strength: 5},
		{ID: "L4", Price: 278, Source: "daily_low 2024-03-01", Kind: "support", Strength: 5},
		{ID: "L5", Price: 322, Source: "volume_poc_1d", Kind: "poc", Strength: 4},
	}
	got := filterLevelsForChart(levels, "1d", ref)
	if len(got) != 4 {
		t.Fatalf("1d chart want 4 daily levels, got %d: %+v", len(got), got)
	}
	for _, l := range got {
		ok := strings.HasPrefix(l.Source, "daily") || strings.HasPrefix(l.Source, "volume_poc_1d")
		if !ok {
			t.Fatalf("unexpected on 1d chart: %+v", l)
		}
	}
}
