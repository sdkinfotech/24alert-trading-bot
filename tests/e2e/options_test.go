package e2e

import (
	"net/url"
	"testing"
	"time"
)

// NM100CE6 NLMK CALL option, strike 100 RUB
const optionInstrumentUID = "045d317b-0f2c-4dad-af8c-16b4195b4c32"

func TestOptionMarketBuy(t *testing.T) {
	rateLimitPause()

	// 1. Get option price
	priceR := doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {optionInstrumentUID}})
	prices := unmarshal[[]LastPrice](t, priceR)
	if len(prices) == 0 {
		t.Skip("no price data for option")
	}
	optionPrice := prices[0].Price
	t.Logf("Option price: %.4f RUB", optionPrice)

	if optionPrice == 0 {
		t.Skip("option not trading (price=0)")
	}

	rateLimitPause()

	// 2. Buy 1 option contract
	buyR, code := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": optionInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "market",
	})

	if code != 201 || buyR.Error != "" {
		t.Skipf("Option trading not available (%d): %s", code, buyR.Error)
	}

	buyResult := unmarshal[OrderResult](t, buyR)
	t.Logf("Option BUY: id=%s status=%s lots_exec=%d premium=%.4f RUB",
		buyResult.OrderID, buyResult.ExecutionStatus, buyResult.LotsExecuted, buyResult.TotalPrice)

	if buyResult.OrderID == "" {
		t.Fatal("no order_id for option buy")
	}

	rateLimitPause()

	// 3. Check position
	positionsR := doGet(t, "/api/v1/positions", url.Values{"account_id": {accountID}})
	positions := unmarshal[[]Position](t, positionsR)
	found := false
	for _, p := range positions {
		if p.InstrumentUID == optionInstrumentUID {
			t.Logf("Option position: qty=%.0f avg=%.4f type=%s", p.Quantity, p.AveragePrice, p.InstrumentType)
			found = true
		}
	}
	if !found {
		t.Log("Option position not yet visible")
	}

	rateLimitPause()

	// 4. Get option orderbook
	bookR := doGet(t, "/api/v1/orderbook/"+optionInstrumentUID, url.Values{"depth": {"5"}})
	book := unmarshal[Orderbook](t, bookR)
	t.Logf("Option orderbook: bids=%d asks=%d last=%.4f", len(book.Bids), len(book.Asks), book.LastPrice)

	rateLimitPause()

	// 5. Sell to close
	sellR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": optionInstrumentUID,
		"quantity":       1,
		"direction":      "sell",
		"order_type":     "market",
	})
	sellResult := unmarshal[OrderResult](t, sellR)
	t.Logf("Option SELL: id=%s status=%s lots_exec=%d premium=%.4f RUB",
		sellResult.OrderID, sellResult.ExecutionStatus, sellResult.LotsExecuted, sellResult.TotalPrice)
}

func TestOptionCandles(t *testing.T) {
	rateLimitPause()
	now := CurrentTime()
	from := now.Add(-24 * time.Hour)
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {optionInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"1h"},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Option candles 1h (24h): %d", len(candles))
}

func TestOptionTradingStatus(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/trading-status/"+optionInstrumentUID, nil)
	ts := unmarshal[TradingStatus](t, r)
	t.Logf("Option trading status: %s limit=%v market=%v api=%v",
		ts.TradingStatus, ts.LimitOrderAvailable, ts.MarketOrderAvailable, ts.APITradeAvailable)
}
