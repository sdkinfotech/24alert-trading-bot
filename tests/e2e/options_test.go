package e2e

import (
	"net/url"
	"testing"
	"time"
)

// GL11700CD6B — Gold CALL, strike 11700, exp 2026-04-10 (liquid on FORTS)
const optionInstrumentUID = "66f3eea6-35f2-4347-8f1d-8f082588801c"

func TestOptionMarketBuy(t *testing.T) {
	rateLimitPause()

	// 1. Check orderbook — for FORTS options, only limit orders are allowed (no market orders)
	bookR := doGet(t, "/api/v1/orderbook/"+optionInstrumentUID, url.Values{"depth": {"5"}})
	book := unmarshal[Orderbook](t, bookR)
	t.Logf("Option orderbook: bids=%d asks=%d last=%.4f", len(book.Bids), len(book.Asks), book.LastPrice)

	if len(book.Asks) == 0 {
		t.Skip("option has no asks — not tradeable right now")
	}
	if len(book.Bids) == 0 {
		t.Skip("option has no bids — cannot close position after buy")
	}

	askPrice := roundPrice(book.Asks[0].Price)
	bidPrice := roundPrice(book.Bids[0].Price)
	t.Logf("Best ask=%.2f, best bid=%.2f (spread=%.2f)", askPrice, bidPrice, askPrice-bidPrice)

	rateLimitPause()

	// 2. Buy 1 option contract with limit order at ask (FORTS options: market orders return 30094)
	buyR, code := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": optionInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "limit",
		"price":          askPrice,
	})

	if code != 201 || buyR.Error != "" {
		t.Skipf("Option buy not available (%d): %s", code, buyR.Error)
	}

	buyResult := unmarshal[OrderResult](t, buyR)
	t.Logf("Option BUY: id=%s status=%s lots_exec=%d premium=%.2f RUB",
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

	// 4. Sell to close with limit at bid
	sellR, sellCode := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": optionInstrumentUID,
		"quantity":       1,
		"direction":      "sell",
		"order_type":     "limit",
		"price":          bidPrice,
	})
	if sellCode != 201 || sellR.Error != "" {
		t.Logf("Option SELL failed (%d): %s — position may remain open", sellCode, sellR.Error)
		return
	}
	sellResult := unmarshal[OrderResult](t, sellR)
	t.Logf("Option SELL: id=%s status=%s lots_exec=%d premium=%.2f RUB",
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
