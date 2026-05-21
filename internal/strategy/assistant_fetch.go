package strategy

import (
	"context"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/marketdata"
)

type assistantCandleSet struct {
	Daily1y   []marketdata.Candle
	Daily90d  []marketdata.Candle
	Hourly1m  []marketdata.Candle
	Hourly1w  []marketdata.Candle
	FiveMin7d []marketdata.Candle
}

func marketCandleToAssistantBar(c marketdata.Candle) AssistantCandleBar {
	return AssistantCandleBar{
		Time:   c.Time.UTC().Format(time.RFC3339),
		Open:   c.Open,
		High:   c.High,
		Low:    c.Low,
		Close:  c.Close,
		Volume: c.Volume,
	}
}

func candlesToAssistantBars(candles []marketdata.Candle) []AssistantCandleBar {
	out := make([]AssistantCandleBar, 0, len(candles))
	for _, c := range candles {
		out = append(out, marketCandleToAssistantBar(c))
	}
	return out
}

func (r *Runner) fetchAssistantCandles(ctx context.Context, uid string) (assistantCandleSet, error) {
	now := time.Now().UTC()
	var out assistantCandleSet
	var err error

	out.Daily1y, err = r.fetchHistoricCandles(ctx, uid, now.AddDate(-1, 0, 0), now, pb.CandleInterval_CANDLE_INTERVAL_DAY)
	if err != nil {
		return out, err
	}
	out.Daily90d = tailCandles(out.Daily1y, 95)

	out.Hourly1m, err = r.fetchHistoricCandles(ctx, uid, now.AddDate(0, 0, -32), now, pb.CandleInterval_CANDLE_INTERVAL_HOUR)
	if err != nil {
		return out, err
	}
	out.Hourly1w = tailCandles(out.Hourly1m, 7*18)

	out.FiveMin7d, err = r.fetchHistoricCandles(ctx, uid, now.Add(-7*24*time.Hour), now, pb.CandleInterval_CANDLE_INTERVAL_5_MIN)
	if err != nil {
		return out, err
	}
	return out, nil
}

func tailCandles(candles []marketdata.Candle, n int) []marketdata.Candle {
	if n <= 0 || len(candles) <= n {
		return candles
	}
	return candles[len(candles)-n:]
}

func lastClose(candles []marketdata.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	return candles[len(candles)-1].Close
}
