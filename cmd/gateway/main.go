package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/24alert/trading-bot/internal/gateway"
	"github.com/24alert/trading-bot/internal/gateway/adapter"
	"github.com/24alert/trading-bot/internal/gateway/cli"
	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/internal/portfolio"
	"github.com/24alert/trading-bot/internal/risk"
	"github.com/24alert/trading-bot/internal/risk/checker"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

func main() {
	root := cli.NewRootCmd()

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the gateway HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd)
		},
	}
	root.AddCommand(serveCmd)

	root.RunE = func(cmd *cobra.Command, args []string) error {
		return runServer(cmd)
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command) error {
	configPath, _ := cmd.Flags().GetString("config")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output, cfg.Logging.FilePath)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		return fmt.Errorf("setup logging: %w", err)
	}

	logger.Info("Gateway starting", slog.Int("port", cfg.Services.GatewayPort))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	token, err := config.GetTInvestToken()
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	tinvestClient, err := tinvest.NewTInvestClient(ctx, cfg.TInvest.Endpoint, token, logger.Logger)
	if err != nil {
		return fmt.Errorf("create tinvest client: %w", err)
	}
	defer tinvestClient.Stop()

	rlm := tinvest.NewRateLimiterManager(cfg.RateLimits)

	orderRepo := order.NewRepository()
	orderSvc := order.NewService(tinvestClient, rlm, orderRepo, logger)

	instrumentCache := marketdata.NewInstrumentCache()
	priceCache := marketdata.NewPriceCache()
	mdSvc := marketdata.NewService(tinvestClient, rlm, instrumentCache, priceCache, logger)

	portfolioSvc := portfolio.NewService(tinvestClient, rlm, logger)

	cb := risk.NewCircuitBreaker(cfg.Risk.CircuitBreakerThreshold, cfg.Risk.CircuitBreakerCooldown)
	sessionChecker := checker.NewSessionChecker(adapter.StubMarketDataQuerier{})
	balanceChecker := checker.NewBalanceChecker(adapter.StubPortfolioQuerier{})
	positionChecker := checker.NewPositionLimitChecker(adapter.StubPortfolioQuerier{}, cfg.Risk.MaxPositionLots)
	riskSvc := risk.NewService(sessionChecker, balanceChecker, positionChecker, cb, cfg.Risk, logger)

	svcs := gateway.Services{
		Orders:     adapter.NewOrderAdapter(orderSvc),
		StopOrders: adapter.NewStopOrderAdapter(orderSvc),
		MarketData: adapter.NewMarketDataAdapter(mdSvc),
		Portfolio:  adapter.NewPortfolioAdapter(portfolioSvc),
		Accounts:   adapter.NewAccountAdapter(portfolioSvc),
		Risk:       adapter.NewRiskAdapter(riskSvc),
	}

	if err := gateway.Run(ctx, cfg, logger, svcs); err != nil {
		logger.Error("gateway stopped with error", "error", err)
		return err
	}
	return nil
}
