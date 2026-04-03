package portfolio

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

// Run initialises the portfolio service and starts the gRPC server.
// It blocks until ctx is cancelled, then performs a graceful shutdown.
func Run(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	token, err := config.GetTInvestToken()
	if err != nil {
		return fmt.Errorf("portfolio service: %w", err)
	}

	tinvestClient, err := tinvest.NewTInvestClient(ctx, cfg.TInvest.Endpoint, token, logger.Logger)
	if err != nil {
		return fmt.Errorf("portfolio service: create tinvest client: %w", err)
	}
	defer tinvestClient.Stop()

	rl := tinvest.NewRateLimiterManager(cfg.RateLimits)
	svc := NewService(tinvestClient, rl, logger)
	sm := NewPortfolioStreamManager(tinvestClient, logger)

	addr := fmt.Sprintf(":%d", cfg.Services.PortfolioPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("portfolio service: listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	logger.Info("Portfolio gRPC server listening", "addr", addr)

	metricsPort := cfg.Services.PortfolioPort + 100
	go func() {
		logger.Info("Portfolio metrics server listening", "port", metricsPort)
		if err := metrics.ServeHTTP(ctx, metricsPort); err != nil {
			logger.Error("Portfolio metrics server error", "error", err)
		}
	}()

	if acct := cfg.TInvest.SandboxAccountID; acct != "" {
		accounts := []string{acct}
		go func() {
			if err := sm.StreamPortfolio(ctx, accounts); err != nil && ctx.Err() == nil {
				logger.Error("StreamPortfolio error", "error", err)
			}
		}()
		go func() {
			if err := sm.StreamPositions(ctx, accounts); err != nil && ctx.Err() == nil {
				logger.Error("StreamPositions error", "error", err)
			}
		}()
	}

	go func() {
		<-ctx.Done()
		logger.Info("Portfolio gRPC server shutting down")
		sm.Stop()
		grpcServer.GracefulStop()
	}()

	_ = svc

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("portfolio service: grpc serve: %w", err)
	}
	return nil
}

// RegisterWithServer registers the portfolio Service handlers on an existing gRPC server.
func RegisterWithServer(_ *grpc.Server, _ *Service) {
	// TODO: generated_pb.RegisterPortfolioServiceServer(srv, handler) once proto is generated
}
