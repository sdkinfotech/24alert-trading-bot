package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger, err := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output, cfg.Logging.FilePath)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}

	logger.Info("Order Service starting", slog.Int("port", cfg.Services.OrderPort))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start metrics server (Phase 2: Metrics Exposition)
	go func() {
		metricsPort := 9101
		logger.Info("Starting metrics server", slog.Int("port", metricsPort))
		if err := metrics.ServeHTTP(ctx, metricsPort); err != nil {
			logger.Error("Metrics server failed", "error", err)
		}
	}()

	if err := order.Run(ctx, cfg, logger); err != nil {
		logger.Error("Order Service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("Order Service stopped")
}
