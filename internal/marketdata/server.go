package marketdata

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

// Run initialises the market data service and starts the gRPC server.
// It blocks until ctx is cancelled, then performs a graceful shutdown.
func Run(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	token, err := config.GetTInvestToken()
	if err != nil {
		return fmt.Errorf("marketdata service: %w", err)
	}

	tinvestClient, err := tinvest.NewTInvestClient(ctx, cfg.TInvest.Endpoint, token, logger.Logger)
	if err != nil {
		return fmt.Errorf("marketdata service: create tinvest client: %w", err)
	}
	defer tinvestClient.Stop()

	rl := tinvest.NewRateLimiterManager(cfg.RateLimits)
	instruments := NewInstrumentCache()
	prices := NewPriceCache()

	svc := NewService(tinvestClient, rl, instruments, prices, logger)
	sm := NewStreamManager(tinvestClient, prices, cfg.MarketDataStream, logger)

	addr := fmt.Sprintf(":%d", cfg.Services.MarketDataPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("marketdata service: listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	logger.Info("MarketData gRPC server listening", "addr", addr)

	go func() {
		if err := sm.Listen(ctx); err != nil && ctx.Err() == nil {
			logger.Error("stream listener error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Info("MarketData gRPC server shutting down")
		sm.Stop()
		grpcServer.GracefulStop()
	}()

	_ = svc

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("marketdata service: grpc serve: %w", err)
	}
	return nil
}

// RegisterWithServer registers the market data Service handlers on an existing gRPC server.
// Call this once proto-generated code is available to wire up the real server descriptor.
func RegisterWithServer(_ *grpc.Server, _ *Service) {
	// TODO: generated_pb.RegisterMarketDataServiceServer(srv, handler) once proto is generated
}
