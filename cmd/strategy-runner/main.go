package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/24alert/trading-bot/internal/journal"
	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/internal/portfolio"
	"github.com/24alert/trading-bot/internal/risk"
	"github.com/24alert/trading-bot/internal/risk/checker"
	"github.com/24alert/trading-bot/internal/strategy"
	"github.com/24alert/trading-bot/internal/strategy/grpcadapter"
	"github.com/24alert/trading-bot/internal/strategy/orb"
	"github.com/24alert/trading-bot/internal/strategy/sma"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/notify/telegram"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

func applyCLIStrategyOverride(cfg *config.Config, o runOpts) error {
	ac := strings.TrimSpace(o.strategyAccount)
	uid := strings.TrimSpace(o.strategyInstrumentUID)
	if ac == "" && uid == "" {
		return nil
	}
	if ac == "" || uid == "" {
		return fmt.Errorf("for live CLI override set both --strategy-account and --strategy-instrument-uid")
	}
	typ := strings.TrimSpace(strings.ToLower(o.strategyType))
	if typ == "" {
		typ = "sma_crossover"
	}
	iid := strings.TrimSpace(o.strategyInstanceID)
	if iid == "" {
		iid = "live-sma-1"
	}
	inst := config.StrategyInstanceConfig{
		ID:          iid,
		Type:        typ,
		AccountID:   ac,
		Instruments: []string{uid},
		Enabled:     true,
		Params: map[string]string{
			"interval": strings.TrimSpace(o.strategyInterval),
			"quantity": strings.TrimSpace(o.strategyQuantity),
		},
	}
	switch typ {
	case "grpc":
		ep := strings.TrimSpace(o.strategyGRPCEndpoint)
		if ep == "" {
			return fmt.Errorf("--strategy-grpc-endpoint is required when --strategy-type=grpc")
		}
		inst.Endpoint = ep
	case "sma_crossover":
		// defaults for fast/slow come from strategy Configure
	case "orb_breakout":
		// defaults for range_candles/cutoff come from strategy Configure
	default:
		return fmt.Errorf("unsupported --strategy-type %q (use sma_crossover, orb_breakout, or grpc)", typ)
	}
	cfg.Strategies.Instances = []config.StrategyInstanceConfig{inst}
	return nil
}

func main() {
	var (
		configPath            string
		strategyAccount       string
		strategyInstrumentUID string
		strategyInstanceID    string
		strategyInterval      string
		strategyQuantity      string
		strategyType          string
		strategyGRPCEndpoint  string
	)
	root := &cobra.Command{
		Use:   "strategy-runner",
		Short: "24Alert strategy runner: market data → signals → risk → orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(runOpts{
				configPath:            configPath,
				strategyAccount:       strategyAccount,
				strategyInstrumentUID: strategyInstrumentUID,
				strategyInstanceID:    strategyInstanceID,
				strategyInterval:      strategyInterval,
				strategyQuantity:      strategyQuantity,
				strategyType:          strategyType,
				strategyGRPCEndpoint:  strategyGRPCEndpoint,
			})
		},
	}
	root.Flags().StringVar(&configPath, "config", "config/config.yaml", "path to config file")
	root.Flags().StringVar(&strategyAccount, "strategy-account", "", "if set with --strategy-instrument-uid, replaces strategies.instances with one enabled instance (live CLI)")
	root.Flags().StringVar(&strategyInstrumentUID, "strategy-instrument-uid", "", "T-Invest instrument UID (see InstrumentByUid / gateway prices API)")
	root.Flags().StringVar(&strategyInstanceID, "strategy-instance-id", "live-sma-1", "instance id when using CLI strategy override")
	root.Flags().StringVar(&strategyInterval, "strategy-interval", "1h", "candle interval for built-in strategies (e.g. 1m, 5m, 1h)")
	root.Flags().StringVar(&strategyQuantity, "strategy-quantity", "1", "order size in lots for built-in strategies")
	root.Flags().StringVar(&strategyType, "strategy-type", "sma_crossover", "strategy type: sma_crossover or grpc")
	root.Flags().StringVar(&strategyGRPCEndpoint, "strategy-grpc-endpoint", "", "required when --strategy-type=grpc")
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

