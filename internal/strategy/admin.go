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
			ID            string            `json:"id"`
			Type          string            `json:"type"`
			Tickers       string            `json:"tickers,omitempty"`
			AccountID     string            `json:"account_id"`
			Instruments   []string          `json:"instruments"`
			Params        map[string]string `json:"params,omitempty"`
			EnabledConfig bool              `json:"enabled_in_config"`
			Running       bool              `json:"running"`
		}
		var out []row
		for _, inst := range r.strategiesCfg.Instances {
			out = append(out, row{
				ID:            inst.ID,
				Type:          inst.Type,
				Tickers:       r.InstanceTickers(inst),
				AccountID:     inst.AccountID,
				Instruments:   append([]string(nil), inst.Instruments...),
				Params:        inst.Params,
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
	mux.HandleFunc("POST /config/reload", func(w http.ResponseWriter, _ *http.Request) {
		added, removed, changed, err := r.ReloadConfig(parent)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"added":   added,
			"removed": removed,
			"changed": changed,
		})
	})
	mux.HandleFunc("GET /instances/{id}/pnl", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		rl, un, tot, source, ok := r.InstancePNLBrokerAware(ctx, id)
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
			"source":         source,
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
	mux.HandleFunc("GET /instances/{id}/portfolio", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		data, ok, err := r.InstancePortfolio(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "unknown instance", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
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
	mux.HandleFunc("GET /instances/{id}/orders", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		limit := 50
		if v := req.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
				limit = n
			}
		}
		rows, err := r.InstanceRecentOrders(req.Context(), id, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	})
	mux.HandleFunc("GET /instances/{id}/stop-orders", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		rows, ok, err := r.InstanceStopOrders(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "unknown instance", http.StatusNotFound)
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
		ctx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
		defer cancel()
		data, ok := r.InstanceIndicatorForChart(ctx, id)
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
	mux.HandleFunc("GET /instruments/catalog", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query().Get("q")
		kind := req.URL.Query().Get("kind")
		limit := 40
		if v := req.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		rows, err := SearchInstruments(req.Context(), q, kind, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	})
	mux.HandleFunc("GET /ai-trader/sessions", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.AITraderSessions())
	})
	mux.HandleFunc("POST /ai-trader/sessions", func(w http.ResponseWriter, req *http.Request) {
		var in AITraderSessionRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, err := r.StartAITraderSession(parent, in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
	mux.HandleFunc("GET /ai-trader/sessions/{instance_id}", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		session, ok := r.AITraderSession(instanceID)
		if !ok {
			http.Error(w, "ai trader session not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
	mux.HandleFunc("POST /ai-trader/sessions/{instance_id}/stop", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		session, ok := r.StopAITraderSession(instanceID)
		if !ok {
			http.Error(w, "ai trader session not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
	mux.HandleFunc("POST /ai-trader/sessions/{instance_id}/start-trading", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		session, err := r.StartAITraderTrading(instanceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
	mux.HandleFunc("GET /ai-trader/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.AITraderPublicConfig())
	})
	mux.HandleFunc("POST /ai-trader/kill-switch", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.SetAITraderKillSwitch(body.Active)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"kill_switch": r.AITraderKillSwitch()})
	})
	mux.HandleFunc("GET /ai-trader/kill-switch", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"kill_switch": r.AITraderKillSwitch()})
	})
	mux.HandleFunc("POST /ai-trader/shadow-mode", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.SetAITraderShadowMode(body.Active)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"shadow_mode": r.AITraderShadowMode()})
	})
	mux.HandleFunc("GET /ai-trader/sessions/{instance_id}/eval", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		res, err := EvalTradeSignalsOnReplay(instanceID, 5)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})
	registerAIChatHandlers(mux, r, parent)

	mux.Handle("/dashboard/", http.StripPrefix("/dashboard", DashboardHandler()))
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
	})
	return corsMiddleware(mux)
}
