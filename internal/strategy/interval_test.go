package strategy

import (
	"testing"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

func TestSubscriptionToCandleInterval(t *testing.T) {
	tests := []struct {
		sub  pb.SubscriptionInterval
		want pb.CandleInterval
	}{
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE, pb.CandleInterval_CANDLE_INTERVAL_1_MIN},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES, pb.CandleInterval_CANDLE_INTERVAL_5_MIN},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIFTEEN_MINUTES, pb.CandleInterval_CANDLE_INTERVAL_15_MIN},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR, pb.CandleInterval_CANDLE_INTERVAL_HOUR},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_DAY, pb.CandleInterval_CANDLE_INTERVAL_DAY},
	}
	for _, tc := range tests {
		got, err := SubscriptionToCandleInterval(tc.sub)
		if err != nil {
			t.Fatalf("SubscriptionToCandleInterval(%v): %v", tc.sub, err)
		}
		if got != tc.want {
			t.Fatalf("SubscriptionToCandleInterval(%v) = %v, want %v", tc.sub, got, tc.want)
		}
	}
}

func TestSubscriptionToCandleIntervalUnspecified(t *testing.T) {
	_, err := SubscriptionToCandleInterval(pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_UNSPECIFIED)
	if err == nil {
		t.Fatal("expected error for UNSPECIFIED")
	}
}

func TestParseSubscriptionInterval(t *testing.T) {
	tests := []struct {
		in   string
		want pb.SubscriptionInterval
	}{
		{"", pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES},
		{"15m", pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIFTEEN_MINUTES},
		{"1h", pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR},
		{"  1h  ", pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR},
	}
	for _, tc := range tests {
		got, err := ParseSubscriptionInterval(tc.in)
		if err != nil {
			t.Fatalf("ParseSubscriptionInterval(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseSubscriptionInterval(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseSubscriptionIntervalUnknown(t *testing.T) {
	_, err := ParseSubscriptionInterval("fortnightly")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIntervalDuration(t *testing.T) {
	tests := []struct {
		sub  pb.SubscriptionInterval
		want time.Duration
	}{
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE, time.Minute},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES, 5 * time.Minute},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIFTEEN_MINUTES, 15 * time.Minute},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR, time.Hour},
		{pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_DAY, 24 * time.Hour},
	}
	for _, tc := range tests {
		got := IntervalDuration(tc.sub)
		if got != tc.want {
			t.Fatalf("IntervalDuration(%v) = %v, want %v", tc.sub, got, tc.want)
		}
	}
}