type runOpts struct {
	configPath            string
	strategyAccount       string
	strategyInstrumentUID string
	strategyInstanceID    string
	strategyInterval      string
	strategyQuantity      string
	strategyType          string
	strategyGRPCEndpoint  string
}

func run(o runOpts) error {
	cfg, err := config.Load(o.configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return fmt.Errorf("load config: %w", err)
	}

	if err := applyCLIStrategyOverride(cfg, o); err != nil {
		return err
	}

	logger, err := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output, cfg.Logging.FilePath)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		return fmt.Errorf("setup logging: %w", err)
	}

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
	orderStream := order.NewStreamManager(tinvestClient, orderRepo, logger)

	instrumentCache := marketdata.NewInstrumentCache()
	priceCache := marketdata.NewPriceCache()
	mdSvc := marketdata.NewService(tinvestClient, rlm, instrumentCache, priceCache, logger)
	streamMgr := marketdata.NewStreamManager(tinvestClient, priceCache, cfg.MarketDataStream, logger)

	portfolioSvc := portfolio.NewService(tinvestClient, rlm, logger)

	pq := &strategy.TinvestPortfolioQuerier{Svc: portfolioSvc}
	mq := &strategy.TinvestMarketDataQuerier{Svc: mdSvc}
	sessionChecker := checker.NewSessionChecker(mq)
	balanceChecker := checker.NewBalanceChecker(pq)
	positionChecker := checker.NewPositionLimitChecker(pq, cfg.Risk.MaxPositionLots)
	cb := risk.NewCircuitBreaker(cfg.Risk.CircuitBreakerThreshold, cfg.Risk.CircuitBreakerCooldown)
	riskSvc := risk.NewService(sessionChecker, balanceChecker, positionChecker, cb, cfg.Risk, logger)

	reg := strategy.NewRegistry()
	sma.RegisterBuiltins(reg)
	orb.RegisterBuiltins(reg)

	deps := strategy.RunnerDeps{
		GRPC: func(endpoint string, timeout time.Duration) (strategy.Strategy, error) {
			return grpcadapter.New(endpoint, timeout, logger)
		},
	}

	var journ journal.Journal = journal.Noop{}
	if cfg.Features.EnableOrderJournal {
		jpath := cfg.Strategies.JournalPath
		if jpath == "" {
			jpath = "data/strategy_journal.db"
		}
		if err := os.MkdirAll(filepath.Dir(jpath), 0755); err != nil {
			return fmt.Errorf("journal dir: %w", err)
		}
		js, err := journal.OpenSQLite(jpath)
		if err != nil {
			return fmt.Errorf("journal sqlite: %w", err)
		}
		journ = js
		defer func() { _ = js.Close() }()
	}

	tg := telegram.New(cfg.Strategies.Notifications.Telegram.BotToken, cfg.Strategies.Notifications.Telegram.ChatID)

	runner := strategy.NewRunner(
		cfg,
		cfg.Strategies,
		reg,
		deps,
		tinvestClient,
		rlm,
		mdSvc,
		streamMgr,
		instrumentCache,
		priceCache,
		orderSvc,
		orderRepo,
		riskSvc,
		orderStream,
		portfolioSvc,
		journ,
		tg,
		logger,
	)

	runnerPort := cfg.Strategies.RunnerPort
	if runnerPort == 0 {
		runnerPort = 9020
	}
	metricsPort := cfg.Strategies.MetricsPort
	if metricsPort == 0 {
		metricsPort = 9120
	}

	admin := strategy.NewManagementHandler(ctx, runner)
	adminSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", runnerPort),
		Handler: admin,
	}
	go func() {
		logger.Info("strategy-runner management listening", "addr", adminSrv.Addr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("management server error", "error", err)
		}
	}()

	metricsSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", metricsPort),
		Handler: promhttp.Handler(),
	}
	go func() {
		logger.Info("strategy-runner metrics listening", "addr", metricsSrv.Addr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	runErr := make(chan error, 1)
	go func() {
		runErr <- runner.Start(ctx)
	}()

	select {
	case <-ctx.Done():
	case err := <-runErr:
		if err != nil && ctx.Err() == nil {
			logger.Error("runner stopped with error", "error", err)
			stop()
		}
	}
	shCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = adminSrv.Shutdown(shCtx)
	_ = metricsSrv.Shutdown(shCtx)
	logger.Info("strategy-runner stopped")
	return nil
}
