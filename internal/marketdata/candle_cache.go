package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// CandleCache is an optional cache layer for historical candles.
type CandleCache interface {
	Get(ctx context.Context, uid string, from, to time.Time, interval pb.CandleInterval) ([]Candle, error)
	Put(ctx context.Context, uid string, interval pb.CandleInterval, candles []Candle) error
}

// NoopCandleCache is a no-op fallback when Redis is unavailable.
type NoopCandleCache struct{}

func (NoopCandleCache) Get(context.Context, string, time.Time, time.Time, pb.CandleInterval) ([]Candle, error) {
	return nil, nil
}

func (NoopCandleCache) Put(context.Context, string, pb.CandleInterval, []Candle) error {
	return nil
}

// RedisCandleCache stores candles in Redis Sorted Sets keyed by
// "candles:{uid}:{interval}" with score = unix timestamp.
type RedisCandleCache struct {
	rdb *redis.Client
}

func NewRedisCandleCache(rdb *redis.Client) *RedisCandleCache {
	return &RedisCandleCache{rdb: rdb}
}

func candleKey(uid string, interval pb.CandleInterval) string {
	return fmt.Sprintf("candles:%s:%s", uid, interval.String())
}

type cachedCandle struct {
	Open       float64   `json:"o"`
	High       float64   `json:"h"`
	Low        float64   `json:"l"`
	Close      float64   `json:"c"`
	Volume     int64     `json:"v"`
	Time       time.Time `json:"t"`
	IsComplete bool      `json:"ic"`
}

func (r *RedisCandleCache) Get(ctx context.Context, uid string, from, to time.Time, interval pb.CandleInterval) ([]Candle, error) {
	key := candleKey(uid, interval)
	results, err := r.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{ //nolint:staticcheck // TODO: migrate to ZRangeArgs
		Min: fmt.Sprintf("%d", from.Unix()),
		Max: fmt.Sprintf("%d", to.Unix()),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ZRANGEBYSCORE: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	candles := make([]Candle, 0, len(results))
	for _, raw := range results {
		var cc cachedCandle
		if err := json.Unmarshal([]byte(raw), &cc); err != nil {
			continue
		}
		candles = append(candles, Candle(cc))
	}
	return candles, nil
}

func (r *RedisCandleCache) Put(ctx context.Context, uid string, interval pb.CandleInterval, candles []Candle) error {
	if len(candles) == 0 {
		return nil
	}
	key := candleKey(uid, interval)
	members := make([]redis.Z, 0, len(candles))
	for _, c := range candles {
		data, err := json.Marshal(cachedCandle(c))
		if err != nil {
			continue
		}
		members = append(members, redis.Z{
			Score:  float64(c.Time.Unix()),
			Member: string(data),
		})
	}

	pipe := r.rdb.Pipeline()
	pipe.ZAdd(ctx, key, members...)
	// Trim to max 2400 entries (T-Invest per-interval limit).
	pipe.ZRemRangeByRank(ctx, key, 0, -2401)
	pipe.Expire(ctx, key, intervalTTL(interval))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline: %w", err)
	}
	return nil
}

func intervalTTL(interval pb.CandleInterval) time.Duration {
	switch interval {
	case pb.CandleInterval_CANDLE_INTERVAL_1_MIN,
		pb.CandleInterval_CANDLE_INTERVAL_2_MIN,
		pb.CandleInterval_CANDLE_INTERVAL_3_MIN,
		pb.CandleInterval_CANDLE_INTERVAL_5_MIN:
		return 7 * 24 * time.Hour
	case pb.CandleInterval_CANDLE_INTERVAL_10_MIN,
		pb.CandleInterval_CANDLE_INTERVAL_15_MIN,
		pb.CandleInterval_CANDLE_INTERVAL_30_MIN:
		return 21 * 24 * time.Hour
	default:
		return 90 * 24 * time.Hour
	}
}
