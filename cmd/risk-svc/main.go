package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/24alert/trading-bot/internal/risk"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
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

	logger.Info("Risk Service starting", slog.Int("port", cfg.Services.RiskPort))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := risk.Run(ctx, cfg, logger); err != nil {
		logger.Error("Risk Service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("Risk Service stopped")
}
