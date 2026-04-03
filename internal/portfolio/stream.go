package portfolio

import (
	"context"
	"sync"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

type PortfolioEvent struct {
	AccountID          string
	TotalAmountShares  float64
	TotalAmountBonds   float64
	TotalAmountETF     float64
	TotalAmountCurrencies float64
	TotalAmountFutures float64
	ExpectedYield      float64
	Positions          []Position
}

type PositionEvent struct {
	AccountID string
	Money     []PositionMoney
	Securities []PositionSecurity
	Date      time.Time
}

type PositionMoney struct {
	Currency        string
	AvailableAmount float64
	BlockedAmount   float64
}

type PositionSecurity struct {
	InstrumentUID string
	FIGI          string
	Blocked       int64
	Balance       int64
}

type PortfolioStreamManager struct {
	tinvestClient *tinvest.Client
	logger        *logging.Logger

	mu              sync.Mutex
	portfolioSubs   []chan<- PortfolioEvent
	positionSubs    []chan<- PositionEvent
}

func NewPortfolioStreamManager(client *tinvest.Client, logger *logging.Logger) *PortfolioStreamManager {
	return &PortfolioStreamManager{
		tinvestClient: client,
		logger:        logger,
	}
}

func (m *PortfolioStreamManager) SubscribePortfolio(bufSize int) <-chan PortfolioEvent {
	ch := make(chan PortfolioEvent, bufSize)
	m.mu.Lock()
	m.portfolioSubs = append(m.portfolioSubs, ch)
	m.mu.Unlock()
	return ch
}

func (m *PortfolioStreamManager) SubscribePositions(bufSize int) <-chan PositionEvent {
	ch := make(chan PositionEvent, bufSize)
	m.mu.Lock()
	m.positionSubs = append(m.positionSubs, ch)
	m.mu.Unlock()
	return ch
}

// StreamPortfolio subscribes to T-Invest portfolio stream.
// Falls back to periodic polling if the stream fails to open.
func (m *PortfolioStreamManager) StreamPortfolio(ctx context.Context, accounts []string) error {
	l := m.logger.WithContext(ctx)
	l.Info("StreamPortfolio starting", "accounts", accounts)

	streamClient := m.tinvestClient.OperationsStreamClient()
	stream, err := streamClient.PortfolioStream(accounts)
	if err != nil {
		l.Warn("StreamPortfolio: stream unavailable, falling back to polling", "error", err)
		return m.pollPortfolio(ctx, accounts)
	}

	go func() {
		if listenErr := stream.Listen(); listenErr != nil {
			l.Error("StreamPortfolio: listen error", "error", listenErr)
		}
	}()

	defer stream.Stop()

	ch := stream.Portfolios()
	for {
		select {
		case <-ctx.Done():
			l.Info("StreamPortfolio stopped")
			return ctx.Err()
		case pf, ok := <-ch:
			if !ok {
				l.Warn("StreamPortfolio: channel closed, falling back to polling")
				return m.pollPortfolio(ctx, accounts)
			}
			m.fanOutPortfolio(pf)
		}
	}
}

// StreamPositions subscribes to T-Invest positions stream.
// Falls back to periodic polling if the stream fails to open.
func (m *PortfolioStreamManager) StreamPositions(ctx context.Context, accounts []string) error {
	l := m.logger.WithContext(ctx)
	l.Info("StreamPositions starting", "accounts", accounts)

	streamClient := m.tinvestClient.OperationsStreamClient()
	stream, err := streamClient.PositionsStream(accounts)
	if err != nil {
		l.Warn("StreamPositions: stream unavailable, falling back to polling", "error", err)
		return m.pollPositions(ctx, accounts)
	}

	go func() {
		if listenErr := stream.Listen(); listenErr != nil {
			l.Error("StreamPositions: listen error", "error", listenErr)
		}
	}()

	defer stream.Stop()

	ch := stream.Positions()
	for {
		select {
		case <-ctx.Done():
			l.Info("StreamPositions stopped")
			return ctx.Err()
		case pd, ok := <-ch:
			if !ok {
				l.Warn("StreamPositions: channel closed, falling back to polling")
				return m.pollPositions(ctx, accounts)
			}
			m.fanOutPosition(pd)
		}
	}
}

func (m *PortfolioStreamManager) fanOutPortfolio(pf *pb.PortfolioResponse) {
	if pf == nil {
		return
	}

	positions := make([]Position, 0, len(pf.GetPositions()))
	for _, p := range pf.GetPositions() {
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

	evt := PortfolioEvent{
		AccountID:             pf.GetAccountId(),
		TotalAmountShares:     moneyToFloat(pf.GetTotalAmountShares()),
		TotalAmountBonds:      moneyToFloat(pf.GetTotalAmountBonds()),
		TotalAmountETF:        moneyToFloat(pf.GetTotalAmountEtf()),
		TotalAmountCurrencies: moneyToFloat(pf.GetTotalAmountCurrencies()),
		TotalAmountFutures:    moneyToFloat(pf.GetTotalAmountFutures()),
		ExpectedYield:         quotationToFloat(pf.GetExpectedYield()),
		Positions:             positions,
	}

	m.mu.Lock()
	subs := make([]chan<- PortfolioEvent, len(m.portfolioSubs))
	copy(subs, m.portfolioSubs)
	m.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub <- evt:
		default:
			m.logger.Warn("StreamPortfolio: subscriber slow, dropping event")
		}
	}
}

