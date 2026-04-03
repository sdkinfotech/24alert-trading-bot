package e2e

import (
	"net/http"
	"net/url"
	"testing"
)

func TestOrder_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r, code := doPost(t, "/api/v1/orders", map[string]any{
		"instrument_uid": testInstrumentUID,
		"quantity":       1,
		"direction":      "buy",
		"order_type":     "market",
	})
	if code == 200 || code == 201 {
		t.Fatalf("expected error, got %d", code)
	}
	t.Logf("Missing account_id: %d — %s", code, r.Error)
}

func TestOrder_MissingInstrumentUID(t *testing.T) {
	rateLimitPause()
	r, code := doPost(t, "/api/v1/orders", map[string]any{
		"account_id": accountID,
		"quantity":   1,
		"direction":  "buy",
		"order_type": "market",
	})
	if code == 200 || code == 201 {
		t.Fatalf("expected error, got %d", code)
	}
	t.Logf("Missing instrument_uid: %d — %s", code, r.Error)
}

func TestOrder_InvalidBody(t *testing.T) {
	rateLimitPause()
	resp, body := doRaw(t, http.MethodPost, "/api/v1/orders", nil)
	if resp.StatusCode == 201 {
		t.Fatalf("expected error for empty body, got 201")
	}
	t.Logf("Empty body: %d — %s", resp.StatusCode, body)
}

func TestListOrders_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/orders", nil)
	if r.Error == "" {
		t.Fatal("expected error for missing account_id")
	}
	t.Logf("Missing account_id on list: %s", r.Error)
}

func TestCancelOrder_NonExistent(t *testing.T) {
	rateLimitPause()
	r := doDelete(t, "/api/v1/orders/nonexistent-order-id", url.Values{"account_id": {accountID}})
	if r.Error == "" {
		t.Log("Cancel non-existent: no error (may be expected)")
	} else {
		t.Logf("Cancel non-existent: %s", r.Error)
	}
}

func TestStopOrder_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r, code := doPost(t, "/api/v1/stop-orders", map[string]any{
		"instrument_uid":  testInstrumentUID,
		"quantity":        1,
		"direction":       "sell",
		"stop_order_type": "stop_loss",
		"stop_price":      100.0,
	})
	if code == 200 || code == 201 {
		t.Fatalf("expected error, got %d", code)
	}
	t.Logf("Missing account_id on stop-order: %d — %s", code, r.Error)
}

func TestCandles_MissingInstrumentUID(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/candles", nil)
	if r.Error == "" {
		t.Fatal("expected error for missing instrument_uid")
	}
	t.Logf("Missing instrument_uid on candles: %s", r.Error)
}

func TestPositions_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/positions", nil)
	if r.Error == "" {
		t.Fatal("expected error for missing account_id")
	}
	t.Logf("Missing account_id on positions: %s", r.Error)
}

func TestPortfolio_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/portfolio", nil)
	if r.Error == "" {
		t.Fatal("expected error for missing account_id")
	}
	t.Logf("Missing account_id on portfolio: %s", r.Error)
}

func TestOperations_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/operations", nil)
	if r.Error == "" {
		t.Fatal("expected error for missing account_id")
	}
	t.Logf("Missing account_id on operations: %s", r.Error)
}

func TestLimits_MissingAccountID(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/limits", nil)
	if r.Error == "" {
		t.Fatal("expected error for missing account_id")
	}
	t.Logf("Missing account_id on limits: %s", r.Error)
}

func TestCancelStopOrder_NonExistent(t *testing.T) {
	rateLimitPause()
	r := doDelete(t, "/api/v1/stop-orders/nonexistent-stop-id", url.Values{"account_id": {accountID}})
	if r.Error == "" {
		t.Log("Cancel non-existent stop order: no error (may be expected)")
	} else {
		t.Logf("Cancel non-existent stop order: %s", r.Error)
	}
}
