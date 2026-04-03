package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// --- mock order service ---

type mockOrderService struct {
	postResult    *OrderResult
	orders        []OrderSummary
	state         *OrderState
	cancelResult  *CancelOrderResult
	replaceResult *OrderResult
	err           error
}

func (m *mockOrderService) PostOrder(_ context.Context, _, _ string, _ int64, _, _ string, _ float64) (*OrderResult, error) {
	return m.postResult, m.err
}
func (m *mockOrderService) GetOrders(_ context.Context, _ string) ([]OrderSummary, error) {
	return m.orders, m.err
}
func (m *mockOrderService) GetOrderState(_ context.Context, _, _ string) (*OrderState, error) {
	return m.state, m.err
}
func (m *mockOrderService) CancelOrder(_ context.Context, _, _ string) (*CancelOrderResult, error) {
	return m.cancelResult, m.err
}
func (m *mockOrderService) ReplaceOrder(_ context.Context, _, _ string, _ int64, _ float64) (*OrderResult, error) {
	return m.replaceResult, m.err
}

// --- mock risk service ---

type mockRiskService struct {
	status *RiskStatus
	err    error
}

func (m *mockRiskService) GetRiskStatus(_ context.Context) (*RiskStatus, error) {
	return m.status, m.err
}
func (m *mockRiskService) ResetCircuitBreaker(_ context.Context) error {
	return m.err
}

func orderRouter(svc OrderService) *chi.Mux {
	r := chi.NewRouter()
	NewOrderHandlers(svc).Routes(r)
	return r
}

func riskRouter(svc RiskService) *chi.Mux {
	r := chi.NewRouter()
	NewRiskHandlers(svc).Routes(r)
	return r
}

// --- order handler tests ---

