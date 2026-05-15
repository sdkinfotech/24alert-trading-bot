package strategy

import (
	"fmt"
	"strings"

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
