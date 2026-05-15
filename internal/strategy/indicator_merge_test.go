package strategy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/24alert/trading-bot/pkg/config"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

func TestParseChartCandleTime(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 30, 0, 123456789, time.UTC)
	cases := []struct {
		name string
		in   interface{}
		want time.Time
		ok   bool
	}{
		{"rfc3339nano", ts.Format(time.RFC3339Nano), ts.UTC(), true},
		{"rfc3339", ts.UTC().Format(time.RFC3339), ts.Truncate(time.Second).UTC(), true},
		{"nil", nil, time.Time{}, false},
		{"empty", "", time.Time{}, false},
		{"number", 12345, time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseChartCandleTime(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("time = %v, want %v", got, tc.want)
			}
		})
	}
}

// Chart merge must use the same interval resolution as runner.startInstance (empty → 5m, not 1h).
func TestMergeChartUsesSameIntervalAsRunnerEmptyParam(t *testing.T) {
	sub, err := ParseSubscriptionInterval("")
	if err != nil {
		t.Fatal(err)
	}
	if sub != pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES {
		t.Fatalf("empty interval: got %v, want FIVE_MINUTES", sub)
	}
	ci, err := SubscriptionToCandleInterval(sub)
	if err != nil {
		t.Fatal(err)
	}
	if ci != pb.CandleInterval_CANDLE_INTERVAL_5_MIN {
		t.Fatalf("candle interval: got %v, want 5_MIN", ci)
	}
}

func TestMergeRESTCandlesIntoIndicatorNilMdSvcReturnsBase(t *testing.T) {
	r := &Runner{mdSvc: nil}
	inst := config.StrategyInstanceConfig{
		ID:          "inst-a",
		Instruments: []string{"uid-1"},
		Params:      map[string]string{"interval": "1h"},
	}
	base := map[string]interface{}{
		"candles": []interface{}{
			map[string]interface{}{
				"time": "2024-06-01T10:00:00Z", "open": 1.0, "high": 2.0, "low": 0.5, "close": 1.5,
			},
		},
	}
	ctx := context.Background()
	got := r.mergeRESTCandlesIntoIndicator(ctx, inst, base)
	if fmt.Sprintf("%p", got) != fmt.Sprintf("%p", base) {
		t.Fatal("expected same underlying map when mdSvc is nil")
	}
}
