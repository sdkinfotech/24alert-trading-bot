package e2e

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	baseURL   string
	accountID string
	client    *http.Client
)

func TestMain(m *testing.M) {
	// Default: local docker compose (gateway published on loopback :18080).
	// Remote / TLS: set API_BASE_URL (e.g. https://host:8080 — nginx may not expose all REST paths).
	baseURL = strings.TrimRight(envOr("API_BASE_URL", "http://127.0.0.1:18080"), "/")
	client = newE2EHTTPClient(baseURL)

	accs, err := getAccounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot get accounts (API_BASE_URL=%q): %v\n", baseURL, err)
		os.Exit(1)
	}
	if len(accs) == 0 {
		fmt.Fprintln(os.Stderr, "FATAL: no sandbox accounts found")
		os.Exit(1)
	}
	accountID = accs[0].ID
	fmt.Printf("Using account: %s (%s)\n", accountID, accs[0].Name)

	os.Exit(m.Run())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newE2EHTTPClient(base string) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if strings.HasPrefix(strings.ToLower(base), "https:") {
		tr.TLSClientConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, //nolint:gosec // e2e against hosts without public CA (e.g. IP TLS)
		}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: tr}
}

// --- HTTP helpers ---

type apiResp struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func doGet(t *testing.T, path string, query url.Values) apiResp {
	t.Helper()
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	resp, err := client.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	return readResp(t, resp)
}

func doPost(t *testing.T, path string, body any) (apiResp, int) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := client.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	return readResp(t, resp), resp.StatusCode
}

func doDelete(t *testing.T, path string, query url.Values) apiResp {
	t.Helper()
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, _ := http.NewRequest(http.MethodDelete, u, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer resp.Body.Close()
	return readResp(t, resp)
}

func doPut(t *testing.T, path string, query url.Values, body any) apiResp {
	t.Helper()
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	defer resp.Body.Close()
	return readResp(t, resp)
}

func doRaw(t *testing.T, method, path string, query url.Values) (*http.Response, []byte) {
	t.Helper()
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, _ := http.NewRequest(method, u, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func readResp(t *testing.T, resp *http.Response) apiResp {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var r apiResp
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("decode response (%d): %s — %v", resp.StatusCode, string(body), err)
	}
	return r
}

func unmarshal[T any](t *testing.T, r apiResp) T {
	t.Helper()
	if r.Error != "" {
		t.Fatalf("API error: %s", r.Error)
	}
	var v T
	if err := json.Unmarshal(r.Data, &v); err != nil {
		t.Fatalf("unmarshal data: %v (raw: %s)", err, string(r.Data))
	}
	return v
}

// --- Domain types ---

type Account struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	AccessLevel string `json:"access_level"`
}

type OrderResult struct {
	OrderID         string  `json:"order_id"`
	ExecutionStatus string  `json:"execution_status"`
	LotsRequested   int64   `json:"lots_requested"`
	LotsExecuted    int64   `json:"lots_executed"`
	TotalPrice      float64 `json:"total_price"`
	Direction       string  `json:"direction"`
	OrderType       string  `json:"order_type"`
	Message         string  `json:"message,omitempty"`
}

type OrderState struct {
	OrderID         string  `json:"order_id"`
	ExecutionStatus string  `json:"execution_status"`
	LotsRequested   int64   `json:"lots_requested"`
	LotsExecuted    int64   `json:"lots_executed"`
	TotalPrice      float64 `json:"total_price"`
	Direction       string  `json:"direction"`
	OrderType       string  `json:"order_type"`
	InstrumentUID   string  `json:"instrument_uid"`
	AccountID       string  `json:"account_id"`
}

type OrderSummary struct {
	OrderID       string  `json:"order_id"`
	InstrumentUID string  `json:"instrument_uid"`
	Direction     string  `json:"direction"`
	OrderType     string  `json:"order_type"`
	Lots          int64   `json:"lots"`
	Price         float64 `json:"price"`
	Status        string  `json:"status"`
}

type StopOrderResult struct {
	StopOrderID string `json:"stop_order_id"`
}

type StopOrderSummary struct {
	StopOrderID   string  `json:"stop_order_id"`
	InstrumentUID string  `json:"instrument_uid"`
	Direction     string  `json:"direction"`
	StopOrderType string  `json:"stop_order_type"`
	Lots          int64   `json:"lots"`
	StopPrice     float64 `json:"stop_price"`
	Price         float64 `json:"price"`
	Status        string  `json:"status"`
}

type Candle struct {
	Open       float64 `json:"open"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Close      float64 `json:"close"`
	Volume     int64   `json:"volume"`
	IsComplete bool    `json:"is_complete"`
}

type Orderbook struct {
	InstrumentUID string         `json:"instrument_uid"`
	Depth         int32          `json:"depth"`
	Bids          []OrderbookRow `json:"bids"`
	Asks          []OrderbookRow `json:"asks"`
	LastPrice     float64        `json:"last_price"`
}

type OrderbookRow struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

type LastPrice struct {
	InstrumentUID string  `json:"instrument_uid"`
	Price         float64 `json:"price"`
}

type TradingStatus struct {
	InstrumentUID        string `json:"instrument_uid"`
	TradingStatus        string `json:"trading_status"`
	LimitOrderAvailable  bool   `json:"limit_order_available"`
	MarketOrderAvailable bool   `json:"market_order_available"`
	APITradeAvailable    bool   `json:"api_trade_available"`
}

type Position struct {
	InstrumentUID  string  `json:"instrument_uid"`
	InstrumentType string  `json:"instrument_type"`
	Quantity       float64 `json:"quantity"`
	AveragePrice   float64 `json:"average_price"`
}

type PortfolioInfo struct {
	AccountID         string     `json:"account_id"`
	TotalAmountShares float64    `json:"total_amount_shares"`
	Positions         []Position `json:"positions"`
}

type WithdrawLimit struct {
	Currency       string  `json:"currency"`
	WithdrawAmount float64 `json:"withdraw_amount"`
}

type OperationsPage struct {
	Operations []Operation `json:"operations"`
	HasNext    bool        `json:"has_next"`
}

type Operation struct {
	ID            string  `json:"id"`
	InstrumentUID string  `json:"instrument_uid"`
	Type          string  `json:"type"`
	State         string  `json:"state"`
	Payment       float64 `json:"payment"`
	Quantity      int64   `json:"quantity"`
}

type MarginInfo struct {
	LiquidPortfolio       float64 `json:"liquid_portfolio"`
	StartingMargin        float64 `json:"starting_margin"`
	MinimalMargin         float64 `json:"minimal_margin"`
	FundsSufficiencyLevel float64 `json:"funds_sufficiency_level"`
}

type RiskStatus struct {
	CircuitBreakerTripped bool `json:"circuit_breaker_tripped"`
	FailureCount          int  `json:"failure_count"`
	Threshold             int  `json:"threshold"`
}

// --- Convenience ---

func getAccounts() ([]Account, error) {
	resp, err := client.Get(baseURL + "/api/v1/accounts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r apiResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode: %w (body: %s)", err, string(body))
	}
	if r.Error != "" {
		return nil, fmt.Errorf("api: %s", r.Error)
	}
	var accs []Account
	if err := json.Unmarshal(r.Data, &accs); err != nil {
		return nil, err
	}
	return accs, nil
}

func rateLimitPause() {
	time.Sleep(500 * time.Millisecond)
}

// CurrentTime returns current UTC time
func CurrentTime() time.Time {
	return time.Now().UTC()
}

func roundPrice(p float64) float64 {
	return math.Round(p*100) / 100
}
