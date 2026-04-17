package portfolio

import (
	"context"
	"fmt"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
	"github.com/24alert/trading-bot/pkg/tinvest"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

type Position struct {
	InstrumentUID  string
	InstrumentType string
	FIGI           string
	Quantity       float64
	AveragePrice   float64
	ExpectedYield  float64
	CurrentPrice   float64
	Currency       string
	Blocked        bool
}

type PortfolioInfo struct {
	AccountID             string
	TotalAmountShares     float64
	TotalAmountBonds      float64
	TotalAmountETF        float64
	TotalAmountCurrencies float64
	TotalAmountFutures    float64
	ExpectedYield         float64
	Positions             []Position
}

type WithdrawLimit struct {
	Currency       string
	BlockedAmount  float64
	WithdrawAmount float64
}

type Operation struct {
	ID            string
	AccountID     string
	InstrumentUID string
	Type          string
	State         string
	Payment       float64
	Currency      string
	Quantity      int64
	Date          time.Time
	Trades        []OperationTrade
}

type OperationTrade struct {
	TradeID  string
	Price    float64
	Quantity int64
	Date     time.Time
}

type Account struct {
	ID          string
	Type        string
	Name        string
	Status      string
	OpenedDate  time.Time
	ClosedDate  time.Time
	AccessLevel string
}

type MarginInfo struct {
	LiquidPortfolio       float64
	StartingMargin        float64
	MinimalMargin         float64
	FundsSufficiencyLevel float64
	AmountOfMissing       float64
	CorrectedMargin       float64
}

type OperationsPage struct {
	Operations []Operation
	NextCursor string
	HasNext    bool
}

type Service struct {
	tinvestClient *tinvest.Client
	rateLimiter   *tinvest.RateLimiterManager
	logger        *logging.Logger
}

func NewService(
	client *tinvest.Client,
	rl *tinvest.RateLimiterManager,
	logger *logging.Logger,
) *Service {
	return &Service{
		tinvestClient: client,
		rateLimiter:   rl,
		logger:        logger,
	}
}

func (s *Service) GetPositions(ctx context.Context, accountID string) ([]Position, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetPositions", "account_id", accountID)

	if err := s.rateLimiter.Wait(ctx, "get_positions"); err != nil {
		return nil, fmt.Errorf("GetPositions: rate limit: %w", err)
	}

	start := time.Now()
	resp, err := s.tinvestClient.OperationsServiceClient().GetPortfolio(accountID, pb.PortfolioRequest_RUB)
	duration := time.Since(start).Seconds()

	metrics.TInvestLatency.WithLabelValues("operations", "GetPortfolio").Observe(duration)
	if err != nil {
		metrics.TInvestRequestsTotal.WithLabelValues("operations", "GetPortfolio", "failure").Inc()
		metrics.TInvestErrorsTotal.WithLabelValues("operations", "GetPortfolio", "error").Inc()
		return nil, fmt.Errorf("GetPositions: %w", err)
	}
	metrics.TInvestRequestsTotal.WithLabelValues("operations", "GetPortfolio", "success").Inc()

	positions := make([]Position, 0, len(resp.GetPositions()))
	for _, p := range resp.GetPositions() {
		positions = append(positions, Position{
			InstrumentUID:  p.GetInstrumentUid(),
			InstrumentType: p.GetInstrumentType(),
			FIGI:           p.GetFigi(),
			Quantity:       quotationToFloat(p.GetQuantity()),
			AveragePrice:   moneyToFloat(p.GetAveragePositionPrice()),
			ExpectedYield:  quotationToFloat(p.GetExpectedYield()),
			CurrentPrice:   moneyToFloat(p.GetCurrentPrice()),
			Currency:       p.GetAveragePositionPrice().GetCurrency(),
			Blocked:        p.GetBlocked(),
		})
	}

	l.Info("GetPositions completed", "account_id", accountID, "count", len(positions))
	return positions, nil
}

func (s *Service) GetPortfolio(ctx context.Context, accountID string) (*PortfolioInfo, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetPortfolio", "account_id", accountID)

	if err := s.rateLimiter.Wait(ctx, "get_portfolio"); err != nil {
		return nil, fmt.Errorf("GetPortfolio: rate limit: %w", err)
	}

	start := time.Now()
	resp, err := s.tinvestClient.OperationsServiceClient().GetPortfolio(accountID, pb.PortfolioRequest_RUB)
	duration := time.Since(start).Seconds()

	metrics.TInvestLatency.WithLabelValues("operations", "GetPortfolio").Observe(duration)
	if err != nil {
		metrics.TInvestRequestsTotal.WithLabelValues("operations", "GetPortfolio", "failure").Inc()
		metrics.TInvestErrorsTotal.WithLabelValues("operations", "GetPortfolio", "error").Inc()
		return nil, fmt.Errorf("GetPortfolio: %w", err)
	}
	metrics.TInvestRequestsTotal.WithLabelValues("operations", "GetPortfolio", "success").Inc()

	positions := make([]Position, 0, len(resp.GetPositions()))
	for _, p := range resp.GetPositions() {
		positions = append(positions, Position{
			InstrumentUID:  p.GetInstrumentUid(),
			InstrumentType: p.GetInstrumentType(),
			FIGI:           p.GetFigi(),
			Quantity:       quotationToFloat(p.GetQuantity()),
			AveragePrice:   moneyToFloat(p.GetAveragePositionPrice()),
			ExpectedYield:  quotationToFloat(p.GetExpectedYield()),
			CurrentPrice:   moneyToFloat(p.GetCurrentPrice()),
			Currency:       p.GetAveragePositionPrice().GetCurrency(),
			Blocked:        p.GetBlocked(),
		})
	}

	info := &PortfolioInfo{
		AccountID:             accountID,
		TotalAmountShares:     moneyToFloat(resp.GetTotalAmountShares()),
		TotalAmountBonds:      moneyToFloat(resp.GetTotalAmountBonds()),
		TotalAmountETF:        moneyToFloat(resp.GetTotalAmountEtf()),
		TotalAmountCurrencies: moneyToFloat(resp.GetTotalAmountCurrencies()),
		TotalAmountFutures:    moneyToFloat(resp.GetTotalAmountFutures()),
		ExpectedYield:         quotationToFloat(resp.GetExpectedYield()),
		Positions:             positions,
	}

	l.Info("GetPortfolio completed", "account_id", accountID, "positions", len(positions))
	return info, nil
}

func (s *Service) GetWithdrawLimits(ctx context.Context, accountID string) ([]WithdrawLimit, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetWithdrawLimits", "account_id", accountID)

	if err := s.rateLimiter.Wait(ctx, "get_withdraw_limits"); err != nil {
		return nil, fmt.Errorf("GetWithdrawLimits: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OperationsServiceClient().GetWithdrawLimits(accountID)
	if err != nil {
		return nil, fmt.Errorf("GetWithdrawLimits: %w", err)
	}

	var limits []WithdrawLimit
	for _, m := range resp.GetMoney() {
		limits = append(limits, WithdrawLimit{
			Currency:       m.GetCurrency(),
			WithdrawAmount: moneyToFloat(m),
		})
	}
	for _, b := range resp.GetBlocked() {
		for i := range limits {
			if limits[i].Currency == b.GetCurrency() {
				limits[i].BlockedAmount = moneyToFloat(b)
			}
		}
	}

	l.Info("GetWithdrawLimits completed", "account_id", accountID, "currencies", len(limits))
	return limits, nil
}

func (s *Service) GetOperations(
	ctx context.Context,
	accountID, instrumentUID string,
	from, to time.Time,
	cursor string,
	limit int32,
) (*OperationsPage, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetOperations", "account_id", accountID, "instrument_uid", instrumentUID, "cursor", cursor, "limit", limit)

	if err := s.rateLimiter.Wait(ctx, "get_operations"); err != nil {
		return nil, fmt.Errorf("GetOperations: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OperationsServiceClient().GetOperationsByCursor(&investgo.GetOperationsByCursorRequest{
		AccountId:    accountID,
		InstrumentId: instrumentUID,
		From:         from,
		To:           to,
		Cursor:       cursor,
		Limit:        limit,
	})
	if err != nil {
		return nil, fmt.Errorf("GetOperations: %w", err)
	}

	var ops []Operation
	for _, item := range resp.GetItems() {
		op := Operation{
			ID:            item.GetId(),
			AccountID:     accountID,
			InstrumentUID: item.GetInstrumentUid(),
			Type:          item.GetType().String(),
			State:         item.GetState().String(),
			Payment:       moneyToFloat(item.GetPayment()),
			Currency:      item.GetPayment().GetCurrency(),
			Quantity:      int64(item.GetQuantityDone()),
			Date:          item.GetDate().AsTime(),
		}
		for _, t := range item.GetTradesInfo().GetTrades() {
			op.Trades = append(op.Trades, OperationTrade{
				TradeID:  t.GetNum(),
				Price:    moneyToFloat(t.GetPrice()),
				Quantity: t.GetQuantity(),
				Date:     t.GetDate().AsTime(),
			})
		}
		ops = append(ops, op)
	}

	page := &OperationsPage{
		Operations: ops,
		NextCursor: resp.GetNextCursor(),
		HasNext:    resp.GetHasNext(),
	}

	l.Info("GetOperations completed", "account_id", accountID, "count", len(ops), "has_next", page.HasNext)
	return page, nil
}

func (s *Service) GetAccounts(ctx context.Context) ([]Account, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetAccounts")

	if err := s.rateLimiter.Wait(ctx, "get_accounts"); err != nil {
		return nil, fmt.Errorf("GetAccounts: rate limit: %w", err)
	}

	start := time.Now()
	resp, err := s.tinvestClient.UsersServiceClient().GetAccounts(nil)
	duration := time.Since(start).Seconds()

	metrics.TInvestLatency.WithLabelValues("users", "GetAccounts").Observe(duration)
	if err != nil {
		metrics.TInvestRequestsTotal.WithLabelValues("users", "GetAccounts", "failure").Inc()
		metrics.TInvestErrorsTotal.WithLabelValues("users", "GetAccounts", "error").Inc()
		return nil, fmt.Errorf("GetAccounts: %w", err)
	}
	metrics.TInvestRequestsTotal.WithLabelValues("users", "GetAccounts", "success").Inc()

	var accounts []Account
	for _, a := range resp.GetAccounts() {
		accounts = append(accounts, Account{
			ID:          a.GetId(),
			Type:        a.GetType().String(),
			Name:        a.GetName(),
			Status:      a.GetStatus().String(),
			OpenedDate:  a.GetOpenedDate().AsTime(),
			ClosedDate:  a.GetClosedDate().AsTime(),
			AccessLevel: a.GetAccessLevel().String(),
		})
	}

	l.Info("GetAccounts completed", "count", len(accounts))
	return accounts, nil
}

func (s *Service) GetMarginAttributes(ctx context.Context, accountID string) (*MarginInfo, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetMarginAttributes", "account_id", accountID)

	if err := s.rateLimiter.Wait(ctx, "get_margin_attributes"); err != nil {
		return nil, fmt.Errorf("GetMarginAttributes: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.UsersServiceClient().GetMarginAttributes(accountID)
	if err != nil {
		return nil, fmt.Errorf("GetMarginAttributes: %w", err)
	}

	info := &MarginInfo{
		LiquidPortfolio:       moneyToFloat(resp.GetLiquidPortfolio()),
		StartingMargin:        moneyToFloat(resp.GetStartingMargin()),
		MinimalMargin:         moneyToFloat(resp.GetMinimalMargin()),
		FundsSufficiencyLevel: quotationToFloat(resp.GetFundsSufficiencyLevel()),
		AmountOfMissing:       moneyToFloat(resp.GetAmountOfMissingFunds()),
		CorrectedMargin:       moneyToFloat(resp.GetCorrectedMargin()),
	}

	l.Info("GetMarginAttributes completed", "account_id", accountID)
	return info, nil
}

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}

func moneyToFloat(m *pb.MoneyValue) float64 {
	if m == nil {
		return 0
	}
	return float64(m.GetUnits()) + float64(m.GetNano())/1e9
}
