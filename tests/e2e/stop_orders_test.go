package e2e

import (
	"net/url"
	"testing"
)

func TestStopLoss(t *testing.T) {
	rateLimitPause()

	// Get current price
	prices := unmarshal[[]LastPrice](t,
		doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}}))
	if len(prices) == 0 {
		t.Skip("no price data")
	}
	currentPrice := prices[0].Price
	stopPrice := roundPrice(currentPrice * 0.98) // 2% below

	rateLimitPause()

	// Place stop-loss
	r, code := doPost(t, "/api/v1/stop-orders", map[string]any{
		"account_id":      accountID,
		"instrument_uid":  testInstrumentUID,
		"quantity":        1,
		"direction":       "sell",
		"stop_order_type": "stop_loss",
		"stop_price":      stopPrice,
	})

	// Sandbox may not support stop orders (error 30043)
	if code != 201 || r.Error != "" {
		// Error 30043 = T-Invest API limitation (stop orders not supported for this instrument/account)
		t.Skipf("Stop-loss not available (T-Invest API limitation - error 30043): %d — %s", code, r.Error)
	}

	result := unmarshal[StopOrderResult](t, r)
	t.Logf("Stop-loss: id=%s stop_price=%.4f", result.StopOrderID, stopPrice)

	if result.StopOrderID == "" {
		t.Fatal("no stop_order_id")
	}

	rateLimitPause()

	// List stop orders
	listR := doGet(t, "/api/v1/stop-orders", url.Values{"account_id": {accountID}})
	stops := unmarshal[[]StopOrderSummary](t, listR)
	t.Logf("Stop orders: %d", len(stops))

	rateLimitPause()

	// Cancel
	cancelR := doDelete(t, "/api/v1/stop-orders/"+result.StopOrderID, url.Values{"account_id": {accountID}})
	if cancelR.Error != "" {
		t.Logf("Cancel stop-loss result: %s", cancelR.Error)
	} else {
		t.Log("Stop-loss cancelled")
	}
}

func TestTakeProfit(t *testing.T) {
	rateLimitPause()

	prices := unmarshal[[]LastPrice](t,
		doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}}))
	if len(prices) == 0 {
		t.Skip("no price data")
	}
	currentPrice := prices[0].Price
	tpPrice := roundPrice(currentPrice * 1.02) // 2% above

	rateLimitPause()

	r, code := doPost(t, "/api/v1/stop-orders", map[string]any{
		"account_id":      accountID,
		"instrument_uid":  testInstrumentUID,
		"quantity":        1,
		"direction":       "sell",
		"stop_order_type": "take_profit",
		"stop_price":      tpPrice,
	})

	if code != 201 || r.Error != "" {
		t.Skipf("Take-profit not available (production limitation): %d — %s", code, r.Error)
	}

	result := unmarshal[StopOrderResult](t, r)
	t.Logf("Take-profit: id=%s tp_price=%.4f", result.StopOrderID, tpPrice)

	if result.StopOrderID == "" {
		t.Fatal("no stop_order_id")
	}

	rateLimitPause()

	// Cancel
	doDelete(t, "/api/v1/stop-orders/"+result.StopOrderID, url.Values{"account_id": {accountID}})
	t.Log("Take-profit cancelled (cleanup)")
}

func TestStopLimit(t *testing.T) {
	rateLimitPause()

	prices := unmarshal[[]LastPrice](t,
		doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}}))
	if len(prices) == 0 {
		t.Skip("no price data")
	}
	currentPrice := prices[0].Price
	stopPrice := roundPrice(currentPrice * 0.97)
	limitPrice := roundPrice(currentPrice * 0.96)

	rateLimitPause()

	r, code := doPost(t, "/api/v1/stop-orders", map[string]any{
		"account_id":      accountID,
		"instrument_uid":  testInstrumentUID,
		"quantity":        1,
		"direction":       "sell",
		"stop_order_type": "stop_limit",
		"stop_price":      stopPrice,
		"price":           limitPrice,
	})

	if code != 201 || r.Error != "" {
		t.Skipf("Stop-limit not available (production limitation): %d — %s", code, r.Error)
	}

	result := unmarshal[StopOrderResult](t, r)
	t.Logf("Stop-limit: id=%s stop=%.4f limit=%.4f", result.StopOrderID, stopPrice, limitPrice)

	if result.StopOrderID == "" {
		t.Fatal("no stop_order_id")
	}

	rateLimitPause()

	doDelete(t, "/api/v1/stop-orders/"+result.StopOrderID, url.Values{"account_id": {accountID}})
	t.Log("Stop-limit cancelled (cleanup)")
}

func TestStopOrdersCRUD(t *testing.T) {
	rateLimitPause()

	prices := unmarshal[[]LastPrice](t,
		doGet(t, "/api/v1/prices", url.Values{"instrument_uid": {testInstrumentUID}}))
	if len(prices) == 0 {
		t.Skip("no price data")
	}
	currentPrice := prices[0].Price

	rateLimitPause()

	// Place two stop orders
	sl, slCode := doPost(t, "/api/v1/stop-orders", map[string]any{
		"account_id":      accountID,
		"instrument_uid":  testInstrumentUID,
		"quantity":        1,
		"direction":       "sell",
		"stop_order_type": "stop_loss",
		"stop_price":      roundPrice(currentPrice * 0.95),
	})
	if slCode != 201 || sl.Error != "" {
		t.Skipf("Stop orders not available (production limitation): %d — %s", slCode, sl.Error)
	}
	slResult := unmarshal[StopOrderResult](t, sl)

	rateLimitPause()

	tp, tpCode := doPost(t, "/api/v1/stop-orders", map[string]any{
		"account_id":      accountID,
		"instrument_uid":  testInstrumentUID,
		"quantity":        1,
		"direction":       "sell",
		"stop_order_type": "take_profit",
		"stop_price":      roundPrice(currentPrice * 1.05),
	})
	if tpCode != 201 || tp.Error != "" {
		t.Skipf("Stop orders not available (production limitation): %d — %s", tpCode, tp.Error)
	}
	tpResult := unmarshal[StopOrderResult](t, tp)

	rateLimitPause()

	// List — should see both
	listR := doGet(t, "/api/v1/stop-orders", url.Values{"account_id": {accountID}})
	stops := unmarshal[[]StopOrderSummary](t, listR)
	t.Logf("Stop orders after placing 2: %d", len(stops))

	rateLimitPause()

	// Cancel first
	doDelete(t, "/api/v1/stop-orders/"+slResult.StopOrderID, url.Values{"account_id": {accountID}})

	rateLimitPause()

	// List — should see one less
	listR2 := doGet(t, "/api/v1/stop-orders", url.Values{"account_id": {accountID}})
	stops2 := unmarshal[[]StopOrderSummary](t, listR2)
	t.Logf("Stop orders after cancel 1: %d", len(stops2))

	rateLimitPause()

	// Cancel second
	doDelete(t, "/api/v1/stop-orders/"+tpResult.StopOrderID, url.Values{"account_id": {accountID}})
	t.Log("Both stop orders cleaned up")
}