func TestPostOrder_Created(t *testing.T) {
	svc := &mockOrderService{postResult: &OrderResult{
		OrderID:         "o1",
		ExecutionStatus: "new",
		LotsRequested:   1,
		Direction:       "buy",
		OrderType:       "market",
	}}
	body, _ := json.Marshal(postOrderRequest{
		AccountID:     "acc1",
		InstrumentUID: "uid1",
		Quantity:      1,
		Direction:     "buy",
		OrderType:     "market",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestPostOrder_MissingFields(t *testing.T) {
	svc := &mockOrderService{}
	body, _ := json.Marshal(postOrderRequest{Quantity: 1})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPostOrder_InvalidJSON(t *testing.T) {
	svc := &mockOrderService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListOrders_MissingAccountID(t *testing.T) {
	svc := &mockOrderService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListOrders_OK(t *testing.T) {
	svc := &mockOrderService{orders: []OrderSummary{
		{OrderID: "o1", Direction: "buy"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?account_id=acc1", nil)
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestGetOrderState_MissingAccountID(t *testing.T) {
	svc := &mockOrderService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/o1", nil)
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCancelOrder_OK(t *testing.T) {
	svc := &mockOrderService{cancelResult: &CancelOrderResult{CancelledAt: time.Now()}}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/orders/o1?account_id=acc1", nil)
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReplaceOrder_MissingAccountID(t *testing.T) {
	svc := &mockOrderService{}
	body, _ := json.Marshal(replaceOrderRequest{Quantity: 2, Price: 100})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/orders/o1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	orderRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- account handler tests ---

type mockAccountService struct {
	accounts []Account
	margin   *MarginInfo
	err      error
}

func (m *mockAccountService) GetAccounts(_ context.Context) ([]Account, error) {
	return m.accounts, m.err
}
func (m *mockAccountService) GetMarginAttributes(_ context.Context, _ string) (*MarginInfo, error) {
	return m.margin, m.err
}

func accountRouter(svc AccountService) *chi.Mux {
	r := chi.NewRouter()
	NewAccountHandlers(svc).Routes(r)
	return r
}

func TestListAccounts_OK(t *testing.T) {
	svc := &mockAccountService{accounts: []Account{{ID: "a1", Name: "test"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	w := httptest.NewRecorder()
	accountRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetMargin_OK(t *testing.T) {
	svc := &mockAccountService{margin: &MarginInfo{LiquidPortfolio: 100000}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/margin/acc1", nil)
	w := httptest.NewRecorder()
	accountRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- market data handler tests ---

type mockMarketDataService struct {
	candles []Candle
	book    *Orderbook
	prices  []LastPrice
	status  *TradingStatus
	err     error
}

func (m *mockMarketDataService) GetCandles(_ context.Context, _ string, _, _ time.Time, _ string) ([]Candle, error) {
	return m.candles, m.err
}
func (m *mockMarketDataService) GetOrderbook(_ context.Context, _ string, _ int32) (*Orderbook, error) {
	return m.book, m.err
}
func (m *mockMarketDataService) GetLastPrices(_ context.Context, _ []string) ([]LastPrice, error) {
	return m.prices, m.err
}
func (m *mockMarketDataService) GetTradingStatus(_ context.Context, _ string) (*TradingStatus, error) {
	return m.status, m.err
}

func mdRouter(svc MarketDataService) *chi.Mux {
	r := chi.NewRouter()
	NewMarketDataHandlers(svc).Routes(r)
	return r
}

func TestGetCandles_MissingUID(t *testing.T) {
	svc := &mockMarketDataService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/candles", nil)
	w := httptest.NewRecorder()
	mdRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetCandles_OK(t *testing.T) {
	svc := &mockMarketDataService{candles: []Candle{{Open: 100, Close: 101}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/candles?instrument_uid=uid1", nil)
	w := httptest.NewRecorder()
	mdRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetOrderbook_OK(t *testing.T) {
	svc := &mockMarketDataService{book: &Orderbook{InstrumentUID: "uid1", Depth: 20}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orderbook/uid1", nil)
	w := httptest.NewRecorder()
	mdRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetPrices_MissingUID(t *testing.T) {
	svc := &mockMarketDataService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices", nil)
	w := httptest.NewRecorder()
	mdRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetPrices_OK(t *testing.T) {
	svc := &mockMarketDataService{prices: []LastPrice{{InstrumentUID: "uid1", Price: 100}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices?instrument_uid=uid1", nil)
	w := httptest.NewRecorder()
	mdRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetTradingStatus_OK(t *testing.T) {
	svc := &mockMarketDataService{status: &TradingStatus{InstrumentUID: "uid1", APITradeAvailable: true}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trading-status/uid1", nil)
	w := httptest.NewRecorder()
	mdRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- portfolio handler tests ---

type mockPortfolioService struct {
	positions []Position
	portfolio *PortfolioInfo
	limits    []WithdrawLimit
	ops       *OperationsPage
	err       error
}

func (m *mockPortfolioService) GetPositions(_ context.Context, _ string) ([]Position, error) {
	return m.positions, m.err
}
func (m *mockPortfolioService) GetPortfolio(_ context.Context, _ string) (*PortfolioInfo, error) {
	return m.portfolio, m.err
}
func (m *mockPortfolioService) GetWithdrawLimits(_ context.Context, _ string) ([]WithdrawLimit, error) {
	return m.limits, m.err
}
func (m *mockPortfolioService) GetOperations(_ context.Context, _, _ string, _, _ time.Time) (*OperationsPage, error) {
	return m.ops, m.err
}

func portfolioRouter(svc PortfolioService) *chi.Mux {
	r := chi.NewRouter()
	NewPortfolioHandlers(svc).Routes(r)
	return r
}

func TestGetPositions_MissingAccountID(t *testing.T) {
	svc := &mockPortfolioService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetPositions_OK(t *testing.T) {
	svc := &mockPortfolioService{positions: []Position{{InstrumentUID: "uid1", Quantity: 5}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions?account_id=acc1", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetPortfolio_MissingAccountID(t *testing.T) {
	svc := &mockPortfolioService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetPortfolio_OK(t *testing.T) {
	svc := &mockPortfolioService{portfolio: &PortfolioInfo{AccountID: "acc1"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio?account_id=acc1", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetLimits_MissingAccountID(t *testing.T) {
	svc := &mockPortfolioService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetLimits_OK(t *testing.T) {
	svc := &mockPortfolioService{limits: []WithdrawLimit{{Currency: "RUB", WithdrawAmount: 100}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits?account_id=acc1", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetOperations_MissingAccountID(t *testing.T) {
	svc := &mockPortfolioService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/operations", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetOperations_OK(t *testing.T) {
	svc := &mockPortfolioService{ops: &OperationsPage{Operations: []Operation{}, HasNext: false}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/operations?account_id=acc1", nil)
	w := httptest.NewRecorder()
	portfolioRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- stop order handler tests ---

type mockStopOrderService struct {
	postResult   *StopOrderResult
	orders       []StopOrderSummary
	cancelResult *CancelStopOrderResult
	err          error
}

func (m *mockStopOrderService) PostStopOrder(_ context.Context, _, _ string, _ int64, _, _ string, _, _ float64) (*StopOrderResult, error) {
	return m.postResult, m.err
}
func (m *mockStopOrderService) GetStopOrders(_ context.Context, _ string) ([]StopOrderSummary, error) {
	return m.orders, m.err
}
func (m *mockStopOrderService) CancelStopOrder(_ context.Context, _, _ string) (*CancelStopOrderResult, error) {
	return m.cancelResult, m.err
}

func stopOrderRouter(svc StopOrderService) *chi.Mux {
	r := chi.NewRouter()
	NewStopOrderHandlers(svc).Routes(r)
	return r
}

func TestPostStopOrder_Created(t *testing.T) {
	svc := &mockStopOrderService{postResult: &StopOrderResult{StopOrderID: "so1"}}
	body, _ := json.Marshal(postStopOrderRequest{
		AccountID:     "acc1",
		InstrumentUID: "uid1",
		Quantity:      1,
		Direction:     "buy",
		StopOrderType: "stop_loss",
		StopPrice:     90,
		Price:         89,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stop-orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	stopOrderRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestPostStopOrder_MissingFields(t *testing.T) {
	svc := &mockStopOrderService{}
	body, _ := json.Marshal(postStopOrderRequest{Quantity: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stop-orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	stopOrderRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListStopOrders_MissingAccountID(t *testing.T) {
	svc := &mockStopOrderService{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stop-orders", nil)
	w := httptest.NewRecorder()
	stopOrderRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListStopOrders_OK(t *testing.T) {
	svc := &mockStopOrderService{orders: []StopOrderSummary{{StopOrderID: "so1"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stop-orders?account_id=acc1", nil)
	w := httptest.NewRecorder()
	stopOrderRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCancelStopOrder_MissingAccountID(t *testing.T) {
	svc := &mockStopOrderService{}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stop-orders/so1", nil)
	w := httptest.NewRecorder()
	stopOrderRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCancelStopOrder_OK(t *testing.T) {
	svc := &mockStopOrderService{cancelResult: &CancelStopOrderResult{CancelledAt: time.Now()}}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stop-orders/so1?account_id=acc1", nil)
	w := httptest.NewRecorder()
	stopOrderRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- risk handler tests ---

func TestGetRiskStatus_OK(t *testing.T) {
	svc := &mockRiskService{status: &RiskStatus{
		CircuitBreakerTripped: false,
		FailureCount:          0,
		Threshold:             5,
		Cooldown:              "5m",
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/risk/status", nil)
	w := httptest.NewRecorder()

	riskRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestResetCircuitBreaker_OK(t *testing.T) {
	svc := &mockRiskService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risk/reset", nil)
	w := httptest.NewRecorder()

	riskRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
