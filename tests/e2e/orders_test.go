package e2e

import (
	"net/url"
	"testing"
	"time"
)

// Well-known cheap instruments for testing.
// Sberbank preferred (TQBR) — liquid, cheap per lot.
const (
	testInstrumentUID = "e6123145-9665-43e0-8413-cd61b8aa9b13" // SBER
	testTicker        = "SBER"
)

func TestMarketOrder_BuySell(t *testing.T) {
	rateLimitPause()

	// 1. Buy 1 lot market
	order, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "market",
	})
	result := unmarshal[OrderResult](t, order)
	t.Logf("Market BUY: id=%s status=%s lots_exec=%d price=%.2f",
		result.OrderID, result.ExecutionStatus, result.LotsExecuted, result.TotalPrice)

	if result.OrderID == "" {
		t.Fatal("no order_id returned")
	}

	rateLimitPause()

	// 2. Check order state
	q := url.Values{"account_id": {accountID}}
	stateR := doGet(t, "/api/v1/orders/"+result.OrderID, q)
	state := unmarshal[OrderState](t, stateR)
	t.Logf("Order state: status=%s lots_req=%d lots_exec=%d",
		state.ExecutionStatus, state.LotsRequested, state.LotsExecuted)

	rateLimitPause()

	// 3. Check position appeared
	posR := doGet(t, "/api/v1/positions", url.Values{"account_id": {accountID}})
	positions := unmarshal[[]Position](t, posR)
	t.Logf("Positions after buy: %d", len(positions))

	rateLimitPause()

	// 4. Sell 1 lot market (close position)
	sellOrder, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "sell",
		"order_type":     "market",
	})
	sellResult := unmarshal[OrderResult](t, sellOrder)
	t.Logf("Market SELL: id=%s status=%s lots_exec=%d price=%.2f",
		sellResult.OrderID, sellResult.ExecutionStatus, sellResult.LotsExecuted, sellResult.TotalPrice)

	rateLimitPause()

	// 5. Check operations (trade history)
	opsR := doGet(t, "/api/v1/operations", url.Values{
		"account_id":     {accountID},
		"instrument_uid": {testInstrumentUID},
	})
	ops := unmarshal[OperationsPage](t, opsR)
	t.Logf("Operations for instrument: %d", len(ops.Operations))
}

func TestLimitOrder_PlaceAndCancel(t *testing.T) {
	rateLimitPause()

	// 1. Get current price
	priceR := doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}})
	prices := unmarshal[[]LastPrice](t, priceR)
	if len(prices) == 0 {
		t.Fatal("no price returned")
	}
	currentPrice := prices[0].Price
	t.Logf("Current price: %.4f", currentPrice)

	// Limit price 5% below market — should not execute
	limitPrice := roundPrice(currentPrice * 0.95)

	rateLimitPause()

	// 2. Place limit order
	orderR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "limit",
		"price":          limitPrice,
	})
	result := unmarshal[OrderResult](t, orderR)
	t.Logf("Limit BUY: id=%s status=%s price=%.4f", result.OrderID, result.ExecutionStatus, limitPrice)

	if result.OrderID == "" {
		t.Fatal("no order_id for limit order")
	}

	rateLimitPause()

	// 3. Check state
	q := url.Values{"account_id": {accountID}}
	stateR := doGet(t, "/api/v1/orders/"+result.OrderID, q)
	state := unmarshal[OrderState](t, stateR)
	t.Logf("Limit order state: %s", state.ExecutionStatus)

	rateLimitPause()

	// 4. List orders
	listR := doGet(t, "/api/v1/orders", url.Values{"account_id": {accountID}})
	orders := unmarshal[[]OrderSummary](t, listR)
	t.Logf("Active orders: %d", len(orders))

	rateLimitPause()

	// 5. Cancel
	cancelR := doDelete(t, "/api/v1/orders/"+result.OrderID, q)
	if cancelR.Error != "" {
		t.Fatalf("cancel failed: %s", cancelR.Error)
	}
	t.Logf("Order cancelled")

	rateLimitPause()

	// 6. Verify cancelled
	stateR2 := doGet(t, "/api/v1/orders/"+result.OrderID, q)
	state2 := unmarshal[OrderState](t, stateR2)
	t.Logf("After cancel: status=%s", state2.ExecutionStatus)
}