func (m *PortfolioStreamManager) fanOutPosition(pd *pb.PositionData) {
	if pd == nil {
		return
	}

	var money []PositionMoney
	for _, mv := range pd.GetMoney() {
		money = append(money, PositionMoney{
			Currency:        mv.GetAvailableValue().GetCurrency(),
			AvailableAmount: moneyToFloat(mv.GetAvailableValue()),
			BlockedAmount:   moneyToFloat(mv.GetBlockedValue()),
		})
	}

	var securities []PositionSecurity
	for _, sv := range pd.GetSecurities() {
		securities = append(securities, PositionSecurity{
			InstrumentUID: sv.GetInstrumentUid(),
			FIGI:          sv.GetFigi(),
			Blocked:       sv.GetBlocked(),
			Balance:       sv.GetBalance(),
		})
	}

	evt := PositionEvent{
		AccountID:  pd.GetAccountId(),
		Money:      money,
		Securities: securities,
		Date:       pd.GetDate().AsTime(),
	}

	m.mu.Lock()
	subs := make([]chan<- PositionEvent, len(m.positionSubs))
	copy(subs, m.positionSubs)
	m.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub <- evt:
		default:
			m.logger.Warn("StreamPositions: subscriber slow, dropping event")
		}
	}
}

const pollInterval = 5 * time.Second

func (m *PortfolioStreamManager) pollPortfolio(ctx context.Context, accounts []string) error {
	l := m.logger.WithContext(ctx)
	l.Info("pollPortfolio starting", "accounts", accounts, "interval", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	opsClient := m.tinvestClient.OperationsServiceClient()

	for {
		select {
		case <-ctx.Done():
			l.Info("pollPortfolio stopped")
			return ctx.Err()
		case <-ticker.C:
			for _, acct := range accounts {
				resp, err := opsClient.GetPortfolio(acct, pb.PortfolioRequest_RUB)
				if err != nil {
					l.Error("pollPortfolio: GetPortfolio failed", "account_id", acct, "error", err)
					continue
				}
				m.fanOutPortfolio(resp.PortfolioResponse)
			}
		}
	}
}

func (m *PortfolioStreamManager) pollPositions(ctx context.Context, accounts []string) error {
	l := m.logger.WithContext(ctx)
	l.Info("pollPositions starting", "accounts", accounts, "interval", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	opsClient := m.tinvestClient.OperationsServiceClient()

	for {
		select {
		case <-ctx.Done():
			l.Info("pollPositions stopped")
			return ctx.Err()
		case <-ticker.C:
			for _, acct := range accounts {
				resp, err := opsClient.GetPositions(acct)
				if err != nil {
					l.Error("pollPositions: GetPositions failed", "account_id", acct, "error", err)
					continue
				}
				m.fanOutPositionsFromSnapshot(acct, resp.PositionsResponse)
			}
		}
	}
}

func (m *PortfolioStreamManager) fanOutPositionsFromSnapshot(accountID string, resp *pb.PositionsResponse) {
	if resp == nil {
		return
	}

	var money []PositionMoney
	for _, mv := range resp.GetMoney() {
		money = append(money, PositionMoney{
			Currency:        mv.GetCurrency(),
			AvailableAmount: moneyToFloat(mv),
		})
	}
	for _, bv := range resp.GetBlocked() {
		for i := range money {
			if money[i].Currency == bv.GetCurrency() {
				money[i].BlockedAmount = moneyToFloat(bv)
			}
		}
	}

	var securities []PositionSecurity
	for _, sv := range resp.GetSecurities() {
		securities = append(securities, PositionSecurity{
			InstrumentUID: sv.GetInstrumentUid(),
			FIGI:          sv.GetFigi(),
			Blocked:       sv.GetBlocked(),
			Balance:       sv.GetBalance(),
		})
	}

	evt := PositionEvent{
		AccountID:  accountID,
		Money:      money,
		Securities: securities,
		Date:       time.Now(),
	}

	m.mu.Lock()
	subs := make([]chan<- PositionEvent, len(m.positionSubs))
	copy(subs, m.positionSubs)
	m.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub <- evt:
		default:
			m.logger.Warn("pollPositions: subscriber slow, dropping event")
		}
	}
}

func (m *PortfolioStreamManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.portfolioSubs {
		close(ch)
	}
	m.portfolioSubs = nil

	for _, ch := range m.positionSubs {
		close(ch)
	}
	m.positionSubs = nil
}
