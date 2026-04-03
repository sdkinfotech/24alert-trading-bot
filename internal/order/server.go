package order

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

// Run initialises the order service and starts the gRPC server.
// It blocks until ctx is cancelled, then performs a graceful shutdown.
func Run(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	token, err := config.GetTInvestToken()
	if err != nil {
		return fmt.Errorf("order service: %w", err)
	}

	tinvestClient, err := tinvest.NewTInvestClient(ctx, cfg.TInvest.Endpoint, token, logger.Logger)
	if err != nil {
		return fmt.Errorf("order service: create tinvest client: %w", err)
	}
	defer tinvestClient.Stop()

	rl := tinvest.NewRateLimiterManager(cfg.RateLimits)
	repo := NewRepository()
	svc := NewService(tinvestClient, rl, repo, logger)
	sm := NewStreamManager(tinvestClient, repo, logger)

	addr := fmt.Sprintf(":%d", cfg.Services.OrderPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("order service: listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	logger.Info("Order gRPC server listening", "addr", addr)

	// Start background streams if a sandbox account is configured.
	if acct := cfg.TInvest.SandboxAccountID; acct != "" {
		accounts := []string{acct}
		go func() {
			if err := sm.StreamOrderStates(ctx, accounts); err != nil {
				logger.Error("StreamOrderStates error", "error", err)
			}
		}()
		go func() {
			if err := sm.StreamTrades(ctx, accounts); err != nil {
				logger.Error("StreamTrades error", "error", err)
			}
		}()
	}

	// Graceful shutdown on context cancellation.
	go func() {
		<-ctx.Done()
		logger.Info("Order gRPC server shutting down")
		grpcServer.GracefulStop()
	}()

	// Expose the service and stream manager for later proto registration.
	_ = svc
	_ = sm

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("order service: grpc serve: %w", err)
	}
	return nil
}

// RegisterWithServer registers the order Service handlers on an existing gRPC server.
// Call this once proto-generated code is available to wire up the real server descriptor.
func RegisterWithServer(_ *grpc.Server, _ *Service) {
	// TODO: generated_pb.RegisterOrderServiceServer(srv, handler) once proto is generated
}
