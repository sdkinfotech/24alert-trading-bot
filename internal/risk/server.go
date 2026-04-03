package risk

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/24alert/trading-bot/internal/risk/checker"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
)

// Run initialises the risk service and starts the gRPC server.
// It blocks until ctx is cancelled, then performs a graceful shutdown.
func Run(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	portfolioAddr := fmt.Sprintf("localhost:%d", cfg.Services.PortfolioPort)
	marketdataAddr := fmt.Sprintf("localhost:%d", cfg.Services.MarketDataPort)

	portfolioConn, err := grpc.NewClient(portfolioAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("risk service: dial portfolio-svc %s: %w", portfolioAddr, err)
	}
	defer portfolioConn.Close()

	marketdataConn, err := grpc.NewClient(marketdataAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("risk service: dial marketdata-svc %s: %w", marketdataAddr, err)
	}
	defer marketdataConn.Close()

	logger.Info("risk service: gRPC client connections created",
		"portfolio_addr", portfolioAddr,
		"marketdata_addr", marketdataAddr,
	)

	// Placeholder queriers — swap with real generated-client wrappers once
	// proto is available.
	pq := &stubPortfolioQuerier{}
	mq := &stubMarketDataQuerier{}

	sessionChk := checker.NewSessionChecker(mq)
	balanceChk := checker.NewBalanceChecker(pq)
	positionChk := checker.NewPositionLimitChecker(pq, cfg.Risk.MaxPositionLots)

	cb := NewCircuitBreaker(cfg.Risk.CircuitBreakerThreshold, cfg.Risk.CircuitBreakerCooldown)
	svc := NewService(sessionChk, balanceChk, positionChk, cb, cfg.Risk, logger)

	addr := fmt.Sprintf(":%d", cfg.Services.RiskPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("risk service: listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	logger.Info("Risk gRPC server listening", "addr", addr)

	metricsPort := cfg.Services.RiskPort + 100
	go func() {
		logger.Info("Risk metrics server listening", "port", metricsPort)
		if err := metrics.ServeHTTP(ctx, metricsPort); err != nil {
			logger.Error("Risk metrics server error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Info("Risk gRPC server shutting down")
		grpcServer.GracefulStop()
	}()

	_ = svc // will be registered with proto-generated descriptor

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("risk service: grpc serve: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Stub queriers — replaced with real gRPC-client wrappers once proto is generated.
// ---------------------------------------------------------------------------

type stubPortfolioQuerier struct{}

func (s *stubPortfolioQuerier) GetPositions(_ context.Context, _ string) ([]checker.PositionInfo, error) {
	return nil, fmt.Errorf("portfolio querier: not implemented (proto not generated)")
}

func (s *stubPortfolioQuerier) GetWithdrawLimits(_ context.Context, _ string) ([]checker.WithdrawLimitInfo, error) {
	return nil, fmt.Errorf("portfolio querier: not implemented (proto not generated)")
}

type stubMarketDataQuerier struct{}

func (s *stubMarketDataQuerier) GetTradingStatus(_ context.Context, _ string) (*checker.TradingStatusInfo, error) {
	return nil, fmt.Errorf("marketdata querier: not implemented (proto not generated)")
}
