package strategy

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func TestComputeDailyLevels(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	daily := []marketdata.Candle{
		{High: 110, Low: 100, Time: now.AddDate(0, 0, -2)},
		{High: 115, Low: 105, Time: now.AddDate(0, 0, -1)},
		{High: 120, Low: 108, Time: now},
	}
	levels := computeDailyLevels(daily, 3)
	if len(levels) != 6 {
		t.Fatalf("expected 6 levels, got %d", len(levels))
	}
}

func TestComputeTapeStats(t *testing.T) {
	now := time.Now().UTC()
	prints := []AITraderPrint{
		{Time: now.Add(-5 * time.Second).Format(time.RFC3339), Direction: "buy", Price: 100, Quantity: 10},
		{Time: now.Add(-3 * time.Second).Format(time.RFC3339), Direction: "sell", Price: 99.9, Quantity: 5},
		{Time: now.Add(-1 * time.Second).Format(time.RFC3339), Direction: "buy", Price: 100.1, Quantity: 8},
	}
	stats := computeTapeStatsLocked(prints, 60)
	if stats.TradeCount != 3 || stats.BuyVolume == 0 {
		t.Fatalf("unexpected tape stats: %+v", stats)
	}
}
