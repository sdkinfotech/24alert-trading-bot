package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/24alert/trading-bot/internal/advisor"
	"github.com/24alert/trading-bot/pkg/metrics"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	log := slog.Default()
	dbPath := advisorDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Error("advisor mkdir data", "error", err)
		os.Exit(1)
	}

	store, err := advisor.OpenStore(dbPath)
	if err != nil {
		log.Error("advisor open store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	svc := advisor.NewService(store, log)

	metricsPort := envInt("ADVISOR_METRICS_PORT", 9130)
	go func() {
		if err := metrics.ServeHTTP(ctx, metricsPort); err != nil {
			log.Error("advisor metrics server", "error", err)
		}
	}()

	addr := fmt.Sprintf(":%d", envInt("ADVISOR_PORT", 9030))
	log.Info("advisor-svc listening", "addr", addr, "db", dbPath)
	if err := advisor.ListenAndServe(ctx, addr, svc); err != nil {
		log.Error("advisor-svc stopped", "error", err)
		os.Exit(1)
	}
}

func advisorDBPath() string {
	if p := os.Getenv("ADVISOR_DB_PATH"); p != "" {
		return p
	}
	return "data/advisor_memory.db"
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