func TestReplaceOrder(t *testing.T) {
	rateLimitPause()

	priceR := doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}})
	prices := unmarshal[[]LastPrice](t, priceR)
	if len(prices) == 0 {
		t.Skip("no price data")
	}
	currentPrice := prices[0].Price

	rateLimitPause()

	// Place limit order far from market
	orderR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "limit",
		"price":          roundPrice(currentPrice * 0.95),
	})
	result := unmarshal[OrderResult](t, orderR)
	t.Logf("Initial limit order: id=%s", result.OrderID)
	if result.OrderID == "" {
		t.Fatal("no order_id")
	}

	rateLimitPause()

	// Replace with new price and quantity
	q := url.Values{"account_id": {accountID}}
	replaceR := doPut(t, "/api/v1/orders/"+result.OrderID, q, map[string]any{
		"quantity": 2,
		"price":    roundPrice(currentPrice * 0.97),
	})
	replaced := unmarshal[OrderResult](t, replaceR)
	t.Logf("Replaced order: id=%s lots_req=%d", replaced.OrderID, replaced.LotsRequested)

	rateLimitPause()

	// Cleanup: cancel
	doDelete(t, "/api/v1/orders/"+replaced.OrderID, q)
	t.Log("Replaced order cancelled (cleanup)")
}

func TestBestpriceOrder(t *testing.T) {
	rateLimitPause()

	orderR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "bestprice",
	})
	result := unmarshal[OrderResult](t, orderR)
	t.Logf("Bestprice BUY: id=%s status=%s lots_exec=%d",
		result.OrderID, result.ExecutionStatus, result.LotsExecuted)

	if result.OrderID == "" {
		t.Fatal("no order_id")
	}

	rateLimitPause()

	// Cleanup: sell
	sellR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "sell",
		"order_type":     "market",
	})
	sellResult := unmarshal[OrderResult](t, sellR)
	t.Logf("Cleanup sell: id=%s status=%s", sellResult.OrderID, sellResult.ExecutionStatus)
}

func TestListOrders(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/orders", url.Values{"account_id": {accountID}})
	orders := unmarshal[[]OrderSummary](t, r)
	t.Logf("Orders: %d", len(orders))
	for _, o := range orders {
		t.Logf("  id=%s dir=%s type=%s lots=%d price=%.4f status=%s",
			o.OrderID, o.Direction, o.OrderType, o.Lots, o.Price, o.Status)
	}
}

func TestFullTradeRoundtrip(t *testing.T) {
	rateLimitPause()

	// 1. Portfolio before
	portBefore := unmarshal[PortfolioInfo](t,
		doGet(t, "/api/v1/portfolio", url.Values{"account_id": {accountID}}))
	t.Logf("Portfolio before: shares=%.2f", portBefore.TotalAmountShares)

	rateLimitPause()

	// 2. Get price
	prices := unmarshal[[]LastPrice](t,
		doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}}))
	t.Logf("Price: %.4f", prices[0].Price)

	rateLimitPause()

	// 3. Get candles
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	candles := unmarshal[[]Candle](t, doGet(t, "/api/v1/candles", url.Values{
		"instrument_uid": {testInstrumentUID},
		"from":           {from.Format(time.RFC3339)},
		"to":             {now.Format(time.RFC3339)},
		"interval":       {"1h"},
	}))
	t.Logf("Candles 1h: %d", len(candles))

	rateLimitPause()

	// 4. Get orderbook
	book := unmarshal[Orderbook](t,
		doGet(t, "/api/v1/orderbook/"+testInstrumentUID, url.Values{"depth": {"10"}}))
	t.Logf("Orderbook: bids=%d asks=%d last=%.4f", len(book.Bids), len(book.Asks), book.LastPrice)

	rateLimitPause()

	// 5. Buy
	buyR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "market",
	})
	buy := unmarshal[OrderResult](t, buyR)
	t.Logf("BUY: id=%s status=%s", buy.OrderID, buy.ExecutionStatus)

	rateLimitPause()

	// 6. Check position
	positions := unmarshal[[]Position](t,
		doGet(t, "/api/v1/positions", url.Values{"account_id": {accountID}}))
	found := false
	for _, p := range positions {
		if p.InstrumentUID == testInstrumentUID {
			t.Logf("Position found: qty=%.0f avg=%.4f", p.Quantity, p.AveragePrice)
			found = true
		}
	}
	if !found {
		t.Log("Position not visible yet (may need time)")
	}

	rateLimitPause()

	// 7. Check operations
	ops := unmarshal[OperationsPage](t, doGet(t, "/api/v1/operations", url.Values{
		"account_id":     {accountID},
		"instrument_uid": {testInstrumentUID},
	}))
	t.Logf("Operations: %d", len(ops.Operations))

	rateLimitPause()

	// 8. Sell (close)
	sellR, _ := doPost(t, "/api/v1/orders", map[string]any{
		"account_id":     accountID,
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "sell",
		"order_type":     "market",
	})
	sell := unmarshal[OrderResult](t, sellR)
	t.Logf("SELL: id=%s status=%s", sell.OrderID, sell.ExecutionStatus)

	rateLimitPause()

	// 9. Portfolio after
	portAfter := unmarshal[PortfolioInfo](t,
		doGet(t, "/api/v1/portfolio", url.Values{"account_id": {accountID}}))
	t.Logf("Portfolio after: shares=%.2f", portAfter.TotalAmountShares)
}
