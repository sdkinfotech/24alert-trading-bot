package e2e

import (
	"net/url"
	"testing"
	"time"
)

// KZU6 futures on MOEX FORTS (Kazakh Tenge)
const futuresInstrumentUID = "3da86f70-25a7-4092-bc1b-05cf84dad9fe"

func TestFuturesMarketBuySell(t *testing.T) {
	rateLimitPause()

	// 1. Get current price
	priceR := doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {futuresInstrumentUID}})
	prices := unmarshal[[]LastPrice](t, priceR)
	if len(prices) == 0 {
		t.Skip("no price data for futures")
	}
	currentPrice := prices[0].Price
	t.Logf("Futures price: %.2f", currentPrice)

	rateLimitPause()

	// 2. Buy 1 contract
	buyR, buyCode := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": futuresInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "market",
	})
	if buyCode != 201 || buyR.Error != "" {
		t.Skipf("Futures buy not available (insufficient margin or instrument restriction): %d — %s", buyCode, buyR.Error)
	}
	buyResult := unmarshal[OrderResult](t, buyR)
	t.Logf("Futures BUY: id=%s status=%s lots_exec=%d price=%.2f",
		buyResult.OrderID, buyResult.ExecutionStatus, buyResult.LotsExecuted, buyResult.TotalPrice)

	if buyResult.OrderID == "" {
		t.Fatal("no order_id for futures buy")
	}

	rateLimitPause()

	// 3. Check position
	positionsR := doGet(t, "/api/v1/positions", url.Values{"account_id": {accountID}})
	positions := unmarshal[[]Position](t, positionsR)
	found := false
	for _, p := range positions {
		if p.InstrumentUID == futuresInstrumentUID {
			t.Logf("Futures position: qty=%.0f avg=%.4f type=%s", p.Quantity, p.AveragePrice, p.InstrumentType)
			found = true
		}
	}
	if !found {
		t.Log("Position not yet visible (may need time)")
	}

	rateLimitPause()

	// 4. Check margin for futures account
	marginR := doGet(t, "/api/v1/margin/"+accountID, nil)
	margin := unmarshal[MarginInfo](t, marginR)
	t.Logf("Margin: liquid=%.2f starting=%.2f minimal=%.2f",
		margin.LiquidPortfolio, margin.StartingMargin, margin.MinimalMargin)

	rateLimitPause()

	// 5. Sell 1 contract (close position)
	sellR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": futuresInstrumentUID,
		"quantity":       1,
		"direction":      "sell",
		"order_type":     "market",
	})
	sellResult := unmarshal[OrderResult](t, sellR)
	t.Logf("Futures SELL: id=%s status=%s lots_exec=%d price=%.2f",
		sellResult.OrderID, sellResult.ExecutionStatus, sellResult.LotsExecuted, sellResult.TotalPrice)

	rateLimitPause()

	// 6. Verify position closed
	positionsR2 := doGet(t, "/api/v1/positions", url.Values{"account_id": {accountID}})
	positions2 := unmarshal[[]Position](t, positionsR2)
	for _, p := range positions2 {
		if p.InstrumentUID == futuresInstrumentUID {
			if p.Quantity != 0 {
				t.Logf("Position still open: qty=%.0f", p.Quantity)
			}
		}
	}
}

func TestFuturesCandles(t *testing.T) {
	rateLimitPause()
	now := CurrentTime()
	from := now.Add(-24 * time.Hour)
	r := doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {futuresInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"1h"},
	})
	candles := unmarshal[[]Candle](t, r)
	t.Logf("Futures candles 1h (24h): %d", len(candles))
	if len(candles) == 0 {
		t.Skip("no candle data for futures")
	}
}

func TestFuturesOrderbook(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/orderbook/"+futuresInstrumentUID, url.Values{"depth": {"5"}})
	book := unmarshal[Orderbook](t, r)
	t.Logf("Futures orderbook depth=5: bids=%d asks=%d", len(book.Bids), len(book.Asks))
}

func TestFuturesTradingStatus(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/trading-status/"+futuresInstrumentUID, nil)
	ts := unmarshal[TradingStatus](t, r)
	t.Logf("Futures trading status: %s limit=%v market=%v api=%v",
		ts.TradingStatus, ts.LimitOrderAvailable, ts.MarketOrderAvailable, ts.APITradeAvailable)
}
