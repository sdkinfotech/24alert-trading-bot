package strategy

import (
	"context"
	"sort"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/marketdata"
)

const (
	aiTraderChartHistoryWindow = 7 * 24 * time.Hour
	aiTraderMaxChartBars1m     = 11000 // ~7 calendar days of 1m bars
	aiTraderMaxChartBars5m     = 2200  // ~7 days of 5m bars
	tinvestMaxCandlesPerReq    = 2400
)

// CandleIntervalDuration returns wall-clock span of one historic candle.
func CandleIntervalDuration(ci pb.CandleInterval) time.Duration {
	switch ci {
	case pb.CandleInterval_CANDLE_INTERVAL_1_MIN:
		return time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_2_MIN:
		return 2 * time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_3_MIN:
		return 3 * time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_5_MIN:
		return 5 * time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_10_MIN:
		return 10 * time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_15_MIN:
		return 15 * time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_30_MIN:
		return 30 * time.Minute
	case pb.CandleInterval_CANDLE_INTERVAL_HOUR:
		return time.Hour
	case pb.CandleInterval_CANDLE_INTERVAL_2_HOUR:
		return 2 * time.Hour
	case pb.CandleInterval_CANDLE_INTERVAL_4_HOUR:
		return 4 * time.Hour
	case pb.CandleInterval_CANDLE_INTERVAL_DAY:
		return 24 * time.Hour
	case pb.CandleInterval_CANDLE_INTERVAL_WEEK:
		return 7 * 24 * time.Hour
	case pb.CandleInterval_CANDLE_INTERVAL_MONTH:
		return 30 * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

func (r *Runner) fetchHistoricCandles(
	ctx context.Context,
	instrumentUID string,
	from, to time.Time,
	interval pb.CandleInterval,
) ([]marketdata.Candle, error) {
	if r.mdSvc == nil {
		return nil, nil
	}
	step := time.Duration(tinvestMaxCandlesPerReq) * CandleIntervalDuration(interval)
	if step <= 0 {
		step = 24 * time.Hour
	}
	var all []marketdata.Candle
	cursor := from.UTC()
	end := to.UTC()
	for cursor.Before(end) {
		chunkEnd := cursor.Add(step)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		part, err := r.mdSvc.GetCandles(ctx, instrumentUID, cursor, chunkEnd, interval)
		if err != nil {
			return all, err
		}
		all = append(all, part...)
		if chunkEnd.Equal(end) || len(part) == 0 {
			break
		}
		last := part[len(part)-1].Time.UTC()
		cursor = last.Add(CandleIntervalDuration(interval))
	}
	return dedupeMarketCandles(all), nil
}

func dedupeMarketCandles(candles []marketdata.Candle) []marketdata.Candle {
	if len(candles) == 0 {
		return nil
	}
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time.Before(candles[j].Time)
	})
	out := make([]marketdata.Candle, 0, len(candles))
	var last time.Time
	for _, c := range candles {
		t := c.Time.UTC()
		if !last.IsZero() && !t.After(last) {
			out[len(out)-1] = c
			continue
		}
		out = append(out, c)
		last = t
	}
	return out
}

func candlesToBars(candles []marketdata.Candle) []AITraderCandleBar {
	out := make([]AITraderCandleBar, 0, len(candles))
	for _, c := range candles {
		out = append(out, marketCandleToBar(c))
	}
	return out
}

func trimChartBars(bars []AITraderCandleBar, max int) []AITraderCandleBar {
	if max <= 0 || len(bars) <= max {
		return bars
	}
	return bars[len(bars)-max:]
}

func (st *aiTraderContextState) setChartBars1m(bars []AITraderCandleBar) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.chartBars = trimChartBars(bars, aiTraderMaxChartBars1m)
}

func (st *aiTraderContextState) setChartBars5m(bars []AITraderCandleBar) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.chartBars5m = trimChartBars(bars, aiTraderMaxChartBars5m)
}
