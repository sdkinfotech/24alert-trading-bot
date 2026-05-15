package strategy

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// corsMiddleware adds CORS headers for the web dashboard dev server.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// NewManagementHandler serves health, instance list, and start/stop controls.
func NewManagementHandler(parent context.Context, r *Runner) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /instances", func(w http.ResponseWriter, _ *http.Request) {
		type row struct {
			ID            string `json:"id"`
			Type          string `json:"type"`
			Tickers       string `json:"tickers,omitempty"`
			AccountID     string `json:"account_id"`
			EnabledConfig bool   `json:"enabled_in_config"`
			Running       bool   `json:"running"`
		}
		var out []row
		for _, inst := range r.strategiesCfg.Instances {
			out = append(out, row{
				ID:            inst.ID,
				Type:          inst.Type,
				Tickers:       r.InstanceTickers(inst),
				AccountID:     inst.AccountID,
				EnabledConfig: inst.Enabled,
				Running:       r.InstanceRunning(inst.ID),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("POST /instances/{id}/start", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		if err := r.StartInstanceByID(parent, id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /instances/{id}/stop", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		r.StopInstance(id)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /instances/{id}/pnl", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		rl, un, tot, ok := r.InstancePNL(id)
		if !ok {
			http.Error(w, "instance not running", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"instance_id":    id,
			"realized_rub":   rl,
			"unrealized_rub": un,
			"total_rub":      tot,
		})
	})
	mux.HandleFunc("GET /instances/{id}/ledger", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		qty, avg, realized, ok := r.InstanceLedgerPositions(id)
		if !ok {
			http.Error(w, "instance not running", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"instance_id":  id,
			"quantities":   qty,
			"avg_prices":   avg,
			"realized_rub": realized,
		})
	})
	mux.HandleFunc("GET /instances/{id}/executions", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		limit := 50
		if v := req.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
				limit = n
			}
		}
		rows, err := r.InstanceRecentExecutions(req.Context(), id, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	})
	mux.HandleFunc("GET /report/daily", func(w http.ResponseWriter, req *http.Request) {
		day := time.Now().UTC()
		if v := req.URL.Query().Get("date"); v != "" {
			if t, err := time.Parse("2006-01-02", v); err == nil {
				day = t.UTC()
			}
		}
		sum, err := r.DailyJournalSummary(req.Context(), day)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sum)
	})
	mux.HandleFunc("GET /instances/{id}/indicator", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		data, ok := r.InstanceIndicatorData(id)
		if !ok {
			http.Error(w, "instance not running or no indicator data", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
	})
	mux.HandleFunc("GET /instances/{id}/signals", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		limit := 200
		if v := req.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
				limit = n
			}
		}
		rows, err := r.InstanceRecentSignals(req.Context(), id, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	})
	mux.HandleFunc("GET /instances/{id}/events", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		limit := 200
		if v := req.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
				limit = n
			}
		}
		rows, err := r.InstanceEvents(req.Context(), id, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	})
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard", DashboardHandler()))
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
	})
	return corsMiddleware(mux)
}
