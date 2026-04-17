package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/24alert/trading-bot/docs"
	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
)

// Services groups all backend service implementations the gateway depends on.
type Services struct {
	Orders      handlers.OrderService
	StopOrders  handlers.StopOrderService
	MarketData  handlers.MarketDataService
	Portfolio   handlers.PortfolioService
	Accounts    handlers.AccountService
	Risk        handlers.RiskService
	Instruments handlers.InstrumentsService
	Stream      *handlers.StreamHandlers // optional, nil disables streaming
}

// Validate returns an error if any required service is nil.
func (s Services) Validate() error {
	if s.Orders == nil || s.StopOrders == nil || s.MarketData == nil ||
		s.Portfolio == nil || s.Accounts == nil || s.Risk == nil || s.Instruments == nil {
		return fmt.Errorf("gateway: all service implementations must be non-nil")
	}
	return nil
}

// Run starts the gateway HTTP server and blocks until ctx is cancelled.
func Run(ctx context.Context, cfg *config.Config, logger *logging.Logger, svcs Services) error {
	if err := svcs.Validate(); err != nil {
		return err
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(metricsMiddleware)
	r.Use(requestLogger(logger))

	r.Get("/health", healthHandler)
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	handlers.NewOrderHandlers(svcs.Orders).Routes(r)
	handlers.NewStopOrderHandlers(svcs.StopOrders).Routes(r)
	handlers.NewMarketDataHandlers(svcs.MarketData).Routes(r)
	handlers.NewPortfolioHandlers(svcs.Portfolio).Routes(r)
	handlers.NewAccountHandlers(svcs.Accounts).Routes(r)
	handlers.NewRiskHandlers(svcs.Risk).Routes(r)
	handlers.NewInstrumentsHandlers(svcs.Instruments).Routes(r)
	if svcs.Stream != nil {
		svcs.Stream.Routes(r)
	}

	addr := fmt.Sprintf(":%d", cfg.Services.GatewayPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func requestLogger(logger *logging.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration", time.Since(start).String(),
			)
		})
	}
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", ww.Status())
		path := r.URL.Path

		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		metrics.HTTPResponseSize.WithLabelValues(r.Method, path).Observe(float64(ww.BytesWritten()))
	})
}
