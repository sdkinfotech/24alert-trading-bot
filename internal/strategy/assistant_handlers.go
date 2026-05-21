package strategy

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func registerAssistantHandlers(mux *http.ServeMux, r *Runner, parent context.Context) {
	mux.HandleFunc("POST /assistant/analyses", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Ticker string `json:"ticker"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a, err := r.startAssistantAnalysis(parent, body.Ticker)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"analysis_id": a.ID,
			"status":      a.Status,
			"ticker":      a.Ticker,
		})
	})

	mux.HandleFunc("GET /assistant/analyses/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		a, ok := r.getAssistantAnalysis(id)
		if !ok {
			http.Error(w, "analysis not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(a)
	})

	mux.HandleFunc("GET /assistant/analyses/{id}/chart", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		tf := strings.TrimSpace(req.URL.Query().Get("tf"))
		if tf == "" {
			tf = "1h"
		}
		p, ok := r.assistantChartPayload(id, tf)
		if !ok {
			http.Error(w, "chart not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)
	})

	mux.HandleFunc("DELETE /assistant/analyses/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		r.deleteAssistantAnalysis(id)
		w.WriteHeader(http.StatusNoContent)
	})
}
