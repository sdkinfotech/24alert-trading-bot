package e2e

import (
	"net/url"
	"testing"
	"time"
)

func TestCandles_1Hour(t *testing.T) {
	rateLimitPause()
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {testInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"1h"},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Candles 1h (24h): %d", len(candles))
	if len(candles) > 0 {
		c := candles[len(candles)-1]
		t.Logf("  Last: O=%.4f H=%.4f L=%.4f C=%.4f V=%d complete=%v",
			c.Open, c.High, c.Low, c.Close, c.Volume, c.IsComplete)
	}
}

func TestCandles_1Min(t *testing.T) {
	rateLimitPause()
	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour)
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {testInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"1min"},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Candles 1min (1h): %d", len(candles))
}

func TestCandles_5Min(t *testing.T) {
	rateLimitPause()
	now := time.Now().UTC()
	from := now.Add(-6 * time.Hour)
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {testInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"5min"},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Candles 5min (6h): %d", len(candles))
}

func TestCandles_1Day(t *testing.T) {
	rateLimitPause()
	now := time.Now().UTC()
	from := now.Add(-30 * 24 * time.Hour)
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {testInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"day"},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Candles day (30d): %d", len(candles))
}

func TestCandles_DefaultParams(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {testInstrumentUID},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Candles default (no from/to, 1h interval): %d", len(candles))
}

func TestOrderbook_Depth5(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/orderbook/"+testInstrumentUID, url.Values{"depth": {"5"}})
	book := unmarshal[Orderbook](t, r)
	t.Logf("Orderbook depth=5: bids=%d asks=%d last=%.4f", len(book.Bids), len(book.Asks), book.LastPrice)
}

func TestOrderbook_Depth20(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/orderbook/"+testInstrumentUID, url.Values{"depth": {"20"}})
	book := unmarshal[Orderbook](t, r)
	t.Logf("Orderbook depth=20: bids=%d asks=%d", len(book.Bids), len(book.Asks))
	if len(book.Bids) > 0 {
		t.Logf("  Best bid: %.4f x %d", book.Bids[0].Price, book.Bids[0].Quantity)
	}
	if len(book.Asks) > 0 {
		t.Logf("  Best ask: %.4f x %d", book.Asks[0].Price, book.Asks[0].Quantity)
	}
}

func TestOrderbook_DefaultDepth(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/orderbook/"+testInstrumentUID, nil)
	book := unmarshal[Orderbook](t, r)
	t.Logf("Orderbook default: depth=%d bids=%d asks=%d", book.Depth, len(book.Bids), len(book.Asks))
}

func TestLastPrice(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}})
	prices := unmarshal[[]LastPrice](t, r)
	if len(prices) == 0 {
		t.Fatal("no prices returned")
	}
	t.Logf("Last price: uid=%s price=%.4f", prices[0].InstrumentUID, prices[0].Price)
	if prices[0].Price <= 0 {
		t.Error("price should be > 0")
	}
}

func TestTradingStatus(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/trading-status/"+testInstrumentUID, nil)
	ts := unmarshal[TradingStatus](t, r)
	t.Logf("Trading status: uid=%s status=%s limit=%v market=%v api=%v",
		ts.InstrumentUID, ts.TradingStatus,
		ts.LimitOrderAvailable, ts.MarketOrderAvailable, ts.APITradeAvailable)
}
