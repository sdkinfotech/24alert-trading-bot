package strategy

import (
	"fmt"
	"strings"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// ParseSubscriptionInterval maps config strings (e.g. "5min") to T-Invest subscription intervals.
func ParseSubscriptionInterval(s string) (pb.SubscriptionInterval, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "5min", "5m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES, nil
	case "1min", "1m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE, nil
	case "2min", "2m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_2_MIN, nil
	case "3min", "3m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_3_MIN, nil
	case "10min", "10m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_10_MIN, nil
	case "15min", "15m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIFTEEN_MINUTES, nil
	case "30min", "30m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_30_MIN, nil
	case "1h", "60min", "60m":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR, nil
	case "2h":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_2_HOUR, nil
	case "4h":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_4_HOUR, nil
	case "1d", "day":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_DAY, nil
	case "1w", "week":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_WEEK, nil
	case "1mo", "month":
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_MONTH, nil
	default:
		return pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_UNSPECIFIED, fmt.Errorf("unknown candle interval %q", s)
	}
}

// SubscriptionToCandleInterval maps a streaming SubscriptionInterval to the
// corresponding historic CandleInterval used by GetCandles.
func SubscriptionToCandleInterval(si pb.SubscriptionInterval) (pb.CandleInterval, error) {
	switch si {
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE:
		return pb.CandleInterval_CANDLE_INTERVAL_1_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_2_MIN:
		return pb.CandleInterval_CANDLE_INTERVAL_2_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_3_MIN:
		return pb.CandleInterval_CANDLE_INTERVAL_3_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES:
		return pb.CandleInterval_CANDLE_INTERVAL_5_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_10_MIN:
		return pb.CandleInterval_CANDLE_INTERVAL_10_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIFTEEN_MINUTES:
		return pb.CandleInterval_CANDLE_INTERVAL_15_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_30_MIN:
		return pb.CandleInterval_CANDLE_INTERVAL_30_MIN, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR:
		return pb.CandleInterval_CANDLE_INTERVAL_HOUR, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_2_HOUR:
		return pb.CandleInterval_CANDLE_INTERVAL_2_HOUR, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_4_HOUR:
		return pb.CandleInterval_CANDLE_INTERVAL_4_HOUR, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_DAY:
		return pb.CandleInterval_CANDLE_INTERVAL_DAY, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_WEEK:
		return pb.CandleInterval_CANDLE_INTERVAL_WEEK, nil
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_MONTH:
		return pb.CandleInterval_CANDLE_INTERVAL_MONTH, nil
	default:
		return pb.CandleInterval_CANDLE_INTERVAL_UNSPECIFIED,
			fmt.Errorf("cannot map subscription interval %v to candle interval", si)
	}
}

// IntervalDuration returns the approximate wall-clock duration for a
// subscription interval, used to compute how far back to fetch history.
func IntervalDuration(si pb.SubscriptionInterval) time.Duration {
	switch si {
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE:
		return time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_2_MIN:
		return 2 * time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_3_MIN:
		return 3 * time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES:
		return 5 * time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_10_MIN:
		return 10 * time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIFTEEN_MINUTES:
		return 15 * time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_30_MIN:
		return 30 * time.Minute
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_HOUR:
		return time.Hour
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_2_HOUR:
		return 2 * time.Hour
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_4_HOUR:
		return 4 * time.Hour
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_DAY:
		return 24 * time.Hour
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_WEEK:
		return 7 * 24 * time.Hour
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_MONTH:
		return 30 * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}
