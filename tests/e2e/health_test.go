package e2e

import (
	"net/http"
	"net/url"
	"testing"
)

func TestHealth(t *testing.T) {
	resp, body := doRaw(t, http.MethodGet, "/health", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("health: expected 200, got %d: %s", resp.StatusCode, body)
	}
	t.Logf("Health: %s", body)
}

func TestSwaggerUI(t *testing.T) {
	resp, _ := doRaw(t, http.MethodGet, "/swagger/index.html", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("swagger: expected 200, got %d", resp.StatusCode)
	}
	t.Log("Swagger UI accessible")
}

func TestListAccounts(t *testing.T) {
	r := doGet(t, "/api/v1/accounts", nil)
	accs := unmarshal[[]Account](t, r)
	if len(accs) == 0 {
		t.Fatal("no accounts returned")
	}
	for _, a := range accs {
		t.Logf("Account: id=%s type=%s name=%s status=%s access=%s", a.ID, a.Type, a.Name, a.Status, a.AccessLevel)
	}
}

func TestPortfolio(t *testing.T) {
	q := url.Values{"account_id": {accountID}}
	r := doGet(t, "/api/v1/portfolio", q)
	p := unmarshal[PortfolioInfo](t, r)
	t.Logf("Portfolio: shares=%.2f, positions=%d", p.TotalAmountShares, len(p.Positions))
}

func TestPositions(t *testing.T) {
	q := url.Values{"account_id": {accountID}}
	r := doGet(t, "/api/v1/positions", q)
	positions := unmarshal[[]Position](t, r)
	t.Logf("Positions: %d", len(positions))
	for _, p := range positions {
		t.Logf("  uid=%s type=%s qty=%.2f avg=%.4f", p.InstrumentUID, p.InstrumentType, p.Quantity, p.AveragePrice)
	}
}

func TestWithdrawLimits(t *testing.T) {
	q := url.Values{"account_id": {accountID}}
	r := doGet(t, "/api/v1/limits", q)
	limits := unmarshal[[]WithdrawLimit](t, r)
	t.Logf("Withdraw limits: %d currencies", len(limits))
	for _, l := range limits {
		t.Logf("  %s: withdraw=%.2f", l.Currency, l.WithdrawAmount)
	}
}

func TestOperations(t *testing.T) {
	q := url.Values{"account_id": {accountID}}
	r := doGet(t, "/api/v1/operations", q)
	page := unmarshal[OperationsPage](t, r)
	t.Logf("Operations: %d, has_next=%v", len(page.Operations), page.HasNext)
}

func TestMargin(t *testing.T) {
	r := doGet(t, "/api/v1/margin/"+accountID, nil)
	m := unmarshal[MarginInfo](t, r)
	t.Logf("Margin: liquid=%.2f starting=%.2f minimal=%.2f sufficiency=%.4f",
		m.LiquidPortfolio, m.StartingMargin, m.MinimalMargin, m.FundsSufficiencyLevel)
}
