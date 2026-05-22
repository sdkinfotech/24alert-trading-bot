package strategy

import (
	"context"
	"encoding/json"
	"errors"
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
	mux.HandleFunc("POST /instances/{id}/flatten", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
		defer cancel()
		res, err := r.FlattenInstance(ctx, id)
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, errFlattenNotRunning) || errors.Is(err, errFlattenNoPosition) {
				code = http.StatusConflict
			}
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("GET /instances/{id}/status", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		st, err := r.InstanceOperationalStatus(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st)
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
	mux.HandleFunc("GET /strategy-lab/catalog", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(strategyLabCatalog())
	})
	mux.HandleFunc("POST /strategy-lab/compare", func(w http.ResponseWriter, req *http.Request) {
		var body StrategyLabCompareRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 4*time.Minute)
		defer cancel()
		out, err := r.StrategyLabCompare(ctx, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	})
	mux.HandleFunc("POST /strategy-lab/optimize", func(w http.ResponseWriter, req *http.Request) {
		var body StrategyLabOptimizeRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 3*time.Minute)
		defer cancel()
		out, err := r.StrategyLabOptimize(ctx, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	})
	mux.HandleFunc("POST /strategy-lab/apply", func(w http.ResponseWriter, req *http.Request) {
		var body StrategyLabApplyRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 45*time.Second)
		defer cancel()
		res, err := r.StrategyLabApply(ctx, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
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
	mux.HandleFunc("GET /ai-trader/sessions/persisted", func(w http.ResponseWriter, _ *http.Request) {
		list, err := r.listPersistedAITraderSessions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []AITraderPersistedSummary{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("POST /ai-trader/sessions/{instance_id}/resume", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		var body struct {
			ReconnectOnly  bool `json:"reconnect_only"`
			ResumeTrading  bool `json:"resume_trading"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		reconnectOnly := body.ReconnectOnly && !body.ResumeTrading
		session, err := r.ResumeAITraderSession(parent, instanceID, reconnectOnly)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
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
	mux.HandleFunc("POST /ai-trader/sessions/{instance_id}/flatten", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		session, err := r.FlattenAITraderSession(instanceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
	mux.HandleFunc("GET /ai-trader/sessions/{instance_id}/broker-position", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		data, ok, err := r.AITraderBrokerSnapshot(ctx, instanceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "ai trader session not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
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
	mux.HandleFunc("POST /ai-trader/analyst/sessions/{instance_id}/postmarket", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		rep, err := r.RunTradeAnalystPostMarket(instanceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rep)
	})
	mux.HandleFunc("GET /ai-trader/analyst/sessions/{instance_id}/report", func(w http.ResponseWriter, req *http.Request) {
		instanceID := req.PathValue("instance_id")
		r.initTradeAnalyst()
		if r.tradeAnalyst == nil {
			http.Error(w, "trade analyst not available", http.StatusServiceUnavailable)
			return
		}
		rep, ok := r.tradeAnalyst.Store().GetReport(instanceID)
		if !ok {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rep)
	})
	mux.HandleFunc("GET /ai-trader/analyst/instruments/{ticker}/journal", func(w http.ResponseWriter, req *http.Request) {
		ticker := req.PathValue("ticker")
		r.initTradeAnalyst()
		if r.tradeAnalyst == nil {
			http.Error(w, "trade analyst not available", http.StatusServiceUnavailable)
			return
		}
		reports, _ := r.tradeAnalyst.Store().ListReportsByTicker(ticker, 30)
		rounds, _ := r.tradeAnalyst.Store().ListRoundsByTicker(ticker, 100)
		stats, _ := r.tradeAnalyst.Store().GetInstrumentStats(ticker)
		hints, _ := r.tradeAnalyst.Store().GetHints(ticker)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ticker": ticker, "stats": stats, "hints": hints,
			"reports": reports, "trade_rounds": rounds,
		})
	})
	registerAIChatHandlers(mux, r, parent)
	registerAssistantHandlers(mux, r, parent)

	mux.Handle("/dashboard/", http.StripPrefix("/dashboard", DashboardHandler()))
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
	})
	return corsMiddleware(mux)
}
